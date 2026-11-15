package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const DefaultManifestURL = "https://github.com/chunbo007/cloudsync/releases/latest/download/latest.json"

type Manifest struct {
	Version  string `json:"version"`
	Download string `json:"download"`
	SHA256   string `json:"sha256"`
	Notes    string `json:"notes"`
}

type Result struct {
	Available bool     `json:"available"`
	Current   string   `json:"current"`
	Manifest  Manifest `json:"manifest"`
}

type Service struct {
	Client      *http.Client
	ManifestURL string
}

func NewService(manifestURL string) *Service {
	if strings.TrimSpace(manifestURL) == "" {
		manifestURL = DefaultManifestURL
	}
	return &Service{
		Client:      &http.Client{Timeout: 20 * time.Second},
		ManifestURL: manifestURL,
	}
}

func (s *Service) Check(ctx context.Context, current string) (Result, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.ManifestURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("创建更新请求：%w", err)
	}
	response, err := s.client().Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("检查更新：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Result{}, fmt.Errorf("检查更新返回 HTTP %d", response.StatusCode)
	}
	var manifest Manifest
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&manifest); err != nil {
		return Result{}, fmt.Errorf("解析更新清单：%w", err)
	}
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Download = strings.TrimSpace(manifest.Download)
	if manifest.Version == "" || manifest.Download == "" {
		return Result{}, fmt.Errorf("更新清单缺少 version 或 download")
	}
	return Result{
		Available: CompareVersions(manifest.Version, current) > 0,
		Current:   current,
		Manifest:  manifest,
	}, nil
}

func (s *Service) Download(ctx context.Context, manifest Manifest) (string, error) {
	if strings.TrimSpace(manifest.Download) == "" {
		return "", fmt.Errorf("更新下载地址为空")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifest.Download, nil)
	if err != nil {
		return "", fmt.Errorf("创建下载请求：%w", err)
	}
	response, err := s.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("下载更新：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("下载更新返回 HTTP %d", response.StatusCode)
	}
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "cloud-storage-sync-update-*.exe")
	if err != nil {
		return "", fmt.Errorf("创建更新临时文件：%w", err)
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		_ = tempFile.Close()
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := io.Copy(tempFile, response.Body); err != nil {
		return "", fmt.Errorf("保存更新文件：%w", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("关闭更新文件：%w", err)
	}
	if manifest.SHA256 != "" {
		if err := verifySHA256(tempPath, manifest.SHA256); err != nil {
			return "", err
		}
	}
	removeTemp = false
	return tempPath, nil
}

func Install(updatePath string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("当前自动更新仅支持 Windows")
	}
	updatePath, err := filepath.Abs(updatePath)
	if err != nil {
		return fmt.Errorf("解析更新文件路径：%w", err)
	}
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径：%w", err)
	}
	currentPath, err = filepath.Abs(currentPath)
	if err != nil {
		return fmt.Errorf("解析当前程序路径：%w", err)
	}
	if filepath.Ext(updatePath) != ".exe" {
		return fmt.Errorf("更新文件必须是 EXE")
	}
	scriptPath, err := os.CreateTemp(os.TempDir(), "cloud-storage-sync-updater-*.cmd")
	if err != nil {
		return fmt.Errorf("创建更新脚本：%w", err)
	}
	scriptName := scriptPath.Name()
	defer scriptPath.Close()
	pid := os.Getpid()
	backupPath := currentPath + ".backup"
	script := "@echo off\r\n" +
		":wait\r\n" +
		"tasklist /FI " + quoteCmd("PID eq "+strconv.Itoa(pid)) + " | find " + quoteCmd(strconv.Itoa(pid)) + " >nul\r\n" +
		"if not errorlevel 1 (timeout /t 1 /nobreak >nul & goto wait)\r\n" +
		"copy /Y " + quoteCmd(currentPath) + " " + quoteCmd(backupPath) + " >nul\r\n" +
		"move /Y " + quoteCmd(updatePath) + " " + quoteCmd(currentPath) + " >nul\r\n" +
		"if errorlevel 1 (copy /Y " + quoteCmd(backupPath) + " " + quoteCmd(currentPath) + " >nul & del /Q " + quoteCmd(updatePath) + " & exit /b 1)\r\n" +
		"start " + quoteCmd("") + " " + quoteCmd(currentPath) + "\r\n" +
		"del /Q " + quoteCmd(backupPath) + " >nul\r\n" +
		"del /Q " + quoteCmd(scriptName) + " >nul\r\n"
	if _, err := scriptPath.WriteString(script); err != nil {
		return fmt.Errorf("写入更新脚本：%w", err)
	}
	if err := scriptPath.Close(); err != nil {
		return fmt.Errorf("保存更新脚本：%w", err)
	}
	command := exec.Command("cmd.exe", "/C", scriptName)
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动更新程序：%w", err)
	}
	return nil
}

func CompareVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	for len(leftParts) < len(rightParts) {
		leftParts = append(leftParts, 0)
	}
	for len(rightParts) < len(leftParts) {
		rightParts = append(rightParts, 0)
	}
	for index := range leftParts {
		if leftParts[index] > rightParts[index] {
			return 1
		}
		if leftParts[index] < rightParts[index] {
			return -1
		}
	}
	return 0
}

func versionParts(version string) []int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		number := 0
		for _, char := range part {
			if char < '0' || char > '9' {
				break
			}
			number = number*10 + int(char-'0')
		}
		result = append(result, number)
	}
	return result
}

func (s *Service) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func verifySHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取更新文件：%w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("计算更新文件校验值：%w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return fmt.Errorf("更新文件校验失败")
	}
	return nil
}

func quoteCmd(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
