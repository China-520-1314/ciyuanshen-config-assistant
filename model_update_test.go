package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchCodexModelChangesOnlyTopLevelModel(t *testing.T) {
	existing := `model_provider = "custom"
model = "old-model" # keep this comment
model_reasoning_effort = "medium"

[model_providers.custom]
name = "custom"
base_url = "https://custom.example/v1"
model = "provider-field"
`
	patched, err := patchCodexModel(existing, "new-model")
	if err != nil {
		t.Fatalf("patchCodexModel returned error: %v", err)
	}
	if !strings.Contains(patched, `model = "new-model" # keep this comment`) {
		t.Fatalf("top-level model was not updated:\n%s", patched)
	}
	if !strings.Contains(patched, `model = "provider-field"`) || !strings.Contains(patched, `base_url = "https://custom.example/v1"`) {
		t.Fatalf("provider content was changed:\n%s", patched)
	}
}

func TestUpdateConfiguredCodexModelKeepsOtherConfig(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "config.toml")
	original := `model_provider = "ciyuanshen"
model = "gpt-5.5"
model_reasoning_effort = "medium"
disable_response_storage = true

[model_providers.ciyuanshen]
name = "ciyuanshen"
base_url = "https://api.ciyuanshen.top/v1"
wire_api = "responses"
requires_openai_auth = true

[other]
keep = "yes"
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updateConfiguredClientModel(home, "codex", "gpt-5.6-terra"); err != nil {
		t.Fatalf("updateConfiguredClientModel returned error: %v", err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if !strings.Contains(text, `model = "gpt-5.6-terra"`) || !strings.Contains(text, `keep = "yes"`) {
		t.Fatalf("model update did not preserve config:\n%s", text)
	}
	if strings.Contains(text, `model = "gpt-5.5"`) {
		t.Fatalf("old model remained:\n%s", text)
	}
}

func TestUpdateConfiguredGeminiModelAddsCompatibilitySettings(t *testing.T) {
	home := t.TempDir()
	envPath := filepath.Join(home, ".gemini", ".env")
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	writeFixture(t, envPath, "GEMINI_API_KEY=test-key\nGEMINI_MODEL=gemini-3.5-flash\n")
	writeFixture(t, settingsPath, `{"mcpServers":{"local":{"command":"demo"}}}`)

	if err := updateConfiguredClientModel(home, "gemini", "gemini-3.8-flash"); err != nil {
		t.Fatalf("updateConfiguredClientModel returned error: %v", err)
	}
	updatedEnv, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if model, ok := envFileValue(string(updatedEnv), "GEMINI_MODEL"); !ok || model != "gemini-3.8-flash" {
		t.Fatalf("GEMINI_MODEL = %q, %t", model, ok)
	}
	updatedSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(updatedSettings, &settings); err != nil {
		t.Fatal(err)
	}
	assertGeminiCLIModelCompatibility(t, settings, "gemini-3.7-flash", "gemini-3.8-flash")
	if _, ok := settings["mcpServers"]; !ok {
		t.Fatal("existing Gemini settings were dropped")
	}
}
