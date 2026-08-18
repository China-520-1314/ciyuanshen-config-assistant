package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
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
