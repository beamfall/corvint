# Runner Selection Plugins V0

Owner: Russell Lewis
Date: 2026-11-07
Intent status: accepted (decision 0592: owner 2026-11-07 for its §1-§2; §3-§7 orchestrator-made under the 2026-11-02 delegation, owner may override)
Delivery status: not-started
Authoritative inputs: owner chat of 2026-11-07 (always run only the required minimum tests, "it
should be perfect"; Go, other languages and E2E, "beamfall support basically"; "each langauge and
runner should be a plugin so other langauges and runners can be easily added"; "Yes, both forms":
compiled-in plugins and an out-of-process runner plugin protocol); decision 0592 (the owner's
choices and the orchestrator-made defaults); decision 0590 and `live-proof-carrying-verification-v0.md`
`LPCV-V0-950..979` (per-test coverage records, freshness, line intersection, misses, qualification);
`language-plugins-v0.md` (`LGP-V0-001..027`); decision 0591 and `core-bundled-apps-v0.md`
(`CMS-V0-001..011`, invariant 7); `analyzer-capability-contract-v0.md` (`ACC-V0-002`, `ACC-V0-005`);
`affected-plan-v0.md` (`AFP-V0-013`, `AFP-V0-014`: trusted planner and trusted qualification
artifact); `affected-plan-selection-v0.md` (`AFP-V0-2484`, `AFP-V0-2485`: must-run and
declared-owner selections); `test-runner-execution-v0.md`; `map-coverage-join-v0.md`.

## Agent digest
- Claim: Runner plugins declare inventory, per-test evidence and qualification; one language-neutral selector narrows qualified runners and runs every other runner FULL.
- Status: accepted (decision 0592: owner 2026-11-07 for its §1-§2; §3-§7 orchestrator-made under the 2026-11-02 delegation, owner may override) / not-started
- Exists: nothing of this contract; it builds on `internal/langplugin`, `internal/testrunner/registry`, `internal/linecov`, `internal/jslinecov`, `internal/browsercov`.
- Blocked on: every slice; first proof is the evidence record and shadow selector over existing Go linecov records (RSP-V0-040).
- Read next: Requirements (Selector, Authority, Qualification); Acceptance evidence; Failure modes; Rollback.

## User job and verified baseline

The owner wants every run, local and CI, to execute only the tests a change can affect, across every
Beamfall runner: Go, TypeScript and JavaScript (node runners and Playwright E2E), Swift
(XCTest, XCUITest, Swift Testing), Kotlin and Java (JUnit, Espresso), C# (`dotnet test`), Rust
(`cargo test`), Python (pytest), shaders, and shell and make targets. Each language and runner is a
plugin, so adding one touches its plugin, never the selector.

"Perfect" is defined (orchestrator-made default, decision 0592 §3) as **sound within a qualified
execution model**: selected ⊇ affected for every runner whose pieces are qualified, an explicit FULL
for every other runner, every project-mandated test always run, and an auditable reason for each
excluded test. Exact minimality is not claimed. Anything uncertain widens toward FULL.

Baseline at `origin/main` c6b6cbb0:

- `internal/langplugin` declares fifteen language manifests over the closed piece set `provider`,
  `analyzer`, `runners`, `test-provider`, `coverage`, `mutation` (`LGP-V0-002`); runners resolve
  through the table-driven `internal/testrunner/registry` (`Build`, `Parse`).
- Per-test line records exist only as developer commands: Go (`internal/linecov`), node runners
  (`internal/jslinecov`, `LPCV-V0-965..969`), and browser E2E (`internal/browsercov`,
  `LPCV-V0-970`). Production exclusion is unadmitted until per-family qualification
  (`LPCV-V0-963`, `LPCV-V0-969`).
- The LPCV record holds executed lines only. Data files read at run time, directory listings, files
  a test looks for and does not find, and processes it starts are not observed, so `LPCV-V0-953`
  sends every such change to the static plan.
- No out-of-process runner exists. `analyzercap` (`corvint-analyzer/v0`, 64 KiB one-shot frames,
  registry-and-lock resolution, no `PATH`, `ACC-V0-002`) is the only external-executable protocol.
- `internal/analyzerexec` contains processes with `sandbox-exec` on macOS; Linux containment is
  unsupported there. No component traces a test's file or process activity.

## Requirements

### Pieces

- `RSP-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  `LGP-V0-002` closed piece set gains `inventory`, `test-evidence` and `qualification`. They are
  declared per runner, not per language: each runner a manifest lists either names all three or
  records each absent one with a non-empty reason, and `langplugin.Validate` refuses anything else.
  A runner with any of the three absent, or not qualified under this spec, is FULL (`RSP-V0-046`).
- `RSP-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  `inventory` piece enumerates every test identity (`LPCV-V0-022`) the runner would run for a given
  checkout and runner configuration, each with its source path, and the runner identity and
  configuration digest it was computed under. Two inventories of the same inputs are byte-equal.
  A missing, failed, truncated or unparseable inventory makes the runner FULL.
- `RSP-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Every
  run, selected or FULL, reconciles the runner's reported results with its planned run set
  (`RSP-V0-004`) and the inventory it ran from. Each planned test identity needs one terminal
  result: passed, failed, or skipped with a runner-reported reason. A planned identity with no
  terminal result is a **missing result**: the run cannot succeed, the core reruns that runner FULL
  in the same invocation (or reports the run failed when the caller forbids a rerun), and the plan
  names the missing identities. A missing result, or a reported test the inventory lacks, is
  **inventory divergence**: results already reported stand, and the runner is FULL until its
  inventory requalifies (`RSP-V0-036`).
- `RSP-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  Selection is exclusion-based: the run set is the head-side inventory minus the tests the selector
  excludes, plus every project-mandated test (`RSP-V0-043`). A test with no fresh record, including
  every test new on the head side, is never excluded. A runner whose native filter cannot express
  the exact run set runs a reported superset or FULL, never a subset.
- `RSP-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  `test-evidence` piece turns a run's native artifacts (coverage files, source maps, runner
  reports, build graph) into `RSP-V0-006` records; the core supplies the trace components
  (`RSP-V0-021`). A plugin never writes the record store directly.

### Evidence record

- `RSP-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) One
  language-neutral record, `corvint-test-evidence/1`, per test and run, naming: the test identity;
  the runner identity (plugin name, version and executable digest, runner version, configuration
  digest); the execution identity (OS, architecture, tracing backend and version, and the
  environment digest of `RSP-V0-044`); the producer provenance of `RSP-V0-042`; the commit; and
  these components, each `present`, `absent` or `incomplete`:
  - `executed`: per file and Git blob OID, executed ranges and the covered-and-unexecuted set
    (`LPCV-V0-950`);
  - `regions`: per recorded blob, its executable body ranges and enclosing-body map;
  - `loaded`: repository files compiled into or loaded by the test's processes;
  - `reads`: repository paths read, with blob OIDs;
  - `listings`: repository directories enumerated;
  - `lookups`: repository paths probed and found missing;
  - `writes`: repository paths written, with the write kind (`create`, `replace`, `append`,
    `overwrite`, `truncate`);
  - `execs`: executables started, with path, digest and whether coverage instrumented them;
  - `external`: whether any process in the tree reached a process outside it (non-loopback or
    loopback network, Unix socket, named pipe or IPC to a pre-existing process).
- `RSP-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Paths
  are repository-relative with both the lexical and the resolved path of a symbolic link recorded;
  on a case-insensitive volume both are case-folded for matching. A read of a path the same process
  tree wrote first is dropped from `reads` only when that write proved the path had no prior
  content: an exclusive create that succeeded, or a create after the tree's own failed lookup of
  the path (`create`), or a rename onto the path of a file the tree itself created (`replace`).
  After an append, a partial or positioned overwrite, or any truncation, the read edge stays,
  because the bytes read can still depend on the original content. Paths outside the repository
  are kept only as counts per class (toolchain, cache, temporary, other) for audit and never decide
  selection, because a diff cannot change them; a path inside a declared sibling repository is
  governed by `RSP-V0-049` instead, and any other outside path by its last clause.
- `RSP-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  Records live in the `LPCV-V0-958` immutable index (derived state, rebuildable, never authority),
  extended with the new components. An existing `linecov` record reads as `executed` present and
  every other component `absent`. The `LPCV-V0-958` size and 100 ms selection bounds still hold.
- `RSP-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  component is attributed to one test only when it came from a process tree that ran that test
  alone, or from runner start and end markers inside one process that the runner's qualification
  covers (`LPCV-V0-962`). Otherwise it is attributed to every test that ran in that tree. Build-phase
  activity the plugin's build graph models goes to `loaded`; every path a build phase reads, lists,
  probes or executes also stays in the `reads`, `listings`, `lookups` or `execs` of every test of
  that build (`RSP-V0-054`), so rules (d) to (f) apply to it whatever that test's coverage of the
  same path.
- `RSP-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  record is fresh for a planned run when `LPCV-V0-951` holds for every path in `executed`,
  `loaded`, `reads` and `execs`, and its runner and execution identities equal the planned run's.
  A stale record decides nothing (`RSP-V0-004`). Records of one test union under `LPCV-V0-956`.

### Selector

- `RSP-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) One
  selector reads only the diff, the records and qualification records admitted under
  `RSP-V0-042`, the inventories, the project's plugin configuration, and the project's must-run
  and declared-owner configuration (`RSP-V0-043`). It holds no language or runner name literal (the
  `LGP-V0-009` ratchet, baseline 0 for its package), never launches a plugin, runner or tracer, and
  writes nothing (invariant 4).
- `RSP-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) For a
  test T with a fresh record and a changed path P, every rule below that holds selects T and is
  named in the plan. Selection is the union of the rules; no rule excludes.
  - (a) P is a runner configuration path the plugin declares (manifests, lockfiles, runner
    configs): the whole runner is FULL;
  - (b) P is in T's `executed` instrumented set, every changed base range of P lies inside a body
    of `regions`, and `LPCV-V0-952` intersection holds;
  - (c) P is in T's `executed` instrumented set or `loaded`, and the precondition of (b) (P in
    `executed` with every changed base range inside a body) does not hold;
  - (d) P, or the lexical or resolved path of a link through which T reached P, is in `reads` or
    `execs`;
  - (e) P is added: its nearest base-side existing ancestor is in `listings`, or P or one of its
    ancestors missing at base is in `lookups`;
  - (f) P is deleted or renamed away: its parent is in `listings`, or (c) or (d) holds;
  - (g) a mode or link-target change is a content change of P.
  The only effect of a (b) non-intersection is that (c) does not select T for P; it never sets
  aside (d) to (g), `RSP-V0-013` or `RSP-V0-043`. A test that executes none of P's changed lines
  but reads P as data is selected by (d). T is excluded only when no rule selects it for any
  changed path, `RSP-V0-013` selects nothing, and `RSP-V0-043` does not mandate it.
- `RSP-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A rule
  that needs a component T's record has `absent` or `incomplete` selects T, naming
  `component-missing:<name>`. Rules (b) and (c) need `executed`, `regions` and `loaded`; rules (d)
  to (f) need `reads`, `listings`, `lookups` and `execs`. A test whose `external` is observed or
  not `present` is always selected. The one exception is a runner qualified under the `untraced`
  model, only as `RSP-V0-045` states; no other runner, OS or device class narrows with a trace
  component missing.
- `RSP-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  boundary classes are: `body-edit`, `declaration-edit`, `data-edit`, `add`, `delete-rename`,
  `exec-target`, `runner-config` and `e2e-server`. Each changed path is assigned one class from the
  rule that decided it. A runner narrows on a class only once its qualification passed that class
  (`RSP-V0-034`); a change in an unqualified class makes the runner FULL.
- `RSP-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  runner is FULL, with the reason named, when: it is not qualified under this spec, including a
  piece absent (`RSP-V0-001`, `RSP-V0-046`); its plugin fails (`RSP-V0-031`); its inventory is
  missing or divergent; the index for the compared base is missing, stale or unverifiable; its
  evidence or qualification is not admitted (`RSP-V0-042`); the change holds an unqualified class;
  its configuration changed; its execution configuration is outside its qualification
  (`RSP-V0-044`); or a confirmed miss disabled it (`RSP-V0-037`).
- `RSP-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  plan names, per runner, its mode (`selected` or `full`, with the `RSP-V0-015` reason) and
  execution model; per excluded test, the record digest, the components consulted, the changed
  paths and classes, and the invalidation condition "any recorded blob OID or identity changes";
  per selected test, every rule that selected it; per mandated test, its mandate (`RSP-V0-043`);
  and the `LPCV-V0-961` counts and timings.
- `RSP-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  plan states its soundness claim as: every test excluded for a `selected` runner is outside every
  rule of `RSP-V0-012` under that runner's qualified classes and execution model, and every
  project-mandated test is in the run set. It never claims minimality, and never claims soundness
  for a shadow plan made before enforcement (`RSP-V0-046`).

### E2E

- `RSP-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) An E2E
  runner is a runner plugin plus the application or server under test built with coverage and
  started inside the traced process tree (the runner's own web-server configuration or a command
  the plugin declares). Server coverage joins the test's record under `LPCV-V0-954`; browser
  coverage comes from `LPCV-V0-970`. A server started outside the tree is `external`, so its tests
  are always selected.
- `RSP-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Server
  coverage is attributed per test only through a per-test reset and dump the qualification covers,
  or by running tests serially against a fresh server; otherwise it is attributed to every test that
  shared the server (`RSP-V0-009`). Device and simulator processes (Espresso, XCUITest) are
  outside the trace, so their tests' trace components are `absent` and `RSP-V0-013` selects them,
  unless the runner is qualified under the `untraced` model (`RSP-V0-045`).

### Tracing service

- `RSP-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  Tracing is a core service, not a plugin piece (decision 0592 §5): one compiled-in backend per OS
  behind one interface, used only by test-execution paths (`corvint test`, collectors,
  qualification), never by a read command.
- `RSP-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  service records, for the whole process tree of a run, opens and reads, failed lookups, directory
  enumerations, writes with their kind, `exec` calls, and connections to processes outside the
  tree, and fills the `reads`, `listings`, `lookups`, `writes`, `execs` and `external` components.
  It runs unprivileged, with no daemon, kernel module, kernel extension, root, network or persisted
  state.
- `RSP-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  Linux backend uses `ptrace` with a seccomp-BPF filter so only the traced system calls stop the
  tracee, follows `fork`, `vfork`, `clone` and `exec`, and covers every path-taking variant
  (`openat2`, `statx`, `faccessat2`, `getdents64`, `execveat`, `renameat2`, `connect`). A process it
  cannot trace (setuid, ptrace refused by policy or a container), an unknown path-taking system
  call, or more than the configured event or path cap marks the trace components `incomplete`.
- `RSP-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) macOS
  and Windows backends are later slices with their own qualification. Until one qualifies, the
  trace components are `absent` on that OS, so `RSP-V0-013` selects every test there; its runners
  narrow only under the `untraced` model (`RSP-V0-045`) qualified on that OS.
- `RSP-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  Tracing must not change outcomes: qualification runs the corpus traced and untraced, and a test
  whose outcome differs disables tracing for that runner (components `absent`). Trace overhead is
  measured per runner and retained; it is a reported cost, not a selection input.

### Two plugin forms and the runner protocol

- `RSP-V0-025`: (accepted, decision 0592 (owner 2026-11-07, "Yes, both forms")) A runner's pieces
  are provided in one of two forms: compiled-in, implementing the in-process runner-plugin
  interface; or out-of-process, one `corvint-runner-<name>` executable speaking `corvint-runner/1`.
  The selector, the store and the conformance kit treat both alike. Go, shell and make, and the
  JavaScript and TypeScript runners stay compiled-in (hot path).
- `RSP-V0-026`: (accepted, decision 0592 (owner 2026-11-07, "Yes, both forms")) `corvint-runner/1`
  is UTF-8 JSON over the plugin's standard input and output, one object per line, each request
  carrying an `id` its response echoes. Decoding is strict: an unknown member, a duplicate member,
  a wrong type or invalid UTF-8 is a protocol error. Standard error is diagnostic only.
- `RSP-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  methods are `hello` (protocol version, plugin name and version, runners served, pieces offered,
  optional features), `command` (the argv, working directory and environment for a purpose:
  `inventory`, `run`, `collect`; no shell string), `inventory` (parse the core-captured output into
  `RSP-V0-002` entries), `parse` (native report to per-test results), `evidence` (artifacts to
  `RSP-V0-006` components other than the trace components) and `shutdown`. The core executes every
  runner command under its tracer; the plugin never starts the runner or any test process.
- `RSP-V0-028`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  Discovery is declared, bounded and project-owned: the tracked `.corvint/runner-plugins.json`
  names each plugin, its protocol, its SHA-256 executable digest and the runners it serves. The core
  resolves the executable only from the release manifest (bundled plugins) or the per-user plugin
  store outside every repository, keyed by digest; never from `PATH`, a repository path, a URL or a
  download. It opens the file without following links, verifies the digest and executes that same
  file (`ACC-V0-002`). A missing or mismatched plugin makes its runners FULL. In CI the pins are
  read from the trusted base (`RSP-V0-042`); a candidate that changes the pin file makes every
  runner the file names, before or after the change, FULL.
- `RSP-V0-029`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) One
  plugin process per run serves every request of that run (decision 0591 §4). Defaults, which the
  project may lower but not raise past the ceiling: message 1 MiB (ceiling 8 MiB); session output
  256 MiB (1 GiB); retained standard error 64 KiB; `hello` 5 s; any other request 60 s (600 s).
  The plugin runs in its own process group with the repository as working directory, is killed with
  the group 2 s after `shutdown` or standard-input end, and must leave the repository's tracked and
  untracked status unchanged, which the core checks after the session.
- `RSP-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Plugin
  output is evidence, never authority: a plugin cannot exclude a test, mark itself or a runner
  qualified, or change a class. Qualification results are computed by the core harness, keyed by
  plugin digest, and admitted only under `RSP-V0-042`, so a new digest starts unqualified.
- `RSP-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  plugin launch failure, crash, timeout, oversize message, protocol error, schema violation
  (`RSP-V0-047`), `error` response, version mismatch, digest mismatch, repository mutation, or
  response naming a runner it did not declare in `hello` ends the session, publishes no record from
  it, and makes that plugin's runners FULL for the run, with the failure named in the plan.
- `RSP-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  `corvint-runner/1` changes only additively through features named in `hello`; a breaking change is
  `corvint-runner/2`, and the core supports the current and previous major versions. When the
  `corvint-plugin/1` envelope (V1-4819) lands, this protocol becomes its runner profile with these
  requirement IDs kept.
- `RSP-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Every
  `corvint-runner/1` request and response has a JSON Schema in the conformance kit with the shapes
  of "Protocol messages" below, and the kit validates every frame in both directions against it.
  Version negotiation: the `hello` request lists the protocol majors the core supports; the plugin
  answers with exactly one of them; an answer naming a major the core did not offer, or none, is a
  version mismatch (`RSP-V0-031`). A feature the plugin did not name in `hello` is never used in
  that session.

### Authority

- `RSP-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) CI
  narrowing derives only from evidence and qualification produced under protected authority,
  following the trusted-planner and trusted-artifact rules of `AFP-V0-013` and `AFP-V0-014`. The
  selector, qualification harness, tracer, `.corvint/runner-plugins.json` pins, qualification
  declarations, the non-semantic environment list (`RSP-V0-044`) and the sampling configuration
  (`RSP-V0-037`) are taken from the trusted base (the protected base revision), never from the
  candidate under test. A record index or qualification record is admitted only when its producer
  provenance (producing core digest, verifier revision, configuration digest, producing ref and
  run, content digest) verifies: the core digest, verifier revision and configuration digest equal
  the trusted base's bindings, the producing ref is protected, the artifact was retained where
  candidate runs cannot write, and its content matches the digest. The index used to select for a
  candidate holds no record produced from that candidate. A candidate that changes any trusted
  input gets FULL for every runner that input governs. Anything not admitted (a forged
  qualification record, fabricated or injected coverage, a pin or corpus fixture the candidate
  modified) makes its runner FULL, with the reason named. Local selection may admit records its own
  core produced, and stays no narrower than CI for the same inputs (`RSP-V0-038`).
- `RSP-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) For a
  change with at least one changed path, every project-mandated test is in the run set whatever the
  evidence says: every must-run selection of `AFP-V0-2484` and every declared-owner selection of
  `AFP-V0-2485`. No exclusion removes one: not a (b) non-intersection, the `untraced` model, a
  fresh record, a narrowed native filter, or any other rule of this spec. A unit `AFP-V0-2485`
  requires whole runs whole; a runner whose native filter cannot express that runs a reported
  superset or FULL.
- `RSP-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  environment digest of `RSP-V0-006` hashes every variable's name and value except the names on
  the core's versioned non-semantic list. A name enters that list only through a core change whose
  qualification shows, per runner and execution configuration, that varying the variable changes
  no outcome and no record component; a plugin may propose a name, as evidence only. A variable
  whose value cannot be captured for hashing makes every test of its runner selected. A
  qualification applies only to the execution configuration it ran under (the argv template minus
  the test filter, runner configuration digest, environment digest, OS, architecture and tracing
  backend); a run under any other execution configuration is unqualified and FULL.
- `RSP-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  `untraced` model is the only way a runner narrows with trace components missing. It assumes the
  runner's tests read no repository file as data, enumerate or probe no repository path, execute no
  tracked script, and reach no outside process whose behaviour depends on the repository. Under it,
  `executed`, `regions` and `loaded` must still be `present`; rules (d) to (f) are not evaluated;
  and any changed path outside every test's `executed` and `loaded`, or any change classed other
  than `body-edit` or `declaration-edit`, makes the runner FULL. Its qualification adds a trial
  class of byte mutants in unexecuted body ranges of sampled `loaded` files, so a test that reads
  source as data is a miss that blocks the model. The plan names the model and its assumptions.
  Device and simulator runners (`RSP-V0-019`) and runners on an OS with no qualified tracing
  backend (`RSP-V0-023`) narrow only under it.

### Qualification

- `RSP-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  `qualification` piece declares the runner's corpus (a pinned repository name and commit; the
  checkout path is an operator input), the boundary classes and execution model it claims (`traced`
  or `untraced`, `RSP-V0-045`), the execution configuration it qualifies (`RSP-V0-044`), and its
  mutation operators (from the language's `mutation` piece; the core supplies the language-neutral
  file operators).
- `RSP-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  core harness qualifies one runner on its corpus by: (a) replay over pinned merges, where every
  test whose FULL outcome differs between base and head must be selected; (b) fault injection per
  claimed class: `LPCV-V0-963` line mutants for `body-edit`, declaration mutants, byte and
  truncation mutants of sampled read data files, a file added in a listed directory and at a
  looked-up path, deletion and rename, mutation of an exec'd tracked script or tool source, a
  server-side mutant for `e2e-server`, and a configuration edit that must yield FULL; (c) one
  negative control per class; and (d) the traced-versus-untraced parity of `RSP-V0-024`. Trials and
  misses are defined by `RSP-V0-035`; any miss, and any replay test left unselected, blocks that
  class.
- `RSP-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  sampling population of a class is every mutant its operators generate over the corpus at the
  pinned commit; mutants are drawn independently and uniformly from it with a recorded seed. A
  trial is one sampled mutant that FULL kills (at least one test fails on it); unkilled and
  equivalent mutants are counted but are not trials. A trial is a miss when any test that fails on
  that mutant under FULL is not selected. The qualification record states per class the seed, the
  population size, mutants sampled, unkilled, trials, selected-set sizes, every miss, and the exact
  one-sided 95% Clopper-Pearson upper bound on the per-trial miss probability (`1 - 0.05^(1/n)`
  for zero misses in n trials: 0.995% at n = 300). The bound claims nothing outside that
  population, and the record never states a zero miss rate. A class with fewer trials than the
  floor in decision 0592 §4 (initially 300) stays unqualified.
- `RSP-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  change to the plugin digest, runner version, tracing backend, selector version, record format,
  execution configuration (`RSP-V0-044`), non-semantic environment list or verifier revision
  (`RSP-V0-042`) invalidates the runner's qualification. Narrowing per runner and class is derived
  from the newest admitted, passing qualification record for the exact identities, never from a
  hand-set flag.
- `RSP-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Sampled
  FULL runs on the default branch extend the `LPCV-V0-957` miss detector to every class. No runner's
  narrowing is enabled until the trusted configuration sets a nonzero sampling rate; a zero or unset
  rate keeps every runner FULL. Each sampled run compares against the plan that actually ran, as
  retained: its exact base and head commits and its original excluded set, never a recomputed one.
  An excluded test whose FULL outcome at head differs from its outcome at that base is a candidate
  miss; one rerun of it at the same base and head confirms it. A confirmed miss disables that
  runner's class until repaired and requalified, and is retained with the plan. The rate's value and
  its CI cost bound are open numbers in decision 0592.

### Coverage breadth

- `RSP-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Ruby is
  in scope (owner 2026-11-07). The `ruby` plugin's runners (`rspec`, `minitest`, `test-unit`,
  `rails-test`) declare the three pieces, with `executed` from `internal/rubylinecov` plus trace
  components, and qualify on `conformance/ruby-profile-v0/rails-app` and any Beamfall Ruby
  repository at step 7 of `RSP-V0-040`. Until then they are FULL (`RSP-V0-046`).
- `RSP-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A test run
  may declare sibling repositories (`--repository ID=DIR`, the `affected` multi-repository model).
  An entry of any path-bearing component (`reads`, `listings`, `lookups`, `execs`) whose resolved
  path lies inside a declared repository is recorded with that repository id, its path relative to
  that repository and, where the path exists, its blob OID, and takes part in freshness and rules
  (a) to (g) against that repository's diff between its pinned base and head. Such an entry outside
  the repository, outside every declared repository and outside the toolchain, cache and temporary
  classes marks its component `incomplete` for that test, which selects it. A change to a sibling
  pin (submodule, lock entry or checkout reference) is a change to every path of that repository
  named in any path-bearing component, so a test that only probed a missing sibling path is
  selected.
- `RSP-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A `script`
  runner kind inventories one identity per command declared in a tracked, project-owned file
  (`.corvint/test-commands.json`; Corvint's `.github/cishards/test-commands.json` is one instance
  of the same schema), runs each command under the tracing service and narrows on file-level
  components only (`reads`, `listings`, `lookups`, `execs`, `external`); it never narrows on
  `executed`. For this kind, rules (b) and (c) do not apply and `RSP-V0-013` requires only the
  components rules (d) to (f) need: `executed`, `regions` and `loaded` are declared `absent` with
  reason `script-kind` and select nothing on their own. It
  covers direct test scripts (`node test/*.ts`, `python3 script/*_test.py`, `script/*_test.sh`),
  aggregate build-tool tasks (Gradle `fastGate`) and compile checks (`xcrun metal`). A command not
  in the declaration is outside every inventory and the runner that would run it is FULL.
- `RSP-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Inventory is
  keyed by target OS and architecture. A test inventoried for one target and absent from another
  is FULL on a target without its own inventory and record; Go build constraints are enumerated per
  CI matrix entry. A record decides only on the target it was collected on, unless the runner's
  qualification covers cross-target reuse.
- `RSP-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Trace
  activity of a build phase (code generation, template rendering, asset compilation) that the
  plugin's build graph does not model is attributed to every test of that build, never dropped, as
  the `reads`, `listings`, `lookups` and `execs` entries of each such test (`RSP-V0-009`); a test's
  instrumented coverage of a path never removes that path's build-phase read entry.

### Record quality and read surface

- `RSP-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A read
  command (`corvint test-evidence`) answers, for a path or line range at a commit, every test whose
  fresh record reaches it, with the component, record digest and record commit; for a test
  identity, the components of its record; and with `--explain TEST` against a diff, the rule chain
  that selected or excluded it. Where no fresh record exists it abstains `UNKNOWN` with the reason.
  It writes nothing (invariant 4) and is exposed through the same flow projections that
  `application-flow-understanding-v1.md` uses to link source, tests and run evidence.
- `RSP-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) There is
  one E2E inventory and one record format: `AFU-V1-019`'s `e2e-safe` selection profile consumes the
  `RSP-V0-002` inventories and `RSP-V0-006` records, and its Playwright discovery is the Playwright
  runner's `inventory` piece. No second E2E selection model narrows a test this spec governs.
- `RSP-V0-055`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A record
  decides an exclusion only after at least two collections agree on `executed`, `reads` and
  `listings`, or the runner's qualification shows one collection suffices. Disagreement widens the
  record to the union (`LPCV-V0-956`) and the plan marks the test `nondeterministic`.
- `RSP-V0-056`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Record
  collection and inventory wall time per runner are measured and retained with the qualification
  record. Selection never waits on collection: a missing or still-collecting record selects its
  test.
- `RSP-V0-057`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A sampled
  FULL failure counts as a miss (`RSP-V0-037`) only after the `LPCV-V0-957` rerun rule confirms it
  as deterministic on the head; a failure that does not reproduce is recorded as a flake against the
  test and never disables a class.
- `RSP-V0-058`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) The
  environment digest of `RSP-V0-044` hashes each value with a keyed hash under a repository-local
  key, so a low-entropy value cannot be recovered from a shared index; a key change makes every
  environment digest stale.

### Policy and rollout

- `RSP-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) CI and
  local selection use the same records and rule (`LPCV-V0-959`, `LPCV-V0-960`). Local selection may
  stay broader than CI until a runner's runtime evidence qualifies on the local OS, and is never
  narrower than CI for the same inputs.
- `RSP-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592)
  Soundness beats frozen narrowing expectations: a selection-baseline or narrowing assertion that
  conflicts with a soundness fix is updated in the same change, with a build-log entry naming the
  fix and the tests it re-selects.
- `RSP-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Rollout
  order, each step enabling narrowing for a runner only on CI Linux after its qualification: (1)
  record format, index extension and shadow selector over existing Go `linecov` records, changing
  no plan; (2) the Linux tracing service; (3) `go-test` pieces, qualified on Corvint and the
  Beamfall Go repositories; (4) shell and make (Bats, declared make targets and the `script` kind of
  `RSP-V0-050`); (5) node runners (vitest, jest, `node --test`) and Playwright E2E; (6) the
  out-of-process protocol, conformance kit and one reference plugin; (7) pytest and `python -m
  unittest`, the Ruby runners of `RSP-V0-048`, `cargo test`, `dotnet test`, JUnit and Gradle, XCTest
  and Swift Testing, shader runners; device runners last. A step's narrowing takes effect only with an admitted qualification record
  (`RSP-V0-042`) and a nonzero sampling rate (`RSP-V0-037`); otherwise it stays shadow.
- `RSP-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  Beamfall or other repository may ship its own runner plugin: source in that repository, the
  operator installs the built executable into the plugin store, and the repository pins its digest
  in `.corvint/runner-plugins.json` and qualifies it on its own corpus. A plugin enters the release
  manifest only with its own spec and decision (decision 0591 §1).
- `RSP-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) A
  runner not qualified under this spec is FULL. Before `RSP-V0-040` enables its first runner (steps
  1 and 2), this spec is shadow-only and changes no plan: the existing plans (the
  `affected-plan-v0.md` static plan or FULL) keep governing every runner, and the shadow plan states
  that they are outside its soundness claim. From the first enabled step, no static narrowing
  survives for a runner this spec does not qualify.

## Protocol messages

Each request is `{"id": string, "method": string, "params": object}`; each response is
`{"id": string, "result": object}` or `{"id": string, "error": {"code": string, "message":
string}}`. Members are required unless marked optional; any other member is a protocol error
(`RSP-V0-026`). Paths are repository-relative or absolute inside the core-owned run directory.

| Method | `params` | `result` |
|---|---|---|
| `hello` | `protocols` (supported majors, descending), `core` (version) | `protocol` (one offered major), `plugin`, `version`, `runners` (names), `pieces` (per runner: names), `features` (names) |
| `command` | `runner`, `purpose` (`inventory`, `run`, `collect`), `tests` (identities; optional, absent for FULL), `run_dir` | `argv` (strings), `cwd`, `env` (string to string), `artifacts` (paths), `superset` (boolean: argv may run more than `tests`) |
| `inventory` | `runner`, `output` (path of the captured output) | `config_digest`, `tests` (each `id`, `path`) |
| `parse` | `runner`, `report` (path) | `results` (each `id`, `outcome` of `passed`, `failed`, `skipped`; `reason` required when skipped) |
| `evidence` | `runner`, `artifacts` (paths) | `records` (each `test`, and `executed`, `regions`, `loaded` as `RSP-V0-006` components) |
| `shutdown` | empty | empty |

## Design notes

**Why exclusion over inventory.** Selecting "tests whose evidence matches" would silently drop tests
the evidence never saw. Running "head inventory minus justified exclusions" makes every unknown
test run, which is the sound default.

**Why selection is a union.** A test can depend on one file in several ways at once, for example
executing it and reading it as text. An ordered first-match rule lets the line-coverage argument
(rule b) hide the read dependency; the union keeps every observed dependency, and a
non-intersection only stops the coarse "file was loaded" rule (c) from selecting.

**Why tracing is a core service (decision 0592 §5).** System calls are the same for every runner, so
one backend per OS serves all plugins; an observer must not be the observed, and a plugin's
self-reported read set is unverifiable; qualifying the tracer once per OS is cheaper than per plugin;
and since out-of-process plugins never execute runners (`RSP-V0-027`), the core owns execution
anyway. Invariant 7: the service is Go code in the core's test-execution path, adds no dependency,
daemon, privilege, kernel component or network, and is unreachable from read commands. Its size and
start cost are measured under `CMS-V0-006`/`-011`; if it fails them it may move to a bundled app
under its own decision.

**Why `ptrace` + seccomp on Linux.** `LD_PRELOAD` misses static Go binaries; `fanotify` misses failed
lookups and needs privilege; eBPF needs privilege. `ptrace` of descendants works unprivileged under
the default Yama scope, including on hosted CI runners, and the seccomp filter limits stops to the
traced calls.

**Why declared discovery and a trusted base.** `PATH` is ambient and unpinned, while records key on
plugin digest; a repository executable path would let a checkout choose the code that produces its
own evidence. A digest pin in project config plus a store outside the repository keeps identity
exact, the same choice `ACC-V0-002`/`-005` made for analyzers. For the same reason a candidate
cannot supply the pins, records or qualification that narrow its own run (`RSP-V0-042`).

**Invariant 7 (decision 0591).** Runners have many interchangeable implementations, the case 0591 §4
reserves plugins for. The selector never calls a plugin, so `affected` pays no plugin cost; plugin
overhead lands in test execution and is measured per boundary under `CMS-V0-010` against the
runner's own wall time. Operator-installed and repository-shipped plugins are outside the default
release manifest.

## Non-goals

- Exact minimality, or soundness outside a runner's qualified classes and execution model.
- Keeping static narrowing for an unqualified runner once this spec enforces (`RSP-V0-046`).
- Go `plugin`, shared objects, interpreters as plugin hosts, `PATH` discovery, downloads, automatic
  installation, a plugin registry service or daemon.
- A plugin deciding selection, qualification or class membership.
- Privileged tracing (eBPF, kernel modules, Endpoint Security entitlements, admin ETW).
- Tracing device or simulator processes, remote execution, flaky-test detection, test ordering, and
  reuse of earlier passes (`test-selection-precision-v0.md` owns PR rerun reuse).

## Trust boundary and limits

The plugin is trusted as much as the repository's own tests (the operator already runs those), but
its output is evidence only. The core owns launch, tracing, the store and qualification, and in CI
only trusted-base inputs and protected-ref artifacts can narrow (`RSP-V0-042`). Limits are
`RSP-V0-029` for plugins and the configured event and path caps for traces; exceeding any is
`incomplete` or a plugin failure, both of which widen. Every failure in this spec widens to FULL or
selects; none narrows.

## Failure modes

| Failure | Detection | Outcome |
|---|---|---|
| Runner lacks a piece, or is not qualified | `Validate`, `RSP-V0-001`, `-046` | runner FULL, reason named |
| Inventory missing, failed or divergent | `RSP-V0-002`, `-003` | runner FULL until requalified |
| Planned test has no terminal result | reconciliation, `RSP-V0-003` | run fails, runner rerun FULL |
| Test new on head, or no fresh record | `RSP-V0-004`, `-010` | test selected |
| Native filter cannot express run set | `RSP-V0-004`, `-043` | reported superset or FULL |
| Test executes none of P's changed lines but reads P | rule (d), `RSP-V0-012` | test selected |
| Read after append, overwrite or truncate | `RSP-V0-007` | read edge kept |
| Component absent or incomplete | `RSP-V0-013` | test selected, `component-missing:<name>` |
| Test reaches an outside process | `external` component | test always selected |
| Untraceable process or event cap hit | `RSP-V0-022` | trace components `incomplete` |
| Tracing changes an outcome | parity, `RSP-V0-024` | tracing disabled for runner |
| Plugin crash, timeout, oversize, bad frame | `RSP-V0-031`, `-047` | session ends, no record, runner FULL |
| Plugin digest mismatch or missing | `RSP-V0-028` | runner FULL |
| Plugin mutates the repository | status check, `RSP-V0-029` | plugin failure, runner FULL |
| Evidence or qualification not admitted | provenance, `RSP-V0-042` | runner FULL |
| Candidate changes pins, corpus or verifier | trusted base, `RSP-V0-042` | governed runners FULL |
| Mandated test otherwise excludable | `RSP-V0-043` | test runs (whole unit) |
| Unlisted or uncapturable environment variable | `RSP-V0-044` | hashed; uncapturable selects |
| Execution configuration outside qualification | `RSP-V0-044` | runner FULL |
| Change in an unqualified class | `RSP-V0-014` | runner FULL |
| Index missing or stale for base | `RSP-V0-015` | runner FULL |
| Sampling rate zero or unset | `RSP-V0-037` | every runner FULL |
| Confirmed miss on default branch | `RSP-V0-037` | class disabled until requalified |
| Selector names a language | ratchet, `RSP-V0-011` | test fails naming the literal |
| Narrowing baseline conflicts with a soundness fix | review, `RSP-V0-039` | baseline updated, build log names it |

## Acceptance evidence

Each slice is accepted on its own retained evidence; nothing here is observed yet. Every fixture is
a golden plan or run receipt; a positive fixture shows the narrowing or behaviour the requirement
permits, a negative one shows the widening it requires. A row passes only when both observed
outcomes equal the stated ones.

| Requirements | Positive fixture and pass condition | Negative fixture and pass condition |
|---|---|---|
| 001 | manifest with all three runner pieces validates | piece neither named nor absent with a reason: `Validate` refuses; absent piece: plan `full`, reason names it |
| 002 | two inventories of one checkout are byte-equal | truncated or unparseable inventory output: runner `full` |
| 003 | narrowed run with a terminal result per planned test succeeds | plugin drops one selected test's result: run not successful, FULL rerun, plan names the identity; extra uninventoried test: divergence, next plan `full` |
| 004 | test with a fresh, non-intersecting record excluded | head-new test and stale-record test: both selected; filter unable to express set: `superset` true or `full`, never a subset |
| 005, 008 | `linecov` record reads as `executed` present, others `absent` | plugin attempt to write the store directly: refused, no record |
| 006, 007 | read after exclusive create, and after rename of a tree-created file: no `reads` edge | read after append, after partial overwrite, after truncate: each keeps its `reads` edge; symlinked read records lexical and resolved paths |
| 009 | per-test tree attributes reads to that test only | shared tree without qualified markers: components on every test in it |
| 010 | record with unchanged blobs and identities is fresh | one changed blob or identity: record stale, test selected |
| 011 | selector package passes with no runner literal | selector holding a runner literal: ratchet test fails naming it |
| 012 | each rule (a) to (g) selects on its seeded diff | golden counterexample: T executes F but not its changed body lines and reads F as text: T selected by (d), plan names (d) |
| 013 | all components present: rule outcome stands | each component `absent` and `incomplete` in turn: `component-missing:<name>`; `external` observed: selected |
| 014, 034 | change in a qualified class narrows | change in an unqualified class: runner `full` |
| 015, 016, 017 | plan names mode, reasons, rules and invalidation per test | each `RSP-V0-015` cause seeded alone: runner `full`, reason names that cause |
| 018, 019 | coverage-built server inside the tree joins records | server outside the tree: tests selected; device runner without `untraced` qualification: tests selected |
| 020, 021, 022 | Linux fixture per traced call fills its component | setuid child, unknown path syscall, capped run: components `incomplete` |
| 023 | OS with a qualified backend narrows on its classes | OS with no backend: every test selected unless `untraced`-qualified there |
| 024 | traced and untraced outcomes equal | outcome differs: tracing disabled, components `absent` |
| 025, 026, 027, 047 | conformance kit passes identically on a compiled-in runner and its out-of-process twin; every frame validates against the schemas; `hello` agrees one offered major | unknown member, duplicate member, wrong type, invalid UTF-8, schema violation, unoffered major, `error` response: each ends the session, runner `full` |
| 028 | pinned digest resolves from the store | digest mismatch: refused, runner `full`; `PATH`-shadowing executable never consulted; candidate edit of the pin file: named runners `full` |
| 029, 031 | session within limits ends cleanly | crash, timeout, oversize, repository mutation, undeclared runner: each `full`, failure named, no record |
| 030, 042 | admitted qualification and base index from a protected-ref run narrow | forged qualification record, fabricated or candidate-produced coverage record, candidate-modified corpus pin fixture: each not admitted, runner `full`, reason named |
| 032 | additive feature named in `hello` is used | unnamed feature: never sent |
| 033, 035 | qualification record holds seed, population, trials, misses and bound; 300 zero-miss trials report 0.995% | 299 trials: class unqualified; one miss: class blocked; unkilled mutants not counted as trials |
| 036 | unchanged identities keep the record | each listed identity changed alone: qualification invalid, runner `full` |
| 037 | sampled FULL run compares against the retained plan's base, head and excluded set | zero or unset rate: every runner `full`; seeded excluded failing test confirmed by rerun: class disabled, plan `full` until requalified |
| 038 | local plan equals CI plan for the same inputs | local narrower than CI: test fails |
| 039 | soundness fix updates its baseline with a build-log entry | baseline left in conflict: check fails |
| 040, 046 | step with an admitted record and nonzero rate narrows on CI Linux | step without one: plan unchanged from shadow; after enforcement, unqualified runner: `full`, no static narrowing |
| 041 | repository-shipped plugin pinned and qualified narrows | unqualified repository plugin: runner `full` |
| 043 | must-run test and declared-owner unit with otherwise excludable records: both run, owner unit whole, plan names each mandate | filter unable to run the owner unit whole: `superset` or `full` |
| 044 | listed non-semantic name varied: record still fresh | unlisted feature flag changed: environment digest changes, record stale, test selected; execution configuration outside qualification: runner `full` |
| 045 | `untraced`-qualified runner narrows a body edit | data, add or delete change: runner `full`; unexecuted-body byte mutant read as data by a test: miss blocks the model |
| 048 | Ruby runner qualified on the Rails corpus narrows | unqualified Ruby runner: `full` |
| 049 | sibling read recorded with repository id and OID; sibling diff touching it selects the test; probe-only fixture: test that only looked up a missing sibling path is selected when a pin change adds it | undeclared outside read: `reads` `incomplete`, test selected; sibling pin change: every test naming that repository in any path-bearing component selected |
| 050 | declared command inventoried, traced, narrowed on file components; adding a path the command probed and found missing selects it | undeclared command: runner `full`; `executed` never used; absent `executed`/`regions`/`loaded` alone does not select |
| 051 | path query lists reaching tests with component, digest and commit; `--explain` prints the rule chain | no fresh record: `UNKNOWN` with reason; repository and trace state unchanged |
| 052 | `e2e-safe` plan equals the RSP plan for Playwright on the same inputs | second inventory disagreeing: refused, runner `full` |
| 053 | test inventoried on each CI target narrows per target | test absent from a target's inventory: `full` there |
| 054 | unmodelled codegen read attributed to every test of the build; generator reads F as text while T has coverage of F: editing an unexecuted body of F selects T by rule (d) | dropped build-phase read: check fails |
| 055 | two agreeing collections decide | disagreeing collections: union, `nondeterministic` |
| 056 | collection and inventory times retained | still-collecting record: test selected |
| 057 | reproduced sampled failure is a miss | non-reproducing failure: flake, class stays enabled |
| 058 | keyed digest stable under one key | key change: every environment digest stale |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| RSP-V0-001 | `internal/langplugin` | (PLANNED) manifest validation refusing an undeclared runner piece |
| RSP-V0-002..004 | runner plugin interface, `internal/testrunner` | (PLANNED) inventory golden, divergence and missing-result fixtures |
| RSP-V0-005..010 | evidence record, `internal/linecov` index | (PLANNED) record round-trip, write-kind, attribution and freshness fixtures |
| RSP-V0-011..017 | selector package | (PLANNED) per-rule golden plans, union counterexample, ratchet at 0, shadow replay |
| RSP-V0-018, 019 | E2E runner plugin, `internal/browsercov` | (PLANNED) Playwright fixture with a coverage-built server |
| RSP-V0-020..024 | tracing service | (PLANNED) Linux syscall fixtures, parity run |
| RSP-V0-025..032, 047 | runner protocol host | (PLANNED) conformance kit and schemas over both forms; failure fixtures |
| RSP-V0-033..037 | qualification harness | (PLANNED) per-runner qualification records; sampled-run fixtures |
| RSP-V0-038..041, 046 | CI selection, project config | (PLANNED) CI and local parity replay; rollout enforcement fixtures |
| RSP-V0-042..045 | admission, mandate, environment and model checks | (PLANNED) forged-provenance, mandate, environment and `untraced` fixtures |
| RSP-V0-048..050, 053, 054 | Ruby plugin, multi-repository tracing, `script` runner, per-target inventory, build-phase attribution | (PLANNED) Rails-corpus qualification; sibling-read, pin-change and undeclared-outside-read fixtures; per-target inventory fixtures |
| RSP-V0-051, 052, 055..058 | `corvint test-evidence`, AFU profile consumption, record determinism, cost, flake handling, keyed env digest | (PLANNED) read-command goldens and no-write check; nondeterministic-record and flake-not-miss fixtures |

## Rollout, rollback and compatibility

Rollout is `RSP-V0-040`; every step before its qualification is shadow-only and changes no plan
(`RSP-V0-046`).

Rollback, per runner: delete or supersede its qualification record, or remove its entry from
`.corvint/runner-plugins.json`; either returns it to FULL at once. Rollback of the whole capability
is a revert of each slice, which restores the plans in force before this spec; records are derived
state, so deleting the index extension needs no migration. The protocol is versioned
(`RSP-V0-032`), and removing a plugin never breaks the core's frozen surface (`CMS-V0-003`).

Maintenance: adding a runner adds its three pieces or their absence reasons in the same commit
(`RSP-V0-001`); a selector rule change requalifies every runner (`RSP-V0-036`).

## Open questions and promotion

- The value of the default-branch FULL sampling rate (nonzero is required, `RSP-V0-037`) and its CI
  cost bound (decision 0592 §4 keeps them open); whether the orchestrator-made floor of 300 trials
  per class should change.
- The CI cost of forcing unqualified runners FULL once enforcement begins (`RSP-V0-046`), measured at
  rollout step 3.
- The macOS and Windows tracing mechanisms.
- The plugin-store install command and its name.
- Promotion to `experimental` needs slice (1) landed; to `implemented`, every in-scope runner either
  qualified or recorded FULL with evidence. Kill criterion: if the Go qualification cannot reach a
  zero-miss record on Corvint and one Beamfall repository, narrowing stays off and this spec records
  `failed` for that runner.
