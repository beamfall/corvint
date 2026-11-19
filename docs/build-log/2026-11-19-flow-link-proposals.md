# Flow link proposals and reviewed selection

V1-5695 delivered OpenSpec tasks 3 through 7 at `ba25d3ca2c57ae87a8405558fae1f2587628281a`.
Tasks 1 and 2 retain their existing proof. Task 8 remains open for a person's acceptance.

## Decisions

The amendment at `42891cab6a013d55a2d3e119f1d7ef893763971f` records A1–A4. G6 resolves each
variation through the intent directory named by the declaration. The link binds the canonical
JSON of that entire record at the pinned commit. Member order and whitespace have no effect.
Changes to a value, a missing member or array order change its SHA-256.

Formula `flow-link-confidence/1` assigns 0.35 to a title match, 0.25 to a flow tag and 0.40 to
static reach to a declared source. It retains top-scoring variations per eligible test, including
ties. Missing inputs contribute zero and appear as `absent`. Scores only order proposals for review.

Selection uses the committed inventory and accepted records. Both the original binding and the
candidate must agree. Unknown reachability, disagreement, stale bindings and missing targets retain
the obligation. A pytest browser test keeps its whole runner instance. Playwright plan entries
carry the selected canonical IDs, including the unassigned test in a mixed file. A file-level
runner or an independent whole-instance obligation may execute additional tests.

## Verification

The eleven rows in the capability spec's traceability table resolve to declared test functions.
The whole appflows, flows CLI and capability catalog packages passed. The fixtures cover private
writes, repeatable scores, terminal acceptance and refusals, mixed-file selection, whole-group
fallback and stale or dangling records in the plan and reports. Native and Windows vet passed.

## Corvint self-use

At the implementation commit, `corvint-docs flows propose --links` exited 0 with empty stderr.
The pinned tree had neither `.corvint/flows.json` nor `.corvint/test-inventory.json`.
Stdout contained the following JSON, formatted here for readability.

```json
{
  "profile": "corvint-flow-link-proposals/0",
  "pinned_commit": "ba25d3ca2c57ae87a8405558fae1f2587628281a",
  "proposals": []
}
```

Stdout and the private worktree Git file were byte-identical at 118 bytes including the final LF.
Their SHA-256 was `cd54a3f05a4942201f48b5e41e1146a6915ed60d09aa750383b76df46712d9ec`.
The private file had mode 0600, and Git reported a clean worktree before and after. No repository
acceptance was performed. The self-use run provides evidence for the empty candidate set only.
