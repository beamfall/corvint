# Human Documentation Compiler V0

Owner: Russell Lewis
Frozen candidate: 2026-08-24
Contract review: independently re-reviewed 2026-09-13 (`docs/reviews/hdc-v0-rereview-2026-09-13.md`);
amendments applied by decision 0209; capsule and renderer clauses deferred
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: partial (HDCV0-023..029, HDCV0-041)
First qualified profile: Material for MkDocs
Authoritative inputs: `docs/PRODUCT.md`, `docs/DOGFOOD.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/applied-intelligence-breakthroughs-v0.md`, and
`docs/specs/use-case-conformance-v0.md`

External profile authority consulted at freeze:

- MkDocs configuration and 1.6 validation documentation:
  `https://www.mkdocs.org/user-guide/configuration/`;
- MkDocs build command documentation:
  `https://www.mkdocs.org/user-guide/cli/#mkdocs-build`;
- Material for MkDocs site creation and configuration:
  `https://squidfunk.github.io/mkdocs-material/creating-your-site/`;
- Material for MkDocs search, privacy, and offline plugin documentation:
  `https://squidfunk.github.io/mkdocs-material/setup/setting-up-site-search/`,
  `https://squidfunk.github.io/mkdocs-material/plugins/privacy/`, and
  `https://squidfunk.github.io/mkdocs-material/plugins/offline/`.

These external pages explain the profile but do not pin a project environment. A delivery receipt
MUST discover and bind the exact locally installed MkDocs, Material, Markdown, PyMdown Extensions,
theme, plugin, and hook environment actually used.

## Agent digest
- Claim: Human Documentation Compiler proposes evidence-bound operational plans whose rendered views cannot strengthen source claims.
- Status: accepted (decision 0047, 2026-09-04)/partial (HDCV0-023..029, HDCV0-041)
- Exists: the amended contract, the `HDCV0-023..026` clause admission and prose templates (decision 0229), the plan-only `HDCV0-027..029` admitted plan wire with `create_file`/`insert_after` operations (decision 0231), the `HDCV0-041` canonical JSON verifier and emitter, and the experimental planning boundary.
- Blocked on: implementation of the remaining pure clauses and pinned renderer qualification (renderer and capsule qualification `NOT_RUN`).
- Read next: Product claim and measurable job; Trust and authority model; Numbered requirements.

## Product claim and measurable job

For a maintainer responsible for a repository-owned Material for MkDocs site, Corvint compiles
revision-pinned evidence into a reviewable Markdown and navigation patch plan, validates the
candidate site with the repository's already-pinned MkDocs environment, and returns exact
uncertainty and local receipts. It never silently changes accepted prose or treats a successful
static-site build as proof that a behavioral claim is true, current, complete, or offline-safe.

The P0 measurable job is: against a frozen repository source state, produce a deterministic draft
whose every behavioral clause is `SUPPORTED`, `CONFLICTED`, or `UNKNOWN`; whose proposed file and
navigation operations have exact byte preconditions; and whose isolated candidate build truth is
reported separately from its offline/privacy truth. A reviewer can accept, reject, or edit the
proposal without Corvint having modified the repository.

The simpler baseline is unconstrained LLM summarization followed by a maintainer running
`mkdocs build --strict`. It is insufficient because it can fabricate behavior, hide contradictions,
misread MkDocs YAML, omit configuration-owned behavior, overwrite accepted prose, use an unpinned
environment, or call a strict build and offline behavior equivalent.

## Verified current state at freeze

- `docs/PRODUCT.md` commits to cited, paragraph-anchored human documentation suitable for MkDocs or
  another repository-owned site and forbids silent overwrite of accepted prose.
- `docs/specs/applied-intelligence-breakthroughs-v0.md` marks the Living Behavioral Spec and Human
  Documentation Compiler `not-started`. Its first required artifact is an evidence-anchored MkDocs
  draft corpus; its outcome gate remains 100 modules and 50 shipped changes.
- `docs/specs/use-case-conformance-v0.md` reports every governed job, including MkDocs and other
  human documentation, as `UNPROVEN`; Corvint and Beamfall dogfood plus hostile-test and sealed-
  benchmark receipts remain mandatory for promotion.
- At this freeze the Corvint repository contains no `mkdocs.yml`, `mkdocs.yaml`, project-owned MkDocs
  executable, Material environment, or MkDocs dependency lock. Therefore Corvint Material build
  delivery and `OFFLINE_QUALIFIED=PASS` are `NOT_OBSERVED`, not implied by this specification.
- Current MkDocs documentation says `mkdocs build` generates a static site and MkDocs 1.6 exposes
  validation settings whose warnings become fatal under `--strict`. Current Material documentation
  requires `theme.name: material`, gives `site_url` build/plugin significance, and says built-in
  search is enabled by default but MUST be explicitly re-added when a `plugins` list is configured.
- Material navigation features, Markdown/PyMdown extensions, extra CSS/JavaScript, theme overrides,
  hooks, privacy, and offline plugins are configuration-owned behavior. Their mere presence does
  not prove they are safe, offline, or semantically understood by Corvint.

At that freeze no implementation, conformance result, Corvint corpus, Beamfall shadow result,
independent review, or external outcome result was claimed. The 2026-09-13 independent re-review and
any later delivered requirement IDs are recorded in the header and Traceability; nothing else is
claimed.

### Experimental planning boundary (2026-09-06)

The current `internal/doccompiler` planning API has no immutable evidence/acceptance verifier or
external offline observer. It therefore admits only structured `UNKNOWN` claims with explicit
uncertainty and no evidence anchors. `SUPPORTED`, `CONFLICTED`, caller-authored `Markdown`, and
unverified evidence are refused rather than presented as qualified documentation. Caller text is
rendered as literal data. Config and existing-document source pins remain byte preconditions for
isolated proposals; they do not authenticate behavioral claims. Receipt construction, verification,
and canonicalization reject every offline `PASS` until a real observer integration exists.

These restrictions close experimental API bypasses of `HDCV0-023..026` and `HDCV0-038/042`; they do
not weaken those requirements or implement the full compiler. `HDCV0-023..026` are delivered
separately by the pure admission contract in `internal/doccompiler/admission.go` (decision 0229), and
`HDCV0-027..029` for the plan-only variant by `internal/doccompiler/admittedplan.go` (decision 0231).
The experimental `Plan` and its `PatchPlan` still refuse `SUPPORTED` and `CONFLICTED` claims. `PatchPlan`
carries its own `corvint-human-documentation-experimental-patch-plan/0` profile (decision 0240); its bytes
are not the `HDCV0-027` wire, `VerifyAdmittedPlan` refuses them, and `internal/docviews` binds the
admitted plan instead (`CATN-V0-001`). The accepted delivery and promotion gates remain unchanged. `internal/doccompiler/integrity_test.go` exercises unadmitted prose, forged
revision/authority, rendering boundaries, stale source pins, and unsupported offline qualification.

## Relationship to continuous documentation maintenance

[`deployment-neutral-index-platform-v0.md`](deployment-neutral-index-platform-v0.md) owns the future
continuous-maintenance direction: source quarantine, durable claim/provenance records, citation and
reverse-dependency invalidation, read-only source repositories, documentation-repository changes
only through reviewed pull requests, and separately publishable artifacts.
This HDC V0 remains the first isolated Material-for-MkDocs renderer/compiler profile. Its frozen
request, containment, validation, and delivery gates are not weakened. Authority is explicitly
scoped: accepted owner intent governs prescriptive contracts; pinned source/tests govern descriptive
present behavior; disagreement is retained as drift with both anchors. The platform direction does
not claim a renderer, documentation PR, publication, or continuously current documentation has been
delivered.

## Resolved interpretations and preserved uncertainty

1. **Portable compiler versus one site generator.** The core clause, evidence, patch-plan, receipt,
   and resource models are portable. Owner direction selects Material for MkDocs as the first and
   only fully qualified P0 execution profile. Another renderer is `UNSUPPORTED`, not approximately
   treated as MkDocs.
2. **YAML parsing versus MkDocs authority.** A bounded lexical pass may reject dangerous paths,
   tags, aliases, or unsupported constructs before execution. It MUST NOT claim to resolve MkDocs
   semantics. Only the pinned MkDocs configuration loader and build command in the project-owned
   environment may establish the effective `docs_dir`, `nav`, plugins, theme, validation, and build
   boundary. An arbitrary YAML library or hand-written Go parser is not equivalent.
3. **Strict build versus offline site.** `BUILD_STRICT_PASS` and `OFFLINE_QUALIFIED` are independent
   truth axes. Exit zero from `mkdocs build --strict` cannot set the latter. Static URL inspection
   cannot prove that executable JavaScript, a plugin, a hook, or a theme override will not access a
   network.
4. **Documentation generation versus acceptance.** P0 writes only an isolated proposal and local
   evidence receipt. Existing repository prose is treated as human-owned, even when Corvint believes
   it is stale. Exact patches are applied only to the isolated candidate snapshot for validation;
   no P0 operation applies them to the repository, changes accepted `nav`, creates a branch or PR,
   commits, pushes, deploys, or merges.
5. **Material privacy/offline features versus Corvint policy.** Corvint records their exact resolved
   state and effects; it never enables, disables, or rewrites them. `privacy.assets_fetch: true`
   expresses build-time fetch intent and is forbidden in P0. Disabling privacy asset handling does
   not make external assets local. The offline plugin changes generated-site behavior and is not a
   substitute for observed network denial.
6. **Portable process code versus portable containment.** The data model and pure compiler MUST
   build on supported Go targets. A platform without verified descendant-process cleanup and the
   required external network-denial harness is not a fully qualified execution tuple. It MUST
   report the missing observation instead of inheriting another OS's claim.

## Definitions and independent truth axes

- **accepted prose**: any tracked documentation byte not introduced in the isolated proposal;
- **candidate snapshot**: a bounded, symlink-free copy of the pinned source inputs plus only the
  proposed operations, outside both repository worktree and Git directory;
- **clause**: one independently reviewable assertion with one epistemic state and one or more exact
  evidence anchors or an explicit frontier;
- **configuration authority snapshot**: the closed, redacted record returned by the pinned MkDocs
  loader after pre-execution safety checks; it is not a generic YAML object;
- **environment pin**: exact identities of the allowed environment root, MkDocs entry point and
  interpreter, installed distributions, project lock or manifest, and platform tuple;
- **evidence anchor**: repository-relative path, immutable Git blob identity, one-based inclusive
  line span, exact span SHA-256, authority class, and inclusion reason;
- **navigation patch**: a proposed byte edit to the exact config source that owns effective `nav`;
  it is not a second navigation model maintained by Corvint;
- **project-owned environment**: an explicit local environment root and pin approved by repository
  policy before the run; ambient `PATH`, user site packages, and a newly installed dependency do not
  qualify;
- **remote fetch-bearing reference**: an output or config reference capable of loading a network
  resource, including script/style/image/font/media sources and CSS `url()`/`@import`; an ordinary
  hyperlink or canonical `site_url` is recorded separately and is not automatically fetch-bearing;
- **source identity**: Git revision plus the ordered path, mode, Git blob identity where available,
  and SHA-256 of every consumed byte;
- **strict candidate build**: exactly one bounded, clean, isolated invocation of the pinned MkDocs
  authority with `build --strict`, the copied config, and an explicitly isolated `site_dir`.

Clause state is exactly:

- `SUPPORTED`: accepted intent or mechanically/observationally qualified evidence directly anchors
  the clause at the frozen source identity;
- `CONFLICTED`: two qualifying sources disagree and both remain visible;
- `UNKNOWN`: no qualifying source establishes the clause, an anchor is stale, or the bounded
  resolver cannot decide. Inference never upgrades `UNKNOWN`.

Build state is exactly `PASS`, `FAIL`, or `NOT_RUN`. `BUILD_STRICT_PASS` is true only for `PASS`.
Offline state is exactly `PASS`, `FAIL`, `NOT_OBSERVED`, or `UNKNOWN`.
`OFFLINE_QUALIFIED` is true only for `PASS`. These vocabularies MUST NOT be merged into a general
success, confidence, readiness, or support score.

## P0 inputs and ownership boundary

The P0 invocation accepts one repository, one frozen source identity, one config path, one approved
Material environment pin, one documentation intent, one output root, and explicit limits. The
intent names target documentation paths, desired audience, and claim questions; it does not provide
authority merely by being in the request.

The request profile is `corvint-human-documentation-request/0`. It is closed: unknown or duplicate
fields fail. Its logical fields are:

| Field | Contract |
|---|---|
| `profile` | literal profile above |
| `repository_root` | absolute lexical root; resolved once and never emitted in a publishable artifact |
| `revision` | immutable 40- or 64-lower-hex Git commit available locally |
| `config_path` | repository-relative `mkdocs.yml` or `mkdocs.yaml`; no absolute path or traversal |
| `environment` | approved root, MkDocs entry point, interpreter, exact expected distribution versions, and lock/manifest SHA-256 |
| `intent` | target paths, audience, questions, and optional accepted requirement IDs |
| `output_root` | absolute, initially absent or empty, outside worktree and Git directory |
| `limits` | values no greater than the P0 maxima below; omitted values use those maxima |
| `offline_observer` | absent, or an explicit external denial-harness identity and result channel; never a shell fragment |

The core MAY consume accepted intent, code, tests, public contracts, examples, CEM/OCM evidence, and
pinned execution receipts. Project authority order follows `docs/SPEC-DRIVEN-DEVELOPMENT.md`.
Generated documentation, source shape, commit messages, and model output cannot establish accepted
intent. Dirty or uncommitted evidence may be shown as an observation bound to SHA-256, but it cannot
create `SUPPORTED` accepted behavior and prevents promotion evidence.

P0 output ownership is limited to the explicit isolated output root. The repository worktree, Git
directory, index, refs, object database, configuration, environment, dependency caches, user home,
and external services are read-only or out of scope. The output root MUST NOT be inside any source,
docs, theme override, asset, environment, or Git path.

## Requirements

This section freezes `corvint-execution-capsule/0`. It is the only production execution profile for
P0 and is normative over the older host-environment wording below. The plan-only compiler remains
portable and read-only. Direct host `Discover`/`Build`, Python, MkDocs, plugins, hooks, themes,
extensions, and project code are synthetic-test seams only and MUST NOT be reachable from a
production command. Until every field and observation required below is available, the result is
`NOT_RUN` with `EXECUTABLE_UNSUPPORTED` and no process start. There is no host fallback.

For `environment.kind=linux-docker-capsule`, this amendment supersedes the listed clauses exactly:

| Superseded clauses | Normative replacement |
|---|---|
| `HDCV0-001..004` | `HDCV0-CAP-001..004` |
| `HDCV0-031..036` | `HDCV0-CAP-005..010` |
| `HDCV0-040` | `HDCV0-CAP-011..013` |
| `HDCV0-051` | `HDCV0-CAP-014` |

The superseded text remains applicable only to decoding historical artifacts and synthetic tests;
it cannot authorize native host execution or weaken this amendment. `HDCV0-041` is not superseded:
it defines canonical JSON for every environment kind, including the capsule clauses that cite it
(decision 0209).

`EXECUTABLE_UNSUPPORTED` applies only to an execution variant. A `{"kind":"plan-only"}` request
starts no process, is never refused as `EXECUTABLE_UNSUPPORTED`, and any receipt that binds its plan
records build `NOT_RUN` and offline `NOT_OBSERVED`. Its output artifact set is fixed by the CLI
bridge change, not by `HDCV0-031` or `HDCV0-CAP-012`.

The execution capsule is an opt-in capability outside the default local product (AGENTS.md
invariant 7): it depends on a separately installed broker, runner, systemd template, and dedicated
Docker Engine, none of which the default install contains or starts. Without that installation the
default binary exposes only the plan-only variant. Capsule delivery is deferred until the unresolved
owner decisions below are closed and its runtime tuple is qualified.

### Closed request and environment union

- `HDCV0-CAP-001`: `corvint-human-documentation-request/0` remains the request profile. Its
`environment` member is a closed tagged union. `{"kind":"plan-only"}` is the complete plan-only
variant and grants no executable authority. The execution variant has `kind` equal to
`linux-docker-capsule`, `profile` equal to `corvint-execution-capsule/0`, and exactly the members
`broker`, `launcher`, `systemd`, `daemon`, `image`, `runner`, `toolchain`, `project_lock_sha256`, and
`transitive_artifacts`. Unknown or duplicate members, a null required member, a number outside the
signed 64-bit integer range, or a non-canonical request fails before filesystem or executable
acquisition. Strings are UTF-8, contain no NUL or C0/C1 control, and are bounded to 4,096 bytes
unless a smaller bound is stated. SHA-256 values are exactly 64 lowercase hex characters; image IDs
are exactly `sha256:` plus 64 lowercase hex characters.

Each execution member is itself closed:

| Object | Exact members |
|---|---|
| `broker` | `profile`=`corvint-execution-capsule-broker/0`, absolute root-owned immutable `path`, executable `sha256`, `file_identity_sha256`, `protocol`=`corvint-execution-capsule-broker-wire/0`, absolute fresh `private_socket_parent`, `cleanup_mode`=`exec-stop-post` |
| `launcher` | absolute root-owned `unit_template_path`, `unit_template_sha256`, absolute root-owned `authorization_path`, `authorization_sha256`, numeric authorized `caller_uid`, numeric authorized `caller_gid`, numeric dedicated `service_uid`, numeric dedicated `service_gid`, `engine_fd_name`=`corvint-hdc-engine` |
| `systemd` | `version`=`261`, exact `boot_id`, absolute typed local `varlink_socket`, `unit_prefix`=`corvint-hdc-capsule@`, `invocation_authority_sha256`, and `effective_properties_sha256` |
| `daemon` | broker-only absolute `engine_socket`, `engine_socket_identity_sha256`, exact `server_version`, `api_version`, `operating_system`=`linux`, `architecture`, `kernel_version`, `engine_major`=`29`, `runtime_name`=`runc`, exact `runtime_commit`, exact `containerd_commit`, `rootless`=`false`, `live_restore`=`false`, `cgroup_version`=`2`, `cgroup_events_sha256`, exact `userns_mode`, outer `supervisor_seccomp_sha256`, child `workload_seccomp_sha256`, `lsm_kind`, `lsm_profile`, `lsm_profile_sha256`, `system_mounts_sha256`, `exclusive_authority_sha256`, `exclusive_lease_sha256` |
| `image` | immutable `id`, `config_sha256`, `manifest_sha256`, `operating_system`=`linux`, `architecture`, ordered `entrypoint`, ordered `cmd`, numeric `uid`, numeric `gid`, `environment_sha256`, `labels_sha256`, `healthcheck`=`none`, exact `stop_signal`, `volumes` as an empty array, `package_inventory_sha256` |
| `runner` | `profile`=`corvint-execution-capsule-runner/0`, absolute container `path`, executable `sha256`, unsigned `size` from 1 through 33,554,432, numeric `mode`=`365` (`0555`), `protocol`=`corvint-execution-capsule-wire/0` |
| `toolchain` | exact `python_version`, `python_abi`, `mkdocs_version`=`1.6.1`, `material_version`=`9.7.7`, `markdown_version`=`3.10.3`, `pymdown_extensions_version`=`11.0.2` |
| each `transitive_artifacts` row | normalized distribution `name`, exact `version`, artifact `sha256`; rows sort by UTF-8 `(name,version,sha256)` and duplicate names fail |

The lock digest and complete transitive-artifact set are mandatory even when the four direct package
versions match. Claimed Dockerfile, build recipe, or base-image ancestry is not intrinsic image
identity and may appear only in a separately owner-signed provenance artifact.

The invocation-authority body is canonical JSON with exactly `authorization_sha256`, `broker_sha256`,
`caller_gid`, `caller_uid`, `engine_socket_identity_sha256`, `profile`, `request_sha256`,
`systemd_boot_id`, `unit_instance`, and `unit_template_sha256`. The effective-property body is
canonical JSON with exactly `ambient_capabilities`=[], `capability_bounding_set`=[],
`device_policy`=`closed`, `engine_fd_name`=`corvint-hdc-engine`,
`engine_fd_source_identity_sha256` equal to the admitted Engine socket identity,
`exec_start`, `exec_stop_post`, `group`,
`kill_mode`=`control-group`, `lock_personality`=true, `memory_deny_write_execute`=true,
`no_new_privileges`=true, `private_devices`=true, `private_tmp`=true,
`protect_home`=true, `protect_system`=`strict`, `read_write_paths`, `remove_ipc`=true,
`restrict_address_families`=[`AF_UNIX`], `restrict_namespaces`=true, `runtime_max_usec`,
`send_sigkill`=true, `system_call_filter_sha256`, `timeout_stop_usec`, and `user`. `exec_start` and
`exec_stop_post` name the same broker path and SHA-256; only their literal mode argument differs.
`read_write_paths` contains only the exact run state/private-socket roots. Arrays retain this stated
order. Both SHA-256 fields use the `kind NUL profile NUL canonical-body` rule with kinds
`corvint-hdc-capsule-invocation-authority` and `corvint-hdc-capsule-effective-properties` and profile
`corvint-execution-capsule/0`.

- `HDCV0-CAP-002`: The request additionally binds the existing repository/config/intent/output/limit
members, exact source/candidate-tree digest, exact read-only source mount target, fresh output/state/
temporary mount targets, exact broker Engine request, ordered capsule environment key/value digest,
workdir, runner UID/GID 0 inside the private user namespace, distinct numeric project UID/GID,
resource limits, aggregate deadline, generated-output limits, and label namespace. The broker-only
Engine socket is admitted by the broker unit and is never opened or passed by the invoking Corvint
client; that client receives only the fresh private `0600` broker socket. Run-local nonce, unit name,
container ID, PID, timestamps, and temporary host paths are diagnostic observations, not deterministic
identity. The caller may lower, never raise, a P0 limit. `offline_observer` MUST be absent.

- `HDCV0-CAP-003`: Admission requires a native Linux host, kernel 5.12 or newer, systemd exactly 261
with its typed local Varlink manager API, unified cgroup v2 with readable `cgroup.events`, and one
already-running dedicated and exclusive rootful local Docker Engine 29 reachable only by the
owner-installed `corvint-hdc-capsule@.service` template. The root-owned authorization policy admits
only the bound caller UID/GID, exact unit prefix, exact request digest, and start/stop operations; it
cannot supply executable paths, arguments, properties, environment, or an Engine method. The system
manager opens the exclusive Engine socket and passes exactly one named `corvint-hdc-engine` descriptor
to the dedicated broker UID/GID. Neither caller nor broker opens the Engine path. The runtime is
`runc` with private user/PID/IPC/UTS/cgroup namespaces. Runner
PID 1 is namespace UID/GID 0 with exactly `CAP_SETUID`, `CAP_SETGID`, `CAP_SETPCAP`, and `CAP_KILL`;
the distinct project UID/GID has no capability. Admission binds an outer supervisor-capable seccomp
profile, a stricter child-only seccomp profile, one enforcing AppArmor or SELinux profile,
`no-new-privileges`, no AF_VSOCK authority, and exactly the read-only system-mount inventory plus
bounded tmpfs mounts. Missing, disabled, permissive, unknown, additional, shared, remotely
addressable, rootless, live-restore, authorization-plugin, Swarm, Kubernetes, ambient-context, or
auto-started state is unsupported. Darwin including Colima, Docker Desktop, Windows, FreeBSD, remote
Engines, rootless Engines, and every VM-backed daemon are `NOT_RUN`; none may inherit Linux evidence.

- `HDCV0-CAP-004`: Corvint admits the root-owned static, CGO-disabled broker executable and owner-installed
unit/authorization bytes with component-relative no-follow metadata/open/fstat/second-metadata
checks. The broker file and every parent are not writable by caller, broker UID/GID, group, or other;
its exact device/inode/mode/uid/gid/link-count/size and bytes are bound. Corvint independently parses
it as ELF with neither `PT_INTERP` nor `DT_NEEDED`; dynamic or unrecognized executables are
unsupported and every loader variable is absent. systemd 261 starts one constrained template
instance over its typed local Varlink API; no transient executable, argv, environment, property, or
FD is caller-selected. Corvint reads back and byte-compares the exact invocation/effective-property
bodies before execution authority. `ExecStart` and `ExecStopPost` use the same admitted absolute file
identity and bytes; either path/parent/identity/content change before start, cleanup, or final probe is
`EXECUTABLE_UNSUPPORTED`, triggers already-available cleanup, and cannot fall back. The broker embeds
the cgroup observer and speaks the frozen Engine API directly over only the systemd-passed Unix FD
with Go standard-library HTTP. No Docker CLI or ambient Docker configuration exists. Corvint performs
no installation, resolution, image creation/import/build/pull/push/removal, tag lookup, network
request, or daemon start/reconfiguration. Container create names the exact immutable image ID; the
broker rejects every Engine image-create, `/images/create`, build, import/load, push, and image-delete
method/path before sending bytes.

### Capsule policy, lifecycle, and archive

- `HDCV0-CAP-005`: Corvint stages only frozen manifest bytes plus exact plan operations into fresh host
state outside worktree and Git roots, then rehashes it before create. The broker admits one exact
closed Engine container-create request naming only the exact admitted image ID, with network `none`,
read-only rootfs, drop all
capabilities then add exactly `SETUID`, `SETGID`, `SETPCAP`, and `KILL` for runner PID 1 inside the
private user namespace, `no-new-privileges`, private PID/IPC/UTS/cgroup namespaces, bounded PIDs,
memory with equal memory-swap, CPUs and ulimits, log driver `none`, and no devices, ports, hosts, DNS,
Docker socket, host namespace, privilege, additional groups, healthcheck, restart policy, image
volumes, or ambient image command. The independently pinned runner replaces the entrypoint; it starts
as namespace UID/GID 0 and receives a different bound non-root project UID/GID. The request allowlist
replaces image environment. Source is one recursively read-only bind requiring kernel support for
recursive read-only; state, temp, and output are bounded `nosuid,nodev,noexec` tmpfs mounts. Output
tmpfs is exactly 256 MiB and is the only writable data mount.

- `HDCV0-CAP-006`: Before start, the broker independently inspects and byte-compares effective image
ID, config, command, user, environment, labels, HostConfig, mounts, network/namespace/security
options, resources, seccomp/LSM/user-namespace request, devices, runtime mounts, and restart policy
against the closed request. The dedicated Engine socket is accessible only to the broker unit
identity for the run lifetime; socket owner/mode/ACL and open-file observations prove no second
client. The broker enforces a closed state machine and rejects every unlisted Engine method/path,
including exec, connect, update, pause, restart, rename, and mount. Daemon events are corroboration
only: a gap or unexpected event fails closed but can never establish exclusivity. A file lock alone
does not prove daemon exclusivity.

After create and effective-policy inspection, while the container remains stopped and before any
attach or start request, the broker performs exactly one bounded Engine
`GET /containers/<full-id>/archive?path=<percent-encoded-runner-path>` request. The path is the exact
absolute runner path; no directory, query option, redirect, content-encoding, or second extraction
is allowed. HTTP response headers are at most 32 KiB and the tar body is at most 33,555,968 bytes.
The body contains exactly one POSIX-ustar regular-file row named by the UTF-8 basename of the runner
path, with one 512-byte header, mode `0555`, uid/gid/device/mtime zero, size from 1 through
33,554,432 bytes, correct checksum, zero padding, and exactly two terminal zero blocks. Links,
directories, specials, PAX/GNU/sparse extensions, absolute/dot/dot-dot names, duplicate or trailing
rows/bytes, nonzero padding, a bad checksum, or response-header/stream-length disagreement fails
before attach or start. The broker parses and hashes the file bytes and requires the exact request
runner size/mode/SHA-256; Engine metadata is not executable identity and no other container-content
read is allowed.

Only after that extraction and the complete pre-start inspection, while the container is still
stopped, may the broker establish and verify exactly one broker-owned attach upgrade against the
fsynced full container ID with stdin enabled, bounded stderr, and reserved framed stdout. Only after
that stream is live may it issue the separate exact start request. Every second, external, late,
pre-create, post-finish, or otherwise out-of-state attach is rejected and invalidates the run. The
immediate first runner byte MUST be observed on the already-established stream; a lost, reordered,
or pre-attach runner byte is `PROTOCOL_INVALID`.

The private RPC profile is `corvint-execution-capsule-broker-wire/0`. Its frame layout is the layout
in `HDCV0-CAP-011` with magic `ATLSBRK0`. Corvint-to-broker types are `PROBE=0x01`, `CREATE=0x02`,
`ATTACH=0x03`, `START=0x04`, `RUNNER_INPUT=0x05`, `CANCEL=0x06`, and `FINISH=0x07`; broker-to-Corvint
types are `RESULT=0x81`, `RUNNER_OUTPUT=0x82`, `OBSERVATION=0x83`, and `ERROR=0x84`. PROBE, CREATE,
ATTACH, START, CANCEL, FINISH, RESULT, OBSERVATION, and ERROR payloads are closed canonical JSON.
RUNNER_INPUT/OUTPUT each carry one complete unchanged `corvint-execution-capsule-wire/0` frame. One
connection, one writer per direction, and the state order PROBE/result, CREATE/result,
ATTACH/result while stopped, START/result on that live attachment, runner frame exchange, and
FINISH/cleanup result are mandatory; CANCEL may replace the remaining forward path only after
CREATE. A runner byte can never be parsed as broker JSON or an Engine request. Unknown, duplicate,
out-of-state, concurrent-writer, partial, or trailing frames fail closed and enter cleanup after
CREATE.

- `HDCV0-CAP-007`: The runner is the sole initial container process, PID 1, a subreaper, and
non-dumpable. It retains namespace root with exactly `SETUID`, `SETGID`, `SETPCAP`, and `KILL`,
generates one random 256-bit ACK secret in runner-only memory, emits `SUPERVISOR_READY` with its
public challenge, and blocks before any child exists. The broker proves singleton PID 1 before
`RELEASE_SETUP`. The runner then forks one blocked child. While `SETPCAP` is effective the child
drops every capability bounding bit, clears supplementary groups, calls `setresgid`/`setresuid` to
the distinct project identity, irreversibly clears all permitted/effective/inheritable/ambient
capabilities, sets no-new-privileges, installs the bound child-only seccomp filter, and blocks before
exec. The outer filter permits only PID 1's required supervisor operations; the child filter denies
ptrace, process-vm, namespace, mount, and cross-process signal surfaces. P0 makes no `hidepid` or
cross-UID proc-visibility claim. Its narrower protection proof is the distinct UID, runner
`PR_SET_DUMPABLE=0`, zero child capability sets, bound seccomp filters, and hostile child canaries
that must receive denial from `ptrace`, `process_vm_readv`/`process_vm_writev`, `/proc/1/mem`,
namespace, mount, and signal attempts.

The runner emits `CHILD_READY`; the broker then binds PID/namespace/cgroup identity, reads effective
OCI state and `/proc/<pid>/mountinfo`, opens and retains the exact `cgroup.events` descriptor, fsyncs
the cgroup path, inode, controller/mount identity, container ID, and PID-membership witness into the
root-owned run lease, proves `populated 1`, verifies exactly the pinned runner plus one blocked child
and no third process, and verifies the child's UID/GID,
empty groups and capability sets, `NoNewPrivs=1`, seccomp mode/filter count, namespaces, and
non-dumpable separation. Filter identity is the admitted runner's frozen filter bytes/control path
plus hostile syscall canaries, never an invented `/proc` BPF digest. Only a valid `RELEASE_EXEC`
challenge response permits child exec. Project children never inherit protocol descriptors.

After release the runner performs, in order: closed-FD/environment canaries; network canaries;
source/output permission canaries; complete package inventory; MkDocs authority load; exact patch
application in private scratch; `mkdocs build --strict --clean`; generated-asset scan; input rehash;
output freeze/manifest; complete child-tree termination using retained `KILL`; and reaping. PID 1
then drops every remaining bounding/permitted/effective/inheritable/ambient capability including
`KILL`, `SETPCAP`, `SETUID`, and `SETGID`, proves all its capability sets zero and singleton PID 1,
emits `DONE`, and blocks for authenticated ACK. Failure at any barrier kills and fails closed.

- `HDCV0-CAP-008`: Output is streamed while the runner remains alive; no post-stop export or second
Docker request exists. Archive sublimits are 50,000 total entries, 25,000 regular files, 256 MiB
regular payload, and 4 MiB aggregate normalized path bytes. The byte-fixed maximal ustar vector is
exactly 306,811,480 bytes: 268,435,456 payload bytes plus 50,000 512-byte headers plus 25,000 times
at most 511 padding bytes plus two 512-byte terminal zero blocks. The archive can never claim the
combined protocol ceiling independently. It is lexicographically path-ordered POSIX ustar. Entries
are only UTF-8 slash-relative regular files or directories with zero uid/gid/device/mtime, empty
owner/group names, and fixed normalized modes. Links, specials, absolute/dot/dot-dot paths, PAX/GNU
extensions, sparse entries, duplicates after normalization, bad checksums, and bytes after the
terminal blocks are `OUTPUT_INVALID`. Limits are checked while streaming and are cumulative across
attempts.

- `HDCV0-CAP-009`: One 120-second ledger covers admission through artifact persistence. Child stdout
and stderr each retain at most 1 MiB. All retained runner-wire bytes, including frame headers, DONE
secret/length words, receipt, control payloads, and archive, have one exact combined ceiling of
335,544,320 bytes. The receipt plus every non-archive control payload and all frame overhead together
are at most 8,388,608 bytes; the archive has the independent 306,811,480-byte maximum and all
structural sublimits in `HDCV0-CAP-008`. Neither partition may borrow from the other, and both must
fit the combined ceiling. Cleanup has one additional five-second observation budget. No retry resets
a time, byte, file, process, event, Engine-I/O, or cleanup ledger. Auto-removal is disabled. The broker fsyncs the returned full container ID before
acknowledging create. On every return path the live broker or systemd `ExecStopPost` cleanup-only
process uses that ID and lease, never a name or glob. It first kills by full ID, then polls until
`Running=false`, `Pid=0`, and zero process rows. The live broker next proves `populated 0` through
its held descriptor. Cleanup-only instead performs a no-follow reopen and full comparison of cgroup
path, inode, controller/mount identity, container ID, and PID membership against the fsynced lease
before kill, then proves `populated 0` through that identity-verified descriptor. If the exact cgroup
has disappeared before cleanup-only can reopen it, cleanup accepts only stopped, `Pid=0`, zero
process rows, container absence, and cgroup-path absence, and returns `INTERRUPTED` with a non-PASS
result. Only after all pre-remove witnesses pass may cleanup remove the container by full ID with its
run-owned volumes; it then requires container 404 and cgroup-path disappearance. Container or cgroup
disappearance before the pre-remove witnesses is `INTERRUPTED` and non-PASS. Final acceptance also
requires broker template instance, private socket, volume, cgroup, and process-row absence. Failure
to prove the trusted runtime boundary empty is `PROCESS_UNCONTAINED`; residue is reported and
requires a separate explicit recovery action.

After removal Corvint rehashes repository inputs, the staged tree, admitted output archive, installed
broker executable, fixed unit template and authorization policy, systemd identity, and image; it
compares the semantic daemon tuple and again requires the broker template instance, private socket,
container, volume, cgroup, and process rows absent before accepting a receipt. Any drift invalidates
the receipt.

- `HDCV0-CAP-010`: P0 exposes no apply mode. Repository, Git, accepted prose, project environment,
image store, daemon configuration, and external services remain unchanged. The only permitted
external-state mutations are start/stop of the constrained systemd broker-template instance and
creation/removal of its private socket,
broker-mediated create/start/kill/remove of the exact run container and run-owned volumes, fresh
temporary stage/state/output, and explicitly requested local artifact files. Corvint never holds the
Engine socket, starts the daemon, pulls/builds/removes an image, serves, deploys, creates a branch/PR,
commits, pushes, or merges.

### Framed protocol, artifacts, and identity

- `HDCV0-CAP-011`: The runner wire profile is `corvint-execution-capsule-wire/0`. Every frame is exactly
`ATLSHDC0` (8 ASCII bytes), one unsigned type byte, one zero flags byte, two zero reserved bytes, one
unsigned 64-bit big-endian payload length, the raw 32-byte SHA-256 of the payload, then that many
payload bytes. No compression, continuation, interleaving, unknown type, trailing byte, or second
logical channel is allowed. Runner-to-broker types are `SUPERVISOR_READY=0x01`, `CHILD_READY=0x02`,
`DONE=0x03`, and `ERROR=0x04`; broker-to-runner types are `RELEASE_SETUP=0x81`,
`RELEASE_EXEC=0x82`, and `ACK=0x83`. Control payloads other than DONE are canonical JSON under
the canonicalization rules of `HDCV0-041`. DONE payload is exactly the 32-byte one-run secret,
unsigned 64-bit big-endian receipt
length, canonical receipt bytes, unsigned 64-bit big-endian archive length, then exact ustar bytes.
The only success sequence is `SUPERVISOR_READY`, `RELEASE_SETUP`, `CHILD_READY`, `RELEASE_EXEC`,
`DONE`, `ACK`, runner exit. ERROR replaces every later runner frame and forbids success ACK. The
broker-owned attached stdout carries frames only; attached stderr is bounded diagnostic transport;
child stdout/stderr are captured inside the runner and never inherit an attach descriptor. A child
byte on the protocol descriptor, concurrent writer, frame reorder, duplicate, digest/length
mismatch, EOF, timeout, or replay is `PROTOCOL_INVALID`.

The runner generates secret `S` before any child. Its public challenge is
`SHA256("corvint-hdc-capsule-challenge" NUL "corvint-execution-capsule-wire/0" NUL S)` and appears in
SUPERVISOR_READY with the protocol, request digest, and runner profile/digest. RELEASE_SETUP contains
the request/SUPERVISOR_READY digests and
`SHA256("corvint-hdc-capsule-release-setup" NUL profile NUL request-digest-raw NUL supervisor-ready-payload-digest-raw NUL challenge-raw)`.
CHILD_READY binds the complete blocked-child state digest. RELEASE_EXEC contains every prior payload
digest, that state digest, and the corresponding domain-separated
`corvint-hdc-capsule-release-exec` SHA-256 response. DONE is permitted only after child-tree cleanup,
runner capability erasure, and singleton proof; it reveals S only then. The broker verifies the
challenge, persists and independently recomputes receipt/archive, and sends an ACK canonical body
containing exactly `profile`, `request_sha256`, `supervisor_ready_sha256`, `release_setup_sha256`,
`child_ready_sha256`, `release_exec_sha256`, `receipt_sha256`, `archive_sha256`, and `authenticator`.
`authenticator` is lowercase-hex
`HMAC-SHA256(key=S, data="corvint-hdc-capsule-ack" NUL "corvint-execution-capsule-wire/0" NUL the-seven-preceding-raw-digests-in-field-order)`.
The runner accepts exactly one matching ACK, re-proves singleton/capability-zero state, zeroes S, and
exits. S is never canonical or persisted: a diagnostic transcript replaces its 32 DONE bytes with
the fixed ASCII marker `REDACTED` and separately binds their raw SHA-256. This is a local one-run
capability protocol, not remote attestation.

- `HDCV0-CAP-012`: A successful output root contains exactly `plan.json`, `proposal.patch`, `draft/`,
`build/site.ustar`, `logs/child.stdout`, `logs/child.stderr`, `receipt.json`, `diagnostic.json`, and
`execution.cem.json`. Empty `draft/` and `logs/` directories are retained; any other entry fails.
`receipt.json` is publishable and excludes absolute roots, nonce, container ID, PID, timestamps, and
temporary paths. `diagnostic.json` is local, secret-screened, and contains the execution key, receipt
digest, complete witness projection and digest, run-local fields, and one exact bounded transcript
projection. Each transcript row contains only frame direction, type, order, length, and payload
SHA-256; control payloads are secret-redacted, and the DONE row contains only receipt SHA-256 plus
archive length and SHA-256. The projection binds its canonical digest and never contains archive
bytes or another artifact's raw bytes. `execution.cem.json` binds the raw byte SHA-256 and relative path of `receipt.json`
and `diagnostic.json`; the receipt in turn binds plan, patch, source manifest, environment, authority
snapshot, argv/environment digest, witness-projection digest, prior control-frame digests, output
archive/manifest, logs, exclusions, uncertainty, and all non-cyclic artifact byte digests. Receipt
never binds DONE or diagnostic bytes. Consumer acceptance requires all three verified canonical JSON artifacts,
complete site archive verification, process exit 0, and empty stderr; no one artifact substitutes.

- `HDCV0-CAP-013`: Canonical JSON retains `HDCV0-041`. A raw artifact digest is
`SHA256(exact-artifact-bytes)`. Each content identity is
`SHA256(kind NUL profile NUL canonical-body-with-that-identity-field-omitted)`, with exact kind/profile
pairs: request `corvint-hdc-capsule-request`/`corvint-human-documentation-request/0`; witness
`corvint-hdc-capsule-witness`/`corvint-execution-capsule/0`; receipt
`corvint-hdc-capsule-receipt`/`corvint-human-documentation-receipt/0`; diagnostic
`corvint-hdc-capsule-diagnostic`/`corvint-execution-capsule-diagnostic/0`; and outer manifest
`corvint-hdc-capsule-cem`/`corvint-hdc-capsule-cem/0`. `NUL` is one `0x00` byte; kind and profile are the
shown ASCII bytes; the body includes its trailing LF. Computation is witness projection, receipt,
diagnostic, outer manifest. Cyclic references, a missing bound artifact, or a digest computed over a
parsed/re-encoded substitute is `EVIDENCE_INVALID`.

- `HDCV0-CAP-014`: The implementation core, broker, cleanup-only mode, runner, cgroup observer, and CLI
use Go 1.27 standard library only. Approved executable boundaries are exactly the admitted static
CGO-disabled Corvint binary under systemd 261 supervision, the frozen dedicated Docker
Engine/containerd/runc authority reachable only by the broker's standard-library Engine client, the
pinned capsule runner, the pinned image-contained Python/MkDocs toolchain, and a separately accepted
external denial observer. No Docker CLI, shell, host Python/MkDocs/project code, dynamic Corvint/broker
binary, Go YAML/Markdown substitute, Python dependency added to Corvint, database, hosted helper,
installation, or fallback executor is allowed.
The Docker provider always sets `OFFLINE_QUALIFIED=NOT_OBSERVED`; `network=none`, failed canaries,
forbidden-fetch preflight, and generated-asset scanning are evidence only. A later independent
observer must cover the complete authority-load/build lifecycle before offline PASS is possible.

Independent capsule conformance MUST cover the closed request and artifact preimages; exact broker
Engine requests and forbidden endpoints; launcher/template/authorization admission, caller
authorization, requested/effective-property recomputation, and executable/template/policy swaps at
start and cleanup; exact runner extraction limits/name/mode/digest; immediate-READY zero-byte loss;
second/external/late attach; image, daemon, source, output, and cgroup-identity/reopen drift; forged
or replayed READY/DONE/ACK frames; same-UID/proc/ptrace/signal attacks; client/broker/runner death,
hang, timeout, cancellation, output overflow, cleanup, residue, and every false-PASS attempt. A
zero-production-import vector MUST prove `docs compile` reaches only the capsule provider. Runtime
vectors use only an already-present exact image and the accepted dedicated native-Linux Engine
tuple; otherwise they are `NOT_RUN`. Go 1.27 focused/full test, race, vet, supported portable-core
cross-builds, golden canonical bytes, real host-observer vectors, and fresh independent security and
contract reviews are required before promotion.

### Capsule errors and CLI completion

The capsule adds `PROTOCOL_INVALID`, `INTERRUPTED`, and `OUTPUT_WRITE_FAILED` to the closed primary
codes in `HDCV0-052`. Validation is stage-ordered and stops at the first failure: request grammar;
platform/capability; repository/root/output paths; environment/lock/toolchain pins; static Corvint
binary; systemd/Varlink/effective unit; broker/private socket/Engine exclusivity; daemon;
image/runner; source/config; create/pre-start inspection; SUPERVISOR_READY/RELEASE_SETUP;
CHILD_READY/RELEASE_EXEC; runner/DONE/archive/ACK; post-run drift; cleanup; artifact persistence;
stdout transport. A container ID triggers broker-or-ExecStopPost cleanup on
every later result. `PROCESS_UNCONTAINED` overrides every earlier primary failure when cleanup proof
fails; otherwise `STALE_SOURCE` overrides a reported build result, cancellation is `INTERRUPTED`, a
deadline/byte/count breach is `LIMIT_EXCEEDED`, malformed framing is `PROTOCOL_INVALID`, an invalid
archive is `OUTPUT_INVALID`, and a nonzero qualified MkDocs result is `BUILD_FAILED`. Secondary causes
remain bounded and cannot change truth.

The HDC production surface is exactly `corvint docs compile --request -`; `docs draft`,
`docs consume`, and `docs maintain` belong to `docs/specs/source-documentation-draft-v0.md` and are
not HDC surfaces. It reads one bounded closed
request from stdin. A fatal error established before the first stdout write emits zero stdout, one
bounded canonical error plus LF on stderr, and exits 2. Success writes the complete verified
`receipt.json` bytes, exits 0, and writes no stderr. Any short/erroring stdout write is retried only
while the writer reports progress; a zero-progress or terminal write error returns
`OUTPUT_WRITE_FAILED`, exit 2, and may leave a retained prefix. Cancellation returns `INTERRUPTED`,
exit 2, and never a success receipt. A consumer accepts only complete independently verified receipt
bytes plus exit 0 and empty stderr.

### Material for MkDocs P0 qualification

### Environment discovery and pinning

- `HDCV0-001`: The caller MUST supply one approved environment root and expected exact versions. Corvint
MUST resolve the MkDocs entry point without an ambient search, verify it is a regular executable,
resolve and record every symlink, and reject any resolution outside the approved environment and
explicit interpreter allowlist.

- `HDCV0-002`: Before configuration loading, Corvint MUST execute bounded identity probes in the same
sanitized environment used for the build. The probes MUST bind the literal MkDocs version output,
entry-point SHA-256, interpreter real path/SHA-256/version, platform/architecture, and exact installed
versions of `mkdocs`, `mkdocs-material`, `Markdown`, `pymdown-extensions` when used, and every allowed
plugin or extension distribution. Missing, duplicate, unowned, or mismatched distributions fail
closed before a build.

- `HDCV0-003`: Discovery is not installation. P0 MUST NOT run `pip`, `uv`, `poetry`, a package manager,
an updater, a dependency resolver with network access, or any command that changes the environment.
It MUST set user-site and bytecode writes off and MUST NOT write dependency caches.

- `HDCV0-004`: The environment receipt MUST bind an existing project lock or pin-manifest digest.
Discovered versions that lack a matching expected pin may be reported as `DISCOVERED_UNPINNED`, but
MUST NOT execute a candidate build or qualify the Material profile.

### MkDocs-owned configuration boundary

- `HDCV0-005`: A non-authoritative lexical safety phase MUST run before any Python project code is
loaded. It may reject malformed encoding, duplicate/ambiguous config ownership, traversal, symlinks,
YAML tags/aliases/merge keys, unsupported inheritance/includes, and configured executable surfaces.
It MUST label its result `PREFLIGHT`, not effective MkDocs configuration.

- `HDCV0-006`: After preflight, a fixed probe running from the pinned interpreter MUST use the pinned
MkDocs configuration loader to produce the effective configuration authority snapshot. Corvint MUST
NOT substitute generic YAML decoding, inferred defaults, documentation examples, or config-text
matching for this snapshot.

- `HDCV0-007`: The snapshot MUST bind the root config and every inherited config byte actually used,
their load order, the effective `docs_dir`, config-owned `site_dir`, `nav` owner/mode/value digest,
`site_url`, `use_directory_urls`, validation settings, theme name/options/features/custom directory,
plugin names/order/config digests/distributions, hooks, Markdown extension names/order/config digests/
distributions from effective `markdown_extensions`, `extra_css`, `extra_javascript`, and other
referenced static assets. Secrets and
credentials MUST be rejected before emission; opaque values are represented by type and SHA-256,
not plaintext.

- `HDCV0-008`: Every config, docs, nav page, extension include, extra asset, theme override, and hook
path MUST resolve within its declared root with component-by-component `lstat` checks. P0 rejects
symlinks, hard-link identity collisions crossing roots, non-regular files, device/FIFO/socket paths,
case-fold collisions on case-insensitive hosts, traversal, and files that change during discovery.

- `HDCV0-009`: Effective `docs_dir` MAY differ from `docs`; Corvint MUST use only the MkDocs-resolved
value. The recorded `docs_dir` and `theme.custom_dir` observations are relative to the config file's
directory on both the lexical and the MkDocs-authoritative path, matching MkDocs' own resolution; an
authoritative value outside that directory fails closed (`mkdocs-docs-dir-escape`,
`mkdocs-theme-dir-escape`; `TestAuthoritativeDocsDirIsConfigRelativeLikeLexical`). Effective `nav` MAY
be explicit or omitted. If omitted, Corvint records MkDocs discovery mode
and MAY propose adding an explicit `nav`, but MUST NOT pretend an inferred list was already accepted.
If `INHERIT` or another include owns `nav`, the proposal MUST target that exact owner or abstain.

- `HDCV0-010`: Candidate builds MUST override `site_dir` on the command line to a fresh directory
inside the isolated output root. A config-owned `site_dir` is recorded for drift but never written.
Any plugin, hook, or theme behavior that rejects or escapes the isolated `site_dir` fails closed.

### Exact Material profile

- `HDCV0-011`: The first qualified P0 profile requires effective `theme.name` to equal `material` and
the exact `mkdocs-material` distribution to match the approved pin. Any alias, derived theme, absent
theme, or other theme is `UNSUPPORTED` for P0 even if `mkdocs build` would accept it.

- `HDCV0-012`: Effective `site_url` MUST be recorded exactly. The profile MUST distinguish its use in
canonical/deployment URLs and plugin decisions from fetch-bearing output. Missing, relative,
credential-bearing, query-bearing, or fragment-bearing values prevent full profile qualification;
Corvint does not invent or normalize a deployment URL.

- `HDCV0-013`: The exact ordered `theme.features` list, including every `navigation.*` entry, MUST be
bound to the environment and candidate receipt. Feature semantics are delegated to the pinned
Material version. Unknown or version-incompatible features are configuration failures or explicit
uncertainty, never silently dropped.

- `HDCV0-014`: If the effective `plugins` key is omitted, the authority snapshot MUST show whether
MkDocs supplied built-in `search`. If any plugins are explicitly configured, `search` MUST be
explicitly present for the P0 search-qualified profile. Corvint MUST NOT insert it automatically.
Absent search may still have a strict build result, but search capability is `UNKNOWN` or
`CONFLICTED`, not `SUPPORTED`.

- `HDCV0-015`: P0 execution allows only the exact pinned built-in `search` plugin unless a later
accepted profile names another plugin and its executable trust evidence. Unknown, third-party,
Material privacy, Material offline, and duplicate plugin instances are discovered and reported
before load/execute but are not run under this P0 profile.

- `HDCV0-016`: Hooks are executable project code. Any effective `hooks` entry prevents P0 execution.
The receipt MUST identify its contained path and digest when safe to read, classify build/offline
truth `NOT_RUN`/`NOT_OBSERVED`, and MUST NOT import or invoke the hook to discover what it does.

- `HDCV0-017`: The authority snapshot MUST record Markdown extensions and option digests in exact
order. P0 allows only pinned Python-Markdown built-ins and explicitly pinned `pymdownx.*` extensions.
An unknown extension is executable project code and prevents execution. Corvint does not implement
Markdown or PyMdown semantics itself.

- `HDCV0-018`: Every `extra_css`, `extra_javascript`, image, font, favicon, logo, and other referenced
asset MUST be classified as contained-local, remote-fetch-bearing, generated, missing, or unknown.
P0 never downloads or rewrites an asset. Remote fetch-bearing config prevents
`OFFLINE_QUALIFIED=PASS`; dynamically computed JavaScript behavior remains `UNKNOWN` without an
external runtime observation.

- `HDCV0-019`: A Material `theme.custom_dir` override is input to the build and MUST be fully
contained, pinned, bounded, and represented in the source manifest. P0 MAY report a strict build for
a trusted, approved override corpus, but MUST report offline/runtime semantics `UNKNOWN` unless the
external denial harness covers them. The first Corvint qualifying corpus MUST include a no-override
case; an override cannot silently replace a Material template without appearing in the receipt.

- `HDCV0-020`: Privacy configuration MUST be captured from the MkDocs authority. Effective
`privacy.assets_fetch: true`, or any equivalent pinned-version state that can fetch/cache/rewrite
external CSS, JavaScript, images, or fonts, is forbidden and prevents P0 execution. A state such as
`privacy.assets: false` that disables handling does not localize external assets and cannot qualify
offline behavior.

- `HDCV0-021`: Offline-plugin configuration MUST be captured exactly, including changes to directory
URLs, search-index delivery, and injected assets. Corvint MUST NOT enable or disable the plugin.
Because the plugin is outside the P0 executable allowlist, its presence produces `NOT_RUN` until a
separate accepted, pinned executable profile and hostile corpus exist.

- `HDCV0-022`: Strict validation is the resolved MkDocs 1.6+ configuration plus the mandatory CLI
`--strict` flag. The receipt MUST retain effective `validation.nav` and `validation.links` severities.
Corvint MUST NOT rewrite `warn`, `info`, or `ignore`, MUST NOT add `--no-strict`, and MUST NOT describe
ignored or informational classes as checked warnings.

### Evidence-anchored compiler and patch-plan contract

- `HDCV0-023`: Every output clause MUST have a stable clause ID (1..256 bytes of UTF-8 with no
whitespace, control, or Unicode format character such as U+200B or U+202E), one exact state, a kind of exactly
`PRESCRIPTIVE` or `DESCRIPTIVE`, a scope of exactly `OBSERVED` or `GENERAL` (an undeclared scope is
`GENERAL`), review text, and anchors. An anchor has exactly the members `path`, `blob`, `start_line`,
`end_line`, `span_sha256`, `authority`, and `reason` of the evidence-anchor definition. An anchor that
is missing a member, stale, path-only, line-only, mutable (no Git blob identity), out of range, larger
than the 8 KiB evidence-span limit, or hash-mismatched is disqualified and cannot contribute to
`SUPPORTED` or `CONFLICTED`. `SUPPORTED` requires at least one qualifying anchor and no frontier.
`CONFLICTED` requires at least two qualifying anchors with distinct `(path, blob, start_line,
end_line)` and no frontier. `UNKNOWN` requires exactly one frontier of `NO_QUALIFYING_SOURCE`,
`STALE_ANCHOR`, or `RESOLVER_UNDECIDED` and may retain anchors only as review context. An anchor is
stale only when its `blob` and the pinned blob are distinct IDs in the same Git object format; a `blob`
in the other format is disqualified, not stale (decision 0238).

- `HDCV0-024`: Authority is artifact-class-specific. An anchor `authority` is exactly
`ACCEPTED_INTENT`, `PINNED_SOURCE`, `PINNED_TEST`, or `EXECUTION_RECEIPT`. A `PRESCRIPTIVE` clause
qualifies only on `ACCEPTED_INTENT`; a `DESCRIPTIVE` clause qualifies only on the other three.
Generated documentation, source shape, commit messages, history, learned traces, model output, and
dirty bytes are never an anchor authority; another class named in the inputs section qualifies only
after an amendment adds it to this set. Accepted owner intent outranks implementation for
prescriptive contracts and desired future behavior; pinned source/tests govern descriptive current
behavior. When the two disagree, Corvint MUST emit `CONFLICTED` drift with both anchors; it MUST NOT
rewrite intent to match code, rewrite current behavior to match intent, hide either side, or choose
the more recent file by default.

- `HDCV0-025`: Tests and build output establish only the exact observed behavior at the pinned source
and environment. A passing test, code path, or strict site build MUST NOT become a universal public
contract, negative claim, exhaustive behavior claim, or acceptance decision. Mechanically: a
`GENERAL` clause whose qualifying anchors are only `PINNED_TEST` or `EXECUTION_RECEIPT` cannot be
`SUPPORTED`; negative and exhaustive clauses are `GENERAL`.

- `HDCV0-026`: Model-generated prose is a candidate transformation only. Each behavioral sentence
MUST be admitted clause-by-clause by deterministic anchor checks. Rendered prose consists only of
admitted clause text and non-claim strings drawn from a closed, versioned Corvint template set; any
other sentence, including connective text, is refused rather than labelled.

- `HDCV0-027`: The plan profile is `corvint-human-documentation-plan/0`. It is closed and contains:
source identity; environment pin; redacted configuration snapshot; ordered clauses; ordered file and
nav operations; operation preconditions; candidate patch SHA-256; uncertainty; limits consumed; and
exclusions. Build state, offline state, and artifact SHA-256 values are receipt members only: the
receipt binds the plan, so the plan cannot bind them without a cycle. Unknown fields, duplicate
keys, duplicate clause/operation IDs, and unordered path sets fail closed.

- `HDCV0-028`: An operation is exactly `create_file`, `insert_after`, or `edit_nav`. It names one
repository-relative Markdown target with no `.git` component in any letter case, the target's
expected Git blob and SHA-256 (or explicit absence for creation), exact original byte range,
replacement SHA-256, clause IDs, and a human-readable reason: non-blank UTF-8 of at most 4,096 bytes
with no control (Cc), format (Cf, including zero-width and bidi-control characters), line separator
(Zl), or paragraph separator (Zp) character, else `invalid-operation`. Absence is proven only when no tracked
path is the target, a parent file of it, or under it, and none differs from such a path, or shares
a directory with it, only by letter case: a case-insensitive filesystem aliases those spellings.
Operations MUST NOT overlap: no target is another operation's target or a path under it, under the
same letter-case aliasing. Whole-file replacement of an existing file is forbidden unless the
request explicitly names a generated, non-authoritative file and repository policy confirms that
status.

- `HDCV0-029`: P0 does not replace existing accepted prose. `edit_nav` under `HDCV0-030` is the only
operation that replaces existing bytes, and only within that span. `insert_after` binds a zero-width byte
position plus the exact target-file digest as an optimistic-concurrency precondition. An edited
target makes the operation stale, as does any index change under which an absent target's
`create_file` precondition would no longer hold (a tracked parent file or tracked descendant). Corvint retains the proposed prose for review but MUST NOT retarget
it automatically.

- `HDCV0-030`: `edit_nav` MUST bind both the exact config-source bytes and the authority snapshot that
proved that source owns effective navigation. The candidate patch is applied only to the isolated
snapshot, then the full config is reloaded by MkDocs. Textually plausible YAML is not acceptance.
The replaced span is exactly the one the authority snapshot proves owns effective `nav`; a lexically
located `nav` section is not that proof.

- `HDCV0-031`: The output root contains only `plan.json`, `proposal.patch`, `draft/`, `build/site/`,
bounded `logs/`, and `receipt.json`. A missing artifact is represented explicitly. Output paths are
relative in canonical artifacts; absolute local roots may appear only in a non-portable diagnostic
section that is secret-screened and excluded from publishable output.

- `HDCV0-032`: P0 MUST NOT expose an apply mode. It MUST perform no repository write, Git operation
that changes state, branch/PR operation, deployment including `gh-deploy`, credential read, daemon
start, database access, hosted telemetry, or merge. A future outward action requires a separate
accepted policy and is outside this profile.

### Build, offline, and receipt contracts

- `HDCV0-033`: The candidate snapshot MUST be created from the frozen manifest outside the repository,
without symlinks, then receive only the declared plan operations. Corvint MUST re-hash every consumed
source before configuration load, before build, and after build. Any difference yields
`STALE_SOURCE`, discards build qualification, and leaves the repository untouched.

- `HDCV0-034`: The build argv is an argument vector, never a shell string: the exact pinned MkDocs
entry point, `build`, `--strict`, `--clean`, `--config-file`, the isolated copied config, `--site-dir`,
and the fresh isolated site root. Corvint MUST set an explicit working directory and MUST reject
configuration or child behavior that writes outside allowed output/temp roots.

- `HDCV0-035`: Build environment is constructed from an allowlist, not inherited wholesale. It MUST
use isolated home/temp/XDG/cache roots, disable Python user site and bytecode writes, disable package
indexes, set deterministic locale/timezone where supported, and omit credentials, tokens, SSH
agents, cloud variables, proxy variables, and repository-external plugin paths. Environment
sanitization is not network-denial evidence.

- `HDCV0-036`: One aggregate deadline covers identity probe, config authority load, candidate copy,
and build. On cancellation, timeout, limit breach, or ordinary child return, Corvint MUST close pipes,
terminate the entire verified process group, reap descendants, and prove cleanup with bounded
observation. An OS implementation that can kill only the direct child is `UNSUPPORTED` for P0
execution.

- `HDCV0-037`: Exit zero is necessary but insufficient for `BUILD_STRICT_PASS`. Corvint MUST also prove
the invoked environment pin, non-truncated logs, unchanged inputs, fresh bounded site root, at least
one generated regular file including the expected entry page, no output escape/symlink/special file,
and a complete output manifest. A fake or early-exiting command cannot pass.

- `HDCV0-038`: `OFFLINE_QUALIFIED=PASS` requires an independently identified external network-denial
harness covering the complete config-load/build process, a PASS receipt from that harness, no
forbidden fetch intent, and a bounded generated-site scan with no unapproved fetch-bearing remote
reference. Config inspection, removed proxy variables, a local build, or strict exit zero alone
produces `NOT_OBSERVED` or `UNKNOWN`, never PASS. The Docker execution capsule is not that external
observer and always produces `NOT_OBSERVED` for the offline axis.

- `HDCV0-039`: If executable JavaScript, custom templates, plugins, hooks, or extensions can construct
runtime network requests beyond the static scanner's proof boundary, the receipt MUST state
`UNKNOWN` unless the external observer covers a local-site runtime exercise. Ordinary external
hyperlinks and `site_url` canonical links remain enumerated but do not by themselves fail the
fetch-bearing asset check.

- `HDCV0-040`: The receipt profile is `corvint-human-documentation-receipt/0`. It MUST bind the request,
plan, candidate patch, source manifest, environment, authority snapshot, exact argv, sanitized-env
key names, exit/signal, build state, offline state and observer, output manifest, logs, exclusions,
uncertainty, and all artifact SHA-256 values. It MUST remain local by default and MUST be usable as
CEM-compatible evidence without claiming that CEM proves the prose.

- `HDCV0-041`: Canonical JSON is valid UTF-8: exactly one JSON value, no insignificant whitespace, and
exactly one trailing LF. Object keys are unique and ordered by their UTF-8 bytes. Numbers are
integers in the signed 64-bit range, written in shortest decimal form (no `-0`, fraction, exponent,
or leading zero). Strings escape exactly `"` as `\"`, `\` as `\\`, U+0008, U+0009, U+000A, U+000C,
and U+000D as `\b`, `\t`, `\n`, `\f`, and `\r`, and every other code point below U+0020 as `\u00xx`
with lowercase hex; every other code point, including `/`, U+007F, U+2028, and U+2029, is the literal
UTF-8. Nesting deeper than 10,000 levels is refused. Bytes that differ from this encoding of their own
parsed value are not canonical. Arrays keep
contract-defined order; paths use `/`, are relative where portable, and sort by UTF-8 bytes where a
contract orders them. A canonical plan or receipt is at most 8 MiB.

- `HDCV0-042`: Receipt limits, failures, `NOT_RUN`, `NOT_OBSERVED`, `UNKNOWN`, contradictions, omitted
inputs, and truncated diagnostics remain visible. Corvint MUST NOT fabricate missing measurements,
infer a denial-harness pass, or convert an unavailable Material environment into a synthetic pass.

### Trust and resource boundary

All request, repository, config, docs, assets, environment metadata, build output, subprocess output,
and observer receipts are hostile local bytes. The approved environment is trusted only to the
exact pin and executable allowlist; configured plugins, extensions, hooks, and overrides are project
code, not data. A P0 run grants them no network, repository write, credential, user-home, daemon,
database, or deployment authority.

P0 hard maxima are:

| Resource | Maximum |
|---|---:|
| config files | 16 |
| config inheritance/include depth | 8 |
| bytes per config or Markdown source | 1 MiB |
| bytes per other input asset | 8 MiB |
| input files / total input bytes | 10,000 / 128 MiB |
| clauses / operations / evidence anchors | 2,000 / 2,000 / 10,000 |
| bytes per evidence span | 8 KiB |
| generated files / total generated bytes | 25,000 / 256 MiB |
| stdout and stderr retained | 1 MiB each |
| canonical plan or receipt | 8 MiB each |
| aggregate wall time | 120 seconds |
| post-cancellation cleanup observation | 5 seconds |

Limits are checked while streaming. Crossing a limit terminates the process group, returns a typed
failure, and cannot emit PASS. The caller may lower but not raise these values in P0. Output that
races, changes after hashing, aliases by Unicode/case, or cannot be inspected without following a
link fails closed.

### Hostile failure modes

| Failure | Required result |
|---|---|
| missing/unpinned/fake MkDocs or Material version mismatch | no config load or build; environment mismatch |
| generic YAML view differs from MkDocs | MkDocs snapshot wins or P0 abstains; no nav plan from generic view |
| duplicate keys, tags, anchors, aliases, merge keys, hostile `INHERIT` | preflight rejection before executable project code |
| inherited config/include escapes or changes `docs_dir`/`nav` ownership | contained exact owner or abstention |
| explicit plugins omit `search` | strict result independent; search claim `UNKNOWN`/`CONFLICTED` |
| unknown plugin/extension or any hook writes a marker | rejected before import; marker absent |
| privacy fetch enabled | no build; forbidden network intent |
| offline plugin or theme override changes output | exact state visible; no inferred offline PASS |
| remote CSS/JS/image/font or dynamic JavaScript | remote asset failure or runtime `UNKNOWN` |
| config/docs/asset/override symlink or path escape | fail closed before copy/build |
| FIFO/device/socket or case-fold collision | fail closed without blocking or aliasing |
| source/config changes between plan and build | `STALE_SOURCE`; no qualified build |
| evidence path/line/hash is invented or stale | clause `UNKNOWN`; hallucinated claim never emitted as supported |
| command exits zero without fresh site output | build `FAIL` |
| stale files pre-exist in site root | fresh-root invariant fails; `--clean` cannot bless them |
| warning is hidden, logs exceed cap, or `--strict` is overridden | build `FAIL`; diagnostics say why |
| endless output, timeout, child forks/daemonizes | bounded termination and proven descendant cleanup; no PASS |
| output count/bytes/path escape exceeds bounds | terminate, discard qualification, retain bounded failure receipt |
| denial harness absent, stale, or mismatched | offline `NOT_OBSERVED`/`UNKNOWN`, never PASS |
| proposal target changed or paragraph moved ambiguously | operation stale; never retarget or apply |

### Deterministic acceptance matrix

- `HDCV0-043`: Unit and conformance suites MUST cover canonical plan/receipt bytes, closed schemas,
stable ordering, duplicate rejection, exact source/environment binding, every clause state, every
build/offline state combination, limit edges at `limit-1`, `limit`, and `limit+1`, and repeatability
across at least two clean runs. The same inputs, environment pin, proposal, and observations MUST
produce byte-identical plan/receipt JSON and patch; wall time, temp paths, PIDs, and random IDs are
excluded from canonical bytes.

- `HDCV0-044`: Config hostility MUST cover duplicate keys, tags, aliases, merges, recursive and escaping
inheritance/includes, `docs_dir` and `nav` relocation, config mutation during load, validation
severity changes, unsupported YAML constructs, and a case where a generic YAML interpretation would
differ from the pinned MkDocs authority.

- `HDCV0-045`: Material fixtures MUST cover minimal `theme.name: material`; exact `site_url`; at least
two `navigation.*` feature combinations; implicit default search; explicit plugins with and without
re-added search; standard Markdown and pinned PyMdown extensions; local and remote extra CSS/JS/
assets; missing assets; theme fonts; contained and escaping overrides; hooks; privacy fetch on/off;
offline plugin state; strict validation warnings; and output with remote fetch-bearing references.

- `HDCV0-046`: Process hostility MUST cover pre-cancellation, timeout, endless stdout/stderr, output
bounds, a child and grandchild surviving ordinary parent exit, a descendant attempting a new
session, ignored termination, pipe inheritance, zero exit with absent/stale/escaping site output,
and interruption cleanup. Every started process is reaped; an uncontainable case makes that OS tuple
unsupported.

- `HDCV0-047`: Evidence hostility MUST cover stale Git blobs, dirty sources, out-of-range and oversized
spans, SHA mismatch, path-only citation, unsupported negative claim, accepted-intent/code conflict,
model-fluent hallucination, and a passing strict build falsely offered as behavioral support.

- `HDCV0-048`: Offline/privacy hostility MUST include a local network canary reached by a plugin, a
hook, Material privacy asset fetching, a remote stylesheet/font/image/script, and dynamically
constructed JavaScript. The denial harness MUST demonstrate blocked connection attempts without
real network access; static scans alone MUST fail the dynamic case or return `UNKNOWN`.

- `HDCV0-049`: Verification for implementation promotion MUST pass focused Go tests after each slice,
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./...`,
`GOTOOLCHAIN=local go test -race -count=1 -timeout 30m ./...`,
`GOTOOLCHAIN=local go vet ./...`, Go 1.27 local-toolchain verification, supported cross-builds of the
portable core, the independent conformance runner, and an independent reviewer. Failed and
`NOT_RUN` evidence remains visible.

- `HDCV0-050`: The first real delivery evidence MUST use a Corvint-contained Material corpus and the
project-owned pinned environment without network or repository mutation. Beamfall runs only after
that corpus passes. A fixture, fake MkDocs binary, Corvint-only self-test, or Beamfall-only shadow
cannot promote the governed use case.

- `HDCV0-051`: The implementation core and optional CLI MUST use Go 1.27 and its standard library
only. The only executable documentation authority is the approved project-owned MkDocs environment;
the only additional executable boundary is the explicitly supplied external denial harness. Corvint
MUST NOT add a Go YAML/Markdown implementation, Python package dependency, shell interpreter,
daemon, database, or hosted helper to approximate the profile.

- `HDCV0-052`: Machine failures use exactly one primary code from `INVALID_REQUEST`,
`ENVIRONMENT_UNPINNED`, `ENVIRONMENT_MISMATCH`, `CONFIG_UNSAFE`, `CONFIG_UNSUPPORTED`,
`EXECUTABLE_UNSUPPORTED`, `PATH_UNSAFE`, `LIMIT_EXCEEDED`, `STALE_SOURCE`, `EVIDENCE_INVALID`,
`BUILD_FAILED`, `PROCESS_UNCONTAINED`, `PROTOCOL_INVALID`, `OUTPUT_INVALID`, `OFFLINE_FAILED`,
`INTERRUPTED`, `OUTPUT_WRITE_FAILED`, or `INTERNAL_ERROR`.
Secondary details are bounded strings and cannot change build/offline truth. An unknown failure code
or a failure paired with build PASS is invalid canonical output.

The `Code` of `internal/doccompiler`'s `Error` (`internal/doccompiler/errors.go:5`) is a secondary
detail under this clause, not a primary code, and keeps its kebab-case spelling (decision 0100). The
closed set of those secondary codes is the table below; each row cites the first emitting site and
quotes the message that site returns, which is the whole of what the row asserts. The change that
delivers the canonical output emitter MUST map every secondary code to exactly one primary code
above and add a failure-code vector per mapping; until then no mapping is asserted.

| Secondary code | First emitting site | Sites | Message at the cited site |
|---|---|---:|---|
| `absence-unproven` | `internal/doccompiler/admittedplan.go:303@7b2dad06` | 1 | "index carries no tracked tree to prove <value> absent" |
| `ambiguous-nav-owner` | `internal/doccompiler/plan.go:268@298c094e` | 1 | "config contains duplicate top-level nav keys" |
| `build-encoding-failed` | `internal/doccompiler/build.go:411` | 1 | "cannot encode build result" |
| `build-output-too-large` | `internal/doccompiler/build.go:364` | 1 | "build output exceeds its bounds" |
| `byte-patch-digest-mismatch` | `internal/doccompiler/build.go:430` | 1 | "byte patch does not bind its original and replacement bytes" |
| `canonical-json-duplicate-key` | `internal/doccompiler/canonical.go:73` | 1 | "object key <value> is duplicated" |
| `canonical-json-invalid-utf8` | `internal/doccompiler/canonical.go:21` | 1 | "canonical JSON is not valid UTF-8" |
| `canonical-json-missing-final-lf` | `internal/doccompiler/canonical.go:24` | 1 | "canonical JSON does not end with one LF" |
| `canonical-json-non-integer` | `internal/doccompiler/canonical.go:56` | 1 | "number <value> is not a signed 64-bit integer" |
| `canonical-json-not-canonical` | `internal/doccompiler/canonical.go:37` | 1 | "bytes differ from the canonical encoding of their value" |
| `canonical-json-syntax` | `internal/doccompiler/canonical.go:45` | 3 | "canonical JSON is not one well-formed value" |
| `canonical-json-too-large` | `internal/doccompiler/canonical.go:18` | 2 | "canonical JSON exceeds <value> bytes" |
| `canonical-json-unencodable` | `internal/doccompiler/canonical.go:193` | 1 | "value cannot be encoded as JSON" |
| `clause-limit-exceeded` | `internal/doccompiler/admission.go:99` | 2 | "more than <value> clauses" |
| `command-cancelled` | `internal/doccompiler/process.go:80` | 1 | "command was cancelled before start" |
| `command-failed` | `internal/doccompiler/process.go:97@ab57d596` | 2 | "pinned command exited with status <value>" |
| `command-output-too-large` | `internal/doccompiler/process.go:89` | 1 | "command output exceeds its stdout or stderr byte limit" |
| `command-start-failed` | `internal/doccompiler/process.go:82` | 1 | "cannot start pinned command" |
| `command-timeout` | `internal/doccompiler/process.go:92` | 1 | "command exceeded its deadline" |
| `corpus-too-large` | `internal/doccompiler/build.go:255` | 2 | "staged corpus exceeds its byte limit" |
| `corpus-unavailable` | `internal/doccompiler/build.go:244` | 3 | "cannot read staged document target" |
| `documentation-limit-exceeded` | `internal/doccompiler/draft.go:76@057646ce` | 5 | "draft exceeds 64 KiB" |
| `duplicate-claim` | `internal/doccompiler/plan.go:161@e2c46350` | 1 | "claim ID is duplicated: <value>" |
| `duplicate-clause` | `internal/doccompiler/admission.go:115` | 2 | "clause ID is duplicated: <value>" |
| `duplicate-document` | `internal/doccompiler/plan.go:45@febb118a` | 1 | "document proposal is duplicated: <value>" |
| `duplicate-nav-entry` | `internal/doccompiler/plan.go:198@e7ecb0af` | 1 | "nav path is duplicated: <value>" |
| `duplicate-operation` | `internal/doccompiler/admittedplan.go:182@c5c36f25` | 2 | "operation ID is duplicated: <value>" |
| `environment-inventory-invalid` | `internal/doccompiler/discover.go:154` | 1 | "pinned Python returned an invalid environment inventory" |
| `environment-untrusted` | `internal/doccompiler/build.go:158` | 3 | "a project-owned environment trust attestation is required" |
| `false-strict-pass` | `internal/doccompiler/build.go:391` | 1 | "mkdocs exited zero without producing index.html" |
| `file-too-large` | `internal/doccompiler/paths.go:127` | 1 | "<value> exceeds its byte limit" |
| `file-unavailable` | `internal/doccompiler/paths.go:78@41a960a6` | 4 | "cannot inspect <value>" |
| `immutable-evidence-unavailable` | `internal/doccompiler/plan.go:171@8c6af7e5` | 4 | "experimental planning cannot qualify <value> claim <value> without immutable evidence and authority verification" |
| `invalid-build-evidence` | `internal/doccompiler/receipt.go:96` | 1 | "passing strict build lacks bounded output evidence" |
| `invalid-build-output` | `internal/doccompiler/build.go:354` | 4 | "cannot inspect build output" |
| `invalid-build-status` | `internal/doccompiler/receipt.go:89` | 1 | "BUILD_STRICT status is invalid" |
| `invalid-byte-patch` | `internal/doccompiler/build.go:425` | 1 | "byte patch range is outside its target" |
| `invalid-claim-status` | `internal/doccompiler/plan.go:165@46e95746` | 1 | "claim <value> has an invalid status" |
| `invalid-claim` | `internal/doccompiler/plan.go:158@5f540871` | 1 | "claim ID and text are required" |
| `invalid-clause` | `internal/doccompiler/admission.go:126` | 5 | "clause ID must be 1..<value> bytes of UTF-8 without whitespace, controls, or format characters" |
| `invalid-corpus-entry` | `internal/doccompiler/build.go:294` | 1 | "staged corpus contains a non-file: <value>" |
| `invalid-docs-dir` | `internal/doccompiler/plan.go:30@cccfd84c` | 1 | "docs_dir is not contained" |
| `invalid-document-path` | `internal/doccompiler/plan.go:93@3515780d` | 1 | "document proposal must be Markdown: <value>" |
| `invalid-environment` | `internal/doccompiler/plan.go:23@f31d881d` | 1 | "discovery environment is incomplete" |
| `invalid-file` | `internal/doccompiler/paths.go:124` | 1 | "<value> is not a regular file" |
| `invalid-nav-entry` | `internal/doccompiler/plan.go:190@91ad33b4` | 1 | "nav entries require a title and contained Markdown path" |
| `invalid-offline-evidence` | `internal/doccompiler/receipt.go:100` | 1 | "offline PASS is unavailable without a concrete build-bound external observer" |
| `invalid-offline-status` | `internal/doccompiler/receipt.go:92` | 1 | "offline qualification is invalid" |
| `invalid-operation` | `internal/doccompiler/admittedplan.go:179@3a80561f` | 4 | "operation ID must be 1..<value> bytes of UTF-8 without whitespace or controls" |
| `invalid-output-parent` | `internal/doccompiler/build.go:183` | 3 | "cannot resolve output parent" |
| `invalid-path` | `internal/doccompiler/paths.go:33` | 1 | "empty path" |
| `invalid-project-root` | `internal/doccompiler/paths.go:14` | 4 | "project root is required" |
| `invalid-receipt-digest` | `internal/doccompiler/receipt.go:103` | 1 | "receipt plan or config digest is invalid" |
| `invalid-receipt-envelope` | `internal/doccompiler/receipt.go:83` | 1 | "receipt profile, stage, or claim is invalid" |
| `invalid-receipt-qualification` | `internal/doccompiler/receipt.go:86` | 1 | "config qualification is invalid" |
| `invalid-source` | `internal/doccompiler/admittedplan.go:124@54dd996f` | 1 | "index revision must be 40 or 64 lowercase hex digits" |
| `invalid-target` | `internal/doccompiler/admittedplan.go:246@d59a5569` | 3 | "target must be a clean repository-relative .md path" |
| `invalid-tool` | `internal/doccompiler/paths.go:168` | 1 | "<value> does not resolve to a regular file" |
| `invalid-toolchain-inventory` | `internal/doccompiler/receipt.go:112` | 1 | "distribution count does not match inventory" |
| `invalid-toolchain-pin` | `internal/doccompiler/receipt.go:106` | 2 | "receipt lacks an exact project lock and environment digest" |
| `isolation-failed` | `internal/doccompiler/build.go:69@79813716` | 8 | "cannot create staged corpus root" |
| `markdown-pin-mismatch` | `internal/doccompiler/discover.go:173` | 1 | "expected Python-Markdown <value>, found <value>" |
| `material-pin-mismatch` | `internal/doccompiler/discover.go:170` | 1 | "expected Material <value>, found <value>" |
| `material-version-unknown` | `internal/doccompiler/discover.go:161` | 1 | "mkdocs-material is absent from the pinned environment inventory" |
| `missing-uncertainty` | `internal/doccompiler/plan.go:177@7a252671` | 1 | "claim <value> must explain its uncertainty" |
| `mkdocs-config-ambiguous` | `internal/doccompiler/discover.go:343` | 1 | "both mkdocs.yml and mkdocs.yaml exist; choose one explicitly" |
| `mkdocs-config-authority-invalid` | `internal/doccompiler/discover.go:232` | 4 | "MkDocs returned an invalid bounded config observation" |
| `mkdocs-config-boundary-mismatch` | `internal/doccompiler/discover.go:240` | 1 | "MkDocs loaded a different config file" |
| `mkdocs-config-missing` | `internal/doccompiler/discover.go:340` | 1 | "neither mkdocs.yml nor mkdocs.yaml exists at the project root" |
| `mkdocs-docs-dir-escape` | `internal/doccompiler/discover.go:245` | 1 | "MkDocs resolved docs_dir outside the config directory" |
| `mkdocs-path-required` | `internal/doccompiler/discover.go:82` | 1 | "an exact project-owned mkdocs path is required" |
| `mkdocs-pin-mismatch` | `internal/doccompiler/discover.go:167` | 1 | "expected MkDocs <value>, found <value>" |
| `mkdocs-site-dir-escape` | `internal/doccompiler/discover.go:249` | 1 | "MkDocs resolved site_dir outside the project" |
| `mkdocs-theme-dir-escape` | `internal/doccompiler/discover.go:264` | 1 | "MkDocs resolved theme.custom_dir outside the config directory" |
| `mkdocs-version-mismatch` | `internal/doccompiler/discover.go:164` | 1 | "mkdocs executable and environment inventory disagree" |
| `mkdocs-version-unknown` | `internal/doccompiler/discover.go:143` | 1 | "pinned mkdocs did not report an exact version" |
| `nav-authority-required` | `internal/doccompiler/admittedplan.go:530@5f65f0b6` | 1 | "edit_nav <value> needs the HDCV0-030 configuration authority snapshot" |
| `nav-candidate-mismatch` | `internal/doccompiler/build.go:110` | 1 | "MkDocs authority did not load the exact proposed nav" |
| `offline-asset-scan-failed` | `internal/doccompiler/build.go:443` | 2 | "cannot inspect generated output" |
| `offline-observer-required` | `internal/doccompiler/build.go:420` | 1 | "offline qualification requires an external observer around the complete build process" |
| `output-inside-project` | `internal/doccompiler/build.go:190` | 1 | "output parent must be outside the project" |
| `overlapping-operation` | `internal/doccompiler/admittedplan.go:190@4a3e1802` | 2 | "more than one operation targets <value>" |
| `path-escape` | `internal/doccompiler/build.go:281@3289850e` | 7 | "cannot resolve corpus path" |
| `plan-digest-mismatch` | `internal/doccompiler/build.go:45` | 1 | "patch plan bytes do not match their digest" |
| `plan-encoding-failed` | `internal/doccompiler/plan.go:415` | 1 | "cannot encode patch plan" |
| `plan-environment-mismatch` | `internal/doccompiler/build.go:41` | 1 | "patch plan does not bind the discovered config" |
| `plan-environment-unsupported` | `internal/doccompiler/admittedplan.go:519@8dacdc04` | 1 | "only the plan-only environment variant is admitted" |
| `plan-limit-exceeded` | `internal/doccompiler/admittedplan.go:127@f159ccd3` | 3 | "more than <value> operations" |
| `plan-not-reproducible` | `internal/doccompiler/admittedplan.go:509@a654944b` | 1 | "plan or proposal patch differs from its admitted recompilation" |
| `plan-profile` | `internal/doccompiler/admittedplan.go:516@3019ed73` | 1 | "plan profile must be <value>" |
| `plan-snapshot-unsupported` | `internal/doccompiler/admittedplan.go:522@1b2eca3d` | 1 | "a plan-only configuration snapshot is <value>" |
| `plan-unknown-field` | `internal/doccompiler/admittedplan.go:489@9850c241` | 1 | "plan is not the closed <value> shape: <value>" |
| `process-containment-unsupported` | `internal/doccompiler/build.go:32` | 1 | "<value>" |
| `profile-unqualified` | `internal/doccompiler/build.go:35` | 1 | "Material P0 profile is not qualified: <value>" |
| `project-lock-pin-mismatch` | `internal/doccompiler/discover.go:99` | 1 | "project-owned lock does not match its expected SHA-256" |
| `project-lock-pin-required` | `internal/doccompiler/discover.go:85` | 1 | "an exact project-owned lock path and SHA-256 are required" |
| `proposal-digest-mismatch` | `internal/doccompiler/build.go:252` | 1 | "document byte patch changed after planning: <value>" |
| `pymdown-pin-mismatch` | `internal/doccompiler/discover.go:176` | 1 | "expected PyMdown Extensions <value>, found <value>" |
| `receipt-encoding-failed` | `internal/doccompiler/receipt.go:75` | 1 | "cannot encode canonical receipt" |
| `receipt-input-invalid` | `internal/doccompiler/receipt.go:37` | 1 | "environment and plan must be closed before receipt creation" |
| `stale-documentation-draft` | `internal/doccompiler/draft.go:283@4090f21c` | 1 | "draft differs from original-source and policy rederivation" |
| `stale-source` | `internal/doccompiler/admittedplan.go:498@9a12a50a` | 4 | "plan revision <value> is not the index revision" |
| `stale-toolchain` | `internal/doccompiler/paths.go:190` | 1 | "<value> no longer matches its pin" |
| `symlink-rejected` | `internal/doccompiler/build.go:288` | 3 | "staged corpus contains a symlink: <value>" |
| `target-conflict` | `internal/doccompiler/admittedplan.go:321@64b50e9f` | 2 | "target <value> is under the tracked file <value>" |
| `target-unverified` | `internal/doccompiler/admittedplan.go:295@0222c3cb` | 1 | "target <value> is not a verified regular text blob" |
| `tool-not-executable` | `internal/doccompiler/paths.go:152` | 1 | "<value> is not executable" |
| `tool-too-large` | `internal/doccompiler/paths.go:175` | 1 | "<value> exceeds its byte limit" |
| `tool-unavailable` | `internal/doccompiler/paths.go:149` | 4 | "cannot inspect <value>" |
| `unadmitted-markdown` | `internal/doccompiler/plan.go:86@f73251ba` | 1 | "experimental plans accept only structured claims; Markdown must be empty" |
| `unadmitted-prose` | `internal/doccompiler/admission.go:331` | 1 | "candidate prose is not admitted clause text within <value>" |
| `unknown-clause` | `internal/doccompiler/admittedplan.go:282@e7f16961` | 1 | "operation <value> names an unadmitted clause <value>" |
| `unordered-set` | `internal/doccompiler/admittedplan.go:569@0420bea2` | 1 | "plan carries an unordered or repeated path or ID set" |
| `unpinned-target` | `internal/doccompiler/admittedplan.go:306@1efbd577` | 2 | "target <value> is tracked but its bytes are not pinned" |
| `unsupported-documentation-source` | `internal/doccompiler/draft.go:46@2fcf341f` | 15 | "package must name one repository-relative Go directory" |
| `unsupported-supported-claim` | `internal/doccompiler/plan.go:168@e67f76d8` | 1 | "SUPPORTED claim <value> has no evidence" |
| `version-pins-required` | `internal/doccompiler/discover.go:88` | 1 | "exact MkDocs, Material, Python-Markdown, and PyMdown Extensions versions are required" |

## Acceptance scenarios

| ID | Scenario | Required evidence |
|---|---|---|
| `HDC-A01` | Corvint-contained minimal Material site | exact environment/config pins, deterministic draft, strict PASS |
| `HDC-A02` | Material nav/search/PyMdown/local assets | exact resolved boundary and repeatable output manifest |
| `HDC-A03` | accepted prose disagrees with observed code | `CONFLICTED`, both anchors, non-overwriting proposal |
| `HDC-A04` | missing evidence invites plausible prose | `UNKNOWN`; no fabricated behavioral sentence |
| `HDC-A05` | config/include/symlink/path escape corpus | fail closed before project code or repository write |
| `HDC-A06` | plugin/hook/privacy network canaries | no real network; refusal or denial-harness observation |
| `HDC-A07` | false strict-pass executable/output corpus | build FAIL despite exit zero |
| `HDC-A08` | timeout/output/descendant corpus | bounded failure and proven cleanup |
| `HDC-A09` | stale source during proposal/build | stale result, invalidated build, unchanged repository |
| `HDC-A10` | same clean input twice | byte-identical plan, patch, receipt, and site manifest |
| `HDC-A11` | Beamfall shadow | review-only artifacts; no Beamfall mutation or promotion claim |

## Delivery sequence, rollout, and rollback

1. **Contract and pure core:** freeze this spec; implement closed wires, clause states, anchors,
   operation preconditions, canonicalization, and bounds with no subprocess.
2. **Hostile fake-provider conformance:** qualify containment, stale-source handling, process cleanup,
   false-pass detection, and deterministic receipts without claiming MkDocs delivery.
3. **Corvint Material corpus:** add a contained Material example/corpus and an already-owned exact
   image/lock. Run real MkDocs/Material only under the accepted native-Linux execution capsule.
   Strict-build evidence may pass while offline remains `NOT_OBSERVED`; an independent external
   denial harness remains a separate future gate. This stage is blocked while the verified-current-
   state absences above remain.
4. **Corvint reviewer trial:** independent maintainers assess anchors, contradictions, proposed prose,
   navigation patch, strict build, and offline receipt. The proposal remains local.
5. **Beamfall shadow:** run the same pinned contract against Beamfall without changing its docs,
   config, branches, PRs, or dependencies. Record compatibility gaps; do not tune away held-out
   failures.
6. **Outcome trial:** only after both dogfoods, run the preregistered 100-module/50-change trial and
   update intent/delivery status through owner review.

Rollback removes the optional command/provider from use and stops producing new proposals. It does
not rewrite repository prose, delete failed receipts, relabel failures, or modify the project-owned
MkDocs environment. Pure core wires and historical local receipts may remain readable. Environment,
config, Material, plugin, extension, or observer drift invalidates qualification for new runs but
does not change historical receipt bytes.

## Compatibility and drift rules

- Support is claimed per `(Corvint compiler version, profile version, MkDocs version, Material
  version, Markdown/PyMdown/plugin versions, environment lock digest, OS, architecture, denial
  harness version)`, never for “MkDocs” or “Material” generally.
- Any config/source/asset/override/hook byte change, Git revision change, executable/interpreter/
  distribution change, lock change, or observer change requires a new receipt and build.
- New MkDocs validation defaults, Material theme features, search behavior, privacy/offline behavior,
  or extension semantics are unsupported until fixtures and hostile cases pass under a new exact
  tuple. Documentation drift alone cannot silently revise this accepted contract.
- A stale plan is still reviewable historical evidence but cannot be applied or advertised as
  current. No automatic migration of config, prose, navigation, or environment is in P0.

## Traceability

The portable Go core is partially delivered. `HDCV0-041` is delivered as the pure canonical JSON
verifier `VerifyCanonicalJSON` and its emitter `CanonicalJSON` (`internal/doccompiler/canonical.go`).
`HDCV0-023..026` are delivered as the pure clause admission contract
(`internal/doccompiler/admission.go`, decision 0229): `AdmitClauses` checks each anchor against a
Git-pinned `contextindex.Index` and downgrades an unestablished clause to `UNKNOWN`, keeping every
anchor, so a disqualified anchor stays review context on `UNKNOWN` or beside a qualifying anchor and
`internal/docviews` copies it without re-qualifying it (`CATN-V0-014`, decision 0268);
`RenderAdmittedProse` and `VerifyAdmittedProse` render and byte-verify prose drawn only from admitted
clause text and the `corvint-hdc-prose-templates/0` set. `HDCV0-027..029` are delivered for the
plan-only environment variant (`internal/doccompiler/admittedplan.go`, decision 0231):
`CompileAdmittedPlan` admits the clauses and emits the closed canonical plan and a `git apply` proposal
patch whose SHA-256 the plan binds; `VerifyAdmittedPlan` refuses any plan that does not recompile to
the same plan and patch bytes; `StaleOperations` reports operations whose precondition no longer
holds without retargeting them. The plan's environment pin is `{"kind":"plan-only"}` and its
configuration snapshot is `NOT_RUN`: no execution pin or MkDocs authority snapshot exists, so
`edit_nav` is refused until `HDCV0-030`. The experimental `Plan` does not emit admitted clauses; its
`PatchPlan` stays an experimental wire under its own profile, and `internal/docviews` binds the
admitted plan (decision 0240).
The experimental plan digest and the receipt bytes and digest hash `CanonicalJSON` bytes.
That migration was a clean cut on 2026-09-13: a plan or receipt digest computed before it, over `encoding/json` bytes, does not verify, and no
compatibility reader exists, because no HDC plan or receipt has been released.
`CanonicalJSONUnbounded` is the same emitter without the 8 MiB plan/receipt bound; the
`internal/docviews` truth-digest input and bundle bytes use it under the downstream `CATN-V0-012`
64 MiB bound (`CATN-V0-015`, decision 0227). The proposed-nav digest is not `HDCV0-041` bytes:
it must equal the MkDocs authority probe's `nav_sha256` (decision 0227). Go therefore spells the
proposed nav as Python `json.dumps(sort_keys=True, separators=(",",":"))` with default
`ensure_ascii` writes it: no trailing LF, printable ASCII other than `"` and `\` literal (including
`<`, `>`, `&`, and `/`), short escapes for `"`, `\`, backspace, form feed, LF, CR, and tab, and every
other code point as lower-case `\uXXXX`, with a UTF-16 surrogate pair above U+FFFF.
`TestProposedNavDigestMatchesMkDocsProbeSpelling` (`internal/doccompiler/compiler_test.go`) pins
bytes and a digest that python3 3.9.6 produced from the probe's encoder; a real MkDocs run over that
nav is `NOT_RUN`. A `proposed_nav_sha256` over non-ASCII, `<`, `>`, or `&` computed before
2026-09-13 used Go `json.Marshal` bytes and never matched the probe; no plan has been released.
For that digest to hold, the experimental `edit_nav` replacement must load back to the exact title
and path strings. Each is a JSON string used as a YAML double-quoted scalar, with U+007F..U+009F
(including NEL, which YAML folds as a line break), U+FFFE, and U+FFFF, which `json.Marshal` leaves
literal and YAML refuses, written as lower-case `\uXXXX`; a quoted title longer than the 1024-character
implicit-key limit is written as an explicit `? ` key. `TestRenderNavEscapesCharactersYAMLCannotCarry`
and `TestRenderNavWritesLongTitlesAsExplicitKeys` pin bytes that PyYAML 6.0.3 `yaml.safe_load` loaded
back exactly; a scratch sweep of every non-surrogate code point in titles and paths loaded exactly
under `SafeLoader` and `CSafeLoader` and reproduced `proposed_nav_sha256`. MkDocs loading is `NOT_RUN`.
The experimental planning boundary (`internal/doccompiler`) and its synthetic hostile corpus are not
requirement delivery. Real MkDocs/Material
execution remains `NOT_RUN`: Corvint has no project-owned qualifying capsule image/lock or accepted
dedicated native-Linux Docker tuple. Direct host execution remains prohibited. The current
macOS/Colima workstation is `NOT_RUN`, and capsule execution alone cannot promote offline truth above
`NOT_OBSERVED`. No strict-build or offline qualification claim follows from the focused synthetic
gates.

| Requirements | Planned implementation | Required evidence |
|---|---|---|
| `HDCV0-CAP-001..014` | `internal/doccompiler/capsule`, the production `corvint docs compile` bridge, and the separately installed static broker/runner | exact closed wires/preimages, systemd/Engine policy recomputation, hostile attach/extraction/cgroup/process/cleanup vectors, zero direct-host production reachability, exact native-Linux runtime tuple, and independent security/contract review; all implementation/runtime evidence initially `NOT_RUN` |
| `HDCV0-001..004` | `internal/doccompiler` environment discovery/pin | mandatory lock SHA-256 plus exact MkDocs/Material/Markdown/PyMdown mismatch, fake-entry-point, and no-install fixtures |
| `HDCV0-005..010` | `internal/doccompiler` preflight, authority probe, containment | effective docs/site roots, nav owner/value digest, validation and ordered option digests; unobserved Unix execution is `NOT_RUN` |
| `HDCV0-011..022` | `internal/doccompiler` Material P0 profile | Material config matrix and exact real-environment receipt |
| `HDCV0-023..026` (delivered) | `internal/doccompiler/admission.go` `AdmitClauses`, `RenderAdmittedProse`, `VerifyAdmittedProse` (decisions 0229, 0238) | `TestClauseAdmissionRefusesMalformedClausesHDCV0023` (closed state/kind/scope/frontier, clause ID grammar including a format character such as U+200B, text grammar, duplicate IDs, clause and anchor limits), `TestClauseAdmissionDisqualifiesAnchorsHDCV0023` (missing member, path-only, line-only, stale, a blob in the other object format, mutable, unpinned, index bytes not the blob, out of range, reversed, over 8 KiB with the 8 KiB boundary admitted, hash mismatch; `CONFLICTED` needs two distinct locations), `TestClauseAuthorityIsArtifactClassSpecificHDCV0024` (prescriptive on intent only, descriptive on source/test/receipt only, unlisted authorities disqualified, intent/code drift `CONFLICTED` with both anchors), `TestTestAndReceiptAnchorsCannotSupportGeneralClausesHDCV0025` (undeclared or `GENERAL` scope on test/receipt anchors stays `UNKNOWN`), and `TestRenderedProseIsAdmittedClauseTextAndTemplatesOnlyHDCV0026` (exact template bytes; connective, rewritten, and upgraded prose refused) |
| `HDCV0-027..029` (delivered, plan-only) | `internal/doccompiler/admittedplan.go` `CompileAdmittedPlan`, `VerifyAdmittedPlan`, `StaleOperations` (decision 0231) | `TestAdmittedPlanWireIsClosedAndDeterministicHDCV0027` (byte-identical recompilation; plan-only pin, `NOT_RUN` snapshot, source identity, admitted clauses, limits, patch digest, no build/offline member; unknown field, experimental `PatchPlan`, foreign profile, duplicate key, non-canonical bytes, duplicate clause/operation IDs, unordered sources/operations/clause IDs, execution pin, snapshot status, `edit_nav`, `null` set, forged digest, and tampered patch refused), `TestAdmittedPlanOperationsBindPreconditionsHDCV0028` (absent/present preconditions with Git blob and SHA-256, clause IDs, reason, replacement digest; the patch applies with `git apply` to the bound bytes; malformed index revision, more than 2,000 operations, overlap (the same target, a target under another, or a case alias of another), tracked-unpinned, unproven absence, tracked-path conflicts including case aliases of a tracked page or directory, escape, non-Markdown, quoted, or `.git`-component targets in any case, unknown or missing clauses, control, format (zero-width space, bidi override), line-separator, or over-long reasons, and unverified target bytes refused), and `TestAdmittedPlanInsertsWithoutReplacingAndRetainsStaleOperationsHDCV0029` (zero-width insertions; an edited or newly tracked target, or a tracked path under an absent target, is reported stale and the plan is retained unchanged). Each refusal code is a row of the `HDCV0-052` secondary-code table. Not delivered: the execution environment pin and configuration snapshot (`HDCV0-001..010`, `HDCV0-CAP-001..004`) |
| `HDCV0-030..032` | `internal/doccompiler` plan compiler | not delivered: `HDCV0-030` needs the MkDocs authority snapshot. The experimental `PatchPlan` `edit_nav` byte patch locates `nav` lexically, so it is not delivery. Required: exact nav byte patches, isolated candidate application, and non-mutation fixtures |
| `HDCV0-041` (delivered) | `internal/doccompiler/canonical.go` `VerifyCanonicalJSON`, `CanonicalJSON` | `TestCanonicalJSONEmitsBytesTheVerifierAccepts` (every emitted document verifies; non-integer, unencodable, and over-8 MiB values refused) and `TestCanonicalJSONVerifierEnforcesHDCV0041` (UTF-8, single value, whitespace, final LF, key order and uniqueness, int64 shortest form, exact escape set, 10,000-level nesting, 8 MiB bound) |
| `HDCV0-033..040`, `HDCV0-042` | `internal/doccompiler` snapshot, supervisor, manifests, receipts | stale, false-pass, bounds, generated-asset scan, and unconditional refusal of post-hoc offline promotion; concrete denial/cleanup integration remains `NOT_RUN` |
| `HDCV0-043..049`, `HDCV0-051..052` | `conformance/human-documentation-compiler-v0` and focused Go tests | deterministic runner, stdlib/executable-boundary audit, failure-code vectors, Go test/race/vet/cross-build, independent review |
| `HDCV0-050` | Corvint contained corpus, then Beamfall shadow adapter | separate Corvint and Beamfall dogfood receipts; both initially absent |

### Local unavailable-state marker audit — 2026-09-12

This audit classifies the 13 occurrences above without changing the closed status vocabulary or
turning a test of an unavailable state into delivery evidence. Counts: (a) 4, (b) 1, (c) 8.

Commands observed on 2026-09-12:

- `HDC-C0`: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/doccompiler -run '^TestReceiptBuildStatusVocabularyIsClosed$'` — PASS (exit 0).
- `HDC-C1`: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/doccompiler -run '^(TestLexicalConfigNeverQualifiesAndRejectsExecutableBoundaries|TestReceiptAPIsRefuseEveryUnobservedOfflinePass|TestReceiptBuildStatusVocabularyIsClosed)$'` — PASS (exit 0).
- `HDC-C2`: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/human-documentation-compiler-v0 -run '^(TestHDCCorpusAndHonestObservations|TestHDCBuildReceiptsCannotInventObservation)$'` — PASS (exit 0).

| Marker | Class | 2026-09-12 result or precise local boundary |
|---|---|---|
| H01, build-state vocabulary | (b) | PASS, exit 0 (`HDC-C0`); the focused closed-vocabulary test was added to the existing receipt test file. |
| H02, capsule-unavailable default | (c) | No `internal/doccompiler/capsule`, production `docs compile` capsule bridge, static broker, runner, or accepted capsule image/lock exists on this branch; no process may start. |
| H03, excluded Engine/host classes | (c) | This host is Darwin, while the clause admits only the specified native-Linux/systemd 261/dedicated-rootful-Engine tuple and expressly excludes macOS and VM-backed daemons. |
| H04, capsule runtime vectors | (c) | The accepted exact image and native-Linux Engine tuple are absent, and the required fresh independent security and contract reviews cannot be produced by a local command. |
| H05, hooks preserve unexecuted truth | (a) | PASS, exit 0 (`HDC-C1`, `HDC-C2`); the hook preflight and hostile corpus exercise refusal without invoking the hook. |
| H06, offline plugin stays outside P0 | (a) | PASS, exit 0 (`HDC-C1`, `HDC-C2`); the offline-plugin preflight and corpus exercise the closed allowlist locally. |
| H07, `HDCV0-042` receipt states remain visible | (a) | PASS, exit 0 (`HDC-C1`, `HDC-C2`); receipt APIs and the conformance runner retain unavailable build/offline states and reject invented observation. |
| H08, `HDCV0-049` failed/unexecuted evidence remains visible | (a) | PASS, exit 0 (`HDC-C2`); the corpus command returns the explicit unexecuted execution state and the build-receipt cases reject synthetic promotion. |
| H09, real MkDocs/Material execution | (c) | The project-owned qualifying capsule image/lock and accepted dedicated native-Linux Docker tuple named by the traceability paragraph do not exist. Direct-host execution is prohibited. |
| H10, macOS/Colima workstation | (c) | The current host is macOS, an expressly unsupported host class; Colima or another VM-backed daemon cannot inherit native-Linux evidence. |
| H11, `HDCV0-CAP-001..014` implementation/runtime row | (c) | The planned capsule package, production bridge, installed broker/runner, accepted Linux runtime tuple, and independent reviews are absent. |
| H12, `HDCV0-005..010` real Unix execution row | (c) | Synthetic preflight tests exist, but governed MkDocs execution is reachable only through the absent qualifying Linux capsule; a Darwin host run is forbidden evidence. |
| H13, `HDCV0-033..042` denial/cleanup integration row | (c) | Local process-group tests cannot qualify session-escaping descendants, and no accepted external network-denial observer covers the complete authority-load/build lifecycle. |

## Promotion and kill criteria

P0 implementation may be called `experimental` only after requirements `HDCV0-001..052` have
executable evidence, the Corvint-contained real Material corpus passes, all hostile cases pass, and an
independent verifier reproduces canonical outputs. It remains `UNPROVEN` under
`docs/specs/use-case-conformance-v0.md` until every required evidence class is complete.

The product job may be promoted only on the frozen gate from
`docs/specs/applied-intelligence-breakthroughs-v0.md`: across 100 modules and 50 shipped changes,
independent maintainers require at least 90% clause correctness, 100% evidence resolvability, zero
fabricated behavior, at least 80% acceptance with minor edits, every injected code/spec contradiction
shown, and at least 40% lower documentation latency than the preregistered baseline. Corvint and
Beamfall dogfood, hostile tests, contract, implementation, and sealed-benchmark receipts are all
mandatory; no class substitutes for another.

One fabricated public contract, one silent overwrite, one repository/environment/outward mutation,
one falsely reported strict PASS, one falsely reported offline PASS, one uncontained subprocess,
one real network access from a P0 run, one credential disclosure, or one escaped write immediately
disables execution and outward-action work. Retain the read-only plan compiler only if it continues
to preserve conflicts and unknowns. Kill or narrow the capability if clause correctness is below
90%, evidence resolvability below 100%, acceptance below 80%, latency improvement below 40%, or if
maintainers cannot distinguish proposal, strict-build, and offline truth from the receipt.

## Unresolved owner decisions

- Which exact Python/base-image/image-config/project-lock/transitive-artifact tuple will instantiate
  the selected MkDocs 1.6.1, Material 9.7.7, Markdown 3.10.3, and PyMdown Extensions 11.0.2 profile?
  No project-owned qualifying image/lock exists at this freeze.
- Which external network-denial harness and OS tuples may issue `OFFLINE_QUALIFIED=PASS`, and whether
  runtime rendering as well as build execution is mandatory for that claim?
- Which authority may approve executable plugins, extensions, hooks, or theme overrides in a future
  profile? P0 approves none beyond pinned built-in search and pinned Markdown/PyMdown extensions.
