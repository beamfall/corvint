# Issue338: bounded source-derived flow documentation

Intent: [issue338](https://github.com/beamfall/corvint/issues/338),
[AFU-V1-043..048](../specs/application-flow-understanding-v1.md#source-derived-non-go-documentation-issue-338).
Baseline: `b580cd7d1eda6a75dfe72f7c51a86b4419612eaa`, including the locally integrated gitlink/root
boundary dependency and issue339 adoption corpus. Source generation is a sibling of accepted-outcome
AFU documentation; it does not change `appflows/docs.go` or grant runtime/intent authority.

The native `docs flows generate/check/finalize` path materializes eight Markdown pages per logical
flow, a typed adoption provider and an immutable generation manifest. Source A and provider B remain
separate. The finalization command verifies the later committed provider and successfully builds its
returned corpus manifest. Logical IDs survive line insertion; changed spans stale only their dependent
items. Imported context is revalidated by corpus Open, receives stable paragraph identities and retains
all original anchors, test role/project/confidence, explicit coverage membership, tickets and intent
comparisons. Only admitted security totals/severity summaries render.

The reviewed closed construct matrix covers Ruby/Rails methods, jobs, literal routes, class hooks and
recognized calls; JS/TS named functions/arrows and literal route/Angular registrations; React JSX
conditionals; and literal Angular HTML attributes. HTML is loaded through immutable Git authority
without changing native index suffix admission. Lexical/dispatch/parser gaps remain explicit.
Diagrams show the particular flow and call inventory with unresolved order. No arbitrary source body
or secret literal is copied into generated prose. Repeated claim identities remain unresolved.

Output uses new directories, retained parent/component handles with identity checks, exclusive files
and bounded non-follow reads. Bounds preserve the existing128MiB corpus profile:64MiB pages/provider,
256MiB generation envelope,320MiB aggregate; at most256 flows /2,048 Markdown pages plus two metadata
files. Check and finalize perform no output writes.

Independent review identified added-ID drift omission, declaration tokens mistaken for calls,
unbound imported prose, incomplete multi-anchor freshness, multiline HTML misbinding, symlink-swap
windows and placeholder diagrams. Targeted regressions now cover each. Final repair also preserves
exact line bindings when an immutable blob is unchanged despite repeated equal spans, validates the
fresh destination/subdirectories through retained parent handles, and admits the93,134,239-byte
issue339-sized corpus input under the128MiB consumer ceiling.

Builder verification (local fixtures, not external migration or runtime parity):

- `GOTOOLCHAIN=local GOCACHE=/tmp/corvint-go-build-cache go test -count=1 -timeout 30m ./internal/flowdocs ./cmd/corvint -run 'TestFlowDocs|TestDocsRootHelpDisclosesLedgerException|TestDocsHelpPrerequisitesAndWorkingExample'` passed after the main review repairs. Actual CLI proof includes Ruby API, React/TS, AngularJS and multiline HTML, eight real files, provider commit, corpus Build/Open and per-item drift.
- Final targeted repair run: `go test -count=1 -timeout 30m ./internal/flowdocs -run 'TestFlowDocs(UnchangedRepeatedImportedSpan|DirectorySwapCannotEscape|ScaleDeterminismSafetyAndGitlink|CorpusEnvelopeBounds)$'` passed; the same local toolchain/cache settings apply.
- `go vet ./internal/flowdocs` and `git diff --check` passed. The scale regression writes150 flows /1,200 pages and checks immutable regeneration, opaque gitlink behavior, no-clobber, symlink refusal and forged predecessor rejection.
- `TestFlowDocsRevalidatedCorpusContext` retains declared fixture journeys/test joins and proves no restricted canary escapes. This fixture never claims an actual browser observation.

Actual dogfood routes used: prechange query and `affected --base` receipts retained in
`/private/tmp/corvint-adoption-336-340/builder-query-338.json` and `builder-affected-338.json`.
The query retained its omissions; it was orientation, not proof. The previous Go-only docs draft route
is inapplicable to the non-Go producer. Core ranking/learning evaluations are not applicable because
this sibling producer does not alter ranking or learning. Root owns the frozen selected checks,
independent repair re-review, final CEM/OCM/seal and actual Playwright qualification. Those remaining
receipts must be retained before reporting issue completion; this builder entry does not claim them.

Reusable150-flow+gitlink source fixture creation is retained at
`/private/tmp/corvint-adoption-336-340/create-flowdocs-fixture.py`. No server was spawned by the builder;
all test commands completed. No commit, push, PR publication or ticket mutation was performed.
Rollback removes the new dispatch/producer; existing corpus and accepted AFU contracts remain intact.
