---
name: fixes
description: Non-defect fixes: tech debt, code quality, misleading text
updated: 2026-09-22
---

# Fixes

Non-defect fixes: tech-debt patches, code-quality adjustments, misleading comments or text. Remove the entry when done. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-22 jstestprovider: unit-path report bound borrows the external provider's 4 MiB constant
`internal/jstestprovider/runner.go:136-138@233969ba` reads the Vitest report through `readBoundedReport` with `externalOutputLimit` (`internal/jstestprovider/external.go:26@6a2a3c6d`, 4 MiB) while the same run's stdout is bounded by `defaultOutputLimit` (`internal/jstestprovider/runner.go:19@581cf4c7`, 16 MiB). A multi-thousand-test suite with stack traces can exceed 4 MiB and now surfaces as `report-not-written`. Done: give the unit path its own named report bound at least equal to the process bound, and cite that value in `docs/specs/js-live-test-provider-v0.md` row `report-not-written`.

### 2026-09-22 specs: two refusal-triggering bounds are unpublished
`externalMaxConfigInputs = 256` (`internal/jstestprovider/external.go:26@6a2a3c6d`) triggers `config-inputs-unobserved` but PWP-V0 names no count; `cleanupReserve` in `internal/behaviorfalsify/execute.go` is 10 s but BBF-V0-010 only says the budget "must also exceed the executor's cleanup reserve". Done: state both numbers next to the vocabulary rows that they trigger.

### 2026-09-22 cmd/corvint: `taskman fixture` output failure is plain text, not an error envelope
`cmd/corvint/taskman_fixture.go` prints `taskman fixture: cannot write fixture output:` on stderr, consistent with its sibling path in the same file but unlike `dogfood_ocm.go` (`emitOCMError`) and `corpus_integration.go` (`emitError`), which `output_write_failure_test.go` pins as envelopes. Exit code is 2 in all three. Done: emit the same envelope shape and add the case to `output_write_failure_test.go`.

### 2026-09-22 cmd/corvint: corpus relay can emit two error envelopes
In `cmd/corvint/corpus_integration.go`, when the wrapped native command already wrote its own error envelope and only the copy to the real stdout fails, the relay appends an `output-failed` envelope, producing two JSON documents on one stream. Done: suppress the second envelope when the native output already carried one, with one test.

### 2026-09-22 contextindex: `citedNumbers` doc comment overstates malformed-range refusal
In `internal/contextindex/authority_trigger.go`, `path:5-0` and `path:9-3` fall through to the single-line-plus-extras branch (0 doubles as the absent sentinel), so only `first` is cited; the doc comment reads as though every malformed range refuses. Done: either refuse inverted or `-0` ranges explicitly or reword the comment, and pin the chosen behavior with one test.

### 2026-09-22 doccorpus: `lost_reverse_links` wire strings embed NUL that `textOK` forbids elsewhere
`internal/doccorpus/encoding.go:54@fcdb12dc` rejects NUL in text fields, yet reverse-link keys are `"variation-test:"+id+"\x00"+testID` and are published verbatim in `lost_reverse_links`; the read side does not validate that field so it round-trips, but the published document contradicts the corpus text rule. Done: use a printable separator on the wire (or encode the pair as two fields) and validate the field on read.

### 2026-09-21 cmd/corvint: `CPUPROFILE` makes two read commands write a file
`cmd/corvint/taskcontext.go:86@aa1f0538` and `main.go:1142` call `os.Create(profilePath)` when `CPUPROFILE` is set, from the documented read-only `context` command and the generic harness-event path. Off by default and operator-pointed, so not an invariant-4 violation, but undocumented. Done: document the env knob beside the invariant, or move profiling behind an explicit flag.
