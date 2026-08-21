package main

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestVerifyCodexConfiguration(t *testing.T) {
	home := isolateHome(t)
	operations, err := configureCodex(home, "test-key", "ignored")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyManagedClientConfiguration(home, "codex", "test-key"); err != nil {
		t.Fatal(err)
	}
	if err := verifyManagedClientConfiguration(home, "codex", "other-key"); err == nil {
		t.Fatal("mismatched API Key should fail verification")
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	configuredContent := string(operationFor(t, operations, configPath).Content)
	writeFixture(t, configPath, strings.Replace(configuredContent, "model_reasoning_effort = \"max\"\n", "model_reasoning_effort = \"ultra\"\n", 1))
	if err := verifyManagedClientConfiguration(home, "codex", "test-key"); err != nil {
		t.Fatalf("a valid non-default reasoning effort should pass: %v", err)
	}
	writeFixture(t, configPath, strings.Replace(configuredContent, "model_reasoning_effort = \"max\"\n", "", 1))
	if err := verifyManagedClientConfiguration(home, "codex", "test-key"); err == nil {
		t.Fatal("an omitted required reasoning effort should fail verification")
	}
	writeFixture(t, configPath, "model_provider = \"other\"\n")
	if err := verifyManagedClientConfiguration(home, "codex", "test-key"); err == nil {
		t.Fatal("unexpected Codex config should fail verification")
	}
}

func TestVerifyCodexConfigurationAcceptsPreservedCustomProvider(t *testing.T) {
	home := isolateHome(t)
	writeFixture(t, filepath.Join(home, ".codex", "config.toml"), `model_provider = "custom"
model = "gpt-5.5"
model_reasoning_effort = "medium"
disable_response_storage = true
preferred_auth_method = "apikey"
service_tier = "fast"
web_search = "live"

[model_providers.custom]
name = "custom"
base_url = "https://custom.example/v1"
wire_api = "responses"
requires_openai_auth = true
`)
	writeFixture(t, filepath.Join(home, ".codex", "auth.json"), `{"OPENAI_API_KEY":"test-key"}`)
	if err := verifyManagedClientConfiguration(home, "codex", "test-key"); err != nil {
		t.Fatalf("custom provider should pass configuration verification: %v", err)
	}
}

func TestEnvFileValue(t *testing.T) {
	content := "# comment\nGEMINI_API_KEY=plain\nQUOTED=\"value with space\"\n"
	if value, ok := envFileValue(content, "GEMINI_API_KEY"); !ok || value != "plain" {
		t.Fatalf("unexpected plain value: %q, %t", value, ok)
	}
	if value, ok := envFileValue(content, "QUOTED"); !ok || value != "value with space" {
		t.Fatalf("unexpected quoted value: %q, %t", value, ok)
	}
}

func TestReadConfiguredClientAPIKeyForAllSupportedClients(t *testing.T) {
	home := isolateHome(t)
	targets := []string{"claude", "codex", "gemini", "grok", "opencode", "openclaw", "hermes"}
	operations, _, err := buildConfiguration(ConfigurationRequest{
		APIKey:  "stored-key",
		Targets: targets,
		Models:  map[string]string{"default": "test-model"},
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
		key, err := readConfiguredClientAPIKey(home, target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if key != "stored-key" {
			t.Fatalf("%s: key = %q, want stored-key", target, key)
		}
	}
}

func TestCheckClientConnectionsVerifiesConfigAndGateway(t *testing.T) {
	home := isolateHome(t)
	operations, err := configureCodex(home, "test-key", "ignored")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			t.Fatal(err)
		}
	}

	var gotAuthorization string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotAuthorization = request.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-test"}]}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	report, err := checkClientConnections(client, ConnectionCheckRequest{Targets: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuthorization != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want Bearer test-key", gotAuthorization)
	}
	if len(report.Results) != 1 || !report.Results[0].Configured || !report.Results[0].Success {
		t.Fatalf("unexpected successful check report: %#v", report)
	}

	badClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"message":"invalid key"}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	failedReport, err := checkClientConnections(badClient, ConnectionCheckRequest{Targets: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(failedReport.Results) != 1 || !failedReport.Results[0].Configured || failedReport.Results[0].Success {
		t.Fatalf("unexpected failed check report: %#v", failedReport)
	}
	if !strings.Contains(failedReport.Results[0].Message, "无效") {
		t.Fatalf("failed check message = %q", failedReport.Results[0].Message)
	}
}

func TestCheckClientConnectionsRejectsUnknownTarget(t *testing.T) {
	isolateHome(t)
	_, err := checkClientConnections(http.DefaultClient, ConnectionCheckRequest{Targets: []string{"unknown"}})
	if err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("expected unknown target error, got %v", err)
	}
}
