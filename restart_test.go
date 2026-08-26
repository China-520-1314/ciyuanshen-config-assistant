package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguredCLIRequiresManualRestart(t *testing.T) {
	result := restartConfiguredTool("codex")
	if result.ClientID != "codex" || result.Attempted || result.Restarted || !result.ManualRestartRequired {
		t.Fatalf("unexpected CLI restart result: %#v", result)
	}
	if !strings.Contains(result.Message, "手动重启") {
		t.Fatalf("manual restart message missing: %q", result.Message)
	}
}

func TestUpdatedClientDisplayNames(t *testing.T) {
	if got := clientDisplayName("claude"); got != "Claude Code CLI/插件" {
		t.Fatalf("Claude display name = %q", got)
	}
	if got := clientDisplayName("codex"); got != "ChatGPT/Codex CLI/Codex插件" {
		t.Fatalf("Codex display name = %q", got)
	}
}

func TestApplicationProcessName(t *testing.T) {
	for path, want := range map[string]string{
		`C:\Program Files\Claude\Claude.exe`:             "Claude",
		"/Applications/Claude.app/Contents/MacOS/Claude": "Claude",
	} {
		if got := applicationProcessName(path); got != want {
			t.Fatalf("applicationProcessName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestMacApplicationBundlePath(t *testing.T) {
	path := "/Applications/Claude.app/Contents/MacOS/Claude"
	if got := macApplicationBundlePath(path); got != "/Applications/Claude.app" {
		t.Fatalf("macApplicationBundlePath(%q) = %q", path, got)
	}
	if got := macApplicationBundlePath("/usr/local/bin/claude"); got != "" {
		t.Fatalf("non-app executable bundle = %q", got)
	}
}

func TestWindowsDesktopRestartScriptUsesKnownExecutable(t *testing.T) {
	script := buildWindowsDesktopRestartScript(`C:\Program Files\Claude\Claude.exe`)
	for _, expected := range []string{
		"Get-Process -Name $processName",
		"Stop-Process -Force",
		"Start-Process -FilePath 'C:\\Program Files\\Claude\\Claude.exe'",
		windowsRestartNotRunningMarker,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("restart script missing %q:\n%s", expected, script)
		}
	}
}

func TestConfigureReturnsRestartResultAfterSuccessfulWrite(t *testing.T) {
	home := isolateHome(t)
	app := NewApp()
	var restarted []string
	app.restartTool = func(clientID string) ToolRestartResult {
		restarted = append(restarted, clientID)
		return ToolRestartResult{ClientID: clientID, Attempted: true, Restarted: true, Message: "已自动重启"}
	}

	result := app.Configure(ConfigurationRequest{APIKey: "test-key", Targets: []string{"codex"}})
	if !result.Success || result.Error != "" {
		t.Fatalf("configuration failed: %#v", result)
	}
	if len(restarted) != 1 || restarted[0] != "codex" {
		t.Fatalf("restart targets = %#v", restarted)
	}
	if len(result.Restarts) != 1 || !result.Restarts[0].Restarted {
		t.Fatalf("restart result = %#v", result.Restarts)
	}
	content, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `model_provider = "ciyuanshen"`) {
		t.Fatal("configuration file was not written before restart")
	}
}
