<p align="center">
  <img src="internal/webui/assets/brand.png" alt="ZCode" width="200">
</p>

<h1 align="center">ZCode</h1>

<p align="center">
  OpenAI- and Anthropic-compatible gateway for ZCode plan quota, with a local operations dashboard.
</p>

<p align="center">
  <a href="https://github.com/ShinRyu04/Zcode-Fucking-Mod/releases/latest"><img src="https://img.shields.io/github/v/release/ShinRyu04/Zcode-Fucking-Mod?label=release" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="MIT license"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/ShinRyu04/Zcode-Fucking-Mod" alt="Go version"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/dependencies-stdlib%20only-informational" alt="Stdlib only"></a>
</p>

ZCode is a full modification of `zcode-bridge`: an OpenAI- and
Anthropic-compatible HTTP front end for the **ZCode start-plan quota**
(GLM-5.3 / GLM-5.3-Flash). Point any harness that speaks `/v1/messages` or
`/v1/chat/completions` at it, and calls are metered against the plan grants
the account already owns (`ZCODE WEEKEND BUILD`, `ZCode Start Plan`) — the same
quota the ZCode desktop panel shows. Nothing bypasses billing: the bridge *is*
a genuine plan-lane client, reusing the desktop login's own credential.

Single static Go binary, standard library only, no runtime dependencies.
The console is embedded vanilla HTML/CSS/JS with no CDN and no build step.

## Quick start

```sh
git clone https://github.com/ShinRyu04/Zcode-Fucking-Mod.git
cd Zcode-Fucking-Mod
go build -o zcode-bridge ./cmd/zcode-bridge
./zcode-bridge login            # device flow; approve in any browser
./zcode-bridge                  # serves on 127.0.0.1:20131
```

On Windows, `start-bridge.cmd` / `start-bridge-hidden.vbs` run the same headless
with `PORT=20135` (`stop-bridge.cmd` stops it). Open
`http://127.0.0.1:20135/dashboard`; the first login uses the bridge key as the
password. Manage or rotate that key under **Pengaturan → Integrasi agen**.

`login` prints an approval URL; open it in a browser signed in to Z.ai (the
desktop app is not required). The plan JWT and refresh token land in
`data/accounts.json` (mode `0600`). Repeat `login --label work` per account;
`login --list`, `login --set-default <id>` and `login --remove <id>` manage the
ring.

### Dashboard login

`/dashboard` and `/chat` are behind a password gate. The password is the
**bridge key**, entered without a username. On this install the key is
**`khalidganteng`**; change it under **Pengaturan → Integrasi agen** or with
`POST /v1/integration/key` (`{"key":"..."}`), then update every client that
sends `Authorization: Bearer <kunci>` or `x-api-key: <kunci>`. When no key is
configured the loopback console stays open. `ZCODE_BRIDGE_TOKEN` overrides the
stored key entirely.

<details>
<summary><b>Features</b></summary>

- `POST /v1/responses` — OpenAI Responses wire format for Codex CLI
  (`wire_api = "responses"`), full streaming lifecycle.
- Native request inspector — no MITM: the bridge tees the exchanges it already
  relays, redacts secrets and shows them under **Inspector**
  (`ZCODE_CAPTURE=1`).
- Real throughput metering — usage blocks, not byte estimates; today/all-time
  aggregates, per-model rows, speed rings, four chart ranges and hover
  crosshair in **Penggunaan**, backed by `GET /v1/usage` and
  `data/usage-YYYYMMDD.jsonl`.
- Network monitor fed by Windows iphlpapi — per-adapter rates, loopback TCP
  attribution and the ZCode client's own IO/CPU counters.
- Console at `/dashboard`: embedded SPA with hash router, dark/light theme,
  EN/ID toggle, password gate, logout, live pipeline card, API docs page and
  `GET /v1/events` SSE feed.
- Account manager: device-flow login from the browser, switching, removal,
  expiry/reauth badges and per-model grants (`GET /v1/accounts/pool`).
- Agent integration card: custom downstream bearer key, base URL, default
  model and copyable curl example.
- Chat workspace at `/chat`: local conversations, streaming, stop/regenerate,
  model picker, import/export and escaped Markdown.
- Terminal panel (`zcode-bridge tui`): usage/rings and network panes, raw ANSI,
  no TUI library.
- Manual plan claim (`zcode-bridge claim list|now`), idle/off-peak channel,
  MCP endpoint, HAR export/replay and `config.yaml` + Docker support.

</details>

<details>
<summary><b>CLI reference</b> — install, companion, terminal, claim</summary>

### Build and login

```sh
go build -o zcode-bridge ./cmd/zcode-bridge
./zcode-bridge login [--label work] [--list] [--set-default <id>] [--remove <id>]
./zcode-bridge                  # serve; PORT/HOST apply
./zcode-bridge tui              # ANSI usage/network panel; q quits
./zcode-bridge claim list       # plan id, priority, window
./zcode-bridge claim now        # claim the highest-priority live campaign
./zcode-bridge update-check     # print latest GitHub tag, never updates
```

### Desktop companion (Windows)

```sh
go build -o zcode-companion.exe ./cmd/zcode-companion
```

`companion.cmd` starts the tray shell. The tray opens the dashboard, toggles
login autostart (per-user `Run` key), checks releases, restarts the bridge and
quits. It spawns `zcode-bridge.exe` from its own directory when `/health` stops
answering and restarts it after a crash; an externally started bridge is left
alone until it disappears.

```sh
zcode-companion.exe -diagnose          # paths, URL, health, autostart state
zcode-companion.exe -autostart=on      # add the Run entry (off removes it)
zcode-companion.exe -no-tray           # supervise without a tray icon
zcode-companion.exe -check-update      # query the latest GitHub release
```

</details>

<details>
<summary><b>Configuration (all flags &amp; env vars)</b></summary>

| Variable | Default | Purpose |
|---|---|---|
| `PORT` / `HOST` | `20131` / `127.0.0.1` | listen address (keep it loopback unless you front it with auth) |
| `ZCODE_BRIDGE_TOKEN` | *(unset = open)* | require `Authorization: Bearer` or `x-api-key`; wins over the dashboard key |
| `ZCODE_AGENT_KEY_FILE` | `data/agent-key.json` | persisted downstream key written by `POST /v1/integration/key` (0600) |
| `ZCODE_DEFAULT_MODEL` | `glm-5.3-flash` | model shown in the dashboard integration snippet |
| `ZCODE_PLAN_JWT` | — | explicit plan JWT (highest-priority credential) |
| `ZCODE_PLAN_JWT_FILE` | `./plan-jwt.json` | single-token fallback file |
| `ZCODE_ACCOUNTS_FILE` | `data/accounts.json` | account ring store (0600) |
| `ZCODE_SHARING_FILE` | `data/sharing.json` | local sharing board store (0600) |
| `ZCODE_IDENTITY_DEVICE_MID` | auto per-account UUID | UUID for billing/control calls; missing it makes `/v1/quota` answer `3001` |
| `ZCODE_SESSION_SEED` | `zcode-bridge` | HMAC seed for derived session ids |
| `ZCODE_IDENTITY_CLIENT_LANGUAGE` / `_TIMEZONE` / `_PLATFORM` | auto | identity header overrides |
| `ZCODE_INFERENCE_ORIGIN` | `https://zcode.z.ai` | plan-lane origin (tests point this at a stub) |
| `ZCODE_OAUTH_ORIGIN` | `https://zcode.z.ai` | OAuth/token origin |
| `ZCODE_MAX_ATTEMPTS` | `3` | transient-retry budget per request |
| `ZCODE_RETRY_BASE_MS` | `1500` | exponential backoff base (3009/429/5xx) |
| `ZCODE_UPSTREAM_TIMEOUT_MS` | `300000` | hard upstream deadline |
| `ZCODE_LANE_WIDTH` | `1` | concurrent in-flight upstream requests (keep 1) |
| `ZCODE_ACCOUNT_COOLDOWN_MS` | `900000` | cooldown after a switchable error (1113/429) |
| `ZCODE_REFRESH_INTERVAL_MS` | `900000` | background refresh check period |
| `ZCODE_FORCE_THINKING` | *(off)* | `1` forces thinking on every request |
| `ZCODE_CORS_ORIGIN` | *(off)* | `*` or comma list; opt-in browser support |
| `ZCODE_SDK_USER_AGENT` | auto | override the SDK user-agent face |
| `ZCODE_CAPTURE` | *(off)* | `1` records exchanges for the inspector |
| `ZCODE_CAPTURE_SIZE` | `200` | inspector ring capacity |
| `ZCODE_CAPTURE_BODY_MAX` | `65536` | bytes kept per body; longer bodies are truncated and flagged |
| `ZCODE_CAPTURE_FILE` / `ZCODE_CAPTURE_DIR` | `data/capture.jsonl` / `data` | inspector persistence target |
| `ZCODE_USAGE_DIR` | `data` | where `usage-YYYYMMDD.jsonl` rows are appended |
| `ZCODE_DB` | `~/.zcode/cli/db/db.sqlite` | ZCode CLI usage database read (never written) |
| `ZCODE_DESKTOP` | *(on)* | `0` stops reading the desktop database |
| `ZCODE_MCP_SERVERS` | *(unset)* | path to or inline JSON listing MCP servers |
| `ZCODE_MCP` | *(off)* | `1` injects MCP tools into non-streaming calls |
| `ZCODE_MCP_MAX_ROUNDS` | `8` | automatic MCP round limit (1–32) |
| `ZCODE_PROCESS_CALIBRATION` | `1` | display scale (0–1000) for client IO rates in `/v1/net` |
| `ZCODE_CONFIG` | `./config.yaml` when present | optional YAML subset; env wins |
| `ZCODE_ASYNC_ENABLED` | *(off)* | `1` exposes the idle/off-peak `/async/v1/*` routes |
| `ZCODE_ASYNC_ORIGIN` | `https://zcode.z.ai` | off-peak control/inference origin |
| `ZCODE_ASYNC_POLL_MS` / `ZCODE_ASYNC_KEEPALIVE_MS` | `5000` / `3000` | ticket poll and SSE keepalive cadence |
| `ZCODE_ASYNC_MAX_WAIT_MS` / `ZCODE_ASYNC_MAX_RETRIES` | `0` / `3` | queue wait cap and expired-ticket retakes |
| `ZCODE_CLAIM_ENABLED` / `ZCODE_CLAIM_AUTO` | `1` / `1` | manual claim scheduler (inert without a plan JWT) |
| `ZCODE_CLAIM_ORIGIN` / `ZCODE_CLAIM_POLL_INTERVAL_MS` / `ZCODE_CLAIM_COOLDOWN_MS` | `https://zcode.z.ai` / `300000` / `600000` | claim control plane, poll and backoff |
| `ZCODE_CLAIM_PLAN_ID` | *(unset)* | pin one campaign instead of the highest priority |
| `ZCODE_UPDATE_CHECK` | *(on)* | `off` disables the startup release notice |

</details>

<details>
<summary><b>API reference</b> — routes and client examples</summary>

### Routes

| Route | Dialect | Notes |
|---|---|---|
| `POST /v1/messages` | Anthropic | pass-through into the plan lane |
| `POST /v1/chat/completions` | OpenAI | translated both ways, incl. streaming + tools |
| `POST /v1/responses` | OpenAI Responses | Codex CLI: input/instructions/tools mapped onto the Anthropic layer |
| `POST /async/v1/messages` | Anthropic | idle/off-peak lane: queue ticket + SSE keepalives (`ZCODE_ASYNC_ENABLED=1`) |
| `POST /async/v1/chat/completions` | OpenAI | same ticket flow, OpenAI stream translation |
| `GET /async/v1/health` | — | queue availability probe |
| `GET /v1/models` | — | live entitlements merged across plans |
| `GET /v1/quota` | — | per-entitlement grant rows (balance-first) |
| `GET /health` | — | lane, credential ring, device-mid state |
| `GET /v1/accounts` | — | non-secret account ring for the dashboard switcher |
| `GET /v1/accounts/pool` | — | per-account status plus per-model grants; `?force=1` bypasses the 45 s cache |
| `POST /v1/accounts/switch` | — | body `{"id":"<account>"}` pins the rotation head |
| `POST /v1/accounts/remove` | — | body `{"id":"<account>"}` forgets one stored account |
| `POST /v1/accounts/login` | — | body `{"label":"…"}` starts a device flow |
| `GET /v1/accounts/login/status?flow_id=…` | — | `pending` / `ok` / `error` / `cancelled` |
| `POST /v1/accounts/login/cancel` | — | body `{"flow_id":"…"}` stops waiting |
| `GET /v1/integration` | — | downstream base URL, bearer key/source, default model, endpoint map |
| `POST /v1/integration/key` | — | generate, rotate or set the downstream key; `{"key":"…"}` stores a custom key |
| `DELETE /v1/integration/key` | — | remove the persisted key (409 when `ZCODE_BRIDGE_TOKEN` is set) |
| `GET /v1/claim` | — | claimable weekend/trial campaigns |
| `POST /v1/claim` | — | body `{"plan_id":"…"}` or empty for highest priority |
| `GET /v1/usage` | — | token + throughput aggregates: `today`, `all_time`, `rings`, `recent`, `desktop` |
| `GET /v1/net` | — | interface rates, bridge TCP count, ZCode process block (Windows) |
| `GET /v1/events` | — | SSE snapshot: `usage`, `net`, throttled `health` |
| `GET /v1/settings` | — | effective configuration, secrets reduced to presence |
| `GET /inspect?limit=N` | — | captured exchanges, newest first |
| `GET /inspect?format=har&limit=N` | — | HAR 1.2 export |
| `POST /inspect/replay?id=N` | — | re-issue one captured request in-process |
| `POST /mcp` / `GET /mcp` | MCP | JSON-RPC 2.0 endpoint and tool catalog page |
| `GET /dashboard` / `GET /chat` | — | console SPA and chat workspace |
| `GET /classic` | — | legacy single-file dashboard |

Streaming: Anthropic callers get the upstream SSE untouched; OpenAI callers get
`chat.completion.chunk` frames translated from it (`reasoning_content` carries
thinking deltas, tool calls are remapped onto OpenAI `tool_calls`, terminated
with `[DONE]`).

### Connect anything

Examples use port `20135` (what `start-bridge.cmd` sets). Running
`zcode-bridge` directly uses `20131`; substitute your `PORT`.

OpenAI wire format:

```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:20135/v1", api_key="bridge")
r = client.chat.completions.create(
    model="glm-5.3-flash",
    messages=[{"role": "user", "content": "hello"}],
)
```

Anthropic wire format (Claude Code, Aider, …):

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

When a key exists, copy it from **Pengaturan → Integrasi agen** (or the
**API Docs** page) and send it as `Authorization: Bearer <kunci>` or
`x-api-key: <kunci>`. `ZCODE_BRIDGE_TOKEN` overrides the dashboard key
entirely.

</details>

<details>
<summary><b>Lane rules, errors, and retries</b></summary>

- The plan lane wants **exactly three system blocks**: the CLI prefix
  verbatim, the canonical stable identity prefix (byte-exact, with
  `cache_control: ephemeral`), then the client's own system text. The bridge
  shapes this from the embedded identity asset, so a client system prompt lands
  in block 3.
- **No signing headers** (`x-client-sig` etc.) on the plan lane; signed
  requests get `1113`.
- Model ids are **lowercase**: `glm-5.3-flash`, not `GLM-5.3-Flash`. The bridge
  normalizes automatically.
- `billing/balance` needs a UUID `X-Device-Mid` or answers `3001`;
  `billing/current` must use bare auth.
- Errors inside HTTP 200: `{code,msg,logid}`; `3012` = identity gate,
  `3008` = user concurrency, `3009` = model concurrency (transient, retried
  with backoff), `1113` = grant spent (rotates accounts when a ring exists).

</details>

<details>
<summary><b>Accounts, claim, and idle channel</b></summary>

### Multi-account

Requests try the default account first; a switchable error (grant spent,
revoked, concurrency) parks that account and rotates to the next healthy one in
the same request. Pin one account per request with `?account=<id>` or
`x-zcode-account`. A 401 triggers one best-effort refresh before rotating.
Background refresh runs every 15 minutes for tokens expiring within 2 hours.

The **Akun** page lists every stored account with expiry and reauth/cooldown
badges. **Hubungkan akun** starts a device flow from the browser;
**Aktifkan** pins the default account; **Hapus** forgets one.

### Plan claim

```sh
zcode-bridge claim list     # plan id, priority, window
zcode-bridge claim now      # claim the highest-priority live campaign
```

The scheduler (`ZCODE_CLAIM_ENABLED=1`, `ZCODE_CLAIM_AUTO=1`) runs every five
minutes and backs off ten minutes after a failure. Captcha-gated campaigns
cannot be solved headlessly; the dashboard's **Klaim plan** page exposes the
same list and a Klaim button.

### Idle channel (off-peak)

Set `ZCODE_ASYNC_ENABLED=1`; the account needs coding-plan entitlement.
`POST /async/v1/messages` or `/async/v1/chat/completions` takes an off-peak
ticket, emits SSE keepalives while queued, then forwards the model stream
through the idle lane. Non-streaming requests wait silently and return a normal
JSON body. `GET /async/v1/health` reports ticket availability.

</details>

<details>
<summary><b>Dashboard, chat, and console behavior</b></summary>

`/dashboard` is an embedded vanilla SPA (hash router, no CDN):

- **Ringkasan** — KPI cards, live pipeline diagram with real request ticker,
  account list.
- **Akun** — account manager and pool table.
- **Klaim plan** — campaign list and claim button.
- **Penggunaan** — today/all-time totals, speed chart with hover tooltip,
  per-model share rings.
- **Request** and **Kuota** — recent request log and per-account grants.
- **Inspector** — captured exchanges with filters and detail view.
- **API Docs** — base URLs, auth headers, endpoint/model tables and copyable
  curl examples.
- **Chat** — `/chat` workspace with local conversations.
- **Lane** and **Sesi operator** — upstream identity and operator summary.
- **Pengaturan** — theme, language, agent integration key.

A password gate covers `/dashboard` and `/chat` when a bridge key exists
(password only, no username); **Keluar** clears the session. The **EN/ID**
toggle translates labels and status text; chat bodies, code and API snippets
are never translated.

</details>

<details>
<summary><b>Files</b></summary>

| Path | Purpose |
|---|---|
| `cmd/zcode-bridge` | entry point: serve, login, claim, tui, update-check |
| `cmd/zcode-companion` | optional Windows tray shell and supervisor |
| `internal/server` | HTTP routes, SSE translation, dashboard APIs, inspector |
| `internal/webui` | embedded console shell and assets |
| `internal/capture` | bounded request/response ring (secrets redacted, JSONL) |
| `internal/offpeak` | idle-channel ticket client |
| `internal/claim` | weekend/trial plan discovery + claim scheduler |
| `internal/config` | optional `config.yaml` mapped onto env vars |
| `internal/update` | GitHub release check |
| `internal/tui` | ANSI terminal panel |
| `internal/usage` | per-request token/throughput meter and aggregates |
| `internal/desktop` | read-only ZCode CLI database aggregation |
| `internal/netmon` | interface/TCP/process counters (Windows) |
| `internal/sqlite` | pure-Go read-only SQLite reader |
| `internal/mcp` | dependency-free MCP client |
| `internal/transport` | plan-lane client: shaping, rotation, retries |
| `internal/models` | OpenAI ↔ Anthropic translation |
| `internal/quota` | billing readout and per-model merge |
| `internal/accounts` | account ring store (0600), cooldowns, refresh |
| `internal/auth` | device flow, token refresh, credential resolve chain |
| `internal/identity` | identity/telemetry headers, device mid, app version |
| `internal/systemprompt` | embedded identity asset + 3-block shaping |

</details>

<details>
<summary><b>Self-hosting, Docker, and tests</b></summary>

Copy `config.example.yaml` to `config.yaml` (or point `ZCODE_CONFIG` at it);
environment variables always win. For containers:

```sh
docker compose up -d --build
```

The image builds a static binary, runs as an unprivileged user, listens on
`0.0.0.0:20131`, and keeps `data/` on a volume. Set `ZCODE_BRIDGE_TOKEN`
before publishing the port.

Keep it on loopback behind your own auth, or set `ZCODE_BRIDGE_TOKEN` and bind
a private interface. systemd unit:

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

```sh
go test ./...            # end-to-end suite against a stub gateway
go test ./internal/server/ -race
```

</details>

<details>
<summary><b>Implementation notes</b></summary>

1. **Plan JWT without `exp` stays valid.** Stored with `expiresAt = 0`;
   `accounts.Usable()` only drops accounts whose recorded expiry has passed.
   File: `cmd/zcode-bridge/main.go`.
2. **Windows scripts** — `start-bridge.cmd`, `start-bridge-hidden.vbs`,
   `stop-bridge.cmd`, `stop-bridge-hidden.vbs` for Task Scheduler.
3. **Hardened `.gitignore`** — never commit `data/`, `plan-jwt.json`, `logs/`,
   binaries, or `*.bak-*` backups.
4. **Responses API for Codex CLI.** `POST /v1/responses` maps `model`, `input`,
   `instructions`, `tools`, `tool_choice`, `max_output_tokens`, `temperature`,
   `top_p` and `stream` onto the chat shape. Codex `custom` tools downgrade to
   a function taking `{"input": string}` and return as `custom_tool_call`;
   namespace tools flatten to `{ns}__{name}`. Streaming emits
   `response.created` → `output_item.added` → `content_part.added` →
   `output_text.delta`* → `output_text.done` → `content_part.done` →
   `output_item.done` → `response.completed`; nothing is buffered. Files:
   `internal/models/responses.go`, `internal/models/responses_stream.go`,
   `internal/server/responses.go`.
5. **Built-in inspector.** Records method, path, route, model, account,
   headers/bodies, status, upstream code, duration, first-token latency and
   rotation attempts. Opt-in `ZCODE_CAPTURE=1`; ring bounded by
   `ZCODE_CAPTURE_SIZE`; bodies cut at `ZCODE_CAPTURE_BODY_MAX`; auth headers
   and JWT-shaped strings are redacted. Files: `internal/capture`,
   `internal/server/inspect.go`.
6. **Usage metering.** Streaming t/s measures from first
   `content_block_delta` to `message_stop`; non-streaming keeps full latency
   and contributes no t/s. Rows append to `data/usage-YYYYMMDD.jsonl`; newest
   10k reload on boot. Files: `internal/usage`, `internal/server/usage.go`.
7. **Desktop usage database.** `internal/sqlite` parses header, WAL, b-trees,
   overflow chains and records without cgo or a write lock.
   `internal/desktop` turns rows into today/all-time totals, peaks, 7-day high,
   per-model trends and 90-bucket charts; `/v1/usage` exposes a separate
   `desktop` object.
8. **MCP endpoint.** `POST /mcp` flattens every configured upstream tool into
   `{server}__{tool}`; stdio and streamable HTTP transports live in
   `internal/mcp`. `GET /mcp` prints the `mcpServers` block for clients.
9. **Network monitor.** `internal/netmon` uses `iphlpapi` (`GetIfTable`,
   `GetAdaptersAddresses`, `GetExtendedTcpTable`, `GetProcessIoCounters`) with
   one-second deltas; `/v1/net` is the JSON face.
10. **Idle channel and claim.** `internal/offpeak` implements ticket
    availability/take/status/settle; `internal/claim` discovers campaigns from
    the billing preview and marketing banner. Files:
    `internal/offpeak`, `internal/claim`, `internal/server/async.go`.

</details>

## License

MIT — see [LICENSE](LICENSE).
