<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-2390: Linux live keep-reporters and the `qualify-keep-reporters` CLI path

Base: `030c7b57` (`origin/main`). Branch `claude/cloud-keep-reporters`. Governing requirements:
`PWP-V0-010..018` and `PWP-V0-055` in `docs/specs/playwright-external-provider-v0.md`; the
remainder named there and in V1-1002 was "Linux ... and the `qualify-keep-reporters` CLI path are
`NOT_RUN`". No Tasks state was written.

## Host and inputs

| Field | Observed |
| --- | --- |
| Host | Claude Code cloud container, Ubuntu 24.04.5 LTS, glibc 2.39, x86_64 |
| Kernel | `Linux 6.18.44-fc-v230 #1 SMP PREEMPT_DYNAMIC` |
| Go | `go1.27.3` (`GOTOOLCHAIN=local go env GOVERSION`), the go.mod pin, from the Go module proxy toolchain |
| Node | official `node-v24.11.1-linux-x64` from nodejs.org (the Node of the macOS V1-1002 run) |
| Playwright | `@playwright/test@1.64.0` from registry.npmjs.org |
| Browser | Chrome for Testing headless shell `156.0.8078.4` (Playwright revision 1248), linux64 zip from `storage.googleapis.com/chrome-for-testing-public`, because the network policy refuses `cdn.playwright.dev` (403). The executable's SHA-256 is `4fa61f0b…5f9d6`. It is placed at `chromium_headless_shell-1248` under `PLAYWRIGHT_BROWSERS_PATH`, and `chromium.launch()` reports `156.0.8078.4` |
| Binaries | `go build ./cmd/corvint-js-test-provider ./cmd/corvint` from this branch |

The fixture is the one in `TestPWPV0010KeepReportersLive`. It keeps the reporters list, html,
json, junit and a local `shape-reporter.cjs`. It has one passing and one failing test, and neither
test requests a browser fixture. So both tests are recorded as `browser: "none"`, reason
`no-browser-use` (`PWP-V3-014`), and the headless shell is installed but never launched by the
fixture. The browserless tests qualify on the runner/Node tuple alone (`PWP-V3-015`), which has no
OS field. So this run qualifies non-interference on Linux. It does **not** admit a Linux browser
tuple: `qualifiedPlaywrightTuple` still requires `darwin`/`arm64`.

## Finding 1: the live tests did not link the loopback client (fixed)

The first live run of `TestPWPV0010KeepReportersLive` failed before Playwright started. Both
receipts had infrastructure `server-not-ready`, detail `no loopback HTTP client is linked: import
internal/jstestprovider/loopbackhttp`. V1-3589 (`810a772e`) moved the live tests into
`internal/jstestprovider/externallive`. Only the parent package's test binary imported
`loopbackhttp` (`MTV-V0-950`), so every live external run in the new package failed closed. That
includes `TestQualifiedPlaywright*Live*` and `TestPWPV0035SensitiveProfileLive`. Nothing reported
a false pass. The run failed closed as `MTV-V0-950` requires.

Fix: `internal/jstestprovider/externallive/loopback_test.go` imports the client. It adds
`TestExternalLiveLinksLoopbackClient`, which needs no Playwright and runs in the ordinary gate. It
runs an external-server e2e against a live `httptest` server and fails on the unlinked-client
refusal. It passes with the import and fails with the import commented out (both observed).

## Finding 2: an all-browserless suite launched by bare `npx` leaves Node unobserved (not fixed)

With the fix, the bare in-process run reaches Playwright. The kept-reporter subtests pass, but the
qualification is `not-qualified` with reasons `keep-reporters-control-unqualified` and
`keep-reporters-keep-unqualified`, and `nodeVersion: ""`. Cause: `nodeVersionFor` observes Node only
from an absolute node `argv[0]` or `npm_node_execpath`. The fallback `recordedNodeVersion` reads
only `use.corvintBrowser.nodeVersion`, which a `browser: "none"` test does not carry. So the
runtime tuple is `unobserved` and no outcome can pass. This is the correct abstention for missing
evidence, not a Linux defect. It holds on any OS whenever every test is browserless and the
provider is not started under npm. It is also why the macOS V1-1002 run, whose tests carried
browser identity, qualified with this fixture. Closing it means a report-level Node version from
the reporter. That changes the reporter-to-provider report, so it needs its own requirement. It
is left for a follow-up ticket and is not changed here.

## Observed runs

All runs were on the host above with `PLAYWRIGHT_BROWSERS_PATH` set to the installed shell.
`PLAYWRIGHT_JSON_OUTPUT_NAME=reports/run.json` and `PLAYWRIGHT_JUNIT_OUTPUT_NAME=reports/junit.xml`
were declared env keys. A loopback `python3 -m http.server` served the readiness URL.

In-process witness, run as:

```sh
npm_node_execpath=<node-v24.11.1>/bin/node CORVINT_PLAYWRIGHT_MODULES=<node_modules> \
CORVINT_PLAYWRIGHT_NODE=<node-v24.11.1>/bin/node PLAYWRIGHT_BROWSERS_PATH=<browsers> \
GOTOOLCHAIN=local go test -count=1 -timeout 30m -run TestPWPV0010KeepReportersLive -v \
  ./internal/jstestprovider/externallive/
```

| Run | Result |
| --- | --- |
| Without the loopback fix | FAIL: both receipts `server-not-ready` (finding 1) |
| Fix, no `npm_node_execpath` | PWP-V0-011, -055, -010, -012 subtests PASS. PWP-V0-015/016 subtest FAIL: `not-qualified`, Node unobserved (finding 2) |
| Fix, `npm_node_execpath` set | PASS, all five subtests. Record `qualified`, no reasons, `nodeVersion` `v24.11.1`. The carried-record keep run projects `keep passes` passing and `keep fails` not passing |

Linux-observed facts from the passing run:
- `FullConfig.reporter` lists the provider `reporter.cjs` first (`PWP-V0-014`), then list, html,
  json and junit, each carrying the injected `configDir` of the original directory, then the
  shape reporter.
- Playwright's own reporter `configDir` is the `/tmp/corvint-playwright-*` controlled-config
  directory. `/tmp` on this host is not a symlink, so `PWP-V0-055` was checked on a direct path,
  and no provider scratch file was a config input.
- `playwright-report/index.html`, `reports/run.json` and `reports/junit.xml` landed under the
  original config directory (`PWP-V0-010`).

CLI path (`PWP-V0-015..018`). The `corvint-js-test-provider` binary was run with
`--external-server --dir <fixture> --config <fixture>/keep.config.cjs --package-json
<fixture>/package.json --lockfile <fixture>/package-lock.json --runner-version 1.64.0 --app-identity
keep-reporters-v1 --server-ready-url http://127.0.0.1:18731/ --test-file <fixture>/keep.spec.cjs
--test-arg keep.spec.cjs --env-key PLAYWRIGHT_BROWSERS_PATH --env-key PLAYWRIGHT_JSON_OUTPUT_NAME
--env-key PLAYWRIGHT_JUNIT_OUTPUT_NAME --timeout 60s`, with the node-v24.11.1 `bin` first on `PATH`.
"npm launch" means the same command run by `npm exec --offline --prefix <fixture> -- <wrapper>`,
which sets `npm_node_execpath`.

| Step | Launch | Exit | Observed |
| --- | --- | --- | --- |
| A `qualify-keep-reporters` | bare | 1 | `not-qualified`; `keep-reporters-control-unqualified`, `keep-reporters-keep-unqualified`; `nodeVersion` `""` |
| B `qualify-keep-reporters` | npm | 0 | `qualified`, no reasons, `1.64.0` / `v24.11.1`, `corvint-playwright-external/0`, 2 tests, 5 entries (4 builtin, the shape reporter `bound`), 3 config inputs (`keep.config.cjs`, `keep.spec.cjs`, `shape-reporter.cjs`); record SHA-256 `416dd0db…4b6f5` |
| C `e2e --keep-reporters --keep-reporters-qualification B` | npm | 0 | Record carried; `keep passes` execution `PASSED`, `keep fails` `FAILED` |
| D `e2e --keep-reporters`, no record | npm | 0 | Both outcomes `INFRASTRUCTURE`; nothing passing (`PWP-V0-012`) |
| E `e2e --keep-reporters --keep-reporters-qualification B` | bare | 0 | Node unobserved, so the record does not match; both outcomes `INFRASTRUCTURE`; nothing passing |
| `qualify-keep-reporters` with `--keep-reporters`, or without `--external-server` | bare | 1 | Refused before any process: "one-shot, external-server only and selects both reporter modes itself" |
| `qualify-keep-reporters --keep-reporters-qualification x` | bare | 1 | Refused: "takes no --keep-reporters-qualification" |
| `e2e --keep-reporters-qualification B` without `--keep-reporters` | bare | 3 | `refusal: keep-reporters-qualification-requires-keep-reporters` |

The receipts and records were kept only in the session scratchpad. They bind host-specific
absolute paths, so they are not committed. The digests above identify them.

## Disposition per requirement on Linux

| Requirement | Linux result |
| --- | --- |
| PWP-V0-010 configDir and kept outputs | PASS (live) |
| PWP-V0-011 entries bound, provider-first | PASS (live) |
| PWP-V0-012 unqualified keep receipt abstains | PASS (live, in-process and CLI step D) |
| PWP-V0-014 provider-first order | PASS (observed `FullConfig.reporter`) |
| PWP-V0-015 `qualify-keep-reporters` CLI | PASS under an npm launch (step B). The bare launch abstains (finding 2) |
| PWP-V0-016 carried record projects exactly | PASS (step C). Abstains with no record (D) or unobserved Node (E) |
| PWP-V0-017 closed record | Records decoded by `e2e` in steps C and E; no separate decode probe |
| PWP-V0-018 missing evidence never qualifies | PASS: Node unobserved gives `not-qualified`, never `qualified` |
| PWP-V0-055 scratch files not inputs | PASS on a non-symlinked `/tmp` only |

Still `NOT_RUN`: Node v22.23.2, `/1` to `/3` keep runs, a keep suite that launches a browser on
Linux (which could not project passing, since no Linux browser tuple is admitted), `--retain`
digests on the CLI path, a symlinked temporary directory on Linux, and `make gate` (owner
preference: focused checks only). Rollback: delete `loopback_test.go`. The live tests then fail
closed again with `server-not-ready`. No receipt, record or projection format changed.
