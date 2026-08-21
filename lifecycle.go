package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const toolLifecycleTimeout = 5 * time.Minute

type ToolLifecycleInfo struct {
	ClientID        string    `json:"clientId"`
	Name            string    `json:"name"`
	Installed       bool      `json:"installed"`
	CurrentVersion  string    `json:"currentVersion,omitempty"`
	LatestVersion   string    `json:"latestVersion,omitempty"`
	UpdateAvailable bool      `json:"updateAvailable"`
	CanInstall      bool      `json:"canInstall"`
	CanUpdate       bool      `json:"canUpdate"`
	DownloadURL     string    `json:"downloadUrl,omitempty"`
	InstallMethod   string    `json:"installMethod,omitempty"`
	CheckedAt       time.Time `json:"checkedAt"`
	Message         string    `json:"message,omitempty"`
	Error           string    `json:"error,omitempty"`
}

type ToolLifecycleRequest struct {
	ClientID string `json:"clientId"`
	Action   string `json:"action"`
}

type ToolLifecycleResult struct {
	Success     bool              `json:"success"`
	Manual      bool              `json:"manual"`
	DownloadURL string            `json:"downloadUrl,omitempty"`
	Message     string            `json:"message,omitempty"`
	Error       string            `json:"error,omitempty"`
	Info        ToolLifecycleInfo `json:"info"`
}

type npmLatestPayload struct {
	Version string `json:"version"`
}

var toolVersionPattern = regexp.MustCompile(`(?i)\bv?(\d+(?:\.\d+){0,3}(?:-[0-9a-z.-]+)?)\b`)

// GetToolLifecycleInfo checks a single tool's local version and its upstream
// release version. It is intentionally separate from environment scanning so
// opening the assistant never fans out into several external requests.
func (a *App) GetToolLifecycleInfo(clientID string) ToolLifecycleInfo {
	info, err := a.toolLifecycleInfo(clientID)
	if err != nil {
		info.Error = err.Error()
	}
	return info
}

func (a *App) toolLifecycleInfo(clientID string) (ToolLifecycleInfo, error) {
	clientID, err := normaliseClientID(clientID)
	if err != nil {
		return ToolLifecycleInfo{ClientID: strings.TrimSpace(clientID), CheckedAt: time.Now()}, err
	}
	definition, ok := clientDefinitionForID(clientID)
	if !ok {
		return ToolLifecycleInfo{ClientID: clientID, CheckedAt: time.Now()}, fmt.Errorf("不支持的工具：%s", clientID)
	}
	info := ToolLifecycleInfo{
		ClientID:    clientID,
		Name:        definition.Name,
		DownloadURL: definition.DownloadURL,
		CheckedAt:   time.Now(),
	}
	if !clientSupported(definition) {
		info.Message = "当前系统暂不支持此客户端的自动配置或更新"
		return info, nil
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return info, errors.New("无法确定当前用户目录")
	}
	if executable := findClientExecutable(definition, home); executable != "" {
		info.Installed = true
		info.CurrentVersion = normaliseToolVersion(executableVersion(executable))
	}

	if definition.NPMPackage != "" {
		if npmPath := findExecutable([]string{"npm"}); npmPath != "" || findWindowsCommandShim([]string{"npm"}, home) != "" {
			info.CanInstall = true
			info.CanUpdate = info.Installed
			info.InstallMethod = "npm"
		} else {
			info.Message = "未检测到 npm，无法自动安装；可前往官方下载页安装"
		}
		latest, latestErr := fetchNPMLatestVersion(a.client, definition.NPMPackage)
		if latestErr != nil {
			info.Error = "暂时无法检查最新版本：" + latestErr.Error()
			return info, nil
		}
		info.LatestVersion = latest
		if info.CurrentVersion != "" {
			info.UpdateAvailable = compareVersions(latest, info.CurrentVersion) > 0
		}
		return info, nil
	}

	if clientID == "hermes" {
		latest, latestErr := fetchGitHubLatestVersion(a.client, "NousResearch/hermes-agent")
		if latestErr != nil {
			info.Error = "暂时无法检查最新版本：" + latestErr.Error()
			return info, nil
		}
		info.LatestVersion = latest
		if info.CurrentVersion != "" {
			info.UpdateAvailable = compareVersions(latest, info.CurrentVersion) > 0
		}
		info.Message = "请使用 Hermes 官方安装程序完成安装或更新"
		return info, nil
	}

	if clientID == "claude-desktop" {
		info.Message = "Claude 客户端由应用内更新，请前往官方下载页获取最新版本"
		return info, nil
	}
	return info, nil
}

// RunToolLifecycleAction only accepts the fixed actions and package metadata
// compiled into this application. The frontend never controls a shell command.
func (a *App) RunToolLifecycleAction(request ToolLifecycleRequest) ToolLifecycleResult {
	clientID, err := normaliseClientID(request.ClientID)
	if err != nil {
		return ToolLifecycleResult{Error: err.Error()}
	}
	info := a.GetToolLifecycleInfo(clientID)
	if info.Error != "" && !info.CanInstall && info.DownloadURL == "" {
		return ToolLifecycleResult{Error: info.Error, DownloadURL: info.DownloadURL, Info: info}
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "install" && action != "update" {
		return ToolLifecycleResult{Error: "不支持的工具操作", Info: info}
	}
	if action == "update" && !info.Installed {
		return ToolLifecycleResult{Error: "当前未检测到该工具，请先安装", DownloadURL: info.DownloadURL, Info: info}
	}
	if !info.CanInstall {
		message := info.Message
		if message == "" {
			message = "该工具无法由助手自动安装，请从官方下载页安装后再一键配置"
		}
		return ToolLifecycleResult{Manual: true, DownloadURL: info.DownloadURL, Message: message, Info: info}
	}

	definition, ok := clientDefinitionForID(clientID)
	if !ok || definition.NPMPackage == "" {
		return ToolLifecycleResult{Manual: true, DownloadURL: info.DownloadURL, Message: "该工具请从官方下载页安装", Info: info}
	}
	home, _ := os.UserHomeDir()
	npmPath := findExecutable([]string{"npm"})
	if npmPath == "" {
		npmPath = findWindowsCommandShim([]string{"npm"}, home)
	}
	if npmPath == "" {
		return ToolLifecycleResult{Manual: true, DownloadURL: info.DownloadURL, Message: "未检测到 npm，请从官方下载页安装", Info: info}
	}

	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), toolLifecycleTimeout)
	defer cancel()
	command := npmLifecycleCommand(ctx, npmPath, definition.NPMPackage)
	output, runErr := command.CombinedOutput()
	if runErr != nil {
		detail := lifecycleOutputDetail(output)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			detail = "安装或更新超时，请检查网络后重试"
		}
		if detail == "" {
			detail = runErr.Error()
		}
		return ToolLifecycleResult{Error: detail, Info: a.GetToolLifecycleInfo(clientID)}
	}

	refreshed := a.GetToolLifecycleInfo(clientID)
	verb := "安装"
	if action == "update" {
		verb = "更新"
	}
	message := verb + "命令已完成"
	if !refreshed.Installed {
		message += "。若尚未被检测到，请重启终端或配置助手后再试"
	}
	return ToolLifecycleResult{Success: true, Message: message, Info: refreshed}
}

func npmLifecycleCommand(ctx context.Context, npmPath, packageName string) *exec.Cmd {
	return npmLifecycleCommandForOS(ctx, npmPath, packageName, runtime.GOOS)
}

// npmLifecycleCommandForOS is kept separate from runtime.GOOS so command
// construction can be regression-tested on non-Windows build hosts. Windows
// npm installations expose npm.cmd, which must be invoked through cmd.exe and
// prefixed with call; otherwise cmd can replace the parent command session.
func npmLifecycleCommandForOS(ctx context.Context, npmPath, packageName, goos string) *exec.Cmd {
	args := []string{"install", "--global", packageName + "@latest"}
	if goos != "windows" {
		return exec.CommandContext(ctx, npmPath, args...)
	}
	// npm on Windows is normally a .cmd shim. All command fragments below are
	// supplied by the compiled client definition, never by the web view.
	line := `call ` + quoteWindowsBatchPath(npmPath) + ` install --global ` + packageName + `@latest`
	return newWindowsShellCommand(ctx, line)
}

// quoteWindowsBatchPath follows the quoting rules used by cmd batch files.
// Besides spaces, cmd treats characters such as '&' and '|' as operators even
// inside an otherwise unquoted command line. Percent signs are doubled for the
// call re-parse so a user's directory name is not interpreted as an env var.
func quoteWindowsBatchPath(path string) string {
	escaped := strings.ReplaceAll(path, `%`, `%%%%`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	if strings.ContainsAny(path, " \t&()^;<>,|") {
		return `"` + escaped + `"`
	}
	return escaped
}

func fetchNPMLatestVersion(client *http.Client, packageName string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	endpoint := "https://registry.npmjs.org/" + url.PathEscape(packageName) + "/latest"
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errors.New("无法创建版本检查请求")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ciyuanshen-config-assistant")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("npm 返回 HTTP %d", response.StatusCode)
	}
	var payload npmLatestPayload
	if err := json.NewDecoder(io.LimitReader(response.Body, 256*1024)).Decode(&payload); err != nil {
		return "", errors.New("npm 版本数据格式无效")
	}
	version := normaliseToolVersion(payload.Version)
	if version == "" {
		return "", errors.New("npm 未返回有效版本号")
	}
	return version, nil
}

func fetchGitHubLatestVersion(client *http.Client, repository string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	endpoint := "https://api.github.com/repos/" + repository + "/releases/latest"
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errors.New("无法创建版本检查请求")
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ciyuanshen-config-assistant")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("GitHub 返回 HTTP %d", response.StatusCode)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 256*1024)).Decode(&release); err != nil {
		return "", errors.New("GitHub 版本数据格式无效")
	}
	version := normaliseToolVersion(release.TagName)
	if version == "" {
		return "", errors.New("GitHub 未返回有效版本号")
	}
	return version, nil
}

func normaliseToolVersion(raw string) string {
	match := toolVersionPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func lifecycleOutputDetail(output []byte) string {
	text := strings.ReplaceAll(decodeLifecycleOutput(output), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	result := strings.TrimSpace(strings.Join(lines, "\n"))
	if len(result) > 1800 {
		runes := []rune(result)
		if len(runes) > 1800 {
			return string(runes[len(runes)-1800:])
		}
	}
	return result
}

// decodeLifecycleOutput normalises output from npm/cmd before it is displayed
// in the Wails toast. Windows may emit the active OEM/ANSI Chinese code page
// (usually CP936/GBK), while newer Node releases emit UTF-8. Preserve valid
// UTF-8 and decode invalid byte sequences as GB18030/GBK instead of rendering
// mojibake such as "����".
func decodeLifecycleOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}
	if utf8.Valid(output) {
		return string(output)
	}
	for _, charset := range []encoding.Encoding{simplifiedchinese.GB18030, simplifiedchinese.GBK} {
		decoded, err := charset.NewDecoder().Bytes(output)
		if err == nil && utf8.Valid(decoded) {
			return string(decoded)
		}
	}
	return string(output)
}
