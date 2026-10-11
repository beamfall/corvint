# V1-1140: discovery producer guidance

Intent: proposed, pending owner acceptance (V1-1140; GitHub #782).
Base: `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`, branch `codex/w5-docs`.
Delivery: experimental bounded slice; ticket acceptance remains partial for
revision-only staleness. No decision file, commit, publication or ticket-store write.

## Contract and implementation

The adopter now receives inert provider command templates and the expected
`.corvint/test-evidence` path for discovery with no usable evidence. Templates
name opt-in JavaScript unit/external E2E retention and Go foreground preview
session retention, including required external application/readiness inputs and
Go authority flags. Qualification is explicitly `NOT_OBSERVED`; commands are
neither probed nor executed (`internal/testvaliditydoc/discover.go:74@697f6441`).

A selected stale projection carries the closed `stale-evidence` abstention and
existing `retained-producer-run` requires value. Bounded observed runtime versions
survive precedence over runtime-tuple abstention. A retained test-repository
revision at run start is echoed only as a 40/64-character lowercase hex object ID;
missing or malformed identity stays `UNKNOWN`. Application-repository revisions
are never substituted (`internal/testvaliditydoc/discover.go:182@ae3d7be7`).
Discovery also exposes a stable receipt-age origin, its selected decodable file's
UTC modification timestamp, rather than a ticking duration. Missing age is
`NOT_OBSERVED`. This is retention age metadata, not authenticated execution time
(`internal/testvaliditydoc/discover.go:121@485e4c00`).

`LPCVD-V0-001..003` are proposed requirements in the owning spec with one Delivery
row per requirement. The distinct prefix was absent from `origin/main`; its
highest existing LPCV ID was 070, and `git ls-remote origin
'refs/heads/codex/w5-*'` returned no matching published branches. REQUIREMENTS.tsv
was regenerated from the staged specs, then staged. Three existing citation
locations were moved to the identical passages; their content hashes stayed fixed.
Rollback removes the additive discovery guidance; producer bytes need no migration.

## Verification and review

Baseline `go test -count=1 -timeout 30m ./internal/testvaliditydoc` passed before
implementation. Final focused checks used `GOTOOLCHAIN=local`, Go 1.27.1,
`TMPDIR=/private/tmp/claude-501/gotmp`, and `GOMAXPROCS=3`:

- `go test -count=1 -timeout 30m ./internal/testvaliditydoc ./internal/mcp/testvaliditybridge ./conformance/mcp-test-validity-v0`: passed.
- `go test -count=1 -timeout 30m ./cmd/corvint -run '^TestTestValidity'`: passed.
- Requested spec-requirements, requirement-definitions, traceability-tests,
  decision-numbers, line-citations, error-code-ownership, unbounded-readers and
  use-case-receipts checks, plus use-case-receipts-test: passed after repairs.
- `sh script/repin-use-case-receipts.sh --check`: all 18 receipts current;
  `cmd/corvint/main.go` was unchanged, so no repin write was needed.
- `git diff --check HEAD`: passed. Full `make gate` and recursive package tests
  were `NOT_RUN` by explicit task scope; focused checks do not replace them.

Focused tests cover missing/unusable evidence and required command inputs
(`internal/testvaliditydoc/discover_test.go:288@dc1a5f96`), stale JS/Go evidence,
age origin selection, and valid/invalid retained revisions combined with stale
and unqualified runtime evidence
(`internal/testvaliditydoc/discover_test.go:368@690b1086`). CLI/MCP canonical byte
parity includes missing, unusable and stale discovery
(`cmd/corvint/test_validity_unix_test.go:152@bc34530f`). New stdio conformance
vectors use isolated repositories and verify the no-write digest on both protocol
versions (`conformance/mcp-test-validity-v0/blackbox_test.go:159@c9455054`).

One independent native reviewer found missing E2E flags, discarded optional
revision evidence, and missing stale/runtime precedence coverage. All three were
repaired; the bounded repair review passed with no remaining implementation
findings. It confirmed the revision-only acceptance gap below. The inherited
builder/reviewer model was retained for this bounded change; live effort and
account/billing telemetry were `NOT_OBSERVED`.

Initial verification exposed a duplicate requirement-definition-shaped reference,
shifted citations, and a new attested fixture that incorrectly expected CURRENT
rather than UNKNOWN external lifecycle freshness. The reference, citation offsets
and fixture baseline were corrected without weakening the original contracts.
macOS `mktemp` ignored the selected TMPDIR for its default template and attempted
a blocked system directory; checks used a scratch PATH wrapper supplying the
approved temporary-directory template explicitly, with no repository script edit.

## Corvint evidence and unresolved boundaries

Corvint query, tracked-path impact and affected routes were used; original outputs
are `/private/tmp/w5-docs-query.json`, `/private/tmp/w5-docs-impact.json` and
`/private/tmp/w5-docs-affected.json`. Query/impact reported revision
`81d2fee439bf27a35a866584c97ab34e5b866f2b`, not this worktree's base, so they do not
establish base-bound context. Scoped original-source reads supplied the fallback.
Impact omitted 33 results and direct consumers; affected retained cross-language
frontiers and unowned documentation paths. Its broad gate advice was overridden
only by the user's explicit focused-test scope, not treated as equivalent proof.

The requested index refresh and initial `make dogfood-change BASE=...` failed
because the shared Git metadata directory is outside writable sandbox roots.
Post-commit CEM/OCM binding and seal are `NOT_PRODUCED`: this task forbids commits.
No provider qualification, live execution, mutation campaign or external adopter
outcome was claimed. Those routes were not applicable to this bounded read change.

Installed Core is 1.0.0-rc.3 build 407; Tasks is
0.0.0-tcp01-unverified+build.407. Both official update checks failed their version
smoke with `descendant snapshot unavailable`; available releases/freshness remain
UNKNOWN, and no installation occurred. Filing tooling failures is held for the
coordinator because this task explicitly prohibits ticket-store writes.

Revision-only current-worktree changes remain undetected by the existing bounded
file-digest freshness check. Admitted basic JS/Go receipts also lack repository
revisions; attested receipts only sometimes retain them. Completing this acceptance
condition needs a separately scoped, read-only current-revision binding; it is not
proved by this patch. Qualification lookup likewise stays NOT_OBSERVED. Receipt
age is exposed as a stable mtime origin for the client to subtract from its clock,
not a numeric elapsed duration. Ticket closure remains with the coordinator.
