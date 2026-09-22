# Beamfall dogfood feasibility probe — 2026-08-28

Pre-work for `GPK-V0-022` (W12). Read-only; the Beamfall repository was never written to.

**Pinned revision:** `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48` (beamfall `main`).
Materialized with `git archive` into a private temp directory and rebuilt as a Git repository —
3,218 tracked files. `git archive` reads the object store, so concurrent fleet writes to the
shared worktree cannot affect the corpus. The previously recorded blocker ("the repository is
under continuous concurrent write ... so it cannot hold the same revision") is therefore **wrong**
and has been corrected in the manifest: the real blocker is that no task is bound to this corpus.

## Results — Go candidate vs Python oracle, same materialized corpus

| case | result |
|---|---|
| `harness event --event session-start` | **byte-identical** |
| `query --task "<project-operations>" --limit 1` | **byte-identical** |
| `impact internal/graph/graph.go --limit 10` | **diverges — DR-0004 ordering** |

`GPK-V0-022` requires that repository status before and after each read-only case be identical.
Measured: `git status --porcelain` returned 0 entries before and 0 after all three cases.

## Two findings worth carrying forward

**W9's project-profile adapter is validated against the real repository.** Both runtimes
independently detect `profile = beamfall` on the pinned corpus. The parity corpus only exercises
the profile path through one synthetic fixture, so this is the first evidence from a real project.

**DR-0004 reproduces on production data, at the truncation boundary.** On result index 4
(`internal/graph/graph_consoleconfig_test.go`) the two runtimes emit the same evidence multiset in
a different order, differing at 4 of 10 positions. The array holds exactly **10** entries —
`MAX_EVIDENCE` — so the cap is binding on real Beamfall data. The sets happen to coincide here, so
no evidence is lost on this result; but the truncation is active, which makes the content-divergence
risk DR-0004 describes live rather than theoretical. A result with an eleventh marker would retain
different subsets in the two runtimes.

## Not yet done for W12

`GPK-V0-021` (Corvint self-dogfood, including the now-live obligation that the final committed Go
migration diff be prepared, verified, reported, and obligation-checked by the Go binary) runs first.
`GPK-V0-022` then needs beamfall-bound perf tasks plus coverage of project-operation authority
(`AGENTS.md`, roadmap-only work source, workflow gates, `make orient`, context-packet tooling),
mixed-worktree abstention, and one real roadmap-authorized change. `GPK-V0-024` is the rollback proof.
