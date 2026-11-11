# Plugin Envelope V0 (`corvint-plugin/1`)

Owner: Russell Lewis
Date: 2026-11-11
Requirement prefix: `CPE-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs:
- decision 0591 (item 1: apps and plugins reach the core only through published envelopes or
  `corvint-plugin/1`; item 4: plugins are separate executables, one process per run, never `.so`,
  admitted only under the 5% overhead bar);
- `AGENTS.md` invariants 2, 4 and 7;
- `docs/specs/core-bundled-apps-v0.md` (`CMS-V0-010`, the plugin row of "Units and edges to cut");
- `docs/specs/runner-selection-plugins-v0.md` (`RSP-V0-025` to `RSP-V0-032`, `RSP-V0-047`: the
  one out-of-process plugin protocol that exists, `corvint-runner/1`, which `RSP-V0-032` says
  becomes a profile of this envelope with its requirement IDs kept);
- `docs/specs/analyzer-capability-contract-v0.md` (`ACC-V0-002`: executable resolution and launch);
- ticket V1-4819 (this contract); V1-4817 (conformance kit); V1-4818 (registries); V1-4824 (test
  provider measurements).

## Agent digest
- Claim: One stdio envelope, `corvint-plugin/1`, for every out-of-process plugin; any plugin failure yields UNKNOWN or abstention, never a narrower answer.
- Status: proposed / experimental
- Exists: nothing; no code implements the envelope yet. `corvint-runner/1` (`RSP-V0-026` to
  `RSP-V0-032`) is the specified runner protocol this envelope generalizes.
- Blocked on: owner acceptance; the conformance kit (V1-4817); the `CMS-V0-010` overhead
  measurement for each plugin boundary.
- Read next: Requirements; Failure modes and test vectors; Rollback.

## User and measurable job

Two users:

- A core maintainer who adds a plugin family (runners, analyzers, later test providers). They need
  one framing, handshake, limit, discovery and failure contract to implement once, so that each
  family's spec states only its methods.
- An agent or operator reading Corvint output. They need a plugin crash, hang, lie or version skew
  to show up as a named UNKNOWN or abstention for exactly the items that plugin was to answer, never
  as a confident narrower result (invariant 2).

Measurable: every failure mode in the table below has a test vector whose expected code and
consumer observation a conformance run can check mechanically; a plugin boundary's overhead is
measured under `CMS-V0-010` before it enters the default path.

## Verified current state (base `b55635262788`)

- `docs/specs/runner-selection-plugins-v0.md` specifies `corvint-runner/1`: newline-delimited
  strict JSON over stdio (`RSP-V0-026`), `hello` negotiation (`RSP-V0-047`), one process per run
  and its limits (`RSP-V0-029`), digest-pinned discovery outside the repository (`RSP-V0-028`),
  evidence-not-authority (`RSP-V0-030`) and fail-to-FULL (`RSP-V0-031`).
- `internal/analyzerenvelope` holds request-envelope helpers (padded base64 length, bounded
  duplicate-member scan) shared by analyzer candidates; analyzers launch under `ACC-V0-002`.
- `docs/specs/core-bundled-apps-v0.md` names `corvint-plugin/1` (V1-4819) as the plugin boundary
  and `CMS-V0-010` as its admission bar; no spec defined the envelope before this one.
- `git grep corvint-plugin/1` at the base finds only those citations: no implementation, no test.

## Requirements

- `CPE-V0-001`: (proposed 2026-11-11; V1-4819) `corvint-plugin/1` MUST be the one envelope for
  every out-of-process Corvint plugin. A plugin MUST serve exactly one profile per process. The
  envelope owns framing, handshake, lifecycle, limits, discovery, authority and failure mapping
  (`CPE-V0-002` to `CPE-V0-016`). A profile, owned by its family's spec, owns only its method
  names, their `params` and `result` shapes, and its conservative fallback (`CPE-V0-011`). The
  first profiles are `runner` (`corvint-runner/1`, keeping the `RSP-V0` IDs per `RSP-V0-032`) and
  `analyzer` (`ACC-V0`); a `test-provider` profile waits for V1-4824's measurements.
- `CPE-V0-002`: (proposed 2026-11-11; V1-4819) Framing MUST be UTF-8 JSON over the plugin's
  standard input and output, one object per line. A request MUST be
  `{"id": string, "method": string, "params": object}`; a response MUST be
  `{"id": string, "result": object}` or `{"id": string, "error": {"code": string, "message":
  string}}`. Decoding MUST be strict: invalid UTF-8, a line that is not one JSON object, an unknown,
  missing or duplicate member, a wrong type, or a response whose `id` is not the outstanding
  request's is a `protocol-error`. Standard error is diagnostic only (`CPE-V0-012`).
- `CPE-V0-003`: (proposed 2026-11-11; V1-4819) The core MUST send one request at a time and wait
  for its response before the next; batching is done by profile methods that take lists, not by
  pipelining. A line the plugin writes while no request is outstanding is a `protocol-error`.
- `CPE-V0-004`: (proposed 2026-11-11; V1-4819) The first request MUST be `hello` with `params`
  `{"protocols": [majors, descending], "core": version, "profile": "NAME/MAJOR"}`. The result MUST
  be `{"protocol": one offered major, "profile": the same "NAME/MAJOR", "plugin": name, "version":
  string, "features": [names]}`. A `protocol` the core did not offer, a different `profile`, or an
  `error` response to `hello` is a `version-mismatch`. A feature the plugin did not name in `hello`
  MUST NOT be used in that session. The last request MUST be `shutdown` with empty `params` and an
  empty `result`.
- `CPE-V0-005`: (proposed 2026-11-11; V1-4819) One plugin process MUST serve every request of one
  core command run (decision 0591 item 4): never one process per file, and no plugin process
  outlives the core command. The plugin runs in its own process group with the repository root as
  working directory and an environment built from an allowlist the profile names; the core kills
  the group 2 s after `shutdown`, after standard-input end, or on cancellation (`CPE-V0-014`).
- `CPE-V0-006`: (proposed 2026-11-11; V1-4819) Limits MUST have these defaults and ceilings, which
  a project or profile may lower but never raise past the ceiling: message 1 MiB (ceiling 8 MiB);
  session output 256 MiB (1 GiB); retained standard error 64 KiB; `hello` 5 s; any other request
  60 s (600 s). A message over the limit is `oversize`, session output over the limit is
  `output-limit`, a request past its deadline is `timeout`.
- `CPE-V0-007`: (proposed 2026-11-11; V1-4819) The core MUST resolve a plugin executable only from
  the release manifest (a bundled plugin) or the per-user plugin store outside every repository,
  keyed by the SHA-256 digest that a tracked, project-owned declaration pins; never from `PATH`, a
  repository path, a URL, a download, a shell string or an interpreter. It MUST open the file
  without following links, verify the digest and execute that same file (`ACC-V0-002`). Each
  profile names its declaration file (for `runner`, `.corvint/runner-plugins.json` per
  `RSP-V0-028`). An absent entry is `not-installed`, a digest that differs is `digest-mismatch`,
  and a declaration that does not parse strictly is `declaration-invalid`.
- `CPE-V0-008`: (proposed 2026-11-11; V1-4819) Plugin output MUST be evidence, never authority
  (invariant 3, `RSP-V0-030`). A plugin MUST NOT be able to widen trust, mark itself or anything
  qualified, exclude an item from a conservative set, or write `.corvint/`, index or trace state
  (invariant 4). The core MUST validate every result against the profile's schema before use and
  attribute it to the plugin's name, version and executable digest; a result that fails the schema,
  or needs a feature the plugin did not name in `hello`, is `schema-violation`. A result naming an item the
  plugin did not declare in `hello`, or did not receive in the request, is `undeclared-name`.
- `CPE-V0-009`: (proposed 2026-11-11; V1-4819) The core MUST record the repository's tracked and
  untracked status before the session and compare it after `shutdown`; any difference is
  `repository-mutated`, and the session's results are discarded as for every other failure.
- `CPE-V0-010`: (proposed 2026-11-11; V1-4819) Every failure code of `CPE-V0-002` to
  `CPE-V0-009` and `CPE-V0-014`, plus `launch-failed` (the executable could not start),
  `plugin-exited` (it exited or closed standard output before `shutdown`) and `error-response`
  (an `error` response to any request after `hello`), MUST end the session at once, or keep it from
  starting, and discard every result of that session; no partial result is published.
- `CPE-V0-011`: (proposed 2026-11-11; V1-4819) A failed or absent plugin MUST make the consuming
  command report UNKNOWN or abstain for every item that plugin was to answer, naming the code and
  the plugin, never a narrower, negative or confident answer (invariant 2). Each profile MUST state
  its conservative fallback: `runner` makes the plugin's runners FULL or FAIL as `RSP-V0-031`
  says; `analyzer` contributes no candidate and the affected query abstains or reports its
  coverage as UNKNOWN. A profile with no conservative fallback MUST NOT be admitted.
- `CPE-V0-012`: (proposed 2026-11-11; V1-4819) Plugin-supplied text (standard error, `error.code`,
  `error.message`, any string result) MUST be treated as untrusted data: bounded, printed escaped,
  passed through the kernel secret screen before any retention, and never parsed as a core code,
  path or command. Only the core's own codes of `CPE-V0-010` decide control flow.
- `CPE-V0-013`: (proposed 2026-11-11; V1-4819) `corvint-plugin/1` MUST change only additively,
  through features named in `hello`. A breaking change is `corvint-plugin/2`, and the core MUST
  support the current and the previous major. A profile versions separately through its
  `NAME/MAJOR` and keeps its requirement IDs across envelope majors.
- `CPE-V0-014`: (proposed 2026-11-11; V1-4819) When the core command is interrupted (SIGINT or
  SIGTERM) or its context is cancelled, the core MUST kill the plugin's process group and report the
  plugin's items as UNKNOWN with code `cancelled`, writing no partial result.
- `CPE-V0-015`: (proposed 2026-11-11; V1-4819) The transport MUST be the plugin's standard input,
  output and error only: no socket, listener, shared memory, dynamic library, daemon or network
  path is part of the envelope, and the core grants the plugin no credential.
- `CPE-V0-016`: (proposed 2026-11-11; V1-4819) A plugin boundary MUST enter a default code path
  only after its `CMS-V0-010` measurement passes (per-run overhead within 5% of `index` and
  `affected` wall time on the frozen repositories, batched calls served by one process). An
  unmeasured boundary is `NOT_OBSERVED` and the implementation stays linked behind its registry.
- `CPE-V0-017`: (proposed 2026-11-11; V1-4819) The conformance kit (V1-4817) MUST hold a JSON
  Schema for every envelope frame and, for every row of "Failure modes and test vectors", a
  vector: a scripted plugin transcript, the expected core code and the expected consumer
  observation. A kit run MUST validate every frame in both directions and fail when any vector's
  observed code or observation differs.

## Failure modes and test vectors

Each vector is a scripted stand-in plugin. `>` is a line the core writes, `<` a line the plugin
writes; `hello` is the valid exchange of `CPE-V0-004` unless the row changes it. "Observation" is
what the consuming command must print for every item the plugin was to answer. The runner column
follows `RSP-V0-031`; the analyzer column follows `CPE-V0-011`.

| # | Condition | Transcript or setup | Code | Observation |
|---|---|---|---|---|
| 1 | executable absent from manifest and store | declaration pins digest D; no file with D | `not-installed` | runner FULL; analyzer abstains; plan names `not-installed` |
| 2 | digest differs | store file's SHA-256 is not the pinned D | `digest-mismatch` | as vector 1, code `digest-mismatch`; the file is not executed |
| 3 | declaration malformed | duplicate member in the declaration file | `declaration-invalid` | as vector 1; no plugin of that file runs |
| 4 | cannot start | file is not executable | `launch-failed` | as vector 1 |
| 5 | wrong major | `< {"id":"1","result":{"protocol":9,...}}` to an offer of `[1]` | `version-mismatch` | as vector 1 |
| 6 | wrong profile | `hello` result `"profile":"analyzer/1"` to a `runner/1` request | `version-mismatch` | as vector 1 |
| 7 | not JSON | `< not json` | `protocol-error` | as vector 1; results of earlier requests discarded |
| 8 | unknown member | `< {"id":"2","result":{...},"extra":1}` | `protocol-error` | as vector 7 |
| 9 | duplicate member | `< {"id":"2","id":"2","result":{}}` | `protocol-error` | as vector 7 |
| 10 | wrong `id` | request `"id":"3"`, response `"id":"4"` | `protocol-error` | as vector 7 |
| 11 | unsolicited line | a line written before any request, or after a response | `protocol-error` | as vector 7 |
| 12 | invalid UTF-8 | a response containing byte `0xFF` | `protocol-error` | as vector 7 |
| 13 | schema violation | a profile result with a wrong type | `schema-violation` | as vector 7 |
| 14 | undeclared item | a result naming a runner or item not in `hello` or the request | `undeclared-name` | as vector 7 |
| 15 | hang | no response within the request deadline | `timeout` | as vector 7; process group killed |
| 16 | oversize message | one line over the message limit | `oversize` | as vector 7 |
| 17 | output flood | session output past its limit | `output-limit` | as vector 7 |
| 18 | crash | plugin exits 1 after `hello` | `plugin-exited` | as vector 7 |
| 19 | error response | `< {"id":"2","error":{"code":"x","message":"y"}}` | `error-response` | as vector 7; `x` and `y` printed escaped as data |
| 20 | writes the repository | plugin creates an untracked file | `repository-mutated` | as vector 7, even though every response was valid |
| 21 | interrupted | SIGINT to the core during a request | `cancelled` | UNKNOWN `cancelled`; no partial result written |
| 22 | unoffered feature used | a result shape that needs a feature absent from `hello` | `schema-violation` | as vector 7 |
| 23 | hostile text | standard error and `error.message` carry a token the secret screen matches and ANSI escapes | `error-response` | as vector 19; retained text screened and escaped |

A vector passes only when both the code and the observation match. A run where the core reports a
narrower or negative answer for any item of a failed plugin fails the vector, whatever its code.

## Non-goals

- `.so` or in-process dynamic plugins, a marketplace, remote fetch, signing infrastructure or a
  third-party trust model (decision 0591; `core-bundled-apps-v0.md` non-goals).
- Installing plugins: the per-user store and how an operator fills it are owned by the registries
  ticket (V1-4818) and the profile specs.
- Sandboxing beyond the process group, the environment allowlist and the after-session repository
  check: the envelope detects a repository mutation, it does not prevent one, and it cannot observe
  network use. Stronger containment is NOT_PROVIDED in V0.
- Defining any profile's methods; `RSP-V0` keeps the runner methods.
- Simpler baseline kept: compiled-in implementations behind their registries remain the default
  until a boundary passes `CPE-V0-016`.

## Trust boundary

The plugin is an untrusted local process the project pinned by digest. Its bytes are trusted only
as far as the digest pin; its output is evidence the core validates, and its text is data. It holds
no authority the core lacks, writes no core state, and cannot narrow a conservative result.

## Acceptance evidence

| Requirement | Implementation | Evidence |
|---|---|---|
| `CPE-V0-001` to `CPE-V0-017` | none yet | NOT_PRODUCED: no envelope host exists. The vectors above are the acceptance tests the host and the conformance kit (V1-4817) must pass; `corvint-runner/1`'s own tests stay its evidence until it moves onto the envelope. |

## Traceability

| Requirement | Test |
|---|---|
| `CPE-V0-001` to `CPE-V0-017` | NOT_PRODUCED (no implementation); vectors 1 to 23 |

## Rollout, compatibility and rollback

- Rollout: first move `corvint-runner/1` onto the envelope as profile `runner/1` with its `RSP`
  IDs and wire shapes unchanged (its methods, limits and failure set already match), then add the
  `analyzer` profile. Each move is one change with the vectors passing.
- Compatibility: `corvint-runner/1` frames are already valid `corvint-plugin/1` frames except for
  the `profile` member of `hello`; until the move, the runner keeps its own protocol name.
- Rollback: this spec adds no code and no stored state. Withdrawing it means deleting this file and
  its README and INDEX rows; `RSP-V0-032`'s pointer then refers to an unwritten envelope again and
  `corvint-runner/1` keeps working unchanged. After an implementation lands, rolling a profile back
  restores its own protocol name and host; no stored format depends on the envelope.

## Open questions for the owner

1. Accept one outstanding request at a time (`CPE-V0-003`) rather than pipelined requests? It
   keeps failure attribution exact; batching is per method.
2. Unify the analyzer registry of `ACC-V0-002` and the runner store of `RSP-V0-028` into one
   per-user plugin store, or keep one store per profile?
3. Should `test-provider` become a profile before V1-4824's measurements, or stay linked?
4. Promotion: accept this spec, then promote each profile's move only with its vectors passing in
   the conformance kit and its `CMS-V0-010` measurement recorded. Kill criterion: no plugin
   boundary passes `CMS-V0-010`, in which case the envelope stays unimplemented and plugins stay
   linked behind registries.
