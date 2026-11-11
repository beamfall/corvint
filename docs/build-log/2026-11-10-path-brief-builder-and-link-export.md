# Path brief builder and requirement-link export (TBC-V1-009 to TBC-V1-012)

Record of the two things built after the owner set the coverage bar of TBC-V1-013 on 2026-11-10
("Accept proposed bar"): the one-time export of the requirement links and the path form of
`corvint context` (change `ticket-brief-context`, PROD-INT-032, ticket V1-5299). Both are
experimental and non-blocking. Nothing here shows that the brief helps an agent.

## Export (TBC-V1-012)

- Program: `internal/pathbrief/cmd/requirement-links-export`, run as
  `requirement-links-export --root ABSOLUTE_ROOT --commit REVISION --out FILE`.
- Input: the `cem/1.0` records under `.corvint/changes` at commit
  `dc84799051004a22992185e61c54c26a28bb9c4d` (`origin/main` when it ran).
- Output: `.corvint/requirement-links.json`, schema `corvint-requirement-links/1`, 1,936,328 bytes,
  sha256 `fd83261bf0a8a32b94288e76b373bc6cd11158a5a80d48e5c1e4dd0c74880674`.
- Two runs on the same input commit gave byte-identical files.

| Count | Value |
| --- | --- |
| Records read | 630 |
| Records skipped | 0 |
| Links (one per path, evidence span and record commit) | 14,331 |
| Links with no requirement ID | 7,423 |
| Links whose path is absent at the input commit | 3,014 |
| Links dropped | 0 |
| Paths | 3,079 |
| Evidence spans | 6,115 |

Links with no requirement ID are 52% of all links. 3,866 of the 6,115 spans hold no definition
line. By link count, 3,414 point at spans with no requirement ID anywhere in them, 2,186 at spans
that only mention an ID, and 1,823 at spans that begin with an ID on a continuation or
cross-reference line. A sample of each kind was read: the spans are prose, tables or
cross-references, so the detection is right and the records simply cite such text. An item for
one of these links names the spec file and span only.

A first scratch run at `f6be4233c` (sha256 beginning `d2be8d88`) is superseded by the run above.

## Builder (TBC-V1-009 to TBC-V1-011)

`corvint context --path PATH [--path PATH ...]` prints one JSON document, profile
`corvint-path-brief/0`, with the HEAD commit and the six sections once per path. Code:
`internal/pathbrief` and `cmd/corvint/context_path.go`; the `--path` branch is in
`cmd/corvint/taskcontext.go`. The ticket form and the ranked form are not changed.

Read by hand on this repository at a clean tree, with a binary built from the branch:

| Section | What was printed |
| --- | --- |
| `instructions` | `OBSERVED`: root `AGENTS.md` and `CLAUDE.md` |
| `owners` | `NOT_OBSERVED`: this repository has no `CODEOWNERS` |
| `tests` | `INCOMPLETE`, `plan-scope-unknown`, test evidence named as missing; 30 to 67 units a path, 4 to 7 shared readers counted |
| `flows` | `NOT_OBSERVED`: `.corvint/flows.json` is absent |
| `requirements` | `INCOMPLETE` `links-not-exhaustive` with items, or `NOT_OBSERVED` `no-link-for-path` |
| `tickets` | `OBSERVED` `open-ticket-scope`: 56 to 121 tickets a path, cut at 64 |

For `cmd/corvint/affected.go`: 64 test units, its own package with reason `DIRECT_SOURCE_CHANGE`;
40 requirement items, 12 with IDs and 28 spans without one, all with `changedSince` true. A call
for three or four paths took about 11 seconds and printed about 78 KB. The refusals for a path
outside the repository, `--base`, `--task`, `--ticket`, task text and a missing value each exit 2.

## Limits found

- The plan is built from the worktree on disk. On a worktree with uncommitted changes outside
  documentation it is refused (`unsupported-affected-mixed-worktree`), so `tests` names
  `affected-plan` as a missing source. An agent in the middle of an edit gets no test list.
- Items are in path or ID order (TBC-V1-010). With more than 64 the cut is alphabetical, so the
  most direct test can fall outside it. A fixed direct-first order would change TBC-V1-010 and is
  the owner's call.
- The first build listed 51 to 63 CI-job units a path, which filled the cut. Make targets, CI jobs
  and frontier sites are now left out of `tests`.
- Retained test evidence was `UNKNOWN` (`inventory-unavailable`, `no-fresh-record`) for every path
  read, so it added no item.
- Open tickets match on broad scope entries such as `internal/` and `docs/specs/`, which is why
  one path lists 56 to 121 of them.
- `flows` has no reader. The completed-ticket source of `requirements` is not read (V1-5301).

## Not run

`make gate`, the exhaustive Go suite, the pilot (TBC-V1-014) and the 1.0.x measurement
(TBC-V1-006). No agent used the brief. The ticket form (TBC-V1-001 to TBC-V1-008) is not started.

## Later the same day: required-field checks

A review found that the export took a record with a readable `spec` and no other required field,
and that the reader took link-file rows with an empty file, a reversed span or no blob. The export
now skips a record that lacks `baseRevision`, `patchSha256`, or an evidence `id`, `path`,
`blobOid`, `span` or `spanSha256`, or a hunk path. The reader refuses a link file with a malformed
commit row, evidence row or path blob. A rerun on the same input commit read 630 records, skipped
0 and gave the same sha256, so the committed file is unchanged.
