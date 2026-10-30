# First terminal run and isolated diagnosis

The first full Go run at `10bb38302fb2312fb3dfc2c45cb0ac9195c3df21` failed; it is not acceptance evidence. Its private retained stdout is `terminal-go-test-stdout.log` in the task checkpoint.

The spec-index test found inconsistent AFU digest keys/status and a stale PWP /3 index entry. Header, digest, README and index metadata were synchronized. No production code changed.

Other failures were deadline, process-observation, cleanup timing and performance-budget failures in cmd/corvint, analyzernativebridge, behaviorfalsify, liveverify/affected, gorunner, session and playwrightminimize. Every failing case passed in isolated serial execution (`failed-cases-serial.log`). This is consistent with package contention; it does not establish correctness under arbitrary host load or erase the first failures.

The repeated terminal run retains the full canonical test set and uses `GOFLAGS=-p=1` to schedule packages serially. No assertions, timeouts, tests or safety checks were weakened. The enrollment, base and requirement scopes are unchanged. The corrected source and CEM are committed before that run; no source/spec changes are permitted during it. The earlier plan for a separate index-only follow-up is superseded because the full run itself exposed the metadata failure.
