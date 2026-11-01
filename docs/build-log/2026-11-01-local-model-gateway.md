# 2026-11-01 local model gateway (experimental)

Base `origin/main` 6d4713fb8b5d05201ea0c26483e5f0e230ee4c78. Tickets V1-4021 (proxy and usage
ledger) and V1-4031 (deterministic Bash folding and archive). Spec
`docs/specs/local-model-gateway-v0.md`, LMG-V0-001 to LMG-V0-010.

## Decision

The owner asked what Corvint can take from Context Mode (contextmode.ai), then asked to "learn what
the gateway does and build it" rather than route traffic through the hosted service. The
mechanisms copied are tool-output folding with a readable full-text archive and a per-request
token ledger. Hosting, accounts, schema slimming, cache-mark rewriting, recall and tool injection
are left out (non-goals; V1-4033 and V1-4035 measure the next two). Folding is a pure function of
the text so the prompt-cache prefix stays byte-stable from turn to turn, and it touches only
non-error `Bash` results, because edits depend on `Read` results staying verbatim.

`corvint-gateway` is its own experimental binary, outside the release bundle and never linked into
`corvint`. It follows the decision 0081 console precedent: operator-started, foreground,
loopback-only. Whether invariant 7 accepts it is the owner's call, V1-4037. Until then the spec stays
experimental and makes no savings claim. Savings are measured by the paired trial, V1-4032.

The owner then asked to use Corvint's repository knowledge to go further. That is V1-4055, a spike
that measures each Corvint-aware fold against this blind baseline. Its highest-value item is joining
ledger rows to dogfood runs, so Corvint's own evaluations get observed billed tokens instead of
`NOT_OBSERVED`.

## Evidence

- Focused tests: `internal/modelgateway` (13 tests, one per spec row) and `cmd/corvint-gateway`
  (command tree) pass. So do `internal/capcatalog`, `internal/specindex`, `tools/capabilities-check`,
  `tools/docs-ci-plan`, and the `cmd/corvint` `TestCAPV0` catalog set. `corvint affected` returned
  scope UNKNOWN (128 selected, 88 unknown; the change touches the shared generated spec and
  read-scope files), so the remaining selection is left to batch CI.
- `TestLMGV0009OversizeRefused` first took 30 s on the WSL host because dialing `127.0.0.1:1`
  times out there instead of being refused. The test now uses a closed httptest server.
- End-to-end check of the built binary against a loopback fake upstream: HTTP 200 and an
  unchanged SSE body. The ledger row has input 21, cache-read 9000, cache-creation 300 and
  output 42. A 28,956-byte request with 1,201 passing-test lines was forwarded as 650 bytes, with
  `FINAL-LINE-42` kept and the original archived at mode 0600. `report` printed a 96.6% cache-read
  share and 97.8% fewer forwarded bytes. A `0.0.0.0` bind was refused.
- During that check, an upstream that was not ready yet produced a 502 with no operator-visible
  cause. `serve` now prints each upstream transport failure to stderr.
- Independent review (Codex, one round) returned 12 findings, all fixed with tests:
  - The gateway trusted `localhost` by name; it now binds and accepts http upstreams only on
    numeric loopback IPs.
  - Folding re-encoded the whole request, which sorted keys and could move the cached prefix; it
    now splices only the folded string bytes. The decoder also rejects trailing `]` or `}`.
  - Forwarding: escaped paths (`%2F`) are kept, every `Connection` header value is honoured,
    `Accept-Encoding` is relayed (gzip is metered from a decoded copy), and a truncated upstream
    stream aborts the client connection.
  - Metering: SSE `data:` lines are assembled per event; a 413 writes a ledger line; ledger write
    failures go to stderr.
  - Folding: stale or foreign archive entries are replaced, long lines are cut on rune
    boundaries, and splitting no longer allocates a slice per line.
- Live qualification against the real API: `NOT_RUN`. A `claude -p` started inside this desktop
  session reports "Not logged in": the host manages that session's auth, and it does not pass to a
  child CLI. The owner runs it from their own terminal:
  `corvint-gateway serve`, then `ANTHROPIC_BASE_URL=http://127.0.0.1:8787 claude -p "..."`, then
  `corvint-gateway report`.
