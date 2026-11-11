# Runner Selection Plugins V0

Owner: Russell Lewis
Date: 2026-11-07
Intent status: accepted (decision 0592: owner 2026-11-07 for its §1-§2; §3-§7 orchestrator-made under the 2026-11-02 delegation, owner may override)
Delivery status: experimental
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
- Status: accepted (decision 0592: owner 2026-11-07 for its §1-§2; §3-§7 orchestrator-made under the 2026-11-02 delegation, owner may override) / experimental
- Exists: `internal/testevidence` record components, attribution, freshness, collection union and keyed environment digest; `internal/linecov` index extension and legacy adapter. Also per-runner piece declarations in `internal/langplugin`, the compiled-in SPI in `internal/testrunner/registry`, and inventory/run-set reconciliation in `internal/testrunner`; the Ruby runners `rspec`, `minitest`, `test-unit` and `rails-test` declare Inventory and TestEvidence present (qualification absent, `unqualified`), every other runner declares its selection pieces absent, and all keep their existing execution path. The language-neutral shadow selector in `internal/runnerselection` consumes these types; it is read only through the opt-in `corvint test-evidence --explain` reader and the `test-evidence` MCP tool profile (RSP-V0-051). The `affected` receipt carries no shadow member (AFP-V0-009 closes its advice members; 2026-11-08 decision, orchestrator-made under the 2026-11-02 delegation). The `internal/runnerpolicy` local/CI policy, miss detection and requalification have no production consumer yet. Rule goldens, ancestor-listing regressions and legacy-index replay fixtures are written; execution, frozen-merge replay and reference-host latency remain NOT_RUN. Also the isolated `internal/proctrace` Linux/amd64 tracing service and fixtures, and the Darwin backend reporting trace components absent with a mechanism decision (no runner integration or macOS tracing qualification yet). Also the internal `script` command plugin factory in `internal/testrunner/registry`, with tracked declaration inventory, core `internal/proctrace` execution and file-component records; native coverage stays absent (`script-kind`) and qualification is absent.
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
  registry-and-lock resolution, no `PATH`, `ACC-V0-002`) was the only external-executable protocol.
- `internal/analyzerexec` contained processes with `sandbox-exec` on macOS; Linux containment was
  unsupported there. Decision 0594 (2026-11-09) removed both host packages; no component traces a
  test's file or process activity.

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
  Vitest project-qualified inventories are `config-unsupported` while the admitted JSON terminal
  reporter cannot identify projects; an empty project is never substituted for a named project.
  JavaScript discovery rejects comma-bearing paths as unsafe selectors, consistently with execution.
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
  because the bytes read can still depend on the original content. Successful opens also retain
  an existence dependency, including write-only opens with no later read, unless tree-owned
  creation is proved. Unknown open forms cannot prove creation. Paths outside the repository
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
  - (f) P is deleted or renamed away: any ancestor directory of its base-side path, from its
    parent through the repository root, is in `listings`, or (c) or (d) holds. Include every
    ancestor conservatively, even if other descendants remain at head: deleting the last tracked
    descendant can remove a directory entry from its parent's listing without a directory diff
    entry. A rename applies this rule to its source and (e) to its destination, including when
    represented as a deletion plus an addition. A listing non-match is certified only for the
    closed set of fully decoded, normalized repository-relative component paths with a known
    repository identity and a complete ancestor chain through that repository's root. Any other
    path form, unresolved identity or incomplete ancestry selects T; it never proves exclusion;
  - (g) a mode or link-target change is a content change of P. A listing of P's parent
    also selects T: directory enumeration can observe entry types. Every mode/link transition
    is retained; a listing non-match uses only the closed decoded path/change forms of (f).
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
  Selection units may be whole test files: use a version-documented per-test list (Vitest `list --json`, Playwright `test --list --reporter=json`, Jest `--collectTests --json` when documented for the detected version), otherwise use runner-discovered files (Jest `--listTests`, Node explicit files or its detected-major default patterns over the tracked tree), executing each selected file whole; undetectable versions, unrecognised configurations/unsupported custom globs, and malformed list output make the runner FULL with `inventory-unavailable`, `config-unsupported`, and `list-output-malformed`, respectively (Round 2, orchestrator-made under delegation).
- `RSP-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Server
  coverage is attributed per test only through a per-test reset and dump the qualification covers,
  or by running tests serially against a fresh server; otherwise it is attributed to every test that
  shared the server (`RSP-V0-009`). Device and simulator processes (Espresso, XCUITest) are
  outside the trace, so their tests' trace components are `absent` and `RSP-V0-013` selects them,
  unless the runner is qualified under the `untraced` model (`RSP-V0-045`).
  Until its attributed dump is joined, each other sharer retains incomplete server evidence
  (`browser-server-dump-pending` for otherwise complete browser coverage). Joining all dumps of
  one server collection may complete those records only when browser and server components are
  themselves complete; an observed dump for another sharer never proves completion.

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
  The qualification declaration of `RSP-V0-033` and the runner's configuration paths (the files whose
  change alters the runner's behavior) are not protocol output: they come from a tracked declaration
  file `.corvint/runner-plugins/<plugin>.declaration.json` beside the pin. The pin carries that
  file's SHA-256, so it is digest-bound, and the core validates it strictly (`RSP-V0-047` rules).
  The plugin never supplies it at runtime, and `hello` does not return it. An absent, mismatched or
  invalid declaration makes the runner FULL `unqualified`, and a change to any declared
  configuration path, or any change while the declaration is invalid, touches every test of that
  runner. This is orchestrator-made under the owner's delegated 1.0 choices (PR #143 review).
  The discovery file and the operator store's equivalent are one strict JSON object with every
  member required, no unknown or duplicate members, unique plugin names matching
  `[a-z0-9][a-z0-9._-]*`, and no runner claimed by two plugins. `sha256` is exactly 64 lowercase
  hexadecimal characters. Canonical output sorts plugins by name. The `executable` member is a
  repository-relative or absolute source path in an operator installation manifest; repository
  discovery never executes that source path. Execution still resolves only an installed store or
  release entry by digest, outside repositories, as above. Unsupported schema or protocol, a
  missing executable or a digest mismatch retains the plugin's runner claims as FULL with a named
  reason; malformed discovery cannot establish a narrower runner set.
  This discovery shape is orchestrator-made under delegation (owner 2026-11-02, T10 round 2):

  ```json
  {"schema":"corvint-runner-plugins/1","plugins":[{"name":"reference","protocol":"corvint-runner/1","executable":"./corvint-runner-reference","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","runners":["reference-go-test"]}]}
  ```
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
  The plugin also supplies that runner's `command`, `inventory` and `parse`, so FULL for an
  external-plugin runner needs the plugin to launch. A required runner adapter that is unavailable,
  cannot launch or is unpinned (including after its pin is removed) makes the run FAIL with a named
  reason such as `runner-adapter-unavailable`; it never passes and never silently skips tests. FULL
  without the plugin exists only where core already has a pre-plugin execution path for that runner
  (rollback below); otherwise the failure stands. Orchestrator-made under the owner's delegated 1.0
  choices (PR #143 review).
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
- `RSP-V0-064`: (experimental under the standing lane auto-accept (2026-11-11); V1-4824) The Go,
  JavaScript and Ruby test providers (`corvint-go-test-provider`, `corvint-js-test-provider`,
  `corvint-ruby-test-provider`) keep their current process boundary: each stays its own executable,
  and the core keeps linking only what it links today (no Go provider package; the JavaScript and
  Ruby document types and decoders its readers use). A move of provider code out of the core
  process MUST be driven by a measurement retained as a `docs/build-log` entry that records, per
  provider, the release-built (`CGO_ENABLED=0 go build -trimpath`) size of each binary, the bytes
  of the core binary attributable to that provider's packages, and the p50 and p95 of at least 20
  timed calls of one fixed call both in process and as an exec of the built provider, with method
  and host. A provider moves only when its attributable bytes are at least 5% of the core binary
  and the move adds no more than its in-process p95 to the p95 of a call; a move then keeps a
  build tag that restores the in-process form, with a test of both forms. The affected-selection
  providers stay in process whatever the numbers (hot path; they never execute repository code).
  The 2026-11-11 measurement (`docs/build-log/2026-11-11-test-provider-process-measurement.md`) is
  no-go for all three. Failure mode: the numbers are host-bound (one Linux amd64 host) and a later
  change to the core's provider imports can cross the threshold unnoticed until remeasured.
  Rollback: none needed while the decision is no-go (no code changed); delete this requirement and
  the build-log entry to drop the threshold.

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
  `qualification` piece declares, in the pinned declaration file of `RSP-V0-028` and never at
  runtime, the runner's corpus (a pinned repository name and commit; the
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
  class. File-class populations must use the core operators and match their closed edit forms:
  one-byte flips, strict prefix truncation, absent-to-present additions, present-to-absent
  deletion, or a deletion plus an identical-content addition for rename. A declared mutation
  piece or operator label cannot substitute for the corresponding edit; unknown forms refuse.
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
  hand-set flag. An invalidated record is restored only by full requalification or by the
  carry-forward of `RSP-V0-061` or `RSP-V0-062`; qualification is triggered by an identity change,
  never by a schedule.
- `RSP-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592) Sampled
  FULL runs on the default branch extend the `LPCV-V0-957` miss detector to every class. No runner's
  narrowing is enabled until the trusted configuration sets a nonzero sampling rate; a zero or unset
  rate keeps every runner FULL. Each sampled run compares against the plan that actually ran, as
  retained: its exact base and head commits and its original excluded set, never a recomputed one.
  An excluded test whose FULL outcome at head differs from its outcome at that base is a candidate
  miss; one rerun of it at the same base and head confirms it. A confirmed miss disables that
  runner's class until repaired and requalified, and is retained with the plan. The rate's value and
  its CI cost bound are open numbers in decision 0592.
- `RSP-V0-059`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592; owner
  ask 2026-11-07: prove each runner in isolation, then trust it until its contract changes) For a
  compiled-in runner, the plugin digest of `RSP-V0-036` is a SHA-256 over the in-process SPI version
  and the non-test source files of the packages that implement that runner's pieces plus every
  package in their transitive import closure inside this module, as `go list -deps` reports it under
  the GOOS, GOARCH and build tags the qualification ran with, plus the `go.sum` lines of the
  closure's external modules. An edit outside that closure leaves the digest and the qualification
  unchanged; an edit inside it starts the runner unqualified. The selector version of `RSP-V0-036`
  is the same digest over `internal/runnerselection` and its closure. A closure that cannot be
  resolved (load error, build-constraint set other than the qualified one) yields no digest, and the
  runner is FULL with reason `plugin-digest-unresolved`.
- `RSP-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592; owner
  ask 2026-11-07) Every runner has an isolated contract suite: the conformance kit of `RSP-V0-047`
  run against that runner alone (compiled-in runners through an in-process adapter) over checked-in
  fixtures, needing no live toolchain: inventory determinism, run-set reconciliation, record schema
  and every component state, and each `RSP-V0-031` failure. Its result is keyed by the runner's
  plugin digest and the kit version; a passing result for the current key is reused, and the suite
  reruns only when that key changes. The harness of `RSP-V0-034` refuses to qualify a runner whose
  contract suite has not passed for the same digest. A kit change reruns every runner's suite but
  requalifies no runner whose `RSP-V0-036` identities are unchanged.
- `RSP-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592; owner
  ask 2026-11-07: patch releases and small changes must not force full requalification) When the
  only identities that changed are the runner version, the plugin digest or the tracing backend
  version, the harness may carry the qualification forward by an equivalence check instead of new
  mutant trials. The check runs the corpus once at its pinned commit under the new identities and
  passes only if: the contract suite of `RSP-V0-060` passes; the inventory is byte-equal to the
  qualification's baseline inventory; for every corpus test, the record components the selector
  consults equal those of the baseline records; and the replay of `RSP-V0-034` (a) leaves no test
  unselected. Comparison is always against the baseline the trials were measured on, never against
  an earlier carry-forward, so equivalences do not drift. Version numbers carry no trust: a patch
  and a major release take the same check. A passing check writes a carry-forward record naming the
  baseline record and the new identities, admitted under `RSP-V0-042`; any difference names the
  first differing test and component and starts full requalification. Until the check passes the
  runner is FULL with reason `requalifying`.
- `RSP-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11); decision 0592; owner
  ask 2026-11-07) A qualification record retains, per trial, the mutant, the tests that failed on it
  under FULL and the baseline records. When the only identity that changed is the selector version,
  the harness re-scores every retained trial offline with the new selector, running no tests; zero
  misses carries the qualification forward under the same bound, and any miss blocks the class as in
  `RSP-V0-034`. A record format change or an execution configuration change (`RSP-V0-044`) always
  needs full requalification.

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
  reason `script-kind` and select nothing on their own. The complete, empty three-component
  marker establishes file-only capability for selection and collection readiness; a partial or
  unknown marker retains native prerequisites. Agreeing collections preserve this capability. It
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

  The expected identities are the planned-run context resolved read-only on the current host
  by the same core functions used by the selector and harness: build target GOOS/GOARCH,
  compiled-in tracing backend and version, the environment digest from the declared execution
  configuration, and each declared runner executable's independently resolved SHA-256 bytes.
  Resolution never executes a runner, including `--version`; version strings carry no trust
  and a digest match suffices. Record-provided identities never supply their own expectation.
  Inventory, qualification and admission come only from the index's stored inventory artifact
  and qualification and carry-forward records already admitted for that commit; this command
  discovers and collects nothing. An executable that cannot be resolved or hashed yields
  UNKNOWN `run-context-unresolved` for its records; missing stored inventory yields
  `inventory-unavailable`; absent admitted qualification explains the runner as FULL
  `unqualified`; legacy records without identities yield UNKNOWN `identity-missing`.
  `--explain TEST` against a diff reports the selector decision from these inputs, or UNKNOWN
  with its reason. This read-context binding is orchestrator-made under delegation (owner
  2026-11-02; restart decision 2026-11-07).

  Release read-context decision (2026-11-08, orchestrator-made under delegation): this release
  reads only existing T1 records in `.corvint/test-evidence` and an explicitly named existing
  `internal/linecov` index extension, using `internal/testevidence/freshness.go` and the existing
  `internal/runnerselection` inputs; it introduces no persisted inventory or admitted-qualification
  storage shape. Missing inventory answers records UNKNOWN `inventory-unavailable`, absent admitted
  qualification explains runners FULL `unqualified`, no fresh record answers UNKNOWN
  `no-fresh-record`, stale records answer UNKNOWN `stale-record` with the freshness reason, and an
  unresolved independent run context answers UNKNOWN `run-context-unresolved`. Multiple reasons
  are retained together. This abstaining read surface is complete delivery for this release, not
  qualification or a production exclusion claim. `test-evidence PATH [--lines START:END]`,
  `--test TEST`, and `--explain TEST --base COMMIT` use `--revision COMMIT` (default HEAD), optional
  repository-relative `--index FILE`, and `--format json|text` (default JSON). The new
  `corvint-test-evidence-query/1` projection retains the existing record and selector types,
  canonical record SHA-256 digests and commits; path/range matches include whole-file dependencies
  as well as intersecting executed lines. Reads are bounded to 512 directory entries, 32 MiB
  aggregate artifacts and 256 KiB compact result; malformed, unsafe, unsupported or capped input
  retains named UNKNOWN, never a certified empty result. `corvint.test_evidence` is opt-in through
  `corvint-mcp --tool-profile test-evidence`; existing profiles keep their tool sets.
  `flows test-evidence --flows DIR` joins the same query to the existing AFU source/test lookup
  in `application-flow-test-evidence/1`, refuses a revision differing from its HEAD link evaluation,
  and never upgrades link authority. The self-development feature route points to these reads.
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
  test and never disables a class. Requalification clears a retained miss only when the newer
  admitted qualification matches its runner, repository, OS, architecture, configuration digest
  (`ConfigDigest`) and environment scope (`EnvironmentDigest`) exactly. The retained plan pins
  these fields; a missing or unknown field on either side keeps the miss and its narrowing block.
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
| A provider's in-core bytes grow past the move threshold unmeasured | remeasurement only (`RSP-V0-064`) | providers stay in their current form until a retained measurement says go |
| Test new on head, or no fresh record | `RSP-V0-004`, `-010` | test selected |
| Native filter cannot express run set | `RSP-V0-004`, `-043` | reported superset or FULL |
| Test executes none of P's changed lines but reads P | rule (d), `RSP-V0-012` | test selected |
| Delete or rename away a directory's last tracked descendant | ancestor listings, rule (f), `RSP-V0-012` | test selected even without a directory diff entry; unknown ancestry selects |
| Read after append, overwrite or truncate | `RSP-V0-007` | read edge kept |
| Component absent or incomplete | `RSP-V0-013` | test selected, `component-missing:<name>` |
| Isolated server dump missing for a sharer | `RSP-V0-019` | executed incomplete, `browser-server-dump-pending`; test selected |
| Vitest inventory names a project absent from terminal JSON | `RSP-V0-002`, `-003` | inventory refused, `config-unsupported`; runner FULL |
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
| Compiled-in plugin closure unresolved | `RSP-V0-059` | runner FULL, `plugin-digest-unresolved` |
| Contract suite failing or absent for the current digest | `RSP-V0-060` | qualification refused, runner FULL |
| Equivalence check differs or not yet run | `RSP-V0-061` | runner FULL `requalifying`, full requalification started |
| Offline re-score finds a miss | `RSP-V0-062` | class blocked |
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
| 012, 014, 034 | deletion/rename-away listing fixtures below select on all ancestor depths | unknown path form cannot certify a listing non-match; test selected |
| 013 | all components present: rule outcome stands | each component `absent` and `incomplete` in turn: `component-missing:<name>`; `external` observed: selected |
| 014, 034 | change in a qualified class narrows | change in an unqualified class: runner `full` |
| 015, 016, 017 | plan names mode, reasons, rules and invalidation per test | each `RSP-V0-015` cause seeded alone: runner `full`, reason names that cause |
| 018, 019 | coverage-built server inside the tree joins records | server outside the tree: tests selected; device runner without `untraced` qualification: tests selected |
| 020, 021, 022 | Linux fixture per traced call fills its component | setuid child, unknown path syscall, capped run: components `incomplete` |
| 023 | OS with a qualified backend narrows on its classes | OS with no backend: every test selected unless `untraced`-qualified there |
| 024 | traced and untraced outcomes equal | outcome differs: tracing disabled, components `absent` |
| 025, 026, 027, 047 | conformance kit passes identically on a compiled-in runner and its out-of-process twin; every frame validates against the schemas; `hello` agrees one offered major | unknown member, duplicate member, wrong type, invalid UTF-8, schema violation, unoffered major, `error` response: each ends the session, runner `full` |
| 028 | pinned digest resolves from the store; pinned declaration file supplies qualification declaration and configuration paths | absent, mismatched or invalid declaration: runner `full` `unqualified`, its config change touches every test; plugin `hello` carrying a declaration ignored; digest mismatch: refused, runner `full`; `PATH`-shadowing executable never consulted; candidate edit of the pin file: named runners `full` |
| 029, 031 | session within limits ends cleanly | crash, timeout, oversize, repository mutation, undeclared runner: each `full`, failure named, no record |
| 030, 042 | admitted qualification and base index from a protected-ref run narrow | forged qualification record, fabricated or candidate-produced coverage record, candidate-modified corpus pin fixture: each not admitted, runner `full`, reason named |
| 031 | external-plugin runner whose plugin is missing, unlaunchable or unpinned: run outcome checked, not only plan mode | run fails `runner-adapter-unavailable`, never passes or skips; runner with a core pre-plugin path runs that path |
| 032 | additive feature named in `hello` is used | unnamed feature: never sent |
| 033, 035 | qualification record holds seed, population, trials, misses and bound; 300 zero-miss trials report 0.995% | 299 trials: class unqualified; one miss: class blocked; unkilled mutants not counted as trials |
| 059, 060 | edit outside a runner's closure: digest, contract-suite result and qualification reused | edit to one file in the runner package and one in a dependency: digest changes, runner `full` until requalified; unresolved closure: `plugin-digest-unresolved`; failing contract suite: qualification refused |
| 061, 062 | runner patch release with equal inventory and records: carried forward without trials, carry-forward record names the baseline; selector change re-scored offline with zero misses: carried forward | one consulted component differs: first difference named, full requalification; second carry-forward compared against the baseline, not the first; re-score miss: class blocked; record-format change: never carried |
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

### Deletion and rename-away listing fixtures (RSP-V0-012)

These are required acceptance fixtures, not observed selector results. Use fresh, complete,
admitted records, qualified `delete-rename` (and `add` for renames), no other selecting rule,
and no mandate. T enumerates `fixtures` and asserts that the entry `group` exists; it never
opens `group` or its descendants. Its only dependency is `listings=["fixtures"]`. Base tracks
`fixtures/keep.txt` and `fixtures/group/only.txt`. Build head checkouts from tracked content,
so empty directories left in a developer's worktree cannot mask the change.

| Mutation / input | Required plan and outcome |
|---|---|
| Delete `fixtures/group/only.txt`; diff contains only that file deletion | T selected by (f); FULL execution fails T because `group` disappeared |
| Rename that file to `elsewhere/only.txt`, encoded either as a rename or delete plus add | T selected by (f) on the old path; FULL execution fails T |
| Instead put the sole file at `fixtures/group/deeper/only.txt`, then delete or rename it away | T selected by (f) through every ancestor; FULL execution fails T |
| Keep an additional tracked `fixtures/group/stays.txt` while deleting `only.txt` | T still selected by (f); FULL execution passes T; conservative overselection is permitted |
| Delete only `unrelated/only.txt`, with its complete normalized ancestor chain known | Rule (f) does not select T; with the stated preconditions T can be excluded |
| Supply an opaque path token whose repository-relative components cannot be decoded, even if its text does not start with `fixtures/` | T selected because ancestry is unproven; no listing non-match may be certified |

The unknown-form fixture exercises the closed allowlist's default, not a list of particular bad
spellings. The source path is checked before any rename destination; destination non-intersection
never cancels source selection. Qualification must cover these fixtures before the changed rule
can narrow (`RSP-V0-034`, `RSP-V0-036`).

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| RSP-V0-001 | `internal/langplugin`, `internal/testrunner/runner_pieces.go` | `TestRSPV0001PresentAndAbsent`, `TestRSPV0001RefusesInvalidPieces`, `TestRSPV0001EveryExistingRunnerDeclared` (Ruby registered, other declarations absent) |
| RSP-V0-002 | `internal/testrunner/runner_inventory.go`, `internal/testrunner/registry/runner_plugin.go` | `TestRSPV0002CanonicalInventory`, `TestRSPV0002InvalidInventoryFull`, `TestRSPV0002PluginInventoryFailures`; Go Ginkgo remains FULL with `go-ginkgo-inventory-unsupported` until native inventory exists: `TestRSPV0002GoGinkgoInventoryUnsupported`; shell glob sources refuse with `shell-source-glob-unresolvable`: `TestRSPV0002ShellGlobSourceUnresolvable` (repair tests written, NOT_RUN: build-only lane); `TestRSPV0002RustIntegrationTestsFailClosed`, `TestRSPV0002RustTargetAllowlist` (cargo-test admits exactly one local library and custom-build targets; binary, integration, unknown or ambiguous target forms refuse inventory and commands); `TestRSPV0005DotnetMTPLocationOnlyMissingCollection` (MTP discovery requires a validated immutable source binding for every identity) |
| RSP-V0-003 | `internal/testrunner/runner_inventory.go`, `internal/testrunner/registry/runner_plugin.go` | `TestRSPV0003TerminalResults`, `TestRSPV0003Divergence`, `TestRSPV0003FullRetry`, `TestRSPV0003RetryPreservesFailure`, `TestRSPV0003RejectedReportFailureStaysSticky` (a failed row returned with a runner or report error, or in a spelling reconciliation refuses, stays sticky across a passing FULL retry), `TestRSPV0003RetainsCorePlanAndResults`, `TestRSPV0003SessionDivergenceRetry`, `TestRSPV0003SessionExtraResult`, `TestRSPV0003SessionExecutionErrorRetainsResults`, `TestRSPV0003ShellDivergence`, `TestRSPV0003ShellRetryPreservesPartialFailure`, `TestRSPV0003ShellPartialResultsBoundary`, `internal/testrunner/golang`: `TestRSPV0003GoDamagedReportRetainsFailureAcrossRetry` (decoded results survive truncated, malformed, capped and unknown-action suffixes); `TestRSPV0003GoSkipRequiresNonLifecycleReason` excludes RUN/PAUSE/CONT/NAME/SKIP lifecycle output from skip explanations (written, NOT_RUN: build-only lane); `TestRSPV0003JVMDamagedSuffixRetainsFailureAcrossRetry` (complete validated JVM suites retain failures alongside damaged or unknown envelope suffix errors, including across a passing FULL retry); `internal/jslinecov/runner_results.go`: `TestRSPV0003NodeTruncatedFailureKept` (a validated Node `test:fail` without its file summary stays a failed file result across a passing FULL retry, counted once when the summary arrives); `internal/testrunner/registry`: `TestRSPV0003RubyRejectedReportKeepsFailures` (failed examples of a rejected Ruby report, from an RSpec outside-example error or a profile-reporter count mismatch, stay failed across a passing FULL retry); `TestRSPV0003JVMCountMismatchRetainsFailureAcrossRetry` (failed testcases of a JVM suite rejected for a count mismatch stay failed across a passing FULL retry; its passes stay unobserved); `TestRSPV0003UnittestTearDownClassKeepsFailure` (an unproven unittest callback such as a failing `tearDownClass` withholds passes but keeps the certified failure across a passing FULL retry); `TestRSPV0003DotnetVSTestCountMismatchKeepsFailure` (validated failed rows of a VSTest TRX report rejected for a wrong total count stay failed across a passing FULL retry; its passes stay unobserved); `TestRSPV0003RustNonterminalRowKeepsLaterFailure` (an unsupported nextest row sorted before a failed row no longer stops parsing; the failure is returned with the nonterminal error); `internal/browsercov`: `TestRSPV0003PlaywrightFailureSurvivesMalformedLaterResult` (an unfinished earlier Playwright row, including an interrupted attempt under an aggregate skip, keeps a later failed row and fails the report); `internal/runnerprotocol`: `TestRSPV0003ReferenceFailureSurvivesIncompleteReport` (a reference Go report rejected as incomplete, as any report with a failed test is without an exit code, or ending in a truncated JSON frame, returns its failed rows with the error; passes stay unobserved) |
| RSP-V0-004 | `internal/testrunner/runner_inventory.go`, `internal/testrunner/registry/runner_plugin.go` | `TestRSPV0004ExclusionRunSet`, `TestRSPV0004InvalidSelectionFull`, `TestRSPV0004SupersetNeverSubset`, `TestRSPV0004SessionSuperset` |
| RSP-V0-005 | `internal/testevidence/capture.go`, native/core evidence assembly | `TestRSPV0005Assemble`; legacy adapter: `TestRSPV0008LegacyIndexes`; `TestRSPV0005DotnetAncestorConfigurationLoaded`, `TestRSPV0005DotnetMissingCollectionRecord`, `TestRSPV0005DotnetBodyMutationLineEndings`, `TestRSPV0005DotnetVSTestAmbiguousAttachments`, `TestRSPV0005RubyRSpecDotSlashEvidence`, `TestRSPV0005PythonCoverageIgnoresProjectConfig`, `TestRSPV0005SwiftLLVMLineEvidence`, `TestRSPV0005RustAncestorCargoConfigurationLoaded`, `TestRSPV0005JVMInheritedJavaToolOptions` (build-only repair; execution NOT_RUN) |
| RSP-V0-006 | `internal/testevidence/record.go`, exported components and bounded record wire | `TestRSPV0006RoundTripStates`, `TestRSPV0006RejectMalformed`, `TestRSPV0006RejectInvalidText` |
| RSP-V0-007 | `internal/testevidence/capture.go`, path capture and write proof | `TestRSPV0007WriteProof`, `TestRSPV0007Paths`, `TestRSPV0007OutsideLinks` (repository links to outside targets stay incomplete), `TestRSPV0007OpenExistenceProvenance`, `TestRSPV0007WriteOnlyOpenExistence`, `TestRSPV0007WriteProofRetainsLinkRetargeting` (content proofs retain lexical dependencies), `TestRSPV0007UnknownWriteProofRetainsRead` (unknown write forms retain reads); `internal/proctrace`: `TestRSPV0007RenameOverSymlink`, `TestRSPV0007RenameOverRegularDestination` (a created source renamed over an existing symlink or regular file keeps the destination as a read dependency); `internal/testrunner/golang`: `TestRSPV0007GoNestedModuleRepositoryPaths`, `TestRSPV0007GoUnknownRepositoryBoundaryRefusesEvidence` bind inventory, executed, regions and loaded paths to the enclosing repository (written, NOT_RUN: build-only lane) |
| RSP-V0-008 | `internal/linecov/evidence.go`, immutable index extension, legacy adapter and bounded encoding | `TestRSPV0008LegacyIndexes`, `TestRSPV0008NativeZeroWidth` (native zero-width blocks retain the file with executed incomplete), `TestRSPV0008Extension`, `TestRSPV0008IndexSize`; reference-host selection latency NOT_RUN (selector is a later slice) |
| RSP-V0-009 | `internal/testevidence/capture.go`, process-tree and marker attribution | `TestRSPV0009Attribution`, `TestRSPV0009ModeledBuildRetainsTraceAndUnion` (modeled build trace retained, including unknown states) |
| RSP-V0-010 | `internal/testevidence/freshness.go`, `internal/testevidence/union.go` | `TestRSPV0010Freshness`, `TestRSPV0010UnionRanges` |
| RSP-V0-011 | `internal/runnerselection`; baseline 0 in `internal/langplugin/langplugin_test.go` | `TestRSPV0011PureAndDetached`, `TestRSPV0011UnknownInputFull`, `TestRSPV0011ZeroLiteralRatchet`, `TestRSPV0011RatchetRejectsRunnerLiteral`; execution NOT_RUN (build-only lane) |
| RSP-V0-012 | `internal/runnerselection/select.go` | `TestRSPV0012RuleGoldens`, `TestRSPV0012NonintersectionAndUnion`, `TestRSPV0012InsertionAndUncoveredLines`, `TestRSPV0012AncestorsAndRenameRepresentations`, `TestRSPV0012LastTrackedDescendantRegression`, `TestRSPV0012AddLookupAndLink`, `TestLPCVV0952PartialCoverageLines`, `TestRSPV0012BodyBoundaryReplacements`, `TestRSPV0012EntryTypeListing`; whole selector package PASS in the self-review repair |
| RSP-V0-013 | `internal/runnerselection/select.go` | `TestRSPV0013PresentComponents`, `TestRSPV0013MissingComponentGoldens`, `TestRSPV0013ExternalAlwaysSelects`; execution NOT_RUN |
| RSP-V0-014 | `internal/runnerselection/select.go` | `TestRSPV0014QualifiedClasses`, `TestRSPV0014UnqualifiedClassFull`; execution NOT_RUN |
| RSP-V0-015 | `internal/runnerselection/select.go` | `TestRSPV0015QualifiedRunner`, `TestRSPV0015FullReasons`, `TestRSPV0015MissingStaleMalformedRecord`; execution NOT_RUN |
| RSP-V0-016 | `internal/runnerselection/types.go`; JSON projection through `internal/testevidencedoc/query.go` (`corvint test-evidence --explain`, RSP-V0-051) | `TestRSPV0016AuditAndCounts`, `TestRSPV0016UnknownInventoryNotZeroTests`; the `affected` receipt carries no shadow (`TestRSPV0040AffectedReceiptCarriesNoShadow`); execution NOT_RUN |
| RSP-V0-017 | `internal/runnerselection/select.go` | `TestRSPV0017ShadowClaim`, `TestRSPV0017FullNeverClaimsSoundness`; execution NOT_RUN |
| RSP-V0-018, 019 | `internal/browsercov/runner_inventory.go`, `runner_evidence.go`, `internal/testrunner/registry/javascript_plugin.go` | `TestRSPV0018PlaywrightInventory`, `TestRSPV0018PlaywrightMalformed`, `TestRSPV0018PlaywrightServerInsideTracer`, `TestRSPV0018ExternalAndUninstrumentedServer`, `TestRSPV0019ServerSharedAndResetDump`, `TestRSPV0019SerialFreshAndMissingReset`, `TestRSPV0019MissingSharerAndBlobDrift`; fixtures NOT_RUN (build-only lane); live qualification NOT_RUN |
| RSP-V0-020 | `internal/proctrace.Backend`, platform backends; no command imports | `TestRSPV0020ImportDirection` (`internal/closureguard`), `TestRSPV0020InvalidRequest`, `TestRSPV0020AbsentBackend` |
| RSP-V0-021 | `internal/proctrace` collector and plain component results | `TestRSPV0021Read`, `TestRSPV0021OutsideExecutableCopiedLoader`, `TestRSPV0021CompleteSingleProcess`, `TestRSPV0021Cancellation`, `TestRSPV0006Components`, `TestRSPV0007WriteKinds`, `TestRSPV0007Symlink`, `TestRSPV0007DirectorySymlink`, `TestRSPV0007ChainedSymlink`, `TestRSPV0007DanglingSymlink`, `TestRSPV0007UnknownMissingSuffix`, `TestRSPV0007UnprovenLinkForms`, `TestRSPV0021RenameDestinationParent`, `TestRSPV0007UnsupportedMatching`, `TestRSPV0049Repositories`, `TestRSPV0007OpenProvenance`, `TestRSPV0007StartupLinks`, `TestRSPV0021SharedMemoryPeer`; startup links retain intermediate dependencies, path-only opens never prove creation, shared mappings retain unknown peers; Darwin absence path: `TestRSPV0021DarwinAbsentComponents`, `TestRSPV0021DarwinNoUnobservedExecution` (written, NOT_RUN in build-only lane); `TestRSPV0021OutsideExecutableRepositoryInterpreter` retains implicit interpreter targets and link chains for outside executables |
| RSP-V0-022 | `internal/proctrace` Linux/amd64 ptrace + seccomp-BPF backend | `TestRSPV0022Syscalls`, `TestRSPV0022Descendants`, `TestRSPV0022Incomplete`, `TestRSPV0022RefusedStart`, `TestRSPV0022InvalidConfiguration`, `TestRSPV0022FilterCoverage`, `TestRSPV0022ClosedSyscallForms`, `TestRSPV0022UnknownStartupPaths`, `TestRSPV0022UnknownMappingForms`, `TestRSPV0007OpenProvenance`, `TestRSPV0022OutsideExecutableInterpreter`, `TestRSPV0022UnknownOutsideExecutableFormat`; closed syscall/selector, open-flag, ordinary-startup-path and private-mapping allowlists, unsupported architectures/matching and concurrent path races widen; Darwin has no qualified analogue: `TestRSPV0022DarwinValidCapsRemainAbsent`, `TestRSPV0022DarwinMalformedRequestsRemainAbsent` (written, NOT_RUN in build-only lane); `TestRSPV0022OutsideUnknownExecutableFormatIncomplete` enforces the ELF64 little-endian x86-64 EXEC/DYN allowlist at every executable location |
| RSP-V0-023 | `internal/proctrace/backend_darwin.go`; mechanism decision `docs/build-log/2026-11-08-rsp-macos-tracing.md` | `TestRSPV0023DarwinDeterministicAbsence`, `TestRSPV0023DarwinUnknownContextRemainsAbsent` (written, NOT_RUN in build-only lane); real macOS fixture coverage NOT_RUN; Windows qualification PLANNED |
| RSP-V0-024 | `internal/proctrace.Compare`; caller-retained timing and outcome values | `TestRSPV0024LiveParity`, `TestRSPV0024GoRuntimeParity`, `TestRSPV0024Parity`, `TestRSPV0024Mismatch`, `TestRSPV0024UnobservedParity`; full Corvint-suite parity NOT_RUN (lane forbids whole-repository tests), runner qualification pending |
| RSP-V0-025 | `internal/testrunner/registry/runner_plugin.go` (compiled-in form); `internal/runnerprotocol` SPI adapter, reference twin and shared kit (out-of-process form) | `TestRSPV0025CompiledInSession`, `TestRSPV0025LegacyAndUnknownAbsent`, `TestRSPV0025TwinParity`, `TestRSPV0025UnknownAndLegacy`, `TestRSPV0025CompiledInClaimsRefused` (NOT_RUN: build-only lane) |
| RSP-V0-026 | `internal/runnerprotocol/schema.go`, `serve.go` | `TestRSPV0026StrictPositive`, `TestRSPV0026StrictNegative`, `TestRSPV0026ServerValidation` (NOT_RUN) |
| RSP-V0-027 | `internal/runnerprotocol/adapter.go`, `serve.go` | `TestRSPV0027Methods`, `TestRSPV0027UnknownMethodAndPath`, `TestRSPV0027NulEnvironmentRefused` (NOT_RUN) |
| RSP-V0-028 | `internal/runnerprotocol/discovery.go`, `platform_linux.go`, `snapshot.go` | `TestRSPV0028DiscoveryPositive`, `TestRSPV0028DiscoveryNegative`, `TestRSPV0028DigestAndSymlink`, `TestRSPV0028TrustedBlobReader`, `TestRSPV0028MissingAndMalformedProjectPins`, `TestRSPV0028DocumentedDiscovery` (runtime qualification pending); PARTIAL: the PR #143 digest-pinned declaration file (`.corvint/runner-plugins/<plugin>.declaration.json`, absent/mismatched/invalid → FULL `unqualified`) is not yet implemented |
| RSP-V0-029 | `internal/runnerprotocol/session.go`, `snapshot.go` | `TestRSPV0029LimitsAndDiagnostics`, `TestRSPV0029LimitsNegative`, `TestRSPV0029ShutdownDeadlineAndExistingDirtyFile`, `TestRSPV0029SymlinkedRepositoryMutation`, `TestRSPV0029SymlinkedRepositoryRunDirectory`, `TestRSPV0029HookMutationFailsSession`, `TestRSPV0029GitMetadataMutation`, `TestRSPV0029IgnoredTreeStillCovered`, `TestRSPV0029UnknownGitPointerFailsClosed` (runtime qualification pending) |
| RSP-V0-030 | `internal/runnerprotocol/adapter.go`, core-owned evidence staging | `TestRSPV0030EvidencePublication`, `TestRSPV0030DiscardAndForgedAuthority`, `TestRSPV0030NativeValidation` (NOT_RUN) |
| RSP-V0-031 | `internal/runnerprotocol/session.go`, fail-closed discovery plans | `TestRSPV0031CleanSession`, `TestRSPV0031FailureFixtures`, `TestRSPV0031UndeclaredResponseAndMissingExecutable` (NOT_RUN); PARTIAL: the PR #143 run FAIL `runner-adapter-unavailable` for an unavailable, unlaunchable or unpinned external-plugin runner is not yet implemented |
| RSP-V0-032 | `internal/runnerprotocol/session.go`, negotiated version and feature guard | `TestRSPV0032NegotiatedFeatures`, `TestRSPV0032UnnegotiatedFeature` (NOT_RUN) |
| RSP-V0-047 | runner protocol host | (PLANNED) conformance kit and schemas over both forms; failure fixtures |
| RSP-V0-033..037 | PARTIAL: qualification harness; closed core file-operator validation; exact replay base binding, negative-control selection equality, retained execution costs and trusted cost ceilings | `TestRSPV0033LanguageNeutralOperators`, `TestRSPV0034ReplayAndPerClassFaults`, `TestRSPV0034MislabeledPopulations`, `TestRSPV0034ReplayPlanBaseBinding`, `TestRSPV0034CommandRefusesUnpinnedReplayPlan`, `TestRSPV0034NegativeControlPlan`, `TestRSPV0036QualificationBudgetCeiling`, `TestRSPV0056ReplayRetainsExecutionCost`, `TestRSPV0056AdmissionCostForms`, `TestRSPV0056CommandPreservesExecutionBudget`; live per-runner qualification and sampling remain planned; Go execution digest stays separate from checkout binding in `internal/testrunner/golang/plugin.go`: `TestRSPV0036GoCheckoutDoesNotChangeConfigurationDigest`, `TestRSPV0036GoIdentityChangesInvalidateCapturedInventory`; `TestRSPV0035GoUnparseableMutantsRefuseWholePopulation` accounts for every render failure and refuses a reduced population (written, NOT_RUN: build-only lane); Go qualification remains absent, with plugin digest, selector version and verifier revision unavailable in `Configuration` (RSP-V0-036 BLOCKED); the RSP-V0-033 read of qualification declaration and configuration paths from the digest-pinned declaration file (RSP-V0-028, PR #143) is not delivered, so the row stays PARTIAL |
| RSP-V0-038, 039, 046 | `internal/runnerpolicy/policy.go`, `baseline.go`, `miss.go` (PARTIAL: no production consumer since the 2026-11-08 removal of the `affected` shadow); `docs/build-log/TEMPLATE-rsp-baseline-update.md` | `TestRSPV0038ParityReplay`, `TestRSPV0038LocalNarrowingWidens`, `TestRSPV0039BaselineUpdateNamesReselectedTests`, `TestRSPV0039ConflictingBaselineRefused`, `TestRSPV0046ShadowThenAdmission`, `TestRSPV0046UnqualifiedAndUnknownFull`, `TestRSPV0046OwnerSamplingRate`, `TestRSPV0046UnsetSamplingFull`, `TestRSPV0046GoverningRunnerCoverage` (whole `internal/runnerpolicy` package passed in self-review repair); owner rate UNSET, CI cost NOT_OBSERVED; no production rollout admitted |
| RSP-V0-041 | `internal/runnerprotocol/discovery.go`, `tools/runner-plugin` operator install command | `TestRSPV0041OperatorInstall`, `TestRSPV0041InstallNegative`, `TestRSPV0041StoreRejectsClaimCollision`, `TestRSPV0041InstallCommand`, `TestRSPV0041CommandRefusesMissingInputs` (NOT_RUN; qualification remains core-owned) |
| RSP-V0-040 | `internal/runnerselection`, read through the opt-in RSP-V0-051 reader (step 1); `internal/testrunner/registry/shell_plugin.go`, `shell_trace.go`, `make_plugin.go` (step 4 pieces only) | `TestRSPV0040NoEnforcementSwitch`, `TestRSPV0040UnqualifiedKeepsGoverningPlan`, `TestRSPV0040LegacyIndexShadowReplay`, `TestRSPV0040InvalidLegacyIndexCannotNarrow`, `TestRSPV0040AffectedReceiptCarriesNoShadow` (default, `--full` and compact receipts carry no shadow); execution, frozen-merge replay and reference-host p95 NOT_RUN; step 3 build-only pieces in `internal/testrunner/golang`, `internal/linecov/go_regions.go` and `internal/liveverify/mutate/selection.go`: `TestRSPV0040GoPiecesRemainShadow`, `TestRSPV0040GoMissingQualificationPreventsExclusion`; execution and retained qualification NOT_RUN, Ginkgo FULL with `go-ginkgo-inventory-unsupported` until native inventory exists (`TestRSPV0002GoGinkgoInventoryUnsupported`, NOT_RUN); step 4 pieces: `TestRSPV0040ShellTraceSession`, `TestRSPV0040ShellTraceRefusals`, `TestRSPV0040MakeUnqualified`, `TestRSPV0040MakeUndeclared` (NOT_RUN, build-only lane); later rollout enablement remains PLANNED |
| RSP-V0-042..045 | admission, mandate, environment and model checks | (PLANNED) forged-provenance, mandate, environment and `untraced` fixtures |
| RSP-V0-048 | Bound Ruby selection factory in `internal/testrunner/registry/ruby_plugin.go`, existing dynamic runners and `internal/rubylinecov/selection.go`; global registration in `SelectionPlugin` with matching language manifests | `TestRSPV0048RubyDiscoveryAndCollectionPieces`, `TestRSPV0048RubyUnqualifiedFull`, `TestRSPV0048RubyDialectRegionsAndCoreTrace`, `TestRSPV0048RubyComputedReadsAndMalformedDialectWiden`, `TestRSPV0048RubyCaptureCapsAndDuplicateIdentity`, `TestRSPV0005DecodeSelectionArtifacts`, `TestRSPV0005ArtifactFormsFailClosed`, `TestRSPV0005RubyCommandArtifactsEvidenceLive`, `TestRSPV0003RubyReporterSkipReasonsLive`, `TestRSPV0002RubyConfigurationIgnoresStaging`; repair execution evidence in the 2026-11-08 self-review repair build log; Rails-corpus qualification NOT_RUN; `TestRSPV0048RubyGloballyRegistered` (global discovery/parsing, bound execution/capture, qualification absent; execution NOT_RUN in build-only repair) |
| RSP-V0-050 | `internal/testrunner/registry/script_plugin.go`, `script_trace.go`; existing declaration schema, per-command compiled-in SPI instances, core trace-to-record adapter | `TestRSPV0050DeclaredCommandInventory`, `TestRSPV0050ExistingDeclarationSchema`, `TestRSPV0050UndeclaredCommandFull`, `TestRSPV0050MalformedDeclaration`, `TestRSPV0050TracedSPIExecution`, `TestRSPV0050FileComponentsOnly`, `TestRSPV0050IncompleteTrace`, `TestRSPV0050RefusedExecution`; `TestRSPV0050FileOnlySelection`, `TestRSPV0050SelectionCapability`, `TestRSPV0050SelectionCapabilityRunnerBinding` (adapter-to-shadow exclusion, missing/unknown component controls and script-runner binding through collection union; binding regression NOT_RUN, build-only); live runner qualification remains planned |
| RSP-V0-049 | `internal/testevidence/capture.go`, repository-keyed reads; `internal/testevidence/freshness.go`, pin invalidation | `TestRSPV0049SiblingRead`, `TestRSPV0049SiblingPinComponents`; sibling diff selection remains planned in the selector slice |
| RSP-V0-053 | `internal/testevidence/record.go`, target key; `internal/testevidence/freshness.go`, exact target identity; `internal/runnerqualification/plan.go`, independent target/configuration admission scopes | `TestRSPV0053TargetKey`, `TestRSPV0053IndependentQualificationScopes`; cross-target reuse remains unqualified |
| RSP-V0-054 | `internal/testevidence/capture.go`, unmodeled build attribution | `TestRSPV0054UnmodeledBuild` |
| RSP-V0-057 | `internal/runnerpolicy/miss.go`; T6-validated requalification; decision `docs/build-log/2026-11-08-rsp-t7-consumption.md` | `TestRSPV0057ConfirmedMissDisablesClass`, `TestRSPV0057FlakeDoesNotDisableClass`, `TestRSPV0057PinnedReplayAndNoAliasing`, `TestRSPV0057UnknownCaptureFull`, `TestRSPV0057UnsetSamplesNothing`, `TestRSPV0057ActualRunBinding`, `TestRSPV0057RequalificationClearsBlock`, `TestRSPV0057InvalidRequalificationKeepsBlock`, `TestRSPV0057NoWorkHistoryComplete`, `TestRSPV0057ConfirmationSupersets`, `TestRSPV0057ConfirmationUnknownPlans`, `TestRSPV0057OtherConfigurationCannotClearMiss` (only the accepted record's own `runnerqualification.Plan.Sequence` clears a miss) (whole `internal/runnerpolicy` package passed in self-review repair); live default-branch sampling and protected retention NOT_OBSERVED |
| RSP-V0-051 | `internal/testevidencedoc`, `corvint test-evidence`, opt-in MCP and AFU projection | `TestRSPV0051QueriesPositiveAndNegative`, `TestRSPV0051Abstentions`, `TestRSPV0051ExplainSelector`, `TestRSPV0051Goldens`, `TestRSPV0051InvalidAndCapped`, `TestRSPV0051IncompleteInventoryCoverage`, `TestRSPV0051ReadNoWrite`, `TestRSPV0051ReadMissingUnsafeAndIndex`, `TestRSPV0051CLIReadAndNoWrite`, `TestCLIKit` (the `test-evidence` row), `TestRSPV0051MCPOptIn`, `TestRSPV0051FlowProjection`, `TestRSPV0051ParseEndOfOptions` (MCP path follows `--`; written; execution NOT_RUN in build-only lane) |
| RSP-V0-052 | `internal/appflows/e2erunner/e2erunner.go` (one Playwright inventory via `browsercov.PlaywrightInventory`, key check, shared selector), `internal/appflows/selection_runner.go` (dependency-free callback binding); T1/T2 consumption, shadow only | `TestRSPV0052E2ESafeConsumesRunnerRecords`, `TestRSPV0052NoLegacyFallbackOnMalformedInventory` (also refuses a separate or malformed list report) |
| RSP-V0-056 | `internal/runnerqualification`, retained collection and inventory timing; existing shadow selector keeps missing records selected | `TestRSPV0056MeasuredCollectionAndInventoryCosts`, `TestRSPV0056UnobservedCostStaysUnqualified`, `TestRSPV0056CollectingRecordSelectsWithoutWaiting` (written, NOT_RUN in build-only lane); live runner cost measurements NOT_OBSERVED |
| RSP-V0-055 | `internal/testevidence/union.go`, distinct collection count, conservative union and nondeterminism; `internal/runnerselection/select.go`, qualification-gated exclusion | `TestRSPV0055Collections`, `TestRSPV0010UnionRanges`, `TestRSPV0055CollectionReadiness`, `TestRSPV0050SelectionCapabilityRunnerBinding` (unbound script markers cannot become ready through union; written, NOT_RUN in build-only repair) |
| RSP-V0-058 | `internal/testevidence/freshness.go`, repository-keyed environment digest | `TestRSPV0058EnvironmentDigest` |
| RSP-V0-059, 060 | (PLANNED) T6 qualification identity (closure digest); T10 conformance kit with a per-runner in-process adapter | (PLANNED) closure-digest and contract-suite fixtures |
| RSP-V0-061, 062 | (PLANNED) T6 harness equivalence check, carry-forward record and per-trial retention with offline re-score | (PLANNED) carry-forward and re-score fixtures |
| RSP-V0-064 | no code change (no-go for all three providers); `docs/build-log/2026-11-11-test-provider-process-measurement.md` | measured evidence: binary sizes, core attribution and 30-call p50/p95 per provider in that entry |


Repair evidence for the JavaScript pieces (`RSP-V0-002`, `RSP-V0-003`, `RSP-V0-005`):
`TestRSPV0002JavaScriptExplicitDiscoveryMatchesExecution` covers literal metacharacter paths
through both discovery and execution; `TestRSPV0002ExplicitFilesUnknownFilterRefused` refuses
an unsupported explicit filter. `TestRSPV0003JavaScriptFailureSurvivesMalformedReport` runs the
native parser through `SelectionSession.Run`, retaining a failure across malformed siblings and
a passing FULL retry. `TestRSPV0003TerminalFailureIndependentOfSiblingAndAggregate` covers
unknown assertion forms and malformed aggregate metadata. `TestRSPV0005BrowserPartialCoverageSelector`
connects native browser coverage to the shadow selector, with full-coverage controls;
`TestRSPV0005WholeLineProofAllowlist` covers unknown unit states, mapped summaries, truncated
and mixed representations. Browser whole-line proof admits only exact direct UTF-16 arrays
with states 0 and 1 for every unit in every observation; all other forms retain unavailable
native evidence and FULL selection. The terminal-result allowlist remains passed, failed, and
runner-reported skip statuses; invalid siblings cannot erase independently validated failures.
These repair fixtures pass in their whole packages; live runner qualification remains NOT_RUN.

The subsequent integration repair adds `TestRSPV0019NeverJoinedSharerSelectsServerChange`
(missing dump reaches the selector), extends `TestRSPV0019ServerSharedAndResetDump` (all
sharers complete in either dump order), and adds `TestRSPV0019DumpCollectionPreservesUnknowns`
(incomplete inputs and mismatched sharers). `JoinServerDumps` keeps collection progress in process
without changing the record wire shape; callers replay available dumps against original browser
records. `TestRSPV0002VitestMultiProjectRefused` covers shared identities, disjoint files and one
named project; `TestRSPV0002CommaPathRefusedBeforeExecution` compares discovery with execution's
existing unsafe-selector refusal. These new/updated fixtures are NOT_RUN in the build-only repair
lane; package test execution and independent review belong to the host verifier.

Self-review repair adds `TestRSPV0003PlaywrightGlobalErrorRetainsFailureAcrossRetry` and
`TestRSPV0003JestFileFailureRetainedAcrossRetry` in the native parser packages. Both exercise
the core reconciliation loop with a missing identity and a passing FULL retry: the independently
validated first failure remains decisive. Playwright certifies clean global diagnostics only
when absent, null or an empty array; nonempty arrays and unknown shapes return the existing
report error alongside results. Jest file units retain a recognized failed file status despite
unknown assertion fields or containers. Passing file status still requires a decoded assertion
array whose records use the existing terminal-status allowlist. Unknown forms cannot certify
success. Both whole-package suites pass; this evidence does not claim live qualification.

## Rollout, rollback and compatibility

Rollout is `RSP-V0-040`; every step before its qualification is shadow-only and changes no plan
(`RSP-V0-046`).

Rollback, per runner: deleting or superseding its qualification record returns it to FULL at once,
and the still-pinned plugin keeps supplying `command`, `inventory` and `parse` for that FULL run.
Removing its entry from `.corvint/runner-plugins.json` also returns it to FULL, but only where core
has a pre-plugin execution path for that runner; otherwise the run fails
`runner-adapter-unavailable` (`RSP-V0-031`). Rollback of the whole capability
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

## Linux dependency repair evidence

For `RSP-V0-021`, failed operations retain their existing-path dependency unless the errno is
`ENOENT`, the closed lookup-only allowlist. `ENOTDIR` retains both read and lookup evidence;
unrecognized errno values retain the read claim. `TestRSPV0021CapturedENOTDIRDeletion` traces a
native executable, proves an unrelated change excludable, then selects deletion and rename of the
regular file whose `O_DIRECTORY` failure it asserted. `TestRSPV0021UnknownFailureRetainsPath`
checks the reported errno and an unknown errno without discarding dependencies.

For `RSP-V0-021` and `RSP-V0-022`, an outside executable classification exempts only its own path.
Implicit interpreter inspection still runs. The supported format allowlist is native ELF64,
little-endian, x86-64, current ELF version, EXEC or DYN type, and System V or Linux ABI;
other formats retain `unsupported-executable-format` and incomplete components.
`TestRSPV0021OutsideExecutableInterpreter` executes a toolchain-root binary whose `PT_INTERP`
points to a repository file, retains that read, and demonstrates a changed result after replacing
only the interpreter. `TestRSPV0022OutsideUnknownExecutable` checks scripts and an unknown format.
Whole-package execution evidence and the repair's scope are recorded in
`docs/build-log/2026-11-07-rspt14-dependency-repair.md`. These fixtures do not qualify a runner or
promote shadow selection to production exclusion.

For `RSP-V0-007`, `RSP-V0-021` and `RSP-V0-012(f)`, successful write-only opens and
mutations retain the existing-path dependency even without a subsequent data read. Only an
understood exclusive create or replacement by a tree-created file establishes ownership;
unknown write forms retain the dependency. `TestRSPV0021CapturedWriteSuccessDependency`
traces native probes without startup reads, selects deletion and rename, excludes unrelated
changes for complete captures, and retains selection for an unknown open-flag form.
`TestRSPV0007WriteDependencyDefault` checks the closed creation-proof allowlist, including
an unknown write kind with a claimed proof. Execution results and the inherited package
failures are retained in `docs/build-log/2026-11-07-rspt14-successful-write-repair.md`.
The earlier repair did not produce `RSP-V0-051`. The 2026-11-08 read-context decision now
permits the command, opt-in MCP tool and AFU projection over existing records with named
abstentions for unavailable inventory and qualification. The rsp051 build adds those surfaces
and positive/negative fixtures; their execution remains NOT_RUN until integration testing.
Persisted inventory/admission storage and production exclusion remain outside this delivery.

## Appendix: emitted codes

Each bounded refusal, reason or fallback code the RSP packages emit, by owning package, so the
error-code ownership gate (ECO-V0-002) finds its owning spec. A code's meaning is the
requirement that cites its emitting package; this list adds no behaviour.

| Package | Codes |
|---|---|
| `internal/appflows` | `inventory-key-mismatch` |
| `internal/jslinecov` | `coverage-cap-or-identity`, `coverage-range-invalid`, `coverage-source-unbound`, `loaded-source-unbound`, `native-components-malformed` |
| `internal/proctrace` | `child-identity-missing`, `clone-untraced`, `clone3-configuration`, `concurrent-filesystem-path-race`, `darwin-mechanism-unqualified`, `descriptor-flags-malformed`, `descriptor-flags-missing`, `empty-path`, `event-cap`, `exec-digest-cap`, `exec-digest-unavailable`, `exec-identity-missing`, `exec-metadata-unavailable`, `executable-capabilities`, `executable-resolution-failed`, `filesystem-matching-unavailable`, `group-stop`, `initial-stop-missing`, `malformed-elf-interpreter`, `malformed-open-how`, `missing-context`, `noncanonical-repository`, `openat2-resolution-policy`, `parent-traversal`, `path-cap`, `path-resolution-failed`, `privileged-executable`, `process-cap`, `procfs-process-relative-path`, `ptrace-options-refused`, `ptrace-resume-failed`, `register-read-failed`, `rename-target-missing`, `seccomp-install-refused`, `shared-memory-path-race`, `symlink-resolution-failed`, `thread-exec-identity-change`, `trace-start-refused`, `unexpected-syscall-stop`, `unexpected-wait-status`, `unresolved-descriptor`, `unresolved-descriptor-path`, `unresolved-dirfd`, `unresolved-elf-interpreter`, `unsupported-ioctl`, `unsupported-open-how`, `unsupported-rename-flags`, `unsupported-syscall-abi`, `wait-failed` |
| `internal/rubylinecov` | `ruby-computed-read-or-dispatch`, `ruby-coverage-files-missing`, `ruby-coverage-line-cap`, `ruby-coverage-line-invalid`, `ruby-coverage-shape-mismatch`, `ruby-coverage-shape-missing`, `ruby-coverage-unattributed-or-incomplete`, `ruby-load-coverage-missing`, `ruby-source-line-cap`, `ruby-source-unpinned` |
| `internal/runnerpolicy` | `base-outcome-not-passed`, `confirmation-outcomes-inconclusive`, `confirmation-unverifiable`, `confirmed-base-pass-head-fail`, `governing-plan-unavailable`, `head-failure-did-not-reproduce`, `local-ci-parity-widened`, `runner-duplicate` |
| `internal/runnerprotocol` | `candidate-pins-unavailable`, `candidate-revision-unavailable`, `native-skip`, `runner-failed`, `trusted-base-unavailable`, `trusted-pins-unavailable` |
| `internal/runnerqualification` | `add-target-not-absent`, `data-target-invalid`, `delete-target-invalid`, `file-operator-unsupported`, `file-target-class-invalid`, `file-target-invalid-or-duplicate`, `file-targets-empty-or-capped`, `population-bytes-capped`, `population-capped`, `population-empty`, `qualification-budget-exceeded`, `qualification-class-missing`, `qualification-class-not-passing`, `qualification-cost-ceiling-exceeded`, `qualification-cost-invalid`, `qualification-identity-changed`, `qualification-incomplete`, `qualification-not-admitted`, `qualification-order-ambiguous`, `qualification-population-digest-invalid`, `qualification-records-capped`, `rename-destination-not-absent` |
| `internal/runnerselection` | `change-path-or-range-unverifiable`, `comparison-identity-unverifiable`, `configuration-changed`, `configuration-path-unverifiable`, `confirmed-miss-disabled`, `evidence-not-admitted`, `execution-configuration-unqualified`, `execution-model-unsupported`, `index-missing-stale-or-unverifiable`, `input-cap-exceeded`, `inventory-divergent`, `runner-cap-exceeded`, `runner-identity-missing`, `sampling-disabled` |
| `internal/shaderdialect` | `shader-aggregate-unsupported`, `shader-body-cap`, `shader-byte-cap`, `shader-continuation-unsupported`, `shader-directive-malformed`, `shader-directive-unsupported`, `shader-include-malformed`, `shader-include-unresolved`, `shader-invalid-text`, `shader-signature-malformed`, `shader-signature-unsupported`, `shader-trailing-declaration`, `shader-unbalanced-body` |
| `internal/testevidence` | `attribution-marker-outside-tree`, `attribution-tests-invalid`, `attribution-tests-missing`, `collection-count-invalid`, `commit-missing-or-malformed`, `cross-repository-link`, `duplicate-record-member`, `environment-exclusion-invalid`, `environment-key-missing-or-short`, `environment-name-invalid`, `environment-value-uncapturable`, `executable-digest-drift-or-missing`, `execution-identity-drift`, `execution-identity-incomplete`, `malformed-record`, `producer-provenance-incomplete-or-duplicate`, `read-blob-malformed`, `read-capture-not-started`, `read-path-unresolved`, `record-depth-cap`, `record-invalid-utf8`, `record-size-cap`, `repository-declaration-invalid`, `runner-identity-drift`, `runner-identity-incomplete`, `sibling-pin-drift`, `target-identity-missing`, `trailing-record-data`, `undeclared-outside-read`, `union-blob-drift`, `union-collection-conflict`, `union-identity-drift`, `unsupported-record-format` |
| `internal/testevidencedoc` | `component-unavailable`, `diff-unresolved`, `file-changed`, `path-changed`, `read-cap-exceeded`, `record-cap-exceeded`, `records-unreadable`, `result-cap-exceeded`, `unsafe-or-capped-file`, `unsupported-record` |
| `internal/testrunner` | `selection-cap-exceeded` |
| `internal/testrunner/golang` | `go-inventory-cancelled`, `go-inventory-capture-incomplete`, `go-inventory-declared-test-missing`, `go-inventory-executed-test`, `go-inventory-key-mismatch`, `go-inventory-output-mismatch`, `go-inventory-package-directory-mismatch`, `go-inventory-package-duplicate`, `go-inventory-package-missing`, `go-inventory-source-ambiguous`, `go-inventory-source-invalid`, `go-inventory-source-unbound`, `go-inventory-source-unparseable`, `go-inventory-unparseable-list`, `go-loaded-outside-repository`, `go-loaded-path-unsupported`, `go-loaded-source-unbound`, `go-output-action-unknown`, `go-output-event-cap`, `go-output-malformed`, `go-output-missing-or-capped`, `go-output-package-or-order-invalid`, `go-output-truncated`, `go-run-failed` |
| `internal/testrunner/registry` | `capture-bound`, `capture-copy-failed`, `checkout-not-immutable`, `class-source-unresolved`, `compile-classpath-unresolved`, `console-discovery-classpath-mismatch`, `console-or-runtime-classpath-unresolved`, `console-pin-mismatch`, `count-cap`, `coverage-source-unresolved`, `dotnet-coverage-artifact-missing-or-duplicate`, `dotnet-coverage-capture-incomplete`, `dotnet-coverage-line-outside-source`, `dotnet-platform-runner-mismatch`, `dotnet-platform-undetermined`, `duplicate-or-capped`, `jacoco-agent-invalid`, `jvm-capture-key-or-attribution-mismatch`, `jvm-coverage-cap`, `jvm-coverage-report-cap`, `jvm-inventory-cancelled`, `jvm-inventory-key-mismatch`, `jvm-jacoco-malformed-or-unmapped`, `jvm-jacoco-unknown-or-malformed-xml`, `jvm-jacoco-xml-export-required`, `jvm-native-not-collected`, `jvm-process-coverage-incomplete`, `jvm-process-parent-unresolved`, `jvm-process-root-divergent`, `jvm-process-tree-incomplete-or-capped`, `jvm-regions-malformed-or-unbound`, `jvm-source-mapping-divergent`, `missing-malformed-or-capped`, `missing-or-divergent-summary`, `native-artifacts-absent`, `python-coverage-cap`, `python-coverage-component-invalid`, `python-coverage-excluded-lines`, `python-coverage-files`, `python-coverage-format`, `python-coverage-line-cap`, `python-coverage-line-invalid`, `python-coverage-lines-malformed`, `python-coverage-malformed`, `python-coverage-member-unsupported`, `python-coverage-source-unpinned`, `runner-plugin-absent`, `runner-unsupported`, `runtime-classpath-invalid`, `rust-build-failed`, `rust-build-local-package-unresolved`, `rust-build-message-malformed`, `rust-build-message-unsupported`, `rust-build-output-incomplete`, `rust-build-package-unresolved`, `rust-build-script-not-captured`, `rust-build-script-output-missing`, `rust-build-script-output-unresolved`, `rust-build-script-without-message`, `rust-build-target-unresolved`, `rust-build-trailing-output`, `rust-cargo-metadata-malformed-or-capped`, `rust-cargo-package-invalid`, `rust-cargo-package-source-unsupported`, `rust-cargo-target-kind-missing`, `rust-cargo-target-unresolved`, `rust-coverage-artifact-missing-or-duplicate`, `rust-coverage-process-tree-incomplete`, `rust-loaded-source-universe-incomplete`, `rust-local-dependency-outside-repository`, `rust-native-link-inputs-unmodeled`, `rust-package-manifest-invalid`, `rust-runner-manifest-not-modeled`, `rust-workspace-member-unresolved`, `rust-workspace-outside-repository`, `script-inventory-cancelled`, `script-trace-component-malformed`, `script-trace-unavailable`, `shell-report-source-mismatch`, `shell-skip-reason-missing`, `shell-terminal-result-unknown`, `shell-trace-execution-failed`, `source-set-incomplete-or-not-native-test`, `source-set-unsupported`, `summary-divergent`, `tool-root-or-staging-path-invalid`, `trailing-discovery`, `unfiltered-request-required`, `unparseable-discovery`, `unsupported-or-unmapped-test` |
| `internal/testrunner/swiftselection` | `swift-inventory-cancelled`, `swift-inventory-cap-exceeded`, `swift-inventory-key-mismatch`, `swift-inventory-missing-truncated-or-capped`, `swift-inventory-source-map-divergence`, `swift-inventory-unmapped-or-duplicate-test` |
