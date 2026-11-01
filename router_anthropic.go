package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Convert once for both response modes so streaming cannot silently drop tool
// calls or report end_turn when Desktop must execute a tool and continue.
func chatToAnthropic(out map[string]any, model string) (map[string]any, error) {
	choices := arr(out["choices"])
	if len(choices) == 0 || obj(obj(choices[0])["message"]) == nil {
		return nil, errors.New("上游未返回有效的对话结果")
	}
	choice := obj(choices[0])
	message := obj(choice["message"])
	content := []any{}
	if text := str(message["content"]); text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	toolCalls := arr(message["tool_calls"])
	for _, raw := range toolCalls {
		call := obj(raw)
		fn := obj(call["function"])
		var input map[string]any
		arguments := str(fn["arguments"])
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		if str(call["id"]) == "" || str(fn["name"]) == "" || json.Unmarshal([]byte(arguments), &input) != nil || input == nil {
			return nil, errors.New("上游工具调用缺少 ID、名称或有效的 JSON 对象参数")
		}
		content = append(content, map[string]any{"type": "tool_use", "id": str(call["id"]), "name": str(fn["name"]), "input": input})
	}
	stopReason := "end_turn"
	if str(choice["finish_reason"]) == "length" {
		stopReason = "max_tokens"
	} else if len(toolCalls) > 0 {
		stopReason = "tool_use"
	}
	usage := map[string]any{"input_tokens": 0, "output_tokens": 0}
	upstreamUsage := obj(out["usage"])
	if value, ok := upstreamUsage["prompt_tokens"]; ok {
		usage["input_tokens"] = value
	}
	if value, ok := upstreamUsage["completion_tokens"]; ok {
		usage["output_tokens"] = value
	}
	id := str(out["id"])
	if id == "" {
		var err error
		id, err = createProvisionID()
		if err != nil {
			return nil, errors.New("生成消息 ID 失败")
		}
	}
	if !strings.HasPrefix(id, "msg_") {
		id = "msg_" + id
	}
	return map[string]any{"id": id, "type": "message", "role": "assistant", "model": model, "content": content, "stop_reason": stopReason, "stop_sequence": nil, "usage": usage}, nil
}

func writeAnthropicResponse(w http.ResponseWriter, message map[string]any, stream bool) {
	if !stream {
		writeRouteJSON(w, message)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	usage := obj(message["usage"])
	start := map[string]any{"id": message["id"], "type": "message", "role": "assistant", "model": message["model"], "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]any{"input_tokens": usage["input_tokens"], "output_tokens": 0}}
	writeAnthropicEvent(w, "message_start", map[string]any{"type": "message_start", "message": start})
	for index, raw := range arr(message["content"]) {
		block := obj(raw)
		var startBlock, delta map[string]any
		if block["type"] == "tool_use" {
			startBlock = map[string]any{"type": "tool_use", "id": block["id"], "name": block["name"], "input": map[string]any{}}
			arguments, _ := json.Marshal(block["input"])
			delta = map[string]any{"type": "input_json_delta", "partial_json": string(arguments)}
		} else {
			startBlock = map[string]any{"type": "text", "text": ""}
			delta = map[string]any{"type": "text_delta", "text": block["text"]}
		}
		writeAnthropicEvent(w, "content_block_start", map[string]any{"type": "content_block_start", "index": index, "content_block": startBlock})
		writeAnthropicEvent(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": index, "delta": delta})
		writeAnthropicEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
	}
	writeAnthropicEvent(w, "message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": message["stop_reason"], "stop_sequence": message["stop_sequence"]}, "usage": map[string]any{"output_tokens": usage["output_tokens"]}})
	writeAnthropicEvent(w, "message_stop", map[string]any{"type": "message_stop"})
}

func routeAnthropicError(w http.ResponseWriter, status int, message string) {
	errorType := "api_error"
	if status >= 400 && status < 500 {
		errorType = "invalid_request_error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]any{"type": errorType, "message": message}})
}
