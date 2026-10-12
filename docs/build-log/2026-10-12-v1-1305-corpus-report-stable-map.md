# 2026-10-12 Corpus projection and OCM report over a cem/1.0 map (V1-1305)

Branch `claude/v1-1305-corpus-ocm-stable` from `origin/main` 9deae4838. Ticket V1-1305 has two
acceptance criteria:

1. `docs corpus cem` over a cem/1.0 map yields the stable projection or a closed, non-internal
   refusal naming the fix.
2. `cem report --ocm` over a cem/1.0 map produces the OCM report through the stable route.

This lane adds no requirement ID.

## Intent

Two commands reached a `cem/1.0` map and failed badly:

- `docs corpus cem` sent the map to the 0.x parser and surfaced its error as `internal-error`.
- `cem report --ocm` refused every OCM.

Both now complete. The 0.x readers remain in place; V1-1278 removes them.

## Decisions

- **Corpus.** `docs corpus cem` keeps refusing a `cem/1.0` map, as `CEM-V1-163` requires. The
  refusal now has the closed code `unsupported-spec`. Its message names
  `corvint docs corpus cem-stable` and the three binding flags it needs (`--expected-base`,
  `--target`, `--stable-artifact-root`). Any other coded 0.x parse refusal keeps its code instead
  of becoming `internal-error`.
  - Why refuse instead of projecting: the 0.x operation has no binding flags. Its flag table lives
    in `cmd/corvint/docs_corpus_help.go`, which is outside this lane.
- **Report.**
  - `workflow.Session` gets a `StableOCMReader`, set by `SetStableOCMReader`. `cem/cli` installs
    the package variable `StableOCM`. `cmd/corvint/ocm.go` sets that variable from `init()` to
    `readStableOCMReview`, which calls `lrfrepo.VerifyStableUniverse`. This is the same pattern
    as `GitNotes` and `Export`; `main.go` is reserved by V1-1301.
  - The reader receives the OCM bytes Core read, bounded to 1 MiB inside the session root. Core
    rebinds `mapSha256` and `cemMapSha256` to those bytes.
  - The worklist rows, verification and copied OCM fields match the 0.x reader's shape.
  - `applyStableOCM` joins obligations onto the stable hunk rows and re-digests the record set.
  - The Markdown reuses the 0.x OCM summary, obligation and unmapped-obligation writers, now
    factored out of `renderReviewProjection`. The 0.x output is unchanged.
  - A report without `--ocm` is byte-identical to before.
- **Imports.** `cmd/corvint/ocm.go` gained one import line, `cemcli`. That shifts the unanchored
  legacy citation `cmd/corvint/ocm.go:248` in `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md`
  by one line: it now lands on `parseOCMFlags`, one line above the default-paths line it meant.
  The citation gate still passes because the line is not blank. Repinning the citation is outside
  this lane.

## Failure modes

| Case | Result |
| --- | --- |
| `docs corpus cem` over `cem/1.0` | Exit 2, `unsupported-spec`, empty stdout |
| `--ocm` naming an absent file | `map-unavailable` |
| `--ocm` naming a path outside the root | `invalid-arguments` (`path-traversal`) |
| Empty `--ocm` | `invalid-arguments` |
| Stable admission refusal | Exit 1; `ocm.valid` false, state `invalid`, one issue carrying the closed code, in the 0.x failure verification shape |
| No stable reader installed (library or MCP callers) | `NOT_PRODUCED` `ocm-verifier-unavailable`, not ok |
| Admission error with no code | Returned as an error, so it surfaces as `internal-error` |

Uncoded admission errors are not expected. Every `VerifyStableUniverse` refusal path observed is
coded.

## Acceptance evidence

- `cmd/corvint` `TestV11305CorpusCEMRefusesStableMapWithFix` (criterion 1). It checks the closed
  code, the named fix and flags, the absence of `internal-error`, and that the named fix succeeds.
- `cmd/corvint` `TestV11305StableReportOCM` (criterion 2):
  - An OCM still bound to the legacy map is invalid with `cem-map-digest-mismatch`.
  - A rebound OCM is valid, with structural coverage 2/1/1, worklist rows, the hunk join, the
    unmapped `TM-V0-009`, and the published Markdown OCM lines.
- `internal/cem/cli` `TestV11305StableReportOCMWithoutVerifier` and the updated
  `TestCoreStableReportRefusals` rows.
- Focused package runs are listed in the PR body.

## Independent review

One Codex round (`codex exec -s read-only`) returned CONCERNS with one confirmed finding: the
invalid stable OCM verification dropped the 0.x failure fields `spec`, `targetRevision`,
`structuralCoverage` and `testExecution`. `stableOCMFailure` now mirrors the 0.x failure shape,
including the short form for `invalid-path` and `repository-object-unavailable`. The legacy-bound
case in `TestV11305StableReportOCM` asserts those fields.

## Retained limits / NOT_PRODUCED

- `CEM-V1-036` (`docs/specs/cem-stable-v1.md`) still says a stable report refuses every `--ocm`.
  The behavior is superseded only by the dated section in `docs/specs/ocm-v0-dogfood.md`. The
  spec line needs an owner amendment.
- The stable OCM route does not apply `--max-unknown` or `--max-mechanical` (refused on stable
  reports anyway), so it never yields `incomplete` or `policyIssues`.
- `VerifyStableUniverse` reruns the stable verifier, so a stable report with `--ocm` verifies the
  map twice.
- The MCP report preview has no OCM input, so it is unaffected.
- `make gate` was not run, per lane policy.

## Rollback

Revert the change commit. No wire profile, stored state or requirement line changed.
