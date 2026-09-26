# Go production kernel migration V0

- Owner: Russell Lewis
- Date: 2026-08-23
- Intent status: accepted
- Delivery status: experimental
- Authoritative inputs: `AGENTS.md`, `ROADMAP.md`, `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`,
  `docs/DOGFOOD.md`, `docs/specs/agent-harness-integration-v0.md`,
  `docs/specs/test-claim-qualification-v0.md`,
  `docs/specs/release-artifact-integrity-v0.md`, `LICENSE`, `LICENSING.md`, and the
  repository owner's 2026-08-23 migration instruction and 2026-09-05 instructions accepting
  `GPK-V0-055`/`GPK-V0-056` and requiring learned-path floor isolation under `GPK-V0-039`/`GPK-V0-044`

## Agent digest
- Claim: Corvint is migrating hot paths to one native-Go binary; GPK-V0-043 and GPK-V0-044 are experimental, while promotion and cutover remain gated.
- Status: accepted/experimental; GPK-V0-043 implementation evidence landed 2026-08-31, GPK-V0-044 implementation evidence landed 2026-09-01, and GPK-V0-055/056 are implemented in the 2026-09-05 repair lane, all without promotion
- Exists: the native Go binary, all six OCM workflow actions, and independently gated migration slices; full cutover remains unqualified.
- Blocked on: remaining parity, benchmark, platform, dogfood, and promotion gates.
- Read next: User and measurable job; Relationship to the deployment-neutral platform direction; Verified current state.

## User and measurable job

An engineer, agent harness, IDE, or future continuous-verification client needs one fast,
dependency-free `corvint` process whose answers are indistinguishable from the accepted Corvint
contracts. The migration succeeds only if Go removes Python startup and deployment cost without
weakening evidence identity, uncertainty, bounds, privacy, local-only operation, or rollback.

This contract authorizes a bounded runtime replacement. It does not authorize new product
semantics, a new wire profile, a database, a daemon, hosted processing, telemetry, a license
change, or a claim that the future continuous-verification product is delivered. The accepted
owner instruction supersedes only `ROADMAP.md`'s V4 “package ... without a rewrite” step and
`docs/PRODUCT.md`'s earlier language-rewrite non-goal for measured production hot paths under this
parity-gated contract. It does not authorize a big-bang rewrite: Python remains authoritative until
each independently gated slice passes. All other roadmap, product, and promotion gates continue to
apply.

## Relationship to the deployment-neutral platform direction

[`deployment-neutral-index-platform-v0.md`](deployment-neutral-index-platform-v0.md) accepts the
future one-kernel, one-immutable-format product direction. It does not change this migration's
frozen parity boundary, delivery state, cutover gates, or rollback. The target production
indexing/query engine is native Go with no Python dependency after the gated cutover; Python remains
authoritative until this contract's existing promotion conditions pass. A separately pinned documentation renderer may
contain Python only outside the engine and only under its own accepted execution profile. Hosted,
review, cloud-range, and vector profiles are descendants of the deployment-neutral contract, not
exceptions smuggled into this migration.

An optional external analyzer process is outside this migration. It may be considered only after
[`Analyzer Capability Contract V0`](analyzer-capability-contract-v0.md) and an exact profile are
accepted; it is never a GPK dependency, parity substitute, fallback, or authority to weaken this
contract's Git-only baseline.

## Verified current state

At base revision `84649ec4ae17192aebd19ca9396a3fd852181c0a`:

- `pyproject.toml` installs `corvint = corvint_cli:main`; Python is the only production CLI.
- `src/corvint_cli.py` owns activation, query, feature, impact, evaluation, explicit outcome,
  harness, CEM, LRF, and OCM commands. Successful commands emit sorted compact JSON plus one LF,
  except LRF's separately frozen canonical bytes. Structured failures emit JSON on stderr and
  exit 2; a valid negative result may exit 1.
- query, feature, impact, and activation compile Git-pinned evidence through
  `src/context_corvint_index.py`. A disposable content-addressed cache may be written under Git
  metadata or an explicit absolute `CORVINT_CACHE_DIR`; repository, index, refs, configuration,
  worktree, CEM/OCM, and trace state are not read-command outputs.
- CEM 0.1/0.2, experimental OCM, LRF, TCQ, and `corvint-harness-event/0` already have canonical
  identities and adversarial tests. TCQ freezes Python test parsing to Python 3.9 grammar; a Go
  implementation cannot substitute the host Go or Python grammar.
- `interop/cem01-go` is a Corvint-authored, standard-library clean-room CEM 0.1 consumer. It proves
  one public protocol is implementable in Go; it is not the product kernel and must remain an
  independent Apache-2.0 interoperability probe.
- a wheel-installed Codex startup hook produced five receipts in 0.58--0.91 seconds after a
  Python cold-path repair. Those observations clear the two-second wrapper timeout but are not a
  p95 and do not satisfy the 250 ms non-query target in `AHI-012`.
- no root Go production module, cross-runtime CLI oracle, Go binary release contract, or completed
  Corvint/Beamfall Go dogfood record existed at that base revision.

Any implementation already present when this spec is reviewed remains experimental until its
requirements below have evidence. File existence is not parity.

## Definitions

- **Python oracle**: the Python implementation pinned to the exact migration base or candidate
  revision and run only by development, conformance, and release gates.
- **candidate**: the Go implementation under test. The first executable is named `corvint` so it
  cannot shadow the production `corvint` command accidentally.
- **observable result**: exit status, stdout bytes, stderr bytes, created/changed/deleted
  non-cache paths, file bytes and modes, repository status, and bounded process termination.
  Disposable runtime-tagged cache names and bytes may differ, but their confinement, modes, bounds,
  cleanup, and effect on command output remain observable.
- **parity case**: one canonical manifest entry containing exact argv, bounded stdin, selected
  environment, repository fixture identity, expected mutations, and oracle result digest.
- **supported slice**: commands for which the candidate claims parity. Any other command remains
  on Python and must be rejected explicitly by `corvint`; it is never silently approximated.
- **cutover**: distributing the Go executable under the name `corvint` or making a maintained
  integration invoke it by default.

## Requirements

### Frozen behavior and exact parity

- `GPK-V0-001`: The migration MUST preserve the complete CLI surface present at the pinned
  migration base: global `--root` and `--version`; `init`, `adopt`, `query`, `feature`, `impact`,
  `eval`, `record`; `harness event`; `lrf`; CEM `begin|prepare|cite|mark|verify|status|report`; and
  OCM `prepare|link|mark|status|verify|report`. Command names, option names, defaults, repetition,
  ordering, help, version text, validation precedence, exit status, stdout, stderr, and requested
  file effects are compatibility surface. A reviewed CLI manifest MUST freeze them before default
  cutover.
- `GPK-V0-002`: For every supported parity case, Go and the Python oracle MUST produce the exact
  same observable result from independent copies of the same starting state. JSON equivalence is
  insufficient: canonical bytes, terminal LF, stream selection, non-cache file effects, file mode,
  and exit code MUST match. Runtime-tagged derived caches are the sole file-byte exception and MUST
  satisfy `GPK-V0-007`. A field addition, omission, reorder, normalization, or error-precedence change is a wire
  change owned by its existing spec, not a migration exception.
  Matching is the pass condition, not the authority: where the two runtimes disagree, `GPK-V0-033`
  governs which one is wrong.
- `GPK-V0-003`: Go MUST reproduce every accepted receipt, map, report, identity, digest preimage,
  ordering rule, enum, and resource bound consumed by the supported CLI slice. This includes
  `corvint-harness-event/0`, CEM 0.1/0.2, experimental OCM 0.1, LRF 0, TCQ 0, activation receipts,
  context receipts, and local trace rows where the owning command is supported. Existing schemas,
  conformance fixtures, and negative vectors remain authoritative; the migration MUST NOT create a
  `go` wire variant.
- `GPK-V0-004`: Invalid, hostile, ambiguous, stale, concurrent, oversized, unsupported, or
  unavailable inputs MUST preserve the Python contract's fail-open/fail-closed choice and error
  precedence. Go panics, partial JSON, stack traces, silent truncation, locale-dependent text, and
  success after an oracle rejection are release blockers.
- `GPK-V0-005`: The parity runner MUST exercise fresh processes, SHA-1 and SHA-256 repositories,
  clean and mixed worktrees, linked worktrees, non-ASCII and CRLF content, symlinks and hostile
  parent paths, malformed/duplicate-key JSON, missing Git objects, timeouts, interrupted writes,
  and every valid and invalid public conformance vector. Mutating items MUST run in independent
  fixture copies so no item can prepare state for another. Replay MUST record one
  stable `CANDIDATE-IDENTITY` line immediately before its summary containing the candidate binary's
  SHA-256 and path-normalized, JSON-encoded `go version -m` build information. Cross-case replay MAY
  use the manifest's positive `workers` field, whose committed default is `1` and whose effective
  value MUST be capped by `GOMAXPROCS`; every parity case and refusal item MUST have its own
  workspace root and MUST execute the candidate exactly once. Complete per-item output MUST be buffered
  and flushed in manifest order (`conformance/cli-parity-v0/runner.go:99-150`, `:176-239`). A mismatch
  between the observed and manifest `timedOut` process evidence MUST be reported before any stdout or
  stderr digest or byte-count mismatch. Fixture
  commit construction, final-index construction, and ordinary-repository snapshot discovery MAY use
  bounded batched or in-process paths only when every commit and tree identity, observed repository
  byte, status byte, and snapshot byte stays identical; linked worktrees and other gitfile layouts MUST
  retain bounded Git discovery. An in-process fixture index MUST NOT write object bytes, and its
  computed root tree MUST equal the imported commit's tree or materialization fails.

### Evidence, Git, privacy, and mutation boundaries

- `GPK-V0-006`: Go MUST resolve one HEAD/commit/tree/object-format snapshot per operation using the
  same revision-stability and mixed-worktree rules as Python. Evidence remains pinned to Git blob
  identity and exact spans. Concurrent HEAD or relevant worktree change MUST retry only within the
  existing bound and otherwise return the same explicit unstable/unsupported result; it MUST NOT
  combine revisions.
- `GPK-V0-007`: `init`, `adopt`, `query`, `feature`, `impact`, `eval`, `lrf`, CEM `verify|status`,
  OCM `verify|status`, and every current harness event, including a session-end outcome, MUST leave
  tracked and untracked
  repository content, Git index/refs/configuration, hooks, attributes, CEM/OCM, and trace state
  byte-for-byte unchanged, except for the gitignored self-observation ledger row `SOL-V0-007`
  appends on a non-`query` `unsupported-*` refusal (AGENTS.md invariant 4), the exception
  `GPK-V0-008` also states. A read may create or prune only a bounded, owner-private,
  content-addressed derived cache in Git metadata or an explicit cache directory. The same command
  MUST return identical bytes with an absent, valid, corrupt, read-only, or disabled cache.
  An explicit cache directory that resolves inside the worktree MUST be rejected before write; the
  Python oracle must receive the same repair before that case enters the parity baseline.
- `GPK-V0-008`: Explicit mutators (`record`; CEM `begin|prepare|cite|mark|report`; OCM
  `prepare|link|mark|report`; and any accepted future equivalent) MUST retain their existing path
  confinement, no-follow traversal, owner-only
  mode, lock, fsync, atomic-promotion, no-partial-output, and clean/mixed-tree rules. Failure or
  interruption MUST leave either the exact old artifact or exact new artifact, never a hybrid.
  A mutator that exits non-zero without promoting an artifact MUST additionally leave the worktree
  and the Git directory byte-identical, including directory entries and their modes, except for the
  gitignored self-observation ledger row `SOL-V0-007` appends on an `unsupported-*` refusal (AGENTS.md
  invariant 4). Argument
  validation and revision resolution MUST therefore precede the creation of any scaffolding,
  including an artifact's owner-private parent directory and any lock: scaffolding that a rejected
  invocation never reaches is a mutation, not a no-op, and the conformance snapshot measures
  directory entries so that it is observable rather than merely asserted. The conformance
  refusal replay admits the ledger exception exactly (decision 0121): Git status is unchanged,
  `.corvint/self-observations.jsonl` is an owner-only regular file afterwards, and every other
  worktree and Git-directory entry, with its mode, is byte-identical. Where the Python oracle
  scaffolds before it validates, that is a `python-defect` under `GPK-V0-033` and is NOT repaired;
  the case is recorded in `conformance/divergence-register.md` and carried as an operator-accepted
  oracle-only divergence naming the exact created path.
- `GPK-V0-009`: The Go kernel MUST remain local-only. It MUST make no network request, DNS lookup,
  telemetry emission, repository upload, model call, credential discovery, or implicit Git fetch.
  Git subprocesses MUST retain the existing sanitized configuration, credential, hook, filter,
  replacement-object, alternates, lazy-fetch, bounded-output, timeout, process-group, and descendant
  cleanup rules owned by each capability. They run the executable `git` names on `PATH`; on macOS,
  when that is Apple's xcrun shim, the kernel resolves the Git it would exec once per
  `PATH`/`DEVELOPER_DIR` value with `xcrun --find git` and spawns that file directly, which
  changes no Git behaviour.
- `GPK-V0-010`: This migration MUST add no database, daemon, resident service, background watcher,
  IPC protocol, or mandatory shared state. Each CLI invocation terminates and reaps descendants.
  Caches are disposable derived files, never authority. A future live-verification worker requires
  a separate accepted contract and cannot be smuggled into this migration.
- `GPK-V0-011`: Harness wrappers and the Go kernel MUST preserve `AHI-014`: raw session IDs,
  transcripts, prompts outside the bounded task field, environment maps, raw tool bodies, secrets,
  and credentials are rejected or omitted. Python-to-Go shadow comparison runs locally and MUST
  persist only fixture data or digests, never private repository source or task text.

### Implementation independence and difficult compatibility

- `GPK-V0-012`: Production Go code MUST live outside `interop/**` and `conformance/**` under the
  product license. It MAY consume their published documents and fixtures but MUST NOT convert the
  independent `interop/cem01-go` probe into the product implementation or count product parity as
  independent interoperability evidence.
- `GPK-V0-013`: The promoted Go binary MUST have no Python interpreter, Python package, cgo,
  package-manager, network, or shell dependency. Git remains the sole required external executable.
  Development and release gates MAY invoke the Python oracle. Until full cutover, the Python
  distribution remains a supported fallback rather than a subprocess hidden behind Go.
- `GPK-V0-014`: Before any TCQ-, OCM-, claim-, or frontier-consuming command moves to Go, its parser
  MUST implement the exact frozen `python-ast/1` Python 3.9 grammar and byte-offset behavior or
  abstain exactly where the accepted TCQ contract requires. Calling an installed Python, accepting
  the Go host's idea of Python syntax, or silently narrowing Python support fails parity.
- `GPK-V0-015`: The initial experimental slice MAY implement only non-indexing harness events
  (`session-start` except `compact`, `post-tool`, `stop`, and `session-end`). It MUST use
  `corvint`, emit exact Python-compatible FALLBACK receipts for those events, and exit 2 with one
  explicit unsupported error for `user-prompt`, `file-change`, and compact-start. It MUST NOT be
  advertised as full harness compatibility. The separate
  `corvint --version` MAY identify the candidate as experimental; it is not CLI-parity evidence and
  MUST become the exact production version output before `cmd/corvint` cutover. Later slices expand
  only after their parity rows pass.
- `GPK-V0-027`: Before Pulse P0-B, `corvint` MAY expose an experimental one-shot `impact` command
  for immediate agent dogfood. Its supported profile is Darwin or Linux, a Git repository (whose
  Go module path MUST contain `/` when any changed path is `.go`, because only rule (a) resolves
  through the module path; `.py` and web paths are admitted in any Git repository, as the oracle
  admits them; decision 0015), and one or more normalized paths whose suffix the immutable index
  admits, with only `--limit` as an impact option. The named reverse-import rules are exactly three:
  (a) `.go` — resolution to the containing Go package's import path, which for
  a root-package file is the module path itself (decision 0023; the oracle spells it `module/.`
  and so finds no importer, DR-0017); (b) `.py` —
  resolution over dotted-module candidates derived from the path; (c) `.ts`, `.tsx`, `.js`, `.jsx`,
  `.mjs`, `.cjs` — resolution of relative specifiers and of the configured alias prefix, where that
  prefix MUST be read from the project profile and MUST NOT be hardcoded to any one repository's
  source root. Under rule (c) a specifier is read only from a `'`- or `"`-quoted string literal in live code, in exactly three positions: after the `from` that closes an `import`/`export` clause, directly after `import`, or as the literal argument of `import(`; text inside a `//` comment (ended by the first `\n`, `\r`, U+2028, or U+2029, the ECMAScript line terminators) or a `/* */` comment, inside another string literal, anywhere inside a template literal (its `${...}` substitutions included), or inside a regular-expression literal is not a specifier and contributes no edge, a `/` reads as division unless it follows nothing or one of the punctuators `( , = : [ & | ? { ; ~ ^ % *` and closes before a line terminator, when it is one regular-expression token whose quotes and backticks open nothing (the `FPK-V0-018` rule; a regex after any other token, and JSX text, stay unread, amendment 2026-09-13), a substitution body is tokenised as code, so a `}` or backtick inside its string literals or comments does not end it and only its unbalanced `}` punctuation does (decision 0208), `require(...)` is not recognized, and a source that ends inside an unterminated block comment or template literal MUST record `WEB_SOURCE_UNPARSED` for its imports rather than return the shortened edge set silently (decision 0101). An indexed path whose suffix has no rule above MUST receive the ordinary impact packet
  with the missing dimension disclosed in `coverage.uncertainty` as `reverse-import results for N
  changed paths with no named resolution rule are outside the native Go impact profile`; the `impact`
  context block MUST be byte-identical to `harness event --event file-change` for the same path, tree,
  and dirty set. This disclosure grants no reverse-import rule. A suffix the index does not admit MUST
  receive `unsupported-impact-path-suffix`. Expected bytes for every rule above are captured from the
  Python oracle, except the unruled-suffix disclosure: the oracle refuses such a path (`DR-0029`), so
  that packet's expected bytes are authored from this clause's quoted text (decision 0057). No rule
  may be introduced whose only authority is candidate behaviour (`GPK-V0-033`).
  It MUST use immutable committed evidence for clean paths, disclose mixed-worktree
  freshness, remain read-only/local-only, bound aggregate source admission to 128 MiB, and reject
  unadmitted-suffix, query, budget, unsupported-platform, and oversized inputs explicitly
  rather than approximate them: a platform other than Darwin or Linux returns
  `unsupported-impact-platform` and `--budget-bytes` returns `unsupported-impact-option`
  (`cmd/corvint/main.go:295,307`). Every evidence array MUST be emitted in a deterministic total order derived
  from stable content — the referenced relation's first-occurrence path, then line, then column, then
  the relation key, then the occurrence's own line and column — so that the `MAX_EVIDENCE` truncation
  retains the same entries on every run and in every runtime. An order derived from index insertion
  or map iteration does NOT satisfy this even where it is stable within one runtime, because
  truncation then makes evidence *content* depend on index build order rather than on the repository.
  Observed byte parity on named Corvint and Beamfall fixtures is dogfood
  evidence, not universal Python parity or authority to cut over the production `corvint` command.
  Amendment (proposed, decision 0398; V1-0339): the aggregate-bound refusal keeps its code and
  names no language. Its message states the admitted source bytes, the file count, the total with
  the per-file framing allowance, the 128 MiB bound, and the remedy that paths under `vendor/`,
  `node_modules/`, `dist/`, `build/`, `target/` or `generated/` are not admitted (`IDX-SNAP-V0-018`).
  The `feature` and query paths rename only its leading subject.
- `GPK-V0-028`: Before the complete Phase 2 query port, `corvint` MAY expose one experimental
  authority-start `query` slice for immediate agent-task orientation. The only supported success
  profile is Darwin or Linux; a UTF-8 task of 1--8,000 characters (decision 0023; the oracle stops
  at 2,000, DR-0016) that the existing intent rules
  classify as `project-operations`; a `--limit` in the inclusive range 1--50 (default 10), echoed
  in the request, with the Python oracle's packet at every limit: the one authoritative
  instruction result followed by at most three advisory `learned-path` candidates (decision 0013
  widened decision 0007 D5's 1--10 to the oracle's full range because the packet above limit 1 is
  limit-independent); optional
  `--budget-bytes` in the inclusive
  Python-oracle range 1,216--1,000,000; one uniquely
  highest-ranked UTF-8, LF-only regular root `AGENTS.md`; and either an absent local trace directory or a mixed
  worktree, for which trace learning is already blocked. The result MUST exactly match the Python
  oracle's exit status and canonical bytes, including instruction-line selection, referenced-tool
  ordering, Git-history learning statistics and digest, critical selector, verification commands,
  freshness, exclusions, and packet-byte fixed point. A `grafted` shallow-boundary or replacement-graft
  commit MUST be excluded from learned paths, history statistics, and the canonical history digest,
  because its path list is computed without the missing parent and is not evidence that every listed
  path changed in that commit. It MUST reuse the bounded immutable Go index,
  remain read-only and local-only, and reject every other intent, out-of-range limit, invalid budget, instruction
  authority, clean-tree trace state, platform, or resource profile explicitly rather
  than falling through to partial semantic query. Task lowercasing MUST reproduce Python's
  `str.lower` on every code point, including the U+0130 special casing, so tokenization stays
  byte-compared with the oracle on non-ASCII tasks; a task that is not valid UTF-8 MUST fail closed
  as `unsupported-query-task`. Above limit 1 the packet admits the oracle's second ranked
  instruction document and its advisory learned paths; it still does not rank symbols, so a
  repository holding symbol-bearing sources under the project-operations path prefixes is a
  declared residual divergence, not a parity claim. This slice does not implement general
  repository search, feature/scenario or symbol ranking, trace parsing, cache, automatic harness
  injection, or production cutover. Packet-budget compilation
  MUST reproduce the Python oracle's compact envelope, stable critical-first selection,
  authoritative/advisory/omission counts, critical-missing selectors, uncertainty ordering,
  state precedence, request and coverage-list fallback compaction, and twice-stabilized
  `packet_bytes` / `within_budget`. A
  conformance-only parity-case manifest MAY freeze argv, fixture identity, and oracle digests; it is
  not a runtime authority-adapter manifest and cannot grant repository authority.
- `GPK-V0-029`: `corvint impact` MAY accept revision-absent `.go` files only when the caller
  explicitly selects `--working-tree-untracked`. The default impact profile and its tracked-revision
  output MUST remain byte-for-byte unchanged. The opt-in profile MUST bind the normalized path,
  stable twice-read bytes and SHA-256 digest, single-link regular-file identity, captured Git
  revision and mixed-worktree status digest, slash-qualified module, parsed package name/imports,
  and revision-pinned same-package/reverse-import context. It MUST disclose that working-tree bytes
  are not immutable revision evidence. A non-`.go` path MUST return
  `unsupported-working-tree-impact-path` before either repository condition is checked; only a `.go`
  path may then return `unsupported-working-tree-impact-repository` for a missing captured index or a
  repository without a slash-qualified Go module. It MUST reject tracked files, paths absent from the captured
  mixed status, symlinks in any path component, hard links, special files, path escape, invalid Go,
  files over 1,000,000 bytes, aggregate target bytes over 64,000,000 bytes, or any byte, identity,
  revision, or status race. The profile is read-only and local-only, performs no source mutation or
  network request, and does not broaden revision authority or claim Python parity.
- `GPK-V0-030`: `corvint impact --base FULL_COMMIT_ID` MAY compile a committed range as the
  separate `corvint-range-impact/0` profile. `--base` MUST be mutually exclusive with path and
  working-tree profiles, MUST be one full lowercase object-format commit ID, and MUST be an
  ancestor of a clean, stably captured `HEAD`. The compiler MUST bind the base/head commit and tree,
  clean-status digest, every changed path/status/mode/base blob/target blob, and every admitted Go
  old/new hunk span (including deletion-only spans) in canonical digests. It MUST reject identity or status
  drift, delete/type or mode changes, symlinks, submodules, binary content, shell-unsafe
  paths, malformed or excluded target Go sources, and path/hunk/target-line bounds. Results MUST be limited to direct
  changed Go paths, feature/scenario markers on target lines added or replaced by the diff, and
  uniquely resolved accepted target-HEAD ADRs cited on those lines; it MUST NOT apply the broader
  path-impact reverse-import, test, or reference closure. Every non-Go range member MUST be counted,
  reasoned, sampled, and digest-bound as an omission. Verification MUST name the changed Go package
  tests, `go mod verify` when module files changed, and the repository gate. The existing path and
  working-tree profiles remain unchanged. The range profile is read-only and local-only and claims
  no Python parity. A dirty worktree returns `unsupported-impact-worktree`
  (`internal/contextindex/range_impact.go:324`) unless `GPK-V0-060` allows every dirty entry, and a
  changed `.go` path in the repository root package returns `unsupported-impact-path`
  (`internal/contextindex/range_impact.go:143`).
- `GPK-V0-031`: Before the complete Phase 2 feature port, `corvint` MAY expose an experimental
  `feature FEATURE_ID` slice on Darwin and Linux; any other platform returns
  `unsupported-feature-platform` (`cmd/corvint/main.go:430`). It MUST accept the Python-oracle `--limit` range
  1--50 and optional `--budget-bytes` range 1,216--1,000,000, preserve validation precedence and
  canonical bytes, and reuse the bounded immutable context index and shared packet-budget selector.
  A canonical unknown feature id MUST succeed with an exact empty `OUT_OF_SCOPE` receipt, including
  in a mixed worktree and under a packet budget. A known feature MUST emit the canonical ledger
  result followed by Python-compatible ranked Go-symbol candidates: exact test-marker proximity,
  imported-package expansion, generic-name suppression, compact-id bonus, tuple ordering, evidence,
  and result limiting are compatibility surface. Because the Python oracle additionally ranks
  Python and web symbols, a known feature with `--limit` greater than one MUST return typed
  `unsupported-feature-repository` when an admitted `.py`, `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, or
  `.cjs` source could enter that unsupported ranking path; `--limit 1` and unknown ids do not compute
  candidates and MUST remain supported. The slice MUST NOT inherit impact's slash-qualified-module
  restriction, read stdin, enter trace-operation locking, mutate repository state, invoke Python, or
  claim help/corpus/inventory promotion. The `feature` inventory row remains `UNSUPPORTED` until the
  independent corpus extension rule is satisfied.
- `GPK-V0-032`: Before complete Phase 2 activation cutover, `corvint` MAY expose one shared
  `init|adopt` Genesis inventory slice on Darwin and Linux. It MUST preserve the Python-oracle
  authority-id, revision, repeated-exclusion, full-receipt, canonical receipt and sealed-summary
  contracts, including `COMPLETE`, `PARTIAL`, and `INVALID` states and exit 1 with an `ok:false`
  stdout envelope for `INVALID`. Duplicate exclusions are a compiler-level `INVALID` receipt, not
  an argument error. The supported repository profile includes SHA-1 and SHA-256 primary and linked
  worktrees, clean and mixed current-worktree status even when inventorying a non-HEAD revision,
  every frozen tracked-entry classification, CRLF and non-ASCII text, symlinks, gitlinks, and safe
  hostile path bytes. Git reads MUST remain bounded, sanitized, local-only, network-inert, pinned to
  one resolved commit/tree, and descendant-reaped on cancellation, timeout, or output excess.
  Repository content and Git metadata MUST remain byte-identical. Unsupported operating systems
  MUST return typed `unsupported-genesis-platform` before repository access; this is an explicit
  exit-2 divergence from the Python oracle. Candidate help MAY describe the slice but claims no
  byte parity. The `init` and `adopt` inventory rows remain `UNSUPPORTED` until the independent
  corpus extension rule is satisfied.
- `GPK-V0-037`: Before complete Phase 2 OCM cutover, `corvint` MAY expose the read-only
  `ocm status|verify|report` slice while `prepare|link|mark` remain typed unsupported. The admitted
  slice MUST reuse the native OCM/CEM repository verifier, preserve the Python oracle's canonical
  stdout, stderr, exit, authority precedence, worklists, counts, policy verdicts, report bytes, and
  location dependence, and leave `status` and `verify` byte-for-byte read-only. `report` MUST
  publish one root-confined, no-follow, owner-private atomic file only when structural verification
  and policy both pass; unlike the Python oracle, which writes an issue report on exit 1, Go MUST
  preserve the oracle's streams and exit while writing nothing on any failed verdict. Absolute or
  outside-root report outputs, and any output other than the default report path with a segment that
  case-folds to `.git`, MUST return typed `unsupported-ocm-output-path` before publication,
  while Python's accepted arbitrary absolute output remains a recorded safety divergence. Until an
  exact output-error envelope is shared, final or ancestor symlink outputs return Go's typed
  `unsafe-output` refusal while Python returns an untyped path-bearing error; neither implementation
  follows the symlink or changes its target. Until an
  exact OCM-owned Python grammar exists, a map containing a `.py` claim MUST return typed
  `unsupported-ocm-python-claims`; malformed or non-object JSON families whose Python error taxonomy
  cannot be preserved by the shared strict parser MUST return typed
  `unsupported-ocm-json-profile` instead of an approximate verdict. Valid-JSON claim or obligation
  structures rejected by the shared parser before the Python validator's later taxonomy MUST
  return typed `unsupported-ocm-closure-profile` rather than an approximate issue code. These
  refusals do not broaden
  the existing Go/JavaScript claim profile or qualify `GPK-V0-014`. The parity inventory row remains
  `UNSUPPORTED` until the independent corpus extension and complete hostile matrix pass.
- `GPK-V0-038`: The harness slice is complete. Under the expansion gate in `GPK-V0-015`, `corvint`
  MAY implement all six lifecycle events and every `startSource`, so the exit-2 refusals that clause
  names for `user-prompt`, `file-change`, and compact-start bind the initial slice only and no
  longer describe the candidate. A compact start MUST rehydrate the dirty working set exactly as the
  oracle does, including which of `COMPLETE`, `PARTIAL`, and `UNTRACKED_ONLY` the rehydration block
  carries, and MUST keep the `PARTIAL`/`COMPLETE` state rather than `UNTRACKED_ONLY` when the dirty
  set is over budget. The two index-backed events inherit the `GPK-V0-027` impact profile. This clause expands the harness surface only. It grants no
  authority to widen the compatibility matrix, which `GPK-V0-023` as amended by decision 0308 keeps
  reported as measured.
  The harness `receiptId` is a **request identity, not an answer digest**: its basis covers the
  adapter, the event, the normalized input, and the repository state, and excludes the `context`,
  `degradations`, `support`, and `frontier` fields the response carries. Two responses that agree on
  the basis share a `receiptId` even where their answers differ, so a `receiptId` MUST NOT be cited
  as an integrity guarantee over what a call returned. Widening the basis is a profile flag day
  (`corvint-harness-event/1`) across every adapter and is deferred (decision 0007, D8).

- `GPK-V0-039`: A context packet asserts that the repository answers the caller's task, and the only
  evidence for that assertion is the words of the task a result actually matched. A packet MUST
  therefore be **withdrawn** — no results, `state` `OUT_OF_SCOPE`, and an abstention whose reason is
  `below-relevance-floor` — unless some single emitted primary result, never a `learned-path`, rests
  on at least two words of the query as written, or on the whole query where it offers fewer than
  two, which is the most a short query can be held to. Support is counted **per result**: one word
  matching result A and a different word matching result B is two unrelated accidents, not evidence.
  It is counted in the query's own
  words, never in the derived terms the query expands into, because expansion adds stems and aliases
  and would let one word look like two. A result carried in behind another — a matched
  record's implementation symbols — contributes no support of its own, because it is evidence about
  that record rather than about the query. An `instructions` result admitted under
  `project-operations` intent rests, in addition to the words its text matched, on the query words
  as written that classified the task: the project-operations terms the shared `queryIntent` rule
  matched. Classification already requires two such words (or the phrase "work queue" plus one),
  so the admitted instruction result is the answer to those words rather than a one-word lexical
  accident, and the floor MUST count them as that result's support (decision 0014). Score MUST NOT be the test: it is an
  unnormalised sum of term counts and id and position bonuses, so it is not comparable between two
  queries, and any threshold over it is a constant fitted to one sample. The floor withdraws the
  whole packet rather than filtering results, so a genuinely relevant low-scoring result is never
  suppressed. Learned support is advisory after this admission and MUST NOT reverse a withdrawal.
  The floor judges the ranking's emitted packet before any packet budget narrows it: a budget that
  drops the only supporting result leaves a `BUDGETED` packet under `GPK-V0-040`, never a
  `below-relevance-floor` withdrawal, because the budget is the cause that packet names (decision
  0208).
  A withdrawn packet cites no result, so its coverage counts are zero, its critical set
  is empty, and its verification plan carries only what a repository with no cited result implies:
  the detected profile's gate. This binds every surface that compiles a query packet, including
  `harness event --event user-prompt`. The Python oracle publishes such a packet as `READY`; that is
  a `python-defect` under `GPK-V0-033`, `src/` is deliberately NOT repaired, and the disagreement is
  registered as `DR-0008` and discriminated by the parity case `harness-user-prompt-out-of-scope`.
- `GPK-V0-040`: A receipt's coverage counts are denominated in the universe of results the ranking
  ADMITTED, never in the narrowed set the receipt emits. `requested_results` MUST therefore name
  what ranking considered, `included_results` what the packet carries, and `omitted_results` the
  difference a ranking ceiling or a packet budget removed; a compilation stage that narrows a
  receipt again MUST carry the earlier stage's admitted count forward rather than remeasure the
  results in hand. Measuring the narrowed set makes `omitted_results` structurally zero, which is
  not a count that happens to be zero but an assertion of completeness the receipt never measured:
  a consumer reading `omitted_results: 0` MUST be entitled to conclude that nothing was dropped,
  and abstention means nothing otherwise. A nonzero omission MUST also be named in `uncertainty`,
  because the number alone does not tell a caller that its packet is a subset. This binds every
  surface that compiles a context receipt, including `impact` and `range impact` under their
  `--limit` ceiling. The Python oracle truncates its result list before it measures it and then
  remeasures the truncated list a second time when it compiles the receipt; that is a
  `python-defect` under `GPK-V0-033`, `src/` is deliberately NOT repaired, and the disagreement is
  registered as `DR-0009` and discriminated by the parity case `impact-ranked-past-limit`.
  Accepted amendment (AT-07, decision 0052): `included_results`, `omitted_results`, `candidates` and
  the top-level `state` are computed before possession suppression and frozen; a new coverage
  member `suppressed_results` carries the count removed by possession; when suppression removes
  every result, `coverage.uncertainty` gains the line "N results suppressed by caller possession"
  and `state` stays as computed (READY/BUDGETED), never NO_CANDIDATES. End of accepted amendment.
  Accepted amendment (decision 0157, 2026-09-12): each competitive feature record `query` routes
  through MUST admit its three strongest implementation declarations into that universe, whatever
  `--limit` the caller named, and MUST NOT admit a fourth. The oracle admits two; that is a
  `python-defect` under `GPK-V0-033`, registered as `DR-0036`, and `src/` remains unrepaired. End of
  accepted amendment.
- `GPK-V0-041`: The typed `unsupported-ocm-python-claims` abstention `GPK-V0-037` requires of the OCM
  read slice binds EVERY Go command that verifies OCM claims, not only `ocm status|verify|report`.
  `GPK-V0-014` admits exactly two treatments of a `.py` claim — the exact frozen `python-ast/1`
  Python 3.9 grammar, or an abstention — and names the third explicitly as a parity failure:
  "accepting the Go host's idea of Python syntax, or silently narrowing Python support." No exact
  OCM-owned Python grammar exists yet, which is the same condition `GPK-V0-037` abstains on, so the
  admitted treatment is the abstention and it MUST be applied wherever the claim verifier runs.
  Concretely, `lrf --ocm` MUST return typed `unsupported-ocm-python-claims` before it verifies a
  claim set containing a `.py` claim, exactly as the read slice does. The refusal is categorical
  rather than conditional on the blob: a candidate MUST NOT decide per blob whether its own
  approximate grammar is close enough, because the cases where the approximation is wrong are
  precisely the ones it cannot recognise. This narrows the candidate's supported profile — an
  invocation the oracle evaluates is refused, and the `lrf` `.py`-claim profile is `UNSUPPORTED`
  rather than parity — and that cost is the point: an abstention that says so is admissible where a
  silent approximation is not. This clause governs OCM claim verification only. The TCQ and
  frontier surfaces reach the same verifier through their own contract, where `TCQ-V0-047` requires
  a per-edge `unsupported-python-grammar` abstention instead of a command-level refusal. That path
  is repaired: `verifyUniverseClaims` (`internal/lrfrepo/universe.go:178`) verifies claim shape
  without the approximate grammar and emits per-edge `PythonClaimEvidence`, and
  `internal/tcq/associate.go` reports the grammar reason ahead of the anchor-profile reason
  (decision 0050 wave, 2026-09-04); `conformance/divergence-register.md` DR-0014 carries the
  measured evidence and is CLOSED. The clause grants no authority to widen the Go claim
  profile and does not qualify `GPK-V0-014`, which the exact grammar alone can satisfy.
- `GPK-V0-045`: An abstaining packet MUST name the reason it abstained at every surface that emits
  it, budget-compacted forms included. Budget compaction MAY drop an abstention's explanatory
  detail — nearest claims beyond the first three, matched terms — but MUST NOT drop
  `abstention.reason`, and MUST NOT drop the `abstention` member itself; a `repository` intent is
  not a licence to remove it. `below-relevance-floor`, `unindexed-worktree-changes`, and
  `no-relevant-candidates` are observable nowhere else in a compacted packet, so without the reason
  a withdrawn packet is indistinguishable from an empty result set and every consumer branch keyed
  on the reason is unreachable rather than merely unused. Unlike `GPK-V0-039` and `GPK-V0-040` this
  is a wire change both runtimes take: the Python oracle receives the identical repair, so the
  clause creates no divergence, and every corpus captured from the oracle — the
  `conformance/cli-parity-v0` manifest and the `conformance/go-query-start-v0` budget vectors —
  MUST be re-captured from the repaired oracle before its cases re-enter the parity baseline.
  `MinPacketBytes` (Go) and `MIN_PACKET_BYTES` (Python) MUST be no smaller than the worst-case
  mandatory abstaining envelope this clause's `abstention.reason` retention produces — the longest
  reason literal, a mixed-worktree `freshness` block, a fully populated `learning` block, and a
  compacted `intent`, at the longest value each mandatory field can carry — rounded up to the next
  64-byte boundary, and MUST be re-derived whenever that mandatory field set changes.
- `GPK-V0-046`: Product invariant 3 ranks project-owned authority above syntax. A `query` packet
  whose every cited non-`learned-path` result carries `authority: syntax` alone has therefore
  matched the task's words and nothing else: no spec, decision, document reference, or project
  instruction corroborates any of them. Such a packet MUST count zero `authoritative_results` and
  MUST carry exactly one deterministic `coverage.uncertainty` entry naming that condition. Ranking
  is unchanged and no result is suppressed: the defect is the packet's claim about its results, not
  their selection, and a floor over score would be the constant fitted to one sample `GPK-V0-039`
  refuses. A result carrying no evidence at all cannot be read as syntax-only and disqualifies the
  packet from the rule, because absent evidence is not weak evidence. This binds every surface that
  compiles a query packet, including `harness event --event user-prompt`. The Python oracle
  receives the identical repair on the same re-capture terms as `GPK-V0-045`.
- `GPK-V0-054`: `coverage.authoritative_results` counts cited results Corvint does not label
  advisory: every included result whose `kind` is not `learned-path`, less the whole-packet
  withdrawal `GPK-V0-046` requires. It is not a count of project-owned authority and MUST NOT be
  read as one; per-row authority is `evidence[].authority` alone. `advisory_results` is its
  complement, and the two MUST sum to `included_results` (decision 0052, 2026-09-04 audit item 1).

### Working-tree and range impact failure codes

Beyond the codes named in `GPK-V0-029` and `GPK-V0-030`, the `internal/worktreeimpact` compiler and
the range compiler in `internal/contextindex/range_impact.go` emits the kebab-case codes below
(decision 0100). Each row cites the first emitting site and quotes the message returned there,
which is the whole of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `impact-range-drift` | `internal/contextindex/range_impact.go:150` | "changed Go path does not match the captured target blob: <value>" |
| `invalid-working-tree-impact-limit` | `internal/worktreeimpact/compiler.go:87` | "working-tree impact limit must be an integer from 1 to 50" |
| `invalid-working-tree-impact-path` | `internal/worktreeimpact/compiler.go:135` | "working-tree impact path must be valid UTF-8 with 1 to 1024 characters" |
| `invalid-working-tree-impact-paths` | `internal/worktreeimpact/compiler.go:90` | "working-tree impact requires 1 to 100 paths" |
| `invalid-working-tree-impact-source` | `internal/worktreeimpact/compiler.go:195` | "working-tree target is not valid Go syntax: <value>" |
| `stale-working-tree-impact` | `internal/worktreeimpact/compiler.go:161` | "working-tree repository root changed while opening" |
| `unsafe-working-tree-impact-file` | `internal/worktreeimpact/compiler.go:218` | "cannot open contained working-tree target: <value>" |
| `unsafe-working-tree-impact-root` | `internal/worktreeimpact/compiler.go:152` | "working-tree impact repository root must be a non-symlink directory" |
| `working-tree-impact-output-failed` | `internal/worktreeimpact/compiler.go:337` | "cannot bind mixed-worktree status" |
| `working-tree-impact-too-large` | `internal/worktreeimpact/compiler.go:66` | "working-tree impact targets exceed 64000000-byte aggregate bound" |

### Performance and packaging

- `GPK-V0-016`: Performance is measured outside both runtimes with an interleaved, preregistered
  harness on the same machine, revision, task corpus, cache state, and sanitized environment. Each
  reported p95 uses at least 100 measured fresh-process samples per runtime after five discarded
  warmups, reports p50/p95/max and failures, and includes Corvint and Beamfall. Measurements with
  unequal outputs, timeouts, hidden cache differences, or changed repositories are invalid.
- `GPK-V0-017`: A slice may cut over only when all its parity cases pass and: (a) non-query harness
  events that do not build an index are at most 100 ms p95 on the reference machine and at most the
  existing 250 ms portable product cap, and index-building non-query events (`file-change`, compact
  `session-start`) are measured with an index snapshot available and are at most that 250 ms cap
  p95 (amended by `docs/decisions/0048-four-owner-calls-2026-09-04.md`); (b) query events after
  an index is available are at most 500 ms p95; (c) the Go non-query p95 is at least 50% lower
  than the paired Python oracle; and (d) no other supported command's p95 or peak resident memory
  regresses by more than 10%. Since 2026-09-11 the `beamfall-snapshot-present` corpus of
  `conformance/perf-v0/manifest.json` is the binding corpus for (a)'s index-building events and
  for (b) (decision 0084, superseding the cold-corpus binding of decisions 0037 item 1 and 0048
  item 1); the cold `beamfall` corpus is reported beside it and is not a threshold, and a formal
  result that measures only the cold corpus reports those criteria `NOT_RUN`. Any threshold not
  measured is `NOT_RUN`, not waived.
- `GPK-V0-051`: `corvint impact --base FULL_COMMIT_ID` MUST admit a `C` or `R` range member when
  Git reports its similarity score and source path, binding the status letter, the similarity score,
  the source path, and the source blob beside the member's own fields, and computing admitted hunks
  against the named source blob. A copy or rename whose source Git does not report MUST stay
  `unsupported-impact-range`. Every other `GPK-V0-030` rejection is unchanged. Accepted by
  `docs/decisions/0050-copy-range-evidence-and-record-admission-2026-09-04.md`.
- `GPK-V0-052`: (accepted, decision 0052, 2026-09-04) when a query term
  exactly names a symbol that `isTestPath` withholds from ranking, `coverage.uncertainty` MUST name
  the withheld count so a caller can tell test evidence exists from a true absence, as exactly the line `N test-path symbol candidates withheld from query ranking; the context verb serves test evidence` with `N` the decimal count and no line when it is zero (decision 0101); ranking,
  abstention and state are unchanged. The term is matched as the task wrote it -- a whole identifier
  token of the task text, not an expanded relevance term -- so a disclosure always names a
  declaration the task actually spelled. The disclosure participates in the byte budget and may
  displace the last fitting result: it is written while results are being fitted, so a packet that
  carries it is one the budget already accounts for, and `within_budget` MUST stay true. Accepted by
  `docs/decisions/0052-owner-accepts-first-wave-proposals-and-authorises-build-2026-09-04.md`.
- `GPK-V0-053`: (accepted 2026-09-04 by decision 0051 item 2, which decides this wording change and
  the `DR-0024` disposition) the `uncertainty` line `GPK-V0-040` requires for a nonzero omission
  MUST name the ceiling that produced it: `omitted by result limit` when the request carries no
  `budget_bytes`, `omitted by packet budget` when it does. A receipt that names a budget the caller
  never set asserts a cause it never measured, and a caller reading it will raise a budget and see
  the same omission unchanged. Ranking, abstention, state, and every budgeted receipt are unchanged.
  The Python oracle names the packet budget in both cases; that is a `python-defect` registered as
  `DR-0024` under `GPK-V0-033`, amending the candidate fragment `DR-0009` and `DR-0025` pin, and
  `src/` is deliberately unrepaired.
- `GPK-V0-058`: (proposed amendment 2026-09-05, owner review pending; not accepted) a harness
  event MAY run its own read inside its `GPK-V0-007` bracket instead of beside a second,
  concurrent bracket of its own. The proof shape is unchanged: one complete identity-and-status
  observation; the event's reads, issued against that observation (an index snapshot hit named
  by its tree and applying its dirty set, or the full build on a miss); one complete
  observation; equal or retry. The `ls-tree` profile read is issued only for the startup
  `session-start` block, the one place the profile is emitted, and the block computed from what
  was read touches no repository state, so it runs beside the closing observation. Acceptance is
  the spawn count and the bytes: with a snapshot present, `file-change`, compact
  `session-start`, and `stop` each spawn exactly four Git processes (eight, eight, and five on the
  concurrent path) and the startup `session-start` spawns five on both; and every receipt --
  `repository`, `receiptId`, `context`, and every degradation -- is byte-identical to the
  concurrent path's over a clean tree, a dirty tree, and a present, absent, and stale snapshot,
  on the fixture repository and on every `cli-parity-v0` harness row. The concurrent path stays
  the default; the shared bracket is reachable only under `CORVINT_HARNESS_SHARED_OBSERVATION=1`
  until accepted. Measurement lives in `docs/plans/shared-observation-prototype-2026-09-05.md`.
- `GPK-V0-065`: (proposed amendment 2026-09-14, owner review pending; not accepted) a repository
  `query` on an index snapshot hit MAY open one stability window on the loader's opening
  observation and close it once, instead of the three concurrent brackets it runs today: the
  loader's pair, the local trace read's pair on either side of its files, and the history-learning
  pair around `git log`. The proof shape is unchanged: one complete identity-and-status
  observation names the snapshot and supplies the dirty set; the trace files and the history are
  read against it; one complete identity-and-status observation closes the window, compared against
  the same index fields each bracket compares today; a change anywhere inside the window is refused
  with `unsupported-query-drift`. Every ranking, learning, abstention, coverage, and state stage is
  arithmetic over the index in memory and is unchanged. The window's closing observation is also
  the identity re-read the loader otherwise makes after decoding the snapshot, so a standalone
  `query`'s loader under this window takes no closing read of its own (as `GPK-V0-058` does for
  harness events): a tree that moves between the opening observation and the decode is refused
  at the closing with `unsupported-query-drift` where the concurrent loader reports a miss and
  builds. A `batch` loader keeps that read: its `context` and `impact` operations answer from the
  hit without a bracket of their own. Within the
  window, an isolated `git status` scan MAY answer its private-metadata probes (`config --list`,
  `ls-files --stage`, `rev-parse --shared-index-path`) from the window's first scan when the
  captured metadata bytes are identical; only a successful probe is reused, the status run itself
  never is, and the concurrent path probes every scan. Acceptance is the spawn count and the
  bytes: on the fixture repository with a snapshot present, standalone `query` and `batch` spawn
  exactly six and eight Git processes on a clean worktree (thirteen and fourteen on the concurrent
  path) and five and seven on a mixed worktree (eight and nine), and every `query` and `batch`
  receipt is byte-identical to the concurrent path's over a clean and a mixed worktree, on the
  fixture and on three real repositories. A snapshot miss carries no loader observation and runs
  the concurrent path unchanged. The concurrent path stays the default; the shared window is reachable only under
  `CORVINT_QUERY_SHARED_OBSERVATION=1` until accepted. Measurement lives in
  `docs/plans/shared-query-observation-2026-09-14.md`.
- `GPK-V0-050`: `corvint record` and `corvint dogfood-record` MUST admit a changed `.gitignore` at
  any depth as a changed path beside the tracked context-index sources they admit today. The file
  MUST NOT be indexed or parsed and contributes no symbols. No other non-source repository file is
  admitted without an amendment naming it. Accepted by
  `docs/decisions/0050-copy-range-evidence-and-record-admission-2026-09-04.md`.
- `GPK-V0-018`: Public Go artifacts MUST be `CGO_ENABLED=0`, built from one exact clean Git commit
  with Go 1.27.1. The root `go.mod` MUST contain exact `go 1.27.1`; an equal `toolchain` directive
  MUST be omitted because Go 1.27 normalizes it away. Every build, test, conformance, benchmark, and release-evidence command MUST run with
  `GOTOOLCHAIN=local`, verify that the selected local toolchain is exactly `go1.27.1`, and fail
  rather than download or select another toolchain. Release gates remain offline, use `-trimpath`,
  reproducible version metadata, and no undeclared runtime dependency. Cutover reports MUST distinguish executable availability from capability support: on Darwin and Linux, Go admits Genesis inventory and secure local trace storage, and Python can load its POSIX trace substrate;
  on Windows, the Python CLI is unavailable because its entry point unconditionally reaches the module-level `fcntl` import, while Go has platform fallback source but Genesis inventory and secure local trace storage are unavailable and the current read-only query profile also refuses, so the Windows release gate remains unmet;
  on every other operating system, Go has those same two capability gaps and every other slice retains its own platform gate, while Python Genesis is source-reachable only on hosts providing `fcntl` and the required POSIX descriptor operations and remains unqualified without native evidence. Retiring Python therefore does not reduce Windows coverage; no other target counts as a coverage loss or gain without its native matrix.
  The release gate MUST build and smoke-test darwin and linux on amd64/arm64 and Windows on amd64, produce one checksum per exact artifact, verify `corvint --version` and a real read-only query, include the applicable unchanged legal files, and prove two same-profile builds are byte-identical before publication.
  Every shipped operating system MUST run its native conformance, filesystem-safety, process-cleanup, and read-nonmutation matrix; cross-compilation is
  not execution evidence. Exact Python process comparison is additionally required wherever the
  Python oracle supports that operating system. Unsupported targets remain named gaps rather than
  inferred support.
- `GPK-V0-057`: A target is shipped only after its native conformance, filesystem-safety,
  process-cleanup, and read-nonmutation matrix has run on that operating system. A target MAY
  remain in the archive matrix as reproducibility evidence without being shipped. Windows/amd64
  MUST remain in the archive matrix and MUST NOT be published until the Windows profile passes.
  Its exit criteria are a qualified read-only query, a native `go test` of the shipped closure,
  explicit Windows skips for POSIX-mode and oracle-backed assertions that cannot run there, and a
  process-cleanup matrix disclosed as `PARTIAL` while containment is unsupported. Accepted by
  `docs/decisions/0061-windows-is-a-deferred-target-and-gitattributes-is-repository-wide-2026-09-05.md`.
- `GPK-V0-019`: The existing Python wheel integrity contract remains binding while a wheel ships.
  A new Go artifact checker or an accepted extension to the release-integrity contract MUST exist
  before publishing Go binaries. The migration MUST NOT turn a Python wheel into an opaque binary
  downloader or select a runtime from the network at install or execution time.

### Conformance authority

- `GPK-V0-033`: The frozen spec, not the Python implementation, is the conformance authority.
  Expected bytes MUST be authored from spec text and MUST NOT be emitted, derived, or transcribed
  from candidate Go code; this preserves the anti-self-certification invariant that `GPK-V0-002`
  exists to protect. The Python oracle remains a mandatory cross-check and MUST run for every parity
  case; removing it is governed by `GPK-V0-025`, never by this clause. Where Go and Python agree and
  both match the spec-authored expectation, the case advances. Where they disagree, the case is
  BLOCKED and MUST be adjudicated against named spec lines before any status change, recording
  exactly one outcome:
  - `python-defect`: the oracle contradicts the spec while the candidate matches it. The case
    expectation is authored from spec text, the candidate passes it unchanged, and the case is marked
    **known-divergent** in the register with its spec citation. The Python oracle MUST NOT be
    repaired: `src/` is scheduled for deletion under `GPK-V0-025`, the expectation no longer derives
    from it, and a repair would erase the recorded evidence that the two runtimes differ. A
    known-divergent case is one where the cross-check is *expected* to disagree; that disagreement is
    not a failure and MUST NOT be reported as one. This supersedes the repair rule in `GPK-V0-007`
    for this outcome only — where the candidate is wrong or the spec is silent, `GPK-V0-007` stands
    unchanged.
  - `go-defect`: the candidate contradicts the spec and MUST be repaired. Python is unchanged, and
    the oracle MUST keep working as a cross-check, because it is the only independent signal standing
    between a candidate defect and a release.
  - `spec-gap`: the spec does not determine the observable. The case is parked, NEITHER runtime wins
    by default, and the owning spec MUST be amended before the case may advance. Both runtimes stay
    as they are until the amendment lands; a `spec-gap` never authorizes a repair to either side.
  A case MUST NOT reach `PASS` by matching Go alone, and a `spec-gap` MUST NOT resolve toward the
  candidate because the candidate is more convenient, newer, or already written. Agreement between
  Go and Python is not proof of correctness: both MAY be wrong against the spec, and a case whose
  expectation was authored from spec text is what distinguishes the two situations.
- `GPK-V0-034`: Every adjudication under `GPK-V0-033` MUST be recorded in
  `conformance/divergence-register.md` with the exact spec lines relied on, the observed bytes from
  each runtime, and the outcome. An open (unadjudicated) entry MUST block `PASS` for its command and
  MUST block that command's contribution to the `GPK-V0-025` retirement window. A green suite over
  an untested divergence is not evidence; a corpus that cannot discriminate a known divergence MUST
  gain a case that can before the affected command is promoted.

- `GPK-V0-035`: Known-divergent cases MUST NOT accumulate unexamined. Every one carries an
  adjudication and a spec citation, and the register MUST be reviewed whenever a command's
  known-divergent count grows, because "the oracle disagrees here" degrades as a signal once it is
  routine and a genuine candidate defect can hide inside the noise. A command whose divergences are
  not individually adjudicated MUST NOT be promoted.
- `GPK-V0-036`: Python retirement is **per command**, and does not wait for the global window
  in `GPK-V0-025`. A command's Python implementation MAY be deleted when all of the following hold:
  its parity rows are `PASS` with expectations authored from spec text; its register entries are all
  adjudicated and none is `go-defect` or `spec-gap`; and its owning specs' clause-coverage table
  (`GPK-V0-034`) contains no `UNVERIFIED` clause. At that point the oracle has no remaining job for
  that command: it is not the expectation, it is not the authority, and it has no unadjudicated
  disagreement left to report. `GPK-V0-025` continues to govern the shipped-runtime default and the
  final removal of Python packaging and CI.

### Licensing, rollout, dogfood, and retirement

- `GPK-V0-020`: The migration MUST NOT alter `LICENSE`, `LICENSING.md`, `PROVENANCE.md`, or the
  path-based split-license boundary. New production Go paths remain Apache-2.0 subject to Commons
  Clause v1.0; existing `conformance/**`, `interop/**`, examples, schemas, and listed protocol docs
  retain plain Apache-2.0. Binary archives carry the exact applicable notices. Commercial Corvint
  Pulse terms and legal conclusions are separate owner/legal decisions, not inferred from Go.
- `GPK-V0-021`: Corvint self-dogfood MUST first use the candidate to orient to this migration, run its
  supported context/harness operations, preserve a private measurement receipt, and compare exact
  outputs with Python. Once CEM/OCM are ported, the final committed Go migration diff MUST be
  prepared, verified, reported, and obligation-checked by the Go `corvint`; before then Python may
  bind the experimental slice but that does not close Go parity.
- `GPK-V0-022`: Beamfall dogfood MUST run second at one exact clean Beamfall revision. It MUST cover
  project-operation authority (`AGENTS.md`, roadmap-only work source, workflow gates, `make orient`,
  and context-packet tooling), Go source/query/impact, mixed-worktree abstention, one real roadmap-
  authorized change when available, and the same output/performance matrix. Corvint MUST make no
  Beamfall code change merely to improve its benchmark. Repository status before and after each
  read-only case MUST be identical.
- `GPK-V0-023`: Rollout order was fixed: experimental candidate; cross-runtime shadow gate; opt-in
  candidate use; maintained integrations switched one at a time only after their exact surface
  passes; Go packaged as `corvint` only after the complete CLI matrix and artifact gate pass; Python
  retained as fallback/oracle; then staged Python retirement. Amended by decision 0308: the tree
  carries one Go runtime and no Python runtime, so the Go binary is `cmd/corvint`, packaged as
  `corvint`, by owner directive. The name carries no matrix claim: the CLI matrix is unchanged and
  stays reported as measured (`full-gpk-v0-005=PARTIAL`, `DR-0038`), the Python oracle survives only
  as the frozen `conformance/cli-parity-v0` provenance, and a partial slice MUST remain visibly
  experimental/FALLBACK and MUST NOT change the global compatibility matrix to FULL.
- `GPK-V0-024`: Rollback MUST require no repository, receipt, map, trace, or cache migration.
  Reinstalling the last Python wheel or selecting the prior integration executable MUST restore the
  old runtime. Runtime-tagged derived caches may be ignored or deleted. Any receipt mismatch,
  release-critical evidence miss, read mutation, panic, unsafe write, process leak, or repeated
  performance-gate failure stops rollout and restores the previous runtime before investigation.
- `GPK-V0-025`: Python production retirement is staged. First, Go is default for one release while
  Python remains a shipped fallback. Second, Python becomes a development-only oracle after two
  consecutive release candidates, at least 100 Corvint changes, and at least 20 Beamfall tasks show
  zero unexplained parity mismatch and zero Go-only critical miss. Python source and its CI lane may
  be removed only after every accepted wire has non-Python conformance authority, every CLI row has
  a Go negative/adversarial suite, rollback no longer depends on Python-generated state, and owner
  approval is recorded. Failed gates reset the retirement window. The window MUST NOT advance while
  `conformance/divergence-register.md` holds an open entry for any command in the supported slice.
- `GPK-V0-026`: Promotion claims are per command and per integration surface. Reports MUST state
  `PASS`, `FAIL`, `UNSUPPORTED`, or `NOT_RUN` for parity, safety, performance, packaging, Corvint
  dogfood, and Beamfall dogfood. A parity inventory row MAY state `PARTIAL` only for a command with
  at least one case or refusal item retired under decision 0308 (`DR-0038`); the row's reason MUST
  name the retired items, and a retired item MUST be reported `RETIRED`, never `PASS` or
  `UNSUPPORTED`. “Written in Go,” a green
  unit suite, or one faster invocation is not delivery evidence.
- `GPK-V0-042`: A **beta rung** exists in `GPK-V0-023`'s order, between opt-in candidate use and
  packaging Go as `corvint`. Admission is per command and per integration surface in the shape
  `GPK-V0-026` already requires, never global. A command admitted to beta MUST remain installed as
  `corvint`; `GPK-V0-015`'s prohibition on installing the candidate as `corvint` is unweakened, and
  the beta rung grants no authority toward it. Reports MUST state a third compatibility label,
  `BETA`, distinct from both `FALLBACK` and `FULL`; a command not admitted to beta keeps its prior
  label unchanged. Admission evidence is exactly the `GPK-V0-017` criteria, the `W11`
  reproducible-build proof, and the `W12` dogfood and rollback runs — no additional evidence class
  is created, and no weaker one is accepted. An open entry in `conformance/divergence-register.md`
  MUST NOT block a command's beta admission, and MUST continue to block that command's retirement
  contribution under `GPK-V0-034` and the retirement window in `GPK-V0-025`; a command admitted to
  beta while holding an open entry MUST disclose that entry in the beta's own release notes, and
  admission without that disclosure is invalid. An entry is open unless the first word of its
  Status bullet is exactly `LANDED`, `CLOSED`, or `ADJUDICATED`; any other word, including an
  emphasized `**OPEN**`, or a missing Status bullet reads as open and obliges that disclosure
  (Go evidence: `TestOpenRegisterEntriesTreatAnUnrecognizedStatusAsOpen`). Rollback under
  `GPK-V0-024` applies to a beta-rung command unchanged.

## Non-goals and simpler baseline

The simpler baseline is the current Python wheel and the repaired sub-two-second harness startup.
It is retained until Go demonstrates exact behavior and a material measured gain.

This migration does not deliver Corvint Pulse, continuous test workers, filesystem watching,
Wallaby-style IDE feedback, runtime-value capture, coverage ingestion, time-travel debugging,
automatic PR merge, a new context algorithm, a new receipt profile, MCP/LSP, hosted services,
accounts, licensing enforcement, or commercial terms. It does not make current experimental
protocols stable or fill an independent interoperability matrix row.

## Trust boundary, limits, and failure policy

Repository paths, Git objects, worktree bytes, JSON, patches, maps, reports, caches, environment,
and subprocess output are untrusted. Existing per-capability byte, count, nesting, time, process,
and Git-operation limits are normative. The Go implementation may centralize enforcement but may
not raise a limit or reorder validation without updating the owning accepted spec and fixtures.

Identity, provenance, safety, mutation, and canonicalization failures fail closed. An unavailable
experimental Go slice fails open only by leaving the production Python command selected; it never
falls through after partially executing a mutator. Unsupported repository language evidence or
dynamic behavior remains explicit `UNKNOWN`. A performance miss blocks cutover but does not change
correct output. A corrupt disposable cache is ignored or safely removed and rebuilt.

## Acceptance and test matrix

| Gate | Required evidence |
|---|---|
| CLI inventory | machine-reviewed manifest covers every base command, option, default, stream, exit class, and mutation class |
| Byte parity | fresh-process differential matrix is exact for success, negative, invalid, hostile, stale, and unsupported cases |
| Public wires | all existing CEM, OCM, LRF, TCQ, harness, activation, receipt, and interop vectors pass unchanged |
| Git | SHA-1/SHA-256, linked worktree, missing object, hostile environment, concurrent HEAD, dirty/untracked, and no-fetch cases |
| Read nonmutation | repository/index/ref/config/hook/attribute/map/trace snapshots unchanged; output equal across cache states |
| Mutators | confined 0600 output, no-follow parents, lock/fsync/atomicity, interruption, collision, and no-partial-output cases |
| Privacy | transcript/session/environment/tool/secret inputs rejected or omitted; comparison artifacts contain only fixtures/digests |
| Processes | timeouts kill and reap all descendants on success, failure, interrupt, and cancellation |
| Python grammar | exact shared Python 3.9 grammar, byte offset, CRLF/non-ASCII, ambiguity, skip, and unsupported-syntax vectors |
| Performance | valid 100-sample paired Corvint and Beamfall measurements satisfy `GPK-V0-016..017` |
| Packaging | clean pinned cross-build, same-profile reproducibility, checksums, legal bytes, install and real read-only smoke |
| Dogfood | Corvint first, Beamfall second; private measurement, exact parity, explicit misses, no benchmark-driven repo edits |
| Rollback | default-to-Python reversal demonstrated without data conversion or cleanup dependency |

The normal Python suite, Python compilation, root Go tests/vet, the independent
`interop/cem01-go` tests/vet, `git diff --check`, and every frozen conformance entry point remain
green throughout the overlap. Go tests do not replace Python oracle comparison until retirement.

## Rollout and rollback

The executable phases and file ownership are in
`docs/plans/go-production-kernel-migration.md`. A phase may advance only when its evidence bundle is
committed or retained under the private dogfood path as required. Compatibility matrices change
only after black-box installation evidence; code landing alone leaves them `FALLBACK`.

Rollback selects the preceding known-good executable and ignores the candidate's runtime-tagged
cache. Wire artifacts remain readable because cutover permits no wire change. No automatic
repository rewrite, cache conversion, trace conversion, or sidecar migration is allowed.

## Traceability

| Requirements | Planned implementation | Promotion evidence |
|---|---|---|
| `GPK-V0-001..005` | `conformance/cli-parity-v0` inventory, source-free fixtures, and differential oracle | query/default-impact seed replay; `TestCompareManifestExpectationReportsTimeoutBeforeStdoutDigestMismatch`; `TestFixtureFastImportPreservesObservedGitLayout_GPKV0005`, `TestFixtureInProcessIndexMatchesGitIndex_GPKV0005`, `TestSnapshotGitDirectoriesRealRepositories`, `TestSnapshotGitDirectoriesPreservesLegacyErrors`, and `TestSnapshotGitDirectoriesDeadlineCleansChild` plus the frozen [fixture-materialization comparison](../../benchmarks/parity-fixture-index/README.md) and [snapshot-discovery comparison](../../benchmarks/parity-snapshot-discovery/README.md) preserve observations while reducing Git launches; exact full success/negative/adversarial matrix remains `PARTIAL`; `TestGoldenMalformedLimitReachesTheVerbRefusal` (`internal/evalrepo`) holds `eval` to the oracle's refusal of a present golden `limit` that is not a JSON integer, where the port had scored it at the default 10; `TestGoldenMalformedBudgetBytesReachesTheOracleTypeRefusal` (`internal/evalrepo`) holds `eval` to the oracle's `validate_budget` type-error text for a present golden `budget_bytes` that is not a JSON integer, where the port had silently coerced it to budget 0 and so surfaced `compileReceipt`'s out-of-range text instead; `TestGoldenNonStringImpactPathReachesTheVerbRefusal` (`internal/evalrepo`) holds `eval`'s `impact` mode to the oracle's `_clean_impact_path` refusal of a non-string entry in a golden `paths` list, where the port's `stringsValue` had silently dropped the entry and could admit a case the oracle refuses whole; `TestGoldenIDStringifiesLikePythonStr`, `TestGoldenPartitionStringifiesLikePythonStr`, and `TestGoldenUnknownModeErrorTextMatchesPythonRepr` (`internal/evalrepo`) hold golden `id`/`partition`/`mode` to the oracle's Python `str()`/`repr()` stringification for JSON bool/null/number values, where the port's plain type assertions had silently coerced every such value to `""` (or `"development"`) instead; `feature_id`, `text`, and the `expected.*` list fields already reach oracle-matching refusals or are behaviorally equivalent under the same field sweep and needed no change |
| `GPK-V0-006..011` | bounded Git/process/file/privacy kernel | hostile Git, nonmutation, atomic-write, privacy, and descendant tests; `internal/contextindex`: `TestBuildPinsCaseFoldCollidingPathsToTheirOwnBlobs` (`GPK-V0-006`: two tracked paths differing only in case each keep their own committed blob, the case-insensitive worktree divergence is reported dirty exactly as Git reports it, and range impact refuses by name); `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (`lrf` CLI-level repository-byte assertion, `GPK-V0-007`), `TestCaseFoldCollidingPathsAreDisclosedNotSilentlyMerged` (`GPK-V0-006` at the CLI: `index`, `query`, `context` and path `impact` disclose both case-colliding blobs and `mixed-worktree`, range `impact` refuses `unsupported-impact-worktree`, every answer deterministic), `TestReadOnlyVerbsWriteNothing` (`GPK-V0-007`: sha256+mtime tripwire over `init`, `adopt`, `query`, `impact`, `docs`, `harness event`, `lrf` allowing only the documented self-observation row); `conformance/cli-parity-v0`: `TestAcceptedDivergenceFrozenAfterDigestsAreTheDeclaredCreation` (`GPK-V0-008`: each accepted row's frozen oracle After digests, which native replay never compares, are rebuilt as the fixture plus the one declared oracle-only creation) |
| `GPK-V0-012..015` | root Go product module and staged command packages | independence review, dependency scan, slice-specific parity |
| `GPK-V0-016..019` | external benchmark and binary artifact checker | paired Corvint/Beamfall results and reproducible cross-build report; paired Python timing is retired by decision 0088; `TestRunProcessReapsDescendantBeforeDelayedSideEffect` preserves native owned-process-group cleanup independently of timing |
| `GPK-V0-057` | `conformance/release-artifact-v0` archive matrix plus platform-scoped publication policy | native query qualification and `go test` of the shipped closure, explicit Windows skip accounting, and `PARTIAL` process-cleanup disclosure while containment is unsupported; Windows publication remains blocked |
| `GPK-V0-020` | strict legal digest pins plus canonical archive member verification in `conformance/release-artifact-v0` | five-target source/archive comparison and dated Go-archive assessment; legal conclusions remain separate owner/legal work |
| `GPK-V0-021..022` | Corvint and Beamfall dogfood packets | private measurements, receipts, status snapshots, misses |
| `GPK-V0-023..026` | per-surface rollout ledger and retirement gate | black-box compatibility, rollback drill, zero-mismatch windows |
| `GPK-V0-027`, `GPK-V0-056` | `cmd/corvint` plus bounded `internal/contextindex` one-shot impact compiler; the Go-module precondition applies to `.go` paths only; index-admitted suffixes without a reverse-import rule retain a disclosed packet; test-inclusive name-reference scan and reference-counted test-convention ranking in `internal/contextindex/impact.go`, while checkpoint callers retain their prior test exclusion | race/vet, cancellation, nonmutation, `impact-python-nomodule` oracle replay in `conformance/cli-parity-v0`, `TestImpactArgvIsClosedToThePortedSeed` and `TestRootPackageDivergenceRejectsAnUnpinnedImporterRewrite` (`conformance/cli-parity-v0`) closing the argv/rewrite self-check gaps `docs/reviews/cli-parity-mutation-audit-2026-09-13.md` found, `TestImpactDisclosesUnruledAdmittedSuffixAcrossSurfaces_GPKV0027`, the rule (c) lexical-position tests `TestWebImportsRejectsCommentedSpecifiers`, `TestWebImportsRejectsQuotedSpecifiers`, `TestWebImportsRejectsTemplateLiteralSpecifiers`, `TestWebImportsLexesSubstitutionBodiesAsCode`, `TestWebImportsEndsLineCommentAtEveryLineTerminator`, `TestWebImportsEndsUnclosedQuoteAtCarriageReturn` (an unclosed `'`/`"` literal ends at `\r` as well as `\n`, and not at U+2028 or U+2029), `TestWebImportsKeepsQuotedCRLFContinuationInsideTheLiteral` (a `\` before `\r\n` continues a `'`/`"` literal through both bytes, so an import spelled in its remainder adds no edge), `TestWebImportsReadsRegexLiteralsAfterExpressionOpeners` (a backtick or quote inside a regex after an expression opener opens nothing, so the import between two such literals keeps its edge), and `TestWebImportsReportsUnterminatedConstructs` (`internal/contextindex/webimports_test.go`), named Corvint/Beamfall fixture dogfood, `TestImpactRanksSamePackageTestsByDeclarationReferences/GPK-V0-056`, `TestBlobAdmissionRefusalNamesTotalAndRemedyWithoutALanguage` (V1-0339), blind-v3 outcome measurement, `DR-0028`, and `DR-0029`; impact promotion remains BLOCKED until its discriminating CLI parity rows exist and pass under `GPK-V0-034` |
| `GPK-V0-028` | bounded `internal/contextindex` authority-start query and packet-budget compiler plus `cmd/corvint` dispatch; advisory learned paths above limit 1 reuse `EvalQuery`'s learned-candidate ranking, whose history parser drops `grafted` commits | `TestAuthorityStartHistorySkipsTheShallowBoundaryCommit`; exact unbudgeted process parity at limits 1, default, 50, and with learned paths, the oracle's limit error at 51, a non-ASCII task, frozen Python budget-vector comparison, hostile rejection, nonmutation, and named Corvint/Beamfall authority-start dogfood |
| `GPK-V0-029` | opt-in `cmd/corvint impact` dispatch plus bounded `internal/worktreeimpact` evidence compiler | `TestCompileValidatesSuffixBeforeRepositoryCondition_GPKV0029`, default byte-parity regression, hostile file/race tests, race/vet/cross-build, and named Corvint untracked-file dogfood |
| `GPK-V0-030` | opt-in `cmd/corvint impact --base` dispatch plus bounded `internal/contextindex` hunk-qualified compiler | path-profile regression, hostile range rejection, race/vet/cross-build, and exact Beamfall ART-SLOTS committed-range dogfood |
| `GPK-V0-031` | bounded `internal/contextindex` Go-symbol feature compiler plus `cmd/corvint` dispatch | fresh-process known/unknown, limit, budget, mixed-worktree, nonmutation, location-independence, and typed-language-boundary oracle tests |
| `GPK-V0-032` | bounded `internal/genesis` repository inventory and sealed summary plus shared `cmd/corvint` activation dispatch | fresh-process init/adopt summary/full oracle parity, SHA-1/SHA-256, linked/mixed/non-HEAD, hostile tree, invalid-state, process cleanup, nonmutation, and location-independence tests |
| `GPK-V0-033` | spec-anchored conformance authority with three adjudication outcomes and no default resolution toward the candidate | a seeded Go/Python divergence is BLOCKED rather than passed; a case whose expectation was authored from candidate output is rejected; `spec-gap` cases stay parked |
| `GPK-V0-034` | divergence register gating `PASS` and the retirement window | an open register entry blocks its command's promotion; a corpus that cannot discriminate a known divergence fails the promotion check |
| `GPK-V0-037` | shared `internal/lrfrepo` OCM/CEM verifier plus bounded `cmd/corvint` read dispatch and atomic report publisher | fresh-process status/verify/report oracle parity, policy thresholds, binding/schema failures, nonmutation, report safety (`TestPublishOCMReportRefusesGitMetadataOutput`), explicit refusals, and location-dependence tests; `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (`dogfood-ocm status` CLI-level repository-byte assertion) |
| `GPK-V0-038` | injected index provider carrying the query, impact, and compaction-rehydration context blocks, leaving `internal/gokernel` dependency-free | `TestImpactDisclosesUnruledAdmittedSuffixAcrossSurfaces_GPKV0027`; eighteen unqualified `harness` parity rows across all six events, both compact rehydration shapes, six negatives, and two hostile bounds; three seeded divergences each failing on the stdout value comparison at the intended case |
| `GPK-V0-039` | per-candidate query-word support recorded by every `internal/contextindex` ranker, read by the packet-withdrawal floor rather than by score | a below-floor query withdraws the packet while every in-scope packet stays byte-identical; `TestEvalQueryRelevanceFloorPrecedesPacketBudget`; `harness-user-prompt-out-of-scope` FAILS against a candidate whose floor is removed |
| `GPK-V0-066` (accepted, decision 0387) | the confident-symbol substitution ahead of the floor withdrawal in `evalQuery` (`internal/contextindex/eval_query.go`) | `TestEvalQueryUnsupportedRecordsYieldToSupportedSymbols`, which FAILS against a candidate without the substitution and keeps the withdrawal when no symbol clears the floor; before/after `corvint eval` and `tools/retrieval-bench --arms corvint` numbers in `docs/BUILD-LOG.md` |
| `GPK-V0-068` (accepted, decision 0396) | the `evalOmitsCompetingRecord` state check in `evalQuery` (`internal/contextindex/eval_query.go`) | `TestEvalQueryLimitOmittingCompetingRecordNeedsWidening`, which FAILS against a candidate without the check at limit 1 and pins `READY` once the limit admits both records; before/after `corvint eval` and `tools/retrieval-bench --arms corvint` numbers in `docs/BUILD-LOG.md` |
| `GPK-V0-070` (proposed, not accepted) | `commentText` and `markerComments` in `internal/contextindex/markers.go`, called from the marker scan in `internal/contextindex/index.go`; `markerCredited`, `reserveCallerRows`, `exportedGoNames` and `namesAny` in `internal/contextindex/impact.go` | `TestMarkersComeOnlyFromCommentSpans` and `TestImpactCreditsOnlyRelatedMarkedTestsAndKeepsCallers` (`internal/contextindex/marker_comments_test.go`), both of which FAIL against the base without the change; before/after `corvint eval` and `tools/retrieval-bench --arms impact` numbers in `docs/BUILD-LOG.md` |
| `GPK-V0-075` (accepted, decision 0412) | the omitted-caller disclosure (`recordGoCaller`, `goImportQualifier`, `goPackageName`, `omittedCallerDisclosures` in `internal/contextindex/impact_callers.go`) threaded through `Impact` (`internal/contextindex/impact.go`) and `EvalImpact` (`internal/contextindex/eval_query.go`); the importing-test carrier reason (`importerCarriers` in `impact` in `internal/contextindex/impact.go`) | `TestImpactDisclosesOmittedDirectGoCallers/GPK-V0-075` and `TestImpactNamesTheImportingTestThatCarriesARelatedMarker` (`internal/contextindex/impact_callers_test.go`), both of which FAIL without the change; the Beamfall before/after packets in decision 0412; `DR-0042` in `conformance/divergence-register.md` |
| `GPK-V0-040` | pre-truncation admitted count captured in `receipt` and carried forward by `compileReceipt`, leaving `setCoverage` the single writer of the three result counts; `EvalQuery` hands `receipt` the whole admitted list instead of a list its own ceiling already narrowed | `impact` and `range impact` report the results a ceiling dropped; `impact-ranked-past-limit` FAILS against a candidate that measures its own truncated output; `TestQueryCoverageCountsAdmittedCandidatesPastLimit` holds `query` to the same count and FAILS against a candidate whose limit-1 packet reports `omitted_results: 0`; the confident-symbol fallback admits at its own caps rather than the caller's ceiling, pinned by `TestQueryCoverageCountsConfidentFallbackPastLimit`; the decision 0157 three-per-feature admission is pinned by `TestQueryAdmitsThreeImplementationsPerFeature`, which FAILS against a two- or four-per-feature candidate. Two denominators remain limit-dependent and are the next slice: feature-symbol admission over the ceiling-narrowed `competitive` list, and the learned-path truncation `evalLearnedCandidates` applies from its `limit` argument |
| `GPK-V0-041` | the `GPK-V0-037` typed `.py`-claim abstention applied at `verifyOptionalOCM`, so the `lrf` OCM leg and the read slice share one refusal instead of one refusal and one approximate grammar | `lrf --ocm` refuses a map carrying a `.py` claim; `lrf-ocm-python-claim-refusal` FAILS against a candidate that verifies that claim with the closed Go grammar |
| `GPK-V0-043`, `GPK-V0-055` | standalone intent validation plus the shared `BuildEval` / `EvalQuery` path for `repository` and `agent-tooling` tasks in `cmd/corvint`; the `GPK-V0-028` authority-start path remains separate; split-before-lower task term derivation in `internal/contextindex/eval_query.go` | fresh-process default and limits 1/10/50, unbudgeted and 2,200-byte budget selection, malformed/bounds, non-ASCII and agent-tooling oracle replay, drift and nonmutation regressions, shared-path structural regression, exact Python-oracle bytes, registered relevance-floor divergence in `conformance/cli-parity-v0`, `TestEvalQueryCamelSplitsTaskBeforeLowering/GPK-V0-055`, development-corpus stable-byte comparison, blind-v3 outcome measurement, and `DR-0027`; query promotion remains BLOCKED until its discriminating CLI parity row exists and passes under `GPK-V0-034` |
| `GPK-V0-044` | repository-intent trace consumption through `internal/tracerecordrepo`, immutable trace snapshots in `internal/contextindex`, and unchanged authority-start dispatch in `cmd/corvint`; `QueryTrace.Revision` carried from `trace.Record` and disclosed in learned-path evidence reasons | exact Python-oracle matching, nonmatching/non-passed, absent, mixed-worktree, budget and packet-field rows in `conformance/cli-parity-v0`; learned-path floor isolation; malformed, unreadable, ancestry, drift, nonmutation, and authority-separation Go regressions; `TestEvalQueryLearnedPathDisclosesTraceRevisionOnDrift` (`internal/contextindex`); `TestFreshProcessRepositoryQueryConsumesMatchingPassedTrace` (`cmd/corvint`) pins the recorded-commit clause in the reason; `query-repository-trace-matching` replays against its pre-amendment frozen bytes as known-divergent `DR-0035` (`conformance/divergence-register.md`), pinned by `TestTraceRevisionDisclosureDivergenceNamesTheManifestRevision` |
| `GPK-V0-045..046` | `compactBudgetEnvelope` and `setCoverage` in `internal/contextindex/receipt.go` with the identical repair in `src/context_corvint_learning.py` | `TestPacketBudgetKeepsTheAbstentionReason` and `TestCoverageWithdrawsAuthorityFromASyntaxOnlyQueryPacket`; `conformance/go-query-start-v0` budget vectors re-captured from the repaired oracle; the `conformance/cli-parity-v0` manifest re-capture is `NOT_RUN` until the repaired oracle is committed and its `expectationProduction.sourceRevision` advances |
| `GPK-V0-048` | `goSymbols`/`goDeclaredNames`/`goGroupNames`/`goSpecNames` in `internal/contextindex/parse.go`, and the `Unparsed` row `recordUnparsed` writes | `TestGoSymbolsNamesEveryGroupMember` (the fixture asserts the lossy scanner reaches only one of six members, so it discriminates); `TestGoSymbolsExcludesFunctionLocalDeclarations`; `TestGoSymbolsNamesGenericDeclarations`; `TestGoSymbolsReportsRefusalAndStaysNonEmpty`; `TestIndexRecordsUnparsedSource` |
| `GPK-V0-047` | index-wide symbol term frequency threaded from `evalRankSymbols` into `evalConfidentSymbols`, plus the `evalLinkedSymbols` dependency-link promotion in `internal/contextindex/eval_dependency.go` | two `internal/contextindex` regressions that FAIL against a ranked-only anchor table and against a candidate with no dependency-link stage; `TestEvalLinkedSymbolsDropsTheFrontierOnlyForAMultiPartNamedRoot`, whose two subtests FAIL against the pre-0157 any-name drop and against a candidate that never drops; byte-identical `evaluation` output against the oracle for `zod` and `execa` on `benchmarks/manifest.json` and `benchmarks/blind-v3-manifest.json` before decision 0157, since when `execa-kill-descendants` is known-divergent `DR-0036` |
| `GPK-V0-049` | the `unparsed` row `recordUnparsed` writes for every refusing grammar and the `unparsed` receipt table in `internal/contextindex/receipt.go` | `TestIndexRecordsUnparsedSource`; `TestUnparsedSeparatesDroppedFromEmpty`; `TestReceiptOmitsUnparsedWhenEverythingParses` (the pair discriminates presence from absence); `TestBudgetedEnvelopeCompactsTheGoOnlyDisclosureMembers` |
| `GPK-V0-052` (accepted, decision 0052, 2026-09-04) | `evalWithheldTestSymbols` and `evalTaskIdentifiers` in `internal/contextindex/eval_query.go`, passed to `receipt` and `compileReceipt` as `extraUncertainty` so `setCoverage` writes it inside budget fitting and the ranking, abstention and state stages never see it | `TestQueryNamesWithheldTestPathSymbols`, whose two subtests discriminate a withheld named test declaration from a non-test declaration that draws no disclosure; `TestQueryDisclosesOnlyTaskNamedTestDeclarations`, which FAILS against a candidate matching expanded terms rather than the task's own identifiers; `TestQueryWithheldDisclosureFitsInsidePacketBudget`, which FAILS against a candidate that appends the line after fitting and returns an over-budget packet with `within_budget: false` |
| `GPK-V0-053` (accepted, decision 0051) | the budget-selected omission cause in `setCoverage` (`internal/contextindex/receipt.go`), which reads the `budget` argument `compileReceipt` already threads rather than re-deriving one | `TestQueryCoverageCountsAdmittedCandidatesPastLimit` (`internal/contextindex/eval_query_coverage_test.go`), which FAILS against a candidate naming a packet budget on an unbudgeted request, paired with `TestQueryWithheldDisclosureFitsInsidePacketBudget` in the same file, which still requires `omitted by packet budget` under a real budget, so the pair discriminates the two causes; `TestCoverageComputesAdvisoryAndCriticalMissing` (`internal/contextindex/receipt_budget_test.go`), whose unbudgeted `setCoverage` call pins the same wording; the `cli-parity-v0` cases `impact-ranked-past-limit` and `query-repository-limit-1` replaying `PASS-WITH-KNOWN-DIVERGENCE` against the amended `DR-0009`/`DR-0025` rewrites |
| `GPK-V0-058` (proposed, not accepted) | `probeRepositorySharing` and `finishConcurrently` in `internal/gokernel/repository.go`, `observeAndBuildContext` in `internal/gokernel/harness.go`, `LoadEventSnapshotObserved` in `internal/contextindex/snapshot.go`, and `harnessSharedIndexedContext` in `cmd/corvint/harness_context.go`, reachable only under `CORVINT_HARNESS_SHARED_OBSERVATION=1` | `TestSharedBracketSpawnsFourGitProcessesUnlessTheProfileIsEmitted` and `TestSharedBracketRerunsTheReadWhenTheObservationMoves` (`internal/gokernel/repository_shared_test.go`), which pin the stage order, the profile-only-when-emitted spawn, and the retry re-running the read; `TestHarnessSharedObservationEmitsByteIdenticalReceipts` and `TestHarnessSharedObservationSpawnCount` (`cmd/corvint/harness_shared_observation_test.go`), which compare both paths' bytes over five repository states and five events and count Git spawns through a PATH shim; the `cli-parity-v0` harness replay on both paths and the Beamfall timing in `docs/plans/shared-observation-prototype-2026-09-05.md` |
| `GPK-V0-065` (proposed, not accepted) | `queryBracket` and `EvalQueryShared` in `internal/contextindex/shared_query.go`, the `bracket` argument `evalQuery` and `evalLearnedCandidates` thread in `internal/contextindex/eval_query.go`, `LoadQuerySnapshot` returning the hit's `LoaderObservation` and `LoadSharedQuerySnapshot` taking no closing identity read in `internal/contextindex/snapshot.go`, `WithProbeReuse` and `probeMemo` in `internal/gitstatus/status.go`, `ReadObserved` and `observedStabilityCheck` in `internal/tracerecordrepo`, and `repositoryQueryContext` and `querySnapshotIndex` in `cmd/corvint/harness_context.go`, reachable only under `CORVINT_QUERY_SHARED_OBSERVATION=1` | `TestEvalQuerySharedMatchesEvalQueryWithFewerGitProcesses` (`internal/contextindex/shared_query_test.go`), which compares the canonical receipt of both paths on a snapshot hit and counts the learn stage's Git calls through a PATH wrapper; `TestEvalQuerySharedRefusesDriftInsideTheWindow`, whose four subtests inject a tree or status change after `git log` and pin the `unsupported-query-drift` refusal with and without the trace read's deferred comparison; `TestQuerySharedObservationEmitsByteIdenticalOutput` (`cmd/corvint/query_shared_observation_test.go`), which compares standalone `query` and `batch` bytes over a clean and a mixed worktree and pins the spawn counts; `TestProbeReuseAnswersRepeatedProbesFromUnchangedMetadata` (`internal/gitstatus/probe_reuse_test.go`), which pins that probes repeat without the opt-in, are answered once under it over unchanged bytes, and repeat over changed bytes; the paired timing on three real repositories in `docs/plans/shared-query-observation-2026-09-14.md` |
| `GPK-V0-060` | `rangeAllowedUntracked` in `internal/contextindex/range_impact.go`; rule, status parser, and digest in `internal/untrackedallowance/allowance.go` | `TestRangeImpactAllowsCommittedIgnoredUntrackedPath`, `TestRangeImpactAllowsDisjointUntrackedPath`, `TestRangeImpactRefusesUntrackedPathOverlappingGoBuild`, `TestRangeImpactBindsUntrackedAllowanceDigest` |
| `GPK-V0-061` | unchanged dirty-path block in `internal/tracerecordrepo/read.go` and `authorityTraceState` in `internal/contextindex/history.go` | `TestReadStaysBlockedForDisjointUntrackedPath`, `TestRepositoryQueryMixedWorktreeDoesNotReadMalformedTraceStore` |
| `GPK-V0-063` (accepted, decision 0166) | `admittedEntries` returns its skipped-suffix count, which `Index.UnsupportedSuffixCount` persists through every snapshot encoding and `receipt` (`internal/contextindex/receipt.go`) adds to `exclusions.count` | `TestReceiptCountsUnsupportedSuffixExclusions` (`internal/contextindex/index_test.go`) over the eager, selective-query, and event-snapshot paths; `TestSectionedSnapshotDecodesEverySectionToTheGobValues` and `TestPackSnapshotDecodesEverySectionToTheGobValues`, whose fixture tracks two `.java` paths; `TestExclusionCountDivergenceAdmitsOnlyALargerCount` (`conformance/cli-parity-v0/runner_test.go`); the 20 `cli-parity-v0` cases replaying `PASS-WITH-KNOWN-DIVERGENCE ... register=DR-0023` byte-exact against their unchanged frozen oracle digests |
| `GPK-V0-059` | `topLevelCommands` and `invalidChoice` in `cmd/corvint/main.go`; shared topics and public inventory in `cmd/corvint/help.go` | `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb` (oracle verbs lead in order, no repeats, every listed verb dispatches, every documented verb is listed), `TestHelpInvocationsAreDeterministicAndDoNotInspectRootOrStdin`, `TestRootHelpListsReleaseAuditCommands` |
| `GPK-V0-062` | `parseHelpInvocation`, `commandHelpTopic`, and `helpFlagRequested` in `cmd/corvint/help.go` | `TestHelpFlagAnywhereFollowsArgparse`, whose help subtest FAILS against the sole-argument parser and whose non-help subtest FAILS against a parser that treats every `--help` token as help |
| `GPK-V0-064` | `argparseOptionLike` classification (`cmd/corvint/help.go`) applied at every next-token value consumption site across `cmd/corvint/*.go` (query, ocm, answerability, prove_checkpoint, calibrate, context_lookup, taskcontext, depsource, dogfood_ocm, dogfood_observe, lrf, necessity, observations, reads, surprise, kernel, migrate_traces, prove, prove_attest_cem, dogfood_record, eval, init_adopt, lease, record; and, by decision 0196, docs, docs_maintain, witness, test_validity, frontier, work, local_completion); `-h` alias handling in `helpFlagRequested`/`parseHelpInvocation`; the `--root` preamble predicate `rootPreambleValue` (`cmd/corvint/help.go`) used by every command detector, `parse()`, and `queryCommandAfterRoots` | `TestQueryTaskValueFollowsArgparseOptionLikeClassification` (`cmd/corvint/query_test.go`), which FAILS against a candidate that treats `--task -x` or `--task --help` as a value; `TestArgparseOptionLikeClassification`, `TestHFlagAliasesHelpFlag`, and `TestRootPreambleFollowsArgparseOptionLikeClassification` (`cmd/corvint/help_test.go`), the last of which FAILS against a candidate that consumes `--root --task` or `--root -h` as a root before any top-level command; `TestCheckpointArgumentDispatchAndExclusivity` (`cmd/corvint/prove_checkpoint_test.go`), whose bare `--checkpoint --task` case now requires the inline `=` form; `TestNativeVerbOptionValuesFollowArgparseOptionLikeClassification` (`cmd/corvint/help_test.go`), which FAILS against a candidate in which any of the seven decision-0196 parsers binds `-x`, `--help`, or `-h` as a value (all seven failed before the guard) |

## Promotion and kill criteria

Promote the Go kernel per supported slice only after every applicable requirement is `PASS` and no
critical `UNKNOWN` remains. Default `corvint` cutover additionally requires the complete CLI,
artifact, rollback, Corvint dogfood, and Beamfall dogfood gates.

Kill or redesign the migration if exact parity requires weakening a receipt, if Go introduces a
daemon/database/network dependency, if the frozen Python grammar cannot be implemented without a
production Python dependency, if any Go-only critical evidence miss occurs, or if valid paired
measurements fail to reduce non-query p95 by at least 50%. In that event the Python product remains
authoritative and independently useful Go protocol components may stay in `interop/**`.

## Unresolved decisions

- public binary signing, provenance attestations, and trusted publishing require a separate
  accepted release contract;
- commercial Corvint Pulse license terms require owner and legal review and do not block the runtime
  migration;
- a future continuous-verification worker may justify a daemon, but this contract deliberately
  supplies no such authority.

## Accepted amendment: general repository-intent CLI query

**Status: accepted 2026-08-31 by repository-owner instruction "accept both"; see
`docs/decisions/0010-query-and-go-archive-acceptance-2026-08-31.md`.** This amendment does not alter
`GPK-V0-028` or accept any other open decision. Its evidence must be added to the traceability and
compatibility ledgers before the slice can be promoted.

- `GPK-V0-043`: `corvint query` MAY add a general `repository`-intent profile on Darwin
  and Linux. The command adapter MUST classify the task with the shared `queryIntent` rules before
  repository access. A `project-operations` task MUST continue through the existing
  `BuildQuery` / `QueryAuthorityStartBudget` authority-start profile with the limits and
  refusals `GPK-V0-028` states after decision 0013. A `repository` task MUST instead build the bounded,
  immutable query index with `BuildEval` and rank it with `EvalQuery`: these are the same index and
  ranking entry points used by `harness event --event user-prompt`, not the authority-start
  entry points. The harness adapter fixes its limit at 10, derives its context budget from the
  harness envelope, and sanitizes the returned context; those adapter steps are not part of
  ranking. The standalone command supplies its requested limit and optional packet budget directly
  to `EvalQuery` and emits the ordinary query context envelope.

  The general profile accepts a non-empty UTF-8 task of at most 8,000 characters (decision 0023); `--limit` defaults
  to 10 and MUST be adapter-enforced in the inclusive range 1--50 before index construction; and
  optional `--budget-bytes` MUST remain in the inclusive range 1,216--1,000,000. `EvalQuery` owns
  record, document, symbol, marker, and learned-history ranking. Its relevance-floor withdrawal
  under `GPK-V0-039`, admitted-result coverage under `GPK-V0-040`, verification, freshness,
  exclusions, uncertainty ordering, stable critical-first packet selection, fallback compaction,
  and twice-stabilized `packet_bytes` / `within_budget` are the command's result and packet
  semantics without a second CLI-specific ranking or packet compiler.

  Symbol ranking treats a task's `v[0-9]+` tokens as a path narrowing, and a narrowing needs
  something to narrow by: a `vN` token MUST narrow the symbol universe only when at least one
  symbol path in that universe (the intent-admitted, non-test symbols the ranker prepares) carries
  some `vN` term, and when no path carries any the tokens MUST be ignored by the narrowing rather
  than empty the universe (decision 0017). Where a versioned path exists, every symbol whose path
  carries none of the task's tokens is still skipped. The Python oracle skips on the tokens
  unconditionally, so on an unversioned tree a task carrying `v18` abstains with
  `no-relevant-candidates` where the candidate answers; that is a `python-defect` under
  `GPK-V0-033`, `src/` is deliberately NOT repaired, and the disagreement is registered as
  `DR-0015` and discriminated by the parity case `query-version-token`.

  Validation MUST fail closed before repository access for an unsupported platform, empty or
  oversized task, and out-of-range or malformed limit or budget. An `agent-tooling` task MUST take
  the same `BuildEval` / `EvalQuery` path as a `repository` task, with the oracle's agent-tooling
  learning task and overlap and the same trace consumption as `GPK-V0-044` (decision 0013 lifted
  the earlier `unsupported-query-intent` and non-ASCII `unsupported-query-task` refusals; the
  codes remain reserved). Repository, authority, stability, resource, and trace failures MUST
  retain their existing typed query errors rather than fall through to partial results. The
  profile remains read-only and local-only, invokes no Python or network service, and grants no
  production cutover.

  When a repository contains an admitted source-language file, no language-specific verification
  command can be derived from its project-owned signals, and the packet would otherwise recommend
  only the signal-free `git diff --check` fallback, `coverage.uncertainty` MUST include the exact
  line `no language-specific verification command is known for this repository`. Documentation-only
  repositories and packets carrying a language-specific or repository-declared gate MUST NOT gain
  that line.

  Acceptance evidence MUST include fresh-process standalone CLI cases for repository tasks at
  limits 1, 10, and 50, the default limit, budgeted and unbudgeted packets, relevance-floor
  withdrawal, malformed and out-of-range inputs, a non-ASCII task and an `agent-tooling` task
  replayed against the oracle, authority-start tasks at the default limit, limit 50, limit 51, a
  non-ASCII task, and a task whose packet carries advisory learned paths, repository drift, read
  nonmutation, and exact Python-oracle bytes wherever this profile does not deliberately retain a
  narrower typed refusal. A structural regression MUST prove both the CLI
  repository profile and harness user-prompt call `BuildEval` / `EvalQuery`, while the
  `project-operations` CLI profile still calls `BuildQuery` / `QueryAuthorityStartBudget`.

## Accepted amendment: standalone repository-query trace state

**Status: accepted 2026-09-01 by explicit repository-owner decision; see
`docs/decisions/0011-standalone-query-trace-state-acceptance-2026-09-01.md`.** This amendment does
not alter the `GPK-V0-028` authority-start profile or accept any other open decision. Its evidence
must be added to the traceability and compatibility ledgers before the slice can be promoted.

- `GPK-V0-044`: The standalone `corvint query` `repository`-intent profile introduced
  by `GPK-V0-043` MUST consume the bounded local trace store with the same learning semantics as
  the shipped Python-oracle harness `user-prompt` query path. This is the primary design because
  that path already treats successful local task traces as advisory learned-path candidates, and
  the native Go `tracerecordrepo.Read` / `trace.Store.Read` path already supplies bounded, stable,
  secret-screened, ancestry-validated reads of the shared trace format. A second standalone trace
  interpretation would be less honest than reusing those shipped contracts.

  On a clean worktree, an absent trace store MUST retain `learning.local_trace_state: "absent"`,
  `learning.local_trace_count: 0`, and `learning.matched_local_traces: 0`. A valid readable store
  MUST set `learning.local_trace_state` to `"ready"`, set `learning.local_trace_count` to the
  complete validated trace count, and set `learning.matched_local_traces` to the number of
  `passed` traces whose task shares at least two query terms. Matching traces contribute only
  `learned-path` candidates for currently indexed paths: changed paths score
  `220 + 20 * matched terms`; opened paths not also changed score `160 + 20 * matched terms`;
  candidates merge by path using the maximum score; evidence authority is `local-task-trace`;
  confidence is `advisory`; and no trace may create or strengthen binding project authority. A
  learned-path candidate's evidence blob always pins to the path's current indexed content, which
  MAY postdate the commit the trace was recorded against; each such evidence item's `reason` MUST
  therefore also name that trace-recorded commit, so a reader can tell the current pin from what the
  trace actually observed rather than have the current blob stand in for it uncontested.
  Learned candidates do not create a packet when no primary result exists, do not contribute
  support to the relevance floor, and at most three fill unused result-limit slots after primary
  results. Primary results MUST clear `GPK-V0-039` independently: a trace may re-rank learned
  candidates only after that admission and MUST NOT convert an otherwise withdrawn packet into a
  publication. Consequently only `learning`, `results`, `state`, `abstention`,
  `coverage`, and `verification` may change as deterministic downstream effects of a present trace
  store; `request`, `revision`, `freshness`, `exclusions`, and `intent` MUST NOT change merely
  because traces are present. `learning.advisory_candidates`, coverage advisory and omission
  counts, the learned-path uncertainty, verification selection, `packet_bytes`, and `within_budget`
  MUST reflect the merged local-trace and Git-history candidate set. The
  `learning.history_tip`, `learning.history_digest`, `learning.history_commits_considered`, and
  `learning.matched_history_commits` fields remain Git-history-only and MUST NOT change merely
  because a trace store is present. A compacted learning block retains exactly `history_tip`,
  `history_digest`, `local_trace_state`, `local_trace_count`, and `advisory_candidates`; it omits
  `matched_local_traces`, `history_commits_considered`, and `matched_history_commits`.

  A mixed worktree MUST retain `learning.local_trace_state: "blocked-mixed-worktree"`, zero local
  trace counts, and no local-trace candidates; the query does not read the trace store in that
  state. Store absence and reads remain nonmutating. An unsafe, unreadable, malformed, unreachable,
  duplicate, oversized, or otherwise statically invalid clean-tree trace store MUST fail closed as
  `unsupported-query-trace-state`, with no partial packet. Any repository identity or worktree
  change, or trace candidate-set or file-byte change during the read, MUST return
  `unsupported-query-drift`. Git-history or ancestry failures not caused by trace-store state retain
  `unsupported-query-history`. Existing repository-profile codes `unsupported-query-platform`,
  `unsupported-query-task`, `unsupported-query-intent`, `unsupported-query-repository`, and
  `unsupported-query-history` remain unchanged; malformed CLI values remain `invalid-arguments`.
  The authority-start-only `unsupported-query-authority` code also remains unchanged.
  `unsupported-query-option` is reserved and no longer emitted: decision 0013 lifted the
  authority-start `--limit` refusal that produced it.

  The rejected alternative is to ignore a present clean-tree trace store and disclose that choice
  through a new `learning.local_trace_state` value. That would be cheaper only at the adapter
  surface: it would discard already-shipped harness learning, make identical `BuildEval` /
  `EvalQuery` repository tasks depend on their caller, and introduce a new packet state rather than
  reuse the bounded Go trace reader.

  This amendment does not widen `project-operations`. That intent MUST continue through
  `BuildQuery` / `QueryAuthorityStartBudget` under `GPK-V0-028`, including its
  `unsupported-query-trace-state` refusal for a present clean-tree trace directory. The existing
  `conformance/cli-parity-v0` case `query-present-trace-store-refusal` currently sends the
  `project-operations` authority-start prompt and therefore MUST remain a rejection. In the same
  commit as implementation, conformance MUST retain that case unchanged and add a
  repository-intent clean-store acceptance case; candidate behavior for the new case changes from
  today's `unsupported-query-trace-state` refusal to exact Python-oracle acceptance. The new matrix
  covers a matching passed trace, a nonmatching or non-passed trace, absent and mixed-worktree
  states, malformed and unreadable stores, drift, packet fields and advisory evidence, budgets,
  and read nonmutation. No conformance fixture or manifest authority changes before owner
  implementation.

- `GPK-V0-047`: `EvalQuery`'s confident-symbol selection has two members that `GPK-V0-043` leaves
  to bare oracle parity and that no case discriminated, so both drifted. This clause states them.

  **Anchor term frequency.** Confident selection narrows the frontier to the paths carrying the
  task's rarest word. That word MUST be chosen against the frequency of every indexed symbol --
  test declarations and intent-excluded declarations included -- and not against the frequency
  across the admitted candidates alone. The two tables disagree wherever a term is concentrated in
  declarations the ranker drops, and only the index-wide one answers the question the anchor asks,
  which is how specific the word is in this repository rather than how common it is among the
  candidates it is about to choose between.

  **Dependency-link promotion.** Where the strongest declaration is written in a web source, the
  declarations it directly calls MUST be resolved -- to declarations in the same file and to
  statically imported ones, breadth first to a depth of two and at most 32 links -- and the two
  strongest of them MUST take the ranks immediately after it, each carrying its relation kind,
  origin selector and depth, and the import and call sites that bind it. Where the task names the
  strongest declaration explicitly and at least one link resolved, the remaining frontier
  declarations MUST be dropped: the caller asked about that declaration, so its call chain answers
  the task and its same-vocabulary neighbours do not. Accepted amendment (decision 0157,
  2026-09-12): the task names the strongest declaration only when that declaration's identifier
  splits into two or more camel, acronym or separator parts and the compacted task contains its
  compacted name. A one-part name such as `kill` is indistinguishable from an ordinary task word,
  so for it the frontier declarations MUST be kept after the promoted links. The oracle drops the
  frontier for a one-part name too; that is a `python-defect` under `GPK-V0-033`, registered as
  `DR-0036`, and `src/` remains unrepaired. End of accepted amendment. Specifier admission and
  target resolution are the reverse-import rules of `GPK-V0-027`, so the alias prefix is the
  project profile's and not the single repository root the oracle hardcodes (`DR-0012`).

  Both were `go-defect` under `GPK-V0-033`: the oracle carries both, the candidate carried neither,
  and the candidate lost a critical `execa` result to the missing promotion. `src/` is unchanged.
- `GPK-V0-048`: Go declaration set. `GPK-V0-031` names a Go-symbol feature compiler and
  `GPK-V0-047` ranks symbols; neither states which Go declarations are symbols, so the two runtimes
  drifted with no clause to decide between them (`DR-0018`). This clause states it. A Go source
  contributes exactly one symbol per name declared at file scope as `go/ast` reports it: every
  `FuncDecl`, and every name of every `ValueSpec` and `TypeSpec` of a `GenDecl`, whether or not the
  declaration is written inside a parenthesized `const (...)`, `var (...)` or `type (...)` group. A
  parenthesized group is a grouping of declarations, not a scope, and MUST NOT change the symbol
  set: `const ( a = 1 )` and `const a = 1` declare the same symbol. Where the file does not parse,
  the lossy scanner's symbols are recorded and the source MUST additionally carry its `Unparsed`
  row, so a parse failure narrows the symbol set visibly rather than silently.

  This is `python-defect` under `GPK-V0-033` and known-divergent: the oracle's per-line
  `_go_symbols` (`src/context_corvint_index.py` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 1058)) emits nothing for a name declared inside a
  group, which contradicts this clause, and the candidate's `go/parser` scan
  (`internal/contextindex/parse.go:295`) satisfies it unchanged. `src/` is NOT repaired: the oracle
  is frozen, and the divergence is recorded rather than closed. It changes no contract metric --
  0/31 critical misses and recall 1.0 hold under both engines -- and moves only non-critical ranked
  selectors on the cobra active-help and Beamfall exact-feature-pairing cases.
- `GPK-V0-049`: Refused-grammar disclosure. `GPK-V0-048` requires a Go source the parser refused to
  carry its `Unparsed` row so that a parse failure narrows the symbol set visibly rather than
  silently. Nothing in that reasoning is specific to Go, and the clause as written left every other
  grammar undetermined. A receipt that drops a source's facts and says nothing has asserted a
  coverage it never measured, which AGENTS.md invariant 2 forbids, and `GPK-V0-014` names
  "silently narrowing Python support" as a parity failure in terms. Every source the index ADMITS
  and counts but extracts no facts from MUST therefore carry an `unparsed` row naming its path, the
  facts withheld, and the refusing grammar -- whatever the language -- and a compiled receipt MUST
  carry the resulting `unparsed` table whenever it is non-empty, subject to the
  `GPK-V0-045` envelope compaction that keeps the count and drops the samples under a budget.

  This is `python-defect` under `GPK-V0-033` and known-divergent: the oracle has no such member at
  all -- `unparsed` does not occur anywhere in `src/` -- so on any repository holding one refused
  source the oracle's receipt claims a coverage it did not measure while the candidate's discloses
  the gap. `src/` is NOT repaired: the oracle is frozen, and the divergence is recorded rather than
  closed. The disagreement is registered as `DR-0021`.

  Its measurement consequence is stated here because it is not obvious: since the member is absent
  from the oracle's envelope entirely, the two receipts can never be byte-equal over a repository
  that holds a refused source, and `GPK-V0-016`'s equal-stdout predicate therefore makes every
  `conformance/perf-v0` task over such a repository invalid by construction rather than merely
  unmeasured. This repository is such a repository: seven `.py` fixtures under
  `internal/pythonsyntax/testdata/ast-parity/` exist precisely to be refused. Measuring across the
  divergence requires a ratified accepted-divergence grant per invocation
  (`docs/decisions/0041-perf-grants-dr0021-corvint-harness-2026-09-03.md`); no clause admits a
  blanket waiver.

## Accepted amendment: ranking corrections

- `GPK-V0-055`: (accepted 2026-09-05 by repository-owner repair instruction, recorded as decision
  0093) `EvalQuery` MUST pass the trimmed, still-cased task into both existing post-tokenization
  pipelines. Relevance terms apply the shared camel/acronym boundary split before Python-compatible
  lowercase, then retain the existing underscore, token, stop-word, stem, alias, and intent filters.
  Ordered relevance-floor words apply that split before lowercase, then retain their existing
  underscore, token, stop-word, order, and deduplication rules without stemming or alias expansion:
  they remain the query words as written under `GPK-V0-039`. Lowercasing used for intent inference
  and other full-text comparisons is unchanged. This applies to standalone `query`, `eval`, and
  `harness event --event user-prompt`. The Python oracle lowercases before splitting and therefore
  loses boundaries such as `stripFinalNewline`; that is a `python-defect` under `GPK-V0-033`,
  registered as `DR-0027`, and `src/` remains unrepaired.
- `GPK-V0-056`: (accepted 2026-09-05 by repository-owner repair instruction, recorded as decision
  0093) path `impact` MUST include same-package `_test.go` files in its existing whole-word scan for
  names declared by the changed non-test `.go` file. A non-twin test-convention row scores 550 plus
  its total matching declaration-name/code-line pairs, capped at 824 so the exact `<stem>_test.go`
  row remains 825 and marker-backed test rows remain 850; equal counts retain the path tie-break.
  The row remains `kind:test`; a convention-ranked row keeps its `test-convention` evidence first
  and appends syntax reference evidence up to the shared evidence cap, while marker-backed
  deduplication preserves the existing `test-marker`, `test-convention`, then syntax evidence order
  under that cap. The test MUST NOT also appear as a `kind:reference` row. The Python oracle
  excludes tests from this scan and leaves non-twin tests flat at 550; that is a `python-defect`
  under `GPK-V0-033`, registered as `DR-0028`, and `src/` remains unrepaired.

The declaration names in that scan are a set, as the oracle's same-package reference scan builds
them: a name the changed file declares more than once (two methods named `String`, repeated `init`)
is one name, so one matching code line is one pair and one evidence item for `kind:test` and
`kind:reference` rows alike, never one per declaration.

## Accepted amendment: top-level invalid-choice list

- `GPK-V0-059`: (accepted 2026-09-12, decision 0092) An unknown top-level command MUST be refused
  with the existing invalid-choice error, exit status, and stream. Its choice list MUST name every
  top-level verb the binary dispatches and every verb root help documents, exactly once. The list
  MUST begin with the pinned migration base's 13 verbs in argparse declaration order (`init`,
  `adopt`, `query`, `feature`, `impact`, `eval`, `lrf`, `record`, `migrate-traces`, `harness`,
  `cem`, `ocm`, `work`), followed by later verbs in root-help order. This keeps the oracle's argparse
  rule that the list enumerates every registered command. A 13-entry list on a binary that
  dispatches more verbs is not a `GPK-V0-001` compatibility requirement. Root help MUST list every
  public verb in that native dispatch inventory. Each listed verb MUST expose a shared help topic:
  `corvint help VERB` and `corvint VERB --help` produce identical stdout, exit 0, do not read
  stdin, and do not inspect the repository. Internal lifecycle entrypoints that are absent from the
  public command inventory remain outside this help contract.

## Accepted amendment: help flag position

- `GPK-V0-062`: (accepted 2026-09-13, decision 0172) Under `GPK-V0-001`, a `--help` token after a
  public command and before `--` MUST print the same stdout as `corvint COMMAND --help`, exit 0,
  read no stdin, and inspect no repository, wherever it appears among the command's arguments,
  following the retired oracle's argparse left-to-right scan. A leading `--help` before the command
  prints root help. `--help` MUST NOT be a help request when it is the pending value of an option
  that takes one (argparse refuses `--task --help` with "expected one argument"; an inline
  `--task=--help` is a value), when it follows `--`, or when a nested `cem`, `ocm`, or `harness`
  choice before it is not a registered action. A negative number or a token containing a space is
  a value, not an option. Positional and value validation MUST NOT precede help, which never
  validates repository input under `GPK-V0-059`. Non-goals: `-h` and option-prefix abbreviations
  remain unsupported. Failure mode: an option missing from the no-value table in
  `cmd/corvint/help.go` makes a later `--help` fall through to the command parser, which refuses it
  with exit 2 rather than printing help.

## Accepted amendment: option value classification and `-h`

- `GPK-V0-064`: (accepted 2026-09-13, decision 0173; root preamble amended by decision 0181) Under `GPK-V0-001`, every `cmd/corvint`
  command parser that consumes "the next token" as an option's value MUST first classify that
  token with the retired oracle's `argparse` rule (`argparseOptionLike`,
  `cmd/corvint/help.go`): a token starting with `-`, longer than one character, that is not a
  negative number and contains no space, is not a value. Such a token MUST instead produce the
  existing `argument --X: expected one argument` refusal, exit 2 — the same Go error path already
  used for a missing trailing value, not a new message. An inline `--opt=-x` form is unaffected: it
  is bound to that token directly and is never subject to this classification. This supersedes
  `GPK-V0-062`'s example of `--task --help` only insofar as it generalizes the refusal to every
  option-taking flag, not only `--help`. `-h` MUST be a full alias for `--help` wherever `--help`
  is accepted (root, every top-level command, and nested `cem`/`ocm`/`harness` choices), with
  byte-identical output and exit code — this supersedes `GPK-V0-062`'s `-h` non-goal.
  `GPK-V0-062`'s option-prefix-abbreviation non-goal is UNCHANGED: abbreviations (`--he`, `--ta`)
  remain refused, now adjudicated as an intentional divergence (`DR-0037`,
  `conformance/divergence-register.md`) rather than an open gap, because accepting them would make
  adding a future option sharing that prefix a breaking change for any script relying on the
  shorter form. The `[--root PATH]` preamble ahead of every command verb follows the same
  classification through one shared next-token predicate, `rootPreambleValue`, used by every
  preamble scan (each command detector, `parse()`, and `queryCommandAfterRoots`): a bare `--root` whose
  next token is missing or option-like MUST be refused with the existing `missing value for --root`
  message, exit 2, before any command or `--help` is dispatched. `--root=VALUE` (including
  `--root=-x`), non-option-like values, and a literal `--` after `--root` stay root values,
  unchanged. Amended by decision 0196: the classification also binds the native verbs the retired
  oracle never had (`docs`, `docs maintain`, `witness`, `test-validity`, `frontier`, `work
  propose-wave`, and the `dogfood` local-completion actions). Each refuses an option-like next
  token through that parser's own existing missing-trailing-value error, unchanged (for example
  `missing value for --base`, `invalid-frontier-input`, `local-completion-option-value-required`),
  since those verbs never had an argparse message to reproduce.
  Failure mode: a parser whose value-consuming loop is not routed through `argparseOptionLike`
  keeps accepting an option-like token as a value, silently diverging from the oracle again.

## Accepted amendment: untracked-path allowance

- `GPK-V0-060`: (accepted 2026-09-12, decision 0142; case-folded by decision 0159) `GPK-V0-030`
  range impact MUST tolerate a dirty worktree only when every
  `git status --porcelain=v1 -z --untracked-files=all` entry is an untracked path that rule
  `corvint-untracked-allowance/1` proves disjoint from the Go build over the captured tree. A path
  overlaps when it is a directory entry, ends in `.go`, has a basename that case-folds equal to
  `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `.gitignore`, or `.gitattributes`, has a path
  segment that case-folds equal to `vendor`, or lies at or below a directory holding a tracked,
  indexed, or excluded `.go` path. The case fold applies unconditionally on every host, including
  a case-sensitive one, where it only loses an allowance rather than admitting an unproven one:
  on a case-insensitive filesystem an untracked path differing from a control name only in case is
  the same directory entry Go and Git resolve (decision 0159). Ignored paths are never listed by
  status and need no classification. With no allowed path, the receipt MUST
  keep `range.status: "CLEAN"` and omit `range.untrackedAllowance`. Otherwise it MUST set
  `range.status: "UNTRACKED-ALLOWED"` and bind `range.untrackedAllowance` as `rule`, `count`, and
  `sha256: "sha256:<hex>"`, where the digest is SHA-256 over the rule, NUL, then each sorted allowed
  path followed by NUL. `range.statusSha256` continues to bind the raw status bytes. Failure modes:
  any tracked entry (modified, staged, deleted, renamed, or copied) or malformed status MUST return
  `unsupported-impact-worktree` with "range impact requires a clean worktree"
  (`internal/contextindex/range_impact.go:324`). The first overlapping untracked path MUST return
  `unsupported-impact-worktree` with "range impact requires a clean worktree; untracked path
  overlaps the Go build: <path>" (`internal/contextindex/range_impact.go:341`). Any status change
  after capture remains `impact-range-drift`. The allowance MUST NOT widen path or working-tree
  impact, which are unchanged. It does not extend to the repository gate the receipt names, which
  keeps refusing listed untracked paths under `ARTIFACT-GO-V0-009`.
- `GPK-V0-061`: (accepted 2026-09-12, decision 0142) The `GPK-V0-060` allowance MUST NOT apply to
  learned-trace reads. Any dirty path, untracked ones included, MUST keep
  `learning.local_trace_state: "blocked-mixed-worktree"` under `GPK-V0-044` and MUST NOT read the
  trace store (`internal/tracerecordrepo/read.go:82`). Trace records name opened and changed paths
  across historical revisions, so disjointness cannot be proven without the read this state forbids. A tree-bounded
  variant is declined (decision 0171): its replay window is named by that read or by the full
  bounded ancestry, and its binding would diverge from the Python-oracle query packet.

## Accepted amendment: suffix exclusions in the receipt count

- `GPK-V0-063`: (accepted 2026-09-12, decision 0166; registers `DR-0023`) A context receipt's
  `exclusions.count` MUST count every tracked path the index did not read. That includes each path
  `admittedEntries` (`internal/contextindex/index.go`) skips because no allow-listed suffix admits
  it, not only the paths that carry an exclusion row. A skipped-suffix path adds no
  `exclusions.samples` entry, so the samples and every other receipt member are unchanged. Under
  invariant 2 a count that omits unread paths asserts coverage the index never had. The count is
  computed once per build and persisted with the index (`Index.UnsupportedSuffixCount`, carried by
  the gob, event, sectioned, and pack encodings), because event loads keep no tracked-path table
  to recount from. It binds every surface that renders `receipt` (`internal/contextindex/receipt.go`):
  `query`, `feature`, `impact`, `range impact`, and the `harness` context blocks. The Python oracle
  counts exclusion rows only. That is a `python-defect` registered as `DR-0023` under `GPK-V0-033`,
  declared per case as one `exclusions.count` rewrite, and `src/` is deliberately unrepaired.

## Accepted amendment: unsupported records yield to supported symbols

- `GPK-V0-066`: (accepted 2026-09-25, decision 0387; V1-0260)
  Records and documents precede confident symbols in a query packet only as answers to the task,
  and `EvalQuery` admits confident symbols as primary results only when no record or document
  matched. A record or document packet whose emitted results fail the `GPK-V0-039` floor answers
  nothing, so before withdrawing it the packet MUST be compiled as though no record or document
  matched: the confident-symbol selection alone, followed by learned paths as usual, judged by the
  same floor over its own emitted results. It is published only when some emitted symbol rests on
  the words `GPK-V0-039` requires; otherwise the withdrawal stands with reason
  `below-relevance-floor`. The substituted packet carries no record tie `NEEDS_WIDENING` state and
  no nearest negative claim, because the withdrawn records are not in it. A packet that already
  clears the floor is unchanged, so a one-word record at a wide limit still precedes the symbols;
  this clause changes only packets `GPK-V0-039` would withdraw. Rollback: remove the substitution
  in `evalQuery`, restoring the withdrawal.

## Proposed amendment: a limit that omits a competing record needs widening

- `GPK-V0-068`: (accepted 2026-09-25, decision 0396; panel finding M2)
  A `query` packet claims `READY` only when the ranking chose between the task's readings on
  evidence, and `GPK-V0-039` rules out score as that evidence. When the result limit omits a
  competitive record, one the ranking admitted before the limit cut, that rests on a word of the
  query as written that no emitted result rests on, the packet MUST carry `state`
  `NEEDS_WIDENING` and an active abstention with reason `omitted-competing-record`. It keeps its
  emitted results. Support is counted as `GPK-V0-039` counts it: per result, in the query's own
  words. A limit that admits every such record leaves the packet unchanged. A packet that
  `GPK-V0-066` compiled from symbols carries no such state, because its records were withdrawn.
  A nearest negative claim stays the named reason when both apply. Rollback: remove the
  `evalOmitsCompetingRecord` check in `evalQuery` and restore the analyzer schema to
  `corvint-analyzer/84`.

## Proposed amendment: workspace package importers are disclosed, not silently omitted

- `GPK-V0-069`: (accepted 2026-09-25, decision 0399; panel blocker B3)
  Rule (c) of `GPK-V0-027` resolves relative and alias specifiers only, so a source that reaches a
  changed web path through a bare workspace package specifier (for example `@scope/contracts`,
  then a barrel re-export) is never found, and the packet reported no uncertainty over that answer.
  For each requested, indexed, non-test changed path with a rule (c) suffix, the holding package is
  the nearest directory below the repository root with an indexed `package.json`; when some indexed
  source's import specifier equals that manifest's `name` or begins with `name/`, or when the
  manifest's `name` cannot be read, the `impact` packet MUST add
  `reverse-import results for N changed paths importable by workspace package name are unresolved`
  to `coverage.uncertainty`, with N the count of such paths. A manifest that parses without a
  `name`, a path under no nested manifest, and a package no specifier names add nothing. This
  disclosure resolves no importer and grants no rule; resolving workspace names, package entry
  points, `exports`, `tsconfig` `paths` and barrel re-exports remains future work that would
  retire the line for the paths it resolves. Evidence:
  `TestImpactDisclosesWorkspacePackageImporters`. Rollback: remove `webWorkspaceImportGap` from
  `setCoverage`, restoring the undisclosed packet.

## Proposed amendment: markers come from comments and relate to the change

- `GPK-V0-070`: (proposed 2026-09-25, panel blocker B4, not accepted; V1-0263)
  A `feature:` or `scenario:` marker is project-authority evidence (`source-marker`,
  `test-marker`), so it MUST come only from a comment span of a source whose suffix has comment
  syntax: `//` and `/* */` for Go, `go.mod`, JavaScript, TypeScript, Rust, Swift, C#, Kotlin and
  Objective-C; `#` for Python, TOML, shell, YAML and Ruby; `--` and `/* */` for SQL; and `<!-- -->`
  for Markdown and MDX. Text inside a string literal (quoted, raw, template or triple-quoted) and
  every line of a data or prose file without comment syntax (`.json`, `.txt`, `.rst`) yields no
  marker. A comment marker keeps its line and byte column. Path `impact` credits a same-package
  marked test (the 850 row of `GPK-V0-056`, and its keys as related feature and scenario records)
  only when the marker relates to the change: the test is the changed file's exact
  `<stem>_test.go` twin, it references a name the changed file declares (the `GPK-V0-056` scan), or
  the changed file carries the same marker key. When the ranked list exceeds the limit, `impact`
  keeps `limit/10` rows (none below limit 10) for cross-package callers, meaning non-test reverse
  importers whose code names an exported declaration of a changed Go file as `alias.Name`. Callers
  already inside the limit count toward that quota; a moved caller displaces the lowest included
  rows and never a requested path row. No score changes. The analyzer schema moves to
  `corvint-analyzer/85` because cached blob facts carry markers. The retired oracle scanned raw
  lines; no frozen parity case discriminates the two, and with the oracle retired (decision 0088)
  no register entry is opened. Rollback: restore the raw-line scan in `index.go`, the unconditional
  same-package marker credit and the unreserved tail in `impact.go`, and the analyzer schema to
  `corvint-analyzer/84`.

## Proposed amendment: repositories owned by another user

- `GPK-V0-071`: (proposed 2026-09-25, not accepted; V1-0288) Every Git subprocess nulls global and
  system configuration (`GPK-V0-009`), so Git's `safe.directory` exception can never be honoured and
  a checkout owned by another user (a bind mount, a container volume, a shared CI workspace) fails
  every Core verb with Git's uncoded `detected dubious ownership` exit 128, whose printed advice
  (`git config --global --add safe.directory`) cannot work under Corvint. The kernel MUST classify
  that exit as one coded refusal, the same code from every verb, whose message names the
  repository path and a remedy that works under the sanitized environment. Owner decision needed
  before implementation: whether the remedy is only "run as the owning user or change ownership",
  or whether an explicit operator opt-in passes `-c safe.directory=<worktree>` and
  `-c safe.directory=<common Git directory>` per invocation. The opt-in bypasses the protection
  Git added against configuration planted by another user, so it is acceptable only if every Git
  subprocess on that path also neutralizes repository-configured filters, hooks, fsmonitor and
  textconv, which is not true of every env builder today. The fix touches the roughly thirty
  per-capability env builders, including `cmd/corvint` files that PR #218 (decision 0393) also
  changes, so it lands after that PR. Rollback: remove the classification, restoring Git's exit.

## Proposed amendment: non-UTF-8 paths in query history

- `GPK-V0-072`: (proposed 2026-09-25, not accepted; V1-0313) `query --task` and `prove --task`
  refuse the whole repository with `unsupported-query-history` when any commit in the bounded
  history names a path whose Git bytes are not UTF-8 (`parseHistory` in
  `internal/contextindex/history.go`), although decision 0394 made such a path an index exclusion
  everywhere else. Proposed: query learning leaves such a path out of its commit's path list, so
  it contributes to no history match, no `history_digest` input, and no candidate, and the
  commit's other paths answer as before; `unsupported-query-history` stays the code for every
  other history failure. No existing digest moves, because every repository this changes is one
  that refuses today. The Python oracle still refuses, so the change is a `python-defect` under
  `GPK-V0-033`, registered in `conformance/divergence-register.md` with one declared oracle-only
  case, and `src/` is not repaired. Owner decision needed: accept the skip (implemented on the
  `skipNonUTF8` seam that PR #219 adds to `parseHistory`, so it lands after that PR) or keep the
  refusal and close V1-0313. Rollback: pass `false` on the query path, restoring the refusal.

## Accepted amendment: omitted direct Go callers are disclosed and importing-test markers are named

- `GPK-V0-075`: (accepted 2026-09-25, decision 0412; V1-0263; proposed in PR #247 as `GPK-V0-068`,
  renumbered because decision 0396 accepted another clause under that ID) This amends the Go
  reverse-import rule and the related-record reason of `GPK-V0-027`. It changes no score, order,
  or row, and it keeps the rejected `GPK-V0-067` re-scoring rejected.
  (a) A direct Go caller is a non-test `.go` reverse importer of a changed `.go` file outside the
  module root package, itself not a changed path, one of whose code lines other than the import
  line names an exported declaration of the changed file through the importer's local name for the
  package: `pkg.Name` with the package clause name, or `alias.Name` when the import binds an alias.
  A blank or dot import names nothing. When the result limit drops a direct Go caller's
  `kind:reverse-import` row from the final ranked order (after any caller reservation
  `GPK-V0-070` makes), `coverage.uncertainty` MUST carry, after every existing line and in rank
  order, `direct Go caller PATH (line N names QUALIFIER.NAME declared by CHANGED) ranked R as a
  S-score reverse-import row and was omitted by result limit L`, naming the first such code line
  for the first changed path in sorted order. At most ten callers are named; any further ones are
  counted in one line, `K more direct Go callers omitted by result limit L`. The budgeted impact
  packet (`EvalImpact`, and so `harness event --event file-change`) carries the same lines.
  (b) A feature or scenario record admitted at 800 keeps the canonical-ledger evidence reason
  `changed path carries KIND:ID` only when a changed path itself carries that marker or when no
  reverse-importing test admitted it. A record a reverse-importing test's marker admits, and no
  changed path carries, MUST instead give `test TEST importing changed CHANGED carries KIND:ID`,
  naming the first such test in reverse-importer order for the first changed path in sorted
  order. Its score stays 800.
  In a Beamfall path impact at limit 10, two feature records admitted only through reverse-
  importing tests' markers (800, each claiming "changed path carries") and same-package references
  (775) filled the limit, and the direct caller's 700 row ranked 30th and was dropped with only a
  count. A root-package changed path is out of scope for (a), because `DR-0017` and the
  broad-module-root reservation govern its importers. The retired oracle emits neither the
  disclosure nor the importing-test reason; that divergence is `DR-0042`, which `GPK-V0-033`
  classes as a `python-defect`. The analyzer schema moves to `corvint-analyzer/86`. Rollback:
  revert the change, which removes the disclosure and the importing-test reason and restores
  `corvint-analyzer/85`.
