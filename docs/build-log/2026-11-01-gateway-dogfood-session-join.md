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
- A live join on a real dogfood run: `NOT_RUN`. A Claude Code session started inside this desktop
  session cannot sign in. The owner runs it by starting `corvint-gateway serve`, opening a Claude Code
  session with `ANTHROPIC_BASE_URL=http://127.0.0.1:8787`, doing a dogfood change in it, then
  running `corvint-gateway report --session KEY` with the key the session's start-of-session hook
  prints in `corvint dogfood status --session-key KEY`.
- Independent review (Codex, one round) returned one finding: an explicit empty `--session=` skipped
  validation and reported every session. `report` now validates any supplied `--session`, and the
  test covers the empty and uppercase forms.
