# Decision 0248 — gate and release scripts: signal traps exit 128+N instead of resuming

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`script/go-archive-gate`, `script/corvint-companion-release-gate`, `script/gate-receipt` and
`script/check-analyzer-python-offline-build.sh` installed one handler for `EXIT HUP INT TERM`. A
signal handler that returns lets the shell resume at the next command, and a shell defers its trap
until a foreground child exits. So when only the wrapper was signalled and the delegated command
still succeeded, the script removed its private state and carried on. A scratch reproduction (stub
`go` blocking on a late step; `TERM` or `HUP` to the wrapper alone; stub then succeeds) observed
exit 0 from all four scripts on 2026-09-13:

- `go-archive-gate`, signalled during `archive-status`, still ran `test PASS = PASS` and exited 0.
- `gate-receipt record`, signalled during `archive-status`, still wrote the receipt.
- `check-analyzer-python-offline-build.sh`, signalled during `go list`, lost its dependency list to
  cleanup. `grep` then failed on the missing file, which silently skipped the Core-dependency
  refusal, and the script exited 0. An unsignalled run over the same stub exits 1 with the refusal.
- `corvint-companion-release-gate`, signalled during its final `go run`, exited with that command's 0.

The call: each script keeps its cleanup on `EXIT` only, and `HUP`, `INT` and `TERM` get handlers
that exit 129, 130 and 143. That exit runs the `EXIT` cleanup, the same shape `script/gate-affected.sh`
and `script/dogfood-*.sh` already use. A signalled run now exits 128 plus the signal number. It
never reaches a later check and never exits 0. When a terminal interrupt reaches the whole process
group and the child exits on its own first, `set -e` may still report the child's failure status.
That exit is non-zero too.

Amended: `GAG-V0-004`, `OACS-V0-003`, `GOC-V0-010`, and the `PUB-V0-002`/`003` companion-gate prose.
No requirement IDs added. New cases in `script/go-archive-gate_test.sh` (case 6),
`script/gate-receipt_test.sh` (signal case) and `script/check-analyzer-python-offline-build_test.sh`
(case 11) failed on the returning traps with exit 0 and pass with the fix. The companion gate has
no `_test.sh`; its evidence is the scratch reproduction above.

Amendment, 2026-09-13 (test harnesses). The 21 `script/*_test.sh` harnesses that installed
`trap cleanup EXIT HUP INT TERM` had the same defect. A scratch probe on
`script/check-traceability-tests_test.sh` sent `TERM` to the harness alone while its last check's
stubbed `git ls-files` slept and then succeeded; the harness printed its pass line and exited 0.
With the split traps above the same probe exits 143. All 21 now use this decision's four-line
idiom. `dogfood-change_test.sh`, `go-archive-gate-injection_test.sh` and
`go-archive-gate-injection-interrupt_test.sh` already did. `no-python-runtime-dependency_test.sh`
and `dogfood-bind-range_test.sh` trap `EXIT` only, so a signal ends them with the default
disposition, which is non-zero. No requirement IDs added or amended: the harnesses' assertions are
unchanged.

Amendment, 2026-09-13 (test-harness descendants). `script/dogfood-change_test.sh` runs five
independent scenario phases concurrently. Its signal cleanup terminated and waited for each phase
subshell, but a command running below that subshell could survive the harness. A scratch probe
started a phase child with a unique argv marker, sent `TERM` to the harness, and observed the child
still alive after the harness exited 143. The phases now run in distinct process groups and cleanup
signals each whole group before waiting; the same probe exits 143 with the marked child gone. This
changes only harness resource ownership. No requirement IDs or production behavior change.
