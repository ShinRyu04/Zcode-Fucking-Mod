package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/azrinaldi/zcode-bridge/internal/capture"
	"github.com/azrinaldi/zcode-bridge/internal/models"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
)

// handleResponses serves POST /v1/responses (OpenAI Responses wire format,
// `wire_api = "responses"` in Codex CLI). The request is translated into the
// chat shape and then through the one Anthropic translation layer; the reply
// is rendered back as Responses output items / SSE events.
func handleResponses(w http.ResponseWriter, r *http.Request, body map[string]any, accountID string) {
	chat, meta := models.ResponsesToChat(body)
	translated := models.OpenAIToAnthropic(chat)
	model := stringOf(translated["model"])
	stream, _ := body["stream"].(bool)

	rec := capture.FromContext(r.Context())
	rec.NoteModel(model)
	rec.NoteStream(stream)

	upstream, err := transport.SendAnthropic(transport.SendOpts{
		Body:            translated,
		AccountID:       accountID,
		ClientSessionID: SessionKeyFor(r, body),
		Context:         r.Context(),
		Hooks:           captureHooks(rec),
	})
	if err != nil {
		rec.NoteError(upstreamCode(err))
		relayUpstreamError(w, err, "openai")
		return
	}
	defer upstream.Body.Close()

	if stream {
		pipeResponsesStream(w, upstream, models.NewResponsesStream(model, meta), rec)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(upstream.Body, 32<<20))
	if err != nil {
		openAIError(w, 502, "Upstream returned a non-JSON response", nil)
		return
	}
	var result map[string]any
	if json.Unmarshal(raw, &result) != nil || result == nil {
		openAIError(w, 502, "Upstream returned a non-JSON response", nil)
		return
	}
	out := models.AnthropicToResponses(result, model, meta)
	if instructions, _ := body["instructions"].(string); instructions != "" {
		out["instructions"] = instructions
	}
	if previous := stringOf(body["previous_response_id"]); previous != "" {
		out["previous_response_id"] = previous
	}
	sendJSON(w, 200, out)
}

// pipeResponsesStream rewrites the upstream Anthropic SSE stream into the
// Responses event lifecycle. Nothing is buffered: each upstream frame is
// translated and flushed as it arrives.
func pipeResponsesStream(w http.ResponseWriter, upstream *http.Response, stream *models.ResponsesStream, rec *capture.Recorder) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	flusher, _ := w.(http.Flusher)

	writeEvents := func(events []map[string]any) bool {
		for _, event := range events {
			encoded, err := json.Marshal(event)
			if err != nil {
				continue
			}
			frame := "event: " + stringOf(event["type"]) + "\ndata: " + string(encoded) + "\n\n"
			if _, err := io.WriteString(w, frame); err != nil {
				return false
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		return true
	}

	handlePayload := func(payload string) bool {
		if payload == "" || payload == "[DONE]" {
			return true
		}
		var event map[string]any
		if json.Unmarshal([]byte(payload), &event) != nil {
			return true
		}
		if strings.Contains(payload, "content_block_delta") {
			rec.NoteFirstToken()
		}
		if !writeEvents(stream.Handle(event)) {
			return false
		}
		return stringOf(event["type"]) != "message_stop"
	}

	reader := bufio.NewReaderSize(upstream.Body, 16<<10)
	var data strings.Builder
	for {
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case trimmed == "":
			payload := data.String()
			data.Reset()
			if !handlePayload(payload) {
				return
			}
		case strings.HasPrefix(trimmed, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
		}
		if readErr != nil {
			handlePayload(data.String())
			writeEvents(stream.Finalize())
			return
		}
	}
}
