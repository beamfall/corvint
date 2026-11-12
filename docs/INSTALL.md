# Install and try Corvint

See the [release inventory](https://github.com/beamfall/corvint/releases) for the latest
published Corvint version and its attached assets. The instructions below apply only when
the corresponding versioned release assets are available. Use the exact release's attached artifact
and installed qualification evidence to determine which platforms and optional workflows were
tested; absence of that evidence is not qualification. To try the source directly, use the
[README source example](../README.md#try-it-in-sixty-seconds).

## Install all native tools from source

The all-tool install also includes `corvint-cli`, an opt-in discovery adapter. For example,
`corvint-cli --binary corvint-analyzer-go --help` serves offline usage without changing that
analyzer's native protocol. `corvint-cli --binary BINARY --version --json` reports the common
CLI release line; direct analyzer version receipts remain native. Use direct invocation for
protocol traffic and long operations. The adapter bounds short CLI execution to 30 seconds
and 1 MiB per output stream, and retains failed native streams/status inside its coded refusal.

With the pinned Go toolchain, run `make install-local` from the source checkout. It builds every
native `cmd/` executable from that tree and installs them into `~/.local/bin`, including
all provider tools. `corvint-tasks` builds from [beamfall/corvint-tasks](https://github.com/beamfall/corvint-tasks)
(decision 0594). Override the destination with
`make install-local LOCAL_BIN_DIR=/absolute/bin` and add that directory to `PATH`. Existing
binaries are replaced atomically with one bounded rollback copy per tool. Historical `.bak-*`
files made by other installers are not owned or removed by this command. Core and companions
share the `VERSION` release line; experimental delivery is still stated by their catalogs.

## Core archive and the core set

Running the core needs Git and a supported local Git repository. A Go compiler is needed only
for building from source. Choose the archives matching the operating system and CPU:

| System | `corvint` archive | Core-set app archives |
|---|---|---|
| macOS, Apple silicon | `corvint_darwin_arm64.tar.gz` | `<app>_darwin_arm64.tar.gz` |
| macOS, Intel | `corvint_darwin_amd64.tar.gz` | none: not a 1.0 platform (`PROD-INT-006`); the core-set install refuses |
| Linux, x86-64 | `corvint_linux_amd64.tar.gz` | `<app>_linux_amd64.tar.gz` |
| Linux, ARM64 | `corvint_linux_arm64.tar.gz` | `<app>_linux_arm64.tar.gz` |

The `corvint` archive holds the one `corvint` executable. `corvint docs` is an exec shim onto the
`corvint-docs` executable, and the MCP servers are their own executables, so a release is the
`corvint` archive plus one archive per core-set app: `corvint-docs`, `corvint-mcp` and
`corvint-docs-mcp` (the catalog's `core` version set, decision 0594 item 8; `ARTIFACT-BIN-V0-509`,
`ARTIFACT-BIN-V0-522`). The other executables under `cmd/` are not released in 1.0
(`ARTIFACT-BIN-V0-520`).
Install the set together, in one directory: `corvint` looks for its apps beside its own
executable first, and a `corvint` installed alone answers every `corvint docs` verb with
`not-installed`. All members of one release carry one version; `corvint` refuses, with
`version-mismatch`, to run a `corvint-docs` beside it from another release.

These names describe build targets, not proof that every target has passed a native test.
Windows is excluded from this release set. Keep the archive's notices and checksum files.
Release archives are unsigned: checksum agreement detects changed bytes, not publisher identity.

For example, with the macOS Apple silicon archive and release `SHA256SUMS` in the same directory:

```sh
# Verify only the archive you downloaded; the manifest may list other build targets.
awk '$2 == "corvint_darwin_arm64.tar.gz"' SHA256SUMS > selected.sha256
test "$(wc -l < selected.sha256 | tr -d ' ')" = 1 &&
  shasum -a 256 -c selected.sha256
```

Continue only if the command succeeds and reports `OK`; repeat it for each core-set app archive
you downloaded. Extract each archive into its own directory, verify the binary against the
checksum inside the archive, then place every verified executable in one install directory:

```sh
mkdir -p corvint-<version>/bin &&
  for archive in corvint corvint-docs corvint-mcp corvint-docs-mcp; do
    tar -xzf "${archive}_darwin_arm64.tar.gz" -C corvint-<version> &&
    ( cd "corvint-<version>/${archive}_darwin_arm64" && shasum -a 256 -c SHA256SUMS ) &&
    cp "corvint-<version>/${archive}_darwin_arm64/${archive}" corvint-<version>/bin/
  done &&
  corvint-<version>/bin/corvint --version &&
  corvint-<version>/bin/corvint-docs --version &&
  corvint-<version>/bin/corvint docs --help
```

Expect `Corvint <version> (build N, commit SHA)` and `corvint-docs <version> (build N, commit
SHA)` with the same version, build number and commit, matching the release manifest. Keep the
extracted directories for their notices, and add `corvint-<version>/bin` to `PATH`, or copy the
verified executables together to a directory you already use for local tools; they must stay in
one directory. Use `command -v corvint` and `corvint --version` to check which binary runs. No
account, service installation, database, model download or default network connection is
required.

Corvint uses `corvint-*` wire/profile identities, `corvint.*` MCP tools, and canonical
`.corvint` and `.context-corvint` repository paths. Release archives, package coordinates, and optional
workflow bundles remain unpublished unless an exact release and its attached qualification evidence
say otherwise.

### Qualified local candidate and versioned install

`corvint-release-candidate` closes the verified core gate output, the per-binary release output
holding `release-manifest.json` and the core-set app archives (`make release-binary` for each
member of the `core` set, then `make release-manifest`), and the source root's Corvint source into
a Core-only `corvint-v<version>-core` candidate (profile `corvint-core-release-candidate/1`). The
combined `corvint-v<version>-qualified` candidate and its `-companion-dir` input were retired with
the companion bundle by decision 0594. The candidate refuses an app recorded at another version or
commit than the core, so the set it carries is one release. Verify its
top-level `SHA256SUMS`, then read `MANIFEST.json` and `QUALIFICATION.json`; `NOT_RUN` is never a
platform pass. The candidate is local evidence, not a tag, signature, upload, publication or
promotion.

```sh
go run ./cmd/corvint-release-candidate \
  -core-dir /absolute/core-gate-output \
  -apps-dir /absolute/release-binary-output \
  -source-root /absolute/corvint-checkout \
  -scratch /absolute/private-scratch \
  -output-parent /absolute/candidates \
  -version <version>
```

`corvint-release-install -candidate /absolute/candidate -store /absolute/store` reverifies that
closed set and installs the matching host core archive with every core-set app beside it at
`<store>/corvint/<version>/<goos>-<goarch>`, one `SHA256SUMS` row per executable; it refuses the
install when an app's `--version` is not the core's version or the installed `corvint docs --help`
fails, retaining nothing. It refuses an existing destination and never creates
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
citation. The [README quickstart](../README.md#try-it-in-sixty-seconds) describes what to
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

Read commands do not change repository or trace state, with two bounded private exceptions under
`.corvint/`: `self-observations.jsonl` (an `unsupported-*` refusal may append one row) and
`unplanned-reads.jsonl` (host-adapter hooks append only while the operator marker
`.corvint/unplanned-reads.enabled` exists). A read-only operator should expect those two ledgers (and the
`.self-observations.*` and `.unplanned-reads.*` temporaries their writers create and remove) to change and
nothing else under a read: they are never an input to ranking, evidence, authority or trace
state. [Automation](AUTOMATION.md) lists each write condition.

## Standalone Tasks archive

Corvint Tasks moved to [beamfall/corvint-tasks](https://github.com/beamfall/corvint-tasks) with decision 0594; its
archive builder and install steps live there.

## Optional workflow bundle

Decision 0594 retired the companion bundle assembler and its gate, so no workflow bundle is
currently assembled; this section describes the earlier plan and stays only for bundles already
retained. The planned, separately assembled **macOS arm64** workflow bundle would contain seven native binaries:
`corvint`, `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`, `corvint-js-test-provider`, `corvint-go-test-provider` and the experimental
`corvint-ruby-test-provider` (profile `corvint-companion-bundle/3`, `CRB-V0-020`; its smoke checks only
the usage line, RSpec execution is not qualified); a VS Code VSIX;
and raw Codex, Claude Code, Gemini CLI and OpenCode integration trees marked `FALLBACK`.
The host applications and their external dependencies are not bundled. Install it only when the
exact release's attached evidence qualifies the archive and the workflow you intend to use.

The retired companion gate named archives `corvint-companion-<corvint-commit-prefix>-<taskman-commit-prefix>-darwin-arm64-<UTC-timestamp>.tar.gz`
and supplied an adjacent `.tar.gz.sha256` file. Use the actual retained filename, verify that
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
| `corvint-mcp` | Impact, status and help; opt-in flows, test-evidence, test-validity and documentation groups |
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

### Work queue adoption

The work queue moved with Corvint Tasks to
[beamfall/corvint-tasks](https://github.com/beamfall/corvint-tasks) (decision 0594); its adoption
steps live there.

## Optional operator updater

The separate experimental `corvint-update` companion checks official public releases and applies
one component at a time. Core remains offline. Build from source with
`GOTOOLCHAIN=local go build -o /absolute/path/corvint-update ./cmd/corvint-update`.

```sh
corvint-update check --allow-network --component core
corvint-update check --allow-network --component tasks
corvint-update apply --allow-network --component tasks
corvint-update rollback --component tasks
```

Default executable paths are under `~/.local/bin`; retained transactions/archive evidence live
under `~/.local/share/corvint/updates`. Explicit `--bin-dir` / `--state-dir` support disposable
qualification or another operator-owned installation. Check is read-only. Apply requires network
opt-in, verifies archive and internal checksums, checks host/build identity, probes version/help,
retains prior bytes and a prepared transaction, then atomically switches the one destination.
Rollback needs no network and refuses foreign destination or saved-byte drift. Failures before
activation leave the destination untouched. Downgrades, unknown builds, symlinked managed paths,
unsafe archives and incomplete release discovery refuse automatic activation.

Index snapshots are keyed by the Core executable, so a Core apply or rollback then runs the new
binary's `corvint --root ROOT index --if-stale` in the foreground for each `--refresh-index ROOT`
and for the checkout you run it from when that checkout is already indexed. The result's
`indexRefresh` lists each outcome (`built`, `fresh`, `skipped`, `failed`); a failed refresh does
not undo the switch. Other checkouts, and any Core binary copied into place by hand, still need
an explicit `corvint index --if-stale`.

Core `v*` and standalone Tasks `tasks-dev-*` channels are selected separately, each from its own
official source: Core from the GitHub releases of `beamfall/corvint` and Tasks from those of
`beamfall/corvint-tasks`, since Core no longer bundles Tasks (decision 0594). RC (`vX.Y.Z-rc.N`) and developer Tasks
releases are published as prereleases and are admitted, so checking them never waits for a stable
release. Every result names `source` and `channel`; when no release can be selected, freshness
stays `UNKNOWN` and `unknownReason` says why and what would resolve it (for example, that the
channel has no published release yet). `--github-token-env NAME` authenticates the release API
reads with the token in that variable (sent to `api.github.com` only, never to download hosts).
Developer Tasks archives retain their explicit unverified/runtime-qualification limits. Checksums detect byte
changes, not publisher identity. Bounded inventory exhaustion/offline state reports freshness
UNKNOWN. This optional operator tool introduces no daemon, config rewrite, store migration,
publisher authentication or power-loss durability; callers schedule it and manage active hosts.
See [Operator update V0](specs/operator-update-v0.md) for the requirements and evidence boundary.

## Upgrade, retry and remove

Install a new version into a new directory. Stop active provider processes and let the MCP
client close its children; disable editor live tests before changing tools. Keep the previous
directory and any ticket data, verify the new archive and version, then update your PATH and
absolute editor/MCP paths. Re-select and pin the editor executable when its identity changes.
Do not mix companions from different manifests. Roll back by restoring the previous directory,
VSIX and configured paths; no persisted-state migration is introduced by this release.

Support window: for every 0.x version only the latest published release receives security fixes
(see the [security policy](../SECURITY.md)). The qualified upgrade path is from the previous
published release; an older version reads a repository after a newer one wrote its snapshot, so
rollback needs no index cleanup.

An interrupted download or extraction is not an installation: retry into a fresh directory and
repeat checksum verification before executing anything. For an interrupted test run, wait for
cleanup, inspect its incomplete/cancelled state, then save again or restart the explicit session.
Never treat a stale prior pass as the current run's result.

To uninstall, disable live tests, close MCP children, uninstall the Corvint VSIX,
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

The index snapshot store is `corvint/index` under the Git common directory (`.git/corvint/index` in
a plain clone), shared by every linked worktree; a worktree whose common directory cannot be
resolved keeps `.corvint/index` as its store. Where the common directory resolves, an older
`.corvint/index` left by a previous release is no longer read and may be deleted by hand. The store
is created 0755 with 0600 files, so when several OS users share one common directory
(`core.sharedRepository`), a second user's `corvint index` fails and their reads miss.

For a damaged derived index, retain the failing receipt and a backup of the snapshot store, then run
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
treats a truncated or byte-corrupted snapshot `*.gob` as a miss, rebuilds in memory, exits 0
with the same packet bytes, and never rewrites the file; `corvint index --if-stale` reports
`mutates: true`, writes a fresh snapshot of the same size, and the next `--if-stale` reports
`fresh`. The rebuilt snapshot is not byte-identical to the original, so compare packets, not
snapshot files, when checking a recovery.

`script/check-install-lifecycle.sh` runs this whole lifecycle in a temporary directory against one
release archive (`CORVINT_LIFECYCLE_ARCHIVE`) or one binary (`CORVINT_LIFECYCLE_BINARY`): verified
install, first index and read, upgrade into a second store, rollback, uninstall with `.corvint`
and the snapshot store retained, backup and restore of both, and both corruption cases. With
`CORVINT_LIFECYCLE_UPGRADE_BINARY` set to a different release, the upgrade's packet must be
non-empty and is compared to the packet that release builds from a cold index and reported `packet=identical` or
`packet=changed`, since releases may change the packet wire. It prints one `step NAME: ok`
line per step and a final `SUMMARY status=PASS|FAIL` line; it does not qualify a future release,
every supported platform, or restoration of arbitrary ticket-store data. See the
[release runbook](RELEASE-RUNBOOK.md) and [security/support boundaries](SECURITY.md).

## Experimental offline upgrade checklist

In the Corvint source checkout, compare the commit IDs recorded for two installed builds:

```sh
corvint --root /path/to/corvint changes --from OLD_BUILD_SHA --to NEW_BUILD_SHA \
  --refs-file /path/to/adopter/workarounds.md --refs-file /path/to/adopter/holds.txt
```

The read-only command never fetches. Its bounded JSON lists first-parent merge PRs and local
Closes/Fixes/Resolves/Refs issue evidence. Explicit closure evidence can produce a
"workaround may be removable" hint with file, line and closing PR; inspect the fix before removing
anything. Refs alone is unknown. Repeat explicit regular files; globs and symlinks are unsupported
(file scanning is supported on macOS/Linux). `truncated: true` means the checklist is incomplete.
Unavailable build objects produce `changes-build-unavailable`; use a checkout already containing
both commits. Each PR row lists `new_flags` added by literal in non-test `cmd/corvint` Go source,
computed from the local diff (a heuristic, not a verified CLI surface diff). Corvint never queries
GitHub: to annotate issues with `github_state`, export the state yourself and pass the file:

```sh
gh issue list --repo beamfall/corvint --state all --limit 1000 --json number,state > issues.json
corvint --root /path/to/corvint changes --from OLD_BUILD_SHA --to NEW_BUILD_SHA \
  --refs-file /path/to/adopter/workarounds.md --issue-state-file /path/to/issues.json
```

A hint for an issue the file reports `OPEN` asks for review instead of suggesting removal.
Without the file `github_state` is `NOT_OBSERVED`. Reopen/revert semantics, closures before the
range and per-PR new commands are not observed. See the proposed
[Upgrade Checklist V0](specs/upgrade-checklist-v0.md) contract (V1-1142, V1-1158).
