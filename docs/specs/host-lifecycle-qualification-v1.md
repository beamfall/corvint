# Host Lifecycle Qualification V1

Owner: Russell Lewis
Date: 2026-09-24
Requirement prefix: `HLQ-V1`
Intent status: accepted
Delivery status: experimental
Authoritative inputs: ticket V1-0016, `agent-harness-integration-v0.md` (`AHI-002`, `AHI-003`,
`AHI-004`, `AHI-006`, `AHI-007`, `AHI-009`, `AHI-010`), `core-compatibility-freeze-v1.md`
(`CCF-V1-001`, `CCF-V1-002`), `PRS-V1-006` in `corvint-1.0-product-and-release-v1.md` as ratified by
decision 0373 (items 5 and 6), and `AGENTS.md` invariants 2, 4 and 7.

## Agent digest
- Claim: Each Core host tuple passes nine lifecycle cases on exact versions in isolated host homes, and a result binds only the tuple that produced it.
- Status: accepted intent (decision 0381 item 1), experimental delivery; the host scope it qualifies is accepted in decision 0373
- Exists: this contract and `conformance/host-lifecycle-v1`; all three tuples PASS on darwin/arm64 with the corvint `1.0.0-rc.1` candidate build, and the Claude Code tuple (2.1.293, adapter 0.3.0) also PASSES with the published `1.0.0-rc.2` build; reports retained (see Results)
- Blocked on: linux tuples and live model-session cases are NOT_RUN; each result is stale at the next release (`HLQ-V1-008`), so the CLI and Codex tuples are unqualified for `1.0.0-rc.2` until rerun against its build
- Read next: Requirements; Results; Known gaps

## Intent and scope

V1-0016 requires each host tuple that Core supports to pass the install, discovery, context,
expansion, change, frontier, degradation, upgrade and uninstall cases. Decision 0373 (items 5 and
6, `PRS-V1-006`) fixes the Core host rows at three tuples, each at FALLBACK on exact versions: the
plain CLI, Codex CLI with the Corvint plugin, and Claude Code with the Corvint plugin. Before this
contract the adapters had static
checks (`compatibility.json` `lastValidation` `STATIC_ONLY`, `lastConformance` `NOT_RUN`) and no
lifecycle run through a host's own package manager.

This contract defines the nine cases for each surface and the runner that executes them. It changes
no runtime, adapter or plugin behaviour. It does not claim FULL support, protected authority, or
behaviour inside a live model session.

## Requirements

- **HLQ-V1-001:** A tuple MUST be named by its surface (`native` or `plugin`), the host executable
  and its exact version, the adapter package version from the installed plugin manifest, the OS and
  architecture, and the exact `corvint --version` line, and its report names the revision of the
  clean checkout that supplied the plugin package. The runner MUST exit 2 without running any case
  when it cannot determine one of these, or when `--source` is not clean. A result binds only that
  tuple. A tuple that
  was not run, or whose run did not pass all nine cases, keeps the support its `compatibility.json`
  declares and MUST NOT inherit another tuple's result.
- **HLQ-V1-002:** The runner MUST execute the nine cases in this order, report each as `PASS`,
  `FAIL` or `NOT_RUN`, and report the tuple as passing only when all nine are `PASS`:

  | Case | Plain CLI predicate | Plugin host predicate | Owning requirement |
  |---|---|---|---|
  | install | the binary is copied onto a private `PATH` and `--version` prints `Corvint …` | the host's marketplace-add and install commands succeed; the plugin's own row in the host listing shows the manifest version; the installed package is byte-equal to the source package | `AHI-002` |
  | discovery | root help's Core section lists the twelve `CCF-V1-001` verbs | the plugin's row in the host listing reports it enabled; every registered hook event runs exactly one command, `corvint adapter <host>`; the registered event set is exact; the package has a skill | `AHI-002` |
  | context | `context --task` returns `tool=context`, `schema_version=1` | SessionStart returns a repository-data envelope whose receipt is `ok`, `mutates=false`, profile `corvint-dogfood-event/0`, pinned to the fixture HEAD with a clean worktree | `AHI-003` |
  | expansion | `query --task "Explain add.go"` cites `add.go` at its HEAD blob | UserPromptSubmit naming `add.go` returns task evidence for `add.go` at its HEAD blob | `AHI-004` |
  | change | `impact add.go` on the edited file returns `tool=impact`, `mode=impact` | after an edit, Claude Code's PostToolUse returns a harness receipt, and the next prompt receipt on either host reports one dirty path and a non-clean worktree | `AHI-007` |
  | frontier | a committed change with an `unknown` CEM hunk and an OCM over one requirement yields `frontier/0` `OPEN` with exit 1 | an unenrolled Stop releases; after `dogfood begin` bound to the hook session, an incomplete Stop blocks once and names the unavailable Frontier authority; the recursive Stop releases. On Codex the enrollment and every Stop receive `CODEX_THREAD_ID`, which differs from the payload `session_id`, so the Stop binds only through the environment | `AHI-006` |
  | degradation | outside a Git repository `context` exits 2 with `ok=false`, `code=invalid-arguments` and writes nothing | malformed hook JSON exits 0 with a visible `malformed-hook-json` degradation and "coding continues" | `AHI-009` |
  | upgrade | after the N-1 binary indexes the fixture and is replaced, the current binary does not reuse its snapshot: `index --if-stale` rebuilds instead of reporting `state=fresh` (a snapshot is keyed by the binary that wrote it), and `context` succeeds | the registered hooks serve valid context under the N-1 binary and after its replacement by the current binary; the host's reinstall keeps the package byte-equal | `AHI-002` |
  | uninstall | the binary is removed and no longer resolves; the fixture worktree is unchanged | the host's disable (Claude Code: then enable), uninstall and marketplace-remove commands succeed; the host no longer lists the plugin; the installed root is gone or carries the host's own `.orphaned_at` retirement marker; no other file under the private `HOME`, whatever its size, names corvint; the fixture worktree is unchanged | `AHI-002` |

  A missing repository-data envelope MUST retain the received `additionalContext` in the error
  as a quoted prefix of at most 2048 bytes, with an explicit omitted-byte count when truncated.
  The context-hook helper MUST also return the full received text on receipt-validation failure.
  A visible `adapter-host-kill-deadline` fallback is thus distinguishable from malformed envelope
  output; neither becomes a successful lifecycle case, and no deadline is widened.

- **HLQ-V1-003:** Each run MUST happen in a fresh private workspace: `HOME`, `CLAUDE_CONFIG_DIR`
  and `CODEX_HOME` point inside it, and `PATH` holds only the private `corvint` directory, the host
  executable's directory, Git's directory and the system directories. Command names MUST resolve
  against that private `PATH`, never the runner's own. The runner MUST NOT read or write the
  operator's host homes, credentials or settings. It MUST exit 2 when a directory on that `PATH`
  other than the private one holds a `corvint`, because that binary would resolve once the private
  one is removed. It MUST remove the workspace when it exits, including after a setup error. On
  SIGINT or SIGTERM it removes the workspace and exits 2 without waiting for a running host child.
  A plugin host's package directory under `--source` MUST hold no untracked or ignored file,
  because the host copies those too.
- **HLQ-V1-004:** Plugin hook cases MUST run the command each hook event registers in the package
  the host installed, read from the installed `hooks/hooks.json`, with a host-shaped JSON payload on
  stdin. This tests the installed adapter at the host's hook boundary. It does not test the host's
  own dispatch: hook trust prompts, host timeouts, and the model session. Those are recorded
  separately as supplementary observations and are not one of the nine cases.
- **HLQ-V1-005:** A passing run MUST NOT raise a tuple above FALLBACK or grant it protected
  authority. It is evidence for the `PRS-V1-006` Core host rows, which decision 0373 fixes at
  FALLBACK on exact versions. FULL support and protected authority are off the Core path (decision
  0373 item 6).
- **HLQ-V1-006:** Adapters MUST retain no independent knowledge store. A run shows this when every
  enveloped context receipt reports `mutates=false`, the uninstall case finds no Corvint state in
  the private `HOME` outside the host-retired package root, and `git status --porcelain` reports
  the fixture worktree unchanged after the change, plugin frontier and uninstall cases. The plain
  CLI frontier case works in a clone and leaves the fixture untouched. The Git directory is not
  compared: Corvint keeps its derived index snapshot under the common Git directory and its local
  completion state under the worktree's Git directory, both in `corvint/`. PostToolUse and Stop outputs are
  checked for their own predicates, not for a `mutates` field.
- **HLQ-V1-007:** The runner MUST print one header line, then one `case` line for each case, then a
  `SUMMARY` line with the counts. Fields are tab-separated. It exits 0 only when all nine cases pass,
  1 otherwise, and 2 on a usage or setup error. Received diagnostic text is quoted so its control
  bytes cannot add report rows or columns; truncation remains explicit.
- **HLQ-V1-008:** A result MUST be recorded here with the digests of both binaries and the source
  revision. Each tuple's raw `--report` file MUST be kept under
  `conformance/host-lifecycle-v1/results/`, and its sha256 recorded with the result, so the verdict
  can be re-derived from the repository. A result becomes stale when the host version, the adapter
  version, or the corvint release changes, and the affected tuple needs a new run.
- **HLQ-V1-009:** A hook output whose degradation frame names a time bound is not evidence for any
  case predicate. The time bounds are `dogfood-event-deadline` (`LCP-V0-008`), its stale-snapshot
  form `dogfood-event-index-snapshot-stale` (`AHI-031`), and the watchdog's
  `adapter-host-kill-deadline` (`AHI-017`), bare or as `corvint-event-rejected:<code>`. The runner
  MUST rerun such a hook invocation, at most three runs in all, and MUST name every time-bound run
  (event, attempt, code, elapsed wall time) in the case line whether the case passes or fails. When
  all three runs fail open on a time bound, the case FAILs naming that code, never as a bare
  `decision <nil>`. Any other degradation is not retried and is judged by the case predicate. No
  deadline is widened, and the enrolled incomplete Stop still passes only on `decision=block`
  naming the unavailable Frontier authority. A failed case line MUST also name the case's last hook
  invocation: event, exit code, elapsed wall time, degradation code or `none`, and stdout and
  stderr each quoted to at most 512 bytes with an explicit omitted-byte count (`HLQ-V1-007`). A
  rerun is safe because hook events are reads (`AGENTS.md` invariant 4). A pass after a retried
  time bound qualifies lifecycle semantics at that load, not latency; host timeouts stay outside the
  nine cases (`HLQ-V1-004`).

## Runner

```sh
GOTOOLCHAIN=local go run ./conformance/host-lifecycle-v1 \
  --host cli|claude-code|codex --corvint FILE --base-corvint N1_FILE --source CHECKOUT --report FILE
```

`--source` is a clean checkout whose `integrations/` supplies the plugin package. Without
`--base-corvint`, the upgrade case is `NOT_RUN`, so the tuple does not pass.

## Results

Run on 2026-09-27 on darwin/arm64 (Darwin 25.6.0) against the `1.0.0-rc.1` candidate build. Plugin
sources came from a clean checkout of the candidate commit
`b967f6bbe33c5eba367a07d7eeb2bd1e372e9162`. The current binary is the `corvint` from the candidate's
`corvint_darwin_arm64.tar.gz` (archive sha256
`15d059f1af7cbf9a2ab4aa9bb1b1f11afa1e00e79229365ce540c74f8493aad5`), binary sha256
`baac338555524a741fe70327cd95d56b5abf0186cac0cc14e199239639053ebf` (`Corvint 1.0.0-rc.1 (build
163)`). The N-1 binary is the `corvint` from the published v0.8.1 archive, sha256
`e8c24949d978bf9af3f5535dcdc22c960d3c2f94ebe48c8d3bd220f2b7836df5` (`Corvint 0.8.1 (build 82)`).

| Tuple | Host version | Adapter | install | discovery | context | expansion | change | frontier | degradation | upgrade | uninstall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| plain CLI, native | none | none | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS |
| Codex CLI, plugin | 0.153.2 | 0.2.2 | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS |
| Claude Code, plugin | 2.1.267 | 0.2.3 | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS |

The runner reports are kept under `conformance/host-lifecycle-v1/results/1.0.0-rc.1-darwin-arm64/`
(`HLQ-V1-008`):

| Tuple | Report | sha256 |
|---|---|---|
| plain CLI, native | `cli.txt` | `067c63e7ea85ba4ae127acd559de2cd8fd1e5f07c748887bb907d6429da56b3a` |
| Codex CLI, plugin | `codex.txt` | `110fda04e9fbf6f1e4735428fb3318e04464d7826a9dcae11e599c9a44798d9d` |
| Claude Code, plugin | `claude-code.txt` | `2844e6eddee6a61988520c18c7959a887a1596180a3b1fdbddc3e172a52968e7` |

This run supersedes the 0.8.1 results, whose reports stay under `results/0.8.1-darwin-arm64/`,
and the 0.8.0 results, which were transcribed without retained reports. It becomes stale at the
next corvint release or a change of host or adapter version.

Every other tuple is `NOT_RUN`. This includes linux/amd64 and linux/arm64, other host versions, and
the Gemini CLI, OpenCode and Pi adapters.

Supplementary live observations from the 0.8.0 run, not part of the nine cases:

- Codex CLI 0.153.2 (the operator's own install, `codex exec --json`) in a fixture repository
  answered one prompt. Its session rollout holds two developer messages carrying the repository-data
  envelope, one from SessionStart and one from UserPromptSubmit.
- Claude Code 2.1.267 (`claude -p --include-hook-events`) reported SessionStart and
  UserPromptSubmit hook responses with Corvint context. The model call itself failed on an expired
  OAuth session, so no model turn was observed.

### V1-0397 diagnostic qualification, 2026-09-28

Three consecutive Claude Code 2.1.267 / adapter 0.2.3 runs on darwin/arm64 passed all
nine cases with exit 0 at frozen source `79b2ef0e151e46628a095d03e4dfb62926b1e04e`.
Candidate: `Corvint 1.0.0-rc.1 (build 176)`, SHA-256
`ad38842c14162a46b06887df601f5385c49adc85b540fb498296b385bd1eefef`.
Prior binary: `Corvint 1.0.0-rc.1 (build 163)`, SHA-256
`4bb95d984436f2ac9d040a27f00523dda553301fe352df03bd1817b84ab9eed2`.
Runner SHA-256: `90fd6ae2fe055a248fbcfef91472d49790e3cd4cfbf8557828b95fce1d9ccaf9`.
This is qualification of that diagnostic candidate, not a new release or support promotion.

Runs lasted 12:41:28–12:41:42, 12:41:42–12:41:52, and 12:41:52–12:42:03 UTC.
Recorded one-minute start/end loads were 93.87/92.53, 92.53/95.70 and 95.70/94.68.
All recorded one/five/fifteen-minute values exceeded 80. Sampling was at run boundaries;
continuous load was NOT_OBSERVED, and no load was manufactured. These successful runs do
not identify the lost historical failure's cause or establish a safe load threshold.

Raw reports and matching `claude-code-N-load.txt` readings are retained under
`conformance/host-lifecycle-v1/results/2026-09-28-hlq-diagnostics/`.

| Run | Report | Report SHA-256 | Load-reading SHA-256 |
|---|---|---|---|
| 1 | `claude-code-1.tsv` | `8322436aae5dbe4465a2a8b8417319296e5506003e57af2435bc1d2a809a3073` | `738b1f589b208013fbc241bcb55dbb0ff02ce4cb567baaefda931b7ab9c1e172` |
| 2 | `claude-code-2.tsv` | `8322436aae5dbe4465a2a8b8417319296e5506003e57af2435bc1d2a809a3073` | `557c1c2844f7f984016f534c016ee51e4a08ac865c0f572bdb543de9fd7fcfc3` |
| 3 | `claude-code-3.tsv` | `8322436aae5dbe4465a2a8b8417319296e5506003e57af2435bc1d2a809a3073` | `7c0c9971cd32f19dd0c403c7c60fc73b722cee60317736df56a62fe03a898980` |

### V1-0844 diagnostic run, 2026-10-09

One Claude Code 2.1.293 / adapter 0.2.3 run on darwin/arm64 under `HLQ-V1-009` passed all nine
cases with exit 0 and no time-bound retry, 20:36:24–20:36:32 UTC, one-minute load 9.31/9.68 on 12
CPUs. Source `02e84575088cfd8a6126f8bb927425891e2a5e1a`; candidate built from it
(`Corvint 1.0.0-rc.2 (build 0)`, SHA-256
`037611665826fb5f9b91127fcf285db09ceabf69002b40750d057f6ecce64484`); prior binary
`Corvint 1.0.0-rc.1 (build 163)`, SHA-256
`baac338555524a741fe70327cd95d56b5abf0186cac0cc14e199239639053ebf`. This is a diagnostic of an
unreleased build on a newer host version, not a tuple result or support promotion. The report is
`results/2026-10-09-v1-0844-frontier/claude-code.tsv` (SHA-256
`a9c910258fa5cf8637a91dd0e9028f31ce17b8fb8310b6868221dd79f14b2356`) with its boundary load reading
`claude-code-load.txt` (SHA-256 `016f857b03aa77fbf5cf8a558ceca8d86e1c7cb0168273cb302fc8784cfbbaf8`).

### V1-1072 Claude Code requalification, 2026-10-09

The Claude Code tuple was rerun on darwin/arm64 for host `2.1.293` and adapter `0.3.0`, and passed all
nine cases with exit 0 between 01:47:57 and 01:48:07 UTC. The one-minute load was 26.85/26.67 on 12
CPUs. Plugin sources came from a clean checkout of `6cb98aa0bffab3d8246a02b87e52a82dceab85d5`.

- Current binary: the `corvint` from the published v1.0.0-rc.2 `corvint_darwin_arm64.tar.gz`
  (archive sha256 `5041bdd99897830029593527959bc39627fcd1cf4a10e192474f9e7887930393`, matching
  `SHA256SUMS`), binary sha256 `a7293de93d52e9e6296ab2eb42ac1b832901df6f0440f61118e84520fdee3e86`
  (`Corvint 1.0.0-rc.2 (build 360)`).
- N-1 binary: the published v1.0.0-rc.1 binary, sha256
  `baac338555524a741fe70327cd95d56b5abf0186cac0cc14e199239639053ebf` (`Corvint 1.0.0-rc.1 (build 163)`).

The report is `results/1.0.0-rc.2-darwin-arm64/claude-code.txt` (SHA-256
`cc757d03caea1c2ddc5f8ebabf38967ab7e758582a19f711d9ec4bc3efcf9c0b`). It supersedes the Claude Code row
of the rc.1 table above for the published matrix only. The CLI and Codex tuples were not rerun, so
they keep their rc.1 results.

A live compaction cycle is `NOT_RUN`: a nested `claude -p` session could not authenticate (OAuth
session expired). The `PreCompact`/`PostCompact` hooks therefore remain statically verified against
`2.1.267` only.

## Known gaps

- V1-0397: the historical Claude Code 2.1.267 / adapter 0.2.3 upgrade failure on
  rc1 build 154 occurred at reported host load 81–109 on 12 CPUs. Its received text was
  discarded, so the exact cause remains UNKNOWN. The same tuple subsequently passed at
  load 97–125; these observations establish neither a safe threshold nor load causation.
  A prior PostToolUse run reported `adapter-host-kill-deadline`, making watchdog degradation
  a testable hypothesis, not a diagnosis of the lost upgrade event. The deterministic fallback
  regression retains that cause text. Three subsequent consecutive diagnostic-candidate runs
  passed with recorded boundary loads above 80 (Results); the historical cause remains unknown.
  Code reading (2026-10-06) narrows it: the plugin fixture never runs `corvint index`, so every
  SessionStart misses the snapshot and builds the index in memory inside the adapter work bound
  (process start + 2 s declared kill − 400 ms reserve − 100 ms grace). An expiry there is named
  `dogfood-event-index-snapshot-stale`, or `adapter-host-kill-deadline` when the watchdog fires
  first; both are time bounds under `HLQ-V1-009`, so a recurrence is now retried and named.
- V1-0844: on the `1.0.0-rc.2` candidate `da78c157` (build 358, load average about 245 on 12
  CPUs) the Claude Code frontier case reported `enrolled incomplete Stop returned decision <nil>`;
  three reruns passed and the hook output was discarded. Proven from code and
  `TestClaudeAdapterStopDeadlineFailsOpenVisibly`: an enrolled Stop whose event deadline expires
  returns only `systemMessage` `Corvint FALLBACK degraded:
  corvint-event-rejected:dogfood-event-deadline; coding continues`, with no `decision`, which the
  pre-`HLQ-V1-009` runner reported as exactly that line; the watchdog path is the same shape with
  `adapter-host-kill-deadline`. Hypothesis, not observed: the rc.2 failure was that time-bound
  fail-open. The other no-decision outputs (no enrollment for the key, another session's notice, a
  non-time-bound rejection, an internal error) have no evident load dependence. The
  `TestClaudeNativeDogfoodLifecycle` deadline subtest forces the expiry after a real enrolled
  incomplete evaluation has decided `block` and gets exactly that fail-open output. On 2026-10-09 a
  real-host run under `HLQ-V1-009` passed (V1-0844 diagnostic run above); a probe of that host
  (runner plus `probe.patch`, timings in `probe-timings.tsv`, both in that results directory) timed
  40 enrolled incomplete Stop hook runs: all blocked, median 323 ms, maximum 553 ms, against the
  1.5 s adapter work bound. Its load (one-minute 26.14 at start, 20.44 at end, 12 CPUs) is an
  unretained observation. A slowdown of roughly
  5x reaches the bound, which the rc.2 load of about 245 makes plausible; the rc.2 output itself
  stays unobserved, so the cause remains the most likely hypothesis, not an observation.

- Codex runs a plugin hook only after the user trusts it interactively. An isolated home has no
  trust, so the Codex hook cases call the registered command directly (`HLQ-V1-004`).
- Codex's plugin CLI has no disable verb. Its uninstall case covers remove and marketplace remove
  only.
- Claude Code retires an uninstalled version in place with `.orphaned_at` and deletes it later. The
  uninstall case accepts that marker and does not wait for the deletion.
- `compatibility.json` in both plugin packages still says `lastConformance` `NOT_RUN`. Recording
  this result there changes the package bytes, so it needs an adapter version bump, which is outside
  this change.

## Traceability

| Requirement | Evidence |
|---|---|
| HLQ-V1-001, HLQ-V1-007 | `conformance/host-lifecycle-v1` header and report lines, and `prepare`; `TestSummaryExitRequiresAllNinePass`, `TestMissingEnvelopeRetainsDiagnostic`; a dirty `--source` exits 2 |
| HLQ-V1-002 | Results table; `TestEnvelopedReceipt`, `TestContextHookRetainsRejectedText`, `TestMissingCoreVerbs`, `TestListed`, `TestRowIsScopedToSelector`, `TestSessionKeyPattern` |
| HLQ-V1-003 | the runner's private environment and `lookPath`; the uninstall case reporting the removed binary unresolvable |
| HLQ-V1-004 | `TestReadHooks`; the discovery case |
| HLQ-V1-005, HLQ-V1-008 | Results and the reports under `conformance/host-lifecycle-v1/results/`; support stays FALLBACK in both `compatibility.json` files |
| HLQ-V1-006 | the context, change, frontier and uninstall cases |
| HLQ-V1-009 | `TestHookTimeBoundDegradation`, `TestDegradationCode`; `TestClaudeAdapterStopDeadlineFailsOpenVisibly` and the enrolled-Stop deadline subtest of `TestClaudeNativeDogfoodLifecycle` in `cmd/corvint` pin the fail-open Stop shape the retry keys on |

## Rollback

Revert the change that introduced this contract: this file, its `INDEX.json` entry and `README.md`
row, the regenerated `REQUIREMENTS.tsv`, the 1.0 spec traceability row, the BUILD-LOG entry and
`conformance/host-lifecycle-v1`. The runner writes only into its own temporary workspace, so no
repository, host or trace state needs repair.
