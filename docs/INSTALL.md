# Install and try Corvint

Corvint `0.5.0a3` is an experimental alpha. The instructions below apply only when the corresponding
versioned release assets are available. Use the exact release's attached artifact and installed
qualification evidence to determine which platforms and optional workflows were tested; absence
of that evidence is not qualification. To try the source directly, use the
[README source example](../README.md#try-it-on-this-repository).

## Core archive

Running the core needs Git and a supported local Git repository. A Go compiler is needed only
for building from source. Choose the archive matching the operating system and CPU:

| System | Archive |
|---|---|
| macOS, Apple silicon | `corvint_darwin_arm64.tar.gz` |
| macOS, Intel | `corvint_darwin_amd64.tar.gz` |
| Linux, x86-64 | `corvint_linux_amd64.tar.gz` |
| Linux, ARM64 | `corvint_linux_arm64.tar.gz` |

These names describe build targets, not proof that every target has passed a native test.
Windows is excluded from this release set. Keep the archive's notices and checksum files.
The alpha is unsigned: checksum agreement detects changed bytes, not publisher identity.

For example, with the macOS Apple silicon archive and release `SHA256SUMS` in the same directory:

```sh
# Verify only the archive you downloaded; the manifest may list other build targets.
awk '$2 == "corvint_darwin_arm64.tar.gz"' SHA256SUMS > selected.sha256
test "$(wc -l < selected.sha256 | tr -d ' ')" = 1 &&
  shasum -a 256 -c selected.sha256
```

Continue only if the command succeeds and reports `OK`. Extract into a new directory, then verify
the binary against the checksum inside the archive:

```sh
mkdir corvint-0.5.0a3
tar -xzf corvint_darwin_arm64.tar.gz -C corvint-0.5.0a3
cd corvint-0.5.0a3/corvint_darwin_arm64
shasum -a 256 -c SHA256SUMS
./corvint --version
```

Expect `Corvint 0.5.0a3 (build N)`, where `N` is the release manifest's exact build number. Keep this directory and add its absolute path to `PATH`, or copy the
verified executable to a directory you already use for local tools. Use `command -v corvint`
and `corvint --version` to check which binary runs. No account, service installation, database,
model download or default network connection is required.

Corvint uses `corvint-*` wire/profile identities, `corvint.*` MCP tools, and canonical
`.corvint` and `.context-corvint` repository paths. Release archives, package coordinates, and optional
workflow bundles remain unpublished unless an exact release and its attached qualification evidence
say otherwise.

### Qualified local candidate and versioned install

`corvint-release-candidate` closes the verified core and companion outputs into
`corvint-v<version>-qualified`. Verify its top-level `SHA256SUMS`, then read `MANIFEST.json` and
`QUALIFICATION.json`; `NOT_RUN` is never a platform pass. The candidate is local evidence, not a
tag, signature, upload, publication or promotion.

```sh
go run ./cmd/corvint-release-candidate \
  -core-dir /absolute/core-gate-output \
  -companion-dir /absolute/companion-retained-output \
  -source-root /absolute/corvint-checkout \
  -scratch /absolute/private-scratch \
  -output-parent /absolute/candidates \
  -version 0.5.0a3
```

`corvint-release-install -candidate /absolute/candidate -store /absolute/store` reverifies that
closed set and installs only the matching host core archive at
`<store>/corvint/<version>/<goos>-<goarch>`. It refuses an existing destination and never creates
or changes `current` or `latest`. Use a canonical absolute store path (resolve system aliases such
as `/var` to `/private/var` on macOS), owned exclusively by you during installation. Symlink
components, case-fold aliases in managed components, non-directory parents, candidate/store overlap
and managed parents with more than 4,096 entries are refused. Keep old version directories for coexistence and rollback; switch
the explicit absolute path used by your shell or host configuration only after checking
`corvint --version`. Removing an old path is a separate operator action.

## First useful result

From a clean Corvint source checkout, with the binary on `PATH`:

```sh
corvint impact cmd/corvint/main.go --limit 5
```

Inspect the requested path, related test paths, Git blob identities and inclusion reasons in
`context.results`. Read freshness and coverage, including omitted results, before following a
citation. The [README's abridged receipt](../README.md#try-it-on-this-repository) shows what to
look for; exact identities and counts vary with the checkout.

In another supported repository, replace the path with a tracked source file. Put the global
root option before the verb: `corvint --root /absolute/path/to/repo impact path/to/file.go`.
An unsupported profile or abstention is an explicit boundary, not a successful answer to the
underlying question. Preserve the error and use the repository's own documentation or tests.

On a clean repository, experimental `corvint overview` summarizes detected languages,
entrypoints, manifests, test conventions, inferred features and index freshness.
`corvint features` narrows that to inferred feature candidates. Both expose source receipts
and omissions; candidates do not become accepted requirements. For an immutable committed range,
`corvint review --base FULL_BASE_SHA --max-refs 8` adds affected-test advice and bounded local
branch overlap hints. Replace `FULL_BASE_SHA` with the full ancestor commit immediately before
the changes. These commands refuse dirty trees, do not execute suggested next calls, and do not
close review or test obligations. See the [guidance contract](specs/repository-guidance-v0.md).

## Optional workflow bundle

The planned, separately assembled **macOS arm64** workflow bundle will contain nine native binaries:
`corvint`, `corvint-console`, `corvint-dashboard-snapshot`, `corvint-tasks`, `corvint-mcp`, `corvint-docs-mcp`,
`corvint-test-validity-mcp`, `corvint-js-test-provider` and `corvint-go-test-provider`; a VS Code VSIX;
and raw Codex, Claude Code, Gemini CLI and OpenCode integration trees marked `FALLBACK`.
The host applications and their external dependencies are not bundled. Install it only when the
exact release's attached evidence qualifies the archive and the workflow you intend to use.

The companion gate names archives `corvint-companion-<corvint-commit-prefix>-<taskman-commit-prefix>-darwin-arm64-<UTC-timestamp>.tar.gz`
and supplies an adjacent `.tar.gz.sha256` file. Use the actual retained filename, verify that
checksum, extract into a fresh directory, and follow its `README.md`, `MANIFEST.json` and internal
`SHA256SUMS`. Retain the whole directory so companion paths, source archives and notices remain
together. Extraction does not install a VSIX or configure an agent host.

### Editor and test feedback

1. Install the bundle's VSIX with VS Code's **Extensions: Install from VSIX** command. Use a
   trusted, local file workspace and configure **Corvint: Executable Path** to the absolute
   `corvint` path. Run **Corvint: Select and Pin Executable**, then **Corvint: Query Evidence**
   to verify the engine connection.
2. Install your project's test dependencies. JS/TS providers require their declared Node,
   Vitest or Playwright versions; E2E also requires the configured browser and app server.
   Go needs its project toolchain. The bundle does not supply these toolchains.
3. Follow the [extension's unit configuration](../extensions/vscode/README.md#automatic-unit-and-e2e-feedback)
   or the [complete unit/E2E fixture](../conformance/interactive-alpha/README.md), replacing
   example paths with your own. Set the provider command, suite, toolchain paths and watched paths,
   then opt in with `corvint.liveTests.enabled: true`.
4. Save an eligible file. Inspect the completed result in Test Explorer and failure diagnostics;
   save again after a fix to rerun the full configured suite. Rapid saves coalesce. External
   filesystem changes do not trigger this editor-save loop. Disable `corvint.liveTests.enabled`
   to stop the provider and pending reruns.

To let agents read completed observations through `corvint-test-validity-mcp`, explicitly set
`corvint.liveTests.retainEvidence: true` (default: `false`); the linked unit configuration includes
this setting. A green execution result does not prove freshness, test adequacy or authority;
unknown axes stay visible. Go's
foreground provider is a separate opt-in, non-policy preview. Host compatibility remains
`FALLBACK`; successful installation alone does not qualify a host/platform tuple.

### Agent tools and source documentation

Use a client that supports the servers' experimental MCP `2026-07-28` stdio contract. Configure
an absolute executable path and `args: ["--root", "/absolute/path/to/repository"]` separately
for each desired server:

| Executable | Tools |
|---|---|
| `corvint-mcp` | Read-only context, Go impact and repository status |
| `corvint-docs-mcp` | `corvint.docs_draft` and `corvint.docs_consume` |
| `corvint-test-validity-mcp` | Discovery and projection of retained test evidence |

See [MCP setup and protocol admission](MCP-SERVER.md) before configuring a client. These servers
use `server/discover`, not the legacy `initialize` lifecycle. Do not assume every MCP host supports
that protocol. A process is bound to one root; V0 root admission can reject linked worktrees.
Stop and reconfigure the process to change roots. A missing executable, unsupported root or
protocol mismatch is a setup failure, not empty evidence.

Source documentation needs a committed owning spec with an admitted Agent digest and an exported
Go package. Drafts cite source; consumption rechecks the exact draft bytes. Generated prose does
not accept human intent. The [source documentation contract](specs/source-documentation-draft-v0.md#cli-contract)
contains a complete CLI draft/consume example and the same MCP tool semantics.

To refresh a source-bound page, use the separate experimental maintenance command:

```sh
corvint docs maintain --page docs/page.md --source docs/specs/owner.md \
  --package internal/example --enable --preview
```

Replace the page, committed owning spec and Go package paths with your project’s paths. Inspect
the proposed bytes and receipt, then invoke with `--apply` instead of `--preview` to authorize
one bounded session. It updates only the selected generated block and preserves surrounding
human prose. A page change detected between that session's read and write refuses with
`maintenance-conflict`; inspect the new page before retrying. A separate `--apply` invocation
computes a new preview, so an earlier CLI preview is not a retained approval token.

For automatic refresh on macOS or Linux, run an explicitly bounded foreground watch:

```sh
corvint docs maintain --page docs/page.md --source docs/specs/owner.md \
  --package internal/example --enable --apply --watch \
  --max-writes 20 --max-wall-clock 1h
```

It applies an eligible block immediately, then checks committed source changes once per second.
Uncommitted source edits do not trigger updates. Keep the selected page unchanged while the watch
runs: an outside edit or deletion stops it with `maintenance-conflict`. Ctrl-C stops the session;
it also stops when either limit is reached. No service is installed. Native Windows refuses watch
before accessing the page; the one-shot command remains available.

The watch is experimental. A source-built watch/update/MCP check has passed; consult the exact
release's installed qualification evidence for packaged status. The read-only docs MCP tools do
not trigger maintenance.

### Tickets and local console

`corvint-tasks` provides the separate local ticket store and roadmap. The optional console combines ticket,
specification and evidence views; it has no autonomous agent dispatch. Start it explicitly using
absolute paths to the extracted companions:

```sh
/path/to/bundle/bin/corvint-console --repo /absolute/path/to/repo \
  --tasks /path/to/bundle/bin/corvint-tasks \
  --snapshot /path/to/bundle/bin/corvint-dashboard-snapshot
```

Open its loopback address, default `http://127.0.0.1:7777`, in your browser. Stop it with Ctrl-C.
It installs no background service. Consult the bundle manifest and exact release evidence before
relying on a browser/platform tuple.

### Work queue adoption

`corvint work observe` needs a committed queue policy, worklist, and adapter. Create them once per
repository and explicitly bind the installed executable. `~/.local/bin`, `/opt/homebrew/bin`, and
`/usr/local/bin` are supported when the selected path is canonical and safe:

```sh
corvint work init --repository NAME --corvint-executable "$(command -v corvint)"
git add .corvint && git commit -m "Adopt the Corvint work queue"
corvint work observe
corvint work propose-wave --envelope capacity.json --limit 4
```

`init` refuses if any of the three files already exists. Add tickets to `.corvint/worklist.json`
(`{"profile":"corvint-worklist/0","tickets":[{"id":…,"title":…,"body":…,"touchPaths":[…]}]}`)
and commit before observing. The capacity envelope names `capacity:NAME:worklist:agent`, the
capability `capability:NAME:worklist:agent`, and `repo:NAME`. Neither command dispatches, leases,
merges, or runs anything. Init rejects relative paths, symlinks, unsafe or writable parent
components, repository-local executables, missing files, and non-Corvint builds. It records the
executable path, SHA-256, version/build, and source identity in the adapter; observation executes only
the verified private exact-byte materialization under a closed environment, never ambient `PATH`.

After replacing or moving Corvint, explicitly regenerate that reviewed binding and commit it:

```sh
corvint work rebind --corvint-executable "$(command -v corvint)"
git add .corvint/work-queue-adapter && git commit -m "Rebind Corvint work-queue executable"
```

| What you see | Cause |
|---|---|
| `ERROR` / `SOURCE_UNQUALIFIED` | no committed adoption, uncommitted worktree changes, or the bound executable path/bytes/version/build/source identity changed; review and commit an explicit `work rebind` |
| `ERROR` / `ADAPTER_FAILED` | the canonical bound adapter exited, timed out, or exceeded an output bound |
| observation `STALE`, empty proposal | the worklist or commit changed while observing |
| `VALIDATED_AT` with `CONTAINMENT_UNQUALIFIED`, `EXECUTABLE_IDENTITY_UNQUALIFIED`, `MUTATION_ENFORCEMENT_UNQUALIFIED`, `NETWORK_UNOBSERVED` | expected: the reviewed binding detects drift, while Darwin cannot claim VPO-V0-024 exact-object execution and the other local axes stay unknown |

## Upgrade, retry and remove

Install a new version into a new directory. Stop active console/provider processes and let the MCP
client close its children; disable editor live tests before changing tools. Keep the previous
directory and any ticket data, verify the new archive and version, then update your PATH and
absolute editor/MCP paths. Re-select and pin the editor executable when its identity changes.
Do not mix companions from different manifests. Roll back by restoring the previous directory,
VSIX and configured paths; no persisted-state migration is introduced by this alpha.

Support window: for every 0.x version only the latest published release receives security fixes
(see the [security policy](../SECURITY.md)). The qualified upgrade path is from the previous
published release; an older version reads a repository after a newer one wrote its snapshot, so
rollback needs no index cleanup.

An interrupted download or extraction is not an installation: retry into a fresh directory and
repeat checksum verification before executing anything. For an interrupted test run, wait for
cleanup, inspect its incomplete/cancelled state, then save again or restart the explicit session.
Never treat a stale prior pass as the current run's result.

To uninstall, disable live tests, close the console and MCP children, uninstall the Corvint VSIX,
remove the Corvint entries you added to host configuration and PATH, then remove your extracted
tool directory or copied executables. Preserve repository files, ticket stores and retained
`.corvint` evidence unless you separately intend to delete them. No system service needs removal.

For support, retain the version, OS/CPU, failing command and exact error, receipt and manifest
identities, and a minimal reproducer. Include missing prerequisites and unknown qualification
axes. Share this through the repository's issue tracker when available or with the maintainer;
review receipts and logs for private repository paths or content before sharing.

## Backup and corrupted-state recovery

Keep the complete verified candidate, including sources, licenses, manifests, checksums and
qualification evidence, on a separate backup medium. Checksums detect corruption, not authenticity;
retain the independently obtained expected digest and provenance. Verify the copied candidate
before executing its installer. Back up repositories (including Git metadata and uncommitted work),
`.taskman`, `.context-corvint`, and `.corvint` separately while their writers are stopped. They carry
intent, tickets, learning or retained evidence that an executable archive cannot restore.

For a corrupted executable, stop its users and select the retained previous version by absolute
path. Do not overwrite the damaged version: preserve it for diagnosis and install the verified
candidate into a fresh canonical store. The installer deliberately refuses even a corrupt existing
destination. Restoring a candidate backup uses that same installation path and verification; there
is no alternate restore archive format or automatic downgrade migration.

For a damaged derived index, retain the failing receipt and a backup of `.corvint/index`, then run
`corvint index --if-stale` explicitly. A symlinked index path is refused: review it before any manual
quarantine. Do not delete all of `.corvint` to rebuild an index. Corrupt ticket/trace/evidence state
requires restoring a consistent backup with matching tools; index rebuilding cannot repair it.
Preserve refusals and ask for diagnosis if the backup's compatibility is unknown.

A normally failed or cancelled candidate install removes its own `.install-*` staging directory
and leaves existing versions intact. After a crash or force kill, first establish that no installer
is active; inspect and quarantine only the abandoned staging directory you own. Automatic scavenging,
concurrent-writer safety and power-loss durability are not provided. Never remove the retained
candidate or another version as part of staging cleanup.

What a damaged snapshot does, measured (`SOP-V0-006` in
[Stable operations V0](specs/stable-operations-v0.md)): a read verb such as `corvint context`
treats a truncated or byte-corrupted `.corvint/index/*.gob` as a miss, rebuilds in memory, exits 0
with the same packet bytes, and never rewrites the file; `corvint index --if-stale` reports
`mutates: true`, writes a fresh snapshot of the same size, and the next `--if-stale` reports
`fresh`. The rebuilt snapshot is not byte-identical to the original, so compare packets, not
snapshot files, when checking a recovery.

`script/check-install-lifecycle.sh` runs this whole lifecycle in a temporary directory against one
release archive (`CORVINT_LIFECYCLE_ARCHIVE`) or one binary (`CORVINT_LIFECYCLE_BINARY`): verified
install, first index and read, upgrade into a second store, rollback, uninstall with `.corvint`
retained, backup and restore of `.corvint`, and both corruption cases. With
`CORVINT_LIFECYCLE_UPGRADE_BINARY` set to a different release, the upgrade's packet must be
non-empty and is compared to the packet that release builds from a cold index and reported `packet=identical` or
`packet=changed`, since releases may change the packet wire. It prints one `step NAME: ok`
line per step and a final `SUMMARY status=PASS|FAIL` line; it does not qualify a future release,
every supported platform, or restoration of arbitrary ticket-store data. See the
[release runbook](RELEASE-RUNBOOK.md) and [security/support boundaries](SECURITY.md).
