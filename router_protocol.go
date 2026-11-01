package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type routeTool struct {
	Name, Namespace string
	Custom          bool
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func arr(v any) []any          { a, _ := v.([]any); return a }
func toolName(name, namespace string) string {
	if namespace != "" {
		return namespace + "__" + name
	}
	return name
}

func responsesToChat(in map[string]any, model string) (map[string]any, map[string]routeTool, error) {
	specs := map[string]routeTool{}
	if str(in["previous_response_id"]) != "" {
		return nil, nil, errors.New("本地路由需要完整对话历史，请新建对话；暂不支持 previous_response_id")
	}
	messages := []any{}
	if instructions := str(in["instructions"]); instructions != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}
	tools := []any{}
	definitions := map[string]string{}
	var addTool func(map[string]any, string) error
	addTool = func(t map[string]any, namespace string) error {
		kind := str(t["type"])
		// Gemini-compatible gateways cannot execute Codex's server-side search
		// tools. Codex may still include these declarations even when
		// web_search is disabled, so omit them instead of rejecting the whole
		// request. Regular function/custom tools remain supported.
		if kind == "web_search" || kind == "tool_search" || kind == "namespace_search" {
			return nil
		}
		if kind == "namespace" {
			for _, child := range arr(t["tools"]) {
				if err := addTool(obj(child), str(t["name"])); err != nil {
					return err
				}
			}
			return nil
		}
		if kind != "function" && kind != "custom" {
			return fmt.Errorf("此模型路由暂不支持工具类型 %q，请关闭对应扩展后重试", kind)
		}
		name := toolName(str(t["name"]), namespace)
		if name == "" || len(name) > 64 {
			return errors.New("工具名称为空或超过上游 64 字符限制")
		}
		definition, _ := json.Marshal(t)
		if previous, exists := definitions[name]; exists {
			if previous == string(definition) {
				return nil
			}
			return errors.New("工具名称冲突")
		}
		definitions[name] = string(definition)
		specs[name] = routeTool{Name: str(t["name"]), Namespace: namespace, Custom: kind == "custom"}
		fn := map[string]any{"name": name, "description": t["description"], "parameters": t["parameters"]}
		if fn["parameters"] == nil {
			fn["parameters"] = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		if kind == "custom" {
			definition, _ := json.Marshal(t)
			fn["description"] = str(t["description"]) + "\nReturn the exact raw tool input in the input string. Original tool definition: " + string(definition)
			fn["parameters"] = map[string]any{"type": "object", "properties": map[string]any{"input": map[string]any{"type": "string"}}, "required": []string{"input"}}
		}
		tools = append(tools, map[string]any{"type": "function", "function": fn})
		return nil
	}
	for _, t := range arr(in["tools"]) {
		if err := addTool(obj(t), ""); err != nil {
			return nil, nil, err
		}
	}
	input := arr(in["input"])
	if text, ok := in["input"].(string); ok {
		input = []any{map[string]any{"role": "user", "content": text}}
	}
	// Newer Codex clients carry tool definitions alongside message history.
	for _, raw := range input {
		item := obj(raw)
		if str(item["type"]) == "additional_tools" {
			for _, tool := range arr(item["tools"]) {
				if err := addTool(obj(tool), ""); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	reasoning := ""
	for _, raw := range input {
		item := obj(raw)
		kind := str(item["type"])
		switch kind {
		case "additional_tools":
			continue
		case "reasoning":
			for _, part := range arr(item["summary"]) {
				reasoning += str(obj(part)["text"])
			}
		case "function_call", "custom_tool_call":
			name := toolName(str(item["name"]), str(item["namespace"]))
			args := str(item["arguments"])
			if kind == "custom_tool_call" {
				encoded, _ := json.Marshal(map[string]any{"input": str(item["input"])})
				args = string(encoded)
			}
			call := map[string]any{"id": item["call_id"], "type": "function", "function": map[string]any{"name": name, "arguments": args}}
			var last map[string]any
			if len(messages) > 0 {
				last = obj(messages[len(messages)-1])
			}
			if str(last["role"]) == "assistant" {
				last["tool_calls"] = append(arr(last["tool_calls"]), call)
			} else {
				last = map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{call}}
				messages = append(messages, last)
			}
			if reasoning != "" {
				last["reasoning_content"] = reasoning
				reasoning = ""
			}
		case "function_call_output", "custom_tool_call_output":
			output := item["output"]
			if _, ok := output.(string); !ok {
				b, _ := json.Marshal(output)
				output = string(b)
			}
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": item["call_id"], "content": output})
		case "message", "":
			role := str(item["role"])
			if role == "developer" {
				role = "system"
			}
			if role != "user" && role != "assistant" && role != "system" {
				return nil, nil, errors.New("无法转换消息角色")
			}
			content, err := routeContent(item["content"])
			if err != nil {
				return nil, nil, err
			}
			message := map[string]any{"role": role, "content": content}
			if role == "assistant" && reasoning != "" {
				message["reasoning_content"] = reasoning
				reasoning = ""
			}
			messages = append(messages, message)
		default:
			return nil, nil, fmt.Errorf("本地路由暂不支持输入类型 %q，请新建对话", kind)
		}
	}
	if len(messages) == 0 {
		return nil, nil, errors.New("对话内容为空")
	}
	request := map[string]any{"model": model, "messages": messages, "stream": in["stream"] == true}
	if in["stream"] == true {
		request["stream_options"] = map[string]any{"include_usage": true}
	}
	if len(tools) > 0 {
		request["tools"] = tools
	}
	for _, k := range []string{"temperature", "top_p", "parallel_tool_calls"} {
		if v, ok := in[k]; ok {
			request[k] = v
		}
	}
	if max, ok := in["max_output_tokens"]; ok {
		request["max_tokens"] = max
	}
	if choice, ok := in["tool_choice"].(string); ok {
		request["tool_choice"] = choice
	} else if choice := obj(in["tool_choice"]); choice != nil {
		if str(choice["type"]) != "function" && str(choice["type"]) != "custom" {
			return nil, nil, errors.New("暂不支持此工具选择方式")
		}
		request["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": toolName(str(choice["name"]), str(choice["namespace"]))}}
	}
	return request, specs, nil
}
func routeContent(v any) (any, error) {
	if text, ok := v.(string); ok {
		return text, nil
	}
	parts := []any{}
	for _, raw := range arr(v) {
		p := obj(raw)
		switch str(p["type"]) {
		case "input_text", "output_text", "text":
			parts = append(parts, map[string]any{"type": "text", "text": p["text"]})
		case "input_image":
			parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": p["image_url"]}})
		default:
			return nil, errors.New("路由目前支持文本和图片输入；不支持此附件类型")
		}
	}
	return parts, nil
}

// anthropicToChat converts Claude Messages content blocks to the OpenAI Chat
// shape accepted by the CiyuanShen gateway. Claude Desktop sends arrays even
// for ordinary text and includes tool_use/tool_result blocks during agent
// turns; forwarding those blocks unchanged causes upstream 4xx/5xx errors.
func anthropicToChat(in map[string]any, model string) (map[string]any, error) {
	messages := []any{}
	if system := in["system"]; system != nil {
		content, err := anthropicContent(system)
		if err != nil {
			return nil, err
		}
		if content != nil && content != "" {
			messages = append(messages, map[string]any{"role": "system", "content": content})
		}
	}
	for index, raw := range arr(in["messages"]) {
		message := obj(raw)
		role := str(message["role"])
		// Desktop gateway histories may embed system/developer instructions
		// in messages instead of using only Anthropic's top-level system field.
		// Normalize these to the gateway's portable Chat system role, keeping
		// their position and content rather than rejecting the entire turn.
		if role == "system" || role == "developer" {
			content, err := anthropicContent(message["content"])
			if err != nil {
				return nil, err
			}
			messages = append(messages, map[string]any{"role": "system", "content": content})
			continue
		}
		if role != "user" && role != "assistant" {
			return nil, fmt.Errorf("Anthropic messages[%d] 消息角色 %q 无效；支持 user、assistant、system、developer", index, role)
		}
		blocks := arr(message["content"])
		if blocks == nil {
			content, err := anthropicContent(message["content"])
			if err != nil {
				return nil, err
			}
			if role == "user" {
				messages = append(messages, map[string]any{"role": role, "content": content})
			} else {
				messages = appendAnthropicAssistantMessage(messages, content, nil)
			}
			continue
		}
		contentParts := []any{}
		toolCalls := []any{}
		toolResults := []any{}
		for _, rawBlock := range blocks {
			block := obj(rawBlock)
			switch str(block["type"]) {
			case "text", "image":
				converted, err := anthropicContent([]any{block})
				if err != nil {
					return nil, err
				}
				contentParts = append(contentParts, arr(converted)...)
			case "thinking", "redacted_thinking":
				// Reasoning is not portable across Chat Completions providers.
				// Keep visible thinking text when present, omit redacted blocks.
				if text := str(block["thinking"]); text != "" {
					contentParts = append(contentParts, map[string]any{"type": "text", "text": text})
				}
			case "tool_use":
				if role != "assistant" {
					return nil, errors.New("tool_use 只能出现在 assistant 消息中")
				}
				arguments, _ := json.Marshal(block["input"])
				toolCalls = append(toolCalls, map[string]any{
					"id":   str(block["id"]),
					"type": "function",
					"function": map[string]any{
						"name":      str(block["name"]),
						"arguments": string(arguments),
					},
				})
			case "tool_result":
				if role != "user" {
					return nil, errors.New("tool_result 只能出现在 user 消息中")
				}
				toolContent, err := anthropicContent(block["content"])
				if err != nil {
					return nil, err
				}
				toolResults = append(toolResults, map[string]any{"role": "tool", "tool_call_id": str(block["tool_use_id"]), "content": toolContent})
			default:
				return nil, fmt.Errorf("暂不支持 Anthropic 内容类型 %q", str(block["type"]))
			}
		}
		if role == "assistant" {
			if len(contentParts) > 0 || len(toolCalls) > 0 {
				messages = appendAnthropicAssistantMessage(messages, contentParts, toolCalls)
			}
		} else {
			// Chat requires tool replies immediately after assistant tool_calls,
			// before ordinary text from the same Anthropic user turn.
			messages = append(messages, toolResults...)
			if len(contentParts) > 0 {
				messages = append(messages, map[string]any{"role": role, "content": contentParts})
			}
		}
	}
	if len(messages) == 0 {
		return nil, errors.New("对话内容为空")
	}
	request := map[string]any{"model": model, "messages": messages, "stream": false}
	for _, key := range []string{"max_tokens", "temperature", "top_p"} {
		if value, ok := in[key]; ok {
			request[key] = value
		}
	}
	if stop, ok := in["stop_sequences"]; ok {
		request["stop"] = stop
	}
	tools := []any{}
	for _, raw := range arr(in["tools"]) {
		tool := obj(raw)
		name := str(tool["name"])
		if name == "" {
			return nil, errors.New("Anthropic 工具缺少名称")
		}
		parameters := tool["input_schema"]
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": name, "description": str(tool["description"]), "parameters": parameters}})
	}
	if len(tools) > 0 {
		request["tools"] = tools
	}
	if choice := obj(in["tool_choice"]); choice != nil {
		switch str(choice["type"]) {
		case "auto":
			request["tool_choice"] = "auto"
		case "any":
			request["tool_choice"] = "required"
		case "tool":
			request["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": str(choice["name"])}}
		}
	}
	return request, nil
}

func appendAnthropicAssistantMessage(messages []any, content any, calls []any) []any {
	if parts, ok := content.([]any); ok && len(parts) == 0 {
		// A tool-only Chat assistant message has null content, not an empty
		// multimodal array (which strict compatible providers reject).
		content = nil
	}
	assistant := map[string]any{"role": "assistant", "content": content}
	if len(calls) > 0 {
		assistant["tool_calls"] = calls
	}
	if len(messages) > 0 {
		previous := obj(messages[len(messages)-1])
		if previous["role"] == "assistant" {
			previousContent := previous["content"]
			if previousContent == nil || previousContent == "" {
				previous["content"] = content
			} else if content != nil && content != "" {
				// Histories can mix string content with content-block arrays.
				// Convert both sides before merging so neither turn is dropped.
				parts := func(v any) []any {
					if text, ok := v.(string); ok {
						return []any{map[string]any{"type": "text", "text": text}}
					}
					return arr(v)
				}
				previous["content"] = append(parts(previousContent), parts(content)...)
			}
			if len(calls) > 0 {
				previous["tool_calls"] = append(arr(previous["tool_calls"]), calls...)
			}
			return messages
		}
	}
	return append(messages, assistant)
}

func anthropicContent(value any) (any, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	blocks := arr(value)
	if blocks == nil {
		return "", nil
	}
	parts := []any{}
	for _, raw := range blocks {
		block := obj(raw)
		switch str(block["type"]) {
		case "text":
			parts = append(parts, map[string]any{"type": "text", "text": str(block["text"])})
		case "image":
			source := obj(block["source"])
			if source["type"] == "base64" {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + str(source["media_type"]) + ";base64," + str(source["data"])}})
			} else if source["type"] == "url" {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": str(source["url"])}})
			} else {
				return nil, errors.New("不支持的 Anthropic 图片来源")
			}
		default:
			return nil, fmt.Errorf("暂不支持 Anthropic 内容类型 %q", str(block["type"]))
		}
	}
	return parts, nil
}

type routeCall struct{ ID, Name, Arguments string }
type routeOutput struct {
	w               http.ResponseWriter
	stream          bool
	sequence        int
	response        map[string]any
	text, reasoning string
	textStarted     bool
	calls           map[int]*routeCall
	specs           map[string]routeTool
	finish          string
	usage           map[string]any
}

func (o *routeOutput) event(kind string, fields map[string]any) {
	if !o.stream {
		return
	}
	fields["type"] = kind
	fields["sequence_number"] = o.sequence
	o.sequence++
	data, _ := json.Marshal(fields)
	fmt.Fprintf(o.w, "event: %s\ndata: %s\n\n", kind, data)
	if f, ok := o.w.(http.Flusher); ok {
		f.Flush()
	}
}
func (o *routeOutput) textItem(status string) map[string]any {
	return map[string]any{"id": "msg_" + str(o.response["id"]), "type": "message", "role": "assistant", "status": status, "content": []any{map[string]any{"type": "output_text", "text": o.text, "annotations": []any{}}}}
}
func (o *routeOutput) chunk(chunk map[string]any) error {
	if chunk["error"] != nil {
		return errors.New("上游流返回错误")
	}
	if u := obj(chunk["usage"]); u != nil {
		o.usage = u
	}
	choices := arr(chunk["choices"])
	if len(choices) == 0 {
		return nil
	}
	choice := obj(choices[0])
	if reason := str(choice["finish_reason"]); reason != "" {
		o.finish = reason
	}
	delta := obj(choice["delta"])
	if delta == nil {
		delta = obj(choice["message"])
	}
	text := str(delta["content"])
	if text != "" {
		if !o.textStarted {
			o.textStarted = true
			item := o.textItem("in_progress")
			item["content"] = []any{}
			o.event("response.output_item.added", map[string]any{"output_index": 0, "item": item})
			o.event("response.content_part.added", map[string]any{"output_index": 0, "content_index": 0, "item_id": item["id"], "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
		}
		o.text += text
		o.event("response.output_text.delta", map[string]any{"output_index": 0, "content_index": 0, "item_id": o.textItem("")["id"], "delta": text})
	}
	o.reasoning += str(delta["reasoning_content"])
	for n, raw := range arr(delta["tool_calls"]) {
		call := obj(raw)
		index := n
		if v, ok := call["index"].(float64); ok {
			index = int(v)
		}
		if index < 0 || index > 127 {
			return errors.New("上游工具调用数量超出限制")
		}
		c := o.calls[index]
		if c == nil {
			c = &routeCall{}
			o.calls[index] = c
		}
		if id := str(call["id"]); id != "" {
			c.ID = id
		}
		fn := obj(call["function"])
		c.Name += str(fn["name"])
		c.Arguments += str(fn["arguments"])
	}
	return nil
}
func (o *routeOutput) complete() error {
	if o.finish == "" {
		return errors.New("上游流意外中断，未收到结束标记")
	}
	if o.finish != "stop" && o.finish != "tool_calls" && o.finish != "length" {
		return fmt.Errorf("上游未正常完成：%s", o.finish)
	}
	output := []any{}
	if o.textStarted {
		item := o.textItem("completed")
		o.event("response.output_text.done", map[string]any{"output_index": 0, "content_index": 0, "item_id": item["id"], "text": o.text})
		o.event("response.content_part.done", map[string]any{"output_index": 0, "content_index": 0, "item_id": item["id"], "part": arr(item["content"])[0]})
		o.event("response.output_item.done", map[string]any{"output_index": 0, "item": item})
		output = append(output, item)
	}
	add := func(item map[string]any) {
		i := len(output)
		o.event("response.output_item.added", map[string]any{"output_index": i, "item": item})
		o.event("response.output_item.done", map[string]any{"output_index": i, "item": item})
		output = append(output, item)
	}
	if o.reasoning != "" {
		add(map[string]any{"id": "rs_" + str(o.response["id"]), "type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": o.reasoning}}})
	}
	indices := []int{}
	for i := range o.calls {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	toolItems := []map[string]any{}
	for _, i := range indices {
		c := o.calls[i]
		spec, ok := o.specs[c.Name]
		if !ok || c.ID == "" {
			return errors.New("上游返回了未知工具或缺少调用编号")
		}
		item := map[string]any{"type": "function_call", "id": "fc_" + c.ID, "call_id": c.ID, "name": spec.Name, "arguments": c.Arguments, "status": "completed"}
		if spec.Namespace != "" {
			item["namespace"] = spec.Namespace
		}
		if spec.Custom {
			var args map[string]any
			if err := json.Unmarshal([]byte(c.Arguments), &args); err != nil {
				return errors.New("上游自定义工具参数不是有效 JSON")
			}
			input, ok := args["input"].(string)
			if !ok {
				return errors.New("上游自定义工具参数缺少 input")
			}
			delete(item, "arguments")
			item["type"] = "custom_tool_call"
			item["input"] = input
		} else if !json.Valid([]byte(c.Arguments)) {
			return errors.New("上游函数参数不是有效 JSON")
		}
		toolItems = append(toolItems, item)
	}
	// Validate the entire tool batch before exposing any executable tool call.
	for _, item := range toolItems {
		field, event := "arguments", "response.function_call_arguments"
		if item["type"] == "custom_tool_call" {
			field, event = "input", "response.custom_tool_call_input"
		}
		index := len(output)
		start := map[string]any{}
		for k, v := range item {
			start[k] = v
		}
		start[field] = ""
		start["status"] = "in_progress"
		o.event("response.output_item.added", map[string]any{"output_index": index, "item": start})
		o.event(event+".delta", map[string]any{"output_index": index, "item_id": item["id"], "delta": item[field]})
		o.event(event+".done", map[string]any{"output_index": index, "item_id": item["id"], field: item[field]})
		o.event("response.output_item.done", map[string]any{"output_index": index, "item": item})
		output = append(output, item)
	}
	if len(output) == 0 {
		return errors.New("上游返回空回复")
	}
	o.response["output"] = output
	o.response["status"] = "completed"
	prompt, _ := o.usage["prompt_tokens"].(float64)
	completion, _ := o.usage["completion_tokens"].(float64)
	cached, _ := obj(o.usage["prompt_tokens_details"])["cached_tokens"].(float64)
	o.response["usage"] = map[string]any{"input_tokens": prompt, "output_tokens": completion, "total_tokens": prompt + completion, "input_tokens_details": map[string]any{"cached_tokens": cached}, "output_tokens_details": map[string]any{"reasoning_tokens": 0}}
	event := "response.completed"
	if o.finish == "length" {
		o.response["status"] = "incomplete"
		o.response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
		event = "response.incomplete"
	}
	o.event(event, map[string]any{"response": o.response})
	if !o.stream {
		writeRouteJSON(o.w, o.response)
	}
	return nil
}
func chatToResponses(w http.ResponseWriter, body io.Reader, stream bool, model string, specs map[string]routeTool) (err error) {
	id, _ := createProvisionID()
	o := &routeOutput{w: w, stream: stream, specs: specs, calls: map[int]*routeCall{}, response: map[string]any{"id": "resp_" + id, "object": "response", "model": model, "created_at": time.Now().Unix(), "status": "in_progress", "output": []any{}, "error": nil}}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		o.event("response.created", map[string]any{"response": o.response})
		o.event("response.in_progress", map[string]any{"response": o.response})
	}
	defer func() {
		if err != nil {
			if stream {
				o.response["status"] = "failed"
				o.response["error"] = map[string]any{"code": "upstream_error", "message": err.Error()}
				o.event("response.failed", map[string]any{"response": o.response})
			} else {
				routeError(w, 502, err.Error())
			}
		}
	}()
	// Bound cumulative data as well as individual SSE lines.
	limited := &io.LimitedReader{R: body, N: 32 << 20}
	if !stream {
		var chunk map[string]any
		if err = json.NewDecoder(limited).Decode(&chunk); err != nil {
			return errors.New("上游回复不是有效 JSON")
		}
		if err = o.chunk(chunk); err != nil {
			return err
		}
		return o.complete()
	}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	data := []string{}
	consume := func() error {
		if len(data) == 0 {
			return nil
		}
		payload := strings.Join(data, "\n")
		data = nil
		if payload == "[DONE]" {
			return nil
		}
		var chunk map[string]any
		if json.Unmarshal([]byte(payload), &chunk) != nil {
			return errors.New("上游流包含无效 JSON")
		}
		return o.chunk(chunk)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err = consume(); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if scanner.Err() != nil || limited.N <= 0 {
		return errors.New("上游流读取失败或超过大小限制")
	}
	if err = consume(); err != nil {
		return err
	}
	return o.complete()
}
