# V1-4315: restore independent live-content binding

Base: `e2ab343f377cd0f9af9fb2943f8d4005f5d02277`. This is an uncommitted review
repair of TCX-V0-977..979, directed by the owner. The prior lane's AC2 result does
not qualify this corrected implementation.

Both supplied P1 findings were reproduced. A Linux MAP_SHARED update to an aged,
unselected ticket left every stat-cache key unchanged: the original writer
published a receipt, and the original gate pre-read accepted the divergent bytes.
Replacing tickets directory A with B between listing/stat and content reads,
then restoring A, returned B's digest from the original cache. The final
directory-swap and child-inode retention regressions also fail against unchanged
HEAD sources supplied through a Go overlay.

Writer pre-effect binding and gate pre-read binding now call uncached TreeDigest:
each reads and hashes the live bounded tree through confined descriptors.
TreeCapture remains a planning/modeling optimization. Retention compares phase-1
metadata with fstat metadata from the descriptor actually read, retains only
matching keys, and discards retention when phase-1 directory identities changed.
The age check still governs planning reuse; it grants no content authority.
TCX-V0-977..979 explicitly preserve that boundary. Rollback is to disable
WithBatchWrites; never restore cached pre-effect binding.

Verification used Go 1.27.3 and script/heavy.sh locally, with writable temporary
cache/lock paths and HEAVY_CONFIG=/dev/null because the configured host paths are
read-only. No corvint-tasks executable was run.

- Package checks: `go test -count=1 -timeout 30m ./internal/tasks/intent/ ./internal/tasks/store/`
  passed: intent 54.770 s, store 349.605 s.
- Final strengthened regressions and affected writer paths:
  `go test -race -count=1 -timeout 30m -run 'TestTCXV097[4-9]|TestCALV0070|TestCALV0116_WriterRouteParity|TestLGLV0845' ./internal/tasks/intent/ ./internal/tasks/store/`
  passed: intent 3.243 s, store 35.656 s. This includes both mmap binding boundaries,
  directory ABA retention, opened-child identity, stable reuse, layout changes,
  and indexed/decoding plan parity.
- gofmt and git diff --check passed. Spec regeneration/check, requirement-definition
  and traceability checks passed using a private temporary Git index/object store;
  generated spec projections were unchanged. The real Git index was not changed.

Local binding-tree microbenchmark, three samples per mode, 500 ms/sample,
4,000/8,000 synthetic fixture tickets on tmpfs, AMD Ryzen 9 5950X, 32 logical CPUs:

| Tickets | Before: original cached tree, median ms | After: corrected live-byte tree, median ms | Added ms |
| --- | --- | --- | --- |
| 4,000 | 4.069 | 8.921 | 4.852 |
| 8,000 | 8.469 | 18.478 | 10.009 |

Command: `go test -timeout 30m -run '^$' -bench '^BenchmarkTCXV0977_BindingTree$' -benchtime=500ms -count=3 ./internal/tasks/intent/`.
The final run had no other checks from this repair active; quiet-host qualification
was not performed. Final planning-cache medians were 4.190/8.747 ms. The intermediate
after-run overlapped package checks and is diagnostic only. These are tree-capture
costs, not end-to-end batch-close timings. AC2 is NOT_REQUALIFIED: its CLI harness
launches corvint-tasks, which the owner forbids here. Correctness takes priority.

Corvint pre-change context found the owning spec with no critical missing source,
but retained bounded omissions and an unresolved ticket anchor. The affected plan
reported UNKNOWN and self-binary-stale; it is not exhaustive coverage. Dogfood
start refused operation-lock-create-failed. Independent review preflight passed,
but review failed before analysis with read-only app-server initialization;
independent findings are NOT_PRODUCED. Commit/CEM binding/check/seal, publication,
native ticket completion and make gate are NOT_RUN under this repair's scope and
environment constraints. The supplied independent P1 findings remain the review
input, not an independent approval of this fix.
