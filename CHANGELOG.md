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

## [Unreleased]

### Added
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
