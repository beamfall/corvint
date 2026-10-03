# Post-merge CI Host V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed
Delivery status: experimental
Authoritative inputs: human request https://github.com/beamfall/corvint/issues/398 (part of #388);
`docs/specs/postmerge-connectors-v0.md`; `docs/specs/authoring-step-scope-v0.md`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; `AGENTS.md`.

## Agent digest
- Claim: Reference CI templates and an optional paired launcher have source conformance; physical isolation and hosted replay remain unqualified.
- Status: proposed/experimental; declared-YAML audit and bounded paired source conformance. Physical boundary and hosted replay qualification remain pending.
- Exists: `protocol/postmerge-host` (graph, templates, installer, adapter contract); `internal/postmergehost` (audit, strict paired wire, fixed Linux shim, bounded launcher and source conformance tests); `cmd/corvint-postmerge-host-launcher` (optional command). Physical host execution is unqualified.
- Blocked on: the merge replay set (#395), a hosted dry-run of that set, `corvint delta` (#389), owner acceptance and native completion.
- Read next: Requirements; Trust boundary; Acceptance evidence.

## User and measurable job

A CI maintainer adopts the post-merge workflow on their own host without designing the credential
split themselves. The measurable job has three parts. The maintainer copies one reference pipeline
and supplies five hooks. An audit shows that no authoring environment can reach an outward-write
credential. Any merged change can be replayed by its full id. The simpler baseline is handwritten
workflow YAML that is reviewed once.

At base `ff3da727`, Corvint has the post-merge connector (PMC-V0), intake (`corvint-intake`), step
attestation (ASS-V0) and metrics (PMM-V0) companions, but no CI host template or host contract.
Corvint stays a local binary. The templates are operator reference material, not a hosted service
(product invariant 7).

## Requirements

- `PCH-V0-001`: Publish a host-neutral workflow graph, `protocol/postmerge-host/workflow-graph.json`,
  with profile `corvint-postmerge-host-graph/0`. It declares nine ordered steps: trigger, intake,
  delta, follow-up-item, authoring, trusted-validation, draft-change-requests, findings, metrics.
  Each step lists the earlier steps it follows, its allowed credential classes and its documented
  Corvint commands. Credential classes have disjoint secret-name prefixes and an outward-write flag.
  Authoring requires attestation and holds no outward-write class. A command that does not exist
  yet is listed as pending. Unknown fields, unsorted lists and forward dependencies refuse.
- `PCH-V0-002`: Document the host adapter contract in `protocol/postmerge-host/README.md`:
  - step and workflow markers (`CORVINT_PM_STEP`, `CORVINT_PM_WORKFLOW`);
  - the secret naming convention;
  - the hooks (export-context, reader, step-host, author and validate), with each hook's
    inputs, outputs and credential classes.
  Raw tracker and forge text stays outside the author worktree. Only `corvint-intake validate`
  output reaches authoring.
- `PCH-V0-003`: Ship a reference pipeline that processes one merged change by full commit id:
  - It accepts only `dry-run` and `recording` modes. Remote live writes are unsupported (PMC-V0-006).
  - It serializes runs per change without cancelling a run in progress.
  - Authoring runs in its own job on a fresh runner, bracketed by `corvint step snapshot`,
    `env-check` and `verify`.
  - Trusted validation runs on another fresh runner. It takes the binding and source item from its
    own export, not from author output.
  - The delta step stays `NOT_PRODUCED` until `corvint delta` exists.
- `PCH-V0-004`: Ship a source trigger. It runs on push to the merged branch only, never on a change
  request. Its push filter declares only `branches`, a non-empty list of literal branch names. It
  continues on error, is bounded to ten minutes, and only dispatches the pipeline. Because of this,
  it cannot block or fail the merged change request.
- `PCH-V0-005`: Ship scheduled reconciliation that re-dispatches recent first-parent merges, so a
  lost dispatch is retried. A manual replay input re-dispatches a committed list of full change ids
  in dry-run mode by default. Repeated runs rely on connector idempotency (PMC-V0-005 to PMC-V0-007).
- `PCH-V0-006`: Pin every action to a full commit and check out with `persist-credentials: false`.
  Action references must name an owner beginning with an ASCII letter or digit; local `./` paths
  are not commit pins even when their path ends in `@<40-hex>`.
  Build Corvint companions from a pinned 40-hex source commit, and refuse any binary whose SHA-256 is
  not in the operator's pin file before its first use.
  Untrusted values reach scripts through `env`, never by expanding expressions into script text.
  A dispatched change id, a replay-set line, the pinned source commit and a companion name are
  each validated as one whole string by `case` patterns and a length check, never by a
  line-oriented `grep`, before any of them is written to `$GITHUB_OUTPUT` or used.
- `PCH-V0-007`: Provide an audit, `internal/postmergehost.Audit`, over a restricted YAML subset. It
  refuses unsupported constructs: anchors, aliases, tags, folded scalars, flow mappings, multiple
  documents, tabs, duplicate keys, unmodelled job keys such as reusable workflows, containers
  and services, unmodelled step keys such as a custom `shell:`, a non-mapping `env`, an `env`
  merge key (`<<`) or non-scalar `env` value at any level, a non-scalar `with` value or `run`, and
  any workflow- or job-level `defaults` other than `run.working-directory` [UNMODELLED_KEY, or
  CUSTOM_SHELL for `defaults.run.shell`].

  It reports each of these, with the finding code in brackets:
  - change-request triggers [CHANGE_REQUEST_TRIGGER];
  - workflow-level write permissions or secrets [WORKFLOW_WRITE_PERMISSION, WORKFLOW_LEVEL_SECRET];
  - secrets without a class prefix [UNCLASSIFIED_SECRET];
  - a credential not allowed for the job's marked steps [CREDENTIAL_NOT_ALLOWED];
  - any outward-write class, write permission or `github.token` in an authoring job
    [AUTHORING_WRITE_CREDENTIAL];
  - an authoring job that performs other steps [AUTHORING_NOT_ISOLATED];
  - unpinned actions [UNPINNED_ACTION] and persisted checkout credentials, matching
    `actions/checkout` in any letter case and with any sub-path [CHECKOUT_PERSISTS_CREDENTIALS];
  - expressions in run scripts or in a `with` input named `script` in any letter case
    [RUN_EXPRESSION_INTERPOLATION], any `with` input name outside lowercase ASCII `[a-z0-9_-]`
    [NON_LOWERCASE_INPUT], and a `with` that is not a mapping [UNMODELLED_KEY];
  - a workflow, job or step `env` key that makes a shell, the dynamic loader, a tool or an
    interpreter run unaudited code: `BASH_ENV`, `ENV`, `BASHOPTS`, `SHELLOPTS`, `PS4`,
    `PROMPT_COMMAND`, `IFS`, `CDPATH`, `PATH`, `HOME`, `CC`, `GOFLAGS`, `GOTOOLCHAIN`,
    `JAVA_TOOL_OPTIONS`, `NODE_OPTIONS`, `PYTHONSTARTUP`, `PYTHONPATH`, `PERL5OPT`, `PERL5LIB`,
    `RUBYOPT`, the runner toggles `ACTIONS_ALLOW_UNSECURE_COMMANDS` (re-enables the `::set-env`
    and `::add-path` workflow commands) and `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION`, or a name
    starting `LD_`, `DYLD_`, `BASH_FUNC_`, `GIT_CONFIG` or `FORCE_JAVASCRIPT_ACTIONS_TO_NODE` (the
    last three choose the Node.js runtime that loads action code), matched in any letter case
    because Windows runners read environment names without regard to case [STARTUP_ENV];
  - a run script that names `GITHUB_ENV` or `GITHUB_PATH`, or contains a literal `set-env` or
    `add-path` command prefix, in any letter case, in either `::` or legacy `##[` form
    [RUNNER_ENV_FILE];
  - a step or `defaults.run` `working-directory` that is not a literal scalar
    [WORKING_DIRECTORY];
  - a value validated by a line-oriented `grep` fed from `printf`, `echo` or a here-string
    [LINE_ORIENTED_VALIDATION];
  - a source-trigger push filter other than a literal `branches` list [SOURCE_TRIGGER_BRANCHES];
  - a Corvint binary run before the pinned install [UNPINNED_BINARY];
  - a Corvint command not documented for the job's steps [UNDOCUMENTED_COMMAND];
  - job dependencies that violate graph order [ORDER_VIOLATION];
  - source-trigger, concurrency and mode-input rule violations.

  Every shipped template audits with zero findings.
- `PCH-V0-008`: The README gives a porting guide that maps markers, credential classes, isolation,
  post-merge triggering, per-change locking, replay and reconciliation to another host. It states
  the qualification limits:
  - declared YAML only;
  - no view of organisation, runner or environment secrets, or the implicit runtime token;
  - same-VM attestation is not confinement;
  - no hosted run.

- `PCH-V0-009`: An optional operator-owned launcher accepts only the exact final paired profile/request/control/event schemas, independent immutable admission pins, fixed author argv/cwd and canonical single output root. Reject unknown/duplicate/null/alias/hash/size/depth errors before execution. It adds no default Core command, daemon, automatic execution or live writer credential.
- `PCH-V0-010`: Native Snapshot/Preflight/Verify observe the same canonical host product worktree and objects across author execution. Derive only the reviewed RO-root/RW-scope/RO-guard topology from the complete native declaration; mask Git administration, refuse sensitive aliases and all other overlapping mounts, and preserve every original native graph/unknown. Copy/digest substitution or inode normalization cannot discharge identity.
- `PCH-V0-011`: A pinned fixed Linux internal shim from the same optional command clears and observes the complete effective author environment before readiness, waits behind the private execution barrier, then direct-execs the exact pinned author with those same bytes and closed control handles. Native Host.Environment derives from this actual author envelope, never image/hold/overrides. Host/controller implicit authority and unauthenticated native assertions remain explicit.
- `PCH-V0-012`: Durable exact run ownership exists before engine create. Independent bounded cleanup handles EOF/INT/TERM/author timeout and late-create ambiguity, joins owned descendants and retains raw actual controller/client/shim/author exit attribution. SIGKILL requires supervisor reconciliation; uncertain creation/cleanup/survivors/incomplete facts remain HELD/BLOCKED. Never infer an author signal from 128+exit or client termination, reallocate an uncertain run, or globally prune.
- `PCH-V0-013`: The selected Add fixture uses actual native intake/BuildAuthorInput, source-derived authored documentation and test, and the real same-object native verification. Fresh trusted validation runs the same authored test bytes against merged addition (PASS) and trusted subtraction control (FAIL). This candidate conformance is not historical labels, delta, accepted NEA proof, drafts, recording or full host replay.
- `PCH-V0-014`: Candidate limits, actual immutable image/tool/source/shim/author pins, observed full image/hold/author environments, mounts/namespaces and lifecycle facts are retained for the exact executed tuple. The candidate has 16MiB private home/temp, not14GiB copied work. No generalized kernel/security/secret absence or filesystem quota claim follows. Hosted qualification preserves PCH-V0-003 separate fresh author-only/trusted jobs and uses a reviewed default-branch experimental scaffold before manual dispatch; full original dry-run replay and audit remain required for completion.


## Non-goals

- A hosted service, daemon, GitHub App, or network dependency in the default local product.
- Remote live writes, vendor API adapters, or running an agent.
- A published audit command.
- A general YAML parser.
- Templates for hosts other than GitHub Actions.
- Authenticating the forge or tracker.
- Proving runtime credential isolation.

## Failure modes

- A secret is configured outside the workflow text, for example at organisation, environment or
  runner level, and is reachable by authoring. The audit cannot see it, so it reports a PASS for the
  declared text only.
- Artifact actions use an implicit runtime token. An author could tamper with the uploaded patch or
  connector input. The trusted job re-derives the binding and source item from its own export and
  checks the patch with `git apply --check`. It does not authenticate the author output.
- Inference, not observed: author code on a hosted runner may obtain the job's
  `ACTIONS_RUNTIME_TOKEN`. With it, the author could delete and re-upload the `before-state`
  artifact, even though it was uploaded before the author ran. The author could also write Actions
  cache entries scoped to the default branch, which other workflows on that branch may restore.
  The templates bind no digest of the before-state from a job the author cannot influence. The
  trusted job and its `validate.sh` hook must therefore treat `before-state`, like author output,
  as untrusted input, never as proof of the pre-authoring state. Workflows that restore Actions
  caches on the same repository inherit the poisoning risk.
- A dispatch fails on push. Nightly reconciliation re-dispatches the change.
- A ported template uses an unmodelled construct. The restricted YAML parser returns
  `unsupported-yaml` for input outside its audited subset; the audit refuses it rather than
  guessing.
- An `env` variable outside the `STARTUP_ENV` denylist (for example a tool-specific variable such
  as `MAVEN_OPTS` or `GIT_SSH_COMMAND`) changes how a tool runs code, or a script
  reaches the runner's env file without naming `GITHUB_ENV` or `GITHUB_PATH` (for example through
  `eval` or a computed path). A script can also set a later step's environment or `PATH` through
  the `set-env` and `add-path` workflow commands when `ACTIONS_ALLOW_UNSECURE_COMMANDS` is set
  outside the workflow text, for example in a self-hosted runner's own environment. The script
  check matches literal command prefixes in both `::` and legacy `##[` form, without regard to
  command-name case, so it misses a command string built at run time.
  The audit cannot see indirect file access or command strings built at run time; a pinned action
  may also write the env file or print these commands. Runner-host
  settings such as `ACTIONS_RUNNER_HOOK_JOB_STARTED` and `ACTIONS_RUNNER_CONTAINER_HOOKS` are read
  from the runner's own environment, not the workflow text, and are outside the audit. The
  operator reviews these by hand.

### Experimental launcher reason and transcript code inventory

These fixed codes belong to this optional paired launcher. Event reasons retain a held result and
the original uncertainty; they do not grant native verification, runtime qualification, or successful
cleanup. Independent cleanup still runs where ownership is established, and its own outcome remains
explicit. An engine-client, shim, controller, or ambiguous exit is never relabelled as an author exit.

| Held reason | Observation or refusal |
|---|---|
| `author-start-unobserved` | The controller cannot establish the actual author start/exit observation, including an author that retires before the process join. |
| `author-timeout-or-supervisor-lost` | The author context deadline expires or its supervisor context is cancelled; this does not infer an author signal. |
| `barrier-disconnected` | The private execute barrier write fails. |
| `boundary-unverified` | The actual ready container inspection does not match the pinned boundary. |
| `cleanup-unresolved` | Independent exact-owner cleanup cannot establish removal and absence. |
| `control-invalid` | The control frame is unreadable or fails the closed wire/identity checks. |
| `control-repeated-or-supervisor-lost` | A further control frame or lost supervisor appears during author execution. |
| `control-timeout` | The ready launcher receives no control frame before its bounded timer expires. |
| `create-ambiguous` | The engine create call or its retained transcript cannot establish an unambiguous result. |
| `environment-unobserved` | The complete actual hold/author execution-envelope observation is missing or fails the pinned comparison. |
| `exec-inspect-unresolved` | An original execution inspection before or after the process-top observation cannot be collected and checked. |
| `hold-start-failed` | Starting the inert owned container fails or cannot be retained. |
| `image-unverified` | The immutable image inspection fails or does not match the pinned image/config/environment. |
| `late-completion-unresolved` | After log completion, final inspection is unavailable or still reports a running execution. |
| `native-control-unverified` | The original native control records do not satisfy the required host/state/preflight binding. |
| `ownership-conflict` | The exact run name is not established absent before creation; a conflicting resource is not removed. |
| `retention-incomplete` | A required original fact or output cannot be retained completely. |
| `retention-overflow` | The cumulative native-record retention reservation exceeds its bound. |
| `shim-create-failed` | Creating the pinned shim execution fails or cannot be retained. |
| `shim-identity-unobserved` | The returned shim execution identity cannot be decoded and pinned. |
| `shim-stream-failed` | The private shim execution stream cannot start. |
| `stream-incomplete` | Author stdout/stderr streaming ends with incomplete collection. |

Transcript operation names identify retained raw controller facts rather than success assertions:

| Transcript operation | Original fact |
|---|---|
| `author-top` | Original controller process-top response used for actual author join attribution. |
| `exec-create` | Original engine response that creates the pinned execution-envelope shim. |
| `exec-final` | Original final execution inspection after log collection. |
| `exec-inspect` | Original execution inspection before the actual process-top join. |
| `exec-inspect-after-top` | Original execution inspection after the actual process-top join. |
| `identity-conflict` | Cleanup inspection does not identify the exact owned resource; cleanup refuses removal. |
| `image-inspect` | Original immutable image inspection response. |
| `inspect-owned` | Original exact-owner inspection during independent cleanup. |
| `prior-absence` | Original exact-run absence response before resource creation. |
| `remove-owned` | Original delete response for the verified exact owned resource. |
| `start-inert` | Original response that starts the inert owned container. |

## Trust boundary

The audit reads declared workflow text and grants no authority. Hooks are operator code. Corvint
does not ship them, and the audit observes only the hook names it invokes. Step attestation is
ASS-V0 local observation, not confinement. Audit coverage is lexical and partial:
`LINE_ORIENTED_VALIDATION` matches only the `printf`/`echo` pipe and here-string forms; expression
checks cover `run` and `with.script` but not other action inputs that evaluate code; `STARTUP_ENV`
is a denylist and `RUNNER_ENV_FILE` matches only the literal names and command prefixes; and the
audit cannot see which branch is the repository default, so the operator checks that the source
trigger names it. The pin file and the source commit are trusted operator
configuration, and rebuilding the pins happens on a trusted machine with the same toolchain.

## Acceptance evidence

Run:

`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/postmergehost`

It checks that:
- the graph validates, and seven malformed graphs refuse;
- all three templates audit clean;
- the authoring job references no write-class secret, write permission or token;
- 79 single mutations each produce their specific finding code, including a reintroduced
  line-oriented `grep` check, a custom step shell, a workflow- or job-level `defaults.run.shell`,
  an unmodelled `defaults.run` key, an expression in `with.script` or `with.Script`, a
  non-lowercase or dotless-i (U+0131) `script` input name, a scalar-expression or sequence `with`,
  `HOME`, `CC`, `GOTOOLCHAIN`, `JAVA_TOOL_OPTIONS`, `GIT_CONFIG_GLOBAL` and a lowercase `ld_preload`
  in step `env`, `BASH_ENV`, `ENV` and `LD_PRELOAD` at workflow, job and step level,
  `ACTIONS_ALLOW_UNSECURE_COMMANDS`, `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION` and
  `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24`, an `env` merge key and a non-scalar `env` value at
  workflow, job and step level (including `<<:` hiding `LD_PRELOAD`), a non-scalar `with` value or
  `run`, an `actions/checkout` spelt in another case or with a `/.` sub-path that persists
  credentials, a non-mapping step `env`, writes to `$GITHUB_ENV` and `$GITHUB_PATH`, a printed
  `::set-env`, an expression or non-scalar
  `working-directory`, and non-literal or empty source-trigger filters;
- the pipeline's change-id check, run under bash and POSIX sh, writes outputs only for one whole
  40- or 64-hex id and refuses a newline-injected `change=` line, and replay dispatches only whole
  ids, including a final line without a trailing newline;
- the YAML subset refuses unsupported syntax;
- `install-pinned.sh` passes `sh -n`, refuses a short or multi-line commit, and refuses companion
  names containing `/`, `.` or a newline before any fetch;
- the README documents every hook the templates call.

Issue #398's first acceptance item is still open. It requires the replay set (#395) to run in
dry-run mode on a host, and that is `NOT_RUN`.

The paired source checks additionally cover strict profile/request/control bindings, canonical
single-link snapshots, complete environment refusal, exec/argv attribution, malformed/truncated
streams, same-object mount and security drift, exact ownership, cancellation-independent cleanup,
native record retention bounds and null inspection mounts. The actual author fixture checks native
Verify and the same authored test bytes against addition/subtraction. The full lifecycle tests use
an in-memory engine; configured mount/environment replies and test-asserted host booleans are not
physical facts. Retained independent source review closes three source findings after two bounded
repair cycles. These checks do not replace the original host replay acceptance item.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| PCH-V0-001 | `protocol/postmerge-host/workflow-graph.json`, `graph.go` | `TestGraphContract` |
| PCH-V0-002, PCH-V0-008 | `protocol/postmerge-host/README.md` | `TestReadmeDocumentsTemplateHooks` |
| PCH-V0-003, PCH-V0-004, PCH-V0-005 | `protocol/postmerge-host/github-actions/*.yml` | `TestReferenceTemplatesAuditClean`, `TestAuditRefusesUnsafeTemplates`, `TestReplayRefusesMalformedChange` |
| PCH-V0-006 | `protocol/postmerge-host/install-pinned.sh`, `github-actions/*.yml`, `audit.go` | `TestInstallPinnedSyntax`, `TestResolveRefusesInjectedChange`, `TestAuditRefusesUnsafeTemplates` |
| PCH-V0-007 | `yaml.go`, `audit.go` | `TestAuthoringEnvironmentHasNoWriteCredential`, `TestAuditRefusesUnsafeTemplates`, `TestParseYAMLSubset`, `TestShellCommands` |
| PCH-V0-009 | `launcher.go`, optional command | `TestHostProfileClosedWire`, `TestHostRequestIdentityRefusals`, `TestHostControlBindingRefusals`, `TestClosedLauncherInvocation` |
| PCH-V0-010, PCH-V0-013 | `launcher.go`, paired-author fixture | `TestHostCandidateMountsRefuseAuthorityAliases`, `TestPairedAuthorSourceConformance` (actual native Verify and guarded negative; physical confinement unqualified) |
| PCH-V0-011 | `envelope_linux.go`, `envelope_other.go`, `launcher.go` | `TestHostEnvelopeCompleteEnvironment`, `TestHostStreamRefusesIncompleteOrForgedReadiness` (physical envelope pending) |
| PCH-V0-012 | `launcher.go` | `TestHostExecAttributionRefusesAmbiguity`, `TestHostCleanupIndependentAndExact`, `TestHostStreamCloseJoinsCancellation`, simulated lifecycle cases in `TestPairedAuthorSourceConformance` |
| PCH-V0-014 | `launcher.go` | `TestHostConfiguredBoundaryRejectsDrift`, `TestHostDocumentedInspectSerialization`, `TestHostNativeRecordsShareRetentionBudget` (exact physical tuple pending) |

## Qualification and rollback

The material is experimental reference material, audited only against declared text. Promotion
requires all of the following:
- owner acceptance;
- a hosted dry-run of the #395 replay set;
- `corvint delta` (#389) replacing the delta placeholder;
- a review of each operator's host-level secret scoping.

Rollback: delete copied workflows from the operator repository and disable/remove the optional
`cmd/corvint-postmerge-host-launcher` and paired candidate changes, preserving ownership holds and
failed evidence. Revert task-owned template changes as appropriate. The optional command is not
installed by default and adds no Core command or store migration.

## Paired optional-host candidate contract (PCH-V0-009..014)

The owner-authorized experimental technical contract below carries the reviewed final paired wire.
It preserves PCH-V0-001..008 and adds no default execution or host qualification. Candidate values
and same-object topology here supersede earlier private14GiB/copy/no-root-bind proposals.

## Selected primitive and fixture

One #395 trusted supervisor retains ownership for the entire invocation. It decodes the actual
native intake/declaration, starts a pinned #398 launcher, observes readiness, takes actual native
Snapshot/Preflight on ONE host worktree, releases the approved author, joins/cleans it, then performs
actual Verify on that SAME worktree. Raw facts/unknowns remain original. Fresh trusted validation
independently reconstructs immutable product and admitted input, verifies the original complete
scope/host graph and authored content, and runs the actual fixture test.

The single selected executable-after-build fixture is paired-add-fixture/. #395's original Add
case governs it. The reviewed Add fixture defines offline author/main.go and a negative subtraction control.
Its source is retained under `internal/postmergehost/testdata/paired-author`; the source test builds
and executes the author with actual native intake.Decode from the owning module. It retains the
original declaration, Host, Before and Preflight across actual author execution and runs native
Verify on those same canonical objects before semantic tests. A semantics-preserving guarded
calc.go mutation produces GUARDED_WRITE; restoration passes. Host confinement booleans in this
source test are assertions, and every native unknown remains in the original receipts. This
local source proof does not establish a physical execution boundary. Fixture setup creates actual private Git base/merge objects from base/ and
merged/, records their real OIDs and constructs the closed candidate at runtime. No placeholder
OID or hand-labelled successful author-input file is accepted. Actual BuildAuthorInput produces
all author input. tests_claimed=[] is truthful because the new test does not exist in either input
commit; calc.go ADD is the verified source claim. Expected conformance outcomes stay outside author.

Author reads admitted closed input and actual immutable calc.go, parses its Add signature/addition
expression and generates docs/add.md plus tests/add_test.go, including Add(2,3)==5. It emits no
verdict/connector artifact. Actual native scope/env observations must pass with ALL unknowns
retained; merged-source go test must pass. The SAME actual authored test bytes against subtraction
in a separate trusted control root must fail. This is author/scope/trusted-validation conformance,
not historical labels, actual delta, raw-reader semantics, NEA accepted proof, drafts or recording.
The earlier Normalize candidate is retained UNSELECTED and creates no second work request.

## Shared exact types and bounds

FileRef exact keys: path:string, sha256:string, bytes:integer. Host path is clean absolute <=512B,
sha256 is exactly64 lowercase hex, byte count is nonnegative and matches an actual regular,
non-symlink/single-link bounded snapshot. References never confer authority by themselves.
Intake is <=1MiB and decoded by actual intake.Decode/BuildAuthorInput; native state/declaration/host
records retain their existing exact native schemas, hashes and limits. This candidate wire is <=4MiB,
depth<=64; at most32 raw references and16MiB retained conformance facts/log/output per case. Overflow,
truncation, incomplete retention or uncertain cleanup returns BLOCKED; no success with partial bytes.
Candidate fixture uses two preexisting native writable directory scopes and at most two new4KiB
regular files. Limits do not claim a hard total-filesystem quota for arbitrary future authors.

Profile exact top-level keys:

    profile, mode, implementation, engine, image, admission, command, limits

- profile = corvint-postmerge-author-host/0; mode = candidate-qualification for this slice.
- implementation = {source_commit, source_tree, launcher_sha256, guest_shim_sha256}, real build pins
  after build. Guest shim is the same optional launcher source built for Linux, a fixed internal
  execution-envelope role, not a new adapter or public Core command.
- engine = {context, client_sha256, server_version, platform}; existing explicit context, no startup
  or context change. Platform=linux/amd64; actual host/kernel/controller facts are retained separately.
- image = {manifest_digest, config_digest, environment, hold_environment}; digest-qualified image. environment is
  the complete separately reviewed image-config key/value/class array (nonnull, classes NONE only).
  Unknown/credential-bearing/image-derived extras block; it is not inferred from passed flags.
  hold_environment is the separately pinned COMPLETE effective inert-process environment, including
  any daemon-added public values; observe it before ready and require exact equality/NONE classes.
  Set fixed hostname corvint-author; no unknown parent environment becomes implicitly safe.
- admission = {product_base, product_merge, product_tree, declaration_sha256, admitted_input_sha256,
  author_sha256}; independent #395 operator/run admission, never author- or fixture-selected.
- command = {executable, argv, working_directory, environment}. working_directory is exactly
  /product; executable is exactly /tools/author. argv is exactly
  [/admitted/author-input.json,/product] for this fixture. environment is a nonnull ordered array
  of {key,value,class}, exactly HOME=/private/home, TMPDIR=/private/tmp, PATH=/usr/local/go/bin:/usr/bin:/bin,
  each class=NONE. This is the COMPLETE effective author process environment, not only override
  arguments. No passed-through host/model/forge/tracker/Actions token, dynamic env or env-file.
- limits = {cpus_nanos,memory_bytes,swap_bytes,pids,shm_bytes,private_tmpfs_bytes,
  author_timeout_seconds,engine_timeout_seconds,cleanup_timeout_seconds,output_bytes}; proposed
  candidate values2e9,8GiB,8GiB,256,1MiB,16MiB,30,30,60,16MiB, respectively.

Exact allowed native declarations and source/input/tool identity are per-run independently pinned
admission inputs. Ephemeral path/run/profile facts are retained EXACT; no new equality/normalization
rule is added for future /1 connector recordings. Operational promotion and generalized author
profiles require their own actual qualification; current mode does not imply either.

Request exact top-level keys:

    profile, run_id, operator_manifest, product, declaration, observer, admitted_input, author, exec_shim, output_root

- profile = corvint-postmerge-author-request/0; run_id =32 lowercase hex generated by supervisor.
  Native SessionID MUST be run_ + run_id, ObserverID observer_ + run_id, CapabilityID authoring
  and Author.ID author. Validate actual native ASCII-letter-first grammar/length. Never pass a
  digit-first hex ID unmodified into native records or relax the native decoder.
- operator_manifest: FileRef to original independent #395 run configuration, never author-accessible.
- product = {root,base,merge,tree}; actual canonical host root and independent full native Git pins.
- declaration: FileRef to COMPLETE corvint-step-declaration/0. Decode with actual native decoder.
  Its Author.Root MUST equal product.root, full pins must match admission/source, and its native
  canonical declaration digest must equal profile.admission.declaration_sha256.
- observer = {id,git_binary}; native ASCII observer ID and FileRef of pinned real host Git executable.
- admitted_input, author: FileRefs, exact admission digests. Author is one approved compiled program.
- exec_shim: FileRef of the actual Linux guest execution-envelope bytes, hash equal to independent
  profile.implementation.guest_shim_sha256. It cannot select an author or change profile settings.
- output_root: one fresh private operator-chosen path outside product, inputs/tools/Git admin/
  observer authority and source. It MUST equal CLI --out after strict canonical parent resolution,
  rejecting symlink/alternate spelling/alias; neither destination may already exist. Launcher creates
  that ONE directory exclusively,0700, and its pending ownership record exclusively,0600. It stores
  controller evidence and original native records, never mounted inside author. Launcher owns
  engine/ownership/log subtrees; trusted supervisor writes actual native Host/Before/Preflight to
  the reserved native/ subtree only after ready. Every RETAINED event/control FileRef must stay
  within this exact evidence root; original approved input refs remain distinct external inputs
  and any retained input copies use inputs/ here. No second unspecified evidence/ownership root.

No request contains engine flags, mount options, arbitrary command/argv/env, narrower post-author
scope, qualification boolean, accepted/verified verdict, source item, connector input or URL.
All array/object members are required; duplicates, case aliases, unknown fields, null arrays,
noninteger counts, malformed Unicode, path/hash/binding mismatch and aliasing fail closed.

## Same objects and fixed mount derivation

Derive topology ONLY from the pinned complete native declaration; do not accept a second mount list.
For this fixture Author.Root -> /product is read-only. Its exact existing docs/ and tests/ directories
are nested RW binds of Author.Root/docs and Author.Root/tests, not copies. docs/.keep and tests/.keep
are native guards and nested RO bindings of their SAME host objects. calc.go/go.mod remain under
RO product. All native admin paths, including actual git/common dirs and .git path, are masked or
absent from guest view; actual host Git/admin files remain available only to native trusted observers.
A source directory containing another sensitive alias is refused, not covered by a lexical mask.

Mount admitted_input -> /admitted/author-input.json, author -> /tools/author and exec_shim ->
/tools/exec-shim read-only, with exact original pinned bytes.
Private home/tmp are fresh bounded guest tmpfs. No raw/labels/operator/observer/controller/output,
runner home/temp, GitHub files, engine/API socket, devices or shared-writer mount exists.
Only the intentional RO-root -> exact-RW-scope -> RO-guard nesting is allowed; every other overlap,
symlink/hardlink/alias or guarded descendant bypass is refused. Preparation of fixture permissions
happens before native Snapshot and is retained; the author never changes host directory ownership.

Native before/after ALWAYS run against the SAME canonical host root and host objects. Docker logical
paths/inodes are retained as mapping facts, never substituted for native host identities. No clone,
collection of a tmpfs product, stripping identity hash or comparison across independent filesystems
is used. New files in docs/tests become actual host files through those binds; original root/scope
ancestor identity/mode and admin state must remain as native Verify requires. File operations and
actual mount enforcement are qualified after build; this contract can be accepted before that proof.

## Single owned launcher lifecycle and exact messages

Proposed invocation:

    corvint-postmerge-host-launcher --profile FILE --profile-sha256 SHA --request FILE --request-sha256 SHA --out FRESH_PRIVATE_DIR

It remains alive under #395 supervisor ownership until execution/cleanup finishes; no detached
handle or caller-owned native attempt is created. Write an exclusive pending ownership record BEFORE
engine create, naming expected unique container name/label/run/profile/request and controller PID/start
identity. No resource can be forgotten because create returned its CID late. Existence/conflict or
uncertain prior resource blocks; no automatic identity retry or broad engine cleanup.

Create/start ONLY this inert image-pinned hold process: Entrypoint=[/bin/sleep], Cmd=[infinity],
WorkingDir=/, User=65532:65532. Inspect these exact predicates, actual requested manifest/config IDs,
complete image Config.Env against profile.image.environment, no image volumes, NetworkMode=none,
ReadonlyRootfs=true, Privileged=false, CapDrop=[ALL], CapAdd=[], only no-new-privileges security opt,
private PID/IPC (never host/container-sharing), RestartPolicy=no, no ports/devices/API socket,
exact profile resource limits and complete derived mount/tmpfs topology. Inspect raw facts remain
retained. The existing PR-container sleep/infinity code is a design lead, not reused author proof.
Inspection serialization may differ from the create request only for reviewed harmless defaults:
ConsoleSize is accepted at exactly [0,0] for this non-TTY candidate; bind Consistency may be empty,
ReadOnly may be omitted only for an expected writable mount, and the documented CreateMountpoint,
NonRecursive, ReadOnlyNonRecursive and ReadOnlyForceRecursive bind fields may be omitted or false.
Null mount elements refuse before normalization. Source/target, propagation, read-only authority,
unknown mount options and nondefault privilege/runtime settings remain exact refusals. The retained
Engine v1.56 response fixture is constructed documentation evidence, not an observed daemon reply.

Image missing, configured mismatch, extra/unknown image env or incomplete enforcement facts blocks;
image provisioning is an explicit qualification step after root admission, never a launcher pull.

The controller resolves only the independently pinned existing Docker context to a canonical local
Unix socket. It accepts no caller-selected URL/socket, TCP/SSH/TLS transport, proxy/redirect fallback,
engine startup, context switch or image pull. Retain the actual server API minimum/maximum and
selected compatible version (candidate implemented range 1.44 through 1.56), server platform and
client bytes. The profile requires an actual Linux/amd64 server; controller architecture alone or
an emulated image does not discharge that predicate.

Fixed controller-only Engine API routes create the shim exec, start its bidirectional stream and
inspect that exact exec ID. Before attributing its eventual exit to the author, surround one fixed
GET /containers/{owned-full-ID}/top with Running exec inspections of the same exec/container/PID.
The bounded table must have exact PID/COMMAND titles, unambiguous canonical PID rows and the exact
post-exec author argv. This observes process arguments, not authenticated executable/source
provenance; independently pinned read-only tools remain necessary. PID reuse/races and fast exit
without the actual start join stay UNKNOWN/HELD, including exit zero. ExecInspect provides no
signal field; retain signal=null and never decode 128+N into an author signal. Client/shim failure,
truncated or disconnected streams, late completion and uncertain cleanup remain separately retained.

Before ready, start only the PINNED trusted guest execution-envelope shim, under the same approved
UID, fixed /product working directory and an explicitly cleared environment. The shim observes its
COMPLETE actual os.Environ key/value-hash list before any author exists; compare exact names/value
hashes to complete profile.command.environment and native declaration classes. No unseen/image
derived/additional key may become observed NONE by inference. Unknown or incomplete observation
blocks. The original raw observation contains names/digests only, no unknown credential value.
The shim waits on a private supervisor barrier and executes no author until execute control.
On release it replaces its own process image with the single pinned author via direct exec, passing
EXACTLY that observed environment/argv and replacing barrier stdin plus marking non-log descriptors close-on-exec
(author stdin=/dev/null). This preserves the observed execution envelope; subsequent author logs
are raw logs and cannot be reparsed as environment-ready/authority frames. Actual runtime enforcement
proof and negative injected-image-env/extra-effective-env controls follow the build. Image hold-env
is separately actually observed and must equal profile.image.hold_environment with no unknown/
credential class because it may be guest-readable; neither Config.Env nor override args stand in
for that observation.

Event exact top-level keys:

    profile, event, run_id, request_sha256, profile_sha256, container_id, facts, author_exit, cleanup, reasons

profile=corvint-postmerge-host-event/0; event=ready|finished|held. container_id is actual full engine
ID or null if unproduced. facts is a nonnull ordered array of {kind,record}, record=FileRef and
kind=create|inspect-ready|hold-environment-ready|environment-ready|exec|inspect-exit|cleanup|ownership. Each original preimage stays retained.
author_exit=null before confirmed actual author execution. Otherwise exact fields {code,signal,
timed_out}: code is null or integer0..255, signal is null or an actually observed OS signal name,
timed_out is boolean from the author supervisor clock. Unknown status stays null with explicit
reason/held result; never infer a signal from an encoded128+ exit or a client kill. Engine-client/
controller/guest-shim exits belong in separately attributed raw facts (process_kind and exact
PID/exec identity), not author_exit. Controller exit is observed by #395 after joining it, not
predicted by the launcher before it exits. cleanup=NOT_RUN|REMOVED|HELD; reasons is a nonnull
array of fixed codes. No event says native stage observed/verified/accepted or grants qualification.
ready requires complete checked actual configured boundary but author_exit=null/cleanup=NOT_RUN.
finished requires actual execution/join plus exact resource removal observations; unresolved facts
produce held/BLOCKED. Native Host assertions/unknowns are not overwritten by these event names.

On ready, #395 derives the COMPLETE existing native Host from the original observed/configured
boundary and real Git pin. Host.Environment MUST come from the actual environment-ready AUTHOR
execution-envelope observation; never from the separately observed inert/image environment or
only supplied overrides. Preserve missing observation as NULL (unobserved), never observed[],
and block this positive fixture when the author envelope is incomplete. Retain all native
unauthenticated/external/transient/concurrency/admin limits. Actual Snapshot and Preflight then run.
Supervisor sends one control frame over private stdin; that channel is never inherited by author.
Control exact keys: profile,operation,run_id,request_sha256,profile_sha256,host,before,preflight,reason.
profile=corvint-postmerge-host-control/0; operation=execute|cancel. Execute uses FileRefs to the actual
full native Host/State/Preflight originals and reason=null; cancel uses those refs=null and one
fixed reason. Control profile_sha256 MUST equal the exact CLI-pinned profile digest and the independently
verified operator-manifest binding; run/request identities must also agree. Ref digests/native
binding must agree; no author-supplied accepted flag or reduced
scope is admitted. Repeated/out-of-order/stale control blocks. EOF/timeout/SIGINT/SIGTERM triggers
bounded cleanup and retained supervisor-lost/cancel facts, never an unowned running author.

Only after execute does launcher run the single pinned program/argv/minimal env, no shell text.
Exit/log bytes and raw execution facts are bounded. Independent cleanup remains available after
cancelled execution context: join/retire descendants, stop/remove only the exact owned container,
verify absence and controller quiescence. Daemon unavailable, late-create ambiguity, identity conflict,
survivor, incomplete collection or failed removal retains ownership/cleanup hold and BLOCKED.
#395 takes after state and native Verify only after author retirement, using original before/Host/
declaration, then independently collects permitted actual content. Docker/launcher exit alone never
replaces native proof. SIGKILL cannot promise a callback: durable ownership plus supervisor-side
reconciliation of the exact run identity is required; uncertain state cannot admit a replacement.

## Fresh trusted validation and Gate A / qualification boundary

The fresh trusted runner independently rederives product/base/merge/tree, admitted input, complete
native scope and operator profile. It verifies original graph preimages and host/control-plane tuple
rather than trusting uploaded before-state or launcher JSON. Exact authored paths/modes/blobs must
agree with native after state and allowed scopes. Then apply only verified content to its fresh
private verification tree and run pinned actual go test with local/offline policy. Reusing these
same authored test bytes against the independent subtraction control must fail. Keep all raw logs,
exit status, before/env/after/native findings/unknowns and kernel/mount/process/cleanup facts.

Implementation admission follows the independently reviewed paired contract. This experimental seed
contains no launcher, runtime qualification or whole-replay evidence. Actual source/tool/image/client/
kernel/request/job facts are produced only by admitted implementation and qualification. Missing
implementation is subsequent work, not a prerequisite to accepting the technical contract.
Original full #395/#398/#388 acceptance remains open; exact #394 Body/repeat timing/raw graph,
private per-run connector object verification and deterministic Evidence URL remain separate
positive draft obligations. No normalization or synthetic stage result is admitted.

## Current candidate qualification limits

The nineteen-file source candidate passed independent source review after two repairs. That verdict
covers source evidence only. The read-only observed local tuple has a macOS arm64 controller and
Colima Docker29.5.2 Linux arm64 server, kernel6.8.0-117-generic, API min1.40/max1.54; it conflicts
with this candidate's Linux/amd64 server predicate. The proposed immutable golang image digest
sha256:eef6a67266eeed3c86dd47fd01b32faa8bf0229eb83eb3d4d466e80391bd3820 is absent on that daemon.
Existing private build artifacts precede the final source repair and do not pin the current source.
No container, image pull, physical author execution or hosted replay has been performed.

The positive #395 paired supervisor/consumer is unavailable; its delivered slice still refuses
before authoring, and its human-owned contract fork is unresolved. The source test's actual author
and same-object native Verify do not substitute for that consumer. Independent physical boundary
qualification requires a separately admitted compatible tuple, exact current build/image/environment
pins and lifecycle negatives. Actual paired and historical replay require the real supervisor,
#389 delta, #394 exact raw report/repeat semantics, native connector graph and deterministic Evidence
URL joins. No normalization or reconstructed chronology is accepted in place of those originals.

The six frozen terminal checks, CEM/OCM binding, publication/integration and native completion are
separate remaining delivery steps. Raw source tests/review and a source PR cannot close whole #398.
Preserve proposed/experimental status and all ownership/cleanup holds until actual qualification.
