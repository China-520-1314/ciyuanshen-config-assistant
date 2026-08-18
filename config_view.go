package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const maxConfigurationViewBytes = 2 << 20

type ClientConfigurationFile struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}

type ClientConfigurationView struct {
	ClientID        string                    `json:"clientId"`
	ClientName      string                    `json:"clientName"`
	Files           []ClientConfigurationFile `json:"files"`
	SecretsRedacted bool                      `json:"secretsRedacted"`
}

var sensitiveConfigurationValue = regexp.MustCompile(`(?im)(["']?[a-z0-9_.-]*(?:api[_-]?key|auth[_-]?token|access[_-]?token|token|secret|password)[a-z0-9_.-]*["']?\s*[:=]\s*)(?:"(?:\\.|[^"])*"|'(?:\\.|[^'])*'|[^,{}\[\]\r\n]+)`)

// GetClientConfiguration returns the target client's active configuration
// files. Values that look like credentials are masked unless the local user
// explicitly asks to reveal them in the configuration viewer.
func (a *App) GetClientConfiguration(clientID string, revealSecrets bool) (ClientConfigurationView, error) {
	clientID, err := normaliseClientID(clientID)
	if err != nil {
		return ClientConfigurationView{}, err
	}
	definition, ok := clientDefinitionForID(clientID)
	if !ok {
		return ClientConfigurationView{}, fmt.Errorf("不支持的工具：%s", clientID)
	}
	if !clientSupported(definition) {
		return ClientConfigurationView{}, errors.New("当前系统暂不支持此客户端的自动配置")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ClientConfigurationView{}, errors.New("无法确定当前用户目录")
	}

	view := ClientConfigurationView{
		ClientID:        clientID,
		ClientName:      definition.Name,
		Files:           []ClientConfigurationFile{},
		SecretsRedacted: !revealSecrets,
	}
	for _, path := range clientConfigurationPaths(clientID, home) {
		file := ClientConfigurationFile{Path: path}
		info, statErr := os.Stat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			view.Files = append(view.Files, file)
			continue
		}
		if statErr != nil {
			return ClientConfigurationView{}, fmt.Errorf("无法读取配置文件 %s：%w", path, statErr)
		}
		if info.IsDir() {
			return ClientConfigurationView{}, fmt.Errorf("配置路径不是文件：%s", path)
		}
		if info.Size() > maxConfigurationViewBytes {
			return ClientConfigurationView{}, fmt.Errorf("配置文件过大，无法在助手中显示：%s", path)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return ClientConfigurationView{}, fmt.Errorf("无法读取配置文件 %s：%w", path, readErr)
		}
		file.Exists = true
		file.Content = string(content)
		if !revealSecrets {
			file.Content = redactConfigurationContent(file.Content)
		}
		view.Files = append(view.Files, file)
	}
	return view, nil
}

func clientConfigurationPaths(clientID, home string) []string {
	if clientID == "claude-desktop" {
		paths, ok := claudeDesktopPaths(home)
		if !ok {
			return []string{}
		}
		return []string{paths.Profile, paths.NormalConfig, paths.ThreePConfig, paths.Meta}
	}

	definition, ok := clientDefinitionForID(clientID)
	if !ok {
		return []string{}
	}
	candidates := definition.Paths(home)
	paths := make([]string, 0, len(candidates)+1)
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 && len(candidates) > 0 {
		paths = append(paths, candidates[0])
	}

	switch clientID {
	case "codex":
		paths = append(paths, filepath.Join(home, ".codex", "auth.json"))
	case "gemini":
		paths = append(paths, filepath.Join(home, ".gemini", "settings.json"))
	}
	return uniquePaths(paths)
}

func redactConfigurationContent(content string) string {
	return sensitiveConfigurationValue.ReplaceAllString(content, `${1}"********"`)
}
