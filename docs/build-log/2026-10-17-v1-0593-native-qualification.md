# 2026-10-17 V1-0593: native runner qualification matrix and three adapter fixes

Outcome. All 18 native profiles now have an actual shared-executor witness for every
`TRE-V0-009` case: pass, assertion failure, skip, zero tests, collection, build or infrastructure
error, and timeout with descendant cleanup. Each witness ran through `corvint-test-runner plan`
and `run` on macOS 26 (Darwin 25.6.0) arm64.

- Before: 37 witnessed, 30 parser-only and 41 missing cases, out of 108.
- After the first pass: 102 witnessed, 5 parser-only and 1 missing (all Ginkgo v2).
- After the Ginkgo follow-up: 108 witnessed, 0 parser-only and 0 missing.

Retry does not apply to these profiles. Nextest pins `retries=0`, Ginkgo pins
`flake-attempts=1`, and no other native profile has a retry option. The matrix and its case names
are in the spec section "Native qualification matrix 2026-10-17 (V1-0593)". The evidence root is
`/private/tmp/cem10-build/native-2026-10-17/`:

- `cases/<case>/` holds the request, plan, tools, receipt and summary, plus `r/` with the raw
  report and streams.
- `casedefs/` holds the case inputs and `qual.py` the driver.
- `CLI-BUILDS.txt` records which CLI build ran each case.
- `cleanup-check.txt` records the descendant-cleanup checks.
- `boost-live-test.log` holds the opt-in `TestBoostTestLiveExecution` PASS.

Decision. Live runs found three adapter bugs. Each one already failed closed as incomplete, but
made a supported case unusable. The fixes are new requirements `TRE-V0-045` to `TRE-V0-047`,
accepted under the owner's standing lane auto-accept (2026-10-11). No reserved decision number
(0539 or 0540) was needed.

- `cargo-doctest`: rustdoc splits `--test-args` on whitespace, and every doctest name contains
  spaces. A selector therefore ran zero doctests. Such selectors are now refused at plan.
- `catch2`: Catch2 3.16.0 ANDs separate test-spec arguments, so two exact names selected nothing
  (exit 2). The names are now passed as one comma-joined OR argument.
- `catch2`: Catch2 exits 4 when every executed test is skipped, and the adapter treated that as a
  contradiction. Exit 4 is now success only for a non-empty, all-`SKIPPED` report.

The pre-fix runs are retained as `cases/cargo-doctest-pass`, `cases/catch2-mixed` and
`cases/catch2-skip`, all MISMATCH. No shared executor or validator code changed. All changes are in
`internal/testrunner/native/native.go` and its tests.

Toolchains. Installs were user-space only, under `/private/tmp/cem10-build/toolchains-2026-10-17/`,
with owner approval. There was no sudo and no PATH or profile edit.

- Rust 1.99.1 (`58cf8f9c3 2026-10-13`), minimal profile, under `rust/`:
  - Installer: rustup-init 1.29.1 from
    `https://static.rust-lang.org/rustup/dist/aarch64-apple-darwin/rustup-init`.
  - Installer SHA-256: `ec1b9233e7f72990ecd8e62063fa7f6c3dfc2bec8e97f88bff165f9100ac696a`, matching
    the published `.sha256`.
  - Command: `RUSTUP_HOME=rust/rustup CARGO_HOME=rust/cargo rustup-init -y --no-modify-path`, with
    the 1.99.1 toolchain.
  - Channel manifest SHA-256: `fb152be8…f463`. The full log is `rust/install.log`.
- cargo-nextest 0.9.146 (commit `8af696dd`), under `nextest/`:
  - Source:
    `https://github.com/nextest-rs/nextest/releases/download/cargo-nextest-0.9.146/cargo-nextest-0.9.146-universal-apple-darwin.tar.gz`.
  - SHA-256: `39785160b3c2f6ed9a765049cf4fa79f3b39aa02eb7598a5a0e2a1a0b9ffb9a8`, matching the published
    `.sha256`.
- NuGet test packages were restored from `https://api.nuget.org/v3/index.json` into
  `nuget/packages`. There are 40 package digests in `nuget/nupkg-sha256.txt`.
  - `dotnet nuget verify --all` passed for every primary MTP, NUnit, MSTest, xunit.v3 and
    TrxReport package (`nuget/verify.txt`).
  - Five old transitive packages fail with an expired repository countersignature
    (NU3028/NU3037). This is retained as a blocker.
- GoogleTest 1.18.0, `gh release download v1.18.0 -R google/googletest`.
  - SHA-256: `6e3191c1455468b3fc35a417fb565c1c5071aee1b7e7f85e30cf48a98d37d8b5`, matching the release
    digest.
  - Tag commit: `063de7e9578f82b369302001269680b4b1553359`.
- Catch2 3.16.0 amalgamated files, `gh release download v3.16.0 -R catchorg/Catch2`.
  - `.cpp` SHA-256: `1fe7f10334e0ae5494419cfa84c270f15235eada0fc02bdb493d1def537601f3`.
  - `.hpp` SHA-256: `d4cc143ea76ae212204363922d8adf376d66a1fda5a33ac73f93a7d1c119f4e0`.
  - Both match the release digests. The `.asc` files were not verified because `gpg` is absent.
- CMocka 2.0.2, `https://cmocka.org/files/2.0/cmocka-2.0.2.tar.xz`.
  - SHA-256: `39f92f366bdf3f1a02af4da75b4a5c52df6c9f7e736c7d65de13283f9f0ef416`, matching the
    committed provenance pin. The `.asc` was not verified because `gpg` is absent.
  - Static library built with CMake 4.4.4. The flags are the provenance flags:
    `BUILD_SHARED_LIBS=OFF`, `WITH_EXAMPLES=OFF`, `UNIT_TESTING=OFF`, Doxygen disabled.
  - `libcmocka.a` SHA-256: `6da0081e…1e44`.
- Boost 1.92.0, `https://archives.boost.io/release/1.92.0/source/boost_1_92_0.tar.bz2`.
  - SHA-256: `5c1d40cb8e19adbf740a4ec2da35b3e58f3f5804b1dce44deb53df72193cbc6c`, matching the
    committed provenance pin. Only the headers and the license were extracted.

Ginkgo. The first pass did not install Ginkgo: the session permission classifier denied
`go install github.com/onsi/ginkgo/v2/ginkgo@v2.33.0`, and the denial was not worked around.
After the owner asked to fix the blockers, the orchestrator installed the Ginkgo 2.33.0 CLI at
`toolchains-2026-10-17/ginkgo/bin/ginkgo` (`go install`, Go checksum database; SHA-256
`610e5b85…a00e`; manifest `toolchains-2026-10-17/MANIFEST-2026-10-17-orchestrator.md`).

- Fixture module `fixtures/ginkgo` (`example.com/gq`): `go get github.com/onsi/ginkgo/v2@v2.33.0
  github.com/onsi/gomega@v1.40.0` and `go mod tidy` with `GOPROXY=https://proxy.golang.org` and
  `GOSUMDB=sum.golang.org`. `go.sum` pins ginkgo `h1:C8gBA6Uc2ZEubiV+SXiu5tZnMTwEmXHgkJwGozKtZf8=`
  and gomega `h1:Vtol0e1MghCD2ZVIilPDIg44XSL9l2QAn8ZNaljWcJc=`. `go mod verify` reported all
  modules verified. Nothing else was downloaded.
- Six suite packages (`mixed`, `empty`, `bsfail`, `bsskip`, `ordered`, `hang`) were built with
  `go test -c` under `GOPROXY=off` and `-mod=readonly`. The binaries, `go.mod` and `go.sum`
  digests are in `fixtures/ginkgo/SHA256SUMS`. The suites call `RunSpecs` directly, so the CLI
  binary itself is not on the execution path.
- Cases, all OK with the unchanged adapter: `ginkgo-mixed`, `-pass`, `-fail`, `-skip`, `-zero`,
  `-beforesuite-fail`, `-beforesuite-skip`, `-ordered`, `-wrong-target` and `-timeout`.
  `ginkgo-timeout` reached its hanging spec, and no `/bin/sleep 987` survived.
- The runs confirm the source-derived limits in "Ginkgo v2 (experimental)": a resolved
  `SuitePath`, the `Suite skipped in BeforeSuite` reason with `SuiteSucceeded` true and exit 0,
  and the Ordered follow-on skip with zero attempts. No new requirement was needed.

Host tools already present:

- Go 1.27.2.
- .NET SDK 9.0.316 with runtime 9.0.18.
- CMake/CTest 4.4.4.
- Apple clang 21.0.0 (`clang-2100.3.34.2`).

Fixture notes:

- CMocka and Boost probes are the committed provenance sources. Separate `hang.c` and `hang.cpp`
  files add the timeout case.
- Every timeout fixture spawns a uniquely numbered `/bin/sleep` descendant. `pgrep` found none
  alive after any batch.
- MTP report directories ran under `/private/tmp/nq/` to stay within the `TRE-V0-027` 103-byte
  socket bound. Copies are kept at `cases/<case>/r-copy`.

Observed native behavior, retained fail-closed:

- CTest exits 8 for a disabled-only selection.
- MTP exits 8 for zero tests.
- Boost.Test exits 200 with no report for a selector that matches nothing. It also lists
  filtered-out tests as skipped, so `ExpectedTests` must be the full inventory.
- GoogleTest reports a `SetUpTestSuite` failure as an unnamed testcase. Parsing it fails with
  `native suite count mismatch`.

Residuals, not fixed here:

- Classify the GoogleTest unnamed suite-setup failure as suite infrastructure.
- CTest writes `Testing/Temporary/LastTest.log` into the caller's build directory.
- MTP leaves its socket files in the report directory's `.tmp` after a timeout.
- Five NuGet packages have expired countersignatures.
- The "Ginkgo v2 (experimental)" section and `ginkgo-provenance.json` still say runtime
  qualification is NOT_RUN. The matrix section records that the live runs supersede that for
  this host tuple.

Release blockers:

- Linux and Windows tuples for all 18 profiles.
- .NET 8 and 10, other MTP and adapter versions, and nightly Rust.
- `gpg` verification of the Catch2 and CMocka signatures.
