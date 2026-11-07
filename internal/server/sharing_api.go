package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/azrinaldi/zcode-bridge/internal/sharing"
)

// handleSharingList returns the board for one browser client so the response
// can mark which posts it owns and which it already liked.
func handleSharingList(w http.ResponseWriter, r *http.Request) {
	client := r.URL.Query().Get("client")
	order := r.URL.Query().Get("sort")
	if order != "top" {
		order = "new"
	}
	posts, err := sharing.List(client, order)
	if err != nil {
		sendJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "count": len(posts), "posts": posts})
}

func handleSharingCreate(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var in sharing.Input
	if json.Unmarshal(raw, &in) != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "body bukan JSON yang valid"})
		return
	}
	post, err := sharing.Create(in)
	if err != nil {
		sharingError(w, err)
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "post": post})
}

func handleSharingLike(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var in struct {
		ID     string `json:"id"`
		Client string `json:"client"`
	}
	if json.Unmarshal(raw, &in) != nil || in.ID == "" {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "body harus {\"id\":\"<post>\",\"client\":\"<id>\"}"})
		return
	}
	post, err := sharing.ToggleLike(in.ID, in.Client)
	if err != nil {
		sharingError(w, err)
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "post": post})
}

func handleSharingComment(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var in struct {
		ID     string `json:"id"`
		Client string `json:"client"`
		Author string `json:"author"`
		Body   string `json:"body"`
	}
	if json.Unmarshal(raw, &in) != nil || in.ID == "" {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "body harus {\"id\":\"<post>\",\"body\":\"...\"}"})
		return
	}
	post, err := sharing.AddComment(in.ID, in.Author, in.Body)
	if err != nil {
		sharingError(w, err)
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "post": post})
}

func handleSharingDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	client := r.URL.Query().Get("client")
	if id == "" {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "id wajib diisi"})
		return
	}
	if err := sharing.Delete(id, client); err != nil {
		sharingError(w, err)
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

func sharingError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, sharing.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, sharing.ErrForbidden):
		status = http.StatusForbidden
	}
	sendJSON(w, status, map[string]any{"ok": false, "error": err.Error()})
}
