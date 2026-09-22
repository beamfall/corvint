---
name: bugs
description: Reproducible defects awaiting a fix
updated: 2026-09-21
---

# Bugs

Reproducible defects: something is broken, with a reproduction. Remove the entry once fixed. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 cmd/corvint: `dogfood-ocm` reports a stdout write failure without an error envelope
`runDogfoodOCM` returns a bare exit 2 when writing its status to stdout fails (`cmd/corvint/dogfood_ocm.go:51-53@d5f6e5d7`), while the sibling `ocm` verb emits an `output-failed` envelope on stderr first (`cmd/corvint/ocm.go:56-59@553e4c50`). A caller with a closed or full stdout gets no diagnostic. `taskman_fixture.go` has the same bare return (`cmd/corvint/taskman_fixture.go:36-38@8313d8e5`) where its other error paths print to stderr. Done: both write failures emit the `output-failed` diagnostic before exit 2, matching `ocm`. Found by the 2026-09-21 cmd/corvint handler audit.

### 2026-09-21 contextindex: `citedNumbers` expands a cited line range with no upper bound
`internal/contextindex/authority_trigger.go:196-202@b8258e73` allocates `end-first+1` ints and loops `first..end` before the `number > len(lines)` guard at `:236-239@fbc71363`; the regex at `:32@138c86b0` accepts any digit run. One backticked citation such as a document line-range citation ending in 9223372036854775807 anywhere in an indexed document panics (`makeslice: cap out of range`) or allocates gigabytes, and `documentCitations` runs over every document before the requested-path filter (`internal/contextindex/authority_trigger.go:137-142@354f6aa7`), so it breaks every `corvint context lookup` authority call, a read command. Done: clamp the range to the target's line count (or a fixed cap) before allocating, with a test.

### 2026-09-21 jstestprovider: sensitive-value redaction is map-order dependent and leaks prefixed secrets
`internal/jstestprovider/sensitive_input_boundary.go:105,111@ec383243` ranges the sensitive set in Go map order and `strings.ReplaceAll`s each value. With `{"hunter2","hunter2extra"}` and text `hunter2extra`, drawing the shorter value first yields `[REDACTED]extra`, so part of the longer secret survives and the same input redacts differently run to run (also breaking digest determinism of documents carrying scrubbed text). Touches the issue #56 slice released in v0.5.0a3. Done: sort values by descending length (or one `strings.NewReplacer`, which prefers longest match) plus a test with prefix-overlapping values.

### 2026-09-21 doccorpus: a bundle carrying a real reconciliation finding is refused as `--previous`
`internal/doccorpus/behavior_adapter.go:287-307@3b7bdc0a` makes `invalid("test variation claim")` / `invalid("orphan claim")` fatal for a prior result, while forward emission at `:1149-1156@1d5f1d6d` deliberately retains such criteria and reports `undocumented-tested-behavior` / `missing-reverse-link`. Run 1 exits 0 with the diagnostic; run 2 with `--previous run1.json` exits 2, so the DCP-V1-032 delta is unusable in exactly the case DCP-V1-031 exists to report. Owner call recorded in `questions.md`. Done: either spec says dangling criteria disqualify a prior and the emitter prunes them, or the validator tolerates what the emitter emits.

### 2026-09-21 doccorpus: `artifacts()` can emit a bundle its own validator rejects
`internal/doccorpus/behavior_adapter.go:1530-1545@9b71e34e` builds `roles` from observations without requiring the observation's input to be retained, while `validBehaviorAdapterArtifacts` (`:374-379@d21ba804`) requires it; `roles` is also last-write-wins, so one input serving discovery and runtime roles emits only the last. Inferred (reviewer did not prove `build` admits an unretained observation); confirm by finding one `build` path that appends an observation without a retained input. Done: emit exactly what the validator accepts, one test round-tripping a bundle through `--previous`.

### 2026-09-21 doccorpus: reverse-link keys collide, one diagnostic omits its test, `hashValue` hides an encode error
`internal/doccorpus/behavior_adapter.go:1655-1695@1376061f` joins reverse-link keys with a raw `:` (elsewhere the file uses `\x00`), so `a`+`b:c` and `a:b`+`c` share `variation-test:a:b:c` and a lost link can go unreported. `:1296-1302@0698613e` emits `missing-reverse-link` naming only the variation, so two offending tests produce byte-identical diagnostics. `internal/doccorpus/encoding.go:38@ff2041d2` discards `Encode`'s error and hashes nil, so every oversized contract or variation gets the same constant digest instead of a refusal. Done: `\x00` separators, the test ID in Subject/Field, and an error return from `hashValue`.

### 2026-09-21 behaviorfalsify: `completeVocabulary` ignores `Disposition`
`internal/behaviorfalsify/execute.go:414-419@a15255e2` counts declared kinds across all planned controls, so nine `not_supported` controls report `complete_vocabulary: true` with `Supported=0`, against BBF-V0-009. Done: count only executable dispositions; assert `CompleteVocabulary` in `TestBBFV0003ClosedControlVocabulary`.

### 2026-09-21 behaviorfalsify: `workspaceDigest` has no length framing
`internal/behaviorfalsify/plan.go:338@1a60bfbf` writes `<relpath>\x00<mode>\x00` then raw content, so file `a` containing `b\x00-rw-r--r--\x00` hashes like empty files `a` and `b`. This digest is the independent BBF-V0-004 observation; crafted residue can reproduce `WorkspaceSHA256` so `classifyAttempt` (`execute.go:176`) still reaches `killed`. Done: length-prefix content.

### 2026-09-21 behaviorfalsify: per-receipt stdout cap can be smaller than a valid receipt
`internal/behaviorfalsify/execute.go:383-395@133b647e` divides the 32 MiB document budget by `8 × controls × attempts`; a legal 64×16 plan gives 4096 bytes while `validateArtifacts` (`:246,:252`) accepts tens of KiB. An honest hook then trips `processSucceeded` (`:344@74940a3b`) and every control reports `infrastructure_failed`. Done: a fixed per-receipt floor or a budget derived from the validator's own bounds.

### 2026-09-21 behaviorfalsify: plan accepts `WallClockSeconds` that cannot execute anything
`internal/behaviorfalsify/plan.go:106@5baa92c5` checks only the BBF-V0-010 upper limits; `WallClockSeconds: 10` with `cleanupReserve` 10s (`execute.go:22`) passes and every control becomes `not_run` / `workspace-or-wall-clock-boundary-unavailable` with no error. Done: refuse `WallClockSeconds <= cleanupReserve` at plan time, one test below 12s.

### 2026-09-21 jstestprovider: unbounded reads on reporter-controlled inputs
`internal/jstestprovider/external.go:429@c94a66df` opens and digests every `report.ConfigFiles` entry with no entry-count cap (a 4 MiB report can name ~130k paths); `runner.go:136` reads the Vitest JSON report with plain `os.ReadFile` while every other report path uses `readBoundedReport`. Done: cap the map length beside the byte bound; use the bounded reader.

### 2026-09-21 jstestprovider: three low defects in readiness, finding cap, and the reporter
`runner.go:361-381` `waitReady` probes with `client.Get` and no request context (the external profile uses `NewRequestWithContext` at `external.go:386-408`), so a stalled server holds past cancellation. `sensitive_input_boundary.go:351` `boundaryAppendFinding` overwrites slot 63 at the cap, so the reported set is neither the first nor the last 64. `qualified-reporter.cjs:381` `fs.statSync` on every `require.cache` key is unguarded and a moved module aborts `onBegin`. Done: context-bound probe, drop-on-cap, try/catch.

### 2026-09-21 cmd/corvint: corpus value flag swallows a following flag
`cmd/corvint/corpus_integration.go:37-40@03659517` consumes the next token as a native value flag's value with only a length check, unlike `rootPreambleValue`'s `argparseOptionLike` gate. `corvint --root --corpus=REPORT.json docs corpus info --artifact a.json` swallows `--corpus=…` as the root and returns a misleading `--corpus is supported only on native evidence reads` refusal. `:124@dece0b9e` also discards the stdout write error on the non-zero-exit relay path. Done: reuse the option-like gate; check the write.

### 2026-09-21 contextindex: double close and a 32-bit length overflow
`internal/contextindex/blob_shards_open.go:111,118@88308008` closes the file twice (deferred close returns `os.ErrClosed`, discarded). `symbolwindows.go:84` computes `4 * count` from an untrusted `uint32` and overflows `int` on 32-bit builds, so `next` can return a short slice and `raw[4*index:]` indexes out of range; inferred, no 32-bit run. Done: single close; check `count > maxInt/4`.
