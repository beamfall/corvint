# Friction batch w2: five dogfood and test-selection fixes (2026-10-31)

Lane friction-w2, base `780f9e97099aa3ccd52744a0c239a744c4a30adf`. Tickets V1-3378 to V1-3382, all
found by the bar-lowering lane on 2026-10-31. Each ticket has a spec commit before its
implementation commit. None was already delivered on the base.

## V1-3378: env-prefixed verify lines (DCW-V0-804)

Decision: a leading `NAME='value'` or `NAME="value"` assignment is admitted only with a value whose
every byte is literal to a POSIX shell: single quotes admit printable ASCII except `'`, and double
quotes also exclude `$`, a backtick, `\` and `!`. The line is recorded as one typed argv
(`--verify-argv-json`), as the DCW-V0-801 quoted `-run` word already is, so no shell parses it. The
assignments are the first argv words without their quotes. That keeps the exact command
(`HEAVY_WHOLE_REASON=whole package`, `script/heavy.sh`, ...) without adding an `env` word the
operator never wrote. A line whose assignments are all unquoted stays a v1 command, so existing
records keep their bytes. The rest of the line must still be a v1 command or a DCW-V0-801 quoted
`-run` line, so a pipe, an expansion or a trailing assignment is still refused
`unsupported-verify-syntax`. Both remediation lines now name the assignment form.

The remediation examples use `GOFLAGS='-count=1 -v'` rather than `script/heavy.sh`: naming
`heavy.sh` in Go source would add five files to the TSP-V0-005 foreign-caller declaration
(`TestRepositoryShellForeignCallersComplete_TSP_V0_005` failed until they were reworded).

## V1-3379: stale REQUIREMENTS.tsv beside an unstaged spec (SRG-V0-800)

Decision: the ticket allowed either failing doc-gates or making `spec-requirements` refuse. The
generator keeps reading only the index (SRG-V0-001, the fresh-clone rule), and
`spec-requirements-check` gains `script/spec-requirements-worktree-check.sh`. That script
regenerates from a private copy of the index with the unstaged tracked spec edits added, and fails
when the result differs from the generated files on disk. A prose-only edit therefore still
passes, and the real index is never written. Untracked specs keep the DCW-V0-352 warning only, so a
stray draft does not fail the gate. The check is skipped under a private `GIT_INDEX_FILE`
(dogfood-finish), whose snapshot is already exact. Live repro on this checkout: appending an
unstaged requirement to the DCW spec fails naming `REQUIREMENTS.tsv`, the spec and the fix.

## V1-3380: propose-citations ranking (DCW-V0-805)

Decision: for a hunk inside a BASE requirement's list item in a spec, the draft ranks first the
decisions that the requirement's definition line cites (`intent decision 0431`, `decision 0509`).
The requirement row itself is the "requirement row" of the acceptance criteria, but it lies in the
file the hunk modifies, which DCW-V0-393 forbids drafting, and `REQUIREMENTS.tsv` is a generated
projection that drafts exclude. The row is therefore named in the note as a review hint. Within the
cited decisions, the line sharing the most distinct words (four or more letters) and numbers with
the hunk's BASE side wins, with at least three shared. Otherwise the first line naming the
requirement wins. Both list forms (`` - `ID`: `` and `- **ID:**`) are recognised. A draft of the
real commit `2865c23b9` now cites decision 0431 lines 105, 128 and 131 for its three requirement
hunks, where it used to cite `documentation-ci-v0.md` 9:9 and decision 0131. Line 128 lies inside
the governing lines 126-129. The fourth hunk sits in AFP-V0-047, whose definition cites no
decision, so it keeps the old path anchor.

Dogfood finding: this lane's own first draft cited the decisions of the requirement *before* each
newly added requirement, such as decision 0388 for the inserted DCW-V0-804 to DCW-V0-806 block. A
hunk that adds a definition without removing it is a new requirement and amends none. A pure
insertion amends the item before it only when its first added line is an indented continuation.
Table rows stop the walk. A definition line rewritten in place (removed and re-added with the same
ID) still amends, which keeps the AFP-V0-029 definition hunk of `2865c23b9` on decision 0431
line 105.

## V1-3381: originating refusal of a stopped closeout (DCW-V0-806)

Repro: an intent spec with 257 requirements under `finish-lane`. `ocm-prepare-001` refuses
`too-many-obligations` with no fix line, and `ocm-aggregate` reports `intent-scope-drift` with a
fix about a changed intents file. Decision: the reason codes stay, because `intent-scope-drift` is
part of the report contract (DCW-V0-058, DCW-V0-349). The output changes instead:
- A stopped pass prints `dogfood-finish-lane: CAUSE <pass> <row> <reason>` for its first failing
  row, plus a `recover:` line about the uncommitted sidecar.
- `ocm-prepare-*: too-many-obligations` gets a fix naming the per-map bound.
- The aggregate's drift fix names the refused prepare row as the cause.

The recovery line holds only when the sidecar is the only uncommitted path, as in this
repository's `.gitignore`. A repository that does not ignore the other `.corvint/` files still
gets `dirty-worktree`, which names them.

## V1-3382: forward-rows test selection and a compact dry run (NSA-V0-800, STR-V0-800)

Finding: on the base, a `bar.jq` edit already narrowed (42 packages, `tools/ci-forward-row`
included, through literal readers), so that criterion was met before this change. A `verdict.sh`
edit fell back to FULL because the shared source walk skips every hidden directory. The shell
plugin owned `.github/**/*.sh` by suffix but declared no unit for it (`UNINDEXED_SOURCE_PATH`).
Decision: the shell plugin also indexes `.github` through `Source.Walk`, on disk and on immutable
sources. The shared walk is left unchanged, because widening it would change every plugin and the
graph cache keys. The `verdict.sh` edit now narrows to the same 42 packages.

`--dry-run --summary` prints a text summary of at most 4096 bytes in place of the JSON: 2.7 KB for
that edit, against 10 KB of JSON (66 KB on the FULL fallback). The JSON stays the contract. The
summary is for reading only.

## Verification

Focused tests only, because `corvint test --base 780f9e97 --dry-run` selects 212 packages whole.
The packages run were `internal/dogfoodflow` (whole), `internal/liveverify/affected/shell` (whole),
`internal/capcatalog`, `internal/liveverify/affected/languages`, and `-run` selections in
`cmd/corvint` and `internal/liveverify/affected`. `make spec-requirements-test` also ran.
`make gate` was not run (NOT_RUN under the owner's focused-test preference).
