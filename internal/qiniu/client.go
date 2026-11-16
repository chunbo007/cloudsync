package qiniu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/cdn"
	"github.com/qiniu/go-sdk/v7/storage"
)

type Credentials struct {
	AccessKey string
	SecretKey string
}

type Client struct {
	credentials auth.Credentials
	config      storage.Config
	bucket      string
	manager     *storage.BucketManager
	uploader    *storage.FormUploader
	cdnManager  *cdn.CdnManager
}

type RemoteFolder struct {
	Prefix string `json:"prefix"`
}

type BucketInfo struct {
	Name    string `json:"name"`
	Region  string `json:"region"`
	Private bool   `json:"private"`
}

type UploadOptions struct {
	Overwrite    bool
	CacheControl string
}

type UploadResult struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
	Hash string `json:"hash,omitempty"`
}

type DeleteBucketResult struct {
	Bucket        string `json:"bucket"`
	Scanned       int    `json:"scanned"`
	Deleted       int    `json:"deleted"`
	AlreadyAbsent int    `json:"alreadyAbsent"`
}

func NewClient(credentials Credentials, bucket, region string) (*Client, error) {
	if credentials.AccessKey == "" || credentials.SecretKey == "" {
		return nil, fmt.Errorf("AccessKey 和 SecretKey 不能为空")
	}
	var regionInfo storage.Region
	var ok bool
	var err error
	if bucket == "" {
		regionInfo = storage.Region{}
	} else if region == "" || region == "auto" {
		zone, zoneErr := storage.GetZone(credentials.AccessKey, bucket)
		err = zoneErr
		if err != nil {
			return nil, fmt.Errorf("自动识别七牛区域：%w", err)
		}
		regionInfo = *zone
	} else {
		regionInfo, ok = storage.GetRegionByID(storage.RegionID(region))
		if !ok {
			return nil, fmt.Errorf("不支持的七牛区域：%s", region)
		}
	}

	mac := auth.New(credentials.AccessKey, credentials.SecretKey)
	config := storage.Config{
		Region:   &regionInfo,
		UseHTTPS: true,
	}

	return &Client{
		credentials: *mac,
		config:      config,
		bucket:      bucket,
		manager:     storage.NewBucketManager(mac, &config),
		uploader:    storage.NewFormUploader(&config),
		cdnManager:  cdn.NewCdnManager(mac),
	}, nil
}

func (c *Client) ListRootFolders() ([]RemoteFolder, error) {
	if err := c.requireBucket(); err != nil {
		return nil, err
	}
	folders := make([]RemoteFolder, 0)
	marker := ""
	for {
		_, prefixes, nextMarker, hasNext, err := c.manager.ListFiles(c.bucket, "", "/", marker, 1000)
		if err != nil {
			return nil, fmt.Errorf("读取 Bucket 根目录：%w", err)
		}
		for _, prefix := range prefixes {
			folders = append(folders, RemoteFolder{Prefix: strings.TrimSuffix(prefix, "/")})
		}
		if !hasNext {
			break
		}
		marker = nextMarker
	}
	return folders, nil
}

func (c *Client) ListBuckets() ([]BucketInfo, error) {
	result := make([]BucketInfo, 0)
	marker := ""
	for {
		output, err := c.manager.BucketsV4(&storage.BucketV4Input{
			Region: "",
			Limit:  100,
			Marker: marker,
		})
		if err != nil {
			return nil, fmt.Errorf("读取 Bucket 列表：%w", err)
		}
		for _, bucket := range output.Buckets {
			result = append(result, BucketInfo{
				Name:    bucket.Name,
				Region:  bucket.Region,
				Private: bucket.Private,
			})
		}
		if !output.IsTruncated {
			break
		}
		marker = output.NextMarker
	}
	return result, nil
}

func (c *Client) ListKeys(prefix string) ([]string, error) {
	if err := c.requireBucket(); err != nil {
		return nil, err
	}
	keys := make([]string, 0)
	marker := ""
	for {
		entries, _, nextMarker, hasNext, err := c.manager.ListFiles(c.bucket, prefix, "", marker, 1000)
		if err != nil {
			return nil, fmt.Errorf("读取远程文件列表：%w", err)
		}
		for _, entry := range entries {
			keys = append(keys, entry.Key)
		}
		if !hasNext {
			return keys, nil
		}
		marker = nextMarker
	}
}

func (c *Client) DeletePrefix(prefix string) error {
	if err := c.requireBucket(); err != nil {
		return err
	}
	keys, err := c.ListKeys(prefix)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := c.manager.Delete(c.bucket, key); err != nil {
			return fmt.Errorf("删除远程文件 %q：%w", key, err)
		}
	}
	return nil
}

func (c *Client) DeleteBucket() (DeleteBucketResult, error) {
	if err := c.requireBucket(); err != nil {
		return DeleteBucketResult{}, err
	}

	result := DeleteBucketResult{Bucket: c.bucket}
	keys, err := c.ListKeys("")
	if err != nil {
		return result, fmt.Errorf("扫描 Bucket 文件：%w", err)
	}
	result.Scanned = len(keys)

	for start := 0; start < len(keys); start += 200 {
		end := start + 200
		if end > len(keys) {
			end = len(keys)
		}
		operations := make([]string, 0, end-start)
		for _, key := range keys[start:end] {
			operations = append(operations, storage.URIDelete(c.bucket, key))
		}
		responses, batchErr := c.manager.Batch(operations)
		if batchErr != nil {
			return result, fmt.Errorf("批量删除文件：%w", batchErr)
		}
		for index, response := range responses {
			switch response.Code {
			case 200:
				result.Deleted++
			case 612:
				result.AlreadyAbsent++
			default:
				key := keys[start+index]
				return result, fmt.Errorf("删除文件 %q 失败（七牛错误码 %d）：%s", key, response.Code, response.Data.Error)
			}
		}
	}

	remaining, err := c.ListKeys("")
	if err != nil {
		return result, fmt.Errorf("确认 Bucket 已清空：%w", err)
	}
	if len(remaining) > 0 {
		return result, fmt.Errorf("Bucket 仍有 %d 个文件未删除，请重试", len(remaining))
	}
	if err := c.manager.DropBucket(c.bucket); err != nil {
		return result, fmt.Errorf("删除 Bucket：%w", err)
	}
	return result, nil
}

func (c *Client) UploadFile(ctx context.Context, localPath, key string, options UploadOptions) (UploadResult, error) {
	if err := c.requireBucket(); err != nil {
		return UploadResult{}, err
	}
	deadline := uint64(time.Now().Add(time.Hour).Unix())
	policy := storage.PutPolicy{
		Scope:      c.bucket + ":" + key,
		Expires:    deadline,
		InsertOnly: 0,
		DetectMime: 0,
	}
	if !options.Overwrite {
		policy.InsertOnly = 1
	}

	token := policy.UploadToken(&c.credentials)
	if options.CacheControl != "" {
		return c.uploadFileWithCacheControl(ctx, localPath, key, token, options.CacheControl)
	}
	var result storage.PutRet
	if err := c.uploader.PutFile(ctx, &result, token, key, localPath, nil); err != nil {
		return UploadResult{}, fmt.Errorf("上传 %q：%w", key, err)
	}
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("读取上传文件大小：%w", err)
	}

	return UploadResult{
		Key:  key,
		Size: fileInfo.Size(),
		Hash: result.Hash,
	}, nil
}

func (c *Client) uploadFileWithCacheControl(ctx context.Context, localPath, key, token, cacheControl string) (UploadResult, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("打开上传文件：%w", err)
	}
	defer file.Close()

	upHost := c.config.UpHost
	if upHost == "" && c.config.Region != nil && len(c.config.Region.SrcUpHosts) > 0 {
		upHost = c.config.Region.SrcUpHosts[0]
	}
	if upHost == "" {
		return UploadResult{}, fmt.Errorf("无法确定七牛上传地址")
	}
	if !strings.HasPrefix(upHost, "http://") && !strings.HasPrefix(upHost, "https://") {
		upHost = "https://" + upHost
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("token", token); err != nil {
		return UploadResult{}, err
	}
	if err := writer.WriteField("key", key); err != nil {
		return UploadResult{}, err
	}
	if err := writer.WriteField("cacheControl", cacheControl); err != nil {
		return UploadResult{}, err
	}
	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return UploadResult{}, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return UploadResult{}, fmt.Errorf("读取上传文件：%w", err)
	}
	if err := writer.Close(); err != nil {
		return UploadResult{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, upHost, &body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("创建上传请求：%w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return UploadResult{}, fmt.Errorf("上传 %q：%w", key, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("读取上传响应：%w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return UploadResult{}, fmt.Errorf("上传 %q 失败（HTTP %d）：%s", key, response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var result storage.PutRet
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return UploadResult{}, fmt.Errorf("解析上传响应：%w", err)
	}
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("读取上传文件大小：%w", err)
	}
	return UploadResult{Key: key, Size: fileInfo.Size(), Hash: result.Hash}, nil
}

func (c *Client) requireBucket() error {
	if c.bucket == "" {
		return fmt.Errorf("Bucket 不能为空")
	}
	return nil
}

func (c *Client) RefreshURLs(urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	for start := 0; start < len(urls); start += 100 {
		end := start + 100
		if end > len(urls) {
			end = len(urls)
		}
		batch := urls[start:end]
		response, err := c.cdnManager.RefreshUrls(batch)
		if err != nil {
			return fmt.Errorf("刷新 CDN：%w", err)
		}
		if response.Code != 200 {
			if response.Error != "" {
				return fmt.Errorf("刷新 CDN：%s（错误码 %d）", response.Error, response.Code)
			}
			return fmt.Errorf("刷新 CDN 失败（错误码 %d）", response.Code)
		}
		if len(response.InvalidUrls) > 0 {
			return fmt.Errorf("刷新 CDN：存在无效地址：%s", strings.Join(response.InvalidUrls, ", "))
		}
	}
	return nil
}

func BuildURL(domain, key string) (string, error) {
	domain = strings.TrimRight(strings.TrimSpace(domain), "/")
	if domain == "" {
		return "", fmt.Errorf("CDN 域名不能为空")
	}
	parts := strings.Split(strings.TrimLeft(key, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return domain + "/" + filepath.ToSlash(strings.Join(parts, "/")), nil
}

func (c *Client) ReadObject(ctx context.Context, domain, key string) ([]byte, error) {
	objectURL, err := BuildURL(domain, key)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建对象请求：%w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("读取对象：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("读取对象返回 HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取对象内容：%w", err)
	}
	return data, nil
}
