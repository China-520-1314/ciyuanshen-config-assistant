package main

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetClientConfigurationMasksSecretsUntilExplicitlyRevealed(t *testing.T) {
	home := isolateHome(t)
	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	writeFixture(t, configPath, "model = \"gpt-test\"\n")
	writeFixture(t, authPath, `{"OPENAI_API_KEY":"secret-key","tokens":{"access_token":"other-secret"}}`)

	app := NewApp()
	masked, err := app.GetClientConfiguration("codex", false)
	if err != nil {
		t.Fatal(err)
	}
	if !masked.SecretsRedacted {
		t.Fatal("expected masked configuration view")
	}
	maskedAuth := configurationViewFile(t, masked, authPath)
	if strings.Contains(maskedAuth.Content, "secret-key") || strings.Contains(maskedAuth.Content, "other-secret") {
		t.Fatalf("configuration view exposed a credential: %s", maskedAuth.Content)
	}
	if !strings.Contains(maskedAuth.Content, "********") {
		t.Fatalf("configuration view did not mask credentials: %s", maskedAuth.Content)
	}
	if !json.Valid([]byte(maskedAuth.Content)) {
		t.Fatalf("masked JSON should remain valid: %s", maskedAuth.Content)
	}

	revealed, err := app.GetClientConfiguration("codex", true)
	if err != nil {
		t.Fatal(err)
	}
	if revealed.SecretsRedacted {
		t.Fatal("expected explicitly revealed configuration view")
	}
	if got := configurationViewFile(t, revealed, authPath).Content; !strings.Contains(got, "secret-key") {
		t.Fatalf("revealed view did not contain original content: %s", got)
	}
}

func TestReadConfiguredClientDefaultModelForManagedClients(t *testing.T) {
	home := isolateHome(t)
	targets := []string{"claude", "codex", "gemini", "grok", "opencode", "openclaw", "hermes"}
	operations, _, err := buildConfiguration(ConfigurationRequest{
		APIKey:  "stored-key",
		Targets: targets,
		Models:  map[string]string{"default": "model-selected"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range targets {
		model, err := readConfiguredClientDefaultModel(home, target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if model != "model-selected" {
			t.Fatalf("%s model = %q, want model-selected", target, model)
		}
	}
}

func TestGetConfiguredToolModelsReturnsConfiguredDefault(t *testing.T) {
	home := isolateHome(t)
	operations, err := configureCodex(home, "stored-key", "gpt-existing")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			t.Fatal(err)
		}
	}
	app := NewApp()
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer stored-key" {
			t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-existing"},{"id":"gpt-other"}]}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	result, err := app.GetConfiguredToolModels("codex")
	if err != nil {
		t.Fatal(err)
	}
	if result.SelectedModel != "gpt-existing" || len(result.Models) != 2 {
		t.Fatalf("unexpected configured models: %#v", result)
	}
}

func configurationViewFile(t *testing.T, view ClientConfigurationView, path string) ClientConfigurationFile {
	t.Helper()
	for _, file := range view.Files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("configuration view did not include %s", path)
	return ClientConfigurationFile{}
}
