# nightly-fix: release-gates failures from run 41849536472

Base: `6c33df33bc5670131482f5263b410cb980580657` (`origin/main`). Tickets: V1-4552, V1-4550,
V1-4551, V1-4553 (with V1-4305 and V1-4413, which the V1-4553 items reproduce). The nightly
release-gates run 41849536472 on that base failed on linux-core and on the macOS legs. Every
amendment below is accepted under the owner's standing lane auto-accept.

## Decisions

- **MCP non-repository root (V1-4552, MCPV0-950).** `script/check-install-lifecycle.sh` step
  `mcp-unavailable-root` launched `corvint mcp --root <non-repository dir>` from the checkout, so
  the MCPV0-861 launch-worktree substitution served the checkout and `corvint.status` answered
  instead of refusing `invalid-root`. The step now runs from inside that directory and refuses with
  a named reason when the directory itself sits inside a Git worktree. The product behavior is
  unchanged; MCPV0-950's evidence text names the launch directory.
- **Spec requirement merge test (V1-4550, SRG-V0-013/016).** Two causes. The Makefile's shared
  `.PHONY: windows-main-check windows-main-test` line put two targets on one line, which the
  SRG-V0-982 one-target-per-line check refuses; it is two lines again. The README.md failure below
  stopped the test first, so that regression landed unnoticed. Separately, SRG-V0-762 now
  projects each spec's id range into its README.md row, so two lanes that add requirements to one
  spec also conflict in that row, and lanes on two specs whose rows are adjacent conflict there
  only. The test asserted the pre-SRG-V0-762 shape. SRG-V0-013 and the SRG-V0-016 limits now record
  the README.md row conflict, resolved by regeneration (`make spec-requirements-resolve`), and the
  test pins that each README.md marker holds only the named specs' rows and that the resolve leaves
  the merged range. This confirms the suspicion in V1-2392 (open, in lane msc-w65); no new ticket
  was filed. A prefix-only README cell that never conflicts is left to V1-2392.
- **macOS `/var` and `/private/var` (V1-4551).** `TestFixtureIgnoresInheritedGitRepository` and
  `interop/cem01-go` `TestCEMV1753` compared an unresolved `t.TempDir()` against paths the product
  reports after resolving symlinks. Both tests now resolve the fixture root with
  `filepath.EvalSymlinks` before comparing. `TestCEMV1753` is the same symlink family, not a
  separate cause. No product change.
- **macOS leg items (V1-4553).** Each item was reproduced on the Mac before its fix.
  - `adopter-ci-advisory-test`: the macOS TMPDIR ends in `/`, BSD `mktemp` keeps the `//`, and the
    script names the repository as `cd && pwd` prints it. The test canonicalizes the root and
    passes it with a trailing slash.
  - `bash32-check`: BSD awk under a UTF-8 locale stops on a binary first line (a tracked PNG). The
    finder runs under `LC_ALL=C` (ASD-V0-600 amended); `check-bash32_test.sh` case 4b covers it.
  - `companion-release-gate-test`: the TERM grace polled `ps` 100 times with no clock bound, which a
    loaded macOS host stretched past the 8 s join. The grace now also stops at a 5 s `SECONDS`
    deadline, and the test injects a 50 ms `ps`. The public-release-check twin loop is outside this
    gate (ASD-V0-942) and is left unchanged.
  - `dogfood-cache-test`: three assertions were stale after V1-4331/V1-4332/V1-4329 (the closeout
    now runs spec-capacity first, which refuses a BASE that is not a commit, and the precheck runs
    the script directly). The test uses the fixture's real base and the current recipe shape, and
    copies `check-spec-readme.sh`, which only the Makefile names. `check-spec-readme.sh` skips a
    tree with no spec corpus, as batch-precheck does (ASD-V0-2010, ASD-V0-2011 amended). The test's
    EXIT trap now names the failing line; Bash 5 runs the DEBUG trap for the EXIT trap's own
    command, so the trap ignores line 1.
  - `TestLGLV0885` (V1-4305): the macOS TMPDIR pushed the hold script past the 128-byte gate argv
    bound. The temp directory sits directly under `/tmp`, marked `fixed-tmp`.
  - `TestASDV0919RefusedLoopbackAddrRefusesAtOnce`: XNU drops a SYN to a bound, non-listening
    socket without a reset, so the dial waited out 5 s. Darwin now uses the closed-listener form
    (`refusedaddr_other.go`); ASD-V0-919 amended.
  - `IDXSNAPV0811` (V1-4413): a Darwin private mapping of a file truncated in place keeps serving
    cached pages instead of faulting. Each body served is still verified against its blob hash, so
    the read either refuses or returns exact bytes. The test moved to
    `mapped_fault_unix_test.go`: a `faultingMapping` region must refuse on both hosts, and the
    private mapping must refuse on Linux and serve only verified bodies on Darwin. IDX-SNAP-V0-811
    amended to state the Darwin behavior.
  - Out of scope, already ticketed: TestTCDV0770, TestTCLV0869, TestPURV02211, TestTCLV0802
    (V1-4335, V1-3558, V1-4526). `TestWd`/`TestBad` lines in the log are fixture output, not
    failures.

## Evidence

- Mac red then green: adopter, bash32, companion, dogfood-cache, TestLGLV0885 (3 of 3),
  TestASDV0919 (red 5.0 s, green 20 of 20), IDXSNAPV0811 (darwin: 10 verified bodies served).
- WSL green: `make install-lifecycle-test`, `spec-requirements-test`, `adopter-ci-advisory-test`,
  `bash32-test`, `bash32-check` (237 scripts), `companion-release-gate-test`,
  `dogfood-cache-test`, `fixed-tmp-paths-check`, `windows-vet`; the focused Go units; full
  `go test` of `internal/testsupport`, `internal/cem2proto` and `internal/contextindex`;
  `interop/cem01-go` `TestCEMV1753`; `make spec-requirements` reports no changes.
- Not run: the full `internal/tasks/store` package (over 500 s at host load 38); its only change,
  `TestLGLV0885`, passed focused on both hosts.
- `script/batch-precheck_test.sh` fails 24 cases on `--sync-check`: it refuses the shell steps that
  ci.yml's doc-gates job gained on the base in 7a0369d3b. This change touches neither file; the
  failure is V1-4525.
- `corvint affected --base 6c33df33b...` returned scope `UNKNOWN` with 181 selected units, wide
  because `internal/testsupport` and `check-spec-readme.sh` fan out to most packages. Under the
  owner's wide-selection rule the directly selected units ran here and batch CI covers the rest.

## Rollback

Revert the commit. No index, store or wire format changed.
