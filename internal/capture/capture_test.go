package capture

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func record(t *testing.T, path, account string, status int, body string, raw []byte) *Recorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(string(raw)))
	req.Header.Set("Authorization", "Bearer super-secret-bridge-token-0123456789")
	req.Header.Set("Content-Type", "application/json")
	rec := Begin(req, raw)
	if rec == nil {
		t.Fatal("recorder is nil while capture is enabled")
	}
	w := rec.Wrap(httptest.NewRecorder())
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	rec.NoteAccount(account)
	rec.NoteModel("glm-5.3-flash")
	rec.NoteFirstToken()
	rec.NoteAttempt(Event{Kind: "attempt", Account: account, Attempt: 1, Status: status})
	rec.Finish()
	return rec
}

func TestDisabledRecorderIsNil(t *testing.T) {
	ResetForTest()
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	if rec := Begin(req, nil); rec != nil {
		t.Fatal("capture must be inert when ZCODE_CAPTURE is unset")
	}
	var rec *Recorder
	rec.NoteModel("x") // nil Recorder must be a no-op
	rec.Finish()
}

func TestRingBoundsAndNewestFirst(t *testing.T) {
	EnableForTest(2, 1<<10)
	record(t, "/v1/messages", "a1", 200, "first", []byte(`{"model":"glm-5.3-flash"}`))
	record(t, "/v1/chat/completions", "a2", 429, "second", []byte(`{"model":"glm-5.3-flash"}`))
	record(t, "/v1/messages", "a3", 500, "third", []byte(`{"model":"glm-5.3-flash"}`))

	list := List(10)
	if len(list) != 2 {
		t.Fatalf("ring kept %d entries, want capacity 2", len(list))
	}
	if list[0].Status != 500 || list[1].Status != 429 {
		t.Fatalf("newest-first order wrong: %+v", list)
	}
	if list[0].Route != "messages" || list[1].Route != "chat.completions" {
		t.Fatalf("routes = %q, %q", list[0].Route, list[1].Route)
	}
	if list[0].Account != "a3" || list[1].Account != "a2" {
		t.Fatalf("account attribution lost: %+v", list)
	}

	ex, ok := Get(list[0].ID)
	if !ok {
		t.Fatal("Get by id failed")
	}
	if ex.ResponseBody != "third" {
		t.Fatalf("response body = %q", ex.ResponseBody)
	}
	if len(ex.Events) != 1 || ex.Events[0].Kind != "attempt" {
		t.Fatalf("events = %+v", ex.Events)
	}
	if ex.ResponseHeaders["Content-Type"] != "text/event-stream" {
		t.Fatalf("response headers = %+v", ex.ResponseHeaders)
	}
}

func TestBodiesTruncateAndSecretsRedact(t *testing.T) {
	EnableForTest(4, 128)
	body := strings.Repeat("A", 200)
	reqBody := `{"note":"eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoidV8xMjMifQ.sig"}`
	record(t, "/v1/messages", "a1", 200, body, []byte(reqBody))

	list := List(1)
	if len(list) == 0 {
		t.Fatal("no captured entry")
	}
	ex, ok := Get(list[0].ID)
	if !ok {
		t.Fatal("Get failed")
	}
	if ex.ResponseBody != strings.Repeat("A", 128) || !ex.Truncated {
		t.Fatalf("body truncation: %q truncated=%v", ex.ResponseBody, ex.Truncated)
	}
	if ex.ResponseBytes != len(body) {
		t.Fatalf("byte count must track the full body, got %d", ex.ResponseBytes)
	}
	if ex.RequestBody == reqBody {
		t.Fatal("JWT-shaped token material must be redacted from bodies")
	}
	if !strings.Contains(ex.RequestBody, "[redacted-jwt]") {
		t.Fatalf("request body = %q", ex.RequestBody)
	}
	if got := ex.RequestHeaders["Authorization"]; got != "[redacted]" {
		t.Fatalf("authorization header = %q", got)
	}
}

func TestClearEmptiesRing(t *testing.T) {
	EnableForTest(4, 1<<10)
	record(t, "/v1/messages", "a1", 200, "x", []byte(`{}`))
	if n := Clear(); n != 1 {
		t.Fatalf("cleared = %d", n)
	}
	if got := List(10); len(got) != 0 {
		t.Fatalf("list after clear = %+v", got)
	}
	if Status().Count != 0 {
		t.Fatalf("count after clear = %d", Status().Count)
	}
}
