# rspt3 existing containment test blocked by process namespace mismatch

V1-5105 adds inactive runner selection interfaces under RSP-V0-001 through
RSP-V0-004 and the compiled-in portion of RSP-V0-025. It does not change the
existing executor, group reaper or their assertions.

The initial whole-package command, with TMPDIR set to the lane scratch directory,
was `script/heavy.sh go test -count=1 -timeout 30m ./internal/testrunner/`.
It exited 1 after 272.981 test seconds. The unchanged
`TestExecuteRetiresEscapedDetachedDescendants` failed in all four cases:
`timeout`, `timeout-graceful`, `interrupt`, and `normal`. Each reported a detached
browser surviving the executor. The log is retained outside the repository at
`/home/agent/lanes/rspt3-evidence/test-testrunner.log`, SHA-256
`352313864ba2cf0db53c040e8febf713c3bfb358191bd0fd8b459616836fae2a`.

A separate one-process probe observed `os.getpid() == 3`, while the first field
of `/proc/self/stat` was `737298`; `/proc/3/stat` did not exist. The probe output
is `/home/agent/lanes/rspt3-evidence/process-namespace.txt`, SHA-256
`7fdea32b2b2e8ea4ed6826b6c768ce1378b6a41ebfaea53ff70f1d7c368b1cd8`.
Reproduce with Python: print `os.getpid()`, the first field read from
`/proc/self/stat`, and whether `/proc/<os.getpid()>/stat` exists.
The Linux group reaper reads that exact per-PID proc path, so this sandbox cannot
provide the consistent process identity its live containment checks require.
The host verifier must rerun the whole package in its native process namespace.

No failing assertion was weakened, no containment implementation was changed,
and no package pass is claimed. Final verification results, including the whole
package rerun on the delivered code, are retained in the lane handback. The local
outcome remains blocked pending host containment verification and independent
review; successful structural evidence binding does not turn this into a pass.
