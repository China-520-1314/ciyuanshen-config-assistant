package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestFetchNPMLatestVersion(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || !strings.Contains(request.URL.String(), "registry.npmjs.org") {
			t.Fatalf("unexpected npm request: %s %s", request.Method, request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"version":"v1.2.3"}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	version, err := fetchNPMLatestVersion(client, "@openai/codex")
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", version)
	}
}

func TestNormaliseToolVersion(t *testing.T) {
	for raw, want := range map[string]string{
		"claude 2.1.4":   "2.1.4",
		"v0.42.0-beta.1": "0.42.0-beta.1",
		"no version":     "",
	} {
		if got := normaliseToolVersion(raw); got != want {
			t.Fatalf("normaliseToolVersion(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestLifecycleOutputDetailDecodesGBK(t *testing.T) {
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("安装失败：网络连接超时"))
	if err != nil {
		t.Fatal(err)
	}
	if got := lifecycleOutputDetail(encoded); got != "安装失败：网络连接超时" {
		t.Fatalf("decoded lifecycle output = %q", got)
	}
}

func TestLifecycleOutputDetailPreservesUTF8(t *testing.T) {
	const want = "更新失败：请重试"
	if got := lifecycleOutputDetail([]byte(want)); got != want {
		t.Fatalf("UTF-8 lifecycle output = %q, want %q", got, want)
	}
}

func TestNpmLifecycleCommandQuotesWindowsPath(t *testing.T) {
	command := npmLifecycleCommandForOS(context.Background(), `C:\Program Files\nodejs\npm.cmd`, "@openai/codex", "windows")
	if len(command.Args) != 5 {
		t.Fatalf("unexpected command args: %#v", command.Args)
	}
	line := command.Args[4]
	if !strings.HasPrefix(line, `call "C:\Program Files\nodejs\npm.cmd"`) {
		t.Fatalf("Windows npm command is not quoted: %q", line)
	}
	if !strings.Contains(line, `install --global @openai/codex@latest`) {
		t.Fatalf("Windows npm command lost package args: %q", line)
	}
}

func TestQuoteWindowsBatchPathEscapesPercent(t *testing.T) {
	got := quoteWindowsBatchPath(`C:\Tools\path%name%\npm.cmd`)
	if got != `C:\Tools\path%%%%name%%%%\npm.cmd` {
		t.Fatalf("quoted percent path = %q", got)
	}
}
