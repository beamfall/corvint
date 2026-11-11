# Go Line-Level Test Selection V0

Owner: Russell Lewis
Date: 2026-11-03
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-4364, V1-4365, V1-4366)
Delivery status: experimental
Authoritative inputs: owner goal of 2026-11-03 (a changed line runs only the tests it can affect,
locally and in CI, fast, proven by fault injection); decision 0590;
`live-proof-carrying-verification-v0.md` "Line-level coverage selection" (LPCV-V0-950..963);
`affected-plan-v0.md` (the static plan this narrows; at its 256-obligation cap, so this companion
holds the new requirements); `session-test-runner-v0.md` (`corvint test`); `AGENTS.md` invariants
2, 4, 7 and 8.

## Agent digest
- Claim: A fresh per-test Go coverage index, bound to the environment it observed, narrows a Go change to the tests its lines can reach, fail-closed.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-4364, V1-4365, V1-4366) / experimental
- Exists: the `linecov-index/4` compact format and lazy reader with per-test environment bindings, `corvint affected --line-index`, `corvint test --line-index`, the `linecov qualify` mutation harness with its report and qualification record, `linecov ci -selection` and the CI line-selection, miss-confirmation and kill-switch steps; a qualified NARROWED selection narrows locally and in CI (`GLS-V0-020`, `GLS-V0-040`, `GLS-V0-042`).
- Blocked on: the owner enablement decision for CI (`CORVINT_LINECOV_NARROWING`, LPCV-V0-955) after a fresh qualification: the proposed environment-completeness repairs (`GLS-V0-051..054`, V1-4514) change qualified sources, so the committed record stops admitting until it is regenerated.
- Read next: Enabled for 1.0; Requirements; Selection rule; Failure modes; Traceability.

## User and verified baseline

A developer or a CI shard changes a few lines of Go. Before this spec, `corvint affected` and CI
selected whole packages (`AFP-V0-*`), and a Go prototype (`tools/linecov`, `internal/linecov`,
decision 0590) could already collect a per-test coverage index and narrow a list of packages, but
nothing a user or CI runs consumed it: `corvint affected` had no line rule, CI ran it as a
report-only shadow (`LPCV-V0-959` step in `.github/workflows/ci.yml`), the gob index of a partial
module took 61 ms p50 and 90 ms p95 just to decode (12.8 MB, 236 packages, measured 2026-11-03 on
the `linecov-index` artifact of run 41663066310), and `linecov qualify` inserted only `panic`
statements. Three of that run's eight collection shards died (runner shutdown, exit 137).

The measurable job: on real merged changes, fewer tests run than the static plan selects, with zero
misses under fault injection, and the selection adds at most 100 ms p95 for a 50-file change
(`LPCV-V0-958`).

## Requirements

### Index format and collector (V1-4364)

- `GLS-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) Writers
  encode `linecov-index/4`: the magic line `linecov-index/4`, an 8-byte header length, a header
  holding the identity (module, commit, runner), the tests table, the unordered and in-order part
  lists, one file-table row per instrumented file (path, blob OID, block count, section offset,
  section length and the section's SHA-256), and an extension holding segments and divergent
  classes; then each file's block section, its block spans and its executing test and segment
  indexes as ascending delta-encoded unsigned varints. The index digest is `sha256:` of the header
  bytes, so it binds every section. Readers still read `linecov-index/2` and `/3`; `linecov merge`
  re-encodes them as `/4`.
- `GLS-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) A selector
  opens a `/4` index lazily: it reads and verifies the header and decodes a file's section only when
  an analysed diff touches that file, verifying the section's SHA-256 and that every test index is in
  range and ascending, every segment index in range and the blocks ordered. A bad magic, a header
  that does not parse or a section outside the file refuses the index; a section that fails
  verification makes that path's analysis an `index-section-invalid` fallback, so a damaged index
  decides nothing. A full decode, as merge, redundancy and the shared-step shadow need, verifies
  every section.
- `GLS-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) Opening the
  index, analysing a change of up to 50 changed Go files and deciding every unit MUST take at most
  100 ms p95 beyond the Git diff, blob and head test-file reads on the reference host
  (`LPCV-V0-958`); `TestLazyPlanBudget` checks it on an index of the measured Corvint shape, and the
  real-index measurement is retained in the build log. The `/4` index of the whole Corvint module
  stays below the `LPCV-V0-958` size bound.
- `GLS-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) The
  collector builds each package's `-cover -coverpkg=<module>/...` test binary, runs each top-level
  test alone with `GOCOVERDIR` propagated to every process it starts (`LPCV-V0-954`) and the whole
  suite once in order (`LPCV-V0-962`), and deletes the binary once that package's runs end, so its
  scratch holds at most the binaries of the packages in flight. `-tests RE` keeps only the isolated
  tests RE matches and `-inorder only|skip` runs only, or leaves out, the in-order suite, so a slow
  package is collected in parts. A part records the packages whose in-order run passed and those it
  skipped; `linecov merge` marks a package unordered when some part skipped its in-order run and none
  ran it. The index keeps every in-order record (amended 2026-11-04, V1-4364 review): an isolated
  record executing the same block does not show that a test behaves in order as it does alone, since
  an earlier test can set state that sends a later one through that block. A package that has
  records but no in-order record (an index pruned before this amendment, or built by hand) is
  unordered. Amended 2026-11-08 (V1-4364 review): every test binary gets the in-order suite record,
  also that of a package whose `_test.go` files declare no named test (a `TestMain`-only package),
  since its `TestMain`, `init` and package variables still run. Such a package's suite run that
  starts an uninstrumented child diverges. A unit whose test binary declares no named test and
  has no complete in-order record runs `whole` as `suite-unrecorded`, and one whose suite record
  the change or drift reaches runs `whole`; the HEAD test lists (`GLS-V0-016`) give such a
  directory an empty, non-nil list, apart from a directory without any `_test.go` file.
  A test that fails, times out or crashes alone, or is cancelled, yields no record. On Unix each
  run has its own process group and cancellation kills its descendants. Profile compaction drops
  only repeated zero-count blocks already seen in that package, retaining new child-binary blocks
  and every executed block; it never assumes all tests start the same binaries. A successful test
  leaving Unix descendants is `incomplete-process-tree`, kills them and publishes no record.
  Amended 2026-11-06 (merge of main's `LPCV-V0-7600`, `LPCV-V0-7601`): collection builds under
  main's window and collector overlays, so the in-order run attributes its reach per test window
  and every process a test starts is audited (`uninstrumented-child`, `missing-child-counters` yield
  no record). The windows widen the test records and never narrow the in-order record: a Go
  in-order record owns every block it executed (`Record.Union`), because a test that reaches a
  block alone and in order can still depend in order on state an earlier test left, which no
  window records. A window opens only in a binary that links coverage counters; a package whose
  instrumented code no test references keeps its union with the `window-order-unavailable`
  reason instead of a divergent in-order run. Amended 2026-11-08 (self-review repair): an uninstrumented child (git, a shell, node,
  a Go binary built without `-cover`, or any unknown executable) makes both isolated and in-order
  records divergent. Only independently instrumented children with counters support a record;
  a launch enabled by an earlier test need not occur in isolation. No complete suite record means
  the package stays unordered and runs whole. Every record also carries the inherited environment
  it was observed under (`GLS-V0-017`).
- `GLS-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) The runner
  identity is `go env` GOVERSION, GOOS, GOARCH, CGO_ENABLED, GOFLAGS and GOEXPERIMENT, the
  architecture feature settings GO386, GOAMD64, GOARM, GOARM64, GOFIPS140, GOMIPS, GOMIPS64,
  GOPPC64, GORISCV64 and GOWASM (each non-empty one as `NAME=value` in that order, as `arch=`;
  amended 2026-11-04: they choose the instructions or FIPS module a build compiles to, so builds
  differing in one can execute different code) plus the `atomic` coverage mode, computed by one
  function for collection and selection. An index whose identity names no `arch=` is a
  `runner-mismatch`. A `-p` or `-p=N`
  flag in GOFLAGS is left out of it: it bounds build parallelism and changes no executed code. Every
  other GOFLAGS flag, `-race` and `-tags` among them, stays. Amended 2026-11-04 (review round d):
  the identity also names the test runtime as `test=`, the flags `go test -json` passes every test
  binary: `-test.paniconexit0`, `-test.v=test2json` (so `testing.Verbose()` is true) and `-test.timeout=D` for a go test `-timeout` D above 0 (default 10m, so
  `t.Deadline()` reports the same deadline). Collection runs each test binary with exactly those
  flags, so a test that reaches code only in verbose mode or only under a deadline is recorded as
  it runs. `tools/linecov collect`, `select` and `ci` take `-test-timeout D` (default 10m); its
  `qualify` and `qualified` compare under the test runtime the index or record names. An identity
  that names no `test=` (an index collected before this amendment) or another one is a
  `runner-mismatch`. A GOFLAGS entry in the effective `go env GOFLAGS` that is not a build flag
  (`GLS-V0-020`'s list), `-p` or `-count=1` is passed by go test to the test binary, which
  collection does not apply, so reading the identity fails and selection is `runner-unavailable`.
  Amended 2026-11-04 (review round e): settings that can change what a test executes but that the
  identity does not name are refused, not modelled: reading the identity fails, so selection is
  `runner-unavailable`, when `go env -changed` reports GODEBUG, CC, CXX, FC, CGO_CFLAGS,
  CGO_CPPFLAGS, CGO_CXXFLAGS, CGO_FFLAGS, CGO_LDFLAGS or PKG_CONFIG changed from its default, or
  `go env GOWORK` names a go.work workspace (`GOWORK=off` and no workspace are read).
  Amended 2026-11-06: the identity also names, as `runtime=`, each set one of GOMAXPROCS, GOGC,
  GOMEMLIMIT and GORACE as `NAME=value`: the Go runtime reads them itself, not through `os.Getenv`,
  so the environment audit of `GLS-V0-017` cannot see them, and a different value may change what
  a program executes. An identity without `runtime=` is a `runner-mismatch`. It also names, as
  `exec=`, the `go test -exec` wrapper the tests run under: "" for none, else the SHA-256 of the
  wrapper file, so an index collected under CI's confinement wrapper (`.github/testconfine`, which
  denies a declared package every read below the root outside its scope and adds
  `-buildvcs=false` to its GOFLAGS) decides nothing for a run without it or under another build of
  it; an unreadable wrapper is `runner-unavailable`. `linecov collect`, `select`, `ci`, `qualify`
  and `qualified` take `-exec PATH`; collection runs every test process as `PATH BINARY ARGS...`
  from the package directory, as go test does.
  Amended 2026-11-08 (self-review repair): caller `-overlay` is outside the closed build-flag
  allowlist. Collection installs its own overlay from checkout sources and cannot certify a caller
  replacement's compiled content. Both direct collection and runner identity refuse it through
  the existing unsupported-GOFLAGS path; unknown flags also refuse. A future composed-overlay
  implementation needs separate content-binding evidence before admission.

### Selection in `corvint affected` (V1-4364)

- `GLS-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364)
  `corvint affected --base FULL_COMMIT_ID --line-index FILE` adds the optional member
  `lineSelection` (profile `affected-line-selection/0`) to the `affected-plan/1` receipt; `plan`,
  `provider` and `advice` are byte-identical with and without it. `--line-index` is outside the
  CCF-V1 freeze (as `--selection-profile` is, CCF-V1-003). It requires `--base` and refuses with exit
  2 and `unsupported-affected-line-index-profile` under `--full`, `--summary`, `--format text`,
  `--by-path`, `--path`, `--paths-from`, `--snapshot`, `--provider`, `--playwright-config` and
  `--appmap-impact`, naming the supported form. An unreadable or non-regular FILE is not a refusal:
  it is the `index-unreadable` fallback. `--line-test-timeout D` (amended 2026-11-04) names the go
  test `-timeout` of the runs the selection is for, so the runner identity binds their `test=`
  runtime (`GLS-V0-009`); it defaults to 10m, go test's default. Without `--line-index`, or with a
  value that is not a non-negative Go duration, it refuses with exit 2 and `invalid-arguments`.
  `--line-exec PATH` (amended 2026-11-06) names the go test `-exec` wrapper the runs are for, so the
  identity binds it (`GLS-V0-009`); without `--line-index` it refuses with exit 2 and
  `invalid-arguments`.
- `GLS-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364)
  `lineSelection.state` is `NARROWED`, `SHADOW` or `FALLBACK`. SHADOW retains the proposed
  units, counts and exclusion evidence but authorizes no execution narrowing; its reason is
  `unqualified` when `GLS-V0-034` admission fails, with the failure in `detail`. FALLBACK names one `reason` from this closed set
  with a `detail`, and then decides no unit, so every unit keeps the static plan: `index-unreadable`,
  `index-format`, `module-mismatch`, `runner-mismatch`, `runner-unavailable`, `index-not-ancestor`
  (the index commit is not an ancestor of the base), `worktree-dirty` (the worktree status the plan
  read is not empty), `static-scope-unknown` (`plan.scope` is UNKNOWN), `fallback-scope-unknown` (the
  static plan of the fallback paths of `GLS-V0-012` is UNKNOWN), `git-unavailable` and
  `environment-unbound` (the index records no environment bindings, `GLS-V0-017`; amended
  2026-11-06). The 2026-11-04 `line-narrowing-disabled` SHADOW reason is withdrawn (Enabled for
  1.0): a selection that passes admission is NARROWED.
- `GLS-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) The analysed
  change is the drift from the index commit to the base and the change from the base to HEAD, each
  path read at both immutable sides. Missing new-side blob or hunk evidence is `unreadable`;
  either side failing to parse is `unparsable`. Both sides must preserve declarations and
  signatures, and each paired hunk must lie within the same function on both sides; a boundary
  change or introduced declaration (including `init`) is `declaration`, or `test-helper` in a test
  file. A changed test may rename or change its signature only while both sides remain top-level
  tests. Each path gets one class: `body` (a Go non-test file recorded at its
  old blob whose every paired hunk lies inside the same function body on both sides), `test-function` (a Go test file whose
  every hunk lies inside top-level Test, Example or Fuzz functions), or a fallback category of
  `LPCV-V0-953`: `non-go`, `added`, `deleted`, `declaration` (a hunk outside every function body),
  `uninstrumented` (a Go non-test file the index does not hold), `blob-mismatch` (the index holds
  another blob of it, including a path the drift already changed), `test-helper` (a Go test-file hunk
  outside every top-level test function), `unparsable`, `unreadable` or `index-section-invalid`. The
  receipt lists every path with its side (`drift` or `change`) and class.
- `GLS-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) Only Go units
  `plan.selected` names are decided. A unit the static plan of the fallback paths selects (one
  `affected` walk over them on the same graph) runs `whole` with the `LPCV-V0-953` category of that
  walk's witnessing path; a unit whose package is unordered (`LPCV-V0-962`) runs `whole` as
  `unordered`, and one reached through its in-order record as `order-sensitive`; an edit of a test
  function also reaches its package's in-order record (amended 2026-11-04), since the state that
  test leaves can change what a later test reaches in order. Static read
  dependencies survive independently of the winning static witness: a unit the static plan's
  reader rules select for any analysed Go path of the drift or the change, run on those paths
  alone (a path literal or glob naming the path, an unbounded reader, a declared read scope or
  existence declaration holding it, an import-graph reader the change can alter, a declared test
  owner of it, or a package embedding files under its directory together with every transitive
  importer and runner of that package and their test-only users and runners, since the embedded
  bytes leave the package through its API; `affected.Graph.Readers`), stays
  `whole` as `static-read`, even when a direct source/import witness won: execution coverage cannot
  prove which source bytes a test observes. A declared must-run unit of `plan.mustRun`
  (`AFP-V0-2484`) runs `whole` as `must-run` (amended 2026-11-08, PR 147 review). Amended 2026-11-06: the 2026-11-04 form kept every
  unit with any path token whole, and since the graph records plain string literals as tokens
  that left almost no unit to decide; the reader rules are the static plan's own answer to which
  units may read a path, so the baseline is unchanged. A unit whose in-order record was observed
  under bindings the run environment does not hold runs `whole` as `environment` (`GLS-V0-018`),
  and a unit whose test binary declares no named test and has no complete in-order record runs
  `whole` as `suite-unrecorded` (`GLS-V0-008`, amended 2026-11-08).
  In any other unit a
  test declared at HEAD (`GLS-V0-016`) is kept when its record executed a changed body range of the
  change (`selected`; a pure insertion intersects when either neighbouring base line ran, and a
  changed code token outside every block selects every test that ran a block of its function), when
  its record executed a changed range of the drift (`stale`), when the change edits its function
  (`direct`) or the drift does (`stale`), when it has no complete record (`unrecorded`: new at
  HEAD, never collected, isolation-divergent, timed out or incomplete), or when its bindings do not
  hold under the run environment (`environment`, counted as fallback category `environment`). The unit's `mode` is `whole`
  when every declared test is kept, `tests` with `run`, the anchored `-run` pattern of the kept
  names, when some are, `compile` (`run` `^$`) when none is and the change edits one of its Go files,
  and `exclude` otherwise.
- `GLS-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) A NARROWED
  selection carries one `exclusion` entry that every left-out test of a `tests`, `compile` or
  `exclude` unit refers to: the index digest as the record digest, every analysed `body` and
  `test-function` range as `path:FIRST-LAST` with its side, and the invalidation condition `any
  recorded blob OID changes`; each such unit states how many declared tests it leaves out.
- `GLS-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) The selection
  reports counts of tests and units kept by coverage, kept by static fallback per category, and left
  out, and `loadMillis` (open and header), `selectMillis` (analysis and decisions) and `gitMillis`
  (diffs, blob and head test reads) apart (`LPCV-V0-961`).
- `GLS-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364) The tests a
  unit declares at HEAD are the top-level `Test`, `Example` and `Fuzz` functions `go test` would run
  of every `_test.go` file in its directory under any build constraint, read by a token scan of the
  file, so a name only a tagged or other-platform build compiles is kept, never excluded.
- `GLS-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364, V1-4365;
  added 2026-11-06) Collection observes the inherited environment every test reads. The collector
  overlay rewrites `os.Getenv`, `os.LookupEnv` and `os.Environ` in the test binary and in every
  instrumented child it starts to report, once per process to the file `CORVINT_LINECOV_ENVAUDIT`
  names, each read of a variable whose value is the one the process started with (`read NAME
  VALUE`, or `unset NAME`), and a read of the whole environment (`all`). A read of a value the
  program set itself, or of a name its parent set for it (`CORVINT_LINECOV_ENVSELF`, written by
  the exec audit from the variables the parent changed), is the program's own setting and is not
  reported. The record binds each reported variable to the SHA-256 of the value it saw or to
  `unset`; the index keeps no value. Neutral names are never bound: PWD (the package directory on
  every host by go test's contract; its absolute prefix is host placement), and OLDPWD, SHLVL and
  `_` (the invoking shell's bookkeeping of its own last command and directory, which differ
  between two invocations of the same run). Amended 2026-11-08 (V1-4364 review): the variables
  the collector sets or changes for every collected process (GOFLAGS, which gains the coverage
  and overlay flags, GOCOVERDIR and `CORVINT_LINECOV_*`) are no longer neutral. A test's
  inherited read of one is reported as `modified NAME` and binds `collector-modified`, which no
  environment holds, so the test is always kept and a package whose in-order record holds one
  runs whole: what it did under the collector's values says nothing of a run without them. Reads
  whose first caller outside package `os` is the coverage runtime (`internal/coverage/cfile`) or
  the exec audit (`os/exec`) are the collector's own and are not reported; the counter windows
  read their variable through the collector-only quiet syscall accessor. A whole-environment read observes the collector's
  variables too, so it also binds `collector-modified`. An unreadable or malformed audit yields
  no record. The index records the bindings table and each test's bindings in the `/4` header,
  and `EnvBound` only when every record was observed. Amended 2026-11-08 (V1-4364 admission
  review; amended self-review repair): the header carries audit version `3` as its final, unreferenced environment-table
  entry, named NUL followed by `linecov-env-audit`. This metadata adds no wire field. The reader
  removes that entry before resolving test bindings and admits only this exact marker, present
  once at the end, with `EnvBound=true`. Missing, unknown, duplicate or misplaced markers, and
  the older gob formats, bind no environment even if the old collector wrote `EnvBound=true`.
  They take the existing `environment-unbound` fallback; merging or rewriting cannot upgrade
  them. Recollection is required. Individual legacy binding comparisons confer no admission.
  Amended 2026-11-08 (self-review repair): the overlay also observes `syscall.Getenv` and
  `syscall.Environ`, including `time`'s lazy TZ lookup, and retains a whole-environment claim for any read before `os` initialization.
  The allowlist is the explicitly supported Unix targets using `syscall/env_unix.go`, with each
  expected accessor signature matched exactly once; other targets or unmatched toolchain forms
  refuse through `unsupported-collector-toolchain`. Named reads bind their values; a whole
  environment consumer of any shape binds `collector-modified` and cannot justify exclusion.
  The only quiet inheritance stack is `syscall.Environ` through `execenv.Default`,
  `Cmd.environ` and the audited `Cmd.Start`; public `Cmd.Environ` and every unknown stack remain
  observations. Early reads cannot prove their original caller/value and also retain that claim.
  Audit versions 1 and 2 lack this completeness evidence and require recollection; the index
  encoding, fields and fallback codes are unchanged.
- `GLS-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4364, V1-4365;
  added 2026-11-06) A selection compares the bindings of every test it would leave out, and of
  each decided package's in-order record, against the environment the tests will run under: this
  process's for `corvint affected`, the profile's for `corvint test` (`GLS-V0-020`), the shard's
  in CI (`GLS-V0-041`) and the harness's own for `linecov qualify`. A test whose binding does not
  hold (a value differs, a variable is set or unset, or a whole-environment read under any other
  environment) is kept as `environment`; a package whose in-order record does not hold runs whole
  as `environment`, since what the order reaches now is unknown. An index that binds no
  environment is the `environment-unbound` FALLBACK in `corvint affected` and keeps every package
  whole in `linecov select`, `ci` and `qualify`; a language shadow compares no environment, as its
  plan is a candidate only. Consequence: an index narrows the tests of a package only under an
  environment equivalent, in every variable its tests read, to the one it was collected under; a
  CI index narrows CI shards whose environment the index workflow shares, and a locally collected
  index narrows local runs. Host properties no variable names (CPU count, time, network, files
  outside the module) stay unmodelled (Non-goals); the main full-run miss detector remains the
  backstop (`GLS-V0-046`).

### Local runner (V1-4366)

- `GLS-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366; the
  2026-11-04 deferral is withdrawn 2026-11-06, see Enabled for 1.0)
  `corvint test --base FULL_COMMIT_ID --line-index FILE` compiles the same selection with the runner
  identity of the Go environment its profile runs tests in, whose values the selection's bindings
  must hold (`GLS-V0-018`), under the CI profile's `--confine` wrapper as its `exec=` (amended
  2026-11-06), and applies a NARROWED one to a NARROWED
  manifest: an `exclude` unit leaves the run set and is listed in `excluded` with its reason, a
  `compile` unit runs `-run ^$`, and a `tests` unit runs the union of its kept tests and any
  STR-V0-063 or TSP-V0-006 slice of the same package. Only a qualified `NARROWED` selection may apply. A FULL manifest, a SHADOW or FALLBACK selection and a
  `whole` unit are unchanged. The manifest records the index digest as `lineIndex`, so every shard of
  one manifest binds the same index. The runner identity is read under the profile's Go environment
  with every build flag the profile passes on the command line added to GOFLAGS as `-name` or
  `-name=value` (amended 2026-11-04): `-race`, `-msan`, `-asan`, `-trimpath`, `-linkshared`,
  `-buildvcs`, `-tags`, `-ldflags`, `-gcflags`, `-asmflags`, `-mod`, `-pgo`, `-installsuffix`,
  `-compiler`, `-gccgoflags`, `-buildmode`, `-modfile` and `-toolexec`, with one or two
  leading dashes and a value inline or as the next word. Amended 2026-11-04 (review round d): those
  flags are added to the effective GOFLAGS, read by `go env GOFLAGS` under the profile's environment,
  so a value saved with `go env -w GOFLAGS=...` is kept; a failing `go env` is `runner-unavailable`.
  `-p`, `-failfast` and `-count=1` are left out of GOFLAGS: `-p` changes no executed code, and
  `-failfast` and `-count=1` change which tests run, not what one executes. Amended 2026-11-04
  (review round e): `-v`, `-json` and `-test.v`, in any spelling or value, are `runner-unavailable`.
  A declared flag follows `corvint test`'s `-json`, so `-v=false` turns `testing.Verbose()` off and
  `-json=false` drops the test2json verbosity collection runs under; the effective verbosity is not
  modelled. A declared `go.env` variable other than a Go toolchain setting the identity reads
  (GOTOOLCHAIN, GOOS, GOARCH, CGO_ENABLED, GOEXPERIMENT, GOFLAGS, GOWORK, GOENV and the architecture
  feature settings) or one that changes no executed code (GOCACHE, GOCACHEPROG, GOMODCACHE, GOPATH,
  GOTMPDIR, GOBIN, GOPROXY, GOSUMDB, GONOPROXY, GONOSUMDB, GOPRIVATE, GOINSECURE, GOVCS, GOAUTH,
  GOTELEMETRY, GOTELEMETRYDIR) is `runner-unavailable` too: an application setting, GODEBUG, a
  runtime setting such as GOMAXPROCS or a C toolchain is visible to the tests but not to collection. `-timeout` is not
  neutral: its duration (default 10m) is the identity's test deadline (`GLS-V0-009`), and a value
  that is not a duration is `runner-unavailable`. Any other flag (a
  test-behaviour, coverage or exec flag, or `-count` other than 1), a valued flag without its value,
  or a value GOFLAGS cannot carry (whitespace, a quote or a backslash) is the `runner-unavailable`
  fallback, whose detail names the flag, so the static plan runs.

### Fault-injection qualification (V1-4365)

- `GLS-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4365)
  `linecov qualify` draws a sample of mutation sites, seeded and deterministic, from the function
  bodies of the files of `-packages` that the index holds, each in a function some record executed.
  Operators: statement deletion (the statement wrapped in `if false { ... }`, so it keeps compiling),
  condition negation (`!(C)` of an if, else-if, for or tagless-case condition) and constant change
  (an integer or float literal plus one, a string literal extended, `true` and `false` swapped). It
  takes sites of every kind first, in turn: `case` expressions, `else-if` conditions, `loop` headers
  and statements inside `closure`s, then plain `statement`s, up to `-mutants`.
- `GLS-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4365) Each mutant
  is committed on a disposable checkout at the index commit, selected by the same decision function
  `corvint affected` uses with every `-suite` package as the static plan, and run by `go test -json`
  over every `-suite` package. A test that fails on the mutant and is not kept is a miss. A mutant
  that does not build, or whose run leaves a suite package without a result, is `invalid`: it is
  counted and replaced by the next site, never counted as killed or as a miss. Before mutating,
  the harness runs a clean baseline, one suite package per invocation, and stops on its first
  failure with the observed package results in the report. A failed baseline cannot qualify.
  Mutant commits use the repository owner identity. Each mutant likewise runs suite packages
  individually, preserving every failure rather than stopping after the first killed test.
  Amended 2026-11-06: a failure counts only for a test its package declares at HEAD; `go test
  -json` frames results from output lines, so a test that prints another run's `--- FAIL: Name`
  (the tools' own tests collect fixture modules) yields a result for no declared test, which is
  output, not a failure. The collector indents a divergent run's output in its diagnostic for the
  same reason. A declared test's failure is never dropped.
- `GLS-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4365) Negative
  controls: `-controls N` statement-deletion mutants in function bodies no record executed. Each
  MUST keep no test of its package (it may build it) and fail none; a kept test or a failing one
  fails the qualification. Amended 2026-11-06: a body counts as unexecuted only in a package whose
  every declared test, and every declared test of each suite package depending on it, has a
  record; an unrecorded test could execute any body, so such packages yield no control and the
  harness names them. The kept set a control may not have is a test of its own package or a test
  kept by coverage anywhere (`selected`, `direct`, `stale`, or a unit the mutated path's witness
  keeps whole); the unrecorded tests of other suite packages are kept whatever the change and do
  not fail a control.
- `GLS-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4365) The report
  (`-report FILE`, JSON `linecov-qualification/1`) retains the seed, index digest and commit, runner,
  the mutated and suite packages, every mutant's site, kind, operator, kept and failing tests, misses
  and selection time, the counts of mutants, kills, invalid mutants and misses, kept-set sizes, and
  every control's result. `-start K -count N` runs the K-th to K+N-1-th sites of the one seeded
  order, and `linecov qualify -summarize REPORT...` refuses reports of another index, seed or suite or
  overlapping ranges and folds the rest. Any miss or failed control exits 1.
- `GLS-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4365) The
  qualification record `.github/linecov/qualification.json` (`linecov-qualified/1`) names the family
  `go`, the summarized report's digest, mutant, kill, control and miss counts, the runner identity it
  ran under, and the blob OID of every selector and collector source file it qualified (the non-test
  Go files of `internal/linecov` that index, select and collect, the qualification and CI helpers
  in `tools/linecov`,
  `cmd/corvint/affected_linecov.go`, and `internal/liveverify/affected/readers.go`, whose reader
  rules decide which units an exclusion may drop; amended 2026-11-08). `linecov qualify -summarize -record FILE` writes it only for
  zero misses, at least one killed mutant and passing controls. Reports bind the implementation
  source blobs at the clean VCS revision recorded in the running executable; an executable without
  that provenance may produce an experiment report but cannot produce a qualification record.
  Summarization refuses mismatching source bindings and cannot relabel an old report with newer
  source blobs. The CI `qualified` check passes the shard's `-exec "$RUNNER_TEMP/test-confine"`
  wrapper (amended 2026-11-08, V1-4365 review), so it computes the same runner identity as a
  record the harness produced under that wrapper. Coverage exclusion in CI requires a record whose blobs equal the
  base commit's (`GLS-V0-043`); any change to those files needs a new qualification. Local
  admission reads the record from the immutable planning base and checks positive mutant, kill
  and control counts, zero misses, the report digest, the runner, and all source blobs against both
  that base and the clean VCS revision stamped in the executing selector. Missing executable
  provenance, a dirty executable build, an unreadable record, or any mismatch retains a SHADOW
  plan (`unqualified`). An index alone grants no local exclusion authority (`LPCV-V0-955/963`).
- `GLS-V0-035`: (experimental under the owner's standing lane auto-accept, 2026-11-11; V1-4365)
  The harness MUST also be qualified on a cross-package seeded-fault fixture: a module whose package
  `lib` carries a site of every `GLS-V0-030` kind and an unexecuted body, whose own test runs only
  one of its functions, and whose package `app` imports `lib` and is the only package whose tests
  run the rest. Every site of the fixture's order, run against the suite `lib,app`, MUST report
  zero misses and a passing control, with at least one `lib` mutant killed only by a test of
  `app` and kept for it through the `-coverpkg` record. An index in which that test's record lost
  the blocks of the function it alone ran (attributed instead to `lib`'s test, so the function
  stays sampled) MUST make qualify fail naming the `app` test as missed. This closes the class a
  single-package fixture cannot reach, a fault whose only failing test lives in another package.
  Non-goals: the per-language corpora of the V1-4365 acceptance (Beamfall repositories per family
  and a public Ruby project), which need external repositories this fixture does not stand in for;
  a larger Corvint-tree sample, which is the record of `GLS-V0-034`. Failure mode: a selector that
  keeps only the mutated package's tests passes the single-package fixture yet excludes the
  importing package's test that a fault fails. Rollback: delete
  `tools/linecov/qualify_crosspkg_test.go`; no production code depends on it.

### CI (V1-4366)

- `GLS-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366; the
  2026-11-04 deferral is withdrawn 2026-11-06, see Enabled for 1.0) A pull-request shard from a branch of the repository into main and a merge-queue shard of main narrow
  by line coverage only while the repository variable `CORVINT_LINECOV_NARROWING` is `on`; otherwise
  the same step runs report-only and changes no list. A SHADOW receipt, including an unqualified
  selector's receipt, always forces report-only mode even when the variable is on. `linecov ci
  -selection` retains the proposed before/after comparison for SHADOW with its state and reason;
  neither that receipt nor its comparison grants narrowing of the shard's executed lists.
- `GLS-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) The step
  takes the newest unexpired `linecov-index` artifact published from main whose recorded commit is
  an ancestor of the shard's base, and plans with `corvint affected --base BASE --line-index` and
  `linecov ci -selection`, both built from the base when the base is on main (else from the newest
  commit of main it descends from), never from the tested tree, under the shard's own GOFLAGS and
  CGO_ENABLED, with the shard's `-timeout` as `--line-test-timeout` and its `test-confine` wrapper
  as `--line-exec` (amended 2026-11-06), so the runner identity can match; the index workflow
  collects under the same wrapper, `CORVINT_CONFINE_ROOT` and `-test-timeout`.
- `GLS-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366; the
  2026-11-04 deferral is withdrawn 2026-11-06, see Enabled for 1.0) `linecov ci
  -selection` rewrites only the shard's whole-package list: an `exclude` unit leaves it, a `tests`
  unit becomes the slice `-run PATTERN PACKAGE`, a `compile` unit `-run ^$ PACKAGE`, and every
  other package, including one the selection does not decide, stays; the slices of split packages
  pass unchanged. It never adds a package. A shard narrowed this way records `lineNarrowed: true`
  in its coverage marker, sets no FULL tested-tree output, and its merge-queue entry or main push
  runs as a narrowed run does: the `AFP-V0-170` record job needs every shard's FULL output under any
  narrowing verdict, since this step narrows a shard whatever the `AFP-V0-098` verdict is (amended
  2026-11-04).
- `GLS-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) A missing,
  expired or unreadable index, an index commit that is not an ancestor of the base, a FALLBACK
  selection, a tool that cannot be built or refuses, a qualification record absent or naming other
  blobs than the base's, a kill switch in effect (`GLS-V0-047`) or any failed step leaves the shard's
  lists exactly as the static plan made them and names the reason in the step summary. A selection
  proof upload must succeed before any shard can narrow; a failed upload keeps the static lists. Nothing is
  ever skipped silently.
- `GLS-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) Every shard
  that planned retains `linecov-narrow-SHARD` (14 days): the selection, the lists before and after
  and the reason, so the exclusion entries of every narrowed run are readable by the miss detector.
  `applied` contains `NARROWED` only after a successful rewrite; the detector compares only packages
  actually excluded or sliced in `after/plan.json`, with matching selection and metadata ranges.
  After tests, uploading this completed evidence must succeed for a shard that applied narrowing;
  failure fails that shard even if the earlier selection proof upload succeeded. Shards that did
  not apply narrowing remain unaffected by an evidence upload failure.
  Shadow plans, unchanged split packages and missing application evidence never establish a miss.
- `GLS-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) The
  `linecov-index` workflow refreshes every record on main on its schedule and on dispatch, collecting
  in 16 shards whose collector deletes each test binary after its package (`GLS-V0-008`) and runs at
  most two test processes per core.
- `GLS-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) Miss
  detection (`LPCV-V0-957`): when a full push run of main fails, the `linecov-miss` workflow reads its
  failed top-level tests and the `linecov-narrow-*` artifacts of the merge-queue and pull-request
  runs of each pull request merged since the last green full run; a failed test one of them left out
  is a suspected miss (`linecov miss`). Each suspect carries the runner identity of the index the
  narrowed selection read (`runner`). It is confirmed only when, built from main under the same
  GOFLAGS, the test passes at that pull request's base and fails twice at its merge commit; amended
  2026-11-08 (V1-4366 review), each run reproduces that runner: the shard's `-race` build, the
  `test-confine` wrapper built as the CI shards build it with `CORVINT_CONFINE_ROOT`, the
  recorded architecture variables, GOEXPERIMENT and an empty runtime environment, and the
  recorded test deadline as `-timeout`. A suspect whose runner is absent or differs in any of
  those fields, or that cannot finish three runs under its deadline inside the job's budget,
  stays suspected and is never confirmed; a confirmation retains the reproduced runner. A failure
  at the base, a pass on rerun, or an unreadable artifact leaves it suspected, reported and never
  confirmed. The final merge must have the selection's exact base and tested tree. Before comparing
  trees, confirmation fetches the recorded tested commit by SHA, including a direct PR run's
  synthetic commit absent from main's history. A fetch failure retains the suspect, names the
  failure in the step summary and fails confirmation visibly; it never silently skips the check.
  Other confirmed misses still retain their evidence and persist the kill switch despite that failure.
  A build failure
  is not a reproduced failing test. For `(outside every test)`, confirmation requires the exact
  package's `start` and terminal package `pass` at base and package `fail` at both head reruns
  (no `Test` member); build-fail events and `[build failed]` or `[setup failed]` output reject
  confirmation. Named tests still require their exact test name and package. Malformed JSON,
  another package, or a missing execution start leaves the package miss suspected. Suspects
  retain the `merged` commit when known.
- `GLS-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) A confirmed
  miss creates the persistent lightweight Git ref `refs/tags/linecov-kill-go` at the failed main
  commit, retains details in `linecov-kill-go` (90 days) naming the test, base, merge and runner,
  and fails the workflow. The ref has no retention expiry and concurrent confirmations preserve
  an existing ref; artifact expiration cannot clear it. Failure to persist the ref fails the
  workflow visibly. While the ref or a retained legacy kill artifact exists, or either lookup
  cannot be read, no CI shard excludes a test by coverage (`LPCV-V0-020`). Only an explicit 404
  admits an absent ref. The owner re-enables it by deleting the ref and any retained kill artifact
  after the cause is repaired and the family requalified (`GLS-V0-034`).

### Measurement (V1-4366)

- `GLS-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4366) The build log
  retains, for 20 consecutive merged pull requests of main, the tests and packages the static plan
  selects and the tests the line selection keeps, with one index at a stated commit, the same build,
  and every FALLBACK reason, plus the `GLS-V0-003` latency and size on the whole-module index.

### Environment completeness and qualification timeouts (V1-4514)

IDs `GLS-V0-051..060` are reserved for this block; `GLS-V0-055..060` are unused.

- `GLS-V0-051`: (proposed 2026-11-11; V1-4514; delivery experimental) Reading the runner identity
  also refuses the settings that the go command, its compiler, cgo or linker read straight from
  the environment but that `go env -changed` does not report (go1.27.3 prints `{}` for
  `FC=x go env -json -changed FC`), so the `GLS-V0-009` refusal of FC never fired. Reading the
  identity fails, naming the variable, when `go env` reports a non-empty FC or GO_EXTLINK_ENABLED
  (from the environment or the `go env -w` file), or when the environment the tests run under sets
  a non-empty GCC, GOCOMPILEDEBUG, GOCLOBBERDEADHASH, GOSSAFUNC or GOSSADIR (the go command reads
  these with `os.Getenv` and folds them into its build action ID; `go env` does not print them).
  AR and GCCGO join the `go env -changed` list of `GLS-V0-009`. Collection and selection read the
  identity through one function, so the setting refuses on either side: `linecov collect` fails,
  and `corvint affected --line-index` and `corvint test --line-index` take the
  `runner-unavailable` FALLBACK, so the static plan stands. An unset or empty setting adds nothing,
  so an identity read without one is unchanged.
- `GLS-V0-052`: (proposed 2026-11-11; V1-4514; delivery experimental) The identity's `runtime=`
  also names, after GOMAXPROCS, GOGC, GOMEMLIMIT and GORACE, each set one of GOTRACEBACK and GOROOT
  (the Go runtime reads them from its own copy of the environment, not through `os.Getenv`:
  GOTRACEBACK when it starts, GOROOT in `runtime.GOROOT()`), then, sorted by name, every set
  variable whose name starts with `LD_` or `DYLD_`, and GLIBC_TUNABLES: the dynamic loader and the
  C library read them outside Go code in a dynamically linked test binary, which every `-race` or
  cgo build is. The `GLS-V0-017` audit cannot see these reads, so an index collected under one
  value is a `runner-mismatch` under another. An unset variable adds nothing, so an identity read
  without any of them is unchanged. A GOROOT that resolves, through symbolic links, to the
  directory `go env GOROOT` reports without it is an alias of the default and adds nothing either;
  a GOROOT naming another directory is bound.
- `GLS-V0-053`: (proposed 2026-11-11; V1-4514; delivery experimental) C code reads the environment
  through the C library's `getenv`, which the `os` audit of `GLS-V0-017` cannot see. Collection
  therefore lists each package's test binary dependencies (`go list -deps -test` under the runner's
  GOFLAGS) and, once, the dependencies of every module package (`go list -deps ./...`, which holds
  every binary a test can build from the module). When a non-standard package among a package's
  test binary dependencies compiles C, C++ or SWIG sources (`CgoFiles`, `SwigFiles`,
  `SwigCXXFiles`), every record of that package binds the whole environment (`*`) as
  `collector-modified`. A record that started an instrumented child binds it too when any
  non-standard dependency of the module compiles such sources. Those tests are always kept, and
  their package runs whole as `environment` (`GLS-V0-018`). A package the listing cannot load
  counts as compiling such sources; a listing that fails outright fails collection. Standard-library cgo is not covered by this rule: the `net` package reads the
  resolver variables (`LOCALDOMAIN`, `RES_OPTIONS`, `HOSTALIASES`) through `os`, so the audit
  binds them, and `os/user` reads none.
- `GLS-V0-054`: (proposed 2026-11-11; V1-4514; delivery experimental) `linecov qualify` runs each
  suite package as `go test -json -timeout=D`, with D the deadline the index names
  (`LPCV-V0-7603`). When the deadline expires the test binary panics, and test2json reports the
  package failed with no result for the test that was running. The harness has no declared failing
  test for that package, so it counts the package as a crash: every test the package declares
  counts as failing, and any of them the selection left out is a miss. A timeout can therefore
  only add misses, never hide one. A package with no result at all leaves the mutant `invalid`.

## Selection rule

The rule is `LPCV-V0-952` applied inside the static plan. The static plan stays the sound
baseline: line coverage only removes tests from units the static plan selected, and only where every
path that reaches the unit is a body or test-function edit whose evidence is fresh. A fallback path
keeps its static reach whole, drift included, so an index a few commits old still narrows the units
the drift did not reach. Unordered packages stay whole even when no test of theirs is reached,
because an order-only block may have no isolated owner.

## Non-goals

- Runner families other than Go keep their static plan (`LPCV-V0-955`); the JavaScript, Ruby and
  browser collectors have their own specs.
- No watch loop (`LPCV-V0-960`) and no symbol-level narrowing of declaration edits (`LPCV-V0-953`).
- No per-test attribution inside the in-order run: a change reaching any block that run executed,
  or an edit of a test function, keeps its package whole. In practice Go line selection therefore
  leaves out or compiles whole packages and rarely slices one.
- CI shards do not refresh records of the tests they run; the scheduled main collection does.
- The selection never adds a test the static plan left out.
- No modelling of host properties no environment variable names (CPU count, time, network,
  files outside the module): collection records the collection host's behaviour. The environment
  a test reads is bound (`GLS-V0-017`, `GLS-V0-018`), so a test that skips a check under one
  environment is never excluded under another.

## Enabled for 1.0

Decided 2026-11-06 by the owner (round g of the lineidx lane): line-level narrowing is enabled for
1.0. The 2026-11-04 cap, which turned every NARROWED selection into SHADOW `line-narrowing-disabled`
because the runner identity did not bind the inherited process environment, is withdrawn, and the
blocker is solved rather than deferred:

1. The inherited environment is bound per record (`GLS-V0-017`) and compared at selection
   (`GLS-V0-018`). A test that returns early when `CI` is set records, under `CI=true`, a binding
   to that value; a local selection with `CI` unset finds the binding does not hold and keeps the
   test, and its package, whatever its records say. Fault injection: the `env` fixture of
   `TestFaultInjection` collects with `LINECOV_FIXTURE_SKIP`, `LINECOV_FIXTURE_ALL` and
   `LINECOV_FIXTURE_TOOL` set, so a guarded body, a whole-environment reader's body and a body only
   a child reaches under another environment are unexecuted; a mutant in each fails nothing and is
   excluded under the collection environment (negative controls), and with the variable unset the
   mutant fails and the bound package is kept. A test's own setting binds nothing.
2. The runtime settings the Go runtime reads itself (GOMAXPROCS, GOGC, GOMEMLIMIT, GORACE) are in
   the identity (`GLS-V0-009`).
3. FC: `go env -json -changed FC` still prints `{}` on go1.27.3, so the FC refusal of `GLS-V0-009`
   does not fire. Proposed repair (V1-4514): the identity refuses a non-empty FC that `go env`
   reports, and the other settings the go command reads without `go env -changed` reporting them
   (`GLS-V0-051`). The Go runtime's and the dynamic loader's own reads are carried in `runtime=`
   (`GLS-V0-052`), and C code's reads bind the whole environment (`GLS-V0-053`).
4. `linecov qualify` runs suites under the deadline the index names (`LPCV-V0-7603`).

What the binding costs: an index narrows only under an environment that holds every variable its
tests read. CI collection and CI shards share the GitHub runner environment but for the
run-identity variables (`GITHUB_RUN_ID`, `GITHUB_SHA`, `GITHUB_EVENT_NAME` and the like), so a
package whose tests read one of them, or the whole environment, runs whole in CI; a locally
collected index narrows local runs on the same host, and a CI index narrows nothing locally
(`HOME`, `PATH` and `TMPDIR` differ). The retained qualification and measurement of the build log
`2026-11-06-lineidx-enable.md` state what that leaves.

## Trust boundary, limits and failure modes

The index is derived state (`LPCV-V0-958`): it is read, never trusted as authority, and every input it
cannot verify widens to the static plan. `corvint affected --line-index` reads the index file, Git
objects and the head test files; it writes nothing.

| Failure | Required behavior |
|---|---|
| Damaged header, wrong magic, unknown format | FALLBACK `index-unreadable` or `index-format` |
| Damaged block section | that path falls back as `index-section-invalid` |
| Runner, module or ancestry mismatch | FALLBACK, static plan |
| Drift reaches a unit through a non-body edit | the unit runs whole |
| A test failed or timed out alone | unrecorded, always kept |
| A test added at HEAD, or only built under a tag | unrecorded, kept |
| A test or in-order record observed under another environment | kept, or the package whole, as `environment` (`GLS-V0-018`) |
| The index binds no environment | FALLBACK `environment-unbound`, static plan |
| FC, GO_EXTLINK_ENABLED, GCC, GOCOMPILEDEBUG, GOCLOBBERDEADHASH, GOSSAFUNC or GOSSADIR set when collecting or selecting, or AR or GCCGO changed | collection fails; selection FALLBACK `runner-unavailable`, static plan (`GLS-V0-051`) |
| GOTRACEBACK, GOROOT, an `LD_*` or `DYLD_*` variable or GLIBC_TUNABLES differs between collection and selection | FALLBACK `runner-mismatch`, static plan (`GLS-V0-052`); a GOROOT that only aliases the go command's default root, through a symbolic link or a trailing separator, keeps the identity; a `GOTOOLCHAIN` switch changes `GOVERSION`, and a GOROOT naming another toolchain is bound, so either is a mismatch |
| A test binary, or a child a test starts, links non-standard C, C++ or SWIG code | its records bind the whole environment: always kept, the package whole as `environment` (`GLS-V0-053`) |
| A qualification suite run times out | the package counts as a crash: every declared test failing; any left out is a miss (`GLS-V0-054`) |
| A test read GOFLAGS, GOCOVERDIR, a `CORVINT_LINECOV_*` variable or the whole environment | binds `collector-modified`: always kept, its package whole when the in-order record holds it (`GLS-V0-017`) |
| A `TestMain`-only package without a complete suite record | the package runs whole as `suite-unrecorded` (`GLS-V0-008`) |
| A suspected miss whose recorded runner is absent or not reproducible, or out of time | stays suspected, never confirmed (`GLS-V0-046`) |
| Mutant kills a test the selector did not keep | miss; qualification fails (`GLS-V0-031`) |
| Main full run fails a test a narrowed run left out | suspected miss; confirmed miss kills exclusion (`GLS-V0-046`, `GLS-V0-047`) |
| Any CI step error | the shard's lists are unchanged (`GLS-V0-043`) |

## Acceptance evidence

Fixture modules in `internal/linecov` tests: a changed line covered by one test selects it alone, an
unexecuted line selects none, a declaration edit takes the static plan, an E2E test reaching the
change only through a started binary is kept, a head-new test is kept, an isolation-divergent test
is kept, a drift body edit marks its tests stale and a drift helper edit keeps its unit whole; a
body reached only under another environment is excluded under the collection environment and kept,
with its package, under the other, for a variable a test reads, the whole environment and a
variable only a child reads; FC set, or a `CI` value a test reads changed, between index collection
and selection keeps the static plan or the reading package whole (`GLS-V0-051`, `GLS-V0-053`); the
`/4` round trip and every damaged-index case; the plan-time budget; the `corvint affected` and
`corvint test` wiring; the qualification harness on a fixture with a seeded miss it must report; the
`linecov ci -selection` and `linecov miss` cases; and the retained Corvint qualification and 20-PR
measurement (build log `2026-11-03-gls-line-selection.md`).

## Rollout and rollback

Local: opt-in by naming `--line-index`; a qualified NARROWED selection narrows the run. CI: the
step narrows while `CORVINT_LINECOV_NARROWING` is `on` and the qualification record matches the
base (`GLS-V0-040`, `GLS-V0-043`); otherwise it plans and records the selection and keeps the
static lists. Rollback: unset the variable or create `refs/tags/linecov-kill-go`; selection returns
to the static plan, and deleting the index loses only performance. An index collected before
`GLS-V0-017` decides nothing (`environment-unbound`) until the next collection.
The `GLS-V0-051..054` repairs change qualified sources (`internal/linecov/runner.go`,
`collect.go`), so the committed record stops admitting and every selection is SHADOW `unqualified`
until the owner regenerates the record with `linecov qualify` (`GLS-V0-034`). They never write or
edit the record and never turn on CI narrowing. Rollback of the repairs: revert their commits.
Selection then returns to the `GLS-V0-009` identity, and the regenerated record would need
regenerating again.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GLS-V0-001, GLS-V0-002 | `internal/linecov/compact.go` | `TestCompactRoundTrip`, `TestCompactDamage`, `TestCompactDamage` |
| GLS-V0-003 | `internal/linecov/compact.go`, `decide.go` | `TestLazyPlanBudget` (p95 of process CPU time, so shard contention cannot fail it; as `MCJ-V0-025`); build log |
| GLS-V0-008 | `internal/linecov/collect.go`, `index.go`, `windows.go`, `audit.go`, `tools/linecov` | `TestMergeResolvesInOrderParts`, `TestFaultInjection`, `TestDecide`, `TestCollectParts`, `TestCompactProfile`, `TestCollectorProcessGroup`, `TestInOrderWindows`, `TestUninstrumentedChild`, `TestOrderedChildMutation`, `TestQualifyRefusesFailingBaseline`, `TestSuiteOnlyPackageKeptWithoutSuiteRecord`, `TestCollectSuiteOnlyPackage`, `TestHeadTestNames` (TestMain-only lists) |
| GLS-V0-009 | `internal/linecov/runner.go`, `collect.go`, `tools/linecov` | `TestCallerOverlayRefused`, `TestRunnerIdentityDropsParallelism`, `TestRunnerIDBindsArchFeatures`, `TestRunnerIDBindsRuntimeSettings`, `TestAffectedLineArchFeatureMismatchKeepsStaticPlan`, `TestRunnerIDBindsTestRuntime`, `TestRunnerIDRefusesUncarriedEnvironment`, `TestFaultInjection` (`body reached only in verbose mode`, `body reached only under a test deadline`), `TestAffectedLineTestRuntimeMismatchKeepsStaticPlan` |
| GLS-V0-010..016 | `internal/linecov/decide.go`, `heads.go`, `cmd/corvint/affected_linecov.go`, `internal/liveverify/affected/readers.go` (`Readers`) | `TestDecide`, `TestHeadTestNames`, `TestAffectedLineIndexNarrowsTheBasePlan`, `TestAffectedLineIndexProfileRefusals`, `TestAffectedLineTestRuntimeMismatchKeepsStaticPlan`, `TestAffectedLineSourceReaderKeptWhole`, `TestAffectedLineInsertionIntroducingInitKeepsWhole`, `TestAnalyzeRequiresBothSides`, `TestLineNarrowingAppliesWhenQualified` |
| GLS-V0-017, GLS-V0-018 | `internal/linecov/envaudit.go`, `audit.go`, `collect.go`, `compact.go`, `index.go`, `decide.go`, `select.go`, `windows.go` | `TestEnvAuditParse`, `TestEnvUnbound`, `TestUnboundIndexDecidesNothing`, `TestEnvAuditVersionAdmission`, `TestSyscallEnvironmentMutation`, `TestFaultInjection` (`environment:` and `collector-set variable:` cases, collector-own read checks), `TestCompactRoundTrip`, `TestAffectedLineFCOrCIChangeKeepsStaticPlan` (a changed `CI` a test reads) |
| GLS-V0-020 | `cmd/corvint/test.go`, `affected_linecov.go`, `internal/sessiontest/plan.go` | `TestLineNarrowingAppliesWhenQualified`, `TestCorvintTestLineIndexNarrowsTheRun`, `TestCorvintTestUnqualifiedLineIndexIsShadow`, `TestRunnerEnvCarriesBuildFlags`, `TestRunnerEnvKeepsSavedGoFlags`, `TestRunnerEnvSavedTestFlagOrFailedGoEnvFallsBack`, `TestRunnerEnvDeclaredVerbosityKeepsInverseVerboseTest`, `TestRunnerEnvRefusesUncarriedDeclaredEnv`, `TestCorvintTestUncarriedGoFlagKeepsStaticPlan`, `TestCorvintTestDeclaredVerbosityKeepsInverseVerboseTest`, `TestLocalQualificationRequiresMatchingEvidence` |
| GLS-V0-030..034 | `tools/linecov/qualify.go` | `TestQualifyFixtureHasNoMiss`, `TestQualifyReportsASeededMiss`, `TestSummarizeAndQualified`, `TestWorkflowQualifiedInvocationMatchesWrapperRecord`; `.github/linecov/qualification.json`; build log |
| GLS-V0-035 | `tools/linecov/qualify.go` (unchanged) over the fixture in `tools/linecov/qualify_crosspkg_test.go` | `TestQualifyCrossPackageFixtureHasNoMiss` (every site of lib, each kind valid, zero misses, a lib mutant killed only by `app.TestReport`, the control keeps none), `TestQualifyReportsACrossPackageSeededMiss` (the seeded index fails qualify naming `app.TestReport`); build log `2026-11-11-selection-baseline-profile.md` |
| GLS-V0-040..047 | `.github/workflows/ci.yml`, `linecov-index.yml`, `linecov-miss.yml`, `tools/linecov` | `TestCISelectionRewritesWholePackages`, `TestCINarrowsWholePackages`, `TestMissFindsExcludedFailures`, `TestLinecovPackageMissConfirmation`, `TestLinecovMissConfirmationReproducesRecordedRunner`, `TestLinecovKillSurvivesArtifactExpiry`, `TestLinecovKillMarkerWriter`, `TestAFPV0170RecordNeedsEveryShardFullUnderAnyVerdict`; workflow YAML and shell syntax checks |
| GLS-V0-050 | build log | `2026-11-03-gls-line-selection.md`, `2026-11-06-lineidx-enable.md` |
| GLS-V0-051, GLS-V0-052 | `internal/linecov/runner.go` | `TestRunnerIDRefusesUnreportedToolchainEnvironment`, `TestRunnerIDBindsRuntimeReadEnvironment`, `TestAffectedLineFCOrCIChangeKeepsStaticPlan` |
| GLS-V0-053 | `internal/linecov/collect.go` | `TestCollectBindsWholeEnvironmentForCCode` |
| GLS-V0-054 | `tools/linecov/qualify.go` (unchanged) | `TestQualifySuiteTimeoutCountsEveryTestFailing` |

## Owner questions

- Whether to turn on `CORVINT_LINECOV_NARROWING` after the qualification and the 20-PR measurement.
- Whether the CI index workflow and the shards should run their tests under one canonical
  environment (an explicit allowlist of variables), so the run-identity variables stop keeping
  whole-environment readers whole in CI (`GLS-V0-018`).
