# zcode-bridge-mod

> **Full modification.**
> Maintained by [ShinRyu04](https://github.com/ShinRyu04).
> Repository:
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

Key features:

- `POST /v1/responses` — OpenAI Responses wire format for Codex CLI
  (`wire_api = "responses"`), streaming lifecycle included.
- A native request inspector — no MITM: the bridge tees the exchanges it
  already relays, redacts secrets, and shows them in the dashboard's
  **Inspector** tab (`ZCODE_CAPTURE=1` to enable).
- Real throughput metering — tokens come from upstream usage blocks, not byte
  estimates; today/all-time aggregates, per-model rows, three speed rings
  (last / highest / 7-day average), four chart ranges (1 h / 6 h / 24 h / 7 d),
  a per-model trend chart and a per-request chart live in the dashboard's
  **Penggunaan** tab, backed by `GET /v1/usage` and
  `data/usage-YYYYMMDD.jsonl`.
- A **network monitor** card fed by Windows iphlpapi — per-adapter rates,
  loopback TCP attribution for the bridge port, and the ZCode client's own
  IO/CPU counters (gated off when the app is not running) — plus a sticky
  status bar and per-module visibility settings that persist in the browser.
- A compact **overlay** readout (`GET /overlay`) for a small always-on-top
  window — card, mini gauge or capsule, remembered per browser — and a
  **Playground** tab that sends real prompts through the same lane so a model
  can be tested without leaving the dashboard.
- A console rewrite at `/dashboard`: an instrument-style SPA (hash router,
  dark/light theme, embedded assets, no CDN) covering overview, accounts, chat,
  sharing, inspector, usage, network, claim and settings, fed live by `GET /v1/events` (SSE over
  `fetch`, so the bridge token travels in a header). The overview has a full
  **account manager** (start a device-flow login from the browser, switch the
  active account, remove one, see expiry/reauth state). Settings has an
  **agent integration** card: downstream bearer key, base URL, default model
  and a copyable curl example. The legacy page stays at `/classic`.
  A topbar **EN/ID** toggle switches the console to English; Indonesian stays
  the default, and chat message bodies plus code are never translated.
  `GET /v1/settings` reports the effective, restart-required configuration
  without secrets.
- A **chat workspace** at `/chat`: multiple conversations in `localStorage`,
  model picker from `/v1/models`, streaming `/v1/chat/completions` with stop
  and regenerate, temperature/max-token controls, import/export and escaped
  Markdown rendering.
- A **terminal panel** (`zcode-bridge tui`): live usage/rings and network
  panes on a raw ANSI console, keys `1`/`2` switch pane, `r` refreshes, `q`
  quits; no TUI library.
- **Account/plan switching** in the dashboard overview (`GET /v1/accounts`,
  `POST /v1/accounts/switch`), including adding an account from the browser
  via the device flow (`POST /v1/accounts/login`, `GET
  /v1/accounts/login/status`, `POST /v1/accounts/login/cancel`) and removing
  one (`POST /v1/accounts/remove`), plus a **manual plan claim** card
  (`GET`/`POST /v1/claim`, `zcode-bridge claim list|now`) that polls the
  weekend/trial campaigns and claims the highest-priority one.
- A **downstream agent key** surfaced in the dashboard: when no
  `ZCODE_BRIDGE_TOKEN` is set, settings can generate or rotate a persisted
  `sk-…` key (`data/agent-key.json`, mode `0600`) used as the bridge bearer,
  and show/copy the base URL plus a curl example. `GET /v1/integration`
  reports the effective key/source, `POST`/`DELETE /v1/integration/key`
  rotate or remove it. The environment token always wins and is read-only.
- A **local sharing board** at `/dashboard#/sharing`: posts with an optional
  link, pasted image, author name, like toggle and comments, plus newest/top
  sorting. Posts are persisted in `data/sharing.json` (0600); everyone who
  opens the same bridge sees the same board. `GET/POST /v1/sharing`,
  `POST /v1/sharing/like`, `POST /v1/sharing/comment` and
  `DELETE /v1/sharing?id=…&client=…` back it.
- An **idle (off-peak) channel** behind `ZCODE_ASYNC_ENABLED=1`:
  `POST /async/v1/messages` and `/async/v1/chat/completions` take an
  off-peak queue ticket, emit SSE keepalives while waiting, and forward the
  model stream once the ticket turns ready (`GET /async/v1/health` probes the
  queue).
- **Inspector persistence, HAR export and replay**: exchanges append to
  `data/capture.jsonl` and reload after a restart; `GET /inspect?format=har`
  exports a HAR 1.2 session; `POST /inspect/replay?id=N` re-sends one captured
  request in-process.
- **config.yaml + Docker**: the optional `config.yaml` maps onto environment
  variables (env always wins), `Dockerfile`/`docker-compose.yml` run the
  bridge non-root on Linux, and a startup release check (`zcode-bridge
  update-check`) prints the latest GitHub tag without ever updating in place.

## Install

No release artifacts are published here: build it yourself (Go 1.27+, no
other dependencies).

```sh
git clone https://github.com/ShinRyu04/zcode-bridge-mod.git
cd zcode-bridge-mod
go build -o zcode-bridge ./cmd/zcode-bridge
./zcode-bridge login            # opens the device flow; approve in a browser
./zcode-bridge                  # serves on 127.0.0.1:20131
```

On Windows, `start-bridge.cmd` / `start-bridge-hidden.vbs` do the same headless
with `PORT=20135` (`stop-bridge.cmd` stops it). Running `./zcode-bridge`
directly keeps the `20131` default; use whichever port you launched with.

`login` prints an approval URL; open it in any browser signed in to Z.ai
(the desktop app is not required). The resulting plan JWT + refresh token is
stored in `data/accounts.json` (mode `0600`). Repeat `login --label work`
per account; use `login --list`, `login --set-default <id>`,
`login --remove <id>` to manage the ring.

## Desktop companion (Windows)

`go build -o zcode-companion.exe ./cmd/zcode-companion` builds an optional tray
shell (stdlib + Win32 only, no assets). `companion.cmd` starts it with the same
loopback `PORT`/`HOST` as `start-bridge.cmd`. The tray menu opens the dashboard
or capsule overlay, toggles login autostart (per-user `Run` key), checks the
latest GitHub release, restarts the bridge and quits (stopping a bridge it
started). It spawns `zcode-bridge.exe` from its own directory whenever
`/health` stops answering and restarts it after a crash; a bridge started
externally (e.g. Task Scheduler) is left alone until it disappears.

### Headless controls

```sh
zcode-companion.exe -diagnose          # paths, URL, health, autostart state
zcode-companion.exe -autostart=on      # add the Run entry (off removes it)
zcode-companion.exe -no-tray           # supervise without a tray icon
zcode-companion.exe -check-update      # query the latest GitHub release
```

## Connect anything

Examples below use port `20135`, which `companion.cmd` and `start-bridge.cmd`
set. Running `zcode-bridge` directly uses `20131`; substitute your `PORT`.

OpenAI wire format (any SDK: set base URL to `http://127.0.0.1:20135/v1`,
api key to whatever / your bridge token):

```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:20135/v1", api_key="bridge")
r = client.chat.completions.create(
    model="glm-5.3-flash",
    messages=[{"role": "user", "content": "hello"}],
)
```

Anthropic wire format (Claude Code, Aider, …: base URL
`http://127.0.0.1:20135`, auth token ignored / bridge token):

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:20135
export ANTHROPIC_AUTH_TOKEN=bridge
```

curl:

```sh
curl -s http://127.0.0.1:20135/v1/chat/completions \
  -H 'content-type: application/json' \
  -d '{"model":"glm-5.3-flash","messages":[{"role":"user","content":"ping"}]}'
```

When a key exists, copy it from the dashboard's **Pengaturan → Integrasi
agen** card and send it as `Authorization: Bearer sk-…`. The card also shows
the base URL and a ready curl command with the model default. Requests stay
open on loopback until a key is generated; `ZCODE_BRIDGE_TOKEN` overrides the
dashboard key entirely.

## Routes

| Route | Dialect | Notes |
|---|---|---|
| `POST /v1/messages` | Anthropic | pass-through into the plan lane |
| `POST /v1/chat/completions` | OpenAI | translated both ways, incl. streaming + tools |
| `POST /v1/responses` | OpenAI Responses | Codex CLI (`wire_api = "responses"`): input/instructions/tools mapped onto the Anthropic layer, full Responses SSE lifecycle |
| `POST /async/v1/messages` | Anthropic | idle/off-peak lane: queue ticket + SSE keepalives, then the normal Anthropic stream (`ZCODE_ASYNC_ENABLED=1`) |
| `POST /async/v1/chat/completions` | OpenAI | same ticket flow, OpenAI stream translation |
| `GET /async/v1/health` | — | queue availability probe (`can_take_number`, `next_take_at`) |
| `GET /v1/accounts` | — | non-secret account ring for the dashboard switcher |
| `POST /v1/accounts/switch` | — | body `{"id":"<account>"}` pins the rotation head (plan switch) |
| `POST /v1/accounts/remove` | — | body `{"id":"<account>"}` forgets one stored account |
| `POST /v1/accounts/login` | — | body `{"label":"…"}` starts a device flow; returns `flow_id`, `authorize_url`, `expires_at` |
| `GET /v1/accounts/login/status?flow_id=…` | — | `pending` / `ok` / `error` / `cancelled`, with the new account on success |
| `POST /v1/accounts/login/cancel` | — | body `{"flow_id":"…"}` stops waiting for approval |
| `GET /v1/sharing?sort=new\|top&client=…` | — | sharing board posts, like state and ownership for that browser client |
| `POST /v1/sharing` | — | body `{"title","body","link","image","author","client"}` creates a post |
| `POST /v1/sharing/like` | — | body `{"id":"…","client":"…"}` toggles one reaction |
| `POST /v1/sharing/comment` | — | body `{"id","author","body","client"}` adds a reply |
| `DELETE /v1/sharing?id=…&client=…` | — | removes a post when `client` matches its creator |
| `GET /v1/integration` | — | downstream base URL, bearer key/source, default model, endpoint map |
| `POST /v1/integration/key` | — | generate or rotate the persisted downstream key (409 when `ZCODE_BRIDGE_TOKEN` is set) |
| `DELETE /v1/integration/key` | — | remove the persisted key (409 when `ZCODE_BRIDGE_TOKEN` is set) |
| `GET /v1/claim` | — | claimable weekend/trial campaigns (billing preview + marketing banner) |
| `POST /v1/claim` | — | body `{"plan_id":"…"}` or empty for highest priority |
| `GET /v1/models` | — | live entitlements merged across plans (static fallback when billing is down) |
| `GET /v1/quota` | — | per-entitlement grant rows (balance-first) |
| `GET /dashboard` | — | console SPA: overview, accounts, chat, sharing, inspector, usage, network, claim, settings (embedded assets, no CDN) |
| `GET /chat` | — | chat workspace: local conversations, streaming, stop/regenerate, import/export |
| `GET /classic` | — | legacy single-file dashboard (previous tabbed page) |
| `GET /v1/events` | — | SSE snapshot every second: `usage`, `net`, throttled `health` (token-gated when set) |
| `GET /v1/settings` | — | effective lane/host/port/capture/claim/env summary, secrets reduced to presence |
| `GET /v1/claim/history` | — | claim attempts made by this process, newest first (max 50) |
| `GET /overlay` | — | compact t/s readout page (current, today's average, tokens, sparkline) refreshed every second |
| `GET /inspect?limit=N` | — | captured exchanges, newest first (summaries only) |
| `GET /inspect?id=N` | — | one captured exchange with headers and bodies |
| `GET /inspect?limit=N&full=1` | — | JSON export with bodies (≤50 entries) |
| `GET /inspect?format=har&limit=N` | — | HAR 1.2 export of the newest exchanges |
| `POST /inspect/replay?id=N` | — | re-issues one captured request through the local handler chain (new capture entry) |
| `DELETE /inspect` | — | drop every captured exchange (and truncate the persistence file) |
| `GET /v1/usage` | — | token + throughput aggregates: `today`, `all_time`, `rings` (last/high/7-day), `recent`, plus a separate `desktop` block read from the ZCode CLI database (needs `ZCODE_BRIDGE_TOKEN` when set) |
| `GET /v1/net` | — | network sample: per-interface rates/counters, `tcp_connections` for the bridge port, and the matched ZCode process block (`supported=false` off Windows; needs `ZCODE_BRIDGE_TOKEN` when set) |
| `POST /mcp` | MCP | JSON-RPC 2.0 endpoint: `initialize`, `tools/list`, `tools/call` over every configured upstream server |
| `GET /mcp` | — | MCP catalog page (servers, flattened tools, ready-to-paste client config); `?format=json` for the object |
| `GET /health` | — | lane, credential ring, device-mid state |

Streaming: Anthropic callers get the upstream SSE stream untouched; OpenAI
callers get `chat.completion.chunk` frames translated from it (`reasoning_content`
carries thinking deltas, tool calls are remapped onto OpenAI's `tool_calls`
indexing, terminated with `[DONE]`).

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `PORT` / `HOST` | `20131` / `127.0.0.1` | listen address (keep it loopback unless you front it with auth) |
| `ZCODE_BRIDGE_TOKEN` | *(unset = open)* | require `Authorization: Bearer` or `x-api-key` on inference routes; wins over the dashboard key |
| `ZCODE_AGENT_KEY_FILE` | `data/agent-key.json` | persisted downstream key written by `POST /v1/integration/key` (0600) |
| `ZCODE_DEFAULT_MODEL` | `glm-5.3-flash` | model shown in the dashboard integration snippet |
| `ZCODE_PLAN_JWT` | — | explicit plan JWT (highest-priority credential) |
| `ZCODE_PLAN_JWT_FILE` | `./plan-jwt.json` | single-token fallback file |
| `ZCODE_ACCOUNTS_FILE` | `data/accounts.json` | account ring store (0600) |
| `ZCODE_SHARING_FILE` | `data/sharing.json` | local sharing board store (0600) |
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
| `ZCODE_MCP_SERVERS` | *(unset = no upstreams)* | path to (or inline) JSON listing MCP servers; accepts both `{"servers":[…]}` and `{"mcpServers":{…}}` |
| `ZCODE_MCP` | *(off)* | `1` = inject configured MCP tools into non-streaming model calls and run `tool_use → tools/call → tool_result` until the model answers |
| `ZCODE_MCP_MAX_ROUNDS` | `8` | automatic MCP round limit (1–32); client-supplied tools are never executed by the bridge |
| `ZCODE_PROCESS_CALIBRATION` | `1` | display scale (0–1000) applied to the ZCode client's IO rates in `/v1/net`; the counters include file/pipe traffic, so this lets the card be matched against interface rates |
| `ZCODE_CONFIG` | `./config.yaml` when present | optional YAML subset loaded at startup; every key maps onto the env var it names (env wins) |
| `ZCODE_CAPTURE_FILE` / `ZCODE_CAPTURE_DIR` | `data/capture.jsonl` / `data` | inspector persistence target; the ring reloads the newest lines when capture is enabled |
| `ZCODE_ASYNC_ENABLED` | *(off)* | `1` = expose the idle/off-peak `/async/v1/*` routes |
| `ZCODE_ASYNC_ORIGIN` | `https://zcode.z.ai` | off-peak control/inference origin |
| `ZCODE_ASYNC_POLL_MS` / `ZCODE_ASYNC_KEEPALIVE_MS` | `5000` / `3000` | ticket poll and SSE keepalive cadence |
| `ZCODE_ASYNC_MAX_WAIT_MS` / `ZCODE_ASYNC_MAX_RETRIES` | `0` (forever) / `3` | queue wait cap and expired-ticket retakes |
| `ZCODE_CLAIM_ENABLED` / `ZCODE_CLAIM_AUTO` | `1` / `1` | manual plan claim scheduler (inert without a plan JWT) |
| `ZCODE_CLAIM_ORIGIN` / `ZCODE_CLAIM_POLL_INTERVAL_MS` / `ZCODE_CLAIM_COOLDOWN_MS` | `https://zcode.z.ai` / `300000` / `600000` | claim control plane, poll and backoff |
| `ZCODE_CLAIM_PLAN_ID` | *(unset)* | pin one campaign instead of the highest priority |
| `ZCODE_UPDATE_CHECK` | *(on)* | `off` disables the startup GitHub release notice |

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

The dashboard has a dedicated **Akun** page (`/dashboard#/accounts`) and the
same manager on the overview. It lists every stored account with its expiry
and reauth/cooldown badges. **Hubungkan akun** starts a device flow from the
browser (open the returned approval URL, then the card refreshes when the
poll succeeds), **Aktifkan** pins the default account and **Hapus** forgets
one. The CLI and the dashboard share the same
`accounts.CompleteDeviceFlow` path, so both store identical credentials.

## Files

| Path | Purpose |
|---|---|
| `cmd/zcode-bridge` | entry point: serve + login subcommand |
| `cmd/zcode-companion` | optional Windows tray shell: bridge supervisor, login autostart, release update check |
| `internal/server` | HTTP routes incl. `/v1/responses`, SSE translation, session affinity, dashboard (status/inspector/usage), overlay modes |
| `internal/capture` | bounded request/response ring for the inspector (opt-in, secrets redacted, JSONL persistence) |
| `internal/offpeak` | idle-channel ticket client (availability/take/status/settle, retakes) |
| `internal/claim` | weekend/trial plan discovery + claim scheduler |
| `internal/config` | optional `config.yaml` subset mapped onto environment variables |
| `internal/update` | GitHub release check + dotted version compare |
| `internal/tui` | ANSI terminal panel (usage/network panes, raw console input) |
| `internal/usage` | per-request token/throughput meter, JSONL persistence, today/all-time aggregates, rings |
| `internal/desktop` | read-only ZCode CLI database aggregation for the panel side |
| `internal/netmon` | interface/TCP/process counters via iphlpapi (Windows) with a neutral stub elsewhere |
| `internal/sqlite` | pure-Go read-only SQLite reader (btree + WAL) |
| `internal/mcp` | dependency-free MCP client (stdio + streamable HTTP) |
| `internal/transport` | plan-lane client: shaping, rotation, retries |
| `internal/models` | OpenAI ↔ Anthropic translation |
| `internal/quota` | billing readout + per-model merge |
| `internal/accounts` | account ring store (0600), cooldowns, refresh |
| `internal/auth` | device flow, token refresh, credential resolve chain |
| `internal/identity` | identity/telemetry headers, device mid, app version |
| `internal/systemprompt` | embedded identity asset + 3-block shaping |

## Terminal panel

```sh
zcode-bridge tui        # same HOST/PORT/token as serve; q quits
```

Two panes read the local API every second: **Penggunaan** (rings, today,
per-model table, recent requests) and **Jaringan** (adapter rates, bridge TCP
count, ZCode process IO/CPU when the app runs). Implementation is raw ANSI +
`kernel32` console mode on Windows / line input elsewhere — no TUI library.

## Idle channel (off-peak)

Off-peak calls run on free-capacity tickets instead of grant seconds. Enable
with `ZCODE_ASYNC_ENABLED=1` (the account needs coding-plan entitlement;
without it the queue answers `coding plan is required`). Clients call
`POST /async/v1/messages` or `/async/v1/chat/completions` with the usual
dialect body and `"stream": true`: the bridge takes a ticket, emits SSE
`keepalive` comments while the ticket is queued, then forwards the model
stream through the idle lane. Non-streaming requests wait silently and return
a normal JSON body. `GET /async/v1/health` reports whether a ticket can be
taken now.

## Plan claim

Weekend/trial campaigns are claimed from the billing preview plus the
marketing banner face:

```sh
zcode-bridge claim list     # plan id, priority, window
zcode-bridge claim now      # claim the highest-priority live campaign
```

The scheduler (`ZCODE_CLAIM_ENABLED=1`, `ZCODE_CLAIM_AUTO=1`) runs every five
minutes and backs off ten minutes after a failure. Captcha-gated campaigns
cannot be solved headlessly — the error names that case, and the ZCode
desktop app remains the fallback. The dashboard's Status tab has the same
list with a Klaim button, and the account ring is switchable there
("Akun aktif"), which changes which plan credential the next inference call
uses.

## Configuration and Docker

Copy `config.example.yaml` to `config.yaml` (or point `ZCODE_CONFIG` at it);
only the documented scalar keys are read, and any environment variable that
is already set wins over the file. For containers:

```sh
docker compose up -d --build
```

The image builds a static binary, runs as an unprivileged user, listens on
`0.0.0.0:20131`, and keeps `data/` (accounts, usage, capture) on a volume.
Set `ZCODE_BRIDGE_TOKEN` before publishing the port. `zcode-bridge
update-check` (or the companion's `-check-update`) prints the latest GitHub
release; nothing is ever replaced in place.

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

## Implementation notes

Behaviour and features that make up this build:

1. **Plan JWT without an `exp` claim stays valid.** Such a token is stored with
   `expiresAt = 0`, and `accounts.Usable()` only drops accounts whose recorded
   expiry has passed. A non-expiring plan token is therefore never discarded
   after 24h and does not fall back to weaker credential sources. File:
   `cmd/zcode-bridge/main.go`.
2. **Windows start/stop scripts** — `start-bridge.cmd`, `start-bridge-hidden.vbs`,
   `stop-bridge.cmd`, `stop-bridge-hidden.vbs`, for running the bridge headless
   from Task Scheduler.
3. **Hardened `.gitignore`** — never commit `data/`, `plan-jwt.json`, `logs/`,
   binaries, or `*.bak-*` backups.
4. **Responses API for Codex CLI**: `POST /v1/responses` accepts `model`, `input` (string or item array),
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
   `wire_api = "responses"`, `base_url = "http://127.0.0.1:20135/v1"`
   (`20131` when running the binary directly). Files: `internal/models/responses.go`,
   `internal/models/responses_stream.go`, `internal/server/responses.go`.
5. **Built-in request inspector**: the bridge already terminates both legs, so it records the exchange it
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
6. **Usage and throughput metering**: every request that reaches the upstream usage block is
   recorded with its model, serving account, input/output/cache-read tokens,
   first-token latency, generation time and tokens/second. Streaming requests
   measure from the first `content_block_delta` to `message_stop`; non-streaming
   ones keep the full latency but contribute no t/s, so the average is not
   diluted by requests that never had a first token. Rows append to
   `data/usage-YYYYMMDD.jsonl`; on boot the newest 10k rows are re-read so
   today's totals and the chart survive a restart. Exposed as `GET /v1/usage`
   and the dashboard's **Penggunaan** tab: today vs all-time totals, three
   rings (last request, highest ever, 7-day token-weighted average), a
   per-model table, and four selectable chart ranges (1 h / 6 h / 24 h / 7 d)
   plus a per-model trend drawn as inline SVG — no charts library, still
   stdlib only. Throughput tiers use the panel's colors
   (`<40`, `40-80`, `80-160`, `160-240`, `240-320`, `320+` tok/s). The cache
   hit rate moved into today's meta row; `GET /overlay` is the floating-widget
   equivalent — run `overlay.cmd` and it opens the page in an Edge app window
   pinned on top (`overlay.ps1` does the pinning). Files: `internal/usage`,
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

9. **One MCP endpoint over many servers**: `POST /mcp` is a JSON-RPC 2.0
   server that flattens every configured upstream tool into
   `{server}__{tool}`. Upstreams are started or connected lazily on the first
   request from `ZCODE_MCP_SERVERS`; both stdio children (newline-delimited
   JSON, with pagination and a stderr ring for diagnostics) and the streamable
   HTTP transport (JSON or SSE answers, `Mcp-Session-Id` continuity) are
   implemented in `internal/mcp` with no dependency. A failing server never
   takes the endpoint down: its error is reported on the `GET /mcp` catalog
   next to the healthy ones, and tool-name collisions are resolved by keeping
   the first and flagging the server. `GET /mcp` doubles as the setup page —
   it prints the exact `mcpServers` block for Claude/Codex-style clients.

10. **Network monitor without dependencies**: `internal/netmon` samples
   interface counters through `iphlpapi`'s `GetIfTable` and maps rows to real
   adapters with `GetAdaptersAddresses`, so filter-layer duplicates and device
   paths never reach the card (friendly names like `Wi-Fi` do). TCP attribution
   uses `GetExtendedTcpTable` filtered by the bridge port, and the ZCode client
   process tree is matched by executable name with `GetProcessIoCounters` and
   `GetProcessTimes` for read/write/CPU deltas; when the app is not running the
   process block is omitted entirely. Rates are one-second deltas, guarded
   against counter wrap; `/v1/net` is the JSON face, the dashboard's Status tab
   shows the card with a 3-second refresh, and a sticky status bar summarizes
   lane, account, request count, speed and network. Module visibility
   (Status/Inspector/Penggunaan/Playground/network card) is a persisted
   browser-side setting. Files: `internal/netmon`, `internal/server/routes.go`,
   `internal/server/dashboard.go`.

11. **Idle/off-peak channel and plan claim**: `internal/offpeak` implements the ticket protocol
    (availability, take, batched status, settle) with retakes, a wait cap and
    SSE keepalives; `POST /async/v1/messages` and `/async/v1/chat/completions`
    bind a ready ticket to the request context, so the existing shaping,
    translation, capture and metering paths stay untouched while
    `internal/transport` switches to the off-peak origin and ticket headers.
    `internal/claim` discovers campaigns from the billing preview and the
    marketing banner face, then claims the highest-priority one; a five-minute
    scheduler plus `zcode-bridge claim list|now` and `/v1/claim` expose it.
    Account/plan switching (`/v1/accounts`, `/v1/accounts/switch`) is the
    dashboard's provider/plan selector. Files:
    `internal/offpeak`, `internal/claim`, `internal/transport/offpeak.go`,
    `internal/server/async.go`, `internal/server/claim_api.go`,
    `internal/server/accounts_api.go`.

12. **Inspector persistence, HAR export and replay**: the redacted ring
    appends to `data/capture.jsonl` (newest 10k reloaded, file compacted past
    8MB), `GET /inspect?format=har` emits HAR 1.2, and
    `POST /inspect/replay?id=N` re-sends a captured request through the local
    handler chain in-process (no proxy hop), refusing truncated bodies so the
    prompt cannot change. Files: `internal/capture/store.go`,
    `internal/server/inspect.go`, `internal/server/dashboard.go`.

13. **Terminal panel**: `zcode-bridge tui` renders usage rings, per-model
    rows and network counters in two ANSI panes (Windows raw console through
    `kernel32`, line input elsewhere) with `1`/`2`/`r`/`q`. Files:
    `internal/tui`, `cmd/zcode-bridge/main.go`.

14. **Deployment conveniences**: optional `config.yaml` (flat YAML subset
    mapped onto env vars, env wins), a non-root Dockerfile,
    `docker-compose.yml`, `config.example.yaml`, and GitHub release checks
    (`internal/update`, `zcode-bridge update-check`, startup notice). Files:
    `internal/config`, `internal/update`.

## License

MIT — see [LICENSE](LICENSE).
