# Local Model Gateway V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `LMG-V0`
Intent status: experimental, owner-directed (2026-11-01; V1-4021, V1-4031); the invariant-7 carve-out is pending owner decision (V1-4037)
Delivery status: experimental
Authoritative inputs: `../../AGENTS.md` invariants 4 and 7, `local-admin-console-v0.md` (the
loopback-only, operator-started precedent, decision 0081), native tickets V1-4021, V1-4031 to
V1-4033, V1-4035 and V1-4037.

## Agent digest
- Claim: A loopback-only, operator-started proxy for the Messages API can record per-request token usage and fold oversized Bash output deterministically.
- Status: experimental, owner-directed (2026-11-01; V1-4021, V1-4031); the invariant-7 carve-out is pending owner decision (V1-4037) / experimental
- Exists: `cmd/corvint-gateway` (`serve`, `report`) and `internal/modelgateway` (`Proxy`, `Folder`, usage ledger); tests named for each requirement.
- Blocked on: savings are unmeasured until the paired trial (V1-4032); acceptance of the carve-out (V1-4037).
- Read next: Requirements; Trust boundary and failure modes; Rollback.

## Intent and scope

Owner instruction, 2026-11-01, verbatim: "I was thinking more that we could learn what the gateway
does and build it". The gateway in question is Context Mode (contextmode.ai), a hosted proxy that
a client reaches through `ANTHROPIC_BASE_URL`. Its published mechanisms are tool-output folding,
a full-text archive the agent reads back, tool-schema slimming, cache-prefix placement and session
recall; its vendor-run paired trials report 10–29% lower cost on short Opus 5.5 tasks and 65.5% on
a 30-step session. Those figures are the vendor's, not Corvint evidence.

Corvint's version keeps the mechanism and drops the hosting: the operator starts it on their own
machine, it listens on loopback only, and the only outbound traffic is the request the agent was
already going to send, to the upstream the operator names. Affected user: an operator running
Claude Code (or another Messages API client that honours a base URL) who wants observed token
usage and smaller requests. Measurable job: per-request input, cache-read, cache-creation and
output token counts that today are `NOT_OBSERVED`, and fewer forwarded bytes on shell-heavy
sessions with unchanged task success.

## Requirements

- **LMG-V0-001:** (experimental, owner-directed, 2026-11-01; V1-4021) The gateway SHALL ship as
  its own executable, `corvint-gateway`, never linked into `corvint`, and SHALL run only for the
  lifetime of the foreground `serve` invocation the operator starts. It installs no service.
- **LMG-V0-002:** (experimental, owner-directed, 2026-11-01; V1-4021) `serve` SHALL bind a
  numeric loopback IP only (such as `127.0.0.1` or `[::1]`) and SHALL refuse any other `--addr`,
  including a name such as `localhost` whose resolution it does not control, before listening. The
  upstream SHALL be an `https` URL, or `http` only for a numeric loopback IP.
- **LMG-V0-003:** (experimental, owner-directed, 2026-11-01; V1-4021) The gateway SHALL forward
  every request to the upstream with the same method, escaped path, query, body and end-to-end
  headers (including `Accept-Encoding`), minus hop-by-hop headers and every header any
  `Connection` field names, and SHALL relay the status, headers and body back, flushing each chunk
  of a streamed response as it arrives. It SHALL NOT follow upstream redirects. With folding off,
  the forwarded request body SHALL equal the received body byte for byte; with folding on, only the
  bytes of each folded JSON string change. If the upstream body fails after the status is sent, the
  gateway SHALL abort the client connection rather than end the response as if complete.
- **LMG-V0-004:** (experimental, owner-directed, 2026-11-01; V1-4021) For each `POST` to a path
  ending in `/v1/messages`, the gateway SHALL append one `corvint-gateway-usage/0` JSON line to
  the ledger: time, path, status, model, whether it streamed, input, cache-read, cache-creation
  and output token counts taken from the response (`message_start` and `message_delta` events, or
  the JSON body's `usage`; an SSE event's `data:` lines are joined before decoding, and a gzip body
  is metered from a decoded copy while the client receives it unchanged), received and forwarded
  request bytes, fold count, duration, and a
  short SHA-256 key of the request's `metadata.user_id` when present. It SHALL NOT record headers,
  credentials, prompt text or tool output. A missing count stays absent, never zero-filled.
- **LMG-V0-005:** (experimental, owner-directed, 2026-11-01; V1-4031) Folding SHALL be a pure
  function of the tool result text and the folding options, so that a request re-sent with a
  longer history forwards byte-identical folded text for every earlier result and the prompt-cache
  prefix stays stable.
- **LMG-V0-006:** (experimental, owner-directed, 2026-11-01; V1-4031) The gateway SHALL fold only
  a `tool_result` whose `tool_use_id` names a `tool_use` block called `Bash` earlier in the same
  request, that is not `is_error`, and whose text (a string, or each `text` block) exceeds the
  threshold (default 8192 bytes). It SHALL NOT alter any other block, so `Read`, `Edit` and
  `Write` results, which edits depend on verbatim, pass unchanged.
- **LMG-V0-007:** (experimental, owner-directed, 2026-11-01; V1-4031) A fold SHALL first drop
  passing Go test noise (`=== RUN`, `=== PAUSE`, `=== CONT`, `--- PASS`, a bare `PASS`, `ok  ` and
  `[no test files]` package lines) and record how many lines it dropped; when the text still
  exceeds the threshold it SHALL keep a head window (default 4096 bytes) and a tail window
  (default 2048 bytes), both cut at line boundaries, or at a UTF-8 rune boundary when one line
  alone exceeds a window. Every folded text SHALL begin with one
  `[corvint-gateway: …]` marker naming the omitted line and byte counts and the archive path.
- **LMG-V0-008:** (experimental, owner-directed, 2026-11-01; V1-4031) Before forwarding a fold,
  the gateway SHALL store the exact original text at `ARCHIVE/<sha256>.txt` (directory 0700, file
  0600, written atomically and idempotently). An existing entry is reused only when it is a 0600
  regular file with exactly that text; anything else is replaced. If the archive write fails, it
  SHALL forward that result unfolded.
- **LMG-V0-009:** (experimental, owner-directed, 2026-11-01; V1-4021) A request body that is not
  valid JSON (including trailing data after the top-level value), or whose shape the folder does not
  recognise, SHALL be forwarded unchanged. A body over the size limit (default 64 MiB) SHALL be
  refused with HTTP 413 without contacting the upstream. A refused or failed request (413, or 502 on
  an upstream failure) SHALL still write a ledger line, and a ledger write failure SHALL be reported
  on stderr.
- **LMG-V0-010:** (experimental, owner-directed, 2026-11-01; V1-4035) `report` SHALL read the
  ledger and print request count, summed token counts, the cache-read share of input, and the
  forwarded-byte reduction, and SHALL write nothing.

## Non-goals

- No hosted service, account, telemetry upload, or third-party endpoint. The upstream defaults to
  `https://api.anthropic.com`.
- No tool injection, sandboxed code execution, memory recall or session resume in V0.
- No tool-schema slimming or cache-mark rewriting until V1-4033 and V1-4035 measure them.
- No savings claim before the paired trial (V1-4032) reports one.
- No repository-aware folding in V0: using Corvint's index and change context to choose what a fold
  keeps is the V1-4055 spike, measured against this blind baseline.
- Not part of the companion release bundle while experimental.

## Trust boundary and failure modes

| Failure | Handling |
|---|---|
| Gateway reachable from another host | LMG-V0-002 refuses non-loopback binds |
| Credentials or content leak into local state | LMG-V0-004 records counts only; the archive holds only folded originals, private to the user |
| A fold hides the line the agent needed | marker names the archive path; the agent `Read`s it; error results never fold (LMG-V0-006) |
| Fold changes between turns and breaks the cache | LMG-V0-005 purity, tested by re-folding |
| Unknown request shape | forwarded unchanged (LMG-V0-009) |
| Archive disk full or unwritable | that result forwards unfolded (LMG-V0-008) |
| Gateway stops mid-session | the client errors on connection refused; the operator unsets `ANTHROPIC_BASE_URL` |

The archive sits outside the project, so a Claude Code session may prompt before reading it; the
operator can add the archive directory to the session's allowed directories.

## Acceptance evidence and traceability

| Requirement | Evidence |
|---|---|
| LMG-V0-001, LMG-V0-002 | `TestLMGV0002RefusesNonLoopback`, `TestLMGV0002UpstreamScheme` |
| LMG-V0-003 | `TestLMGV0003PassthroughByteForByte`, `TestLMGV0003StreamsAndNoRedirect`, `TestLMGV0005FoldSplicesOnlyFoldedText`, `TestLMGV0009TruncatedStreamAborts` |
| LMG-V0-004 | `TestLMGV0004LedgerFromStream`, `TestLMGV0004LedgerFromJSON`, `TestLMGV0004MultiLineEventsAndGzip` |
| LMG-V0-005 to LMG-V0-008 | `TestLMGV0005FoldIsDeterministic`, `TestLMGV0005FoldSplicesOnlyFoldedText`, `TestLMGV0006OnlyBashResultsFold`, `TestLMGV0007GoTestNoiseAndWindows`, `TestLMGV0007LongLineCutOnRuneBoundary`, `TestLMGV0008ArchiveReplacesStaleEntry`, `TestLMGV0008ArchiveFailureForwardsUnfolded` |
| LMG-V0-009 | `TestLMGV0009MalformedBodyPassesThrough`, `TestLMGV0009OversizeRefused`, `TestLMGV0009TruncatedStreamAborts` |
| LMG-V0-010 | `TestLMGV0010Report` |

Live qualification (V1-4021): a Claude Code print-mode run with `ANTHROPIC_BASE_URL` pointing at
the gateway succeeds and writes ledger lines; recorded in the delivering build-log entry.

## Rollback

Stop the `serve` process and unset `ANTHROPIC_BASE_URL`; nothing else changed. Removing the
capability deletes `cmd/corvint-gateway`, `internal/modelgateway`, its command-tree snapshot and
this spec. The ledger and archive under the gateway state directory can be deleted by the operator.
