---
name: tests
description: Test coverage gaps and flaky tests to stabilise
updated: 2026-09-21
---

# Tests

Missing, weak, or flaky tests, named by file and behaviour. Remove the entry when the test exists and passes. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 cmd/corvint: output-write-failure suite omits `dogfood-ocm` and `taskman-fixture`
`cmd/corvint/output_write_failure_test.go:24-70@97c21948` covers affected, context-lookup, task-context, index build/if-stale and docs, but not the `dogfood-ocm` or `taskman-fixture` verbs, which is why the missing `output-failed` envelope in `dogfood_ocm.go` went unnoticed. Done: one case per verb asserting the stderr diagnostic and exit 2 on a failing stdout writer.

### 2026-09-21 behaviorfalsify: vocabulary and wall-clock edges are unasserted
`TestBBFV0003ClosedControlVocabulary` asserts `Counts`/`Fallback` but not `CompleteVocabulary`, so the `Disposition`-blind count in `execute.go:413-419` passes. Every test uses `WallClockSeconds >= 12`, so a plan at or below the 10s `cleanupReserve` (`plan.go:106`) is never exercised. Add one case each.

### 2026-09-21 jstestprovider: no prefix-overlapping sensitive values in redaction tests
`sensitive_input_boundary.go:105-111` has no test with one sensitive value that is a prefix of another, which is the shape that exposes map-order-dependent output. Add a deterministic case with `{"hunter2","hunter2extra"}`.

### 2026-09-21 contextindex: no test for an oversized cited line range
`authority_trigger.go:196-202` has no test with a citation whose end line vastly exceeds the target's length (`file.md:1-2000000000`); add one that asserts a bounded result rather than a panic or allocation.
### 2026-09-18 cmd/corvint: `TestWorkScriptRejectsCallerScratch/ambient-target` saw its fixture change once on CI
On main run 35407266937 (`f870f41`, attempt 1) `work_materialization_test.go:603` reported a caller/common manifest change 80ms into the subtest; the sibling kinds passed. It did not reproduce in 100 Linux repetitions, git 2.55 leaves a fresh fixture's `.git` unchanged, and the script has no write path into the caller in ambient mode. The assertion now prints both manifests. Done means the next failure's diff names the writer and it is fixed, or the test stays green long enough to close this out.
