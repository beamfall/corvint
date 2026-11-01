# Lane clismall-sp2: small CLI, guidance, test-infrastructure tickets

Lane clismall-sp2 (Unraid, git 2.43.0, Go 1.27.3, umask 0002, no Landlock, no Node), base
origin/main `8daec98ce4349c1583186692726d2c3a782ac647`, branch `claude/lane-clismall-sp2-1101`.
Requirement IDs use the lane block 955-959 in each governing prefix.

## Delivered

- **V1-3528** (RGV-V0-955): `overview` and `features` no longer refuse a worktree whose only dirt
  is untracked paths. The snapshot reads only the pinned HEAD tree, which no untracked path can
  change, so they emit the receipt with `omissions.untracked` and one `unknown` line that says
  `MIXED worktree` and names the paths with the RGV-V0-050 bound and global-excludes hint. Decision:
  `review` and any tracked change still refuse, because review composes an affected plan over the
  worktree. The recheck before emission must see the same untracked set, else status drift.
  Consistency with `affected` (AC2): on the same tree affected plans the path into `plan.dirty`
  without refusing; it warns `MIXED` only under a `--base` range, so the test asserts the planned
  dirty path, not the word. `genesis.OpenGuidanceTolerating` is the new entry point.
- **V1-3529** (BKM-V0-955): the cause was not repository-root resolution. `moduleBoundary` admits
  a module boundary only through a go.mod that is itself a declared manifest source, and the Beamfall
  manifest declared `bundle.go` alone. Decision: keep the rule (no auto-declaring go.mod) and make
  the unknown name each ancestor go.mod path and why it failed: tracked but undeclared (with the
  `--source REPO:PATH` remedy), declared with a stale blob, or none tracked at the probed paths. The
  change is in `internal/breakagemap`; `internal/liveverify` was not touched.
- **V1-3607** (TCD-V0-955): `corvint-tasks ticket delivery-screen` accepts a bare `--json` as a
  no-op. The verb lives in `internal/tasks/cli`, a tasks-lane area; the change is the argument
  filter, the usage line and its catalog tree.
- **V1-3574** (IDX-SNAP-V0-955): the input-digest pin `auditedSHA256` is gone; the test logs the
  input digest only. The golden output pin is the one content pin, so two branches that each edit
  `receipt.go` without changing the golden pack rewrite no pin line (the shell fixture merges two
  such branches and sees a conflict only in `receipt.go` itself). AC2: the mutant test
  `TestAnalyzerSchemaOutputDetectsAnalyzerMutation` adds one symbol to the built index and proves the
  digest moves. AC3: `script/analyzer-schema.sh check` prints on stderr, without failing,
  `analyzer inputs changed since <merge base>: <paths>`; it runs as `make analyzer-schema-check`
  in doc-gates and the closeout precheck and by hand at change start. Wiring it into
  `make dogfood-change` is left to the dogfood-scripts lane, which owns that script. Residual: the
  golden corpus bounds the claim, as IDX-SNAP-V0-107 already says.
- **V1-3251**: `conformance/cli-parity-v0` gets a `TestMain` that calls `setProcessUmask()` as
  `main` does; `cmd/corvint-go-test-provider`'s `TestMain` pins umask 022 (`pinTestUmask`, unix
  only). The cli-parity failures reproduced under umask 0002 and pass under 0002 and 022 after the
  fix. The provider `toolchain` case passed here under 0002 before the fix; it is pinned the same
  way, but its WSL failure was not reproduced on this host.
- **V1-3531** (CEM-V1-955): `lrf --cem` (and `--ocm`/`--patch`) keeps the inside-root rule, the
  frozen `lrf/0` admission; an absolute path outside the root now refuses naming the rule, the
  resolved path, the root and `corvint cem verify --map` as the stable reader that accepts it.
  Decision: absolute paths only; relative `..` paths keep their existing read refusal, so the OCM
  readers that share `readRootInput` behave as before.
- **V1-3532** (RGV-V0-956): a `features` label that is a canonical feature ID carries `featureId`
  and the inert `featureCall` argv. Decision: no slugifying of prose labels; `corvint feature`
  answers READY only for a declared record, so the ID asserts no declaration. Help text updated.
- **V1-3542** (LCP-V0-955): the hook packet's governance selector in
  `coverage.unavailable_selectors` now carries `reason` (`downgraded-authority` with the TCP-V0-055
  to 057 `warnings`, so a modified `AGENTS.md` names `self-modified-authority`, or `unread`).
- **V1-3286** (CAP-V0-955): `clitree.Synopses` skips indented lines under an example header,
  before or after Usage. The regenerated `corvint` command tree drops the `docs` example lines that
  had become usage forms and a stray `-m` flag, and `docs consume` gains its required inputs.
  Help dispatch in `internal/clitree` was not touched.
- **V1-3582**: reproduced on git 2.43 (`--ref-format=reftable` exits 129). New
  `internal/gitprobe.Reftable` probes by running `git init --ref-format=reftable` and names the
  version and the 2.45 minimum; the gitstatus reftable case skips with it. A fake-git test covers
  both answers. `internal/tasks/store/claim_head_test.go` (V1-3568, tasks lane) can reuse it.
- **V1-3585**: AC1 was already met by `make test-read-scopes-check` (doc-gate):
  `TestRootResolvedRefusalNamesLocatingLiteralAndRule_V1_3234` covers an empty scope (`"p":[]`)
  and `TestUndeclaredRepositoryLiteralFailsNamingPackageAndPath_V1_1351` the package and path; both
  pass. AC2: new `docs/TEST-READ-SCOPES.md` explains an empty entry list, the static check, and the
  Landlock runtime twin commands, linked from `docs/SHELL-TEST-REGISTRATION.md`. A heuristic that
  flagged every empty-scope package whose tests locate the root was tried and dropped: it flagged
  about 15 packages that pass confined CI. The runtime twin could not be run here (`landlock abi 0`).
- **V1-3588**: git 2.43 prints the graft deprecation hint on stderr under `GIT_GRAFT_FILE=/dev/null`
  when it parses a commit. Every setter already read stdout apart from stderr; four lacked the
  suppression. `internal/gitnotes`, `internal/stepverify` and `internal/behaviorfalsify` now pass
  `-c advice.graftFileDeprecated=false`; the dashboard already did in `process.go`. AC1 grep: the only
  `GIT_GRAFT_FILE=` setters without the token in the same file are
  `internal/dashboard/repository/startup.go` (suppressed in its argv) and `internal/gitprobe` (it
  provokes the hint on purpose). AC3: `gitprobe.GraftAdvice` detects a hint-printing git, and the
  gitnotes and behaviorfalsify tests fail with the suppression removed and pass with it.
- **V1-3589**: measured on Unraid (12 cores, HEAVY slots 2). Before: cold test compile 12.72 s with
  std excluded; 7.30 s with production dependencies warm. The dominant cost was not one dependency
  but variant rebuilds: `external_live_test.go` (an external test package) imports
  `testvaliditydoc` and `testvaliditybridge`, which import `jstestprovider`, so Go rebuilt appmap
  (1.94 s), doccorpus (1.90 s), testsource (1.53 s), appflows (0.80 s) and the bridges against the
  test variant. The largest shared dependency, contextindex (4.18 s cold), is cached after a lane's
  first build. Decision: move the two opt-in live test files into `internal/jstestprovider/externallive`
  with adjusted relative paths, its own unbounded-readers entry and the spec's run command. After:
  cold 5.84 s, warm dependencies 2.24 s.
- **V1-3608**: `go test ./internal/tasks/ticket -run '^TestIssue502_RecordEscalationsKey$'
  -regenerate-issue502` writes the codec's encoding to both byte-identical fixture copies; it
  reproduced the committed bytes exactly from two corrupted files. Documented above the fixture
  constants and in the taskman copy's comment. The flag lives in a tasks-lane package.

## Not delivered

- **V1-3546** BLOCKED: it applies the one-hop symlink admission and typed refusals of
  TCP-V0-270 to 275 (lane V1-1272) to explicit task_evidence paths. Those requirements and their
  code are on no branch reachable from origin/main (`git grep TCP-V0-270` is empty at the base and
  on origin/main), so there is no admission to apply. Unblocks when V1-1272 lands.

## Tests

Focused, through `~/lanes/heavy.sh`: internal/genesis, cmd/corvint (guidance, LRF, help, catalog,
host adapter, local completion), internal/breakagemap, internal/tasks/cli, internal/contextindex
(analyzer schema, prompt, authority), conformance/cli-parity-v0 (umask 0002 and 022),
cmd/corvint-go-test-provider (umask 0002 and 022), internal/lrfrepo, internal/clitree,
internal/capcatalog, tools/capabilities-check, tools/test-read-scopes-check, internal/gitstatus,
internal/gitprobe, internal/gitnotes, internal/behaviorfalsify, internal/jstestprovider and
externallive, internal/tasks/ticket, internal/taskman, and `script/analyzer-schema_test.sh`.
Environment failures at base too: `TestHostAdapterJavaScript*` and
`TestQualifiedReporterSensitiveRedaction` need Node, which this host lacks.

## Friction

- heavy.sh: `heavy.sh env HEAVY_WHOLE_REASON=... go test ./pkg` is refused `whole-reason-missing`;
  the reason must be in heavy.sh's own environment, and the refusal does not say so. Repro:
  `~/lanes/heavy.sh env HEAVY_WHOLE_REASON=x go test ./internal/lrfrepo`.
- heavy.sh: a compile-only `go test -c -o /dev/null ./pkg` (runs no test) and `go test -run .` are
  refused as whole-package runs. Repro: `~/lanes/heavy.sh go test -c -o /dev/null ./internal/jstestprovider`.
- `cmd/corvint` `TestHostAdapterJavaScriptHosts` and `...HarnessInterruption` fail rather than skip
  on a host without Node. Repro on Unraid: `go test ./cmd/corvint -run HostAdapterJavaScript`.
- `make spec-requirements` refuses an Intent status header that grows past its frozen length; the
  message is clear, but every lane's first amendment trips it. Repro: append an acceptance note to
  a spec's `Intent status:` line.
- The confined-CI runtime twin cannot run in the Unraid container (`test-confine -probe` prints
  `landlock abi 0`), so read-scope findings that no literal shows surface only in CI there.
- `git grep TCP-V0-270` is empty on origin/main although V1-3546 (filed from lane V1-1272)
  presents the admission as existing; the ticket gives no dependency on V1-1272.
