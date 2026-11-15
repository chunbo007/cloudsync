package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"qiniu-site-manager/internal/credentials"
	"qiniu-site-manager/internal/database"
	"qiniu-site-manager/internal/qiniu"
	"qiniu-site-manager/internal/syncer"
	"qiniu-site-manager/internal/updater"
)

const CurrentVersion = "v2.23.0"

type App struct {
	ctx         context.Context
	db          *database.DB
	credentials credentials.Store
	updater     *updater.Service
}

type AccountInput struct {
	ID        int64  `json:"id"`
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type BucketSelection struct {
	Name    string `json:"name"`
	Region  string `json:"region"`
	Private bool   `json:"private"`
}

type MappingInput struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	LocalPath    string `json:"localPath"`
	RemotePrefix string `json:"remotePrefix"`
}

type DeleteBucketInput struct {
	AccountID int64  `json:"accountId"`
	Bucket    string `json:"bucket"`
	Confirm   string `json:"confirm"`
}

type SyncMappingInput struct {
	ClearRemote      bool   `json:"clearRemote"`
	Overwrite        bool   `json:"overwrite"`
	HTMLCacheControl string `json:"htmlCacheControl"`
}

type SyncProgress struct {
	MappingID int64  `json:"mappingId"`
	Uploaded  int    `json:"uploaded"`
	Total     int    `json:"total"`
	Current   string `json:"current"`
	Status    string `json:"status"`
}

func NewApp() (*App, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取应用配置目录：%w", err)
	}
	appDir := filepath.Join(configDir, "QiniuSiteManager")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建应用配置目录：%w", err)
	}
	db, err := database.Open(filepath.Join(appDir, "qiniu-manager.db"))
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	store, err := credentials.NewDefaultStore(filepath.Join(appDir, "credentials.json"))
	if err != nil {
		db.Close()
		return nil, err
	}
	return &App{db: db, credentials: store, updater: updater.NewService(updater.DefaultManifestURL)}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) CheckForUpdate() (updater.Result, error) {
	return a.updater.Check(a.context(), CurrentVersion)
}

func (a *App) DownloadAndInstallUpdate(manifest updater.Manifest) error {
	updatePath, err := a.updater.Download(a.context(), manifest)
	if err != nil {
		return err
	}
	if err := updater.Install(updatePath); err != nil {
		_ = os.Remove(updatePath)
		return err
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		runtime.Quit(a.context())
	}()
	return nil
}

func (a *App) ListAccounts() ([]database.Account, error) {
	return a.db.ListQiniuAccounts(a.context())
}

func (a *App) SaveAccount(input AccountInput) (int64, error) {
	if input.Provider == "" {
		input.Provider = "qiniu"
	}
	if input.Provider != "qiniu" {
		return 0, fmt.Errorf("当前只支持七牛云")
	}
	if input.Name == "" || input.AccessKey == "" {
		return 0, fmt.Errorf("配置名称和 AccessKey 不能为空")
	}

	secretRef := "account/qiniu/" + input.Name
	if input.SecretKey == "" && input.ID > 0 {
		existing, err := a.db.GetQiniuAccount(a.context(), input.ID)
		if err != nil {
			return 0, err
		}
		secretRef = existing.SecretRef
	} else {
		if input.SecretKey == "" {
			return 0, fmt.Errorf("新账号必须填写 SecretKey")
		}
		if err := a.credentials.Save(a.context(), secretRef, []byte(input.SecretKey)); err != nil {
			return 0, err
		}
	}
	if input.ID > 0 {
		if err := a.db.UpdateQiniuAccount(a.context(), input.ID, input.Name, input.AccessKey, secretRef); err != nil {
			return 0, err
		}
	} else {
		_, err := a.db.SaveQiniuAccount(a.context(), database.Account{
			Provider:  input.Provider,
			Name:      input.Name,
			AccessKey: input.AccessKey,
			SecretRef: secretRef,
		})
		if err != nil {
			return 0, err
		}
	}
	accounts, err := a.db.ListQiniuAccounts(a.context())
	if err != nil {
		return 0, err
	}
	for _, saved := range accounts {
		if saved.Name == input.Name {
			return saved.ID, nil
		}
	}
	return 0, fmt.Errorf("保存账号后未找到账号")
}

func (a *App) DeleteAccount(accountID int64) error {
	account, err := a.db.GetQiniuAccount(a.context(), accountID)
	if err != nil {
		return err
	}
	if err := a.db.DeleteQiniuAccount(a.context(), accountID); err != nil {
		return err
	}
	return a.credentials.Delete(a.context(), account.SecretRef)
}

func (a *App) TestConnection(input AccountInput) ([]qiniu.BucketInfo, error) {
	if input.SecretKey == "" && input.ID > 0 {
		account, err := a.db.GetQiniuAccount(a.context(), input.ID)
		if err != nil {
			return nil, err
		}
		secret, err := a.credentials.Load(a.context(), account.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("本地账号凭据无效，请编辑账号并重新保存 SK：%w", err)
		}
		input.AccessKey = account.AccessKey
		input.SecretKey = string(secret)
	}
	client, err := qiniu.NewClient(qiniu.Credentials{
		AccessKey: input.AccessKey,
		SecretKey: input.SecretKey,
	}, "", "")
	if err != nil {
		return nil, err
	}
	buckets, err := client.ListBuckets()
	if err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		return nil, fmt.Errorf("七牛接口返回 0 个 Bucket，请确认 AK/SK 对应账号以及该账号是否能在控制台看到 Bucket")
	}
	return buckets, nil
}

func (a *App) ListConfiguredBuckets(accountID int64) ([]database.Bucket, error) {
	return a.db.ListBuckets(a.context(), accountID, true)
}

func (a *App) DeleteBucket(input DeleteBucketInput) (qiniu.DeleteBucketResult, error) {
	input.Bucket = strings.TrimSpace(input.Bucket)
	if input.AccountID <= 0 || input.Bucket == "" {
		return qiniu.DeleteBucketResult{}, fmt.Errorf("账号和 Bucket 不能为空")
	}
	if input.Confirm != input.Bucket {
		return qiniu.DeleteBucketResult{}, fmt.Errorf("请输入完整的 Bucket 名称确认删除")
	}
	account, err := a.db.GetQiniuAccount(a.context(), input.AccountID)
	if err != nil {
		return qiniu.DeleteBucketResult{}, err
	}
	secret, err := a.credentials.Load(a.context(), account.SecretRef)
	if err != nil {
		return qiniu.DeleteBucketResult{}, fmt.Errorf("本地账号凭据无效，请编辑账号并重新保存 SK：%w", err)
	}
	client, err := qiniu.NewClient(qiniu.Credentials{
		AccessKey: account.AccessKey,
		SecretKey: string(secret),
	}, input.Bucket, bucketRegion(a.context(), a.db, input.AccountID, input.Bucket))
	if err != nil {
		return qiniu.DeleteBucketResult{}, err
	}
	result, err := client.DeleteBucket()
	if err != nil {
		return result, err
	}
	if err := a.db.DeleteBucket(a.context(), input.AccountID, input.Bucket); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) SaveBucketSelections(accountID int64, selections []BucketSelection) error {
	buckets := make([]database.Bucket, 0, len(selections))
	for _, selection := range selections {
		if strings.TrimSpace(selection.Name) == "" {
			continue
		}
		buckets = append(buckets, database.Bucket{
			AccountID: accountID,
			Name:      selection.Name,
			Region:    selection.Region,
			Private:   selection.Private,
		})
	}
	return a.db.SaveBucketSelections(a.context(), accountID, buckets)
}

func (a *App) ListBucketFolders(accountID int64, bucketName string) ([]qiniu.RemoteFolder, error) {
	account, err := a.db.GetQiniuAccount(a.context(), accountID)
	if err != nil {
		return nil, err
	}
	secret, err := a.credentials.Load(a.context(), account.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("本地账号凭据无效，请编辑账号并重新保存 SK：%w", err)
	}
	client, err := qiniu.NewClient(qiniu.Credentials{
		AccessKey: account.AccessKey,
		SecretKey: string(secret),
	}, bucketName, bucketRegion(a.context(), a.db, accountID, bucketName))
	if err != nil {
		return nil, err
	}
	return client.ListRootFolders()
}

func (a *App) ChooseDirectory() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用尚未启动")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择本地目录"})
}

func (a *App) OpenLocalDirectory(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("本地目录不能为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("本地目录不可用：%w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("本地路径不是目录")
	}
	return exec.Command("explorer.exe", filepath.Clean(path)).Start()
}

func (a *App) ListMappings(accountID int64, bucketName string) ([]database.Site, error) {
	return a.db.ListSitesForBucket(a.context(), accountID, bucketName)
}

func (a *App) ListAllMappings() ([]database.Site, error) {
	return a.db.ListAllSites(a.context())
}

func (a *App) SaveMapping(accountID int64, bucketName string, input MappingInput) error {
	if input.Name == "" {
		input.Name = filepath.Base(filepath.Clean(input.LocalPath))
	}
	remotePrefix := strings.Trim(strings.TrimSpace(input.RemotePrefix), "/")
	if input.ID > 0 {
		return a.db.UpdateSite(a.context(), input.ID, input.Name, input.LocalPath, remotePrefixWithSlash(remotePrefix))
	}
	_, err := a.db.SaveSiteForBucket(a.context(), accountID, bucketName, database.Site{
		Name:         input.Name,
		RemotePrefix: remotePrefixWithSlash(remotePrefix),
		LocalPath:    input.LocalPath,
		VersionFile:  "",
	})
	return err
}

func remotePrefixWithSlash(prefix string) string {
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

func (a *App) DeleteMapping(mappingID int64) error {
	return a.db.DeleteSite(a.context(), mappingID)
}

func (a *App) SyncMapping(mappingID int64, input SyncMappingInput) (syncer.Result, error) {
	site, err := a.db.GetSite(a.context(), mappingID)
	if err != nil {
		return syncer.Result{}, err
	}
	account, err := a.db.GetQiniuAccount(a.context(), site.AccountID)
	if err != nil {
		return syncer.Result{}, err
	}
	buckets, err := a.db.ListBuckets(a.context(), site.AccountID, false)
	if err != nil {
		return syncer.Result{}, err
	}
	var bucketName string
	for _, bucket := range buckets {
		if bucket.ID == site.BucketID {
			bucketName = bucket.Name
		}
	}
	if bucketName == "" {
		return syncer.Result{}, fmt.Errorf("找不到路径映射所属 Bucket")
	}
	secret, err := a.credentials.Load(a.context(), account.SecretRef)
	if err != nil {
		return syncer.Result{}, fmt.Errorf("本地账号凭据无效，请编辑账号并重新保存 SK：%w", err)
	}
	client, err := qiniu.NewClient(qiniu.Credentials{
		AccessKey: account.AccessKey,
		SecretKey: string(secret),
	}, bucketName, bucketRegion(a.context(), a.db, site.AccountID, bucketName))
	if err != nil {
		return syncer.Result{}, err
	}
	return syncer.UploadDirectory(a.context(), client, site.LocalPath, syncer.Options{
		ClearRemote:      input.ClearRemote,
		Overwrite:        input.Overwrite,
		RemotePrefix:     site.RemotePrefix,
		HTMLCacheControl: input.HTMLCacheControl,
		Progress: func(uploaded, total int, current, status string) {
			runtime.EventsEmit(a.context(), "sync-progress", SyncProgress{
				MappingID: mappingID,
				Uploaded:  uploaded,
				Total:     total,
				Current:   current,
				Status:    status,
			})
		},
	})
}

func bucketRegion(ctx context.Context, db *database.DB, accountID int64, bucketName string) string {
	buckets, err := db.ListBuckets(ctx, accountID, false)
	if err != nil {
		return ""
	}
	for _, bucket := range buckets {
		if bucket.Name == bucketName && bucket.Region != "auto" {
			return bucket.Region
		}
	}
	return ""
}

func (a *App) context() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}
