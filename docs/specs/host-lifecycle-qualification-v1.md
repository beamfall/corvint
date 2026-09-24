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
- Exists: this contract and `conformance/host-lifecycle-v1`; all three tuples PASS on darwin/arm64 with corvint 0.8.0 (see Results)
- Blocked on: a run on the next release with its reports retained (`HLQ-V1-008`); linux tuples and live model-session cases are NOT_RUN
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
  1 otherwise, and 2 on a usage or setup error.
- **HLQ-V1-008:** A result MUST be recorded here with the digests of both binaries and the source
  revision. Each tuple's raw `--report` file MUST be kept under
  `conformance/host-lifecycle-v1/results/`, and its sha256 recorded with the result, so the verdict
  can be re-derived from the repository. A result becomes stale when the host version, the adapter
  version, or the corvint release changes, and the affected tuple needs a new run.

## Runner

```sh
GOTOOLCHAIN=local go run ./conformance/host-lifecycle-v1 \
  --host cli|claude-code|codex --corvint FILE --base-corvint N1_FILE --source CHECKOUT --report FILE
```

`--source` is a clean checkout whose `integrations/` supplies the plugin package. Without
`--base-corvint`, the upgrade case is `NOT_RUN`, so the tuple does not pass.

## Results

Run on 2026-09-24 on darwin/arm64 (Darwin 25.6.0). Plugin sources came from a clean checkout of
`e667812381ab4d5c860dde753e0a7580ec3c975d`; the Claude Code and Codex packages are unchanged since
`df66aba4`. The
current binary is the `corvint` from the published v0.8.0 `corvint_darwin_arm64.tar.gz`, sha256
`95ae7446dd249c659db3a0571b39e05dee5ba83f113cf061f1a20cd0604a710e` (`Corvint 0.8.0 (build 65)`).
The N-1 binary is the `corvint` from the published v0.7.0 archive, sha256
`5fdbab207f6d15bd8ef341365642769cb58a11a76d935c37df77776ad0d09bad` (`Corvint 0.7.0 (build 46)`).

| Tuple | Host version | Adapter | install | discovery | context | expansion | change | frontier | degradation | upgrade | uninstall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| plain CLI, native | none | none | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS |
| Codex CLI, plugin | 0.153.2 | 0.2.2 | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS |
| Claude Code, plugin | 2.1.267 | 0.2.3 | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS | PASS |

These 0.8.0 results were transcribed before `HLQ-V1-008` required retained reports, so no runner
report backs them. They become stale at the next corvint release.

Every other tuple is `NOT_RUN`. This includes linux/amd64 and linux/arm64, other host versions, and
the Gemini CLI, OpenCode and Pi adapters.

Supplementary live observations, not part of the nine cases:

- Codex CLI 0.153.2 (the operator's own install, `codex exec --json`) in a fixture repository
  answered one prompt. Its session rollout holds two developer messages carrying the repository-data
  envelope, one from SessionStart and one from UserPromptSubmit.
- Claude Code 2.1.267 (`claude -p --include-hook-events`) reported SessionStart and
  UserPromptSubmit hook responses with Corvint context. The model call itself failed on an expired
  OAuth session, so no model turn was observed.

## Known gaps

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
| HLQ-V1-001, HLQ-V1-007 | `conformance/host-lifecycle-v1` header and report lines, and `prepare`; `TestSummaryExitRequiresAllNinePass`; a dirty `--source` exits 2 |
| HLQ-V1-002 | Results table; `TestEnvelopedReceipt`, `TestMissingCoreVerbs`, `TestListed`, `TestRowIsScopedToSelector`, `TestSessionKeyPattern` |
| HLQ-V1-003 | the runner's private environment and `lookPath`; the uninstall case reporting the removed binary unresolvable |
| HLQ-V1-004 | `TestReadHooks`; the discovery case |
| HLQ-V1-005, HLQ-V1-008 | Results and the reports under `conformance/host-lifecycle-v1/results/`; support stays FALLBACK in both `compatibility.json` files |
| HLQ-V1-006 | the context, change, frontier and uninstall cases |

## Rollback

Revert the change that introduced this contract: this file, its `INDEX.json` entry and `README.md`
row, the regenerated `REQUIREMENTS.tsv`, the 1.0 spec traceability row, the BUILD-LOG entry and
`conformance/host-lifecycle-v1`. The runner writes only into its own temporary workspace, so no
repository, host or trace state needs repair.
