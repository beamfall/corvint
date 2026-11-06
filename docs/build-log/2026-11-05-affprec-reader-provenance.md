# Affected reader precision: affprec lane

Date: 2026-11-05
Owner direction: V1-4497, V1-4036, V1-3992, V1-4143, V1-4453 and V1-4535.

## Decision and boundary

Implement experimental AFP-V0-9000 through AFP-V0-9005 in the bounded companion
`docs/specs/affected-reader-precision-v0.md`. The existing affected-plan spec has
256 obligations. A new prefix with only the allocated four-digit suffixes is
silently omitted from REQUIREMENTS.tsv by the generator’s SRG-V0-790 prefix rule;
reuse the owning AFP-V0 prefix in a companion file instead of changing the generator
or consuming an unallocated three-digit ID. The index now contains all six clauses.

File-local read-kind declarations are project authority, not inferred whole-program
analysis or confinement grants. The three Tasks scans read Go source and module
boundaries, so their root walks acquire positive reader globs. The mutation export
tests use disposable repositories; their parent literals include rejected symlink
targets. A separate fixtures declaration removes only those lexical unbounded
reasons. Ordinary path literals, other files’ reasons, imports and execution edges
remain. Malformed, duplicate, non-test, inline and unparsed declarations keep the
conservative behavior. Removing a declaration restores the previous selection.

Move the two parent-path help examples into verbatim embedded text, preserving
their published bytes (SHA-256 checks against the lane base). The embed remains
a dependency and a real production reader still propagates to importers. No JSON
members, error/exit contracts, stderr, committed goldens or core-freeze outputs change.
The unbounded-reader inventory drops precisely Tasks and mutation export tests.

## Measured precision and acceptance

Matched historical source replay: export `b0cf21d35` without Git metadata, observe
it using the candidate selector, and compare the identical source tree with only
the source-kind/fixture declarations and embedded help applied. Both arms use the
same committed read-scope base (`115c44f98` for the fix, `24d8c04f9f5ffaaed86759951e520cb775dc2eff`
for the complete historical range). This reproduces the original fix count of 97.
The complete-range count differs from the old runtime’s 195 because this observer
has evolved; do not attribute that cross-version difference to this lane.

| Replay | Before | After | Retained uncertainty |
| --- | ---: | ---: | --- |
| heavy | 72 | 71 | UNKNOWN, 1 unknown entries |
| test-only | 62 | 61 | UNKNOWN, 2 unknown entries |
| rescue-fix | 97 | 96 | UNKNOWN, 2 unknown entries |
| rescue-data | 80 | 78 | UNKNOWN, 1 unknown entries |
| rescue-full | 210 | 209 | UNKNOWN, 85 unknown entries |

At the lane base the matched current-tree replays are heavy 72 → 71, historical
test-file paths 61 → 60, and dirty foreign-caller declaration plus spec 48 → 46.
The explicit importer fixture also narrows a test-only edit 34 → 2, retaining
the true reader of that very test file. The historical commit named in V1-4143
actually changes three test files plus one spec; the exported fourth test path
and original 49-unit receipt were not supplied. These are measured replay shapes,
not a claim that an unavailable 49-unit receipt was reproduced.

- V1-4497, AFP-V0-9003/9005: 71 units is below the reported 86; the remaining
  shell foreign-invocation frontier is retained. A true script reader remains.
- V1-4036, AFP-V0-9000/9001: the actual boundary source is tested against hidden
  JSON and Go/module changes; the same JSON’s real reader remains. All three
  Tasks root scans require declarations to remove the package-wide false cause.
- V1-3992, AFP-V0-9002: a new importer of the actual help source selects only its
  own unit for its own test edit. Embeds and a real parent-path production reader
  retain importer selection. Both usage strings remain byte-identical.
- V1-4143, AFP-V0-9003/9005: both live/historical replay and the importer fixture
  narrow with measured counts; another test’s true file reader stays selected.
- V1-4453, AFP-V0-9004: the full dirty declaration retains its self-change frontier.
  The documented conditional measurement is `affected --worktree --path PATH`,
  which uses the current declaration without claiming the excluded edit is verified.
  Without `--worktree`, the existing mixed-worktree refusal remains. The regression
  covers a newly written declaration, a true declaration reader, and invalid data.
- V1-4535, AFP-V0-9003/9005: the exact rescue fix narrows 97 → 96 and its data/spec/
  script projection narrows 80 → 78. All six original failure-test units from the
  immutable rescue build log remain selected (release-artifact, py-signal-lint,
  companionrelease, the main command, and the two changed shell-test units).

The original rescue fix includes Go source and tests; it is not data-only. Its
two retained frontiers are Go build variants and unmodelled shell invocation.
The full range additionally has four genuinely unowned inputs (`.corvint/agent-grep-exclude`,
`.corvint/long-line-allow`, `.gitignore`, `Makefile`) and two unselected web-flow
binary consumers. Its 79 language frontiers include TypeScript bare imports,
dynamic/runtime and plugin frontiers bearing on those unowned inputs. Narrowing
them requires actual reader/runtime/module declarations, not an extension exemption.
Replaying historical script paths against the current checkout, where those scripts
do not exist, raises UNINDEXED_SOURCE_PATH and unrelated plugin frontiers. That
losing measurement is retained outside the tree and is not historical acceptance.

## Verification and limits

Whole-package runs, one package per script/heavy.sh command, cover affected,
affected/golang, liveverify/mutate, tasks and tasks/cli. The spec index also runs
as a complete package. Commands, durations and any retained failure appear in the
lane handback. The first Tasks CLI run failed the pre-existing compaction test
because observedAt advanced from 15:09:30Z to 15:09:31Z across the compared reads;
no assertion or unrelated implementation was changed. The complete rerun passed; retain the failed first run as clock/load friction.

The first frozen selection-baseline run passed: 33/43 merged changes narrowed,
0 misses across 26 known failing packages from 15 failed runs, floor 30. A final
candidate run is retained separately after the last semantic narrowing change.
Mandatory independent review belongs to orchestration; no model-backed reviewer
runs in this lane. Repository-wide make gate, Darwin/Windows qualification, and
independent interoperability outcomes are NOT_RUN; this is scoped WSL work and
no push or release occurs. No token, cache or performance savings are claimed.

Pre-change dogfood context/query/impact receipts were retained before editing.
Initial no-diff CEM/outcome steps were NOT_PRODUCED by design. The first impact
attempt used a directory and refused unsupported-impact-path-suffix; the retry
used original .go paths and recorded all receipts. Final CEM/check results are
reported in the handback; sealing, publication and Tasks writes are prohibited.

## Evidence digests

Raw source exports, plans and logs live outside the repository at
`/home/russell/lanes/affprec-evidence/`. The measurement helper and overlay script
are retained there so the counterfactual replay can be repeated.

- `rescue-prior-before.jsonl`: SHA-256 `391c387343ce0d09ad01cfa1534702af8cc6f3aa7aef375f1f24639a7cd38690`.
- `rescue-final-after.jsonl`: SHA-256 `d78fa36f1ecfa145baac90769a8c387087140bd6a7a5b772cb88633ab93a4f03`.
- `replay-before.jsonl`: SHA-256 `149187ba3b0c956214ee78a7b0fd156f54be0a607c17629a19a5c94ed4774cb6`.
- `replay-final-after.jsonl`: SHA-256 `5ccaeab4491985ad0dd6a688c23658cb91684f009d1b387bf03f7a6e973fe281`.
- `apply-precision.py`: SHA-256 `28d1071fa61850f16fbeaa9c58087bcb6b6d037d33b45dc6768501444a67a26f`.
- `measure/main.go`: SHA-256 `e9b0d4172556ac3f9639a8cd1b449585abb40acc6aa6c29d9d54036e6d08047d`.
- `dirty-conditional-measurement.json`: SHA-256 `e1b8c94dc0041ccd36fa287b5120029ddd87225b320392a63434214061a21e94`.

The measurement-summary JSON binds the counts, exact removed-unit sets and all
six retained historical failure-test unit IDs. Full plans retain every frontier
and its bearer; no missing evidence is replaced with a bounded verdict.

Immutable replay identities:

- historical target: `b0cf21d357e5e1455f9c81d47f2053c317053433`.
- historical fix base: `115c44f9814ccfd2930b90b9c692fd549dbdde28`.
- test-only source commit: `0043172bff4b6300869104eb94ad59b74d18014b`.
