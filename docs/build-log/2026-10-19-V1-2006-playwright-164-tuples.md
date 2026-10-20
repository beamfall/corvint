# V1-2006: remaining Playwright 1.64.0 tuples and the e2e refusal exit status

Date: 2026-10-19 (runs on 2026-10-20 local). Spec: `docs/specs/playwright-external-provider-v0.md`
`PWP-V0-035..038`. Ticket V1-2006; GitHub #1263 (follow-up), #1275; #1270 is partly addressed.

## Problem

V1-2005 qualified Playwright 1.64.0 only for `/0` and `/3` on Darwin arm64 / Node v22.23.2 with
the bundled headless shell. `/1` and `/2` receipts, Node 24 receipts and system-Chrome receipts on
1.64.0 therefore read `unqualified` without a live observation either way. Separately (#1275), a
typed `e2e` admission refusal exited 1 with an `infrastructure failure:` stderr line, the same
status as a real infrastructure failure.

## Decisions

- Each tuple is admitted only by its own requirement and a passing live run, as `PWP-V0-009` did.
  No semver range: decision 0448 admits Node releases one exact release at a time.
- `PWP-V0-035`: `/1` and `/2` are candidates on 1.64.0 under Node v22.23.2 only. `/2` keeps
  `PWP-V2-006` (never projects passing), so its candidacy changes only discovery and abstention
  reporting. No `/2` live test existed on any version, so this lane adds the env-gated
  `TestPWPV0035SensitiveProfileLive` with a real `fill` step (fixtures
  `testdata/external/sensitive.{config,spec}.cjs`). The typed value is a literal, not a declared env
  value, because declared env values are retained in the receipt.
- `PWP-V0-036`: Node v24.11.1 is a candidate on 1.64.0 for `/0` and `/3` (both ran live), never
  `/1` or `/2` (not run on Node 24) and never the system-browser tuple.
- `PWP-V0-037`: the system-Chrome version is now keyed by runner version
  (`qualifiedSystemBrowsers`, Go and reporter) instead of one constant, so 1.63.0 keeps
  153.0.8010.48 and 1.64.0 admits only the observed 155.0.8059.85. The live `/3` branch now
  expects the reporter-admitted system tuple to stay non-passing (the Go predicate still excludes
  `/3`), rather than requiring an infrastructure record.
- `PWP-V0-038` (#1275): a typed `E2ERefusal` from `e2e` exits 3 with one `refusal: <code>[: detail]`
  stderr line (CR/LF escaped). 2 was not reused because flag parsing already exits 2 for usage
  errors. `qualify-keep-reporters` is unchanged. The exit-status contract is in `--help` and the
  package comment.
- #1270 (one shared tuple set for receipts and freshness, Node major/minor ranges, refusal naming
  the failing tuple field) is not done here: ranges contradict owner decision 0448, and moving
  `PTF-V0` freshness to 1.64.0 or v22.23.2 needs its own live freshness run. Only its "Node 24 on
  1.64.0" item is covered, as the exact v24.11.1 tuple.

## Measurements and evidence (Darwin arm64, Playwright 1.64.0 unless noted; load average 70-197)

- Node v24.11.1 install: official `node-v24.11.1-darwin-arm64.tar.gz`, SHA-256
  `b05aa3a66efe680023f930bd5af3fdbbd542794da5644ca2ad711d68cbd4dc35`, checked OK against the
  release `SHASUMS256.txt`. PGP was `NOT_RUN` (no gpg on host); `.sig` and `.asc` are retained with
  the manifest in `toolchains-2026-10-17/node24/`. The tar listing had no absolute or `..` members.
- Node v22.23.2 (control v22.23.3): the first run failed. `TestQualifiedPlaywrightLive` hit the
  45 s `--timeout` bound at load ~76, which is a retained failed observation. The `/3` system branch
  failed until the expectation above was corrected. The rerun of `TestQualifiedPlaywrightLive` and
  `TestQualifiedPlaywrightAttemptsLive` passed in 180.8 s. System Chrome 155.0.8059.85 projected
  passing on `/0` and stayed non-passing on `/3`, and the control abstained
  `runtime-tuple-unqualified`. Both devices-spread tests passed (1.3 s each).
- Node v24.11.1 (control v22.23.3): all four live tests passed in 74.9 s, and the system browser
  stayed unqualified on Node 24.
- `/1`: `TestApplicationAttestationDockerComposeQualification` with `-corvint-docker-qualification`
  and v22.23.2 first on `PATH` passed in 98.2 s. The attested run projected passing. The
  wrong-revision, wrong-image, test-drift, restarted-server and interrupted-provisioning controls
  all held.
- `/2`: `TestPWPV0035SensitiveProfileLive` passed on Node v22.23.2 in 19.0 s with one redacted
  step, no typed value in the encoded receipt, a candidate tuple, a non-passing projection and
  retained discovery. It also passed on 1.63.0 (1.4 s).
- 1.63.0 regression (`PLAYWRIGHT_BROWSERS_PATH` = the retained 1243 cache, Node v22.23.2, control
  v22.23.3): Live, Attempts and `/2` passed in 96.8 s. Chrome 155 stayed unqualified on 1.63.0.
  An earlier attempt without the browser path failed with a missing executable, which is a
  harness setup error, not a provider failure.
- Unit tests: each new test failed with the production files reverted to base, and the CLI test
  failed with the exit branch disabled (`version refusal exited 1`). See the PR for the commands.

## Identity

Receipts that built before are byte-identical. No receipt struct, encoding, profile or bundled tuple
changed, and the 1.63.0 system tuple value is unchanged. The reporter's bytes changed (two lookup
tables), as did the CLI exit status and stderr line for refusals.

## NOT_RUN

- Linux (colima linux/aarch64). The existing Docker test runs Playwright on the Darwin host, so it
  is not a Linux Playwright observation. A container run would need a linux-arm64 Node, a Playwright
  1.64.0 Linux headless-shell download, and new Linux digest rows. That was not produced within the
  lane budget, so it is recorded as unavailable.
- `/1` and `/2` on Node v24.11.1, bundled headed Chromium, and PGP verification of the Node archive.
- `/1` on 1.64.0 has no explicit Node log line. The Node is implied by the passing projection,
  because only v22.23.2 is admitted for `/1` on 1.64.0.

## Rollback

Revert the commit. The 1.64.0 `/1`, `/2` and v24.11.1 cases leave `ReceiptRuntimeTuple`, the 1.64.0
system-Chrome entry leaves both tables, and the exit-status branch is removed. Receipts already
written on those tuples then read `unqualified` and are neither deleted nor rewritten, and refusals
exit 1 again.
