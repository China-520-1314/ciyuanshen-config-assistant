package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestAntigravityNativeGateway(t *testing.T) {
	app := NewApp()
	app.antigravityAppKey = "upstream-key"
	app.antigravityAppToken = "local-token"
	app.antigravityAppModel = "gemini-test"
	calls := 0
	app.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.ciyuanshen.top" || r.URL.Path != "/v1beta/models/gemini-test:streamGenerateContent" || r.URL.Query().Get("alt") != "sse" || strings.Contains(r.URL.String(), "token") || r.URL.Query().Has("key") {
			t.Fatalf("wrong upstream URL: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer upstream-key" || r.Header.Get("x-goog-api-key") != "upstream-key" {
			t.Fatal("wrong upstream credentials")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"contents":[]}` {
			t.Fatalf("request changed: %s", body)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"candidates\":[]}\n\n"))}, nil
	})}
	for _, tc := range []struct {
		path, token, origin string
		status              int
	}{
		{"/v1beta/models/gemini-test:streamGenerateContent?alt=sse&key=local-token", "", "", 200},
		{"/v1beta/models/gemini-test:generateContent", "wrong", "", 401},
		{"/v1beta/models/gemini-other:generateContent", "local-token", "", 404},
		{"/v1internal:loadCodeAssist", "local-token", "", 404},
		{"/v1beta/models/gemini-test:generateContent", "local-token", "https://example.com", 403},
	} {
		req := httptest.NewRequest("POST", tc.path, strings.NewReader(`{"contents":[]}`))
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		req.Header.Set("Origin", tc.origin)
		out := httptest.NewRecorder()
		app.serveAntigravityApp(out, req)
		if out.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.path, out.Code, out.Body.String())
		}
		if tc.status == 200 && out.Body.String() != "data: {\"candidates\":[]}\n\n" {
			t.Fatal("native stream was wrapped")
		}
	}
	if calls != 1 {
		t.Fatalf("upstream calls %d", calls)
	}
}

func TestAntigravityGatewayEnvironment(t *testing.T) {
	env := antigravityGatewayEnvironment([]string{"PATH=/bin", "AGY_LLM_GATEWAY_API_KEY=stale", "agy_llm_gateway_models=old", "CLOUD_CODE_URL=old"}, "http://127.0.0.1:123", "token", "gemini-test")
	joined := strings.Join(env, "\n")
	if len(env) != 5 || strings.Contains(joined, "stale") || strings.Contains(joined, "old") || !strings.Contains(joined, "AGY_LLM_GATEWAY_WIRE_PROTOCOL=genai") || !strings.Contains(joined, "AGY_LLM_GATEWAY_MODELS=gemini-test") {
		t.Fatalf("wrong environment: %v", env)
	}
}
