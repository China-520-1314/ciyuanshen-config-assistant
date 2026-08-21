package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type clientDefinition struct {
	ID              string
	Name            string
	Commands        []string
	ExecutablePaths func(home string) []string
	Kind            configKind
	Paths           func(home string) []string
	Supported       func() bool
	NPMPackage      string
	DownloadURL     string
}

func clientDefinitions() []clientDefinition {
	return []clientDefinition{
		{ID: "codex", Name: "ChatGPT/Codex Cli/Codex插件", Commands: []string{"codex"}, Kind: configTOML, NPMPackage: "@openai/codex", DownloadURL: "https://developers.openai.com/codex/cli/", Paths: func(home string) []string {
			return []string{filepath.Join(home, ".codex", "config.toml")}
		}},
		{ID: "claude", Name: "Claude Code终端", Commands: []string{"claude"}, Kind: configJSON, NPMPackage: "@anthropic-ai/claude-code", DownloadURL: "https://docs.anthropic.com/en/docs/claude-code/setup", Paths: func(home string) []string {
			return []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".claude", "claude.json")}
		}},
		{ID: "claude-desktop", Name: "Claude Code客户端", ExecutablePaths: claudeDesktopExecutablePaths, Kind: configJSON, Paths: claudeDesktopConfigPaths, Supported: claudeDesktopSupported, DownloadURL: "https://claude.com/download"},
		{ID: "gemini", Name: "Gemini CLI", Commands: []string{"gemini"}, Kind: configEnv, NPMPackage: "@google/gemini-cli", DownloadURL: "https://github.com/google-gemini/gemini-cli", Paths: func(home string) []string {
			return []string{filepath.Join(home, ".gemini", ".env")}
		}},
		{ID: "grok", Name: "Grok Build", Commands: []string{"grok", "grok-build"}, Kind: configTOML, NPMPackage: "@xai-official/grok", DownloadURL: "https://x.ai/grok", Paths: func(home string) []string {
			return []string{filepath.Join(home, ".grok", "config.toml")}
		}},
		{ID: "opencode", Name: "OpenCode", Commands: []string{"opencode"}, Kind: configJSON5, NPMPackage: "opencode-ai", DownloadURL: "https://opencode.ai/docs", Paths: opencodePaths},
		{ID: "openclaw", Name: "OpenClaw", Commands: []string{"openclaw"}, Kind: configJSON5, NPMPackage: "openclaw", DownloadURL: "https://docs.openclaw.ai/", Paths: func(home string) []string {
			return []string{filepath.Join(home, ".openclaw", "openclaw.json")}
		}},
		{ID: "hermes", Name: "Hermes Agent", Commands: []string{"hermes"}, Kind: configYAML, DownloadURL: "https://github.com/NousResearch/hermes-agent", Paths: hermesPaths},
	}
}

func opencodePaths(home string) []string {
	paths := []string{}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "opencode", "opencode.json"))
	}
	if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
		paths = append(paths, filepath.Join(appData, "opencode", "opencode.json"))
	}
	paths = append(paths, filepath.Join(home, ".config", "opencode", "opencode.json"))
	return uniquePaths(paths)
}

func hermesPaths(home string) []string {
	paths := []string{}
	if hermesHome := strings.TrimSpace(os.Getenv("HERMES_HOME")); hermesHome != "" {
		paths = append(paths, filepath.Join(hermesHome, "config.yaml"))
	}
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		paths = append(paths, filepath.Join(localAppData, "hermes", "config.yaml"))
	}
	paths = append(paths, filepath.Join(home, ".hermes", "config.yaml"))
	return uniquePaths(paths)
}

func scanEnvironment() EnvironmentReport {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	report := EnvironmentReport{
		OS:        runtime.GOOS,
		Home:      home,
		ScannedAt: time.Now(),
		Clients:   make([]ClientStatus, 0, len(clientDefinitions())),
	}
	for _, definition := range clientDefinitions() {
		status := ClientStatus{ID: definition.ID, Name: definition.Name, Supported: clientSupported(definition), ConfigState: "missing"}
		if !status.Supported {
			status.ConfigState = "unsupported"
			status.Detail = "当前系统暂不支持此客户端的自动配置"
			report.Clients = append(report.Clients, status)
			continue
		}
		if executable := findClientExecutable(definition, home); executable != "" {
			status.Installed = true
			status.ExecutablePath = executable
			status.Version = executableVersion(executable)
		}
		for _, candidate := range definition.Paths(home) {
			if _, statErr := os.Stat(candidate); statErr == nil {
				status.ConfigPath = candidate
				status.ConfigExists = true
				status.ConfigState, status.Detail = inspectConfig(candidate, definition.Kind)
				break
			}
		}
		if status.Installed && status.ConfigExists {
			status.Detail = "已检测到客户端与配置文件"
		} else if status.Installed {
			status.Detail = "已安装，首次配置将创建配置文件"
		} else if status.ConfigExists {
			status.Detail = "检测到配置文件，但未找到命令行程序"
		} else {
			status.Detail = "未检测到"
		}
		report.Clients = append(report.Clients, status)
	}
	return report
}

func clientDefinitionForID(id string) (clientDefinition, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, definition := range clientDefinitions() {
		if definition.ID == id {
			return definition, true
		}
	}
	return clientDefinition{}, false
}

func clientSupported(definition clientDefinition) bool {
	return definition.Supported == nil || definition.Supported()
}

func findClientExecutable(definition clientDefinition, home string) string {
	if executable := findExecutable(definition.Commands); executable != "" {
		return executable
	}
	if definition.ExecutablePaths == nil {
		return findWindowsCommandShim(definition.Commands, home)
	}
	for _, candidate := range definition.ExecutablePaths(home) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return findWindowsCommandShim(definition.Commands, home)
}

func findExecutable(commands []string) string {
	for _, command := range commands {
		for _, candidate := range executableCandidates(command) {
			if path, err := exec.LookPath(candidate); err == nil {
				return path
			}
		}
	}
	return ""
}

func executableCandidates(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	result := []string{command}
	if runtime.GOOS == "windows" && !strings.Contains(filepath.Base(command), ".") {
		result = append(result, command+".cmd", command+".exe", command+".bat")
	}
	return result
}

// Wails-launched Windows processes can inherit a stale PATH that omits the
// npm global bin directory. Check the standard Node/npm locations directly so
// an installed CLI is not reported as missing just because of that PATH.
func findWindowsCommandShim(commands []string, home string) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	dirs := []string{}
	appendDir := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			dirs = append(dirs, value)
		}
	}
	appendDir(filepath.Join(home, "AppData", "Roaming", "npm"))
	appendDir(os.Getenv("APPDATA"))
	if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
		appendDir(filepath.Join(appData, "npm"))
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		appendDir(filepath.Join(local, "Programs", "nodejs"))
	}
	appendDir(filepath.Join(os.Getenv("ProgramFiles"), "nodejs"))
	appendDir(filepath.Join(os.Getenv("ProgramFiles(x86)"), "nodejs"))
	for _, command := range commands {
		for _, dir := range uniquePaths(dirs) {
			for _, candidate := range executableCandidates(command) {
				path := filepath.Join(dir, candidate)
				if info, err := os.Stat(path); err == nil && !info.IsDir() {
					return path
				}
			}
		}
	}
	return ""
}

func executableVersion(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if len(line) > 80 {
		return line[:80]
	}
	return line
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
