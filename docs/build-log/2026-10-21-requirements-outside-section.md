# V1-0659: 648 requirement definitions sit outside their spec's Requirements section

Spike ticket V1-0659. Decision delegated to the orchestrator (decision 0548).

## Measurement

Base `90ab9f33e`. A scratch script counted bold requirement definitions (`**PREFIX-Vn-NNN**` at the
start of a list item or paragraph) in every `docs/specs/*.md`. It split them by whether they sit
inside the exact `## Requirements` section. That is the only section `lrfrepo/ocm.go`
`requirementsFromBlob` reads: `sectionEnd` stops at the next `## ` heading.
`SPEC-DRIVEN-DEVELOPMENT.md:101` requires the same form.

| | Definitions |
|---|---|
| Total | 4926 |
| Inside `## Requirements` | 4278 |
| Outside | 648, across 14 specs |

Largest outside counts:

- `cem-stable-v1`: 237 outside, 10 inside.
- `test-source-analysis-v0`: 198 outside, 41 inside.
- `corvint-tasks-agent-leases-v0`: 256 inside, which is exactly `maxObligations`, plus 8 outside.

The ticket's original figure of 79 was taken at an unrecorded revision and is `NOT_OBSERVED` here.
The count has grown since. Definitions outside the section are invisible to OCM obligation coverage,
so dogfood can neither require nor credit them. They are not lost to `REQUIREMENTS.tsv`, which scans
the whole file.

## Proposed recommendation

1. **Ratchet now.** Add a docs check that fails when a spec gains a definition outside
   `## Requirements` beyond its recorded count at this base. The existing 648 become an explicit
   baseline that may only shrink.
2. **Move per spec, in separate lanes.** Restructure each spec so its numbered definitions sit under
   `## Requirements`, one spec per change. `cem-stable-v1` and `test-source-analysis-v0` touch specs
   that open lanes are editing (spec-ocm-citations, cem-batch), so a mass move in this spike would
   conflict with them.
3. **Re-home the leases overflow.** Moving the leases spec's 8 outside definitions would exceed the
   256 cap. Re-home them to domain specs (PSR/ATR), as accepted on 2026-10-17; do not raise the bound.

No code was changed in this spike.
