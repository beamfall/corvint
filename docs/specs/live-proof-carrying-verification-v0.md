# Live Proof-Carrying Verification V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: not-started
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/specs/applied-intelligence-breakthroughs-v0.md`,
`docs/specs/test-claim-qualification-v0.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Live Proof-Carrying Verification proposes bounded provider receipts and exclusion witnesses for change-specific verification.
- Status: accepted (decision 0047, 2026-09-04)/not-started
- Exists: the parent contract and several experimental provider/affected-selection slices; the product is not delivered.
- Blocked on: qualified cross-language providers, exclusion witnesses, execution containment, and promotion evidence.
- Read next: User and measurable job; Competitive and novelty boundary; V0 system boundary.

## User and measurable job

After each eligible edit, Corvint should run the smallest defensible verification scope, stream useful
failures and runtime observations to humans and agents, and state exactly what the result does and
does not establish. A passing subset must never masquerade as a passing repository. Every result is
bound to the exact source, configuration, toolchain, provider, and environment that executed it.

The product hypothesis is a cross-language continuous-verification plane over the Corvint Evidence
OS. V0 is not delivered. It is not permission to advertise language support, safe test omission,
Wallaby parity, or merge authorization before the promotion gates in this document pass.

## Competitive and novelty boundary

Continuous affected-test execution, inline errors and coverage, focused test runs, runtime values,
execution stories, time-travel debugging, editor integration, and agent access to runtime evidence
are established product capabilities. Wallaby documents these capabilities for JavaScript and
TypeScript, including automatic affected-test execution and real-time feedback, selected-test
execution, coverage and runtime values, time-travel debugging, and Skill/MCP access for coding
agents:

- `https://wallabyjs.com/docs/`
- `https://wallabyjs.com/docs/features/selected-tests/`
- `https://wallabyjs.com/docs/features/time-travel-debugger.html`
- `https://wallabyjs.com/docs/features/mcp/`

Corvint MUST treat those as incumbent capabilities, not Corvint inventions. The Corvint hypothesis is the
proof boundary around continuous verification:

1. selection and exclusion carry independently replayable witnesses;
2. uncertainty automatically widens verification rather than disappearing;
3. passing, failing, unknown, and stale scope are distinct machine states;
4. results cannot attach to a different worktree or unsaved-buffer state; and
5. the same evidence can inform an IDE, an agent, review, and policy without upgrading an
   observation into universal behavioral proof.

Cross-language operation is product scope, not itself a novelty claim. Runtime providers remain the
authority for what executed and how they instrument it. Corvint is responsible for identity,
composition, conservative scope, provenance, and honest uncertainty.

## V0 system boundary

```text
editor / filesystem / agent edit
              |
              v
content-addressed workspace source identity
              |
              v
incremental dependency + obligation graph
              |
              v
selection plan ---- unknown frontier ----> widening policy
              |                                  |
              +---------------+------------------+
                              v
                    isolated runtime providers
                 Go | Python | JavaScript/TypeScript
                              |
                              v
 test events + coverage + bounded runtime observations
                              |
                              v
 exact execution identity + selected/excluded/unknown scope
                              |
                +-------------+-------------+
                v                           v
             IDE API                    agent API
```

The coordinator may be implemented in Go to meet cold-start, distribution, and process-control
requirements. That implementation choice does not change the portable evidence protocol or make a
performance claim by itself.

## Evidence states and terms

- **Workspace Source Identity (WSI):** a digest over repository identity, base commit, index state,
  every admitted materialized input path/mode/content digest, accepted unsaved-buffer overlays,
  relevant repository configuration, and the declared execution profile. Excluded ignored,
  untracked, generated, external, or secret-bearing inputs form a bounded explicit frontier. Paths
  are named only when repository policy permits it; otherwise typed counts and digests preserve the
  gap without leaking names or bytes.
- **Workspace Execution Identity (WEI):** a digest over one WSI plus the verification plan, discovered
  test-set identity, provider and runtime/toolchain versions, actual admitted environment fingerprint,
  instrumentation/observation policy, and other qualified execution inputs. Selection consumes the
  WSI; execution produces the WEI, avoiding a circular identity.
- **Verification plan:** ordered selected checks with a witnessed reason for each inclusion, an
  exclusion certificate for each known eligible check not selected, required project gates, and an
  unknown frontier.
- **Runtime evidence:** provider-observed test discovery, dependency, execution, coverage, failure,
  timing, log, or value data. It proves only the bounded observation it records.
- **Result axes:** execution is `PASSED|FAILED|INCOMPLETE`, scope is `BOUNDED|UNKNOWN`, and currency
  is `CURRENT|STALE`. Clients MUST preserve all three rather than collapsing them into one green or
  red status.
- **`PASS_IN_SCOPE`:** presentation label permitted only for `PASSED + BOUNDED + CURRENT`.
- **`FAIL_IN_SCOPE`:** presentation label for a current observed failure; bounded or unknown scope
  remains separately visible because other failures may still be unexamined.
- **`UNKNOWN_SCOPE`:** presentation label when Corvint cannot justify the required selection or
  exclusion boundary, regardless of the checks already observed.
- **`STALE`:** presentation label when the current WSI or execution inputs no longer match the result
  WEI. A stale result may be inspected historically but not displayed as current evidence.
- **Widening:** deterministic escalation from tests to package/module, repository gate, or declared
  full CI because the narrower plan cannot close its unknown frontier.

`PASS_IN_SCOPE` is never renamed `SAFE`, `CORRECT`, `ALL_TESTS_PASS`, or `MERGEABLE`. A separate
repository policy may consume a complete receipt, but Corvint does not invent or bypass that policy.

## Requirements

### Snapshot identity and event ordering

- `LPCV-V0-001`: Every edit event MUST produce or reference one WSI before selection begins.
  Execution MUST produce a complete WEI before results can be cached, displayed as current, or
  consumed by policy; otherwise scope is unknown.
- `LPCV-V0-002`: The WSI MUST cover accepted unsaved editor buffers as content-addressed overlays.
  Overlay bytes MUST execute in an isolated materialization; they MUST NOT be written into the
  user's worktree by the verifier.
- `LPCV-V0-003`: Source or configuration fields needed by the WSI, and environment or toolchain
  fields needed by the WEI, that affect execution but cannot be exactly observed MUST appear in the
  unknown frontier and trigger the registered widening or abstention policy.
- `LPCV-V0-004`: Results MUST be published through a monotonic event sequence bound to both run ID
  and WEI. Late events from an older run MUST NOT replace, merge into, or visually decorate a newer
  workspace result.
- `LPCV-V0-005`: Cache keys MUST include the complete WEI, verification plan, provider, runtime,
  instrumentation mode, and observation policy. A partial match is a cache miss.
- `LPCV-V0-006`: Filesystem races MUST be detected by a pre/post WSI check. A changed input
  makes the run `STALE`; Corvint MUST NOT relabel it as a failure or pass for the newer state.

### Incremental graph and invalidation

- `LPCV-V0-007`: The dependency graph MUST distinguish declared/static edges, provider-observed
  load/import edges, coverage edges, test-claim edges, generated-code/configuration edges, and
  inferred edges. Each edge carries source, revision/WEI, provider, freshness, and confidence class.
- `LPCV-V0-008`: A runtime trace or coverage hit proves an observed execution edge only. It MUST NOT
  prove that an unobserved edge, path, test, platform, or behavior is absent.
- `LPCV-V0-009`: Graph updates MUST be transactional per completed run. Cancelled, timed-out,
  crashed, stale, or partially decoded runs cannot publish a complete edge set.
- `LPCV-V0-010`: Deletion, rename, generator/configuration change, dynamic loading, reflection,
  native extension, build tag, environment-conditioned import, test discovery change, or provider
  version change MUST have an explicit invalidation rule. An unsupported rule widens scope.
- `LPCV-V0-011`: Incremental and clean reconstruction MUST produce the same canonical graph and
  selection plan for the same admissible inputs. Any differential invalidates the incremental cache
  and falls back to clean reconstruction.
- `LPCV-V0-012`: Graph storage MUST remain rebuildable from repository authority and qualified local
  observations. Deleting it loses performance, not source truth.

### Test selection, exclusion, and widening

- `LPCV-V0-013`: Each selected test or gate MUST name at least one machine-checkable witness, such
  as a changed dependency edge, qualified test claim, changed public contract, declared project
  gate, prior relevant failure, or conservative boundary rule.
- `LPCV-V0-014`: Each discovered eligible test not selected MUST have a bounded exclusion entry
  naming the inspected universe, applicable provider, reason, evidence identity, and invalidation
  condition. It is not a claim that the test can never fail.
- `LPCV-V0-015`: Declared mandatory project gates MUST remain mandatory. Corvint MAY schedule them
  later or leave them for CI when policy permits, but it MUST NOT describe a local subset as closing
  those gates.
- `LPCV-V0-016`: Ambiguous discovery, unsupported dynamic behavior, stale evidence, missing graph
  partitions, provider disagreement, configuration drift, critical untracked inputs, or a failed
  exclusion check MUST widen scope or return `UNKNOWN_SCOPE`.
- `LPCV-V0-017`: Widening MUST be monotonic within a WEI: Corvint may add verification scope but MUST
  NOT remove a previously required check because a later heuristic is more optimistic.
- `LPCV-V0-018`: Users MAY request a focused test run. The receipt MUST label it user-scoped and MUST
  NOT emit an automatic affected-scope or repository-wide conclusion.
- `LPCV-V0-019`: Selection MUST be deterministic for fixed admissible inputs. Learned ranking MAY
  change order after a separately qualified profile, but MUST NOT suppress a test required by the
  conservative plan.
- `LPCV-V0-020`: One confirmed release-blocking failure missed by an automatic exclusion MUST disable
  autonomous narrow selection for the affected provider/profile until the cause is fixed, the shadow
  corpus is replayed, and the promotion gate passes again.

### Runtime-provider protocol

- `LPCV-V0-021`: A runtime provider MUST expose versioned discovery, plan, execute, cancel, health,
  and shutdown operations plus a capability manifest. Unknown fields are rejected or preserved
  according to the protocol version; capability absence is never inferred as support.
- `LPCV-V0-022`: Providers MUST report stable test identities, parent suites, source anchors,
  discovery inputs, terminal status, duration, process exit, and structured failures. Provider-local
  identifiers MUST be namespaced and MUST NOT be compared across providers without a registered map.
- `LPCV-V0-023`: Providers MUST stream events with run ID, WEI, provider sequence, and terminal
  completeness marker. Truncation, duplicate terminal events, sequence gaps, malformed data, or an
  unsupported status makes the affected scope unknown.
- `LPCV-V0-024`: The provider protocol and conformance vectors MUST remain independent of a specific
  IDE, agent, test framework, language, Corvint implementation language, or commercial packaging.
- `LPCV-V0-025`: Framework-native filters, coverage, caching, fixtures, sharding, and reporters MAY
  be used only when their exact versions and semantics are captured. Corvint MUST NOT claim stronger
  isolation or selection guarantees than the provider supplies.
- `LPCV-V0-026`: Provider failure MUST degrade to a visible full-command recommendation or
  `UNKNOWN_SCOPE`; it MUST NOT silently reuse a prior pass.

### Coverage, failures, and runtime observations

- `LPCV-V0-027`: Coverage MUST be represented as a WEI-bound observation with provider,
  instrumentation mode, test set, and completeness frontier. Line or branch coverage is not a test
  claim, behavioral proof, or verified absence certificate.
- `LPCV-V0-028`: Failure evidence MUST preserve the raw provider status and normalized failure class.
  Product failure, assertion failure, discovery failure, infrastructure failure, timeout, cancellation,
  crash, and stale execution MUST remain distinguishable.
- `LPCV-V0-029`: Logs and runtime values are opt-in evidence classes with byte, depth, item, and time
  bounds. Default capture MUST redact registered secrets, avoid ambient environment capture, and
  omit raw values when a typed summary or digest satisfies the request.
- `LPCV-V0-030`: Runtime-value inspection MUST name the exact test, process, frame/source anchor,
  execution ordinal, instrumentation mode, and WEI. Corvint MUST NOT claim hidden model reasoning or
  values outside the provider-observed execution.
- `LPCV-V0-031`: Persisted observations MUST have explicit retention and deletion controls. Secret
  screening failure, user cancellation, or repository policy denial prevents persistence and is
  visible in the receipt.

### IDE and agent interfaces

- `LPCV-V0-032`: IDE and agent clients MUST consume one versioned local API and identical canonical
  receipt bytes. Presentation-specific filtering MUST NOT change the underlying evidence state.
- `LPCV-V0-033`: The API MUST support current snapshot, run lifecycle, test result, coverage range,
  failure, selected/excluded/unknown scope, runtime-observation request, cancellation, and receipt
  verification. Unsupported operations return explicit capability errors.
- `LPCV-V0-034`: Inline decorations MUST be removed or marked stale before a newer WEI is shown.
  Color, icon, or wording MUST preserve `PASS_IN_SCOPE`, `FAIL_IN_SCOPE`, `UNKNOWN_SCOPE`, and
  `STALE` distinctions without a generic green state for incomplete scope.
- `LPCV-V0-035`: Agent requests MUST be bounded by repository policy and capability grants. An agent
  may inspect evidence or request a run; it cannot obtain uncaptured secrets, weaken an oracle,
  suppress mandatory gates, mutate source, merge, or publish merely because it can call the API.
- `LPCV-V0-036`: Every expansion from summary to raw failure, coverage, log, or runtime value MUST be
  traceable to the same WEI and evidence handle or visibly fail closed after expiry/deletion.

### Security and process containment

- `LPCV-V0-037`: Test commands MUST be selected from repository-owned configuration or an explicit
  user action and passed as structured arguments. Repository text, test names, file paths, and agent
  output MUST NOT be interpolated into an ambient shell command.
- `LPCV-V0-038`: Each run MUST use an owned process group or equivalent containment primitive,
  bounded CPU/memory/output/time, controlled working directory, minimal environment, and reliable
  cancellation. EXIT, interrupt, timeout, crash, and upgrade paths MUST terminate descendants.
- `LPCV-V0-039`: Network is denied by default for verification workers unless repository policy and
  a specific provider capability explicitly permit it. Network permission and a policy-bounded,
  secret-screened endpoint observation belong in the receipt; Corvint does not claim network absence
  it cannot observe or persist raw credentials, query strings, or payloads.
- `LPCV-V0-040`: Path canonicalization MUST reject workspace escape, symlink race, device/special
  file, socket, and unsafe permission cases before materialization or observation persistence.
- `LPCV-V0-041`: Providers run at repository-code trust level and MUST be described accordingly.
  Corvint MUST NOT market process containment as a security sandbox until an independent adversarial
  evaluation establishes that claim for each operating system.
- `LPCV-V0-042`: Upgrade and adapter discovery MUST be separate from execution. A newly discovered
  binary or manifest cannot activate until signature/provenance policy, compatibility conformance,
  and rollback checks required by the host have passed.

### Language tranche and compatibility

- `LPCV-V0-043`: The first validation tranche is Python on Corvint self-dogfood, Go on Beamfall
  dogfood, and JavaScript/TypeScript on at least one independently maintained repository. Passing one
  adapter MUST NOT produce a generic cross-language support claim.
- `LPCV-V0-044`: Initial framework candidates are Python `unittest` and `pytest`, Go `go test`, and
  JavaScript/TypeScript `node:test`, Vitest, and Jest. Each exact framework/runtime/OS tuple remains
  `NOT_RUN`, `FALLBACK`, or `QUALIFIED`; family names cannot inherit qualification.
- `LPCV-V0-045`: Every qualified tuple MUST pass clean discovery, cold and warm start, edit burst,
  rename/delete, unsaved overlay, focused run, automatic selection, widening, cancellation, timeout,
  crash, malformed output, cache corruption, toolchain/config drift, restart, upgrade, downgrade, and
  uninstall vectors.
- `LPCV-V0-046`: Cross-language repositories MUST compose per-provider plans into one receipt while
  retaining provider-specific unknown frontiers. One provider pass cannot hide another provider's
  failure or unsupported surface.

### Shared test-validity projection

- `LPCV-V0-047`: every client that presents a test-level result to a human or agent (the CLI/MCP
  surface and the VS Code extension alike) MUST project it through one shared native
  test-validity shape carrying exactly five independent axes: association (which source the test
  covers), hygiene (empty/always-skipped/wrong target), freshness (stale execution vs current
  source, this specification's `CURRENT|STALE` currency term), execution
  (pass/fail/skipped/infrastructure/cancelled), and measured strength (mutation witness:
  surviving vs killed). No axis MUST rewrite another, and the shape MUST NOT carry a derived
  boolean or generic pass/fail summary field. A killed mutation establishes only the witnessed
  distinction between the mutant and the original; it MUST NOT be presented as general test
  adequacy. The clause binds only a surface that presents a test-level result: MCP 2026-07-28 V0
  presents none (`MCPV0-008`) and gains no tool for it; a descendant MCP profile owns that projection (`mcp-test-validity-profile-v0.md`, decision 0147).
- `LPCV-V0-048`: passing MUST NOT be presented as implying adequacy on its own: a `PASSED`
  execution axis carries no default strength claim, and the strength axis MUST read
  `NOT_MEASURED` until an actual mutation run supplies a witness.
- `LPCV-V0-049`: an input the projector cannot classify at all (including an operationally
  unsupported claim, receipt, or report) MUST yield every axis stated `UNSUPPORTED` with a
  reason, never a collapsed universal "valid" boolean and never a silently omitted axis.
- `LPCV-V0-050`: the Go projector (`internal/testvalidity`) and the VS Code TypeScript mirror
  (`extensions/vscode/src/testvalidity.ts`) MUST agree on one shared vector file
  (`conformance/test-validity-v0/vectors.json`) covering at minimum: empty test, always-skipped
  test, wrong target, stale execution, unmatched report, passing reported result, unsupported
  input, surviving mutation, killed mutation, skipped reported result, and skipped result with a
  cause (`LPCV-V0-052`). Adding a vector without both projectors
  reproducing its expected projection is a regression.
- `LPCV-V0-051`: `corvint test-validity [--receipt FILE]` is the CLI surface that presents
  test-level results, and is experimental. It MUST read at most 4 MiB of one closed
  provider document of exactly one unambiguous kind: a `corvint-js-test-provider` stdout document,
  or a completed (`passed`, `failed`, or `stale`) `corvint-go-live-session-event/0` document. For
  JavaScript it MUST recompute every per-test and run projection from the `receipt` member through
  `internal/testvalidity`. For Go it MUST recompute the run projection from `state` and `identity`
  and each test projection from the retained `{package,name,action}` observation through the same
  `ProjectGoSession`/`ProjectGoTest` functions the session producer uses. Neither path trusts a
  carried `projection`, `testProjections[].projection`, or `runProjection`. A missing or unknown Go
  test action leaves execution `UNSUPPORTED`; a missing identity leaves freshness `UNKNOWN`, and
  neither case may become a pass by reading a carried projection. It MUST emit one
  `corvint-test-validity/0` document `{schema, source, kind, tests, run}`. Go tests additionally retain
  `package`, the document retains a nonzero `testsOmitted`, uses `source:"corvint-go-test-provider"`
  and `kind:"go-session"`, and every Go result MUST visibly carry `tier:"preview"` and
  `promotable:false`; it is not qualification or policy evidence. The three Go-only members and
  `tests[].package` MUST be omitted for JavaScript so every existing JavaScript output byte remains
  unchanged. Without `--receipt` it MUST exit 0 with `source: "none"`, an empty
  `tests` array, and a run projection whose every axis is `UNSUPPORTED` with reason
  `no-input-supplied`. An unreadable, oversized, unknown-field, trailing-data, receipt-less,
  non-`unit`/`e2e`, non-completed Go, unrecognized-kind, or ambiguous-kind input MUST exit 2 with
  code `invalid-test-validity-receipt` and no stdout. Without `--discover` (`LPCV-V0-053`) it reads
  no repository; in every mode it runs no test and writes nothing. Initial symlinks MAY resolve to a regular receipt; the resolved leaf MUST open
  without following symlinks or blocking on a FIFO, and its descriptor MUST match the pre-open
  device/inode identity. The 4 MiB bound MUST apply to descriptor reads with at most one extra
  overflow-detection byte. A platform lacking safe descriptor opens MUST refuse the receipt
  (currently outside Darwin/Linux).
- `LPCV-V0-052`: an execution report MAY state the outcome `SKIPPED` for a test the runner
  reported as skipped. The projector MUST map `SKIPPED` with an empty cause to the execution state
  `SKIPPED` with reason `test-skipped` (the TCQ report term), and MUST NOT map it to `PASSED` or
  any other passing state: a skip executed no assertion. A `SKIPPED` report that carries a
  non-empty cause is outside the vocabulary and MUST project the execution axis `UNSUPPORTED` with
  reason `unsupported-execution-outcome`. Freshness still follows the report's currency, and the
  association, hygiene, and strength axes are unaffected. Both projectors MUST reproduce the
  `skipped-reported-result` and `skipped-with-cause-abstains` vectors.
- `LPCV-V0-053`: `corvint [--root R] test-validity --discover` (decision 0202) MUST select its
  input automatically from exactly one retained-evidence location, `.corvint/test-evidence` under the
  symlink-resolved worktree root, through the shared `testvaliditydoc.Discover`. `--discover` takes
  no value, may be given once, and is mutually exclusive with `--receipt` (exit 2, no stdout). Each
  path component MUST be a real directory: a symlinked or non-directory component MUST exit 2 with
  code `invalid-test-evidence-location`, and more than 256 entries MUST exit 2 with code
  `test-evidence-limit-exceeded`; a missing component is no evidence, not an error. Discovery MUST
  list only that one directory, never recurse or follow a symlink, consider only regular `*.json`
  entries newest first by modification time (then name, descending), and read at most 16 of them
  through the same safe `LPCV-V0-051` reader and decoder. The first that decodes as one closed
  provider document (a completed Go event included) is projected; a non-regular, unreadable or
  undecodable entry, a non-completed Go event, and an entry beyond the 16 attempts are counted in
  `discovery.skipped`, never projected. The document gains one member, `discovery:
  {location, evidence?, freshness?, skipped}`, where `evidence` is the worktree-relative selected
  path; this member is emitted only by discovery, so every `--receipt` and no-input byte is
  unchanged. With no usable entry the document is the `LPCV-V0-049` abstention plus `discovery`.
  Discovery writes nothing, runs no test, and MUST produce the same document as the
  `corvint-test-validity-mcp` `discover` argument (`MTV-V0-009`).
- `LPCV-V0-054`: a discovered document's freshness MUST be bound to the worktree as it is now,
  never taken from the retained file's age or its carried projections. For a JavaScript receipt
  every bound `testFileDigests` and `configFile` path MUST be an absolute path inside the worktree,
  outside `.git`, read through the confined reader, and its sha256 compared with the bound digest.
  A changed or removed bound file MUST project freshness `STALE` with reason
  `retained-digest-mismatch`. A path outside the worktree, an unreadable bound file, no bound file
  digest at all, a package digest that names a package or lock file, or an app-build digest (none
  of which can be recomputed from the retained document) MUST project `UNKNOWN` with reasons
  `retained-bound-path-outside-worktree`, `retained-bound-source-unreadable`,
  `retained-identity-unbound`, `retained-package-identity-unverifiable`, or
  `retained-app-build-identity-unverifiable`. A completed Go event does not retain its watched file
  set, so it MUST project `UNKNOWN` (`retained-session-identity-unverifiable`), or `STALE`
  (`workspace-execution-identity-mismatch`) when its own state is `stale`, and MUST keep `tier:"preview"` and `promotable:false` (`GLTP-V0-048`). Only
  fully matched bound digests may state `CURRENT`. The binding, anchored `retained-evidence:PATH`,
  replaces the freshness axis of the run and of every test that is not already `STALE`, and is
  repeated as `discovery.freshness`; the association, hygiene, execution and strength axes are
  unchanged, so a passed execution with unmatched identity is never a current pass.
- `LPCV-V0-055`: producer retention (decision 0218) MUST be opt-in and bounded.
  `corvint-js-test-provider unit|e2e --retain` and `corvint-go-test-provider session --retain` MUST
  behave exactly like the retained legacy executable entry points and
  write every stdout byte exactly as without the flag. They MUST also retain the same bytes into
  `.corvint/test-evidence` under the worktree root: the nearest ancestor of `--dir` (itself included)
  holding a `.git` entry for JavaScript, and the authority bundle's repository root for Go. JavaScript
  retains its one document. Go retains each `passed`, `failed` or `stale` event line and no other
  event. The retained file MUST be named `<provider>-<19-digit unix nanoseconds>-<16 lowercase
  hex>.json`, so `LPCV-V0-053` selects it without any change to discovery. It MUST be written
  atomically: an exclusive dot-named temporary file in the same directory, write, fsync, close, and
  rename. The file MUST be created `0600`, and a missing `.corvint` or `test-evidence` directory
  `0700`. A symlinked or non-directory component MUST be refused, and a component must be the same
  file after it is opened as the no-follow `Lstat` that admitted it. After each write the provider
  MUST prune that directory to its own newest 32 regular files by name. Once 32 such files exist,
  the same prune MUST also remove its own dot-named temporaries whose document name sorts below the
  oldest kept name, so temporaries a crash left before rename cannot fill the discovery bound; a
  writer still holding one would have its document pruned on arrival. It MUST NOT remove any
  entry that matches neither its own name pattern nor its own temporary pattern. A write whose own prune removed it (a clock
  behind earlier names) retained nothing and MUST be a retention failure. A retention failure MUST be reported on stderr
  as `retention failure: ...` and MUST make the exit nonzero (JavaScript at once, a Go session when
  it ends), but the stdout document or event MUST already have been emitted. Without `--retain`
  neither provider writes into `.corvint/test-evidence`. The one-shot Go authority run does not accept
  the flag, because its transcript is not a document discovery decodes. Retention is local derived state, never
  authority, and runs no additional test.

#### Projector axis reasons

The Go projector `internal/testvalidity` sets axis reasons from the kebab-case codes below
(decision 0100). Each row cites the first emitting site and states only the condition checked there.

| Code | First emitting site | At the cited site |
|---|---|---|
| `no-association-input` | `internal/testvalidity/projection.go:202` | no claim facts; the association and hygiene axes are `UNSUPPORTED` |
| `no-execution-input` | `internal/testvalidity/projection.go:234` | neither an execution outcome nor a claim report state; the execution axis is `UNSUPPORTED` |
| `no-execution-report` | `internal/testvalidity/projection.go:218` | no execution facts or an empty currency; the freshness axis is `UNKNOWN` |
| `no-input-supplied` | `internal/testvalidity/projection.go:182` | the input carries no claim, execution, or mutation facts; every axis is `UNSUPPORTED` |
| `no-mutation-run` | `internal/testvalidity/projection.go:280` | no mutation facts; the strength axis is `NOT_MEASURED` |
| `test-skipped` | `internal/testvalidity/projection.go:249` | the execution outcome is `SKIPPED` with an empty cause; the execution axis is `SKIPPED` (`LPCV-V0-052`) |
| `unsupported-execution-outcome` | `internal/testvalidity/projection.go:247` | the execution outcome is `SKIPPED` with a non-empty cause, or (`:256`) none of `PASSED`, `FAILED`, `SKIPPED`, or `INCOMPLETE`; the execution axis is `UNSUPPORTED` |
| `unsupported-report-state` | `internal/testvalidity/projection.go:274` | the claim report state is none of the known `tcq` report states; the execution axis is `UNSUPPORTED` |
| `workspace-execution-identity-mismatch` | `internal/testvalidity/projection.go:222` | the execution currency is `STALE`; the freshness axis is `STALE` (`LPCV-V0-054`; pinned by the `stale-execution` vector in `TestProjectMatchesSharedVectors`) |

## Promotion experiments

All promotion corpora, changes, defects, splits, metrics, exclusions, and stopping rules MUST be
registered before observing results. Corvint self-dogfood is friction evidence, not independent market
or correctness evidence.

### P0 — protocol and identity

- Frozen provider protocol, canonical JSON, compatibility rules, and adversarial conformance vectors.
- Reference and independent verifiers agree on canonical plan and receipt bytes; clean, warm-cache,
  and incremental construction produce identical bytes from the same fixture inputs.
- One million randomized out-of-order/stale event schedules yield zero newer-WEI contamination.
- Forced interruption leaves zero child process, overlay materialization, socket, or current-result
  residue on every claimed operating system.

### P1 — self-dogfood and Beamfall shadow

- Corvint Python adapter runs in shadow against every full local gate for at least 200 eligible changes.
- Beamfall Go adapter runs in shadow against `make gate` for at least 200 eligible changes.
- The selector cannot suppress a project-mandated gate; shadow savings are measured separately from
  the authoritative full result.
- Any mismatch is retained, classified, repaired, and replayed rather than removed from the corpus.

### P2 — full-CI shadow safety

Across at least 1,000 sealed changes spanning ten repositories and the qualified language tuples:

- zero missed release-blocking failures relative to the registered full CI oracle;
- at least 95% recall for all independently labelled affected failing tests;
- at least 50% lower median executed-test compute and wall time;
- no more than 5% unnecessary full-suite widening after excluding cases preregistered as inherently
  dynamic or unsupported;
- 100% selection/exclusion receipt verification and zero stale-result attachment; and
- calibrated `UNKNOWN_SCOPE` superior to an always-narrow baseline on Brier score and selective risk.

One missed release-blocking failure fails P2 and returns automatic selection to shadow-only.

### P3 — continuous-experience performance

On preregistered small, medium, and large repositories using production builds and clean machines:

- filesystem/editor edit acknowledgement p95 at most 50 ms;
- warm plan publication p95 at most 100 ms;
- first terminal result for eligible unit-edit changes p95 at most 500 ms and median at most 200 ms;
- stale decoration withdrawal p99 at most 50 ms after a newer edit is observed;
- idle coordinator CPU median below 1% of one core and bounded resident-memory profiles published by
  repository class; and
- no result loss or ordering violation during a 100-edit burst and 24-hour soak.

These are promotion gates, not current measurements. Slow test execution is reported separately from
Corvint selection and transport overhead.

### P4 — independent usability and proof value

- At least 30 developers complete debugging and change-verification tasks across all three language
  families with non-inferior correctness and at least 30% lower median time to the first actionable
  failure than conventional watch/full-test workflows.
- At least 90% of sampled selection explanations are judged correct and useful by maintainers.
- Maintainers detect all planted stale, partial-pass, provider-conflict, and unknown-scope cases.
- Agent-assisted tasks show non-inferior correctness and fewer unsupported “tests pass” claims than
  the same agents using raw runner output.

## Failure, degradation, and rollback

| Failure | Required behavior | Recovery/rollback |
|---|---|---|
| Identity cannot close | `UNKNOWN_SCOPE`; no current result | repair observer or run explicit full command |
| Incremental/clean differential | invalidate affected cache and graph | rebuild clean; retain differential fixture |
| Provider crash, hang, or malformed event | terminate owned tree; incomplete scope unknown | restart pinned provider or show full-command fallback |
| Selection miss against full CI | disable autonomous narrowing for tuple/profile | shadow-only until cause repair and full replay pass |
| Coverage/instrumentation mismatch | discard coverage evidence, not test result | rerun qualified instrumentation or expose absence |
| Secret-screening/policy failure | do not persist raw observation | revoke handle and retain only bounded failure receipt |
| Upgrade/conformance failure | do not activate candidate | restore last qualified binary/configuration atomically |
| Coordinator corruption | stop serving current evidence | delete rebuildable state and reconstruct from authority |
| Client/API incompatibility | explicit unsupported-version state | use compatible API or disable client; never guess |

The kill switch disables continuous workers and automatic narrow selection without changing repository
files. The always-available recovery is the repository's ordinary explicit test or gate command.

## Product and licensing boundary

The proposed product boundary is described in an internal product brief (not part of the public tree). This spec
does not amend `LICENSE`, `LICENSING.md`, ownership, contributor terms, or any third-party license.
Protocol schemas, conformance vectors, Core behavior, commercial binaries, and thin integrations may
have different existing or future terms only through an owner-approved licensing decision.

Offline license issuance, activation, renewal, revocation, organization identity, metering, and
commercial enforcement are future external product concerns. They are not V0 verification features,
MUST NOT enter the portable protocol or evidence receipts, and require separate security, privacy,
availability, legal, and failure-mode review before implementation.

## Non-goals

- Claiming Wallaby's established JavaScript capabilities as Corvint inventions.
- Shipping or claiming parity with Wallaby, any language, framework, IDE, or operating system in V0.
- Time-travel debugging, deterministic replay, heap snapshots, profiler replacement, or universal
  runtime-value capture.
- Replacing `go test`, `pytest`, `unittest`, Jest, Vitest, `node:test`, coverage tools, debuggers, CI,
  repository gates, or framework-native authority.
- Treating coverage, a trace, a selected test pass, or full CI as proof of general program
  correctness or change safety.
- Generating tests, weakening assertions, rewriting product code, mutating repository configuration,
  auto-fixing failures, merging, deploying, or publishing.
- Cloud execution, cross-tenant data, telemetry by default, a hosted dashboard, or required account
  connectivity.
- A security-sandbox claim, arbitrary untrusted-code execution service, or secret-management system.
- Learning-based test omission before a separately accepted, outcome-attested, fail-closed profile
  satisfies the conservative plan.
- Implementing licensing, entitlement, payment, account, or update-policy machinery in the portable
  protocol or Corvint Core as part of this slice.

## Traceability

| Requirement group | Delivery | First authoritative evidence |
|---|---:|---|
| `LPCV-V0-001..006` exact workspace identity | not-started | cross-platform WEI golden and stale-event adversarial corpus |
| `LPCV-V0-007..012` incremental graph | not-started | clean/incremental differential suite |
| `LPCV-V0-013..020` selection and widening | not-started | 1,000-change full-CI shadow corpus |
| `LPCV-V0-021..026` provider protocol | not-started | two independent provider implementations and conformance suite |
| `LPCV-V0-027..031` runtime evidence | not-started | privacy/coverage/runtime-value adversarial vectors |
| `LPCV-V0-032..036` IDE and agent API | not-started | identical-receipt two-client conformance |
| `LPCV-V0-037..042` containment and updates | not-started | process-tree, path, permission, network, and rollback matrix |
| `LPCV-V0-043..046` language tranche | not-started | Corvint, Beamfall, and independent JS/TS shadow reports |
| `LPCV-V0-047` five-axis projection on every presenting client | delivered for the experimental CLI and VS Code; MCP V0 out of scope (decision 0103); experimental descendant MCP profile `MTV-V0-001..009` (decision 0147) | `TestTestValidityProjectsReceiptWithoutTrustingCarriedProjections` exercises the CLI projection; `TestTestProjections` and `extensions/vscode/test/liveTestProtocol.test.ts` exercise the Go-session projection consumed by VS Code. MCP 2026-07-28 V0 presents no test-level result (`MCPV0-008`), and the descendant projection is the experimental `corvint-test-validity-mcp` profile (`MTV-V0-001..009`, decision 0147) rather than MCP V0. |
| `LPCV-V0-048` passing does not imply adequacy | delivered | `TestTestValidityProjectsReceiptWithoutTrustingCarriedProjections` and `TestProjectionForState` keep strength `NOT_MEASURED` for passing execution without mutation evidence. |
| `LPCV-V0-049` wholly unsupported input abstains on every axis | delivered | `TestTestValidityWithoutReceiptAbstainsOnEveryAxis`; the shared `unsupported-input` case is also exercised by `TestProjectMatchesSharedVectors` and `extensions/vscode/test/testvalidity.test.ts`. |
| `LPCV-V0-050` shared Go/TypeScript vector agreement | delivered | `TestProjectMatchesSharedVectors`, `TestSharedVectorsCoverRequiredCases`, and `extensions/vscode/test/testvalidity.test.ts` all consume `conformance/test-validity-v0/vectors.json`; an Electron-host execution is `NOT_RUN`. |
| `LPCV-V0-051` experimental CLI provider projection | delivered for JavaScript receipts and completed Go preview-session events; Go input is non-promotable | `TestTestValidityProjectsReceiptWithoutTrustingCarriedProjections`, `TestTestValidityWithoutReceiptAbstainsOnEveryAxis`, `TestTestValidityRefusesNonReceiptInput`, `TestTestValidityRefusesSpecialReceipt`, `TestTestValidityReadsReceiptSymlink`, `TestReceiptBoundsOpenedFile`, `internal/testvaliditydoc`'s `TestGoSessionIsProjectedFromObservations`, `TestGoSessionCarriedProjectionsAreIgnored`, `TestUnknownAndAmbiguousProviderKindsAreRefused`, `TestGoSessionDisclosesPreviewTierAndCannotPromote`, and the `test-validity` case in `TestHelpInvocationsAreDeterministicAndDoNotInspectRootOrStdin`. The `go-session-recomputed-preview` MCP vector exercises the same shared builder; no qualified Go session matrix has run. |
| `LPCV-V0-052` a reported skip is `SKIPPED`, never a pass | delivered | The `skipped-reported-result` and `skipped-with-cause-abstains` vectors in `conformance/test-validity-v0/vectors.json` (decision 0149) are consumed by `TestProjectMatchesSharedVectors`, `TestSharedVectorsCoverRequiredCases`, and `extensions/vscode/test/testvalidity.test.ts`. |
| `LPCV-V0-053` bounded read-only discovery of retained evidence | delivered at experimental tier for the CLI and the descendant MCP profile (decision 0202); the opt-in producer retention of `LPCV-V0-055` writes into the location | `internal/testvaliditydoc`'s `TestDiscoverSelectsNewestRetainedEvidence`, `TestDiscoverRefusesSymlinkedEvidence`, and `TestDiscoverWithoutEvidenceIsUnsupported`; `cmd/corvint`'s `TestTestValidityDiscoveryMatchesMCPDocument` (CLI and MCP documents canonically byte-equal, `--discover --receipt` refused); vectors `discover-retained-evidence` and `discover-with-receipt-refused` in `TestVectorsAndReadOnly`. |
| `LPCV-V0-054` discovered freshness is bound to current digests | delivered at experimental tier; Go events stay preview and never `CURRENT` | `TestDiscoverUnmatchedIdentityIsNeverCurrent` (changed and removed test file `STALE`, outside-worktree path and Go session `UNKNOWN`, stale Go session `STALE`, each with its reason) and `TestDiscoverSelectsNewestRetainedEvidence` (matched digest `CURRENT`); package and app-build digests are not recomputed, so they are `UNKNOWN` by construction, not measured. |
| `LPCV-V0-055` opt-in bounded atomic producer retention | delivered at experimental tier (decision 0218); no real-provider Electron run and no qualified Go session matrix has retained evidence (`NOT_RUN`) | `cmd/corvint-js-test-provider`'s `TestEmitRetainsStdoutBytesPrunesAndRefusesSymlink` and `cmd/corvint-go-test-provider`'s `TestSessionRetainsCompletedEventBytesPrunesAndRefusesSymlink` (retained bytes equal stdout, 32 kept, a foreign entry untouched, a symlinked `.corvint` refused with stdout still emitted); `internal/testevidence`'s `TestRetainReportsDocumentItsPruneRemoved` (a document named behind 32 own names is pruned and reported as a failure) and `TestRetainPrunesCrashLeftoverTemporaries` (225 own temporaries named below the oldest kept document are removed, a newer and a foreign temporary kept); `cmd/corvint`'s `TestTestValidityDiscoverSelectsProducerRetainedDocument` (a retained document is the one `test-validity --discover` selects, `CURRENT`).; the VS Code extension passes the flag only behind `corvint.liveTests.retainEvidence` (decision 0230, `VSC-V0-070`), unit-tested, with no Electron run. |
