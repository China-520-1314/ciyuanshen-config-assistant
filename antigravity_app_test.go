package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAntigravityAppProxyServesConfiguredModel(t *testing.T) {
	app := NewApp()
	app.antigravityAppKey = "test-key"
	app.antigravityAppModel = "gemini-3-pro"
	request := httptest.NewRequest("POST", "/v1internal:fetchAvailableModels", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	app.serveAntigravityApp(response, request)
	if response.Code != 200 {
		t.Fatalf("catalog status = %d, want 200", response.Code)
	}
	var catalog struct {
		Models map[string]map[string]any `json:"models"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	model := catalog.Models["custom-ciyuanshen"]
	if model["displayName"] != "gemini-3-pro" || model["model"] != antigravityAppModelID {
		t.Fatalf("unexpected model catalog entry: %#v", model)
	}
}

func TestAntigravityAppRequestAndGeminiEndpoint(t *testing.T) {
	body, err := antigravityAppRequestBody([]byte(`{"model":"placeholder","request":{"contents":[{"role":"user"}]}}`))
	if err != nil || string(body) != `{"contents":[{"role":"user"}]}` {
		t.Fatalf("request extraction = %s, %v", body, err)
	}
	if _, err = antigravityAppRequestBody([]byte(`not-json`)); err == nil {
		t.Fatal("invalid Cloud Code JSON was accepted")
	}
	if got, want := antigravityAppGeminiEndpoint("gemini-3-pro", "key+value", true), "https://api.ciyuanshen.top/v1/models/gemini-3-pro:streamGenerateContent?alt=sse&key=key%2Bvalue"; got != want {
		t.Fatalf("stream endpoint = %q, want %q", got, want)
	}
}

func TestAntigravityAppConfiguredModelsUseMemory(t *testing.T) {
	app := NewApp()
	if _, err := app.GetConfiguredToolModels("antigravity-app"); err == nil || !strings.Contains(err.Error(), "一键配置") {
		t.Fatalf("missing setup guidance: %v", err)
	}
	app.antigravityAppKey = "memory-key"
	app.antigravityAppModel = "gemini-3-pro"
	app.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer memory-key" {
			t.Fatal("did not use in-memory key")
		}
		return testRawJSONResponse(r, `{"data":[{"id":"gemini-3-pro"},{"id":"gpt-test"}]}`), nil
	})}
	result, err := app.GetConfiguredToolModels("antigravity-app")
	if err != nil || len(result.Models) != 1 || result.Models[0].ID != "gemini-3-pro" || result.SelectedModel != "gemini-3-pro" {
		t.Fatalf("unexpected models: %#v, %v", result, err)
	}
	validation, err := app.ValidateToolKey(ToolKeyValidationRequest{ClientID: "antigravity-app", APIKey: "memory-key"})
	if err != nil || len(validation.Models) != 1 {
		t.Fatalf("manual key validation failed: %#v %v", validation, err)
	}
}

func TestAntigravityAppAccountGroups(t *testing.T) {
	app := NewApp()
	app.account = dashboardSession{AccessToken: "session"}
	app.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/user/self/groups":
			return testDashboardResponse(r, map[string]any{"gemini": map[string]any{"desc": "Gemini", "ratio": 1}}), nil
		case "/api/user/models":
			return testDashboardResponse(r, []string{"gemini-3-pro", "gpt-test"}), nil
		case "/api/token/":
			return testDashboardResponse(r, map[string]any{"page": 1, "page_size": 100, "total": 0, "items": []dashboardToken{}}), nil
		default:
			t.Fatalf("unexpected URL %s", r.URL.Path)
			return nil, nil
		}
	})}
	result, err := app.GetAccountToolOptions("antigravity-app")
	if err != nil || len(result.Groups) != 1 || len(result.Groups[0].Models) != 1 || result.Groups[0].Models[0].ID != "gemini-3-pro" {
		t.Fatalf("account groups lost: %#v %v", result, err)
	}
}

func TestAntigravityAppStreamingAndOfficialControl(t *testing.T) {
	app := NewApp()
	app.antigravityAppKey = "gateway-key"
	app.antigravityAppModel = "gemini-test"
	app.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "daily-cloudcode-pa.googleapis.com" {
			if r.Header.Get("Authorization") != "Bearer google-token" || strings.Contains(r.URL.String(), "gateway-key") {
				t.Fatal("control credentials mixed")
			}
			return testRawJSONResponse(r, `{"currentTier":{"id":"free"}}`), nil
		}
		if r.URL.Host != "api.ciyuanshen.top" || !strings.Contains(r.URL.Path, "streamGenerateContent") || r.Header.Get("Authorization") != "" {
			t.Fatal("wrong model request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: {\"candidates\":[{\"index\":0}]}\n\n")), Header: make(http.Header)}, nil
	})}
	for _, path := range []string{"/v1internal:streamGenerateContent", "/v1internal:loadCodeAssist"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{"request":{"contents":[]}}`))
		req.Header.Set("Authorization", "Bearer google-token")
		out := httptest.NewRecorder()
		app.serveAntigravityApp(out, req)
		if out.Code != 200 {
			t.Fatalf("%s: %d %s", path, out.Code, out.Body.String())
		}
		if strings.Contains(path, "stream") && !strings.Contains(out.Body.String(), `"response":{"candidates"`) {
			t.Fatal(out.Body.String())
		}
	}
}
func TestAntigravityAppConnectionCheckUsesSession(t *testing.T) {
	app := NewApp()
	report, err := app.CheckClientConnections(ConnectionCheckRequest{Targets: []string{"antigravity-app"}})
	if err != nil || len(report.Results) != 1 || !strings.Contains(report.Results[0].Message, "代理未启动") {
		t.Fatalf("%#v %v", report, err)
	}
	app.antigravityAppServer = &http.Server{}
	app.antigravityAppKey = "key"
	app.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return testRawJSONResponse(r, `{"data":[{"id":"gemini-test"}]}`), nil
	})}
	report, err = app.CheckClientConnections(ConnectionCheckRequest{Targets: []string{"antigravity-app"}})
	if err != nil || !report.Results[0].Success || !strings.Contains(report.Results[0].Message, "不代表") {
		t.Fatalf("%#v %v", report, err)
	}
}
