package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
	"github.com/azrinaldi/zcode-bridge/internal/quota"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
)

type poolRow struct {
	Model     string  `json:"model"`
	PlanName  string  `json:"plan_name"`
	Total     float64 `json:"total"`
	Used      float64 `json:"used"`
	Remaining float64 `json:"remaining"`
	EndsAt    any     `json:"ends_at"`
}

type poolAccount struct {
	accounts.Summary
	Rows  []poolRow `json:"rows"`
	Error string    `json:"error,omitempty"`
}

type poolResult struct {
	OK       bool          `json:"ok"`
	At       int64         `json:"at"`
	Models   []string      `json:"models"`
	Accounts []poolAccount `json:"accounts"`
}

const poolTTL = 45 * time.Second

var (
	poolMu    sync.Mutex
	poolCache *struct {
		at   time.Time
		data poolResult
	}
)

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case jsonNumber:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// jsonNumber is the subset of encoding/json.Number used above without
// pulling the import into the type switch.
type jsonNumber interface{ Float64() (float64, error) }

// handleAccountsPool reports every stored account with its per-model plan
// grants. One upstream read per account, cached briefly, because the
// balance endpoint meters like any other control call.
func handleAccountsPool(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") != ""
	poolMu.Lock()
	defer poolMu.Unlock()
	now := time.Now()
	if !force && poolCache != nil && now.Sub(poolCache.at) < poolTTL {
		sendJSON(w, 200, poolCache.data)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	sums := accounts.Summarize()
	byID := make(map[string]accounts.Summary, len(sums))
	for _, s := range sums {
		byID[s.ID] = s
	}
	cands, err := transport.Candidates("", "")
	if err != nil {
		sendJSON(w, 200, map[string]any{"ok": false, "error": err.Error(), "accounts": []any{}, "models": []any{}})
		return
	}

	out := make([]poolAccount, len(cands))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, c := range cands {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			pa := poolAccount{}
			if s, ok := byID[c.ID]; ok {
				pa.Summary = s
			} else {
				pa.ID = firstNonEmpty(c.ID, c.Label, "default")
				pa.Label = firstNonEmpty(c.Label, c.ID, "default")
			}
			q, qerr := quota.ReadQuota(ctx, c.JWT, c.DeviceID)
			if qerr != nil {
				pa.Error = qerr.Error()
				pa.Rows = []poolRow{}
				out[i] = pa
				return
			}
			pa.Rows = make([]poolRow, 0, len(q.Rows))
			for _, row := range q.Rows {
				if row.Model == "" {
					continue
				}
				total := toFloat(row.TotalUnits)
				used := toFloat(row.UsedUnits)
				rem := toFloat(row.RemainingUnits)
				if rem == 0 && total > 0 {
					rem = total - used
					if rem < 0 {
						rem = 0
					}
				}
				pa.Rows = append(pa.Rows, poolRow{
					Model: row.Model, PlanName: firstNonEmpty(row.PlanName, row.Plan),
					Total: total, Used: used, Remaining: rem, EndsAt: row.EndsAt,
				})
			}
			if len(pa.Rows) == 0 && len(q.Errors) > 0 {
				pa.Error = strings.Join(q.Errors, "; ")
			}
			out[i] = pa
		}(i)
	}
	wg.Wait()

	models := make([]string, 0, 8)
	seen := map[string]bool{}
	for _, a := range out {
		for _, row := range a.Rows {
			if !seen[row.Model] {
				seen[row.Model] = true
				models = append(models, row.Model)
			}
		}
	}
	res := poolResult{OK: true, At: now.Unix(), Models: models, Accounts: out}
	if len(out) > 0 {
		poolCache = &struct {
			at   time.Time
			data poolResult
		}{at: now, data: res}
	}
	sendJSON(w, 200, res)
}
