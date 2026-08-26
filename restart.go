package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const toolRestartTimeout = 15 * time.Second

type toolRestartMode uint8

const (
	// Terminal and editor-plugin sessions carry user-selected arguments and
	// working directories, so recreating them would be destructive.
	toolRestartManual toolRestartMode = iota
	toolRestartDesktopApplication
)

// ToolRestartResult describes the best-effort restart performed after a
// configuration transaction has fully written and validated its files.
// Restart failures never undo a valid configuration.
type ToolRestartResult struct {
	ClientID              string `json:"clientId"`
	Attempted             bool   `json:"attempted"`
	Restarted             bool   `json:"restarted"`
	ManualRestartRequired bool   `json:"manualRestartRequired"`
	Message               string `json:"message"`
}

func (a *App) restartConfiguredTools(targets []string) []ToolRestartResult {
	restart := a.restartTool
	if restart == nil {
		restart = restartConfiguredTool
	}

	results := make([]ToolRestartResult, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		clientID := strings.ToLower(strings.TrimSpace(target))
		if clientID == "" || seen[clientID] {
			continue
		}
		seen[clientID] = true
		results = append(results, restart(clientID))
	}
	return results
}

func restartConfiguredTool(clientID string) ToolRestartResult {
	definition, ok := clientDefinitionForID(clientID)
	if !ok {
		return ToolRestartResult{
			ClientID:              strings.TrimSpace(clientID),
			ManualRestartRequired: true,
			Message:               "配置已写入，但未能确定工具；请手动重启该工具使配置生效。",
		}
	}

	result := ToolRestartResult{ClientID: definition.ID}
	if definition.RestartMode != toolRestartDesktopApplication {
		result.ManualRestartRequired = true
		result.Message = fmt.Sprintf("%s 配置已写入。为避免中断当前 CLI/插件会话，未自动重启；如该工具正在运行，请手动重启该工具（关闭后重新打开），配置才会生效。", definition.Name)
		return result
	}

	home, err := os.UserHomeDir()
	if err != nil {
		result.ManualRestartRequired = true
		result.Message = fmt.Sprintf("%s 配置已写入，但无法确定应用路径；请手动重启该工具使配置生效。", definition.Name)
		return result
	}
	executable := findClientExecutable(definition, home)
	if executable == "" {
		result.ManualRestartRequired = true
		result.Message = fmt.Sprintf("%s 配置已写入，但未找到可重启的应用程序；请手动重启该工具使配置生效。", definition.Name)
		return result
	}

	result.Attempted = true
	ctx, cancel := context.WithTimeout(context.Background(), toolRestartTimeout)
	defer cancel()
	restarted, err := restartDesktopClientApplication(ctx, executable)
	if err != nil {
		result.ManualRestartRequired = true
		result.Message = fmt.Sprintf("%s 配置已写入，但自动重启失败；请手动重启该工具使配置生效。", definition.Name)
		return result
	}
	if !restarted {
		result.Attempted = false
		result.Message = fmt.Sprintf("%s 配置已写入。未检测到正在运行的实例，下次启动时将自动生效。", definition.Name)
		return result
	}

	result.Restarted = true
	result.Message = fmt.Sprintf("%s 已自动重启，新配置已生效。", definition.Name)
	return result
}

func restartDesktopClientApplication(ctx context.Context, executable string) (bool, error) {
	switch runtime.GOOS {
	case "windows":
		return restartWindowsDesktopApplication(ctx, executable)
	case "darwin":
		return restartMacDesktopApplication(ctx, executable)
	default:
		return false, errors.New("当前系统不支持桌面应用自动重启")
	}
}

const windowsRestartNotRunningMarker = "__CIYUANSHEN_RESTART_NOT_RUNNING__"

func restartWindowsDesktopApplication(ctx context.Context, executable string) (bool, error) {
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", buildWindowsDesktopRestartScript(executable))
	output, err := command.CombinedOutput()
	if strings.Contains(string(output), windowsRestartNotRunningMarker) {
		return false, nil
	}
	if err != nil || ctx.Err() != nil {
		return false, errors.New("Windows 应用重启命令执行失败")
	}
	return true, nil
}

func buildWindowsDesktopRestartScript(executable string) string {
	processName := applicationProcessName(executable)
	return fmt.Sprintf("$ErrorActionPreference='Stop'; $processName=%s; $processes=@(Get-Process -Name $processName -ErrorAction SilentlyContinue); if ($processes.Count -eq 0) { Write-Output %s; exit 0 }; $processes | Stop-Process -Force -ErrorAction Stop; for ($attempt=0; $attempt -lt 50; $attempt++) { if (-not (Get-Process -Name $processName -ErrorAction SilentlyContinue)) { break }; Start-Sleep -Milliseconds 200 }; if (Get-Process -Name $processName -ErrorAction SilentlyContinue) { throw '应用未能退出' }; Start-Process -FilePath %s -ErrorAction Stop", powershellQuote(processName), powershellQuote(windowsRestartNotRunningMarker), powershellQuote(executable))
}

func restartMacDesktopApplication(ctx context.Context, executable string) (bool, error) {
	processName := applicationProcessName(executable)
	running, err := macProcessRunning(ctx, processName)
	if err != nil {
		return false, err
	}
	if !running {
		return false, nil
	}

	if err := exec.CommandContext(ctx, "pkill", "-TERM", "-x", processName).Run(); err != nil && ctx.Err() != nil {
		return false, errors.New("macOS 应用退出超时")
	}
	for attempt := 0; attempt < 25; attempt++ {
		running, err = macProcessRunning(ctx, processName)
		if err != nil {
			return false, err
		}
		if !running {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if running {
		if err := exec.CommandContext(ctx, "pkill", "-KILL", "-x", processName).Run(); err != nil && ctx.Err() != nil {
			return false, errors.New("macOS 应用退出超时")
		}
		for attempt := 0; attempt < 10; attempt++ {
			running, err = macProcessRunning(ctx, processName)
			if err != nil {
				return false, err
			}
			if !running {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if running {
			return false, errors.New("macOS 应用未能退出")
		}
	}

	appPath := macApplicationBundlePath(executable)
	if appPath != "" {
		if err := exec.CommandContext(ctx, "open", appPath).Start(); err != nil {
			return false, errors.New("macOS 应用启动失败")
		}
		return true, nil
	}
	if err := exec.CommandContext(ctx, executable).Start(); err != nil {
		return false, errors.New("macOS 应用启动失败")
	}
	return true, nil
}

func macProcessRunning(ctx context.Context, processName string) (bool, error) {
	err := exec.CommandContext(ctx, "pgrep", "-x", processName).Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	if ctx.Err() != nil {
		return false, errors.New("macOS 应用状态检查超时")
	}
	return false, errors.New("macOS 应用状态检查失败")
}

func applicationProcessName(executable string) string {
	base := filepath.Base(strings.ReplaceAll(strings.TrimSpace(executable), "\\", "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func macApplicationBundlePath(executable string) string {
	directory := filepath.Clean(filepath.Dir(executable))
	for directory != "." && directory != string(filepath.Separator) {
		if strings.EqualFold(filepath.Ext(directory), ".app") {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return ""
}
