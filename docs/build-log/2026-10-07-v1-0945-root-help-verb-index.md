## 2026-10-07 V1-0945: root help becomes a verb index; context and query default limits kept

Ticket V1-0945 (token-usage audit). Proposed requirement CCF-V1-009 (not accepted).

### Root help

`corvint --help` drops the 50-line per-verb usage block, the 60-line wrapped command descriptions and
the long `--root` and support-boundary prose. It keeps three generic usage lines, one short line per
`topLevelCommands` verb under `Commands:`, the CCF-V1-008 `Command maturity:` section (preamble
shortened; the `\n  Core (` and `  Experimental, no stability promise;` anchors that
`TestRootHelpLabelsEveryVerbWithMaturityAndOwner` and `conformance/host-lifecycle-v1`'s
`missingCoreVerbs` parse are unchanged), and a condensed support boundary that still names the
read-only verbs, the self-observation ledger appenders and the writers. Per-verb help is unchanged.

Measured with the worktree build, `corvint --help | wc -c`: 11,710 bytes before, 3,967 after (-66%).
Bound: `rootHelpMaxBytes` = 4,096, pinned by `TestRootHelpIsACompactVerbIndex`.

The ticket's target of about 1.5 KB is not reached. Two accepted contracts set the floor: CCF-V1-008
requires both the `Command maturity:` section (936 bytes, every experimental verb with its owner
prefix) and every verb under `Commands:`, and GPK-V0-059 requires root help to list every public
verb. A 1.5 KB root help needs an owner amendment to CCF-V1-008, for example letting the `Commands:`
list carry the maturity label so each verb is named once.

### Default limits (reviewed before any change; kept)

`context` default `--limit` is 20 (TCP-V0, `taskContextDefaultLimit`); `query` default is 10, not 20
as the ticket recorded (`queryHelp`: "default: 10; range: 1-50"). Measured bytes on this worktree
(task "verb-list top-level help and default limits for context and query", `--subject
cmd/corvint/help.go` for context): context 13,210 at 20, 6,967 at 10, 4,224 at 5; query 12,527 at the
default 10 and 11,621 at 5, so query size is not driven by its limit.

Frozen evaluation: the `tools/retrieval-bench` context arm (TCP-V0-050 (a), decisions 0070/0377)
scores recall@5/10/20. Recorded runs, recall@10 versus recall@20 of the context arm:

| Run | code2test | comment2context | edit2ripple | trace2code |
| --- | --- | --- | --- | --- |
| `benchmarks/results/arb-v2-context-baseline-2026-09-05` (full subsets) | 0.3557 / 0.4849 | 0.3438 / 0.4792 | 0.5086 / 0.6078 | 0.5429 / 0.8333 |
| `docs/BUILD-LOG.md` V1-0089 (2026-09-23, 20 samples per subset, off arm) | 0.3583 / 0.4283 | 0.4917 / 0.6333 | 0.4875 / 0.5917 | 0.4500 / 0.8167 |

Every positive subset loses 0.07 to 0.37 recall between rank 20 and rank 10, so a default of 10 cannot
be shown not to regress the frozen evaluation; the default stays 20. A fresh bench run at
`--limit 10` was NOT_RUN in this lane (the bench samples and corpus snapshots are not in the
worktree). `query`'s default is pinned byte-exact by `conformance/cli-parity-v0` (the repository
profile's default case); under CCF-V1-006 changing it is breaking and needs a divergence-register
row and a decision, so it stays 10. Agents that want smaller packets pass `--limit` explicitly.

### Verification

Focused `cmd/corvint` help/root-help tests and `conformance/host-lifecycle-v1`
`TestMissingCoreVerbs` pass; logs under the lane's td directory. Corvint `affected` selected the
whole `cmd/corvint` unit (scope UNKNOWN) and did not select `conformance/host-lifecycle-v1` or
`conformance/release-artifact-v0`, which read root help through a built binary at runtime.
