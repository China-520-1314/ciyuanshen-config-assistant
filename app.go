package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateManifestURL   = "https://api.ciyuanshen.top/downloads/ciyuanshen-config-assistant/update.json"
	githubReleaseAPIURL = "https://api.github.com/repos/China-520-1314/ciyuanshen-config-assistant/releases/latest"
	defaultGatewayURL   = "https://api.ciyuanshen.top/v1"
	claudeGatewayURL    = "https://api.ciyuanshen.top"
	geminiGatewayURL    = "https://api.ciyuanshen.top"
	geminiAPIVersion    = "v1"
	managedProviderName = "ciyuanshen"
	// Update installers can be several megabytes and GitHub may take longer
	// than routine API calls to begin or finish a download.
	updateDownloadTimeout = 10 * time.Minute
)

// appVersion is a variable so release builds can inject their tag with
// -ldflags "-X main.appVersion=..." while local builds keep a useful default.
var appVersion = "0.2.11"

type InstallUpdateResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// App is the bridge exposed to the Wails frontend. It never persists the API
// key in the assistant's own data directory; the key only lives in memory for
// the duration of a scan or configuration transaction.
type App struct {
	ctx         context.Context
	client      *http.Client
	operation   sync.Mutex
	lifecycleMu sync.Mutex
	accountMu   sync.RWMutex
	account     dashboardSession
	provisionMu sync.Mutex
	provisions  map[string]provisionedToolKey
}

type AppInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	UpdateManifestURL string `json:"updateManifestUrl"`
	GatewayURL        string `json:"gatewayUrl"`
}

type ClientStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Supported      bool   `json:"supported"`
	Installed      bool   `json:"installed"`
	ExecutablePath string `json:"executablePath"`
	ConfigPath     string `json:"configPath"`
	ConfigExists   bool   `json:"configExists"`
	ConfigState    string `json:"configState"`
	Version        string `json:"version"`
	Detail         string `json:"detail"`
}

type EnvironmentReport struct {
	OS        string         `json:"os"`
	Home      string         `json:"home"`
	ScannedAt time.Time      `json:"scannedAt"`
	Clients   []ClientStatus `json:"clients"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type ModelResponse struct {
	Models   []Model `json:"models"`
	Status   int     `json:"status"`
	Message  string  `json:"message,omitempty"`
	Endpoint string  `json:"endpoint"`
}

type ConfigurationRequest struct {
	APIKey  string            `json:"apiKey"`
	Targets []string          `json:"targets"`
	Models  map[string]string `json:"models"`
}

type FilePreview struct {
	ClientID string `json:"clientId"`
	Path     string `json:"path"`
	Action   string `json:"action"`
}

type ConfigurationPreview struct {
	Files    []FilePreview `json:"files"`
	Warnings []string      `json:"warnings"`
	Error    string        `json:"error,omitempty"`
}

type ConfigureResult struct {
	Success    bool          `json:"success"`
	Backup     *BackupInfo   `json:"backup,omitempty"`
	Files      []FilePreview `json:"files"`
	Warnings   []string      `json:"warnings"`
	Error      string        `json:"error,omitempty"`
	Configured []string      `json:"configured"`
	FinishedAt time.Time     `json:"finishedAt"`
}

// NewApp creates the application service.
func NewApp() *App {
	return &App{
		client:     &http.Client{Timeout: 15 * time.Second},
		provisions: map[string]provisionedToolKey{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:              "词元神配置助手",
		Version:           appVersion,
		UpdateManifestURL: updateManifestURL,
		GatewayURL:        defaultGatewayURL,
	}
}

func (a *App) ScanEnvironment() EnvironmentReport {
	return scanEnvironment()
}

func (a *App) FetchModels(apiKey string) (ModelResponse, error) {
	return a.fetchGatewayModels(apiKey)
}

func (a *App) PreviewConfiguration(request ConfigurationRequest) ConfigurationPreview {
	operations, warnings, err := buildConfiguration(request)
	preview := ConfigurationPreview{Warnings: warnings}
	if err != nil {
		preview.Error = err.Error()
		return preview
	}
	for _, operation := range operations {
		preview.Files = append(preview.Files, FilePreview{
			ClientID: operation.ClientID,
			Path:     operation.Path,
			Action:   operation.Action,
		})
	}
	return preview
}

func (a *App) Configure(request ConfigurationRequest) ConfigureResult {
	a.operation.Lock()
	defer a.operation.Unlock()

	result := ConfigureResult{FinishedAt: time.Now()}
	operations, warnings, err := buildConfiguration(request)
	result.Warnings = warnings
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Files = make([]FilePreview, 0, len(operations))
	for _, operation := range operations {
		result.Files = append(result.Files, FilePreview{
			ClientID: operation.ClientID,
			Path:     operation.Path,
			Action:   operation.Action,
		})
	}

	backup, err := createBackup(operations)
	if err != nil {
		result.Error = "创建配置备份失败：" + err.Error()
		return result
	}
	result.Backup = &backup

	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			if restoreErr := restoreBackup(backup); restoreErr != nil {
				result.Error = fmt.Sprintf("写入 %s 失败，回滚也失败：%v；备份仍保留在 %s", operation.Path, err, backup.Path)
			} else {
				result.Error = fmt.Sprintf("写入 %s 失败，已自动回滚：%v", operation.Path, err)
			}
			return result
		}
	}

	for _, operation := range operations {
		if err := validateWrittenConfig(operation); err != nil {
			_ = restoreBackup(backup)
			result.Error = fmt.Sprintf("写入后校验失败，已回滚：%v", err)
			return result
		}
	}
	result.Success = true
	seenTargets := make(map[string]bool, len(request.Targets))
	for _, target := range request.Targets {
		normalized := strings.ToLower(strings.TrimSpace(target))
		if normalized == "" || seenTargets[normalized] {
			continue
		}
		seenTargets[normalized] = true
		result.Configured = append(result.Configured, normalized)
	}
	return result
}

func (a *App) ListBackups() ([]BackupInfo, error) {
	return listBackups()
}

func (a *App) GetBackupRoot() string {
	return backupRoot()
}

func (a *App) RestoreBackup(id string) error {
	a.operation.Lock()
	defer a.operation.Unlock()
	return restoreBackupByID(id)
}

func (a *App) DeleteBackup(id string) error {
	a.operation.Lock()
	defer a.operation.Unlock()
	return deleteBackupByID(id)
}

func (a *App) CheckForUpdates() UpdateInfo {
	githubUpdate := checkGitHubRelease(a.client, appVersion, githubReleaseAPIURL)
	if githubUpdate.Error == "" {
		return githubUpdate
	}
	manifestUpdate := checkForUpdates(a.client, appVersion, updateManifestURL)
	if manifestUpdate.Error == "" {
		return manifestUpdate
	}
	githubUpdate.Error = githubUpdate.Error + "；" + manifestUpdate.Error
	return githubUpdate
}

// InstallLatestUpdate downloads and verifies the latest Windows installer,
// then hands off to a detached PowerShell process so the current executable
// can exit before NSIS replaces it.
func (a *App) InstallLatestUpdate() InstallUpdateResult {
	result := InstallUpdateResult{}
	if runtime.GOOS != "windows" {
		result.Error = "自动安装更新目前仅支持 Windows"
		return result
	}
	update := a.CheckForUpdates()
	if update.Error != "" {
		result.Error = update.Error
		return result
	}
	if !update.UpdateAvailable {
		result.Error = "当前已经是最新版本"
		return result
	}
	if err := validateUpdateDownloadURL(update.DownloadURL); err != nil {
		result.Error = err.Error()
		return result
	}
	tempFile, err := os.CreateTemp("", "ciyuanshen-config-assistant-update-*.exe")
	if err != nil {
		result.Error = "创建更新临时文件失败：" + err.Error()
		return result
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
	}()
	request, err := http.NewRequest(http.MethodGet, update.DownloadURL, nil)
	if err != nil {
		_ = os.Remove(tempPath)
		result.Error = "更新下载地址无效"
		return result
	}
	request.Header.Set("User-Agent", "CiyuanShen-Config-Assistant/"+appVersion)
	response, err := newUpdateDownloadHTTPClient(a.client).Do(request)
	if err != nil {
		_ = os.Remove(tempPath)
		result.Error = "下载更新失败：" + err.Error()
		return result
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		_ = os.Remove(tempPath)
		result.Error = fmt.Sprintf("更新下载服务返回 HTTP %d", response.StatusCode)
		return result
	}
	if _, err := io.Copy(tempFile, io.LimitReader(response.Body, 300*1024*1024)); err != nil {
		response.Body.Close()
		_ = os.Remove(tempPath)
		result.Error = "保存更新包失败：" + err.Error()
		return result
	}
	response.Body.Close()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		result.Error = "关闭更新文件失败：" + err.Error()
		return result
	}
	expected := normaliseSHA256(update.SHA256)
	if expected == "" {
		_ = os.Remove(tempPath)
		result.Error = "更新清单缺少有效 SHA256，已取消安装"
		return result
	}
	if expected != "" {
		actual, hashErr := fileSHA256(tempPath)
		if hashErr != nil || !strings.EqualFold(actual, expected) {
			_ = os.Remove(tempPath)
			if hashErr != nil {
				result.Error = "校验更新包失败：" + hashErr.Error()
			} else {
				result.Error = "更新包校验失败，已取消安装"
			}
			return result
		}
	}
	currentExecutable, executableErr := os.Executable()
	if executableErr != nil {
		_ = os.Remove(tempPath)
		result.Error = "无法确定当前程序路径：" + executableErr.Error()
		return result
	}
	processName := strings.TrimSuffix(filepath.Base(currentExecutable), filepath.Ext(currentExecutable))
	installScript := buildUpdateInstallScript(os.Getpid(), processName, currentExecutable, tempPath)
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", installScript)
	command.Stdin = nil
	if err := command.Start(); err != nil {
		_ = os.Remove(tempPath)
		result.Error = "启动更新安装程序失败：" + err.Error()
		return result
	}
	result.Success = true
	result.DownloadURL = update.DownloadURL
	result.Message = "更新包已下载，应用将关闭并自动安装最新版"
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
	return result
}

// newUpdateDownloadHTTPClient keeps the normal client transport settings while
// isolating the installer download from the short timeout used by API calls.
func newUpdateDownloadHTTPClient(base *http.Client) *http.Client {
	client := &http.Client{Timeout: updateDownloadTimeout}
	if base == nil {
		return client
	}
	client.Transport = base.Transport
	client.CheckRedirect = base.CheckRedirect
	client.Jar = base.Jar
	return client
}

func validateUpdateDownloadURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.ToLower(parsed.Scheme) != "https" {
		return errors.New("更新下载地址不是 HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "objects.githubusercontent.com" && host != "api.ciyuanshen.top" && host != "ciyuanshen.top" {
		return errors.New("更新下载地址不是受信任的官方地址")
	}
	return nil
}

func normaliseSHA256(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "sha256:"))
	if len(value) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func powershellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func buildUpdateInstallScript(oldPID int, processName, currentExecutable, installerPath string) string {
	// The app can have more than one process with the same image (for example
	// after a second launch). Waiting on only the initiating PID leaves another
	// process holding the installed executable and makes NSIS show a file-lock
	// retry dialog. Stop every instance of this image, then wait for handles to
	// drain before starting the installer.
	return fmt.Sprintf("$oldPid=%d; $processName=%s; Start-Sleep -Milliseconds 700; Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue; Get-Process -Name $processName -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue; for ($attempt=0; $attempt -lt 50; $attempt++) { if (-not (Get-Process -Name $processName -ErrorAction SilentlyContinue)) { break }; Start-Sleep -Milliseconds 200 }; Start-Sleep -Milliseconds 500; Start-Process -FilePath %s -ArgumentList '/S' -Wait; Start-Process -FilePath %s; Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue", oldPID, powershellQuote(processName), powershellQuote(installerPath), powershellQuote(currentExecutable), powershellQuote(installerPath))
}

func (a *App) OpenExternalURL(url string) error {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(url)), "https://") {
		return errors.New("只允许打开 HTTPS 地址")
	}
	if a.ctx == nil {
		return errors.New("应用尚未初始化")
	}
	wailsRuntime.BrowserOpenURL(a.ctx, strings.TrimSpace(url))
	return nil
}
