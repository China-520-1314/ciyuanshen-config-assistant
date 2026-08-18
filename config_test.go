package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "localappdata"))
	t.Setenv("HERMES_HOME", "")
	return home
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func operationFor(t *testing.T, operations []fileOperation, path string) fileOperation {
	t.Helper()
	for _, operation := range operations {
		if operation.Path == path {
			return operation
		}
	}
	t.Fatalf("operation for %s not found", path)
	return fileOperation{}
}

func TestConfigureClaudePreservesExistingSettings(t *testing.T) {
	home := isolateHome(t)
	path := filepath.Join(home, ".claude", "settings.json")
	writeFixture(t, path, `{"permissions":{"allow":["Bash"]},"env":{"OLD_VALUE":"keep"}}`)

	operations, err := configureClaude(home, "test-key", "gpt-test")
	if err != nil {
		t.Fatal(err)
	}
	operation := operationFor(t, operations, path)
	if operation.Action != "update" {
		t.Fatalf("expected update action, got %q", operation.Action)
	}
	var root map[string]any
	if err := json.Unmarshal(operation.Content, &root); err != nil {
		t.Fatal(err)
	}
	env, ok := root["env"].(map[string]any)
	if !ok {
		t.Fatalf("env was not preserved as an object: %#v", root["env"])
	}
	if env["ANTHROPIC_BASE_URL"] != claudeGatewayURL || env["ANTHROPIC_AUTH_TOKEN"] != "test-key" {
		t.Fatalf("unexpected Claude gateway values: %#v", env)
	}
	if env["ANTHROPIC_MODEL"] != "gpt-test" || env["OLD_VALUE"] != "keep" {
		t.Fatalf("existing/model values were not preserved: %#v", env)
	}
	if _, exists := env["ANTHROPIC_API_KEY"]; exists {
		t.Fatal("legacy ANTHROPIC_API_KEY should be removed")
	}
}

func TestConfigureGeminiUpdatesEnvWithoutDroppingComments(t *testing.T) {
	home := isolateHome(t)
	envPath := filepath.Join(home, ".gemini", ".env")
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	writeFixture(t, envPath, "# keep this comment\nGEMINI_API_KEY=old\nOTHER=value\n")
	writeFixture(t, settingsPath, `{"mcpServers":{"local":{"command":"demo"}}}`)

	operations, err := configureGemini(home, "key with space", "gemini-test")
	if err != nil {
		t.Fatal(err)
	}
	envOperation := operationFor(t, operations, envPath)
	envContent := string(envOperation.Content)
	for _, expected := range []string{
		"# keep this comment",
		`GEMINI_API_KEY="key with space"`,
		"GEMINI_MODEL=gemini-test",
		"GOOGLE_GEMINI_BASE_URL=" + geminiGatewayURL,
		"GOOGLE_GENAI_API_VERSION=" + geminiAPIVersion,
		"OTHER=value",
	} {
		if !strings.Contains(envContent, expected) {
			t.Fatalf("updated .env is missing %q:\n%s", expected, envContent)
		}
	}
	if strings.Count(envContent, "GEMINI_API_KEY=") != 1 {
		t.Fatalf("GEMINI_API_KEY was duplicated:\n%s", envContent)
	}

	settingsOperation := operationFor(t, operations, settingsPath)
	var settings map[string]any
	if err := json.Unmarshal(settingsOperation.Content, &settings); err != nil {
		t.Fatal(err)
	}
	security := settings["security"].(map[string]any)
	auth := security["auth"].(map[string]any)
	if auth["selectedType"] != "gemini-api-key" {
		t.Fatalf("unexpected Gemini auth selection: %#v", auth)
	}
	if _, ok := settings["mcpServers"]; !ok {
		t.Fatal("existing Gemini settings were dropped")
	}
}

func TestBuildConfigurationRejectsInvalidRequests(t *testing.T) {
	isolateHome(t)
	if _, _, err := buildConfiguration(ConfigurationRequest{}); err == nil {
		t.Fatal("expected missing API key error")
	}
	if _, _, err := buildConfiguration(ConfigurationRequest{APIKey: "key"}); err == nil {
		t.Fatal("expected missing target error")
	}
	if _, _, err := buildConfiguration(ConfigurationRequest{
		APIKey:  "key",
		Targets: []string{"unsupported"},
		Models:  map[string]string{"default": "model"},
	}); err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("expected unsupported target error, got %v", err)
	}
}

func TestValidateWrittenConfigReadsTheWrittenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, "not-json")
	err := validateWrittenConfig(fileOperation{
		Path:    path,
		Kind:    configJSON,
		Content: []byte(`{"this":"would be valid in memory"}`),
	})
	if err == nil {
		t.Fatal("expected validation to fail for the file on disk")
	}
}

func TestConfigureCodexReplacesConfigAndAuth(t *testing.T) {
	home := isolateHome(t)
	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	writeFixture(t, configPath, "model = \"old-model\"\n[model_providers.old]\nbase_url = \"https://old.example\"\n")
	writeFixture(t, authPath, `{"OPENAI_API_KEY":"old-key","tokens":{"access_token":"keep-out"}}`)

	operations, err := configureCodex(home, "new-key", "ignored-model")
	if err != nil {
		t.Fatal(err)
	}
	configOperation := operationFor(t, operations, configPath)
	expectedConfig := `model = "ignored-model"
model_provider = "ciyuanshen"
review_model = "gpt-5.6-sol"
model_reasoning_effort = "medium"
disable_response_storage = true
preferred_auth_method = "apikey"
service_tier = "fast"
web_search = "live"

[model_providers.ciyuanshen]
name = "ciyuanshen"
base_url = "https://ciyuanshen.top/v1"
wire_api = "responses"
`
	if string(configOperation.Content) != expectedConfig {
		t.Fatalf("unexpected Codex config:\n%s", configOperation.Content)
	}

	authOperation := operationFor(t, operations, authPath)
	var auth map[string]string
	if err := json.Unmarshal(authOperation.Content, &auth); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 1 || auth["OPENAI_API_KEY"] != "new-key" {
		t.Fatalf("auth.json was not replaced: %#v", auth)
	}
}

func TestConfigureAllSupportedClientsFromEmptyFiles(t *testing.T) {
	home := isolateHome(t)
	targets := []string{"claude", "codex", "gemini", "grok", "opencode", "openclaw", "hermes"}
	operations, _, err := buildConfiguration(ConfigurationRequest{
		APIKey:  "test-key",
		Targets: targets,
		Models:  map[string]string{"default": "gpt-test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			t.Fatal(err)
		}
		if err := validateWrittenConfig(operation); err != nil {
			t.Fatalf("%s output did not validate: %v", operation.ClientID, err)
		}
	}
	if len(operations) != 9 {
		t.Fatalf("expected 9 file operations, got %d", len(operations))
	}
	for _, target := range targets {
		if err := verifyManagedClientConfiguration(home, target, "test-key"); err != nil {
			t.Fatalf("%s output did not pass connection configuration check: %v", target, err)
		}
	}
}
