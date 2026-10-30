# Experimental Appium Android runner

`appium-uiautomator2-wdio` is a fixed WebdriverIO/Mocha client profile for an
explicit operator-owned loopback Appium server and Android emulator. It does not
start a server, create or select a default emulator, install an app, or acquire
device authority. Configuration and test JavaScript remain trusted executable
project code. A loopback endpoint is not a network or device permission grant.

The request requires independently pinned executable, configuration and native
`@wdio/json-reporter` module; one exact pinned `.js`, `.cjs` or `.mjs` selector;
`project` equal to `http://127.0.0.1:PORT/`; and `target` equal to an explicit
`emulator-NNNN` UDID. A generated configuration fixes the local runner, endpoint,
one Mocha worker, one spec and one native report file. It requires exactly one
Android/UiAutomator2 capability with that UDID and explicit app package/activity.
Only typed app identity, four qualified boolean options and two bounded timeout
options are copied from capabilities. Other supplied fields, including nested
capabilities and connection overrides, are discarded. The configuration copies
only the bounded Mocha timeout; hooks, suites, services, session attachment and
retry scheduling are excluded. Native ConfigParser/session-sanitizer regression
checks verify the effective schedule and connection against the installed WDIO.
The independently pinned reporter supplies `appium-wdio.json`; caller-provided
report paths are refused. No arbitrary argv or automatic package install exists.

Parsing reuses the reviewed WebdriverIO native outcome parser and additionally
requires the qualified Android/UiAutomator2 capability tuple, matching native
UDID/deviceUDID matching caller-bound `target`, a native session UUID, and named
case identities from that session. Exactly one canonical native JavaScript file
URI must match caller-bound `sourceRoot` plus the one literal `selectors` entry;
its path is retained in every test row. These caller fields are copied by the
executor and establish comparison metadata, not immutable source authority.
Missing reports/session, empty suites, collection/setup errors, contradictory
native counts, unknown states, overflow, timeout or cancellation cannot establish
a complete passed observation. Retry details remain `NOT_REPORTED`; device
state, server identity, runtime dependency closure and semantic adequacy remain
unproved. Native returned capability fields are observations, not authentication
of the declared server/device.

Actual qualification used WebdriverIO 9.32.0, Appium 3.8.0, UiAutomator2 8.7.0,
Node 22.23.3 and an isolated Android14/API34 AOSP-ATD arm64 emulator from an
already installed image. A task-owned APK exposes a real text view. The native
matrix reads that view for pass and deliberate assertion failure, and separately
runs skipped, empty, setup-error and collection-error fixtures through the common
executor. Raw native reports and hashes are in `testdata`; executable/config/APK
hashes, receipts and cleanup evidence are retained under
`/private/tmp/cem10-build/mobile`. Parser mutation tests do not qualify additional
runtimes. Other Appium drivers, devices, versions, retries and lifecycle variants
remain open qualification obligations.

UiAutomator2 changes Android hidden-API settings during session startup, even
with `skipDeviceInitialization` and `noReset`. Qualification therefore used a
new disposable task-owned emulator, preserving existing devices and their
settings. Ordinary app/server code is outside an OS sandbox. The external
qualification harness owns all server/emulator cleanup; client process cleanup
alone is not evidence that a remote device/session was retired.

`TestLiveAppium` runs only with explicitly supplied `CORVINT_MOBILE_FIXTURE`,
`CORVINT_APPIUM_ENDPOINT`, `CORVINT_APPIUM_DEVICE`, `CORVINT_WDIO_JSON_REPORTER`
and `CORVINT_NODE`. It downloads nothing and starts no server/emulator.
