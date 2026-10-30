BASE ?= main
DOGFOOD_GOCACHE := $(or $(CORVINT_GOCACHE),$(GOCACHE),$(or $(XDG_CACHE_HOME),$(HOME)/.cache)/corvint-go-build-cache)
CORVINT_GOCACHE ?= /tmp/corvint-go-build-cache
CORVINT_BIN ?= corvint
.DEFAULT_GOAL := gate
# GO_TEST_TIMEOUT is the per-package hang detector, not a performance budget (decision 0082).
# cmd/corvint measured 596.0s and internal/tasks/store 1,304-1,448s (V1-1294) against Go's former
# implicit 10m package deadline. Frozen replay has a separate 9m context covering cold candidate
# build and replay; the 30m package deadline does not extend it. Root -p 1 serializes top-level Go
# build/test programs as a bounded scheduling experiment; nested build flags and intra-package
# concurrency remain unchanged. Cost: docs/agent-memory/optimizations.md; no deadline changes here.
GO_TEST_TIMEOUT ?= 30m

# GO_TEST_COMMAND is the go test invocation of the specindex, core-n1 and tasks targets; go-test
# instead runs the declaration .github/cishards/test-commands.json through tools/test-commands-run
# (STR-V0-006; test-commands-check keeps that rule to one line). GO_TEST_FLAGS is the invocation
# without -count=1: the ledger tier (GL-V0-004) adds it itself where a package's reads lack bounds.
GO_TEST_FLAGS = -p 1 -timeout $(GO_TEST_TIMEOUT)
GO_TEST_COMMAND = $(HEAVY_NICE_PREFIX) env GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) -count=1

.PHONY: build gate capability-catalog capability-catalog-check instruction-conflicts gate-receipt-clear gate-receipt-test gate-affected gate-affected-test host-adapter-test host-adapter-node-test go-version go-test go-vet cross-vet windows-vet go-format-check go-format-test go-archive-gate go-archive-gate-test interop-gate spec-requirements spec-requirements-resolve spec-requirements-check spec-requirements-test spec-acceptance-audit requirement-definitions-check requirement-definitions-test traceability-tests-check traceability-tests-test decision-numbers-check decision-numbers-test analyzer-schema-check analyzer-schema-test instruction-conflicts-test eol-policy-check eol-policy-test license-headers-check license-headers-test line-citations-check line-citations-test ci-least-privilege-check ci-least-privilege-test unbounded-readers-check test-read-scopes-check test-commands-check capabilities-check release-checklist-test analyzer-python-offline-build-test analyzer-python-ratchets-test release-artifact-reproducibility-test sql-native-ratchets sql-native-ratchets-test companion-release-gate public-release-check core-n1-replay dogfood-change dogfood-check dogfood-seal dogfood-finish dogfood-inputs-check dogfood-batch-check dogfood-bind-range dogfood-bind-range-test lane-scripts-test error-code-ownership-check error-code-ownership-test use-case-receipts-check use-case-receipts-test host-package-versions-check host-package-versions-test host-package-version-bump host-package-version-verify diagnostic-coverage-check go-archive-gate-injection-test install-lifecycle-test hostile-regressions-check hostile-regressions-test no-python-runtime-dependency-test goos-filenames-check goos-filenames-test go-cache-trim go-cache-trim-test scratch-sweep scratch-sweep-test pre-push-windows-vet-test pre-push-dogfood-seal-test dogfood-closeout cem-stable-regen cem-stable-regen-capture cem-stable-regen-replace cem-stable-regen-widen cem-stable-regen-check cem-release-suites cem-release-gate spec-toolchain-conformance spec-toolchain-latest-test batch-precheck batch-precheck-test ai-coauthor-trailers-check ai-coauthor-trailers-test dogfood-cache-test dogfood-bin-reuse-test

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
GATE_STEPS = host-adapter-test go-version go-test go-vet cross-vet go-format-check go-format-test go-archive-gate go-archive-gate-test interop-gate spec-requirements-check spec-requirements-test requirement-definitions-check requirement-definitions-test traceability-tests-check traceability-tests-test decision-numbers-check decision-numbers-test analyzer-schema-check analyzer-schema-test instruction-conflicts-test eol-policy-check eol-policy-test license-headers-check license-headers-test line-citations-check line-citations-test ci-least-privilege-check ci-least-privilege-test unbounded-readers-check test-read-scopes-check test-commands-check release-checklist-test gate-receipt-test error-code-ownership-check error-code-ownership-test host-package-versions-check host-package-versions-test diagnostic-coverage-check no-python-runtime-dependency-test goos-filenames-check goos-filenames-test go-cache-trim-test scratch-sweep-test pre-push-windows-vet-test pre-push-dogfood-seal-test batch-precheck-test ai-coauthor-trailers-test dogfood-cache-test dogfood-bin-reuse-test
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
	@$(HEAVY_NICE_PREFIX) env $(GATE_LEDGER) run $* -- $(SUB_MAKE) $*
endif

# Native adapter and opt-in handoff regressions are owned by the reproducible source gate.
# host-adapter-node-test is its node half; batch-precheck runs it too, so an adapter version or
# capability-tuple drift fails before merge instead of only in the nightly (ASD-V0-044, V1-3062).
host-adapter-node-test:
	node --test integrations/opencode/*.test.mjs
	node --test integrations/pi/runtime.test.mjs integrations/pi/extension.test.mjs integrations/pi/tools.test.mjs integrations/pi/process.test.mjs integrations/pi/core.test.mjs integrations/pi/capabilities.test.mjs integrations/pi/operations.test.mjs integrations/pi/tasks.test.mjs integrations/pi/cockpit.test.mjs integrations/pi/workflow.test.mjs

host-adapter-test: host-adapter-node-test
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test -count=1 -timeout $(GO_TEST_TIMEOUT) ./cmd/corvint -run '^(TestDogfoodEvent|TestCodexHostAdapter|TestClaudeNativeDogfoodLifecycle|TestClaudeSourceHandoffCLI|TestHostAdapter|TestPi)'

go-version:
	test "$$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.3"

go-test:
	go run ./tools/test-commands-run

# gate-affected is the fast tier: `corvint test` (STR-V0-012), built from this checkout by
# script/gate-affected-corvint.sh, runs the Go units the change reaches as test-commands.json
# declares, or FULL with the reason on stderr; then the cheap non-test checks run. BASE (default
# main) joins BASE..HEAD to the dirty worktree, `BASE=` tests the worktree alone, and a BASE that
# does not resolve is a usage error. Not the push gate: `make gate` stays that. Rollback:
# script/gate-affected.sh stays (docs/specs/session-test-runner-v0.md, Rollout).
gate-affected: go-version
	@script/gate-affected-corvint.sh "$(BASE)"
	@$(MAKE) go-vet go-format-check spec-requirements-check decision-numbers-check line-citations-check

gate-affected-test:
	@script/gate-affected_test.sh
	@script/gate-affected-corvint_test.sh

go-vet:
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go vet ./...

# cross-vet catches cross-OS compile gaps that go-vet's native-host run cannot see
# (docs/agent-memory/tests.md, 2026-09-12): it type-checks the root module and interop/cem01-go
# for GOOS=windows and GOOS=linux, and for the unqualified Unix platforms freebsd, openbsd,
# netbsd and solaris (PRS-V1-019, V1-0393). CGO stays disabled so the check never depends on a
# cross-compiling C toolchain being installed.
CROSS_VET_UNQUALIFIED_GOOS = freebsd openbsd netbsd solaris
cross-vet:
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go vet ./...
	cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
	cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go vet ./...
	set -e; for goos in $(CROSS_VET_UNQUALIFIED_GOOS); do \
		GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=$$goos GOARCH=amd64 CGO_ENABLED=0 go vet ./...; \
		(cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local GOOS=$$goos GOARCH=amd64 CGO_ENABLED=0 go vet ./...); \
	done

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

# cem-stable-regen adds one additive member to the sealed cem/1.0 stable result and re-binds every
# digest that seals it (CEM-V1-091). Capture the native values first, then regenerate:
#   make cem-stable-regen-capture CAPTURE=/tmp/member.jsonl
#   make cem-stable-regen MEMBER=<name> SCHEMA=<fragment.json> CAPTURE=/tmp/member.jsonl [VALUES=<overrides.json>] \
#     [CELL_VALUES=<cell-overrides.json>] NOTE="<driver>"
# cem-stable-regen-replace re-emits recorded values of existing members (CEM-V1-091, decision 0530):
#   make cem-stable-regen-replace MEMBER=<name[,name]> RECORD=<record.json> CAPTURE=/tmp/member.jsonl \
#     [VALUES=<overrides.json>] [CELL_VALUES=<cell-overrides.json>]
# cem-stable-regen-widen adds recorded values to one string const or enum of both RESULT.schema.json
# copies (CEM-V1-620): make cem-stable-regen-widen RECORD=<widen-record.json>
# cem-stable-regen-check verifies the sealed chain without writing.
AFTER ?= artifactChecks

cem-stable-regen-capture:
	@test -n "$(CAPTURE)" || { echo "set CAPTURE=<file.jsonl>" >&2; exit 2; }
	rm -f "$(abspath $(CAPTURE))"
	CEM_STABLE_REGEN_CAPTURE="$(abspath $(CAPTURE))" GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test -count=1 ./internal/cem/verify -run '^TestStableS0EPublicCases$$'

cem-stable-regen:
	@test -n "$(MEMBER)" && test -n "$(SCHEMA)" && test -n "$(NOTE)" || { echo "set MEMBER, SCHEMA, NOTE and CAPTURE or VALUES" >&2; exit 2; }
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/cem-stable-regen --member "$(MEMBER)" --schema "$(SCHEMA)" --after "$(AFTER)" $(if $(CAPTURE),--capture "$(CAPTURE)") $(if $(VALUES),--values "$(VALUES)") $(if $(CELL_VALUES),--cell-values "$(CELL_VALUES)") --note "$(NOTE)"

cem-stable-regen-replace:
	@test -n "$(MEMBER)" && test -n "$(RECORD)" || { echo "set MEMBER, RECORD and CAPTURE or VALUES" >&2; exit 2; }
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/cem-stable-regen --replace --member "$(MEMBER)" --record "$(RECORD)" $(if $(CAPTURE),--capture "$(CAPTURE)") $(if $(VALUES),--values "$(VALUES)") $(if $(CELL_VALUES),--cell-values "$(CELL_VALUES)")

cem-stable-regen-widen:
	@test -n "$(RECORD)" || { echo "set RECORD=<widen-record.json>" >&2; exit 2; }
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/cem-stable-regen --widen --record "$(RECORD)"

cem-stable-regen-check:
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/cem-stable-regen --check

# cem-release-suites runs the sealed protocol/cem-1.0 stable suites against two built release
# binaries and refuses unless both report the same clean commit, which the checkout must be at
# (CEM-V1-572, V1-2323):
#   make cem-release-suites CORVINT_RELEASE_BIN=<corvint> CEM01_RELEASE_BIN=<cem01-go>
# Build stamps are unauthenticated, so the tool reports the revision and each binary's SHA-256
# rather than proving provenance. Every input is read from that commit's Git objects, not the
# worktree. The tamper matrix, citation vectors, neutral criterion and receipt authentication run
# through both binaries. The repository-envelope cells are prose S0E recipes, so the tool runs
# them as source-level Go tests in the same committed tree; that is not a binary-level run.
cem-release-suites:
	@test -n "$(CORVINT_RELEASE_BIN)" && test -n "$(CEM01_RELEASE_BIN)" || { echo "set CORVINT_RELEASE_BIN and CEM01_RELEASE_BIN" >&2; exit 2; }
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/cem-release-suites --corvint "$(CORVINT_RELEASE_BIN)" --cem01 "$(CEM01_RELEASE_BIN)"

# cem-release-gate builds corvint and cem01-go from the clean committed HEAD into a private
# directory, runs the Node check of the committed readiness declaration, then the sealed suites
# with release readiness over both binaries (CEM-V1-600..605, V1-2323). Exit 3 is the readiness
# refusal: the committed declaration names rows one host cannot observe (binary-level repository
# envelope, the other release platform, external interoperability), so a pass on this host is
# never release support. It is opt-in, not a `make gate` step; CI wiring is owner .github work.
# CEM_RELEASE_RECORD=PATH writes this platform's cem-release-run/1 record and
# CEM_RELEASE_OUTCOME=PATH the cem-release-gate-outcome/1 verdict (CEM-V1-611, CEM-V1-613); both
# paths must not exist yet. `go run ./tools/cem-release-suites --aggregate --readiness PATH
# --outcome OUT RECORD...` judges the records of one revision from several hosts (CEM-V1-612).
cem-release-gate: go-version
	@test -z "$$(git status --porcelain)" || { echo "cem-release-gate: the checkout has changes or untracked files; release binaries are built from a clean HEAD" >&2; exit 2; }
	node --test tools/cem-release-suites/readiness.test.mjs
	@bin=$$(mktemp -d) && trap '/bin/rm -rf "$$bin"' EXIT INT TERM && \
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -o "$$bin/corvint" ./cmd/corvint && \
	(cd interop/cem01-go && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -o "$$bin/cem01-go" .) && \
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/cem-release-suites --corvint "$$bin/corvint" --cem01 "$$bin/cem01-go" --readiness tools/cem-release-suites/READINESS.json $(if $(CEM_RELEASE_RECORD),--record "$(CEM_RELEASE_RECORD)") $(if $(CEM_RELEASE_OUTCOME),--outcome "$(CEM_RELEASE_OUTCOME)")

# spec-requirements regenerates docs/specs/REQUIREMENTS.tsv and projects the spec headers onto
# docs/specs/INDEX.json and the docs/specs/README.md table (SRG-V0-012). The generator reads the
# Git index, so stage the changed spec first. One generator pass writes all three (SRG-V0-018);
# nothing is copied unless all three regenerate.
SPEC_GENERATE = script/gen-spec-requirements.sh --all "$$tmp"
SPEC_GENERATED = REQUIREMENTS.tsv INDEX.json README.md

spec-requirements:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT INT TERM; $(SPEC_GENERATE) && \
	for file in $(SPEC_GENERATED); do cp "$$tmp/$$file" "docs/specs/$$file" || exit 1; done

# spec-requirements-resolve finishes a merge whose only conflicts left are in the generated
# REQUIREMENTS.tsv, INDEX.json or README.md: it regenerates and stages them (SRG-V0-041).
spec-requirements-resolve:
	@script/spec-requirements-resolve.sh

spec-requirements-check:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT INT TERM; $(SPEC_GENERATE) || exit 1; stale=0; \
	for file in $(SPEC_GENERATED); do cmp "$$tmp/$$file" "docs/specs/$$file" || stale=1; done; \
	test $$stale = 0 || { echo "docs/specs/REQUIREMENTS.tsv, INDEX.json or README.md is stale: stage the changed spec, then run make spec-requirements" >&2; exit 1; }
	$(GO_TEST_COMMAND) ./internal/specindex

# spec-requirements-test pins SRG-V0-001: the generator enumerates the Git index, so no
# untracked, deleted-from-the-worktree, or unstaged spec can change the index a fresh clone
# reproduces. It is the drift detector for a return to a worktree listing. It also pins the
# acceptance columns, the header projection and the audit (SRG-V0-009, -010, -012) and shows that
# two lanes adding requirements to one spec conflict only in its requirement list and the TSV
# rows (SRG-V0-013).
spec-requirements-test:
	@script/gen-spec-requirements_test.sh
	@script/spec-requirements-conflict_test.sh

# spec-acceptance-audit lists the requirements of one acceptance mode by spec, standing by
# default, and reports a validated spec that holds a standing requirement as NOT_PROMOTABLE
# (SRG-V0-010). It is a report, not a gate step: ACCEPTANCE=owner or unrecorded lists the others.
ACCEPTANCE ?= standing
spec-acceptance-audit:
	@script/spec-acceptance-audit.sh --acceptance $(ACCEPTANCE)

# requirement-definitions-check fails when a requirement id survives only as a table row.
# REQUIREMENTS.tsv is generated by an extractor that falls back to a table row when no clause
# body defines an id, so a deleted clause body otherwise leaves every gate green.
requirement-definitions-check:
	@script/check-requirement-definitions.sh

# requirement-definitions-test runs the gate's own fixture cases (V1-2311); before it existed no
# target ran script/check-requirement-definitions_test.sh.
requirement-definitions-test:
	@script/check-requirement-definitions_test.sh

traceability-tests-check:
	@script/check-traceability-tests.sh

traceability-tests-test:
	@script/check-traceability-tests_test.sh

# decision-numbers-check grandfathers exactly the cross-lineage 0016 pair (decision 0048) and
# fails on any other collision, and fails when docs/decisions/README.md's index is missing a
# tracked decision file's row or cites the same file's row twice (V1-0265).
#
# It also runs analyzer-schema-check, the other numbered identifier concurrent pull requests can
# collide on: two branches that bump corvint-analyzer to the same number with different audit
# digests merge without a textual conflict on the number (V1-1732, IDX-SNAP-V0-061). CI's
# documentation step reaches it through this target until that step names it directly.
decision-numbers-check: analyzer-schema-check
	@script/check-decision-numbers.sh

decision-numbers-test:
	@script/check-decision-numbers_test.sh

# analyzer-schema-check refuses a stale analyzer audit digest and a branch that changed the
# analyzer pins while keeping a schema number origin/main already uses; `script/analyzer-schema.sh
# bump` is the one-command recovery for both (V1-2130, V1-1732, IDX-SNAP-V0-060/061).
analyzer-schema-check:
	@script/analyzer-schema.sh check

analyzer-schema-test:
	@script/analyzer-schema_test.sh

# instruction-conflicts is a local, read-only doctor and never a gate step: it compares AGENTS.md and
# CLAUDE.md with the host files named in INSTRUCTION_HOST_FILES (hook settings, a memory directory,
# global instruction files) and reports CONFLICT, NO_CONFLICT_FOUND or UNKNOWN (V1-0721). Host files
# are not Git-pinned, so its findings are local and uncertain. instruction-conflicts-test runs its
# fixtures, which reproduce the two 2026-10-04 contradictions, and is a gate step.
# INSTRUCTION_HOST_FILES splits on spaces; call the script directly for a path containing one.
# Without it the target gives the repository-layer verdict and reports the host layer NOT_OBSERVED
# (--repository-only, V1-1756).
INSTRUCTION_HOST_FILES ?=
instruction-conflicts:
	@script/check-instruction-conflicts.sh $(if $(strip $(INSTRUCTION_HOST_FILES)),$(addprefix --host ,$(INSTRUCTION_HOST_FILES)),--repository-only)

instruction-conflicts-test:
	@script/check-instruction-conflicts_test.sh

# eol-policy-check fails when a tracked path stops reporting `attr/-text` or commits CRLF or
# mixed endings, exempting only the two intentional interop fixtures. Decision 0061 made
# .gitattributes repository-wide `* -text`, so Git normalises nothing and a CRLF-committing
# contributor is not corrected automatically; this is where that drift is caught.
eol-policy-check:
	@script/check-eol-policy.sh

eol-policy-test:
	@script/check-eol-policy_test.sh

# license-headers-check fails when a tracked non-Markdown file outside the historical records carries a
# former AGPL SPDX identifier, or a non-Markdown file under examples/ lacks
# `SPDX-License-Identifier: Apache-2.0` (decision 0586; LICENSING.md is the authority).
license-headers-check:
	@script/check-license-headers.sh

license-headers-test:
	@script/check-license-headers_test.sh

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

# ci-least-privilege-check fails when any tracked .github/workflows/*.yml workflow-level permissions
# stop being exactly `contents: read` (or a job-level block grants more) without a matching row in
# script/ci-least-privilege-exceptions.tsv, an actions/checkout step stops disabling persisted
# credentials, or any `uses:` reference stops being pinned to a full 40-character commit SHA
# (ARTIFACT-V0-008, docs/specs/release-artifact-integrity-v0.md).
ci-least-privilege-check:
	@script/check-ci-least-privilege.sh

ci-least-privilege-test:
	@script/check-ci-least-privilege_test.sh

# spec-toolchain-conformance installs the OpenSpec and Spec Kit releases pinned in
# interop/spec-toolchains/versions.json with checksum verification, runs their validators, and
# asserts that Corvint's derived obligations equal upstream's own view (STA-V0-011,
# docs/specs/spec-toolchain-adapters-v0.md). It needs the npm registry, GitHub and PyPI, so it is
# not a gate step: the path-filtered spec-toolchain-conformance workflow runs it.
spec-toolchain-conformance:
	@script/spec-toolchain-conformance.sh

# spec-toolchain-latest-test checks script/spec-toolchain-latest.sh (STA-V0-012) offline against
# recorded registry answers; the script itself is operator-run and needs the network. Like
# spec-toolchain-conformance it is not a gate step (so not a shell-test prerequisite either): the
# spec-toolchain-conformance workflow runs it.
spec-toolchain-latest-test:
	@script/spec-toolchain-latest_test.sh

# unbounded-readers-check fails unless the test packages selected on every change are exactly
# the units recorded in .corvint/unbounded-readers.json (AFP-V0-025,
# docs/specs/affected-plan-v0.md).
unbounded-readers-check:
	@GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/unbounded-readers >/dev/null

# test-read-scopes-check fails, naming the package and path, when a test file of a package declared
# in .corvint/test-read-scopes.json carries a literal naming a tracked repository path outside that
# package's entries, which the confined CI shard would deny (AFP-V0-051, docs/specs/affected-plan-v0.md).
test-read-scopes-check:
	@GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/test-read-scopes-check >/dev/null

# test-commands-check reads this Makefile's text (not make's output) and fails unless the go-test
# rule is exactly `go-test:` + `go run ./tools/test-commands-run` (the runner of the declaration
# .github/cishards/test-commands.json), no line here or in an included file sets SHELL,
# .SHELLFLAGS or MAKEFLAGS or defines a target- or pattern-specific variable that could reach
# go-test, every declared language command that names a make target exists, and no GATE_STEPS
# recipe runs a suite of a language declared none (STR-V0-006, docs/specs/session-test-runner-v0.md).
test-commands-check:
	@GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/test-commands-check >/dev/null

# capabilities-check fails when docs/CAPABILITIES.md or the README capability table drifts from the
# commands, cmd/ binaries and MCP tools in the code (CAP-V0-003, docs/specs/capability-catalog-v0.md).
# go test ./tools/capabilities-check runs the same check, so it is not a separate GATE_STEPS step.
capabilities-check:
	@GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/capabilities-check

# capability-catalog regenerates internal/capcatalog/catalog.json from the binary's own
# `corvint help --json`, and the command-tree snapshots under internal/capcatalog/trees from each
# binary's own `help --tree` / `help --json`; capability-catalog-check fails when a snapshot, the
# shipped agent guide or any node's help drifts from the code (CAP-V0-019, CAP-V0-040 to CAP-V0-048,
# docs/specs/capability-catalog-v0.md).
capability-catalog:
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./cmd/corvint help --json > internal/capcatalog/catalog.json
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./cmd/corvint help --tree > internal/capcatalog/trees/corvint.json
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./cmd/corvint-tasks help --json > internal/capcatalog/trees/corvint-tasks.json
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./cmd/corvint-test help --json > internal/capcatalog/trees/corvint-test.json

capability-catalog-check:
	$(GO_TEST_COMMAND) -run '^TestCAPV0(015|016|017|018|019|020|021|022|023|024|025|026|027|028|029|030|031|032|04[0-9])' ./cmd/corvint ./internal/capcatalog ./internal/clitree ./internal/tasks/cli ./cmd/corvint-test ./tools/capabilities-check

# host-package-versions-check fails when a host package manifest's version field (AHI-020,
# docs/specs/agent-harness-integration-v0.md) was last bumped in a commit older than the last
# commit that changed any other shipped file in that package. AHI-240: it also verifies that every
# version location of every package agrees, so a manifest-only bump fails here.
host-package-versions-check:
	@script/check-host-package-versions.sh
	@script/host-package-version.sh verify >/dev/null

host-package-versions-test:
	@script/check-host-package-versions_test.sh
	@script/host-package-version_test.sh

# AHI-173: bump every version location of one host package together (VERSION defaults to the next
# patch), or verify that every package's locations agree.
host-package-version-bump:
	@test -n "$(PKG)" || { echo 'usage: make host-package-version-bump PKG=<package> [VERSION=X.Y.Z]' >&2; exit 2; }
	@script/host-package-version.sh bump $(PKG) $(VERSION)

host-package-version-verify:
	@script/host-package-version.sh verify $(PKG)

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

# goos-filenames-check fails when a Go file under cmd/ or internal/ has a GOOS/GOARCH filename
# suffix outside darwin/linux/windows/amd64/arm64 (`foo_js.go` builds only for GOOS=js) and is
# not in script/goos-filename-allowlist.txt (ASD-V0-020, V1-1910).
goos-filenames-check:
	@script/check-goos-filenames.sh

goos-filenames-test:
	@script/check-goos-filenames_test.sh

# go-cache-trim bounds the Go build caches and removes stale go-build* work directories; it is a
# dry run unless APPLY=1 (ASD-V0-021, V1-0360). Extra options go in GO_CACHE_TRIM_ARGS.
go-cache-trim:
	@script/go-cache-trim.sh $(if $(filter 1,$(APPLY)),--apply) $(GO_CACHE_TRIM_ARGS)

go-cache-trim-test:
	@script/go-cache-trim_test.sh

# scratch-sweep prunes worktree registrations whose directory is gone and removes closed-session
# scratch older than 24 h, archiving unfinished Git work first; dry run unless APPLY=1 (ASD-V0-022,
# V1-1978). Pass --root/--repo/--keep/--archive in SCRATCH_SWEEP_ARGS.
scratch-sweep:
	@script/scratch-sweep.sh $(if $(filter 1,$(APPLY)),--apply) $(SCRATCH_SWEEP_ARGS)

scratch-sweep-test:
	@script/scratch-sweep_test.sh

# pre-push-windows-vet-test pushes through script/pre-push-windows-vet.sh, the installable Git
# pre-push hook that runs make windows-vet when Go inputs change (ASD-V0-023, V1-1725).
pre-push-windows-vet-test:
	@script/pre-push-windows-vet_test.sh

# pre-push-dogfood-seal-test pushes through script/pre-push-dogfood-seal.sh, the installable Git
# pre-push hook that refuses a branch whose commits no sealed CEM binds (DCW-V0-348, V1-2805).
pre-push-dogfood-seal-test:
	@script/pre-push-dogfood-seal_test.sh

# batch-precheck runs, in a few minutes, every generated-artifact drift, catalog and import-direction
# check batch CI runs: the doc-gates steps it derives from ci.yml, after the mechanical repins; gofmt;
# the capability catalog; and the cross-lane drift tests as one offloaded go test. Independent steps
# run in parallel and it prints a PASS/FAIL table with per-step seconds; a full pass on a clean,
# unmoved checkout writes a receipt bound to HEAD (ASD-V0-029, V1-2551; ASD-V0-034 to 036,
# V1-2899/V1-2900). batch-regen regenerates every derived file in one commit (ASD-V0-033), and
# batch-push-check, run before `git push` of a claude/batch-* branch, refuses unless the receipt
# names REV (default HEAD; ASD-V0-035).
# With BASE=<sha> from the command line or environment, a failure the base already shows prints as
# a base-present WARN and only introduced ones fail (ASD-V0-032, V1-2895). A step whose command,
# environment and declared inputs are unchanged since a ledger pass prints REUSE instead of rerunning
# (ASD-V0-042, V1-2985; CORVINT_GATE_LEDGER=off reruns every step).
# batch-review-packet BASE=<sha> [HEAD=<rev>] prints the bounded, read-only batch reviewer input for
# the checked-out merged range: the reviewed revision, lane overlap, corvint impact and dogfood
# batch-check (ASD-V0-043, V1-2986).
.PHONY: batch-regen batch-push-check batch-review-packet
batch-precheck:
	@script/batch-precheck.sh $(if $(DOGFOOD_BASE),--base '$(DOGFOOD_BASE)')

batch-regen:
	@script/batch-regen.sh

batch-push-check:
	@script/batch-precheck.sh --verify-receipt $(or $(REV),HEAD)

batch-precheck-test:
	@script/batch-precheck_test.sh

# ai-coauthor-trailers-check [DOGFOOD_BASE=<sha>] refuses, naming each commit, when base..HEAD
# (default origin/main..HEAD) has a Co-authored-by trailer for an AI agent; make batch-precheck runs
# it as a step. script/check-ai-coauthor-trailers.sh --install-hook refuses one before the commit
# exists (ASD-V0-560, V1-3227).
ai-coauthor-trailers-check:
	@script/check-ai-coauthor-trailers.sh $(if $(DOGFOOD_BASE),'$(DOGFOOD_BASE)')

ai-coauthor-trailers-test:
	@script/check-ai-coauthor-trailers_test.sh

batch-review-packet:
	@test -n "$(DOGFOOD_BASE)" || { echo "$(DOGFOOD_BASE_REFUSAL)" >&2; exit 2; }
	@script/batch-review-packet.sh '$(DOGFOOD_BASE)' '$(or $(HEAD),HEAD)'

# diagnostic-coverage-check fails when a diagnostic.Refusal literal lacks a subject, a registered
# fix list or terminal reason, when Go that imports internal/diagnostic parses a refusal message,
# or when the covered-site count differs from script/diagnostic-coverage.count (DRC-V0-011/012).
diagnostic-coverage-check:
	@script/check-diagnostic-coverage.sh

install-lifecycle-test:
	@script/check-install-lifecycle_test.sh
	@script/select-previous-release_test.sh
	@script/core-n1-source_test.sh
	@script/core-n1-replay-default_test.sh

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
# builds CORE_N1_TAG from `git archive` and runs every frozen Core mode under it beside this build
# (TestCoreVerbsEmitTheFrozenProfiles), then each Core verb's refusal under it
# (TestCoreRefusalsKeepTheFrozenEnvelope, CCF-V1-010). Left unset, the tag is the one
# script/select-previous-release.sh picks for HEAD, the same selector script/core-n1-source.sh uses,
# so pre-release tags count (V1-3203); when no release tag precedes HEAD it reports NOT_APPLICABLE
# FIRST_RELEASE and exits 0, and a selector refusal exits 2. An empty CORE_N1_TAG= is refused.
CORE_N1_TAG_GIVEN = $(if $(filter undefined,$(origin CORE_N1_TAG)),,1)

core-n1-replay:
	@tag='$(CORE_N1_TAG)'; \
	if [ -z '$(CORE_N1_TAG_GIVEN)' ]; then \
	  code=0; tag=$$(script/select-previous-release.sh "$$(git rev-parse HEAD)") || code=$$?; \
	  case $$code in \
	    0) echo "core-n1-replay: CORE_N1_TAG=$$tag, the newest release tag before HEAD" >&2 ;; \
	    3) echo "core-n1-replay: NOT_APPLICABLE FIRST_RELEASE: no release tag precedes HEAD; the N-1 checks are not applicable"; exit 0 ;; \
	    *) echo "core-n1-replay: previous release selection refused (exit $$code); pass CORE_N1_TAG=<tag>" >&2; exit 2 ;; \
	  esac; \
	elif [ -z "$$tag" ]; then echo "CORE_N1_TAG is required" >&2; exit 2; fi; \
	tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && mkdir "$$tmp/src" && \
	git archive -o "$$tmp/src.tar" "$$tag" && tar -x -C "$$tmp/src" -f "$$tmp/src.tar" && \
	(cd "$$tmp/src" && GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -o "$$tmp/corvint" ./cmd/corvint) && \
	CORVINT_CORE_N1_BINARY="$$tmp/corvint" $(GO_TEST_COMMAND) -run '^(TestCoreVerbsEmitTheFrozenProfiles|TestCoreRefusalsKeepTheFrozenEnvelope)$$' ./cmd/corvint

# Dogfood targets never use the file default `BASE ?= main` (local main may not be origin/main):
# BASE must come from the command line or the environment, or the target refuses before any work
# (ASD-V0-019, V1-1755). dogfood-finish passes the empty value on so its one-line refusal names it.
DOGFOOD_BASE = $(if $(filter file,$(origin BASE)),,$(BASE))
DOGFOOD_BASE_REFUSAL = BASE is required: pass BASE=<full origin/main sha>; the Makefile default (main) is not used by dogfood targets

dogfood-change:
	@test -n "$(DOGFOOD_BASE)" || { echo "$(DOGFOOD_BASE_REFUSAL)" >&2; exit 2; }
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" $(HEAVY_NICE_PREFIX) script/dogfood-change.sh "$(DOGFOOD_BASE)"

# Without BASE, dogfood-check takes the base of the CEM committed at HEAD, and the script refuses
# naming BASE when none is bound (ASD-V0-085, V1-3172).
dogfood-check:
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" $(HEAVY_NICE_PREFIX) script/dogfood-check.sh $(if $(DOGFOOD_BASE),"$(DOGFOOD_BASE)",--bound-base)

dogfood-seal:
	@test -n "$(DOGFOOD_BASE)" || { echo "$(DOGFOOD_BASE_REFUSAL)" >&2; exit 2; }
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" script/dogfood-seal.sh "$(DOGFOOD_BASE)"

# Each input reaches the wrapper as one single-quoted word, whatever bytes its path holds.
dogfood_finish_quote = '$(subst ','\'',$(1))'

# script/dogfood-finish.sh refuses every missing input at once; LANE=1 also requires INTENTS,
# VERIFY and OUTCOME (ASD-V0-015).
dogfood-finish:
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" DOGFOOD_FINISH_LANE=$(call dogfood_finish_quote,$(LANE)) script/dogfood-finish.sh $(call dogfood_finish_quote,$(DOGFOOD_BASE)) \
		$(call dogfood_finish_quote,$(CITATIONS)) $(call dogfood_finish_quote,$(INTENTS)) \
		$(call dogfood_finish_quote,$(VERIFY)) $(call dogfood_finish_quote,$(OUTCOME))

# dogfood-inputs-check is the input pre-flight alone (DCW-V0-380): it builds only the tree binary and
# checks VERIFY, INTENTS and CITATIONS against a freshly prepared map, binding nothing.
dogfood-inputs-check:
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" DOGFOOD_FINISH_PREFLIGHT=1 DOGFOOD_FINISH_LANE=$(call dogfood_finish_quote,$(LANE)) script/dogfood-finish.sh $(call dogfood_finish_quote,$(DOGFOOD_BASE)) \
		$(call dogfood_finish_quote,$(CITATIONS)) $(call dogfood_finish_quote,$(INTENTS)) \
		$(call dogfood_finish_quote,$(VERIFY)) $(call dogfood_finish_quote,$(OUTCOME))

# V1-1894 / DCW-V0-349: CI and local closeout consume this same ordered list.
DOC_GATE_TARGETS := host-package-versions-check host-package-versions-test spec-requirements-check requirement-definitions-check traceability-tests-check traceability-tests-test decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check test-read-scopes-check test-commands-check use-case-receipts-check use-case-receipts-test
.PHONY: doc-gates
doc-gates:
	@$(MAKE) --no-print-directory $(DOC_GATE_TARGETS)
	@GOTOOLCHAIN=local go run ./conformance/use-cases-v0

# dogfood-closeout is the one lane closeout command (DCW-V0-349, V1-1894): dogfood-finish with the
# same variables, whose input pre-flight refuses a bad VERIFY, INTENTS or CITATIONS file before the
# base build or any bind (DCW-V0-380), then batch-precheck, which derives the shared doc-gates
# target from ci.yml; batch-precheck-test checks that connection; it stops at the first failing step.
dogfood-closeout:
	@$(MAKE) --no-print-directory dogfood-finish
	@$(MAKE) --no-print-directory batch-precheck

dogfood-batch-check:
	@test -n "$(DOGFOOD_BASE)" || { echo "$(DOGFOOD_BASE_REFUSAL)" >&2; exit 2; }
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" script/dogfood-batch-check.sh "$(DOGFOOD_BASE)"

dogfood-bind-range:
	@test -n "$(DOGFOOD_BASE)" || { echo "$(DOGFOOD_BASE_REFUSAL)" >&2; exit 2; }
	@test -n "$(TARGET)" || { echo "TARGET is required" >&2; exit 2; }
	@CORVINT_GOCACHE="$(DOGFOOD_GOCACHE)" script/dogfood-bind-range.sh "$(DOGFOOD_BASE)" "$(TARGET)"

dogfood-bind-range-test:
	@script/dogfood-bind-range_test.sh

# The dogfood wrapper fixtures run in the gate and CI's shell job (ASD-V0-091, V1-3250).
dogfood-cache-test:
	@script/dogfood-cache_test.sh

dogfood-bin-reuse-test:
	@script/dogfood-bin-reuse_test.sh

# The lane procedure scripts (docs/LANE-ORCHESTRATION.md, ASD-V0-014..018); no Go build.
lane-scripts-test:
	@script/lane-scripts_test.sh

.PHONY: tasks-build tasks-test
# The in-tree corvint-tasks companion binary (decision 0397). It is never a corvint subcommand;
# tasks-test runs its own packages, which also stay in go-test's ./... because they share the module.
CORVINT_TASKS_BIN ?= corvint-tasks
tasks-build: go-version
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -trimpath -ldflags "-X main.build=$$(git rev-list --count --first-parent HEAD)" -o $(CORVINT_TASKS_BIN) ./cmd/corvint-tasks

tasks-test: go-version
	$(GO_TEST_COMMAND) ./cmd/corvint-tasks/... ./internal/tasks/...
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go vet ./cmd/corvint-tasks/... ./internal/tasks/...

# test-shard runs shard SHARD of SHARDS of one slow Go test package through script/heavy.sh, so a
# package over the agent's 600 s foreground limit runs as bounded pieces that together cover every
# top-level test exactly once (ASD-V0-026, tools/test-shards). Reference splits:
# PKG=./internal/tasks/store SHARDS=8 and PKG=./internal/tasks/cli SHARDS=4, SHARD=0..SHARDS-1.
.PHONY: test-shard
test-shard: go-version
	@test -n "$(PKG)" && test -n "$(SHARDS)" && test -n "$(SHARD)" || { echo "usage: make test-shard PKG=./internal/tasks/store SHARDS=8 SHARD=0" >&2; exit 2; }
	@sel=$$(script/heavy.sh env GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go run ./tools/test-shards -shards '$(SHARDS)' -shard '$(SHARD)' '$(PKG)') && \
	  flag=$${sel%% *} && pattern=$${sel#* } && \
	  HEAVY_WHOLE_REASON='make test-shard $(SHARD) of $(SHARDS) $(PKG)' script/heavy.sh env GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go test -count=1 -timeout $(GO_TEST_TIMEOUT) $(TEST_SHARD_FLAGS) "$$flag" "$$pattern" '$(PKG)'

# shell-test is the shell full command .github/cishards/test-commands.json declares (STR-V0-004):
# exactly the GATE_STEPS steps whose recipe runs a script/*_test.sh, so it runs the shell suites
# make gate runs and no others. TestSTRV0004ShellTestIsTheGateShellSteps
# (tools/test-commands-check) keeps the two lists equal.
.PHONY: shell-test
shell-test: go-format-test go-archive-gate-test spec-requirements-test requirement-definitions-test traceability-tests-test decision-numbers-test analyzer-schema-test instruction-conflicts-test eol-policy-test license-headers-test line-citations-test ci-least-privilege-test release-checklist-test gate-receipt-test error-code-ownership-test host-package-versions-test no-python-runtime-dependency-test goos-filenames-test go-cache-trim-test scratch-sweep-test pre-push-windows-vet-test pre-push-dogfood-seal-test batch-precheck-test ai-coauthor-trailers-test dogfood-cache-test dogfood-bin-reuse-test

# windows-vet is the local twin of the go-static step "Windows vet including test files
# (AFP-V0-042)" in .github/workflows/ci.yml: the same GOOS, GOARCH, CGO_ENABLED, empty GOFLAGS (no
# build tags) and GOWORK=off as that job, over the same two modules, test files included, on the
# go1.27.3 CI pin (decision 0557). GOENV=off ignores settings saved by `go env -w`, which an empty
# GOFLAGS alone would fall back to; the CI runner has none. Run it
# before pushing a change that touches a .go, go.mod or go.sum file (AFP-V0-054,
# docs/specs/affected-plan-v0.md); TestAFPV0054 keeps the two in step.
WINDOWS_VET_ENV = GOCACHE=$(CORVINT_GOCACHE) GOENV=off GOTOOLCHAIN=local GOWORK=off GOFLAGS= CGO_ENABLED=0 GOOS=windows GOARCH=amd64
windows-vet: go-version
	$(WINDOWS_VET_ENV) go vet ./...
	cd interop/cem01-go && $(WINDOWS_VET_ENV) go vet ./...

# Read-latency and memory budgets over a generated realistic-size fixture (PRB-V0-029,
# docs/specs/performance-profiling-and-read-budget-v0.md). It builds corvint and
# corvint-corpus-mcp, generates the fixture in a temporary directory (minutes, several GiB of
# memory) and measures six scenarios. The harness exits 1 when an enforced ceiling is exceeded or
# unknown (unless the run is NOT_JUDGED, below) and 2 when it cannot run; it is run as a built binary, not `go run`, so make's
# "Error 1" or "Error 2" line keeps the two apart. The #1598 and #1601 targets report as pending. PERF_BUDGET_ARGS passes options, for example
# --fixture DIR to reuse a fixture or --scale smoke. A contended host, or one without a load
# record, is NOT_JUDGED (PRB-V0-039..042): the target succeeds and says so on stderr, unless
# PERF_BUDGET_REQUIRE_JUDGEMENT=1, which makes it fail with "Error 3".
PERF_BUDGET_REQUIRE_JUDGEMENT ?=
.PHONY: perf-budget
perf-budget: go-version
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT INT TERM; \
	GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local go build -o "$$tmp/perf-budget" ./internal/perfbudget/cmd/perf-budget && \
	"$$tmp/perf-budget" $(if $(filter 1,$(PERF_BUDGET_REQUIRE_JUDGEMENT)),--require-judgement) $(PERF_BUDGET_ARGS)

# AHI-199 (V1-2440): heavy targets (GO_TEST_COMMAND, the ledger/% gate steps, dogfood-change/check)
# run below interactive priority so hook adapters keep their CPU share under lane test load. The
# STR-V0-006-pinned go-test and ledger/go-test recipes stay unprefixed; script/heavy.sh lowers them.
# HEAVY_NICE=0 opts out; a host without nice(1) runs them unchanged.
HEAVY_NICE ?= 10
HEAVY_NICE_PREFIX = $(if $(filter-out 0,$(HEAVY_NICE)),$(if $(shell command -v nice 2> /dev/null),nice -n $(HEAVY_NICE)))

# cce-live runs the live CCE prove tests and fails, rather than skips, when the Playwright
# toolchain (CORVINT_PLAYWRIGHT_MODULES, node) is missing (CCE-V0-102). A CI lane calls it on a
# macOS arm64 runner with a qualified Playwright tuple; the ordinary lane does not. Each live test
# runs in its own process of one test binary, CCE_LIVE_PARALLEL at once, and prints its start and
# duration as it ends (V1-2908, internal/changee2e/cce-live.sh).
CCE_LIVE_PARALLEL ?= 3
.PHONY: cce-live
cce-live:
	CORVINT_CCE_LIVE=required GOCACHE=$(CORVINT_GOCACHE) GOTOOLCHAIN=local sh internal/changee2e/cce-live.sh $(CCE_LIVE_PARALLEL) $(GO_TEST_TIMEOUT)
