# V1-1983: docs corpus refusals name their span, and oversized paragraphs stay verbatim

Date: 2026-10-19
Ticket: V1-1983 (found while measuring V1-1982).
Requirement: DCP-V1-146 (`docs/specs/documentation-corpus-v1.md`), accepted under the owner's
standing auto-accept of lane requirements (2026-10-11).

## Problem

`corvint docs corpus build` on Corvint's own `docs` scope at 547d26417 refused with
`native paragraph bound exceeded` under `corvint-corpus-input/1` and `vacuous or oversized anchor`
under `/2`. Neither message named a file. In a scope of 1,641 Markdown pages, the author had no way to
find the offending page. A scan found three paragraphs over the 64 KiB excerpt bound, all large
Markdown tables:

| Path | Lines | Bytes |
|---|---|---|
| `docs/decisions/README.md` | 3–434 | 175,141 |
| `docs/specs/README.md` | 7–235 | 134,983 |
| `docs/specs/falsifiable-packet-v0.md` | 55–933 | 82,472 |

## Decisions

- **Refusals are located.** An anchor refusal from the native compiler now appends
  `at <path>:<start>-<end>`. The record-bound refusal names the bound and the path and line where
  the bound was crossed. Under `/1` it also says that a `/2` manifest raises the bound. The error
  code is still `corpus-refused`.
- **An oversized paragraph is split, not truncated.** A blank-line paragraph over 64 KiB becomes
  consecutive line-aligned verbatim excerpts, each within the bound. Each excerpt keeps the
  existing `…:paragraph:<start line>` claim ID. Joined in order, the excerpts reproduce the
  paragraph byte for byte. The 64 KiB excerpt bound and "refuse, never truncate" both still hold.
  A single line over the bound still refuses, now with its location.
- **No identity change for existing corpora.** Paragraphs within the bound produce the same claims
  as before. Any input that is now split was refused before, so no artifact that built previously
  changes.

## Measurement

At 547d26417 with the patched binary, on Corvint's `docs` scope:

| Manifest | Outcome | Time | Peak RSS |
|---|---|---|---|
| `/1` | `native paragraph bound exceeded: more than 4096 records at docs/TASKS-EXTERNAL-AGENTS.md:806; a corvint-corpus-input/2 manifest raises the bound` | 3.9 s | 0.89 GB |
| `/2` | builds | 8.1 s | 1.53 GB |

The `/1` refusal is intended: the scope has about 30,700 paragraphs, against the legacy bound of
4,096 records.

## Evidence

- `TestDCPV1146OversizedParagraphStaysVerbatim` and `TestDCPV1146RefusalsNameTheSpan` both fail
  on the unpatched compiler, with the unlocated `vacuous or oversized anchor`.

## Rollback

Revert the change. Oversized paragraphs then refuse the build again, and refusals lose their
location. No artifact or wire schema changes.
