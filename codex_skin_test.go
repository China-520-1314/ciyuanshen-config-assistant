package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexSkinLibraryAndValidation(t *testing.T) {
	isolateHome(t)
	a := NewApp()
	lib, err := a.GetCodexSkins()
	if err != nil || len(lib.Themes) != 6 {
		t.Fatalf("presets: %v", err)
	}
	s := lib.Themes[0]
	s.ID = "custom-test"
	s.Name = "测试 $& 皮肤"
	s.Favorite = true
	if err = a.SaveCodexSkin(s); err != nil {
		t.Fatal(err)
	}
	lib, err = a.GetCodexSkins()
	if err != nil || len(lib.Themes) != 7 || !lib.Themes[6].Favorite {
		t.Fatalf("save: %v", err)
	}
	if err = a.DeleteCodexSkin(s.ID); err != nil {
		t.Fatal(err)
	}
	lib, err = a.GetCodexSkins()
	if err != nil || len(lib.Themes) != 6 {
		t.Fatalf("delete: %v", err)
	}
	for _, change := range []func(*CodexSkin){func(s *CodexSkin) { s.Background = "https://example.com/x.png" }, func(s *CodexSkin) { s.Accent = "red; background:url(https://example.com)" }, func(s *CodexSkin) { s.ID = "../escape" }, func(s *CodexSkin) { s.FontSize = 100 }, func(s *CodexSkin) { s.Background = "data:image/png;base64,ZmFrZQ==" }} {
		bad := s
		change(&bad)
		if validateSkin(bad) == nil {
			t.Fatal("unsafe skin accepted")
		}
	}
	payload := skinPayload(s)
	if strings.Contains(payload, "__DREAM_SKIN_") && strings.Contains(payload, "_JSON__") {
		t.Fatal("unresolved renderer placeholders")
	}
	// Optional browser fixture for the real renderer smoke check.
	if out := os.Getenv("CIYUAN_SKIN_TEST_OUTPUT"); out != "" {
		if err = os.WriteFile(filepath.Join(out, "payload.js"), []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(s)
		if err = os.WriteFile(filepath.Join(out, "skin.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCodexSkinRejectsRemoteDebugger(t *testing.T) {
	for _, address := range []string{"ws://example.com:19329/devtools/page/a", "ws://127.0.0.1:9222/devtools/page/a", "ws://127.0.0.1:19329/other"} {
		if _, err := skinEvaluate(skinTarget{WS: address}, "1"); err == nil {
			t.Fatal("untrusted debugger accepted")
		}
	}
}
