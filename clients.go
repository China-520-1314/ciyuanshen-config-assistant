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
	ID       string
	Name     string
	Commands []string
	Kind     configKind
	Paths    func(home string) []string
}

func clientDefinitions() []clientDefinition {
	return []clientDefinition{
		{ID: "claude", Name: "Claude Code", Commands: []string{"claude"}, Kind: configJSON, Paths: func(home string) []string {
			return []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".claude", "claude.json")}
		}},
		{ID: "codex", Name: "ChatGPT/Codex Cli/Codex插件", Commands: []string{"codex"}, Kind: configTOML, Paths: func(home string) []string {
			return []string{filepath.Join(home, ".codex", "config.toml")}
		}},
		{ID: "gemini", Name: "Gemini CLI", Commands: []string{"gemini"}, Kind: configEnv, Paths: func(home string) []string {
			return []string{filepath.Join(home, ".gemini", ".env")}
		}},
		{ID: "grok", Name: "Grok Build", Commands: []string{"grok", "grok-build"}, Kind: configTOML, Paths: func(home string) []string {
			return []string{filepath.Join(home, ".grok", "config.toml")}
		}},
		{ID: "opencode", Name: "OpenCode", Commands: []string{"opencode"}, Kind: configJSON5, Paths: opencodePaths},
		{ID: "openclaw", Name: "OpenClaw", Commands: []string{"openclaw"}, Kind: configJSON5, Paths: func(home string) []string {
			return []string{filepath.Join(home, ".openclaw", "openclaw.json")}
		}},
		{ID: "hermes", Name: "Hermes Agent", Commands: []string{"hermes"}, Kind: configYAML, Paths: hermesPaths},
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
		status := ClientStatus{ID: definition.ID, Name: definition.Name, ConfigState: "missing"}
		if executable := findExecutable(definition.Commands); executable != "" {
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

func findExecutable(commands []string) string {
	for _, command := range commands {
		if path, err := exec.LookPath(command); err == nil {
			return path
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
