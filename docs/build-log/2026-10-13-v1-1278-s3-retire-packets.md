# 2026-10-13 cem/0.x retirement, stage S3: packets, examples, routes and specs (V1-1278)

Branch `claude/v1-1278-s3-retire-packets`, from `claude/v1-1278-s2b-core-removal` (`5a056d0a`).
Decision 0525 §1 step 7 (only CEM 1.0) and §6 (superseded records). Requirements
`CEM-V1-220..225` (new, proposed) in `docs/specs/cem-stable-v1.md`, under the owner's standing
auto-accept of lane requirements (2026-10-11). Accepted requirements whose subject is deleted are
marked retired in place rather than renumbered (`CEM-V1-PSR-001..004`, `FPK-V0-035`,
`CEM-PILOT-009`, `011..012`, `020..027`, `CEM-SM-011..017`, `CEM-GO-004..006`).

## Intent

S2b left the product with one wire, `cem/1.0`. S3 removes the frozen `cem/0.x` material that
only that retired wire needed: the protocol and conformance packets, the example recipes, the
external interop runner, the portable consumer's 0.x routes, the last Core remnants and the 0.x
specs. Git history keeps every deleted byte; nothing here changes `cem/1.0` behavior.

## Decisions

- Deleted `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`,
  `conformance/cem-0.2` and `docs/CEM-CI.md` (`CEM-V1-220`). `conformance/cem-0.2` is deleted
  with the rest because its only consumer was the deleted 0.2 verifier.
- Self-contained 1.0 packet (`CEM-V1-221`). The corrected 0.3 algorithm text is copied verbatim,
  with a provenance header, to `protocol/cem-1.0/stable/inherited/cem-0.3-algorithms.md` and
  listed in the packet `manifest.json`; no other packet file changes. The patch, base-tree and
  hunk-identity vectors the inherited mechanics are tested against move unchanged (`git mv`) to
  `interop/cem01-go/testdata/inherited-vectors/`, and the EOL policy exemption and
  `eol-policy-gate-v0.md` move with them.
- `interop/cem01-go` verifies `cem/1.0` only (`CEM-V1-222`). The `cem/0.1` default `verify`,
  `verify-03` and `ci` routes refuse: a named `cem/0.x` map exits 1 with `unsupported-spec` and a
  fix line naming `verify-stable-canonical`; every other non-1.0 invocation exits 2 with
  `invocation`. `ci.go`, `intoto.go`, `packet03_test.go`, `manifest_test.go` and their tests are
  deleted; the README is rewritten for 1.0.
- `examples/cem`, `script/cem-recipes_test.sh` and its gate member `cem-recipes-test`, and
  `tools/cem-interop-runner` are deleted with their Makefile, gate-ledger, CI shard,
  test-read-scope and unbounded-reader entries (`CEM-V1-223`). `tools/cem-citation-baseline`
  needed no change: S2a (`744e8fc4`) already removed its 0.x fallback.
- Core remnants (`CEM-V1-224`): `internal/dogfoodflow` always passes `--artifacts` to `cem status`
  (`stableStatusArguments`); the experimental `internal/frontiernextrepo` recompute refuses and
  returns an empty universe, and its unreachable helpers and `package_inputs_test.go` are deleted;
  `internal/cem/patch` errors say "outside the CEM patch grammar" instead of naming `cem/0.1`, with
  unchanged codes; comments point at the inherited algorithm text.
- Specs (`CEM-V1-225`). `cem-0.2-canonical-binding.md` and `cem-0.3-structural-mechanical.md` are
  retired by decision 0525 and superseded by `cem-stable-v1.md`, as the charter requires. Two
  more specs are retired the same way because their whole subject is deleted:
  `cem-go-portability-probe.md` (the 0.1 Go probe) and `cem-external-interop-v0.md`
  (`interop/cem-0.1` and the runner). Each keeps its body and the requirement IDs
  `REQUIREMENTS.tsv` traces; `INDEX.json` and `docs/specs/README.md` carry the supersession.
  `cem-pilot-kit.md` stays accepted/experimental with its 0.x rows retired. Amendment lines were
  added to `lexical-relevance-floor-v0.md`, `change-frontier-profile-1.md` and
  `change-witness-relation-v0.md`; `harness-authority-relation-v0.md` has no 0.x sentence and is
  unchanged; `external-frontier-obligations-v0.md` was amended by S2a, and only its README row is
  updated here.
- Docs: `docs/DOGFOOD.md` (`--spec` admits only `cem/1.0`; the frontier step describes the stable
  view), `README.md`, `ROADMAP.md`, `docs/INSTALL.md`, `docs/AUTOMATION.md`,
  `docs/ARCHITECTURE.md` (a decision 0525 note above the dated history),
  `docs/CHANGE-EVIDENCE-MAP.md` (marked historical), `docs/README.md`, `tests/README.md` and
  `proof-tour-v0.md`. `LICENSING.md` drops `docs/CEM-CI.md` and the deleted packet paths; every
  listed Apache-2.0 path exists and the boundary is not widened.
- Citations into deleted files are rewritten as prose pinned to commit `5a056d0a` (decisions 0094
  and 0098 and the retired traceability rows); one `interop/cem01-go` citation is repinned.

## Failure modes

- A link or line citation into a deleted packet fails `line-citations-check`; it is rewritten as
  prose pinned to `5a056d0a`.
- A traceability row that names a deleted test fails `traceability-tests-check`; such rows are
  rewritten as retirement notes, never marked `(PLANNED)`.
- A `cem/0.x` map handed to the portable consumer is refused, never verified.
- A manifest entry that drifts from the inherited file fails the packet digest check.
- A build of `interop/cem01-go` inside the checkout leaves an untracked `cem01-go` binary that is
  not gitignored; it was caught before commit and removed.

## Acceptance evidence

Pre-change `make dogfood-change BASE=dc6811176beb52ec2df509f2fd4cff3da0ffa916` returned 2 with
`coordination-time-impact NOT_PRODUCED unsupported-impact-range`, `prechange-query NOT_OBSERVED
agent-receipt-absent` and `prechange-impact NOT_OBSERVED agent-receipt-absent`, plus two
`sealed-cem-merged` notes (binds `50e99b32` and `59c66e6d`).

`git grep -l -E 'cem/0\.[123]' -- . ':!docs/decisions' ':!docs/build-log' ':!.corvint' ':!.taskman'`
lists 144 files, each classified:

| Class | Files | Why it stays |
| --- | --- | --- |
| Tests and test fixtures | 60 | Refusal cases that hand a retired map to a reader and assert `unsupported-spec` |
| Frozen packets, evidence and schemas | 30 | `protocol/cem-1.0` provenance and inherited text, frozen cli-parity, frontier-v0, lrf-v0, ocm-v0, perf-v0 and use-case receipts, `.agent-evidence`, and `docs/cem-0.1`/`cem-0.2`/`lrf-0` schemas that `protocol/cem-1.0/ALGORITHMS.md` and `LICENSING.md` cite |
| Specs | 23 | Retired spec bodies, dated history, amendments and current refusal text |
| Go non-test | 20 | Refusal messages (`wire.RetiredSpec`, cem01-go `retiredFix`), retired-route comments, `internal/lrf` frozen constants, the `sourceSpec` provenance enum and `help.go` refusal text |
| Guides and history documents | 10 | `docs/BUILD-LOG.md`, `docs/RELEASE-NOTES.md`, the historical `CHANGE-EVIDENCE-MAP.md`, removal notes in `ROADMAP.md`, `DOGFOOD.md`, `ARCHITECTURE.md`, `tests/README.md`, `interop/cem01-go/README.md`, and the retired trial arms in `tools/cem-trial/README.md` and `benchmarks/README.md` |
| Generated | 1 | `docs/specs/REQUIREMENTS.tsv` |

`git grep -n -E 'protocol/cem-0|interop/cem-0|conformance/cem-0|examples/cem\b'` finds only
history: frozen `protocol/cem-1.0` provenance (`HISTORY-124.json`, `MIGRATION.json`, the
inherited text and `ALGORITHMS.md:16`), retired spec bodies, retirement notes in
`cem-stable-v1.md`, `cem-pilot-kit.md` and `eol-policy-gate-v0.md`, decisions, build-log
entries, `docs/BUILD-LOG.md`, the frozen `docs/worklist.json` snapshot, `perf-v0` raw outputs,
sealed `.corvint/changes` maps, `.taskman` records and the frozen
`script/line-citation-legacy-documents.txt` list.

Verification commands and results are recorded in the pull request body.

## Limits

- `protocol/cem-1.0/stable/ALGORITHMS.md` (around line 489) still says the Git recipe is
  "inherited from the corrected public 0.3 packet"; the text is now in `inherited/`, and the
  wording is left to V1-1333 because the packet file is frozen.
- `docs/cem-0.1.schema.json` and `docs/cem-0.2.schema.json` are kept: the 1.0 packet text cites
  them.
- The relocated `maps/valid` vectors are kept for provenance; no test reads them now.
- `script/line-citation-legacy-documents.txt` is a frozen allowlist and still names the deleted
  conformance READMEs.
- `internal/lrf` keeps its frozen 0.x constants; `docs/DOGFOOD.md` keeps the sentence that CEM 0.1
  could cite only evidence present at its base revision as history; the `cem-pilot-kit.md` claim
  still describes experimental CEM 0.1 use.
- `harness-authority-relation-v0.md` has no 0.x mention and was not changed.
- No independent consumer of `cem/1.0` exists beyond the Corvint-owned `interop/cem01-go`.

## Rollback

Revert the V1-1278 S3 commit. The packets, examples, runner, portable 0.x routes and spec states
return unchanged; no committed map, seal, golden or task record depends on them.

Decision 0525 §1 step 7 is complete.
