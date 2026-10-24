# Native Windows test qualification run

Date: 2026-10-23. Ticket: V1-2441. Base: `origin/main` `0641ad2e09a6da042e144fa5ded4861637e550a9`.

## Run

- Host: the owner's Windows machine (AMD Ryzen 9 5950X, 64 GB RAM), reached over SSH.
- Toolchain: go1.27.2 windows/amd64 (official archive, checksum verified), Git for Windows
  2.55.0.windows.3, `core.autocrlf=false`, so the checkout has LF line endings.
- Settings: `CGO_ENABLED=0 GOTOOLCHAIN=local GOENV=off`. Each package ran on its own with
  `go test -count=1 -timeout 30m -v` in a clone at `C:\cv`.
- `go vet ./...`, at the root and in `interop/cem01-go`, exited 0.
- An earlier run on the same host was discarded. A user Go env file had injected
  `GOFLAGS=-run=^NoSuchTest$`, so most packages ran no tests. That file was written by
  `cmd/corvint-go-test-provider` `TestSessionIgnoresTheUserGoEnvFile`. The test redirected
  `HOME` and `XDG_CONFIG_HOME`, but on Windows `os.UserConfigDir` reads `%AppData%`, so it wrote
  into the real user profile. This change redirects `AppData` and refuses a config directory
  outside the test root. The file left on the host needs the owner to remove it.

## Result

- 362 packages: 189 pass and 173 fail. Subtest results: 15,136 PASS, 6,287 FAIL and 448 SKIP.
  The run took about 7,100 s.
- **Coverage limit:** about 573 test files carry darwin, linux or unix build tags, and 223 more
  carry POSIX file-name suffixes. A passing package therefore exercised only its portable subset.
  This run does not show that those POSIX paths work on Windows.
- **Most failures are deliberate fail-closed refusals:** procgroup, open-no-follow, the CEM
  publish lock, the Tasks safe-open boundary, the work source, watch and file transport. They
  fail rather than skip. `docs/specs/corvint-1.0-product-and-release-v1.md` lists
  `windows/amd64` as UNSUPPORTED. The scope question is filed as V1-2485.

## Triage

| Class | Packages | Ticket |
|---|---:|---|
| Fail-closed platform refusal reported as FAIL | 62 | V1-2486 |
| Real path or logic defect (or still unexplained, triaged there) | 38 | V1-2488 |
| Tasks PathText rejects `C:\` absolute paths | 7 | V1-2488 (with V1-2444) |
| Test helper lacks `.exe` or calls POSIX tools and spies | 33 | V1-2490 |
| Refusal masked as "evaluated revision is not a commit" | 10 | V1-2487 |
| Test assumes POSIX modes, TMPDIR, sticky bit or `/bin/sh` | 8 | V1-2491 |
| Stale-lock takeover: "Access is denied" under contention | 1 | V1-2489 |
| 30-minute hang (jstestprovider; corpus-mcp is counted in the masked row) | 1 | V1-2492 |
| Environment: gopls absent, `-race` needs cgo, TLS, opt-in Mocha/WDIO/cmocka | 8 | none |
| Fixed or skipped by this change (analyzer `.exe` fixes are counted in the helper row) | 5 | V1-2441 |

The change fixes:
- `cemcandidate` volume paths;
- `.exe` suffixes in the analyzer and dogfood-flow test helpers;
- the dashboard clock test's root path;
- the dogfood-workers mode check;
- the companion-release POSIX fake-toolchain skip;
- two appmap nil-map dereferences in failure reports;
- the go-test-provider config-directory isolation.

## Per-package matrix

| Package | Result | Seconds | PASS | FAIL | SKIP | Class |
|---|---|---:|---:|---:|---:|---|
| `benchmarks` | FAIL | 6 | 0 | 4 | 0 | fail-closed refusal (V1-2486) |
| `benchmarks/dogfood-measure` | FAIL | 9 | 0 | 15 | 0 | fail-closed refusal (V1-2486) |
| `benchmarks/dogfood-workers` | FAIL | 1 | 0 | 1 | 0 | POSIX test, fixed in PR |
| `benchmarks/runner` | FAIL | 2 | 0 | 1 | 0 | fail-closed refusal (V1-2486) |
| `benchmarks/selfuse-batch` | FAIL | 1 | 0 | 2 | 0 | fail-closed refusal (V1-2486) |
| `cmd/corvint` | FAIL | 314 | 0 | 1108 | 0 | fail-closed refusal (V1-2486) |
| `cmd/corvint-analyzer-c-jni` | FAIL | 6 | 0 | 1 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-dotnet` | ok | 1 | 0 | 0 | 0 |  |
| `cmd/corvint-analyzer-go` | FAIL | 4 | 0 | 53 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-html-css` | FAIL | 18 | 0 | 12 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-java` | FAIL | 1 | 0 | 1 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-js` | FAIL | 5 | 0 | 5 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-kotlin-android` | ok | 1 | 0 | 0 | 0 |  |
| `cmd/corvint-analyzer-objective-c` | FAIL | 1 | 0 | 1 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-python` | FAIL | 17 | 0 | 21 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-ruby` | ok | 1 | 0 | 0 | 0 |  |
| `cmd/corvint-analyzer-rust` | ok | 2 | 0 | 0 | 0 |  |
| `cmd/corvint-analyzer-shader` | FAIL | 6 | 0 | 5 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-shell` | FAIL | 13 | 0 | 1 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-sql-native` | FAIL | 4 | 0 | 30 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-structured-data` | FAIL | 4 | 0 | 4 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `cmd/corvint-analyzer-swift` | ok | 1 | 0 | 0 | 0 |  |
| `cmd/corvint-behavior-falsify` | ok | 1 | 0 | 0 | 0 |  |
| `cmd/corvint-cem-candidate` | FAIL | 1 | 0 | 1 | 0 | bug, fixed in PR |
| `cmd/corvint-cem-experiments` | ok | 1 | 0 | 0 | 0 |  |
| `cmd/corvint-companion-release` | FAIL | 2 | 0 | 1 | 0 | POSIX test, skipped in PR |
| `cmd/corvint-console` | ok | 3 | 0 | 0 | 0 |  |
| `cmd/corvint-corpus-http` | ok | 7 | 0 | 0 | 0 |  |
| `cmd/corvint-corpus-mcp` | FAIL | 1802 | 0 | 6 | 0 | masked refusal (V1-2487) |
| `cmd/corvint-corpus-parity` | FAIL | 4 | 0 | 2 | 0 | path/logic bug (V1-2488) |
| `cmd/corvint-corpus-republish` | FAIL | 2 | 0 | 2 | 0 | path/logic bug (V1-2488) |
| `cmd/corvint-dashboard-snapshot` | FAIL | 2 | 0 | 3 | 0 | test, fixed in PR |
| `cmd/corvint-docs-mcp` | FAIL | 4 | 0 | 6 | 0 | path/logic bug (V1-2488) |
| `cmd/corvint-go-test-provider` | FAIL | 5 | 0 | 8 | 0 | fail-closed refusal (V1-2486) |
| `cmd/corvint-go-toolchain-receipt` | ok | 1 | 1 | 0 | 0 |  |
| `cmd/corvint-intake` | ok | 1 | 2 | 0 | 0 |  |
| `cmd/corvint-js-test-provider` | FAIL | 4 | 50 | 3 | 0 | fail-closed refusal (V1-2486) |
| `cmd/corvint-lsp` | ok | 2 | 2 | 0 | 1 |  |
| `cmd/corvint-mcp` | ok | 3 | 38 | 0 | 18 |  |
| `cmd/corvint-playwright-minimize` | ok | 2 | 1 | 0 | 0 |  |
| `cmd/corvint-postmerge-connect` | FAIL | 1 | 0 | 6 | 0 | fail-closed refusal (V1-2486) |
| `cmd/corvint-postmerge-host-launcher` | ok | 2 | 3 | 0 | 1 |  |
| `cmd/corvint-postmerge-workflow` | ok | 0 | 0 | 0 | 0 |  |
| `cmd/corvint-public-release-check` | ok | 1 | 2 | 0 | 0 |  |
| `cmd/corvint-pulse` | ok | 1 | 27 | 0 | 0 |  |
| `cmd/corvint-readiness-record` | ok | 1 | 3 | 0 | 0 |  |
| `cmd/corvint-release-candidate` | ok | 0 | 0 | 0 | 0 |  |
| `cmd/corvint-release-gate` | ok | 1 | 2 | 0 | 0 |  |
| `cmd/corvint-release-install` | ok | 0 | 0 | 0 | 0 |  |
| `cmd/corvint-remote-provider` | ok | 1 | 2 | 0 | 0 |  |
| `cmd/corvint-ruby-test-provider` | ok | 1 | 1 | 0 | 1 |  |
| `cmd/corvint-tasks` | ok | 0 | 0 | 0 | 0 |  |
| `cmd/corvint-test` | FAIL | 21 | 4 | 14 | 0 | masked refusal (V1-2487) |
| `cmd/corvint-test-runner` | FAIL | 1 | 16 | 6 | 1 | fail-closed refusal (V1-2486) |
| `cmd/corvint-test-validity-mcp` | ok | 2 | 3 | 0 | 0 |  |
| `cmd/corvint-tests-accept` | ok | 2 | 1 | 0 | 0 |  |
| `cmd/corvint-update` | ok | 0 | 0 | 0 | 0 |  |
| `cmd/corvint-web-flows` | ok | 1 | 4 | 0 | 0 |  |
| `cmd/corvint-work-queue` | FAIL | 1 | 0 | 32 | 0 | fail-closed refusal (V1-2486) |
| `conformance` | ok | 1 | 1 | 0 | 0 |  |
| `conformance/cli-parity-v0` | FAIL | 22 | 83 | 37 | 0 | fail-closed refusal (V1-2486) |
| `conformance/dashboard-snapshot-v0` | FAIL | 6 | 139 | 19 | 0 | path/logic bug (V1-2488) |
| `conformance/frontier-v0` | ok | 1 | 111 | 0 | 5 |  |
| `conformance/go-live-test-v0` | ok | 19 | 92 | 0 | 0 |  |
| `conformance/harness-event-v0` | ok | 1 | 3 | 0 | 0 |  |
| `conformance/host-lifecycle-v1` | FAIL | 1 | 11 | 7 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `conformance/human-documentation-compiler-v0` | FAIL | 2 | 29 | 5 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `conformance/mcp-2026-07-28` | FAIL | 19 | 30 | 30 | 3 | masked refusal (V1-2487) |
| `conformance/mcp-test-validity-v0` | FAIL | 12 | 31 | 15 | 0 | path/logic bug (V1-2488) |
| `conformance/ocm-v0` | FAIL | 6 | 29 | 18 | 0 | fail-closed refusal (V1-2486) |
| `conformance/perf-v0` | ok | 1 | 2 | 0 | 0 |  |
| `conformance/pulse-snapshot-v0` | ok | 4 | 28 | 0 | 5 |  |
| `conformance/release-artifact-v0` | FAIL | 15 | 59 | 21 | 4 | test helper .exe/POSIX tool (V1-2490) |
| `conformance/release-artifact-v0/archive-verifier` | ok | 1 | 9 | 0 | 0 |  |
| `conformance/release-artifact-v0/archivebuild` | ok | 1 | 4 | 0 | 0 |  |
| `conformance/release-artifact-v0/archiveverify` | ok | 1 | 35 | 0 | 0 |  |
| `conformance/release-artifact-v0/archivewire` | ok | 0 | 0 | 0 | 0 |  |
| `conformance/use-cases-v0` | ok | 2 | 33 | 0 | 0 |  |
| `examples/evidence-provider/v0` | ok | 1 | 8 | 0 | 0 |  |
| `examples/evidence-provider/v0/check` | ok | 1 | 1 | 0 | 0 |  |
| `examples/evidence-provider/v0/conformance` | FAIL | 3 | 0 | 3 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `experimental/analyzers/sqlnative` | ok | 2 | 174 | 0 | 2 |  |
| `integrations/testfixture` | ok | 0 | 0 | 0 | 0 |  |
| `internal/analyzercap` | FAIL | 6 | 28 | 86 | 1 | path/logic bug (V1-2488) |
| `internal/analyzerdotnet` | ok | 3 | 183 | 0 | 0 |  |
| `internal/analyzerexec` | ok | 1 | 1 | 0 | 0 |  |
| `internal/analyzergo` | FAIL | 1 | 101 | 1 | 0 | path/logic bug (V1-2488) |
| `internal/analyzerhtmlcss` | ok | 3 | 66 | 0 | 1 |  |
| `internal/analyzerjs` | ok | 1 | 212 | 0 | 0 |  |
| `internal/analyzerkotlinandroid` | FAIL | 13 | 63 | 2 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/analyzernativebridge` | FAIL | 8 | 261 | 14 | 0 | path/logic bug (V1-2488) |
| `internal/analyzerpython` | ok | 2 | 171 | 0 | 2 |  |
| `internal/analyzerruby` | FAIL | 3 | 148 | 2 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/analyzerrust` | ok | 1 | 91 | 0 | 0 |  |
| `internal/analyzershader` | ok | 28 | 92 | 0 | 0 |  |
| `internal/analyzershell` | ok | 5 | 176 | 0 | 0 |  |
| `internal/analyzerstructured` | ok | 2 | 121 | 0 | 0 |  |
| `internal/analyzerswift` | ok | 1 | 20 | 0 | 1 |  |
| `internal/appflows` | FAIL | 38 | 89 | 109 | 0 | masked refusal (V1-2487) |
| `internal/appmap` | FAIL | 99 | 89 | 248 | 0 | masked refusal (V1-2487) |
| `internal/attest` | ok | 1 | 44 | 0 | 0 |  |
| `internal/authorityevent` | ok | 1 | 9 | 0 | 0 |  |
| `internal/authoritystore` | FAIL | 4 | 47 | 38 | 0 | path/logic bug (V1-2488) |
| `internal/behaviorfalsify` | FAIL | 10 | 93 | 66 | 4 | path/logic bug (V1-2488) |
| `internal/behaviorfalsify/cli` | ok | 0 | 0 | 0 | 0 |  |
| `internal/betarung` | ok | 1 | 23 | 0 | 0 |  |
| `internal/breakagemap` | FAIL | 31 | 0 | 0 | 0 | path/logic bug (V1-2488) |
| `internal/capcatalog` | ok | 1 | 16 | 0 | 0 |  |
| `internal/cem/cemcode` | ok | 0 | 0 | 0 | 0 |  |
| `internal/cem/cli` | FAIL | 1 | 13 | 14 | 16 | fail-closed refusal (V1-2486) |
| `internal/cem/corpusbinding` | ok | 84 | 1 | 0 | 0 |  |
| `internal/cem/coverprofile` | ok | 0 | 0 | 0 | 0 |  |
| `internal/cem/gitauth` | FAIL | 74 | 137 | 27 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/cem/gitrun` | ok | 2 | 15 | 0 | 0 |  |
| `internal/cem/mdreport` | ok | 0 | 0 | 0 | 0 |  |
| `internal/cem/patch` | ok | 1 | 44 | 0 | 0 |  |
| `internal/cem/publish` | FAIL | 1 | 23 | 1 | 1 | POSIX test (V1-2491) |
| `internal/cem/sim` | ok | 1 | 7 | 0 | 0 |  |
| `internal/cem/stablefixture` | ok | 0 | 0 | 0 | 0 |  |
| `internal/cem/verify` | FAIL | 4 | 121 | 4 | 13 | POSIX test (V1-2491) |
| `internal/cem/wire` | ok | 2 | 174 | 0 | 5 |  |
| `internal/cem/workflow` | FAIL | 8 | 23 | 95 | 2 | fail-closed refusal (V1-2486) |
| `internal/cemcandidate` | FAIL | 2 | 3 | 1 | 8 | bug, fixed in PR |
| `internal/cemchain` | ok | 1 | 0 | 0 | 2 |  |
| `internal/cemdiscriminate` | ok | 0 | 0 | 0 | 0 |  |
| `internal/changee2e` | FAIL | 5 | 3 | 6 | 0 | masked refusal (V1-2487) |
| `internal/changewitness` | ok | 1 | 30 | 0 | 0 |  |
| `internal/compactionkernel` | ok | 1 | 10 | 0 | 0 |  |
| `internal/companionrelease` | FAIL | 3 | 85 | 65 | 4 | fail-closed refusal (V1-2486) |
| `internal/console` | FAIL | 5 | 0 | 9 | 4 | fail-closed refusal (V1-2486) |
| `internal/contextindex` | FAIL | 164 | 680 | 702 | 34 | fail-closed refusal (V1-2486) |
| `internal/corpusindex` | ok | 39 | 41 | 0 | 0 |  |
| `internal/corpusrepublish` | ok | 24 | 27 | 0 | 0 |  |
| `internal/corpusserve` | ok | 10 | 8 | 0 | 0 |  |
| `internal/criterionexperiment` | FAIL | 6 | 38 | 1 | 0 | path/logic bug (V1-2488) |
| `internal/dashboard/adapters` | FAIL | 4 | 156 | 4 | 0 | path/logic bug (V1-2488) |
| `internal/dashboard/authority` | ok | 1 | 0 | 0 | 0 |  |
| `internal/dashboard/model` | ok | 2 | 41 | 0 | 0 |  |
| `internal/dashboard/repository` | FAIL | 2 | 16 | 8 | 1 | path/logic bug (V1-2488) |
| `internal/dashboard/roadmap` | ok | 1 | 17 | 0 | 12 |  |
| `internal/dashboard/source` | ok | 3 | 58 | 0 | 1 |  |
| `internal/delta` | FAIL | 48 | 58 | 11 | 1 | path/logic bug (V1-2488) |
| `internal/depsource` | FAIL | 7 | 46 | 1 | 0 | path/logic bug (V1-2488) |
| `internal/diagnostic` | ok | 3 | 6 | 0 | 0 |  |
| `internal/disagree` | ok | 1 | 26 | 0 | 0 |  |
| `internal/doccompiler` | FAIL | 2 | 80 | 4 | 9 | fail-closed refusal (V1-2486) |
| `internal/doccorpus` | FAIL | 530 | 732 | 189 | 3 | fail-closed refusal (V1-2486) |
| `internal/doccorpus/selfcorpus` | FAIL | 20 | 0 | 2 | 0 | fail-closed refusal (V1-2486) |
| `internal/docmaintain` | FAIL | 10 | 8 | 42 | 0 | fail-closed refusal (V1-2486) |
| `internal/docviews` | ok | 11 | 47 | 0 | 0 |  |
| `internal/dogfoodflow` | FAIL | 65 | 282 | 40 | 5 | fail-closed refusal (V1-2486) |
| `internal/dogfoodocm` | ok | 1 | 16 | 0 | 0 |  |
| `internal/dogfoodoperation` | ok | 1 | 6 | 0 | 0 |  |
| `internal/evalrepo` | ok | 1 | 26 | 0 | 0 |  |
| `internal/evidencereach` | ok | 2 | 14 | 0 | 0 |  |
| `internal/extevidence` | FAIL | 62 | 119 | 15 | 5 | fail-closed refusal (V1-2486) |
| `internal/flowcoverage` | FAIL | 35 | 9 | 20 | 0 | masked refusal (V1-2487) |
| `internal/flowcoverage/testfixture` | ok | 0 | 0 | 0 | 0 |  |
| `internal/flowdocs` | FAIL | 30 | 30 | 3 | 0 | path/logic bug (V1-2488) |
| `internal/frontier` | ok | 1 | 11 | 0 | 0 |  |
| `internal/frontiernext` | ok | 0 | 0 | 0 | 0 |  |
| `internal/frontiernextrepo` | FAIL | 2 | 0 | 1 | 0 | fail-closed refusal (V1-2486) |
| `internal/frontierrepo` | ok | 1 | 0 | 0 | 7 |  |
| `internal/gateselect` | ok | 16 | 88 | 0 | 0 |  |
| `internal/genesis` | FAIL | 6 | 33 | 16 | 0 | fail-closed refusal (V1-2486) |
| `internal/gitancestry` | ok | 1 | 2 | 0 | 0 |  |
| `internal/gitnotes` | FAIL | 26 | 18 | 21 | 1 | path/logic bug (V1-2488) |
| `internal/gitstatus` | FAIL | 1 | 5 | 2 | 1 | path/logic bug (V1-2488) |
| `internal/gokernel` | FAIL | 6 | 49 | 30 | 0 | fail-closed refusal (V1-2486) |
| `internal/goplsclient` | FAIL | 1 | 3 | 8 | 1 | environment (gopls absent) |
| `internal/groupreap` | ok | 7 | 41 | 0 | 0 |  |
| `internal/intake` | FAIL | 6 | 21 | 2 | 0 | path/logic bug (V1-2488) |
| `internal/jstestprovider` | FAIL | 1802 | 125 | 18 | 4 | hang (V1-2492) |
| `internal/liveverify/affected` | FAIL | 27 | 186 | 4 | 2 | fail-closed refusal (V1-2486) |
| `internal/liveverify/affected/cfamily` | ok | 1 | 17 | 0 | 0 |  |
| `internal/liveverify/affected/dotnet` | ok | 1 | 9 | 0 | 1 |  |
| `internal/liveverify/affected/golang` | FAIL | 16 | 122 | 24 | 1 | fail-closed refusal (V1-2486) |
| `internal/liveverify/affected/java` | ok | 2 | 35 | 0 | 0 |  |
| `internal/liveverify/affected/java/javalex` | ok | 0 | 0 | 0 | 0 |  |
| `internal/liveverify/affected/kotlin` | ok | 1 | 54 | 0 | 1 |  |
| `internal/liveverify/affected/languages` | ok | 1 | 1 | 0 | 0 |  |
| `internal/liveverify/affected/python` | ok | 1 | 42 | 0 | 0 |  |
| `internal/liveverify/affected/ruby` | ok | 3 | 37 | 0 | 1 |  |
| `internal/liveverify/affected/rust` | ok | 2 | 23 | 0 | 1 |  |
| `internal/liveverify/affected/shell` | ok | 1 | 18 | 0 | 0 |  |
| `internal/liveverify/affected/swift` | ok | 1 | 25 | 0 | 1 |  |
| `internal/liveverify/affected/typescript` | FAIL | 13 | 381 | 15 | 1 | environment (Mocha opt-in) |
| `internal/liveverify/godiscovery` | ok | 1 | 85 | 0 | 0 |  |
| `internal/liveverify/gorunner` | ok | 1 | 7 | 0 | 0 |  |
| `internal/liveverify/gotest` | FAIL | 2 | 97 | 2 | 0 | path/logic bug (V1-2488) |
| `internal/liveverify/jsresolve` | ok | 1 | 13 | 0 | 0 |  |
| `internal/liveverify/mutate` | FAIL | 1 | 37 | 10 | 23 | fail-closed refusal (V1-2486) |
| `internal/liveverify/parentverify` | FAIL | 3 | 16 | 6 | 0 | fail-closed refusal (V1-2486) |
| `internal/liveverify/processidentity` | ok | 0 | 0 | 0 | 0 |  |
| `internal/liveverify/provider` | FAIL | 1 | 43 | 37 | 1 | fail-closed refusal (V1-2486) |
| `internal/liveverify/pymutate` | FAIL | 1 | 11 | 1 | 6 | test helper .exe/POSIX tool (V1-2490) |
| `internal/liveverify/pyresolve` | ok | 1 | 32 | 0 | 0 |  |
| `internal/liveverify/rbmutate` | FAIL | 1 | 8 | 3 | 15 | test helper .exe/POSIX tool (V1-2490) |
| `internal/liveverify/rbmutate/minirspec` | ok | 0 | 0 | 0 | 0 |  |
| `internal/liveverify/session` | ok | 0 | 0 | 0 | 0 |  |
| `internal/localauthority` | ok | 1 | 29 | 0 | 0 |  |
| `internal/localcompletion` | FAIL | 91 | 71 | 141 | 11 | path/logic bug (V1-2488) |
| `internal/lrf` | ok | 1 | 6 | 0 | 0 |  |
| `internal/lrfrepo` | FAIL | 24 | 145 | 30 | 10 | fail-closed refusal (V1-2486) |
| `internal/lspevidence` | FAIL | 1 | 0 | 1 | 0 | environment (gopls absent) |
| `internal/lspprovider` | FAIL | 2 | 4 | 2 | 1 | environment (gopls absent) |
| `internal/lspsnapshot` | ok | 1 | 8 | 0 | 0 |  |
| `internal/lspstdio` | FAIL | 4 | 21 | 21 | 2 | path/logic bug (V1-2488) |
| `internal/mcp/bridge` | FAIL | 7 | 63 | 13 | 5 | masked refusal (V1-2487) |
| `internal/mcp/corpusbridge` | FAIL | 182 | 9 | 23 | 7 | fail-closed refusal (V1-2486) |
| `internal/mcp/docsbridge` | FAIL | 4 | 16 | 4 | 1 | path/logic bug (V1-2488) |
| `internal/mcp/protocol` | ok | 1 | 23 | 0 | 0 |  |
| `internal/mcp/server` | ok | 1 | 32 | 0 | 1 |  |
| `internal/mcp/testvaliditybridge` | FAIL | 2 | 7 | 3 | 0 | path/logic bug (V1-2488) |
| `internal/migrationratchet` | ok | 1 | 28 | 0 | 0 |  |
| `internal/necessity` | FAIL | 3 | 1 | 7 | 0 | fail-closed refusal (V1-2486) |
| `internal/obscorpus` | ok | 1 | 11 | 0 | 0 |  |
| `internal/observations` | FAIL | 1 | 37 | 39 | 0 | fail-closed refusal (V1-2486) |
| `internal/opencodequalification` | FAIL | 4 | 62 | 11 | 1 | fail-closed refusal (V1-2486) |
| `internal/outcomecal` | ok | 1 | 6 | 0 | 0 |  |
| `internal/perfbudget` | FAIL | 21 | 10 | 1 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/perfbudget/cmd/perf-budget` | ok | 2 | 1 | 0 | 0 |  |
| `internal/plansnapshot` | ok | 4 | 15 | 0 | 0 |  |
| `internal/playwrightminimize` | ok | 1 | 28 | 0 | 0 |  |
| `internal/playwrightminimize/cli` | ok | 0 | 0 | 0 | 0 |  |
| `internal/playwrightruns` | FAIL | 1 | 66 | 1 | 0 | path/logic bug (V1-2488) |
| `internal/postmergeconnector` | FAIL | 2 | 4 | 15 | 0 | path/logic bug (V1-2488) |
| `internal/postmergehost` | FAIL | 5 | 173 | 11 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/postmergemetrics` | FAIL | 1 | 13 | 36 | 0 | path/logic bug (V1-2488) |
| `internal/postmergeproof` | ok | 2 | 135 | 0 | 0 |  |
| `internal/postmergeproof/procfs` | ok | 1 | 1 | 0 | 2 |  |
| `internal/postmergeworkflow` | FAIL | 47 | 79 | 38 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/procgroup` | FAIL | 91 | 33 | 28 | 0 | fail-closed refusal (V1-2486) |
| `internal/procprofile` | ok | 1 | 10 | 0 | 0 |  |
| `internal/projectpath` | FAIL | 1 | 21 | 3 | 0 | path/logic bug (V1-2488) |
| `internal/projectprofile` | ok | 1 | 2 | 0 | 0 |  |
| `internal/proofabstraction` | ok | 1 | 32 | 0 | 0 |  |
| `internal/pulse` | ok | 1 | 14 | 0 | 0 |  |
| `internal/pythongrammar` | ok | 1 | 14 | 0 | 0 |  |
| `internal/pythonsyntax` | ok | 1 | 175 | 0 | 0 |  |
| `internal/receiptbundle` | FAIL | 3 | 3 | 15 | 3 | path/logic bug (V1-2488) |
| `internal/releasecandidate` | FAIL | 2 | 42 | 5 | 21 | test helper .exe/POSIX tool (V1-2490) |
| `internal/releasegate` | FAIL | 9 | 17 | 27 | 3 | bug/refusal (V1-2488) |
| `internal/remoteprovider` | FAIL | 2 | 18 | 4 | 0 | environment? (TLS) |
| `internal/repoenvelope` | ok | 1 | 4 | 0 | 0 |  |
| `internal/requestpath` | ok | 2 | 21 | 0 | 0 |  |
| `internal/revieworder` | ok | 2 | 63 | 0 | 0 |  |
| `internal/rootalias` | ok | 0 | 0 | 0 | 0 |  |
| `internal/rubyautoload` | ok | 1 | 3 | 0 | 0 |  |
| `internal/rubyprofile` | FAIL | 7 | 12 | 12 | 1 | fail-closed refusal (V1-2486) |
| `internal/rubytestprovider` | FAIL | 1 | 1 | 1 | 20 | POSIX test (V1-2491) |
| `internal/runhygiene` | ok | 0 | 0 | 0 | 0 |  |
| `internal/runtimeenv` | ok | 1 | 4 | 0 | 0 |  |
| `internal/scopelease` | FAIL | 8 | 32 | 1 | 0 | bug (V1-2489) |
| `internal/secretscreen` | ok | 6 | 757 | 0 | 0 |  |
| `internal/semescalate` | ok | 1 | 71 | 0 | 0 |  |
| `internal/sessiontest` | ok | 5 | 70 | 0 | 0 |  |
| `internal/sharedpath` | FAIL | 77 | 13 | 20 | 0 | path/logic bug (V1-2488) |
| `internal/skillexport` | ok | 1 | 4 | 0 | 0 |  |
| `internal/slotlearn` | ok | 1 | 10 | 0 | 0 |  |
| `internal/specindex` | FAIL | 4 | 8 | 1 | 0 | path/logic bug (V1-2488) |
| `internal/stepnegation` | FAIL | 1 | 52 | 1 | 0 | POSIX test (V1-2491) |
| `internal/stepverify` | ok | 1 | 4 | 0 | 30 |  |
| `internal/stepverify/safeopen` | ok | 0 | 0 | 0 | 0 |  |
| `internal/taskman` | ok | 2 | 217 | 0 | 0 |  |
| `internal/taskprojection` | ok | 1 | 1 | 0 | 0 |  |
| `internal/tasks` | ok | 3 | 5 | 0 | 0 |  |
| `internal/tasks/archive` | FAIL | 5 | 38 | 40 | 1 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/authority` | FAIL | 1 | 5 | 3 | 0 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/cli` | FAIL | 40 | 89 | 600 | 8 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/criterionbinding` | ok | 1 | 10 | 0 | 0 |  |
| `internal/tasks/defects` | FAIL | 1 | 2 | 7 | 0 | fail-closed refusal (V1-2486) |
| `internal/tasks/dispatch` | FAIL | 1 | 17 | 1 | 0 | fail-closed refusal (V1-2486) |
| `internal/tasks/fixture` | ok | 0 | 0 | 0 | 0 |  |
| `internal/tasks/importer` | ok | 1 | 3 | 0 | 0 |  |
| `internal/tasks/intent` | FAIL | 1 | 116 | 34 | 0 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/journal` | FAIL | 2 | 21 | 166 | 1 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/mutation` | FAIL | 1 | 252 | 1 | 0 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/obligation` | FAIL | 1 | 17 | 1 | 0 | fail-closed refusal (V1-2486) |
| `internal/tasks/release` | ok | 1 | 15 | 0 | 0 |  |
| `internal/tasks/safeopen` | ok | 0 | 0 | 0 | 0 |  |
| `internal/tasks/scale` | FAIL | 1 | 3 | 2 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `internal/tasks/scopes` | FAIL | 1 | 0 | 2 | 0 | fail-closed refusal (V1-2486) |
| `internal/tasks/service` | FAIL | 18 | 13 | 52 | 0 | fail-closed refusal (V1-2486) |
| `internal/tasks/snapshot` | FAIL | 4 | 245 | 5 | 2 | bug: PathText rejects C:\ (V1-2444/V1-2488) |
| `internal/tasks/store` | FAIL | 18 | 104 | 646 | 36 | fail-closed refusal (V1-2486) |
| `internal/tasks/supervisor` | FAIL | 1 | 13 | 2 | 0 | fail-closed refusal (V1-2486) |
| `internal/tasks/ticket` | ok | 1 | 112 | 0 | 0 |  |
| `internal/tasks/transaction` | ok | 18 | 480 | 0 | 0 |  |
| `internal/tasks/wire` | ok | 1 | 44 | 0 | 0 |  |
| `internal/tcq` | ok | 1 | 152 | 0 | 0 |  |
| `internal/testacceptance` | FAIL | 2 | 46 | 50 | 7 | fail-closed refusal (V1-2486) |
| `internal/testacceptance/cli` | ok | 0 | 0 | 0 | 0 |  |
| `internal/testevidence` | FAIL | 2 | 7 | 4 | 0 | path/logic bug (V1-2488) |
| `internal/testplan` | FAIL | 4 | 81 | 8 | 0 | masked refusal (V1-2487) |
| `internal/testrunner` | ok | 1 | 12 | 0 | 0 |  |
| `internal/testrunner/cli` | ok | 0 | 0 | 0 | 0 |  |
| `internal/testrunner/dynamic` | FAIL | 2 | 66 | 29 | 4 | test helper .exe/POSIX tool (V1-2490) |
| `internal/testrunner/mobile` | FAIL | 1 | 10 | 7 | 1 | environment (WDIO opt-in) |
| `internal/testrunner/native` | FAIL | 1 | 271 | 6 | 0 | POSIX test (V1-2491) |
| `internal/testrunner/platform` | FAIL | 1 | 37 | 10 | 0 | POSIX test (V1-2491) |
| `internal/testrunner/registry` | FAIL | 1 | 2 | 2 | 1 | environment (cmocka opt-in) |
| `internal/testrunner/sql` | FAIL | 1 | 17 | 1 | 0 | POSIX test (V1-2491) |
| `internal/testsource` | FAIL | 39 | 255 | 115 | 1 | masked refusal (V1-2487) |
| `internal/testsupport` | FAIL | 1 | 1 | 1 | 0 | fail-closed refusal (V1-2486) |
| `internal/testvalidity` | ok | 1 | 20 | 0 | 0 |  |
| `internal/testvaliditydoc` | FAIL | 1 | 63 | 3 | 0 | fail-closed refusal (V1-2486) |
| `internal/testvaliditydoc/externalorigins` | ok | 1 | 11 | 0 | 0 |  |
| `internal/toolchainreceipt` | FAIL | 1 | 2 | 1 | 0 | POSIX test (V1-2491) |
| `internal/touchsurprise` | FAIL | 5 | 2 | 9 | 0 | fail-closed refusal (V1-2486) |
| `internal/trace` | FAIL | 5 | 103 | 43 | 0 | fail-closed refusal (V1-2486) |
| `internal/tracemigraterepo` | ok | 0 | 0 | 0 | 0 |  |
| `internal/tracerecordrepo` | FAIL | 49 | 104 | 77 | 2 | fail-closed refusal (V1-2486) |
| `internal/tracerepopaths` | ok | 0 | 0 | 0 | 0 |  |
| `internal/unplannedread` | FAIL | 1 | 18 | 10 | 0 | fail-closed refusal (V1-2486) |
| `internal/untrackedallowance` | ok | 1 | 1 | 0 | 0 |  |
| `internal/update` | FAIL | 2 | 10 | 19 | 0 | fail-closed refusal (V1-2486) |
| `internal/upgradechanges` | FAIL | 60 | 19 | 17 | 0 | path/logic bug (V1-2488) |
| `internal/witness` | FAIL | 3 | 38 | 2 | 2 | fail-closed refusal (V1-2486) |
| `internal/witnesscollapse` | ok | 1 | 16 | 0 | 2 |  |
| `internal/worklistadapter` | ok | 0 | 0 | 0 | 0 |  |
| `internal/workqueue` | FAIL | 2 | 514 | 1 | 0 | fail-closed refusal (V1-2486) |
| `internal/worksource` | FAIL | 1 | 1 | 44 | 0 | fail-closed refusal (V1-2486) |
| `internal/worktreeimpact` | FAIL | 5 | 11 | 11 | 0 | fail-closed refusal (V1-2486) |
| `internal/wp3codec` | ok | 1 | 42 | 0 | 0 |  |
| `protocol/integrations` | ok | 1 | 2 | 0 | 0 |  |
| `script/spec-coverage-audit` | ok | 1 | 2 | 0 | 0 |  |
| `tools/beamfall-shadow` | ok | 1 | 0 | 0 | 0 |  |
| `tools/beamfall-shadow/fdexec` | ok | 0 | 0 | 0 | 0 |  |
| `tools/beamfall-shadow/fixtures/matrix/go` | ok | 0 | 0 | 0 | 0 |  |
| `tools/capabilities-check` | FAIL | 6 | 19 | 1 | 0 | path/logic bug (V1-2488) |
| `tools/cem-citation-baseline` | FAIL | 4 | 3 | 4 | 0 | fail-closed refusal (V1-2486) |
| `tools/cem-stable-regen` | ok | 19 | 47 | 0 | 0 |  |
| `tools/cem-structural-closure` | ok | 1 | 3 | 0 | 0 |  |
| `tools/cem-trial` | FAIL | 28 | 61 | 5 | 0 | path/logic bug (V1-2488) |
| `tools/ci-forward-row` | FAIL | 4 | 2 | 111 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `tools/ci-nightly-signal` | FAIL | 1 | 1 | 6 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `tools/ci-reuse-plan` | FAIL | 2 | 18 | 6 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `tools/ci-shard-costs` | FAIL | 2 | 66 | 2 | 0 | path/logic bug (V1-2488) |
| `tools/ci-test-slices` | ok | 8 | 14 | 0 | 1 |  |
| `tools/compat-trial` | FAIL | 1 | 0 | 0 | 0 | fail-closed refusal (V1-2486) |
| `tools/compat-trial/build-fixtures` | FAIL | 1 | 0 | 1 | 0 | fail-closed refusal (V1-2486) |
| `tools/compat-trial/fixtures/new` | ok | 0 | 0 | 0 | 0 |  |
| `tools/compat-trial/fixtures/old` | ok | 0 | 0 | 0 | 0 |  |
| `tools/corvint-pr-tests` | ok | 1 | 12 | 0 | 0 |  |
| `tools/cw-trial` | FAIL | 4 | 28 | 1 | 9 | fail-closed refusal (V1-2486) |
| `tools/docs-ci-plan` | FAIL | 43 | 51 | 4 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `tools/dogfood-guard-hunks` | ok | 0 | 0 | 0 | 0 |  |
| `tools/fixture-maintenance-check` | ok | 1 | 2 | 0 | 0 |  |
| `tools/gate-affected-select` | ok | 2 | 5 | 0 | 0 |  |
| `tools/gate-ledger` | FAIL | 20 | 145 | 24 | 0 | test helper .exe/POSIX tool (V1-2490) |
| `tools/heading-nav-bench` | ok | 2 | 7 | 0 | 0 |  |
| `tools/native-hook-observer` | FAIL | 1 | 14 | 3 | 0 | fail-closed refusal (V1-2486) |
| `tools/qualify-opencode` | ok | 0 | 0 | 0 | 0 |  |
| `tools/retrieval-bench` | FAIL | 2 | 59 | 31 | 2 | test helper .exe/POSIX tool (V1-2490) |
| `tools/str-parity` | ok | 1 | 2 | 0 | 0 |  |
| `tools/test-commands-check` | ok | 1 | 2 | 0 | 5 |  |
| `tools/test-commands-run` | FAIL | 1 | 2 | 1 | 0 | environment (-race needs cgo) |
| `tools/test-read-scopes-check` | ok | 5 | 3 | 0 | 0 |  |
| `tools/test-shards` | ok | 9 | 4 | 0 | 0 |  |
| `tools/unbounded-readers` | ok | 2 | 5 | 0 | 0 |  |
| `interop/cem01-go` | FAIL | 16 | 0 | 0 | 0 | test helper .exe/POSIX tool (V1-2490) |
