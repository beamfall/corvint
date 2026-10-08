# 2026-10-08: owner acceptances for V1-0523, V1-0465, V1-0506 and V1-0449

## Authority

Owner approval in chat, 2026-10-08: "approve all, and you can self approve these tickets". The
change is docs-only; no code, ticket store, Beamfall workspace or Flow-Proof store was touched.

## Recorded acceptances

| Ticket | Decision | Contract | Intent change | Delivery |
|---|---|---|---|---|
| V1-0523 | 0450 | `dogfood-query-abstention-v0.md` QAT-V0-001..005 | proposed -> accepted | experimental (unchanged) |
| V1-0465 | 0451 | `flow-document-maintenance-v0.md` FDM-V0-001..007 | proposed -> accepted | experimental (unchanged) |
| V1-0506 | 0452 | Pi tuple amendment in `local-completion-policy-v0.md` LCP-V0-008/009 and `agent-harness-integration-v0.md` AHI-024 | accepted direction, plus the amendment recorded as accepted | implemented / experimental (unchanged) |
| V1-0449 | 0453 | Tasks takeover of the Beamfall production queue | decision only | not executed |

Status strings agree across each spec header, Agent digest, `docs/specs/README.md` and
`docs/specs/INDEX.json`. None of the four contracts marked individual requirements "proposed", so
no requirement line changed.

## Why delivery did not change

- QAT: the spec's evidence is focused synthetic tests; independent implementation review and the
  live final original-task CLI qualification are recorded as not produced.
- FDM: release-boundary qualification, docs-MCP parity and concurrent-process qualification are
  NOT_RUN, and the spec's promotion condition needs them.
- Pi: the adapter stays FALLBACK; continuation still requires exact-host qualification, and the
  protected profile stays unadmitted.
- Beamfall takeover: the takeover remains a manual owner step. CAL-V0-027 states it does not
  switch Beamfall, and decision 0426's qualification obligations still apply.

## Evidence and limits

Doc gates ran on the staged change (see the lane report). `corvint context` for this task selected
only `docs/BUILD-LOG.md` and `docs/specs/README.md`, not the four governing specs. No ticket was
completed; each stays open for its own native closeout. `make gate` was NOT_RUN (docs-only).

## Rollback

Revert this commit. The four specs return to their previous intent strings and the decisions
0450..0453 are removed; regenerate `docs/specs/REQUIREMENTS.tsv`.
