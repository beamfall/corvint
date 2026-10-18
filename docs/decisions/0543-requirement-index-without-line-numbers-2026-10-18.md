# Decision 0543 — The committed requirement index carries no line numbers

Date: 2026-10-18. Status: accepted. Authority: the owner's standing auto-accept of lane
requirements (2026-10-11), with the design choice delegated to the lane by the orchestrator.
Tickets: V1-1525 (see also V1-0332). Adds SRG-V0-015..017 to
`docs/specs/spec-requirement-index-generator-v0.md`, amends SRG-V0-006 and SRG-V0-010, and
extends SRG-V0-013.

## Context

`docs/specs/REQUIREMENTS.tsv` carried each requirement's line number, ordered by id. Any edit
to a spec moved the line numbers of every requirement below it. Two pull requests that touched the
same spec, even in different sections, therefore each rewrote the same block of rows and
conflicted. PR #1089's only conflict was line numbers: 42 ATR rows were re-pointed. The generated
`INDEX.json` and `README.md` hold no line numbers and only conflicted when two changes disagreed on
a header or both appended a new spec.

Options considered:

1. **Drop the volatile column and order rows by definition position (chosen).** Line shifts
   rewrite no row. A new row lands next to its spec neighbours, so changes in different sections
   of one spec insert at different places and merge cleanly.
2. **A union or custom merge driver in `.gitattributes`.** GitHub's merge, the merge queue and
   `dogfood-batch-check` (which accepts only trees plain `ort` can reproduce) do not run custom
   drivers. Union merge would also keep both stale line values as duplicate rows.
3. **Shard the index per spec.** This removes cross-spec conflicts, which were already rare
   because rows were ordered by id. It does not help two changes to one spec, which is the
   observed case, and it multiplies the files every reader opens.
4. **Stop committing the index.** This breaks the committed, agent-readable route in `AGENTS.md`
   and the pinned-blob reads of the compaction kernel.

## Decision

1. The committed index has the header `id file title acceptance acceptedBy`. Its rows are ordered
   by file, then by the definition's position, then by id (SRG-V0-015).
2. `script/gen-spec-requirements.sh --lines` prints the previous layout, with the line column and
   ordered by id, as a derived view that is never committed. At the base of this change it was
   byte-identical to the committed index it replaced.
3. Readers take a requirement's line from the spec (SRG-V0-017):
   - Go readers use `specindex.DefinitionLine`, which applies the generator's ranks.
   - Shell readers use `--lines`.
   - Go readers parse through `specindex.ParseIndex`, which reads both layouts, so an index
     pinned at an older revision still resolves.
4. The conflict property and its limit are SRG-V0-016, demonstrated by
   `script/spec-requirements-conflict_test.sh` through the real `make` target.

## Effects

- **spec-requirements-check:** unchanged. It still regenerates and byte-compares the three
  committed files and runs `internal/specindex`. It does not compare the `--lines` view.
- **specindex tests:** `TestRequirementsAcceptanceColumns` reads the five-column header.
  New tests pin the ordering, the line resolver over every committed row, both layouts, and the
  ranks.
- **Agent read routes:**
  - The id lookup is unchanged: `rg -n '^ID\t' docs/specs/REQUIREMENTS.tsv` still returns the
    row with the file and title.
  - A route that needs the line adds one step: `rg -n -F 'ID' <file>`, or
    `script/gen-spec-requirements.sh --lines`.
  - `docs/AGENT-ROUTES.md` says so.
- **Readers migrated in the same change:** the console clause lookup, the compaction kernel (from
  the pinned spec blob), the roadmap evidence links, `script/spec-coverage-audit` and
  `script/spec-acceptance-audit.sh`. The acceptance audit's output is byte-identical before and
  after in all three modes. Readers that use only the id and file columns needed no change:
  `check-requirement-definitions.sh`, `check-line-citations.sh`, `internal/dogfoodflow` and the
  OCM spec test.
- **Limit:** changes whose new rows are adjacent in the committed order still conflict on those
  rows. Examples are two changes that both append to one section, or two new specs appending to
  `INDEX.json` and `README.md`. So does an edit that changes an indexed field next to another
  change's new row, such as a reflow that changes a requirement's first line and so its title.
  Only prose edits that leave every indexed field unchanged are covered. As before, these
  conflicts are resolved by resolving the spec and regenerating.
- **CI:** no `.github/` change is needed.

## Rollback

Revert the change's commits and run `make spec-requirements`. The generator then prints the line
column in id order again, and the readers return to reading it. No state outside the repository
changes.
