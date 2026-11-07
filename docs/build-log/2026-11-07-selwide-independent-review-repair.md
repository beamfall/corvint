# Selwide independent review repair

Review verdict: CHANGES_REQUIRED. Base:
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`. Scope: V1-5018,
AFP-V0-9012, AZU-V0-005 and AZU-V0-9921. The other selwide tickets,
V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014, are unchanged this round.

## Findings and decisions

P1: selection retained `nested/.helpers` and its importer, but CI narrowing
intersected them with wildcard discovery and silently removed the helper.
The sharded PR driver rejected that same selection as absent from its universe.
Both execution paths now add hidden packages named by the indexed selection
as explicit `go list` arguments alongside the wildcard. The Go tool remains the
runtime authority; a listing failure follows the existing refusal/fallback.
This reuses the session runner's explicit-package approach without a new wire
member or executable. `TestHiddenShardedExecution_AFPV09012` uses a real Git
fixture, the real planner and selector, and four driver shards. A changed hidden
helper selects itself and its visible importer; both failing tests must emit
Go test events and fail their owning shard. The CI shell regression requires
the explicit argument and retains the helper in selected and executed lists.

P2: the declared-output exclusion ran after `FilesIncluding` validated names.
Move the same exclusion into the acceptance predicate, before representability
validation. `.evidence/name\xff.go` is retained output under a declaration and
must not abort graph construction. The existing hidden-name test now covers
that combination on immutable sources and Unix disk, comparing graph digests.
Its existing assertions still require refusal for claimed invalid Go names and
invalid embed targets. No output is reclassified as compiled source.

Non-goals: no JSON, wire, diagnostic, exit-code, golden or core-freeze changes;
no broadened read-scope declarations or selection uncertainty allowlist.
Rollback: revert this repair's source/spec/test commit; the earlier independent
review findings then remain unresolved. No persistent migration is required.

## Verification

All raw logs stay in `/home/russell/lanes/selwide-evidence/`; digests and final
whole-package results follow below. Go tests run only through `script/heavy.sh`,
one package per command, with `TMPDIR=/home/russell/lanes/selwide.tmp`, local Go
1.27.3, and no exported Git identity. The required affected receipt selects all
four touched Go packages and retains scope UNKNOWN (71 unknown entries).
The installed planner reports itself stale; this receipt is selection guidance,
not current-tree semantic qualification. The new driver regression builds and
uses the current tree's planner and selector.

Pre-change capture is reused. Independent review and host-wide verification
remain orchestrator-owned. No Tasks writes, publication, model-backed review,
whole-repository gate or seal is run. Windows/Darwin execution is NOT_RUN on
this scoped Linux round. Final CEM checking is structural evidence and does not
close native tickets or remove earlier host-validation limitations.

| Evidence file | SHA-256 | Result |
|---|---|---|
| `review-affected-whole.log` | `3628d76011216a3e9473f5ebbfe0e581d50d6814d968ca36c0f063240537ee57` | github.com/Beamfall/corvint/internal/liveverify/affected	137.581s |
| `review-golang-whole.log` | `dec6b2d61915a9f10f83ceb8d8b2eefddd557bb78a9a038c57eeeb08a4d38ecf` | github.com/Beamfall/corvint/internal/liveverify/affected/golang	8.966s |
| `review-forward-whole.log` | `2e2d98117effa2819776f8b0f0d05c59af2cef300325fb090b1962c11493f6d9` | github.com/Beamfall/corvint/tools/ci-forward-row	64.301s |
| `review-pr-driver-whole.log` | `cce457dd9cbc5c4466ca777fe02666a219304b11d7bf3649b31a207daa63cce0` | github.com/Beamfall/corvint/tools/corvint-pr-tests	116.059s |
| `review-sharded-focused.log` | `4df30424a8fdfc004bcb9cf72d967f889fead4c55ccdb84f2ae2a796139e02d5` | github.com/Beamfall/corvint/tools/corvint-pr-tests	20.823s |
| `review-spec-citations-check.log` | `38df37c486907ca0c4de4ebbb94b35bf11f48ea0da84c755340e234f484fed3b` | github.com/Beamfall/corvint/internal/specindex	1.037s |
| `review-affected.json` | `7f051fd8a811a2e0567b91635fa1596fa9bc9dca806a6aa6c8676afa57e4d639` | UNKNOWN retained |

Observed friction: shared heavy admission delayed the Go-provider suite by
169.2 s; `script/heavy.sh` also reported /tmp free=2G below its advisory 3G
floor on some commands. No lock-directory, admission or disk overrides were
used. Spec regeneration reported no changes; citation repinning moved none.
