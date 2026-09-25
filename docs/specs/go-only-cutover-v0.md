# Go-only cutover V0

- Owner: Russell Lewis
- Date: 2026-09-11
- Intent status: accepted
- Delivery status: experimental
- Authoritative inputs: `AGENTS.md`, `docs/specs/go-production-kernel-migration-v0.md`,
  `docs/specs/release-artifact-integrity-v0.md`, and decision 0088.

## Agent digest
- Claim: The legacy Python engine, live oracle and wheel are retired; native Go carries the product and conformance checks.
- Status: accepted/experimental; owner ordered the cutover and cancelled the pending Python comparison run.
- Exists: native Go CLI, Go conformance suites and reproducible Go archive machinery.
- Blocked on: `make gate` receipt (`GOC-V0-010`) and final CEM at the frozen release candidate, both NOT_RUN; at `573bd72b` the OCM links 0 of 9 after prepare and 8 of 9 after replaying decision 0234's committed link plan.
- Read next: Requirements; Acceptance; Rollout and rollback.

## User and measurable job

A user installs and runs the native Go product without the retired Python implementation or wheel.
A maintainer verifies it against independent frozen fixtures and explicit spec assertions without
repeatedly executing the old Python runtime. The pending paired Packet 5 retry is cancelled,
not measured, passed or reinterpreted.

## Verified current state

At `9ca27f9a`, `src/` contains eighteen legacy modules, `pyproject.toml` exposes `corvint_cli:main`,
Go tests execute the Python oracle, and paired performance validation still times both runtimes.
Go's implementation is already under `cmd/corvint` and `internal/`; its archive gate is native Go.
The existing source-only, platform, safety, evidence and integration qualifications remain distinct.

CEM/OCM closure of the committed cutover range (confirmed 2026-09-13, decision 0234). The CEM
committed at `573bd72b` binds base `9ca27f9a`. A clean worktree at that commit reproduces the
following from committed inputs plus three private coordinator inputs: an intent manifest naming
only this spec, an empty citation plan (the committed CEM is already cited) and a local outcome of
`blocked` for `make gate`, which was not run there. `make dogfood-change
BASE=9ca27f9a62a2add263ff559fd711feea5cdfd93d` exits 0 with `complete: true`. Its only
`NOT_PRODUCED` step is `prechange-impact unsupported-impact-range` (`GOC-V0-009`). The CEM is
`ready-for-ci`, with 512 of 513 hunks supported and 1 unknown intent bootstrap. `ocm prepare`
alone yields a `ready-for-review` OCM that links 0 of 9 requirements, all `unassessed`. OCM maps
are local reviewer artifacts that bind the exact target commit (`OCM-V0-002`, `OCM-V0-009`), so no
link state is committed. Replaying the eight `ocm link` rows recorded in decision 0234 against that
map links `GOC-V0-001..006`, `008` and `009`. `GOC-V0-007` stays `unassessed`. A rerun of
`dogfood-change` keeps those links. `make dogfood-check` with the same base then exits 0 with
`dogfood-check: PASS`, `outputsAgree: true` and OCM coverage 8 of 9. `GOC-V0-010` postdates
`573bd72b` and is outside that OCM scope. Changes after `573bd72b` are outside that CEM. A final
CEM at the release candidate stays with the release coordinator. Under load, CEM and OCM
commands intermittently reported `git-timeout` or `repository-object-unavailable`; clean reruns passed.

Artifact witness and exact-target gate. `script/go-archive-gate` already records the private archive
witness bound to the exact commit, tree and archive digests (`GOC-V0-006`). `GOC-V0-010` binds a
complete `make gate` run to that witness. At the frozen release candidate, in a clean worktree, run
`PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH make gate` and then `script/release-checklist`.
The gate must exit 0 and the checklist must print `PASS` on both the `go-archive` and `full-gate`
rows; `script/release-checklist --pre-promotion` exits 0 only then and only when no row is
`FAIL`, before any tag (ticket V1-0189). Together they would prove that every gate step passed
at that exact commit and tree, and that the archive witness is the one the same run recorded.
Both remain NOT_RUN: running the gate at the release candidate belongs to the coordinator, not to
this change.

## Requirements

- `GOC-V0-001`: The current tree MUST contain no legacy `src/corvint_cli.py` or
  `src/context_corvint*.py` engine modules, Python wheel entry point or wheel release job.
  `VERSION` MUST carry the product version used by release and dogfood scripts.
- `GOC-V0-002`: Default Go verification MUST NOT execute, reconstruct from Git, or install the
  retired Python oracle. Existing immutable historical fixtures and reports MUST retain their
  original provenance and verdicts. Expectations MUST NOT be generated from the current candidate.
- `GOC-V0-003`: Removal of live comparisons MUST preserve unique positive, negative, malformed-input,
  read-nonmutation, private-mode, error-precedence and boundary assertions as independent
  spec assertions or frozen expectation checks. A candidate agreeing with itself is not conformance.
- `GOC-V0-004`: Legacy trace migration MUST remain supported using independently authored canonical
  input fixtures and Go validation; preparing a legacy input MUST NOT require the retired runtime.
- `GOC-V0-005`: The cancelled Packet 5 paired retry MUST NOT run. A historical PASS/FAIL/NOT_RUN
  MUST NOT be relabeled. Release validation MUST use native correctness and artifact checks;
  absolute performance claims require native measurements and stay NOT_RUN until those exist.
  No Go/Python relative-speed threshold remains a Go-only release prerequisite.
- `GOC-V0-006`: Go archives MUST remain reproducible, source-bound, offline, checksum-verified and
  native-smoke-tested where the host permits. Unsupported platform and integration surfaces MUST
  keep their existing explicit status; removing Python does not grant FULL or publication authority.
- `GOC-V0-007`: Rollback MUST use a retained prior Go artifact or Git revision, without receipt,
  map, repository or trace migration. Historical Python evidence remains inspectable through Git;
  rollback MUST NOT require reinstalling Python as a product dependency.

- `GOC-V0-008`: Maintained Corvint adapters and developer tools MUST have no Python interpreter
  dependency. Required behavior and safety tests MUST be ported to Go; obsolete migration-only
  tools MAY be retired with an explicit replacement or historical-only disposition. Python source
  used as analysis fixture data is not executable Corvint tooling. An explicitly requested command
  in a user's Python project may use that project's own interpreter; Corvint MUST NOT require it for
  its own build, tests, hooks, packaging or default execution.
- `GOC-V0-009`: The cutover's final coordinator MAY complete with `prechange-impact` visibly
  `NOT_PRODUCED unsupported-impact-range` because the full immutable delivery range exceeds the
  experimental compiler's supported shape. It MUST retain a closed private artifact binding the
  exact argv, base, target, exit status and raw stdout/stderr digests. Only that typed exit qualifies;
  malformed output, another exit, missing evidence or drift blocks completion. This is context
  abstention, not impact success: ordinary inspection MUST record the miss, and CEM/OCM closure,
  selected checks, clean-target validation and independent verifier agreement remain unchanged.
- Proposed amendment, not accepted (V1-0264, DCW-V0-025): widen the qualifying typed exit above to
  exactly `unsupported-impact-range`, `unsupported-impact-repository` and `unsupported-impact-path`,
  each retained under its own code with the same artifact bindings. Every other exit still blocks
  completion. Until the owner accepts this amendment, the accepted text above governs.
- `GOC-V0-010`: A complete `make gate` MUST first remove the private receipt
  `<git-dir>/corvint/release-gate-receipt` and, only from a clean worktree, stamp HEAD's commit and
  tree in `<git-dir>/corvint/release-gate-start`, finishing both before any other gate step starts,
  including under `make -j`; a step run on its own MUST NOT clear the receipt. Only after every
  step has passed, it MUST record that
  receipt as one mode-0600 line: `corvint-gate-receipt/0 <commit> <tree> <witness-sha256>`. The line
  binds HEAD and the SHA-256 of the archive witness that `go-archive-gate` recorded in the same run.
  Recording MUST refuse and leave no receipt when the witness is absent or a symlink, when the
  worktree has any change or untracked file or `git status` fails (a tracked path flagged
  skip-worktree or assume-unchanged, or an ignored `.go` file outside a `.`- or `_`-prefixed or
  `testdata` directory, counts as a change: `git status` does not report either, and `go ./...`
  compiles the second; decision 0238), when no start stamp exists or it
  differs from HEAD's commit and tree, when `archive-status` for HEAD is not `PASS`, or when
  the witness changes while it is judged. A `record` signalled by `HUP`, `INT` or `TERM` MUST exit
  129, 130 or 143 and leave no receipt (decision 0248). `script/release-checklist` MUST report the `full-gate` row
  as follows. It is `PASS` only when the receipt names HEAD and the digest of the current witness.
  It is `NOT_RUN` when the receipt is absent, stale or names another witness. It is `FAIL` when the
  receipt is a symlink, a non-regular file or noncanonical, including any byte after its one LF. The receipt is local operator evidence,
  not a signature or CI attestation (decision 0224).

## Non-goals and boundaries

This cutover does not rewrite the engine in Rust, change protocol bytes to make tests pass,
remove support for analyzing Python projects, or delete historical evidence. Python-language
fixtures and optional pytest execution describe user projects, not the retired Corvint engine.
Installed protected hooks and their pinned executable are outside repository cutover authority.
The owner additionally requested removal of every Python runtime dependency, including optional
host adapters and developer tools. Installed protected configuration remains user-owned.

## Failure modes and trust boundary

A missing independent expectation, weakened refusal, changed read state, secret exposure, unsafe
write, process leak or failed artifact gate blocks completion. Never satisfy a removed oracle check
by skipping it or substituting the candidate as its own oracle. Preserve bounded child ownership,
timeouts and cleanup. No source, receipt or historical benchmark is rewritten to manufacture PASS.

## Acceptance

The smallest path is a successful native Go build and real read-only query after deleting `src/`.
Then the focused replacement regressions, complete `make gate`, independent review and final
CEM/OCM workflow must pass. Source and CI inspection must show no live Corvint Python oracle caller.
Native archive witnesses remain bound to the exact final commit/tree. Performance remains
unmeasured when the owner has cancelled measurement; it is not an inferred PASS.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GOC-V0-001 | source removal, VERSION, packaging/CI changes | `cmd/corvint/go_only_cutover_test.go::TestGoOnlySourceAndVersion`; native build/read-only query observed; a CI-workflow scan fails if any `.github/workflows/*.y*ml` names wheel/pypi/twine/bdist_wheel/cibuildwheel/setup.py; exact-target gate required |
| GOC-V0-002 | Go tests and frozen conformance runners | `conformance/cli-parity-v0/runner_test.go::TestGPKV0002ManifestReplay`; immutable expected bytes, no live oracle; `::TestGPKV0002ReplayRefusesLiveOracle` and `::TestGPKV0002CaptureCannotRegenerateExpectationsFromCandidate` assert `replay`/`capture` refuse a live-oracle override and expectation regeneration under decision 0088 |
| GOC-V0-003 | spec-authored regression assertions | `cmd/corvint/main_test.go::TestFreshProcessCLICompatibilityEdgesHaveStableNativeResults`, including exact `invalid-harness-input` message-precedence assertions (nesting depth vs. duplicate key vs. UTF-8 validity vs. unpaired surrogate); deletion disposition in `tests/README.md`; independent review repaired migration and process cleanup gaps |
| GOC-V0-004 | Go migration fixture and assertions | `cmd/corvint/migrate_traces_test.go::TestMigrateTracesApplyMatchesPythonOracle`; independently authored bytes, SHA-256, private mode and quarantine assertions |
| GOC-V0-005 | release checklist and legacy perf refusal | `conformance/perf-v0/main_test.go::TestRetiredPerfProtocolPreservesEvidence`; frozen verdict bytes survive rejected run/diagnose/capture; `script/release-checklist_test.sh` asserts the exact owner-cancellation reason text for the `native-performance` row and statically refuses any relative-speed/"Nx"/"faster" wording in `script/release-checklist` |
| GOC-V0-006 | existing Go archive checker | `conformance/release-artifact-v0/archive_gate_test.go::TestRawCommitExportBuildCarriesExactVCSIdentity`; final gate must retain the exact-target archive witness; `conformance/release-artifact-v0/smoke_test.go::TestWriteChecksumsUsesCanonicalSHA256SUMSFormat` pins the SHA256SUMS line format and `::TestSmokeTestExecutesRealSubprocessAndDetectsFailures` drives `smokeTest`/`querySmoke` against a real subprocess, requiring genuine PASS and genuine FAIL on a wrong version banner, wrong resolved intent, and a mutated fixture repository |
| GOC-V0-007 | retained prior Go revision `9ca27f9a62a2add263ff559fd711feea5cdfd93d` | no automated test, by design: rollback is an unchanged, previously-accepted executable-selection policy exercised operationally, not a new Go-only behavior; operational smoke evidence is recorded separately, not represented as a new automated OCM test claim |
| GOC-V0-008 | native adapters and developer tools | `cmd/corvint/host_adapter_javascript_test.go::TestHostAdapterJavaScriptHosts`, native hook/process tests and per-tool authored regressions; `script/no-python-runtime-dependency_test.sh` statically refuses a literal Python interpreter invocation (`python`/`python3`/`pip`/`pytest`) in `Makefile`, `script/*.sh`, or `integrations/**/*.{mjs,json}`, allowing only lines guarded by the `CORVINT_TEST_EXTERNAL_PYTEST` opt-in and masking the `corvint-analyzer-python` Go tool name and the check's own name before matching, so neither exempts the rest of a line; `make gate` runs it as the `no-python-runtime-dependency-test` step. The Pi and protected-Pi TUI tests drive their PTY through the Go helper `tools/pi-tui-fixture`, their signal-ignoring descendant fixtures are `/bin/sh`, and the protected build extracts the pinned Node binary with `/usr/bin/tar`; exact-target gate required |
| GOC-V0-009 | explicit delivery context abstention | `cmd/corvint/go_only_cutover_test.go::TestGoOnlyContextAbstentionRemainsClosed` executes `script/dogfood-change_test.sh` refusal, malformed-output, crash and drift cases; actual refusal and ordinary-inspection evidence remain required |
| GOC-V0-010 | `script/gate-receipt`, `make gate` clear-first/record-last wiring, `full-gate` checklist row | `script/gate-receipt_test.sh` covers the canonical binding, mode 0600, the clear step, each refusal leaving no receipt (including a worktree dirty at the start, a commit made during the run, a failing `git status`, a tracked file edited under skip-worktree or assume-unchanged, and an ignored `.go` file Go compiles, while one under a dot directory records), a `record` signalled with `TERM` during a passing status check exiting 143 with no receipt (decision 0248), the Makefile wiring, and under `make -j8` the real Makefile graph starting no step before the clear finished while a standalone step does not clear; `script/release-checklist_test.sh` covers `full-gate` PASS for a bound receipt, NOT_RUN for a stale receipt or a changed or missing witness, and FAIL for a malformed, trailing-byte or symlinked receipt; the gate run at the release candidate is still NOT_RUN |

At `573bd72b`, the cutover OCM scope links eight of nine requirements only after decision 0234's
committed link plan is replayed onto a freshly prepared local map. Each row names the
requirement-labelled test case and the supported CEM hunk whose target lines contain it. Prepare alone links none.
`GOC-V0-007` keeps the prior executable-selection policy unchanged, has no requirement-labelled
claim and stays `unassessed`. A manual rollback smoke is separate evidence.
Structural links do not grant a gate PASS or integration/publication qualification.

## Rollout and rollback

Remove the old engine and packaging, replace live-oracle assertions, then complete the native gates.
The owner explicitly supersedes GPK-V0-025's Python coexistence windows and GPK-V0-033's mandatory
live cross-check for this cutover; independent spec conformance remains binding. On a failed gate,
repair the Go path or restore the prior Go revision. Do not restart the cancelled paired run.
Delivery remains experimental until acceptance evidence is recorded; native integration support,
publication and product promotion remain separate.

## Native tooling contracts

`benchmarks/dogfood-workers` replaces the Python worker-accounting command with the same
`corvint-dogfood-workers/0` receipt. Missing counters and decreasing cumulative counters remain
`NOT_OBSERVED`; cancelled and failed workers remain in totals. Receipt publication is atomic and
owner-only. Native CLI regressions retain the independently authored repeated-snapshot and
missing-request cases.

`benchmarks/dogfood-measure` replaces the interpreter-based measurement supervisor. Its new
`corvint-dogfood-measure/1` receipt identifies the Go runtime and executable digest, uses a monotonic
wall clock, and retains nearest-rank p50/p95/MAD, input fingerprints, rotating workload blocks,
private cache phases, live output bounds, process-group cleanup, and the 30-sample claim floor.
The input protocol remains `/0`; `{root}` is supported and the retired `{python}` placeholder
is rejected before any workload starts. CPU/RSS accounting is supported on Linux and macOS;
other hosts refuse measurement rather than report unavailable accounting as zero. Existing
Python-produced measurement artifacts retain their original `/0` identity and verdicts.
Synthetic fixture tests of this tool are not product performance qualification.

External Python-project execution remains a separate, opt-in integration qualification:
`CORVINT_TEST_EXTERNAL_PYTEST=1` enables the existing pytest toolchain tests when explicitly selected.
The default Corvint source gate does not start that interpreter. Frozen Python source and AST
verdict fixtures remain native-Go conformance inputs. The obsolete live AST differential experiment
and whole-retired-engine source walk are retired; the authored valid/invalid syntax table and
recorded `ast-parity/EXPECTED.tsv` checks remain required.

### Protected native hook replacement

The Go `native-hook` entry point replaces the repository's Python authority wrapper. It derives
its own canonical `/Library/CorvintAuthority/versions/<release>/corvint` path, refuses uninstalled
copies, and process-replaces itself with exactly `LANG=C.UTF-8` and `PATH=<release>/bin` before
reading host input. This preserves the old closed environment without a supervising child.
The host retains its two-second watchdog; a single 1600 ms post-bootstrap context covers bounded
normalization and the existing independent consumer. Only the fixed active-enrollment pointer may
supply a Stop enrollment lookup; qualified lifecycle normalization never reads that pointer.
Caller root, consumer, policy, transcript and claimed qualification fields are discarded.

Prepared native bundles use `corvint-authority-release/1` and `corvint-authority-source-release/1`.
Their `authority-hook.json` is an immutable, non-executable declaration containing the final
consumer path and digest. Its digest binds configuration; the consumer digest binds the executable
normalizer. Qualified-hook preparation emits `/1` evidence and validates both identities before
writing an authority-`NONE` sidecar. Existing installed `/0` artifacts are not rewritten, admitted,
or promoted by this cutover. Native qualification must independently cover the new exact bundle.

#### Native hook fallback reasons

The `native-hook` entry point selects the fallback reasons below (decision 0100). Each row cites
the first site and states only the condition checked there. The fallback `systemMessage` is fixed
text per mode and does not include the reason; only `corvint-invocation-timeout` changes the
qualified-mode message.

| Code | First emitting site | At the cited site |
|---|---|---|
| `consumer-unadmitted` | `cmd/corvint/native_hook.go:47` | the running executable path cannot be read or is not an admitted canonical release path (pattern mismatch, unclean, or a missing or symlinked component); also returned when the process replacement into the closed environment fails |
| `input-unavailable` | `cmd/corvint/native_hook.go:68` | bounded reading of host input failed before the 1600 ms context expired |
| `invalid-mode` | `cmd/corvint/native_hook.go:43` | arguments were given and they are not exactly `--qualified-lifecycle` |
| `invalid-native-hook-input` | `cmd/corvint/native_hook.go:133` | normalization refuses the host input (over the input bound, invalid UTF-8, undecodable JSON, or fields or values the selected mode does not accept); the entry point maps it to the `invalid-input` fallback |
| `not-enrolled` | `cmd/corvint/native_hook.go:73` | without `--qualified-lifecycle`, after host input was read, reading the active-enrollment pointer failed: it is unreadable or does not decode to exactly `profile` `corvint-protected-active-enrollment/0` plus an `enrollmentHandle` of 64 lowercase hex digits |

### Native conformance and tooling dispositions

The use-case ledger and Human Documentation Compiler conformance commands now live in native Go
packages in their original directories. Ledger content addresses, all six promotion classes,
receipt reuse/path/digest refusals, and the 19-job closed set are unchanged. The documentation
runner retains the 19 frozen cases, canonical observations, exact source/claim/patch bindings,
external environment checks, private isolated fixtures, and owned-group interruption cleanup.
Neither runner promotes an unobserved outcome: all current use cases remain UNPROVEN, and a
structural build/denial receipt cannot prove an execution that the runner did not observe.

`script/spec-coverage-audit` replaces the Python mention inventory. Its separation of test/case
mentions, fixture names, comments, binary files and symlinks remains an inventory rather than
behavioral verification. The original real-Git TCQ static/dynamic vectors are consumed by
`internal/frontierrepo/tcq_conformance_test.go`, including the independent legacy Python-claim
input and hostile XML/partial-tuple refusals; no Python code is executed. The public OCM writer's
existing Python-enumeration refusal remains unchanged.

The redundant Python CEM 0.1/0.2, LRF and TCQ drivers are retired in favor of the existing native
corpus suites plus the real-Git TCQ vectors. Frontier's Python derivation generator is retired;
hand-authored vectors and the independent native reference codec remain binding. Historical LRF
Python mismatches retain their failed status. No candidate-generated expectation replaces them.

The wheel-install first-run experiment is retired with the wheel. The dated blob-shard benchmark,
next-generation evidence-restoration script and September 8 unprivileged local-authority campaign
supervisor are historical-only tooling, retained through Git at `9ca27f9a`; their reports are not
rerun, rewritten, or promoted. Native archive integrity and the local-authority Go module tests
remain separate evidence and do not replace a native first-use or production qualification.
The shipped Python aider bridge is retired; the existing optional caller-provided alternative
retriever command remains external, with no claim of a native reimplementation of aider ranking.

The native batch-investigation supervisor uses audited cooperative lifecycle hooks. Each has an
allowance of at most one second, in addition to the normal two-second group-shutdown budget.
The startup hook observes only the launched leader's descendants; the shutdown hook joins its
bounded observer, checks retained PID/start identities before TERM and KILL, and reports observed
survivors or probe failures before any fallback. Hook error, panic or allowance exhaustion never
skips owned-group termination, reap or pipe drain. An exited leader can have unobserved reparented
children; that completeness remains explicitly unverified, never universal containment.

### Native experiment and regression closure

`experiments/cem-30x30` retains the seven preregistered gates using exact Go rational arithmetic,
closed blinded observation schemas, deterministic assignment, and hash-bound human task registry.
Synthetic gate outcomes remain `NOT_RUN`; the native scorer creates no human qualification claim.
Its frozen pass/failure fixtures and all eight schema documents are unchanged. Decimal elapsed
observations now use exact rational subtraction consistently with their admitted number schema.

`tests/README.md` maps the retired Python test-driver families to native suites. Gemini/OpenCode
boundary checks execute their actual JavaScript adapters with the selected host runtime. The
Codex/Claude wrappers now invoke the linked Go kernel, so legacy wrapper-to-Python IPC fault
injection is retired with that boundary; closed input, kernel protocol, authority and process
supervision checks remain. `script/dogfood-w12.sh` is retired as a Python/candidate migration-only
comparison. No retained historical failed or unrun result is promoted by these changes.

### Native host regression fixture and cleanup observations

The Gemini/OpenCode transport tests use an independently authored native fixture, built under
bounded process ownership. It emits fixed receipt fields, independently computes the canonical
hash, and supplies explicit tamper/hang cases. It is not the candidate implementation or a source
of frozen expectations. HTML characters, Unicode separators and literal escape sequences are
included in the canonical-hash checks. Valid cases have no timeout retry; production deadlines
remain unchanged.

A reproduced Gemini group-signal race now preserves unexpected cleanup errors as
`corvint-process-cleanup-unconfirmed` instead of throwing from output or timer callbacks. Only
`ESRCH`, or `EPERM` after the owned leader's observed exit, is treated as benign. Darwin also returns
`EPERM` for a group whose members have all exited but are not yet reaped, so an `EPERM` before the
leader's exit is observed is decided at close: the group is signalled once more, and only a delivered
signal or `ESRCH` confirms cleanup; a repeated `EPERM` stays unconfirmed. The adapter still
waits for close and pipe drain; it does not report uncertain cleanup as successful containment.
The test supervisor composes caller cancellation with shutdown and includes an outer-supervisor
interruption regression proving its detached native descendant exits.
