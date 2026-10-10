# Decision 0496 — Windows test vet in PR CI (AFP-V0-042) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept AFP-V0-042"). The `.github` change itself has separate owner consent under decision
0390 ("I consent to V1-1117 under decision 0390", 2026-10-10); the binding form of that consent
remains the admin-posted `ci-control-plane` status on the reviewed head SHA.

## Context

Two Windows cross-vet breaks in test files reached main within two days (V1-1092 and the
`stubStageSleep` helper from V1-0825) and were caught only by the nightly release-gates run. PR CI
built for Windows but never type-checked test files. V1-1117 proposed `AFP-V0-042` in
`docs/specs/affected-plan-v0.md`.

## Decision

The owner accepts `AFP-V0-042` as written: the `go-static` job runs
`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...` on the root module and on
`interop/cem01-go`, skipped only on a pull request whose merge commit provably changes no `.go`,
`go.mod` or `go.sum` path; any doubt runs the vet.

## Limits

This settles intent only. Hosted timing and hosted behaviour are `NOT_OBSERVED` until the
change's own CI runs; linux/arm64 vet stays in `make cross-vet` and the nightly run. `make gate`
is `NOT_RUN`.

## Rollback

Revert this decision and return `AFP-V0-042` to proposed. To withdraw the behaviour, delete the
step and the `fetch-depth: 2` line from `go-static` and regenerate `docs/specs/REQUIREMENTS.tsv`.
