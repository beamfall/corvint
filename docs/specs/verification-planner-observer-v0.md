# Verification Planner and Observer V0

**Owner:** Russell Lewis
**Date:** 2026-08-23
**Intent status:** proposed
**Delivery status:** deferred
**Deferral note:** selection/widening clauses belong to `live-proof-carrying-verification-v0.md`;
policy clauses belong to `verification-placement-matrix-v0.md`. No text is ported by this deferral.
**Wire profiles:** `verification-authority-policy/0`, `verification-selector/0`,
`verification-work-packet/0`, `verification-plan/0`, `verification-observation/0`,
`verification-run/0`

## Agent digest
- Claim: Repository policy may produce a deterministic verification plan; execution observation remains a separately qualified later slice.
- Status: proposed/deferred
- Exists: a closed P0 plan-only and later P1 observation contract; no implementation is claimed.
- Blocked on: owner-selected repository policy root and independent P0 qualification before any P1 observer.
- Read next: Decision; Current state and authority boundary; Status.

## 1. Decision

Corvint may gain a generic verification planner and, only after the planner is independently
qualified, a conservative process observer. Repository authority begins at one fixed, tracked policy
root. Corvint validates immutable source and policy bytes, derives one deterministic plan, and cannot
invent, weaken, or replace the repository's canonical checks.

Delivery is split deliberately:

1. **P0 — plan only.** Corvint reads the repository policy, source, caller-reported selector document,
   and optional work-packet evidence and emits an immutable plan. It does not execute a planned
   check, emit an observation, or claim that the selector process ran.
2. **P1 — observe later.** A separately promoted slice may re-run the tracked selector, reconstruct
   byte-identical source, policy, packet, and checks, emit a new plan whose only changed fields are
   selector/execution authority and derived plan ID, execute checks sequentially, and emit per-check
   observations plus one terminal run manifest. P1 inherits every P0 authority rule and has
   additional executable, environment, containment, and staleness gates.

Both slices are one-shot and local. They have no daemon, ambient shell, arbitrary command strings,
runtime authority-adapter manifest, merge decision, or Change Frontier closing authority. Pulse P0-A
remains snapshot-and-lease only. Beamfall's `make gate`, Gate B, finish, receipt validation, latest-
wins policy, re-gate, and guarded merge remain authoritative.

## 2. User and job to be done

The user is a repository maintainer or roadmap worker. They need Corvint to:

1. prove which immutable source and repository policy selected the checks;
2. preserve focused, packet-delta, and mandatory canonical tiers without caller suppression;
3. produce a reviewable plan before any planned command runs;
4. later report process execution, source currency, integration-policy currency, and unknown scope as
   independent facts; and
5. leave work authorization, test meaning, Gate B, receipts, and merge admission with the repository.

## 3. Current state and authority boundary

Corvint does not currently provide this capability. Current impact-receipt verification hints are
derived and display-bounded, Pulse P0-A cannot execute tests, and the harness reports
`frontier.state: UNAVAILABLE`.

Beamfall already owns the adopter policies:

- `script/tests-for-change.sh` selects focused checks;
- roadmap packets bind ticket context, base, acceptance criteria, checks, Gate B, and review plan;
- `make gate` is the canonical project gate; and
- finish and guarded merge validate repository-produced evidence at the exact current branch head.

Corvint is authoritative only for canonical decoding, deterministic derivation, identities it computes,
and—after P1 promotion—the direct-child/process-group events it actually observes. A digest proves
byte identity, not authorization. Caller-provided packet bytes remain caller-reported until the
repository independently validates them. Corvint never converts that evidence into repository
authority.

## Requirements

### 4.1 Canonical framing and identities

- **VPO-V0-001.** Every V0 JSON document MUST be one Change Frontier canonical JSON body followed by
exactly one LF. The body uses the dependency-free codec: no JSON numbers, whitespace, optional
escaping, normalization, duplicate keys, BOM, invalid UTF-8, invalid Unicode scalar, unknown key, or
value outside this specification. Counts and ordinals are canonical decimal strings. Parsing MUST
reserialize to the identical body before any identity is calculated.

- **VPO-V0-002.** Every document MUST contain exactly its named `profile`. Unknown profiles fail
closed. Canonical bodies exclude the terminal LF; a field described as an exact-byte SHA-256 hashes
the complete bytes including that LF unless its rule expressly says otherwise.

- **VPO-V0-003.** Content identities use
`SHA-256(UTF8(identityKind) || 0x00 || UTF8(identityProfile) || 0x00 || canonicalBody)`.
The following table freezes every V0 preimage; no inferred alias is permitted.

| Field | `identityKind` | `identityProfile` | Canonical preimage body |
|---|---|---|---|
| policy `id` | `verification-authority-policy` | `verification-authority-policy/0` | complete policy object without `id` |
| source `id` | `verification-source` | `verification-source/0` | complete nested source object without `id` |
| `trackedMaterializationSha256` | `verification-materialization` | `verification-materialization/0` | canonical ordered array of target entry commitments |
| `commandSha256` | `verification-command` | `verification-command/0` | exactly `{"argv":...,"cwd":...,"env":...}` |
| `checkId` | `verification-check` | `verification-check/0` | complete canonical plan-check object without `checkId` |
| plan `id` | `verification-plan` | `verification-plan/0` | complete plan object without top-level `id` |
| `environmentSha256` | `verification-environment` | `verification-environment/0` | canonical sorted array of every exact child `{name,value}` pair |
| observation `id` | `verification-observation` | `verification-observation/0` | complete observation without top-level `id` |
| run `id` | `verification-run` | `verification-run/0` | complete run without top-level `id` |

Prefixed IDs are exactly `<identityKind>:sha256:<64-lowercase-hex>`. `commandSha256` is the bare
64-hex digest from its row; `environmentSha256` is also bare 64-hex.
`trackedMaterializationSha256` retains the prefixed form. Exact-byte digest fields are frozen below:

| Field | Exact SHA-256 input |
|---|---|
| policy `sha256`, packet `authorityPolicySha256` | complete canonical policy bytes including LF |
| selector `sha256`, executable `fileSha256` | raw regular-file/blob bytes |
| selector `outputSha256` | complete canonical selector bytes including LF |
| `workPacketSha256` | complete canonical sidecar bytes including LF |
| packet criteria, leaf, claim, and review-plan digests | their complete repository-supplied raw bytes |
| source `statusSha256` | complete raw NUL-delimited Git status stdout |
| materialization entry `rawSha256` | exact regular-file bytes or symlink-link bytes |
| executable `pathSha256` | exact native absolute path bytes passed to the OS after lexical normalization |
| output stdout/stderr digests | every corresponding byte Corvint observed before a terminal limit |
| `repositoryReceiptSha256` | complete bounded repository-owned raw receipt bytes |

Exact-byte digests are not content identities. Corvint does not canonicalize or interpret repository
receipt bytes. A native path that cannot round-trip without byte change is unqualified.

- **VPO-V0-004.** Arrays whose order is not expressly semantic MUST be sorted by canonical encoded
bytes and contain no duplicates. `argv`, plan-check order, run-observation order, and reason order
after the fixed tier composition are semantic. Complete-document identities never include the
terminal LF; exact-byte fields do.

### 4.2 Repository-tracked authority root

The only V0 authority root is the exact target-tree path `.corvint/verification-policy.json`:

```json
{
  "canonicalChecks": [
    {
      "argv": ["make", "gate"],
      "cwd": ".",
      "env": [],
      "reason": {"kind": "PROJECT_GATE", "subjects": ["make gate"]},
      "tier": "CANONICAL"
    }
  ],
  "environmentProfile": "verification-env/posix-local/0",
  "id": "verification-authority-policy:sha256:64-lowercase-hex",
  "integrationReference": "refs/heads/main",
  "profile": "verification-authority-policy/0",
  "selectorPath": "script/tests-for-change.sh"
}
```

- **VPO-V0-005.** The policy MUST contain exactly the six keys shown. `integrationReference` MAY be
null; otherwise it is one full `refs/heads/...` reference with no symbolic indirection. Corvint MUST
read the policy from the target Git blob, never from caller-selected bytes or a runtime manifest.
The blob MUST be a regular `100644` target-tree entry at the literal path, and the policy ID and
exact-byte SHA-256 MUST verify before any selector, packet, or check is admitted.

- **VPO-V0-006.** `selectorPath` MUST be a normalized repository-relative UTF-8 path to a regular
`100755` target-tree blob. `canonicalChecks` MUST contain 1 through 64 canonical command objects in
repository order. Beamfall policy MUST contain the exact root check `argv:["make","gate"]`; neither
a selector, work packet, CLI flag, limit, budget, nor Corvint heuristic can remove, rename, reorder,
downgrade, alias, parameterize, or satisfy a policy-root canonical check.

### 4.3 Exact source and changed paths

- **VPO-V0-007.** P0 accepts only one ordinary non-bare worktree whose `HEAD^{commit}` equals the full
`targetRevision`. `targetRevision^{tree}` MUST equal `targetTree`; the index MUST represent exactly
that tree; porcelain-v2 status with all untracked paths MUST be empty. Corvint MUST reject unborn or
detached-unresolvable HEAD, conflicts, sparse checkout, submodule entries, intent-to-add,
skip-worktree, assume-unchanged, split index, alternate index, replace/graft objects, shallow-
ambiguous history, or a non-commit base or target. Ignored paths are not admitted execution inputs;
P1 executes only a separately verified target-tree materialization that contains no ignored or
caller-worktree-only path.

- **VPO-V0-008.** Corvint MUST independently walk every target-tree entry in Git byte order. For each
regular file it compares mode and raw worktree bytes with the exact blob; for a symlink it binds the
link bytes and requires a normalized, repository-contained target resolving to another tracked
target-tree entry. Special files and escaping or dangling symlinks are rejected. The commitment
`trackedMaterializationSha256` is the VPO content identity over the canonical ordered array of
`{blobOid,mode,path,rawSha256}` entries under kind `verification-materialization`, profile
`verification-materialization/0`. Filesystem timestamps and the Git index stat cache are never
evidence of equality.

- **VPO-V0-009.** The changed-path base is the unique commit returned by fixed Git plumbing equivalent
to `git merge-base --all BASE TARGET`; zero or multiple results are `CHANGED_PATHS_UNAVAILABLE`.
Corvint then consumes the NUL-delimited raw output equivalent to
`git diff-tree --no-commit-id -r -z --name-status --no-ext-diff --no-textconv --find-renames=50% --diff-filter=ACDMRTUXB MERGE_BASE TARGET`.
For `R` and `C`, both old and new paths enter the set; every other record contributes its one path.
Paths are decoded only when they are valid UTF-8, 1 through 4096 bytes, slash-separated, normalized,
repository-relative, and contain no empty, `.`, `..`, backslash, NUL, C0/C1 control, or absolute
prefix. A non-UTF-8 or malformed status record fails closed. The unique set sorts by raw UTF-8 bytes,
is capped at 256, and is the only authoritative `changedPaths` value.

- **VPO-V0-010.** All Git calls discard ambient Git variables and configuration that can change
objects, index, diff, status, filters, hooks, or path interpretation; use no optional locks; close
stdin; bound output/time; and pin the repository common directory and object format. On Darwin and
Linux, Git acquisition uses the fixed baseline in VPO-V0-022 with a fresh empty HOME/TMPDIR, no
command override, and replacement objects disabled. On Darwin, when that PATH resolves Git to
Apple's `/usr/bin/git` shim, Corvint executes `usr/bin/git` under the developer directory named by
the root-owned `/var/db/xcode_select_link` directly, because the shim can re-resolve its toolchain
cache under the fresh HOME and warn on stderr; ambient `DEVELOPER_DIR` is never consulted, and if
the link does not name a regular executable Git the shim runs under the same boundary. Any stderr
from the executed Git fails acquisition. A required Git fact that cannot be acquired
under this boundary is `SOURCE_UNQUALIFIED`, not a weaker identity.

### 4.4 Selector and work-packet inputs

The selector document remains:

```json
{
  "changedPaths": ["sorted/repository/path"],
  "checks": [
    {
      "argv": ["executable", "argument"],
      "cwd": ".",
      "env": [{"name": "GOTOOLCHAIN", "value": "local"}],
      "reason": {"kind": "CHANGED_PATH", "subjects": ["sorted/repository/path"]},
      "tier": "FOCUSED"
    }
  ],
  "profile": "verification-selector/0"
}
```

- **VPO-V0-011.** The selector document MUST contain exactly `changedPaths`, `checks`, and `profile`,
at most 256 paths, at most 256 checks, and no command strings, exclusions, scores, optional checks,
canonical authority, or executable code. Every P0 check has exact keys `argv,cwd,env,reason,tier`,
tier `FOCUSED`, and reason kind `CHANGED_PATH|CONSERVATIVE_BOUNDARY`; subjects are a non-empty sorted
subset of Corvint's exact changed paths. The echoed path array MUST byte-equal Corvint's array.

- **VPO-V0-012.** P0 accepts the canonical selector bytes as `CALLER_REPORTED` input only. It binds
their exact-byte digest and the policy-root selector blob identity but does not assert the selector
ran or that the output is repository-authorized. P1 MUST launch the exact policy selector in a fresh
verified target-tree materialization with fixed argv
`[SELECTOR,"--corvint-json","verification-selector/0","--",PATH...]`, require canonical stdout
byte-equal to the P0 selector document, empty stderr, exit zero, and unchanged materialization, then
rebuild the plan with only `inputs.selector.authorityClass`, `executionAuthority`, and the derived
plan `id` changed. Any other byte difference prevents check execution. Corvint never repairs selector
intent.

The work-packet sidecar is:

```json
{
  "authorityPolicySha256": "64-lowercase-hex",
  "baseRevision": "object-id",
  "claimSha256": "64-lowercase-hex",
  "criteriaSha256": "64-lowercase-hex",
  "gateBRequired": true,
  "leafPacketSha256": "64-lowercase-hex",
  "profile": "verification-work-packet/0",
  "reviewPlanDigest": "64-lowercase-hex",
  "ticket": "repository-ticket-id",
  "verifyChecks": []
}
```

- **VPO-V0-013.** The sidecar MUST contain exactly the ten keys shown. Its policy digest MUST equal
the target policy's exact bytes. `criteriaSha256`, `leafPacketSha256`, `claimSha256`,
`reviewPlanDigest`, and `ticket` are either all non-null or all null. All-null is a non-roadmap
envelope and requires `gateBRequired:false`; roadmap form binds the exact externally supplied
leaf-packet, criteria, claim, and review-plan bytes. `verifyChecks` has selector command shape, tier
`DELTA`, reason kind `WORK_PACKET`, and at most 256 entries. Canonical checks exist only in the
tracked policy root.

- **VPO-V0-014.** Corvint verifies every declared digest and base equality, but the packet and its
roadmap interpretation remain `CALLER_REPORTED`; only the repository may authorize the ticket or
validate Gate B. Packet, criteria, claim, base, review-plan, Gate B, verify-check, policy, or external
byte drift invalidates the plan. Corvint offers no override or Gate B acknowledgement and cannot use a
false flag to waive repository policy. P0 may plan caller-reported delta checks, but V0 P1 MUST NOT
execute a plan containing any such check. Repository authorization of packet delta execution needs
a future policy-rooted verifier/profile; local possession of packet bytes is not that authority.

### 4.5 Deterministic plan

```json
{
  "checks": [],
  "executionAuthority": "PLAN_ONLY",
  "id": "verification-plan:sha256:64-lowercase-hex",
  "inputs": {
    "authorityPolicy": {
      "blobOid": "object-id",
      "path": ".corvint/verification-policy.json",
      "sha256": "64-lowercase-hex"
    },
    "changedPaths": [],
    "integration": {"reference": "refs/heads/main", "revision": "object-id"},
    "selector": {
      "authorityClass": "CALLER_REPORTED",
      "blobOid": "object-id",
      "mode": "100755",
      "outputSha256": "64-lowercase-hex",
      "path": "script/tests-for-change.sh",
      "sha256": "64-lowercase-hex"
    },
    "workPacketAuthority": "CALLER_REPORTED",
    "workPacketSha256": "64-lowercase-hex"
  },
  "profile": "verification-plan/0",
  "source": {
    "baseRevision": "object-id",
    "headRevision": "object-id",
    "id": "verification-source:sha256:64-lowercase-hex",
    "objectFormat": "sha1",
    "statusSha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "targetRevision": "object-id",
    "targetTree": "object-id",
    "trackedMaterializationSha256": "verification-materialization:sha256:64-lowercase-hex"
  },
  "unknowns": []
}
```

- **VPO-V0-015.** The plan and nested objects contain exactly the keys shown. `integration` is null
when policy `integrationReference` is null; otherwise Corvint resolves that exact full reference once
to a commit and records both reference and revision. Integration does not enter source ID. Movement
of that reference later makes integration-policy currency stale but does not rewrite a valid
observation of immutable target source.

- **VPO-V0-016.** `statusSha256` hashes the complete output of fixed direct argv
`git status --porcelain=v2 --untracked-files=all -z` under VPO Git policy and is the empty-byte digest
in V0. Source ID binds exactly base, head equal to target, target commit/tree, object format, empty
status, and tracked materialization. Plan ID additionally binds the exact authority policy, changed
paths, optional integration epoch, selector executable/output and authority class, work packet and
authority class, checks, execution authority, and unknowns.

- **VPO-V0-017.** Check order is selector `FOCUSED`, packet `DELTA`, policy `CANONICAL`. Corvint MAY
coalesce only byte-identical `(argv,cwd,env)` tuples, retaining the union of reasons, strongest tier
`CANONICAL > DELTA > FOCUSED`, and position of the strongest-tier occurrence. It never coalesces an
alias or semantic guess. Canonical checks are never subject to caller limit or budget.

- **VPO-V0-018.** A plan check contains exactly `argv,checkId,commandSha256,cwd,env,ordinal,reasons,tier`.
`commandSha256`, `checkId`, source ID, and plan ID use the exact table in VPO-V0-003. Ordinals start
at `0`, have no leading zero, and are consecutive. P0 emits only `executionAuthority: PLAN_ONLY`.
P1 may emit `SELECTOR_OBSERVED` only after VPO-V0-012 replay succeeds; neither value means checks
passed or are authorized for merge. `SELECTOR_OBSERVED` additionally requires no caller-reported
delta check; otherwise the immutable plan remains `PLAN_ONLY`.

- **VPO-V0-019.** Plan unknown codes are exactly `AUTHORITY_CONFLICT`,
`CHANGED_PATHS_UNAVAILABLE`, `SELECTOR_OUTPUT_INVALID`, `SOURCE_UNQUALIFIED`, and
`WORK_PACKET_INVALID`. They are unique and canonical-byte sorted. Any input/identity failure emits a
bounded structured planning error and no plan; `unknowns` remains reserved for a structurally valid
non-executable diagnostic plan. P0 plans are never executable regardless of empty unknowns.

- **VPO-V0-020.** Planning MUST NOT consume impact-receipt verification hints or display-bounded
paths/checks. Conformance includes more than 20 paths and more than 8 checks and proves every
authoritative item remains represented.

### 4.6 P1 command and environment boundary

- **VPO-V0-021.** `argv` contains 1 through 64 UTF-8 strings, each 1 through 4096 bytes and at most
32768 aggregate bytes. It rejects NUL, C0/C1 controls, invalid Unicode, and a platform encoding that
cannot preserve exact strings. `cwd` is `.` or a target-tree directory, normalized and proven inside
the isolated materialization with no symlink, mount, case-folding, or traversal escape.

- **VPO-V0-022.** P1 discards the ambient environment. Profile `verification-env/posix-local/0`
supports Darwin and Linux and sets only: `PATH` to the platform list below; `LANG=C`; `LC_ALL=C`;
`TZ=UTC`; `NO_COLOR=1`; `GIT_CONFIG_NOSYSTEM=1`; `GIT_TERMINAL_PROMPT=0`;
`GIT_OPTIONAL_LOCKS=0`; and fresh Corvint-owned `HOME` and `TMPDIR` directories mode `0700` inside the
run root. The only V0 command env override is `GOTOOLCHAIN=local`. All other names, duplicates,
credential-like names, loader/injection, shell-control, Git-rebinding, and path-rebinding variables
are rejected. The exact complete environment is canonically hashed into `environmentSha256`.

| Platform | Fixed PATH |
|---|---|
| Darwin | `/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin` |
| Linux | `/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin` |

- **VPO-V0-023.** Corvint directly executes argv and never inserts a shell. It rejects shell `-c`,
`cmd /c`, PowerShell `-Command`, `env` indirection, command strings, pipelines, redirection,
substitution, or metacharacter interpretation. Repository scripts and `make` may internally use a
shell because repository code runs at repository-code trust; Corvint does not construct or sandbox
that internal boundary.

- **VPO-V0-024.** A qualified executable launch requires the kernel to execute the same opened object
whose identity and SHA-256 Corvint records: Linux `execveat(AT_EMPTY_PATH)` or an equivalent fd
primitive with the same guarantee. Because the V0 observation binds one executable object, scripts
and interpreter chains cannot be qualified in V0. A script may run only after its absolute
interpreter and script receive no-follow pre/post identity and digest checks, and its observation
always adds `EXECUTABLE_IDENTITY_UNQUALIFIED`; `#!/usr/bin/env` and relative interpreters are
rejected. Bare commands resolve once through the fixed PATH. On Darwin, or any path lacking
exact-object exec, Corvint uses the same unqualified pre/post checks and cannot claim exact executed
bytes or silently promote the result. A swap, interpreter drift, or repeated child PATH lookup is
stale or incomplete.

### 4.7 P1 observations and terminal run

One observation is emitted for every attempted check. Nullable fields make pre-launch and observer
failures representable:

```json
{
  "authorityClass": "CORVINT_PROCESS_OBSERVED",
  "checkId": "verification-check:sha256:64-lowercase-hex",
  "commandSha256": "64-lowercase-hex",
  "containmentClass": "DARWIN_PROCESS_GROUP_UNQUALIFIED",
  "endedAt": "2026-08-23T12:00:01.000000000Z",
  "environmentSha256": "64-lowercase-hex",
  "executable": {"fileId": null, "fileSha256": null, "mode": null, "pathSha256": null},
  "execution": {"exitCode": null, "incompleteReason": "START_FAILED", "signal": null, "state": "INCOMPLETE"},
  "id": "verification-observation:sha256:64-lowercase-hex",
  "mergeAuthority": false,
  "output": {
    "retained": false,
    "stderrBytes": "0",
    "stderrSha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "stdoutBytes": "0",
    "stdoutSha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  },
  "planId": "verification-plan:sha256:64-lowercase-hex",
  "profile": "verification-observation/0",
  "runId": "32-lowercase-hex",
  "scope": "UNKNOWN",
  "source": {
    "endSourceId": "verification-source:sha256:64-lowercase-hex",
    "startSourceId": "verification-source:sha256:64-lowercase-hex"
  },
  "sourceCurrency": "CURRENT",
  "startedAt": "2026-08-23T12:00:00.000000000Z",
  "tier": "CANONICAL",
  "unknowns": ["EXCLUSIONS_UNQUALIFIED", "NETWORK_UNOBSERVED", "PROVIDER_UNQUALIFIED"]
}
```

- **VPO-V0-025.** Observation objects contain exactly the shown keys. One OS-cryptographic 128-bit
`runId` is shared by all observations and the terminal manifest for one invocation. Times are UTC
RFC 3339 with nine fractional digits derived from one monotonic interval. Executable fields are all
non-null only after qualification begins; they remain all null for pre-open failure. `endSourceId`
is null when post-source acquisition fails. Counts are canonical decimals. The observation ID
uses VPO-V0-003. A non-null `fileId` is `linux:dev:<decimal>:ino:<decimal>` or
`darwin:dev:<decimal>:ino:<decimal>` from the opened descriptor; `mode` is exactly four lowercase
octal permission digits. `fileSha256` and `pathSha256` use the exact-byte table in VPO-V0-003.

- **VPO-V0-026.** V0 containment classes are `LINUX_CGROUP_V2`,
`LINUX_PROCESS_GROUP_UNQUALIFIED`, and `DARWIN_PROCESS_GROUP_UNQUALIFIED`. Only a delegated cgroup-v2
subtree with a cleanup supervisor that survives coordinator interruption may claim escape-resistant
cleanup. A Unix process group cannot prove absence of a `setsid` escape or cleanup after Corvint crash;
both process-group classes always add `CONTAINMENT_UNQUALIFIED`. Detected residue makes execution
incomplete and source stale. An unsupported enforcement primitive fails closed; Windows and other
operating systems require a future environment/containment profile. Process containment is never
marketed as a security sandbox.

- **VPO-V0-027.** Execution states are `PASSED`, `FAILED`, and `INCOMPLETE`. `PASSED` is normal exit
zero; `FAILED` is a normal non-zero or non-observer-caused signal; observer termination is always
`INCOMPLETE`. `incompleteReason` is null for pass/fail and otherwise exactly one of `START_FAILED`,
`TIMEOUT`, `CANCELLED`, `OUTPUT_LIMIT`, `MEMORY_LIMIT`, `OBSERVER_FAILED`,
`CLEANUP_UNCERTAIN`, or `SOURCE_CHECK_FAILED`. `exitCode` exists only for normal exit; `signal`
exists only for a reported signal. Every state/cause combination has one conformance vector.

- **VPO-V0-028.** V0 scope is exactly `UNKNOWN`; `BOUNDED` and `PASS_IN_SCOPE` are not V0 states.
Unknowns always include the sorted base set `EXCLUSIONS_UNQUALIFIED`, `NETWORK_UNOBSERVED`, and
`PROVIDER_UNQUALIFIED`. Add `CONTAINMENT_UNQUALIFIED` for an unqualified containment class,
`EXECUTABLE_IDENTITY_UNQUALIFIED` for a non-exact launch, `PROCESS_RESIDUE` when detected, and
`SOURCE_CHANGED` when pre/post source differs or cannot be completed. No other unknown code exists.

- **VPO-V0-029.** `CORVINT_PROCESS_OBSERVED` means only that Corvint observed the declared direct child,
available group/job events, exit, and streamed byte digests. With an unqualified executable or
containment class it does not assert exact executed bytes or escaped-descendant absence. It never
asserts command truth, hermeticity, network denial, provider qualification, all-tests pass, source
safety, Frontier closure, or mergeability. `mergeAuthority` is always false.

- **VPO-V0-030.** Output is digest-only by default and in V0: `retained` is always false. Corvint streams
through bounded SHA-256 and byte counters, persists no stdout/stderr body, and terminates at the
fixed output ceiling; the digest covers every byte observed before termination. Receipts omit source,
output, absolute roots, secret env, and duplicate argv. Raw output retention, expansion, and evidence
handles require a future profile with permissions, screening, deletion, and retention policy.

- **VPO-V0-031.** Network is `HOST_INHERITED_UNOBSERVED`. Corvint claims no denial, sandbox,
hermeticity, environment equivalence, or provider identity. The environment digest proves bytes
Corvint supplied, not that repository code ignored host state.

The terminal manifest is:

```json
{
  "id": "verification-run:sha256:64-lowercase-hex",
  "integrationCurrency": "CURRENT",
  "mergeAuthority": false,
  "observationIds": ["verification-observation:sha256:64-lowercase-hex"],
  "pendingCheckIds": [],
  "planId": "verification-plan:sha256:64-lowercase-hex",
  "profile": "verification-run/0",
  "repositoryReceiptSha256": null,
  "runId": "32-lowercase-hex",
  "sourceCurrency": "CURRENT",
  "state": "COMPLETED",
  "stopReason": null
}
```

### 4.8 Staleness, run completeness, and repository authority

- **VPO-V0-032.** P1 recomputes source immediately before selector launch, before every check, and
after owned-group cleanup. It re-reads exact policy, selector blob, packet and external bytes,
environment profile, executable chain, plan, and integration epoch without timestamp shortcuts. The
selector and every attempted check receive a distinct fresh verified target-tree materialization and
fresh HOME/TMPDIR; each is discarded only after cleanup and the post-source comparison. Output,
cache, ignored, or generated state from one process cannot become an unbound input to the next.
Planned commands never execute in the caller's mutable worktree.

- **VPO-V0-033.** Source drift changes `sourceCurrency` to `STALE`; integration-reference movement
changes only `integrationCurrency` to `STALE`. Packet, policy, selector, command, or environment drift
invalidates the plan and stops the run. Cancellation, timeout, incomplete sequence, observer failure,
or detected residue is incomplete. A same-tree new target commit is source-stale because target
commit is bound; unrelated integration movement never falsifies what executed at the immutable
target.

- **VPO-V0-034.** `verification-run/0` contains exactly the shown keys. Observation IDs are complete,
unique, and in check ordinal order; pending check IDs are the exact unattempted suffix. `state` is
`COMPLETED|STOPPED|INCOMPLETE`; `stopReason` is null only for completed, otherwise
`CHECK_FAILED|CHECK_INCOMPLETE|SOURCE_STALE|POLICY_STALE|CANCELLED|OBSERVER_FAILED`. `STOPPED` uses
`CHECK_FAILED|SOURCE_STALE|POLICY_STALE|CANCELLED`; `INCOMPLETE` uses
`CHECK_INCOMPLETE|OBSERVER_FAILED`. `sourceCurrency` is current only when every observation and the
terminal source comparison are current. `integrationCurrency` is current only when the terminal
integration revision equals the plan epoch. Run ID is invocation-wide. The run `id` binds the
complete manifest via VPO-V0-003, and late events remain keyed to that original run and cannot
replace newer evidence.

- **VPO-V0-035.** P1 executes sequentially and stops at first failed or incomplete check. Canonical
checks remain in the plan and every unattempted one remains pending. V0 has no cache reuse, dirty
overlay, watcher currency, daemon, cross-host lease, remote execution, or continuing-current claim.

- **VPO-V0-036.** Presentation may say `PLAN_ONLY`, `FOCUSED_OBSERVED`, `CANONICAL_PENDING`,
`CANONICAL_OBSERVED`, `STALE`, or `UNKNOWN_SCOPE`, always displaying execution,
source currency, integration currency, and unknown scope separately. `OBSERVED` is not a pass
synonym. `SAFE`, `CORRECT`, `ALL_TESTS_PASS`, `MERGEABLE`, `FRONTIER_EMPTY`, generic green, and
focused/delta pass labels are forbidden.

- **VPO-V0-037.** Beamfall may reference a Corvint run only alongside raw Beamfall-produced canonical
gate-receipt bytes. After process cleanup, Corvint may hash one explicit bounded receipt supplied for
that run and record only its exact `repositoryReceiptSha256`; non-null presence does not affect run
state or grant authority. Corvint never discovers, parses, converts, authors, signs, or upgrades those
bytes. Beamfall independently verifies receipt bytes, command, head, tree, internal step order,
completeness, result, claim, Gate B, latest-wins, and re-gate policy. A Corvint observation or run
without a valid Beamfall receipt is never Beamfall gate evidence.

- **VPO-V0-038.** Corvint cannot write a merge decision, complete a ticket, waive Gate B, invoke guarded
merge, or treat packet digests as authorization. A merge, rebase, target change, packet change,
integration-policy change, or repository receipt rejection prevents reuse. Repository re-gating is
always authoritative.

### 4.9 Pulse, harness, and Change Frontier compatibility

- **VPO-V0-039.** VPO does not change `pulse-snapshot/0`, Pulse lease semantics, PSL-V0-023, Pulse
commands, or Pulse delivery claims. A future Pulse profile may carry an opaque VPO handle only after
separate qualification.

- **VPO-V0-040.** `frontier/0` remains unchanged. No VPO object closes caller-reported continuation,
sets `shouldContinue:true`, or turns unavailable Frontier state into empty. A future process-owned
closing relation requires a new Frontier profile.

- **VPO-V0-041.** Harness adapters may display a plan/run handle but cannot reinterpret it as stop
authority, completion proof, merge authorization, or tool-lifecycle completion. Host adapters must
preserve identical conservative semantics.

### 5. Trust and resource boundaries

- **VPO-V0-042.** P0 trusts Git object hashing, the canonical codec, and the literal tracked authority
policy. It does not trust caller summaries, selector output, work-packet interpretation, impact
hints, repository code, output, PATH, or a Corvint status to authorize work or merge. P1 additionally
trusts only the exact OS observations supported by its recorded executable and containment classes.

- **VPO-V0-043.** Before implementation, conformance freezes complete JSON/nesting/output byte limits,
selector and Git timeouts, per-tier deadlines, memory ceilings, cleanup deadlines, supported OS
tuples, fixed PATHs, and error envelopes. The fixed document limits include at most 256 paths, 256
focused checks, 256 delta checks, 64 canonical checks, 64 argv elements, 4096 bytes per element, and
32768 aggregate argv bytes. Inability to enforce a claimed limit is failure or `INCOMPLETE`, never a
warning.

### 6. Failure behavior

Input, source, policy, and identity failures produce no executable plan. P0 may emit a bounded error
only; it never starts planned checks. P1 failures after a plan exists emit an observation whenever
the attempt can be identified and always attempt one terminal run manifest. An uncatchable crash may
prevent a receipt; absence never means success. Earlier immutable evidence is retained as historical
digest metadata only and cannot advance later checks or mutate repository authority.

### 7. Acceptance evidence

No row is a current implementation claim.

| Slice | Required acceptance evidence |
|---|---|
| P0 policy | Fixed-path tracked policy, wrong-path/caller-policy rejection, canonical-check suppression and alias attacks, one-byte policy drift, ID vectors. |
| P0 source | HEAD/target mismatch, index/tree mismatch, skip/assume/sparse/submodule/alternate/shallow rejection, raw-byte and symlink checks, ignored-path non-admission. |
| P0 paths | Exact merge-base/diff plumbing; rename/copy endpoints; deletion; non-UTF-8, control, oversized, malformed, and more-than-20 path cases. |
| P0 wires | Independent producer/verifier agreement for policy, selector, packet, plan, source, command, check, exact-byte framing, LF, and every identity preimage. |
| P0 plan | More than 8 checks; deterministic ordering/coalescing; policy canonical gates never removed; P0 proves no planned child executes. |
| P1 replay | Selector fixed argv, exact-output replay, mutation, timeout, malformed output, stderr, exit, and source drift. |
| P1 launch | Shell/indirection rejection; PATH and shebang attacks; fd-exec identity; Darwin unqualified ceiling; pre/post swap; env erasure and digest. |
| P1 process | Every pass/fail/incomplete cause; group escape; cgroup/job cleanup; interruption; residue; start and post-source failures remain canonical. |
| P1 run | Complete ordered observations, exact pending suffix, stop reasons, late-event isolation, separate source/integration currency, digest-only output. |
| Beamfall | Beamfall-produced receipt digest round-trip; Beamfall independently rejects wrong step/head/tree/result/claim/Gate B/stale bytes; Corvint-only receipt rejection. |
| Compatibility | Pulse, harness, and `frontier/0` bytes and authority remain unchanged. |

### 8. Minimal slices and promotion gates

#### 8.1 P0 plan-only implementation

- **VPO-V0-044.** The first eligible implementation is an experimental Go plan command backed by a
single `internal/verification` planning package and `conformance/verification-v0`. It reads Git
objects and bounded caller documents, emits `verification-plan/0` with `PLAN_ONLY`, and launches no
selector or planned check. Its command surface, exact limits, errors, and independent verifier MUST
be frozen before implementation. It does not touch Pulse, harness authority, Frontier, runtime
adapter inventory, evidence storage, or any repository workflow.

#### 8.2 P1 observer implementation

- **VPO-V0-045.** P1 cannot begin until P0 has independent byte-for-byte verification and adversarial
source/authority conformance. P1 is a separate experimental command/package slice that replays the
selector, reconstructs an eligible `SELECTOR_OBSERVED` plan with no packet delta check, executes in
a fresh verified materialization, emits observations and one terminal run, and remains shadow-only
on unqualified executable or containment tuples. P1 adoption changes no repository merge or gate
receipt contract.

#### 8.3 Dogfood and larger qualification

- **VPO-V0-046.** P0 dogfood covers 20 consecutive Corvint committed changes before Beamfall planning
shadow. P1 dogfood then covers 20 Corvint changes and at least five real Beamfall authorized tasks,
with zero canonical-gate suppression, source misattachment, unauthorized roadmap conclusion,
repository-worktree mutation, shell insertion by Corvint, or false cleanup claim. Escape-resistant
cleanup promotion applies only to qualified containment tuples. Live proof P1 remains the larger
200-Corvint/200-Beamfall shadow gate; any missed release blocker, false-current receipt, cleanup leak,
or authority mismatch resets the affected sample and returns execution to shadow-only.

- **VPO-V0-047.** V0 cannot be promoted as safe omission, `PASS_IN_SCOPE`, Wallaby equivalence,
generic language qualification, Frontier closure, merge gate, security sandbox, or cross-host
verification. Rollback removes Corvint planning/execution from dogfood and returns callers to direct
repository commands. Repository policy, packets, receipts, Gate B, finish, and guarded merge remain
unchanged; old Corvint digests are historical only.

## 9. Traceability

| Contract area | Requirements |
|---|---|
| Canonical framing and identities | VPO-V0-001..004 |
| Repository authority, exact source, paths | VPO-V0-005..010 |
| Selector and packet evidence | VPO-V0-011..014 |
| Deterministic P0 plan | VPO-V0-015..020 |
| P1 command/environment/executable | VPO-V0-021..024 |
| Observation, containment, output | VPO-V0-025..031 |
| Currency, terminal run, Beamfall authority | VPO-V0-032..038 |
| Pulse/harness/Frontier non-authority | VPO-V0-039..041 |
| Trust and bounds | VPO-V0-042..043 |
| Delivery, dogfood, rollback | VPO-V0-044..047 |

## 10. Status

The contract remains proposed and delivery remains not-started. P0 implementation is unauthorized
until its numeric limits, exact error envelope, fixtures, and independent identity vectors are
frozen. P1 is additionally unauthorized until P0 promotion and the supported executable and
containment tuples pass their own adversarial conformance.
