package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"0.2.0", "0.1.9", 1},
		{"v0.1.0", "0.1.0", 0},
		{"0.1.0-beta", "0.1.0", -1},
		{"0.1.0", "0.1.0-beta", 1},
		{"0.1", "0.1.0", 0},
	}
	for _, test := range tests {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestCheckForUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/bad" {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		if request.URL.Path == "/invalid" {
			_, _ = writer.Write([]byte("not json"))
			return
		}
		_, _ = writer.Write([]byte(`{"version":"0.2.0","downloadUrl":"https://ciyuanshen.top/downloads/app.exe","releaseNotes":"修复配置检测","publishedAt":"2026-08-18"}`))
	}))
	defer server.Close()

	result := checkForUpdates(server.Client(), "0.1.0", server.URL)
	if result.Error != "" || !result.UpdateAvailable || result.LatestVersion != "0.2.0" {
		t.Fatalf("unexpected update result: %#v", result)
	}
	if result.ReleaseNotes != "修复配置检测" {
		t.Fatalf("release notes were not returned: %#v", result)
	}

	badDownload := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"version":"0.2.0","downloadUrl":"http://insecure.example/app.exe"}`))
	}))
	defer badDownload.Close()
	result = checkForUpdates(badDownload.Client(), "0.1.0", badDownload.URL)
	if !strings.Contains(result.Error, "HTTPS") {
		t.Fatalf("insecure download should be rejected: %#v", result)
	}

	for _, path := range []string{"/bad", "/invalid"} {
		result = checkForUpdates(server.Client(), "0.1.0", server.URL+path)
		if result.Error == "" {
			t.Fatalf("expected error for %s", path)
		}
	}
}
