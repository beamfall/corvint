# Go line selection: review round c fixes

Tickets: V1-4364, V1-4366. Lane base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Observed
2026-11-04 on the WSL lane host (go1.27.3 linux/amd64). Delivery remains experimental; this
entry records no promotion or qualification. Each fix below fails closed rather than model a
case it cannot prove.

## Decisions

- In-order records are always kept (`GLS-V0-008`, `LPCV-V0-962`). Build used to drop a
  package's in-order record when an isolated record executed each of its blocks, and merge
  pruned it the same way. That loses attribution: in the `flag` fixture, TestA sets `On` and
  calls `F`; TestB calls `F` only when `On` is set. Alone, TestB never reaches `F`, and TestA's
  isolated record covers every block the in-order run executes, so the in-order record was
  dropped and a change to `F` kept only TestA. TestB fails under that change. The record is
  now kept, a test-function edit also reaches it (the state a test leaves can change what a
  later test reaches), and a package with records but no in-order record (an older pruned or
  hand-built index) is unordered.
- The runner identity names the architecture feature settings (`GLS-V0-009`): GO386, GOAMD64,
  GOARM, GOARM64, GOFIPS140, GOMIPS, GOMIPS64, GOPPC64, GORISCV64 and GOWASM, as
  `arch="NAME=value ..."` for each non-empty one. On this host the default identity carries
  `arch="GOAMD64=v1 GOFIPS140=off"`, so every index collected before this change is a
  `runner-mismatch` once, and the static plan runs until main recollects.
- `RunnerEnv` (`GLS-V0-020`) now carries every declared build flag in GOFLAGS: the sanitizers,
  `-trimpath`, `-linkshared`, `-buildvcs`, `-tags`, `-ldflags`, `-gcflags`, `-asmflags`,
  `-mod`, `-pgo`, `-installsuffix`, `-compiler`, `-gccgoflags`, `-overlay`, `-buildmode`,
  `-modfile` and `-toolexec`. It drops only `-p`, `-timeout`, `-v`, `-json`, `-failfast` and
  `-count=1`, and refuses any other flag, or a value GOFLAGS cannot carry, as
  `runner-unavailable`. A refused selection starts no `go env` read.
- The `ci-tested-tree-record` job (`AFP-V0-170`) needs all 16 `full<index>` outputs FULL under
  any narrowing verdict. The line-level step narrows independently of the AFP-V0-098 verdict, so
  the old `verdict != 'MET' ||` disjunct let a line-narrowed shard under a NOT_MET verdict
  record a full tested tree that a merge-queue REUSE would then accept. `ci-narrowed-tree-record`
  is unchanged: under a non-MET verdict a line-narrowed run records neither tree, and the queue
  runs FULL.

## Fail-before evidence

Each regression was run against the pre-fix code and failed:

- `TestFaultInjection` cases "block an isolated record shares, reached by a later test only in
  order" and "test function leaving state a later test reads": `MISS: flag.TestB`.
- `TestRunnerIDBindsArchFeatures`: GOAMD64=v1 and v2 gave identical identities.
- `TestAffectedLineArchFeatureMismatchKeepsStaticPlan`: an index collected under GOAMD64=v1
  read under v2 gave `SHADOW unqualified` (narrowing proposed) instead of
  `FALLBACK runner-mismatch`.
- `TestRunnerEnvCarriesBuildFlags`: `-ldflags=-X=example.com/m/p.mode=on` was dropped from
  GOFLAGS, so the runner identity equalled the default one; `-shuffle=on`, `-short`, `-cover`,
  `-count=3`, `-exec`, and values with spaces or quotes were accepted silently.
- `TestCorvintTestUncarriedGoFlagKeepsStaticPlan`, with the refusal not wired: `SHADOW` instead
  of `FALLBACK`.
- `TestAFPV0170RecordNeedsEveryShardFullUnderAnyVerdict`, which evaluates the job's `if:` with a
  small Actions-expression evaluator (`TestActionsExprEvaluator`): with verdict `NOT_MET`,
  `UNKNOWN` or empty and any one shard's output empty, the old predicate recorded a tree. With a
  `MET` verdict (negative control) it recorded none, and with every shard FULL every verdict
  records one.

## Product consequence and retained limit

The in-order run attributes no block to a test. A collected in-order run executes every block
that any isolated test executes, so a change reaching any tested block, and any test-function
edit, now keeps the whole package. In practice Go line selection leaves out or compiles whole
packages and rarely slices one with `-run`. To restore intra-package slicing soundly, the
collector would need per-test attribution inside the in-order run, for example by dumping and
resetting counters (`runtime/coverage.WriteCounters` and `ClearCounters`) at each top-level
test's end. That is not built here, and the non-goal in `GLS-V0` says so.

Focused tests run: `./internal/linecov/`, `./internal/sessiontest/`, `./tools/linecov/`,
`./tools/ci-reuse-plan/`, `./internal/browsercov/`, `./internal/jslinecov/`,
`./internal/linecov/v8/`, `./internal/rubylinecov/`, and the `TestAffectedLine*`,
`TestCorvintTestLineIndexNarrowsTheRun`, `TestCorvintTestUnqualifiedLineIndexIsShadow` and
`TestCorvintTestUncarriedGoFlagKeepsStaticPlan` tests of `./cmd/corvint/`. The full
`cmd/corvint` package and hosted CI behaviour of the changed record job were `NOT_RUN`.
