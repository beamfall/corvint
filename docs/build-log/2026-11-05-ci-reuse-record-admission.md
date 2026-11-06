# CI reuse after record completion

Owner intent: lane cireuse charter, 2026-11-05, follow-up to V1-4727. Base:
`28a3ff86a2c3d17662765da12cc9251c06466f5f`. Requirement: CIN-V0-7600 amends
AFP-V0-024, AFP-V0-046, AFP-V0-170 and AFP-V0-734 without changing decision shapes,
record formats, exit codes, stderr, committed goldens or workflow files.

## Measured cause and cost

The retained public API responses for PR #99 run `42106672783` show `completed/failure`:
`go-macos` failed after 7,432 seconds (123.87 minutes); every other non-skipped job succeeded.
Its **full** record job was skipped; narrowed record job `127535388505` succeeded after
11 seconds and printed one record. All 16 Linux shards and `go-product` succeeded. The
record's tree `cb64f7c8f2166319713fe00984c2cfd974e73375`, base
`cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`, and head
`e468668f2d2ea198271ce26717f8f8dd6e998203` exactly match the tree and parents of merge-queue
commit `28a3ff86a2c3d17662765da12cc9251c06466f5f` (run `42125954732`).

The charter reported roughly 25 minutes of repeated Linux work. The retained queue jobs
measure 16 successful shards, 539–1,032 seconds each, with the slowest taking 17.2 minutes;
the workflow's start-to-update interval is 18 minutes. These are observations of this run,
not a predicted saving or a post-deployment measurement. The old `admits` required the
**whole run** to be `completed/success` before reading its record. `continue-on-error` on
macOS alone cannot admit a run while it is still running.

## Admission decision

Trust stays with the record job's own log and immutable tree identity. Completion of an
advisory workflow job has no authority over those results. The latest listing must show
exactly one completed successful record, with a positive unique job ID bound to the run
and its head. Its one well-formed record line must match the deciding tree/head/shard
count, the queue target for a reused queue run, and the selection base for narrowed reuse.
Run repository, workflow and event restrictions remain enforced.

The required set is derived from the record conditions already in `ci.yml`: a full record
attests every `go-product-shard (i)`, `0..N-1`; a narrowed record additionally attests
`docs-plan`, `narrowing-verdict` and `go-product`. Each must be completed/success on the
same run/head and have a unique positive ID. Full recording explicitly allows docs-plan
failure and a non-MET narrowing verdict, so it does not attest those jobs' success. Other
checks continue in the deciding workflow; this change grants no static or gate reuse.

Missing, duplicate, renamed/noncanonical or extra shard identities; duplicate IDs; a
failed, cancelled, skipped or unfinished required/record job; unreadable evidence; repeated
or invalid record lines; and mismatched identities all refuse reuse. Precise diagnostics
use the existing `reason` member. Invalid required evidence also cannot supply a delta.
Full-record preference, narrowed same-range admission, push refusal of narrowed records,
and moved-base delta behavior remain intact. Existing refusal assertions were retained;
failed/cancelled/unfinished run fixtures now include the corresponding unsuccessful
required shard rather than treating aggregate run status as its substitute.

## Verification and replay

The four new tests (`TestCINV07600RecordAdmitsBeforeAdvisoryCompletion`,
`TestCINV07600RecordAdmissionFailsClosed`,
`TestCINV07600NarrowedRecordRequiresItsAggregateAndGuards`, and
`TestCINV07600ReplayPR99`) each failed against the base `main.go`; no assertion was weakened.
The whole affected `tools/ci-reuse-plan` package, including existing workflow tests, passed
through `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/ci-reuse-plan/`.

The small tracked `tools/ci-reuse-plan/testdata/record-admission` fixture projects only the
API fields consumed by the tool from the real run/jobs listings (all 37 jobs retained),
plus the exact timestamped record line. Raw inputs and binary/replay outputs remain under
`/home/russell/lanes/cireuse-evidence/`. Replay uses:

```sh
ci-reuse-plan --repository beamfall/corvint --event merge_group \
  --tree cb64f7c8f2166319713fe00984c2cfd974e73375 \
  --head e468668f2d2ea198271ce26717f8f8dd6e998203 \
  --target 28a3ff86a2c3d17662765da12cc9251c06466f5f \
  --base cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419 --shards 16 \
  --runs tools/ci-reuse-plan/testdata/record-admission/runs.json \
  --jobs tools/ci-reuse-plan/testdata/record-admission/jobs \
  --logs tools/ci-reuse-plan/testdata/record-admission/logs
```

The base binary says FULL, reason `no admitted successful run`; the changed binary says
REUSE, scope NARROWED, run `42106672783`, record `127535388505`. A separate replay changes
only the retained run's status/conclusion to `in_progress/null` (a synthetic timing state,
not a historical API observation): the base still says FULL, the changed binary still
says REUSE. Both states have the following decision digests:

| Artifact | SHA-256 |
| --- | --- |
| Old decision | `507b5d5b22623fc85669b70c1f0bce840183f75cbe1b5eea2ec76d95cb7b386a` |
| New decision | `6d69127e13571d665c4b4e1bd7ba615dfde76c66ea500c75e4c356b50ec42c85` |
| Base binary | `35f6a32fdb4e36bf53f3a71010face9986afc8ad1d6ccab34445dbfaa2d56cea` |
| Changed binary | `c52b4560d96671e0afb5f0fcddf589004c30541a8c21e51a2ad842a8fea79999` |
| Raw PR run response | `2ede17cd5598a1695e2e464ba501908604f4d7825c1558374e2cf7694279cf04` |
| Raw PR jobs response | `4c2d60d1cf4d84f96a423866ffa0e96dc86a59c421546410515c7b743c898f25` |
| Raw record job log | `7f4c7c0a4a9dd2f4aa4604de9de97ec3f8ba3a59a5a55de52ddf0721aa619ec3` |
| Raw queue run response | `e8fc6f6c7530bd2f4b88452ee711eaa70df72b5649e6f767e0abd7e577328015` |
| Raw queue jobs response | `b5b4348d3bd39c2ea5ecccc3f7693764283717f681c1e5e5cf7b552fe997c72e` |
| Projected PR run fixture, 331 bytes | `eb1eb35421d61932656e2ab31d557be3b7b808ab32337b43435d542ad87b0a12` |
| Projected jobs fixture, 8,308 bytes | `5069fa86ce59f8006cfe8b06fe99e4f0a06edd16596c0684098c2c34b10f6b82` |
| Record excerpt, 265 bytes | `5345da7a05ac8b5287a8fc5b411cf5f120cc3245b7dd98339e467f7c95a73427` |
| Failing base test output | `36420b622b11263616ad2791b9e7d25ec09aac637254ac0062333f60a4f369ce` |

## Limits, self-use and rollback

Hosted reuse remains NOT_OBSERVED. The workflow's `docs-plan` reuse step still fetches
runs with `status=success`; it must separately omit that query parameter to supply running
or failed candidates to this tool. No `.github/**` file was edited, as required by the
charter. PR #100's advisory conclusion change alone does not remove that filter. The
queue also builds its tool from its first parent, so deployment takes effect only after
this change lands. Rollback restores whole-run completed/success admission and FULL fallback.

Used: `make dogfood-change` at the unchanged base; `make dogfood-prechange` retained context,
query and impact receipts under the worktree's private Git `corvint/` directory. Context
reported two omitted results; query reported four omitted results; impact reported 56 omitted
results and 74 evidence rows omitted by its ceiling. No governing critical path was missing.
`corvint affected --base` selected this package directly but reported scope UNKNOWN and
78 unknown entries; the installed binary reported self-binary-stale (54 differing Go paths).
The host verifier owns broader selection/gates. Retrieval, ranking, learning, transport and
frozen-evaluation changes are not applicable; their evaluations are NOT_RUN. Independent
review is deferred to the orchestrator; no model-backed reviewer was run.

`corvint-tasks ticket show V1-4727` refused because the ticket is not in the local projection
and there is no journal. The suggested staged queue snapshot also lacks it (produced
2026-11-04T17:49:52Z); native ticket state is NOT_OBSERVED, and no Tasks write was attempted.
The supplied charter supplies this work's intent. Broad tests, doc-gates, selection-baseline,
Windows vet, push, PR publication and sealing are NOT_RUN under the lane instructions.
The initial pre-change report's CEM/OCM/outcome steps were NOT_PRODUCED `no-diff-yet`, and
its query packet coverage was NOT_PRODUCED `packet-coverage-unreadable`; the actual separate
prechange receipts were produced. Final CEM binding/checking is recorded in the handback.
