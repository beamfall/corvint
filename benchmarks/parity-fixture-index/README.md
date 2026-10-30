# Parity fixture index batching (development screen)

The parity runner builds every fixture in a fresh Git repository. Until this slice it wrote each
tracked entry with its own `git hash-object -w --stdin` and `git update-index --cacheinfo`, two
processes per entry. It now stages each blob's exact bytes (a symlink's target) in a private sibling
directory, writes all of them with one `git hash-object -w --no-filters --stdin-paths`, and records
all of them with one `git update-index --add -z --index-info`. Per-tree Git processes fall from
`2N` to two. No fixture, repository, cache, home or temp root is shared between executions, so
decision 0060's isolation is unchanged; no corpus, manifest, worker default or oracle expectation
changes, and the `CORVINT_GOCACHE` opt-in is untouched.

## Method

A scratch, uncommitted harness wrapped `replay` on the committed corpus (133 parity cases, three
refusals, `workers=1`) with one prebuilt candidate binary for all runs. It summed each Git and
candidate child's reaped `rusage` CPU and the runner's own `RUSAGE_SELF` CPU. Runs were ordered
before, after, before, after, after, before on 2026-09-12 on one host with load averages 24-67.
Wall time moved with load, so CPU and CPU shares are the comparison; absolutes are not portable.
The corpus now makes 136 executions and 272 snapshots: one candidate execution per item since
decision 0088 retired the live oracle, not the 402 executions and 804 snapshots once estimated.

## Results

| Run | Replay wall s | Total CPU s | Git processes |
| --- | ---: | ---: | ---: |
| before 1 / 2 / 3 | 183.8 / 221.2 / 170.7 | 92.7 / 101.7 / 91.6 | 3306 |
| after 1 / 2 / 3 | 176.0 / 142.8 / 108.9 | 71.9 / 67.8 / 61.4 | 1842 |

The median total CPU fell from 92.7 to 67.8 seconds (26.9%). Median CPU by phase:

| Phase | Processes before -> after | CPU s before | Share before | CPU s after | Share after |
| --- | ---: | ---: | ---: | ---: | ---: |
| Blob write and index (`hash-object`, `update-index`) | 1820 -> 356 | 31.9 | 34.5% | 7.3 | 10.8% |
| Tree, commit, ref, index reset (`write-tree`, `commit-tree`, `update-ref`, `read-tree`) | 670 | 12.7 | 13.7% | 13.8 | 20.3% |
| `git init` and `git config` | 272 | 6.2 | 6.6% | 6.5 | 9.6% |
| Snapshot Git (`rev-parse`, `status`) | 544 | 9.5 | 10.3% | 10.2 | 15.1% |
| Candidate process | 136 | 20.7 | 22.3% | 21.7 | 32.0% |
| Runner in-process (spawning, tree walk, JSON, SHA-256) | - | 11.9 | 12.5% | 8.2 | 12.1% |

Within the runner's own CPU, the snapshot tree walks took 0.6-0.7 seconds (under 1%), snapshot JSON
and digests under 0.1 seconds, and fixture file writes 0.15 seconds. Snapshots do not re-hash
unchanged trees redundantly enough to matter; their cost is the two Git processes each.

## Equivalence

All six runs emitted byte-identical replay output (every `PASS`, qualification, `UNSUPPORTED`,
`CANDIDATE-IDENTITY` and `SUMMARY` line). A per-execution dump of fixture commit, tree and fixture
digest, plus all four before and after snapshot digests and entry counts, was byte-identical across
all six runs. The frozen manifest's repository digests cover `.git` bytes, including loose objects
and the index, so the canonical replay also checks this. `TestFixtureBatchedIndexMatchesPerEntryObjects`
pins the new invariant: every tree entry's object, mode and path equal an individual unfiltered
write, including a symlink, an executable, an empty file, a space in a path, a parent commit and a
worktree `.gitattributes` that would change the object if filters applied.

## Rollback

Revert the `indexFixtureEntries` change in `conformance/cli-parity-v0/fixture.go`; no stored state
depends on it.

## Initialization and final-index follow-up

Each fresh execution also ran `git config core.filemode true` after `git init`, then a final
`git read-tree HEAD` after the batched index had produced that exact tree. The otherwise-empty
per-execution template now contains the initial `core.repositoryformatversion` and forced
`core.filemode` values; `git init` appends its remaining detected settings in the same order and
with the same bytes. The final `read-tree` is gone. This removes two Git processes from each of 136
fresh executions (272 total, 1,842 -> 1,570) without sharing a fixture or cache.

### Method and results

The same committed 133-case/three-refusal corpus, `workers=1`, fixed prebuilt candidate, cumulative
runner-plus-reaped-child CPU definition, and loaded host were used. Frozen before and after runner
binaries ran in the same before, after, before, after, after, before order. `/usr/bin/time -p`
reported the cumulative CPU; replay output was captured and compared byte-for-byte. This first
screen used a command-line `core.filemode` setting during init:

| Run | Replay wall s | Total CPU s |
| --- | ---: | ---: |
| before 1 / 2 / 3 | 213.27 / 196.28 / 246.06 | 54.88 / 51.02 / 58.07 |
| after 1 / 2 / 3 | 208.46 / 189.74 / 198.25 | 48.89 / 47.46 / 48.15 |

Median total CPU fell from 54.88 to 48.15 seconds (12.3%). Median wall time fell from 213.27 to
198.25 seconds (7.0%). One additional after arm stopped at the known pre-existing `GOC-V0-008`
`cem-verify-policy-limit` mismatch and was excluded.

That source was not retained: command-line config does not force the repository's stored
`core.filemode` when filesystem detection selects another value. After replacing it with the final
template config, a new frozen run completed one before/after pair: 303.54/427.86 wall seconds and
54.31/66.87 CPU seconds. Load rose from roughly 55 to 78 on 12 CPUs between the arms. Three attempts
at the next before arm then stopped at unrelated known pre-existing parity failures:
`harness-user-prompt-non-ascii`, `cem-begin-map`, and `lrf-ocm-python-claim-refusal`. The final-source
CPU result is therefore inconclusive; neither the preliminary 12.3% reduction nor the loaded pair's
increase is a final-source performance claim. The deterministic process reduction remains 272.

### Equivalence

A scratch comparison materialized the old and new paths for ten distinct manifest cases/fixtures.
It recursively compared every `.git` directory entry and every file byte, covering config, index,
HEAD, refs, and the loose-object path/content set; all ten were identical. The scratch source was
deleted after the passing run. `TestFixtureNeedsNoPostInitializationGitRewrite` permanently proves
that applying the removed `git config` and `git read-tree HEAD` operations to a newly materialized
fixture changes neither config nor index bytes. The complete before/after replay transcripts were
also byte-identical in every included run.

Rollback restores the standalone `git config` and final `git read-tree HEAD` calls. No stored state
depends on this follow-up.

## Commit batching and snapshot follow-up

The remaining per-commit `write-tree`, `commit-tree`, `update-ref`, parent indexing and index reset
now become one bounded `git fast-import` stream per fixture. The final fixture index is still built
with the existing unfiltered `hash-object` and `update-index`, then one `write-tree` preserves its
exact cache-tree extension. `fastimport.unpackLimit` and depth zero retain the observed loose-object
layout. The import's final commit is returned by `get-mark`, and its differing index-independent
reflog messages are replaced with the exact deterministic no-message bytes formerly written by
`update-ref`.

For snapshot discovery, an ordinary repository's real `.git` directory is resolved in-process.
Gitfile and linked-worktree layouts keep the existing combined `rev-parse` and bounded legacy
fallback. Resolving the physical directory is necessary on hosts where Git canonicalizes a temp
root such as `/var` to `/private/var`; this preserves the runner's existing path comparison and
therefore its observed snapshot bytes. Each snapshot still runs the same porcelain-v1 status
command.

### Method and deterministic result

An uncommitted test harness executed all 133 manifest cases and three refusals serially with one
fixed prebuilt candidate. For every execution it serialized the fixture commit, tree and fixture
digest plus every entry, mode, content byte, status byte and component digest in both the before and
after snapshots. Thus the comparison covered objects, refs, index, config and reflogs, including
candidate mutations. Only the absolute scratch root was removed before comparison. The complete
baseline and candidate streams had the same SHA-256,
`db2f8b679949241c1cfa50422f7fa235cca5b03ede35a51e8eef13926e88b175`, and all 136 commit/tree/fixture
identity triples matched.

The harness wrapped only Git calls whose sanitized fixture `HOME` ended in `.corvint-git-home`, so
candidate Git calls were excluded consistently. It reproduced the 1,570-call baseline. Of the 136
fixtures, 94 have one commit and 42 have a parent: batching reduces their 1,026 materialization
calls to five per fixture, 680 total. In-process ordinary-repository discovery removes one call from
each of 272 snapshots. The deterministic total is therefore 952, down 618 calls (39.4%). Wall time
was not used on the loaded host.

Every execution still owns a fresh repository, process, cache, home, temp root and workspace under
decision 0060. No fixture, manifest case, expected digest, worker setting, cache, or oracle contract
changed. Rollback restores the plumbing calls and removes the ordinary-directory fast path; no
stored state depends on either optimization.

## In-process final index follow-up

`hash-object`, `update-index`, and `write-tree` are gone from fixture materialization.
`fast-import` already receives every tracked entry inline and, with the unbounded unpack limit,
leaves every blob, tree and commit loose through Git's own `unpack-objects`, so the runner writes no
object bytes and Go's zlib never produces a stored object. The runner then writes the index Git
produced from `update-index --index-info` plus `write-tree`: version 2, zero stat data, the TREE
cache-tree extension with subtrees ordered by name length then bytes, and the SHA-1 trailer. It reads
the imported loose commit and fails materialization if the computed root tree differs.

### Method and deterministic result

The same uncommitted harness shape as the commit-batching follow-up ran all 133 manifest cases and
three refusals serially with one prebuilt candidate. It first timed 136 materializations alone, then
ran every execution and serialized fixture identities, exit status, every before/after snapshot
entry path, type, mode and content digest (scratch root removed), status bytes and component digests.
A `PATH` shim counted Git calls whose sanitized `HOME` ended in `.corvint-git-home`. Frozen before and
after test binaries ran before, after, before, after, before, after on 2026-09-12, with load averages
falling from about 59 to 10 on 12 CPUs.

| Arm | Materialization-only wall s | Harness wall s | Materialization Git calls | Execution-pass Git calls |
| --- | ---: | ---: | ---: | ---: |
| before 1 / 2 / 3 | 66.69 / 39.08 / 41.75 | 166.73 / 107.12 / 112.58 | 680 | 952 |
| after 1 / 2 / 3 | 39.50 / 23.14 / 22.90 | 101.84 / 81.39 / 75.86 | 272 | 544 |

Materialization calls fall from five to two per fixture (`init`, `fast-import`), 408 fewer; the
deterministic corpus total falls from 952 to 544. Median materialization-only wall time fell from
41.75 to 23.14 seconds and median harness wall from 112.58 to 81.39 seconds; load moved during the
runs, so only the call counts are portable. All six evidence streams had SHA-256
`7fc75ba4c0d0dea31a6bc34b81b8948db6c7b104f07c16eb1c89177867b6dcee`.

`TestFixtureInProcessIndexMatchesGitIndex_GPKV0005` keeps the retired Git construction as a test
oracle and compares commit and tree identities plus every snapshot byte (worktree, config, HEAD,
refs, reflogs, loose objects, index) and status for every manifest fixture and for nested,
length-ordered, symlink, executable, empty-directory and untracked-only shapes. Reversing the
cache-tree subtree order made it fail on `git/index`.

Rollback restores `indexFixtureEntries` and the final `write-tree` in
`conformance/cli-parity-v0/fixture.go`; no stored state depends on the change.
