package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		_, _ = writer.Write([]byte(`{"version":"0.2.0","downloadUrl":"https://api.ciyuanshen.top/downloads/app.exe","releaseNotes":"修复配置检测","publishedAt":"2026-08-18"}`))
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

func TestCheckGitHubReleasePrefersTheInstaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Fatalf("unexpected Accept header: %q", request.Header.Get("Accept"))
		}
		_, _ = writer.Write([]byte(`{
  "tag_name":"v0.2.0",
  "body":"修复更新检测",
  "published_at":"2026-08-18T12:00:00Z",
  "assets":[
    {"name":"ciyuanshen-config-assistant.exe","browser_download_url":"https://github.com/example/app.exe"},
    {"name":"ciyuanshen-config-assistant-amd64-installer.exe","browser_download_url":"https://github.com/example/installer.exe"}
  ]
}`))
	}))
	defer server.Close()

	result := checkGitHubRelease(server.Client(), "0.1.0", server.URL)
	if result.Error != "" || !result.UpdateAvailable {
		t.Fatalf("unexpected GitHub update result: %#v", result)
	}
	if result.LatestVersion != "0.2.0" || result.DownloadURL != "https://github.com/example/installer.exe" {
		t.Fatalf("installer was not selected: %#v", result)
	}
}

func TestCheckGitHubReleaseRejectsMissingInstallerForNewVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{
  "tag_name":"v0.2.0",
  "assets":[
    {"name":"ciyuanshen-config-assistant.exe","browser_download_url":"https://github.com/example/app.exe"}
  ]
}`))
	}))
	defer server.Close()

	result := checkGitHubRelease(server.Client(), "0.1.0", server.URL)
	if !strings.Contains(result.Error, "Windows 安装包") {
		t.Fatalf("missing installer should be reported: %#v", result)
	}
}

func TestBuildUpdateInstallScriptStopsAllExistingInstances(t *testing.T) {
	script := buildUpdateInstallScript(42, "ciyuanshen-config-assistant", `C:\Program Files\ciyuanshen\ciyuanshen-config-assistant.exe`, `C:\Users\Public\update.exe`)
	for _, expected := range []string{
		"Stop-Process -Id $oldPid -Force",
		"Get-Process -Name $processName",
		"Start-Sleep -Milliseconds 500",
		"-ArgumentList '/S'",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("update script missing %q:\n%s", expected, script)
		}
	}
}

func TestNewUpdateDownloadHTTPClientUsesDedicatedTimeout(t *testing.T) {
	base := &http.Client{Timeout: 15 * time.Second, Transport: http.DefaultTransport}
	client := newUpdateDownloadHTTPClient(base)
	if client.Timeout != updateDownloadTimeout {
		t.Fatalf("download timeout = %s, want %s", client.Timeout, updateDownloadTimeout)
	}
	if client.Timeout <= base.Timeout {
		t.Fatalf("download timeout %s must exceed API timeout %s", client.Timeout, base.Timeout)
	}
	if client.Transport != base.Transport {
		t.Fatal("download client must preserve the base transport")
	}

	defaultClient := newUpdateDownloadHTTPClient(nil)
	if defaultClient.Timeout != updateDownloadTimeout {
		t.Fatalf("default download timeout = %s, want %s", defaultClient.Timeout, updateDownloadTimeout)
	}
}
