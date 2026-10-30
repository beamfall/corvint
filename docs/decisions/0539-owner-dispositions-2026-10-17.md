# Decision 0539 — Owner dispositions of the 46 blocked tickets, 2026-10-17

Date: 2026-10-17. Status: accepted (owner answers in chat, 2026-10-17). Authority: the owner.
Scope: the tickets named below only. This record is not a standing rule, and it does not extend
any approval to other tickets, later revisions, or release publication.

## Context

On 2026-10-17 the queue held 46 tickets in `MANUAL`, `EXTERNAL` or `APPROVAL_REQUIRED` state. A
read-only classification against `origin/main` 55abb815c sorted them as `ALREADY_DONE` (8),
`OBSOLETE` (1), `DECIDE` (18), `CONSENT` (4), `OWNER_ACTION` (2), `WAITING` (4) and
`EXTERNAL_UPSTREAM` (9). The owner then answered the `DECIDE`, `CONSENT` and release questions.
This record keeps those answers, the evidence for the `ALREADY_DONE` and `OBSOLETE`
classifications, and the items that stay with the owner. The read-only audit of delivered but
open tickets that ran the same day is in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-delivered-open-audit.md`.

## Owner answers

### Evaluation spend

1. **V1-0415** (ContextBench full sample and paired SWE-bench Pro): deferred past 1.0. The RUN grant
   `owner-run-v1-0415-20261008` was issued at ticket revision 1 and is not renewed.
2. **V1-0836** (three-arm sealed CEM reviewer trial): deferred past 1.0. Its dependencies V1-1266
   and V1-1449 are open, and the spend is large.
3. **V1-0573** (CEM 0.4 criterion-level acceptance research): deferred past 1.0. RC3 is CEM 1.0
   only.
4. **V1-0838** (human leg of the reviewer-facing CEM projection study): deferred past 1.0.
5. **V1-0835** (blind-v7 held-out partition): RUN re-granted at ticket revision 2, for case
   construction only. Its preregistered pool and committed-coin method serve V1-0019. Running the
   trial itself is not granted.

### Product policy (11)

1. **V1-0060**: ratify the tolerant reading of `DCP-V1-032` that decision 0337 implemented. A
   dangling-criteria prior does not disqualify a delta.
2. **V1-0631**: the dispatcher does not readmit `RETRY_EXHAUSTED` tickets. Readmission stays
   owner-only under `CAL-V0-043`.
3. **V1-0381**: `dogfood check` binds the impact root by Git content (tree or commit) on both the
   abstention and the normal path, instead of comparing a directory path on one path only.
4. **V1-1327**: the remote corpus HTTP boundary stays operator-owned, loopback by default, with a
   recorded threat model. A qualified authenticated reference deployment is not part of 1.0.
5. **V1-0656**: the optional authority-screen classes and entry-point gaps after PR #467 are 1.0
   non-goals.
6. **V1-0108**: EEP, ETS and EFO stay experimental for 1.0. Adoption is tracked after 1.0.
7. **V1-0994**: the `CCF-V1-008` amendment (maturity label in the Commands list) is accepted.
8. **V1-0574**: the two unavailable original references in Falsifiable Packet V0 are labelled
   private/unavailable; the claims that rest on them stay.
9. **V1-0657**: close PR #458 and keep its local-action pin bypass findings as tickets, after
   checking that the bypass is not reachable on `main`.
10. **V1-0474**: close draft PR #351 and keep its heading-aware navigation pilot evidence.
11. **V1-0232**: a freeze exception for corvint-tasks (frozen after rc.1) covers this
    gate-integrity defect: release gate attestations are checked against the policy argv.

### CI consent

1. **V1-0389**: the ci-control-plane success post on the PR head SHA is granted, for this ticket
   only, so `make traceability-tests-test` can join the doc-gates job. This lifts, for V1-0389
   alone, the earlier refusal to post ci-control-plane statuses.
2. **V1-1097** (per-package test-result reuse by input-closure digest) and **V1-1099** (job-level
   input predicates for the archive, interop and intake jobs): granted, behind a shadow mode
   first. The shadow must show its result before any job is skipped or any result is reused.
3. **V1-0422** (signed archives and a package-manager channel): deferred past 1.0. Decision 0428 (no
   signing) is kept.

### Release

1. **V1-0019**: prepare the validation of the RC on untouched repositories with a preregistered
   pool of candidate repositories, a committed coin and frozen cases. urfave/cli no longer counts
   as held out, because the V1-0859 and V1-0431 fixes were derived from its misses. Publishing
   rc.3 still needs the owner's approval (decision 0491).
2. **V1-0471** (move internal specs and planning to private authority): deferred past 1.0. Published
   tags already contain `docs/specs`, so the move can only be forward-only.
3. **V1-0234** (remove Claude as a contributor by history rewrite): deferred past 1.0, in the same
   window as V1-0471.

## Classifications with evidence

These classifications come from the read-only audit. Each ticket still closes only through its
own native completion, which the orchestrator performs.

### ALREADY_DONE (8)

| Ticket | Evidence |
|---|---|
| V1-1096 | PR #736 (merge fac080be5); `AFP-V0-041` accepted by decision 0488; build log 2026-10-10-ci-test-level-shards. A current hosted shard timing is cited at completion. |
| V1-1094 | PR #744 (merge 26abaafcf); decision 0500; `ci.yml` shards 0..7 and `REUSE_SHARDS` "8". A current hosted run is cited for the 17-minute target. |
| V1-0379 | PR #269 (merge b29d35e53); decision 0002, amendment of 2026-09-26; `LICENSING.md` frontier command file boundary. |
| V1-0184 | PRs #191 (a4d6de0dc) and #199 (086213744); `conformance/use-cases-v0/ledger.json` VERIFIED. Caveat: the runs were agent-run, not owner-run (commit ede9447a6). |
| V1-0259 | Decision 0386, route (b); PRs #187 (`DCW-V0-024`) and #190 (OIF-V0). |
| V1-0398 | `CAL-V0-001..003`, slice S1 of the agent leases spec; build log 2026-09-27-corvint-tasks-non-fixture-writer. |
| V1-1102 | Decision 0431 (merge-queue group size 1); queue enabled 2026-10-12; build cap raised to 3 by the owner on 2026-10-14. |
| CORE-ROADMAP-ADD-COMMIT-FAILURE-1 | Already filed upstream as BUGREV0926-CORE-ROADMAP-ADD-COMMIT-FAILURE-1 (beamfall/core 23ec13f80, open). The Corvint copy links that ID and is archived. |

### OBSOLETE (1)

- **V1-0039** (`AFP-V0-017` qualification at 201 commits): the 201-commit threshold is met (623
  first-parent commits), but decision 0431 item 5 removed the 200-row campaign as a narrowing
  precondition. Narrowing now goes through V1-0811, V1-0812 and V1-1315. The ticket is archived,
  citing decision 0431 item 5.

## Still with the owner

- **V1-0641** (Gemini CLI live qualification): the owner provides non-interactive Gemini or Vertex
  credentials in the local environment. An agent never enters them.
- **V1-0014** (independent proof producers and consumers): waits for an independent implementer;
  post-1.0 under decision 0373 item 7.
- **V1-0020** (publish the 1.0 RC) and **V1-0021** (promote 1.0 stable): publication and promotion
  approval are not delegated (decision 0534 item 10).
- **V1-1315** (`corvint test` cutover): waits for V1-0811, V1-0812 and the `AFP-V0-029` bar, then
  needs `AFP-V0-016` consent.

### Upstream items (listed, not filed)

None of these is filed by this record. Each needs its owning repository's queue:
CORE-GATE-TRANSIENT-CLASSIFIER-PROMPT-1, CORE-INTENT-MINORITY-EVIDENCE-1,
CORE-MULTIREPO-COMPLETION-FENCE-1, CORE-SAME-OWNER-CLOSED-SESSION-REBIND-1,
CORE-MERGE-PRESERVE-ANCESTRY-1 and CORE-ROADMAP-SECTION-DISCOVERY-1 (beamfall/core roadmap);
CORE-CLOSED-SESSION-KEEPER-RESPAWN-1 (owning repository is an owner call);
CODEX-NATIVE-APPROVAL-RELAY-VISIBILITY-1 (openai/codex); and V1-1246 (Claude Code feedback).

## Acceptance recorded with this change

- `CAL-V0-017` amendment and A27 (V1-1081: `complete` also accepts a commit reachable from the
  intent branch's configured remote-tracking upstream, never fetches or moves a ref, and refuses an
  unintegrated commit with `COMMIT_NOT_INTEGRATED`): approved by the owner on 2026-10-16. The
  spec now records it as accepted by this decision.
- `EEP-TR-012..014` (V1-0102: optional provider `capabilities` declaration, the Core check before
  composition, and no widening): approved by the owner on 2026-10-17, after #1053 recorded them as
  unaccepted (decision 0351 had only "a delegated call" on issue #64). The spec now records them
  as accepted by this decision.

## Amendment after review (2026-10-17)

The independent review of #1053 found the V1-1096 and V1-1094 rows overstated. Their code and
decisions are delivered, but the acceptance evidence is not: V1-1096 lacks ten paired shard timing
runs, and V1-1094 lacks three hosted runs of 17 minutes or less. Both are `PARTIAL`, not
`ALREADY_DONE`, and stay open until that evidence is retained. The `ALREADY_DONE` count in the
classification above is therefore 6.

## Limits

This record closes no ticket and starts no work by itself. The deferrals move work past 1.0 and
do not cancel it. Grants apply at the ticket revision named, or the current revision where none is
named. `make gate` was not run for this change (`NOT_RUN`).

## Rollback

Revert this file and its index row. To undo the recorded acceptance, remove the
`(accepted, owner 2026-10-16, decision 0539; V1-1081)` parenthetical from `CAL-V0-017` and restore
"proposed" in the spec's input line, V1-1081 row and A27 entry, then regenerate
`docs/specs/REQUIREMENTS.tsv`. Ticket-level effects are rolled back in the task store by the
orchestrator.
