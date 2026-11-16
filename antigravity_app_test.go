package main

import (
	"encoding/json"
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
	model := catalog.Models[antigravityAppModelID]
	if model["displayName"] != "gemini-3-pro" || model["model"] != "gemini-3-pro" {
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
