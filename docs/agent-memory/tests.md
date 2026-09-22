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

### 2026-09-18 cmd/corvint: `TestWorkScriptRejectsCallerScratch/ambient-target` saw its fixture change once on CI
On main run 35407266937 (`f870f41`, attempt 1) `work_materialization_test.go:603` reported a caller/common manifest change 80ms into the subtest; the sibling kinds passed. It did not reproduce in 100 Linux repetitions, git 2.55 leaves a fresh fixture's `.git` unchanged, and the script has no write path into the caller in ambient mode. The assertion now prints both manifests. Done means the next failure's diff names the writer and it is fixed, or the test stays green long enough to close this out.
