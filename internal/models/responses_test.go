package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponsesRequestMapsOntoChat(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}
	body := map[string]any{
		"model":             "glm-5.3-flash",
		"instructions":      "be brief",
		"max_output_tokens": float64(256),
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "cuaca Jakarta?"},
			}},
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": `{"city":"Jakarta"}`},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "sunny"},
		},
		"tools": []any{
			map[string]any{"type": "function", "name": "get_weather", "parameters": schema},
			map[string]any{"type": "custom", "name": "apply_patch", "description": "patch files"},
			map[string]any{"type": "web_search"},
		},
		"tool_choice": "auto",
	}

	chat, meta := ResponsesToChat(body)
	messages := chat["messages"].([]any)
	if len(messages) != 4 {
		t.Fatalf("messages = %d: %#v", len(messages), messages)
	}
	if first := messages[0].(map[string]any); first["role"] != "system" || first["content"] != "be brief" {
		t.Fatalf("instructions not hoisted: %#v", first)
	}
	user := messages[1].(map[string]any)
	parts, _ := user["content"].([]any)
	if user["role"] != "user" || len(parts) != 1 || parts[0].(map[string]any)["text"] != "cuaca Jakarta?" {
		t.Fatalf("user content = %#v", user)
	}
	assistant := messages[2].(map[string]any)
	calls, _ := assistant["tool_calls"].([]any)
	if len(calls) != 1 {
		t.Fatalf("tool_calls = %#v", assistant)
	}
	call := calls[0].(map[string]any)
	if call["id"] != "call_1" || call["function"].(map[string]any)["name"] != "get_weather" {
		t.Fatalf("tool call = %#v", call)
	}
	tool := messages[3].(map[string]any)
	if tool["role"] != "tool" || tool["tool_call_id"] != "call_1" || tool["content"] != "sunny" {
		t.Fatalf("tool reply = %#v", tool)
	}
	if chat["max_tokens"] != float64(256) || chat["tool_choice"] != "auto" {
		t.Fatalf("chat knobs = %#v", chat)
	}

	tools := chat["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools = %d (hosted tools must be dropped): %#v", len(tools), tools)
	}
	custom := tools[1].(map[string]any)["function"].(map[string]any)
	if custom["name"] != "apply_patch" {
		t.Fatalf("custom tool name = %v", custom["name"])
	}
	props := custom["parameters"].(map[string]any)["properties"].(map[string]any)
	if _, ok := props["input"]; !ok {
		t.Fatalf("custom tool schema must take an input string: %#v", custom)
	}
	if !meta.CustomTools["apply_patch"] {
		t.Fatalf("custom tool not recorded: %#v", meta.CustomTools)
	}

	translated := OpenAIToAnthropic(chat)
	if translated["system"] != "be brief" {
		t.Fatalf("system = %v", translated["system"])
	}
	wire := translated["messages"].([]map[string]any)
	last := wire[len(wire)-1]
	blocks, _ := last["content"].([]any)
	if len(blocks) != 1 || blocks[0].(map[string]any)["type"] != "tool_result" {
		t.Fatalf("tool_result block missing: %#v", last)
	}
	if blocks[0].(map[string]any)["tool_use_id"] != "call_1" {
		t.Fatalf("tool_use_id = %#v", blocks[0])
	}
}

func TestResponsesNamespaceToolsFlatten(t *testing.T) {
	body := map[string]any{
		"model": "glm-5.3-flash",
		"input": "hi",
		"tools": []any{map[string]any{
			"type": "namespace", "name": "fs",
			"tools": []any{map[string]any{"type": "function", "name": "read", "parameters": map[string]any{"type": "object"}}},
		}},
	}
	chat, meta := ResponsesToChat(body)
	tools := chat["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %#v", tools)
	}
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "fs__read" {
		t.Fatalf("flattened name = %v", fn["name"])
	}
	ns, ok := meta.Namespaces["fs__read"]
	if !ok || ns.Namespace != "fs" || ns.Name != "read" {
		t.Fatalf("namespace meta = %#v", meta.Namespaces)
	}
}

func TestAnthropicToResponsesNonStream(t *testing.T) {
	meta := newResponsesMeta()
	meta.CustomTools["apply_patch"] = true
	result := map[string]any{
		"id": "msg_abc", "model": "glm-5.3-flash", "stop_reason": "tool_use",
		"content": []any{
			map[string]any{"type": "thinking", "thinking": "plan"},
			map[string]any{"type": "text", "text": "calling tools"},
			map[string]any{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": map[string]any{"city": "Jakarta"}},
			map[string]any{"type": "tool_use", "id": "toolu_2", "name": "apply_patch", "input": map[string]any{"input": "*** patch"}},
		},
		"usage": map[string]any{"input_tokens": float64(10), "output_tokens": float64(4), "cache_read_input_tokens": float64(2)},
	}
	out := AnthropicToResponses(result, "glm-5.3-flash", meta)
	if out["object"] != "response" || out["status"] != "completed" {
		t.Fatalf("response envelope = %#v", out)
	}
	if out["output_text"] != "calling tools" {
		t.Fatalf("output_text = %v", out["output_text"])
	}
	items := out["output"].([]any)
	if len(items) != 4 {
		t.Fatalf("output items = %d: %#v", len(items), items)
	}
	if items[0].(map[string]any)["type"] != "reasoning" {
		t.Fatalf("reasoning item = %#v", items[0])
	}
	msg := items[1].(map[string]any)
	if msg["type"] != "message" || !strings.Contains(mustJSON(msg), "calling tools") {
		t.Fatalf("message item = %#v", msg)
	}
	fn := items[2].(map[string]any)
	if fn["type"] != "function_call" || fn["call_id"] != "toolu_1" || fn["arguments"] != `{"city":"Jakarta"}` {
		t.Fatalf("function_call item = %#v", fn)
	}
	custom := items[3].(map[string]any)
	if custom["type"] != "custom_tool_call" || custom["input"] != "*** patch" {
		t.Fatalf("custom_tool_call item = %#v", custom)
	}
	usage := out["usage"].(map[string]any)
	if usage["total_tokens"] != int64(14) || usage["input_tokens_details"].(map[string]any)["cached_tokens"] != int64(2) {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestResponsesStreamTextLifecycle(t *testing.T) {
	stream := NewResponsesStream("glm-5.3-flash", newResponsesMeta())
	events := []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": "msg_stub", "model": "glm-5.3-flash", "usage": map[string]any{"input_tokens": float64(0), "output_tokens": float64(0)}}},
		{"type": "content_block_start", "index": float64(0), "content_block": map[string]any{"type": "text", "text": ""}},
		{"type": "content_block_delta", "index": float64(0), "delta": map[string]any{"type": "text_delta", "text": "STUB"}},
		{"type": "content_block_delta", "index": float64(0), "delta": map[string]any{"type": "text_delta", "text": "-OK"}},
		{"type": "content_block_stop", "index": float64(0)},
		{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}, "usage": map[string]any{"input_tokens": float64(10), "output_tokens": float64(5)}},
		{"type": "message_stop"},
	}
	var emitted []map[string]any
	for _, event := range events {
		emitted = append(emitted, stream.Handle(event)...)
	}
	types := eventTypes(emitted)
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("event order:\n got %v\nwant %v", types, want)
	}
	for i, event := range emitted {
		if int(event["sequence_number"].(int)) != i {
			t.Fatalf("sequence_number at %d = %v", i, event["sequence_number"])
		}
	}
	if emitted[3]["delta"] != "STUB" || emitted[5]["text"] != "STUB-OK" {
		t.Fatalf("deltas/done = %#v %#v", emitted[3], emitted[5])
	}
	completed := emitted[len(emitted)-1]["response"].(map[string]any)
	if completed["status"] != "completed" {
		t.Fatalf("status = %v", completed["status"])
	}
	output := completed["output"].([]any)
	item := output[0].(map[string]any)
	content := item["content"].([]any)
	if content[0].(map[string]any)["text"] != "STUB-OK" {
		t.Fatalf("final output text = %#v", content)
	}
	usage := completed["usage"].(map[string]any)
	if usage["input_tokens"] != int64(10) || usage["output_tokens"] != int64(5) || usage["total_tokens"] != int64(15) {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestResponsesStreamToolLifecycle(t *testing.T) {
	meta := newResponsesMeta()
	meta.CustomTools["apply_patch"] = true
	stream := NewResponsesStream("glm-5.3-flash", meta)
	handle := func(events ...map[string]any) []map[string]any {
		var out []map[string]any
		for _, event := range events {
			out = append(out, stream.Handle(event)...)
		}
		return out
	}
	// Two tool calls: a plain function and a custom tool.
	emitted := handle(
		map[string]any{"type": "message_start", "message": map[string]any{"model": "glm-5.3-flash"}},
		map[string]any{"type": "content_block_start", "index": float64(0), "content_block": map[string]any{"type": "tool_use", "id": "toolu_1", "name": "get_weather"}},
		map[string]any{"type": "content_block_delta", "index": float64(0), "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"city":`}},
		map[string]any{"type": "content_block_delta", "index": float64(0), "delta": map[string]any{"type": "input_json_delta", "partial_json": `"Jakarta"}`}},
		map[string]any{"type": "content_block_stop", "index": float64(0)},
		map[string]any{"type": "content_block_start", "index": float64(1), "content_block": map[string]any{"type": "tool_use", "id": "toolu_2", "name": "apply_patch"}},
		map[string]any{"type": "content_block_delta", "index": float64(1), "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"input":"*** patch"}`}},
		map[string]any{"type": "content_block_stop", "index": float64(1)},
		map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "tool_use"}},
		map[string]any{"type": "message_stop"},
	)
	types := eventTypes(emitted)
	if types[0] != "response.created" || types[len(types)-1] != "response.completed" {
		t.Fatalf("bounds = %v", types)
	}
	if countType(types, "response.function_call_arguments.delta") != 2 {
		t.Fatalf("arguments deltas = %v", types)
	}
	if countType(types, "response.function_call_arguments.done") != 1 {
		t.Fatalf("arguments done = %v", types)
	}
	if countType(types, "response.custom_tool_call_input.done") != 1 {
		t.Fatalf("custom done = %v", types)
	}
	if countType(types, "response.output_item.done") != 2 {
		t.Fatalf("output items done = %v", types)
	}
	for _, event := range emitted {
		if event["type"] == "response.function_call_arguments.done" {
			if event["arguments"] != `{"city":"Jakarta"}` {
				t.Fatalf("function arguments = %v", event["arguments"])
			}
		}
		if event["type"] == "response.custom_tool_call_input.done" {
			if event["input"] != "*** patch" {
				t.Fatalf("custom input = %v", event["input"])
			}
		}
	}
	completed := emitted[len(emitted)-1]["response"].(map[string]any)
	output := completed["output"].([]any)
	if len(output) != 2 {
		t.Fatalf("final output = %#v", output)
	}
	if output[0].(map[string]any)["type"] != "function_call" || output[1].(map[string]any)["type"] != "custom_tool_call" {
		t.Fatalf("final item types = %#v", output)
	}
}

func TestResponsesStreamFinalizeIsIdempotent(t *testing.T) {
	stream := NewResponsesStream("glm-5.3-flash", newResponsesMeta())
	first := stream.Handle(map[string]any{"type": "message_stop"})
	second := stream.Handle(map[string]any{"type": "message_stop"})
	if len(first) == 0 || len(second) != 0 {
		t.Fatalf("finalize emitted %d then %d events", len(first), len(second))
	}
	if eventTypes(first)[0] != "response.created" {
		t.Fatalf("empty stream must still create a response: %v", eventTypes(first))
	}
	completed := first[len(first)-1]
	if completed["type"] != "response.completed" {
		t.Fatalf("last event = %v", completed["type"])
	}
}

func TestResponsesStreamErrorEvent(t *testing.T) {
	stream := NewResponsesStream("glm-5.3-flash", newResponsesMeta())
	events := stream.Handle(map[string]any{"type": "error", "error": map[string]any{"type": "overloaded_error", "message": "busy"}})
	types := eventTypes(events)
	if types[len(types)-1] != "response.failed" {
		t.Fatalf("types = %v", types)
	}
	response := events[len(events)-1]["response"].(map[string]any)
	errObj := response["error"].(map[string]any)
	if errObj["message"] != "busy" {
		t.Fatalf("error = %#v", errObj)
	}
}

func eventTypes(events []map[string]any) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event["type"].(string))
	}
	return out
}

func countType(types []string, want string) int {
	n := 0
	for _, typ := range types {
		if typ == want {
			n++
		}
	}
	return n
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
