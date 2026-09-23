# Corvint 0.5.0a3 — public alpha release notes

These enduring notes describe the experimental alpha scope required by
`docs/specs/public-release-v0.md` PUB-V0-007. Availability, artifact identity and tested platform
status come only from versioned release assets and their attached qualification evidence. These
notes do not claim that publication occurred, authenticate the publisher or promote a command.

## Scope

Version `0.5.0a3` (`VERSION`). This is the owner-accepted public-alpha scope from
`docs/specs/public-release-v0.md`: the native Go CLI, automatic docs via MCP, automatic unit/E2E
test tracking for agent and VS Code, an optional local dashboard, and a task manager with a
roadmap. The release scope is accepted and its implementation details remain proposed. Individual
source implementations and focused checks do not qualify an assembled artifact or installed
workflow; use the exact release's attached evidence for those claims. See [installation and first
use](INSTALL.md) for the core archive path, optional tools, prerequisites, recovery and removal.

This alpha adds the Playwright 1.63 standard-device-spread regression from GitHub issue #49. A
project using `use: {...devices['Desktop Chrome']}` now retains its effective browser, viewport,
user agent, config digest and stable test identity while the separately qualified bundled-browser
tuple remains fail-closed on revision, manifest, path and executable digest drift.

## Experimental features

Everything in this list carries spec intent `proposed` and/or delivery `experimental` in
`docs/specs/README.md` / `docs/specs/INDEX.json`, or is one of decision 0089's eight proposed
slices. Release qualification does not change those intent, delivery or promotion labels.

- **Native Go CLI cutover** (`docs/specs/go-only-cutover-v0.md`, intent accepted / delivery
  experimental) — `query`, `context`, `impact`, `affected`, `prove`, `index` run on the Go engine;
  `make gate` now records a private receipt binding HEAD to that run's archive witness
  (`GOC-V0-010`), shown as the checklist's `full-gate` row. A release asset is qualified only when
  its attached evidence binds that run to the exact source and archive.
- **Retrieval quality** (`query`, `context`, learned task traces; `docs/V4-STATUS.md`,
  `docs/decisions/0307-alpha-release-bar-blind-v6-attempted-fail-2026-09-17.md`) — experimental
  and unqualified. The one blind-v6 attempt (2026-09-17, retrieval cycle line `c31d928e`, not
  integrated here) met the alpha bar on top-5 (0.571 vs exact-search baseline 0.343), abstention
  (5/10) and latency (7.19 s) but failed on must-exclude hits (7/36). The shipped engine carries no
  held-out retrieval qualification; do not rely on ranking or abstention.
- **Reproducible Go release archives** (`docs/specs/go-archive-gate-v0.md`, intent accepted /
  delivery implemented; `docs/specs/release-artifact-integrity-v0.md`, intent proposed overall /
  delivery experimental, with `ARTIFACT-GO-V0-001..009` accepted) — archives rebuild
  byte-identically on every `make gate` run, and real-command negative output injection (extra,
  missing and symlinked archives, plus an interrupted build) ran against real producer output on
  2026-09-13 via `script/go-archive-gate-injection_test.sh`; the integrity spec as a whole is not
  owner-accepted.
- **CEM 0.1 / 0.2** (intent proposed, delivery experimental) — reference producer, verifier,
  schemas and conformance vectors exist; independent third-party interoperability is unproven.
- **Host adapters** — Codex, Claude Code, Gemini CLI, OpenCode, Pi (`docs/specs/agent-harness-integration-v0.md`,
  delivery experimental) — developer preview; every host reports `FALLBACK`, none is `FULL`.
  Native local lifecycle evidence for Codex CLI and Pi is branch-local and not runtime authority.
- **MCP server `corvint-mcp`** (`docs/specs/mcp-server-2026-07-28-v0.md`, intent proposed) —
  source behavior remains experimental. Installed discovery and tool checks are
  release-evidence-specific and do not establish full protocol or host compatibility.
- **Automatic docs via MCP** (PUB-V0-009; `docs/specs/source-documentation-draft-v0.md`, intent
  proposed) — explicitly enabled `docs maintain` refreshes one generated page block. The
  experimental macOS/Linux `--watch --apply` profile responds to eligible committed source
  changes within explicit time/write limits and stops on outside page edits; one-shot preview
  and apply remain available. Separate MCP tools expose draft generation and source re-derivation.
  A real source-built watch/update/MCP check passed. Owner acceptance of the SDD contract,
  the separate renderer and combined installed maintenance/MCP qualification remain open.
- **Automatic Go unit/E2E test tracking** (PUB-V0-006; `docs/specs/go-live-test-provider-v0.md`,
  `docs/specs/live-proof-carrying-verification-v0.md`) — the underlying LPCV contract is accepted
  as a direction, but the LPCV spec itself records delivery as "not-started"; operation coverage,
  containment, network denial and the frozen Execute deadline are unqualified.
- **JS/TS unit + E2E tracking** (`docs/specs/js-live-test-provider-v0.md`, intent proposed) — needs
  its own numbered accepting decision, a frozen wire codec and CI qualification; the fixture app
  is retained under `conformance/interactive-alpha/fixture`.
- **VS Code extension** (`docs/specs/vscode-extension-v0.md`, intent proposed, delivery deferred) —
  not in this release or its bundle; Electron/remote-host compatibility, MCP runtime conformance
  and performance are unqualified.
- **Local admin console** (optional bundle; `docs/specs/local-admin-console-v0.md`, decision 0081) —
  consult the exact release evidence for installed-path browser qualification (PUB-V0-004/005).
- **Evidence dashboard** (`docs/specs/local-observability-dashboard-v0.md`, intent proposed) —
  needs owner acceptance, a UI shape brief and a frozen browser/platform matrix.
- **Task manager `corvint-tasks` + roadmap** (PUB-V0-010) — ticket and roadmap tools live in the
  separate planned Corvint Tasks source repository. The optional bundle must bind its exact source and retain
  its license/provenance notices. No autonomous agent dispatch is included.
- **Repository guidance** (`docs/specs/repository-guidance-v0.md`, accepted owner intent /
  experimental delivery) — `features` discovers inferred candidates; `overview` adds languages,
  entrypoints, manifests, test conventions and freshness; `review --base FULL_SHA` composes
  affected-test advice and bounded local branch overlap hints. Clean committed inputs are
  required. Source receipts, unknowns and omissions remain visible; candidates are not accepted
  intent, and overlap hints do not prove merge safety.
- **Optional workflow bundle candidate** (`corvint-companion-bundle/2`) — nine Go binaries and
  five raw host integration trees (no VSIX) are the packaging scope. Project test toolchains and host dependencies remain
  prerequisites. The bundle is macOS arm64 only; its exact artifact and installed qualification
  are release-evidence-specific. It does not broaden the default one-binary CLI or promote host
  support beyond `FALLBACK`.
- **Decision 0089's eight proposed verbs** — `depsource`, `lease`, `calibrate`, `necessity`,
  `answerability`, `surprise`, `reads`, `kernel` (`docs/decisions/0089-*.md`) — every slice stays
  `proposed` intent and `experimental` delivery until its own gate runs; two harness call sites
  (`unplannedread.HookPostTool`, `compactionkernel.InjectionBlock`) are deliberately left unwired.

## Qualified platforms

- **Native `corvint` smoke qualification runs only on the build host's own `GOOS`/`GOARCH`**
  (`conformance/release-artifact-v0/smoke.go`: a target is smoke-tested only when it matches
  `runtime.GOOS`/`runtime.GOARCH`). The archives for `darwin_amd64`, `darwin_arm64`, `linux_amd64`,
  `linux_arm64` and `windows_amd64` are all built and checksum-verified by `script/go-archive-gate`,
  but only the host platform that actually ran the gate has been native-smoke-tested; the others
  keep their prior unqualified status per `GOC-V0-006`.
- **The optional workflow bundle** (`corvint-companion-bundle/2`) contains nine Go tools (`corvint`, `corvint-console`,
  `corvint-dashboard-snapshot`, `corvint-tasks`, three MCP servers and two test providers) and five
  raw host-package trees (Codex, Claude Code, Gemini CLI, OpenCode, Pi), no VSIX. It targets **macOS arm64 only** (PUB-V0-004;
  `internal/companionrelease`/`cmd/corvint-companion-release`
  refuses any other target and reports `darwin/amd64`, `linux/amd64`, `linux/arm64` and
  `windows/amd64` as `NOT_RUN` in every report). The exact release evidence records installed-path
  browser/UI qualification for the console (PUB-V0-004/005).
- **Windows artifacts are not qualified and do not enter this release set.** Per PUB-V0-007,
  unqualified Windows artifacts must not ship; the `windows_amd64` archive that `go-archive-gate`
  produces is a reproducibility check only, not a release deliverable.
- **No Rust artifact exists or ships.** Decision 0088 confirms the product stays Go-only; a Rust
  rewrite was considered and not authorized.

## Signing and publisher identity

The `0.5.0a3` archives are **not signed** (decision 0329, retaining decision 0108's policy). Each archive's `SHA256SUMS` and the
byte-identical double build prove integrity and reproducibility only (`ARTIFACT-GO-V0-008`).
Publisher identity is `NOT_VERIFIED`: nothing in this release authenticates who built or published
an archive, and a `PASS` from `archive-verifier` asserts bytes only.

## Default trust boundary (CEM)

The default CEM profile trusts the local Git executable, the local Git object database,
repository metadata, and same-user OS integrity as roots of trust that must remain stable during
one verifier call (`docs/CHANGE-EVIDENCE-MAP.md:241-242@012d2dcc`). CEM 0.1 is not a same-UID
sandbox and is not a signature format; authenticity of a change still requires an external signed
commit, CI artifact, or signature envelope. A separately specified protected profile exists for hostile
object-store mutation but is not the default and must not be presented as one.

## Performance

Current native (Go-only) performance is **unmeasured**. The previously published Python-comparison
numbers predate decision 0088's Go-only cutover; the owner cancelled the proposed paired remeasurement
(`GOC-V0-005`), so no relative or absolute native performance claim is qualified for this release.

## PR test selection

`affected` and `review` provide test advice. The experimental Go package executor has not passed
its full/selected shadow promotion gate; this alpha does not qualify selected-only PR CI. Keep
full-suite fallback for missing, unsupported or uncertain evidence, plus unconditional static,
security, build, interop and full main/release checks. Executable JS/E2E selection is unqualified.
Local checks do not establish hosted-CI qualification.

## Hosted CI

Local checks do not establish hosted-CI qualification. Consult the tagged revision's hosted
workflow record for hosted status; an absent or unsuccessful run provides no hosted evidence.

## Release readiness and promotion

No command is promoted: every command keeps its current label (decision 0109). The release set is
the `darwin_amd64`, `darwin_arm64`, `linux_amd64` and `linux_arm64` archives with the archive gate's
`SHA256SUMS`; that file also names `corvint_windows_amd64.zip`, which is deliberately not attached.
Native performance remains unmeasured and no command is promoted. Publication status is established
only by the versioned release and its owner publication receipt, not by this tracked document.

## Rollback

See [upgrade, retry and removal](INSTALL.md#upgrade-retry-and-remove) for operational steps.

Roll back an installed optional workflow bundle by restoring the preceding verified artifact and
configuration. No ticket-store format or persisted-state migration is introduced by this release
(`docs/specs/public-release-v0.md`, Acceptance and rollback).
