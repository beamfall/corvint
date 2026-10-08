# 2026-10-08: Batch H integration (V1-1026, V1-1027, V1-1028, V1-1029)

## Intent

Integrate four finished, independently reviewed lanes as one batch branch,
`claude/batch-h-2026-10-08`, based on `origin/main` `a5527cf4` and then brought up to `origin/main`
`388fb832` (PR #689, batch G). Each lane is merged with `git merge --no-ff`, so its reviewed commits
stay intact. The batch fixes GitHub issues #685 (V1-1027), #686 (V1-1028) and #687 (V1-1029).

V1-0431 was first merged here too, but the batch then exceeded the 256-obligation aggregate cap of
the change-evidence binder (314 obligations across five intent specs). It was taken out and moves
to batch I with its own specs; the rollback branch `batch-h-with-0431` (`3df23a2a`) keeps that
first integration.

## Merges

| Order | Ticket | Lane head | Merge commit | Conflicts |
| --- | --- | --- | --- | --- |
| 1 | V1-1026 milestone policy | `415413e7` | `fdd20f3c` | none |
| 2 | V1-1027 injected state names (#685) | `3f9c58a2` | `f2ca1570` | `INDEX.json`, `README.md`, `REQUIREMENTS.tsv` (generated) |
| 3 | V1-1028 keep-reporters qualification (#686) | `c9348a8d` | `795a9f26` | `INDEX.json`, `README.md`, `REQUIREMENTS.tsv` (generated) |
| 4 | V1-1029 repository-qualified know-how (#687) | `86e119f6` | `97fcc223` | `INDEX.json`, `README.md`, `REQUIREMENTS.tsv` (generated) |
| 5 | owner acceptances, decisions 0459-0462 | `d1d27884` | (direct commit) | none |
| 6 | `origin/main` `388fb832` (batch G) | - | `1bd68c0a` | leases spec, `INDEX.json`, `README.md`, `REQUIREMENTS.tsv` |

No requirement was renumbered: batch G holds `CAL-V0-194`, and this batch's `CAL-V0-195..196`,
`AMAP-V0-021..023`, `PWP-V0-014..018` and `KHN-V0-024..027` are unused on main.

## Owner acceptance

The owner accepted the batch's requirements in chat on 2026-10-08 ("accept the specs for batch H
too"). Decisions 0459 (`CAL-V0-195..196`), 0460 (`AMAP-V0-021..023`), 0461 (`PWP-V0-014..018`) and
0462 (`KHN-V0-024..027`) record that acceptance. Delivery stays experimental and no ticket is
completed by the decisions. Decisions 0456-0458 are reserved by PR #690.

## Conflict resolutions

- `docs/specs/corvint-tasks-agent-leases-v0.md`, `INDEX.json`, `README.md`: the delivery status keeps
  every clause in requirement order (V1-1021 `CAL-V0-192..193`, V1-0672 `CAL-V0-194`, V1-1026
  `CAL-V0-195..196`); the V1-1026 clause now reads accepted (decision 0459). The traceability table
  keeps main's `CAL-V0-194` row before this batch's `CAL-V0-195` and `CAL-V0-196` rows. Lines that
  differed only in trailing whitespace take main's side.
- `docs/specs/REQUIREMENTS.tsv`: generated; regenerated with `make -s spec-requirements` after
  staging.

## Verification

Darwin, Go 1.27.1 (`GOTOOLCHAIN=local`), shared loaded host.

- Before the main merge, `-race -p 2 -count=1 -timeout 30m` over the 25 touched packages: 24 `ok`;
  `internal/tasks/store` hit the 30m timeout while running `TestCALV0071_MultiRepositoryProgramFakeHost`
  after about 8s in that test, with every earlier test passing. The package takes about 1431s
  without the race detector, so the timeout is attributed to the package's length under race on a
  loaded host (inferred, not proven). It was rerun alone under race with `-timeout 45m` and timed out again
  at 2700s with no individual test failing; it then passed without the race detector
  (`GOMAXPROCS=3 -count=1 -timeout 45m`, `ok` in 1520s). The race run of the lane's own KHN and
  claim store tests passed in the V1-1029 lane, and CI runs the package under race on Linux. The
  package's length is tracked by the existing ticket on `internal/tasks/store` tripping the 30m
  hang detector.
- After the main merge: `go build ./...` and `go vet` over `internal/tasks/...`, `internal/taskman`,
  `internal/appmap`, `internal/jstestprovider`, `cmd/corvint-js-test-provider` and
  `internal/testvaliditydoc` are clean; `GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m` over
  `internal/tasks/cli`, `internal/tasks/intent`, `internal/tasks/wire`, `internal/tasks/dispatch` and
  `internal/taskman`: all five `ok` (cli 383s, intent 56s, wire 0.3s, dispatch 144s, taskman 19s).
- The doc gates (`spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check`,
  `diagnostic-coverage-check`) and `go test ./internal/specindex` pass.

Independent review: Codex (`gpt-6-astra`, read-only) over the batch reported no findings; it could
not run Go tests in its sandbox. Each lane also had its own Codex review with P0 to P2 findings fixed.

NOT_RUN: `go test ./...`, `make gate`, the opt-in live Playwright keep-reporters qualification
(`PWP-V0-015..016` stay unpromoted until a live `qualified` record), adopter-scale application-map
qualification, and durable multi-repository know-how qualification.
