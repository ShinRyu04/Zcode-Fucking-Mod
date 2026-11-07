package models

import (
	"encoding/json"
	"strings"
	"time"
)

// ResponsesStream converts the upstream Anthropic SSE events into the
// Responses event lifecycle Codex requires. It consumes one event at a time;
// the caller writes whatever it returns and calls Finalize on EOF/stop.
//
// Lifecycle per item:
//
//	message.created (response.created)
//	message: output_item.added -> content_part.added -> output_text.delta*
//	         -> output_text.done -> content_part.done -> output_item.done
//	reasoning: output_item.added -> reasoning_summary_part.added
//	         -> reasoning_summary_text.delta* -> ...done -> ...part.done
//	         -> output_item.done
//	tool: output_item.added -> function_call_arguments.delta*
//	         -> .done -> output_item.done
//	         (custom tools: one custom_tool_call_input.delta + .done)
//	response.completed
//
// Codex wedges a turn when a tool item never receives arguments.done and
// output_item.done, so both are always emitted at close.
type ResponsesStream struct {
	Meta       ResponsesMeta
	Model      string
	responseID string
	createdAt  int64
	seq        int

	created   bool
	completed bool
	nextIndex int
	order     []int
	blocks    map[int]*respBlock
	finished  []any

	inputTokens  int64
	outputTokens int64
	cachedTokens int64
	finishReason string
}

type respBlock struct {
	kind      string // text | thinking | tool
	index     int
	itemID    string
	callID    string
	name      string
	custom    bool
	namespace Namespace
	text      strings.Builder
	args      strings.Builder
	textDone  bool
	partOpen  bool
}

// NewResponsesStream starts a stream translator for one response.
func NewResponsesStream(model string, meta ResponsesMeta) *ResponsesStream {
	return &ResponsesStream{
		Meta:       meta,
		Model:      model,
		responseID: newItemID("resp"),
		createdAt:  nowUnix(),
		blocks:     map[int]*respBlock{},
	}
}

// Handle consumes one Anthropic SSE event and returns the Responses events to
// emit, in order.
func (s *ResponsesStream) Handle(event map[string]any) []map[string]any {
	var out []map[string]any
	out = append(out, s.ensureCreated()...)
	switch stringField(event, "type") {
	case "message_start":
		if msg, ok := event["message"].(map[string]any); ok {
			if model := stringField(msg, "model"); model != "" && s.Model == "" {
				s.Model = model
			}
			if usage, ok := msg["usage"].(map[string]any); ok {
				s.trackUsage(usage)
			}
		}
	case "content_block_start":
		index := intOf(event["index"])
		block, _ := event["content_block"].(map[string]any)
		out = append(out, s.openBlock(index, block)...)
	case "content_block_delta":
		index := intOf(event["index"])
		delta, _ := event["delta"].(map[string]any)
		out = append(out, s.blockDelta(index, delta)...)
	case "content_block_stop":
		out = append(out, s.closeBlock(intOf(event["index"]))...)
	case "message_delta":
		if delta, ok := event["delta"].(map[string]any); ok {
			if reason := stringField(delta, "stop_reason"); reason != "" {
				s.finishReason = reason
			}
		}
		if usage, ok := event["usage"].(map[string]any); ok {
			s.trackUsage(usage)
		}
	case "message_stop":
		out = append(out, s.Finalize()...)
	case "error":
		out = append(out, s.fail(event)...)
	}
	return out
}

// Finalize closes every open item and emits response.completed. Idempotent.
func (s *ResponsesStream) Finalize() []map[string]any {
	if s.completed {
		return nil
	}
	out := s.ensureCreated()
	for _, index := range s.order {
		if block, ok := s.blocks[index]; ok && !block.textDone {
			out = append(out, s.closeBlock(index)...)
		}
	}
	if len(s.finished) == 0 {
		// Codex expects at least one output item even for an empty turn.
		index := s.allocIndex()
		id := newItemID("msg")
		item := messageItem(id, "")
		item["content"] = []any{map[string]any{"type": "output_text", "text": "", "annotations": []any{}}}
		out = append(out,
			s.event("response.output_item.added", map[string]any{
				"output_index": index,
				"item": map[string]any{
					"type": "message", "id": id, "role": "assistant", "status": "in_progress",
					"content": []any{map[string]any{"type": "output_text", "text": "", "annotations": []any{}}},
				},
			}),
			s.event("response.output_item.done", map[string]any{"output_index": index, "item": item}),
		)
		s.finished = append(s.finished, item)
	}
	s.completed = true
	status := "completed"
	response := map[string]any{
		"id":         s.responseID,
		"object":     "response",
		"created_at": s.createdAt,
		"model":      s.modelName(),
		"status":     status,
		"output":     s.finished,
	}
	if s.finishReason == "max_tokens" {
		response["status"] = "incomplete"
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	usage := map[string]any{
		"input_tokens":          s.inputTokens,
		"input_tokens_details":  map[string]any{"cached_tokens": s.cachedTokens},
		"output_tokens":         s.outputTokens,
		"output_tokens_details": map[string]any{"reasoning_tokens": 0},
		"total_tokens":          s.inputTokens + s.outputTokens,
	}
	response["usage"] = usage
	out = append(out, s.event("response.completed", map[string]any{"response": response}))
	return out
}

func (s *ResponsesStream) fail(event map[string]any) []map[string]any {
	if s.completed {
		return nil
	}
	s.completed = true
	message := stringField(event, "message")
	code := stringField(event, "code")
	if message == "" {
		if errObj, ok := event["error"].(map[string]any); ok {
			message = stringField(errObj, "message")
			code = stringField(errObj, "code")
		}
	}
	if message == "" {
		message = "upstream stream failed"
	}
	return []map[string]any{s.event("response.failed", map[string]any{
		"response": map[string]any{
			"id": s.responseID, "object": "response", "created_at": s.createdAt,
			"model": s.modelName(), "status": "failed", "output": []any{},
			"error": map[string]any{"code": code, "message": message},
		},
	})}
}

func (s *ResponsesStream) ensureCreated() []map[string]any {
	if s.created {
		return nil
	}
	s.created = true
	return []map[string]any{s.event("response.created", map[string]any{
		"response": map[string]any{
			"id": s.responseID, "object": "response", "created_at": s.createdAt,
			"model": s.modelName(), "status": "in_progress", "output": []any{},
		},
	})}
}

func (s *ResponsesStream) openBlock(index int, block map[string]any) []map[string]any {
	if block == nil {
		return nil
	}
	switch stringField(block, "type") {
	case "text":
		b := &respBlock{kind: "text", index: s.allocIndex(), itemID: newItemID("msg")}
		s.blocks[index] = b
		s.order = append(s.order, index)
		b.partOpen = true
		return []map[string]any{
			s.event("response.output_item.added", map[string]any{
				"output_index": b.index,
				"item": map[string]any{
					"type": "message", "id": b.itemID, "role": "assistant", "status": "in_progress",
					"content": []any{map[string]any{"type": "output_text", "text": "", "annotations": []any{}}},
				},
			}),
			s.event("response.content_part.added", map[string]any{
				"output_index": b.index, "content_index": 0, "item_id": b.itemID,
				"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
			}),
		}
	case "thinking", "redacted_thinking":
		b := &respBlock{kind: "thinking", index: s.allocIndex(), itemID: newItemID("rs")}
		s.blocks[index] = b
		s.order = append(s.order, index)
		return []map[string]any{
			s.event("response.output_item.added", map[string]any{
				"output_index": b.index,
				"item": map[string]any{
					"type": "reasoning", "id": b.itemID, "summary": []any{}, "status": "in_progress",
				},
			}),
			s.event("response.reasoning_summary_part.added", map[string]any{
				"output_index": b.index, "summary_index": 0, "item_id": b.itemID,
				"part": map[string]any{"type": "summary_text", "text": ""},
			}),
		}
	case "tool_use":
		name := stringField(block, "name")
		b := &respBlock{
			kind: "tool", index: s.allocIndex(), itemID: newItemID("fc"),
			callID: stringField(block, "id"), name: name,
			custom: s.Meta.CustomTools[name],
		}
		if b.callID == "" {
			b.callID = newItemID("call")
		}
		if ns, ok := s.Meta.Namespaces[name]; ok {
			b.namespace = ns
		}
		if b.custom {
			b.itemID = newItemID("ctc")
		}
		s.blocks[index] = b
		s.order = append(s.order, index)
		item := map[string]any{
			"type": "function_call", "id": b.itemID, "call_id": b.callID,
			"name": b.visibleName(), "arguments": "", "status": "in_progress",
		}
		if b.custom {
			item = map[string]any{
				"type": "custom_tool_call", "id": b.itemID, "call_id": b.callID,
				"name": name, "input": "", "status": "in_progress",
			}
		} else if b.namespace.Namespace != "" {
			item["namespace"] = b.namespace.Namespace
		}
		return []map[string]any{s.event("response.output_item.added", map[string]any{
			"output_index": b.index, "item": item,
		})}
	}
	return nil
}

func (s *ResponsesStream) blockDelta(index int, delta map[string]any) []map[string]any {
	b, ok := s.blocks[index]
	if !ok || delta == nil {
		return nil
	}
	switch stringField(delta, "type") {
	case "text_delta":
		text := stringField(delta, "text")
		if text == "" {
			return nil
		}
		b.text.WriteString(text)
		return []map[string]any{s.event("response.output_text.delta", map[string]any{
			"output_index": b.index, "content_index": 0, "item_id": b.itemID, "delta": text,
		})}
	case "thinking_delta":
		text := stringField(delta, "thinking")
		if text == "" {
			return nil
		}
		b.text.WriteString(text)
		return []map[string]any{s.event("response.reasoning_summary_text.delta", map[string]any{
			"output_index": b.index, "summary_index": 0, "item_id": b.itemID, "delta": text,
		})}
	case "input_json_delta":
		fragment := stringField(delta, "partial_json")
		if fragment == "" {
			return nil
		}
		b.args.WriteString(fragment)
		if b.custom {
			// Custom tool input is buffered: streaming raw {"input":...}
			// fragments would leak the envelope and JSON escapes.
			return nil
		}
		return []map[string]any{s.event("response.function_call_arguments.delta", map[string]any{
			"output_index": b.index, "item_id": b.itemID, "call_id": b.callID,
			"name": b.visibleName(), "delta": fragment,
		})}
	}
	return nil
}

func (s *ResponsesStream) closeBlock(index int) []map[string]any {
	b, ok := s.blocks[index]
	if !ok || b.textDone {
		return nil
	}
	b.textDone = true
	switch b.kind {
	case "text":
		text := b.text.String()
		item := messageItem(b.itemID, text)
		s.finished = append(s.finished, item)
		events := []map[string]any{
			s.event("response.output_text.done", map[string]any{
				"output_index": b.index, "content_index": 0, "item_id": b.itemID, "text": text,
			}),
			s.event("response.content_part.done", map[string]any{
				"output_index": b.index, "content_index": 0, "item_id": b.itemID,
				"part": map[string]any{"type": "output_text", "text": text, "annotations": []any{}},
			}),
		}
		if b.partOpen {
			b.partOpen = false
		}
		return append(events, s.event("response.output_item.done", map[string]any{
			"output_index": b.index, "item": item,
		}))
	case "thinking":
		text := b.text.String()
		item := map[string]any{
			"type": "reasoning", "id": b.itemID, "status": "completed",
			"summary": []any{map[string]any{"type": "summary_text", "text": text}},
		}
		s.finished = append(s.finished, item)
		return []map[string]any{
			s.event("response.reasoning_summary_text.done", map[string]any{
				"output_index": b.index, "summary_index": 0, "item_id": b.itemID, "text": text,
			}),
			s.event("response.reasoning_summary_part.done", map[string]any{
				"output_index": b.index, "summary_index": 0, "item_id": b.itemID,
				"part": map[string]any{"type": "summary_text", "text": text},
			}),
			s.event("response.output_item.done", map[string]any{"output_index": b.index, "item": item}),
		}
	case "tool":
		args := strings.TrimSpace(b.args.String())
		if args == "" {
			args = "{}"
		}
		if b.custom {
			input := customToolInput(args)
			item := map[string]any{
				"type": "custom_tool_call", "id": b.itemID, "call_id": b.callID,
				"name": b.name, "input": input, "status": "completed",
			}
			s.finished = append(s.finished, item)
			events := []map[string]any{}
			if input != "" {
				events = append(events, s.event("response.custom_tool_call_input.delta", map[string]any{
					"output_index": b.index, "item_id": b.itemID, "call_id": b.callID,
					"name": b.name, "delta": input,
				}))
			}
			events = append(events,
				s.event("response.custom_tool_call_input.done", map[string]any{
					"output_index": b.index, "item_id": b.itemID, "call_id": b.callID,
					"name": b.name, "input": input,
				}),
				s.event("response.output_item.done", map[string]any{"output_index": b.index, "item": item}),
			)
			return events
		}
		item := map[string]any{
			"type": "function_call", "id": b.itemID, "call_id": b.callID,
			"name": b.visibleName(), "arguments": args, "status": "completed",
		}
		if b.namespace.Namespace != "" {
			item["namespace"] = b.namespace.Namespace
		}
		s.finished = append(s.finished, item)
		return []map[string]any{
			s.event("response.function_call_arguments.done", map[string]any{
				"output_index": b.index, "item_id": b.itemID, "call_id": b.callID,
				"name": b.visibleName(), "arguments": args,
			}),
			s.event("response.output_item.done", map[string]any{"output_index": b.index, "item": item}),
		}
	}
	return nil
}

// visibleName is the name the client sees: the flattened name when a
// namespace tool was restored, the original name otherwise.
func (b *respBlock) visibleName() string {
	if b.namespace.Name != "" {
		return b.namespace.Name
	}
	return b.name
}

func (s *ResponsesStream) trackUsage(usage map[string]any) {
	num := func(key string) int64 {
		v, _ := usage[key].(float64)
		if v != v {
			return 0
		}
		return int64(v)
	}
	if v := num("input_tokens"); v > s.inputTokens {
		s.inputTokens = v
	}
	if v := num("output_tokens"); v > 0 {
		s.outputTokens = v
	}
	if v := num("cache_read_input_tokens"); v > s.cachedTokens {
		s.cachedTokens = v
	}
}

func (s *ResponsesStream) event(typ string, fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields)+2)
	for k, v := range fields {
		out[k] = v
	}
	out["type"] = typ
	out["sequence_number"] = s.seq
	s.seq++
	return out
}

func (s *ResponsesStream) allocIndex() int {
	index := s.nextIndex
	s.nextIndex++
	return index
}

func (s *ResponsesStream) modelName() string {
	if s.Model == "" {
		return DefaultModel
	}
	return s.Model
}

func intOf(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return -1
}

func nowUnix() int64 { return time.Now().Unix() }
