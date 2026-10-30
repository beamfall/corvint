# Decision 0349 — Export admitted learned rules as SKILL.md documents

Date: 2026-09-22. Status: accepted; experimental delivery. Adds `LTA-V0-006` to `LTA-V0-008` to
`docs/specs/learned-trace-admission-v0.md`; it changes no trace schema, cap, or scoring constant.

## Context

Ticket V1-0095: admitted learned traces had no export format a host could consume. Corvint's
local store holds schema-1 rows (task, opened and changed paths, verification commands, outcome)
that `record` admits through the writer screens and the reader re-validates on every read. The
learned-path evaluation gate (`LTA-V0-001`) admits or rejects the mechanism at build time and
never an individual row, so the repository holds no per-row admission record and no per-row
evaluation result. Claude Code and Codex both load skills through the Agent Skills contract: a
directory whose `SKILL.md` carries YAML frontmatter (`name` of 1 to 64 lowercase alphanumerics
and single hyphens equal to the directory, `description` of 1 to 1,024 characters) and a body,
with detail in `references/` loaded on demand.

## Decision

1. `corvint [--root PATH] skill-export --out DIR` is a new explicit read-only verb. It reads the
   pinned trace store and committed index through `tracerecordrepo.Read`, writes one
   `DIR/corvint-learned-<16 trace-id hex>/` per admitted rule holding `SKILL.md` and
   `references/trace.md`, and prints a JSON manifest. It refuses a `DIR` inside `.corvint` and
   touches no repository or trace state (`LTA-V0-006`).
2. An admitted learned rule is a stored row with outcome `passed` that the reader re-validated.
   The admission evidence digest is `sha256:` over the exact stored row bytes (`trace.Encode`),
   the same bytes the reader screened; the digest is derived, not stored, because no separate
   admission record exists. The evaluation result is emitted verbatim as `NOT_RECORDED`, the only
   honest value V0 can name; `failed` and `blocked` rows and rows whose digest does not
   re-validate are not exported (`LTA-V0-007`). The row's own admission does not require a
   per-row evaluation, so `NOT_RECORDED` rows remain exportable.
3. Progressive disclosure: `SKILL.md` holds the frontmatter, the task, and the provenance lines;
   `references/trace.md` holds the opened and changed paths and verification commands. The
   description folds the task onto one printable line bounded at 200 runes (`LTA-V0-008`).
4. The host round-trip fixture is a Go test whose parser mirrors a loader's frontmatter read and
   the Agent Skills bounds. No real Claude Code or Codex host was run; that remains a follow-up.

## Rollback

Remove the verb, `internal/skillexport`, and the three requirements. Exported directories are
operator-owned copies outside the repository; no trace, row, index, or frozen partition changes.
