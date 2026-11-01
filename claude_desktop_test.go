package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestClaudeDesktopRoutesUseSafeAliasesForActualModels(t *testing.T) {
	routes, ids := claudeDesktopRoutes([]string{"gpt-6.1-sol", "gemini-3-pro", "deepseek-v4"})
	wantIDs := []string{"claude-sonnet-5", "claude-opus-5", "claude-haiku-5"}
	if len(ids) != len(wantIDs) {
		t.Fatalf("route ids = %v, want %v", ids, wantIDs)
	}
	for i, id := range wantIDs {
		if ids[i] != id {
			t.Fatalf("route %d = %q, want %q", i, ids[i], id)
		}
	}
	if routes["claude-sonnet-5"] != "gpt-6.1-sol" || routes["claude-opus-5"] != "gemini-3-pro" || routes["claude-haiku-5"] != "deepseek-v4" {
		t.Fatalf("unexpected route mapping: %#v", routes)
	}
}

func TestClaudeDesktopManyLabeledRoutes(t *testing.T) {
	for _, count := range []int{1, 5, 6, 200} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			selected := make([]string, count)
			for i := range selected {
				selected[i] = fmt.Sprintf("deepseek-v4.1-flash-%d", i)
			}
			routes, ids := claudeDesktopRoutes(append(selected, selected[0]))
			if len(ids) != count || len(routes) != count {
				t.Fatalf("models dropped: %d / %d", len(ids), len(routes))
			}
			specs := claudeDesktopRouterModels(ids, routes, ids[count-1])
			if obj(specs[0])["labelOverride"] != selected[count-1] {
				t.Fatalf("default not first: %#v", specs[0])
			}
			for _, raw := range specs {
				spec := obj(raw)
				id := str(spec["name"])
				if !isClaudeDesktopCompatibleModel(id) || mappedRouterModel(id, routes) != spec["labelOverride"] || mappedRouterModel(id+"[1m]", routes) != spec["labelOverride"] {
					t.Fatalf("label/route mismatch: %#v", spec)
				}
			}
			for i := 0; i < 100; i++ {
				if mappedRouterModel("claude-sonnet-5-20261001", routes) != selected[0] {
					t.Fatal("version alias routed to extra model")
				}
			}
			if count > 4 && ids[4] != "claude-sonnet-5-r2" {
				t.Fatal(ids)
			}
		})
	}
}

func TestClaudeDesktopLabeledProfileLifecycle(t *testing.T) {
	a, base := isolatedRouterApp(t)
	path := filepath.Join(filepath.Dir(base), claudeDesktopProfileID+".json")
	original := []byte(`{"custom":"keep","inferenceModels":["claude-sonnet-5"]}`)
	if err := atomicWrite(path, original); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	selected := []string{"deepseek-v4.1-flash", "alpha", "beta", "gamma", "fifth", "sixth"}
	status, err := a.startClaudeDesktopRouter(RouterRequest{DefaultModel: "sixth"}, "key", path, "", listener, selected)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := readJSONMap(path)
	if err != nil {
		t.Fatal(err)
	}
	specs := arr(profile["inferenceModels"])
	if len(specs) != 6 || obj(specs[0])["labelOverride"] != "sixth" || profile["custom"] != "keep" {
		t.Fatalf("bad profile: %#v", profile)
	}
	if mappedRouterModel(status.Alias, a.routers["claude-desktop"].routes) != "sixth" {
		t.Fatal(status)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+a.routers["claude-desktop"].token)
	response := httptest.NewRecorder()
	a.routers["claude-desktop"].ServeHTTP(response, request)
	catalog := routerInput(t, response.Body.String())
	if len(arr(catalog["data"])) != 6 || obj(arr(catalog["data"])[0])["display_name"] != "deepseek-v4.1-flash" {
		t.Fatalf("catalog: %s", response.Body.String())
	}
	if _, err = a.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil || string(restored) != string(original) {
		t.Fatalf("restore: %s %v", restored, err)
	}
}

func TestClaudeDesktopRejectsOverLimitWithoutWriting(t *testing.T) {
	a, base := isolatedRouterApp(t)
	path := filepath.Join(filepath.Dir(base), claudeDesktopProfileID+".json")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	selected := make([]string, 201)
	for i := range selected {
		selected[i] = fmt.Sprint(i)
	}
	_, err = a.startClaudeDesktopRouter(RouterRequest{}, "key", path, "", listener, selected)
	if err == nil || !strings.Contains(err.Error(), "200") {
		t.Fatalf("expected limit: %v", err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("profile written despite limit")
	}
	if _, err = os.Stat(routerJournalPath()); !os.IsNotExist(err) {
		t.Fatal("journal written despite limit")
	}
}
