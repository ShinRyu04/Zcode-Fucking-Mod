package claim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAndClaimBest(t *testing.T) {
	var claimed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/zcode-plan/billing/preview":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"plans": []any{
					map[string]any{"plan_id": "p-low", "name": "Low", "priority": 1},
					map[string]any{"plan_id": "p-high", "name": "High", "priority": 9},
				},
			}})
		case "/api/v1/marketing/touch":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"deliveries": []any{map[string]any{
					"campaign_id": "c1",
					"banner": map[string]any{
						"buttons": []any{map[string]any{
							"action": map[string]any{"type": "claim_zcode_plan", "args": map[string]any{"plan_id": "p-banner"}},
						}},
					},
					"zcode_plan": map[string]any{"plan_id": "p-banner", "name": "Banner", "priority": 5},
				}},
			}})
		case "/api/v1/zcode-plan/billing/claim":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			id, _ := body["plan_id"].(string)
			claimed = append(claimed, id)
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"plan": map[string]any{"plan_id": id, "starts_at": 1700000000000.0},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("ZCODE_CLAIM_ORIGIN", srv.URL)
	t.Setenv("ZCODE_IDENTITY_DEVICE_MID", "11111111-2222-3333-4444-555555555555")
	restore := SetJWTResolverForTest(func() string { return "jwt" })
	defer restore()

	plans, err := ListPlans(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 3 {
		t.Fatalf("plans = %+v", plans)
	}
	if plans[0].PlanID != "p-high" {
		t.Fatalf("highest priority first: %+v", plans)
	}

	out, err := ClaimBest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.PlanID != "p-high" {
		t.Fatalf("outcome = %+v", out)
	}
	if len(claimed) != 1 || claimed[0] != "p-high" {
		t.Fatalf("claimed = %v", claimed)
	}
}

func TestClaimCaptchaBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"code": 3007, "message": "captcha verify failed"})
	}))
	defer srv.Close()
	t.Setenv("ZCODE_CLAIM_ORIGIN", srv.URL)
	t.Setenv("ZCODE_IDENTITY_DEVICE_MID", "11111111-2222-3333-4444-555555555555")
	restore := SetJWTResolverForTest(func() string { return "jwt" })
	defer restore()

	out, err := Claim(context.Background(), "p1")
	if err != ErrCaptchaRequired {
		t.Fatalf("err = %v, want ErrCaptchaRequired", err)
	}
	if fmt.Sprint(out.Code) != "3007" || out.OK {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestClaimPinnedPlan(t *testing.T) {
	t.Setenv("ZCODE_CLAIM_PLAN_ID", "pinned")
	t.Setenv("ZCODE_CLAIM_ORIGIN", "http://127.0.0.1:1")
	restore := SetJWTResolverForTest(func() string { return "jwt" })
	defer restore()
	if _, err := ClaimBest(context.Background()); err == nil {
		t.Fatal("unreachable origin must fail")
	}
}
