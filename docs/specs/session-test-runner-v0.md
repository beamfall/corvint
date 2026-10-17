# Session Test Runner V0

Owner: Russell Lewis
Date: 2026-10-11
Requirement prefix: `STR-V0`
Intent status: accepted (decision 0524; V1-0040); STR-V0-050..054 accepted under the owner's standing auto-accept (2026-10-11; V1-1579; GitHub #1022)
Delivery status: experimental (slice 1: Go adapter, dry-run, receipts, universe placement, verify-shards; slice 2: driver parity report, local balanced placement; redesign: project-owned test-commands declaration and gate; slice 3: `make gate-affected` on `corvint test`, CI shadow dry-run beside the driver; V1-1257: the promotion measurements are met, `docs/build-log/2026-10-16-v1-1257-str-v0-promotion-evidence.md`, but STR-V0-004's versioned capability manifest and STR-V0-011 have no executable evidence yet)
Authoritative inputs: V1-0040 owner request (2026-09-19 recommendation, widened 2026-10-11);
`docs/specs/affected-plan-v0.md` (AFP-V0-011..013, 022, 023, 028..030, 041);
`docs/specs/live-proof-carrying-verification-v0.md` (LPCV-V0-021..026);
`docs/specs/go-live-test-provider-v0.md`; AGENTS.md invariants 2, 3, 4, 7 and 8.

## Agent digest
- Claim: `corvint test` plans from the diff, runs only selected tests through runner adapters, shards deterministically, and falls back alike locally and in CI.
- Status: accepted (decision 0524; V1-0040); STR-V0-050..054 accepted under the owner's standing auto-accept (2026-10-11; V1-1579; GitHub #1022)/experimental (slice 1: Go adapter, dry-run, receipts, universe placement, verify-shards; slice 2: driver parity report, local balanced placement; redesign: project-owned test-commands declaration and gate; slice 3: `make gate-affected` on `corvint test`, CI shadow dry-run beside the driver; V1-1257: the promotion measurements are met, `docs/build-log/2026-10-16-v1-1257-str-v0-promotion-evidence.md`, but STR-V0-004's versioned capability manifest and STR-V0-011 have no executable evidence yet)
- Exists: `corvint test` (`cmd/corvint/test.go`, `internal/sessiontest`), behind `make gate-affected` through `script/gate-affected-corvint.sh` (slice 3; `script/gate-affected.sh` stays as the rollback); in CI a shadow `--profile ci --dry-run` step beside the deciding driver `tools/corvint-pr-tests` with `tools/gate-affected-select`, built from its own pin `CORVINT_TEST_SHADOW_SOURCE` (AFP-V0-047) while the driver pins stay empty, so it plans and retains each PR shard's selection without deciding; `.github/cishards` (complete-universe partition and test slices), `cmd/corvint-go-test-provider` (experimental Go execute provider); `corvint test shards` and `--placement measured` (STR-V0-050..054, experimental: a duration-balanced shard plan from retained full-run cost tables, executed locally only).
- Blocked on: `implemented` waits for executable evidence of STR-V0-004's versioned LPCV-V0-021 capability manifest and of STR-V0-011 (no other `corvint test` reader), or an owner amendment narrowing them; CI narrowing waits for the AFP-V0-029 shadow bar (met at 2026-10-14T17:59:52Z, NOT MET again at 1/5 failing rows on 2026-10-16) and V1-0812 before `corvint test` decides or narrows anything in CI; balanced placement stays unpromoted until the local 20% bar is measured or an accepted AFP-V0-028 amendment defines the CI comparison; the AFP-V0-011 list stays `NOT_OBSERVED`; the full-suite commands are declared in `.github/cishards/test-commands.json`, kept equal to `make go-test` by `make test-commands-check`.
- Read next: Requirements; Boundaries and failure modes; Rollout and rollback.

## User and verified baseline

Affected user: an engineer or agent who wants the tests a change needs, run once, quickly, with
the same answer on their machine and in CI. Measurable job: for one session (one base, one
revision, one worktree state) produce and execute a deterministic run set, record exactly what ran,
and never present an omission as a pass.

Verified state at `645304a2fb71065f9061d8d8d866600d703e84c6`:

- `corvint affected --base` emits `affected-plan/1` with selected units across eight language
  plugins, an unknown frontier, and runnable argv only for Go (`provider.go.packages`); the
  Playwright profile emits file commands separately. It executes nothing (AFP-V0-001).
- Two drivers plan the same selection through different code: `make gate-affected`
  (`script/gate-affected.sh`, AFP-V0-011/012) locally, and `tools/corvint-pr-tests` with
  `tools/gate-affected-select` (AFP-V0-013) in CI. Both run Go only.
- `.github/workflows/ci.yml` runs eight `go-product-shard` jobs. Its trusted tool pins are empty,
  so the race-test invocation never narrows today; it is skipped only in the `DOCS` and `REUSE`
  modes, and a `ci:batched` constituent (AFP-V0-027) skips the shard jobs. When pinned, each
  shard would intersect its selection with the AFP-V0-022 complete-universe partition, leaving
  shards idle when the selection clusters, so full and narrowed shards cannot omit a package. `tools/corvint-pr-tests` places whole packages
  and refuses sharded execution while AFP-V0-041 slices are active, as they are for
  `internal/tasks/store`.
- `.github/cishards` already places packages by advisory cost and splits allowed packages into
  test-name slices (AFP-V0-041); placement never changes membership.
- LPCV (decision 0047) specifies runtime providers with discovery, plan, execute and cancel, but is
  `not-started`; the Go live-test provider is experimental and its containment is unqualified.

Simpler baseline: keep the two drivers and run the full suite. That baseline remains the fallback
of every requirement below and the binding gate until this spec is promoted.

## Requirements

- `STR-V0-001`: `corvint test --base FULL_COMMIT_ID [--shard I/N] [--placement universe|balanced]
  [--dry-run] [--receipt PATH]` SHALL plan in process through the same code and profile as
  `corvint affected --base` and record that plan's graph and plan digests and the STR-V0-017
  source identity. It is an executing
  command, never classified as a read command: it MUST NOT write tracked files, the index, trace
  stores or the `.corvint/` ledgers, and writes only its receipt and runner-owned caches.
  `--dry-run` prints the run set, argv and verdict and executes nothing.
- `STR-V0-002`: The run set SHALL be, per language, the union of the plan's selected units and the
  AFP-V0-012 attribution for every dirty path, expanded into execution units. An execution unit is
  a canonical (unit, slice) pair: slice `whole`, or one AFP-V0-041 slice, the catch-all included,
  taken from the slice table in the run's inputs. A split unit is complete only when every one of
  its expected slices is complete. Advice strings MUST NOT execute. Repository-declared
  mandatory checks (`advice.checks`) are reported as still required and never counted as run.
- `STR-V0-003`: The command SHALL run the complete relevant suite (`FULL`), naming the reason as
  `corvint-test: FALLBACK <reason>`, under every AFP-V0-011 fallback condition, and also when a
  module-level frontier or any `plan.unknown` entry outside the AFP-V0-028 reviewed allowlist is
  present. `FULL` covers every language with selected or unknown units, never only the languages
  that narrowed. The repository declares its full-suite commands in one project-owned file,
  `.github/cishards/test-commands.json` (profile `corvint-test-commands/0`): the Go full-suite
  environment and flags, and for each other language either its full command or `{"none":
  reason}`. `corvint test` reads that file as a plain, bounded, strictly-schemed read, independently
  of the planner, and SHALL NOT interpret the Makefile. A declared command runs from the
  repository root; an `argv[0]` containing a path separator is a repository-relative path resolved
  against the root, never against the caller's working directory, and a bare name is looked up on
  `PATH`. When no plan can be produced, `FULL` SHALL run every full command it declares. When the declaration cannot be read (absent, oversized,
  unknown or malformed member, missing `go`), the run is `INCOMPLETE` in every scope and exits
  nonzero. The verdict is a pure function of the plan, the declared full scope and the source
  identity, so every host and shard reaches it from the same inputs.
- `STR-V0-004`: Execution SHALL go through registered runner adapters, one per runner, each with a
  versioned capability manifest (the discover, plan, execute and cancel subset of LPCV-V0-021) and
  the exact runner version captured before execution. The first adapter is Go. A selected or
  unknown unit in a language with no qualified adapter SHALL run that language's declared full
  command. `{"none": reason}` is a gate-scoped assertion: the repository's full gate (`make gate`),
  its existing drivers (`script/gate-affected.sh`, `tools/corvint-pr-tests`) and CI run no suite
  for that language. It SHALL NOT be declared for a language whose suite any gate step, driver or
  CI job does run; such a language declares that step's command (the `integrations/pi`
  `typescript` suites run under `make gate`'s `host-adapter-test`, so `typescript` declares
  `["make", "host-adapter-test"]`, which also runs that step's `cmd/corvint` Go subset). A suite
  outside the gate (the `extensions/vscode` npm test, the `integrations/pi-protected` and
  `tools/web-flows` `node --test` suites) is a recorded limit this runner does not run, not a
  contradiction of `none`. For a `none` language the manifest records `<language>: <reason>` in
  `noSuite`, nothing runs for it, and the run is not `INCOMPLETE` on its account; the STR-V0-006
  gate refuses a `none` language whose runner a `make gate` step recipe names. When the
  declaration names the language neither way, the run is `INCOMPLETE` and exits nonzero. A
  selected unit is never silently skipped.
- `STR-V0-005`: An adapter SHALL receive only its selected targets and invoke the runner's native
  target filter (for example `go test <packages>`, `cargo test -p <crate>`, `swift test --filter`,
  `dotnet test <project>`, `gradle :<module>:test`, or Playwright files reconciled by discovery as
  `typescript-javascript-affected-adapter-v0.md` requires), so the runner compiles only those
  targets' dependency closure. Corvint SHALL NOT run a separate whole-repository build first, and
  SHALL NOT claim build or isolation guarantees stronger than the runner supplies (LPCV-V0-025).
- `STR-V0-006`: Adapter argv SHALL be typed with no shell. Environment, AFP-V0-023 `test-confine`
  wrapper and cache policy SHALL equal the repository's full-suite invocation; only the target set
  may differ between narrowed and `FULL` runs (as AFP-V0-028 requires in CI). Locally the Go
  environment and flags come from the declaration (STR-V0-003), and the Makefile SHALL delegate
  to it rather than restate it: `make go-test` is the one fixed recipe line
  `go run ./tools/test-commands-run`, and `tools/test-commands-run` reads the declaration through
  the STR-V0-003 reader (an unreadable or absent declaration, an argument, or a directory without
  `go.mod` is a refusal, exit 2, before `go` starts), runs `go test <go.flags> ./...` from the
  repository root with each `go.env` entry replacing any inherited value of the same name, passes
  stdout and stderr through and returns `go`'s exit status. `make gate` runs Go tests through
  `ledger/go-test` (GL-V0-004) rather than `go-test`, so that rule's command is pinned to the
  declaration by rule (f). A maintained repository gate (`make test-commands-check`,
  a `make gate` step and part of the CI doc-gates job) keeps the Makefile to that delegation.
  Rules (a), (b), (c) and (f) read source text only; (d) and (e) dry-run make (`make -s -n` on a
  minimal `PATH`), which expands the Makefile, including any parse-time `$(shell ...)`, so those
  two are not text-only: (a) exactly one rule line reads
  `go-test:` (no prerequisites, no `;` recipe, no `::`), followed by exactly one recipe line
  `go run ./tools/test-commands-run` (blank, comment and conditional-directive lines between do
  not end the recipe, so a tab line inside an `ifeq` branch after it counts as a second recipe
  line; a backslash continuation joins into the compared text), and any other rule line names `go-test`
  only as a prerequisite; a rule whose target could match `go-test` (a `%` pattern it fits, or a
  target word make would expand) MAY list prerequisites (GOC-V0-010's order-only
  `gate-receipt-clear` line) but SHALL carry no recipe, tab-indented or after `;`; (b) no line of
  the Makefile or of a file it `include`s assigns `SHELL`, `.SHELLFLAGS`, `MAKEFLAGS` or
  `.RECIPEPREFIX`, globally, through `export`/`override`/`private`, or for any target or pattern;
  exports `MAKEFLAGS`; exports every variable (`export` alone, `.EXPORT_ALL_VARIABLES`); names
  `.IGNORE` as a target (which would let a failing recipe succeed); or
  defines a target- or pattern-specific variable whose target could match `go-test`; (c) the text
  is readable without make: `$(eval ...)`/`${eval ...}` anywhere, a `define` block, an `include`
  name holding `$` or a glob character and a missing non-optional include are each refused, and
  includes are followed at most four deep; (d) `GATE_STEPS` is exactly one `GATE_STEPS = ...` line
  with a `$`-free value, and for each language declared `none` the gate dry-runs those steps
  (`make -s -n`, together first, per step only to name a violation) and fails when a recipe
  word's basename names that language's runner (a closed per-language list: for example `node`,
  `npm`, `bun`, `pytest`, `python3`, `cargo`, `swift`, `dotnet`, `gradle`), inspecting only the
  words make prints, not what a script those words name runs, and refusing a `none` language it
  has no runner list for; (e) every declared `make` language command names a target that
  dry-runs; (f) when a `ledger/go-test:` rule exists, it is exactly that line with the one recipe
  `@$(GATE_LEDGER) go-test -- go test $(GO_TEST_FLAGS)`; `GATE_LEDGER`, `GO_TEST_FLAGS`,
  `GO_TEST_TIMEOUT` and `CORVINT_GOCACHE` each have exactly one global `=`, `:=`, `::=` or `?=`
  assignment and none for a target; no variable is set for `ledger/go-test`, `ledger/%` or an
  expanded target word; and, expanding only those four, the `NAME=VALUE` words before `go` in
  `GATE_LEDGER` equal the declared `go.env` and `GO_TEST_FLAGS` equals the declared `go.flags`
  without `-count=1`, which gate-ledger adds itself only for the packages it cannot key, running
  the others through Go's test cache (GL-V0). Without that rule `make gate` reaches `go-test`
  through the `ledger/%` sub-make. A line inside a conditional counts as live, so the check is
  stricter than make, never looser. Recorded limits: the environment make itself is started with
  (a caller's `GOFLAGS` or `GO_TEST_TIMEOUT`, or a Makefile `export GOFLAGS`) reaches `go` and is
  not refused; the `none` check sees only the direct recipe words of the
  `GATE_STEPS` steps, so a suite started inside a script, by a driver or by a CI job is the
  owner's to review, not the gate's. The gate was first a make-observing one (a recording `go`
  shim, a control recipe and a `make -p` parse; decision 0526, review rounds 4 to 8); the owner
  replaced it on 2026-10-12 because each round found another make semantic it had to
  reverse-engineer, and acceptance now follows one independent review round of this text check
  with findings fixed or recorded here, not a `VERDICT: PASS`.
  The reader (STR-V0-003) refuses any environment name that is not a portable identifier
  (`[A-Za-z_][A-Za-z0-9_]*`), any word with whitespace or a control character, because
  `corvint test` passes declared words to exec literally, and a declared `go.env` `PATH`, because
  the `go` executable is found on the caller's `PATH` and a declared one would not select it. The manifest binds the declaration's
  digest as `declaredDigest`.
- `STR-V0-007`: `--shard I/N` with `--placement universe` (the default) SHALL use the AFP-V0-022
  complete-universe partition intersected with the run set, unchanged. An empty intersection runs
  nothing and says so. Every shard records the partition digest; a digest mismatch fails the shard.
- `STR-V0-008`: `--placement balanced` (experimental, off by default) SHALL partition the run
  set's execution units themselves by the advisory cost table, so a narrowed run uses every shard.
  Every execution unit SHALL be placed exactly once. Because membership then depends on the plan,
  every shard of one run SHALL reproduce the same STR-V0-018 manifest digest; a shard that cannot
  fails rather than falling back alone. `FULL` runs use the complete-universe partition in either
  mode. Balanced placement MAY execute only in the local profile. In CI it SHALL only be computed
  and retained beside the executing universe placement, because AFP-V0-028 requires the
  complete-universe intersection; executing it in CI needs an accepted AFP-V0-028 amendment.
- `STR-V0-009`: `corvint test verify-shards --manifest PATH RECEIPT...` SHALL read the expected run
  from the STR-V0-018 manifest, never from the receipts. It reports `PASS` only when there is
  exactly one receipt for each shard index `0..N-1` of the manifest's shard count, empty shards
  included; every receipt carries the manifest digest and source identity and records result
  `PASS`; the union of executed
  execution units equals the manifest's with none twice; every execution unit has exactly one
  passing terminal outcome; and every adapter invocation exited zero. A missing, extra, duplicate
  or foreign receipt, a receipt whose result is `INCOMPLETE` (drift, cancellation or an exceeded
  bound), or an execution unit without an outcome, is `INCOMPLETE`; any failed, timed
  out or build-failed outcome, or any nonzero exit, is `FAIL`. Both exit nonzero, `INCOMPLETE`
  taking precedence in the report. A CI required check SHALL depend on this fan-in, not on
  per-shard exits alone, and an unsharded run applies the same rules to its single receipt.
- `STR-V0-010`: Each shard, including an empty one, SHALL write one bounded `corvint-test-run/0`
  receipt (at most 1 MiB) with tool version, base, revision, source identity, plan and manifest
  digests, scope (`NARROWED`, `FULL` with reason, `NOTHING`), result (`PASS`, `FAIL`,
  `INCOMPLETE`), placement mode, partition digest, shard index and count, and per adapter the
  runner identity and version, exact argv, environment digest, exit, elapsed time and per
  execution unit terminal outcome. A narrowed run SHALL summarise as "selected units passed: N of M"
  and name where the full suite runs, never "suite passed".
- `STR-V0-011`: Receipts are evidence only. They MUST NOT feed ranking, selection, learning or
  authority; a later CEM citation needs its own accepted requirement.
- `STR-V0-012`: Local and CI SHALL run the same binary code path. The CI profile adds only
  admission: planner and runner built outside the PR checkout from the reviewed pinned source
  (AFP-V0-013/028), the merge-queue backstop and the forward-qualification trip and kill switch
  (AFP-V0-029/030). Every AFP-V0-028 FULL condition still applies in CI, and CI executes only
  universe placement (STR-V0-008). A local narrowed PASS
  SHALL NOT be reported as a gate PASS; `make gate` and the required CI checks stay binding.
  (Slice 3.) `make gate-affected` SHALL run `corvint test --base BASE` in the local profile through
  `script/gate-affected-corvint.sh`, which builds the binary from the checkout under test into a
  private temporary directory, never from `PATH`, and passes its exit status through; `BASE=`
  is HEAD (the dirty worktree alone), and a BASE that does not resolve to a commit is a usage
  error (exit 2), not the previous full fallback. The make-introduced `MAKEFLAGS`, `MAKELEVEL` and
  `MFLAGS` are inherited unchanged, as `make go-test`'s own `go test` inherits them (no make
  jobserver is passed through). The ci profile's Go environment and flags SHALL equal the full
  branch of the `go-product-shard` job, the environment its `go test` sees (the job `env` over the
  runner's own) and its two `go test -exec` invocations, not the driver's closed environment; they
  are constants held equal to `.github/workflows/ci.yml` by a maintained test, not read from the
  declaration's `go` section, which declares the local full suite. The ci profile SHALL NOT launch
  a declared language command, a make target included: the CI full branch runs only Go, so a
  declared command is reported in `checks` as still required and not run. The partition inputs
  are the tested checkout's `.github/cishards` JSON files, the bytes the full branch's protected
  helper copies, placed by the algorithm compiled into the pinned binary; a changed `.github/`
  path and partition drift stay AFP-V0-028 FULL conditions that CI admission applies.
- `STR-V0-013`: Neither `make gate-affected` nor `tools/corvint-pr-tests` SHALL switch to
  `corvint test` until a parity check over the retained substitute corpus,
  `tools/str-parity/testdata/pairs-645304a2.txt` (the 50 first-parent pairs ending at
  `645304a2fb71065f9061d8d8d866600d703e84c6`, the ones slice 2 measured), shows the same Go
  package set, argv, environment and fallback verdict as the driver it replaces, with every
  difference explained in a build-log entry. The original AFP-V0-011 pair list and the AFP-V0-029
  shadow rows were not retained and stay `NOT_OBSERVED`; parity is measured over the committed
  corpus, never over a list reconstructed from memory. Switching the CI call site is a `.github/`
  change under AFP-V0-016 consent, SHALL advance `CORVINT_PR_FORWARD_EPOCH`, because forward
  qualification binds the driver's identity, and SHALL first meet the AFP-V0-029 shadow bar
  before any CI narrowing. (Slice 3.) With parity met, `make gate-affected` switched
  (STR-V0-012). The CI call site gains only a shadow step: the pinned `corvint test --profile ci
  --placement universe --dry-run` for each shard, retained as an artifact, while
  `tools/corvint-pr-tests` keeps deciding and executing and CI narrowing is unchanged.
  `CORVINT_PR_FORWARD_EPOCH` is introduced at `1` in the `go-product-shard` job environment;
  the advance was vacuous, since no forward shadow row existed in any epoch. AFP-V0-047 builds
  the shadow planner from its own pin `CORVINT_TEST_SHADOW_SOURCE`, so the shadow runs on every
  PR shard while the driver pins stay empty, and its retained selections are the first rows of
  epoch `1`; the shadow still never decides, executes or narrows.
- `STR-V0-014`: A hangup, interrupt or termination SHALL kill every owned process group, exit
  nonzero and write an `INCOMPLETE` receipt; it MUST NOT start the fallback.
- `STR-V0-015`: The run SHALL be bounded: the plan's existing walk and status bounds, at most 4096
  execution units, the receipt bound, the runner's native per-unit timeout unchanged from the
  profile's current call site (for Go, a per-package hang detector: the declared `-timeout` flag
  locally, which `make go-test` runs unchanged through `tools/test-commands-run` (STR-V0-006), and the `50m`
  that `tools/corvint-pr-tests` and the full CI branch pass in CI), and a
  separate total wall-clock budget per shard that covers runner discovery and every adapter
  invocation of the shard (slice 3): 6 hours locally, an outer hang bound above the per-package detector rather than a
  speed target (at the declared `-p 1` a change reaching about 236 packages outran 70 minutes, and
  `script/gate-affected.sh` had no total budget; a full local run's duration is `NOT_OBSERVED`), and
  60 minutes in the ci profile, which leaves the `go-product-shard` job's 75-minute timeout at
  least 10 minutes for checkout, toolchain and tool builds, so the runner, not the job timeout,
  kills a slow shard. Once the budget is spent nothing further launches. Exceeding a bound yields
  `INCOMPLETE` and nonzero, never a narrower run.
- `STR-V0-016`: Adapters execute repository code with the invoking user's authority. The local
  profile claims no containment or network denial. The CI profile keeps read-only workflow
  permissions, no persisted credentials or secrets, and no `pull_request_target` execution.
- `STR-V0-017`: The source identity SHALL bind the bytes that execute: the HEAD commit and tree,
  plus, for every dirty path, its SHA-256 or a deletion marker, captured before planning. The run
  SHALL recompute it before each adapter launch and after the last one; any difference is drift,
  and the shard is `INCOMPLETE`. In CI the AFP-V0-013 captured merge commit with a clean tree is
  the source identity.
- `STR-V0-018`: Before execution the command SHALL derive one canonical `corvint-test-manifest/0`
  from the plan, declared full scope, slice table, cost table, source identity, placement mode and
  shard count, holding the expanded execution units and their shard assignment, at most 1 MiB. Its
  digest is recorded in every receipt. `--dry-run` writes it; in CI one job SHALL compute it and
  pass it to every shard and to the fan-in, and each shard SHALL recompute and match it.
- `STR-V0-019`: Every `go test` command that a `make gate` step recipe runs SHALL carry a
  `-timeout` flag (`$(GO_TEST_TIMEOUT)`, decision 0082), because Go's 600s default panics
  `cmd/corvint` under load. `make test-commands-check` dry-runs the `GATE_STEPS` steps and
  fails naming each `go test` command without it.

Duration-balanced shard plans (V1-1579, GitHub #1022; experimental). A directory or fixed-count
partition leaves shards finishing far apart, and the slowest shard ends every full run. The plan
weighs each listed file by its measured history instead, and feeds the existing sharded run and
STR-V0-009 fan-in through a third placement.

- `STR-V0-050`: `corvint test shards --count N --files PATH [--durations PATH]... [--runs K]` SHALL
  print one canonical `corvint-test-shards/0` plan on stdout: the shard count, K, the revisions of
  the runs used, the per-file weights, the flagged unknown files, the fallback reason (empty when
  history decided), each shard's index, files in lexical order and summed weight, and its
  partition digest. `--files` lists the universe, whitespace-separated (for the Go adapter, one
  package import path per entry): 1 to 4096 entries, no entry twice, valid UTF-8 with no control
  character, at most 1 MiB. N is 1 to 64 and K 1 to 64 (default 5); more than 64 `--durations`
  is the STR-V0-053 fallback. Every input is read through its opened descriptor, only when it
  is a regular file (a FIFO or other non-regular file is refused without blocking), and at
  most 1 MiB of it. The encoded plan is bounded by what those inputs admit: 6 MiB (each name
  escapes to at most twice its bytes and appears at most three times) plus 16 bytes per
  appearance of each of the 4096 entries plus 64 KiB for the fixed members, 64 shard headers,
  64 revisions and the fallback reason, so a plan from admitted inputs always encodes. It is a
  read command (invariant 4): it reads only its named inputs and writes only stdout and one
  stderr line, never the repository, the index, trace stores or the `.corvint/` ledgers. A usage
  error (bad count, K or list, unreadable list) exits 2 and prints no plan; a plan exits 0.
- `STR-V0-051`: Each `--durations` file is one retained full run in the `corvint-ci-package-costs/0`
  shape `tools/ci-shard-costs refresh` writes, given oldest first. A table is admitted only when it
  decodes strictly (no unknown member, nothing trailing) with that profile, a 40- or 64-hex source
  revision, and 1 to 4096 durations, each positive and at most one day, keyed by an entry without
  whitespace. The last K admitted tables are used. A listed file weighs the median of its
  durations in those runs that record it (the mean of the two middle values, rounded down, for an
  even count). A listed file none of them records SHALL weigh the median of the known listed
  files' weights and be listed in `unknown`, so every file has a positive estimate.
- `STR-V0-052`: The plan SHALL take the files heaviest first (ties by name) and put each on the
  lightest shard so far (ties by lowest index), so every listed file appears in exactly one shard.
  The partition digest is `sha256:` over the canonical JSON of the profile, the count and each
  shard's file list in index order. The same files, count, K and history bytes SHALL give the
  same plan bytes and digest.
- `STR-V0-053`: When no table is supplied, more than 64 are, any supplied table is unreadable or
  not admitted, or no listed file has a duration in the used runs, the plan SHALL be the complete
  round-robin (file `i` of the sorted list on shard `i mod N`) with no weights, history or unknown
  files, `fallback` naming the reason, and stderr `corvint-test: shards FALLBACK <reason>`. One bad
  table discards the whole history rather than weighing from the rest. Over 64 tables none is
  read. A table's durations are checked in sorted key order, so the reason is the same on every
  run, and the reason is cut to at most 1024 bytes of valid UTF-8.
- `STR-V0-054`: `corvint test --placement measured --shard-plan PATH` (experimental, off by
  default) SHALL place every Go package of the run set, NARROWED or FULL, as one whole execution
  unit on the shard the plan assigns it; AFP-V0-041 test slices do not apply. The plan is read
  strictly: canonical bytes of the profile within the STR-V0-050 plan bound, `count` shards
  indexed in order, no file twice, 1 to 4096 files, and a partition digest that recomputes,
  else exit 2. The manifest's
  `partitionDigest` SHALL be the plan's, so the STR-V0-018 manifest digest, every receipt and the
  STR-V0-009 fan-in bind the plan, and `verify-shards` stays `INCOMPLETE` for a missing shard
  receipt. A plan whose count differs from the run's, or that omits a run-set package, places no
  Go unit and is `INCOMPLETE`. `--shard-plan` applies only to measured placement. As for balanced
  placement (STR-V0-008), measured placement executes only in the local profile (`--profile ci`
  with `--dry-run` only), and a sharded execution needs one `--manifest` from a `--dry-run`.

## Non-goals

- A watch daemon, IDE integration, or per-keystroke runs; that is LPCV's live loop.
- Learned or cost-based suppression of any selected unit; costs change placement only.
- Replacing vet, format, build, interop, documentation or artifact checks; they stay full.
- Retry and flake policy (AFP-V0-029 owns reruns), remote execution, or a hosted service.
- Test-level selection inside a package beyond the existing AFP-V0-041 slices.
- Relaxing any AFP-V0-028..030 admission condition or the shadow bar.
- Retaining durations: the shard planner reads tables an operator already retained (STR-V0-051);
  it fetches no hosted run and never reads `corvint-test-run/0` receipts (STR-V0-011).
- Splitting one listed file across shards: a file heavier than the ideal shard sets the floor of
  a measured plan.

## Boundaries and failure modes

| Condition | Result |
|---|---|
| Plan cannot be produced, base unresolved, module-level frontier, unallowed unknown | `FULL` with named reason |
| Language with selected units but no adapter, undeclared in `test-commands.json` | `INCOMPLETE`, nonzero |
| Language with no adapter declared `{"none": reason}` (gate-scoped: no gate step, driver or CI job runs its suite) | nothing runs; `<language>: <reason>` recorded in the manifest's `noSuite` |
| Runner version cannot be read | that adapter `INCOMPLETE`, nonzero |
| Partition, manifest or source identity differs across shards | shard fails; fan-in `INCOMPLETE` |
| Worktree bytes change during the run | drift, shard `INCOMPLETE` |
| Missing shard receipt, duplicate execution unit, unit without terminal outcome | fan-in `INCOMPLETE` |
| Any failed outcome or nonzero adapter exit | `FAIL`, nonzero |
| `.github/cishards/test-commands.json` unreadable (absent, oversized, unknown or malformed member, no `go`) | `INCOMPLETE` in every scope, nonzero, no unit runs |
| The Makefile `go-test` rule is not exactly the `tools/test-commands-run` delegation (a prerequisite, `::`, a changed, continued or second recipe line, no recipe, a second rule, a recipe on a rule whose target could match `go-test`), a Makefile or included line sets `SHELL`, `.SHELLFLAGS`, `MAKEFLAGS` or `.RECIPEPREFIX`, exports `MAKEFLAGS` or every variable, or defines a target- or pattern-specific variable that could reach `go-test`, the Makefile needs make to be read (`$(eval)`, `define`, a computed or missing `include`), `GATE_STEPS` is not one literal line, a declared `make` target is absent, or a `make gate` step recipe runs a `none` language's runner | `make test-commands-check` fails (`make gate`, CI doc-gates) |
| The declaration is unreadable or absent when `make go-test` runs, or the runner is started with arguments or outside the repository root | `tools/test-commands-run` refuses with exit 2 before `go` starts; `go test` does not run |
| Bound exceeded | `INCOMPLETE`, nonzero |
| Signal | owned groups killed, `INCOMPLETE`, no fallback |
| CI admission fails (pins, queue rules, `.github/` change, trip, kill switch) | `FULL` |
| `test shards` history absent, unreadable, not admitted, over 64 tables, or naming no listed file | complete round-robin, `fallback` reason, stderr `FALLBACK`, exit 0 (STR-V0-053) |
| `test shards` list empty, oversized, repeated or malformed entry; bad count or K | exit 2, no plan |
| `--shard-plan` unreadable, non-canonical, incomplete or with a foreign digest | exit 2, nothing runs |
| Measured plan for another shard count, or omitting a run-set package | no Go unit, `INCOMPLETE` |

Every failure fails closed toward a larger run or a nonzero exit. No path narrows on missing
evidence (invariant 2), and repository declarations outrank the planner (invariant 3).

## Acceptance evidence

Testing matrix (all `NOT_RUN` until implementation):

- fixture repositories per verdict (`NARROWED`, `FULL` for each fallback reason, `NOTHING`,
  `INCOMPLETE`), each asserting the exact receipt;
- `--dry-run` executes no process other than Git;
- universe placement equals the current `go-product-shard` membership for the same inputs;
- balanced placement places every execution unit once for 1 to 16 shards, a split package's
  catch-all slice included;
- `verify-shards` reports `INCOMPLETE` for a dropped, duplicated, extra or foreign-digest receipt
  and a missing empty-shard receipt, and for a shard that recorded `INCOMPLETE` after drift while
  its outcomes all passed with zero exits, and `FAIL` for a failed outcome or nonzero exit with every
  digest correct;
- editing an already-dirty file between planning and launch, or during the run, yields drift;
- planning failure runs the declared full scope, and an unreadable declaration is `INCOMPLETE`
  in every scope with no runner started;
- a language without an adapter runs its declared full command, records a declared `none`
  assertion in `noSuite` and runs nothing, or is `INCOMPLETE` when undeclared;
- a declared repository-relative command (`./tools/...`) runs from `--root` when `corvint` is
  invoked from another directory;
- `make test-commands-check` passes on the repository and fails for an absent declaration, a
  declared `make` target that does not exist, a `none` language whose runner a gate step runs, a
  `none` language the gate has no runner list for, and for each Makefile text rule: a changed,
  continued or second `go-test` recipe line (also after a comment line), a missing recipe, a
  prerequisite or `::` on the `go-test` line, a second `go-test:` rule, `SHELL`, `.SHELLFLAGS`,
  `MAKEFLAGS` or `.RECIPEPREFIX` assigned (plain, exported, for `go-test`, for a pattern),
  `export MAKEFLAGS`, bare `export`, `.EXPORT_ALL_VARIABLES`, a target-, pattern-, match-anything-
  or expanded-target variable reaching `go-test`, a recipe on a pattern or expanded-target rule
  (tab-indented or after `;`), `$(eval ...)`, `define`, a computed or missing `include`, a second
  or expanded `GATE_STEPS`, and a `SHELL` set in a nested included file; and passes for an
  unrelated pattern variable or rule, `go-test` as a prerequisite, an empty conditional and a
  missing optional include;
- `tools/test-commands-run` under a fake `go` on `PATH` receives `test <go.flags> ./...`, the
  repository root as its directory and `go.env` over an inherited `GOCACHE` while an inherited
  `GOFLAGS` is kept, propagates `go`'s exit status, and refuses arguments, an unreadable or
  absent declaration and a directory without `go.mod` with exit 2; `make -n go-test` prints the
  one line `go run ./tools/test-commands-run`;
- signal tests show killed groups and no fallback start;
- the STR-V0-013 parity report over the retained substitute corpus;
- `test shards` weighs by the median of the last K runs, flags unknown files, places every file
  once for 1 to 16 shards, repeats its bytes and digest, falls back to the reported round-robin for
  each history failure, and writes nothing but stdout and stderr; a measured plan drives a
  two-shard local run whose union `verify-shards` accepts and whose missing receipt is
  `INCOMPLETE`.

Promotion from experimental requires the parity report, 20 merged PRs through the queue on the new
CI call site with complete forward rows and no trip, and median and p90 PR wall time and
runner-minutes per merged PR measured before and after, retained in a build-log entry. Unavailable
hosted measurements stay `NOT_OBSERVED`. Balanced placement is promoted separately. Locally it
qualifies only if executed runs over the same frozen pairs, with the same shard count, lower the
p90 of the slowest shard's wall time by at least 20% against universe placement, with every
execution unit placed once; otherwise it stays unpromoted. In CI it qualifies only after an accepted AFP-V0-028 amendment defines an executed
comparison, and only if that lowers p90 narrowed-PR wall time over 20 PRs. Computed placement
estimates never count as observed timing.

Kill criteria: an unexplained parity difference that cannot be fixed; any queue `MISS` caused by the
runner rather than the selector; or balanced placement failing the local 20% bar or the CI p90 bar
above, which removes balanced placement only.

## Rollout and rollback

1. Go adapter, `--dry-run`, receipts and universe placement, used by hand alongside both drivers.
   Done (slice 1).
2. Parity report (STR-V0-013); then `make gate-affected` calls `corvint test`. Done: the parity
   report (slice 2, decision 0526), and the call site (slice 3, `script/gate-affected-corvint.sh`).
3. CI call site behind AFP-V0-016 consent with a new forward epoch; shadow first, as AFP-V0-029.
   Slice 3 adds the shadow `--dry-run` step and `CORVINT_PR_FORWARD_EPOCH: "1"`. The shadow now
   runs: AFP-V0-047 builds it from `CORVINT_TEST_SHADOW_SOURCE`, outside the tested checkout and
   apart from the empty driver pins, and retains each PR shard's selection for the AFP-V0-029
   forward rows. Step 3 remains a shadow, NOT promoted: the Go tests step still executes every
   shard in full, and nothing narrows until the AFP-V0-029 bar is met and V1-0812 lands.
   Promotion evidence (V1-1257, 2026-10-16): parity over the retained corpus and 142 hosted shadow
   runs with no `UNEXPLAINED` difference or `MISS`, 20 queue-merged epoch-1 PRs with complete
   forward rows and no AFP-V0-030 trip, and observed before and after timing. The delivery status
   stays experimental until the STR-V0-004 manifest and STR-V0-011 have executable evidence; this
   step stays a shadow.
4. Balanced placement executes locally; in CI it is computed and retained while universe
   placement executes, until an accepted AFP-V0-028 amendment says otherwise.
5. Further adapters, each with its own fixture qualification, before its language may narrow.
6. Duration-balanced shard plans (STR-V0-050..054): `corvint test shards` and local measured
   placement, experimental; CI keeps universe placement and `.github/` is unchanged. Promotion
   follows balanced placement's local 20% bar against universe placement.

Rollback of STR-V0-050..054: revert their commit; `test shards` and `--placement measured` are
additive, read no state other than their named inputs, and change no default, CI step or receipt
reader. Rollback of the V1-1257 evidence edits: revert their commit; they change no code, CI or selection.
Rollback: restore the previous call site (`script/gate-affected.sh`, `tools/corvint-pr-tests`),
which this spec leaves in place until step 3 is promoted. For slice 3: point the `gate-affected`
target back at `GO_TEST_COMMAND='$(GO_TEST_COMMAND)' script/gate-affected.sh "$(BASE)"` (the
script and its test stay in the tree, and `gate-affected-test` still runs it), and delete the two
shadow steps and `CORVINT_PR_FORWARD_EPOCH` from `.github/workflows/ci.yml`, which changes no CI
decision (clearing `CORVINT_TEST_SHADOW_SOURCE` alone stops the shadow, AFP-V0-047); `CORVINT_PR_NARROWING=off` restores full
CI immediately (AFP-V0-030). Removing the command removes no other capability.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| STR-V0-001 | `cmd/corvint/test.go` (`compileAffectedGraph` shared with `affected`) | `cmd/corvint/session_test_command_unix_test.go` TestSTRV0018CLIDryRunLaunchesNoRunner, TestSTRV0010CLIRunWritesReceipt, TestSTRV0001CLIRefusesProtectedOutputPaths |
| STR-V0-002 | `internal/sessiontest/plan.go`, `internal/gateselect` | `internal/sessiontest/plan_test.go` TestSTRV0003BuildVerdicts, TestSTRV0007UniversePlacementEqualsCIPlan |
| STR-V0-003 | `internal/sessiontest/plan.go`, `internal/sessiontest/declared.go` (`ReadDeclared`: strict, bounded read of `.github/cishards/test-commands.json`) | TestSTRV0003BuildVerdicts, TestSTRV0003ReadDeclared, TestSTRV0003CLINoPlanRunsDeclaredScope, TestSTRV0003CLINoPlanRunsDeclaredLanguageCommand, TestSTRV0003CLIUnreadableDeclarationIsIncomplete, TestSTRV0003DeclaredRelativeCommandResolvesAgainstRoot, TestSTRV0003CLIDeclaredRelativeCommandRunsFromRoot |
| STR-V0-004 | `internal/sessiontest/run.go` (Go adapter; declared language argv), `declaredLanguage` in `plan.go` (gate-scoped `none` to `noSuite`, undeclared to `INCOMPLETE`); `tools/test-commands-check` refuses a `none` language a `GATE_STEPS` recipe runs; `internal/sessiontest/adapter.go` (`AdapterManifests`: the in-process `corvint-test-adapter/0` capability manifest of the Go and declared adapters, exactly the LPCV-V0-021 `cancel`, `discover`, `execute` and `plan` operations; `refuseUnmanifested` in `Execute` refuses an absent, re-versioned, misnamed or incomplete manifest before discovery) | TestSTRV0004ParseTestEvents, TestSTRV0004RunnerDiscoveryRefusals, TestSTRV0003BuildVerdicts, TestSTRV0003CLINoPlanRunsDeclaredLanguageCommand; `internal/sessiontest/adapter_test.go` TestSTRV0004AdapterCapabilityManifest (pinned version and operations), TestSTRV0004AdapterManifestRefusals (absent, other or empty version, other adapter, missing or unknown operations); TestSTRV0004RunnerRefusesAdapterWithoutManifest (no probe or launch for an absent or mismatched Go manifest; an unregistered adapter refused by name while Go still runs; the declared command neither probed nor run without the declared manifest) |
| STR-V0-005 | `Invocations` in `run.go` | TestSTRV0006Invocations, TestSTRV0010CLIRunWritesReceipt |
| STR-V0-006 | `Invocations` (typed argv from the declaration's Go environment and flags); `tools/test-commands-run` behind the fixed Makefile `go-test` recipe; `tools/test-commands-check` behind `make test-commands-check` (`GATE_STEPS`, CI doc-gates) | TestSTRV0006Invocations, TestSTRV0003ReadDeclared, `tools/test-commands-run/main_test.go` TestSTRV0006RunnerRunsTheDeclaration (fake `go`: argv, directory, environment precedence, exit status), TestSTRV0006RunnerRefusals, TestSTRV0006Invocation; `tools/test-commands-check/main_test.go` TestSTRV0006DeclarationMatchesMakefile (repository pass, three declaration mutations and an absent file), TestSTRV0006MakefileTextRules (thirty-five Makefile text refusals, among them a recipe line inside a conditional and `.IGNORE`, and five passes), TestSTRV0006LedgerMatchesDeclaration (twelve `ledger/go-test` refusals: flags, timeout, `-race`, cache, ledger environment, an appended or unknown variable, a changed or second recipe line, target variables; the rule removed passes), TestSTRV0006IncludedFile (nested include, prerequisite-only `$` target line), TestSTRV0006Classify |
| STR-V0-007 | `placeGo` over `cishards.PlanFrom` | TestSTRV0007UniversePlacementEqualsCIPlan, TestSTRV0009CLIShardsAndVerify |
| STR-V0-008 | `placeGo` balanced branch over the run set (`Input.Placement`); `cmd/corvint/test.go` refuses `--profile ci` execution and a sharded run without `--manifest` | `internal/sessiontest/balanced_test.go` TestSTRV0008BalancedPlacesEveryUnitOnce (1 to 16 shards, catch-all slice), TestSTRV0008BalancedUsesEveryShard, TestSTRV0008BalancedFullAndManifest; TestSTRV0008CLIUsageRefusals, TestSTRV0008CLIBalancedShards; the slice-3 CI shadow retains universe placement only, so retaining balanced placement in CI and the local 20% bar are `NOT_RUN` |
| STR-V0-009 | `internal/sessiontest/verify.go`, `corvint test verify-shards` | TestSTRV0009VerifyShardsMatrix, TestSTRV0009CLIShardsAndVerify |
| STR-V0-010 | `Receipt`, `Finish`, `testWriteReceipt` | TestSTRV0011FinishResultAndSummary, TestSTRV0018ManifestStrictRoundTrip, TestSTRV0010CLIRunWritesReceipt |
| STR-V0-011 | no reader other than `verify-shards` (`runTestVerifyShards` over `sessiontest.ReadReceipt` and `Verify`) | `cmd/corvint/session_test_receipt_readers_test.go` TestSTRV0011ReceiptsHaveNoOtherReader: the exact set of receipt sites in product Go (receipt symbols), scripts (including extensionless `#!` files), workflows and Makefiles (receipt profile and file name), with spy controls; a second reader in `cmd/corvint`, in `internal/sessiontest` or in a script fails it |
| STR-V0-012 | `--profile ci` argv and environment on the same code path (`ciGoEnv`, `ciGoFlags` in `run.go`); the ci branch of `declaredLanguage` in `plan.go` (declared commands to `checks`); `gate-affected` in `Makefile` over `script/gate-affected-corvint.sh` | TestSTRV0006Invocations; `internal/sessiontest/ciconstants_test.go` TestSTRV0012CIConstantsMatchFullBranch (job env and both full-branch `go test` lines of `go-product-shard`); TestSTRV0003BuildVerdicts (two ci-profile cases: a declared command and a make target become `checks`); `script/gate-affected-corvint_test.sh` via `make gate-affected-test` (clean, narrowed, `go.mod` FULL, committed range, failing test, bad and option-shaped BASE, SIGTERM forwarding); `docs/build-log/2026-10-12-session-test-runner-slice-3.md` (a real `make gate-affected` run); CI admission (pinned build, queue backstop, trip) stays the driver's, and the shadow step runs from `CORVINT_TEST_SHADOW_SOURCE` (AFP-V0-047; `TestAFPV0047WorkflowBindings`), with its first hosted run `NOT_OBSERVED` |
| STR-V0-013 | `tools/str-parity` (offline replay of `corvint test --dry-run` against `script/gate-affected.sh` and `tools/corvint-pr-tests` over `tools/str-parity/testdata/pairs-645304a2.txt`); slice 3: the local call site switched, and the shadow steps and `CORVINT_PR_FORWARD_EPOCH` in `.github/workflows/ci.yml` | `tools/str-parity/main_test.go` TestSTRV0013ParityReading, TestSTRV0013CorpusReading; `docs/build-log/2026-10-11-session-test-runner-parity.md` and `docs/build-log/2026-10-12-session-test-runner-declaration.md`: the retained 50-pair corpus, no `UNEXPLAINED` difference; the AFP-V0-011 list and the AFP-V0-029 shadow rows are `NOT_OBSERVED`, and the AFP-V0-029 shadow bar is still owed before CI narrowing; `docs/build-log/2026-10-12-session-test-runner-slice-3.md`: zero shadow rows, a vacuous epoch advance; `docs/build-log/2026-10-13-ci-forward-rows.md`: the AFP-V0-047 shadow pin and forward rows, hosted rows `NOT_OBSERVED`; `docs/build-log/2026-10-16-v1-1257-str-v0-promotion-evidence.md`: the 50-pair corpus rerun at the promotion base and 142 hosted shadow runs against the driver, no `UNEXPLAINED` difference, and the 20-PR forward-row count |
| STR-V0-014 | `internal/sessiontest/procgroup_unix.go`, `testExecute`; signal forwarding in `script/gate-affected-corvint.sh` | TestSTRV0014RunnerCancelAndBudgetKillGroup, TestSTRV0014CLISignalKillsRunAndExits; `script/gate-affected-corvint_test.sh` (SIGTERM reaches the running corvint, exit 143; HUP, INT or TERM during the build stops every build descendant, V1-1310; TERM with a failing `ps` stops them from a stand-in `/proc` table, V1-1400) |
| STR-V0-015 | `MaxUnits`, `MaxDocumentBytes`, `DefaultBudget`, `CIBudget`, `Runner.ShardBudget` (one deadline per shard in `Execute`) | TestSTRV0015UnitBound, TestSTRV0014RunnerCancelAndBudgetKillGroup, TestSTRV0015BudgetBoundsTheShard, TestSTRV0012CIConstantsMatchFullBranch (the ci budget against the job's `timeout-minutes`) |
| STR-V0-016 | local profile claims no containment (help text); the slice-3 shadow steps run under the workflow's existing read-only permissions with no persisted credentials | help text; `make ci-least-privilege-check`; the AFP-V0-047 shadow checkout keeps `persist-credentials: false`; hosted shadow runs observed from AFP-V0-047 (`docs/build-log/2026-10-16-v1-1257-str-v0-promotion-evidence.md`: 142 hosted runs retained shadow selections; the permissions themselves rest on `make ci-least-privilege-check`) |
| STR-V0-017 | `internal/sessiontest/identity.go`, drift rechecks in `Execute` | TestSTRV0017Identify, TestSTRV0017RunnerDrift |
| STR-V0-018 | `internal/sessiontest/manifest.go`, `--dry-run --manifest`, shard recompute match | TestSTRV0018ManifestStrictRoundTrip, TestSTRV0018CLIDryRunLaunchesNoRunner, TestSTRV0009CLIShardsAndVerify |
| STR-V0-019 | `checkGoTestTimeouts` in `tools/test-commands-check/main.go`; `host-adapter-test` passes `-timeout $(GO_TEST_TIMEOUT)` (V1-1552) | `tools/test-commands-check/main_test.go` TestSTRV0019GateGoTestHasTimeout (repository pass; an unbounded gate-step `go test` refused; `-timeout=` passes) |
| STR-V0-050 | `ListedFiles`, `PlanShards` in `internal/sessiontest/shards.go`; `runTestShards` in `cmd/corvint/test.go` | `internal/sessiontest/shards_test.go` TestSTRV0050ListedFiles, TestSTRV0050NearBoundaryPlanRoundTrip (4096 escaping names at the 1 MiB list bound, 4095 unknown, 64 shards: encodes over 1 MiB within the plan bound and reads back byte-identically; non-UTF-8 refused); `cmd/corvint/session_test_shards_unix_test.go` TestSTRV0050CLIShardsPlan (deterministic stdout, plan reads back, input directory unchanged, seven usage refusals), TestSTRV0050ShardsInputReadIsBounded (FIFO refused without blocking; a file growing past the bound is never returned over it) |
| STR-V0-051 | `readHistory`, `historyWeights`, `median` | TestSTRV0051MedianOfLastKRuns (K=2 and K=3 over three runs, a partially recorded file, an unknown file, K bounds), TestSTRV0050CLIShardsPlan |
| STR-V0-052 | `PlanShards` placement, `ShardPartitionDigest` | TestSTRV0052LongestFirstExactlyOnceDeterministic (a worked longest-first example; 97 files on 1 to 16 shards placed once with equal bytes and digest on repeat) |
| STR-V0-053 | `historyWeights` fallback reasons, round-robin branch of `PlanShards` | TestSTRV0053FallbackIsCompleteRoundRobin (thirteen history failures), TestSTRV0053FallbackReasonIsDeterministic (sixteen invalid durations give identical plan bytes over 50 runs; a long reason is cut), TestSTRV0053CLIShardsOverTableBound (65 tables: exit 0, FALLBACK), TestSTRV0050CLIShardsPlan (absent and missing history) |
| STR-V0-054 | `ReadShardPlan`; `placeMeasured` and the measured partition digest in `Build`; `--placement measured --shard-plan` in `parseTestOptions` and `runTest` | TestSTRV0054ReadShardPlan (seven malformed plans), TestSTRV0054MeasuredPlacement (NARROWED, FULL, other count, omitted package, no plan); TestSTRV0054CLIMeasuredShards (two-shard run, `verify-shards` PASS and missing-shard `INCOMPLETE`, five usage refusals) |

## Open decisions

- Whether the Playwright profile's file commands become the first non-Go adapter or stay separate.
- Adapter order after Go, likely by the languages adopters run most.
- Whether balanced placement should ever become the default.
