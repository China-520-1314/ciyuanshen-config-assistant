package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestClaudeDesktopProfileListsAvailableModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	content, err := claudeDesktopProfileContent(path, "test-key", "claude-opus-4-6", []string{
		"claude-sonnet-4-6", "claude-opus-4-6", "gpt-6.1-sol", "claude-haiku-4-5", "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		InferenceModels []string `json:"inferenceModels"`
	}
	if err := json.Unmarshal(content, &profile); err != nil {
		t.Fatal(err)
	}
	want := []string{"claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5"}
	if len(profile.InferenceModels) != len(want) {
		t.Fatalf("models = %v, want %v", profile.InferenceModels, want)
	}
	for i := range want {
		if profile.InferenceModels[i] != want[i] {
			t.Fatalf("models = %v, want %v", profile.InferenceModels, want)
		}
	}
}

func TestClaudeDesktopInferenceModelsLimit(t *testing.T) {
	available := make([]string, maxClaudeDesktopModels+10)
	for i := range available {
		available[i] = "claude-sonnet-test-" + string(rune('a'+i/26)) + string(rune('a'+i%26))
	}
	models := claudeDesktopInferenceModels("claude-opus-default", available)
	if len(models) != maxClaudeDesktopModels || models[0] != "claude-opus-default" {
		t.Fatalf("expected selected model first and %d total, got %d", maxClaudeDesktopModels, len(models))
	}
}
