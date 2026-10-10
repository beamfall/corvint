# 2026-10-10 — PR CI vets test files for Windows (V1-1117, AFP-V0-042)

Two Windows-only test breaks reached `main` and were caught only by the nightly release gates:
V1-1092 (`prompt_fragments_test.go` lacked its build constraint) and the V1-0825 `undefined:
stubStageSleep` in a tagged test helper (fixed by PR beamfall/corvint#739). PR CI ran
`GOOS=windows go build ./...`, which never compiles `_test.go` files.

## Decision

Add one step to the existing `go-static` job rather than a new job: `GOOS=windows GOARCH=amd64
CGO_ENABLED=0 go vet ./...` on the root module and `interop/cem01-go`. `go-static` already runs
beside the shards and is required through `go-product`, so no ruleset or job-graph change is
needed. The step is skipped only on a pull request whose merge commit (checked out at
`fetch-depth: 2`) has the event base as first parent and changes no `.go`, `go.mod` or `go.sum`
path; any other case runs it. `GOOS=linux GOARCH=arm64` stays in `make cross-vet`.

## Evidence

- `actionlint .github/workflows/ci.yml`: clean. `make ci-least-privilege-check
  ci-least-privilege-test`: pass.
- Local fixture (step script extracted from `ci.yml`, run with `bash -e` in a scratch clone with
  simulated merge commits):
  - docs-only merge: exit 0, summary "Windows vet skipped"; same result in a `--depth 2` clone.
  - merge adding `cmd/corvint/zz_v1117_windows_test.go` (`//go:build windows`, calls an undefined
    function): native `go vet` and `GOOS=windows go build ./...` pass; the step exits 1 with
    `undefined: stubStageSleepV1117`.
  - base with PR #739's fixes, push event: exit 0.
  - current `main` (bf04e8a0) without #739: exit 1 on the two known breaks
    (`internal/tasks/dispatch/prompt_fragments_test.go`, `internal/tasks/journal/projection_order_test.go`),
    so this change must land after #739.
- Wall time, local Apple Silicon, `GOMAXPROCS=4`, empty `GOCACHE` after the native `go vet ./...`
  (the order `go-static` runs in, which also uses `cache: false`): 44.9 s for the Windows vet of the
  root module; 4.9 s when warm. Estimate on the 4-vCPU hosted runner: about 1–2 minutes added to
  `go-static`. `go-static` runs in parallel with the go-product shards, which take far longer, so
  the expected added PR wall time to `go-product` is about zero unless `go-static` becomes the
  longest leg. Hosted timing is NOT_OBSERVED until this change's own CI runs.

## Consent

`.github` changes need owner consent under decision 0390 (a `ci-control-plane` success status on
the reviewed head SHA). The owner consented in chat on 2026-10-10; the consent record is added by
the orchestrator.

Rollback: delete the step and the `fetch-depth: 2` line from `go-static`.
