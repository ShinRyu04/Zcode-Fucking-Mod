package server

import (
	"net/http"
	"path/filepath"
	"testing"
)

func TestSharingLifecycle(t *testing.T) {
	setupBridge(t)
	t.Setenv("ZCODE_SHARING_FILE", filepath.Join(t.TempDir(), "sharing.json"))

	code, raw, body := doRequest(t, http.MethodPost, "/v1/sharing", map[string]any{
		"title": "Endpoint gratis", "body": "Coba lane ini", "link": "https://example.com/x",
		"author": "Aya", "client": "client-a",
	})
	if code != 200 {
		t.Fatalf("create status %d: %s", code, raw)
	}
	post, _ := body["post"].(map[string]any)
	id, _ := post["id"].(string)
	if id == "" || post["mine"] != true || post["author"] != "Aya" {
		t.Fatalf("unexpected created post: %s", raw)
	}

	code, raw, body = doRequest(t, http.MethodGet, "/v1/sharing?client=client-a", nil)
	if code != 200 || body["count"].(float64) != 1 {
		t.Fatalf("list status %d: %s", code, raw)
	}
	posts := body["posts"].([]any)
	first := posts[0].(map[string]any)
	if first["likes"].(float64) != 0 || first["liked"] != false || first["mine"] != true {
		t.Fatalf("unexpected list projection: %s", raw)
	}

	code, raw, body = doRequest(t, http.MethodPost, "/v1/sharing/like", map[string]any{"id": id, "client": "client-b"})
	if code != 200 {
		t.Fatalf("like status %d: %s", code, raw)
	}
	liked := body["post"].(map[string]any)
	if liked["likes"].(float64) != 1 || liked["liked"] != true {
		t.Fatalf("like not applied: %s", raw)
	}
	code, raw, body = doRequest(t, http.MethodPost, "/v1/sharing/like", map[string]any{"id": id, "client": "client-b"})
	if code != 200 || body["post"].(map[string]any)["likes"].(float64) != 0 {
		t.Fatalf("like toggle failed %d: %s", code, raw)
	}

	code, raw, body = doRequest(t, http.MethodPost, "/v1/sharing/comment", map[string]any{
		"id": id, "author": "Bima", "body": "Masih jalan?", "client": "client-b",
	})
	if code != 200 {
		t.Fatalf("comment status %d: %s", code, raw)
	}
	comments := body["post"].(map[string]any)["comments"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["body"] != "Masih jalan?" {
		t.Fatalf("comment missing: %s", raw)
	}

	code, raw, _ = doRequest(t, http.MethodDelete, "/v1/sharing?id="+id+"&client=client-b", nil)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner delete status %d: %s", code, raw)
	}
	code, raw, _ = doRequest(t, http.MethodDelete, "/v1/sharing?id="+id+"&client=client-a", nil)
	if code != 200 {
		t.Fatalf("owner delete status %d: %s", code, raw)
	}
	_, _, body = doRequest(t, http.MethodGet, "/v1/sharing", nil)
	if body["count"].(float64) != 0 {
		t.Fatalf("post still present after delete: %v", body)
	}
}

func TestSharingValidation(t *testing.T) {
	setupBridge(t)
	t.Setenv("ZCODE_SHARING_FILE", filepath.Join(t.TempDir(), "sharing.json"))

	for name, payload := range map[string]map[string]any{
		"missing title": {"body": "x", "client": "c"},
		"empty post":    {"title": "x", "client": "c"},
		"bad link":      {"title": "x", "link": "javascript:alert(1)", "client": "c"},
		"bad image":     {"title": "x", "image": "data:text/html;base64,AAAA", "client": "c"},
		"long body":     {"title": "x", "body": string(make([]byte, 5000)), "client": "c"},
	} {
		code, raw, _ := doRequest(t, http.MethodPost, "/v1/sharing", payload)
		if code != http.StatusBadRequest {
			t.Errorf("%s status %d: %s", name, code, raw)
		}
	}
}
