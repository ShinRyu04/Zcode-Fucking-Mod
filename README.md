# zcode-bridge

> **Full modification — this is NOT the original project.**
> This repository is a **modified derivative** of
> [azrinaldi/zcode-bridge](https://github.com/azrinaldi/zcode-bridge) (MIT License,
> copyright (c) 2026 azrinaldi). Code here differs from upstream in behaviour, not
> only in packaging. Maintained by [ShinRyu04](https://github.com/ShinRyu04).
> Upstream stays the canonical source; see [Modifications](#modifications) for what changed.
> This fork lives at
> [ShinRyu04/zcode-bridge-mod](https://github.com/ShinRyu04/zcode-bridge-mod).

[![license](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![go](https://img.shields.io/github/go-mod/go-version/ShinRyu04/zcode-bridge-mod)](go.mod)
[![deps](https://img.shields.io/badge/dependencies-stdlib%20only-informational)](go.mod)

OpenAI- and Anthropic-compatible HTTP front end for the **ZCode start-plan
quota** (GLM-5.3 / GLM-5.3-Flash). Point any harness that speaks
`/v1/messages` or `/v1/chat/completions` at it, and calls are metered against
the plan grants the account already owns (`ZCODE WEEKEND BUILD`, `ZCode Start
Plan`) — the same quota the ZCode desktop panel shows. Nothing bypasses
billing: the bridge *is* a genuine plan-lane client, reusing the desktop
login's own credential.

Single static Go binary, standard library only, no runtime dependencies.

This fork adds, on top of upstream:

- `POST /v1/responses` — OpenAI Responses wire format for Codex CLI
  (`wire_api = "responses"`), streaming lifecycle included.
- A native request inspector — no MITM: the bridge tees the exchanges it
  already relays, redacts secrets, and shows them in the dashboard's
  **Inspector** tab (`ZCODE_CAPTURE=1` to enable).
- Real throughput metering — tokens come from upstream usage blocks, not byte
  estimates; today/all-time aggregates, per-model rows, speed gauges and two
  charts (15-minute rolling + per-request) live in the dashboard's
  **Penggunaan** tab, backed by `GET /v1/usage` and
  `data/usage-YYYYMMDD.jsonl`.
- A compact **overlay** readout (`GET /overlay`) for a small always-on-top
  window, and a **Playground** tab that sends real prompts through the same
  lane so a model can be tested without leaving the dashboard.

## Install

This fork publishes no release artifacts: build it yourself (Go 1.27+, no
other dependencies). Upstream's prebuilt binaries are published on
[azrinaldi/zcode-bridge releases](https://github.com/azrinaldi/zcode-bridge/releases/latest)
but do not contain the changes below.

```sh
git clone https://github.com/ShinRyu04/zcode-bridge-mod.git
cd zcode-bridge-mod
go build -o zcode-bridge ./cmd/zcode-bridge
./zcode-bridge login            # opens the device flow; approve in a browser
./zcode-bridge                  # serves on 127.0.0.1:20131
```

On Windows, `start-bridge.cmd` / `start-bridge-hidden.vbs` do the same headless
(`stop-bridge.cmd` stops it).

`login` prints an approval URL; open it in any browser signed in to Z.ai
(the desktop app is not required). The resulting plan JWT + refresh token is
stored in `data/accounts.json` (mode `0600`). Repeat `login --label work`
per account; use `login --list`, `login --set-default <id>`,
`login --remove <id>` to manage the ring.

## Connect anything

OpenAI wire format (any SDK: set base URL to `http://127.0.0.1:20131/v1`,
api key to whatever / your bridge token):

```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:20131/v1", api_key="bridge")
r = client.chat.completions.create(
    model="glm-5.3-flash",
    messages=[{"role": "user", "content": "hello"}],
)
```

Anthropic wire format (Claude Code, Aider, …: base URL
`http://127.0.0.1:20131`, auth token ignored / bridge token):

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:20131
export ANTHROPIC_AUTH_TOKEN=bridge
```

curl:

```sh
curl -s http://127.0.0.1:20131/v1/chat/completions \
  -H 'content-type: application/json' \
  -d '{"model":"glm-5.3-flash","messages":[{"role":"user","content":"ping"}]}'
```

## Routes

| Route | Dialect | Notes |
|---|---|---|
| `POST /v1/messages` | Anthropic | pass-through into the plan lane |
| `POST /v1/chat/completions` | OpenAI | translated both ways, incl. streaming + tools |
| `POST /v1/responses` | OpenAI Responses | Codex CLI (`wire_api = "responses"`): input/instructions/tools mapped onto the Anthropic layer, full Responses SSE lifecycle |
| `GET /v1/models` | — | live entitlements merged across plans (static fallback when billing is down) |
| `GET /v1/quota` | — | per-entitlement grant rows (balance-first) |
| `GET /dashboard` | — | built-in status page: lane, credentials, quota, plus **Inspector**, **Penggunaan** and **Playground** tabs (no token needed) |
| `GET /overlay` | — | compact t/s readout page (current, today's average, tokens, sparkline) refreshed every second |
| `GET /inspect?limit=N` | — | captured exchanges, newest first (summaries only) |
| `GET /inspect?id=N` | — | one captured exchange with headers and bodies |
| `GET /inspect?limit=N&full=1` | — | JSON export with bodies (≤50 entries) |
| `DELETE /inspect` | — | drop every captured exchange |
| `GET /v1/usage` | — | token + throughput aggregates: `today`, `all_time`, `recent`, plus a separate `desktop` block read from the ZCode CLI database (needs `ZCODE_BRIDGE_TOKEN` when set) |
| `GET /health` | — | lane, credential ring, device-mid state |

Streaming: Anthropic callers get the upstream SSE stream untouched; OpenAI
callers get `chat.completion.chunk` frames translated from it (`reasoning_content`
carries thinking deltas, tool calls are remapped onto OpenAI's `tool_calls`
indexing, terminated with `[DONE]`).

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `PORT` / `HOST` | `20131` / `127.0.0.1` | listen address (keep it loopback unless you front it with auth) |
| `ZCODE_BRIDGE_TOKEN` | *(unset = open)* | require `Authorization: Bearer` or `x-api-key` on inference routes |
| `ZCODE_PLAN_JWT` | — | explicit plan JWT (highest-priority credential) |
| `ZCODE_PLAN_JWT_FILE` | `./plan-jwt.json` | single-token fallback file |
| `ZCODE_ACCOUNTS_FILE` | `data/accounts.json` | account ring store (0600) |
| `ZCODE_IDENTITY_DEVICE_MID` | auto (per-account UUID from login; random on legacy resolve) | UUID for billing/control calls; without it `/v1/quota` answers `3001` |
| `ZCODE_SESSION_SEED` | `zcode-bridge` | HMAC seed for derived session ids |
| `ZCODE_IDENTITY_CLIENT_LANGUAGE` / `_TIMEZONE` / `_PLATFORM` | auto | identity header overrides |
| `ZCODE_INFERENCE_ORIGIN` | `https://zcode.z.ai` | plan-lane origin (tests point this at a stub) |
| `ZCODE_OAUTH_ORIGIN` | `https://zcode.z.ai` | OAuth/token origin |
| `ZCODE_MAX_ATTEMPTS` | `3` | transient-retry budget per request |
| `ZCODE_RETRY_BASE_MS` | `1500` | exponential backoff base (3009/429/5xx) |
| `ZCODE_UPSTREAM_TIMEOUT_MS` | `300000` | hard upstream deadline |
| `ZCODE_LANE_WIDTH` | `1` | concurrent in-flight upstream requests (the plan lane meters serially; keep 1) |
| `ZCODE_ACCOUNT_COOLDOWN_MS` | `900000` | cooldown after a switchable error (1113/429) |
| `ZCODE_REFRESH_INTERVAL_MS` | `900000` | background refresh check period |
| `ZCODE_FORCE_THINKING` | *(off)* | `1` = force thinking on every request |
| `ZCODE_CORS_ORIGIN` | *(off)* | `*` or comma list; opt-in browser support |
| `ZCODE_SDK_USER_AGENT` | auto | override the SDK user-agent face |
| `ZCODE_CAPTURE` | *(off)* | `1` = record request/response exchanges for the inspector (ring lives in memory) |
| `ZCODE_CAPTURE_SIZE` | `200` | inspector ring capacity (exchanges kept) |
| `ZCODE_CAPTURE_BODY_MAX` | `65536` | bytes of each body the inspector keeps; longer bodies are truncated and flagged |
| `ZCODE_USAGE_DIR` | `data` | where `usage-YYYYMMDD.jsonl` rows are appended (0700 dir, 0600 file) |
| `ZCODE_DB` | `~/.zcode/cli/db/db.sqlite` | ZCode CLI usage database read (never written) for the desktop half of the Penggunaan tab |
| `ZCODE_DESKTOP` | *(on)* | `0` = stop reading the desktop database entirely |

## Lane rules (verified against production)

- The plan lane wants **exactly three system blocks**: the CLI prefix
  verbatim, the canonical stable identity prefix (byte-exact, with
  `cache_control: ephemeral`), then the client's own system text. The bridge
  shapes this from the embedded identity asset — send a client system
  normally, it lands in block 3.
- **No signing headers** (`x-client-sig` etc.) on the plan lane; the lane is
  already authenticated by the plan JWT and signed requests get `1113`.
- Model ids are **case-sensitive lowercase**: `glm-5.3-flash`, not
  `GLM-5.3-Flash`. The bridge normalizes automatically.
- `billing/balance` needs a UUID `X-Device-Mid` or it answers `3001`;
  `billing/current` must use bare auth (control headers empty its payload).
- Errors inside HTTP 200: the `{code,msg,logid}` envelope; `3012` = identity
  gate, `3008` = user concurrency, `3009` = model concurrency (transient,
  retried with backoff), `1113` = grant spent (rotates accounts when a ring
  exists).

## Multi-account

Requests try the default account first; a switchable error (grant spent,
revoked, concurrency) parks that account (cooldown → re-auth on 401/403) and
rotates to the next healthy one in the same request. Pin one account per
request with `?account=<id>` (or the `x-zcode-account` header) to skip
rotation. A 401 triggers one best-effort refresh against the token endpoint
before rotating. Background refresh runs every 15 minutes for tokens
expiring within 2 hours.

## Files

| Path | Purpose |
|---|---|
| `cmd/zcode-bridge` | entry point: serve + login subcommand |
| `internal/server` | HTTP routes incl. `/v1/responses`, SSE translation, session affinity, dashboard (status/inspector/usage) |
| `internal/capture` | bounded request/response ring for the inspector (opt-in, secrets redacted) |
| `internal/usage` | per-request token/throughput meter, JSONL persistence, today/all-time aggregates |
| `internal/transport` | plan-lane client: shaping, rotation, retries |
| `internal/models` | OpenAI ↔ Anthropic translation |
| `internal/quota` | billing readout + per-model merge |
| `internal/accounts` | account ring store (0600), cooldowns, refresh |
| `internal/auth` | device flow, token refresh, credential resolve chain |
| `internal/identity` | identity/telemetry headers, device mid, app version |
| `internal/systemprompt` | embedded identity asset + 3-block shaping |

## Testing

```sh
go test ./...            # end-to-end suite against a stub gateway
go test ./internal/server/ -race
```

## Deploying

Keep it on loopback behind your own auth, or set `ZCODE_BRIDGE_TOKEN` and
bind a private interface. systemd unit:

```ini
[Unit]
Description=zcode-bridge
After=network-online.target

[Service]
ExecStart=/usr/local/bin/zcode-bridge
Environment=ZCODE_BRIDGE_TOKEN=change-me
Restart=on-failure
User=you

[Install]
WantedBy=multi-user.target
```

## Modifications

Changes made in this fork, relative to upstream `azrinaldi/zcode-bridge`:

1. **Plan JWT without an `exp` claim no longer self-destructs after 24h.**
   Upstream stamped `expiresAt = now + 24h` when the plan JWT carried no `exp`,
   and `accounts.Usable()` skips accounts whose `expiresAt` has passed — so a
   perfectly valid, non-expiring plan token was dropped one day after
   `zcode-bridge login`, silently falling back to weaker credential sources and
   causing stalls. Now `expiresAt` stays `0` for such tokens (which
   `auth.IsExpired` already treated as never expiring). File:
   `cmd/zcode-bridge/main.go`.
2. **Windows start/stop scripts** — `start-bridge.cmd`, `start-bridge-hidden.vbs`,
   `stop-bridge.cmd`, `stop-bridge-hidden.vbs`, for running the bridge headless
   from Task Scheduler.
3. **Hardened `.gitignore`** — never commit `data/`, `plan-jwt.json`, `logs/`,
   binaries, or `*.bak-*` backups.
4. **Responses API for Codex CLI** (wire format documented by
   [TriDefender/zcode-api](https://github.com/TriDefender/zcode-api)):
   `POST /v1/responses` accepts `model`, `input` (string or item array),
   `instructions`, `tools`, `tool_choice`, `max_output_tokens`, `temperature`,
   `top_p`, `stream`. The request is converted to the chat shape and then goes
   through the one Anthropic translation layer; `input[]` history (messages,
   `function_call`, `function_call_output`, `custom_tool_call`) is rebuilt with
   the assistant-tool_calls -> tool-reply pairing the API requires. Tools:
   `function` keeps its schema, Codex's `custom` tools (`exec`, `apply_patch`)
   downgrade to a function taking `{"input": string}` and come back as
   `custom_tool_call` items, `namespace` tools flatten to `{ns}__{name}` and are
   restored, hosted tools are dropped. Streaming emits
   `response.created` -> `output_item.added` -> `content_part.added` ->
   `output_text.delta`* -> `output_text.done` -> `content_part.done` ->
   `output_item.done` -> `response.completed` (tool and reasoning items get
   `function_call_arguments.done` / summary events before their
   `output_item.done`, because Codex wedges a turn without them); no response is
   buffered. Non-streaming returns one `response` object with `output_text`,
   `output[]` and `usage`. Codex config:
   `wire_api = "responses"`, `base_url = "http://127.0.0.1:20131/v1"`.
   Files: `internal/models/responses.go`,
   `internal/models/responses_stream.go`, `internal/server/responses.go`.
5. **Built-in request inspector** (idea from
   [TheHolyOneZ/ZProxy](https://github.com/TheHolyOneZ/ZProxy), but no MITM):
   the bridge already terminates both legs, so it records the exchange it
   forwards — method, path, route, model, serving account, request/response
   headers and bodies, status, upstream envelope code
   (3012/3008/3009/1113), duration, first-token latency and every upstream
   attempt/rotation. Opt-in via `ZCODE_CAPTURE=1`; off means `Begin` returns
   nil and the handlers pay nothing. The ring is bounded
   (`ZCODE_CAPTURE_SIZE`, default 200) with bodies cut at
   `ZCODE_CAPTURE_BODY_MAX` (default 64KB). The dashboard's Inspector tab
   filters by method/status/code, searches paths, opens a detail view and
   exports JSON; `Authorization`, cookie and API-key headers plus JWT-shaped
   strings in bodies are redacted before anything is stored. The JSON
   endpoints (`/inspect`, `/v1/usage`) require `ZCODE_BRIDGE_TOKEN` when it is
   set; `/dashboard` itself stays open. Files: `internal/capture`,
   `internal/server/inspect.go`, `internal/server/dashboard.go`,
   `internal/transport` (attempt hooks).
6. **Usage and throughput metering** (presentation borrowed from
   [Masterchiefm/zcode-speed-panel](https://github.com/Masterchiefm/zcode-speed-panel),
   measurement redone): every request that reaches the upstream usage block is
   recorded with its model, serving account, input/output/cache-read tokens,
   first-token latency, generation time and tokens/second. Streaming requests
   measure from the first `content_block_delta` to `message_stop`; non-streaming
   ones keep the full latency but contribute no t/s, so the average is not
   diluted by requests that never had a first token. Rows append to
   `data/usage-YYYYMMDD.jsonl`; on boot the newest 10k rows are re-read so
   today's totals and the chart survive a restart. Exposed as `GET /v1/usage`
   and the dashboard's **Penggunaan** tab: today vs all-time totals, a focal
   average t/s number, per-model table, and a 60-request chart with speed tiers
   (`<40`, `40-80`, `80-160`, `160-240`, `240-320`, `320+` tok/s) drawn as
   inline SVG — no charts library, still stdlib only. The panel's gauges are
   reproduced natively (last request, today's average, cache-hit rate) along
   with a 15-minute rolling chart at 10-second resolution and a per-request
   tier chart; `GET /overlay` is the floating-widget equivalent — run
   `overlay.cmd` and it opens the page in an Edge app window pinned on top
   (`overlay.ps1` does the pinning). Files: `internal/usage`,
   `internal/server/usage.go`, `internal/server/overlay.go`,
   `internal/server/dashboard.go`.
7. **Dashboard playground**: a **Playground** tab posts a prompt to
   `/v1/chat/completions` through the same lane (streaming or not), renders the
   answer and the reasoning block, and reports model/latency/tokens. Useful to
   verify a model id or a tool profile without wiring a client. File:
   `internal/server/dashboard.go`.

8. **Desktop usage database, read without a driver**: the panel's second data
   source — the ZCode CLI's `db.sqlite` — is now read by a small pure-Go
   reader (`internal/sqlite`) that parses the file header, the WAL (including
   checksums and the committed prefix), table b-trees, overflow chains and
   record serialisation. No cgo, no dependency, no lock and no write: the
   desktop application keeps the file open in WAL mode and never notices.
   `internal/desktop` turns those rows into today/all-time totals, average and
   peak t/s, the 7-day high (gated at ≥300 generated tokens and ≥1 s, the same
   rule the panel uses to reject millisecond noise), per-model trends, 90-bucket
   charts for 15 m / 1 h / 6 h / 24 h, and the "a call is running" signal from
   `message` rows with no completion stamp. `/v1/usage` exposes it as a
   separate `desktop` object so bridge-measured and application-reported
   numbers are never silently mixed; the dashboard's **Penggunaan** tab shows
   both side by side with its own range and view switches.

## Credits

This fork is a combination of ideas from several public projects — the bridge
base is [azrinaldi/zcode-bridge](https://github.com/azrinaldi/zcode-bridge), and
the features above were ported (not vendored) from:

| Project | What was taken |
|---|---|
| [TheHolyOneZ/ZProxy](https://github.com/TheHolyOneZ/ZProxy) | request/response inspector idea — re-done without MITM |
| [TriDefender/zcode-api](https://github.com/TriDefender/zcode-api) | `POST /v1/responses` wire format for Codex CLI |
| [Masterchiefm/zcode-speed-panel](https://github.com/Masterchiefm/zcode-speed-panel) | speed-panel presentation (gauges, tiers, rolling chart, floating readout) — measurement replaced by real usage-block math |

No code is copied from those projects; everything is implemented in Go with the
standard library only.

## License

MIT — see [LICENSE](LICENSE).
