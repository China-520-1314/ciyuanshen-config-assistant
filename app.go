package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateManifestURL   = "https://ciyuanshen.top/downloads/ciyuanshen-config-assistant/update.json"
	defaultGatewayURL   = "https://ciyuanshen.top/v1"
	claudeGatewayURL    = "https://ciyuanshen.top"
	geminiGatewayURL    = "https://ciyuanshen.top"
	geminiAPIVersion    = "v1"
	managedProviderName = "ciyuanshen"
)

// appVersion is a variable so release builds can inject their tag with
// -ldflags "-X main.appVersion=..." while local builds keep a useful default.
var appVersion = "0.1.0"

// App is the bridge exposed to the Wails frontend. It never persists the API
// key in the assistant's own data directory; the key only lives in memory for
// the duration of a scan or configuration transaction.
type App struct {
	ctx       context.Context
	client    *http.Client
	operation sync.Mutex
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
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:              "慈元神配置助手",
		Version:           appVersion,
		UpdateManifestURL: updateManifestURL,
		GatewayURL:        defaultGatewayURL,
	}
}

func (a *App) ScanEnvironment() EnvironmentReport {
	return scanEnvironment()
}

func (a *App) FetchModels(apiKey string) (ModelResponse, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ModelResponse{}, errors.New("请先输入 API Key")
	}

	request, err := http.NewRequest(http.MethodGet, defaultGatewayURL+"/models", nil)
	if err != nil {
		return ModelResponse{}, errors.New("无法创建模型请求")
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("无法连接慈元神网关：%w", err)
	}
	defer response.Body.Close()

	payload, err := decodeModelPayload(response.Body)
	if err != nil {
		return ModelResponse{}, errors.New("网关返回的数据格式无法识别")
	}
	payload.Status = response.StatusCode
	payload.Endpoint = defaultGatewayURL + "/models"
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusUnauthorized {
			payload.Message = "API Key 无效或已过期"
		} else if payload.Message == "" {
			payload.Message = fmt.Sprintf("网关返回 HTTP %d", response.StatusCode)
		}
		return payload, errors.New(payload.Message)
	}
	if len(payload.Models) == 0 {
		return payload, errors.New("Key 可用，但网关没有返回可用模型")
	}
	return payload, nil
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

func (a *App) RestoreBackup(id string) error {
	a.operation.Lock()
	defer a.operation.Unlock()
	return restoreBackupByID(id)
}

func (a *App) CheckForUpdates() UpdateInfo {
	return checkForUpdates(a.client, appVersion, updateManifestURL)
}

func (a *App) OpenExternalURL(url string) error {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(url)), "https://") {
		return errors.New("只允许打开 HTTPS 地址")
	}
	if a.ctx == nil {
		return errors.New("应用尚未初始化")
	}
	runtime.BrowserOpenURL(a.ctx, strings.TrimSpace(url))
	return nil
}
