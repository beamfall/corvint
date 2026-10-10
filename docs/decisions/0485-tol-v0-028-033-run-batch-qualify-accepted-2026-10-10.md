# Decision 0485 — batch runner and qualification verdict (TOL-V0-028..033) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept tol 028-033"), as relayed to the implementing lane by the coordinating agent. The
implementing lane did not observe the owner's words directly; this record cites the relay.

## Context

GitHub beamfall/corvint#714 (native ticket V1-1075) asked for a way to run an obligation batch and
qualify a candidate without an agent session. The lane proposed `TOL-V0-028..033` in
`docs/specs/corvint-tasks-obligation-ledger-v0.md`:
- the closed local run configuration and argv-only command rules (TOL-V0-028);
- the `run-batch` verb, which witnesses through the unchanged witness path and writes one cause
  per ledger entry (TOL-V0-029);
- the clean-checkout rule and the optional pool lane (TOL-V0-030);
- the repeat-failure refusal at one fixture digest (TOL-V0-031);
- the `qualify` verdict and its named reasons (TOL-V0-032);
- the verdict pack, the manifest and the interrupt rules (TOL-V0-033).

Decisions 0456 and 0484 did not cover these additions. AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts `TOL-V0-028..033` with the text as amended by the first independent review
round (Codex round 1 on 80b1f2c7). That amendment states:
- An acquire refusal's code is the job's code. Capture and release run on a fresh context bounded
  by `timeoutSeconds`. A refused release fails the job: `run-batch` witnesses nothing and refuses
  with the release's code, and `qualify` is `NOT_QUALIFIED` with `LANE_FAILED` (TOL-V0-030).
- The fixture digest for `.` is the commit's root tree (TOL-V0-031).
- A neighbour result with retry above 0 is `RETRY` even when the base shares its failure, and
  top-level neighbour report errors disqualify whether or not a test failed (TOL-V0-032).
- The interrupt boundary is the witness submit. An interrupt seen after the report and its source
  evidence are read, but before the witness mutation is submitted, credits nothing and refuses
  `GATE_FAILED` with `LANE_FAILED:`. Once submitted, the witness mutation is atomic. The manifest
  hashes each file as a stream (TOL-V0-033).

The following record the acceptance:
- the spec's intent line, digest status, requirement statuses and traceability rows;
- the matching entries in `docs/specs/README.md` and `docs/specs/INDEX.json`;
- the lane's build log, `docs/build-log/2026-10-10-gh714-run-batch-qualify.md`.

## Limits

This decision settles intent only. The delivery status stays experimental. The evidence is focused
tests with a fake shell test command that writes Playwright-shaped json; the interrupt tests
replace the job context and the release refusal is injected.

The following are `NOT_RUN`:
- a real Playwright suite under `run-batch` and `qualify`;
- a real `SIGINT` or `SIGTERM` delivered to a running job;
- untracked-file change detection by the checkout re-check.

The production qualified-version list stays empty, so a real report still refuses
`UNSUPPORTED_VERSION` and `run-batch` credits nothing until a live fixture qualifies. Test-slot
accounting and releasing the claim with HANDOFF inside the job are not implemented. No native
ticket is completed by this decision.

## Rollback

To roll back the decision only, revert it and return the TOL-V0-028..033 status text to proposed.

To withdraw the behavior as well, also revert the #714 change:
- the `run-batch` and `qualify` verbs and their configuration reader;
- the batch outcome and cause helpers and the store's batch runner helpers.

Then regenerate `docs/specs/REQUIREMENTS.tsv`. Stores are unchanged: the additions write no state
beyond the witness event of the unchanged witness path, and results directories become inert files.
