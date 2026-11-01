#!/usr/bin/env bash
# DCW-V0-038: observe every wrapper build environment without invoking Go or writing caches.
# DCW-V0-038 / V1-1893: a failed build shows the compiler error tail and keeps the full log.
# ASD-V0-019 / V1-1755: dogfood make targets refuse a defaulted BASE.
# Bash 3.2 does not apply errexit to [[ ]] in loops; assertions need explicit exits.
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd)
# DCW-V0-811: a host without rg asserts through the grep -P stand-in; only a host with neither refuses.
# shellcheck source=script/test-rg-fallback.sh
source "$source_root/script/test-rg-fallback.sh"
if ! test_rg_fallback; then
  printf 'dogfood-cache_test: REFUSE unsupported-environment-missing-rg (and grep has no -P for the stand-in)\n' >&2
  exit 1
fi
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-cache-test.XXXXXX")
cleanup() {
  find "$root" -type f -exec /bin/rm -f {} +
  find "$root" -depth -type d -exec rmdir {} +
  [[ -z $rg_fallback_dir ]] || /bin/rm -rf "$rg_fallback_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$root/repo/script" "$root/bin"
source "$source_root/script/dogfood-fixture-scripts.sh"
dogfood_fixture_copy "$source_root" "$root/repo/script" dogfood-{change,check,seal,batch-check,finish}.sh
/bin/cp -f "$source_root/Makefile" "$root/repo/"
printf 'test-version\n' > "$root/repo/VERSION"
printf 'base-source\n' > "$root/repo/source-marker"
git -C "$root/repo" init -q
git -C "$root/repo" add .
git -C "$root/repo" -c user.name=test -c user.email=test@example.invalid commit -qm fixture
base=$(git -C "$root/repo" rev-parse HEAD)
printf 'tree-source\n' > "$root/repo/source-marker"
cat > "$root/bin/corvint" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ ${1-} == --version ]]; then
  printf 'Corvint %s (build 0)\n' "${TEST_VERSION:-test-version}"
else
  printf '%s\n' "$*" >> "$TEST_FLOW_LOG"
  [[ "${1-} ${2-}" != 'dogfood batch-check' ]] || printf 'dogfood-batch-check: PASS constituents=0 commits=1\n'
fi
EOF
cat > "$root/bin/go" <<'EOF'
#!/usr/bin/env bash
set -eu
[[ $1 == build && $2 == -o && ${!#} == ./cmd/corvint ]] || exit 1
printf '%s\t%s\t%s\t%s\n' "$GOCACHE" "$GOTOOLCHAIN" "$(cat source-marker)" "$*" >> "$TEST_BUILD_LOG"
[[ ${TEST_FAIL_SOURCE-} != "$(cat source-marker)" ]] || { printf 'early compile noise\n%.0s' 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 >&2; printf 'cmd/corvint/x.go:1:1: undefined: fakeCompileError\n' >&2; exit 1; }
# The interrupted build signals its own process group, the wrapper's, as a terminal interrupt does.
[[ ${TEST_SIGNAL_SOURCE-} != "$(cat source-marker)" ]] || { kill "-$TEST_SIGNAL" 0; exit 1; }
/bin/cp -f "$TEST_FAKE_CORVINT" "$3"
EOF
chmod +x "$root/bin/"* "$root/repo/script/"*.sh
export TEST_FAKE_CORVINT="$root/bin/corvint" TEST_BUILD_LOG="$root/build.log" TEST_FLOW_LOG="$root/flow.log"
export PATH="$root/bin:$PATH"
unset CORVINT_BIN CORVINT_GOCACHE GOCACHE TEST_FAIL_SOURCE TEST_VERSION
cd "$root/repo"

# A path with spaces also checks that the selected value remains one environment value.
for route in direct make; do
  for selection in default caller corvint both empty-corvint empty-all make-argument; do
    unset CORVINT_GOCACHE GOCACHE
    args=()
    expected=${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache
    case $selection in
      caller|empty-corvint) export GOCACHE="$root/caller cache"; expected=$GOCACHE ;;
      corvint|both) export CORVINT_GOCACHE="$root/corvint cache"; expected=$CORVINT_GOCACHE ;;
      make-argument)
        [[ $route == make ]] || continue
        export GOCACHE="$root/caller cache"
        args=("CORVINT_GOCACHE=$root/argument cache"); expected="$root/argument cache" ;;
    esac
    [[ $selection != both ]] || export GOCACHE="$root/caller cache"
    [[ $selection != empty-corvint && $selection != empty-all ]] || export CORVINT_GOCACHE=
    [[ $selection != empty-all ]] || export GOCACHE=
    for action in change check seal batch-check; do
      : > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
      status=0
      if [[ $route == direct ]]; then
        bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
      else
        make -s "dogfood-$action" "BASE=$base" ${args[@]+"${args[@]}"} > "$root/output" 2>&1 || status=$?
      fi
      if [[ $action == seal ]]; then
        # No CEM exists: prove seal reaches check, then refuses before a commit or rename.
        [[ $status == 2 ]] || exit 1
        rg -q '^dogfood-seal: REFUSE nothing-to-seal$' "$root/output" || exit 1
      else
        [[ $status == 0 ]] || exit 1
      fi
      count=0
      while IFS=$'\t' read -r cache toolchain source argv; do
        [[ $cache == "$expected" && $toolchain == local ]] || {
          printf '%s/%s/%s: expected cache=%s toolchain=local; got cache=%s toolchain=%s\n' \
            "$route" "$selection" "$action" "$expected" "$cache" "$toolchain" >&2
          exit 1
        }
        count=$((count + 1))
        if [[ $count == 1 ]]; then [[ $source == tree-source ]] || exit 1; else [[ $source == base-source ]] || exit 1; fi
        [[ $argv == *' -trimpath ./cmd/corvint' ]] || exit 1
      done < "$TEST_BUILD_LOG"
      if [[ $action == change ]]; then
        [[ $count == 1 ]] || exit 1
        rg -q "^dogfood change --corvint-bin .* $base$" "$TEST_FLOW_LOG" || exit 1
      elif [[ $action == batch-check ]]; then
        [[ $count == 1 ]] || exit 1
        rg -q "^dogfood batch-check --verifier .* $base$" "$TEST_FLOW_LOG" || exit 1
        # ASD-V0-090 (V1-3249): the verdict is retained in a report file whose path is printed.
        report="$(git rev-parse --absolute-git-dir)/corvint/batch-check-report.txt"
        rg -qF "dogfood-batch-check: REPORT $report verdict=PASS" "$root/output" || exit 1
        rg -q "^dogfood-batch-check: REPORT base=$base head=[0-9a-f]{40} verdict=PASS status=0$" "$report" || exit 1
        rg -qx 'dogfood-batch-check: PASS constituents=0 commits=1' "$report" || exit 1
        /bin/rm -f "$report"
      else
        [[ $count == 2 ]] || exit 1
        rg -q "^dogfood check --base-verifier .* --tree-verifier .* $base$" "$TEST_FLOW_LOG" || exit 1
      fi
    done
  done
done

# Existing version refusals happen before builds; a valid override bypasses only change's build.
export CORVINT_GOCACHE="$root/private cache" CORVINT_BIN="$TEST_FAKE_CORVINT"
for action in change check; do
  : > "$TEST_BUILD_LOG"
  status=0
  TEST_VERSION=wrong bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
  [[ $status == 2 && ! -s $TEST_BUILD_LOG ]] || exit 1
  rg -q "^dogfood-$action: REFUSE corvint-version-mismatch expected=test-version$" "$root/output" || exit 1
  bash "script/dogfood-$action.sh" "$base"
  if [[ $action == change ]]; then [[ ! -s $TEST_BUILD_LOG ]] || exit 1; else
    [[ $(wc -l < "$TEST_BUILD_LOG") -eq 2 ]] || exit 1
    rg -q -- "--override-verifier $CORVINT_BIN $base$" "$TEST_FLOW_LOG" || exit 1
  fi
done
# A PATH-only CORVINT_BIN is resolved to its absolute path before it reaches --override-verifier (V1-0685).
: > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
fake_dir=$(dirname "$TEST_FAKE_CORVINT")
PATH="$fake_dir:$PATH" CORVINT_BIN=$(basename "$TEST_FAKE_CORVINT") bash script/dogfood-check.sh "$base"
rg -q -- "--override-verifier $TEST_FAKE_CORVINT $base$" "$TEST_FLOW_LOG" || exit 1
# A relative PATH entry resolves to an absolute path before the wrapper changes directory.
: > "$TEST_FLOW_LOG"
# The fake binary's directory is a sibling of the repository, so the relative entry is computed
# in the shell (GOC-V0-008: no Python runtime) and checked to name the same directory.
rel_dir=../$(basename "$fake_dir")
[[ $(cd "$rel_dir" && pwd -P) == "$(cd "$fake_dir" && pwd -P)" ]] || exit 1
PATH="$rel_dir:$PATH" CORVINT_BIN=$(basename "$TEST_FAKE_CORVINT") bash script/dogfood-check.sh "$base"
rg -q -F -- "--override-verifier $PWD/$rel_dir/$(basename "$TEST_FAKE_CORVINT") $base" "$TEST_FLOW_LOG" || exit 1
# An unresolvable name is refused as unresolved, before the version probe.
for action in change check; do
  status=0
  CORVINT_BIN=corvint-no-such-binary bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
  [[ $status == 2 ]] || exit 1
  rg -q "^dogfood-$action: REFUSE corvint-bin-unresolved$" "$root/output" || exit 1
done
# ASD-V0-090: a report directory that cannot be made still runs the check, keeps its verdict and
# exit status, and says NOTE report NOT_PRODUCED.
evidence_dir="$(git rev-parse --absolute-git-dir)/corvint"
/bin/mv -f "$evidence_dir" "$evidence_dir.aside" 2> /dev/null || :
: > "$evidence_dir"
: > "$TEST_FLOW_LOG"
status=0
bash script/dogfood-batch-check.sh "$base" > "$root/output" 2>&1 || status=$?
/bin/rm -f "$evidence_dir"
[[ ! -e "$evidence_dir.aside" ]] || /bin/mv -f "$evidence_dir.aside" "$evidence_dir"
[[ $status == 0 ]] || exit 1
rg -q "^dogfood batch-check --verifier .* $base$" "$TEST_FLOW_LOG" || exit 1
rg -q '^dogfood-batch-check: NOTE report NOT_PRODUCED path=unavailable$' "$root/output" || exit 1
rg -qx 'dogfood-batch-check: PASS constituents=0 commits=1' "$root/output" || exit 1
unset CORVINT_BIN
for action in change check batch-check; do
  for source in tree-source base-source; do
    [[ $action == check || $source != base-source ]] || continue
    : > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
    status=0
    TEST_FAIL_SOURCE=$source bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
    [[ $status == 2 && ! -s $TEST_FLOW_LOG ]] || exit 1
    reason=current-tree-corvint-build-failed
    [[ $source != base-source ]] || reason=base-corvint-build-failed
    rg -q "^dogfood-$action: REFUSE $reason$" "$root/output" || exit 1
    # DCW-V0-332 (V1-1929): each build-failed refusal names its recovery.
    fix='Non-composing constituents'
    [[ $source != base-source ]] || fix='no change can be sealed on it'
    [[ $(grep -c "^dogfood-$action: fix: .*$fix" "$root/output") == 1 ]] || exit 1
    # V1-1893: the compiler error tail is shown and the full log is kept at the named path.
    rg -q '^cmd/corvint/x.go:1:1: undefined: fakeCompileError$' "$root/output" || exit 1
    build_log=$(sed -n 's/^dogfood: full build log: //p' "$root/output")
    [[ -n $build_log && $(grep -c 'early compile noise' "$build_log") == 20 ]] || exit 1
    [[ $(grep -c 'early compile noise' "$root/output") == 19 ]] || exit 1
    /bin/rm -f "$build_log"
  done
done
: > "$TEST_BUILD_LOG"
# ASD-V0-019 (V1-1755): dogfood make targets refuse a BASE taken from the Makefile default
# (local main) before running any script; an explicit or environment BASE still reaches it.
for target in dogfood-change dogfood-check dogfood-seal dogfood-batch-check; do
  status=0
  env -u BASE make -s "$target" > "$root/output" 2>&1 || status=$?
  [[ $status == 2 && ! -s $TEST_BUILD_LOG ]] || exit 1
  rg -q '^BASE is required: pass BASE=<full origin/main sha>' "$root/output" || exit 1
  [[ $(make -s -n "$target" BASE=feedface | grep -c 'script/dogfood-[a-z-]*\.sh "feedface"') == 1 ]] || exit 1
done
status=0
env -u BASE make -s dogfood-bind-range TARGET=x > "$root/output" 2>&1 || status=$?
[[ $status == 2 ]] && rg -q '^BASE is required' "$root/output" || exit 1
[[ $(BASE=feedface make -s -n dogfood-change | grep -c 'script/dogfood-change\.sh "feedface"') == 1 ]] || exit 1
status=0
env -u BASE make -s dogfood-finish LANE=1 > "$root/output" 2>&1 || status=$?
[[ $status == 2 && ! -s $TEST_BUILD_LOG ]] && rg -q 'REFUSE missing-inputs BASE ' "$root/output" || exit 1
# ASD-V0-085 (V1-3172): with no CEM bound at HEAD the refusal above also names its recovery; once
# one is committed, dogfood-check without BASE checks against its baseRevision and says so, and a
# recorded base HEAD does not contain is refused before any build.
env -u BASE make -s dogfood-check > "$root/output" 2>&1 || :
rg -qF '  fix: make dogfood-check BASE=<the full sha passed to dogfood-change>' "$root/output" || exit 1
git clone -q "$root/repo" "$root/bound"
(
  cd "$root/bound"
  mkdir .corvint
  printf '{"artifacts":[],"baseRevision":"%s","hunks":[]}\n' "$base" > .corvint/change.cem.json
  git add .corvint/change.cem.json
  git -c user.name=test -c user.email=test@example.invalid commit -qm bind
  : > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
  env -u BASE make -s dogfood-check > "$root/output" 2>&1 || exit 1
  rg -qFx "dogfood-check: BASE=$base from the CEM bound at HEAD (.corvint/change.cem.json baseRevision)" "$root/output" || exit 1
  rg -q "^dogfood check --base-verifier .* --tree-verifier .* $base$" "$TEST_FLOW_LOG" || exit 1
  printf '{"baseRevision":"%s"}\n' feedfacefeedfacefeedfacefeedfacefeedface > .corvint/change.cem.json
  git -c user.name=test -c user.email=test@example.invalid commit -qam unrelated-base
  : > "$TEST_BUILD_LOG"
  status=0
  env -u BASE make -s dogfood-check > "$root/output" 2>&1 || status=$?
  [[ $status == 2 && ! -s $TEST_BUILD_LOG ]] && rg -q '^BASE is required: pass BASE=<full origin/main sha>; no CEM bound at HEAD' "$root/output" || exit 1
  # Only the top-level member counts: a baseRevision nested in another member is not the CEM's base.
  # A top-level member repeated under any value type, or a CEM that is not valid JSON, fails closed.
  for cem in '{"artifacts":[{"baseRevision":"%s"}],"hunks":[]}' '{"baseRevision":{},"baseRevision":"%s"}' '{"baseRevision":"%s",}'; do
    printf "$cem\n" "$base" > .corvint/change.cem.json
    git -c user.name=test -c user.email=test@example.invalid commit -qam malformed-base
    status=0
    env -u BASE make -s dogfood-check > "$root/output" 2>&1 || status=$?
    [[ $status == 2 && ! -s $TEST_BUILD_LOG ]] && rg -q '^BASE is required: pass BASE=<full origin/main sha>; no CEM bound at HEAD' "$root/output" || exit 1
  done
) || exit 1
# ASD-V0-045 (V1-3119): dogfood-closeout without a plan refuses before any build or batch-precheck
# step and names the drafting and pre-flight commands.
status=0
make -s dogfood-closeout LANE=1 BASE=feedfacefeedfacefeedfacefeedfacefeedface > "$root/output" 2>&1 || status=$?
[[ $status == 2 && ! -s $TEST_BUILD_LOG ]] || exit 1
rg -q '^dogfood-finish-lane: REFUSE missing-inputs CITATIONS INTENTS VERIFY OUTCOME$' "$root/output" || exit 1
rg -qF 'corvint dogfood propose-citations feedfacefeedfacefeedfacefeedfacefeedface > plan.tsv' "$root/output" || exit 1
rg -qF 'make dogfood-inputs-check BASE=feedfacefeedfacefeedfacefeedfacefeedface' "$root/output" || exit 1
! rg -q 'batch-precheck' "$root/output" || exit 1
# DCW-V0-960 (V1-3047): the closeout runs its input steps with LANE=1 without being told, so a plan
# alone still names the missing INTENTS, VERIFY and OUTCOME before any build.
printf '1\tsource-marker\t1:1\timplementation\n' > "$root/closeout-cites.tsv"
status=0
env -u LANE make -s dogfood-closeout BASE=feedfacefeedfacefeedfacefeedfacefeedface CITATIONS="$root/closeout-cites.tsv" > "$root/output" 2>&1 || status=$?
[[ $status == 2 && ! -s $TEST_BUILD_LOG ]] || { cat "$root/output" >&2; exit 1; }
rg -q '^dogfood-finish-lane: REFUSE missing-inputs INTENTS VERIFY OUTCOME$' "$root/output" || { cat "$root/output" >&2; exit 1; }
# DCW-V0-900: the closeout checks its inputs, then runs batch-precheck before any bind or seal, and
# again after the seal for the receipt of the sealed head.
recipe=$(awk '/^dogfood-closeout:/ { on = 1; next } on && /^\t/ { sub(/^\t@\$\(MAKE\) --no-print-directory /, ""); print; next } on { exit }' "$source_root/Makefile" | tr '\n' ' ')
[[ $recipe == 'dogfood-inputs-check batch-precheck dogfood-finish batch-precheck ' ]] || { echo "dogfood-closeout recipe: $recipe" >&2; exit 1; }
# ASD-V0-087 (V1-3154): when the change touches dogfood tooling, a CORVINT_BIN override is refused
# before any build, naming the binary, the changed path and the recovery; unset, the lane-built tree
# binary runs the steps. A change outside that tooling keeps the override.
printf '1\tsource-marker\t1:1\timplementation\n' > "$root/cites.tsv"
mkdir -p internal/dogfoodflow
printf 'package dogfoodflow\n' > internal/dogfoodflow/new.go
git add internal/dogfoodflow/new.go
: > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
status=0
CORVINT_BIN="$TEST_FAKE_CORVINT" bash script/dogfood-finish.sh "$base" "$root/cites.tsv" > "$root/output" 2>&1 || status=$?
[[ $status == 2 && ! -s $TEST_BUILD_LOG && ! -s $TEST_FLOW_LOG ]] || { cat "$root/output" >&2; exit 1; }
rg -qF "dogfood-finish-lane: REFUSE corvint-bin-predates-tree-tooling CORVINT_BIN=$TEST_FAKE_CORVINT changed=internal/dogfoodflow/new.go (1 paths)" "$root/output" || exit 1
rg -q '^fix: unset CORVINT_BIN and rerun the same make target .*/\.git/corvint/corvint then runs every step' "$root/output" || exit 1
DOGFOOD_FINISH_PREFLIGHT=1 bash script/dogfood-finish.sh "$base" "$root/cites.tsv" > "$root/output" 2>&1 || { cat "$root/output" >&2; exit 1; }
rg -q -- '^dogfood inputs-check --base .* --corvint-bin .*/repo/\.git/corvint/corvint$' "$TEST_FLOW_LOG" || exit 1
git rm -q --cached internal/dogfoodflow/new.go
/bin/rm -f internal/dogfoodflow/new.go
rmdir internal/dogfoodflow internal
: > "$TEST_FLOW_LOG"
CORVINT_BIN="$TEST_FAKE_CORVINT" DOGFOOD_FINISH_PREFLIGHT=1 bash script/dogfood-finish.sh "$base" "$root/cites.tsv" > "$root/output" 2>&1 || { cat "$root/output" >&2; exit 1; }
rg -qF -- "--corvint-bin $TEST_FAKE_CORVINT" "$TEST_FLOW_LOG" || exit 1
# DCW-V0-151 (V1-1442): an interrupt during either finish build names the step and the empty
# NOT_PRODUCED block, exits 128 plus the signal and never reaches finish-lane. set -m gives the
# wrapper its own process group, so the signal never reaches this test.
for interrupt in tree-source:INT:build-tree:130 base-source:TERM:build-base:143; do
  IFS=: read -r source signal step code <<< "$interrupt"
  : > "$TEST_FLOW_LOG"
  set -m
  TEST_SIGNAL_SOURCE=$source TEST_SIGNAL=$signal \
    bash script/dogfood-finish.sh "$base" "$root/cites.tsv" > "$root/output" 2> "$root/stderr" &
  pid=$!
  set +m
  status=0
  wait "$pid" || status=$?
  # DCW-V0-380: the input pre-flight runs between the two builds, so an interrupted base build
  # follows exactly that one inputs-check and never reaches finish-lane.
  flow=
  [[ $step != build-base ]] || flow="dogfood inputs-check --base $base --citations $root/cites.tsv --corvint-bin *[^/]/repo/.git/corvint/corvint"
  # The git dir is the resolved path, which may differ from $root through a symlink.
  # shellcheck disable=SC2053
  [[ $status == "$code" && $(cat "$TEST_FLOW_LOG") == $flow ]] || { printf '%s: status=%s flow=%s\n' "$step" "$status" "$(cat "$TEST_FLOW_LOG")" >&2; exit 1; }
  [[ $(cat "$root/output") == "dogfood-finish-lane: $step INTERRUPTED"$'\n'"dogfood-finish-lane: NOT_PRODUCED rows=0" ]] || exit 1
  rg -q "^dogfood-finish-lane: INTERRUPTED $step$" "$root/stderr" || exit 1
  rg -q "^  fix: rerun make dogfood-finish BASE=$base with the same inputs" "$root/stderr" || exit 1
done
# The stub never creates even the selected caches, and no real Go build runs in this test.
[[ ! -e "$root/caller cache" && ! -e "$root/corvint cache" && ! -e "$root/private cache" && ! -e "$root/argument cache" ]] || exit 1
# PUB-V0-053: the coordinators read the --version line by its prefix, so the commit member and
# a development version pass and another version, a bare version or a second line refuse.
(
  repo=$source_root go_cache=$root/match-cache
  source "$source_root/script/dogfood-build-corvint.sh"
  commit=$(printf 'a%.0s' {1..40})
  for line in "Corvint 1.0.0-rc.3 (build 12)" "Corvint 1.0.0-rc.3 (build 12, commit $commit)" \
    "Corvint 1.0.0-rc.3.dev (build 7, commit $commit, modified)" "Corvint 1.0.0-rc.3.dev (commit unknown)"; do
    dogfood_version_matches "$line" 1.0.0-rc.3 || exit 1
  done
  for line in "Corvint 1.0.0-rc.2 (build 12)" "Corvint 1.0.0-rc.3" "Corvint 1.0.0Xrc.3 (build 1)" \
    "Corvint 1.0.0-rc.3.devx (build 1)" $'Corvint 1.0.0-rc.3 (build 1)\nCorvint 1.0.0-rc.3 (build 1)'; do
    ! dogfood_version_matches "$line" 1.0.0-rc.3 || exit 1
  done
  dogfood_version_matches "Corvint 0.4.0a4-dev (commit unknown)" 0.4.0a4 || exit 1
) || { printf 'dogfood_version_matches: FAIL\n' >&2; exit 1; }
printf 'DCW-V0-038 dogfood cache selection: PASS\n'
