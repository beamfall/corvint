# Root README accuracy against public source and release evidence

Date: 2026-10-02

The README distinguishes the published Core release candidate from newer source capabilities,
and distinguishes Core compatibility from the separately qualified outcomes required for stable
1.0. This is a documentation correction; it adds no product behavior or qualification.

## Original evidence and corrections

The audit reads the complete root README at public commit
`f51f3c9e6fbfc5a6b219692e8bf46e4e43297a36` and follows its links to the original contracts.
The [expanded 1.0 scope](../specs/corvint-1.0-product-and-release-v1.md#expanded-10-product-scope)
requires Flows, safe navigation, documentation/MCP, full Tasks takeover of Beamfall's roadmap and
automatic documentation under decision 0426. The README's promotion summary previously omitted
these outcomes. Core's single-binary boundary and stability promise remain separate from them.

Official release metadata observed on 2026-10-02 lists
[Core v1.0.0-rc.1](https://github.com/beamfall/corvint/releases/tag/v1.0.0-rc.1) with four
macOS/Linux archives, `SHA256SUMS` and `verification-report.json`. The independent
[Tasks developer release](https://github.com/beamfall/corvint/releases/tag/tasks-dev-20260929.2)
contains a darwin/arm64 archive, checksums and build-verification evidence. A companion bundle
builder does not establish that a published Core archive includes companion executables. Release
observation and checksum agreement do not establish publisher identity or runtime qualification.

The quickstart sample is an abridged, value-preserving projection of actual `impact
cmd/corvint/main.go --limit 5` output at the public commit above, replacing the historical
`0.4.0a4` example. It preserves the exact requested-path/test blob identities, fresh tree identity,
five included results and 267 omitted results. Those counts and hashes describe that source
snapshot; they are not a fixed result for every checkout or complete impact coverage.

[Tasks supervision](../TASKS-SUPERVISION.md) documents explicitly configured foreground Codex
programs and continuous dispatch. The prior categorical no-dispatch claim conflicted with these
source capabilities. The standalone developer binary's help and current source differ; neither
source presence nor an unverified build label transfers Core qualification.

The [Go LSP guide](../LSP.md) and current source help admit `--lsp gopls|off` and the explicit
MCP `task-review-lsp` profile; published rc.1 help lacks those newer selectors. The README labels
that source dependency. The separate editor definition/context prototypes remain experimental,
with no automatic editor navigation or exact-client promotion claim.

The [OpenCode adapter](../../integrations/opencode/README.md) separates exact native
`integrationSupport: FULL` tuples from Core lifecycle `FALLBACK` and execution authority `NONE`.
The [CEM contract](../CHANGE-EVIDENCE-MAP.md) separates the frozen `cem/0.2` minimum/N-1 `cem/0.1`
reader from experimental `cem/0.3`. Both distinctions are reflected without promoting wider
support. Flow coverage and editor experiments are grouped with their companion documentation,
while the license and provenance links remain intact.

The development instructions avoid repeating root tests/vet before `make gate`, which already
includes them and the interoperability/archive/documentation checks. Scoped work retains the
owning contract's focused checks, affected-plan uncertainty, dogfood and independent review.
Optional live qualification campaigns retain their declared Python/external-tool prerequisites.

The frontier decision brief's README citation is relocated to the same byte-identical workflow
excerpt, preserving its `3297e31e` content hash. No decision or requirement meaning changes.

The framework-aware test-selection inventory is also made visible: the public
[language registry](../../internal/liveverify/affected/languages/languages.go) and its Go,
JavaScript/TypeScript, Python, Ruby, Rust, Swift, Kotlin and .NET implementations recognize test
units without launching their runners. Installed `affected` help confirms this planning boundary.
The optional Go/pytest mutation path in `prove` has separate sandbox and offline prerequisites.
Unmerged runner companion profiles are not presented as delivered source or release support.

## Verification and limits

Before editing, actual Core/source CLI outputs and official release metadata were inspected.
All local README links and Markdown anchors resolved at the audit base. The installed read-only
query and impact routes were used; ranked context was a discovery lead, not governing authority.
Impact retained 267 omissions. Release checks initially refused under restricted process/network
access and succeeded with authorized access; no release was installed or published.

Focused documentation and spec-index checks passed for the source correction. Direct application
readback verifies the reviewed README bytes, the unchanged citation excerpt and local links.
Independent proposal review retains the release/source, runner and qualification boundaries;
post-commit CEM/dogfood evidence is produced through the existing contract. Repository-wide `make gate`, broad live LSP/editor
qualification, external interoperability and whole-product promotion are not produced by this
prose correction. The README retains rc.1's failed Core-job evaluation, unqualified retrieval,
unproven reviewer benefit, unsigned publisher identity and platform qualification limits.
