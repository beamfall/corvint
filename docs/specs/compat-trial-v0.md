# Compatibility Trial V0

Owner: Russell Lewis
Date: 2026-09-04
Requirement prefix: `CTR-V0`
Intent status: accepted for CTR-V0-001..010 (decision 0052); CTR-V0-011/012 proposed
Delivery status: not-started
Authoritative inputs: `ROADMAP.md` (AT-09, AT-10), `docs/plans/EXCEPTIONAL-CORVINT-2026-09-04.md`
(`P`; resource profile, baseline, screen, cost/ratio rules), `docs/plans/EXCEPTIONAL-CORVINT-EXECUTION-2026-09-04.md`
(`E`; E00b split, qualified-containment rule), `AGENTS.md` invariants 2 and 4.

## Agent digest
- Claim: Freezes the compatibility replay experiment's descriptor, comparison policy, resource profile, labels, baseline and preregistered rules before a runner exists.
- Status: accepted for CTR-V0-001..010 (decision 0052); CTR-V0-011/012 proposed/not-started
- Exists: a coordinator-accepted ten-case developmental label registry and ten-case exclusion registry in `benchmarks/compat-trial-v0.json`; the experiment has not run.
- Blocked on: independent accepted-intent references, the baseline's five pinned artefacts, executable case descriptors, and qualified execution containment; no scored result exists.
- Read next: CTR-V0-001 through CTR-V0-005; CTR-V0-009; `benchmarks/compat-trial-v0.md`; `benchmarks/compat-baseline-v0.md`.

## Intent and scope

`ROADMAP.md:335-338` (AT-09) requires freezing "comparison semantics, resource profile, independent
labels, 20 development cases and strong differential baseline" and preregistering "the 40-case
screen and full-cost/undefined-ratio rules from the program," before AT-10 builds any runner. This
document freezes the policy and ten eligible pilot labels; the complete 20-case development corpus,
baseline artefact pins, and runs remain unproduced. It authors no code. AT-10 depends on this spec
(`ROADMAP.md:343`).

Affected user: the AT-10 runner owner and the owner adjudicating the eventual screen. Measurable job: a descriptor, comparison policy, resource profile and label schema stable enough that a runner built against them cannot silently redefine equality, cost or "solved" after seeing results. the execution plan item E00b (an internal planning record, not part of the public tree) scopes this freeze to compatibility cases and policy only, not admission repair, batching or checkpoint recovery.

## Requirements

- `CTR-V0-001`: Descriptor schema. The fields below are frozen; a runner MUST NOT add, rename or drop one without a spec amendment. Every source, executable and fixture reference is a `{path, sha256}` pair — no path is trusted without its digest.
  - `manifest`: `partition` (development/heldout/pilot), `source` (population description), `comparison_policy_ref`, `resource_profile_ref`, `baseline_ref`, `preregistration_ref`, `tasks[]`.
  - `tasks[]`: `id`, `kind`, `repo`, `base_commit`, `snapshot`+`snapshot_sha256` (fixture bundle), `source` (how found), `old{commit,executable,executable_sha256,build_identity}` and `new{...}` (paired binaries under test), `argv`, `stdin{path,sha256}`, `cwd`, `env` (selected, not inherited), `accepted_intent_ref`, `historical_observation_ref`, `expected_unknowns`, `expected_effects`, `forbidden_claims`, `gold_ref`.
  - `gold` (via `gold_ref`): `label`, `critical`, `provenance` — CTR-V0-004 governs its values.
  - `resource_profile` (via `resource_profile_ref`): `host_profile`, `repetitions`, `input_bytes`, `output_bytes_per_stream`, `fixture_entries`, `fixture_bytes`, `process_timeout`, `request_timeout`, `build_bounds`, `allowed_effects` — CTR-V0-003 governs its values.
  This reuses `tools/cw-trial`'s manifest/task shape (`tools/cw-trial/main.go:130`) rather than a second convention.
  - `build_identity` (per version) MUST be `{go_version, GOOS, GOARCH, CGO_ENABLED, build_flags}`; `go_version` MUST equal `go1.27.1` under `GOTOOLCHAIN=local`, `build_flags` MUST include `-trimpath`, and `old.build_identity` MUST equal `new.build_identity`. `env` is an allowlist — every unlisted variable MUST be cleared; `tools/cw-trial/main.go:1460` and `tools/cem-trial/main.go:901` explicitly pass `os.Environ()` (full parent-environment inheritance) and MUST NOT be reused unmodified. `cwd` MUST be inside the per-repetition scratch copy of the pinned snapshot, never a live checkout (`AGENTS.md` invariant 4). `snapshot_sha256` MUST digest a canonical tar of the snapshot (sorted paths, zeroed mtimes, mode preserved). Every repetition MUST run under `umask 022`, `TZ=UTC`, `LC_ALL=C`. `expected_effects` (declared scratch-file effects, `P:236`) is now part of `tasks[]` above.

- `CTR-V0-002`: Comparison policy v0. Equality is exact bytes of stdout, stderr and exit status between `old` and `new` on frozen `argv`/`stdin`/`cwd`/`env`, three repetitions per version in a fresh scratch location; disagreeing repetitions are `instability`, not a verdict (`P:238`, "Inconsistent repetitions are inconclusive"). A parsed-JSON explanation layer MAY report where a JSON-valued stream differs, but per `P:239` ("Explanation never silently changes the equality predicate") MUST NOT change the byte-equality verdict; a JSON-value comparator (object order, number semantics, duplicate keys, missing/null, array order) is a separate, later, named policy. Byte equality includes any trailing newline. Exit status is a non-negative integer or the literal `signal:<name>`; these MUST NOT collapse to one value the way `conformance/perf-v0/process.go` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 139-143) collapses a signalled process and an absent `ProcessState` to `-1`. Each version MUST be internally stable (byte-identical across its own three repetitions) before any cross-version verdict; adjudication compares all nine `old`x`new` repetition pairs — unanimous agreement is `compatible`, two internally-stable but disagreeing outputs is `different`, any internal instability is `instability`, not a verdict.

- `CTR-V0-003`: Resource profile, quoted verbatim from `P:237–241`: "64 KiB input, 1 MiB output per stream, 100 fixture entries and 10 MiB aggregate fixture bytes," refused/reported rather than truncated into an apparent equality; "fresh scratch locations" per repetition; "per-process timeout 10 seconds and cumulative request budget 120 seconds initially"; and, quoted from `P:244`, "no automatic network, model call, installation, repo edits, commits, merge or publishing" — declared effects are observed and reported; denial of undeclared effects is `NOT_PRODUCED` until the `E:99` host containment is qualified (`allowed_effects` in CTR-V0-001 is an explicit allowlist, never ambient). Every observation MUST record `GOOS`/`GOARCH` (CTR-V0-012).

- `CTR-V0-004`: Case labels. Every case carries `label ∈ {defect, intentional, compatible}`,
  `critical ∈ {true, false, undefined}`, `provenance`, and `accepted_by`, which is the literal string
  `NOT_PRODUCED` until an owner or delegated coordinator — never the case's author or a runner —
  independently accepts it. No case may enter a scored run with `accepted_by: NOT_PRODUCED` or
  `critical: undefined`. Decision 0052 delegates this pilot's labels to the coordinator;
  `benchmarks/compat-trial-v0.json` freezes ten eligible read-only CLI cases as 6 defect, 1
  intentional, and 3 compatible, each with an immutable commit, parent, evidence references,
  provenance, and `accepted_by: coordinator:decision-0052`. The other ten candidates are frozen in
  that file's exclusion registry with reasons. This underfilled, imbalanced ten-case set is a
  protocol-debug pilot only; it neither satisfies nor replaces `P:257`'s 20-case 10/5/5 development
  mix or CTR-V0-006's separate 40 new cases. None of these twenty Corvint-history candidates is a
  held-out case. Eligibility covers an implementation or dependency on a read-only CLI verb's
  reachable path: a positive predicts different old/new bytes or exit status, while a compatible
  control predicts exact equality despite the implementation change. The appendix and candidate-side
  tests support developmental label discovery but are not independent accepted-intent authority.
  Every pilot `accepted_intent_ref` is therefore `NOT_PRODUCED`, and no case may be scored until a
  case-specific pre-run owner/spec/test-oracle artefact independently states its intent predicate.

- `CTR-V0-005`: Baseline. Quoted from `P:269–271`, the primary comparator is "a capable agent
  assisted by a straightforward differential harness," with ordinary reduction and test tools and
  equal snapshots, seeds, access, build availability, and total budget across arms. The accepted
  coordinator call in `benchmarks/compat-baseline-v0.md` defines that harness as supplying `git diff`,
  `git bisect run`, `go test`, and an exact byte differ, with no Corvint access. After an independent
  reviewer improves it on the ten pilot cases, five artefacts are frozen by path and sha256: harness,
  prompt, dated model-identity record, tool allowlist, and budget configuration. Until all five are
  produced, the baseline is **specified but not frozen** and every scored case is `NOT_RUN`. Arm order
  is randomized from a recorded seed and separate operators prevent contamination. This comparator
  remains operator-run outside the replay binary and does not add a model to Corvint.

- `CTR-V0-006`: Preregistered 40-case screen. Quoted verbatim from `P:273`: "Freeze 40 new cases from at least four unrelated projects: twenty independently labelled positive incompatibilities, ten intentional differences and ten compatible controls." Labels are withheld from the runner; unsupported cases retained in the denominator. Per `P:286`, the pass rule is "at least sixteen of twenty positives independently solved (80%), zero intentional/compatible controls falsely classified as defects, no extra critical miss relative to the primary baseline, positive completion at least as high as that baseline, and `C_Corvint / C_baseline <= 0.70`"; unsupported/inconclusive cases count as unsolved. A later superiority claim additionally needs, per `E:65`, "an upper one-sided 95% confidence bound on the active-cost ratio no greater than 0.70, with completion non-inferiority margin at most five percentage points" and zero additional critical misses; without a sample size and clustered analysis sized for that claim in advance, the trial stays screening-only.

- `CTR-V0-007`: Cost and ratio rules, quoted short from `P:277–284`. Cost: "`C_arm = sum(active_minutes for all positive attempts) / count(independently solved positives)`" (`P:282`). "Unsolved attempts remain in the numerator"; "zero solved positives gives infinite cost" (`P:282`). "Missing timing invalidates the cost comparison rather than becoming zero" (`P:284`). "Zero baseline cost or otherwise undefined ratios are inconclusive" (`P:284`). Zero-solved/undefined bootstrap draws "are never silently discarded or replaced with epsilon denominators" (`P:284`).

- `CTR-V0-008`: Inconclusive states stay distinct from production failures. Quoted from `P:243`: "Timeouts, compilation/setup failures, resource exhaustion, skipped tests, missing sandbox and inconsistent runs are distinct inconclusive states. They are not behavioral counterexamples unless the independently accepted predicate explicitly concerns bounded completion." A scorer MUST record which of these six an inconclusive case hit; collapsing them into one `INCONCLUSIVE` bucket is not this policy.

- `CTR-V0-009`: Preregistered rubric, budgets, and scorer decisions. The following are coordinator
  calls delegated by decision 0052; they are frozen before any result and are not empirical claims.
  - **Criticality.** Decide from `accepted_intent_ref` and captured old/new bytes, never from either
    arm's answer and never by assuming the candidate side is the faulty side. `critical: true` means
    failing to detect the difference could make a consumer act on an undisclosed false statement,
    under at least one of: (i) either side cites content absent at that side's pinned revision; (ii) a
    verdict, gate, authority, or coverage field is missing or contradicts the truth at that side's
    revision; (iii) a refusal/unknown required by AGENTS.md invariant 2 is replaced or omitted in
    favour of an unqualified answer. `critical: false` means a difference matching none of (i)–(iii),
    including ordering, formatting, escaping, help, counts, timing, added-field-only JSON growth that
    does not correct (ii), or performance. `critical: undefined` means accepted intent cannot decide
    the predicate and blocks that case before scoring. Only `critical: true` positives feed the no-
    extra-critical-miss gate. Relabeling after a scored run is a spec amendment, never a scorer
    decision.
  - **Per-case budgets.** `repetitions = 3`; 64 KiB input; 1 MiB output per stream; 100 fixture
    entries; 10 MiB fixture aggregate; 10 seconds per process; `request_timeout = 120 seconds` per
    `tasks[]` entry; build wall time 300 seconds per version; one attempt per case; 45 human active
    minutes per positive attempt per arm. Build and failed-attempt minutes remain in arm cost.
    `allowed_effects = []` for the ten pilot cases. A repeated execution of the same frozen descriptor
    is an amendment, not another attempt.
  - **Scorer.** A solved positive is a `defect` whose classification matches gold under independent
    adjudication and whose disagreement is reproduced within the frozen budget. Every other positive,
    including each CTR-V0-008 inconclusive state and unsupported input, is unsolved and remains in the
    denominator and cost numerator. A critical miss is an unsolved `critical: true` positive; Corvint
    has an extra critical miss iff its integer count exceeds the baseline's. Active time accumulates
    in whole seconds; compute `C_Corvint / C_baseline` as an exact rational and round to three decimals
    only for display. `<= 0.70` and equal completion pass their respective tie boundaries. Zero
    baseline cost, missing timing, zero solved positives, or any otherwise undefined ratio is
    inconclusive and cannot pass.
  - **Intervals and claim scope.** The ten-case pilot reports per-case states and counts with no
    confidence interval or superiority claim. The 40-case screen uses 10,000 paired cluster-bootstrap
    resamples clustered by change, fixed seed `20260904`, and an upper one-sided 95% cost-ratio bound;
    undefined draws stay `+inf`, and if more than 5% are undefined the bound is undefined. The 40-case
    screen remains screening-only because no sample-size justification establishes the five-point
    completion non-inferiority claim in `E:65`.

- `CTR-V0-010`: Containment and harness reuse. Replay stays local; per `E:99`, external execution needs a host profile that "bounds input/output/wall time, denies unrequested network and writes, and proves interruption leaves no descendants, including process-group/session escape"; until qualified, "external execution remains unavailable; read-only inspection and owned synthetic runner tests may continue." No such profile is qualified today.
  `internal/procgroup` is the primary reusable containment and MUST be what a runner extends: `Setpgid` at process start (`internal/procgroup/process_posix.go:31`), non-racing exit observation (`internal/procgroup/process_posix.go:38-49`) and group termination (`internal/procgroup/process_posix.go:62-77`). The current supervisor leaves umask to its caller (`internal/procgroup/process.go:219-225`); the replay profile still requires `umask 0o022` per CTR-V0-001. Its `setsid`-escape case remains `PARTIAL` (`TestRunProcessSetsidEscapeQualificationIsPartial`), so full descendant containment is not qualified. `tools/retrieval-bench/procgroup_unix.go:14-16` remains a second `Setpgid`+`Cancel`+`WaitDelay` pattern.
  Three other existing harnesses each have a named gap and MUST NOT be reused as-is:
  - `tools/cw-trial` gives partition/source/tasks schema (`tools/cw-trial/main.go:130`), Codex/script dispatch with resumable checkpoints, and bounded capture/timeouts with shared owned-group cleanup in `runCommand` (`tools/cw-trial/main.go:1432`; proposed CWT-V0-014). It still lacks paired binaries, frozen CLI replay, byte comparison and escaped-group/session containment. It supplies EOF stdin and full inherited environment rather than pinned descriptor stdin and an environment allowlist.
  - `tools/cem-trial` gives isolated control/treatment dispatch and checkpoints, and bounded capture/timeouts with shared owned-group cleanup in `runCommand` (`tools/cem-trial/main.go:873`; proposed CRT-V0-011). It still lacks paired CLI replay, byte verdicts and escaped-group/session containment. It likewise supplies EOF stdin and full inherited environment.
  - `conformance/perf-v0` gives pinned corpora and task command/argv/stdin (`conformance/perf-v0/manifest.go` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 328,346-348)), bounded timed dispatch with owned-process-group cleanup and manual pipe drains (`conformance/perf-v0/process.go` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 47-178); `process_posix.go:21-133`), stdout/stderr/exit comparison (`conformance/perf-v0/runner.go` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 565)), and JSON-difference explanation (`conformance/perf-v0/diagnose.go` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 149)). It still lacks generic old/new native binaries, resumable checkpoints and `setsid` containment, assumes a Python oracle, and exposes neither the observation envelope nor adjudication seam the replay runner requires; this measurement-specific `package main` implementation MUST NOT be reused as the replay runner.

- **CTR-V0-011:** Proposed; unaccepted and does not authorize implementation. Changed-executable and contamination refusal. A run whose observed `old` or `new` executable sha256 differs from the descriptor's `executable_sha256` MUST be refused before scoring, not scored as a result. Gold MUST NOT be derived from any run of `new`; `gold_ref.provenance` MUST name gold's source, and the scorer MUST refuse gold whose provenance names the candidate (`new`) as its source (`P:258`, "candidate cannot author baseline").

- **CTR-V0-012:** Proposed; unaccepted and does not authorize implementation. Observation record shape. Every executed repetition MUST produce one record: `{descriptor_sha256, version (old|new), repetition, started_at, wall_ms, exit (integer or signal:<name>), stdout_sha256, stderr_sha256, stdout_bytes, stderr_bytes, effects_observed[], inconclusive_state (one of the six named in CTR-V0-008, or null), goos, goarch}`. A runner MUST NOT add, rename or drop a field without a spec amendment.
  Accepted amendment (AT-10, decision 0052): a repetition is *executed* only when its exit is observed; a launched repetition whose exit is never observed produces no record and is counted by the runner envelope. End of accepted amendment.

## Non-goals

Building `tools/compat-trial` or any runner code; fabricating paths or digests for baseline artefacts
that have not been produced; qualifying the external-execution host profile; a JSON-value comparator
beyond raw bytes; running the pilot or 40-case screen. `script/tests-for-change.sh` is absent from
this repository (Beamfall's selector, `docs/specs/verification-planner-observer-v0.md:60`) and
supplies nothing to reuse or critique here.

## Failure modes

A case entering a scored run with `accepted_by: NOT_PRODUCED` or `accepted_intent_ref: NOT_PRODUCED`; a runner redefining "solved," "critical" or "cost" after seeing results instead of reading this document; a JSON-explanation layer silently changing an equality verdict (CTR-V0-002 forbids this); external replay attempted before the E:99 host profile is qualified; treating an inconclusive state (CTR-V0-008) as a pass, fail, or behavioral counterexample; extrapolating the enriched 20/10/10 screening mix into a deployment-cost claim (`P:290`).

## Acceptance evidence and traceability

No requirement below has an implementation yet — this document is the freeze, not the runner.

| Requirement | Evidence |
|---|---|
| CTR-V0-001 | this spec's field list; `tools/cw-trial/main.go:130` precedent |
| CTR-V0-002 | `P:238–239` |
| CTR-V0-003 | `P:237–241` |
| CTR-V0-004 | accepted pilot and exclusion registries in `benchmarks/compat-trial-v0.json`; Appendix below; `P:257` |
| CTR-V0-005 | comparator contract and explicit `NOT_PRODUCED` pins in `benchmarks/compat-baseline-v0.md`; `P:269–271` |
| CTR-V0-006 | `P:273`, `P:286`, `E:65` |
| CTR-V0-007 | `P:277`, `P:282`, `P:284` |
| CTR-V0-008 | `P:243` |
| CTR-V0-009 | frozen rules above and `benchmarks/compat-trial-v0.md`; `P:277–288`, `E:65–67`; decision 0052 coordinator call |
| CTR-V0-010 | `E:99`; `conformance/cli-parity-v0`, `tools/cw-trial`, `tools/cem-trial`, `conformance/perf-v0` citations above |
| CTR-V0-011 | `P:258`; not implemented |
| CTR-V0-012 | CTR-V0-008 (inconclusive states); not implemented |

## Appendix: accepted pilot labels and exclusions

Decision 0052 delegates these developmental labels to the coordinator. They are preregistered calls
backed by the immutable commit/path discovery evidence below; they are not observed runner results
or independent accepted-intent predicates. The JSON registry contains full commit and parent
identities, provenance, exclusion reasons, and the fail-closed `NOT_PRODUCED` intent state.

| Commit | Pilot label | Critical | Evidence |
|---|---|---:|---|
| `dfd1d04` | intentional | false | OCM help surface (`cmd/corvint/help.go`, `cmd/corvint/help_test.go` at the commit) |
| `0c9421a` | defect | false | Unicode escaping changes emitted bytes (`cmd/corvint/main.go`, `cmd/corvint/main_test.go`) |
| `21a147c` | defect | true (rubric i) | relative/star-import resolution (`internal/liveverify/pyresolve/pyresolve.go`, `cmd/corvint/prove_python_test.go`) |
| `4ed3a3d` | defect | true (rubric ii) | result rows gain kind/id binding (`cmd/corvint/prove.go`, `cmd/corvint/prove_mutate_test.go`) |
| `abcbcff` | defect | true (rubric i) | generated marker must start a line (`internal/contextindex/index.go`, `internal/contextindex/generated_header_test.go`) |
| `1e51fe7` | defect | true (rubric ii) | disclosure list no longer outbids evidence (`internal/contextindex/receipt.go`, `internal/contextindex/receipt_budget_test.go`); unsupported if no frozen trigger can be produced |
| `4f3dc7e` | compatible | false | profiling-disabled output preserved (`cmd/corvint/taskcontext.go`, `cmd/corvint/perf_bench_test.go`) |
| `afb0132` | compatible | false | term-table parallelism preserves ordering/output (`internal/contextindex/termtable.go`, `internal/contextindex/termtable_test.go`) |
| `130a667` | compatible | false | snapshot/status overlap is an internal allocation change (`internal/contextindex/snapshot.go`) |
| `34a3049` | defect | true (rubric i) | auxiliary-symbol pruning changes selected evidence (`internal/contextindex/eval_query.go`, `internal/contextindex/eval_query_precision_test.go`) |

Excluded before partitioning: `fa0ee11` (writer-only), `f5c0216` (tests-only), `32b14f3` and
`3530e89` (harness-event internals), `b5825ed` (multi-purpose 51-file wave), `2d7e318` (requires a
multi-command make gate), `49dd289` and `4e1acae` (mutation execution), `58939fd` (observation
writer), and `74afb8a` (dogfood-record writer errors). They are not a holdout.

## Rollout, rollback, compatibility

This spec ships no code and touches no existing package; deleting it and its README/index rows is a
full rollback with zero blast radius. It gates AT-10 (`ROADMAP.md:343`, "Depends: AT-09"): AT-10's
runner MUST implement CTR-V0-001 through CTR-V0-003 and CTR-V0-010 as written, and MUST NOT mark any
case `accepted_by` other than `NOT_PRODUCED` on its own authority. CTR-V0-004 labels and CTR-V0-009
decisions are now frozen. Promotion to `implemented` still requires the five baseline artefact pins,
case-specific independent accepted-intent references, executable descriptors, and delivered runner
evidence; until then delivery remains `not-started`.
