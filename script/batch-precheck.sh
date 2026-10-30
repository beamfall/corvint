#!/usr/bin/env bash
# script/batch-precheck.sh (ASD-V0-029, V1-2551; ASD-V0-032, V1-2895; ASD-V0-034..036, V1-2899,
# V1-2900): run, in a few minutes, every generated-artifact drift, catalog, enumeration and
# import-direction check that batch CI runs, so a batch integrator finds them before the push
# instead of after about 26 minutes of CI. `make batch-precheck` runs it; `make batch-push-check`
# verifies its receipt.
#
#   script/batch-precheck.sh                        repin, run every step, print a PASS/FAIL table,
#                                                   exit 1 on any FAIL, record the receipt on a pass
#   script/batch-precheck.sh --base REV             as above, but rerun each failed step on a detached
#                                                   worktree of REV and exit 1 only for failures REV
#                                                   does not already show
#   script/batch-precheck.sh --sync-check           only verify the step list still mirrors CI
#   script/batch-precheck.sh --list                 print the steps without running them
#   script/batch-precheck.sh --verify-receipt [REV] refuse unless a passing receipt names REV
#                                                   (default HEAD); run it before `git push` of a
#                                                   claude/batch-* branch
#
# The doc-gate steps are derived at run time from the `run:` commands of the `doc-gates` job of
# .github/workflows/ci.yml (read only), one step per make target and per `go run`, under that
# job's `env:`; the sync check fails on a doc-gates command that is neither, so the list cannot
# drift from CI (ASD-V0-036); a doc-gates step form it cannot classify (a flow mapping, a quoted or
# empty `run` key, a continued plain scalar, an alias, `shell:` or `working-directory:`) is such a
# command, so it fails closed. Before the steps, `script/batch-regen.sh --mechanical` applies the
# mechanical repins (host package version bumps, moved line citations, use-case receipts); a repin
# that changed a file fails the run, naming the files to commit. The other steps are
# capabilities-check and capability-catalog-check (go-product runs both as tests),
# host-adapter-node-test (the node half of host-adapter-test, which no PR CI job runs; ASD-V0-044,
# V1-3062), the `gofmt -l` sets of the go-static and go-interop jobs, and `batch-drift`: one `go test -count=1 -run` over the union of DRIFT_TESTS
# in every package at once, through script/heavy.sh, so one offload covers them (ASD-V0-034).
# DRIFT_TESTS is a maintained list, not a selection: `corvint test --dry-run` over a batch diff
# falls back to the whole package universe, and these tests read specs, catalogs and goldens no
# Go import names. The sync check fails when a prefix names no test, and when a test that reads
# CORVINT_UPDATE_GOLDEN is missing from DRIFT_TESTS or from `script/batch-regen.sh --golden-tests`.
#
# Drift and catalog selection (ASD-V0-050, ASD-V0-051; V1-3134, V1-3136): with a base (--base or
# BATCH_PRECHECK_BASE), batch-drift runs only the DRIFT_TESTS entries the batch diff can reach, and
# capability-catalog-check is skipped, with a printed reason, when none of its inputs changed. The
# diff is `git diff --name-only --no-renames BASE` plus untracked, unignored files; the plan is
# `corvint affected --base BASE` (BATCH_PRECHECK_PLANNER names the binary). An entry with no
# declared input runs when the plan selects its package; `@go` runs it when the plan selects its
# package and a Go input changed (a *.go or go.mod/go.sum/go.work file, or a path in a directory
# that holds a .go file at the base or in the checkout); `@root` (a root-level file), a `DIR/`
# prefix or a path glob (whose `*` stays within one path component) runs it whenever such a path
# changed. The catalog check is one such entry over the packages its Makefile recipe names, with
# CATALOG_INPUTS. Everything runs, with the reason printed, when there is no base, the base is not
# a commit, jq or go.mod's module is missing, an ignored file a go test can read is present, a
# changed path needs quoting, git or the planner fails, the plan is not a runnable
# affected-plan/1, or the plan carries an unknown other than the three repository-wide
# LANGUAGE_FRONTIER details every plan here carries. The skipped set is printed, and a run whose
# selection is empty passes the step without running go test. The make steps share batch-drift's
# GOCACHE, so the catalog check reuses its compiled test binaries.
#
# Steps run BATCH_PRECHECK_JOBS (default 4) at a time, longest first, except SERIAL_TARGETS
# (host-adapter-node-test), which run alone afterwards (ASD-V0-047, V1-3142). Each prints one line when it
# ends; then a table in step order with each step's seconds, the summed and wall seconds, and the
# last 40 output lines of each failure. A run that passes every step on a checkout with no tracked
# change, no untracked, unignored file and no ignored file a go test can read as package input
# before or after, and whose HEAD did not move, writes the receipt
# `$(git rev-parse --git-path batch-precheck-receipt)` naming HEAD (ASD-V0-035); any other run
# removes it.
#
# Ledger reuse (ASD-V0-042, V1-2985): each step runs through tools/gate-ledger
# (docs/specs/gate-ledger-v0.md GL-V0-051) as `NAME+SALT`, where NAME is the target of a `make T`
# step, go-format-check for a gofmt step and batch-precheck (the whole tree) otherwise, and SALT
# hashes the step's name, its command and every GO*, CGO_* and CORVINT_* environment variable. A
# step whose key has a recorded pass prints REUSE and counts as passed; a changed input, command,
# environment or tool identity misses and runs. Reuse is off, and every step runs, under
# CORVINT_GATE_LEDGER=off (the cold-run switch), when the index differs from the worktree (a step
# such as line-citations-check reads the index, which the ledger does not key), when an ignored
# file a go test can read is present, or when the ledger does not build; BATCH_PRECHECK_LEDGER
# names a prebuilt tools/gate-ledger binary. The --base rerun never goes through the ledger.
#
# With --base (or BATCH_PRECHECK_BASE), after the parallel run each failed step is rerun, one at a
# time, at REV; it is base-present only when it also fails at REV and every one of its normalized
# output lines (only volatile tokens erased: the checkout and temporary paths, a pid, and timings
# on go test result lines) also appears in REV's output; it then prints as WARN and does not fail the run (ASD-V0-032, V1-2895). Anything else, including a REV
# that cannot be checked out and a failed mechanical repin, stays an introduced FAIL. A run with a
# base-present WARN writes no receipt, since not every step passed.
#
# Not mirrored: the full go-product shards (-race, the whole suite), go vet and the Windows build
# and vet (`make windows-vet`), the protected .github helper self-tests, interop tests, and the
# docs-plan/artifact-integrity/cce-live/release jobs. They are not drift checks.
set -uo pipefail
export LC_ALL=C

root=${BATCH_PRECHECK_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)} || exit 2
cd "$root" || exit 2
workflow=${BATCH_PRECHECK_WORKFLOW:-.github/workflows/ci.yml}
heavy=${BATCH_PRECHECK_HEAVY-script/heavy.sh}
regen=${BATCH_PRECHECK_REGEN-script/batch-regen.sh}
jobs=${BATCH_PRECHECK_JOBS:-4}
gocache=${CORVINT_GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}

# Targets go-product runs as tests of ./tools/capabilities-check and ./cmd/corvint, and the node
# half of host-adapter-test, which no PR CI job runs (ASD-V0-044, V1-3062).
EXTRA_TARGETS=(capability-catalog-check capabilities-check host-adapter-node-test)
# What capability-catalog-check reads beyond Go source: the root guides (README.md, llms.txt,
# AGENTS.md) and top-level docs (CAPABILITIES.md, E2E-ADOPTION.md); its snapshots and ratchet sit
# in Go package directories (ASD-V0-051).
CATALOG_INPUTS=(@go @root 'docs/*.md')
planner=${BATCH_PRECHECK_PLANNER-corvint}
# Steps with wall-clock bounds (node process-lifecycle tests) run alone after the parallel steps
# (ASD-V0-047, V1-3142).
SERIAL_TARGETS=(host-adapter-node-test)
# "<dir>|<gofmt -l arguments>": go-static runs from the root, go-interop from interop/cem01-go.
GOFMT_SETS=(
  ".|cmd internal tools .github/cishards .github/testconfine"
  "interop/cem01-go|."
)
# "<package> <test-name prefix (ERE)>[ <input>...]": the golden, receipt, enumeration, build and
# import-direction tests that failed batch CI (V1-2551, V1-2899). capability-catalog-check covers
# the TestCAPV00xx catalog and command-tree tests, TestCAPV0019 and TestCAPV0047 among them. The
# optional inputs narrow the entry under a base (ASD-V0-050): the cmd/corvint tests read only
# their package's goldens and fixtures, the frozen-profile test also the core freeze spec;
# TestCAPV0005 reads the routing docs and shipped skills. An entry without inputs (the archive
# test, whose entry and byte bounds cover the whole tree, among them) runs whenever the plan
# selects its package.
if [[ -n ${BATCH_PRECHECK_DRIFT:-} ]]; then
  DRIFT_TESTS=()
  while IFS= read -r line; do [[ -n $line ]] && DRIFT_TESTS+=("$line"); done <<< "$BATCH_PRECHECK_DRIFT"
else
  DRIFT_TESTS=(
    "./cmd/corvint TestAFUV1024StrictAndCoverageBytesUnchanged @go"
    "./cmd/corvint TestAFUV1040SelectionCorpusReport @go"
    "./cmd/corvint TestCoreVerbsEmitTheFrozenProfiles @go docs/specs/core-compatibility-freeze-v1.md"
    "./cmd/corvint TestAffectedReceiptMembersAreClosedAndByteStable @go"
    "./cmd/corvint TestMCPV0100HelpToolMatchesTheCLI @go"
    "./cmd/corvint TestMCPV0105CompanionHelpMatchesTheCLI @go"
    "./tools/gate-ledger TestGoTestKeys"
    "./tools/capabilities-check TestCAPV0005RoutingPointersNameTheCatalog @go AGENTS.md README.md docs/README.md docs/AGENT-ROUTES.md integrations/ protocol/integrations/"
    "./internal/lrfrepo Test[A-Za-z0-9]*Specs?Enumerat"
    "./internal/tasks TestImportDirection @go"
    "./internal/tasks/store TestCALV0070_"
    "./internal/tasks/journal TestCALV0070_"
    "./internal/tasks/safeopen TestCALV0070_"
    "./internal/companionrelease TestCurrentTasksSourceArchiveBuildsOffline"
    "./internal/companionrelease TestCorvintBundledBinariesBuildTwiceIdentically @go"
  )
fi

# job_block NAME prints the lines of one top-level job of the workflow.
job_block() {
  awk -v job="  $1:" '$0 == job { on = 1; next } on && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ { exit } on' "$workflow"
}

contains() {
  local want=$1 x
  shift
  for x in "$@"; do [[ $x == "$want" ]] && return 0; done
  return 1
}

# doc_gate_commands prints each command line of every `run:` of the doc-gates job, in order: a
# one-line `run: CMD`, or each non-blank, non-comment line of a `run: |` block; a line ending in a
# backslash is joined to the next. Any other line that names a `run` key (a flow mapping such as
# `- {run: make x}`, a quoted key, an empty `run:`, a plain scalar continued on the next line), a
# merge key or alias, or a `shell:`/`working-directory:` key is printed after a tab, as a form it
# cannot classify, so doc_gates reports it unknown and the sync check fails closed (ASD-V0-036).
doc_gate_commands() {
  job_block doc-gates | awk -v cont=-1 '
    function emit(s) {
      sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s)
      if (pending != "") { s = pending " " s; pending = "" }
      if (s ~ /\\$/) { pending = substr(s, 1, length(s) - 1); return }
      if (s != "" && s !~ /^#/) print s
    }
    function unparsed(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); print "\t" s }
    block {
      if ($0 !~ /[^[:space:]]/) next
      match($0, /^[[:space:]]*/)
      if (RLENGTH > indent) { emit($0); next }
      block = 0
    }
    cont >= 0 {
      if ($0 ~ /[^[:space:]]/ && $0 !~ /^[[:space:]]*#/) {
        match($0, /^[[:space:]]*/)
        if (RLENGTH > cont) { unparsed($0); next }
        cont = -1
      }
    }
    /^[[:space:]]*#/ || /^[[:space:]]*(- )?name:/ { next }
    /^[[:space:]]*(- )?run:[[:space:]]*[|>][-+]?[[:space:]]*$/ { match($0, /^[[:space:]]*/); indent = RLENGTH; block = 1; next }
    /^[[:space:]]*(- )?run:[[:space:]]*[^[:space:]]/ {
      s = $0; match(s, /run:/); cont = RSTART - 1
      sub(/^[[:space:]]*(- )?run:[[:space:]]*/, "", s)
      if (s ~ /^".*"$/ || s ~ /^\047.*\047$/) s = substr(s, 2, length(s) - 2)
      emit(s); next
    }
    /(^|[[:space:]{,])["\047]?run["\047]?[[:space:]]*:/ || /<<[[:space:]]*:/ || /(:|^[[:space:]]*-)[[:space:]]*[*&][A-Za-z0-9_-]/ ||
      /^[[:space:]]*(- )?(shell|working-directory):/ { unparsed($0) }'
}

# doc_gates prints "make<TAB>TARGET", "gorun<TAB>PACKAGE" or "unknown<TAB>COMMAND" per doc-gates
# command; a make option other than -s/-k/--silent/--no-print-directory, a VAR=value, a target that
# is not a plain name and a package that is not a plain ./path are unknown, so no shell syntax from
# ci.yml ever reaches a step.
doc_gates() {
  local cmd words w targets ok
  while IFS= read -r cmd; do
    if [[ $cmd == $'\t'* ]]; then
      printf 'unknown\t%s\n' "${cmd#$'\t'}"
      continue
    fi
    read -r -a words <<< "$cmd"
    if [[ ${words[0]-} == make ]]; then
      targets=() ok=1
      for w in "${words[@]:1}"; do
        case $w in
          -s | -k | --silent | --no-print-directory) ;;
          -* | *=*) ok=0 ;;
          *) [[ $w =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] && targets+=("$w") || ok=0 ;;
        esac
      done
      if ((ok)) && ((${#targets[@]})); then
        for w in "${targets[@]}"; do printf 'make\t%s\n' "$w"; done
        continue
      fi
    elif [[ ${#words[@]} == 3 && ${words[0]} == go && ${words[1]} == run && ${words[2]} =~ ^\./[A-Za-z0-9_./-]*$ ]]; then
      printf 'gorun\t%s\n' "${words[2]}"
      continue
    fi
    printf 'unknown\t%s\n' "$cmd"
  done < <(doc_gate_commands)
}

# doc_gate_env prints the doc-gates job-level `env:` as KEY=VALUE lines for env(1).
doc_gate_env() {
  job_block doc-gates | awk '
    /^    env:[[:space:]]*$/ { on = 1; next }
    on && /^      [A-Za-z_][A-Za-z0-9_]*:/ {
      key = $1; sub(/:$/, "", key); value = $0
      sub(/^[^:]*:[[:space:]]*/, "", value); sub(/[[:space:]]+$/, "", value)
      if (value ~ /^".*"$/ || value ~ /^\047.*\047$/) value = substr(value, 2, length(value) - 2)
      printf "%s=%s\n", key, value; next
    }
    on { exit }'
}

# golden_tests prints "<package> <name>" for each test function under cmd, internal and tools whose
# body calls os.Getenv("CORVINT_UPDATE_GOLDEN") (a helper's call is not attributed to a test).
golden_tests() {
  local file
  git ls-files -- 'cmd/*_test.go' 'internal/*_test.go' 'tools/*_test.go' 2> /dev/null | while IFS= read -r file; do
    grep -q 'Getenv("CORVINT_UPDATE_GOLDEN")' "$file" || continue
    awk -v pkg="./${file%/*}" '/^func / { name = "" } /^func Test[A-Za-z0-9_]*\(/ { name = $2; sub(/\(.*/, "", name) } /Getenv\("CORVINT_UPDATE_GOLDEN"\)/ && name != "" { print pkg " " name }' "$file"
  done | sort -u
}

# drift_names PKG NAME succeeds when a DRIFT_TESTS prefix of PKG matches NAME.
drift_names() {
  local line re
  local pkg prefix
  for line in "${DRIFT_TESTS[@]}"; do
    read -r pkg prefix _ <<< "$line"
    [[ $pkg == "$1" ]] || continue
    re="^$prefix"
    [[ $2 =~ $re ]] && return 0
  done
  return 1
}

sync_check() {
  local bad=0 kind value job dir args line pkg prefix inputs found goldens= decl=()
  [[ -f $workflow ]] || { printf 'batch-precheck: workflow %s not found\n' "$workflow" >&2; return 2; }
  [[ -n $(job_block doc-gates) ]] || { printf 'batch-precheck: no doc-gates job in %s\n' "$workflow" >&2; return 1; }
  [[ -n $(doc_gates) ]] || { printf 'batch-precheck: the doc-gates job in %s has no run command\n' "$workflow" >&2; return 1; }
  while IFS=$'\t' read -r kind value; do
    [[ $kind == unknown ]] || continue
    printf 'batch-precheck: %s doc-gates runs `%s`, which batch-precheck cannot mirror (only make targets and go run)\n' "$workflow" "$value" >&2
    bad=1
  done < <(doc_gates)
  # Every gofmt -l of go-static (root) and go-interop (interop/cem01-go).
  for job in go-static go-interop; do
    while read -r args; do
      dir=.
      [[ $job == go-interop ]] && dir=interop/cem01-go
      contains "$dir|$args" "${GOFMT_SETS[@]}" || {
        printf 'batch-precheck: %s %s runs gofmt -l %s in %s, which batch-precheck does not\n' "$workflow" "$job" "$args" "$dir" >&2
        bad=1
      }
    done < <(job_block "$job" | grep -oE 'gofmt -l [^)"]+' | sed -E 's/^gofmt -l //; s/[[:space:]]+$//')
  done
  for line in "${DRIFT_TESTS[@]}"; do
    read -r pkg prefix inputs <<< "$line"
    found=$(grep -lE "^func ${prefix}[A-Za-z0-9_]*\(t \*testing\.T\)" "$pkg"/*_test.go 2> /dev/null | head -n 1)
    [[ -n $found ]] || {
      printf 'batch-precheck: drift test prefix %s names no test in %s\n' "$prefix" "$pkg" >&2
      bad=1
    }
    # A declared input that names no tracked path would narrow the entry to nothing (ASD-V0-050).
    read -ra decl <<< "$inputs"
    for found in ${decl[@]+"${decl[@]}"}; do
      case $found in @go | @root) continue ;; @*) ;; *) [[ -n $(git ls-files -- "$found" 2> /dev/null | head -n 1) ]] && continue ;; esac
      printf 'batch-precheck: drift test %s %s declares input %s, which is not @go, @root or a tracked path\n' "$pkg" "$prefix" "$found" >&2
      bad=1
    done
  done
  if [[ -n $regen ]]; then
    goldens=$("$regen" --golden-tests) || { printf 'batch-precheck: %s --golden-tests failed\n' "$regen" >&2; return 1; }
  fi
  while read -r pkg found; do
    [[ -n $pkg ]] || continue
    drift_names "$pkg" "$found" || {
      printf 'batch-precheck: %s %s reads CORVINT_UPDATE_GOLDEN but is not in DRIFT_TESTS\n' "$pkg" "$found" >&2
      bad=1
    }
    [[ -z $regen ]] || grep -qxF -- "$pkg $found" <<< "$goldens" || {
      printf 'batch-precheck: %s %s reads CORVINT_UPDATE_GOLDEN but %s --golden-tests does not name it\n' "$pkg" "$found" "$regen" >&2
      bad=1
    }
  done < <(golden_tests)
  return "$bad"
}

# Selection state (ASD-V0-050, ASD-V0-051): sel_on=1 once select_plan narrowed the run; otherwise
# sel_why says why everything runs. drift_on[i] is 1 for each DRIFT_TESTS entry that runs and
# cat_skip, when set, is why capability-catalog-check is skipped.
sel_on=0 sel_why='no --base or BATCH_PRECHECK_BASE' sel_sha='' sel_planned='' sel_changed='' sel_go='' why=''
drift_on=() cat_skip=''
# The repository-wide frontier unknowns every plan of this checkout carries; any other unknown
# runs everything.
FRONTIER_OK='["go:build-constraint-variants","go:nested-module-frontier","shell:foreign-invocation-unmodelled"]'

# select_plan reads the batch diff and the affected plan into sel_changed, sel_go and sel_planned
# and sets sel_on=1, or leaves sel_on=0 with the reason in sel_why. It never fails the run.
select_plan() {
  local plan module reasons untracked
  [[ -n $base ]] || return 0
  sel_sha=$(git rev-parse --verify -q "$base^{commit}") || { sel_why="base $base is not a commit"; return 0; }
  [[ -n $planner ]] || { sel_why='BATCH_PRECHECK_PLANNER is empty'; return 0; }
  command -v jq > /dev/null 2>&1 || { sel_why='jq is not installed'; return 0; }
  module=$(awk '$1 == "module" { print $2; exit }' go.mod 2> /dev/null)
  # The module path may be quoted, as the go command and the planner accept: strip the quotes.
  [[ $module == \"*\" || $module == \`*\` ]] && module=${module:1:${#module}-2}
  [[ -n $module ]] || { sel_why='go.mod names no module'; return 0; }
  [[ $module != *[\"\`\\[:space:]]* ]] || { sel_why="the go.mod module path $module cannot be matched"; return 0; }
  [[ -z $(ignored_inputs) ]] || { sel_why='an ignored file a go test can read is present'; return 0; }
  sel_changed=$(git -c core.quotePath=true diff --name-only --no-renames "$sel_sha" --) || { sel_why="git diff against $sel_sha failed"; return 0; }
  untracked=$(git -c core.quotePath=true ls-files -o --exclude-standard) || { sel_why='git ls-files failed'; return 0; }
  sel_changed=$(printf '%s\n%s\n' "$sel_changed" "$untracked" | sed '/^$/d' | sort -u)
  # A quoted name (a control character, quote, backslash or non-ASCII byte) cannot be matched.
  ! grep -q '^"' <<< "$sel_changed" || { sel_why='a changed path needs quoting'; return 0; }
  { git ls-files -- '*.go' && git ls-tree -r --name-only "$sel_sha"; } > "$dir/select-go" 2> /dev/null || { sel_why='git could not list the Go files'; return 0; }
  printf '%s\n' "$sel_changed" > "$dir/select-changed"
  sel_go=$(awk 'NR == FNR { if ($0 ~ /\.go$/) { d = $0; if (sub(/\/[^\/]*$/, "", d)) dirs[d] = 1 } next }
    $0 == "" { next }
    { n = split($0, c, "/"); b = c[n]
      if ($0 ~ /\.go$/ || b == "go.mod" || b == "go.sum" || b == "go.work" || b == "go.work.sum") { print; next }
      d = $0; while (sub(/\/[^\/]*$/, "", d)) if (d in dirs) { print; next } }' "$dir/select-go" "$dir/select-changed") || { sel_why='the Go input match failed'; return 0; }
  plan=$("$planner" affected --base "$sel_sha" 2> "$dir/planner.err" < /dev/null) || { sel_why="$planner affected exited $? ($(head -c 200 "$dir/planner.err" | tr '\n' ' '))"; return 0; }
  jq -e '.ok == true and .profile == "affected-plan/1" and .provider.go.state == "RUNNABLE" and (.plan.selected | type) == "array" and (.plan.unknown | type) == "array"' <<< "$plan" > /dev/null 2>&1 ||
    { sel_why="the plan is not a runnable affected-plan/1: $(jq -r '.summary.line // .code // "unparsable"' <<< "$plan" 2> /dev/null | head -n 1)"; return 0; }
  reasons=$(jq -r --argjson ok "$FRONTIER_OK" '[.plan.unknown[] | select(.reason != "LANGUAGE_FRONTIER" or ([.detail] - $ok | length) > 0) | "\(.reason) \(.detail)"] | unique | join(", ")' <<< "$plan") ||
    { sel_why='the plan unknowns are unreadable'; return 0; }
  [[ -z $reasons ]] || { sel_why="the plan has unknowns: $reasons"; return 0; }
  # The plan must be bound to this base, and every selected entry must name a unit; a Go unit that
  # does not map into this module could be a package the batch breaks, so it runs everything.
  jq -e --arg b "$sel_sha" '.range.base == $b' <<< "$plan" > /dev/null 2>&1 ||
    { sel_why="the plan is bound to base $(jq -r '.range.base // "none" | tostring' <<< "$plan" 2> /dev/null | head -c 80), not $sel_sha"; return 0; }
  reasons=$(jq -r --arg m "go:$module" '[.plan.selected[] | if type != "object" or (.unitId | type) != "string" then "an entry without a string unitId"
      elif (.unitId | startswith("go:")) and .unitId != $m and (.unitId | startswith($m + "/") | not) then "go unit \(.unitId) is outside module \($m | ltrimstr("go:"))"
      else empty end] | unique | .[:3] | join(", ")' <<< "$plan") || { sel_why='the plan selection is unreadable'; return 0; }
  [[ -z $reasons ]] || { sel_why="the plan selection cannot be mapped: $reasons"; return 0; }
  sel_planned=$(jq -r --arg m "go:$module" '.plan.selected[].unitId | select(startswith("go:")) | if . == $m then "." else "./" + ltrimstr($m + "/") end' <<< "$plan") ||
    { sel_why='the plan selection is unreadable'; return 0; }
  [[ -z $sel_go || -n $sel_planned ]] || { sel_why='a Go input changed but the plan selects no Go package'; return 0; }
  sel_on=1
}

# wanted "PKG..." [INPUT...] succeeds when the entry must run under the selection; otherwise it
# sets why and fails. An explicit input that changed runs it regardless of the plan.
wanted() {
  local pkgs=$1 tok p planned=0 go=0 x
  shift
  for x in $pkgs; do grep -qxF -- "$x" <<< "$sel_planned" && planned=1; done
  for tok in "$@"; do
    # shellcheck disable=SC2053 # a glob token matches on purpose; the slash count keeps * in one level.
    case $tok in
      @go) go=1 ;;
      @root) grep -q '^[^/][^/]*$' <<< "$sel_changed" && return 0 ;;
      */) while IFS= read -r p; do [[ $p == "$tok"* ]] && return 0; done <<< "$sel_changed" ;;
      *) while IFS= read -r p; do [[ $p == $tok && ${p//[^\/]/} == "${tok//[^\/]/}" ]] && return 0; done <<< "$sel_changed" ;;
    esac
  done
  if (($# == 0)); then
    ((planned)) && return 0
    why='the plan does not select its package'
  elif ((go)); then
    ((planned)) && [[ -n $sel_go ]] && return 0
    if ((planned)); then why='no Go input and no declared input changed'; else why='no declared input changed and the plan does not select its package'; fi
  else
    why='no declared input changed'
  fi
  return 1
}

# select_report decides drift_on and cat_skip from the selection and prints the outcome.
select_report() {
  local i n=${#DRIFT_TESTS[@]} k=0 pkg prefix inputs decl=() skipped=() pkgs
  drift_on=()
  for ((i = 0; i < n; i++)); do
    drift_on[i]=1
    ((sel_on)) || continue
    read -r pkg prefix inputs <<< "${DRIFT_TESTS[i]}"
    read -ra decl <<< "$inputs"
    wanted "$pkg" ${decl[@]+"${decl[@]}"} && continue
    drift_on[i]=0
    skipped+=("$pkg $prefix ($why)")
  done
  if ((!sel_on)); then
    printf 'batch-precheck: batch-drift runs every drift test and capability-catalog-check runs (%s)\n' "$sel_why"
    return 0
  fi
  for ((i = 0; i < n; i++)); do ((drift_on[i])) && k=$((k + 1)); done
  printf 'batch-precheck: batch-drift runs %d of %d drift tests for the diff since %s (%d changed paths)\n' "$k" "$n" "$sel_sha" "$(grep -c . <<< "$sel_changed")"
  for i in ${skipped[@]+"${!skipped[@]}"}; do printf 'batch-precheck:   skipped %s\n' "${skipped[i]}"; done
  pkgs=$(awk '/^capability-catalog-check:/ { getline; print; exit }' Makefile 2> /dev/null | grep -oE '\./[^[:space:]]+' | tr '\n' ' ')
  if [[ -z $pkgs ]]; then
    printf 'batch-precheck: capability-catalog-check runs (the Makefile recipe names no package)\n'
  elif wanted "$pkgs" "${CATALOG_INPUTS[@]}"; then
    printf 'batch-precheck: capability-catalog-check runs (a catalog input changed since %s)\n' "$sel_sha"
  else
    cat_skip="no catalog input changed since $sel_sha: $why"
    printf 'batch-precheck: capability-catalog-check skipped: %s\n' "$cat_skip"
  fi
}

# Steps as "name<TAB>command", longest first; each command runs under bash -c from the root.
steps() {
  local kind value t s dir args line pkg prefix pkgs=() re='' envs=() i n=${#DRIFT_TESTS[@]} k=0
  for ((i = 0; i < n; i++)); do
    [[ ${drift_on[i]:-1} == 1 ]] || continue
    k=$((k + 1))
    read -r pkg prefix _ <<< "${DRIFT_TESTS[i]}"
    contains "$pkg" "${pkgs[@]+"${pkgs[@]}"}" || pkgs+=("$pkg")
    re+="${re:+|}$prefix"
  done
  # One go test over every package; a package that runs no test fails the step, since its prefix
  # would otherwise pass silently.
  # BATCH_PRECHECK_HEAVY is one executable path, quoted as one word: no expansion, no splitting.
  local heavyq= label
  [[ -z $heavy ]] || heavyq=$(printf '%q' "$heavy")
  label="${#pkgs[@]} packages"
  ((k == n)) || label+=", $k of $n tests"
  if ((k == 0)); then
    # Every entry was skipped under the selection (ASD-V0-050); no go test runs.
    printf 'batch-drift (0 of %d tests)\tprintf "%%s\\n" %q\n' "$n" "batch-drift: no drift test is reachable from the batch diff; every one is skipped"
  else
    printf 'batch-drift (%s)\tout=$(%s env GOCACHE=%q GOTOOLCHAIN=local go test -count=1 -run %q %s 2>&1); rc=$?; printf "%%s\\n" "$out"; [ "$rc" = 0 ] || exit "$rc"; case $out in *"no tests to run"*|*"no test files"*) exit 1 ;; esac\n' \
      "$label" "$heavyq" "$gocache" "^($re)" "${pkgs[*]}"
  fi
  # The make steps share batch-drift's GOCACHE (ASD-V0-051); a skipped catalog check prints why.
  for t in "${EXTRA_TARGETS[@]}"; do
    if [[ $t == capability-catalog-check && -n $cat_skip ]]; then
      printf 'make %s\tprintf "%%s\\n" %q\n' "$t" "skipped: $cat_skip"
    else
      printf 'make %s\tenv CORVINT_GOCACHE=%q make -s %s\n' "$t" "$gocache" "$t"
    fi
  done
  while IFS= read -r line; do envs+=("$(printf '%q' "$line")"); done < <(doc_gate_env)
  while IFS=$'\t' read -r kind value; do
    case $kind in
      make) printf 'make %s\tenv %s make -s %q\n' "$value" "${envs[*]-}" "$value" ;;
      gorun) printf 'go run %s\tenv %s GOCACHE=%q go run %q\n' "$value" "${envs[*]-}" "$gocache" "$value" ;;
    esac
  done < <(doc_gates)
  for s in "${GOFMT_SETS[@]}"; do
    dir=${s%%|*}
    args=${s#*|}
    printf 'gofmt -l %s (%s)\tout=$(cd %q && gofmt -l %s) || exit 1; [ -z "$out" ] || { printf "%%s\\n" "$out"; exit 1; }\n' "$args" "$dir" "$dir" "$args"
  done
  printf 'ai-coauthor-trailers (%s)\t%q --batch %q\n' "$(git rev-parse --short HEAD)" "${0%/*}/check-ai-coauthor-trailers.sh" "$base" # ASD-V0-560, V1-3227
  printf 'sync-check\t%q --sync-check\n' "$0"
}

base=${BATCH_PRECHECK_BASE:-}
receipt_path() { git rev-parse --path-format=absolute --git-path batch-precheck-receipt; }

verify_receipt() {
  local want head receipt
  receipt=$(receipt_path) || return 2
  want=$(git rev-parse --verify --quiet "${1:-HEAD}^{commit}") || { printf 'batch-precheck: %s is not a commit\n' "${1:-HEAD}" >&2; return 2; }
  [[ -f $receipt ]] || { printf 'batch-precheck: refused: no precheck receipt; run make batch-precheck at %s first\n' "$want" >&2; return 1; }
  # The whole receipt must be well formed: five lines in order, every step passed.
  awk 'NR == 1 && $0 != "corvint-batch-precheck-receipt/0" { bad = 1 }
    NR == 2 && !($1 == "head" && $2 ~ /^[0-9a-f]+$/ && NF == 2) { bad = 1 }
    NR == 3 && !($1 == "steps" && $2 ~ /^[1-9][0-9]*$/ && NF == 2) { bad = 1 } NR == 3 { steps = $2 }
    NR == 4 && !($1 == "passed" && $2 == steps && NF == 2) { bad = 1 }
    NR == 5 && !($1 == "finished" && NF == 2) { bad = 1 }
    END { exit (bad || NR != 5) }' "$receipt" || { printf 'batch-precheck: refused: %s is not a well-formed passing precheck receipt; rerun make batch-precheck\n' "$receipt" >&2; return 1; }
  head=$(awk '$1 == "head" { print $2 }' "$receipt")
  [[ $head == "$want" ]] || { printf 'batch-precheck: refused: the precheck receipt names %s, not %s; rerun make batch-precheck\n' "${head:-nothing}" "$want" >&2; return 1; }
  printf 'batch-precheck: receipt matches %s (%s)\n' "$want" "$(awk '$1 == "finished" { print $2 }' "$receipt")"
}

tracked_clean() { [[ -z $(git status --porcelain --untracked-files=no 2> /dev/null) ]]; }
# ignored_inputs prints each gitignored path a go test can read as package input, so the checked
# tree is not HEAD (ASD-V0-035): a Go source or module file, a path under a testdata directory, and
# any ignored entry directly inside a directory that holds a tracked .go file. Only a path inside a
# directory the go command skips (a directory component starting with . or _) and not under a
# testdata directory is dropped, since no package there runs a test; a dot or underscore name
# under testdata or directly in a package directory (pkg/testdata/_cases/x.json, pkg/.fixture)
# is still test input.
ignored_inputs() {
  {
    git ls-files -o -i --exclude-standard -- '*.go' '*go.mod' '*go.sum' '*go.work' '*go.work.sum' '*testdata/*' || echo '<git ls-files failed>'
    git ls-files -- '*.go' | awk '{ d = $0; if (sub(/\/[^\/]*$/, "", d) == 0) d = "."; print "T\t" d }'
    git ls-files -o -i --exclude-standard --directory | awk '{ print "I\t" $0 }'
  } | awk -F '\t' '
    # skipped reports whether directory d lies in a directory the go command skips, judged only
    # above the first testdata component.
    function skipped(d,    n, c, i) {
      if (d == ".") return 0
      n = split(d, c, "/")
      for (i = 1; i <= n; i++) {
        if (c[i] == "testdata") return 0
        if (c[i] ~ /^[._]/) return 1
      }
      return 0
    }
    function parent(p,    d) { d = p; sub(/\/$/, "", d); if (sub(/\/[^\/]*$/, "", d) == 0) d = "."; return d }
    $1 == "T" && NF == 2 { dirs[$2] = 1; next }
    $1 == "I" && NF == 2 { d = parent($2); if ((d in dirs) && !skipped(d)) print $2; next }
    !skipped(parent($0)) { print }' | sort -u
}
# tree_clean also refuses untracked, unignored files and ignored package inputs, which a go test or
# generator may read.
tree_clean() { local out; out=$(git status --porcelain 2> /dev/null) && [[ -z $out && -z $(ignored_inputs) ]]; }

case ${1:-} in
  --sync-check) sync_check; exit $? ;;
  --list) steps | cut -f1; exit 0 ;;
  --verify-receipt) verify_receipt "${2:-}"; exit $? ;;
  --base) [[ $# == 2 && -n $2 ]] || { printf 'usage: script/batch-precheck.sh --base REV\n' >&2; exit 2; }; base=$2 ;;
  '') ;;
  *) printf 'usage: script/batch-precheck.sh [--base REV|--sync-check|--list|--verify-receipt [REV]]\n' >&2; exit 2 ;;
esac

# normalize DIR < output: one finding per line, with only volatile tokens erased, so the same
# finding compares equal at the base and the head: the checkout path (as given, as the shell prints
# it, and with symlinks resolved), a temporary-directory path, a `pid N`, the heavy.sh banner that
# names the tree it ran on, and a timing only on a go test result line (`ok`, `FAIL`, `--- FAIL:`
# and the like). A number inside a diagnostic, such as `timeout limit 2s exceeded`, stays.
normalize() {
  PRECHECK_DIRS="$1"$'\n'"$(cd "$1" && pwd)"$'\n'"$(cd "$1" && pwd -P)" PRECHECK_TMP="${TMPDIR:-/tmp}" awk 'BEGIN {
    n = split(ENVIRON["PRECHECK_DIRS"], d, "\n")
    t = ENVIRON["PRECHECK_TMP"]; sub(/\/+$/, "", t)
    np = split(t "\n/private/tmp\n/tmp\n/private/var/folders\n/var/folders", tp, "\n")
  }
  # tmpdirs replaces each path that starts, at a token start, with a temporary-directory prefix.
  function tmpdirs(s,    k, out, i, rest, c) {
    for (k = 1; k <= np; k++) {
      if (tp[k] == "") continue
      out = ""
      while ((i = index(s, tp[k] "/")) > 0) {
        c = i > 1 ? substr(s, i - 1, 1) : " "
        rest = substr(s, i + length(tp[k]))
        if (c !~ /[[:space:]"\047=(:]/) { out = out substr(s, 1, i + length(tp[k]) - 1); s = rest; continue }
        match(rest, /^[^[:space:]:"\047)]*/)
        out = out substr(s, 1, i - 1) "<tmp>"; s = substr(rest, RLENGTH + 1)
      }
      s = out s
    }
    return s
  }
  /^heavy\.sh: running / { next }
  /^gate-ledger: / { next }
  {
    for (k = 1; k <= n; k++)
      while (d[k] != "" && (i = index($0, d[k])) > 0) $0 = substr($0, 1, i - 1) "<root>" substr($0, i + length(d[k]))
    $0 = tmpdirs($0)
    # A process id: pid, PID or pid= followed by digits, at a word start.
    line = $0; out = ""
    while (match(line, /(^|[^A-Za-z0-9_])[Pp][Ii][Dd][ =:]?[0-9]+/)) {
      out = out substr(line, 1, RSTART - 1) (substr(line, RSTART, 1) ~ /[Pp]/ ? "" : substr(line, RSTART, 1)) "pid <n>"
      line = substr(line, RSTART + RLENGTH)
    }
    line = out line
    # A timing is a whole token: digits[.digits][m]s between line ends, blanks or parentheses, so
    # a name such as v1schema.go keeps its digits; it is erased only on a go test result line.
    if (line ~ /^(ok|FAIL|PASS|---[[:space:]]+(FAIL|PASS|SKIP):)([[:space:]]|$)/) {
      out = ""
      while (match(line, /[0-9]+(\.[0-9]+)?m?s/)) {
        pre = substr(line, 1, RSTART - 1); post = substr(line, RSTART + RLENGTH)
        if ((pre == "" || substr(pre, length(pre)) ~ /[[:space:](]/) && (post == "" || substr(post, 1, 1) ~ /[[:space:])]/))
          out = out pre "<t>"
        else
          out = out pre substr(line, RSTART, RLENGTH)
        line = post
      }
      line = out line
    }
    sub(/[[:space:]]+$/, "", line)
    if (line != "") print line
  }' | sort -u
}

basewt= baseerr=
# base_worktree creates, once, a detached worktree of $base; on failure it records why.
base_worktree() {
  [[ -n $basewt ]] && return 0
  [[ -n $baseerr ]] && return 1
  local sha dir
  if ! sha=$(git rev-parse --verify -q "$base^{commit}"); then
    baseerr="base $base is not a commit"
    return 1
  fi
  dir=$(mktemp -d "${TMPDIR:-/tmp}/batch-precheck-base.XXXXXX") || { baseerr="mktemp failed"; return 1; }
  if ! git worktree add -q --detach "$dir/wt" "$sha" > "$dir/err" 2>&1; then
    baseerr="git worktree add $sha failed: $(head -n 3 "$dir/err" | tr '\n' ' ')"
    /bin/rm -rf "$dir"
    return 1
  fi
  basewt=$dir/wt
}

# at_base CMD HEADLOG: 0 when CMD also fails at the base and the head shows no finding the base
# lacks. Any error while comparing returns 1, so the failure stays introduced.
at_base() {
  local cmd=$1 headlog=$2 baselog
  base_worktree || return 1
  baselog=$basewt.log
  # Git location variables would point the base run back at the head checkout.
  (unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX GIT_OBJECT_DIRECTORY
    cd "$basewt" && BATCH_PRECHECK_ROOT=$basewt bash -c "$cmd") > "$baselog" 2>&1 < /dev/null && return 1
  normalize "$root" < "$headlog" > "$baselog.head" || return 1
  normalize "$basewt" < "$baselog" > "$baselog.base" || return 1
  [[ -s $baselog.head ]] || return 1
  comm -23 "$baselog.head" "$baselog.base" > "$baselog.new" || return 1
  [[ ! -s $baselog.new ]]
}

dir=
cleanup() {
  [[ -z $dir ]] || /bin/rm -rf "$dir"
  if [[ -n $basewt ]]; then
    git worktree remove --force "$basewt" > /dev/null 2>&1
    /bin/rm -rf "$(dirname "$basewt")"
    git worktree prune > /dev/null 2>&1
    basewt=
  fi
}


receipt=$(receipt_path) || exit 2
/bin/rm -f "$receipt"
# Step scratch stays off a RAM-backed TMPDIR (ASD-V0-055, V1-3205, V1-3206): on a Linux host /tmp
# is often a tmpfs, and batch-drift's cold double builds alone can fill it while the parallel steps
# compile there, so a step fails with no space left on device and passes on its rerun.
# BATCH_PRECHECK_TMPDIR, when set, is the TMPDIR of the run and of every step; otherwise a TMPDIR
# (default /tmp) that is a tmpfs or ramfs is replaced by ${XDG_CACHE_HOME:-$HOME/.cache}/corvint-tmp,
# with one printed line. The directory is created mode 0700 when missing.
tmpfs_dir() { [[ $(uname -s) == Linux ]] && case $(stat -f -c %T "$1" 2> /dev/null) in tmpfs | ramfs) true ;; *) false ;; esac; }
# Entries of that default directory older than a day, left by killed steps, are removed first,
# since nothing resets it as a reboot resets a tmpfs. With neither XDG_CACHE_HOME nor HOME set, the
# tmpfs TMPDIR is kept. A relative BATCH_PRECHECK_TMPDIR is made absolute, since steps change directory.
step_tmp=${BATCH_PRECHECK_TMPDIR:-} cache_home=${XDG_CACHE_HOME:-${HOME:+$HOME/.cache}}
if [[ -z $step_tmp && -n $cache_home ]] && tmpfs_dir "${TMPDIR:-/tmp}"; then
  step_tmp=$cache_home/corvint-tmp
  printf 'batch-precheck: TMPDIR %s is RAM-backed; steps use %s (BATCH_PRECHECK_TMPDIR chooses another)\n' "${TMPDIR:-/tmp}" "$step_tmp"
  [[ -d $step_tmp ]] && find "$step_tmp" -mindepth 1 -maxdepth 1 -mmin +1440 -exec /bin/rm -rf {} + 2> /dev/null
fi
if [[ -n $step_tmp ]]; then
  (umask 077 && mkdir -p "$step_tmp") && step_tmp=$(cd "$step_tmp" && pwd) ||
    { printf 'batch-precheck: cannot create TMPDIR %s\n' "$step_tmp" >&2; exit 2; }
  export TMPDIR=$step_tmp
fi
# A TMPDIR with less than BATCH_PRECHECK_MIN_TMP_KIB (4 GiB) free warns before any step runs:
# compiler, test and sort spills there fail with no space left on device (ASD-V0-054).
tmp_kib=$(df -Pk "${TMPDIR:-/tmp}" 2> /dev/null | awk 'NR == 2 { print $4 }')
if [[ $tmp_kib =~ ^[0-9]+$ ]] && ((tmp_kib < ${BATCH_PRECHECK_MIN_TMP_KIB:-4194304})); then
  printf 'batch-precheck: WARN low-tmp-space avail=%sKiB under %s; steps may fail with no space left on device: set TMPDIR to a directory with room\n' "$tmp_kib" "${TMPDIR:-/tmp}"
fi
dir=$(mktemp -d "${TMPDIR:-/tmp}/batch-precheck.XXXXXX") || exit 2
trap cleanup EXIT
trap 'cleanup; exit 129' HUP
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM
start=$SECONDS
head_before=$(git rev-parse HEAD)
clean_before=0 pristine_before=0
tracked_clean && clean_before=1
tree_clean && pristine_before=1

# The mechanical repins run alone, before any check reads the files they write.
repin_rc=0
if [[ -n $regen ]]; then
  t0=$SECONDS
  "$regen" --mechanical > "$dir/repin.log" 2>&1 < /dev/null || repin_rc=1
  if ((clean_before)) && ! tracked_clean; then
    { printf 'mechanical repins changed tracked files; commit them and rerun:\n'; git status --porcelain --untracked-files=no; } >> "$dir/repin.log"
    repin_rc=1
  fi
  printf '%s %s\n' "$repin_rc" $((SECONDS - t0)) > "$dir/repin.rc"
  printf 'batch-precheck: %s repin (mechanical) (%ss)\n' "$( ((repin_rc)) && echo FAIL || echo PASS)" $((SECONDS - t0))
fi

# Ledger reuse is decided once, after the repins wrote what they write (ASD-V0-042).
ledger='' ledger_off=''
if [[ ${CORVINT_GATE_LEDGER:-} == off ]]; then
  ledger_off='CORVINT_GATE_LEDGER=off'
elif ! git diff --quiet > /dev/null 2>&1; then
  ledger_off='the index differs from the worktree'
elif [[ -n $(ignored_inputs) ]]; then
  ledger_off='an ignored file a go test can read is present'
elif [[ -n ${BATCH_PRECHECK_LEDGER:-} ]]; then
  ledger=$BATCH_PRECHECK_LEDGER
elif GOTOOLCHAIN=local GOCACHE=$gocache go build -o "$dir/gate-ledger" ./tools/gate-ledger > "$dir/ledger-build.log" 2>&1 < /dev/null; then
  ledger=$dir/gate-ledger
else
  ledger_off='tools/gate-ledger did not build'
fi
if [[ -n $ledger ]]; then
  printf 'batch-precheck: ledger reuse on (%s)\n' "$ledger"
else
  printf 'batch-precheck: ledger reuse off (%s); every step runs\n' "$ledger_off"
fi

# Drift and catalog selection against the base (ASD-V0-050, ASD-V0-051).
select_plan
select_report
steps > "$dir/steps"
n=$(wc -l < "$dir/steps" | tr -d ' ')
# run_one N runs step N, through the ledger when reuse is on; its rc file holds "RC SECS" and, for
# a reused pass, a third word `reuse`.
run_one() {
  local line name cmd rc lname salt reused='' t0=$SECONDS log=$BATCH_PRECHECK_DIR/$1.log
  line=$(sed -n "$1p" "$BATCH_PRECHECK_DIR/steps")
  name=${line%%$'\t'*}
  cmd=${line#*$'\t'}
  if [[ -n ${BATCH_PRECHECK_LEDGER_BIN:-} ]]; then
    case $name in
      'make '*) lname=${name#make } ;;
      'gofmt '*) lname=go-format-check ;;
      *) lname=batch-precheck ;;
    esac
    salt=$({ printf '%s\n%s\n' "$name" "$cmd"; env | grep -E '^(GO[A-Z0-9_]*|CGO_[A-Z0-9_]*|CORVINT_[A-Z0-9_]*)=' | sort; } | git hash-object --stdin) || salt=
    if [[ -n $salt ]]; then
      "$BATCH_PRECHECK_LEDGER_BIN" run "$lname+$salt" -- bash -c "$cmd" > "$log" 2>&1 < /dev/null
      rc=$?
      [[ $rc == 0 ]] && grep -q '^gate-ledger: HIT ' "$log" && reused=reuse
    else
      bash -c "$cmd" > "$log" 2>&1 < /dev/null
      rc=$?
    fi
  else
    bash -c "$cmd" > "$log" 2>&1 < /dev/null
    rc=$?
  fi
  printf '%s %s %s\n' "$rc" $((SECONDS - t0)) "$reused" > "$BATCH_PRECHECK_DIR/$1.rc"
  if [[ -n $reused ]]; then
    printf 'batch-precheck: REUSE %s (%ss): %s\n' "$name" $((SECONDS - t0)) "$(grep -m1 '^gate-ledger: HIT ' "$log" | sed 's/^gate-ledger: HIT [^ ]* //')"
  else
    printf 'batch-precheck: %s %s (%ss)\n' "$( ((rc)) && echo FAIL || echo PASS)" "$name" $((SECONDS - t0))
  fi
  return 0
}
export -f run_one
export BATCH_PRECHECK_DIR=$dir BATCH_PRECHECK_LEDGER_BIN=$ledger
# ASD-V0-047 (V1-3142): a SERIAL_TARGETS step runs alone after the parallel ones, so its timing
# bounds never compete with this run's own go test steps; its rerun (ASD-V0-046) is alone too.
parallel=() serial=()
for ((i = 1; i <= n; i++)); do
  name=$(sed -n "${i}p" "$dir/steps" | cut -f1)
  if contains "${name#make }" "${SERIAL_TARGETS[@]+"${SERIAL_TARGETS[@]}"}"; then serial+=("$i"); else parallel+=("$i"); fi
done
((${#parallel[@]})) && printf '%s\n' "${parallel[@]}" | xargs -P "$jobs" -n 1 bash -c 'run_one "$1"' _
for i in "${serial[@]+"${serial[@]}"}"; do run_one "$i"; done

# failing_tests LOG prints the go test and subtest names (`--- FAIL: NAME`) and node/TAP `not ok`
# names a step log reports, comma-separated, at most 20, or nothing.
failing_tests() {
  { grep -oE -e '--- FAIL: [^ ]+' "$1" | sed 's/^--- FAIL: //'
    grep -E '^not ok ' "$1" | sed -E 's/^not ok [0-9]+ (- )?//'; } 2> /dev/null |
    awk 'NF && !seen[$0]++' | head -n 20 | paste -sd , - | sed 's/,/, /g'
}

# host_resource LOG prints, trimmed, the first line of LOG naming an exhausted host resource: a full
# disk, a quota, exhausted memory or file descriptors. Such a failure says nothing about the change,
# so it is named host-resource, never introduced, and a FLAKY pass names it; the step still fails
# (ASD-V0-054).
host_resource() {
  grep -m 1 -i -E 'no space left on device|disk quota exceeded|cannot allocate memory|out of memory|too many open files' "$1" 2> /dev/null |
    sed -E 's/^[[:space:]|]+//' | cut -c 1-200
}

# ASD-V0-046 (V1-3122): each failed step reruns once, one at a time, before anything is classified.
# A pass on the rerun is FLAKY: counted passed, with both attempts named. A second failure keeps the
# rerun's log for the --base comparison and the report; attempt 1's log stays beside it.
flaky=0 flakes=()
for ((i = 1; i <= n; i++)); do
  read -r rc secs _ < "$dir/$i.rc" 2> /dev/null || { rc=1 secs=0; }
  [[ $rc == 0 ]] && continue
  line=$(sed -n "${i}p" "$dir/steps")
  name=${line%%$'\t'*}
  [[ -f $dir/$i.log ]] && /bin/mv -f "$dir/$i.log" "$dir/$i.first.log"
  first=$(failing_tests "$dir/$i.first.log")
  t0=$SECONDS
  bash -c "${line#*$'\t'}" > "$dir/$i.log" 2>&1 < /dev/null
  rerc=$?
  resecs=$((SECONDS - t0))
  if [[ $rerc == 0 ]]; then
    flaky=$((flaky + 1))
    flakes+=("$name")
    printf '0 %s flaky\n' $((secs + resecs)) > "$dir/$i.rc"
    hr=$(host_resource "$dir/$i.first.log")
    printf 'batch-precheck: FLAKY %s: attempt 1 failed rc=%s (%ss)%s%s; attempt 2 passed (%ss)\n' \
      "$name" "$rc" "$secs" "${first:+, failing: $first}" "${hr:+, host-resource: $hr}" "$resecs"
    # The first attempt's own lines name the cause a FLAKY pass would otherwise lose (ASD-V0-055).
    tail -n 20 "$dir/$i.first.log" 2> /dev/null | sed 's/^/  | attempt 1: /'
  else
    printf '%s %s\n' "$rerc" $((secs + resecs)) > "$dir/$i.rc"
    again=$(failing_tests "$dir/$i.log")
    hr=$(host_resource "$dir/$i.log")
    printf 'batch-precheck: FAIL %s: attempt 1 rc=%s (%ss)%s; attempt 2 rc=%s (%ss)%s%s\n' \
      "$name" "$rc" "$secs" "${first:+, failing: $first}" "$rerc" "$resecs" "${again:+, failing: $again}" "${hr:+, host-resource: $hr}"
  fi
done

# With --base, each failed step reruns at REV one at a time, since they share one base worktree; a
# base-present step leaves a .warn file beside its rc file.
if [[ -n $base ]]; then
  for ((i = 1; i <= n; i++)); do
    read -r rc secs < "$dir/$i.rc" 2> /dev/null || { rc=1 secs=0; }
    [[ $rc == 0 ]] && continue
    line=$(sed -n "${i}p" "$dir/steps")
    name=${line%%$'\t'*}
    # The base rerun would exhaust the same resource, so it is not run (ASD-V0-054).
    hr=$(host_resource "$dir/$i.log")
    if [[ -n $hr ]]; then
      printf 'batch-precheck: FAIL %s (%ss) host-resource: %s; not compared with %s: free disk or memory (TMPDIR=%s) and rerun\n' \
        "$name" "$secs" "$hr" "$base" "${TMPDIR:-/tmp}"
    elif at_base "${line#*$'\t'}" "$dir/$i.log"; then
      : > "$dir/$i.warn"
      printf 'batch-precheck: WARN %s (%ss) base-present: fails the same way at %s\n' "$name" "$secs" "$base"
      tail -n 40 "$dir/$i.log" | sed 's/^/  | /'
    else
      printf 'batch-precheck: FAIL %s (%ss) introduced: %s\n' "$name" "$secs" "${baseerr:-not present at $base}"
    fi
  done
fi

pass=0 fail=0 summed=0 failed=() warn=0 warned=() reused=0
# row NAME RCFILE prints one table row; a step that left no rc file failed. A reused ledger pass
# prints REUSE and counts as passed.
row() {
  local rc secs flag
  read -r rc secs flag < "$2" 2> /dev/null || { rc=1 secs=0 flag=; }
  summed=$((summed + secs))
  if [[ $rc == 0 && $flag == flaky ]]; then
    pass=$((pass + 1))
    printf 'batch-precheck: | FLAKY | %5ss | %s\n' "$secs" "$1"
  elif [[ $rc == 0 && $flag == reuse ]]; then
    pass=$((pass + 1))
    reused=$((reused + 1))
    printf 'batch-precheck: | REUSE | %5ss | %s\n' "$secs" "$1"
  elif [[ $rc == 0 ]]; then
    pass=$((pass + 1))
    printf 'batch-precheck: | PASS | %5ss | %s\n' "$secs" "$1"
  elif [[ -f ${2%.rc}.warn ]]; then
    warn=$((warn + 1))
    warned+=("$1")
    printf 'batch-precheck: | WARN | %5ss | %s\n' "$secs" "$1"
  else
    fail=$((fail + 1))
    failed+=("$1")
    printf 'batch-precheck: | FAIL | %5ss | %s\n' "$secs" "$1"
  fi
}
printf 'batch-precheck: | result | seconds | step\n'
[[ -f $dir/repin.rc ]] && row 'repin (mechanical)' "$dir/repin.rc"
for ((i = 1; i <= n; i++)); do row "$(sed -n "${i}p" "$dir/steps" | cut -f1)" "$dir/$i.rc"; done
printf 'batch-precheck: %d passed, %d failed in %ss wall, %ss summed\n' "$pass" "$fail" $((SECONDS - start)) "$summed"
((flaky)) && printf 'batch-precheck: %d step(s) FLAKY, passed only on their one rerun: %s\n' "$flaky" "$(IFS=,; printf '%s' "${flakes[*]}")"
((reused)) && printf 'batch-precheck: %d of %d steps reused a ledger pass on identical inputs (CORVINT_GATE_LEDGER=off reruns them)\n' "$reused" "$n"
if ((warn)); then
  printf 'batch-precheck: WARNING %d base-present failure(s) at %s, not counted: %s\n' "$warn" "$base" "$(IFS=,; printf '%s' "${warned[*]}")"
fi
if ((fail)); then
  if ((repin_rc)); then
    printf 'batch-precheck: FAIL repin (mechanical) output:\n'
    tail -n 40 "$dir/repin.log" | sed 's/^/  | /'
  fi
  for ((i = 1; i <= n; i++)); do
    read -r rc _ < "$dir/$i.rc" 2> /dev/null || rc=1
    [[ $rc == 0 || -f $dir/$i.warn ]] && continue
    printf 'batch-precheck: FAIL %s output:\n' "$(sed -n "${i}p" "$dir/steps" | cut -f1)"
    again=$(failing_tests "$dir/$i.log")
    [[ -z $again ]] || printf 'batch-precheck: failing tests: %s\n' "$again"
    tail -n 40 "$dir/$i.log" | sed 's/^/  | /'
  done
  printf 'batch-precheck: failed: %s\n' "$(IFS=,; printf '%s' "${failed[*]}")"
  exit 1
fi
if ((warn)); then
  printf 'batch-precheck: no receipt: %d step(s) failed, though only base-present\n' "$warn"
elif ((pristine_before)) && tree_clean && [[ $(git rev-parse HEAD) == "$head_before" ]]; then
  # Written beside the receipt and renamed over it, so a failed write leaves no receipt.
  if printf 'corvint-batch-precheck-receipt/0\nhead %s\nsteps %s\npassed %s\nfinished %s\n' \
    "$head_before" "$((pass + fail))" "$pass" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$receipt.tmp" && /bin/mv -f "$receipt.tmp" "$receipt"; then
    printf 'batch-precheck: receipt for %s written to %s\n' "$head_before" "$receipt"
  else
    /bin/rm -f "$receipt.tmp"
    printf 'batch-precheck: could not write the receipt %s\n' "$receipt" >&2
    exit 1
  fi
else
  printf 'batch-precheck: no receipt: the checkout had uncommitted, untracked or ignored package-input files or HEAD moved, so the checked tree is not HEAD\n'
  ignored=$(ignored_inputs)
  [[ -z $ignored ]] || printf 'batch-precheck: ignored files a go test can read (move or delete them, then rerun):\n%s\n' "$(sed 's/^/  /' <<< "$ignored")"
fi
