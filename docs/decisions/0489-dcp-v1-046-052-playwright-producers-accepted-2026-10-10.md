# Decision 0489 — Playwright discovery and witness producers (DCP-V1-046..052) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept DCP-V1-046..052").

## Context

GitHub #717 reported that `corvint docs corpus behavior-adapter` needs inputs that nothing in
Corvint produced. The producers lane proposed seven requirements in
`docs/specs/documentation-corpus-v1.md`:

- `DCP-V1-046..048` (V1-1082): `docs corpus discover-playwright` converts a caller-run
  `playwright test --list --reporter=json` report into the canonical
  `corvint-playwright-discovery/1` record. Every anchor is pinned to the migration's source
  revision, and execution identity comes from Playwright's own test IDs or a qualified receipt.
  Inconsistent listings are refused with no output.
- `DCP-V1-049..052` (V1-1083): `docs corpus witness-playwright` imports a qualified Playwright run
  as runtime witnesses. A test is witnessed only when its qualified receipt test passed. Every other
  test stays unwitnessed with one named reason, and missing or untrustworthy shared evidence
  yields uncertainty, never a witness.

Independent review round 1 raised four findings. Commit 9f9d2a80 fixed them: witnesses are
attributed by full title, retries must be explicit, events must be valid, and the documentation
revision is validated. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `DCP-V1-046` through `DCP-V1-052` as written. The requirement statuses, the
`docs/specs/README.md` row and the lane's build log (`docs/build-log/2026-10-10-gh717-producers.md`)
record the acceptance. The spec's overall intent status stays proposed, as in decision 0482: its
other requirements are not accepted by this decision.

## Limits

This decision settles intent only. The evidence is focused tests and the doc gates. Live adopter
qualification is `NOT_OBSERVED`, and `make gate` is `NOT_RUN`.

- Corvint never runs Playwright. The caller supplies the listing and the report.
- V1-1085 (the adopter-specific `/1` revisions member) is not addressed. It conflicts with proposed
  AFU-V1-054..056 and remains an owner fork.

## Rollback

Revert this decision and return the DCP-V1-046..052 status text to proposed, pending owner
acceptance. To withdraw the behavior, also revert the producers change:
`internal/doccorpus/behavior_playwright_discovery.go`, `behavior_playwright_witness.go`,
`internal/liveverify/affected/typescript/playwright_listed_tests.go`, the two subcommands in
`cmd/corvint/docs_corpus_playwright.go` and `docs_corpus.go`. Then regenerate
`docs/specs/REQUIREMENTS.tsv`. No stored state changes: both commands write nothing.
