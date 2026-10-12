# 2026-10-12 CEM provider-neutral criterion walkthrough (V1-1263)

Branch `claude/v1-1263-neutral-criterion` from `origin/main` 19779517f. Decision 0523 P1,
requirement `CEM-V1-083` (proposed) in `docs/specs/cem-stable-v1.md`. This entry is the timed
walkthrough the pilot kit points to: a `cem/1.0` map with only a plain criterion, produced on a
repository that is not a Corvint repository and verified by both consumers.

## Setup

- Binaries `corvint`, `corvint-test-runner`, `corvint-cem-candidate` and `cem01-go` were built from
  the lane worktree with `GOTOOLCHAIN=local` (go1.27.2, darwin/arm64) after the wire and portable
  changes; the build took about 6 s and is not in the table.
- Repository `greet`: a two-commit Go module (`SPEC.md`, `go.mod`, `greet.go`, `greet_test.go`)
  in a fresh scratch directory, with no `.taskman/`, no Tasks store and no Corvint configuration.
  The second commit makes `Greet("")` return `Hello, stranger!` and adds its test. `SPEC.md` line 6
  holds that acceptance text.
- Every step ran through a wrapper that records wall seconds and the exit code. The operator was an
  agent, not a person new to Corvint, so human reading, decision and typing time is NOT_OBSERVED.

## Timing

| Step | Command | Seconds | Exit | Result |
| --- | --- | --- | --- | --- |
| 1 | create the repository (2 commits) | 0.205 | 0 | base `cd210a37f`, target `ddc67e3e3` |
| 2 | `corvint --root R cem prepare --base B --target T` | 0.155 | 0 | `cem/0.2` map, 2 hunks |
| 3 | `cem cite --map M --hunk 1 --evidence-path SPEC.md --lines 6:6 --relation specification` | 0.065 | 0 | hunk 1 supported |
| 4 | the same for `--hunk 2` | 0.071 | 0 | hunk 2 supported |
| 5 | `cem status --map M --expected-base B --target HEAD --max-unknown 0 --max-mechanical 0` | 0.301 | 0 | `ready-for-ci`, after committing the map over the target |
| 6 | `corvint-test-runner plan --request ... --executable GO --executable-sha256 ...` | 0.014 | 0 | `go-test` plan |
| 7 | `corvint-test-runner run --plan ... --approve ... --experimental --trusted-local` | 4.188 | 0 | both tests PASSED |
| 8 | `corvint-cem-candidate assemble-stable --experimental --request ...` without Tasks values | 0.286 | 1 | refusal: `stable assembly refused: incomplete stable assembly request` |
| 9 | hand-assemble the plain `cem/1.0` map (labelled scratch script) | 0.100 | 0 | one plain criterion on the `SPEC.md` span |
| 10 | `corvint cem verify-stable --repository R --map MAP --expected-base B --target T --artifacts DIR` | 0.197 | 0 | ACCEPT |
| 11 | `cem01-go verify-stable-canonical` with the same arguments | 0.697 | 0 | ACCEPT; result identical to step 10 except `runtime` |
| 12 | `cem status --map /absolute/stable.json ...` | 0.026 | 2 | refusal: `invalid-arguments`, map path must be repository-relative |
| 13 | `cem verify --map /absolute/stable.json ...` | 0.018 | 2 | the same refusal |
| 14 | `cem status --map stable.cem.json ...` (repository-relative, no `--artifacts`) | 0.019 | 2 | REJECT at `authority-arguments`, `artifacts-required` |
| 15 | `cem verify --map stable.cem.json ...` (no `--artifacts`) | 0.020 | 2 | the same refusal |
| 16 | `cem status --map stable.cem.json ... --artifacts DIR` | 0.187 | 0 | `ok: true`, ACCEPT, `changeIntegrity` VERIFIED, `referenceIntegrity` REFERENCE_INTEGRITY_ONLY |
| 17 | `cem verify --map stable.cem.json ... --artifacts DIR` | 0.199 | 0 | ACCEPT |

Machine time for steps 1 to 17: 6.75 s. Before this change, the installed `corvint 1.0.0-rc.3`
refuses the same plain map with REJECT at stage `wire`, code `missing-field`.

## Refusals met

1. Step 8: the only stable producer, `corvint-cem-candidate assemble-stable`, accepts only the
   Tasks-bound `cem-stable-assembly/1` request (`ticketId`, `attemptId`, `capture`, `verification`,
   `snapshotHeadArtifact` required). No flag or request form emits a plain criterion.
2. Steps 12 and 13: `cem status` and `cem verify` take `--map` only as a repository-relative path.
3. Steps 14 and 15: a `cem/1.0` map with references needs `--artifacts`, even when the
   artifacts are not opened beyond their digests.

## Public case

`protocol/cem-1.0/stable/neutral-criterion/` is derived from this run. Its repository recipe
rebuilds the same two trees with the fixed `Fixture` identity, so its commit IDs differ from the
walkthrough's. The runner plan and receipt are replaced by opaque fixtures, because the verifier
checks artifacts only by SHA-256 and the real files carry absolute scratch paths. `EXPECTED.json`
holds the ACCEPT result (all keys except `runtime`) and seven refusals, each recorded from the
native binary and matched by the portable binary: plain plus `ticketId` gives `missing-field`;
full Tasks plus `evidenceId`, and a foreign key, give `unknown-field`; no `evidenceId` gives
`missing-field`; a malformed `evidenceId` gives `invalid-field` (all at stage `wire`); a span-digest
mismatch gives `reference-coherence` and an unresolved evidence ID gives `unresolved-reference`
(stage `references`). All case files are pinned in `protocol/cem-1.0/stable/manifest.json`.

## Limits

- No plain-form producer: the missing piece is a plain request form for
  `corvint-cem-candidate assemble-stable` (or a `corvint cem` verb that emits `cem/1.0`). The map
  in step 9 and the public map are HAND-ASSEMBLED.
- K4 (a person new to Corvint produces and verifies a map on a non-Corvint repository in one
  engineer-day) is not demonstrated: the machine path is under 7 s, but the operator was an agent
  and stranger time is NOT_OBSERVED.
- Runner receipts, criterion links and runner artifacts are still required by the existing
  coherence rules; a plain criterion removes only the Tasks artifact roles.
- `stable.schema.json` was not validated with a JSON Schema validator (none installed): NOT_RUN.
  The schema is checked only as JSON.
- The `manifest.json` entries for `REPOSITORY-ENVELOPE.md` and `REPOSITORY-ENVELOPE.cells.json`
  were already stale on `origin/main`. They are left unchanged because those files are owned
  elsewhere.
