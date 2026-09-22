# W12 / `GPK-V0-022` — Beamfall dogfood at one pinned clean revision

Date 2026-08-29. Harness: `script/dogfood-w12.sh`. Candidate built from this branch;
oracle `PYTHONPATH=<repo>/src python3 -m corvint_cli`. **No timing is measured here** —
`GPK-V0-016` timing runs are interleaved and load sensitive, so mixing the two would
invalidate them. The timing matrix lives in the `beamfall-dogfood-2026-08-28*` runs.

## Corpus identity

The clause requires "one exact clean Beamfall revision". The live checkout at
`~/projects/beamfall-workspace/beamfall` is under continuous
concurrent write by an agent fleet and cannot hold the predicate, so the corpus is a
pinned replay: `git archive fa3b1e7fe5bc6c10e4b09b2729f364780f567a48 | tar -x`, then
`git init && git add --all --force && git commit`.

| | |
|---|---|
| pinned source revision | `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48` |
| **tree** | `3a4251e0f0ebe70a489e296b32189634250f69d4` — **equals the manifest pin exactly** |
| replay commit | `b8bd6673153e6b4c55005dab6b2b312f6344b823` (synthetic; content identity is the tree) |
| status before and after the whole run | `e3b0c442...b7852b855` (the empty-status digest) |

`git add --all --force` is required: Beamfall tracks paths its own `.gitignore` covers, and
a plain add silently drops them and rebuilds a tree that cannot equal the pin.

## Case matrix — clean corpus

| case | oracle B | candidate B | byte-equal | rc o/c | status stable |
|---|---:|---:|---|---|---|
| harness-session-start | 867 | 867 | **yes** | 0/0 | yes |
| harness-post-tool | 819 | 819 | **yes** | 0/0 | yes |
| harness-stop | 833 | 833 | **yes** | 0/0 | yes |
| harness-user-prompt | 5527 | 4784 | no | 0/0 | yes |
| query-authority-start | 5134 | 5134 | **yes** | 0/0 | yes |
| impact-go-source (`internal/app/app.go`) | 16744 | 16744 | **yes** | 0/0 | yes |
| impact-refuses-python (`script/bf_tools.py`) | 2221 | 0 | no | 0/2 | yes |
| harness-rejects-unknown-field | 0 | 0 | **yes** | 2/2 | yes |

Six of eight byte-equal. The two that are not:

- **harness-user-prompt** — the known symbol-result divergence, DR-0005 in
  `conformance/divergence-register.md`, adjudicated `spec-gap` and parked. Note what this
  matrix shows about its scope: `query-authority-start` is byte-equal at 5134 B on the same
  corpus, so the `query` **command** agrees exactly. Only the harness **event** diverges. The
  two differ by one argument — `GPK-V0-028` mandates explicit `--limit 1` for the supported
  query slice, where only the top-ranked authority survives, while the harness context block
  runs at `harnessContextLimit = 10` (`cmd/corvint/harness_context.go:53`), which is where the
  symbol results would appear. `GPK-V0-028` also excludes "symbol ranking" and "automatic
  harness injection" from the slice by name, so the clause mandating exact query byte parity
  does not govern the event that diverges. **Not** a defect found here.
- **impact-refuses-python** — not a divergence. The candidate emits a structured refusal,
  `{"code":"unsupported-impact-path","error":"native Go impact currently supports only .go
  paths","ok":false}`, exit 2. `GPK-V0-023` requires a partial slice to stay visibly
  experimental, which is exactly what this is. The supported `.go` surface is byte-equal
  above. Recorded as a declared capability gap, not a parity failure.

`harness-rejects-unknown-field` is a negative case: both runtimes reject
`{"toolName":"Read"}` with byte-identical `invalid-harness-input` on stderr and exit 2.

## Clause verdicts

| `GPK-V0-022` requirement | verdict | evidence |
|---|---|---|
| runs at one exact clean Beamfall revision | **PROVEN** | tree equals the manifest pin byte for byte |
| project-operation authority (`AGENTS.md`, roadmap-only work source, workflow gates) | **PROVEN** | `query-authority-start` byte-equal at 5134 B; both select `AGENTS.md` as the binding authority |
| Go source / query / impact | **PROVEN** | `query-authority-start` and `impact-go-source` both byte-equal |
| mixed-worktree abstention | **PROVEN** | see below |
| repository status identical before and after each read-only case | **PROVEN** | `status_stable=yes` on 8 of 8 cases, both runtimes, in both runs |
| Corvint made no Beamfall change to improve its benchmark | **PROVEN** | Beamfall is read through a `git archive` replay; the live checkout is never written. No commit in this branch touches Beamfall |
| one real roadmap-authorized change **when available** | **NOT_APPLICABLE** | see below |
| same output/performance matrix | **PARTIAL** | output matrix here; timing in `beamfall-dogfood-2026-08-28-optimized/` |

### Mixed-worktree abstention

Same matrix re-run against a copy dirtied by one tracked edit plus one untracked file
(`beamfall-dogfood-w12-2026-08-29-mixed/`). Both runtimes reported identical state
transitions, and the equality pattern was unchanged:

| | clean | mixed |
|---|---|---|
| `repository.worktreeState` | `clean` | `mixed` |
| `repository.dirtyPathCount` | 0 | 2 |
| `context.state` | `READY` | `STALE_INDEX` |
| `context.freshness.state` | `fresh` | `mixed-worktree` |
| `context.learning.local_trace_state` | `absent` | `blocked-mixed-worktree` |

Oracle and candidate agreed on every field, in both conditions. The candidate abstains
rather than serving a stale index, and says so in the receipt.

### "One real roadmap-authorized change when available" — NOT_APPLICABLE

**This is a judgement call, recorded so it is cheap to reverse.** The clause conditions
this on availability. The corpus is a frozen `git archive` replay with no roadmap
authority attached, and the live Beamfall checkout is excluded on purpose — it is under
continuous fleet write, which is the same reason the corpus is pinned at all. There is
therefore no roadmap-authorized change available to this dogfood without violating either
the pinned-revision predicate or the workspace rule against writing to a shared checkout.
Reading it the other way would make `GPK-V0-022` unsatisfiable rather than conditional.

If the intent is that W12 must land one real ticket through Beamfall's roadmap using the
candidate, that is a separate exercise against the live repo under fleet coordination, and
this row should be reopened.
