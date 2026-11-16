# Link-coverage check of the path brief on the frozen 20-task set (TBC-V1-013)

Result of the check that TBC-V1-013 requires before the link export and the path brief builder are
written (change `ticket-brief-context`, PROD-INT-032, tickets V1-5299 and V1-5300). When the
check ran the owner had not set the coverage bar; the bar and the comparison are under "Owner's
bar" below. Nothing was built for the check.

## Method

- Task set: `benchmarks/ticket-brief-task-set/task-set.json`, sha256
  `73d79afd8a6b50a038cf94be2c66f30f84aa548bb3bc5005718ba5a664868615`, committed in `52e7a3b45`
  before any run. It holds the 20 most recently completed tickets that have a requirement
  reference, a declared path and identifiable closing commits (36 of 62 candidates had them).
- Per task: a checkout of the base commit; input paths are the non-test paths the closing commits
  changed that exist at the base (313 in total).
- `tests`: one `corvint affected --by-path` call per input path with the installed build 144. A
  unit counts as listed when a path's plan names it with a witness other than `FRONTIER_SITE`.
  Units named only as shared unbounded readers do not count; this is how the rule "a test listed
  only because the plan's scope is `UNKNOWN` does not count" was applied. A changed Go test file is
  listed when its package is listed; another test file when its own unit is listed.
- `corvint test-evidence` returned `UNKNOWN` (`inventory-unavailable`, `no-fresh-record`) for every
  path tried, so retained test evidence added nothing to `tests`.
- `requirements`: the sealed records at the base commit whose own commit is an ancestor of it;
  hunks on the changed paths; their `specification` and `decision` links; requirement IDs from
  definition lines inside each span at its pinned blob.
- Scratch scripts, not committed. No agent ran.

## Result

| Ticket | Input paths | Changed tests listed | Outside input dirs | Cited reqs listed |
| --- | --- | --- | --- | --- |
| V1-2583 | 3 | 1 of 1 | 0 of 0 | 0 of 3 |
| V1-2718 | 1 | 0 of 0 | 0 of 0 | 0 of 3 |
| V1-4355 | 9 | 3 of 3 | 1 of 1 | 0 of 2 |
| V1-4910 | 2 | 3 of 3 | 2 of 2 | 0 of 1 |
| V1-4731 | 7 | 3 of 3 | 0 of 0 | 1 of 1 |
| V1-4729 | 19 | 7 of 7 | 0 of 0 | 1 of 1 |
| V1-0623 | 82 | 47 of 53 | 12 of 13 | 0 of 1 |
| V1-4386 | 3 | 0 of 1 | 0 of 1 | 0 of 5 |
| V1-4358 | 25 | 7 of 8 | 0 of 1 | 0 of 1 |
| V1-4356 | 31 | 15 of 17 | 6 of 7 | 0 of 1 |
| V1-4751 | 4 | 1 of 3 | 0 of 2 | 0 of 1 |
| V1-2662 | 40 | 12 of 12 | 0 of 0 | 1 of 2 |
| V1-4360 | 6 | 1 of 1 | 1 of 1 | 1 of 2 |
| V1-4354 | 3 | 1 of 1 | 1 of 1 | 0 of 2 |
| V1-4368 | 7 | 0 of 3 | 0 of 3 | 1 of 4 |
| V1-4367 | 9 | 0 of 3 | 0 of 3 | 0 of 4 |
| V1-4374 | 4 | 0 of 1 | 0 of 1 | 0 of 4 |
| V1-4357 | 4 | 1 of 1 | 0 of 0 | 0 of 3 |
| V1-4361 | 5 | 3 of 3 | 0 of 0 | 0 of 2 |
| V1-3244 | 49 | 20 of 33 | 1 of 14 | 0 of 1 |

"Outside input dirs" counts only changed test files that share no directory with an input path.
A test in the same directory as an input path is listed because the unit is the whole Go package,
which a directory listing already shows.

Totals over the 20 tasks:

- Changed test files listed: 125 of 157 (80%); 15 tasks have at least one, and one task changed
  no test file.
- Changed test files outside the input directories listed: 24 of 50 (48%); 7 tasks have at least
  one.
- Cited requirements listed: 5 of 44 (11%); 5 tasks have at least one (AZU-V0-002 twice,
  AFP-V0-160, AFP-V0-097, LPCV-V0-950). Eight more cited IDs are only mentioned inside a listed
  span; a spot check showed cross-references and broad spans, not missed definition lines.
- Tasks with a listed changed test or a listed cited requirement: 16 of 20. Counting only tests
  outside the input directories: 11 of 20.

How much is listed to reach that:

- Every input path's plan has scope `UNKNOWN` in all 20 tasks, so no list is complete.
- One path lists a median of 43 Go packages. Per task the lists name 65 to 529 units, which is 8%
  to 64% (median 36%) of the Go packages that have tests. Every task's list is longer than the
  64-item cap of TBC-V1-010.
- The `requirements` lists name 0 to 515 IDs per task; 13 of 20 are longer than the cap.
- 258 of the 404 changed paths that exist at the base have a kept link.

Reading: the links find the tests a change touches about as often as the proposed bar asks, but
they do so by listing a third of the repository's test packages, and half of the tests they find
sit beside the file. The requirement links rarely name the requirement the ticket cites. Whether a
changed test or a cited requirement survives the 64-item cap was not measured, and no agent ran, so
this does not show that the sections help an agent.

## Superseded first run and corrections

The first run is replaced by the figures above for two measurement errors found in its output:

- It made one call with all of a task's paths. In that form build 144 moves a unit that is an
  unbounded reader for any asked path to the shared list and drops it from every path's own list,
  including the path's own package (suspected defect, V1-5302). The first run listed 94 of 166
  changed test files; 21 of the unlisted Go test files sat beside an input path.
- It scored 9 fixture files under `testdata/` directories as tests. The rerun excludes them. This
  definition was changed after the first figures were seen.

The count of tasks with at least one listed changed test is 15 in both runs. The `requirements`
figures are the same in both runs.

## Limits

- Closing commits are approximate: 6 tasks use the whole lane range and one uses a carry commit,
  so their changed paths include other tickets' work. The set is the newest 20 with identifiable
  commits, not a random sample; 26 of 62 candidates were excluded for having none.
- Build 144 ran against older checkouts and reported `self-binary-stale` on every call.
- The requirement-ID pattern is a scratch regular expression, not the `REQUIREMENTS.tsv` generator.
- Completed tickets do not record their commits (V1-5301), which is why the commits had to be
  inferred.

## Owner's bar

On 2026-11-10, after this result was reported, the owner was asked "Which coverage bar do you set
for the path brief?" and answered "Accept proposed bar". The bar is therefore: at least half of
the tasks have a listed test that the closing commits changed or a listed requirement that the
ticket cites. The result is 16 of 20 tasks, so the bar is met; on the stricter count without
same-directory tests it is 11 of 20. The limits above stand: the bar counts lists before the
64-item cap and says nothing about agent benefit.

## Not run

No Go test, no `make gate` and no agent run. The export (TBC-V1-012) and the builder (TBC-V1-009
to TBC-V1-011) were not started when the check was recorded. Both were built later the same day:
`docs/build-log/2026-11-10-path-brief-builder-and-link-export.md`.

## 2026-11-16 addendum: exclusions and section states (V1-5300)

TBC-V1-013. The frozen task-set bytes still hash to
`73d79afd8a6b50a038cf94be2c66f30f84aa548bb3bc5005718ba5a664868615`.
This addendum recomputes ancestry exclusions from each exact base tree; it does not rerun
the historical build-144 coverage measurement or change the owner-bar comparison above.

For each task, enumerate `git ls-tree -r --name-only BASE .corvint/changes`, retaining
only `.corvint/changes/<40-hex-commit>.cem.json`. Compare the filename commit with the
complete `git rev-list BASE` ancestor set. A record not in that set is excluded before
reading its links. The counts include the same record again when it occurs at another
task's base: these are task-record observations, not distinct records.

Section states are explicit: **O** means OBSERVED_PARTIAL, never complete (all historical
path plans were UNKNOWN); **N** means NOT_OBSERVED. The Tests column preserves the
historical path-plan observation above, not a new execution. Retained test evidence is N
for every task: the historical `test-evidence` calls returned inventory-unavailable /
no-fresh-record. For Requirements, this lane inspected ancestor-admitted CEM hunks on
existing changed paths and their specification/decision evidence at pinned blobs; N
means no requirement definition was observed in those spans. A zero *matching ticket
requirement* in the original table does not imply that the whole section was absent.

| Ticket | Exact base | Sealed | Excluded non-ancestors | Admitted | Tests | Requirements | Retained test evidence |
|---|---|---:|---:|---:|---|---|---|
| V1-2583 | `ae6437cb8a97f0e5ccac22ac5d96009265173256` | 218 | 11 | 207 | O | O | N |
| V1-2718 | `3af3a036d8654ffceffd6a22099e46b41947c7bf` | 180 | 11 | 169 | O | N | N |
| V1-4355 | `32e93e84d7b2bfc4deeff935b6ffa9c2c84239fe` | 323 | 11 | 312 | O | O | N |
| V1-4910 | `1a401eb5ee68ff4a0541534b33a5d6538793e9ea` | 428 | 11 | 417 | O | O | N |
| V1-4731 | `9db7a94169821d9b28701eaa88866aee2fec1083` | 374 | 11 | 363 | O | O | N |
| V1-4729 | `9db7a94169821d9b28701eaa88866aee2fec1083` | 374 | 11 | 363 | O | O | N |
| V1-0623 | `6c33df33bc5670131482f5263b410cb980580657` | 349 | 11 | 338 | O | O | N |
| V1-4386 | `bb729132b84e07ad849ed7d36882f3d4bae9ffd1` | 328 | 11 | 317 | O | O | N |
| V1-4358 | `3626583ece7ec02b403537fd0d861109a92e371b` | 351 | 11 | 340 | O | O | N |
| V1-4356 | `6c33df33bc5670131482f5263b410cb980580657` | 349 | 11 | 338 | O | O | N |
| V1-4751 | `8fc0cf37642e7012f4c26671c9ed97a8706d6e09` | 397 | 11 | 386 | O | O | N |
| V1-2662 | `6f12f4704235dc8f9bea19fc12995bfb68c746c3` | 345 | 11 | 334 | O | O | N |
| V1-4360 | `23368e8dda547c6d58b0681aa2dd116eda592da7` | 328 | 11 | 317 | O | O | N |
| V1-4354 | `b82f846566f6521cc0981a09250061815927068d` | 321 | 11 | 310 | O | O | N |
| V1-4368 | `f044de2fe6526158f7dd7044c974214e7e423a2a` | 335 | 11 | 324 | O | O | N |
| V1-4367 | `c82e85099bd0a13b54234eb6fb20e63c234e1a64` | 333 | 11 | 322 | O | O | N |
| V1-4374 | `c82e85099bd0a13b54234eb6fb20e63c234e1a64` | 333 | 11 | 322 | O | O | N |
| V1-4357 | `23368e8dda547c6d58b0681aa2dd116eda592da7` | 328 | 11 | 317 | O | O | N |
| V1-4361 | `ea09e3db5565637336f482a308f18ae2b08f33b2` | 328 | 11 | 317 | O | O | N |
| V1-3244 | `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b` | 24 | 16 | 8 | O | O | N |

Observed totals: 6346 sealed-record observations,
225 excluded non-ancestor observations,
6121 admitted observations. The exclusions name
16 distinct record paths across the 20 bases. Requirements is NOT_OBSERVED for
V1-2718; retained test evidence is NOT_OBSERVED for all 20 tasks.

Raw ancestry lists and the bounded measurement scripts are outside the repository at
`/home/agent/lanes/r1docs-evidence/`. SHA-256 bindings:

- `count-sealed.py`: `38abf7750742d2c1acd49ecc1689fea9a186f65625ca5cb087e0d08d2c883939`.
- `sealed-counts.json`: `bf367e1b712e3f702d65b35ff1ac9e8ccb33f02679ce163af6851a85726842f7`.
- `section-states.py`: `5a020e1249af69e268be4f30e0c0e0fa3082ede9d0e8a2305d113d8a01363265`.
- `section-states.json`: `b9236b5b213cb9d17b43c10348ae6153599a1fb42e52b64ed208f567228a1399`.

Coverage percentages, cap survival and agent benefit were NOT_MEASURED in this lane.
The pre-existing coverage values and owner acceptance remain the dated observations
above. No export or builder work is part of this addendum.
