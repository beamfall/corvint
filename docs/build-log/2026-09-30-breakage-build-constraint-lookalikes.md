# Breakage map: build-constraint lookalikes

Owner request: find and fix bugs. The owner explicitly authorized continued use of
usage credits after the reported weekly limit. Base is public `origin/main`
`9afd8313edad658d6c40b1abe454fe7417eb6cad`; the isolated branch is
`codex/bug-hunt-0930`. Native ticket:
`ticket:corvint:main:BUG-BREAKAGE-CONSTRAINT-LOOKALIKES-0930`.

## Confirmed defects and repair

Whole-source substring matching treated `//go:build` and `// +build` inside string
literals as constraints. Checking every underscore-separated filename component
also withheld ordinary sources such as `api_linux_helpers.go` and
`caller_amd64_helpers_test.go`. Four committed two-repository Compile regressions
produced no caller edges and an API build-constraints gap before repair.

`BKM-V0-004` now has explicit boundary language and named acceptance witnesses.
The implementation uses parsed comments and physical header offsets, standard Go
directive recognition, and the final filename component before optional `_test`.
It retains Go's historical/reserved filename targets and conservative gaps for
actual constraints. No behavioral, coverage, complete-inventory or promotion
qualification is inferred; this capability remains proposed and experimental.

One Sol/low native reviewer was selected for routine AST and filename logic; the
builder's inherited live model/effort settings were not changeable through these
tools. The reviewer found that exact directive-shaped comments after source begins
were still withheld. Six additional failing cases reproduced header, blank-line
and block-comment boundaries; the repair now respects those boundaries. Independent
re-review reported no findings and its focused regressions passed (3.921 seconds).
The full package passed after repair (33.715 seconds); the public CLI checks and
spec index checks also passed. Final selected checks and CEM/OCM observations are
bound separately by the enrolled workflow, rather than inferred from this prose.

## Self-use, verification limits and native completion

Used: unchanged original `query --task 'find and fix bugs'`, tracked-path impact,
`affected`, pre-change `make dogfood-change`, explicit intent/check enrollment and
the post-commit CEM/OCM workflow. The primary-worktree query abstained for unindexed
changes; the clean-checkout query returned OUT_OF_SCOPE. Original source/spec reads
provided the needed evidence. Tracked-path impact was READY with no critical missing
paths. `affected` selected 136 units and retained UNKNOWN scope plus conservative
unbounded/path-literal readers. Repository-wide and release validation are NOT_RUN
under the owner's standing scoped-work preference; package tests, CLI checks, vet
and documentation checks are the selected verification, not equivalent coverage.
No retrieval, ranking, learning, wire profile or release promotion was changed, so
their frozen evaluation campaigns were not applicable to this repair.

Initial dogfood retained NOT_PRODUCED `git-diff-failed`, `cem-map-not-produced`,
`exit-2`, `intent-scope-drift` and `map-unavailable` for the empty base/target range.
Original receipts, red/green logs, the enrollment plan and review checkpoint are
retained under `/private/tmp/corvint-bug-hunt-evidence/`, with original pre-change
receipts also in this worktree's private Git Corvint directory. Billed task tokens,
cache use and complete task cost remain NOT_OBSERVED; shared account balance is not
task attribution. Core build 163 and Tasks build 202 were retained. Official updater
checks observed `v1.0.0-rc.1` and `tasks-dev-20260929.2`; release evidence is not broad
qualification and publisher identity remains NOT_VERIFIED. No downgrade occurred.

Native filing/readback and receipt audit succeeded, with journal structural
consistency and projection agreement; actor authentication, historical acceptance,
runtime qualification and liveness remain NOT_OBSERVED. A scoped claim was refused
RESOURCE_COLLISION because another live attempt reserves the shared CEM paths.
No reservation or queue policy was bypassed. The ticket remains OPEN: publication
of a reviewed patch does not provide integration/landing or a successful native
submit/gate/complete write. No merge, release, deletion or force-push was authorized.

Rollback restores the prior build-constraint classifier and these scoped tests/spec
clarifications. This repair adds no persistent state or migration.
