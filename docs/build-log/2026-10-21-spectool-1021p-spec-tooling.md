# 2026-10-21 spectool-1021p: spec tooling diagnostics and README ranges

Tickets: V1-2168, V1-2160, V1-2146. Requirements SRG-V0-042, SRG-V0-043 and RDG-V0-022 are
accepted under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-2168: `script/check-requirement-definitions.sh` silently skipped a line shaped
  `` - `ID` (V1-x): text ``, so the id was never defined and nothing named the line.
- V1-2160: `acceptance()` in `script/gen-spec-requirements.sh` silently classified near-miss
  acceptance notes. On main, 74 definitions carried one: 10 "owner lane auto-acceptance" notes read
  as `owner` instead of `standing`, 8 ATR `(accepted, V1-1547` notes and 4 SBQ "AT-07 accepted
  contract" notes had no decision number or date, and 52 notes were unparenthesized
  (`accepted, decision 0506. Text`), so they read as `unrecorded`.
- V1-2146: the Contract-cell id range in `docs/specs/README.md` was hand-written and had drifted in
  13 rows (for example STR, PSR, BBF, DCP-V1).

## Decisions

- RDG-V0-022 flags the misshapen shape only when the id has no definition elsewhere, so traceability
  notes such as the three in `task-context-packet-v0.md` keep passing.
- SRG-V0-042 makes `make spec-requirements` refuse the three near-miss classes with
  `<path>:<line>: <ID>: <reason>` and the accepted forms. The 74 corpus notes were rewritten to an
  accepted form in the same change. Each rewrite keeps the recorded authority: the DCW/LCP and ATR
  notes name the standing auto-accept that their spec headers already record for 2026-10-11, and the
  SBQ notes name decision 0052, which the spec identifies as AT-07. The remaining rewrites only add
  the parentheses.
- SRG-V0-043 regenerates a README row's single trailing `PREFIX-NNN[..NNN]` range from the
  definitions the row's spec holds. A row that shows no range is left alone.
- `docs/SPEC-DRIVEN-DEVELOPMENT.md` lists the accepted definition and acceptance-note forms and the
  near misses.

## Evidence

- `script/gen-spec-requirements_test.sh` passes. Cases 6, 7 and 11 cover the exact forms, the near
  misses, and range rewrite and refusal. They fail against the base generator.
- `script/check-requirement-definitions_test.sh` passes. The RDG-V0-022 case fails against the base
  gate.
- On the regenerated corpus, 74 TSV acceptance rows changed mode and 13 README ranges were
  corrected. `INDEX.json` is unchanged apart from the new requirements.

## NOT_RUN

- Tier 2 of `corvint affected` is left for batch CI.
- Not diagnosed: 31 TQD-V0 definitions keep a trailing `Status: (accepted ...)` note, which still
  reads as `unrecorded`. 182 README rows show no range at all, and none is added.
- `script/check-requirement-definitions_test.sh` is still not wired into a make target.

## Rollback

Revert the commit. The generator and gate changes are independent of each other. Reverting the
corpus rewrites returns the 74 rows to their previous modes.
