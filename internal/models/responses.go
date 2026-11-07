package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// This file translates the OpenAI Responses wire format (POST /v1/responses,
// wire_api = "responses") onto the Anthropic messages layer used upstream.
//
// Request path:  ResponsesToChat -> OpenAIToAnthropic (one translation layer)
// Response path: AnthropicToResponses / ResponsesStream (Anthropic SSE)
//
// Tool bookkeeping travels with the request: Codex declares `custom` tools
// (exec, apply_patch) and namespace tools, which the Anthropic schema cannot
// express, so they are flattened on the way out and restored on the way back.

// Namespace records a flattened namespace tool's original identity.
type Namespace struct {
	Namespace string
	Name      string
}

// ResponsesMeta carries the tool mappings needed to restore item types.
type ResponsesMeta struct {
	CustomTools map[string]bool
	Namespaces  map[string]Namespace
}

func newResponsesMeta() ResponsesMeta {
	return ResponsesMeta{CustomTools: map[string]bool{}, Namespaces: map[string]Namespace{}}
}

// ResponsesToChat converts a Responses request into the chat-completions body
// shape OpenAIToAnthropic already understands.
func ResponsesToChat(body map[string]any) (map[string]any, ResponsesMeta) {
	meta := newResponsesMeta()
	var messages []any

	if instructions, _ := body["instructions"].(string); strings.TrimSpace(instructions) != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}
	messages = append(messages, responsesInputToMessages(body["input"])...)

	chat := map[string]any{
		"model":    stringField(body, "model"),
		"messages": messages,
	}
	if v, ok := numberField(body, "max_output_tokens"); ok && v > 0 {
		chat["max_tokens"] = v
	}
	for _, key := range []string{"temperature", "top_p"} {
		if v, ok := numberField(body, key); ok {
			chat[key] = v
		}
	}
	if streamTrue(body) {
		chat["stream"] = true
	}
	if tools := responsesTools(body["tools"], &meta); len(tools) > 0 {
		chat["tools"] = tools
		if choice := responsesToolChoice(body["tool_choice"]); choice != nil {
			chat["tool_choice"] = choice
		}
	}
	return chat, meta
}

// responsesInputToMessages expands `input` (string or item array) into chat
// messages. Function-call history is rebuilt as assistant tool_calls followed
// by tool replies, the pairing chat completions requires.
func responsesInputToMessages(input any) []any {
	if text, ok := input.(string); ok {
		return []any{map[string]any{"role": "user", "content": text}}
	}
	items, ok := input.([]any)
	if !ok {
		return nil
	}
	var out []any
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := item["type"].(string)
		switch typ {
		case "", "message":
			role, _ := item["role"].(string)
			if role == "" {
				role = "user"
			}
			if role == "developer" {
				role = "system"
			}
			out = append(out, map[string]any{"role": role, "content": responsesContent(item["content"])})
		case "function_call", "custom_tool_call":
			name, _ := item["name"].(string)
			callID, _ := item["call_id"].(string)
			if callID == "" {
				callID, _ = item["id"].(string)
			}
			args := stringifyContent(item["arguments"])
			if typ == "custom_tool_call" {
				args = stringifyContent(map[string]any{"input": stringField(item, "input")})
			}
			if args == "" {
				args = "{}"
			}
			out = append(out, map[string]any{
				"role":    "assistant",
				"content": nil,
				"tool_calls": []any{map[string]any{
					"id": callID, "type": "function",
					"function": map[string]any{"name": name, "arguments": args},
				}},
			})
		case "function_call_output", "custom_tool_call_output", "tool_output":
			callID, _ := item["call_id"].(string)
			out = append(out, map[string]any{
				"role": "tool", "tool_call_id": callID,
				"content": stringifyContent(item["output"]),
			})
		case "reasoning":
			// Anthropic thinking blocks cannot be replayed without signatures;
			// the history text is not part of the current turn.
		}
	}
	return out
}

// responsesContent normalizes Responses content (string or input_* parts)
// into OpenAI chat content parts.
func responsesContent(content any) any {
	if s, ok := content.(string); ok {
		return s
	}
	parts, ok := content.([]any)
	if !ok {
		return ""
	}
	var out []any
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch stringField(part, "type") {
		case "input_text", "output_text", "text", "summary_text", "":
			if text, ok := part["text"].(string); ok {
				out = append(out, map[string]any{"type": "text", "text": text})
			}
		case "input_image":
			url := stringField(part, "image_url")
			if url == "" {
				continue
			}
			out = append(out, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
		}
	}
	if len(out) == 0 {
		return ""
	}
	return out
}

// responsesTools converts Responses tool declarations into chat tools.
// function keeps its JSON schema; custom (exec, apply_patch) becomes a function
// taking {"input": string}, the documented downgrade Codex needs; namespace
// tools are flattened to {ns}__{name}; hosted tools have no upstream analogue
// and are dropped.
func responsesTools(raw any, meta *ResponsesMeta) []any {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []any
	for _, item := range list {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch stringField(tool, "type") {
		case "function":
			fn := tool
			if nested, ok := tool["function"].(map[string]any); ok {
				fn = nested
			}
			name := stringField(fn, "name")
			if name == "" {
				continue
			}
			schema := fn["parameters"]
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			out = append(out, chatTool(name, stringField(fn, "description"), schema))
		case "custom":
			name := stringField(tool, "name")
			if name == "" {
				continue
			}
			meta.CustomTools[name] = true
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input": map[string]any{"type": "string", "description": "Raw tool input, verbatim."},
				},
				"required": []any{"input"},
			}
			out = append(out, chatTool(name, stringField(tool, "description"), schema))
		case "namespace":
			ns := stringField(tool, "name")
			inner, _ := tool["tools"].([]any)
			for _, rawInner := range inner {
				innerTool, ok := rawInner.(map[string]any)
				if !ok {
					continue
				}
				child := innerTool
				if nested, ok := innerTool["function"].(map[string]any); ok {
					child = nested
				}
				name := stringField(child, "name")
				if name == "" {
					continue
				}
				flat := ns + "__" + name
				meta.Namespaces[flat] = Namespace{Namespace: ns, Name: name}
				schema := child["parameters"]
				if schema == nil {
					schema = map[string]any{"type": "object", "properties": map[string]any{}}
				}
				out = append(out, chatTool(flat, stringField(child, "description"), schema))
			}
		}
	}
	return out
}

func chatTool(name, description string, schema any) map[string]any {
	fn := map[string]any{"name": name, "parameters": schema}
	if description != "" {
		fn["description"] = description
	}
	return map[string]any{"type": "function", "function": fn}
}

func responsesToolChoice(raw any) any {
	switch choice := raw.(type) {
	case string:
		switch choice {
		case "auto", "none", "required":
			return choice
		}
	case map[string]any:
		if stringField(choice, "type") == "function" {
			if name := stringField(choice, "name"); name != "" {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
		}
	}
	return nil
}

// --- responses ---

// ResponsesUsageReported maps Anthropic usage onto the Responses usage object.
// Both *_tokens_details objects are always emitted: async-openai deserializes
// them as required fields and fails the whole turn when they are missing. A
// reported 0 means "upstream did not break this bucket down".
func ResponsesUsageReported(usage map[string]any) map[string]any {
	if usage == nil {
		return nil
	}
	num := func(key string) int64 {
		v, _ := usage[key].(float64)
		if v != v {
			return 0
		}
		return int64(v)
	}
	input := num("input_tokens")
	output := num("output_tokens")
	return map[string]any{
		"input_tokens":          input,
		"input_tokens_details":  map[string]any{"cached_tokens": num("cache_read_input_tokens")},
		"output_tokens":         output,
		"output_tokens_details": map[string]any{"reasoning_tokens": 0},
		"total_tokens":          input + output,
	}
}

// AnthropicToResponses converts a non-streaming Anthropic message into a
// Responses object.
func AnthropicToResponses(result map[string]any, model string, meta ResponsesMeta) map[string]any {
	var output []any
	var text strings.Builder
	blocks, _ := result["content"].([]any)
	var toolBlocks []map[string]any
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch stringField(block, "type") {
		case "text":
			text.WriteString(stringField(block, "text"))
		case "thinking":
			if thinking := stringField(block, "thinking"); thinking != "" {
				output = append(output, map[string]any{
					"type":    "reasoning",
					"id":      newItemID("rs"),
					"summary": []any{map[string]any{"type": "summary_text", "text": thinking}},
					"status":  "completed",
				})
			}
		case "tool_use":
			toolBlocks = append(toolBlocks, block)
		}
	}
	if text.Len() > 0 || len(toolBlocks) == 0 {
		output = append(output, messageItem(newItemID("msg"), text.String()))
	}
	for _, block := range toolBlocks {
		output = append(output, toolResponseItem(block, meta))
	}

	status := "completed"
	if stringField(result, "stop_reason") == "max_tokens" {
		status = "incomplete"
	}
	id, _ := result["id"].(string)
	if id == "" {
		id = newItemID("resp")
	}
	usage, _ := result["usage"].(map[string]any)

	out := map[string]any{
		"id":          "resp_" + strings.TrimPrefix(id, "msg_"),
		"object":      "response",
		"created_at":  time.Now().Unix(),
		"model":       firstNonEmpty(model, stringField(result, "model"), DefaultModel),
		"status":      status,
		"output":      output,
		"output_text": text.String(),
		"usage":       ResponsesUsageReported(usage),
	}
	if status == "incomplete" {
		out["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	return out
}

func messageItem(id, text string) map[string]any {
	return map[string]any{
		"type": "message", "id": id, "role": "assistant", "status": "completed",
		"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
	}
}

// toolResponseItem restores a tool_use block to the item type Codex expects.
func toolResponseItem(block map[string]any, meta ResponsesMeta) map[string]any {
	name := stringField(block, "name")
	callID := stringField(block, "id")
	args, err := json.Marshal(block["input"])
	if err != nil || len(args) == 0 {
		args = []byte("{}")
	}
	if meta.CustomTools[name] {
		return map[string]any{
			"type": "custom_tool_call", "id": newItemID("ctc"), "call_id": callID,
			"name": name, "input": customToolInput(string(args)), "status": "completed",
		}
	}
	item := map[string]any{
		"type": "function_call", "id": newItemID("fc"), "call_id": callID,
		"name": name, "arguments": string(args), "status": "completed",
	}
	if ns, ok := meta.Namespaces[name]; ok {
		item["name"] = ns.Name
		item["namespace"] = ns.Namespace
	}
	return item
}

// customToolInput unwraps the {"input": "..."} envelope; malformed JSON falls
// back to the raw argument text (some models emit the bare string).
func customToolInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var wrapper struct {
		Input *string `json:"input"`
	}
	if json.Unmarshal([]byte(trimmed), &wrapper) == nil && wrapper.Input != nil {
		return *wrapper.Input
	}
	if s, ok := unquoteJSON(trimmed); ok {
		return s
	}
	return trimmed
}

func unquoteJSON(raw string) (string, bool) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", false
	}
	var s string
	if json.Unmarshal([]byte(raw), &s) != nil {
		return "", false
	}
	return s, true
}

var itemSeq int64

func newItemID(prefix string) string {
	itemSeq++
	return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.Itoa(int(itemSeq%1296))
}
