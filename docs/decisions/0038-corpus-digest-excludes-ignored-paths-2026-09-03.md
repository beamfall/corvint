# Decision 0038 — the perf corpus digest excludes Git-ignored paths

Date: 2026-09-03. Status: accepted. Authority: repository owner, verbatim instruction "exclude
gitignored paths from the corpus digest" (2026-09-03), given after the packet-5 rerun's first task
came back `insufficient_evidence`.

## What was observed

`harness-session-start` on the `corvint` corpus reported `state=insufficient_evidence` with sound
timings (oracle p95 358.8 ms, candidate p95 95.9 ms) while `harness-stop` and `harness-post-tool`
reported `valid`. The same task was `valid` in seven runs on 2026-08-28 and 2026-08-29.

The cause is not the kernel. `internal/gokernel/harness.go:472@23c153ec` appends one advisory row to
`<root>/.corvint/self-observations.jsonl` for exactly three events — `session-start`, `user-prompt`
and `file-change` — which is precisely the set that failed, and `stop` and `post-tool`, which are
not in that set, passed. `conformance/perf-v0/corpus.go`'s `digestCorpus` hashed the whole worktree
excluding only the Git directory, so one appended row per measured sample changed the digest and
`requireCorpusUnchanged` raised `corpus-changed`. One invalid task sinks the whole run
(`overallOutcome`), so the measurement could not qualify packet-5 however long it ran.

## The call

The digest now excludes the paths `git ls-files --others --ignored --exclude-standard --directory`
reports. GPK-V0-022 requires that "repository status before and after each read-only case MUST be
identical", and an ignored path never appears in `git status`; the digest was therefore stricter
than the requirement it enforced. The ledger write is deliberate, disclosed in top-level help since
decision 0036, fail-open, bounded, secret-screened, and confined to a path the repository ignores.

Three boundaries hold. A tracked file that `.gitignore` also covers stays covered, because
`--others` lists only untracked paths and a tracked file's change does appear in `git status` —
Beamfall has exactly such a path. A sibling of an ignored file in the same directory stays covered:
excluding a file used to return `filepath.SkipDir`, which skipped the rest of its directory, and now
returns `nil`. And the ignored-path count is recorded in the digest as `ignoredPaths` but
deliberately not compared, because the first measured sample creates the ledger and comparing the
count would fail the task for the very write this record permits.

## Preregistration and rollback

This changes a validity predicate in a preregistered protocol, so it is a dated preregistration
change on the same footing as decision 0037 item 1: the next full run is the first under this
predicate, and no earlier result is re-scored. The imported formal `NOT_RUN` result at
`conformance/perf-v0/results/packet-5-current-pin-requalification-2026-08-31-formal/report.json`
stays untouched (decision 0012 R2). The 2026-09-03 run that exposed this was stopped after four
tasks and produced no report. Rollback is reverting `ignoredPaths` and the `digestTree` exclusion
branch; `TestCorpusDigestExcludesIgnoredPathsButNotTheirSiblings` fails when either is reverted.

## What this record does not do

It does not weaken GPK-V0-022, which is unchanged and now enforced as written rather than more
strictly. It does not decide whether a read command should append an observation ledger at all;
product invariant 4 and the disclosure decision 0036 made still govern that, and if the answer
later becomes no, this exclusion is harmless rather than load-bearing.
