package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"qiniu-site-manager/internal/qiniu"
)

type Version struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime,omitempty"`
}

func CompareVersions(left, right string) int {
	leftParts := strings.Split(strings.TrimPrefix(left, "v"), ".")
	rightParts := strings.Split(strings.TrimPrefix(right, "v"), ".")
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftValue := 0
		rightValue := 0
		if index < len(leftParts) {
			fmt.Sscanf(leftParts[index], "%d", &leftValue)
		}
		if index < len(rightParts) {
			fmt.Sscanf(rightParts[index], "%d", &rightValue)
		}
		if leftValue > rightValue {
			return 1
		}
		if leftValue < rightValue {
			return -1
		}
	}
	return 0
}

type Options struct {
	ClearRemote      bool
	Overwrite        bool
	RefreshCDN       bool
	CDNDomain        string
	VersionFile      string
	RemotePrefix     string
	HTMLCacheControl string
	Progress         func(uploaded, total int, current, status string)
}

type Result struct {
	Uploaded      int      `json:"uploaded"`
	Deleted       int      `json:"deleted"`
	RefreshedURLs int      `json:"refreshedUrls"`
	LocalVersion  string   `json:"localVersion,omitempty"`
	Files         []string `json:"files"`
}

func ReadVersion(path, versionFile string) (Version, error) {
	data, err := os.ReadFile(filepath.Join(path, versionFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Version{}, nil
		}
		return Version{}, fmt.Errorf("读取版本文件：%w", err)
	}
	return ParseVersion(data)
}

func ParseVersion(data []byte) (Version, error) {
	var version Version
	if err := json.Unmarshal(data, &version); err != nil {
		return Version{}, fmt.Errorf("解析版本文件：%w", err)
	}
	if strings.TrimSpace(version.Version) == "" {
		return Version{}, fmt.Errorf("版本文件缺少 version 字段")
	}
	return version, nil
}

func UploadDirectory(ctx context.Context, client *qiniu.Client, localPath string, options Options) (Result, error) {
	if client == nil {
		return Result{}, fmt.Errorf("七牛客户端不能为空")
	}
	if _, err := os.Stat(localPath); err != nil {
		return Result{}, fmt.Errorf("本地目录不可用：%w", err)
	}

	if options.ClearRemote && options.RemotePrefix != "" {
		if err := client.DeletePrefix(options.RemotePrefix); err != nil {
			return Result{}, err
		}
	}
	total, err := countFiles(localPath)
	if err != nil {
		return Result{}, fmt.Errorf("统计本地文件：%w", err)
	}

	result := Result{Files: make([]string, 0)}
	var refreshURLs []string
	err = filepath.WalkDir(localPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		if options.RemotePrefix != "" {
			key = strings.TrimRight(options.RemotePrefix, "/") + "/" + key
		}
		if options.Progress != nil {
			options.Progress(result.Uploaded, total, key, "uploading")
		}
		cacheControl := ""
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".html" || extension == ".htm" {
			cacheControl = options.HTMLCacheControl
		}
		uploaded, err := client.UploadFile(ctx, path, key, qiniu.UploadOptions{
			Overwrite:    options.Overwrite || options.ClearRemote,
			CacheControl: cacheControl,
		})
		if err != nil {
			return err
		}
		result.Uploaded++
		result.Files = append(result.Files, uploaded.Key)
		if options.Progress != nil {
			options.Progress(result.Uploaded, total, key, "uploaded")
		}

		if options.RefreshCDN && options.CDNDomain != "" && (extension == ".html" || extension == ".htm") {
			url, buildErr := qiniu.BuildURL(options.CDNDomain, key)
			if buildErr != nil {
				return buildErr
			}
			refreshURLs = append(refreshURLs, url)
			if strings.EqualFold(key, "index.html") {
				rootURL := strings.TrimRight(strings.TrimSpace(options.CDNDomain), "/") + "/"
				refreshURLs = append(refreshURLs, rootURL)
			}
		}
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("上传网站目录：%w", err)
	}

	if options.RefreshCDN && len(refreshURLs) > 0 {
		if err := client.RefreshURLs(refreshURLs); err != nil {
			return result, err
		}
		result.RefreshedURLs = len(refreshURLs)
	}

	if options.VersionFile != "" {
		version, err := ReadVersion(localPath, options.VersionFile)
		if err == nil {
			result.LocalVersion = version.Version
		}
	}

	return result, nil
}

func countFiles(localPath string) (int, error) {
	total := 0
	err := filepath.WalkDir(localPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			total++
		}
		return nil
	})
	return total, err
}
