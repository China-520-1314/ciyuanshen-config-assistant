package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func routerInput(t *testing.T, raw string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRouterToolRoundTrip(t *testing.T) {
	in := routerInput(t, `{"instructions":"help","input":[{"role":"developer","content":"rules"},{"role":"user","content":[{"type":"input_text","text":"edit"},{"type":"input_image","image_url":"data:image/png;base64,AA=="}]}],"tools":[{"type":"namespace","name":"functions","tools":[{"type":"function","name":"shell","parameters":{"type":"object"}},{"type":"custom","name":"apply_patch","description":"patch files"}]}]}`)
	chat, specs, err := responsesToChat(in, "claude-test")
	if err != nil {
		t.Fatal(err)
	}
	if chat["model"] != "claude-test" || len(arr(chat["messages"])) != 3 || str(obj(arr(chat["messages"])[1])["role"]) != "system" {
		t.Fatalf("bad request: %#v", chat)
	}
	if !specs["functions__apply_patch"].Custom {
		t.Fatal("custom tool lost")
	}
	up := `{"choices":[{"finish_reason":"tool_calls","message":{"reasoning_content":"plan","tool_calls":[{"id":"c1","type":"function","function":{"name":"functions__shell","arguments":"{\"cmd\":\"pwd\"}"}},{"id":"c2","type":"function","function":{"name":"functions__apply_patch","arguments":"{\"input\":\"*** Begin Patch\\n*** End Patch\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":3}}`
	w := httptest.NewRecorder()
	if err := chatToResponses(w, strings.NewReader(up), false, "claude-test", specs); err != nil {
		t.Fatal(err)
	}
	response := routerInput(t, w.Body.String())
	items := arr(response["output"])
	if len(items) != 3 || str(obj(items[2])["type"]) != "custom_tool_call" || str(obj(items[2])["namespace"]) != "functions" || str(obj(items[2])["input"]) != "*** Begin Patch\n*** End Patch" {
		t.Fatalf("bad output: %s", w.Body.String())
	}
	in["input"] = append(items, map[string]any{"type": "function_call_output", "call_id": "c1", "output": "/tmp"}, map[string]any{"type": "custom_tool_call_output", "call_id": "c2", "output": "ok"})
	chat, _, err = responsesToChat(in, "deepseek-test")
	if err != nil {
		t.Fatal(err)
	}
	messages := arr(chat["messages"])
	assistant := obj(messages[1])
	if len(arr(assistant["tool_calls"])) != 2 || assistant["reasoning_content"] != "plan" || str(obj(messages[3])["role"]) != "tool" {
		t.Fatalf("bad replay: %#v", chat)
	}
}

func TestRouterStreamsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, event string
		fail              bool
	}{
		{"text", "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n", "response.completed", false},
		{"truncated", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", "response.failed", true},
		{"bad-json", "data: invalid\n\n", "response.failed", true},
		{"limit", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n", "response.incomplete", false},
		{"error", "data: {\"error\":{\"message\":\"secret-upstream-key\"}}\n\n", "response.failed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := chatToResponses(w, strings.NewReader(tc.body), true, "grok-test", nil)
			if (err != nil) != tc.fail || !strings.Contains(w.Body.String(), "event: "+tc.event+"\n") || strings.Contains(w.Body.String(), "secret-upstream-key") {
				t.Fatalf("err=%v response=%s", err, w.Body.String())
			}
			sequence := 0
			for _, line := range strings.Split(w.Body.String(), "\n") {
				if strings.HasPrefix(line, "data: ") {
					e := routerInput(t, strings.TrimPrefix(line, "data: "))
					if e["sequence_number"] != float64(sequence) {
						t.Fatal("unordered events")
					}
					sequence++
				}
			}
		})
	}
}

func TestRouterStreamInterleavedTools(t *testing.T) {
	var body strings.Builder
	for _, chunk := range []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"shell","arguments":"{\"cmd\":"}},{"index":1,"id":"b","function":{"name":"patch","arguments":"{\"input\":"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"patch text\"}"}},{"index":0,"function":{"arguments":"\"pwd\"}"}}]},"finish_reason":"tool_calls"}]}`,
	} {
		fmt.Fprintf(&body, "data: %s\n\n", chunk)
	}
	w := httptest.NewRecorder()
	err := chatToResponses(w, strings.NewReader(body.String()), true, "gemini-test", map[string]routeTool{"shell": {Name: "shell"}, "patch": {Name: "patch", Custom: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"response.function_call_arguments.done", "response.custom_tool_call_input.done", "patch text", "response.completed"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("missing %s: %s", want, w.Body.String())
		}
	}
}

func TestRouterAdditionalToolsAndInvalidBatch(t *testing.T) {
	in := routerInput(t, `{"input":[{"role":"user","content":"hi"},{"type":"additional_tools","tools":[{"type":"function","name":"shell"}]}],"tools":[{"type":"function","name":"shell"}]}`)
	chat, specs, err := responsesToChat(in, "test")
	if err != nil || len(arr(chat["tools"])) != 1 || len(arr(chat["messages"])) != 1 {
		t.Fatalf("carrier conversion: %v %+v", err, chat)
	}
	w := httptest.NewRecorder()
	body := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"shell","arguments":"{}"}},{"index":1,"id":"b","function":{"name":"shell","arguments":"invalid"}}]},"finish_reason":"tool_calls"}]}` + "\n\n"
	if err := chatToResponses(w, strings.NewReader(body), true, "test", specs); err == nil {
		t.Fatal("invalid tool batch accepted")
	}
	if strings.Contains(w.Body.String(), "response.function_call_arguments") {
		t.Fatal("partial tool batch exposed before validation")
	}
}

func TestRouterHTTPForwarding(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer upstream-test-key" {
					t.Error("wrong upstream request")
				}
				var v map[string]any
				_ = json.NewDecoder(r.Body).Decode(&v)
				if v["model"] != "deepseek-test" || v["stream"] != stream {
					t.Error("mapping not applied")
				}
				chunk := `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`
				if stream {
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
				} else {
					io.WriteString(w, chunk)
				}
			}))
			defer upstream.Close()
			p := &modelRouter{token: "local-test", key: "upstream-test-key", upstream: upstream.URL, client: upstream.Client(), status: RouterStatus{Model: "deepseek-test"}}
			for _, auth := range []bool{false, true} {
				r := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(fmt.Sprintf(`{"model":"ignored","input":"hi","stream":%t}`, stream)))
				if auth {
					r.Header.Set("Authorization", "Bearer local-test")
				}
				w := httptest.NewRecorder()
				p.ServeHTTP(w, r)
				if !auth && w.Code != 401 || auth && (w.Code != 200 || !strings.Contains(w.Body.String(), "ok")) {
					t.Fatalf("%d %s", w.Code, w.Body.String())
				}
			}
			r := httptest.NewRequest("GET", "/v1/models", nil)
			r.Header.Set("Authorization", "Bearer local-test")
			r.Header.Set("Origin", "https://example.com")
			w := httptest.NewRecorder()
			p.ServeHTTP(w, r)
			if w.Code != 401 {
				t.Fatal("browser origin accepted")
			}
			if p.status.Requests != 1 || p.status.Failures != 0 {
				t.Fatalf("wrong stats: %+v", p.status)
			}
		})
	}
}

func isolatedRouterApp(t *testing.T) (*App, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("APPDATA", filepath.Join(home, "config"))
	t.Setenv("CODEX_HOME", "")
	a := NewApp()
	a.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != defaultGatewayURL+"/models" {
			t.Errorf("unexpected URL: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"claude-test"}]}`)), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() {
		if a.router != nil {
			_ = a.router.server.Close()
		}
	})
	return a, filepath.Join(home, ".codex", "config.toml")
}

func TestRouterLifecycle(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			a, path := isolatedRouterApp(t)
			original := []byte("# preserve exactly\nmodel = 'old'\napproval_policy = 'on-request'\n")
			if exists {
				if err := atomicWrite(path, original); err != nil {
					t.Fatal(err)
				}
			}
			authPath := filepath.Join(filepath.Dir(path), "auth.json")
			if err := atomicWrite(authPath, []byte(`{"OPENAI_API_KEY":"unchanged-test"}`)); err != nil {
				t.Fatal(err)
			}
			request := RouterRequest{APIKey: "test", Model: "claude-test"}
			status, err := a.StartModelRouter(request)
			if err != nil {
				t.Fatal(err)
			}
			if !status.Running || !strings.HasPrefix(status.Address, "http://127.0.0.1:") {
				t.Fatalf("bad status: %+v", status)
			}
			root, err := readTOMLMap(path)
			if err != nil {
				t.Fatal(err)
			}
			if root["model_provider"] != routerProvider || root["model"] != routerAlias {
				t.Fatal("not configured")
			}
			if _, err := a.StartModelRouter(request); err == nil {
				t.Fatal("double start accepted")
			}
			if result := a.Configure(ConfigurationRequest{}); result.Success || !strings.Contains(result.Error, "路由") {
				t.Fatal("config guard missing")
			}
			var wg sync.WaitGroup
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); _ = a.GetModelRouterStatus() }()
			}
			wg.Wait()
			if _, err := a.StopModelRouter(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if exists && (err != nil || !bytes.Equal(got, original)) || !exists && !os.IsNotExist(err) {
				t.Fatalf("restore failed %q %v", got, err)
			}
			auth, _ := os.ReadFile(authPath)
			if string(auth) != `{"OPENAI_API_KEY":"unchanged-test"}` {
				t.Fatal("auth changed")
			}
			if _, err := os.Stat(routerJournalPath()); !os.IsNotExist(err) {
				t.Fatal("journal not removed")
			}
		})
	}
}

func TestRouterConflictAndCrashRecovery(t *testing.T) {
	a, path := isolatedRouterApp(t)
	original := []byte("model = 'before'\n")
	if err := atomicWrite(path, original); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartModelRouter(RouterRequest{APIKey: "test", Model: "claude-test"}); err != nil {
		t.Fatal(err)
	}
	installed, _ := os.ReadFile(path)
	changed := append(append([]byte{}, installed...), []byte("\n# another application changed this\n")...)
	if err := atomicWrite(path, changed); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StopModelRouter(); err == nil {
		t.Fatal("must preserve external edit")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, changed) {
		t.Fatal("external edit overwritten")
	}
	if err := atomicWrite(path, installed); err != nil {
		t.Fatal(err)
	}
	if err := a.router.server.Close(); err != nil {
		t.Fatal(err)
	}
	a.router = nil // Simulate restart after the listener died.
	if a.GetModelRouterStatus().LastError == "" {
		t.Fatal("recovery not surfaced")
	}
	if _, err := a.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("crash recovery failed")
	}
}

func TestRouterRejectsInvalidModelAndUnsupportedInput(t *testing.T) {
	a, path := isolatedRouterApp(t)
	if _, err := a.StartModelRouter(RouterRequest{APIKey: "test", Model: "unknown"}); err == nil {
		t.Fatal("unknown model accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid request changed config")
	}
	for _, raw := range []string{`{"input":"hi","previous_response_id":"old"}`, `{"input":"hi","tools":[{"type":"web_search"}]}`, `{"input":[{"role":"user","content":[{"type":"input_file"}]}]}`} {
		if _, _, err := responsesToChat(routerInput(t, raw), "test"); err == nil {
			t.Fatalf("unsupported request accepted: %s", raw)
		}
	}
	if err := createRouterJournal([]byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := createRouterJournal([]byte("second")); err == nil {
		t.Fatal("journal overwritten")
	}
	got, _ := os.ReadFile(routerJournalPath())
	if string(got) != "first" {
		t.Fatal("exclusive creation failed")
	}
}

// Opt-in real client contract check. Only the canned local upstream is used;
// the CLI has an isolated home and receives no real credentials or task.
func TestRouterCodexCLIContract(t *testing.T) {
	if os.Getenv("CIYUANSHEN_TEST_CODEX_CLI") != "1" {
		t.Skip("set CIYUANSHEN_TEST_CODEX_CLI=1 to check an installed Codex CLI")
	}
	binary, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	a, path := isolatedRouterApp(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]any
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			t.Error(err)
		}
		if v["model"] != "claude-test" {
			t.Error("incorrect actual model")
		}
		if calls.Add(1) == 1 {
			found := false
			for _, tool := range arr(v["tools"]) {
				if str(obj(obj(tool)["function"])["name"]) == "functions__exec" {
					found = true
				}
			}
			if !found {
				t.Error("Codex custom exec tool was not forwarded")
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"fixture-call\",\"function\":{\"name\":\"functions__exec\",\"arguments\":\"{\\\"input\\\":\\\"text('TOOL_ROUTER_OK')\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		found := false
		for _, message := range arr(v["messages"]) {
			m := obj(message)
			if m["role"] == "tool" && strings.Contains(str(m["content"]), "TOOL_ROUTER_OK") {
				found = true
			}
		}
		if !found {
			t.Error("Codex did not replay custom tool output")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ROUTER_CONTRACT_OK\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	if _, err := a.StartModelRouter(RouterRequest{APIKey: "test", Model: "claude-test"}); err != nil {
		t.Fatal(err)
	}
	a.router.mu.Lock()
	a.router.upstream = upstream.URL
	a.router.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "exec", "--skip-git-repo-check", "--ephemeral", "-s", "read-only", "-C", filepath.Dir(filepath.Dir(path)), "This is a local protocol fixture. Print the tool marker with text() and then reply with the fixture marker.")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "CODEX_HOME=" + filepath.Dir(path), "XDG_CONFIG_HOME=" + os.Getenv("XDG_CONFIG_HOME"), "LANG=C.UTF-8"}
	output, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("ROUTER_CONTRACT_OK")) {
		t.Fatalf("Codex contract failed: %v\n%s\nrouter: %+v", err, output, a.GetModelRouterStatus())
	}
	if _, err := a.StopModelRouter(); err != nil {
		t.Fatal(err)
	}
}
