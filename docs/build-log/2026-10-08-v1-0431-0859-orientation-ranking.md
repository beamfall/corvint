# 2026-10-08: record-data gate in the subjectless lexical fill (V1-0431, V1-0859)

Human-owned intent: tickets V1-0431 (lexical rows score flat; a paired test is not admitted ahead
of unrelated tests) and V1-0859 (documentation and non-Go lexical matches cut by source-first
ordering). The 2026-10-06 amendment (`2026-10-06-v1-0859-orientation-misses.md`, TCP-V0-013 and
TCP-V0-059 to TCP-V0-061) already landed the strength score, the merged documentation order with
its share and gate, the omission statement and the Go pair promotion. This entry diagnoses what
origin/main (`3e501e17`) still misses and records one further, proposed rule.

Spec: `docs/specs/task-context-packet-v0.md`, new TCP-V0-063 (proposed, experimental).

## Diagnosis on origin/main

Reproduction: the origin/main build run at limit 20 on the eight rc.2 development cases (urfave/cli
277cbea3 and 4c05a5cb; Corvint fc594fa6, f451a34f, f7f586bd, d867347d, 4c607266, 57f06a6a) and on
the five-pull-request replay of the 2026-10-06 entry (#571, #614, #594, #557, #610), with
`CORVINT_CONTEXT_GRAPH` and `CORVINT_CONTEXT_RECENCY` unset. It reproduces that entry exactly:
rc.2 critical rows carried 16/20 (urfave/cli) and 13/47 (Corvint), ten treatment-only misses, and
the replay 18/59.

- V1-0431 criterion 1 (strength score) and criterion 2 (paired test ahead of unrelated tests) are
  met on main by TCP-V0-060 and `admitLexicalPairs`; no residual case is a flat-score or pairing
  miss.
- The ten residual treatment-only misses: two weak urfave/cli code files (`flag_impl.go`,
  `flag_bool_with_inverse.go`, code ranks 37 and 38) dropped by the limit; documentation cut by
  the share's gate (`docs/DOGFOOD.md`, `README.md`, `agent-harness-integration-v0.md`,
  `proof-carrying-context-optimization-v0.md`, `ROADMAP.md`); and `tools/retrieval-bench/{README.md,
  main.go, main_test.go}`, weaker than every carried row.
- New finding: record data floods the code class. Of the 260 rows the thirteen packets carry, 43
  are `.json` records (benchmark receipts and runs, `tools/cw-trial` task fixtures, conformance
  manifests, `.taskman/tickets/*.json`), against no rc.2 critical file and one replay gold file
  (`docs/specs/INDEX.json`). Their short, field-dense bodies outscore the code that writes or
  reads them under BM25 at `b` 0.3, and since TCP-V0-013 treats every non-documentation hit as
  code, they take head positions and push gold code past the limit.

The documentation residue is the share's gate working as specified (a documentation hit that does
not outscore the lead stays deferred); relaxing it was rejected on 2026-10-06 by the replay and is
not reopened without held-out evidence.

## Variants screened

Each variant was an environment-switched experimental build, measured on the same thirteen cases at
limit 20; record data is `.json .jsonl .ndjson .csv .tsv`. YAML, TOML and XML were deliberately
excluded: they are build, CI and project configuration a change edits.

| variant | rc.2 critical (urfave/cli + Corvint) | replay | record-data rows | per-case loss |
| --- | --- | --- | --- | --- |
| origin/main | 16 + 13 | 18/59 | 43 | — |
| defer all record data after code and documentation | 16 + 14 | 19/59 | 0 | none, but drops record data that leads every code hit |
| gate (chosen): lead or quota of two, rest deferred | 16 + 14 | 18/59 | 23 | none at limit 20 |
| gate, record data also removed from the documentation lead | 16 + 14 | 17/59 | — | pr594 loses `internal/tasks/cli/cli.go` |
| gate, deferred record data before deferred documentation | 16 + 14 | 18/59 | — | none |
| gate, YAML/TOML/XML also record data | 16 + 14 | 18/59 | — | none |

The gate was chosen over the full deferral as the conservative option: it keeps a record that
outscores every code hit (a task about a ticket or a manifest), mirrors the documentation gate
already accepted, and changes no other class. The documentation lead keeps counting record data,
since removing it lost a replay row. Deferred record data follows deferred documentation, since
documentation carries the authority this product ranks first; the order made no measured
difference.

## Rule chosen (TCP-V0-063)

A record-data hit takes no head position and no documentation share position. It joins the merged
order when it outscores every other code hit competing for the fill, or as one of the first two
that do not (`contextRecordDataQuota`); the rest follow every code hit and every deferred
documentation hit. `coverage.uncertainty` gains one line when record-data hits the packet does not
carry outscore a carried code hit, naming the count and the strongest.

Independent review (Codex, read-only) found that with `CORVINT_CONTEXT_RECENCY=on` the TCP-V0-035
reorder sorted every `lexical` row together, so a recent deferred record climbed past the code it
was gated behind (reproduced: `data/f.json` moved to the first position at limit 8). Fixed:
`recencyLexical` reorders record data inside its own positions, and the gate line reads every
record-data hit against the carried rows, as the share line does for documentation; pinned by
`TestTaskContextRecencyKeepsTheRecordDataGate`, which fails against the unfixed reorder. Recency is
off by default, so the measured numbers below are unchanged.

## Measured before and after (development evidence)

Same thirteen cases; main is `3e501e17`, after is this branch's build.

| limit | rc.2 critical, main | rc.2 critical, after | replay, main | replay, after | record-data rows, main / after |
| --- | --- | --- | --- | --- | --- |
| 10 | 7 + 11 | 7 + 11 | 14/59 | 14/59 | 16 / 9 |
| 20 | 16 + 13 | 16 + 14 | 18/59 | 18/59 | 43 / 23 |
| 50 | 19 + 18 | 19 + 18 | 27/59 | 28/59 | 148 / 27 |

- Limit 20: Corvint fc594fa6 gains `internal/dogfoodflow/change.go` (gold) and
  `conformance/cli-parity-v0/runner_test.go` in place of `conformance/perf-v0/seven-command-manifest.json`
  and `internal/betarung/admissions.json`; no case loses a gold row. Treatment-only misses stay 2 + 8.
- Limit 50: pr594 gains two gold code rows (`internal/tasks/store/lease.go`,
  `internal/tasks/authority/fixture_session.go`); pr557 loses the gold record `docs/specs/INDEX.json` (bm25 11.40, position 48 on main),
  deferred behind two task-store records that took the quota. The packet states it in the new
  line. This is the falsifier's shape on development data, recorded in TCP-V0-063.
- No named documentation miss is recovered by any record-data variant; those remain the
  documentation gate's, unchanged.

The gain is small: one critical row at the default limit and a net of one replay row at limit 50,
with a large cut in record-data rows. The thirteen cases are development evidence; they chose the
rule and cannot validate it.

## Not run

- Held-out validation (`benchmarks/untouched-repository-v1/cobra`, frozen 2026-09-27, never
  executed): `NOT_RUN`. The local cobra clone is shallow, and fetching
  `https://github.com/spf13/cobra.git` at `adbc8813901bba65827259daa8e22ff94ec1f30e` needs network
  approval this lane did not have. Its harness also takes a verified release-artifact directory,
  not a source build, and a run spends the set (`benchmarks/README.md`), so the run is an owner
  decision. Disclosure: this lane read the cobra corpus case list, including one case's critical
  files; that knowledge did not change the rule (no cobra case was replayed), but the owner should
  weigh it.
- `tools/retrieval-bench` report (TCP-V0-014's falsifier): `NOT_RUN`; its samples are not local and
  the download was not approved.
- Blind-v3 critical misses and the Atlas goldens: not applicable; they rank another repository's
  index by query, not the subjectless packet's lexical fill.
- The private third-party leg of V1-0859: `NOT_RUN`, as on 2026-10-06.
- `TestAnalyzerSchemaInputs` (`internal/contextindex/analyzer_schema_test.go`): fails, because
  `taskcontext.go` is a pinned input. The change is query-time ranking and alters no pack fact or
  encoding, so by the 2026-10-07 precedent (`8208764d`) the schema stays `corvint-analyzer/112` and
  only the audited input SHA moves, to
  `ebbf9b01c1b0d73be9ef700e4d99ea20e4eae23e1331769411ec72f473104026`. This lane's attempt to repin
  it was refused by the session's permission policy, so the repin is an owner step.

## Rollback

Delete `isRecordDataSuffix`, `contextRecordDataQuota`, the `data` field, the `lexicalCode`,
`lexicalData` and `lexicalRecords` fields, the record-data branch and lead in `lexicalRows`, the
record-data line in `lexicalCoverage`, the record-data class in `recencyLexical` and
`taskcontext_recorddata_test.go`. No state persists; no golden changed.
