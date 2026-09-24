# Decision 0376: Owner acceptance of DCW-V0-018/019, AFP-V0-021 and the receipt bundle intent

Date: 2026-09-24. Status: accepted (owner answer "approved to all", 2026-09-24). Authority: the
repository owner approved a listed set of pending acceptances in one answer. This record carries
the specification part of that answer; the rest (release steps, branch cleanup, merges) is not a
specification change and is not recorded here.

## Accepted

1. `DCW-V0-018` (OCM links only from a row of the optional `DOGFOOD_OCM_LINKS` plan) and
   `DCW-V0-019` (a nonempty `DOGFOOD_CITATIONS` plan must match the prepared map before any cite)
   in `docs/specs/daily-change-evidence-workflow-v0.md`. They were delivered in tickets V1-0142
   and V1-0173 respectively, with the evidence in that spec's traceability table.
2. `AFP-V0-021` (the literal-reader rule selects every unit whose own files name a dirty path,
   witness kind `PATH_LITERAL_READER`) in `docs/specs/affected-plan-v0.md`, ticket V1-0126.
3. The intent of `docs/specs/receipt-bundle-v0.md` (ticket V1-0197), with its two ticket
   deviations confirmed: the bundle carries the GOC-V0-010 full-gate receipt instead of the gate
   ledger, which GL-V0-006 keeps out of product paths; and only a directory is delivered, no
   archive.

## Not accepted here

`DCW-V0-016` (packet coverage entry, ticket V1-0200) was not in the approved list and stays
proposed. Accepting intent is not delivery qualification, measured promotion, or a release claim:
each spec keeps its delivery status (`experimental`) and its `NOT_OBSERVED` rows.

## Effects

The four specs' status lines, `docs/specs/README.md` and `docs/specs/INDEX.json` name this
decision. `docs/specs/REQUIREMENTS.tsv` is regenerated.

## Rollback

Revert the commit that introduced this decision. The status lines return to `proposed` and no code,
wire contract or persisted state changes.
