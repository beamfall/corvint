# V1-1046 waitid state-change reports in two unreaped waits

Decision: `waitProcessExitUnreaped` (internal/dashboard/repository/process_wait_darwin.go) and
`exitedUnreaped` (internal/testsupport/pipedrain_unix.go) now poll past a waitid record whose
si_code is CLD_TRAPPED/STOPPED/CONTINUED (4..6), which Darwin returns despite WEXITED
(golang/go#19314), and return only on a real exit, still unreaped. This mirrors
internal/procgroup (V1-1037) and internal/groupreap. The 3-line predicate is duplicated rather than
shared: the existing helpers are unexported in packages these do not import, and a new shared
package was out of scope.

Evidence: `TestWaitProcessExitUnreapedIgnoresStoppedChild` and `TestExitedUnreapedIgnoresStoppedChild`
SIGSTOP a `sleep`, assert the wait does not return within 500 ms, SIGCONT (still blocked), SIGKILL
(returns), then assert `Wait` still reaps. Both FAIL on the base source ("wait returned <nil> for a
stopped child") and PASS after. `go vet` clean for darwin and GOOS=linux.

Limits: NOT_RUN make gate. Linux waitid already blocks through a stop, so the tests only discriminate
on Darwin.
