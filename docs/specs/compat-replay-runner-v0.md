# Compatibility Replay Runner V0

Owner: Russell Lewis
Date: 2026-09-04
Requirement prefix: `CRR-V0`
Intent status: accepted (decision 0052)
Delivery status: experimental
Revision: 15 (2026-09-12), full manifest replay citation repinned to its test declaration
Authoritative inputs: `ROADMAP.md:343-348` (AT-10), `docs/specs/compat-trial-v0.md`
(`CTR-V0-001..010` accepted by decision 0052; `CTR-V0-011/012` retain proposed status),
`internal/procgroup/process_posix.go` and `process_test.go`,
`conformance/perf-v0/process.go`, `process_posix.go` and `process_test.go`, `AGENTS.md` invariants 2 and 4.

## Agent digest
- Claim: Specifies `tools/compat-trial`'s descriptor validation, containment, comparison and status envelope AT-10's runner implements under frozen CTR-V0-001-003/010.
- Status: accepted (decision 0052)/experimental
- Exists: `tools/compat-trial` (`descriptor.go`, `fixture.go`, `runner.go`, `snapshot.go`, `main.go`) and `internal/procgroup` (`process.go`, `process_posix.go`, `process_other.go`, `adjudicate.go`), the latter extracted from `cli-parity-v0` per `CRR-V0-003`(d); no acceptance row below is marked passed, the full manifest replay (`TestGPKV0002ManifestReplay`, `conformance/cli-parity-v0/runner_test.go:1218@6ead4d11`) passes after the extraction (418 s isolated, 491 s inside `make gate`; the earlier 10-minute timeouts were contention from parallel runs, not a hang), and the scored trial and external/setsid containment remain unqualified.
- Blocked on: scoring on CTR-V0-004/005/009; `make gate` passing every row before promotion to `implemented`.
- Read next: `CRR-V0-003`(d), then `CRR-V0-006`(e)'s truth table, then CRR-V0-001..007.

### Status-marker classification (2026-09-12)

All 29 original no-run-status occurrences in this document are category (c): they are closed wire-status
values, definitions of when that value is emitted, or expected fixture outcomes. None is an
execution-evidence placeholder. The requirement-ID and named-test search finds local coverage in
`tools/compat-trial/*_test.go` and `internal/procgroup/*_test.go`, but a passing test preserves the
required no-run output; it cannot replace that protocol value with `PASS` without changing the
wire contract. The full manifest replay is likewise unrelated to these literals and was not rerun.

## Intent and scope

`ROADMAP.md:345-351` (AT-10) owns the "proposed `tools/compat-trial` descriptor, runner and host
envelope" (`:346`), verified by "contamination and changed-executable cases; interruption leaves
no descendants, including process-group/session escape" (`:349-350`).
`docs/specs/compat-trial-v0.md:187` states AT-10's runner "MUST implement CTR-V0-001 through
CTR-V0-003 and CTR-V0-010 as written," and MUST NOT mark a case `accepted_by` anything but
`NOT_PRODUCED` on its own authority. This is that contract, for the AT-10 runner owner and any
later user of `tools/compat-trial`'s output.

Measurable job: a runner that cannot accept a malformed or unverified descriptor, inherit parent
environment, truncate output into a false equality, overrun its budget, or report a verdict for a
case it did not fully observe — and that leaks no descendant *from the owned process group*;
`setsid` escape stays uncontained and refused (`CRR-V0-003`(c)). Verified current state:
`tools/compat-trial` and `internal/procgroup` exist as the Agent digest records. The two generic
trial harnesses in `CTR-V0-010` still lack process groups; perf-v0 now contains its own bounded
measurement-specific group cleanup but not this runner's observation/adjudication contract.

## Requirements

- `CRR-V0-001`: Descriptor validation, fail-closed before any launch.
  - (a) Schema: a manifest MUST be rejected for any `CTR-V0-001` field added, renamed or dropped,
    at `manifest`, `tasks[]`, `gold` and `resource_profile` level alike.
  - (b) Reference syntax: every `executable`, `stdin`, `snapshot` and fixture reference MUST carry
    a `{path, sha256}` of 64 lowercase hex; empty, short, uppercase or placeholder digests reject.
    `manifest.source` and `tasks[].source` are excluded here: `compat-trial-v0.md:28,29` defines
    both as free-text descriptions ("population description", "how found"), not `{path, sha256}`
    objects, and no `CTR-V0-001` field pins their shape further — see the proposed amendment
    below. `CRR-V0-001` MUST NOT reject a `source` field for lacking a digest it was never
    required to carry.
  - (c) Reference content: syntax alone is never sufficient. Each pinned digest MUST be recomputed
    over the bytes, and a well-formed but mismatched digest MUST be rejected under (e) — `old`/
    `new` `executable_sha256` (`CTR-V0-011`) and `snapshot_sha256` alike. `source` carries no
    digest and is therefore never checked here.

    Accepted amendment (AT-10, decision 0052) to `CTR-V0-001`: `manifest.source` and
    `tasks[].source` are each a JSON string, never an object, and never digest-bearing — the two
    `source` fields `compat-trial-v0.md:28-29` names are plain descriptive text
    ("population description", "how found") the runner MUST accept as any non-empty string and
    MUST NOT recompute or reject for shape. The complete list of `CTR-V0-001` fields (b)/(c)
    actually govern is exactly: `snapshot`+`snapshot_sha256`, `old.executable`+
    `old.executable_sha256`, `new.executable`+`new.executable_sha256`, and `stdin`+`stdin.sha256`
    — every other reference-shaped field in `compat-trial-v0.md:27-33` is prose or a ref pointer
    (`comparison_policy_ref`, `resource_profile_ref`, `baseline_ref`, `preregistration_ref`,
    `accepted_intent_ref`, `historical_observation_ref`, `gold_ref`), not a `{path, sha256}` pair.
    End of accepted amendment.
  - (d) Inherited constraints (`docs/specs/compat-trial-v0.md:33`) stay binding and checked here:
    `build_identity` = `{go_version, GOOS, GOARCH, CGO_ENABLED, build_flags}` with `go1.27.0` and
    `-trimpath`, `old` == `new` identity, `cwd` inside the per-repetition scratch copy, and
    `snapshot_sha256` over a canonical tar. `resource_profile.repetitions` (the field is
    `compat-trial-v0.md:31`, whose values `CTR-V0-003` governs without fixing a count) MUST
    equal 3, and any other value MUST be rejected here. That rejection is a `CRR-V0`-local
    strengthening of `CTR-V0-002`'s prose — "three repetitions per version"
    (`compat-trial-v0.md:35`), which states a comparison policy and not a descriptor rejection
    rule — and not an inherited `CTR-V0-003` constraint. It is imposed because `CRR-V0-004`'s
    nine pairs and `CRR-V0-006`(e)'s six-record rows hard-code that count. The same closure
    applies to `resource_profile`'s other `CTR-V0-003`-governed fields
    (`compat-trial-v0.md:31`): `input_bytes` MUST equal 64 KiB, `output_bytes_per_stream` MUST
    equal 1 MiB, `fixture_entries` MUST equal 100 and `fixture_bytes` MUST equal 10 MiB — the
    same four constants `CRR-V0-005`(a) enforces at runtime — and a descriptor naming any other
    value for one of them MUST be rejected here as `descriptor-invalid`, exactly as
    `repetitions` != 3 is above.
  - (e) Zero launches: every rejection in (a)-(d) and (f) MUST happen before the first
    `Cmd.Start`, and the case's envelope MUST show no started process — zero records and
    `unobserved_launches` == 0 (`CRR-V0-007`(b)) — not a late failure during replay.
  - (f) Effect-field typing and containment: `compat-trial-v0.md:31` lists
    `resource_profile.allowed_effects` and `:33` defines `tasks[].expected_effects` as "declared
    scratch-file effects" without typing either element, so this spec pins both here. An element
    is an object `{path, kind}`: `path` is a scratch-relative path literal — no leading `/`, no
    `..` segment, no URL scheme — and `kind` is one of the closed enum `write`, `create`,
    `delete`, `mode` (revision 12: this object form replaces a struck `path!delete`/`path!mode`
    suffix-literal grammar). That typing, like (d)'s `repetitions` == 3, is a `CRR-V0`-local
    strengthening of a value `CTR-V0-003` governs — `compat-trial-v0.md:36` ends "CTR-V0-003
    governs its values" and `:42` states `allowed_effects` "is an explicit allowlist, never
    ambient" — and not an inherited `CTR-V0-003` constraint; it amends no `CTR-V0` clause. The
    struck suffix grammar could not close because `!` is an ordinary filename byte: no escape
    rule distinguished a literal `foo!mode` filename from a declared mode change on `foo`, so
    that suffix syntax was ambiguous by construction; the `{path, kind}` object removes the
    ambiguity because `path` is never parsed for a suffix. Containment is over (path, kind)
    pairs, never string equality: `{path, kind: create}` or `{path, kind: write}` each permit
    only that one kind of that path, `{path, kind: delete}` permits delete of it and
    `{path, kind: mode}` permits a mode change of it, and every (path, kind) pair a
    `tasks[].expected_effects` element declares MUST be permitted by some `allowed_effects`
    element naming the same path and kind. An element that is not an object of that shape, and a
    declared (path, kind) pair no `allowed_effects` element permits, MUST each be rejected as
    `descriptor-invalid` before any launch, landing on `CRR-V0-006`(e)'s row 2.
  - (g) Fixture ownership: per `CTR-V0-010` (`compat-trial-v0.md:54`), "external execution
    remains unavailable; read-only inspection and owned synthetic runner tests may continue," so
    every `executable` (`old`/`new`) and `stdin` reference MUST resolve to an entry in a closed
    synthetic fixture registry — a `{name, sha256}` list this spec's fixture manifest owns,
    disjoint from and never populated by a descriptor — before (c)'s digest check runs.
    A descriptor whose `executable_sha256` matches no registry entry's `sha256` MUST be rejected
    pre-launch as `fixture-not-registered`, distinct from (c)'s mismatched-but-otherwise-pinned
    digest rejection, because an arbitrary executable outside the registry is exactly the
    external-execution case `CTR-V0-010` forbids, digest or no digest.
    The ordering over those three fields is fixed and total, so the refusal-typing buckets of
    decision 0053 never blur: (b)'s lexical 64-lowercase-hex check runs FIRST over
    `old.executable_sha256`, `new.executable_sha256` and `stdin.sha256`, then (g)'s registry
    lookup, then (c)'s content check. A digest that is lexically malformed MUST therefore be
    rejected as `descriptor-invalid` even when it also names no registry entry — the registry is
    a set of well-formed digests, and a string that cannot be one is a descriptor defect, not an
    ownership question. A digest that is well-formed but unregistered is `fixture-not-registered`
    whether or not the referenced bytes would have hashed to it, because (c) never runs.
- `CRR-V0-002`: Environment allow-list, reconciled with the mandatory determinism variables.
  - (a) The launched command's environment MUST equal the descriptor's `env` allowlist
    (`CTR-V0-001`) exactly, set through `Cmd.Env` on every launch, never inherited;
    `tools/cw-trial/main.go:1460` and `tools/cem-trial/main.go:901` explicitly inherit the full environment and are not the replay policy.
    `Cmd.Env` MUST also be non-nil for an *empty* allowlist, because a nil `Cmd.Env` inherits
    `os.Environ()`. At the extraction base, `conformance/cli-parity-v0/process.go` used
    `append([]string(nil), spec.Env...)` and returned nil for an empty allowlist; the delivered
    `internal/procgroup/process.go:231` uses a non-nil zero-capacity slice instead.
  - (b) `docs/specs/compat-trial-v0.md:38` makes `TZ=UTC` and `LC_ALL=C` mandatory per repetition
    and (a) forbids undeclared variables, so `env` MUST declare both: missing either rejects under
    CRR-V0-001(e), a conflicting value rejects, and neither may be injected behind the allowlist.
- `CRR-V0-003`: Process-group containment, its regression, its limit, and its required change.
  - (a) Mechanism: at the extraction base, `conformance/cli-parity-v0/process.go`,
    `process_posix.go` and `main.go` were every one of them `package main`, so no other package
    could import them and neither reuse nor a wrapper on top was available — the mechanism became
    reachable through (d)'s extraction. The cited pre-extraction lines are `process_posix.go`'s `Setpgid` (`:25`), the
    `umask 0o022` lock (`:31`), non-racing reap (`:36`), group termination (`:60`). Cancellation
    is that file pair's own select in `process.go`: the per-process timer (`:191`), the request
    budget's `ctx.Done` (`:198`), the `TimedOut` arm (`:202`), and the `ShutdownTimeout` deadline
    (`:208`) that bounds termination, cleanup, reap and quiescence proof. The one alternative
    `CTR-V0-010` names is weighed and rejected here: `compat-trial-v0.md:55`'s "second working
    `Setpgid`+`Cancel`+`WaitDelay` pattern" is three lines
    (`tools/retrieval-bench/procgroup_unix.go:14-16`) and could be written directly in
    `tools/compat-trial` without touching an in-use conformance harness, but it yields
    containment alone — no `Started`/`ExitObserved` record, no per-stream overflow, drain or
    quiescence observation, and no adjudication seam — which `CRR-V0-006`(d) and (e)'s row 7
    both require, so the extraction stays on the critical path. The two trial harnesses named in
    `CTR-V0-010` now reuse owned-group supervision under proposed CWT-V0-014/CRT-V0-011;
    their inherited environment, EOF stdin and truncation policy still do not satisfy replay. Perf-v0 now has its own
    measurement-specific group cleanup (`TestRunSampleReapsGrandchildBeforeDelayedSideEffect`),
    but it still lacks this package's observation and adjudication contract.
  - (b) Regression: add a grandchild-survival test modelled on
    `TestRunProcessReapsDescendantBeforeDelayedSideEffect`: the descendant is reaped, its side
    effect never lands.
  - (c) Qualification: `setsid` session escape is NOT contained. A case requiring full descendant
    containment MUST be refused *before launch* and recorded `NOT_RUN` with inconclusive state
    `missing sandbox` and zero records — those two carry the refusal. The input selecting that
    refusal is `Spec.RequireDescendantCleanup`
    (`internal/procgroup/process.go:206-210`), a runner-internal, fixture-only flag: no
    `CTR-V0-001` `tasks[]` or `resource_profile` field carries it (`compat-trial-v0.md:34`,
    `:36`) and `CRR-V0-001`(a) rejects any added field, so no conforming descriptor can reach
    `CRR-V0-006`(e)'s row 1. That row's fixture is the `setsid` helper run with the flag set by
    `TestRunProcessSetsidEscapeQualificationIsPartial`, the precedent for refusing rather than
    attempting it; the test asserts `Started` false. Its
    `PARTIAL` verdict belongs to that package's own vocabulary and is NOT a status here, because
    `CRR-V0-006` declares four vocabularies and `PARTIAL` is in none of them. This refusal covers
    `setsid` escape alone, and it is the only cause of `CRR-V0-006`(e)'s row 1. There is no
    effect-class refusal here: the only declarable effect surface is `tasks[].expected_effects`,
    which `CRR-V0-001`(f) types as scratch-relative path literals and which is therefore
    filesystem-in-scratch by construction. An element that is not one — absolute, containing
    `..`, or URL-form — is `descriptor-invalid` under `CRR-V0-001`(f) and lands on
    `CRR-V0-006`(e)'s row 2, not row 1. No-leak covers the owned process group only.
  - (d) Required change, an extraction and not a reuse: exactly four files —
    `conformance/cli-parity-v0/process.go`, `process_posix.go`, `process_other.go` (its
    `//go:build !darwin && !linux` stub defines the nine platform symbols the package uses —
    seven from `process.go`, plus `processExists` and `processGroupID` from `process_test.go`
    (current calls: `internal/procgroup/process_test.go:352` and `internal/procgroup/process_test.go:132`) — `processPlatformSupported`, `configureProcessCommand`,
    `startProcessCommand`, `waitProcessExitUnreaped`, `terminateProcessGroup`,
    `cleanupProcessGroupBeforeReap`, `proveProcessGroupQuiescent`, `processExists`,
    `processGroupID`, so omitting it leaves
    `internal/procgroup` undefined off darwin/linux) and `process_test.go` (`package main`,
    with current `Run` calls at `internal/procgroup/process_test.go:102` and `internal/procgroup/process_test.go:115`) — MUST move out of `package main` into a
    new importable package `internal/procgroup`, which both `conformance/cli-parity-v0` and
    `tools/compat-trial` then import. The move is mechanical, not textually verbatim: it exports
    the four identifiers the remaining `package main` files reference — `runProcess` and
    `processSpec` (`fixture.go:469`, `runner.go:286`, `:954`, `:1039`), `processObservation`
    under its one exported name `procgroup.Observation` — `runner.go:28`, `:388`, `:831` and
    `runner_test.go:290`, `:341` are the call sites that rename updates — and
    `defaultProcessOutputLimit` (`runner.go:289`) — and rewrites those call sites accordingly; no
    logic changes. `processError`, `processVerdictPartial` and `descendantCleanupUnsupported`
    have no consumer outside the moved files and stay unexported. More than the moved
    fields are needed: `Observation` (`process.go:39-54` pre-rename) carries, in full, `Stdout`,
    `Stderr`, `ExitStatus int`, `Err error`, `Started`, `TimedOut`, `Cancelled`,
    `OutputOverflow`, `WaitCompleted`, `PipesDrained`, `OwnedProcessGroupCleanup`,
    `DescendantCleanupStatus` and `DescendantCleanupQualification` — and nothing naming an
    exit's kind. The post-reap quiescence proof MUST bind the group leader's PID to its start-time
    identity recorded immediately after launch. When a signal-0 group probe reports the numeric
    group still present after reap, a readable current start time different from the recorded one
    proves that PID/PGID has been reused and is not the owned group; an equal or unreadable identity
    remains conservatively present. A probe answering `EPERM` names a member this process cannot
    signal and is presence, never a quiescence proof
    (`TestRunProcessQuiescenceTreatsPermissionDeniedAsPresent`). Linux uses `/proc/<pid>/stat`
    field 22 after the last `)` and Darwin uses `/bin/ps -p <pid> -o lstart=` under fixed locale and timezone. When the start-time
    identity cannot be recorded, or cannot be re-read while a post-reap group is present, the runner
    retains the signal-only quiet-period probe and records `DescendantCleanupQualification` as
    `PARTIAL`. This adds no exported field and changes no frozen wire shape or bytes.
    An unobserved exit and a
    signalled exit both read as numeric `ExitStatus` -1 — `internal/procgroup/process.go:385-391` assigns `ExitStatus` only when
    `WaitCompleted` is true and `ProcessState` is non-nil, and `ExitCode()` is -1 for a
    signalled process — which is exactly the collapse `CTR-V0-002` forbids
    (`docs/specs/compat-trial-v0.md:40`); and its one shared `OutputOverflow` cannot name the
    stream `CRR-V0-005`(b) must name. `internal/procgroup` MUST therefore gain, in that same
    change: `ExitObserved bool`, true only when the leader's exit status was actually read;
    `Signal string`, empty when the process exited normally and otherwise pinned to
    `syscall.Signal.String()`'s spelling — Go's own table, e.g. `killed` for `SIGKILL`, not
    `SIGKILL`/`KILL` — so two conforming runners emit the identical `signal:killed` union value
    for the same kill; and
    `StdoutOverflow bool`/`StderrOverflow bool` carrying each capture's own `exceeded`
    (capture result and per-stream assignment at `internal/procgroup/process.go:378-383`,
    the exported observation fields at `internal/procgroup/process.go:87-107`) in addition to
    their aggregate at `internal/procgroup/process.go:384`. `Cmd.Env`
    (`internal/procgroup/process.go:231`) MUST likewise be non-nil even when the `env` allowlist is empty,
    per `CRR-V0-002`(a). The extraction MUST also expose (e)'s adjudication as a pure function
    over the case's planned count, its pre-launch refusal and its observations —
    `procgroup.Adjudicate(planned int, refusal PrelaunchRefusal, observations []Observation)
    Adjudication`. It returns one envelope struct, not a bare tuple: `Adjudication` carries the
    exported Go fields `Status`, `Outcome`, `RecordCount int`, `Launches`, `UnobservedLaunches`,
    `CaseInconclusiveState` (`CRR-V0-007`(b), `CRR-V0-006`(c)), `Reason string` (below) and
    `FullyObserved bool`, so every
    field an acceptance row asserts is a returned value a wrong implementation cannot omit. This
    is the complete exported surface of the extraction, frozen here: the four exported types are
    `Observation`, `Adjudication`, `PrelaunchRefusal` and its sibling closed-value string types
    `Status`, `Outcome` and `CaseInconclusiveState`; each of those three carries its own exported
    constants for every enum member named in this spec (`Status`: `PASS`, `FAIL`, `NOT_RUN`;
    `Outcome`: `compatible`, `different`, `instability`, `withheld`; `CaseInconclusiveState`: the
    `CTR-V0-008` six plus null; `PrelaunchRefusal`: `none`, `descriptor-invalid`, `digest-mismatch`,
    `fixture-not-registered`, `missing-sandbox`, `resource-bound`, `budget-expired`; `Reason`: the
    eight values below) — a caller MUST NOT need an unexported sentinel or a bare string literal
    to construct or compare any of them. Those
    exported Go names are what a test and `tools/compat-trial` read off the returned struct; the
    snake_case forms `status`, `outcome`, `record_count`, `launches`, `unobserved_launches`,
    `case_inconclusive_state` and `reason` used elsewhere in this spec are the runner envelope's
    *wire* keys
    for the same values, never Go field names — unexported Go fields would be unreadable from the
    importing tool this same clause requires. There is no second state
    field: `CaseInconclusiveState` alone is (e)'s (c) column. `RecordCount` is a count and not
    the records: none of `CTR-V0-012`'s record fields (`compat-trial-v0.md:63`) is an
    `Adjudicate` input, so the runner, not this function, emits the `CTR-V0-012` records, exactly
    `RecordCount` of them. `RecordCount` MUST equal the number of passed observations whose
    `ExitObserved` is true and nothing else, because `CRR-V0-006`(a) emits one record per observed
    exit and none for an unobserved one; it is not derived from `planned`, from
    `len(observations)` or from `Launches`. `planned` is three per version
    under `CRR-V0-001`(d), six per case,
    over which *incomplete* and *fully observed* are defined. `PrelaunchRefusal` is a closed
    enum — none; descriptor-invalid, a `CRR-V0-001` structure, typing or lexical containment
    rejection; digest-mismatch, an actual (b)/(c) hash mismatch (decision 0053); fixture-not-registered, (g)'s registry-lookup refusal,
    decided before `Cmd.Start` and distinct from descriptor-invalid though both land on row 2;
    missing-sandbox, (c)'s `setsid` containment refusal;
    resource-bound, a `CRR-V0-005`(a) pre-launch IO bound; budget-expired, the `CRR-V0-005`(d)
    task entry's cumulative budget expired before this case's first launch — and it, not a field of any
    observation, is what decides (e)'s rows 1, 2, 3 and 4 when the slice is empty. `Adjudicate`
    is total over its input domain: called with `refusal` `none` and an empty `observations`
    slice — a combination no legitimate row above produces, since `none` always pairs with at
    least one observation once a launch is attempted — it MUST NOT panic or return a zero
    `Adjudication`; it returns `NOT_RUN` with `Reason` `no-launch-observed`, distinct from every
    named `PrelaunchRefusal` cause, so a malformed caller is diagnosable rather than silently
    reading as row 1. `Adjudication` carries this as an additional exported field, `Reason
    string`, wire key `reason`: a closed enum with one value per `NOT_RUN` cause —
    `descriptor-invalid` (structure, typing or lexical containment), `digest-mismatch` (a (b)/(c)
    hash check), `fixture-not-registered` ((g)'s registry lookup),
    `start-failed` (the child never started, with no earlier repetition `Started`: either
    `Cmd.Start` failed, or the runner's per-repetition pre-start scratch setup — the temporary
    root, the snapshot materialization, the executable write and the pre-state capture that
    precede `procgroup.Run` — failed and yielded a zero `Observation`; `Adjudicate` sees only a
    never-`Started`, never-`Cancelled` observation and cannot distinguish the two, and `reason` is
    a closed wire enum `tools/compat-trial/runner_test.go`'s
    `TestUnexecutableOwnedBinaryIsStartFailureWithoutLaunch` already pins for the `Cmd.Start`
    branch, so the setup branch widens this reason rather than adding an eighth value),
    `missing-sandbox`, `resource-bound`, `budget-expired` and `no-launch-observed` — finer-grained
    than `CaseInconclusiveState`'s six `CTR-V0-008` buckets and never a substitute for it: a
    digest mismatch and a `Cmd.Start` failure both land on `CaseInconclusiveState`
    compilation/setup failure (`CRR-V0-006`(a)/(c)) but carry distinct `Reason` values, so the two
    causes of a row 2 case no longer collapse into one indistinguishable state. Because
    `budget-expired` selects row 4 from the refusal alone, the runner need not call `runProcess`
    with an already-expired context to reach that row. `DescendantCleanupStatus` and
    `DescendantCleanupQualification` are recorded on the observation but are never decisive for
    a row: a (c) refusal reaches the adjudication as `missing-sandbox`, not as those fields. The
    adjudication otherwise reads only `Started`, `ExitObserved`, `Signal`, `TimedOut`,
    `Cancelled`, `StdoutOverflow`, `StderrOverflow`, `PipesDrained`,
    `OwnedProcessGroupCleanup` and `ExitStatus`. With those three inputs every (e) row 1-7 is
    provable without launching a process. `Adjudicate` is scoped to those seven rows: it decides
    `Status`, `CaseInconclusiveState`, `RecordCount`, `Launches` and `UnobservedLaunches`, and
    reports `Outcome` `withheld` for every one of them, with `FullyObserved` false.
    Rows 8-12 are outside it, and are signalled by exactly one returned value: a fully observed
    case returns `FullyObserved` true with `Status`, `Outcome` and `CaseInconclusiveState` left
    at their zero values, which is the only return in which they are zero.
    The runner, not `Adjudicate`, then picks the row, the `status` and the `outcome` from
    two inputs the function never receives: the case's undeclared-effect fact per
    `CRR-V0-006`(d), and `CRR-V0-004`'s nine `old`x`new` byte comparisons over the captured
    stdout, stderr and exit status of each repetition, tagged by version and repetition in the
    runner's own `CTR-V0-012` records. For rows 8-12 the runner derives `status` itself, straight
    from `CRR-V0-006`(d)'s `PASS`/`FAIL` formula (every planned observation completed AND no
    undeclared effect observed) applied to the same `FullyObserved` true case: rows 8-9 are
    `FAIL`, rows 10-12 are `PASS`, and `Adjudicate`'s zero-value `Status` for those rows is never
    itself a legal envelope value.
    Until that extraction lands the runner cannot emit `CTR-V0-012`'s `exit` union at all, so it
    MUST NOT launch: every case not already refused by `CRR-V0-001` or (c) is `NOT_RUN` with
    inconclusive state compilation/setup failure and zero records, per `CRR-V0-007`(a), and the
    acceptance table below is evaluated only once this extraction has landed.
  - (e) Joined-group scope: a `Spec.JoinAncestorProcessGroup` leader inherits its starter's
    process group instead of creating one (`internal/procgroup/process_posix.go:183-188`), so it
    signals and probes only its own pid and holds no evidence about its descendants. Its
    `Observation` MUST report `DescendantCleanupStatus` `ancestor-process-group`, qualification
    `PARTIAL`, and `OwnedProcessGroupCleanup` false (`internal/procgroup/process.go:223-226`,
    `internal/procgroup/process.go:370`), so `Adjudicate` never counts it as a completed
    observation; the owning ancestor's `Run` carries the descendant cleanup proof. The flag is
    trusted caller intent: nothing verifies that the starter runs inside an owned group.
- `CRR-V0-004`: Byte-exact comparison. Per `CTR-V0-002` the runner MUST run three repetitions per
  version (`CRR-V0-001`(d) pins `repetitions` == 3) and compare stdout/stderr/exit status by exact
  bytes. A version disagreeing with itself is `instability` — never a verdict, never resolved by
  majority vote or by a retry that drops the odd observation, and never overridden by the other
  eight pairs agreeing; two stable versions whose nine pairs agree are `compatible`, two that
  disagree are `different`. Each of those three is the `CRR-V0-006`(b) outcome unless (b)
  requires `withheld` — a missing planned observation or an observed undeclared effect — which
  outranks all three, so `CRR-V0-006`(e) rows 5-9 carry `withheld` however the bytes compared.
- `CRR-V0-005`: Bounded IO and bounded time, refused rather than truncated or overrun.
  - (a) Limits: `CTR-V0-003`'s bounds — 64 KiB input, 1 MiB output per stream, 100 fixture
    entries, 10 MiB aggregate — MUST be refused and reported, never truncated into an apparent
    equality; each needs an at-bound case (accepted) and a bound+1 case (refused). The two
    per-process bounds MUST be set explicitly on every launch: `processSpec.OutputLimit` == 1 MiB
    and `processSpec.InputLimit` == 64 KiB, never left zero. This is the same hazard (d) gates on
    `ShutdownTimeout`: `normalizeProcessSpec` substitutes `defaultProcessInputLimit` and
    `DefaultOutputLimit`, both `16 << 20` (`internal/procgroup/process.go:21-22`),
    for a zero value (`internal/procgroup/process.go:439-449`), so a runner leaving either zero silently gets 16 MiB and never
    refuses at this requirement's bounds — as `runner.go:289` already does by passing
    `defaultProcessOutputLimit`. The 10 MiB aggregate and the 100-entry fixture bound are the
    runner's own pre-launch checks and have no package default to inherit.
  - (b) Independent streams: the output bound is per stream, never summed. Stdout at the bound
    with stderr one byte over MUST be refused naming stderr, and the mirror naming stdout; 1 MiB
    on stdout *and* 1 MiB on stderr MUST be accepted, never refused as 2 MiB combined. Naming the
    stream requires `CRR-V0-003`(d)'s `StdoutOverflow`/`StderrOverflow`. The shared supervisor
    preserves these flags and their aggregate `OutputOverflow` (`internal/procgroup/process.go:378-384`).
    Replay retains the zero-value `OverflowFail` policy: the trial adapters' explicit
    `OverflowTruncate` policy MUST NOT be reused for byte-equality judgments.
  - (c) Stdin: supply `stdin{path,sha256}` from the descriptor rather than hard-code
    EOF stdin as the trial `runCommand` adapters do (`tools/cw-trial/main.go:1458`, `tools/cem-trial/main.go:899`), and
    refuse before launch when the stdin file's computed digest differs from the pinned `sha256`.
  - (d) Budgets: `CTR-V0-003`'s 10 s per-process ceiling and 120 s request budget per `tasks[]`
    entry, cumulative across that entry's six repetitions (decision 0054), MUST each be enforced by *actively* cancelling and terminating the
    owned process group at the deadline, never by a check between repetitions: a repetition
    launched at 119 s is killed at 120 s, not left to finish at 129 s. The mechanism is
    shared supervisor's select — the per-process timer (`internal/procgroup/process.go:315`),
    the task entry's cumulative budget `ctx.Done` (`internal/procgroup/process.go:325-327`),
    and the timer arm setting `TimedOut` (`internal/procgroup/process.go:328-329`) — with
    the `ShutdownTimeout` deadline (`internal/procgroup/process.go:344`) bounding cleanup.
    `ShutdownTimeout` MUST be positive and <= 1 s here. It is not left to the package default:
    `normalizeProcessSpec` substitutes `defaultProcessShutdownLimit`, 2 s
    (`internal/procgroup/process.go:23`), for a zero value (`internal/procgroup/process.go:433-437`),
    which would let a kill at the 120 s deadline run to 122 s. A repetition never launched because
    its task entry's cumulative budget expired is `budget-expired`; when no repetition in that entry
    launched, the case is `NOT_RUN` per (e). Each later entry receives a fresh budget derived from
    the outer caller context, whose cancellation still cancels all remaining work.
- `CRR-V0-006`: Four separate vocabularies, none derived from another.
  - (a) Repetition exit: `CTR-V0-012`'s frozen union — a non-negative integer or `signal:<name>`
    (`compat-trial-v0.md:63`) — is NOT widened; no third value such as `unobserved` is added. A
    repetition whose exit is never observed emits no record at all; the accepted
    amendment at `compat-trial-v0.md:64` (decision 0052) is what reconciles that with `CTR-V0-012`'s
    "every executed repetition MUST produce one record". It has exactly two sites, both reachable
    only after `Started` is set (`internal/procgroup/process.go:265`): an incomplete wait
    (`WaitCompleted` false at `internal/procgroup/process.go:361-362`) and an absent `ProcessState` (`internal/procgroup/process.go:385`). Both are
    `CRR-V0-003`(d)'s `ExitObserved` false; with `ExitObserved` true the record's `exit` is the
    integer `ExitStatus` (`internal/procgroup/process.go:388`) or `signal:` plus `Signal`, never -1 standing for either.
    A failed `Cmd.Start` (`internal/procgroup/process.go:260-263`) returns
    before `internal/procgroup/process.go:265` ever sets `Started`, so it is not a launch and not an unobserved launch: it is a
    pre-launch compilation/setup failure adjudicated by (e)'s never-launched rows. Each site is
    reported through (c) and (d) with a reason, not an invented exit.
  - (b) Comparison outcome, per case: `compatible`, `different` or `instability` per `CTR-V0-002`
    (`compat-trial-v0.md:35`), or `withheld` — the required outcome whenever a planned observation
    is missing or an undeclared effect was observed, a refusal to compare rather than a fourth
    verdict, never read as `compatible`.
    This field is never derived from (a), (c) or (d).
  - (c) Inconclusive state, per case: exactly one of `CTR-V0-008`'s six — timeout,
    compilation/setup failure, resource exhaustion, skipped test, missing sandbox, inconsistent
    run (`compat-trial-v0.md:47`) — or null; one `INCONCLUSIVE` bucket is forbidden.
    `skipped test` is unreachable in replay: no (e) row selects it and this runner skips no case,
    so it is retained in the enum and in the order below only for `CTR-V0-008` parity, and a
    runner that emits it is wrong. The case's state is chosen by (e)'s row order, which governs
    a case outright; this clause's order is scoped to causes coinciding *within one repetition*
    and applies to the case only where an (e) row defers to it explicitly, as row 7 does. It
    never overrides a row that matched on causes present in a different repetition. Within that
    scope, when causes
    coincide the earliest-blocking wins, in this total order: missing sandbox > compilation/setup
    failure > skipped test > resource exhaustion > timeout > inconsistent run (last because it is
    the residue: it wins only when nothing above it blocked; exhaustion outranks timeout because
    an overflow kill is the bound's, so an overflow inside the 10 s window is resource exhaustion,
    and a deadline kill coexisting with a version that disagrees with itself is timeout).
    `CTR-V0-012`'s per-record `inconclusive_state` (`compat-trial-v0.md:63`) is NOT re-scoped: it
    stays per record, and only the repetition(s) the cause actually touched carry it. The per-case
    state is a separate envelope field, `case_inconclusive_state`, carried beside
    `unobserved_launches` (`CRR-V0-007`(b)); (e)'s (c) column is that envelope field, and no
    record may carry a state contradicting it.
  - (d) Execution status, per case: the closed `PASS`/`FAIL`/`NOT_RUN` enum, with the same three
    string values as `conformance/release-artifact-v0/report.go:3-9`'s `statusPass`/`statusFail`/
    `statusNotRun` cited as precedent, not as an import — that file is `package main`
    (`CRR-V0-003`(a) makes the identical point about `cli-parity-v0`) and its consts are
    unexported, so `internal/procgroup` MUST declare these three values itself, and no clause
    requires the two declarations to stay in sync beyond matching string spellings. Status
    records execution completeness and effect declaration *alone*, independent of (b) and (c). A
    planned observation is *completed* only
    when its repetition set `Started` (`internal/procgroup/process.go:265`), was neither
    terminated by the runner itself (`TimedOut`/`Cancelled`) nor stopped at an output bound
    (`CRR-V0-003`(d)'s per-stream `StdoutOverflow`/`StderrOverflow`), its exit was then observed —
    `ExitObserved` true requires `WaitCompleted` and a non-nil `ProcessState` — and both
    `PipesDrained` and `OwnedProcessGroupCleanup` are true. Those last two belong in
    *completed* because a repetition whose pipes never drained has bytes the byte-exact
    comparison never saw, and one whose owned group never quiesced left descendants; either is
    reported by `processRunError` and `proveProcessGroupQuiescent` and folds into (e)'s
    row 7 with (c) inconsistent run. `CRR-V0-005`(d)'s positive, <= 1 s `ShutdownTimeout` makes
    both failures more reachable than the 2 s package default would, which is exactly why they
    are gated here rather than assumed. A repetition
    the runner killed still completes the exit and wait path, so an exit
    observed after a kill is NOT a completed observation and can never make the case `PASS`.
    `PASS` = every planned observation completed AND no undeclared effect observed; `FAIL` = at
    least one repetition set `Started` and at least one planned observation was not completed, or
    an undeclared effect was observed; `NOT_RUN` = no repetition ever set `Started`. A non-null (c)
    does NOT forbid `PASS`: fully observed instability with no undeclared effect is `PASS`. An
    undeclared effect therefore excludes `PASS` outright, so no case matches both an
    agreeing-versions row and the undeclared-effect row of (e). Effect statuses reuse
    `PRODUCED`/`NOT_PRODUCED` (`internal/observations/observations.go:321`,
    `cmd/corvint/dogfood_observe.go:73`); `NOT_PRODUCED` covers two distinct causes this runner
    MUST distinguish rather than collapse: an effect *denied* per `CTR-V0-003`
    (`compat-trial-v0.md:37` — unreachable until the `E:99` host profile is qualified, since this
    runner enforces no denial itself, kept in the enum for that future profile the way
    `CRR-V0-006`(c) keeps `skipped test`) and a declared effect absent from the before/after diff:
    that diff is the only effect evidence this runner has, so a declared effect it does not show
    is `NOT_PRODUCED`, while one it does show is `PRODUCED`; neither is on its own a `FAIL`. Every `effects_observed[]`
    entry therefore carries a closed `basis` field beside its `{path, kind}` and `status`
    — `kind` stays the create/write/delete/mode vocabulary below, `basis` names the cause:
    `declared-absent` for the diff-absent case, `declared-observed` for an observed declared effect,
    `denied` for the unreachable `CTR-V0-003` denial, and `undeclared-observed` for an effect actually observed that no `allowed_effects`/
    `expected_effects` element permits — so a `NOT_PRODUCED` entry's `basis` alone says which of
    the two causes produced it. That status has one carrier and it is not silence: the case's
    `effects_observed[]` MUST hold an entry naming that declared path and kind with status
    `NOT_PRODUCED` and basis `declared-absent`, so `effects_observed[]` lists declared and undeclared
    effects observed plus declared effects absent, each entry carrying its own kind, status
    and basis. Emitting
    nothing for an absent declared effect, or failing the case for it, are both non-conforming.
    An undeclared effect actually observed is never `NOT_PRODUCED` but a `FAIL` case listing it
    in `effects_observed[]` with basis `undeclared-observed`, outcome `withheld`. The observed
    effect surface is bounded and stated as such: an *observed effect*
    is a file-system effect inside that repetition's own scratch copy, obtained by diffing the
    scratch tree before and after the repetition against *that case's* declared
    `tasks[].expected_effects` (`compat-trial-v0.md:33`). That diff observes net transitions
    only: a create-then-delete, a write of identical bytes or a chmod-then-restore within the same
    repetition leaves the before/after trees equal and produces no entry at all — this runner has
    no event-level filesystem instrumentation, so a transient effect is stated as unobservable and
    out of scope for both `effects_observed[]` and `PASS`, never inferred. The diff is per
    repetition but the fact (e)'s rows 8-12 turn on is per case: the case's undeclared-effect fact
    is true when *any* one of the six repetitions shows an undeclared effect, never only when all
    six do. A runner requiring agreement across repetitions would report a version that writes on
    one repetition alone as `PASS`.

    Accepted amendment (AT-10, decision 0052) to `CTR-V0-003`: `CTR-V0-012`'s `effects_observed[]`
    (`compat-trial-v0.md:63`) is a per-record field and stays exactly that — this amendment adds
    no case-level meaning to it and re-scopes nothing there. The runner envelope instead gains a
    sibling field, `case_effects_observed[]`, carrying the union of every repetition's
    `effects_observed[]` entries for the case, deduplicated by (path, kind) pair and sorted by
    path then kind. Each entry is `{path, kind, status, basis}`, reusing the descriptor's
    `{path, kind}` effect grammar below for `path`/`kind`, with `status` one of
    `PRODUCED`/`NOT_PRODUCED` and `basis` one of `declared-absent`, `declared-observed`, `denied`,
    `undeclared-observed`. `case_effects_observed[]`
    is what (e)'s rows 8-9 name and what an acceptance assertion against a case's (rather than one
    repetition's) effects reads; it is an envelope field beside `launches`,
    `unobserved_launches` and `case_inconclusive_state` (`CRR-V0-007`(b)), never a `CTR-V0-012`
    record field. End of accepted amendment.

    The alphabet of that diff is closed — create, write, delete and mode change — and each
    observed effect is classified against the case's list under `CRR-V0-001`(f)'s `{path, kind}`
    grammar, which replaces a struck `path!delete`/`path!mode` suffix-literal grammar (revision
    12): because `!` is an ordinary filename byte, no suffix rule could distinguish a literal
    `foo!mode` filename from a declared mode change on `foo` without an escape convention this
    spec never gave, so the object form removes the ambiguity by never parsing `path` for a
    suffix at all. A `{path, kind: create}` or `{path, kind: write}` entry declares only that one
    kind of that path; delete is declared only by `{path, kind: delete}` and mode change only by
    `{path, kind: mode}`. Each entry declares the kind it names and nothing else, so a task that
    also creates or writes a path it unlinks MUST list `{path, kind: create}` (and/or `write`)
    alongside `{path, kind: delete}`; `{path, kind: delete}` alone covers only a path already
    present in the pinned snapshot. So an unlisted `unlink` or `chmod` of an otherwise declared
    path is itself an undeclared effect, as is any of the four kinds on a path outside the case's
    list. `resource_profile.allowed_effects` (`compat-trial-v0.md:31`) is the
    profile-level outer bound a task declaration may not exceed, never the per-case diff target:
    a `tasks[].expected_effects` element outside `allowed_effects` is a descriptor error refused
    before launch under `CRR-V0-001`(f), not an observation. Diffing against the profile-level
    list instead would read a correctly declared per-task effect as undeclared and force a
    conforming case to (e)'s rows 8-9. An effect of another class — the
    network, model call, installation, repo edit, commit, merge or publishing that
    `CTR-V0-003` forbids (`compat-trial-v0.md:37`) — cannot be written as a `CRR-V0-001`(f) path
    literal at all, so a descriptor attempting one is refused before launch as
    `descriptor-invalid`, landing on (e)'s row 2, rather than launched and watched. `PASS` asserts
    the absence of undeclared effects over that bounded surface only, never over effects this
    runner has no means of observing.
    Containment over that surface is lexical classification only: `CRR-V0-001`(f) rejects a path
    literal that is absolute, contains `..`, or is URL-form, but does not resolve symlinks or
    otherwise prove a declared scratch-relative path stays inside the scratch tree at runtime. An
    outward symlink planted inside scratch, or a write reached through one, can therefore land
    outside the scratch copy and outside this diff's view. Full path-escape containment —
    symlink resolution and any other beneath-scratch proof — is a named non-goal for v0
    (see Non-goals); this runner makes no sandbox claim beyond the lexical check, and `CRR-V0-003`
    governs process-group containment separately from this filesystem-effect surface.
  - (e) Truth table over the per-repetition fields plus the case's effect and comparison facts.
    *Launched* = at least one repetition set `Started`
    (`internal/procgroup/process.go:265`); *runner-terminated* = a `Started` repetition with
    `TimedOut` (`internal/procgroup/process.go:329`) or `Cancelled` (`internal/procgroup/process.go:326`);
    *overflowed* = a `Started` repetition with either per-stream overflow flag of
    `CRR-V0-003`(d) (`internal/procgroup/process.go:382-383`); *incomplete* = any planned
    repetition that did not complete per (d), whether it never set `Started` or then failed
    `ExitObserved` (`internal/procgroup/process.go:385-392`), `PipesDrained`
    (`internal/procgroup/process.go:376-377`) or `OwnedProcessGroupCleanup`
    (`internal/procgroup/process.go:370`); *unobserved* = the `ExitObserved`-false subcase alone,
    which is what `CRR-V0-007`(b)'s counter reads; *fully observed*
    = every planned repetition completed per (d). Every case MUST match exactly one row.

| Case | (d) status | (b) outcome | (c) inconclusive | Records |
|---|---|---|---|---|
| 1. Not launched; 003(c) containment refusal (`internal/procgroup/process.go:206-210`) | NOT_RUN | withheld | missing sandbox | 0 |
| 2. Not launched; any pre-launch failure other than rows 1, 3 and 4's causes: 001 validation, a digest mismatch (001(c), 005(c)), a nil context (`internal/procgroup/process.go:196-198`), an invalid spec (`internal/procgroup/process.go:212-215`), an unsupported platform (`internal/procgroup/process.go:200-204`), a pipe failure (`internal/procgroup/process.go:245-255`), or a failed `Cmd.Start` (`internal/procgroup/process.go:260-263`) | NOT_RUN | withheld | compilation/setup failure | 0 |
| 3. Not launched; a 005(a) pre-launch bound (64 KiB stdin, 100 fixture entries, 10 MiB aggregate) | NOT_RUN | withheld | resource exhaustion | 0 |
| 4. Not launched; its task entry's cumulative budget expired first | NOT_RUN | withheld | timeout | 0 |
| 5. Launched and overflowed, regardless of causes in other repetitions; this case's remaining repetitions never launched | FAIL | withheld | resource exhaustion | those produced |
| 6. Launched and runner-terminated, not overflowed, regardless of causes in other repetitions | FAIL | withheld | timeout | those produced |
| 7. Launched, not overflowed, not runner-terminated, at least one planned repetition incomplete | FAIL | withheld | by (c)'s order over the causes present: compilation/setup failure, else timeout, else inconsistent run | those produced |
| 8. Fully observed; undeclared effect; a version disagrees with itself | FAIL | withheld | inconsistent run | 6 |
| 9. Fully observed; undeclared effect; both versions internally stable | FAIL | withheld | null | 6 |
| 10. Fully observed; no undeclared effect; a version disagrees with itself | PASS | instability | inconsistent run | 6 |
| 11. Fully observed; no undeclared effect; both stable, nine pairs agree | PASS | compatible | null | 6 |
| 12. Fully observed; no undeclared effect; both stable, at least one pair differs | PASS | different | null | 6 |

Each row's selecting predicate, in evaluation order; every row after the first also requires
that no row above it matched, so the twelve are mutually exclusive and jointly exhaustive.
- Row 1: no repetition set `Started` and `CRR-V0-003`(c) refused the case (`internal/procgroup/process.go:206-210`), the
  cause (c)'s total order ranks first.
- Row 2: no repetition set `Started`, and the cause was any pre-launch failure other than
  rows 1, 3 and 4's — validation, a digest mismatch, a nil context (`internal/procgroup/process.go:196-198`), an invalid
  spec (`internal/procgroup/process.go:212-215`), an unsupported platform (`internal/procgroup/process.go:200-204`), a pipe failure (`internal/procgroup/process.go:245-255`) or a
  `Cmd.Start` failure (`internal/procgroup/process.go:260-263`, which returns before `internal/procgroup/process.go:265`).
- Row 3: no repetition set `Started`, and a `CRR-V0-005`(a) pre-launch bound was the cause.
- Row 4: no repetition set `Started`, no cause above it is present, and either arm holds: the
  refusal is `budget-expired`, the runner having refused the case directly against its expired
  task-entry request budget; or no repetition set `Started` and some observation has `Cancelled`
  true, which is the runner instead letting `runProcess`'s *pre-launch* `ctx.Err()` check
  (`internal/procgroup/process.go:217-220`) refuse the launch — that check sets `Cancelled` and never sets `Started`, and
  `PrelaunchRefusal` is `none` on that arm. The second arm selects row 4, never row 2. The
  post-launch `Cancelled` at `internal/procgroup/process.go:326` sits in a select the code reaches only after `Started`
  at `internal/procgroup/process.go:265`, so `!Started && Cancelled` is empty; `internal/procgroup/process.go:326` belongs to row 6 alone.
- Row 5: some repetition has `Started` and either per-stream overflow flag (`internal/procgroup/process.go:331`,
  `internal/procgroup/process.go:382-383`), regardless of causes present in other repetitions — (c)'s order does not
  reach across repetitions to displace this row.
- Row 6: some repetition has `Started && (TimedOut || Cancelled)` and no
  repetition overflowed, likewise regardless of causes present in other repetitions. Unlike
  row 5, a per-process timeout does NOT abort the case: `CRR-V0-007`(c) mandates the abort for
  overflow alone, so the case's remaining repetitions are still launched and each is observed
  normally. Only `CRR-V0-005`(d)'s expired task-entry budget stops later repetitions in that entry, and then
  by the pre-launch `ctx.Err()` refusal (`internal/procgroup/process.go:217-220`) rather than by this row. So a row 6 case
  ordinarily has `Launches` == 6 with `UnobservedLaunches` and `RecordCount` following from the
  per-repetition `ExitObserved` flags — `RecordCount` counting the observed exits, a killed
  repetition whose exit was still read among them — and only a budget expiry mid-case leaves
  `Launches` < 6.
- Row 7: no repetition overflowed and none was runner-terminated, yet at least one planned
  repetition is *incomplete* — the residual row, which absorbs every incomplete repetition of a
  launched case whatever its cause, including a repetition that never reached `Started` at all
  after an earlier repetition already set it. Its (c) state is the first cause present in (c)'s
  total order: compilation/setup failure when a later repetition failed before `Started` (a
  pipe failure, a `Cmd.Start` failure, or an invalid spec); timeout when the pre-launch `ctx.Err()`
  check refused a later repetition against the expired task-entry budget; inconsistent run
  otherwise, covering `ExitObserved` false and any drain or quiescence failure reported by
  `processRunError`/`proveProcessGroupQuiescent`. That order is read only over *present* observations —
  a planned repetition for which the case has no `Observation` at all (never dispatched, never a
  refusal recorded for it individually) contributes no cause to it and is never read as
  compilation/setup failure by omission; such a repetition, and a case whose planned count
  exceeds the observations it can produce for any other reason, resolve to inconsistent run as
  the order's residue.
- Row 8: rows 5-7 all false (fully observed), `effects_observed[]` holds an undeclared effect,
  and some version's three repetitions are not byte-identical.
- Row 9: row 8's predicate with both versions internally stable, so (c) is null.
- Row 10: fully observed, no undeclared effect, and some version's three repetitions are not
  byte-identical.
- Row 11: fully observed, no undeclared effect, both versions internally stable, and all nine
  `old`x`new` pairs byte-equal.
- Row 12: row 11's predicate with at least one of the nine pairs unequal.

An undeclared effect never nulls or outranks (c): when it coincides with a non-null (c) cause
the (c) row wins — rows 5-7 whenever an observation is incomplete, row 8 when the case is
fully observed but internally unstable — and row 9, the effect row with a null (c), applies
only when (c) is null.

- `CRR-V0-007`: Unexecuted, interrupted and unobserved cases stay distinct.
  - (a) Never launched: a case refused at validation (`CRR-V0-001`), at a pre-launch bound or
    digest check (`CRR-V0-005`(a),(c)), by `CRR-V0-003`(c), or because `CRR-V0-005`(d)'s
    task-entry budget expired before its first launch, and a case whose `Cmd.Start` itself
    failed (`internal/procgroup/process.go:260-263`, returning before `Started` is set at
    `internal/procgroup/process.go:265`) and no earlier repetition of the case set `Started`, MUST be recorded `NOT_RUN` with
    its reason and zero records — never omitted, never a pass, whatever its `accepted_by` value.
    That qualifier is load-bearing: a pre-launch failure on repetition two or later, after an
    earlier repetition of the same case already set `Started`, is not `NOT_RUN` at all but (e)'s
    row 7 and `FAIL`, since (d) defines `NOT_RUN` as no repetition ever setting `Started`.
    Cases that are `NOT_RUN` carry different (c) states per (e): validation,
    any digest mismatch and a failed `Cmd.Start` are compilation/setup failure, a 005(a)
    pre-launch bound is resource exhaustion, 003(c) is missing sandbox, and a case never
    launched because its task-entry budget expired (005(d)) is timeout. Later manifest entries
    are unaffected except by cancellation of the outer caller context.
  - (b) Zero records is not a signature of `NOT_RUN`, which is defined by no repetition ever
    setting `Started`: a launched case whose every repetition hit `CRR-V0-006`(a) also holds zero
    records and is still `FAIL`. Deriving execution status from record count is forbidden. So that
    zero records is explained rather than ambiguous, each case's runner envelope carries
    `unobserved_launches` (integer): the count of repetitions that set `Started`
    (`internal/procgroup/process.go:265`) and ended with `CRR-V0-003`(d)'s `ExitObserved`
    false (`WaitCompleted` false at `internal/procgroup/process.go:361-362`, or a nil `ProcessState` at `internal/procgroup/process.go:385`); the flag, not
    `ExitStatus` == -1, is what the counter reads.
    A failed `Cmd.Start` never sets `Started` and never increments it, so a case refused
    that way carries `unobserved_launches` == 0. The envelope also carries `launches` (integer):
    the count of this case's repetitions that did set `Started` (`internal/procgroup/process.go:265`), of which
    `unobserved_launches` is the `ExitObserved`-false subset, so `launches` == 0 exactly when the
    case is `NOT_RUN` per (d). `launches`, `unobserved_launches`, `case_inconclusive_state`
    (`CRR-V0-006`(c)), `reason` (`CRR-V0-003`(d)'s `Reason`) and `case_effects_observed[]`
    (`CRR-V0-006`(d)'s accepted amendment) are
    envelope fields, never `CTR-V0-012` record fields. The labelled
    accepted amendment at `compat-trial-v0.md:64` requires a runner-envelope count of launched
    repetitions whose exit was never observed but names no field for it; `unobserved_launches`
    is this spec's name for the counter the amendment requires.
  - (c) Post-launch overflow: output tripping `CRR-V0-005`(a),(b) after the process started is NOT
    `NOT_RUN`. Records already produced are preserved per `CTR-V0-012` (`compat-trial-v0.md:63`),
    the case is `FAIL` with resource exhaustion, outcome `withheld`, never truncated-byte
    equality. The abort is scoped to that case alone: its remaining repetitions are never
    launched and stay part of the same `FAIL` case rather than becoming `NOT_RUN` cases of their
    own, and every later case is launched and adjudicated normally. Only the expired cumulative
    budget (`CRR-V0-005`(d)) leaves later cases never launched.

## Non-goals

Executing anything but the runner's own synthetic fixtures. Per `CTR-V0-010`
(`docs/specs/compat-trial-v0.md:54`), until the `E:99` host profile is qualified "external
execution remains unavailable; read-only inspection and owned synthetic runner tests may
continue" — a fresh scratch `cwd` is not a containment profile, so replaying real candidate
binaries is out of scope. Also out: symlink resolution or any other beneath-scratch proof for
`CRR-V0-001`(f)'s effect paths — the containment there is lexical classification only, and full
path-escape containment stays a v0 non-goal with no sandbox claim; a fixture registry closes the
executable surface instead (`CRR-V0-001`(g)). Also out: scoring commits before
`CTR-V0-004`/`005`/`009` resolve;
reusing `conformance/perf-v0`'s `runSample`; and reusing `conformance/cli-parity-v0` *unchanged*
— `CRR-V0-003`(d)'s mechanical move of `process.go`, `process_posix.go`, `process_other.go` and
`process_test.go` into `internal/procgroup`, plus that package's four observation fields,
non-nil `Cmd.Env` and `Adjudicate` seam, is a required change, not a wrapper this runner may add
on top. That move is the one change permitted to `conformance/cli-parity-v0`: no behaviour
change beyond (d)'s named additions, no file other than those four leaves it, `main.go`,
`runner.go`, `fixture.go` and the rest stay `package main` there with only their call sites
rewritten against the exported identifiers, and the package MUST keep building and producing
byte-identical parity results against the extracted package.
`CTR-V0-012`'s record shape and `exit` union are not
amended; the definition of *executed* is amended by the amendment in
`docs/specs/compat-trial-v0.md`, accepted by decision 0052.

## Failure modes

| Failure | Consequence if unhandled |
|---|---|
| A well-formed digest never recomputed over the bytes | A changed executable replays as pinned |
| A stream's bound checked against the sum, or output truncated | Different outputs compare equal |
| The 120 s budget checked between repetitions | A repetition runs past the deadline |
| A third `exit` value added to `CTR-V0-012`'s union | A frozen record shape silently widened |
| Execution status inferred from record count | A launched, unobserved case reads as never run |
| Outcome or inconclusive state folded into `PASS`/`FAIL` | An inconclusive case reads as a score |
| An observed undeclared effect filed `NOT_PRODUCED` | A real write reads as a denied one |

### Process supervisor error codes

`internal/procgroup` sets the kebab-case `processError` codes below on `Observation.Err` (decision
0100). Each row cites the first emitting site and states only the condition checked there. After
start, `processRunError` joins every code whose condition holds, so one observation can carry
several.

| Code | First emitting site | At the cited site |
|---|---|---|
| `process-after-start-failed` | `internal/procgroup/process.go:400` | the `AfterStart` hook returned an error; joined to any run error |
| `process-before-stop-failed` | `internal/procgroup/process.go:397` | the `BeforeStop` hook returned an error; joined to any run error |
| `process-cancelled` | `internal/procgroup/process.go:219` | the context is already done before the command is built; after start it is joined again when the context finished first |
| `process-cleanup-failed` | `internal/procgroup/process.go:566` | termination, group cleanup, or quiescence reported an error |
| `process-exit-observation-failed` | `internal/procgroup/process.go:547` | the exit observer reported an error |
| `process-output-overflow` | `internal/procgroup/process.go:563` | output overflowed and the spec overflow policy is `OverflowFail` |
| `process-pipe-drain-failed` | `internal/procgroup/process.go:569` | the output pipes were not fully drained or draining reported an error |
| `process-pipe-failed` | `internal/procgroup/process.go:247` | creating the stdout or stderr pipe failed |
| `process-platform-unsupported` | `internal/procgroup/process.go:203` | `processPlatformSupported` is false; descendant cleanup is recorded unsupported and partial |
| `process-spec-invalid` | `internal/procgroup/process.go:197` | the context is nil, or `normalizeProcessSpec` refused the spec |
| `process-start-failed` | `internal/procgroup/process.go:262` | starting the command failed |
| `process-timeout` | `internal/procgroup/process.go:560` | the run timer fired before exit was observed |
| `process-wait-failed` | `internal/procgroup/process.go:554` | wait returned an error that is not an `*exec.ExitError` |
| `process-wait-timeout` | `internal/procgroup/process.go:550` | waiting for the process did not complete |

## Acceptance evidence and traceability

`tools/compat-trial` and `internal/procgroup` exist (see the Agent digest), but no row below is
marked passed. Each row names the fixture and the one assertion a wrong runner fails;
a filename alone discharges nothing. Helpers and fixtures live in `tools/compat-trial/testdata`.
`CTR-V0-002` freezes `argv`/`stdin`/`cwd`/`env` identically across a version's three repetitions,
so no descriptor-visible signal carries a repetition ordinal; a fixture below that must behave
differently on one named repetition (e.g. "repetition 2 only") reads that ordinal from a
runner-internal, fixture-only side channel outside the frozen `env` allowlist — a counter file at
a fixed path outside every repetition's fresh scratch copy, incremented once per invocation and
read by the helper before it acts — the same category of fixture-only mechanism `CRR-V0-003`(c)'s
`RequireDescendantCleanup` already establishes for a runner-internal test input no descriptor
field carries. A row asserting `Adjudicate`'s own return values (`procgroup.Adjudicate(...)`)
calls it directly over synthetic, hand-constructed `procgroup.Observation` values, never a real
process race: no row below depends on a live process's scheduling to land a specific
`ExitObserved`, `TimedOut` or `Cancelled` combination where a synthetic call proves the same
thing deterministically.

| Requirement | Fixture and the assertion that rejects a wrong runner |
|---|---|
| CRR-V0-001(a) | `valid_minimal` loads AND `manifest_unknown_field`, `tasks_renamed_field`, `gold_dropped_field`, `resource_profile_unknown_field` each reject |
| CRR-V0-001(b) | `source_empty_sha256`, `executable_short_sha256`, `stdin_uppercase_sha256`, `fixture_missing_sha256`, `executable_sha256_65_chars`, `source_sha256_nonhex` each reject |
| CRR-V0-001(c) | 64-hex but wrong `old.executable_sha256`, wrong `new.executable_sha256`, wrong `source` and wrong `snapshot_sha256`: each rejects, launches == 0 |
| CRR-V0-001(d) | `build_identity_wrong_go_version`, `identity_mismatch` (`old.build_identity` != `new.build_identity`), `repetitions_not_three` (2 and 4), `cwd_outside_scratch`, `snapshot_noncanonical`, `input_bytes_not_64kib`, `output_bytes_per_stream_not_1mib`, `fixture_entries_not_100`, `fixture_bytes_not_10mib`: each rejects |
| CRR-V0-001(e) | counting launcher stub: launches == 0 and `Started` false after each rejection |
| CRR-V0-001(f) | `expected_effects_outside_allowed_effects` (a `tasks[].expected_effects` element on a path no `resource_profile.allowed_effects` element names), `effect_kind_not_permitted` (`allowed_effects` `[{path:"a.txt",kind:"delete"}]` with `expected_effects` `[{path:"a.txt",kind:"create"}]`, the path permitted but not the create kind) and `descriptor_url_form_effect` (an element in URL form): each refused before launch as `descriptor-invalid` — `NOT_RUN`, compilation/setup failure, `Started` false, zero records, (e) row 2, never row 1. `effect_kind_permitted` (`allowed_effects` `[{path:"a.txt",kind:"create"}, {path:"a.txt",kind:"delete"}]` with `expected_effects` `[{path:"a.txt",kind:"delete"}]`) loads, so a runner comparing bare paths without matching `kind` fails the pair above. Revision 12 struck `effect_bang_in_path` (`foo!mode` as a literal filename) and `effect_double_suffix` (`foo!delete!mode`): the retired suffix syntax made a literal `foo!mode` filename unexpressible without an escape rule, so no fixture can construct that rejection under the `{path, kind}` grammar |
| CRR-V0-001(g) | `fixture_not_registered` (a syntactically valid, correctly-digested `executable_sha256` naming a binary absent from the fixture registry): refused pre-launch as `fixture-not-registered`, distinct from a (c) digest mismatch — `NOT_RUN`, compilation/setup failure, `Started` false, zero records, (e) row 2. `malformed_unregistered_executable_sha256` (a 63-hex `old.executable_sha256`, which no registry entry can match either): refused as `descriptor-invalid`, not `fixture-not-registered`, pinning (b)-before-(g) ordering |
| CRR-V0-002(a) | parent `CORVINT_LEAK=1`; all six repetitions' captured stdout == allowlist, `CORVINT_LEAK` absent in each |
| CRR-V0-002(b) | `env_missing_tz`/`_lc_all`/`_conflicting_tz` reject; `valid_minimal` sees both |
| CRR-V0-003(a) | fork helper printing both PIDs: descendant's process-group id == leader pid |
| CRR-V0-003(b) | grandchild helper: grandchild gone, side-effect file absent 50 ms after exit |
| CRR-V0-003(c) | `setsid` helper run with the fixture-only `Spec.RequireDescendantCleanup` set by the test (`internal/procgroup/process.go:206-210`), the only input that reaches this refusal: `NOT_RUN`, missing sandbox, `Started` false, zero records |
| CRR-V0-003(d) | `internal/procgroup` exists, holds `process.go`, `process_posix.go`, `process_other.go` and `process_test.go`, and exports `Observation` (the renamed `processObservation`) carrying `ExitObserved`, `Signal`, `StdoutOverflow` and `StderrOverflow`, plus `Adjudicate` and a non-nil `Cmd.Env` for an empty `spec.Env`; the moved tests pass in their new package, `conformance/cli-parity-v0` builds importing the package and its parity results are byte-identical to the pre-extraction run; a `SIGKILL`ed helper reports `ExitObserved` true with `Signal` == `killed`, and `TestRunProcessShutdownDeadlineLeavesExitUnobserved` substitutes never-ready receive channels at the `waitProcessExitUnreaped`/`command.Wait()` observation call sites and reports `ExitObserved` false, `WaitCompleted` false and `ExitStatus` -1 without a live scheduler race — the signalled and unobserved cases never share one interpretation |
| CRR-V0-003(e) | `TestJoinedRunDoesNotClaimOwnedGroupCleanup`: a joined `descendant-hold` run times out with its descendant still alive and reports `OwnedProcessGroupCleanup` false, `ancestor-process-group` and `PARTIAL` |
| CRR-V0-004 | byte-identical helper, old == new: `compatible`, `PASS`, null state, six records |
| CRR-V0-004 | nanosecond-timestamp helper: `instability`, six records, the unstable version's three mutually unequal |
| CRR-V0-004 | helper whose `old` repetition 2 alone differs from `new`: `instability`, never `different` |
| CRR-V0-005(a) | at-bound accepted; 65537 stdin, 1 MiB+1 out, 101 entries, 10 MiB+1 refused |
| CRR-V0-005(b) | stdout at bound with stderr at bound+1, and the mirror: both refused by stream |
| CRR-V0-005(a) | every launch's `Spec.OutputLimit` == 1 MiB and `Spec.InputLimit` == 64 KiB, each set explicitly and never left zero, so neither inherits the 16 MiB `DefaultOutputLimit`/`defaultProcessInputLimit` `normalizeProcessSpec` would substitute (`internal/procgroup/process.go:21-22`, `internal/procgroup/process.go:439-449`) |
| CRR-V0-005(b) | 1 MiB stdout and 1 MiB stderr in one repetition: accepted and compared |
| CRR-V0-005(c) | pinned stdin echoed byte-identical; copy mutated after pinning refused early |
| CRR-V0-005(d) | helper launched at 119 s: terminated by 121 s, group gone, rest `NOT_RUN` |
| CRR-V0-005(d) | every launch's `Spec.ShutdownTimeout` is positive and <= 1 s, never left zero and never the 2 s `defaultProcessShutdownLimit` `normalizeProcessSpec` would substitute (`internal/procgroup/process.go:23`, `internal/procgroup/process.go:433-437`) |
| CRR-V0-006 | `SIGKILL`, `Start`-failure, `different` and timeout helpers each match one (e) row |
| CRR-V0-006(e) row 6 | `procgroup.Adjudicate(6, none, …)` over six `Started && TimedOut` completed observations: `FAIL`, row 6, timeout, `Launches` == 6 — never an aborted-case `Launches` == 2. A per-process-timeout helper run end to end MUST also show repetition 3 still launched after repetition 1's timeout, never a case aborted on the first timeout |
| CRR-V0-006(a) | no `exit` outside integer/`signal:`; an unobserved repetition emits no record |
| CRR-V0-006(c) | overflow helper emitting 1 MiB+1 at 200 ms, inside the 10 s ceiling: `case_inconclusive_state` == resource exhaustion, never timeout |
| CRR-V0-006(c) | 30 s helper whose `old` repetitions already disagree byte-wise, killed at the 10 s ceiling: `case_inconclusive_state` == timeout, never inconsistent run |
| CRR-V0-006(e) rows 8, 9 | undeclared-write helper in two variants — nanosecond-timestamp (self-disagreeing) and byte-identical (stable): both `FAIL`, `withheld`, the write listed in `effects_observed[]` and `case_effects_observed[]` with basis `undeclared-observed`, six records; `case_inconclusive_state` == inconsistent run for the self-disagreeing variant and null for the stable one |
| CRR-V0-006(d) | single-repetition-write helper: an undeclared write performed on repetition 2 of the six only, every other repetition leaving the scratch tree unchanged — `FAIL`, `withheld`, that write listed in `effects_observed[]` (basis `undeclared-observed`), six records. A runner that sets the case's undeclared-effect fact only when all six repetitions show the effect reports `PASS` and fails the row |
| CRR-V0-006(d) | declared-but-absent helper: `tasks[].expected_effects` declares `{path:"a.txt",kind:"create"}` and no repetition ever writes it — `PASS`, six records, and `effects_observed[]`/`case_effects_observed[]` hold an entry for `a.txt` with status `NOT_PRODUCED` and basis `declared-absent`. A runner emitting no entry, or one failing the case for the absence, fails the row |
| CRR-V0-006(d) | undeclared-deletion helper unlinking a path present in the pinned snapshot whose declared entries carry no `{path, kind: delete}`: `FAIL`, `withheld`, the delete listed in `effects_observed[]` with basis `undeclared-observed`; the same helper with `{path, kind: delete}` declared is `PASS` and lists it as `PRODUCED` with basis `declared-observed` |
| CRR-V0-006(d) | undeclared-mode helper chmodding a path whose declared entries carry no `{path, kind: mode}`: `FAIL`, `withheld`, the mode change listed in `effects_observed[]` with basis `undeclared-observed`; the same helper with `{path, kind: mode}` declared is `PASS` |
| CRR-V0-006(e) rows 1-4 | `procgroup.Adjudicate(6, refusal, nil)` called directly with an empty observation slice: `missing-sandbox` yields row 1, `descriptor-invalid` row 2, `resource-bound` row 3, `budget-expired` row 4 — four distinct `CaseInconclusiveState` values from the refusal input alone, with no `runProcess` call for any of them. Row 4's second arm too: `procgroup.Adjudicate(6, none, []Observation{{Started: false, Cancelled: true}})` is row 4, `NOT_RUN` with timeout, never row 2's compilation/setup failure |
| CRR-V0-006(e) rows 8-12 | `procgroup.Adjudicate(6, none, …)` over six `procgroup.Observation` values completed per (d) (`Started`, `ExitObserved`, `PipesDrained` and `OwnedProcessGroupCleanup` all true): the returned `Adjudication` has `FullyObserved` true with `Status`, `Outcome` and `CaseInconclusiveState` all zero, `RecordCount` == 6, `Launches` == 6, `UnobservedLaunches` == 0 — a runner that returns a `PASS`/`FAIL` status here, or that leaves `FullyObserved` false, fails the row |
| CRR-V0-006(a), CRR-V0-007(b) | `procgroup.Adjudicate(6, none, …)` over a mixed set: three `Started` observations of which two are `ExitObserved` true and completed and one is `ExitObserved` false, and three never-`Started` ones — `Launches` == 3, `UnobservedLaunches` == 1, `RecordCount` == 2, `FAIL`, row 7. A runner deriving `RecordCount` from `planned`, from `len(observations)` or from `Launches` fails it |
| CRR-V0-006(e) row 7, `planned` | `procgroup.Adjudicate(6, none, …)` over exactly three observations, all completed per (d): `FAIL`, row 7, inconsistent run, `Launches` == 3 — the three unpassed planned repetitions are incomplete because `planned` is 6. A runner reading `len(observations)` in place of `planned` returns `FullyObserved` here and fails the row |
| CRR-V0-006(e) row 7, CRR-V0-007(b) | `procgroup.Adjudicate(6, none, …)` called directly, no process launched, over six synthetic `procgroup.Observation{Started: true, ExitObserved: false, TimedOut: false, Cancelled: false}` values: the returned `Adjudication` carries `Status` `FAIL`, `CaseInconclusiveState` inconsistent run, `RecordCount` == 0, `Launches` == 6, `UnobservedLaunches` == 6 and `FullyObserved` false, never `NOT_RUN` — every one of those read off the returned struct's exported fields, not off the fixture. No timing-dependent process fixture is used for this row: a real 1 ns `ShutdownTimeout` stub is a scheduler race at `internal/procgroup/process.go:361-362` against `waitForProcessEvent`'s expired-deadline branch (`internal/procgroup/process.go:491-499`) and proves nothing repeatably |
| CRR-V0-006(e) row 7, (c) subcases | `procgroup.Adjudicate(6, none, …)` over synthetic `procgroup.Observation` sets mixing one completed repetition with a later incomplete one: a never-`Started` `Cmd.Start` failure yields compilation/setup failure, an expired-budget refusal before `Started` (`internal/procgroup/process.go:217-220`) yields timeout, and `PipesDrained` or `OwnedProcessGroupCleanup` false yields inconsistent run — each `FAIL`, each row 7, and the returned `Adjudication`'s `UnobservedLaunches` counting only the `ExitObserved`-false repetitions while `Launches` counts every `Started` one |
| CRR-V0-007(a) | 001 and 005(c) refusals appear as `NOT_RUN` with a reason and zero records |
| CRR-V0-007(a) | unexecutable `old` binary, `Cmd.Start` fails: `NOT_RUN`, compilation/setup failure, zero records, `unobserved_launches` == 0 |
| CRR-V0-007(c) | counting stub, 1 MiB+1 after 100 ms on the second of six repetitions: that case `FAIL`, record kept, resource exhaustion, `withheld`, and `launches` == 2 for it (its remaining four never launch); the next case still `launches` == 6 and its status is not `NOT_RUN` |
| Promotion: scratch | helper creates with 0666/0777; observed modes `0644`/`0755` under umask 022 |
| Promotion: timeout | 30 s child killed at the 10 s ceiling, group gone, timeout state set |
| Promotion: effects | declared effects `PRODUCED`; an undeclared write is `FAIL` and listed |

## Rollout, rollback, compatibility

This spec now ships `tools/compat-trial` and `internal/procgroup` (see the Agent digest); delivery
status stays experimental because no acceptance row above is marked passed. Rollback is deleting
those two packages, this file, and its `docs/specs/README.md` and `docs/specs/INDEX.json` rows,
then regenerating the requirement index
with `script/gen-spec-requirements.sh > docs/specs/REQUIREMENTS.tsv`. That regeneration is a
mandatory part of *every* edit to this file, not of rollback alone, because each clause's line
number is recorded there and `Makefile:130`'s `spec-requirements-check` compares it byte for byte.
It implements the AT-10 slice of `docs/specs/compat-trial-v0.md` (`CTR-V0-001`-`003`,
`CTR-V0-010`, "as written") without adjudicating a label or naming the baseline; promote to
`implemented` only once `tools/compat-trial` exists and every row above passes `make gate`.
AT-10's third verify clause, "replay eligible Corvint changes" (`ROADMAP.md:350`), is not
discharged by this spec and stays open until the `E:99` host profile is qualified, because
Non-goals and `CTR-V0-010` (`docs/specs/compat-trial-v0.md:54`) keep replaying real candidate
binaries out of scope; only the runner's own synthetic fixtures are replayed here.
