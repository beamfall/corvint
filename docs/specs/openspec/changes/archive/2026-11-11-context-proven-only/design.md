# Design: `context-proven-only`

## Decisions

- What counts as proven: a surface with a recorded measurement or an outside result behind it, or a
  deterministic projection of project-owned facts (instruction files, linked tests, kept requirement
  links, the affected plan, Git ancestry). Ranked, inferred or summarised context is not proven
  (decision 0597, Evidence).
- `context` stays a Core verb because the path brief and the instruction alias live under it and
  decision 0596 (O39) keeps `corvint context` in the release smoke.
- The instruction doctor gets its own verb, `corvint instructions`, because its only tie to
  `context --task` was the packet it attached to. The new verb takes `--subject`, `--host`,
  `--host-version`, `--cwd` and `--profile`, and prints the same `instruction_load_set` object.
- The path brief keeps three of six sections. `owners`, `flows` and `tickets` have no measurement
  behind them (the link-coverage check of 2026-11-10 measured only `tests` and `requirements`), and
  the completed-ticket and claim-scope matches are path prefix matches over Tasks fields. The
  `requirements` section reads only `.corvint/requirement-links.json`.
- `review` keeps what is a fact of the diff and the refs: the affected receipt (RGV-V0-007, 013, 014,
  015) and the branch overlap (RGV-V0-008 to 011). The feature list was inferred by regex
  (RGV-V0-003, 004) and goes, with `overview` and `features`.
- The shared substrate rule: in `internal/contextindex` the lane deletes the files the charter names
  and every symbol that becomes unreachable, and keeps any file a KEEP user still reaches. The
  snapshot pack sections (term table, co-change, recency) are part of the persisted index encoding;
  removing them is a format change outside this change.
- `.github` stays out of the code branch (decision 0594 item 7); a second branch carries only the
  CI shard table rows for deleted tests and packages.

## Order

Six steps, one commit each, after the commit of this folder: surfaces and strings; the instruction
verb and the path brief; the bench layer; the ranking and trace code; the specs and build log; the
CI shard tables on their own branch.

## Verification

Build and vet after each step, focused tests of edited packages that still exist, the doc gates, and
`make windows-vet` once at the end. No `go test ./...` and no `make gate` (build-only lane).

## Rollback

Revert the merge commit. Nothing is migrated.

## Canonical spec copy

The canonical specs stay flat in `docs/specs/*.md` (H13), so the deltas under `specs/` are copied
by hand in step 5 and the change is archived with `--skip-specs`:

- `ticket-brief`: each statement replaces the text of the same ID in `ticket-brief-v1.md`.
- `agent-instruction-doctor`: the statement replaces IID-V0-001 in
  `agent-instruction-doctor-v0.md`.
- `repository-guidance`: the MODIFIED statements replace their IDs in
  `repository-guidance-v0.md`, and each REMOVED ID is marked retired there.
- `core-compatibility-freeze`: an amendment note under CCF-V1-002 in
  `core-compatibility-freeze-v1.md`, in the form of the existing decision 0594 notes.
- `retired-context-families`: each family's spec header is marked retired in the form of
  `falsifiable-packet-v0.md`; every ID stays listed as retired and is never reused.

Each amended statement carries the note `(amended 2026-11-11, decision 0597)`.
