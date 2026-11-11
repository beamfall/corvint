# Context replay and requirement-link count behind the path-brief amendment

Finding recorded for the amendment of 2026-11-10 to change `ticket-brief-context` (TBC-V1-009 to
TBC-V1-014, PROD-INT-032; proposed when this was measured, see "Owner acceptance" below). Read at `origin/main`
`f6be4233cf209f251d7a039515c3e832cdad8c33` with the installed `corvint` build 144. Scratch scripts,
not committed. Nothing here proves a requirement of the amendment; it is why the amendment was
drafted.

## Replay: ranked `corvint context` against a term grep

Method: the 60 most recently merged pull requests of this repository, kept when they changed one to
six non-test `.go` files (16 kept, 39 changed files). For each, `corvint context --task TITLE
--limit 8` was run with the pull request title as the task, and its first eight results were
compared with the changed files. The baseline took the title's words of four or more letters, ran
`git grep -l -i -F` for each, weighted a file by the rarity of the words it matched and kept the
first eight.

| Method | Changed files listed (of 39) | Pull requests with at least one (of 16) |
| --- | --- | --- |
| `corvint context` | 9 (0.23) | 7 |
| Term grep | 13 (0.33) | 8 |

Limits: 16 tasks is a small sample. Both methods ran on the tree after the merges and used titles
written after the work, so both figures are higher than a real pre-change run would give. The
ranked packet spends some of its eight results on `AGENTS.md`, tests and docs, which this count
does not credit. No agent ran, so this says nothing about tool calls or task success.

Reading: the ranked packet does not beat a plain grep at naming the files to change. This agrees
with the value audit of 2026-10-26, where agents reached the files at step 0 or 1 unaided. It
supports removing the ranking (TBC-V1-005, accepted plan) and does not support any ranked
replacement.

## Requirement links in the sealed change records

`.corvint/changes` holds 630 `cem/1.0` records with 32,971 hunks and 29,096 basis links: 17,481
`specification`, 6,486 `dependency`, 4,095 `test-claim`, 752 `decision`, 260 `implementation`, 22
`call-site`. Of the 1,847 tracked non-test `.go` files, 586 have a hunk with a basis link and 442
have one whose evidence is a span under `docs/specs`.

Requirement IDs inside those spans: the 18,233 `specification` and `decision` links point at 6,115
distinct evidence spans (median 201 bytes). Read at each span's pinned blob, 11,314 links (62%)
have a span with a line that begins with a requirement ID, 1,992 (11%) have a span that only
mentions an ID, and 4,927 (27%) have a span with no ID. The ID pattern was a scratch regular
expression, not the `REQUIREMENTS.tsv` generator, so the split is approximate.

Commit history as the other source of path-to-requirement links: none of the last 300 commits on
`origin/main` carries a `Ticket:` trailer, and 45 of the 300 non-merge subjects cite a `V1-` ticket
ID.

Reading: the sealed records are the only dense path-to-requirement link store in the repository, and
change `source-removal` deletes them. TBC-V1-012 proposes a one-time export of those links. They are
partial (442 of 1,847 files, and about a quarter of the links name no requirement), so TBC-V1-010
never reports the `requirements` section as complete. The count does not show that an agent does
better with the links; TBC-V1-013 and TBC-V1-014 are the checks for that.

## Owner acceptance

On 2026-11-10 the owner was asked "Do you accept PROD-INT-032 so the coverage check can run?" and
answered "Accept into 1.0". PROD-INT-032 now carries the marker `(accepted 2026-11-10; V1-5299)`
and an `owner` row in `docs/specs/openspec/acceptance-log.tsv`, written by the lane on that
instruction. The answer does not set the coverage bar of TBC-V1-013 and does not answer the other
questions listed in the change's `design.md`; those stay open.

## Not run

No Go test, no `make gate` and no agent run: the change is prose under `docs/` and one generated
index row.
