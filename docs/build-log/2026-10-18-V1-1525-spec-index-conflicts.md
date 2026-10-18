# 2026-10-18 V1-1525: the requirement index stops conflicting on line shifts

GitHub-issue lane for V1-1525 (see also V1-0332). Concurrent spec pull requests conflicted in
the generated `docs/specs/REQUIREMENTS.tsv`. Its rows carried line numbers in id order, so an edit
that shifted a spec rewrote every row below it. #1089's only conflict was line numbers: 42 ATR rows
were re-pointed. Decision 0543 chooses the remedy:

- The committed index drops the line column and orders rows by file, then by definition position.
- `script/gen-spec-requirements.sh --lines` keeps the old layout as an uncommitted view.
- Readers resolve the line from the spec.

The new requirements `SRG-V0-015..017` in `docs/specs/spec-requirement-index-generator-v0.md`
(with the SRG-V0-006 and SRG-V0-010 amendments and the SRG-V0-013 extension) are accepted under
the owner's standing auto-accept of lane requirements (2026-10-11); V1-1525. No CAL-V0 change.

Collision checks:

- **Requirement IDs.** origin/main `2ab0e744` has `SRG-V0-014` as its highest SRG ID. No open
  pull request touches the SRG spec or adds an SRG ID.
- **Decision number.** origin/main's highest decision is 0539. Open PRs claim 0540 and 0541
  (#1100) and 0540 (#1094), so this lane takes 0543. #1100 and #1094 collide with each other on
  0540; that is reported as a residual and not fixed here.

## Design

- **Generator (SRG-V0-015).** TSV mode prints `id file title acceptance acceptedBy`. Rows are
  sorted by file, then by the definition line the ranks already choose, then by id. `--lines`
  prints the previous six-column, id-ordered layout. At the base it was byte-identical to the
  committed file it replaced.
- **Readers (SRG-V0-017).**
  - `internal/specindex` gains `ParseIndex`, which reads both layouts, and `DefinitionLine`, which
    applies the generator's ranks to the spec text.
  - The console clause lookup, the compaction kernel (from the pinned spec blob), the roadmap
    evidence links and `script/spec-coverage-audit` use them.
  - `script/spec-acceptance-audit.sh` takes its line numbers from `--lines`. Its standing, owner
    and unrecorded outputs are byte-identical to the base.
  - `DefinitionLine` matched `--lines` for all 4232 committed rows.
- **Conflict reproduction (SRG-V0-016).** `script/spec-requirements-conflict_test.sh` builds
  scratch repositories and merges each pair of lane branches in both orders through
  `make spec-requirements`. The lanes are:
  - X: a row added after SEC-V0-002;
  - Y: a row added after SEC-V0-011, in a later section of the same spec;
  - Z: a prose paragraph above both;
  - O: a row added to another spec.

  | Pairs | Base generator | This change |
  |---|---|---|
  | X+Y, X+Z, Y+Z | conflict in `REQUIREMENTS.tsv` | merge cleanly; all three generated files equal regeneration |
  | X+O | merges cleanly | merges cleanly |

- **Limit.** New rows that are adjacent in the committed order still conflict: for example two
  changes that both append to the end of one section, or two new specs that both append to
  `INDEX.json` and `README.md`. They are resolved by resolving the spec and regenerating, as
  before. This limit is part of SRG-V0-016 and its failure table.
- **CI.** No `.github/` change is needed. `spec-requirements-check` still byte-compares the
  three committed files.

## Review fixes

An independent review of PR #1108 found two P2s; both are fixed here.

1. **The acceptance audit joined `--lines` by id only.**
   - Effect: a stale row naming the wrong file was printed with the other file's line, and the
     audit exited 0.
   - Fix: the audit now joins on id and file and exits 1 on a mismatch.
   - Test: `script/gen-spec-requirements_test.sh` case 8. It failed before the fix (exit 0).
2. **SRG-V0-016 promised too much for prose edits.**
   - Change: the guarantee now covers only edits that leave every indexed field unchanged.
   - Limit added: a reflow that changes a requirement's first line, and so its title, conflicts
     with a row inserted right after it.
   - Test: `script/spec-requirements-conflict_test.sh` shows this as an expected
     `REQUIREMENTS.tsv`-only conflict in both orders, resolved by regeneration.
