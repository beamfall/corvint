# Design: flow-link-proposals

## Decisions

1. **Propose.** `corvint-docs flows propose --links [--flow ID] [--all]` reads the flows in
   `.corvint/flows.json`, the test inventory (TIV-V1-001), stored run evidence (RVD-V1-007) and
   static reachability, and proposes for each unassigned E2E test the flow variations it most likely
   exercises. `--all` proposes for every E2E test in the inventory (bulk bootstrap). It runs no
   tests.

2. **Private proposals file.** Proposals are written to `<git-dir>/corvint/flow-link-proposals.json`
   (profile `corvint-flow-link-proposals/0`), replacing the previous file. It is never committed and
   never under `.corvint/`, so it never forces a full run (owner decision R31) and never reaches
   selection.

3. **Confidence score.** Each proposal holds `confidence` from 0.00 to 1.00 with two decimals and
   `confidenceFormula` naming the formula version. The formula is deterministic: the same inputs give
   the same score. Inputs are cited evidence items, each with a fixed weight, for example: routes the
   test visited in a stored run match the variation's routes; static reachability from the test file
   reaches the variation's entry symbols; the test title path matches the flow or variation name; a
   tag in the test source names the flow. The weights are Open 1. The score orders and filters
   proposals for review; it never accepts, and never enters selection.

4. **Accept.** `corvint-docs flows accept --flow ID --by NAME [--variation V] [--only TEST_ID...]`
   copies the chosen proposals of one flow into `.corvint/flows.json` as link records, with
   `accepted_by` = NAME and `accepted_at` = the current UTC time. Acceptance is per flow. It refuses
   with `flow-accept-needs-person` when standard input is not a terminal, and no MCP tool, hook,
   Tasks gate or Action step calls it. Corvint cannot prove that a person ran the command; the review
   of the commit that changes `.corvint/flows.json` is the check. Whether the terminal check is kept
   is Open 2.

5. **Link record.** `{test_id, variation_id, evidence[], confidence, confidenceFormula, accepted_by,
   accepted_at, pinned_commit, test_content_hash, routes_digest}`. `test_id` is the canonical test ID
   (CTI-V1-005 for Playwright, which holds the file path and title path). `pinned_commit` is HEAD at
   acceptance. `test_content_hash` is the blob digest of the test file at that commit;
   `routes_digest` is the digest of the variation's routes at that commit.

6. **Only accepted links narrow.** Selection reads link records from `.corvint/flows.json` and
   nothing else. A test with no accepted link is `unassigned` and always runs (G11). Tags in test
   source are evidence for proposals only.

7. **Staleness and agreement.** A link is stale when the test file's blob digest differs from
   `test_content_hash` or the variation's routes digest differs from `routes_digest`; a stale link's
   test runs as `unassigned` with reason `link-stale`. A link whose test is not reached by static
   reachability from the changed files while its flow is, or the reverse, does not narrow: the test
   runs with reason `link-reachability-disagrees`. A link whose test ID is not in the inventory or
   whose variation no longer exists is dangling, reported with `link-dangling`, and never narrows.

8. **Other browser runners.** pytest with pytest-playwright and RSpec system tests are kind `e2e`
   (G13). Their links may be proposed and accepted, but in 1.0 they select their whole group with a
   reason; only Playwright narrows by links.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| No proposals file | `flows accept` refuses with `flow-proposals-absent`; selection unaffected. |
| Proposals file from another commit | `flows accept` refuses with `flow-proposals-stale`; rerun propose. |
| Accept run from a script without a terminal | Refused, `flow-accept-needs-person`. |
| Test file changed after acceptance | Link stale; test runs as `unassigned`, `link-stale`. |
| Routes of the variation changed | Link stale; test runs, `link-stale`. |
| Reachability disagrees with the link | Test runs, `link-reachability-disagrees`. |
| Test renamed or deleted | Link dangling, reported, `link-dangling`; never narrows. |
| Unknown flow file version | Refused per the G6 schema rules (owned elsewhere); every E2E test runs. |
| Score inputs missing (no run evidence) | Score computed from the remaining evidence; missing inputs listed. |

## Acceptance evidence

- PROD-DOCS-012 scenario: `flows propose --links` on a fixture with Playwright tests shows proposals
  with evidence and scores, and a plan made before acceptance runs every E2E test as `unassigned`.
- After `flows accept --flow F --by NAME`, the plan narrows only through F's links, with tier `flow`.
- Stale, disagreeing and dangling fixtures each run the test with their reason.
- A determinism test: two proposal runs on the same inputs give identical scores.
- Proposals and acceptance recorded on Corvint's own flows (PROD-DOCS-012 acceptance evidence).

## Rollback

Revert the change's commits. Link records already in `.corvint/flows.json` stay as data; with the
reader reverted they are ignored, and every E2E test runs as before. The private proposals file can be
deleted. FLP-V1-001 to FLP-V1-011 stay listed as retired if the change is withdrawn.

## Open

1. The confidence formula's inputs and weights.
2. Whether `flows accept` keeps the terminal check.
3. The proposals file name and profile.
