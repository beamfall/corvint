# Tasks source archive dependency closure — V1-0456

Base: `e7842a777133c0ef603e7e1567d1deb77aa168f3`, the coordinator's prepared
integration of existing PR315 with main. That dependency work is not this change.
Owning requirements: PUB-V0-012 and PUB-V0-013 in `public-release-v0.md`;
decision 0397's CAL-V0-022 addendum retains the import-direction boundary.

## Observed blocker and correction

The retained baseline export failed to build because the Tasks pack adapter imports
contextindex and runtimeenv, while the source subset contained only cmd/corvint-tasks
and internal/tasks. The coordinator's original packaging-baseline.log is retained;
that failed build was not repeated.

The existing export allowlist now includes those packages and their in-module
transitive dependencies: diagnostic, gitstatus, projectprofile, pythongrammar,
pythonsyntax, secretscreen and untrackedallowance. No exporter or dependency framework
is added. Selection retains the original source commit, tree, blobs and file bytes,
including package notices and root LICENSE, LICENSE-APACHE-2.0, LICENSING.md and
PROVENANCE.md. Packaging does not authorize additional import edges or let Core link
Tasks mutation code.

The new npm-independent regression assembles the actual source tarball using the
release path, decodes/extracts that member, verifies the materialized Git blobs,
and runs the advertised default `go build ./cmd/corvint-tasks` with GOWORK off,
GOPROXY/GOSUMDB off, and isolated cold Go caches. A second archive omits diagnostic,
a transitive contextindex dependency, and must fail to build naming that package.
This is scoped archive proof; the nine-component double-build/VSIX release gate
remains separate and NOT_RUN.

## Verification and handoff

The subset unit and existing Tasks import-direction/negative-control tests pass;
companionrelease vet and requirement locators pass. The actual offline archive
regression will run once after this source commit, preserving its exact tested
commit/tree/archive identity in private evidence. Final independent review,
CEM/OCM/dogfood closure and dependency integration remain pending.

Private enrollment, measurement, pre-change query/impact, affected plan and logs:
`/private/tmp/corvint-bugfix-20260928/v1-0456/`. The initial no-diff dogfood attempt
retains its NOT_PRODUCED reasons. The existing tested owned-process wrapper is reused.
No account, network dependency, release publication, policy edit or cobra access is
introduced. Rollback reverts the additional export prefixes and this packaging
amendment; the prior standalone-build blocker must then remain visible.
