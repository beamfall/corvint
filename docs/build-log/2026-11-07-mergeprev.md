<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5043: local merge-conflict prevention

Owner intent: the mergeprev lane charter, 2026-11-07, subsumes V1-4716 and the conflict-free
part of V1-4704. Base: `cb403735903b1e54a8469373489960c0b2b89d99`. Requirements
`DWC-V0-7700`..`DWC-V0-7705` extend the existing daily-workflow companion under the owner's
standing auto-accept; its obligation count remains below the OCM bound. No new release executable,
wire shape, existing error/exit/stderr contract, golden or core-freeze output was changed.

## Decisions and resulting behavior

1. Share native-Go rules between development tool `tools/merge-drivers` and batch replay.
   Append-union uses Git diff3 and exactly the host reference's whitespace-base rule, ours then
   theirs, with its heading separator. Base-text edits retain their conflict markers.
2. Union registry entries atomically inside the scripts/packages/tests maps. Combining independent
   fields inside a competing same-entry edit would conceal disagreements, so it remains a conflict.
   Arrays retain their order, duplicate keys refuse, numbers retain precision, and successful
   output uses sorted object keys, two-space indentation and LF. Attributes name only the three
   registries; CEMs and other policy/receipt JSON acquire no merge rule.
3. Keep generated projections' ours bytes, then regenerate from staged inputs before committing.
   The documented local merge-forward sequence includes `make spec-requirements` and the existing
   freshness check remains fail-closed. Installation writes three local Git driver entries only,
   is idempotent, and uses the development Go tool without a Python runtime.
4. Replay from ort's scratch tree and parent blobs, including recognized files both parents
   changed even when ort merges them cleanly: Git invokes custom drivers in that case too.
   Replay requires one merge base and regular 100644 files, resolves only the named paths, and
   compares the entire tree and modes. Projection changes use the existing private generator
   replay; no candidate bytes or local Git configuration define a rule. Other resolutions retain
   the existing unbound/dropped-side checks. Derivation names are joined only for combined rules;
   the existing summary shape is unchanged.

## Fault injection and local verification

All Go tests below ran through `script/heavy.sh`, one package per command, with
`TMPDIR=/home/agent/lanes/mergeprev.tmp`, Go 1.27.3, no exported commit identity, and the configured
`/home/agent/lanes/.flock-slots` semaphore. The affected plan selected all three touched Go packages.

| Requirement | Verification | Observed result |
|---|---|---|
| DWC-V0-7700 | `go test -count=1 -timeout 30m ./internal/mergedrivers`, `TestAppendDWCV07700` | PASS; headings, rows, whitespace base, final LF, CRLF, real edits, malformed markers |
| DWC-V0-7701 | whole `./tools/merge-drivers`, `TestProjectionDWCV07701StaleCheck` | PASS; real generator accepts fresh output and rejects a staged added requirement with stale projections |
| DWC-V0-7702 | whole `./internal/mergedrivers`, `TestJSONDWCV07702` | PASS; disjoint keys, same-key divergence, atomic entries, deletion, precision, duplicate/malformed input |
| DWC-V0-7703 | whole `./tools/merge-drivers`, `TestGitDriversDWCV07703` | PASS; actual Git driver merges, retained edit conflicts, untouched ours on JSON conflict, keep-ours projection, repeated Make installation |
| DWC-V0-7704 | focused `./internal/dogfoodflow` driver cases without regeneration | PASS; append/JSON driver merges, clean-text JSON canonicalization, config non-execution, repository status unchanged, added hand file/key refusal, divergent registry refusal |
| DWC-V0-7704 | `go test -count=1 -timeout 30m ./internal/dogfoodflow` including all existing tests | FAIL; existing host cleanup/regeneration failures plus the three new regeneration-dependent combinations; see external blocker below |
| DWC-V0-7705 | two fetched historical lane merges below | Both improve from 3 conflicts to 0; stale projections refuse before regeneration and pass afterwards |

`make spec-requirements` and `make -s spec-requirements-check line-citations-check`: PASS.
`script/check-line-citations.sh --repin` reached a fixed point in its two internal passes,
repinning three moved Makefile citations in two specs. The generator reported existing full-spec
headroom warnings; no capacity gate was weakened. `tools/test-read-scopes-check`: PASS after
declaring actual source reads and marking scratch-only fixture path literals.

The complete touched `internal/dogfoodflow` suite ran twice (final run 33.073 seconds of package
time). A separate clean scratch clone at the untouched base reproduces
`TestAggregateCallerHoldRetainsLockAndStopsFollowOnWork`,
`TestBatchCheckDWCV0001RegeneratedRequirementsMerge`,
`TestBatchCheckDCWV0401AcceptsRegeneratedCommits`, and
`TestChangeDCWV0398CitesRegeneratedHunksOnly` failures. The cleanup failure reports
`/proc quiet proof unavailable` for a vanished leader; regeneration is `NOT_REPRODUCED` at base
too. No existing assertion was changed. Regeneration-dependent positive batch qualification is
therefore **NOT_PRODUCED on this host**, pending the orchestrator's host verifier; new integration
tests remain strict. V1-5043 is **PARTIAL only for this external qualification blocker**, with the
implementation, replay, and fault-injection tests delivered. Native ticket completion and
independent review belong to the orchestrator, and were not attempted by this lane.

## Historical replay

Fetched `main`, `claude/unbrd`, and `claude/selbatch` from configured origin. Main is
`cb403735903b1e54a8469373489960c0b2b89d99`; lane revisions:

| Lane | SHA | Before: merge exit / conflict paths | After: merge exit / conflict paths |
|---|---|---|---|
| unbrd | `1b974300f2f624f862fe0ae1f32456bf2cbc5ab5` | 1 / 3 | 0 / 0 |
| selbatch | `445dcdd46def75fe7bfec0eb8497c5a3c4ebb8c6` | 1 / 3 | 0 / 0 |

Both before sets are `docs/specs/README.md`, `docs/specs/REQUIREMENTS.tsv`, and
`docs/specs/affected-plan-selection-v0.md`. Both after sets are empty. A scratch clone borrowed
local immutable objects and checked out each lane SHA; no source checkout was merged or reset.
The before run had no custom drivers. The after run installed the Make target, supplied these
attributes through scratch `.git/info/attributes`, and substituted the lane-built development
executable's absolute path because historical snapshots lack its source. Every actual merge used
`git merge --no-ff --no-commit MAIN`. Both staged after merges failed the real generator's
`--check` (exit 1), then `make spec-requirements` passed and `--check` returned 0. GitHub server-side
merge behavior is not qualified by this result.

Raw logs, scratch clones, executable and digests remain outside Git under
`/home/agent/lanes/mergeprev-evidence/`. Retained SHA-256 digests:

| Evidence | SHA-256 |
|---|---|
| replay-summary.txt | `b61100922b70c20f9692cef4f02a06e43d82d20d3af1015f6b40af20ccdd7c44` |
| replay-regeneration-summary.txt | `fcb0661a8312c7eb38e131adbb0eb51ed0090421a8b9b73acd29f7934c88cde0` |
| baseline-replay-tests.log | `bf98378bac7c652b4f411e7c5d8fd6ab5c0a247bf18de59fe5b790b3ffa856e9` |
| library-tests-final.log | `99e81fd06de0924c5abe55746618f87e2947ad4ffae554f33b2e1fc2cf1e15a4` |
| tool-tests-final.log | `45609be7fff978bcb446e2a51c7c06fd02505125e28cece47be338443693a588` |
| dogfoodflow-tests-final.log | `36c415e9071dd22f0dec2cbe0d1302627b81e51ddc630dea9dfaf793bd65a714` |
| affected-final.json | `27842a36cbf91af4cd97f9240d58b6ee0a1d638e21873ef1d9489b3a5093d445` |
| source-digests.txt | `be117628115ecc1403f789c218640e23d4313f583d9d0b58691f54fe3eeddee8` |
| driver-no-regen-tests.log | `ceac119afd5d8bd609f35bd69393f5bc2efc19aa459f5b1d6d1881bc91f2ad3d` |
| spec-line-checks.log | `12c65d49866724b2f54988b753bed1e97e8e62e144455fee3573696be75ebfd4` |

## Dogfood, exclusions and friction

Starting `make dogfood-change BASE=...` produced PRECHANGE/no-diff-yet. Prechange context and query
were retained; impact refused mixed non-Go input (`unsupported-impact-path-suffix`). The bounded
context packet named DWC-V0-001/deriveMerge with two supporting terms but omitted five results;
the implementation and original companion contract were read explicitly. No coverage/savings
claim is made from that packet. The final affected plan reported UNKNOWN: 134 selected units,
81 unknown entries (79 LANGUAGE_FRONTIER, 2 UNOWNED_DIRTY_PATH), plus `self-binary-stale` for the
installed reader. Those limits remain visible; only touched whole packages and the requested
spec/citation checks were run. Index installation, learning/evaluation, task mutations, publication,
whole-repository gates, model-backed review and sealing were excluded by the lane charter or
inapplicability. Billed tokens and host performance comparisons are NOT_OBSERVED.

Numbered reproducible friction:

1. `corvint-tasks ticket show V1-5043`: NOT_OBSERVED/MALFORMED because the committed snapshot lacks
   the new ticket and `/home/agent/projects/corvint/.git/taskman` has no journal. The charter supplies
   intent; no Tasks write was attempted.
2. Starting dogfood reported five of seven self-observations not recorded, first
   `cem-prepare: observation-prohibited-content`. Original report remains private; no ledger
   content was changed to make it pass.
3. `make dogfood-prechange BASE=... TASK=... PATHS='... .gitattributes Makefile ...'` retains
   context/query but refuses impact with `unsupported-impact-path-suffix`.
4. Base and changed `internal/dogfoodflow` tests reproduce the sandbox `/proc` cleanup and
   regeneration failures above. The host verifier must rerun the strict new regeneration cases
   and existing whole package; no runtime safety check was relaxed.
5. Local installation in this linked lane would need
   `/home/agent/projects/corvint/.git/config` (and `.git/config.lock`), outside the permitted metadata
   paths. It was tested in scratch repositories instead; no shared config write was attempted.
6. First stale-projection fixture lacked a sentence in its README description; the real generator
   refused it. The fixture was corrected, then the whole tool package passed. Initial new batch
   fixtures lacked the archive directory; that fixture error was repaired before final testing.
7. Initial read-scope check flagged scratch-only README/registry literals. Explicit fixture
   annotations fixed the declarations without broadening read scopes; its rerun passed.

Rollback: remove new attributes/local driver entries, development tool and driverMerge call.
Legacy DWC-V0-001 replay remains; no remote history rewrite or published release is involved.
