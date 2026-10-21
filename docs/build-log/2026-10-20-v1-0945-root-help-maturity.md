## 2026-10-20 V1-0945: root help names each verb once with its maturity; CCF-V1-009 accepted

Decision 0548 (V1-0945 row) accepts CCF-V1-009 (4,096-byte bound) and records the ticket's ~1.5 KB
target as a limit, and applies the CCF-V1-008 amendment that decision 0539 item 7 accepted (V1-0994):
the `Commands:` list carries the maturity label, so each verb is named once.

### Change

`Commands:` now has two groups. `  Core (decision 0332; ...)` lists the twelve frozen Core verbs in
`frozenCoreVerbs` order; `  Experimental, no stability promise; owning spec in parentheses:` lists
every other verb with its gloss and owner prefix, for example `    feature  bounded feature context
(GPK-V0)`. The separate `Command maturity:` section (`commandMaturityHelp`) is gone. The `\n  Core (`
and `\n  Experimental,` anchors that `conformance/host-lifecycle-v1` `missingCoreVerbs` parses are
kept. Per-verb help is unchanged.

### Measured bytes

Command: `corvint --help | wc -c` on a worktree build (`go build ./cmd/corvint`), host load average
18-72 (high band; bytes are load-independent).

| Version | Bytes |
| --- | --- |
| before CCF-V1-009 (2026-10-07) | 11,710 |
| CCF-V1-009 delivered (base 2b0cfce94) | 3,967 |
| this change | 3,665 |

Section breakdown of this change: header and usage 144, `Commands:` 2,578 (50 verbs, glosses, two
group headers, 38 owner labels), global options 182, support boundary 761.

Limit: ~1.5 KB is not reached. The ticket review (V1-0994) estimated about 1.8 KB, which assumed
the label fold alone; the glosses CCF-V1-009 requires take most of the 2,578 `Commands:` bytes, and
the support-boundary disclosures (761 bytes) are also required. Reaching ~1.5 KB would need dropping
glosses or disclosures, an owner amendment of CCF-V1-009 that decision 0548 did not take.

### Default limits

Reviewed again against the frozen evaluations recorded in
`2026-10-07-v1-0945-root-help-verb-index.md`: the retrieval-bench context arm loses 0.07 to 0.37
recall between rank 20 and rank 10 on every subset, and `query`'s default 10 is byte-pinned by
`conformance/cli-parity-v0`. No frozen run shows a non-regressing smaller default, so the `context`
default (20) and `query` default (10) are unchanged. A fresh bench run at `--limit 10` stays
NOT_RUN (bench samples and corpus snapshots are not in the worktree).
