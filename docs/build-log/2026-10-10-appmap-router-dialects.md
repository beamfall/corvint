# V1-1131 — experimental literal router dialects and declared-scope coverage

Intent: owner issue 773; proposed AMAP-V0-040..048 in
`docs/specs/application-map-v0.md`, pending owner acceptance. Base:
`55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`; branch `codex/w5-appmap-a`.

The builder dispatches Rails and React declarations into the existing screen resolver, test join
and collision index. New dialect files keep extraction separate from the other lane's edge
coverage, flow/Playwright, grouping and impact work. Optional `router_globs` enumerates regular
tracked files at the map revision and reports undeclared matches with `router-not-declared` and
`app_coverage`. This is only the declared glob scope; 65 explicit routers already refused before
this change. Discovery never admits a router automatically.

Rails covers literal named routes, nested/scoped/singleton resources, shallow nesting and one-level
revision-pinned draw files. React covers literal createBrowserRouter arrays and Route trees under
createRoutesFromElements or Routes, index routes, pathless containers and terminal splats.
Whole-file source anchors conservatively preserve lexical scope freshness; draw context anchors
join lineage. Dynamic constructs remain UNKNOWN. Existing byte/state limits apply, and attempted
draws share the 64 router-file budget. Shared parameter/splat keys preserve mixed-dialect ambiguity. A final source check of
[the Rails Journey pattern compiler](https://github.com/rails/rails/blob/main/actionpack/lib/action_dispatch/journey/path/pattern.rb)
identified required nonempty Rails globs; `splat_nonempty` preserves that difference from React
while retaining shared collision keys. An empty/nonempty mixed-dialect regression pins it.

Independent read-only native review found and drove repairs for Ruby backticks, conditional React
root extraction, early-return bypass of that uncertainty, JSX quote entity/escape semantics,
React patterns outside the exact template grammar, and deep shallow-resource ancestry. Regression
witnesses now cover each. Reviewer approved the repaired scope after independent overlay probes
and the deep-shallow test; the draw byte/file bounds were reviewed by inspection and have builder
regression tests. No adopter-scale or runtime Rails/React qualification is claimed.

Verification evidence is retained under `/private/tmp/w5-appmap-a-*.log`: baseline, focused parser
and public CLI witnesses passed; final app-map tests and requested documentation gates are recorded
below after execution. `cmd/corvint/main.go` is unchanged, so receipt repinning is not applicable.
The requirements inventory is generated from the staged spec using `make -s spec-requirements`.
On this host bare BSD mktemp ignored TMPDIR and attempted an unwritable platform temp directory;
an external scratch PATH wrapper supplies `mktemp -p /private/tmp/claude-501/gotmp`, without changing
repository gate scripts. Go uses GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp and
GOMAXPROCS=3. No exhaustive `make gate` or full cmd/corvint suite is claimed.

Corvint routes: **used** query and affected (original packets retained in
`/private/tmp/w5-appmap-a-query.json`, `w5-appmap-a-affected.json` and
`w5-appmap-a-affected-diff.json`); static language-frontier unknowns remain open and owner-scoped
checks govern this slice. **Unavailable** index refresh: shared `.git/corvint/index` write was
refused. Initial `make dogfood-change` also refused its shared worktree Git evidence directory.
Post-commit CEM/OCM binding/seal is **deferred** by the explicit no-commit instruction. Ranking,
learning, external providers and release qualification are **not applicable**.

Installed Core is `1.0.0-rc.3 (build 407)`; Tasks is
`0.0.0-tcp01-unverified+build.407`. Official update checks for both failed their process smoke
(`descendant snapshot unavailable`), so available release versions/freshness remain UNKNOWN; no
update or verification bypass occurred. Account spend telemetry is NOT_OBSERVED. The repository
Tasks store was not written, as instructed; tooling friction filing and ticket completion remain
with the coordinating owner. Official Rails and React documentation was read through Context7;
initial local npm cache access failed, then scratch-cache reads succeeded.

Deferred: imported/computed route arrays, JavaScript/Ruby execution, Rails custom inflections,
engine internals and unsupported routing options/patterns, standalone JSX declarations outside the
admitted roots, and adopter runtime qualification. Unknown evidence is retained for unsupported
syntax. Rollback removes new dialect/glob declarations and rebuilds maps; no storage migration.
No commits, pushes, decision files, or native ticket-store writes belong to this builder slice.

A documentation-gate attempt rejected an extra Agent digest field; the V1-1131 status was moved
into the existing Status field. Another attempt exposed the host mktemp handling of explicit
absolute templates; the scratch wrapper now passes those through and supplies -p only for the
default-template case. These failed attempts are retained; no gate was weakened.

Final focused command: `go test ./internal/appmap/ ./cmd/corvint -run
'TestAMAPV004|TestAMAPV000[123]|TestAMAPV0FlowsAppmapCLI' -count=1 -timeout 30m`:
PASS, appmap 17.925s and CLI 2.433s. This includes each new requirement witness, hostile
literal/dynamic cases, draw file/byte bounds, freshness and mixed-dialect CLI lookup.
Independent review also approved the final nonempty-splat delta and ran its regression.

Requested doc gates: spec-requirements-check, requirement-definitions-check,
traceability-tests-check, decision-numbers-check, line-citations-check,
error-code-ownership-check, unbounded-readers-check, use-case-receipts-check and
use-case-receipts-test passed. The final rerun after staging all task paths passed and removes the prior
line-citation gate's unstaged REQUIREMENTS.tsv limitation; exact final output is
`/private/tmp/w5-appmap-a-doc-gates-final.log`. All 18 use-case receipts were current and all
five receipt tests passed. `git diff --check` and `git diff --cached --check` passed.

The broader `go test ./internal/appmap/ ./cmd/corvint -run 'TestAMAPV0' -count=1
-timeout 30m` invocation also passed: appmap 526.299s, selected CLI tests 2.039s
(`/private/tmp/w5-appmap-a-final-tests.log`). It started before the final splat repair; the final
focused run above rechecked the affected parser, lookup and CLI paths after that repair. The
broader run includes the existing exhaustive projection-budget loop; no full cmd/corvint package
run occurred. All owned test/gate processes completed. HEAD remains the recorded base; all task
files are staged and no commit, push or ticket-store write occurred.
