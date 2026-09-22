# Decision 0044 — perf grants for DR-0009 on the three Beamfall invocations left ungranted

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction "get corvint
to release" (2026-09-04), applied to the one packet-5 obstacle that a grant can remove. This record
grants nothing the diagnosis below did not reproduce, and it does not pre-judge the thresholds.

## What is ratified

`conformance/perf-v0` may measure three tasks across the recorded output difference of `DR-0009`,
each grant relaxing `unequal-stdout` and nothing else:

| Task | Entry | Argv |
|---|---|---|
| `beamfall-harness-file-change` | `DR-0009` | `harness event … --event file-change --input -` |
| `beamfall-harness-file-change-snapshot-present` | `DR-0009` | the same argv, on the `beamfall-snapshot-present` corpus |
| `beamfall-impact-path-snapshot-present` | `DR-0009` | `impact internal/graph/graph.go --limit 10`, on the `beamfall-snapshot-present` corpus |

The existing grants (decision 0007 item D9, decisions 0040 and 0041) are unchanged and not widened.

## The evidence the grants rest on

The 2026-09-03 run3 (`conformance/perf-v0/results/packet-5-current-pin-requalification-2026-09-03-run3/`,
the first under decision 0042's 120 s timeout) left exactly these three tasks
`insufficient_evidence`, each with the single reason `unequal-stdout` over 100 measured samples per
runtime: no timeout, no sample failure, no exit-status difference, and a stable oracle. Each was
then reproduced on 2026-09-04 with `go run ./conformance/perf-v0 diagnose -task <id>`, which
materialises the same archived corpus at the same pin under the same sanitized environment and
diffs the two stdout documents member by member. All three differ in exactly four members and no
others:

- `beamfall-harness-file-change` and `beamfall-harness-file-change-snapshot-present`:
  `coverage.requested_results` 10 versus 205, `coverage.omitted_results` 5 versus 200, the
  `uncertainty[0]` line "5 ranked results omitted by packet budget" versus "200 …", and
  `coverage.packet_bytes` 5,444 versus 5,449 as their consequence. Oracle stdout
  `68d461fc…`, candidate stdout `61a930b1…`, identical on both corpora.
- `beamfall-impact-path-snapshot-present`: `coverage.requested_results` 10 versus 205,
  `coverage.omitted_results` 0 versus 195, one extra candidate `uncertainty` line, and
  `coverage.packet_bytes` 12,682 versus 12,730. Oracle stdout `d947234a…`, candidate stdout
  `2e0c1f6b…` — byte-identical to the cold `beamfall-impact-path` pair decision 0040 already
  diagnosed, which is what makes the snapshot-present task the same divergence and not a new one.

That is `DR-0009` in every member: the oracle denominates coverage in its own truncated output, so
its `omitted_results` is structurally zero and its `requested_results` is the limit; `GPK-V0-040`
requires the denomination the candidate uses. The entry is `LANDED` / `python-defect`, `src/` is
frozen and deliberately unrepaired, so no repair to either runtime can close it. The ranked result
ids are identical in both runtimes on every task. No `DR-0004` marker permutation appeared in the
snapshot-present impact sample or across run3's 100 samples, so no `DR-0004` grant is written for
it; if a later run shows one, that is its own diagnosis.

## What this record does not do

It does not authorise a packet-5 retry. Decision 0016's `P5R-V0-004` requires a retry to carry a
new prospective preregistration and a new Corvint pin beside the grants; neither is written here,
because run3 also shows that the index-building harness events (`file-change` and compact
`session-start`) sit at 835–1,080 ms p95 on both corpora against `GPK-V0-017(a)`'s 100 ms cap, so a
retry under the current clause would measure `FAIL`, not `PASS`. Whether that clause is amended for
index-building events, or the alpha ships with a measured `FAIL`, is an owner call recorded in
`docs/agent-memory/questions.md`; the grants here are needed under either answer. It does not
repair `src/`, widen any other grant, or change a threshold.

## Rollback

Remove the three `decision 0044` grants from `ratifiedAcceptedDivergences` in
`conformance/perf-v0/manifest.go`, the matching rows in `TestOnlyRatifiedGrantsAreInTheSet` and
`TestCommittedManifestKeysOnlyTheRegisteredTasks`, and the `acceptedDivergences` keys from the three
tasks in `conformance/perf-v0/manifest.json`. The tasks return to `insufficient_evidence` on the
next run, which is exactly what run3 reported.
