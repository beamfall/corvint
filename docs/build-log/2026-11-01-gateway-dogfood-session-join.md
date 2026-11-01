# 2026-11-01 gateway ledger joined to dogfood runs (experimental)

Base `origin/main` f6f21ec2eaa8753d1e2426e1b39d0d3ddf4e2a86. Ticket V1-4055, candidate 1. Spec
`docs/specs/local-model-gateway-v0.md`, LMG-V0-013 and LMG-V0-014.

## Decision

Corvint's dogfood evaluations report billed tokens as `NOT_OBSERVED`. The gateway ledger observes
them per request, but its `sessionKey` is a short hash of the whole `metadata.user_id`, which no
Corvint record carries. A capture against a loopback listener showed what Claude Code sends:
`metadata.user_id` is a JSON string holding `device_id`, `account_uuid` and `session_id`, and
`session_id` is the session id the Claude Code hooks receive. Corvint's Claude adapter already
names each dogfood enrollment by a domain-separated SHA-256 of that id (`claudeSessionHash`,
`cmd/corvint/host_adapter.go`). The ledger now records the same hash as `claudeSessionKey`, and
`report --session KEY` reports one run. The join is exact, not a time-window match.

The domain string is duplicated in `internal/modelgateway` rather than shared: the gateway must not
link `cmd/corvint`, and a test pins the key that `TestClaudeNativeDogfoodLifecycle` enrols under.
The direction stays one-way: the gateway reads nothing of Corvint's state, and Corvint commands
never read the ledger (invariant 4).

## Evidence

- `TestLMGV0013ClaudeSessionKey`, `TestLMGV0014FilterSession` and `TestLMGV0014ReportSession` pass,
  with the rest of `internal/modelgateway` and `cmd/corvint-gateway`, under `-race`. The command-tree
  snapshot is regenerated for the new `--session` flag.
- Live join against the real API, run from the owner's signed-in terminal panel: the PR build of
  `corvint-gateway serve` on `127.0.0.1:8797`, then `claude -p --session-id
  6f1c2d3e-4b5a-4c7d-8e9f-0a1b2c3d4e5f` in the Corvint checkout with `ANTHROPIC_BASE_URL` set,
  asked to echo the `--session-key` its Corvint session-start hook handed it. It answered
  `f914cbaa…be4402`, which equals the hash recomputed by hand, and both ledger lines (HTTP 200,
  `claude-opus-5-5`) carry that `claudeSessionKey`. `report --session` on that key printed 2 requests:
  1,204 uncached input, 37,153 cache-creation and 152 output tokens. An unknown key printed
  `NOT_OBSERVED`. The print-mode session was not enrolled, so that proved only the key match.
- Enrolled dogfood run through the gateway: a second print-mode session (`--session-id
  3c5e7a9b-…`) in a fresh linked worktree at `31f3134ba` took `c65aa8c2…691070` from its own
  session-start hook context, ran `corvint dogfood begin` with a plan for this spec and two checks,
  added the V1-4055 live-qualification paragraph to the spec, committed it (`ca1f68fbc`, carried here)
  and ran `dogfood verify` for `make spec-requirements-check` and `go vet` on both packages; both
  qualified with exit 0. `dogfood status` reports the enrollment active. `report --session` on that key
  printed 13 requests: 1,852 uncached input, 466,687 cache-read, 32,964 cache-creation and 3,556
  output tokens, a 93.1% cache-read share, and no cache breaks. The enrollment was then cancelled
  rather than sealed, because its one commit is carried in this change.
- That worktree's first session got no session key: its hook degraded with
  `dogfood-event-index-snapshot-stale`, and the next session received the key. Filed as V1-4182.
- Independent review (Codex, one round) returned one finding: an explicit empty `--session=` skipped
  validation and reported every session. `report` now validates any supplied `--session`, and the
  test covers the empty and uppercase forms.
