# 2026-10-20 V1-1553: stop device processes after an Android or Appium timeout (TRE-V0-087)

Decision: device-bound profiles now declare fixed cleanup steps in their invocation, and the
executor runs them after a timed-out or interrupted TEST phase, each under its own 10 s bound,
and records the outcome (`not-required`, `clean`, `incomplete`) per step. Anything short of
`clean` adds the `device-cleanup` execution problem, so the observation is incomplete. A
cleanup failure is never reported as a clean stop.

- Android (`android-junit`, `espresso`, `uiautomator`): resolve the instrumentation's target
  package on the device from `pm list instrumentation`, `am force-stop` the target and then the
  test package, then verify through `dumpsys activity processes` and `pidof`. The target, not
  the test package, is stopped first because the 2026-10-17 matrix
  (`runs/android-junit/timeout/cleanup-check.txt`) showed only that stop ending the
  instrumentation. Each step must print a fixed last line, because adb builds before 7.0 drop
  the remote exit status.
- Appium: the fixed configuration's `before` hook writes the session id beneath the report
  directory, and cleanup deletes `/session/<id>` on the plan's loopback endpoint. Session
  enumeration was rejected: Appium 3.8.0 refuses `GET /appium/sessions` unless the operator
  enables `session_discovery` (`runs/appium-uiautomator2-wdio/timeout/cleanup-check.txt`), and
  it would also find sessions the run does not own.
- `corvint-test-runner run` refuses a device-bound plan written before cleanup existed and asks
  for a re-plan, rather than running it without cleanup. Other plans keep their bytes.

Rejected: killing the instrumentation by pid (a device pid is not an identity the run owns and
can be reused); stopping only the test package (shown not to work live).

Evidence: focused tests with a recording fake `adb` whose device shell keeps state, covering a
clean stop, a stop that does not take effect, an uninstalled instrumentation, and a transport
that drops the exit status; and an `httptest` WebDriver endpoint covering 200, 404 invalid
session, 404 unknown command, 500 and an unrecorded session. Live device and Appium witnesses
are NOT_RUN, so the ticket's second acceptance criterion remains open.

Independent review (Codex, one round) found four P2 gaps, all fixed: a failing `dumpsys` or a
missing `pidof` verified the stop (probes now fail as `instrumentation-probe-failed` /
`process-probe-failed`); a `browser.reloadSession()` replacement was never recorded (the
`onReload` hook appends it and cleanup deletes every recorded id); a FIFO session record blocked
the open (it is now `Lstat`-checked and opened nonblocking); and any 200 counted as deleted even
when its body stalled (the body must be read whole before the deadline and be `{"value":null}`).
Retained limits: the `dumpsys` header comes from AOSP source, not a capture, and a session
created but killed before its hook ran stays unrecorded.

Rollback: revert the commits. Plans written with `deviceCleanup` then fail closed decoding.
