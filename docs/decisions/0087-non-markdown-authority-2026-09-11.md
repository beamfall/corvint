# Decision 0087 — Markdown stays the only documentation kind that can own authority

Date: 2026-09-11. Status: accepted. Authority: repository owner, delegated 2026-09-11.

## What is decided

1. `documentKind` (`internal/contextindex/parse.go`) stays Markdown-only for the `instructions`,
   `decision`, and `spec` kinds: a non-Markdown documentation file (`.rst` and any other admitted
   documentation suffix) remains retrievable as `documentation` but cannot become a `governing` row
   under `TCP-V0-008` or own a spec-definition row under `TCP-V0-009`.
2. Widening this requires a new numbered decision that names the admitted non-Markdown suffixes and
   their parsing contract; it is not implied by a suffix's admission as searchable documentation
   (decision `0065-documentation-is-searchable-evidence-with-its-own-placement-2026-09-05.md`).

## Why

Authority resolution is deterministic and tested only against Markdown: `documentKind` admits only
`.md` paths to these kinds, and `documentRecord` (same file) reads the title, headings, status, and
summary with the Markdown heading, frontmatter, and fence grammar. Admitting another
grammar (`.rst` sections, or any other format) without its own parser and fixtures would let a
project's choice of documentation syntax outrank its actually-owned authority (AGENTS.md invariant
3: project-owned authority outranks syntax, history, and learned task traces) — the opposite of
what invariant 3 requires.

## What this record does not do

It does not change `documentKind`, the `TCP-V0-008`/`TCP-V0-009` resolvers, or any parsing code; no
non-Markdown suffix is admitted to authority by this record.

Rollback: remove this file and the non-goals sentence it added to
`docs/specs/task-context-packet-v0.md`.
