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
to TBC-V1-011) were not started when the check was recorded.
