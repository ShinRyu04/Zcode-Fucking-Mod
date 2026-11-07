// Package tui renders a small live terminal panel over the bridge's JSON
// endpoints. It is deliberately dependency-free: ANSI escapes for painting and
// Win32 console calls for raw key input on Windows, a line-based fallback
// elsewhere.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Config points the panel at a running bridge.
type Config struct {
	BaseURL string
	Token   string
}

type usageModel struct {
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read_tokens"`
	AvgTPS       float64 `json:"avg_tps"`
	HighTPS      float64 `json:"high_tps"`
}

type usageView struct {
	Today struct {
		Requests     int                   `json:"requests"`
		InputTokens  int                   `json:"input_tokens"`
		OutputTokens int                   `json:"output_tokens"`
		CacheRead    int                   `json:"cache_read_tokens"`
		AvgTPS       float64               `json:"avg_tps"`
		HighTPS      float64               `json:"high_tps"`
		PerModel     map[string]usageModel `json:"per_model"`
	} `json:"today"`
	Rings struct {
		LastTPS        float64 `json:"last_tps"`
		HighTPS        float64 `json:"high_tps"`
		SevenDayAvgTPS float64 `json:"seven_day_avg_tps"`
	} `json:"rings"`
}

type healthView struct {
	Lane     string `json:"lane"`
	Version  string `json:"version"`
	Accounts []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"accounts"`
}

type netView struct {
	Supported      bool    `json:"supported"`
	TotalInBps     float64 `json:"total_in_bps"`
	TotalOutBps    float64 `json:"total_out_bps"`
	TCPConnections int     `json:"tcp_connections"`
	Interfaces     []struct {
		Name string `json:"name"`
	} `json:"interfaces"`
	Process struct {
		Found      bool    `json:"found"`
		Name       string  `json:"name"`
		ReadBps    float64 `json:"read_bps"`
		WriteBps   float64 `json:"write_bps"`
		CPUPercent float64 `json:"cpu_percent"`
	} `json:"process"`
}

// Run draws the panel until q/Ctrl-C. Keys: r refresh, 1 usage, 2 network.
func Run(cfg Config) error {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://127.0.0.1:20131"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	keys := make(chan byte, 16)
	go RunInput(ctx, keys)
	enableANSI()

	client := &http.Client{Timeout: 900 * time.Millisecond}
	pane := 0
	refresh := time.NewTicker(time.Second)
	defer refresh.Stop()
	lastErr := ""
	var usage usageView
	var health healthView
	var net netView

	fetch := func() {
		if err := getJSON(client, cfg, "/health", &health); err != nil {
			lastErr = "health: " + err.Error()
			return
		}
		if err := getJSON(client, cfg, "/v1/usage", &usage); err != nil {
			lastErr = "usage: " + err.Error()
			return
		}
		if err := getJSON(client, cfg, "/v1/net", &net); err != nil {
			lastErr = "net: " + err.Error()
		} else {
			lastErr = ""
		}
	}
	fetch()
	paint(cfg, health, usage, net, pane, lastErr)

	for {
		select {
		case <-ctx.Done():
			return nil
		case key := <-keys:
			switch key {
			case 'q', 'Q', 3:
				fmt.Print("\x1b[0m\x1b[?25h\x1b[2J\x1b[H")
				return nil
			case 'r', 'R':
				fetch()
			case '1':
				pane = 0
			case '2':
				pane = 1
			}
			paint(cfg, health, usage, net, pane, lastErr)
		case <-refresh.C:
			fetch()
			paint(cfg, health, usage, net, pane, lastErr)
		}
	}
}

func getJSON(client *http.Client, cfg Config, path string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 (set ZCODE_BRIDGE_TOKEN)")
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(dst)
}

// paint writes one full frame. It never mutates shared state.
func paint(cfg Config, h healthView, u usageView, n netView, pane int, lastErr string) {
	w := consoleWidth()
	if w < 40 {
		w = 80
	}
	var b strings.Builder
	b.WriteString("\x1b[?25l\x1b[H\x1b[2J")
	line := func(format string, args ...any) {
		text := fmt.Sprintf(format, args...)
		text = truncate(text, w-2)
		b.WriteString(" ")
		b.WriteString(text)
		b.WriteString("\x1b[K\r\n")
	}
	line("\x1b[1mzcode-bridge\x1b[0m · %s · akun %d · v%s\x1b[K",
		or(h.Lane, "lane?"), len(h.Accounts), or(h.Version, "?"))
	line("\x1b[2m%s\x1b[0m", strings.Repeat("─", w-2))

	if pane == 0 {
		line("\x1b[1m%8.1f\x1b[0m t/s sekarang   \x1b[2m| tertinggi %.1f · rata-rata 7 hari %.1f\x1b[0m",
			u.Rings.LastTPS, u.Rings.HighTPS, u.Rings.SevenDayAvgTPS)
		line("hari ini  %s permintaan · %s token keluar · %s masuk · cache %s",
			fmtInt(u.Today.Requests), fmtNum(u.Today.OutputTokens), fmtNum(u.Today.InputTokens), pct(u.Today.CacheRead, u.Today.InputTokens+u.Today.CacheRead))
		line("")
		line("\x1b[2m%-24s %6s %10s %8s\x1b[0m", "model", "req", "token", "avg t/s")
		models := make([]usageModel, 0, len(u.Today.PerModel))
		for name, m := range u.Today.PerModel {
			if m.Model == "" {
				m.Model = name
			}
			models = append(models, m)
		}
		sort.Slice(models, func(i, j int) bool { return models[i].OutputTokens > models[j].OutputTokens })
		if len(models) > 8 {
			models = models[:8]
		}
		if len(models) == 0 {
			line("\x1b[2mbelum ada permintaan tercatat\x1b[0m")
		}
		for _, m := range models {
			line("%-24s %6d %10s %8.1f", truncate(m.Model, 24), m.Requests, fmtNum(m.OutputTokens), m.AvgTPS)
		}
	} else {
		if !n.Supported {
			line("\x1b[2mmonitor jaringan tidak tersedia di host ini\x1b[0m")
		} else {
			line("jaringan   ↓ \x1b[1m%s\x1b[0m  ↑ \x1b[1m%s\x1b[0m   %d TCP · %d antarmuka",
				rate(n.TotalInBps), rate(n.TotalOutBps), n.TCPConnections, len(n.Interfaces))
			line("")
			if n.Process.Found {
				line("proses     %s · baca %s · tulis %s · CPU %.1f%%",
					n.Process.Name, rate(n.Process.ReadBps), rate(n.Process.WriteBps), n.Process.CPUPercent)
			} else {
				line("\x1b[2mproses     ZCode tidak berjalan\x1b[0m")
			}
		}
	}
	line("")
	if lastErr != "" {
		line("\x1b[31m%s\x1b[0m", lastErr)
	}
	line("\x1b[2m[q] keluar  [r] muat ulang  [1] penggunaan  [2] jaringan  %s\x1b[0m", time.Now().Format("15:04:05"))
	fmt.Print(b.String())
}

func or(a, b string) string {
	if strings.TrimSpace(a) == "" {
		return b
	}
	return a
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

func fmtInt(n int) string {
	if n < 10000 {
		return fmt.Sprintf("%d", n)
	}
	return fmtNum(n)
}

func fmtNum(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2f B", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1f M", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.0f K", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

func pct(part, total int) string {
	if total <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", float64(part)*100/float64(total))
}

func rate(bps float64) string {
	switch {
	case bps >= 1048576:
		return fmt.Sprintf("%.1f MB/s", bps/1048576)
	case bps >= 1024:
		return fmt.Sprintf("%.1f KB/s", bps/1024)
	}
	return fmt.Sprintf("%.0f B/s", bps)
}
