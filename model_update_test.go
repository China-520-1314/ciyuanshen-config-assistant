package main

import (
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
