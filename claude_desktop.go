package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Claude Desktop stores third-party gateway profiles outside its normal user
// configuration. These paths follow the profile layout used by Claude Desktop
// and CC Switch on Windows and macOS.
const (
	claudeDesktopProfileID   = "ciyuanshen-config-assistant"
	claudeDesktopProfileName = "词元神配置助手"
)

type claudeDesktopPathSet struct {
	NormalConfig string
	ThreePConfig string
	Profile      string
	Meta         string
}

func claudeDesktopSupported() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

func claudeDesktopPaths(home string) (claudeDesktopPathSet, bool) {
	var normalDir string
	var threePDir string
	switch runtime.GOOS {
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		normalDir = filepath.Join(localAppData, "Claude")
		threePDir = filepath.Join(localAppData, "Claude-3p")
	case "darwin":
		appSupport := filepath.Join(home, "Library", "Application Support")
		normalDir = filepath.Join(appSupport, "Claude")
		threePDir = filepath.Join(appSupport, "Claude-3p")
	default:
		return claudeDesktopPathSet{}, false
	}
	configLibrary := filepath.Join(threePDir, "configLibrary")
	return claudeDesktopPathSet{
		NormalConfig: filepath.Join(normalDir, "claude_desktop_config.json"),
		ThreePConfig: filepath.Join(threePDir, "claude_desktop_config.json"),
		Profile:      filepath.Join(configLibrary, claudeDesktopProfileID+".json"),
		Meta:         filepath.Join(configLibrary, "_meta.json"),
	}, true
}

func claudeDesktopConfigPaths(home string) []string {
	paths, ok := claudeDesktopPaths(home)
	if !ok {
		return []string{}
	}
	return []string{paths.Profile, paths.NormalConfig, paths.ThreePConfig, paths.Meta}
}

func claudeDesktopExecutablePaths(home string) []string {
	switch runtime.GOOS {
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
		paths := []string{
			filepath.Join(localAppData, "Programs", "Claude", "Claude.exe"),
			filepath.Join(localAppData, "Claude", "Claude.exe"),
		}
		if programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "Claude", "Claude.exe"))
		}
		return uniquePaths(paths)
	case "darwin":
		return []string{
			"/Applications/Claude.app/Contents/MacOS/Claude",
			filepath.Join(home, "Applications", "Claude.app", "Contents", "MacOS", "Claude"),
		}
	default:
		return []string{}
	}
}

func configureClaudeDesktop(home, key, model string) ([]fileOperation, error) {
	paths, ok := claudeDesktopPaths(home)
	if !ok {
		return nil, errors.New("Claude Code客户端仅支持 Windows 和 macOS")
	}
	if !isClaudeDesktopCompatibleModel(model) {
		return nil, errors.New("Claude Code客户端仅支持 Claude Sonnet、Opus、Haiku 或 Fable 模型")
	}

	normalConfig, err := claudeDesktopDeploymentModeContent(paths.NormalConfig)
	if err != nil {
		return nil, fmt.Errorf("读取 Claude Code客户端配置失败：%w", err)
	}
	threePConfig, err := claudeDesktopDeploymentModeContent(paths.ThreePConfig)
	if err != nil {
		return nil, fmt.Errorf("读取 Claude Code客户端第三方配置失败：%w", err)
	}
	profile, err := claudeDesktopProfileContent(paths.Profile, key, model)
	if err != nil {
		return nil, fmt.Errorf("生成 Claude Code客户端配置失败：%w", err)
	}
	meta, err := claudeDesktopMetaContent(paths.Meta)
	if err != nil {
		return nil, fmt.Errorf("生成 Claude Code客户端配置索引失败：%w", err)
	}

	return []fileOperation{
		newOperation("claude-desktop", paths.NormalConfig, configJSON, normalConfig),
		newOperation("claude-desktop", paths.ThreePConfig, configJSON, threePConfig),
		newOperation("claude-desktop", paths.Profile, configJSON, profile),
		newOperation("claude-desktop", paths.Meta, configJSON, meta),
	}, nil
}

func claudeDesktopDeploymentModeContent(path string) ([]byte, error) {
	root, err := readJSONMap(path)
	if err != nil {
		return nil, err
	}
	root["deploymentMode"] = "3p"
	return marshalJSON(root)
}

func claudeDesktopProfileContent(path, key, model string) ([]byte, error) {
	root, err := readJSONMap(path)
	if err != nil {
		return nil, err
	}
	root["coworkEgressAllowedHosts"] = []any{"*"}
	root["disableDeploymentModeChooser"] = true
	root["inferenceGatewayApiKey"] = key
	root["inferenceGatewayAuthScheme"] = "bearer"
	root["inferenceGatewayBaseUrl"] = claudeGatewayURL
	root["inferenceProvider"] = "gateway"
	root["inferenceModels"] = []any{model}
	return marshalJSON(root)
}

func claudeDesktopMetaContent(path string) ([]byte, error) {
	root, err := readJSONMap(path)
	if err != nil {
		return nil, err
	}
	entries, _ := root["entries"].([]any)
	filtered := make([]any, 0, len(entries)+1)
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if ok && fmt.Sprint(item["id"]) == claudeDesktopProfileID {
			continue
		}
		filtered = append(filtered, entry)
	}
	root["entries"] = append(filtered, map[string]any{
		"id":   claudeDesktopProfileID,
		"name": claudeDesktopProfileName,
	})
	root["appliedId"] = claudeDesktopProfileID
	return marshalJSON(root)
}

func isClaudeDesktopCompatibleModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "claude-sonnet-") ||
		strings.HasPrefix(model, "claude-opus-") ||
		strings.HasPrefix(model, "claude-haiku-") ||
		strings.HasPrefix(model, "claude-fable-")
}
