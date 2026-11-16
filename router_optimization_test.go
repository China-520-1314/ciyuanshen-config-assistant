package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRouterModelOptionsLifecycle(t *testing.T) {
	a, path := isolatedRouterApp(t)
	original := []byte("model = 'old'\nmodel_context_window = 900000\nmodel_auto_compact_token_limit = 800000\n")
	if err := atomicWrite(path, original); err != nil {
		t.Fatal(err)
	}
	disabled := false
	options := map[string]RouterModelOptions{"alpha": {ContextWindow: 200000, Reasoning: true, ImageInput: &disabled, ParallelToolCalls: &disabled}}
	status, err := a.StartModelRouter(RouterRequest{APIKey: "test", Models: []string{"alpha", "beta"}, ModelOptions: options})
	if err != nil {
		t.Fatal(err)
	}
	if status.ModelOptions["alpha"].ContextWindow != 200000 {
		t.Fatal("options missing from status")
	}
	root, err := readTOMLMap(path)
	if err != nil {
		t.Fatal(err)
	}
	if root["model_context_window"] != nil || root["model_auto_compact_token_limit"] != nil {
		t.Fatal("global context overrides catalog")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "ciyuanshen-model-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := routerInput(t, string(data))
	alpha, beta := obj(arr(catalog["models"])[0]), obj(arr(catalog["models"])[1])
	if alpha["context_window"] != float64(200000) || alpha["supports_parallel_tool_calls"] != false || len(arr(alpha["input_modalities"])) != 1 || len(arr(alpha["supported_reasoning_levels"])) != 3 {
		t.Fatalf("bad alpha: %s", data)
	}
	if beta["context_window"] != float64(114688) || len(arr(beta["supported_reasoning_levels"])) != 0 {
		t.Fatalf("bad default catalog: %s", data)
	}
	journal, err := readRouterJournal()
	if err != nil || journal.ModelOptions["alpha"].ContextWindow != 200000 {
		t.Fatalf("options not persisted: %+v %v", journal, err)
	}
	if _, err := a.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
	root, err = readTOMLMap(path)
	if err != nil || root["model_context_window"] != int64(900000) {
		t.Fatalf("context not restored: %+v %v", root, err)
	}
}

func TestRouterModelCapabilitiesOnWire(t *testing.T) {
	disabled := false
	options := RouterModelOptions{Reasoning: true, ImageInput: &disabled, ParallelToolCalls: &disabled}
	input := routerInput(t, `{"input":"hello","reasoning":{"effort":"high"},"parallel_tool_calls":true}`)
	chat, _, err := responsesToChatWithOptions(input, "gemini-fixture", options)
	if err != nil || chat["reasoning_effort"] != "high" || chat["parallel_tool_calls"] != false {
		t.Fatalf("bad options mapping: %+v %v", chat, err)
	}
	chat, _, err = responsesToChat(input, "unknown-fixture")
	if err != nil || chat["reasoning_effort"] != nil {
		t.Fatalf("reasoning enabled without opt-in: %+v %v", chat, err)
	}
	input["reasoning"] = map[string]any{"effort": "ultra"}
	if _, _, err := responsesToChatWithOptions(input, "gemini-fixture", options); err == nil {
		t.Fatal("unsupported effort silently accepted")
	}
	image := routerInput(t, `{"input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AA=="}]}]}`)
	if _, _, err := responsesToChatWithOptions(image, "gemini-fixture", options); err == nil {
		t.Fatal("disabled image accepted")
	}
}

func TestRouterReasoningUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		body := `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":80},"completion_tokens_details":{"reasoning_tokens":12}}}`
		if stream {
			body = "data: " + body + "\n\ndata: [DONE]\n\n"
		}
		w := httptest.NewRecorder()
		if err := chatToResponses(w, strings.NewReader(body), stream, "fixture", nil); err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if !stream {
			response = routerInput(t, w.Body.String())
		} else {
			for _, line := range strings.Split(w.Body.String(), "\n") {
				if strings.HasPrefix(line, "data: ") {
					event := routerInput(t, strings.TrimPrefix(line, "data: "))
					if event["type"] == "response.completed" {
						response = obj(event["response"])
					}
				}
			}
		}
		usage := obj(response["usage"])
		if obj(usage["output_tokens_details"])["reasoning_tokens"] != float64(12) || obj(usage["input_tokens_details"])["cached_tokens"] != float64(80) || usage["total_tokens"] != float64(120) {
			t.Fatalf("bad usage: %s", w.Body.String())
		}
	}
}

func TestRouterRetryPreservesFinalErrorBody(t *testing.T) {
	for _, status := range []int{429, 503, 400} {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(status)
			io.WriteString(w, `{"error":{"message":"fixture unavailable"}}`)
		}))
		p := &modelRouter{client: server.Client()}
		resp, err := p.doChatRequest(context.Background(), server.URL, "fixture", []byte(`{}`))
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		expected := int32(3)
		if status == 400 {
			expected = 1
		}
		if err != nil || calls.Load() != expected || !json.Valid(body) {
			t.Fatalf("status=%d calls=%d body=%s err=%v", status, calls.Load(), body, err)
		}
	}
}

func TestRouterRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := make(chan struct{})
	p := &modelRouter{client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(called)
		return &http.Response{StatusCode: 503, Header: http.Header{"Retry-After": []string{"30"}}, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})}}
	done := make(chan error, 1)
	go func() { _, err := p.doChatRequest(ctx, "http://fixture.invalid", "fixture", nil); done <- err }()
	<-called
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry ignored cancellation")
	}
	now := time.Now().UTC().Truncate(time.Second)
	for _, value := range []string{"31", "9999999999", now.Add(time.Minute).Format(http.TimeFormat)} {
		if _, retry := routerRetryDelay(value, 0, now); retry {
			t.Fatalf("retried before server delay: %s", value)
		}
	}
	if delay, retry := routerRetryDelay(now.Add(5*time.Second).Format(http.TimeFormat), 0, now); !retry || delay != 5*time.Second {
		t.Fatalf("bad HTTP date delay: %s %v", delay, retry)
	}
}
