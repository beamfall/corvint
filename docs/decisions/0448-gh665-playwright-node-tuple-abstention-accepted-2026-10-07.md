# Decision 0448 — typed discovery abstention and the Node v24.11.1 Playwright tuple accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's summary of the issue 665 requirements and its proposed defaults for the lane's owner
questions.

## Context

Issue [665](https://github.com/beamfall/corvint/issues/665) reported that a consumer pinned to Node
v24.11.x never retained Playwright evidence, so test-validity discovery answered `source:"none"` with
no reason. The V1-0976 lane proposed `LPCV-V0-056` in
`docs/specs/live-proof-carrying-verification-v0.md` and `PWP-V0-009` in
`docs/specs/playwright-external-provider-v0.md`, ran the live `/0` matrix once on Node v24.11.1, and
Codex reviewed it for two rounds until it reported no finding
(`docs/build-log/2026-10-07-gh665-playwright-node-tuple-abstention.md`). AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `LPCV-V0-056` (the closed `discovery.abstention` reason set) and `PWP-V0-009`
(exactly Darwin arm64 / Node `v24.11.1` / `@playwright/test@1.63.0` with the bundled headless shell
under `/0`) as written.

The owner also answers the lane's owner questions with the fail-closed defaults:

- Further Node patch releases are admitted one exact release at a time, through the predicate and a
  Corvint change backed by a live run. A consumer-side qualification record is not adopted.
- Only `v24.11.1` is admitted; `v24.11.0` and the rest of `v24.11.x` keep abstaining.
- `/1`, `/2` and `/3` on Node 24 stay unadmitted until each has its own live run.

## Limits

This decision settles intent only. The live run was one `/0` matrix on one macOS arm64 host; the
non-vacuous control assertion added after it, Linux, hosted CI, `/1` to `/3` on Node 24 and
`make gate` were not run. The Node tarball's SHA-256 matched the published value; its GPG signature
was not verified.

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers in both specs, then
regenerate `docs/specs/REQUIREMENTS.tsv`. Reverting the `PWP-V0-009` predicate change makes Node
v24.11.1 runs abstain again; reverting `LPCV-V0-056` removes the `discovery.abstention` member.
