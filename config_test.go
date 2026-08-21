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

func TestConfigureCodexAddsDefaultsAndPreservesExistingConfig(t *testing.T) {
	home := isolateHome(t)
	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	writeFixture(t, configPath, "model = \"old-model\"\ncustom_top_level = \"keep\"\n[model_providers.old]\nbase_url = \"https://old.example\"\ncustom = true\n")
	writeFixture(t, authPath, `{"OPENAI_API_KEY":"old-key","tokens":{"access_token":"keep-out"}}`)

	operations, err := configureCodex(home, "new-key", "ignored-model")
	if err != nil {
		t.Fatal(err)
	}
	configOperation := operationFor(t, operations, configPath)
	configContent := string(configOperation.Content)
	for _, expected := range []string{
		`model = "old-model"`,
		`custom_top_level = "keep"`,
		`model_provider = "ciyuanshen"`,
		`model_reasoning_effort = "max"`,
		`disable_response_storage = true`,
		`preferred_auth_method = "apikey"`,
		`service_tier = "fast"`,
		`web_search = "live"`,
		`[model_providers.old]`,
		`base_url = "https://old.example"`,
		`custom = true`,
		`[model_providers.ciyuanshen]`,
		`name = "ciyuanshen"`,
		`base_url = "https://api.ciyuanshen.top/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
	} {
		if !strings.Contains(configContent, expected) {
			t.Fatalf("Codex config is missing %q:\n%s", expected, configContent)
		}
	}

	authOperation := operationFor(t, operations, authPath)
	var auth map[string]string
	if err := json.Unmarshal(authOperation.Content, &auth); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 1 || auth["OPENAI_API_KEY"] != "new-key" {
		t.Fatalf("auth.json was not updated: %#v", auth)
	}
}

func TestConfigureCodexUsesNewDefaultModelForEmptyConfig(t *testing.T) {
	home := isolateHome(t)
	operations, err := configureCodex(home, "new-key", "")
	if err != nil {
		t.Fatal(err)
	}
	content := string(operationFor(t, operations, filepath.Join(home, ".codex", "config.toml")).Content)
	if !strings.Contains(content, `model = "gpt-5.6-terra"`) {
		t.Fatalf("empty Codex config did not receive the new default model:\n%s", content)
	}
	if strings.Contains(content, "review_model") {
		t.Fatal("legacy review_model should not be added to a new Codex config")
	}
}

func TestPatchCodexConfigPreservesExistingManagedValuesAndOtherTables(t *testing.T) {
	existing := `# keep this comment
model_provider = "user-provider"
model = "user-model"
disable_response_storage = false

[model_providers.ciyuanshen]
name = "user-name"
custom_provider_value = "keep"

[other]
model = "other-model"
custom = true
`
	patched := patchCodexConfig(existing, "ignored-model")
	for _, expected := range []string{
		`model_provider = "user-provider"`,
		`model = "user-model"`,
		`disable_response_storage = false`,
		`model_reasoning_effort = "max"`,
		`preferred_auth_method = "apikey"`,
		`service_tier = "fast"`,
		`web_search = "live"`,
		`[model_providers.user-provider]`,
		`name = "user-provider"`,
		`custom_provider_value = "keep"`,
		`base_url = "https://api.ciyuanshen.top/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`[other]`,
		`model = "other-model"`,
		`custom = true`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("patched Codex config is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Count(patched, `model_provider =`) != 1 || strings.Count(patched, `model = "user-model"`) != 1 {
		t.Fatalf("existing top-level assignments were duplicated:\n%s", patched)
	}
}

func TestPatchCodexConfigUnifiesDuplicateCustomProvider(t *testing.T) {
	existing := `model_provider = "ciyuanshen"
model = "gpt-5.5"
model_reasoning_effort = "medium"
disable_response_storage = true
preferred_auth_method = "apikey"
service_tier = "fast"
web_search = "live"

[model_providers.custom]
name = "custom"
wire_api = "responses"
requires_openai_auth = true

[model_providers.ciyuanshen]
name = "ciyuanshen"
base_url = "[https://ciyuanshen.top/v1](https://ciyuanshen.top/v1)"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "ignored-model")
	if strings.Count(patched, "[model_providers.") != 1 {
		t.Fatalf("provider tables were not unified:\n%s", patched)
	}
	for _, expected := range []string{
		`model_provider = "ciyuanshen"`,
		`[model_providers.ciyuanshen]`,
		`name = "ciyuanshen"`,
		`base_url = "https://api.ciyuanshen.top/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("unified Codex config is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Contains(patched, "model_providers.custom") || strings.Contains(patched, `name = "custom"`) {
		t.Fatalf("stale custom provider remained:\n%s", patched)
	}
	if strings.Count(patched, `name = "ciyuanshen"`) != 1 || strings.Count(patched, `base_url = "https://api.ciyuanshen.top/v1"`) != 1 {
		t.Fatalf("provider fields were duplicated:\n%s", patched)
	}
}

func TestPatchCodexConfigKeepsCustomProviderName(t *testing.T) {
	existing := `model_provider = "custom"
[model_providers.custom]
name = "custom"
base_url = "https://custom.example/v1"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "")
	if !strings.Contains(patched, `model_provider = "custom"`) || !strings.Contains(patched, `[model_providers.custom]`) || !strings.Contains(patched, `name = "custom"`) {
		t.Fatalf("custom provider name was not preserved:\n%s", patched)
	}
	if !strings.Contains(patched, `base_url = "https://custom.example/v1"`) {
		t.Fatalf("custom provider URL was overwritten:\n%s", patched)
	}
	if strings.Contains(patched, `[model_providers.ciyuanshen]`) {
		t.Fatalf("unexpected ciyuanshen provider was added:\n%s", patched)
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
