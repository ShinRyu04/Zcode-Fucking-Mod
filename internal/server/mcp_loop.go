package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/capture"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
	"github.com/azrinaldi/zcode-bridge/internal/usage"
)

const defaultMCPRounds = 8

// mcpToolLoopEnabled is deliberately opt-in. Injecting tools changes model
// behavior; exposing /mcp alone does not.
func mcpToolLoopEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ZCODE_MCP")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func mcpToolRounds() int {
	if raw := strings.TrimSpace(os.Getenv("ZCODE_MCP_MAX_ROUNDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 32 {
			return n
		}
	}
	return defaultMCPRounds
}

func hasRequestTools(body map[string]any) bool {
	existing, ok := body["tools"].([]any)
	return ok && len(existing) > 0
}
func shouldAutoMCP(body map[string]any) bool {
	return mcpToolLoopEnabled() && !hasRequestTools(body)
}

func prepareMCPTools(body map[string]any) bool {
	if !shouldAutoMCP(body) || len(mcpServers().Tools()) == 0 {
		return false
	}
	if hasRequestTools(body) {
		return false
	}
	tools := mcpServers().Tools()
	injected := make([]any, 0, len(tools))
	for _, tool := range tools {
		schema := tool.InputSchema
		if len(schema) == 0 || string(schema) == "null" {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		var input any
		if json.Unmarshal(schema, &input) != nil {
			input = map[string]any{"type": "object"}
		}
		entry := map[string]any{"name": tool.Name, "input_schema": input}
		if tool.Description != "" {
			entry["description"] = tool.Description
		}
		injected = append(injected, entry)
	}
	body["tools"] = injected
	return true
}

type mcpTurnResult struct {
	Response map[string]any
	Raw      []byte
	Rounds   int
}

// runMCPToolLoop executes automatic MCP calls for one non-streaming Anthropic
// request. Every model turn remains inside the caller's existing transport
// lane, preserving account/session affinity and gateway concurrency limits.
func runMCPToolLoop(ctx context.Context, body map[string]any, accountID, sessionID string, rec *capture.Recorder, meter *usage.Meter) (mcpTurnResult, error) {
	if !prepareMCPTools(body) {
		return sendMCPModelTurn(ctx, body, accountID, sessionID, rec, meter)
	}

	for round := 0; round < mcpToolRounds(); round++ {
		turn, err := sendMCPModelTurn(ctx, body, accountID, sessionID, rec, meter)
		if err != nil {
			return turn, err
		}
		turn.Rounds = round + 1
		uses := mcpToolUses(turn.Response)
		if len(uses) == 0 {
			return turn, nil
		}
		appendMCPMessages(body, map[string]any{"role": "assistant", "content": turn.Response["content"]})
		results := make([]any, 0, len(uses))
		for _, use := range uses {
			callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
			raw, callErr := mcpServers().Call(callCtx, use.Name, use.Input)
			cancel()
			block := map[string]any{"type": "tool_result", "tool_use_id": use.ID, "is_error": callErr != nil}
			if callErr != nil {
				block["content"] = []any{map[string]any{"type": "text", "text": callErr.Error()}}
			} else {
				block["content"] = mcpResultBlocks(raw)
			}
			results = append(results, block)
		}
		appendMCPMessages(body, map[string]any{"role": "user", "content": results})
	}
	return mcpTurnResult{}, fmt.Errorf("MCP tool loop exceeded %d rounds", mcpToolRounds())
}

func sendMCPModelTurn(ctx context.Context, body map[string]any, accountID, sessionID string, rec *capture.Recorder, meter *usage.Meter) (mcpTurnResult, error) {
	upstream, err := transport.SendAnthropic(transport.SendOpts{
		Body:            body,
		AccountID:       accountID,
		ClientSessionID: sessionID,
		Context:         ctx,
		Hooks:           hooksFor(rec, meter),
	})
	if err != nil {
		return mcpTurnResult{}, err
	}
	defer upstream.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(upstream.Body, 32<<20))
	if readErr != nil {
		return mcpTurnResult{}, readErr
	}
	var response map[string]any
	if json.Unmarshal(raw, &response) != nil || response == nil {
		return mcpTurnResult{}, errors.New("upstream returned a non-JSON response")
	}
	meter.NoteUsage(mapOf(response["usage"]))
	return mcpTurnResult{Response: response, Raw: raw}, nil
}

type mcpToolUse struct {
	ID    string
	Name  string
	Input json.RawMessage
}

func mcpToolUses(response map[string]any) []mcpToolUse {
	blocks, _ := response["content"].([]any)
	uses := make([]mcpToolUse, 0)
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok || (block["type"] != "tool_use" && block["type"] != "mcp_tool_use") {
			continue
		}
		name, _ := block["name"].(string)
		id, _ := block["id"].(string)
		if name == "" || id == "" || !mcpServers().Has(name) {
			continue
		}
		input, err := json.Marshal(block["input"])
		if err != nil || len(input) == 0 || string(input) == "null" {
			input = json.RawMessage(`{}`)
		}
		uses = append(uses, mcpToolUse{ID: id, Name: name, Input: input})
	}
	return uses
}

// appendMCPMessages appends turns to body["messages"] preserving whatever
// slice type the caller installed. OpenAI translation builds a typed
// []map[string]any; a plain []any assertion would silently drop the whole
// conversation and the gateway answers 1214 "messages invalid".
func appendMCPMessages(body map[string]any, extra ...any) {
	var merged []any
	switch existing := body["messages"].(type) {
	case []any:
		merged = existing
	case []map[string]any:
		merged = make([]any, 0, len(existing)+len(extra))
		for _, message := range existing {
			merged = append(merged, message)
		}
	default:
		merged = make([]any, 0, len(extra))
	}
	body["messages"] = append(merged, extra...)
}

// mcpResultBlocks maps one MCP tools/call result onto Anthropic text blocks.
func mcpResultBlocks(raw json.RawMessage) []any {
	if len(raw) == 0 || string(raw) == "null" {
		return []any{}
	}
	var result struct {
		Content []any `json:"content"`
	}
	if json.Unmarshal(raw, &result) == nil && len(result.Content) > 0 {
		out := make([]any, 0, len(result.Content))
		for _, part := range result.Content {
			switch value := part.(type) {
			case string:
				out = append(out, map[string]any{"type": "text", "text": value})
			case map[string]any:
				if value["type"] == "text" {
					if text, ok := value["text"].(string); ok {
						out = append(out, map[string]any{"type": "text", "text": text})
					}
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []any{map[string]any{"type": "text", "text": string(raw)}}
}

// mcpStreamingConflict rejects automatic loops before headers are written.
// Streaming remains available when clients own their tools and perform the
// follow-up turn themselves.
func mcpStreamingConflict(body map[string]any) bool {
	stream, _ := body["stream"].(bool)
	if !stream || !mcpToolLoopEnabled() {
		return false
	}
	if hasRequestTools(body) {
		return false
	}
	return len(mcpServers().Tools()) > 0
}
