# Changelog

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [SemVer](https://semver.org/).

## [fork] - 2026-11-07

### Fixed
- Plan JWT carrying no `exp` claim is no longer stamped with a 24h `expiresAt`,
  which previously made `accounts.Usable()` drop a valid non-expiring account
  a day after login.

### Added
- Windows start/stop scripts (`.cmd` / `.vbs`) for Task Scheduler use.

### Changed
- `.gitignore` now blocks `logs/`, `data/`, JWT files and `*.bak-*` backups.

## [0.1.5-mod] - 2026-11-09

### Added
- Dashboard claim now runs the real Aliyun captcha widget when the gateway
  asks for it: `GET /v1/claim/verification-config` exposes the app's
  `data.configs.captcha` (region/prefix/scene id), the page loads the same
  `AliyunCaptcha.js`, and `POST /v1/claim` forwards the solved
  `captchaVerifyParam` as `X-Aliyun-Captcha-Verify-Param`. The user solves
  the challenge; nothing is bypassed. Fallback stays: claim from the app.

## [0.1.4-mod] - 2026-11-09

### Removed
- The native companion pet window and the `/overlay/pet` presentation mode,
  including their tray toggles and position file. The companion keeps the tray,
  autostart, update check, and bridge supervision.

### Fixed
- Claim discovery derives `X-Device-Mid` from the active account when no
  environment override is set, so marketing/banner campaigns remain visible
  after a direct binary launch.
- Identity client version default raised to 3.17.2: the gateway only serves
  its current marketing/banner campaigns to that app version, so 3.16.3 saw
  no claimable plans.
- Billing claim sends the active account's device mid and reports the
  gateway's Aliyun captcha (`3007`) as an explicit "claim in the app" state
  (`perlu-app` in history) instead of retrying; the scheduler backs off six
  hours because the bridge cannot solve that captcha.

### Added
- `GET /v1/accounts/pool` (45 s cache, `?force=1`): per-account status plus
  per-model plan grants; the dashboard accounts page renders it as a pool
  table with activate/remove actions.
- Local sharing board at `/dashboard#/sharing`: title, message, link, image
  attachment up to 120 KB, author name, one like toggle per browser, comments,
  and newest/top sorting. Persisted in `data/sharing.json` (0600) and backed by
  `GET/POST /v1/sharing`, `POST /v1/sharing/like`,
  `POST /v1/sharing/comment` and `DELETE /v1/sharing`.
- Bilingual dashboard: a topbar **EN/ID** toggle switches the `/dashboard`
  console (overview, accounts, chat, inspector, usage, network, claim,
  settings) to English. Indonesian remains the default; model ids, chat
  message bodies and code are left untouched.
- Console SPA at `/dashboard` (hash router, dark/light, embedded CSS/JS, no
  CDN): overview, inspector, usage, network, claim and read-only settings.
  The previous tabbed page moved to `/classic`; `/chat` is the new chat
  workspace with local conversations, streaming, stop/regenerate and
  import/export.
- `GET /v1/events`: SSE snapshot (usage, network, throttled health) consumed
  with `fetch` so the bridge token can be sent as a header.
- `GET /v1/settings`: effective configuration summary with capture/claim/env
  state; secrets are reported as presence only. `GET /v1/claim/history`
  returns this process's claim attempts (max 50).
- Dashboard account manager: the overview lists every stored account with
  expiry/reauth/cooldown badges, switches the active one and removes entries.
  `POST /v1/accounts/login` starts a device flow from the browser,
  `GET /v1/accounts/login/status` polls it and `POST
  /v1/accounts/login/cancel` stops it; the CLI and dashboard share
  `accounts.CompleteDeviceFlow`.
- Dashboard agent integration: `GET /v1/integration` returns the downstream
  base URL, bearer key/source, default model and endpoint map;
  `POST`/`DELETE /v1/integration/key` generate, rotate or remove a persisted
  `sk-…` key in `data/agent-key.json` (0600). `ZCODE_BRIDGE_TOKEN` still wins
  and is read-only from the UI. Settings shows the key, a copy button and a
  ready curl example.
- Native layered pet in `zcode-companion`: per-pixel alpha, always-on-top,
  click-through by default, never activates, mood follows `/v1/usage`
  (offline/sleep/idle/thinking/happy/turbo), position persisted, tray toggles
  for visibility and click-through.
- `internal/sqlite`: a dependency-free, read-only SQLite reader (file header,
  WAL with checksum-validated committed prefix, table b-trees, overflow
  payloads, record decoding, column names parsed from `CREATE TABLE`). Reads
  the 127 MB ZCode database in ~25 ms.
- `internal/desktop`: ZCode CLI usage aggregation — today/all-time totals,
  average and peak t/s, 7-day high (≥300 tokens and ≥1 s) and 7-day average,
  per-model stats, 15 m/1 h/6 h/24 h buckets (10 s/40 s/4 min/16 min), in-flight
  sessions from `message` rows without a completion stamp.
- `/v1/usage` now carries a separate `desktop` object (path, availability,
  snapshot); `ZCODE_DB` and `ZCODE_DESKTOP` control it.
- MCP endpoint: `POST /mcp` (JSON-RPC 2.0 `initialize`/`tools/list`/
  `tools/call`, batch and notifications included) aggregating every upstream
  server listed in `ZCODE_MCP_SERVERS`, tools flattened as `{server}__{tool}`;
  `GET /mcp` serves a catalog page plus the matching client config.
- `internal/mcp`: dependency-free MCP client — stdio children and streamable
  HTTP (JSON or SSE, `Mcp-Session-Id`), pagination, per-server failure
  reporting, deterministic namespacing.
- Optional automatic MCP loop (`ZCODE_MCP=1`) for non-streaming Anthropic and
  OpenAI chat calls: inject namespaced tools, execute every returned MCP
  `tool_use`, append the matching `tool_result`, and continue inside the same
  plan lane/session until the model answers. Client-supplied tools retain the
  existing caller-owned round-trip; `ZCODE_MCP_MAX_ROUNDS` bounds recursion.
- Dashboard **Penggunaan**: "Aplikasi ZCode" section with the same numbers,
  a range switch (15 mnt / 1 jam / 6 jam / 24 jam), a curve/per-model switch,
  and an explicit note that these rows come from `db.sqlite`, not from bridge
  traffic.
- Dashboard speed panel: three native gauges (last request t/s, today's
  average, cache-hit rate), a 15-minute rolling chart at 10-second resolution,
  and exact token counts instead of abbreviated ones. `GET /overlay` serves the
  same numbers as a compact one-second-refresh page for a floating window;
  `overlay.cmd` + `overlay.ps1` open it as a pinned Edge app window.
- Dashboard playground: send a prompt to `/v1/chat/completions` (streaming or
  not) and see the answer, reasoning block, model and latency.
- Usage metering: `GET /v1/usage` plus a **Penggunaan** dashboard tab. Each
  request that reports an upstream usage block is recorded (model, serving
  account, input/output/cache-read tokens, first-token latency, generation
  time, tokens/second) to `data/usage-YYYYMMDD.jsonl` (`ZCODE_USAGE_DIR`
  overrides the directory) and aggregated into today/all-time totals with a
  per-model breakdown. The tab shows a focal average t/s, the per-model table
  and a 60-request inline-SVG chart colour-coded by speed tier. Streaming
  requests measure from first token to completion; non-streaming ones are
  counted but kept out of the t/s average.
- `POST /v1/responses`: OpenAI Responses wire format for Codex CLI
  (`wire_api = "responses"`). Requests reuse the chat -> Anthropic translation;
  replies are rendered as Responses output items for both non-streaming and
  the streaming event lifecycle (created, output_item/content_part added,
  output_text deltas/done, function-call argument deltas, item done,
  completed). Custom (`exec`, `apply_patch`) and namespace tools are
  downgraded upstream and restored in the reply; hosted tools are dropped.
- Request inspector: opt-in capture ring (`ZCODE_CAPTURE=1`,
  `ZCODE_CAPTURE_SIZE`, `ZCODE_CAPTURE_BODY_MAX`) recording every inference
  exchange with account attribution, upstream attempt/rotation events,
  duration and first-token latency; `GET /inspect` (list, detail by `id`,
  `full=1` export) and `DELETE /inspect`; dashboard "Inspector" tab with
  method/status filters, path search and a detail panel. Secrets are
  redacted before storage; the endpoints honour `ZCODE_BRIDGE_TOKEN`.
- Dashboard speed panel parity with the speed panel: three rings (last request,
  highest ever, 7-day token-weighted average), four selectable ranges
  (1 h / 6 h / 24 h / 7 d) and a per-model trend chart; module visibility
  settings persisted in `localStorage`, and a sticky status bar showing lane,
  account, request count, speed and network.
- `GET /v1/net` plus `internal/netmon`: Windows interface counters
  (`GetIfTable` mapped to real adapters through `GetAdaptersAddresses`),
  loopback TCP attribution for the bridge port, and the ZCode client's IO/CPU
  deltas (`GetProcessIoCounters`/`GetProcessTimes`, opt-in
  `ZCODE_PROCESS_CALIBRATION`). Non-Windows hosts report `supported: false`;
  a missing client hides the process block. Dashboard card refreshes every
  three seconds.
- Overlay presentation modes: `/overlay` (card), `/overlay/gauge` (and
  `/overlay/mini`), `/overlay/pet` and `/overlay/capsule`; the last choice is
  remembered in `localStorage`.
- `cmd/zcode-companion`: Windows tray shell (stdlib + Win32 only) that
  supervises `zcode-bridge.exe` (spawn when `/health` is down, restart after a
  crash), toggles login autostart through the per-user `Run` key, opens the
  dashboard/overlay, and checks the latest GitHub release. Flags:
  `-diagnose`, `-no-tray`, `-autostart=on|off`, `-check-update`;
  `companion.cmd` launches the tray.
- `zcode-bridge tui`: ANSI terminal panel with usage/network panes, raw console
  input on Windows and a line-input fallback elsewhere.
- Idle/off-peak channel (`ZCODE_ASYNC_ENABLED=1`): `/async/v1/messages` and
  `/async/v1/chat/completions` take an upstream queue ticket
  (`/api/v1/off-peak/ticket/*`), emit SSE keepalives while queued, retake an
  expired ticket up to `ZCODE_ASYNC_MAX_RETRIES`, then forward the model
  stream through the off-peak lane; `/async/v1/health` probes availability.
  The ticket is bound to the request context so shaping, translation, capture
  and metering reuse the normal path.
- Manual plan claim: `zcode-bridge claim list|now`, `GET`/`POST /v1/claim` and
  a dashboard card. Campaigns come from the billing preview plus marketing
  banner buttons; the scheduler (`ZCODE_CLAIM_ENABLED`, `ZCODE_CLAIM_AUTO`)
  runs every five minutes with a ten-minute failure cooldown and leaves
  captcha-gated campaigns to the desktop app.
- Account/plan switch: `GET /v1/accounts` and `POST /v1/accounts/switch`
  (`{"id":…}`) plus a Status-tab selector that changes the rotation head.
- Inspector persistence and replay: redacted exchanges append to
  `data/capture.jsonl` (reloaded up to the ring size, compacted past 8 MB),
  `GET /inspect?format=har` exports HAR 1.2, and
  `POST /inspect/replay?id=N` re-issues a captured request in-process
  (truncated bodies refused); dashboard gains "Ekspor HAR" and "Putar ulang".
- Optional `config.yaml` (`ZCODE_CONFIG`) mapped onto environment variables
  with env precedence, `config.example.yaml`, non-root `Dockerfile`,
  `docker-compose.yml`, and GitHub release checking (`internal/update`,
  `zcode-bridge update-check`, startup notice; `ZCODE_UPDATE_CHECK=off`
  disables it).

## [0.1.0] - 2026-11-01

First public release.

### Added

- `POST /v1/messages` (Anthropic) pass-through into the ZCode plan lane.
- `POST /v1/chat/completions` (OpenAI) with bidirectional translation,
  including SSE streaming (`chat.completion.chunk`, `reasoning_content`,
  remapped `tool_calls` indexes, `[DONE]`) and tool round-trips.
- `GET /v1/models` with live entitlement merging across plans and a static
  fallback when billing is unreachable.
- `GET /v1/quota` per-entitlement grant rows, balance-first with a
  `billing/current` fallback on bare auth headers.
- `GET /health` reporting lane, credential ring and device-mid state without
  exposing secrets.
- `login` device flow, account ring store (`0600`, multi-account rotation,
  cooldowns, `--label` / `--list` / `--set-default` / `--remove`), and a
  15-minute background refresh for tokens expiring within two hours.
- Three-block identity shaping from the embedded identity asset, derived
  per-conversation session ids, and lowercase model-id normalization.
- Serialized upstream lane with exponential backoff for transient
  `3008`/`3009` concurrency errors.
- Optional `ZCODE_BRIDGE_TOKEN` gate and opt-in `ZCODE_CORS_ORIGIN`.
- Prebuilt binaries for linux/darwin/windows on amd64/arm64.

[Unreleased]: https://github.com/azrinaldi/zcode-bridge/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/azrinaldi/zcode-bridge/releases/tag/v0.1.0
