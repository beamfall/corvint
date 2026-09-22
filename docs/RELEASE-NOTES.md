# Release notes

## 0.5.0a3 experimental alpha

`v0.5.0a3` integrates every GitHub issue open after `v0.5.0a2` (decision 0329): #53 behavior-contract
provider records and variation reconciliation frontiers, #54 criterion-level falsification controls
for browser behavior contracts, #55 OpenCode file-change events under bursts and unadmitted
suffixes, #56 redaction and rejection of sensitive input values in browser runtime evidence, and
#57 authoritative affected-test planning from explicit dirty-worktree snapshots. It also carries
`main`'s gate ledger, which records one full-gate pass per distinct content. It retains `v0.5.0a2`'s
unsigned prerelease, four non-Windows CLI archives and no-promotion boundaries; no companion is
attached because no exact-candidate companion qualification was run.
The candidate also fixes thirteen defects found by a pre-release audit: `dogfood-ocm`,
`taskman-fixture` and `corpus` now report output-write failures with an error envelope, and a
`--root` preamble refuses an option-like value; context-index authority citations bound a cited
line range to the target's length, a shard writer closes its file once, and a truncated symbol
window table is refused; sensitive-input redaction applies longer values first so a shorter
sensitive value that prefixes a longer one no longer leaks its tail, Vitest reports and external
config inputs are read under fixed bounds, the readiness probe honours cancellation, and the
qualified reporter tolerates a moved cached module; behavior falsification counts only executed
controls toward complete vocabulary, frames workspace digests by file length, gives every receipt at
least the validator's accepted size, and refuses a wall-clock budget inside the cleanup reserve;
the documentation-corpus behavior adapter separates reverse-link keys with NUL (which now also
appears in `lost_reverse_links` strings), names the test in each `missing-reverse-link`
diagnostic, surfaces an encode error as a digest mismatch, and emits only bundles its own validator
accepts. The analyzer schema moves to `corvint-analyzer/73`.

No retrieval, host, editor or command is promoted by this version change. Native performance remains
unmeasured, publisher identity remains `NOT_VERIFIED`, live Playwright `/2` and external MCP host
qualification remain `NOT_RUN`, and local qualification does not substitute for hosted CI. Exact
availability and platform claims come only from this version's attached assets and qualification
evidence; the detailed active scope is in `docs/RELEASE-NOTES-alpha.md`.

## 0.5.0a2 experimental alpha

The owner selected `v0.5.0a2` for the issue #49 release (decision 0328). It retains `v0.5.0a1`'s
unsigned prerelease, four non-Windows CLI archives, separately qualified macOS arm64 companion and
no-promotion boundaries. The companion carries the `corvint-js-test-provider` fix and regression for
ordinary Playwright `devices['Desktop Chrome']` project spreads while preserving issue #50's stricter
bundled-browser revision, manifest, path and executable-digest qualification.

No retrieval, host, editor or command is promoted by this version change. Native performance remains
unmeasured, publisher identity remains `NOT_VERIFIED`, and local qualification does not substitute
for hosted CI. Exact availability and platform claims come only from this version's attached assets
and qualification evidence; the detailed active scope is in `docs/RELEASE-NOTES-alpha.md`.

## 0.5.0a1 experimental alpha

The owner selected `v0.5.0a1` for the integrated candidate (decision 0327), superseding the
unperformed `v0.4.0a4` publication target. It remains an unsigned prerelease with publisher identity
`NOT_VERIFIED`, four non-Windows CLI archives and only a separately qualified optional companion.
Historical entries below retain their original versions; no feature or retrieval promotion follows.

This public prerelease candidate uses the native Go engine. The Python engine, live oracle and
wheel are retired under decision 0088. The owner cancelled the paired Python performance retry;
that unrun experiment is not a release prerequisite or a successful measurement. No Rust runtime
is included. Versioned CLI archives and the optional workflow bundle are usable only where the
exact release's attached artifact and installed qualification evidence says they were tested.

The optional macOS arm64 workflow bundle (`corvint-companion-bundle/2`) contains nine Go tools and
five raw agent-host package trees (Codex, Claude Code, Gemini CLI, OpenCode, Pi); every host tree
is labelled `FALLBACK` and none is qualified `FULL`. The VS Code extension is deferred and is not
in the bundle. Other workflow-bundle platforms and Windows CLI shipment remain outside this
release set. Dashboard, ticket-management and live test integration remain experimental
requirements under `docs/specs/public-release-v0.md`.
The task manager's autonomous dispatcher and process runtime remain unavailable.

Retrieval (`query`, `context`, learned task traces) is experimental and unqualified. The last
held-out qualification attempt, blind-v6 on 2026-09-17 under decision 0307's competition-relative
alpha bar, ran on the separate retrieval cycle line (`c31d928e`, not integrated here) and failed:
top-5 task success 0.571 against the exact-search baseline 0.343, abstention 5/10 and mean latency
7.19 s met the bar, but 7 of 36 must-exclude checks were hit. No sealed held-out partition remains.
The engine in this release has no held-out retrieval qualification of its own; ranking and
abstention are not a release claim.

The default CEM profile trusts the local Git executable and object database plus a stable
same-user operating-system boundary. It does not universally authenticate hostile object-store
mutation; the separately qualified protected profile must not be presented as a default guarantee.
Performance, FULL integration and workflow promotion remain separate evidence claims. Local checks
do not constitute a successful hosted CI run; consult the tagged revision's hosted workflow record.

Checksums verify integrity, not publisher identity. Publication status comes from the versioned
release and its owner receipt; this document does not assert that publication occurred.

The per-surface PUB-V0-007 detail (experimental features, qualified platforms, trust boundary,
performance and hosted CI) is in `docs/RELEASE-NOTES-alpha.md`.

### Verification

Go-only commands that apply to this candidate; no wheel, Python oracle, or Rust check exists
(decision 0088; `tools/release_artifact.py` and `requirements/release-build.txt` were removed by
`54735d98`, 2026-09-11):

```sh
test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.1"
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./...
GOTOOLCHAIN=local go vet ./...
(cd interop/cem01-go && GOTOOLCHAIN=local go test -count=1 -timeout 30m ./... && GOTOOLCHAIN=local go vet ./...)
```

```sh
PATH=/opt/homebrew/Cellar/go/1.27.1/bin:$PATH GOTOOLCHAIN=local make go-archive-gate
script/release-checklist
```

`make go-archive-gate` (`script/go-archive-gate`, ~6 minutes, needs a clean tree and the pinned
`go1.27.1` toolchain) hermetically builds and verifies the five release archives plus
`SHA256SUMS` and `verification-report.json` (`docs/specs/go-archive-gate-v0.md`,
`ARTIFACT-GO-V0-001..007`). `script/release-checklist` is read-only (it never builds, tags, signs,
publishes, or promotes) and reports `native-runtime`, `native-performance`, `go-archive`, `tag`,
`publication`, and `promotion` rows; it carries no `wheel-integrity` or `packet-5` row for this
candidate (`docs/specs/release-artifact-integrity-v0.md`).

## Development history since v0.4.0a3

The following records describe earlier development and measurements, not fresh `0.4.0a4`
qualification. Original revision and version identities are retained. Decision 0088 supersedes
Python/wheel and Packet 5 retry prerequisites below.

### What shipped

- Packet 5 retry preregistration and the checklist row that can bind it (decision
  `0085-packet-5-retry-preregistration-2026-09-11.md`): `conformance/perf-v0` admits scoped
  `corvint-perf-v0/2` manifests by raw digest, refuses overrides for them, and writes
  `corvint-perf-report-v0/2` receipts with `slicePerformanceStatus`; cold-corpus index-building
  and query rows are `REPORTED` beside the binding snapshot-present rows (decisions 0048 item 1
  and 0084); four snapshot-present grants are ratified for the Corvint mirrors;
  `script/release-checklist` binds a formal result to `HEAD` by source identity
  (`ARTIFACT-RDY-V0-002` amended) instead of a candidate revision no commit can equal.
- **Expert-panel decisions 0057 to 0064** (`docs/decisions/0057-*.md` to `0064-*.md`, memos under
  `docs/reviews/`): unruled-suffix impact is disclosed rather than refused; the learning loop gets
  an evaluation gate, retention and a wider secret screen (decision 0058); the extension surface is
  embedded data with an exact adapter bound; the parity runner isolates per case; Windows is a
  deferred target; the live test provider sits behind a containment interlock (`GLTP-V0-044`);
  vocabulary rows carry a citation check in `prove`; the add-on portfolio is sequenced after this
  tag with one positioning falsifier.
- **Documentation as searchable evidence** (decision 0065): documentation suffixes are indexed,
  placed after the fifth code row (`TCP-V0-013`), and LFS pointers are excluded.
- **`context` ranking**: the lexical slot ranks by BM25 over body, path and whole-identifier
  fields (`TCP-V0-014`, decision 0066 accepted); one query-time `test` row links tests and code
  (`TCP-V0-015`, decision 0067 proposed, its trace2code loss disclosed); the packet withholds its
  rows on an unsupported conjunction of the task's names and reports `coverage.answerability`
  (`TCP-V0-016`, decision 0068 proposed); read-only `context defs|refs|grep` lookups over the
  snapshot (`TCP-V0-017` proposed); an opt-in ranking recipe behind `CORVINT_CONTEXT_RECIPE`
  (`TCP-V0-018` proposed) whose `r` and `r+anchor` variants are falsified and whose code-first
  variant awaits a held-out run (decision 0071). The default packet order is unchanged by the
  proposed clauses.
- **Index storage**: the Index Snapshot spec is accepted with automatic refresh (decision 0049,
  `IDX-SNAP-V0-011..012`); an opt-in sectioned snapshot (`IDX-SNAP-V0-014`) and the zero-copy
  Corvint Pack with block verification and a co-change section (`IDX-SNAP-V0-015`,
  `CORVINT_SNAPSHOT_FORMAT=pack`) are prototypes; decision 0069 makes the pack the storage direction.
  Harness events can share one repository observation (`GPK-V0-058`,
  `CORVINT_HARNESS_SHARED_OBSERVATION=1`); a snapshot miss reuses the loader's observation.
- **Measurement**: the retrieval bench reports a baseline ladder (`grep`, `grep-ident`, `bm25`
  over `all` and `ident` terms, `context`), paired bootstrap intervals, repository folds,
  registration digests and per-arm latency; decision 0070 makes that the promotion gate for
  ranking lanes and declares the v2 positives development data.
- **Impact and record**: copy and rename range members and a changed `.gitignore` are admitted
  (decision 0050, `GPK-V0-049..050`); the tail reserves test-convention evidence; range refresh
  per decision 0049.
- **`prove`**: a replayable mutation-witness prototype with offline DSSE verification; the
  syntax-row falsifier and drift gate (decision 0063).
- **Platform and adapters**: every host hook fits its deadline and agrees on the prompt bound;
  injected repository data is framed as untrusted on every host; process groups own their umask
  and reap hook groups; quiescence proofs pin the leader's start-time identity; cold start fails
  closed on unknown TypeScript runners.
- **Gate**: every `path:line` citation in `docs/specs` and `docs/agent-memory` is checked; a
  requirement id that survives only as a table row fails the gate; analyzer permutation gates are
  sharded; the seven-command `GPK-V0-017(d)` manifest is preregistered and measured once.

### Wave 14-15 (2026-09-11/12): security hardening, agentic-completeness verbs, and public-alpha wiring

This range's ~760 unmerged commits (integration branch `integration-lanes-2026-09-12`) are
dominated by the public-alpha push. Security-relevant fixes, new CLI surface, and the decisions
that govern them are grouped below; narrower internal repairs are in `docs/BUILD-LOG.md`.

**Security hardening (descriptor-safe reads, symlink/FIFO refusal, `.git` case-folding):**

- Bounded CLI file reads (`FPK-V0-012/021/024/031`) now lstat the path, open no-follow and
  nonblocking, and re-check the opened descriptor's identity with `fstat` before any content read,
  so a FIFO or a symlink swapped in after the check is refused rather than blocking or misread.
- The test-validity MCP bridge and CLI reader (`MTV-V0-003/006/007`, `LPCV-V0-051`) adopt the same
  no-follow/nonblocking/pinned-parent-descent pattern, closing an outside-read gap reachable
  through a leaf or parent symlink swapped in after the existing same-file check but before `fstat`.
- The test-validity MCP bridge refuses any path segment equal to `.git` under case folding
  (`MTV-V0-003`): on a case-insensitive Darwin volume, a `.GIT/HEAD` receipt argument previously
  passed the case-sensitive segment rule and read `.git` content.
- `internal/projectpath` now proves symlink containment for an absent path suffix (resolving the
  root, the nearest existing ancestor, and each missing component's dangling-link chain) instead of
  an unsafe lexical fallback; this closes the same gap in both the native host adapter and the
  unplanned-read classifier (`AHI-014`, `URE-V0-001`).
- Depsource's external reads (module downloads, ziphash verification) enforce their declared memory
  bounds: Git stdout is capped at 8 MiB during command execution, and ziphash records are verified
  through a 1025-byte limiter that maps an over-cap record to the existing abstention
  (`DSE-V0-002/004/009`).
- The analyzer source traversal under the Darwin sandbox (`ACC-V0-002`) no longer leaks a file
  descriptor on a successful multi-level traversal or on a failed first-component `Lstat`; both
  paths now close explicitly rather than relying on process exit.

**Bug fixes:**

- OCM requirement enumeration is fence-aware (`OCM-V0-001`): a `## ` line inside a fenced code
  block no longer ends the Requirements section, and a bare `##` does. Comment masking now tells a
  JavaScript regex literal from division (`OCM-V0-005`), so a quote inside `/'/` cannot unmask a
  later commented-out test as a live claim.
- Work-source Git/pipe-drain timeouts are now finite hang detectors (10 minutes / 1 minute) and
  both expiry causes map to `INPUT_LIMIT`; previously a pipe-drain wait-delay error was
  misclassified as `SOURCE_UNQUALIFIED` (`WQO-V0-015/032`).
- `WQO-V0-042` native-index Go import resolution now walks every tracked `go.mod`, selects the
  longest module-path prefix, and refuses external, ambiguous, or cross-nested-module imports
  instead of inventing repository neighbours for a work-queue collision.
- CEM's cumulative Git-command deadline is now an explicit 30-minute hang detector rather than a
  fixed ceiling that could fail a slow-but-legitimate large CEM under host load (`GPK-V0-003`,
  `CEM-GO-003`).
- `dogfood event` no longer blocks past its own deadline waiting on an uncancellable in-memory
  index compile after a snapshot miss; it now aborts and reports `dogfood-event-deadline`
  (`LCP-V0-008`).
- Host adapter deadlines for Claude Code, Codex, and Gemini CLI are now derived from each host's
  declared kill timeout minus a fixed reserve, with a Go-side watchdog, so a slow hook call is
  reported by Corvint before the host force-kills it rather than after (`AHI-017/018`).
- `LookupTreeEntry` and `CanonicalDiff` (`internal/cem/gitauth`) now re-hash every tree and blob
  body they read from Git against the identity they read it under, closing a substitution gap where
  a mislabeled nested tree or diff blob was previously accepted (`CEM-CB-023/024`); Git's own
  `100664`-to-`100644` mode normalization is preserved separately for verified raw tree reads
  (decision 0126).
- `script/go-archive-gate` removes any pre-existing private build witness before building, so a
  failed witness write can no longer let a stale witness from a prior run pass the gate
  (`GAG-V0-007`).
- Answerability's `supported` marker now names the first three best-supporting indexed sources and
  their exact `supports`/`lacks` sets instead of an unsupported bare claim (`TCP-V0-016`, decision
  0146, amending decision 0068).
- The `context` task-context packet now reports `subject.evidence_gap` when the index could not pin
  a subject's `blob_hash`, instead of silently omitting the field (`TCP-V0-005`).
- Five scoped bug-hunt fixes: Go `//line` directives are now ignored when computing physical
  change-witness spans (`CWR-V0-014`); a JSON decoder that admitted a truncated semantic proposal
  document past its container-continuation check now requires a complete value (`SEG-011`); an
  integer-overflow and negative-cost bypass in semantic-escalation cost budgeting is closed
  (`SEG-015`); returned candidate slices are now cloned so caller mutation cannot corrupt the
  cached derivation ledger (`SEG-017`); and observation-ledger repair now drops an unterminated
  tail row without discarding the successfully appended event ahead of it (`SOL-V0-003`).

**New and changed CLI / wire contracts:**

- `corvint test-validity [--receipt FILE]` is a new experimental verb presenting Go and JavaScript
  test-run and per-test results through one shared projection (`LPCV-V0-051`); it also now accepts
  a completed Go live-session-event document, not only a JavaScript provider document, and refuses
  an unknown or ambiguous receipt kind with `invalid-test-validity-receipt`.
- Eight new experimental verbs ship under decision 0089: `depsource`, `lease`, `calibrate`,
  `necessity`, `answerability`, `surprise`, `reads`, and `kernel`. All are intent `proposed` /
  delivery `experimental`, and none is consumed by an existing capability; two harness call sites
  (`unplannedread.HookPostTool`, `compactionkernel.InjectionBlock`) were later wired behind
  operator opt-ins only (decision 0099). A prototype `symbol` verb from the same assessment was
  deleted rather than shipped, because `corvint context defs|refs|grep` already answers the same
  question with blob-pinned evidence.
- The unaccepted `lane-plan` command is retired: its interception, help entry, command inventory
  row, CLI implementation, tests, and the proposed Lane Plan V0 spec are removed (decision 0156);
  `work observe` and `work propose-wave` are unaffected, and the removed `LPL-V0-001..016` IDs are
  historical, not accepted obligations.
- Root and per-topic CLI help now lists `adapter`, `dogfood-ocm`, and `witness` (previously absent
  from help and the invalid-choice inventory), and `dogfood-ocm` argument-parsing failures use the
  shared argument-error renderer, exiting 2 with `invalid-arguments` (`GPK-V0-059`).
- A separate `corvint-docs-mcp` stdio server ships automatic docs draft/consume (`SDD-V0-006`); the
  frozen three-tool `corvint-mcp` profile (decision 0103) is untouched, and the test-validity
  projection reaches MCP through its own descendant profile rather than widening that frozen one
  (decision 0147).
- Local Git-status exclude handling is confirmed unchanged behavior, not new policy (decision
  0158): no wire, receipt, or digest change.
- Range impact gains a scoped untracked-path allowance (decision 0142).

**Public alpha and release readiness:**

- At the 2026-09-12 preparation snapshot, decision 0107 had chosen the optional companion's
  `corvint-taskman` license (AGPL-3.0-or-later), while the separate repository still lacked committed
  `LICENSE`/`PROVENANCE.md`; the companion gate therefore refused at the notices step in that snapshot.
- Decision 0108: the `0.4.0a4` alpha archives ship unsigned; `SHA256SUMS` and the double build
  prove integrity and reproducibility only, not publisher identity.
- Decision 0109 sets the alpha's publication destination (a GitHub prerelease at `v0.4.0a4`), the
  post-publication receipt's shape, a no-promotion rule for this alpha, and the readiness bar
  (native-performance, publication, and promotion staying `NOT_RUN` by design).
- Decision 0141 resolves a conflict decision 0109 left open: the publication receipt binds to the
  tagged revision, not `HEAD`.
- The companion install smoke path (`internal/companionrelease/smoke.go`) is exercised end to end
  against real binaries for the first time; two defects that would have hit the first licensed
  bundle are fixed: a fixed create payload predating `atm`'s `supersedes` field, and a fixture
  repository with no commit that `corvint-dashboard-snapshot snapshot --root` refuses without `HEAD`.
- Dashboard, roadmap, and task-manager integration slices (IPR-01 through IPR-10) land under an
  execution-disabled planning store; independent reviews found and repaired a session data race, a
  JS test-provider E2E path that reported pass on an empty report, dashboard rendering that showed
  a dirty worktree as current, and a docs-maintenance gap where a symlinked page or spoofed marker
  text could be read as the managed block. None of these slices claims qualification.

**Darwin sandbox:**

- Decision 0145: the Darwin analyzer sandbox profile gains exactly one allowance, listing the root
  directory's entries (`(allow file-read-data (literal "/"))`), fixing a SIGABRT that made every
  contained payload look like a timeout on macOS 26.6.2; `/System`, repository paths, and Mach
  lookup/registration stay denied.
- Decision 0153: keeps the existing 100 ms Darwin invocation cap and zero retries; documents a
  first-invocation `TIMEOUT` as expected Darwin behavior (`spctl`/`syspolicy_check` failing before
  any payload runs) rather than adding an unverified staging warmup.

**Further decisions in this range** (accepted unless noted; see `docs/decisions/` for full text):
0089 agentic-completeness wave; 0091 `cem status` `baseSpan` is a Go defect; 0092 three Go-only
observables become spec-owned; 0093 records `GPK-V0-055/056` acceptance; 0094 `relocated` for an
unmoved span is frozen cem/0.1 semantics; 0095 the index path screen is the current Go set,
`.claude` included; 0097 over-bound prompts get a disclosed anchor query, not a stored pointer;
0098 a same-path type change is `deleted` in frozen cem/0.1 drift; 0100 doc-compiler kebab codes
are secondary error-code details; 0101 three divergence-wording regions become owning-spec text;
0102 the learned-trace path screen adopts the index-snapshot screen (`IDX-SNAP-V0-018`); 0104
sql-native ratchets pin the toolchain by content; 0105 `SRG-V0`/`GAG-V0`/`OACS-V0` stay proposed;
0111 work-queue producer tests share one compiler cache; 0121 the refusal snapshot admits exactly
the ignored ledger; 0122 CEM/OCM next actions keep the contract command name `corvint`; 0130 the
work-queue adapter build trims paths inside the adapter; 0131 gate-affected attributes dirty paths
from a static import/path-literal index; 0132 claimless analyzer candidates are frozen as delivery
status `deferred`; 0136 extensionless root files are line-citation targets; 0148 contained analyzer
staging is content-addressed and reused; 0149 the test-validity execution report gains a `SKIPPED`
outcome; 0150 `EPG-V0` owns and accepts the existing end-of-line policy gate; 0151 the
change-witness symbol resolver is Go's standard-library parser over Git objects; 0152 the Semantic
Escalation Gate's first slice is an unwired deterministic gate; 0154 ratchets unpinned citations
and pins every new document. Decisions 0086 (one prospective Packet 5 retry import, status stays
`NOT_RUN`) and 0087 (Markdown stays the only documentation kind that can own authority) predate
this wave but were not previously cited by number above.

### What was measured

- `context` snapshot hit 70 to 80 ms wall against ripgrep 80 to 110 ms on the same tree; a miss
  (new commit or rebuilt binary) 480 to 560 ms; the bench never hits, so every bench latency is a
  full build (`docs/plans/context-miss-quick-wins-2026-09-05.md`, `docs/plans/corvint-pack-prototype-2026-09-05.md`).
- Corvint Pack in-process load 27.4 to 15.7 ms, heap 48 MB to 3 MB, subject hit 0.55 to 0.16 s wall,
  120 packet cells byte-identical, 721 of 721 flipped blocks refused; the 10 ms and 4 KB row
  targets were not decidable under load.
- Ranking, recall@5/@10/@20 for the `context` arm on the v2 positives: code2test .225/.356/.485,
  comment2context .221/.344/.479, edit2ripple .323/.509/.608, trace2code .421/.543/.833. Paired
  intervals resolve "beats grep" on code2test and comment2context only; `bm25:ident` beats
  `context` on trace2code recall@5 by 0.081 (CI [-0.158, -0.003]). Test-code linking lifts
  code2test recall@20 to .512 and costs trace2code recall@20 .833 to .804. Answerability lifts
  no-gold selective success 0 to 0.08 and counterfactual 0 to 0.312 at a 0.01 trace2code cost.
- Go precision on the development corpus 0.803 at recall 1.0 against the 0.80 gate.
- Snapshot-reader pack v2 (benchmark-only codec, not shipped in `internal/contextindex`): a 100k-row
  one-path read drops from 26.1 MB / 848.5 ms (pack v1) to 14 KB / 0.18 ms median cold, on a
  load-contaminated 12-core host (`benchmarks/snapshot-reader/README.md`).
  `DNIP-IDX-009` is unchanged and no format selection follows from this measurement alone.
- Retrieval-eval comparability (`REC-V0-001..007`): a registered dev/held-out split (22/9 of 31
  release rows) reports recall 1.0 and precision 0.860656 (dev) / 0.720426 (held-out) with 0
  critical misses in both; held-out rows are all `previously-observed`, so no generalization claim
  follows. Adapter-deadline fixes measured over 24 runs each: Go `user-prompt` max 1680 ms and
  Codex max 1762 ms against their host kill deadlines, versus 24-of-24 kills before the fix.
- Two ranking experiments stayed offline and did not change production ranking: a fitted
  multi-signal calibration lost to the existing relation-tier order on held-out and
  leave-one-repository-out splits (no-go), and a symbol-frontier/admission-cap variant gained 4
  gold rows in-sample but touches Python-oracle parity, so it is filed as a promotion follow-up
  rather than shipped.

### Known limitations and open owner calls

- Superseded 2026-09-12 by decision 0088: the retry below is cancelled, not pending. Packet 5 stays
  `NOT_RUN`: the retry preregistration `conformance/perf-v0/packet-5-retry-manifest.json` (decision
  `0085-packet-5-retry-preregistration-2026-09-11.md`, twelve rows, Corvint pin `d4f69c2f`) is
  registered but the formal run is not started; decision 0037 item 1's report path is amended
  only when the result lands. The 2026-09-03 `/1` run measured snapshot-present compact
  `session-start` at 955 ms p95 against the 250 ms cap.
- The core retrieval claim is established at top ranks on two of four subsets and tied or
  fold-dependent on the other two; the honest external ceiling without embeddings is
  RepoMap-class structure-aware retrieval, which `context` is below on three subsets at recall@20.
- The learning loop has no measured consumer in `context` (`docs/agent-memory/ideas.md`,
  "traces feed `context` or the learning claim is withdrawn").
- Idle-host re-measurement of the pack and miss-path falsifiers, the first `v2_abstention` run
  under the ladder, and the blob-keyed shard store (decision 0069 step c) are open.
- Every `NOT_PRODUCED` dogfood step for this range is listed in `.corvint/dogfood-report.json` and
  named in the landing's BUILD-LOG entry.
- In this 2026-09-12 preparation snapshot, the optional companion bundle could not be assembled:
  `corvint-taskman` had no tracked `LICENSE` or `PROVENANCE.md`, so
  `script/corvint-companion-release-gate` refused at the `atm` notices step.
- In the same preparation snapshot, GitHub Actions hosted CI was blocked by account
  billing/spending restrictions; no hosted-CI run backed a check in that range.
- The task manager's non-inferiority gate margin rule, capacity-reservation planner, and reservation
  liveness checks (taskman decisions 0006/0009, TCP-02c/03/04a/04b) landed with race-clean tests,
  but read verbs never wire an `AttemptOracle`, so `SELECTED`/`DEFERRED` planner outcomes are proven
  only by fixtures, not live attempt evidence.
- `calibrate` returns every bucket empty against this repository's own trace store, because the
  schema it reads carries no `packet_stance`/`relevance_score`; `depsource` module verification and
  `surprise` (dirty-worktree refusal) likewise have fixture-only or negative-path evidence on this
  repository (decision 0089 wave).

## 0.4.0a3 (2026-09-04)

This section covers work landed on `main` between the `v0.4.0a2` tag (8feba58, 2026-09-04) and the
`v0.4.0a3` tag. The tag is a local alpha of a reproducible artifact and not a publication, promotion,
or readiness claim: the publication and promotion rows of `script/release-checklist` stay
`NOT_RUN`, packet 5 stays `NOT_RUN` for the reason given below, and the V4 generalization gate is
unmet. Decisions are cited by full filename.

### What shipped

- **Wave 17** (`docs/BUILD-LOG.md`, 2026-09-04): parallel term-table inversion in the index build;
  the session-start snapshot read; `LoadEventSnapshot` and a compact event snapshot for
  `file-change` and compact `session-start`; `evalPruneAuxiliarySymbols` and the Go precision gap
  review (`docs/reviews/GO-PRECISION-GAP-2026-09-04.md`); the analyzer-shell test package
  parallelised; the Python syntax differential driver (`CORVINT_PYTHON_DIFFERENTIAL=1`); the
  observation ledger sweeps temporaries left by a dead writer; OCM resume after a sidecar-only
  move; the dogfood scripts build the binary they run.
- **Work Queue Observation V0** accepted and amended with derived path clashes and a maximal
  collision-free wave (`docs/decisions/0046-work-queue-observation-v0-acceptance-and-clash-wave-amendment-2026-09-04.md`,
  `WQO-V0-041..045`), then implemented as experimental: `internal/workqueue`, `corvint work
  observe` and `propose-wave`, the self-dogfood adapter (`script/corvint-work-queue`,
  `docs/worklist.json`, `.corvint/work-queue-policy.json`), and `conformance/work-queue-v0`. The
  exact wave search carries a clique-cover bound and a node budget after review found it
  enumerating every equal-size maximum.
- **Eight proposed specs accepted as intent**
  (`docs/decisions/0047-batch-acceptance-eight-proposed-specs-2026-09-04.md`); delivery stays
  not-started. The frontier decision brief and the native CEM claim carry real statuses in
  `docs/specs/INDEX.json`.
- **The four open owner calls are made** (`docs/decisions/0048-four-owner-calls-2026-09-04.md`):
  `GPK-V0-017(a)` measures index-building events with a snapshot under the 250 ms portable cap;
  the `0016` decision pair is grandfathered and `make decision-numbers-check` joins `make gate`;
  the Beamfall work-queue shadow is deferred to the Beamfall repository; this alpha is cut at the
  wave-17 head.

### What was measured

- Index build p50 1,253 → 1,013 ms; session-start p95 99 → 85 ms; `file-change` p95 158 → 151 ms
  and compact `session-start` 166 → 147 ms with a snapshot present (the gob snapshot must be read
  whole; the next lever is a sectioned format, `docs/agent-memory/optimizations.md`).
- Go precision on the development corpus 0.780 → 0.795 at recall 1.0 against the 0.80 gate
  (`benchmarks/results/v4-development-go-2026-09-04.json`); the earlier Python baseline was never
  same-revision, so the remaining gap is under 0.01.
- `cmd/corvint-analyzer-shell` tests 77 → 7 s; the Python syntax differential (13,720 inputs) leaves
  176 over-permissive verdicts, all from opaque balanced spans (`docs/agent-memory/fixes.md`).
- The dogfood loop for `8feba58..a373f15` (121 hunks, every hunk cited) produced every step except
  range impact, which refuses the copy `benchmarks/results/v4-development.json →
  v4-development-go-2026-09-04.json`; `make dogfood-check` therefore records `FAIL
  dogfood-report-drift` on an incomplete report, and no exception path exists to record it
  otherwise (`docs/agent-memory/fixes.md`, five loop frictions).

### Known limitations and open owner calls

- Packet 5 stays `NOT_RUN`: under the amended `GPK-V0-017(a)` the snapshot-present corpus binds
  only once an automatic index refresh ships and is dogfooded (decision 0037 item 1). That refresh
  is the follow-up that unblocks the row; no retry preregistration was written.
- The Beamfall adapter and independent recorder for the work-queue shadow cycles are owned by the
  Beamfall repository; `WQO-V0-035..037` stay `NOT_RUN`.
- The `0.4.0a2` notes listed DR-0018 as still open; it was closed in wave 8 by `GPK-V0-048`
  (`docs/BUILD-LOG.md`), so that line was stale.

Still unproven, and the reason this is an alpha rather than a release: the core retrieval claim is
not established (blind-v3 8/10 critical misses; recorded at repository revision
`daf0fe05d8a32fa448d71cf92da20c009bd349a8`, the valid 2026-09-04 CEM reviewer-trial pilot re-run
using Corvint `0.4.0a3` scored 5 pairs with 0 dropped pairs and 0 errored lanes, with mean missed
evidence of 0.90 for control and 0.86 for treatment, `delta: 0.04`, and `relative_reduction:
0.044444`; no sealed held-out outcome run exists; the 2026-09-03 pilot remains published invalid
with 31 errored lanes, 5 dropped pairs, and 0 scored pairs).

From `script/release-checklist` (read-only; exits 1 while incomplete), run at the tagged revision:

| Check | Status | Requirement | Reason |
|---|---|---|---|
| packet-5 | NOT_RUN | GPK-V0-017 | formal current-pin requalification is incomplete |
| wheel-integrity | PASS | ARTIFACT-V0-001..010 | current-revision wheel build and verification pass |
| go-archive | PASS | ARTIFACT-GO-V0-001..007 | current-revision archive build and verification pass |
| tag | PASS | ARTIFACT-RDY-V0-003 | expected release tag points at HEAD |
| publication | NOT_RUN | ARTIFACT-RDY-V0-004 | no repository-owner publication receipt is present |
| promotion | NOT_RUN | ARTIFACT-RDY-V0-005 | no repository-owner promotion acceptance is present |

## 0.4.0a2 (2026-09-04)

This section covers work landed on `main` between the `v0.4.0a1` tag (2026-09-03) and the
`v0.4.0a2` tag. Like its predecessor, the tag is a local alpha of a reproducible artifact and not a
publication, promotion, or readiness claim: the publication and promotion rows of
`script/release-checklist` stay `NOT_RUN`, packet 5 stays `NOT_RUN` for the reason given below, and
the V4 generalization gate is unmet. Decisions are cited by full filename.

### What shipped

- **Wave 15 landed** (`docs/BUILD-LOG.md`, 2026-09-03): `Source.Text()` reports whether a source
  was loaded, so a rule reading an unloaded source refuses instead of reading an empty file
  (invariant 2); `stabilizePacketBytes` refuses a receipt that never reaches a fixed point; the
  observations writer validates its own contract (SOL-V0-002); `dogfood-record --verify` names
  unsupported shell syntax; `harness event --budget-bytes` bounds at `gokernel.MinOutputBytes`;
  `cem cite` refuses an evidence path the CEM's own hunks modify; TCQ-V0-047's per-edge
  `unsupported-python-grammar` abstention; three more Python false-accept classes rejected;
  `dogfood-check` refuses a dirty tree and `docs/DOGFOOD.md` documents the order that produces a
  complete report; `make release-wheel-witness` builds the release-builder profile;
  `perf-v0 diagnose` reproduces one task on both runtimes member by member; the perf sample timeout
  rises to 120 s (`docs/decisions/0042-perf-sample-timeout-amendment-2026-09-03.md`); owner
  approval for staged Python retirement is recorded
  (`docs/decisions/0043-python-retirement-owner-approval-2026-09-03.md`).
- **Wave 16** (`docs/BUILD-LOG.md`, 2026-09-04): DR-0009 grants for the last three ungranted
  perf-v0 invocations (`docs/decisions/0044-perf-grants-dr0009-beamfall-harness-and-snapshot-2026-09-04.md`);
  `file-change` and compact `session-start` read the index snapshot (`IDX-SNAP-V0-010`);
  `GLTP-V0-037` freezes the unit-coverage tokens and digest preimages; decision 0045 records the
  OCM-V0-013 / CF-V0-030 acceptance; perf-v0 refuses a corpus pin unreachable from `HEAD`;
  impact convention-authority and ordering tests, a decision 0014 mutant-killing test,
  `BenchmarkTaskContext` with a `CORVINT_CPUPROFILE` hook, and the IDX-SNAP-V0-007 eviction test.

### What was measured

- Packet-5 protocol, run3 (`conformance/perf-v0/results/packet-5-current-pin-requalification-2026-09-03-run3/`,
  22 tasks, 100 samples per runtime, 120 s timeout): every `unequal-stdout` task is now diagnosed
  and granted; valid index-building harness events measure 835.5–955.1 ms p95 on both corpora
  against the 250 ms cap of `GPK-V0-017(a)`. Two run3 tasks are excluded under `GPK-V0-016`
  because their state is `insufficient_evidence` (`unequal-stdout`):
  `beamfall-harness-file-change-snapshot-present` (1,079.5 ms) and `beamfall-harness-file-change`
  (942.4 ms). The cheap events
  measure 76–95 ms, `user-prompt` 363/405 ms cold and 193 ms with a snapshot against its 500 ms cap.
- IDX-SNAP-V0-010 on a Beamfall clone with a snapshot present, 30 fresh processes, p50/p95:
  file-change 973.6/1,773.1 → 163.0/170.0 ms; compact session-start 1,086.3/1,311.1 → 135.0/189.2 ms
  (not a harness verdict; the binding cold corpus is unchanged).

### Known limitations and open owner calls

Still open and owner-owned (`docs/agent-memory/questions.md`):

1. `GPK-V0-017(a)` cannot be met by the index-building harness events on the binding cold corpus.
   Either the clause is amended so those events are measured after an index is available with
   their own cap, or the alpha ships with packet 5 measured `FAIL` after a retry preregistration
   under `P5R-V0-004`. Neither the retry preregistration nor a new Corvint pin was written without
   that call.
2. DR-0018 and the `0016` decision-number collision, carried from `0.4.0a1`.

Still unproven, and the reason this is an alpha rather than a release: the core retrieval claim is
not established (blind-v3 8/10 critical misses; recorded at repository revision
`daf0fe05d8a32fa448d71cf92da20c009bd349a8`, the valid 2026-09-04 CEM reviewer-trial pilot re-run
using Corvint `0.4.0a3` scored 5 pairs with 0 dropped pairs and 0 errored lanes, with mean missed
evidence of 0.90 for control and 0.86 for treatment, `delta: 0.04`, and `relative_reduction:
0.044444`; no sealed held-out outcome run exists; the 2026-09-03 pilot remains published invalid
with 31 errored lanes, 5 dropped pairs, and 0 scored pairs).

From `script/release-checklist` (read-only; exits 1 while incomplete), run at the tagged revision:

| Check | Status | Requirement | Reason |
|---|---|---|---|
| packet-5 | NOT_RUN | GPK-V0-017 | formal current-pin requalification is incomplete |
| wheel-integrity | PASS | ARTIFACT-V0-001..010 | current-revision wheel build and verification pass |
| go-archive | PASS | ARTIFACT-GO-V0-001..007 | current-revision archive build and verification pass |
| tag | PASS | ARTIFACT-RDY-V0-003 | expected release tag points at HEAD |
| publication | NOT_RUN | ARTIFACT-RDY-V0-004 | no repository-owner publication receipt is present |
| promotion | NOT_RUN | ARTIFACT-RDY-V0-005 | no repository-owner promotion acceptance is present |

## 0.4.0a1 (2026-09-03)

This section covers work landed on `main` between the `v0.4.0a0` tag (2026-08-30) and the
`v0.4.0a1` tag. The tag is a local alpha of a reproducible artifact; it is not a publication,
promotion, or readiness claim (`script/release-checklist` rows publication and promotion stay
`NOT_RUN`, and the V4 generalization gate is unmet). Decisions are cited by full filename because
numbers collide across lineages (`docs/decisions/README.md`).

### What shipped

- **Release-hardening wave** (`docs/decisions/0036-release-hardening-wave-2026-09-02.md`, three
  sub-waves): a `query` packet that abstains now always carries `abstention.reason`
  (GPK-V0-045); a packet whose only non-advisory support is `authority: syntax` reports zero
  `authoritative_results` with one deterministic uncertainty line (GPK-V0-046); `query` and the
  `user-prompt` harness event read the `corvint index` snapshot when the tree matches
  (IDX-SNAP-V0-008/009); `impact` and `harness event`'s ledger appends are now disclosed in
  top-level help rather than described as non-mutating; three kernel paths that reported a
  failure as an answer are repaired (Git error classification, one-`Unparsed`-row-per-source
  import refusal, CEM U+2028/U+2029 byte parity); `benchmarks/run.py` gained `--engine corvint`
  and `--repos`, and a subset run can no longer report `v4_release.ready`; the Go engine's
  `EvalQuery` confident-symbol selection gained index-wide term frequency and dependency-link
  promotion (GPK-V0-047); web (JavaScript/TypeScript) symbols joined the index instead of a
  per-query enrichment; `make gate` is now reproducible from a fresh clone; a frozen blind-v4
  partition, quieter gates, bounded reads, and one secret screen also landed.
- **Release-checklist and trial waves** (waves 4-6, 2026-09-03): `script/release-checklist` reads a
  wheel witness instead of emitting a hard-coded `NOT_RUN`, and `tools/release_artifact.py check`
  writes that witness bound to the commit and tree it built;
  `docs/decisions/0037-release-owner-calls-2026-09-03.md` answers the four owner calls carried by
  the previous alpha (below); `docs/specs/cem-reviewer-trial-v0.md` (CRT-V0-001..010) and
  `tools/cem-trial` preregister and implement the paired reviewer trial that is the one CEM
  measurement needing no outside users; `conformance/perf-v0` gains the preregistered
  `beamfall-snapshot-present` corpus beside the binding cold one; `script/dogfood-change.sh` no
  longer dirties the tree before `dogfood-record`, so `local-outcome` can be PRODUCED at all; and a
  Go/Python selector drift over parenthesized `const (...)`/`var (...)` groups is registered as the
  spec gap DR-0018 rather than repaired toward either runtime.
- **Retrieval/context-index mechanisms**: a reference slot for files naming a subject symbol
  (`docs/decisions/0035-reference-slot-2026-09-02.md`); a lexical/sibling term-table lookup
  (`docs/decisions/0033-term-table-2026-09-02.md`); an index snapshot that removes the rebuild
  from a packet call (`docs/decisions/0032-index-snapshot-2026-09-02.md`); sibling rows sharing
  a basename term rank first (`docs/decisions/0031-sibling-name-overlap-2026-09-02.md`); Swift
  module importers naming a subject symbol are reverse-import rows
  (`docs/decisions/0030-swift-module-importers-2026-09-02.md`); corroborated rows rank first
  (`docs/decisions/0027-corroborated-rank-2026-09-02.md`); packet rows carry an action
  (`docs/decisions/0028-row-actions-2026-09-02.md`); a co-change slot with an age-dependent
  commit cap (`docs/decisions/0025-cochange-slot-2026-09-02.md`); anywhere-in-tree pair rows
  rate medium (`docs/decisions/0026-pair-anywhere-medium-2026-09-02.md`); `t.Run` literals
  anchor Go test claims (`docs/decisions/0029-run-case-anchors-2026-09-02.md`).
- **Confidently-wrong trial harness (cw-trial)**: a task-context packet for the trial's corvint arm
  (`docs/decisions/0024-task-context-packet-2026-09-02.md`, `0023-corvint-arm-answers-2026-09-02.md`);
  a no-repository-access dispatch mode (`docs/decisions/0022-trial-no-repository-access-2026-09-02.md`);
  retrieval-native metrics scored beside the agent outcome
  (`docs/decisions/0034-retrieval-native-metrics-2026-09-02.md`).
- **Query/impact surface**: the `vN` version-token filter no longer empties the universe
  (`docs/decisions/0017-version-token-filter-2026-09-01.md`); three standalone query refusal
  paths lifted (`docs/decisions/0013-query-refusal-paths-2026-09-01.md`); a project-operations
  instruction result rests on its classifying words
  (`docs/decisions/0014-project-operations-floor-support-2026-09-01.md`); `impact` admits `.py`
  and web paths without a Go module (`docs/decisions/0015-impact-without-go-module-2026-09-01.md`);
  a change-anchored packet joins `prove --base`, range impact, and mutation
  (`docs/decisions/0016-change-anchored-packet-2026-09-01.md`); change mode counts coverage per
  changed path (`docs/decisions/0018-affected-coverage-summary-2026-09-01.md`); `reference-resolves`
  reaches JavaScript/TypeScript (`docs/decisions/0019-javascript-reference-resolves-2026-09-01.md`);
  `test-kills-mutant` runs for Python (`docs/decisions/0020-python-test-kills-mutant-2026-09-01.md`);
  nested modules without `go.work` are admitted only through the importer's own `replace`
  (`docs/decisions/0021-nested-module-replace-admission-2026-09-01.md`).
- **Acceptance/governance**: harness authority boundary
  (`docs/decisions/0009-harness-authority-boundary.md`); query and Go-archive acceptance
  (`docs/decisions/0010-query-and-go-archive-acceptance-2026-08-31.md`); standalone query
  trace-state acceptance (`docs/decisions/0011-standalone-query-trace-state-acceptance-2026-09-01.md`);
  expert-panel ratifications (`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`);
  perf-corpus repin (`docs/decisions/0008-perf-corpus-repin-2026-08-30.md`).

### What was measured

- Development corpus (31 cases, 5 repositories), both engines: Python 0/31 critical misses,
  recall 1.0, byte-weighted precision 0.804952 at `corvint_commit`
  `3fe7ac60459076005964bd8a0f729b028550879f`; native
  `--engine corvint` 0/31, recall 1.0,
  precision 0.780332. `benchmarks/results/v4-development.json`,
  `benchmarks/results/v4-development-go.json`.
- Beamfall clone (3,233 files) query latency, interleaved fresh-process n=20, snapshot present vs.
  earlier baseline: `query --limit 1` p50/p95 337.7/361.2 -> 181.0/193.1 ms; `harness user-prompt`
  421.3/451.0 -> 219.2/231.0 ms (`docs/decisions/0036-release-hardening-wave-2026-09-02.md`, "What
  was measured"; GPK-V0-017(b) is met only on the path where `index` has already run).
- Native Go install/build: cold `go build` 4.27 s, warm 0.32 s, cold/warm binary byte-identical;
  first cold query 0.42 s, warm 0.44 s, byte-identical to cold.
  `benchmarks/results/install-smoke-go.json`.
- Python wheel install: 2.45 s install, first useful query 0.46 s, CEM verify 0.70 s, total 3.61 s.
  `benchmarks/results/install-smoke.json`.
- CEM 0.1 kit run standalone (not an independence claim; needs an outside author): `doctor` PASS,
  50/50 fixture digests, same-author Go probe 31/31 (`docs/decisions/0036-...`, "What was measured").
- Change-anchored packet on a real committed change: `benchmarks/results/prove-change-77d4d62-first-run.json`
  (`docs/decisions/0018-affected-coverage-summary-2026-09-01.md`).
- Confidently-wrong trial pilot and later runs: `benchmarks/results/cw-trial-pilot-first-run.json`,
  `benchmarks/results/cw-trial-heldout-v1-development-run-3.json`,
  `benchmarks/results/cw-trial-unseen-corvint-v1-run-3.json` (decisions 0022, 0025, 0026).
- CEM reviewer trial pilot, published invalid and carrying no estimate:
  `benchmarks/results/cem-reviewer-trial-pilot.json`. The agent budget ran out mid-run, 31 of 50
  lanes exited non-zero, and no pair scored ("no scored pair: the run measured nothing"). It stands
  as evidence of the scorer defect it exposed - a failed invocation was being scored as a total
  miss - not as a measurement of CEM. Nothing is yet known about whether CEM reduces missed
  evidence.
- Blind-v3 (held-out, public repositories), unchanged by this wave: 8/10 critical misses, recall
  0.181818, precision 0.258638, top-five 0.6, abstention/epistemic accuracy 0.0
  (`benchmarks/results/blind-v3-first-run.json`; also cited in `docs/V4-STATUS.md`). No blind-v4
  result is reported: `benchmarks/blind-v4-manifest.json` is frozen (31 cases) but has never been
  run (`first_observed_at` is null).

### Known limitations and open owner calls

The four owner calls carried by `v0.4.0a1`'s first tagging were answered by
`docs/decisions/0037-release-owner-calls-2026-09-03.md` (accepted): the Packet 5 cold path stays
binding and a snapshot-present corpus is added beside it; `MinPacketBytes` rises from 1,024 to a
test-derived floor of 1,216; blind-v4 stays sealed under a preregistered run condition; and
reference chains are dropped, the informal 0.02 precision allowance replaced by a gate-tied rule.

Still open and owner-owned (`docs/agent-memory/questions.md`):

1. DR-0018: accept the clause that would close the Go/Python declaration-set drift, or keep it a
   registered spec gap? Neither runtime may win it unilaterally under GPK-V0-033.

Still unproven, and the reason this is an alpha rather than a release: the core claim that Corvint
retrieves better evidence than a conventional search is not established. Blind-v3 stands at 8/10
critical misses, the Go engine is below the 0.80 precision gate on its own development corpus, and
the one trial designed to settle the CEM half of the question has produced a valid pilot re-run,
recorded at repository revision `daf0fe05d8a32fa448d71cf92da20c009bd349a8` using Corvint `0.4.0a3`:
5 scored pairs, 0 dropped pairs, 0 errored lanes, mean missed evidence of 0.90 for control and 0.86
for treatment, `delta: 0.04`, and `relative_reduction: 0.044444`. It has not produced the sealed
held-out outcome run. The 2026-09-03 pilot remains published invalid with 31 errored lanes, 5
dropped pairs, and 0 scored pairs.

From `script/release-checklist` (read-only; exits 1 while incomplete), current run:

| Check | Status | Requirement | Reason |
|---|---|---|---|
| packet-5 | NOT_RUN | GPK-V0-017 | formal current-pin requalification is incomplete |
| wheel-integrity | PASS | ARTIFACT-V0-001..010 | current-revision wheel build and verification pass |
| go-archive | PASS | ARTIFACT-GO-V0-001..007 | current-revision archive build and verification pass |
| tag | PASS | ARTIFACT-RDY-V0-003 | expected release tag points at HEAD |
| publication | NOT_RUN | ARTIFACT-RDY-V0-004 | no repository-owner publication receipt is present |
| promotion | NOT_RUN | ARTIFACT-RDY-V0-005 | no repository-owner promotion acceptance is present |

### Verification (historical, recorded for the `v0.4.0a1` tag)

This is the exact procedure run against the `v0.4.0a1` tag above, kept as a historical record, not
current instruction: decision 0088 retired the Python engine and wheel, and `tools/release_artifact.py`
and `requirements/release-build.txt` no longer exist in this checkout (removed by `54735d98`,
2026-09-11). See "Verification" under the `0.4.0a4 candidate` section above for the current,
Go-only procedure. The wheel-integrity row needed the pinned release-builder profile, which no
Make target created:

```sh
python3.14 -m venv BUILDER
BUILDER/bin/pip install --require-hashes -r requirements/release-build.txt
BUILDER/bin/python tools/release_artifact.py check --revision "$(git rev-parse HEAD)" --output TMP
script/go-archive-gate
script/release-checklist
```

```sh
test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.0"
GOTOOLCHAIN=local go test -count=1 ./...
GOTOOLCHAIN=local go vet ./...
(cd interop/cem01-go && GOTOOLCHAIN=local go test -count=1 ./... && GOTOOLCHAIN=local go vet ./...)
```
