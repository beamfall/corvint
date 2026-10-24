# p1-skh-b: site knowledge stage 7 — the guarded how draft (V1-1904)

Date: 2026-10-24. Lane: local p1-skh-b (V1-1904, stacked on #1705).
Spec: `docs/specs/site-knowledge-howto-v0.md`. Platform for every result: linux/WSL2 (go1.27.3).

## Owner authority

`SKH-V0-070`..`074` are accepted under the owner's standing auto-accept (2026-10-11). The
refinements `SKH-V0-153`..`162` come from this lane's reserved block and are experimental.

## Decisions

- **Encoding: a `draft` head field on `site-knowledge-how/0`.** Status, label, sorted gaps, source
  test, proposed path, guard line, lines, one evidence row per line (`copied` with path, source
  line and blob; `guard`; `todo` with its gap code) and the SHA-256 digest of the lines. No new
  schema name: the draft is an optional head field present only on the draft surfaces.
- **Copy the primary test's whole file, not a step list.** Copying the file minus the other mined
  tests' spans keeps imports, `test.use`, `describe` wrappers, fixtures and page-object calls in
  the suite's own conventions with no synthesis. Composing statements from several tests would
  need control flow the generator may not write (`SKH-V0-072`, V1-1677 kill).
- **Parameter-slot substitution is the identity in V0.** The primary test already reaches the goal
  with its own values; rewriting a value would be a synthesised value. Recorded as a V0 limit.
- **Surface: extend `flows appmap plan --request --draft` and `corvint.map_plan`.** `--source`
  switches the request to one goal and the output to the how answer with its draft; the AMSP-V0
  planner path is unchanged without `--source`. No parallel generator command exists.
- **The read writes nothing.** `SKH-V0-074`'s caller-named output is served by the caller or the
  stage 8 companion writing the printed lines; core keeps invariant 4.

## Evidence (linux/WSL2)

- `go test ./internal/appmap/` (SKH-V0 filter), `./cmd/corvint-corpus-mcp/`, and `./cmd/corvint/`
  filtered to appmap, flows, help, capability and AMSP tests: pass.

## Rollback

Revert the stage 7 commit; the how answer loses its `draft` field and `flows appmap plan` and
`corvint.map_plan` refuse `--source` / `sources` again. No stored state depends on it.
