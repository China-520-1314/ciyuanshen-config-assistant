package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type updateRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip updateRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

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

func TestPreferUpdateSourceUsesHealthyMirrorBeforeGitHub(t *testing.T) {
	mirror := UpdateInfo{CurrentVersion: "0.2.13", LatestVersion: "0.2.14", UpdateAvailable: true, DownloadURL: "https://api.ciyuanshen.top/downloads/installer.exe"}
	github := UpdateInfo{CurrentVersion: "0.2.13", LatestVersion: "0.2.14", UpdateAvailable: true, DownloadURL: "https://github.com/example/installer.exe"}
	if got := preferUpdateSource(mirror, github); got.DownloadURL != mirror.DownloadURL {
		t.Fatalf("selected download URL = %q, want mirror URL %q", got.DownloadURL, mirror.DownloadURL)
	}
}

func TestPreferUpdateSourceFallsBackToGitHub(t *testing.T) {
	mirror := UpdateInfo{Error: "更新清单格式无效"}
	github := UpdateInfo{CurrentVersion: "0.2.13", LatestVersion: "0.2.14", UpdateAvailable: true, DownloadURL: "https://github.com/example/installer.exe"}
	if got := preferUpdateSource(mirror, github); got.DownloadURL != github.DownloadURL {
		t.Fatalf("selected download URL = %q, want GitHub fallback URL %q", got.DownloadURL, github.DownloadURL)
	}
}

func TestPreferUpdateSourceReportsBothFailures(t *testing.T) {
	result := preferUpdateSource(UpdateInfo{Error: "镜像失败"}, UpdateInfo{Error: "GitHub 失败"})
	if result.Error != "镜像失败；GitHub 失败" {
		t.Fatalf("combined error = %q", result.Error)
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

func TestSelectMacInstallerPrefersUniversalDMG(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "ciyuanshen-config-assistant-macos-universal.zip", BrowserDownloadURL: "https://github.com/example/app.zip"},
		{Name: "ciyuanshen-config-assistant-macos-universal.dmg", BrowserDownloadURL: "https://github.com/example/app.dmg"},
		{Name: "ciyuanshen-config-assistant-amd64-installer.exe", BrowserDownloadURL: "https://github.com/example/app.exe"},
	}
	asset := selectReleaseAsset(assets, "darwin")
	if asset.Name != "ciyuanshen-config-assistant-macos-universal.dmg" {
		t.Fatalf("macOS asset = %q, want universal DMG", asset.Name)
	}
}

func TestSelectMacInstallerFallsBackToUniversalZIP(t *testing.T) {
	assets := []githubReleaseAsset{{Name: "ciyuanshen-config-assistant-macos-universal.zip", BrowserDownloadURL: "https://github.com/example/app.zip"}}
	asset := selectReleaseAsset(assets, "darwin")
	if asset.Name != "ciyuanshen-config-assistant-macos-universal.zip" {
		t.Fatalf("macOS fallback asset = %q, want universal ZIP", asset.Name)
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

func TestDownloadUpdateWithFallbackUsesGitHubAfterPrimaryHTTPFailure(t *testing.T) {
	payload := []byte("verified GitHub installer")
	hash := sha256.Sum256(payload)
	primaryURL := "https://api.ciyuanshen.top/downloads/ciyuanshen-config-assistant/installer.exe"
	fallbackURL := "https://github.com/China-520-1314/ciyuanshen-config-assistant/releases/download/v0.2.16/installer.exe"
	var primaryRequests, fallbackRequests int
	client := &http.Client{Transport: updateRoundTripper(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case primaryURL:
			primaryRequests++
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("mirror unavailable")), Request: request}, nil
		case fallbackURL:
			fallbackRequests++
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload)), Request: request}, nil
		default:
			t.Fatalf("unexpected update URL: %s", request.URL)
			return nil, nil
		}
	})}
	var progress []UpdateProgress
	downloaded, err := downloadUpdateWithFallback(client, UpdateInfo{DownloadURL: primaryURL, SHA256: hex.EncodeToString(hash[:])}, func(primary UpdateInfo) (UpdateInfo, error) {
		if primary.DownloadURL != primaryURL {
			t.Fatalf("fallback received the wrong primary update: %#v", primary)
		}
		return UpdateInfo{DownloadURL: fallbackURL, SHA256: hex.EncodeToString(hash[:])}, nil
	}, func(next UpdateProgress) {
		progress = append(progress, next)
	})
	if err != nil {
		t.Fatalf("download with fallback failed: %v", err)
	}
	defer os.Remove(downloaded.path)
	if primaryRequests != 1 || fallbackRequests != 1 {
		t.Fatalf("primary requests = %d, fallback requests = %d; want one each", primaryRequests, fallbackRequests)
	}
	if downloaded.downloadURL != fallbackURL {
		t.Fatalf("download URL = %q, want GitHub fallback URL %q", downloaded.downloadURL, fallbackURL)
	}
	content, err := os.ReadFile(downloaded.path)
	if err != nil {
		t.Fatalf("read fallback installer: %v", err)
	}
	if !bytes.Equal(content, payload) {
		t.Fatalf("fallback installer = %q, want %q", content, payload)
	}
	if !containsUpdateProgressStage(progress, "fallback") {
		t.Fatalf("fallback progress was not emitted: %#v", progress)
	}
}

func TestDownloadUpdateWithFallbackRetriesGitHubAfterPrimaryChecksumFailure(t *testing.T) {
	payload := []byte("verified GitHub installer")
	hash := sha256.Sum256(payload)
	primaryURL := "https://api.ciyuanshen.top/downloads/ciyuanshen-config-assistant/installer.exe"
	fallbackURL := "https://github.com/China-520-1314/ciyuanshen-config-assistant/releases/download/v0.2.16/installer.exe"
	var fallbackRequests int
	client := &http.Client{Transport: updateRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == primaryURL {
			invalid := []byte("truncated mirror installer")
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), ContentLength: int64(len(invalid)), Body: io.NopCloser(bytes.NewReader(invalid)), Request: request}, nil
		}
		if request.URL.String() == fallbackURL {
			fallbackRequests++
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload)), Request: request}, nil
		}
		t.Fatalf("unexpected update URL: %s", request.URL)
		return nil, nil
	})}
	downloaded, err := downloadUpdateWithFallback(client, UpdateInfo{DownloadURL: primaryURL, SHA256: hex.EncodeToString(hash[:])}, func(UpdateInfo) (UpdateInfo, error) {
		return UpdateInfo{DownloadURL: fallbackURL, SHA256: hex.EncodeToString(hash[:])}, nil
	}, nil)
	if err != nil {
		t.Fatalf("download with checksum fallback failed: %v", err)
	}
	defer os.Remove(downloaded.path)
	if fallbackRequests != 1 {
		t.Fatalf("fallback requests = %d, want 1", fallbackRequests)
	}
	if downloaded.downloadURL != fallbackURL {
		t.Fatalf("download URL = %q, want fallback URL %q", downloaded.downloadURL, fallbackURL)
	}
}

func containsUpdateProgressStage(progress []UpdateProgress, stage string) bool {
	for _, item := range progress {
		if item.Stage == stage {
			return true
		}
	}
	return false
}

func TestUpdateProgressWriterReportsActualDownloadProgress(t *testing.T) {
	var destination bytes.Buffer
	var progress []UpdateProgress
	writer := newUpdateProgressWriter(&destination, 10, func(next UpdateProgress) {
		progress = append(progress, next)
	})

	for _, chunk := range []string{"abc", "defgh", "ij"} {
		if _, err := writer.Write([]byte(chunk)); err != nil {
			t.Fatalf("write progress chunk: %v", err)
		}
	}
	writer.emit(true)

	if destination.String() != "abcdefghij" {
		t.Fatalf("downloaded content = %q", destination.String())
	}
	if len(progress) < 3 {
		t.Fatalf("received %d progress events, want at least 3", len(progress))
	}
	last := progress[len(progress)-1]
	if last.Stage != "downloading" || last.DownloadedBytes != 10 || last.TotalBytes != 10 || last.Percent != 100 {
		t.Fatalf("unexpected final progress: %#v", last)
	}
}
