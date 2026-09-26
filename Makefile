BASE ?= main
CORVINT_GOCACHE ?= /tmp/corvint-go-build-cache
CORVINT_BIN ?= corvint
.DEFAULT_GOAL := gate

# GO_TEST_TIMEOUT is the per-package hang detector, not a performance budget (decision 0082).
# cmd/corvint measured 596.0s against Go's former implicit 10m package deadline. Frozen replay
# has a separate 9m context covering cold candidate build and replay; the 30m package deadline
# does not extend it. Root -p 1 serializes top-level Go build/test programs as a bounded scheduling
# experiment; nested build flags and intra-package concurrency remain unchanged. Cost is tracked
# in docs/agent-memory/optimizations.md; neither deadline is changed by this experiment.
GO_TEST_TIMEOUT ?= 30m

# GO_TEST_COMMAND is the one go test invocation both tiers run; go-test appends `./...` and
# gate-affected appends the selected packages, so the flags cannot drift apart. GO_TEST_FLAGS is
# the same invocation without -count=1: the gate's ledger tier (GL-V0-004) adds -count=1 itself
# for the packages whose reads no literal bounds and lets Go's test cache answer the rest.
GO_TEST_FLAGS = -p 1 -timeout $(GO_TEST_TIMEOUT)
GO_TEST_COMMAND = GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) -count=1

.PHONY: build gate gate-receipt-clear gate-receipt-test receipt-bundle-verify-test gate-affected gate-affected-test host-adapter-test go-version go-test go-vet cross-vet go-format-check go-format-test go-archive-gate go-archive-gate-test interop-gate spec-requirements spec-requirements-check spec-requirements-test requirement-definitions-check traceability-tests-check decision-numbers-check decision-numbers-test eol-policy-check eol-policy-test line-citations-check line-citations-test ci-least-privilege-check ci-least-privilege-test release-checklist-test analyzer-python-offline-build-test analyzer-python-ratchets-test release-artifact-reproducibility-test sql-native-ratchets sql-native-ratchets-test companion-release-gate public-release-check core-n1-replay dogfood-change dogfood-check dogfood-seal dogfood-bind-range dogfood-bind-range-test error-code-ownership-check error-code-ownership-test use-case-receipts-check use-case-receipts-test cem-verify-pr-test cem-recipes-test host-package-versions-check host-package-versions-test diagnostic-coverage-check go-archive-gate-injection-test install-lifecycle-test hostile-regressions-check hostile-regressions-test no-python-runtime-dependency-test

build: go-version
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -ldflags "-X main.build=$$(git rev-list --count --first-parent HEAD)" -o $(CORVINT_BIN) ./cmd/corvint

# Optional protected Pi distribution; dependency installation/download is explicit.
.PHONY: pi-protected-build pi-protected-test
pi-protected-build:
	bun integrations/pi-protected/build.mjs

pi-protected-test: pi-protected-build
	node --test integrations/pi-protected/*.test.mjs
	cd tools/local-authority && GOTOOLCHAIN=local go test -count=1 -timeout 30m ./... && GOTOOLCHAIN=local go vet ./...

# gate is the reproducible verification gate: it must pass from a fresh clone with no private
# .corvint/ state. dogfood-check is a separate authoring-time discipline step (AGENTS.md), run
# explicitly with BASE=<sha> after committing the change and its CEM (docs/DOGFOOD.md); it is
# not a gate prerequisite because it depends on
# untracked, gitignored .corvint/ artifacts that a clean checkout never has.
# GATE_STEPS are the gate prerequisites after gate-receipt-clear.
GATE_STEPS = host-adapter-test go-version go-test go-vet cross-vet go-format-check go-format-test go-archive-gate go-archive-gate-test interop-gate spec-requirements-check spec-requirements-test requirement-definitions-check traceability-tests-check decision-numbers-check decision-numbers-test eol-policy-check eol-policy-test line-citations-check line-citations-test ci-least-privilege-check ci-least-privilege-test release-checklist-test gate-receipt-test error-code-ownership-check error-code-ownership-test cem-verify-pr-test host-package-versions-check host-package-versions-test diagnostic-coverage-check no-python-runtime-dependency-test
gate: gate-receipt-clear $(addprefix ledger/,$(GATE_STEPS))
	@script/gate-receipt record

# GOC-V0-010: when gate is a goal, every step waits for the clear to finish, so under `make -j`
# the receipt is removed and the start stamped before any step starts. A step run on its own
# does not clear the receipt.
ifneq ($(filter gate,$(or $(MAKECMDGOALS),gate)),)
$(GATE_STEPS) $(addprefix ledger/,$(GATE_STEPS)): | gate-receipt-clear
endif

# ledger/STEP runs STEP through tools/gate-ledger (docs/specs/gate-ledger-v0.md): the step is
# skipped only when a pass is recorded for byte-identical inputs, from any worktree of this
# user, and it records only after STEP exits zero. ledger/go-test splits `./...` into the
# resolved packages, each keyed on its proven bound (the go list test closure plus every path
# gate-affected-select -bounds attributes to it) so a pass hits from any worktree, and the
# unresolved packages, which run with -count=1 under a whole-tree key. Every step still runs
# unchanged on its own target, and CORVINT_GATE_LEDGER=off makes ledger/STEP exactly `make STEP`.
# The sub-make receives the same makefiles as this one so an overriding makefile
# (script/gate-receipt_test.sh) reaches it.
GATE_LEDGER = GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/gate-ledger
SUB_MAKE = $(MAKE) $(addprefix -f ,$(MAKEFILE_LIST))
ifeq ($(CORVINT_GATE_LEDGER),off)
ledger/%:
	@$(SUB_MAKE) $*
else
ledger/go-test:
	@$(GATE_LEDGER) go-test -- go test $(GO_TEST_FLAGS)
ledger/%:
	@$(GATE_LEDGER) run $* -- $(SUB_MAKE) $*
endif

# Native adapter and opt-in handoff regressions are owned by the reproducible source gate.
host-adapter-test:
	node --test integrations/pi/runtime.test.mjs integrations/pi/extension.test.mjs integrations/pi/tools.test.mjs
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test -count=1 ./cmd/corvint -run '^(TestDogfoodEvent|TestCodexHostAdapter|TestClaudeNativeDogfoodLifecycle|TestClaudeSourceHandoffCLI|TestHostAdapter|TestPi)'

go-version:
	test "$$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.1"

go-test:
	$(GO_TEST_COMMAND) ./...

# gate-affected is the fast tier: Corvint's own affected-test selection (`corvint affected`)
# narrows go-test to the Go packages the change reaches, then the cheap non-test checks run.
# script/gate-affected.sh prints the selection and falls back to the full `./...` run, saying
# why on stderr, whenever the plan cannot justify a narrower set. BASE (default main) joins
# the committed range BASE..HEAD to the dirty worktree; `make gate-affected BASE=` tests the
# dirty worktree alone. It is not the push gate: `make gate` stays that, unchanged, until the
# 200-commit shadow evidence exists (docs/specs/affected-plan-v0.md).
gate-affected: go-version
	@GO_TEST_COMMAND='$(GO_TEST_COMMAND)' script/gate-affected.sh "$(BASE)"
	@$(MAKE) go-vet go-format-check spec-requirements-check decision-numbers-check line-citations-check

gate-affected-test:
	@script/gate-affected_test.sh

go-vet:
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go vet ./...

# cross-vet catches cross-OS compile gaps that go-vet's native-host run cannot see
# (docs/agent-memory/tests.md, 2026-09-12): it type-checks the root module and interop/cem01-go
# for GOOS=windows and GOOS=linux. CGO stays disabled so the check never depends on a
# cross-compiling C toolchain being installed.
cross-vet:
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go vet ./...
	cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
	cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go vet ./...

go-archive-gate:
	@script/go-archive-gate

# go-archive-gate-test covers GAG-V0-007 on a fixture repository: `archive` records its witness
# best-effort, so the gate removes any pre-existing witness before building and a run whose witness
# write failed can no longer be satisfied by one an earlier run left behind.
go-archive-gate-test:
	@script/go-archive-gate_test.sh

# go-archive-gate-injection-test is GAG-V0-006 and GAG-V0-004 evidence against the real producer: one
# real five-target build in a clone of HEAD, then extra, missing and symlinked outputs injected before
# the unchanged gate's checks and a SIGINT during the real build. It needs the manifest's exact Go
# toolchain first on PATH and is not a `gate` prerequisite, because `go-archive-gate` already pays for
# one real build per `make gate` run. The interrupt test first proves an interrupted harness exits 130.
go-archive-gate-injection-test:
	@script/go-archive-gate-injection-interrupt_test.sh
	@script/go-archive-gate-injection_test.sh

interop-gate:
	cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test -count=1 -timeout $(GO_TEST_TIMEOUT) ./... && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go vet ./...

# spec-requirements regenerates docs/specs/REQUIREMENTS.tsv. The generator reads the Git index,
# so stage the changed spec first.
spec-requirements:
	@tmp=$$(mktemp); trap 'rm -f "$$tmp"' EXIT INT TERM; script/gen-spec-requirements.sh >"$$tmp" && cp "$$tmp" docs/specs/REQUIREMENTS.tsv

spec-requirements-check:
	@tmp=$$(mktemp); trap 'rm -f "$$tmp"' EXIT INT TERM; script/gen-spec-requirements.sh >"$$tmp"; cmp "$$tmp" docs/specs/REQUIREMENTS.tsv || { echo "REQUIREMENTS.tsv is stale: stage the changed spec, then run make spec-requirements" >&2; exit 1; }

# spec-requirements-test pins SRG-V0-001: the generator enumerates the Git index, so no
# untracked, deleted-from-the-worktree, or unstaged spec can change the index a fresh clone
# reproduces. It is the drift detector for a return to a worktree listing.
spec-requirements-test:
	@script/gen-spec-requirements_test.sh

# requirement-definitions-check fails when a requirement id survives only as a table row.
# REQUIREMENTS.tsv is generated by an extractor that falls back to a table row when no clause
# body defines an id, so a deleted clause body otherwise leaves every gate green.
requirement-definitions-check:
	@script/check-requirement-definitions.sh

traceability-tests-check:
	@script/check-traceability-tests.sh

# decision-numbers-check grandfathers exactly the cross-lineage 0016 pair (decision 0048) and
# fails on any other collision, and fails when docs/decisions/README.md's index is missing a
# tracked decision file's row or cites the same file's row twice (V1-0265).
decision-numbers-check:
	@script/check-decision-numbers.sh

decision-numbers-test:
	@script/check-decision-numbers_test.sh

# eol-policy-check fails when a tracked path stops reporting `attr/-text` or commits CRLF or
# mixed endings, exempting only the two intentional interop fixtures. Decision 0061 made
# .gitattributes repository-wide `* -text`, so Git normalises nothing and a CRLF-committing
# contributor is not corrected automatically; this is where that drift is caught.
eol-policy-check:
	@script/check-eol-policy.sh

eol-policy-test:
	@script/check-eol-policy_test.sh

# go-format-check fails when a tracked Go file is not gofmt-clean, using the pinned toolchain's
# gofmt over `git ls-files`, so scratch trees and vendored Go are out of scope. go vet says
# nothing about formatting, which is how an unformatted file sat in the tree unnoticed.
go-format-check:
	@script/check-go-format.sh

go-format-test:
	@script/check-go-format_test.sh

# line-citations-check fails when a `path:line` citation in docs/specs or docs/agent-memory
# names a missing file, a line past its end, or a blank/bracket-only line. Specs cite a working
# tree and land in the same commit as the code, so this is where stale citations are caught.
# A citation pinned as `path:line@<hex>` also fails when the cited content itself changed;
# `script/check-line-citations.sh --hash <path:line>` prints the anchor to write.
line-citations-check:
	@script/check-line-citations.sh

line-citations-test:
	@script/check-line-citations_test.sh

# ci-least-privilege-check fails when .github/workflows/ci.yml's workflow-level permissions
# stop being exactly `contents: read` (or a job-level block grants more), an actions/checkout step stops disabling persisted
# credentials, or any `uses:` reference stops being pinned to a full 40-character commit SHA
# (ARTIFACT-V0-008, docs/specs/release-artifact-integrity-v0.md).
ci-least-privilege-check:
	@script/check-ci-least-privilege.sh

ci-least-privilege-test:
	@script/check-ci-least-privilege_test.sh

# host-package-versions-check fails when a host package manifest's version field (AHI-020,
# docs/specs/agent-harness-integration-v0.md) was last bumped in a commit older than the last
# commit that changed any other shipped file in that package.
host-package-versions-check:
	@script/check-host-package-versions.sh

host-package-versions-test:
	@script/check-host-package-versions_test.sh

release-checklist-test:
	@script/release-checklist_test.sh

# no-python-runtime-dependency-test is the GOC-V0-008 falsifying assertion: no build, test, hook or
# packaging path names a Python interpreter.
no-python-runtime-dependency-test:
	@script/no-python-runtime-dependency_test.sh

# GOC-V0-010: the full-gate receipt is cleared before the first gate step and recorded only after the
# last one passed, bound to HEAD and the archive witness this run recorded.
gate-receipt-clear:
	@script/gate-receipt clear

gate-receipt-test:
	@script/gate-receipt_test.sh

# RCB-V0-006: the offline receipt-bundle verifier reports a match, a tampered, a missing, and an
# extra file.
receipt-bundle-verify-test:
	@script/verify-receipt-bundle_test.sh

# cem-verify-pr-test runs examples/cem/verify-pr.sh against a fixture repository (CEM-PILOT-009).
# It builds corvint from this checkout unless CORVINT_BIN names one.
cem-verify-pr-test:
	@script/cem-verify-pr_test.sh

# cem-recipes-test runs the three examples/cem/recipes against fixture repositories
# (CEM-PILOT-024..027). It builds corvint from this checkout unless CORVINT_BIN names one; it is
# not a gate member.
cem-recipes-test:
	@script/cem-recipes_test.sh

# The three optional artifact wrappers are not gate members; their tests drive each wrapper
# with a stub `go` (OACS-V0).
analyzer-python-offline-build-test:
	@script/check-analyzer-python-offline-build_test.sh

analyzer-python-ratchets-test:
	@script/check-analyzer-python-ratchets_test.sh

release-artifact-reproducibility-test:
	@script/check-release-artifact-reproducibility_test.sh

# error-code-ownership-check fails when an emitted kebab-case code in tracked non-test Go under
# cmd/ and internal/ is named by no docs/specs/*.md and is not in
# script/error-code-ownership.allowlist, or when a listed code has since been owned or retired, so
# the list only shrinks (ECO-V0, docs/specs/error-code-ownership-gate-v0.md).
error-code-ownership-check:
	@script/check-error-code-ownership.sh

error-code-ownership-test:
	@script/check-error-code-ownership_test.sh

# use-case-receipts-check fails, naming each receipt to repin, when a subject pinned by a receipt
# under conformance/use-cases-v0/receipts/ changed (V1-0268); script/repin-use-case-receipts.sh
# without --check does the repin.
use-case-receipts-check:
	@script/repin-use-case-receipts.sh --check

use-case-receipts-test:
	@script/repin-use-case-receipts_test.sh

# diagnostic-coverage-check fails when a diagnostic.Refusal literal lacks a subject, a registered
# fix list or terminal reason, when Go that imports internal/diagnostic parses a refusal message,
# or when the covered-site count differs from script/diagnostic-coverage.count (DRC-V0-011/012).
diagnostic-coverage-check:
	@script/check-diagnostic-coverage.sh

install-lifecycle-test:
	@script/check-install-lifecycle_test.sh

hostile-regressions-check:
	@script/check-hostile-regressions.sh

hostile-regressions-test:
	@script/check-hostile-regressions_test.sh

# sql-native-ratchets is opt-in only and is NOT a `make gate` prerequisite: it builds
# cmd/corvint twice and runs 30 benchmark samples pinned to one local Go toolchain
# install (docs/specs/sql-native-ratchet-gate-v0.md, SNR-V0), far too slow for the gate. Invoke
# it directly when evaluating the experimental sql-native analyzer candidate.
sql-native-ratchets:
	@script/check-sql-native-ratchets.sh

# sql-native-ratchets-test exercises the gate's lock/lock-test-hold, source-byte-ceiling, Core-base
# export, and causal_parent presence-guard control flow with disposable fixtures; it is fast (no cmd/corvint
# build, no benchmarks) and is NOT part of `make gate`, matching its sibling ratchet-script tests.
sql-native-ratchets-test:
	@script/check-sql-native-ratchets_test.sh

# companion-release-gate is opt-in and is not a `make gate` prerequisite. It builds
# all nine bundled binaries twice, including the MCP servers and test providers,
# and independently assembles and verifies the archives (PUB-V0-013/014).
# The native CLI archive gate remains independent (PUB-V0-002). corvint-tasks
# builds from this checkout's cmd/corvint-tasks (decision 0397).
companion-release-gate:
	@script/corvint-companion-release-gate

# public-release-check is the opt-in retained-artifact qualification. Required
# paths and frozen source identities are passed through the named CORVINT_* env.
# CORVINT_PUBLIC_RELEASE_QUALIFICATION selects editor (default) or core (bundle /2,
# PUB-V0-020); core also needs CORVINT_NODE_SHA256, CORVINT_NPM_SHA256,
# CORVINT_GO_AUTHORITY_BUNDLE and CORVINT_GO_AUTHORITY_SHA256, which editor refuses.
public-release-check:
	@script/public-release-check

# core-n1-replay is the opt-in CCF-V1-007 N-1 replay and is not a `make gate` prerequisite. It
# builds CORE_N1_TAG, by default the newest release tag before HEAD, from `git archive` and runs
# every frozen Core mode under it beside this build (TestCoreVerbsEmitTheFrozenProfiles).
CORE_N1_TAG ?= $(shell git describe --tags --abbrev=0 --match 'v[0-9]*' --exclude '*-*' HEAD^ 2>/dev/null)

core-n1-replay:
	@test -n "$(CORE_N1_TAG)" || { echo "CORE_N1_TAG is required" >&2; exit 2; }
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && mkdir "$$tmp/src" && \
	git archive -o "$$tmp/src.tar" "$(CORE_N1_TAG)" && tar -x -C "$$tmp/src" -f "$$tmp/src.tar" && \
	(cd "$$tmp/src" && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -o "$$tmp/corvint" ./cmd/corvint) && \
	CORVINT_CORE_N1_BINARY="$$tmp/corvint" $(GO_TEST_COMMAND) -run '^TestCoreVerbsEmitTheFrozenProfiles$$' ./cmd/corvint

dogfood-change:
	@test -n "$(BASE)" || { echo "BASE is required" >&2; exit 2; }
	@script/dogfood-change.sh "$(BASE)"

dogfood-check:
	@test -n "$(BASE)" || { echo "BASE is required" >&2; exit 2; }
	@script/dogfood-check.sh "$(BASE)"

dogfood-seal:
	@test -n "$(BASE)" || { echo "BASE is required" >&2; exit 2; }
	@script/dogfood-seal.sh "$(BASE)"

dogfood-bind-range:
	@test -n "$(BASE)" || { echo "BASE is required" >&2; exit 2; }
	@test -n "$(TARGET)" || { echo "TARGET is required" >&2; exit 2; }
	@script/dogfood-bind-range.sh "$(BASE)" "$(TARGET)"

dogfood-bind-range-test:
	@script/dogfood-bind-range_test.sh

.PHONY: tasks-build tasks-test
# The in-tree corvint-tasks companion binary (decision 0397). It is never a corvint subcommand;
# tasks-test runs its own packages, which also stay in go-test's ./... because they share the module.
CORVINT_TASKS_BIN ?= corvint-tasks
tasks-build: go-version
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -ldflags "-X main.build=$$(git rev-list --count --first-parent HEAD)" -o $(CORVINT_TASKS_BIN) ./cmd/corvint-tasks

tasks-test: go-version
	$(GO_TEST_COMMAND) ./cmd/corvint-tasks/... ./internal/tasks/...
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go vet ./cmd/corvint-tasks/... ./internal/tasks/...
