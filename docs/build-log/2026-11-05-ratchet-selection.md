# V1-4729: ownership ratchet selection

Date: 2026-11-05
Intent: owner lane charter V1-4729; experimental AZU-V0-9900..9902 refine AZU-V0-002.
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.

## Decision

Tracked path membership is an ownership-audit input even when no content reader names the path.
Observe the base tree, HEAD tree and current index separately, so staged additions, deleted
worktree files and both sides of a rename trigger the maintained gate. Untracked files alone
and ordinary content edits do not. These are per-plan inputs, outside the graph digest/cache.
A gate-only selection names the ratchet and its runnable command; an independently selected
package keeps whole-package coverage and records the additional named obligation.

A newly tracked Go-hosted fixture whose enclosing package is selected does not additionally
force unrelated plugins into participation. Foreign content indexed below the fixture or a
known resolution input keeps the path open. All other participation, reached-unit, narrowed
reader and runtime frontier rules remain. Crucially, the missing declaration stays an
`UNOWNED_DIRTY_PATH` with the three declaration files and their purposes in its fix hint; the
plan stays UNKNOWN. Selecting the gate does not declare ownership or qualify zero unknowns.

The CLI's full inventory is the right AZU-V0-002 ownership result. The maintained test was
already using that classifier, but compared it with a shrinking debt baseline. The production
`AuditOwnership` entry point now builds and classifies both, and the fixture checks their exact
raw-list equality. The base/main snapshot has 12,304 tracked paths and 1,694 unowned paths;
all 1,694 exactly equal the existing baseline (zero additions, zero stale entries). Thus a
passing ratchet means no debt regression, not an empty audit. Neither baseline nor CLI exit
semantics changed, and the original baseline assertions remain intact.

## Historical replay

The input is exact commit `0c8ae2da918307490edc10dcb1b8320005ec39ee`, compared with its
parent `28a3ff86a2c3d17662765da12cc9251c06466f5f`, materialized in an isolated local clone.
The historical binary was built from that exact clean commit; the patched binary uses this
lane's implementation. Both runs use the same replay tree and parent, with no fixture edits.

| Measurement | Historical | Patched |
| --- | ---: | ---: |
| `test --dry-run --summary` execution `units` | 43 | 43 |
| affected selected units | 290 | 290 |
| affected unknown entries | 78 | 4 |
| `LANGUAGE_FRONTIER` | 75 | 1 |
| `UNOWNED_DIRTY_PATH` | 3 | 3 |

Commands (each binary was invoked with `--root` pointing at the replay clone):

```sh
corvint test --base 28a3ff86a2c3d17662765da12cc9251c06466f5f --dry-run --summary
corvint affected --base 28a3ff86a2c3d17662765da12cc9251c06466f5f
```

The summary does not itself print an unknown count; the companion affected receipt supplies
that number. The remaining frontier is `shell:foreign-invocation-unmodelled`, whose independent
bearing is retained. All three historical fixture paths retain their reason and declaration
hint. The old replay already selected the affected package incidentally through
`IMPORT_GRAPH_READER` on `tools/ci-reuse-plan/main.go`; the delivery adds an explicit named
ratchet obligation rather than claiming a new package in this particular replay. A separate
added-file fixture has no incidental reader and proves the new named selection and dry-run argv.

Raw replay artifacts stay outside the repository at
`/home/russell/lanes/ratchetsel-evidence/`. SHA-256 digests:

| Artifact | SHA-256 |
| --- | --- |
| `replay-historical-affected.json` | `eaa487341307e64ac45d6d9179774185d43a0a06769b8be6559b64781b3aa2c4` |
| `replay-after-affected.json` | `2cafb07d84ef22cb069240ebca96906ecb35aac58c03c0b6f5f382302ca67015` |
| `replay-historical-summary.txt` | `7af0ee0eb4e025bcf7b703ff5907c9709d15e56a334d95b3f24212bd57969960` |
| `replay-after-summary.txt` | `c7a5455957b6af1ff84c3d462912c4ab7ad2cd0c8ba402f8bb4f1c39cea912e8` |
| `main-audit.json` | `4e2e53b3897454c3c8c1b433e42f1e733cd149ea91da6e63e9061e9e541a80fe` |

## Content-only compatibility and verification

At the same historical tree, append only a newline to existing
`tools/ci-reuse-plan/main.go`, then run `test --base 0c8ae2da9 --dry-run` with each binary.
Both stdout (11,810 bytes) and stderr (3,832 bytes) compare byte for byte with `cmp`:
stdout SHA-256 `e9481057602f43510baec8f81bacd99eef88596a10652f58d5db435350948900`,
stderr SHA-256 `e9668b91625de6bc1f0e9cdf601b64c6abb67cefc4a59914373536f5f27f6d5f`.
A final-tree binary reproduced both the replay receipts and this content-only comparison byte for byte after the final code edits. No committed golden, JSON shape, field name, error/exit contract or core-freeze output changes.
The two moved documentation citations retain their original content hashes after repinning.

The first complete affected-package run failed only the existing incremental live-budget test:
p95 181.63 ms, calibration p95 32.36 ms, ratio 5.61 above 5.00 under host contention.
The new closure helper unnecessarily built a Go-package directory map on ordinary plans; it
now returns immediately when there is no membership or committed-path input. No performance
assertion or threshold changed. The second whole-package run also exceeded the budget (best ratio 6.22). A focused unchanged-base run passed; a subsequent focused patched run passed at p95 77.83 ms / probe p95 21.46 ms (ratio 3.63). The final complete affected-package run passed (including the maintained ownership ratchet and existing performance test). Both earlier failures remain retained; no performance assertion or threshold was changed.

The whole CLI package also exposed pre-existing sandbox failures. Three representative tests reproduce on the unchanged base: profile paths are discovered inside `/home/russell/lanes`, a nominal non-repository resolves to that parent, and scratch materialization rejects an ancestor owned by another UID (`/home` reports UID 65534). These block full package qualification here; the host verifier must rerun it. Two exact unowned-entry expectations were extended with the authorized new hint, keeping their existing reason, detail, scope, count and selection checks. The focused rerun passes.

Focused regressions cover membership, staged and untracked distinctions, declaration edits,
the three historical fixture paths, named dry-run projection, content-only byte equality,
foreign fixture content, resolution inputs, reached frontiers and another open unowned path.
The lane handback retains the complete touched-package results, command logs and terminal CEM
check. Repository-wide gates, frozen selection-baseline replay and independent review are left
to the orchestrator under the charter; this is not a wider AZU-V0-009 qualification.

## Dogfood and limits

`make dogfood-change BASE=9db7a94169821d9b28701eaa88866aee2fec1083` recorded PRECHANGE.
The requirement-named context, query and impact receipts were retained through
`make dogfood-prechange` before edits. A tree-built binary supplied affected selection before
package tests. The installed binary refused `--ownership-audit`, so no audit claim uses it.
Spec requirements were regenerated from the staged spec; line citations were repinned.
Final binding/checking uses the same base and stops without sealing, publishing or task writes.
The native ticket body is NOT_OBSERVED: both the local intent projection and supplied queue
snapshot predate V1-4729. The owner's complete lane charter supplies this slice's intent.

Rollback removes the scoped selection inputs, named gate projection and shared audit wrapper,
restoring the missed ownership-obligation risk and redundant frontier cascade. It needs no
migration and changes no baseline or declaration grammar. The independent host review and
repository-wide qualification remain NOT_PRODUCED by this lane, as explicitly assigned.
