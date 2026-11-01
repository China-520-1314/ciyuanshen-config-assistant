package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestAnthropicDesktopSystemRoles(t *testing.T) {
	for _, role := range []string{"system", "developer"} {
		for _, content := range []string{`"desktop instructions"`, `[{"type":"text","text":"desktop instructions","cache_control":{"type":"ephemeral"}}]`} {
			t.Run(role+"/"+content, func(t *testing.T) {
				in := routerInput(t, fmt.Sprintf(`{"system":"top-level instructions","messages":[{"role":%q,"content":%s},{"role":"user","content":"hello"}]}`, role, content))
				out, err := anthropicToChat(in, "actual-model")
				if err != nil {
					t.Fatal(err)
				}
				messages := arr(out["messages"])
				if len(messages) != 3 || obj(messages[1])["role"] != "system" {
					t.Fatalf("system role lost: %#v", messages)
				}
				encoded, _ := json.Marshal(messages)
				if !strings.Contains(string(encoded), "desktop instructions") || strings.Contains(string(encoded), "cache_control") {
					t.Fatalf("system content not converted: %s", encoded)
				}
			})
		}
	}
}

func TestAnthropicRejectsInvalidRolesWithLocation(t *testing.T) {
	for _, message := range []string{`{"role":"unexpected","content":"hi"}`, `{"content":"hi"}`, `null`} {
		_, err := anthropicToChat(routerInput(t, `{"messages":[`+message+`]}`), "model")
		if err == nil || !strings.Contains(err.Error(), "messages[0]") {
			t.Fatalf("expected useful role diagnostic, got %v", err)
		}
	}
}

func TestAnthropicToolResultsPrecedeUserText(t *testing.T) {
	in := routerInput(t, `{"messages":[{"role":"user","content":"run tools"},{"role":"assistant","content":[{"type":"tool_use","id":"a","name":"read","input":{}},{"type":"tool_use","id":"b","name":"read","input":{}}]},{"role":"user","content":[{"type":"text","text":"continue"},{"type":"tool_result","tool_use_id":"a","content":"first"},{"type":"tool_result","tool_use_id":"b","content":[{"type":"text","text":"second"}]}]}]}`)
	out, err := anthropicToChat(in, "model")
	if err != nil {
		t.Fatal(err)
	}
	messages := arr(out["messages"])
	if len(messages) != 5 || obj(messages[2])["role"] != "tool" || obj(messages[3])["role"] != "tool" || obj(messages[4])["role"] != "user" {
		t.Fatalf("tool results must immediately follow assistant calls: %#v", messages)
	}
	if obj(messages[1])["content"] != nil {
		t.Fatalf("tool-only assistant content must be null: %#v", messages[1])
	}
}

func TestAnthropicAssistantMergingPreservesContent(t *testing.T) {
	for _, contents := range [][]string{
		{`"first"`, `"second"`},
		{`"first"`, `[{"type":"text","text":"second"}]`},
		{`[{"type":"text","text":"first"}]`, `"second"`},
	} {
		in := routerInput(t, fmt.Sprintf(`{"messages":[{"role":"assistant","content":%s},{"role":"assistant","content":%s}]}`, contents[0], contents[1]))
		out, err := anthropicToChat(in, "model")
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(out)
		if len(arr(out["messages"])) != 1 || !strings.Contains(string(encoded), "first") || !strings.Contains(string(encoded), "second") {
			t.Fatalf("assistant history lost: %s", encoded)
		}
	}
}

// Exercise the authenticated HTTP endpoint, not only the conversion helper:
// Desktop sees Claude aliases while the gateway receives the actual model.
func TestClaudeDesktopMappedConversation(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			var requests []map[string]any
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer upstream-key" {
					t.Errorf("unexpected upstream request: %s", r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requests = append(requests, body)
				w.Header().Set("Content-Type", "application/json")
				if len(requests) == 1 {
					io.WriteString(w, `{"id":"chatcmpl-fixture","model":"actual-model","usage":{"prompt_tokens":42,"completion_tokens":12},"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"checking","tool_calls":[{"id":"call-a","type":"function","function":{"name":"read","arguments":"{\"path\":\"a.txt\"}"}},{"id":"call-b","type":"function","function":{"name":"read","arguments":"{\"path\":\"b.txt\"}"}}]}}]}`)
				} else {
					io.WriteString(w, `{"id":"chatcmpl-final","usage":{"prompt_tokens":65,"completion_tokens":6},"choices":[{"finish_reason":"stop","message":{"content":"all done"}}]}`)
				}
			}))
			defer upstream.Close()
			p := &modelRouter{token: "local-token", key: "upstream-key", upstream: upstream.URL, routes: map[string]string{"claude-sonnet-5": "actual-model"}, client: upstream.Client(), status: RouterStatus{Model: "actual-model"}}
			request := func(body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer local-token")
				w := httptest.NewRecorder()
				p.ServeHTTP(w, r)
				if w.Code != 200 {
					t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
				}
				return w
			}
			w := request(fmt.Sprintf(`{"model":"claude-sonnet-5","stream":%t,"messages":[{"role":"system","content":[{"type":"text","text":"desktop rules"}]},{"role":"developer","content":"developer rules"},{"role":"user","content":"inspect"}],"tools":[{"name":"read","input_schema":{"type":"object"}}]}`, stream))
			if requests[0]["model"] != "actual-model" || obj(arr(requests[0]["messages"])[0])["role"] != "system" {
				t.Fatalf("upstream mapping/content incorrect: %#v", requests[0])
			}
			var blocks []any
			if stream {
				var events []string
				var usage map[string]any
				arguments := map[int]string{}
				stop := ""
				for _, line := range strings.Split(w.Body.String(), "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					e := routerInput(t, strings.TrimPrefix(line, "data: "))
					events = append(events, str(e["type"]))
					switch str(e["type"]) {
					case "message_start":
						if obj(e["message"])["model"] != "claude-sonnet-5" || obj(obj(e["message"])["usage"])["input_tokens"] != float64(42) {
							t.Fatalf("Desktop alias/usage missing: %#v", e)
						}
					case "content_block_start":
						if int(e["index"].(float64)) != len(blocks) {
							t.Fatalf("invalid block index: %#v", e)
						}
						blocks = append(blocks, e["content_block"])
					case "content_block_delta":
						index := int(e["index"].(float64))
						delta := obj(e["delta"])
						if delta["type"] == "input_json_delta" {
							arguments[index] += str(delta["partial_json"])
						} else if delta["type"] == "text_delta" {
							obj(blocks[index])["text"] = str(obj(blocks[index])["text"]) + str(delta["text"])
						}
					case "content_block_stop":
						index := int(e["index"].(float64))
						if args, ok := arguments[index]; ok {
							obj(blocks[index])["input"] = routerInput(t, args)
						}
					case "message_delta":
						stop, usage = str(obj(e["delta"])["stop_reason"]), obj(e["usage"])
					}
				}
				want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "content_block_start", "content_block_delta", "content_block_stop", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
				if !reflect.DeepEqual(events, want) || stop != "tool_use" || usage["output_tokens"] != float64(12) {
					t.Fatalf("incomplete streamed tool response: %s", w.Body.String())
				}
			} else {
				out := routerInput(t, w.Body.String())
				blocks = arr(out["content"])
				if out["model"] != "claude-sonnet-5" || out["stop_reason"] != "tool_use" || obj(out["usage"])["output_tokens"] != float64(12) {
					t.Fatalf("incorrect response metadata: %#v", out)
				}
			}
			if len(blocks) != 3 || obj(blocks[1])["id"] != "call-a" || obj(obj(blocks[2])["input"])["path"] != "b.txt" {
				t.Fatalf("tool calls lost: %#v", blocks)
			}
			history, _ := json.Marshal(blocks)
			w = request(fmt.Sprintf(`{"model":"claude-sonnet-5","stream":%t,"messages":[{"role":"user","content":"inspect"},{"role":"assistant","content":%s},{"role":"user","content":[{"type":"text","text":"summarize"},{"type":"tool_result","tool_use_id":"call-a","content":"a"},{"type":"tool_result","tool_use_id":"call-b","content":"b"}]}]}`, stream, history))
			messages := arr(requests[1]["messages"])
			if len(messages) != 5 || obj(messages[2])["role"] != "tool" || obj(messages[3])["role"] != "tool" || !strings.Contains(w.Body.String(), "all done") {
				t.Fatalf("tool round trip failed: %#v / %s", requests[1], w.Body.String())
			}
		})
	}
}

func TestClaudeDesktopConversionFailureIsReported(t *testing.T) {
	p := &modelRouter{token: "local", status: RouterStatus{Model: "actual"}}
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"messages":[{"role":"invalid","content":"hi"}]}`))
	r.Header.Set("Authorization", "Bearer local")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	if w.Code != 400 || p.status.Requests != 1 || p.status.Failures != 1 || p.status.LastError == "" {
		t.Fatalf("conversion failure not surfaced to mapping page: %d %#v", w.Code, p.status)
	}
	if routerInput(t, w.Body.String())["type"] != "error" {
		t.Fatalf("expected Anthropic error envelope: %s", w.Body.String())
	}
}

func TestAnthropicResponseMetadataAndInvalidUpstream(t *testing.T) {
	for _, tc := range []struct {
		name, response, stop string
		invalid              bool
	}{
		{"token limit", `{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`, "max_tokens", false},
		{"text only", `{"choices":[{"finish_reason":"stop","message":{"content":"ok"}}]}`, "end_turn", false},
		{"tool only", `{"choices":[{"finish_reason":"tool_calls","message":{"content":null,"tool_calls":[{"id":"a","function":{"name":"read","arguments":"{}"}}]}}]}`, "tool_use", false},
		{"no-argument tool", `{"choices":[{"finish_reason":"tool_calls","message":{"content":null,"tool_calls":[{"id":"a","function":{"name":"read","arguments":""}}]}}]}`, "tool_use", false},
		{"no choices", `{}`, "", true},
		{"empty choices", `{"choices":[]}`, "", true},
		{"no message", `{"choices":[{}]}`, "", true},
		{"invalid arguments", `{"choices":[{"message":{"tool_calls":[{"id":"a","function":{"name":"read","arguments":"broken"}}]}}]}`, "", true},
		{"null arguments", `{"choices":[{"message":{"tool_calls":[{"id":"a","function":{"name":"read","arguments":"null"}}]}}]}`, "", true},
		{"missing tool ID", `{"choices":[{"message":{"tool_calls":[{"function":{"name":"read","arguments":"{}"}}]}}]}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := chatToAnthropic(routerInput(t, tc.response), "claude-sonnet-5")
			if tc.invalid {
				if err == nil {
					t.Fatalf("invalid upstream accepted: %#v", out)
				}
				return
			}
			if err != nil || out["stop_reason"] != tc.stop || !strings.HasPrefix(str(out["id"]), "msg_") || obj(out["usage"])["input_tokens"] != 0 {
				t.Fatalf("metadata/usage fallback: %#v %v", out, err)
			}
			w := httptest.NewRecorder()
			writeAnthropicResponse(w, out, true)
			if !strings.Contains(w.Body.String(), `"stop_reason":"`+tc.stop+`"`) || !strings.Contains(w.Body.String(), "event: message_stop") {
				t.Fatalf("incorrect streaming finish: %s", w.Body.String())
			}
			if tc.stop == "tool_use" && (strings.Contains(w.Body.String(), `"type":"text"`) || !strings.Contains(w.Body.String(), `"type":"tool_use"`)) {
				t.Fatalf("tool-only response needs a tool block and no empty text block: %s", w.Body.String())
			}
		})
	}
}
