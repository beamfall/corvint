# 2026-10-17 V1-0594: JVM, Android, shell, SQL and Appium runner qualification

Follow-up to the Apple slice of the same date. The owner approved user-space toolchain installs
under `/private/tmp/cem10-build/toolchains-2026-10-17/` (browsers excluded). Gradle and Maven
resolved dependencies from Maven Central and Google Maven into that directory only.

Result over the 114 applicable cells (19 profiles, cases P, F, S, Z, B and T; retry is not
applicable):

| | A | A-r | Parser-only | None |
| --- | --- | --- | --- | --- |
| Base `8cd20e92b` | 0 | 0 | 71 | 43 |
| After the Apple slice | 18 | 0 | 59 | 37 |
| Now | 111 | 3 | 0 | 0 |

A-r marks the three Android timeout cells; the device cleanup residual is described below. Every
witness went through `corvint-test-runner plan` and `run --experimental --trusted-local`. Each
retains its request, plan, receipt, command and raw native report under
`/private/tmp/cem10-build/platform-2026-10-17/runs/<profile>/<case>/`. `RUNNER-SHAS` maps each
receipt to the runner build that wrote it. The spec section "Platform qualification matrix
2026-10-17 (V1-0594)" holds the tuple, per-profile results and residuals.

Android, UI and device identity. A lane-owned AVD (`cem10-platform-0594`, API 34 `google_apis`
arm64) ran headless. Each request binds the emulator serial, the instrumentation component and
both APK hashes. Each case also records the device fingerprint and the sha256 of the installed
`base.apk`. Those hashes matched the built APKs. The AVD and the Appium server were removed
afterwards. Teardown evidence: `teardown.txt`.

The live runs found three defects, and each one is fixed:

- `TRE-V0-075` (Apple slice): the Swift Testing selector anchor.
- `TRE-V0-076`: a pgTAP timeout retired psql, but its server backend stayed in `pg_sleep` with
  its transaction open. psql now starts with `client_connection_check_interval=1000`, and the
  backend aborts once its client is gone.
- `TRE-V0-077`: a Robolectric class configured for an SDK outside `enabledSdks` was reported by
  JUnit Vintage as an outcome-free `[runner:…]` leaf. The runner then accepted it as one PASSED
  test, which is a false pass. Such a leaf is now `CONTAINER_WITHOUT_TESTS`, and the
  observation is incomplete. junit4 shares the parser.

Each defect has a retained live before/after pair and a focused unit test.

Device cleanup residual, not fixed. After an Android runner timeout, the device still had the
instrumentation and the target app process. Killing the adb client does not end processes on
the device. The runner has no device cleanup phase. Only `am force-stop` of the target package
ended them; the harness did that by hand. Closing this needs a device cleanup contract for the
shared executor. That is outside a minimal shared-executor change, so it remains a blocker for
claiming a clean Android timeout.

Other residuals are listed in the spec section. The main ones:

- Failure kinds stay UNKNOWN for several native formats.
- Kotest is refused for double-reported errors.
- Gradle and Maven write their outputs into the fixture tree.
- AGP resolved to a prerelease, 9.5.0-alpha10.
- The toolchain's first Robolectric SDK jar was the wrong build.

Rollback: revert the `PGOPTIONS` entry and the Vintage runner-leaf refusal with their tests, and
the spec section text. The evidence directory is outside the repository and is unaffected.
