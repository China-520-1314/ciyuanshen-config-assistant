package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func startThreeRouters(t *testing.T) (*App, map[string]string) {
	t.Helper()
	a, codexPath := isolatedRouterApp(t)
	home := filepath.Dir(filepath.Dir(codexPath))
	paths := map[string]string{"codex": codexPath, "claude": filepath.Join(home, ".claude", "settings.json"), "claude-desktop": filepath.Join(home, "desktop", claudeDesktopProfileID+".json")}
	for _, id := range []string{"claude", "claude-desktop"} {
		if err := atomicWrite(paths[id], []byte(`{"original":"`+id+`"}`)); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"codex", "claude"} {
		if _, err := a.StartModelRouter(RouterRequest{Client: id, APIKey: "key-" + id, Models: []string{"alpha", "beta"}, DefaultModel: "beta"}); err != nil {
			t.Fatal(err)
		}
	}
	// Exercise the Desktop start helper on Linux with an isolated Desktop profile.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a.routerMu.Lock()
	_, err = a.startClaudeDesktopRouter(RouterRequest{DefaultModel: "beta"}, "key-desktop", paths["claude-desktop"], home, listener, []string{"alpha", "beta"})
	a.routerMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return a, paths
}

func TestThreeRoutersIndependentLifecycle(t *testing.T) {
	a, paths := startThreeRouters(t)
	statuses := a.GetModelRouterStatuses()
	addresses := map[string]bool{}
	tokens := map[string]bool{}
	for _, id := range routerClients {
		p := a.routers[id]
		if !statuses[id].Running || statuses[id].Client != id || addresses[statuses[id].Address] || tokens[p.token] {
			t.Fatalf("not isolated: %#v", statuses)
		}
		addresses[statuses[id].Address] = true
		tokens[p.token] = true
		if _, err := os.Stat(routerJournalPath(id)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.StartModelRouter(RouterRequest{Client: "claude", APIKey: "key", Model: "alpha"}); err == nil {
		t.Fatal("duplicate client accepted")
	}
	var wg sync.WaitGroup
	for _, id := range routerClients {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				p := a.routers[id]
				req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
				req.Header.Set("Authorization", "Bearer "+p.token)
				w := httptest.NewRecorder()
				p.ServeHTTP(w, req)
				if w.Code != 200 {
					t.Errorf("%s catalog: %d", id, w.Code)
				}
				_ = a.GetModelRouterStatuses()
			}
		}()
	}
	wg.Wait()
	otherBefore, err := os.ReadFile(paths["claude-desktop"])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.StopClientModelRouter("claude"); err != nil {
		t.Fatal(err)
	}
	statuses = a.GetModelRouterStatuses()
	if statuses["claude"].Running || statuses["claude"].Recoverable || !statuses["codex"].Running || !statuses["claude-desktop"].Running {
		t.Fatal(statuses)
	}
	restored, _ := os.ReadFile(paths["claude"])
	if string(restored) != `{"original":"claude"}` {
		t.Fatal(string(restored))
	}
	otherAfter, _ := os.ReadFile(paths["claude-desktop"])
	if string(otherAfter) != string(otherBefore) {
		t.Fatal("other profile changed")
	}
	for _, id := range []string{"codex", "claude-desktop"} {
		req, _ := http.NewRequest("GET", statuses[id].Address+"/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+a.routers[id].token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
	}
	if _, err = a.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
	for _, id := range routerClients {
		if _, err := os.Stat(routerJournalPath(id)); !os.IsNotExist(err) {
			t.Fatalf("journal remains: %s", id)
		}
	}
}

func TestMultiRouterCrashRecoveryAndLegacy(t *testing.T) {
	a, paths := startThreeRouters(t)
	for _, p := range a.routers {
		_ = p.server.Close()
	}
	a.routers = nil
	restarted := NewApp()
	if !restarted.GetModelRouterStatuses()["claude-desktop"].Recoverable {
		t.Fatal("missing recovery")
	}
	if _, err := restarted.StopClientModelRouter("claude"); err != nil {
		t.Fatal(err)
	}
	if !restarted.GetModelRouterStatuses()["codex"].Recoverable {
		t.Fatal("other recovery removed")
	}
	if _, err := restarted.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
	// Emulate the shared recovery file written by v0.2.53 for Desktop.
	original := []byte(`{"old":true}`)
	installed := []byte(`{"routed":true}`)
	if err := atomicWrite(paths["claude-desktop"], installed); err != nil {
		t.Fatal(err)
	}
	j := routerJournal{Path: paths["claude-desktop"], Original: original, Installed: installed, Existed: true, Address: "127.0.0.1:1", Models: []string{"claude-sonnet-5"}, Routes: map[string]string{"claude-sonnet-5": "alpha"}, DefaultModel: "alpha"}
	raw, _ := json.Marshal(j)
	if err := atomicWrite(routerJournalPath(), raw); err != nil {
		t.Fatal(err)
	}
	states := restarted.GetModelRouterStatuses()
	if !states["claude-desktop"].Recoverable || states["codex"].Recoverable || states["claude-desktop"].Models[0] != "alpha" {
		t.Fatal(states)
	}
	if _, err := restarted.StartModelRouter(RouterRequest{Client: "codex", APIKey: "key", Model: "alpha"}); err == nil {
		t.Fatal("legacy shared slot overwritten")
	}
	if _, err := restarted.StopClientModelRouter("codex"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(routerJournalPath()); err != nil {
		t.Fatal("wrong client removed legacy journal")
	}
	if _, err := restarted.StopClientModelRouter("claude-desktop"); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(paths["claude-desktop"])
	if string(restored) != string(original) {
		t.Fatal(string(restored))
	}
}

func TestStopAllContinuesAfterOneRecoveryFailure(t *testing.T) {
	a, _ := startThreeRouters(t)
	p := a.routers["codex"]
	validPath := p.journal.Path
	p.journal.Path = filepath.Join(t.TempDir(), "invalid.json")
	_, err := a.StopModelRouter()
	if err == nil || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("expected aggregated failure: %v", err)
	}
	if a.routers["claude"] != nil || a.routers["claude-desktop"] != nil || a.routers["codex"] == nil {
		t.Fatal("failed to restore healthy clients")
	}
	p.journal.Path = validPath
	if _, err = a.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
}
