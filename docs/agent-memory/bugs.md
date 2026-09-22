---
name: bugs
description: Reproducible defects awaiting a fix
updated: 2026-09-22
---

# Bugs

Reproducible defects: something is broken, with a reproduction. Remove the entry once fixed. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-22 cmd/corvint: trace refusal fixture snapshot loses a temporary Git pack index

The V1-0027 frozen `make gate` on `b835a7464836ee8d25ea828de0f2921ad672a254` failed
`TestRepositoryQueryTraceStateFailuresAreTypedAndNonmutating/oversized` at
`cmd/corvint/query_repository_test.go`'s pre-query `repositoryBytesDigest`: `lstat` of
`.git/objects/pack/.tmp-33234-pack-91a588530539e510fb26f337e9f515a61b7e61ba.idx` returned ENOENT;
TempDir cleanup then reported `.git` directory not empty. Cause UNKNOWN; no retry performed.
Retained stdout/stderr and leftover fixture location: `/private/tmp/corvint-v100-provider-kit/checkpoint.json`.
Investigate fixture Git lifecycle and digest traversal before attributing mutation to the query.

### 2026-09-22 behaviorfalsify: receipt output floor can exceed the BBF-V0-010 32 MiB document bound
`internal/behaviorfalsify/execute.go:390-401@2f6621df` returns `max(share, acceptedReceiptBound(control))`, and the floor is `2 * strings * 4096` where `strings` grows with `len(control.UnrelatedCriteria)` and `len(control.RequiredSetup)`. `internal/behaviorfalsify/plan.go:187-214@d5d8b66d` caps those lists only by uniqueness and the ID pattern, so a control with roughly 2000 criteria yields a per-receipt `OutputLimit` above 32 MiB, which BBF-V0-010 names as the input/output bound; `ensureDocumentBound(report)` then fails the whole report with `document exceeds bound` instead of truncating one receipt. Found by the v0.5.0a3 independent review. Done: cap the two ID list lengths in `validateControl` (or `min` the floor against the document bound), state the cap in BBF-V0-010, and add one test for a control at the cap.

### 2026-09-21 doccorpus: a bundle carrying a real reconciliation finding is refused as `--previous`
`internal/doccorpus/behavior_adapter.go:297-311@b43f74a1` makes `invalid("test variation claim")` / `invalid("orphan claim")` fatal for a prior result, while forward emission at `:1160-1166@110e8943` deliberately retains such criteria and reports `undocumented-tested-behavior` / `missing-reverse-link`. Run 1 exits 0 with the diagnostic; run 2 with `--previous run1.json` exits 2, so the DCP-V1-032 delta is unusable in exactly the case DCP-V1-031 exists to report. Owner call recorded in `questions.md`. Done: either spec says dangling criteria disqualify a prior and the emitter prunes them, or the validator tolerates what the emitter emits.
