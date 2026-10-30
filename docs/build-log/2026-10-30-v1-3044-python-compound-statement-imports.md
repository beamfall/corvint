# Python imports in one-line compound statements — V1-3044

The affected Python scanner did not read an import that forms the body of a one-line compound
statement, such as `if x: import a`, `try: import a` or `except ImportError: import b`. As a
result, a test whose only import of a changed module sat in that position was not selected.

## Contract and implementation

The AFP-V0-680 amendment covers this case and needs no new ID. The ID space of
`affected-plan-v0.md` is constrained:
- the `## Requirements` section has 255 of the 256 OCM obligations;
- `AFP max + 500` overflows the three-digit ID form.

A piece that starts with a compound keyword is now read as follows:
- The header runs to the first colon that is outside brackets and string literals and is not the
  walrus `:=`. Everything after that colon is the body, and an import body adds its edges.
- An import read this way raises `python:conditional-import`, the same flag an indented import
  raises.
- If no header colon is found, the line stays unread, which is the previous behaviour.

## Executed evidence

- `TestAFPV0680CompoundStatementBodyIsAnImport` passes.
  - It has 12 positive cases.
  - It has 4 negative cases: `if x: pass`, a string, a dict key and `iffy: int`.
  - It fails on the base code.
- The `internal/liveverify/affected/python` package passes.

## Rollback

Revert the commit. The scanner then ignores these imports again.
