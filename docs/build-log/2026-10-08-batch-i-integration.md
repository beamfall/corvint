# 2026-10-08: Batch I integration (V1-0431, V1-0859, V1-1030, V1-0651, V1-0967, V1-1012)

## Intent

Integrate five finished, independently reviewed lanes as one batch branch,
`claude/batch-i-2026-10-08`, based on `origin/main` `388fb832` (PR #689, batch G). Each lane is
merged with `git merge --no-ff`, so its reviewed commits stay intact. V1-0431 first rode in batch H
and moved here when batch H exceeded the change-evidence binder's 256-obligation cap.

## Merges

| Order | Ticket | Lane head | Merge commit | Conflicts |
| --- | --- | --- | --- | --- |
| 1 | V1-0431 and V1-0859 orientation ranking | `aba8eacb` | `79f2c16a` | none |
| 2 | V1-1030 fixture maintenance | `5b1334f1` | `c875214d` | none |
| 3 | V1-0651 sandbox launcher refusal | `cff93aac` | `1930b03b` | none |
| 4 | V1-0967 dot-prefixed web specifiers | `c8790a97` | `74c422a3` | none |
| 5 | V1-1012 local-completion `ok` mirrors the exit | `e69b3143` | `df411661` | none |
| 6 | owner acceptances, decisions 0463-0465, analyzer pin | `e00156bc` | (direct commit) | none |

## Owner acceptance

The owner accepted the three lane requirements in chat on 2026-10-08 ("accept TCQ-V0-059 too",
"accept GPK-V0-082 too", "accept LCP-V0-017 too"). Decisions 0463 (`TCQ-V0-059`), 0464
(`GPK-V0-082`) and 0465 (`LCP-V0-017`) record that acceptance. Delivery stays experimental and no
ticket is completed by the decisions. Decisions 0456-0458 are held by PR #690 and 0459-0462 by
batch H.

## Analyzer pin

V1-0967 changes `internal/contextindex/webresolve.go`, so the analyzer input SHA in
`internal/contextindex/analyzer_schema_test.go` moved to `9e651047...`. The schema stays
`corvint-analyzer/112`: `WebImportResolver` is constructed only by `buildWebImportGraph`, whose
one caller is the query-time impact path (`impact.go`), so no stored index encoding changes. This
follows the SHA-only repin precedent of `aba8eacb`.

## Verification

Darwin, Go 1.27.1 (`GOTOOLCHAIN=local`), shared loaded host.

- Before the last three merges, `GOMAXPROCS=4 go test -p 2 -count=1 -timeout 30m` over the
  batch's selected packages: 53 `ok`; `internal/authoritystore` failed three Darwin tests with
  `protected-authority-unavailable`, a host condition tracked by V1-0778 and outside this batch.
  `interop/cem01-go` tests pass.
- After all merges: `go build ./...`; `go vet` over `internal/liveverify/mutate`,
  `internal/contextindex` and `cmd/corvint`; `GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m`
  over `internal/liveverify/mutate`, `internal/contextindex`, `internal/localcompletion` and
  `internal/diagnostic`: all four `ok`; `cmd/corvint` with `-run 'LocalCompletion|Dogfood|UseCase'` is `ok` (165s).
- `script/repin-use-case-receipts.sh` reports all 18 receipts current. The doc gates
  (`spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check`, `diagnostic-coverage-check`) and
  `go test ./internal/specindex` pass.

Independent review: Codex (`gpt-6-astra`, read-only) reported no findings over the first two
merges. A second Codex review of the last three merges and `e00156bc` found one P2: decision
0463 described the change as a verdict-classification fix; it now says only the error detail
changes, as `TCQ-V0-059` states. Each lane also had its own review with findings fixed.

NOT_RUN: `go test ./...`, `make gate`, and live macOS sandbox qualification for `TCQ-V0-059`.
