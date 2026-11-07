# Native coverage live qualification follow-up

Owner request: lane covlive, V1-4369, V1-4370, V1-4371, V1-4375 and V1-4376,
base `39c03198086681fb24f509fbd00b34696d3268c4`, inheriting covcoll's fixture implementation.
Requirements: LPCV-V0-950, 954, 955, 963, 8200..8207 and 9400..9402.

Correction from self-review: the original JVM completeness and selection claims below are
withdrawn for the default launch tuple. Its jspawnhelper image was overwritten by a later Java
image in one PID. The repair record `2026-11-05-covlive-self-review-repair.md` supersedes those
receipts with cumulative image accounting and an explicitly FORK-only positive witness. Body
boundaries alone also did not establish eligibility; source-provider classification is now a
separate required input bound to both revisions and hunks. The original raw evidence is retained.

## Decisions and observed defects

Preserve native inventory instead of reconstructing it from terminal output. The live cmocka
invocation exposed that the collector erased the expected identities required by cmocka and
Boost.Test before either native builder could admit the run. LPCV-V0-9400 keeps the full ordinary
inventory and Boost inventory, and narrows cmocka's isolated inventory to the selected identity.

Use ConsoleLauncher's exact class#method selector for simple zero-argument discovery IDs
(LPCV-V0-9402). The official JUnit 1.11.4 console rejected the prior --select-unique-id invocation.
Dynamic, nested, escaped and parameterized IDs retain explicit caller mapping or refusal. Kotest
keeps its native unique-ID route; the existing Kotest fixture assertion exposed an overly broad
first repair and remains intact. No existing assertion, wire member, error/exit code, stderr
contract, golden or freeze output was changed.

Keep stock runner/build processes visible (LPCV-V0-9401). A blocking Linux ptrace observer records
launch, executable and retirement events, normalizes parent thread IDs to process IDs, and
separates threads from processes. Coverage file discovery does not prove completeness. Stock
cargo and SwiftPM launchers have no native export even when their instrumented descendants write
LLVM profiles; their collector records remain refused. No family is enabled for production
exclusion, and no inherited process-completeness check is relaxed.

## Measured controlled-project evidence

Raw evidence is under `/home/russell/lanes/covlive-evidence/`; the external handback identifies the
final source/bind heads, exact commands, terminal checks and artifact digests. Original failed
rounds remain beside successful captures. Projects execute under private lane scratch. Committed
fixture source bytes make the controlled-project inputs reviewable and reproducible; final live
indices bind their Git blob OIDs and the operator-supplied source commit.

The JVM and C/cmocka projects each run one ordinary suite and two isolated tests, including an
instrumented child. Their existing compact indices retain two test records, zero-hit lines and
unordered-package fallback. Each project also rebuilds four actual constant mutants with seed
9401: three kills, one unexecuted-line negative control, zero missed failing tests, and selected
set sizes 1, 1, 0, 1. The child-only mutant selects its parent test. The native fixture matrix
retains declaration fallback and conservative gap/insertion behavior. These eight controlled
mutants are development evidence, not LPCV-V0-963 qualification: they do not cover its full
statement/condition/gap sample or a pinned real Beamfall repository.

Rust and Swift Testing each execute the ordinary suite and both passing identities in isolation,
retain real LLVM LCOV exports and native reports, and decode those exports through the collector.
Their full process records remain incomplete because stock cargo/SwiftPM launchers do not emit
coverage. .NET SDK installation and --version succeed, but restoring the pinned xUnit/coverlet
project fails on the host's unavailable NuGet-Migrations named mutex before any tests execute.
The opt-in test retains that exact setup error and reports the .NET live run skipped/PARTIAL.
Xcode-only criteria are PARTIAL: Xcode is unavailable on WSL/Linux.

The requested `~/.local/toolchains` directory is read-only in this sandbox. Official toolchains
were staged under `/home/russell/lanes/covlive.tmp/toolchains` instead: Temurin 21.0.13+9, Maven
3.9.16, .NET SDK 8.0.427, CMake 4.4.5 with system GCC 15.2.0, rustc/cargo 1.99.1 with
llvm-tools-preview, and Swift 6.4.2. Published archive checksums matched for the JDK, Maven, SDK,
CMake, rustup and all four Rust components. JaCoCo 0.8.13 artifacts match Maven Central's published
SHA-1 values; their SHA-256 digests are separately retained. JUnit 1.11.4 matches its published
SHA-256. Swift's archive signature verified against the official Swift 6.x release key; its
computed SHA-256 is retained, but a published checksum was NOT_PRODUCED (both checksum suffixes
returned 404). Official Ubuntu dependency packages match their published SHA-256 values. Swift
needed ncurses, libxml2 and ICU dependencies; those were placed in its existing RUNPATH without
changing executable bytes after SwiftPM filtered LD_LIBRARY_PATH from compiler children.

Every ticket remains PARTIAL at its external qualification boundary. Authenticated read-only
repository discovery pinned beamfall/android at 887ef1d0a0598fa9ec0b8e1cb9b4dd7ae0e1f6a4,
beamfall/windows at 6ab0ef8446b85b6d8db5cac067affd70f2baa46c,
beamfall/linux at 95c0cda0d47187363a639e42030465bffe7ce33b and beamfall/apple at
c73138ea86e484f49333fade93144809e38d582d. Original manifests and agent contracts are retained.
These inputs are accessible; missing credentials are not a blocker. The Android app requires its
Android/Gradle environment, the Windows app targets net10.0-windows with WinUI, the Apple package
requires Apple platforms and its owner gate requires macOS, and the Rust workspace declares
GTK4/libadwaita. None of those real-repository fault-injection gates was run here. The requested
installation location is externally blocked; the stock-launcher capture/export and host gaps above
remain. The scratch-project witnesses do not stand in for the owner-host qualification. C/C++
retains its conservative source-provider/qualification boundary as well; no qualified native body
provider or full pinned-repository mutant receipt was produced.
Native Tasks completion and independent review belong to the orchestrator; this lane performs no
Tasks writes, publication, model-backed review or seal.

## Verification and dogfood dispositions

Used: initial `make dogfood-change` (its incomplete bind inputs remain visible), native Corvint
context and impact reads, affected plans before tests, existing whole native package tests through
the configured heavy semaphore, native capture/export, controlled mutation comparisons, spec
projection, collision checks, read-scope checking and line-citation repinning/checking. The new
fixtures are within the package's existing confined read scope. No read scope was broadened.
Prechange receipt capture refused because the inherited source tree already differs from the
mandated base; the separate context/impact reads are retained without relabelling them bound
prechange receipts. Initial and final NOT_PRODUCED rows remain in the handback.

The terminal whole-package native run includes all existing tests and the live harness, with the
immutable fixture-source commit supplied. The final handback retains its result and the clean
post-commit CEM report/check. The explicit local outcome is blocked by the external qualifications
above. Structural dogfood PASS is not ticket acceptance. Repository-wide tests, make gate,
Windows vet and independent model review are NOT_RUN under the scoped lane instructions.

Rollback removes the live harness and the two collector repairs, retaining the conservative
production fallback and all failed/unavailable evidence. No promotion or exclusion is implied.
