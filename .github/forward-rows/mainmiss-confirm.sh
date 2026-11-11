#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# CIN-V0-7922 (V1-5198): confirm or clear the suspected misses of one corvint-main-miss/0 document
# (mainmiss.sh) by reruns, the package-level analogue of linecov-miss.yml's GLS-V0-046 protocol.
# For each suspected pull request and each package its narrowed selection skipped, the package is
# run once at the first parent of the pull request's merge commit on main (its base) and twice at
# the merge commit. A package that passes at the base and fails both times at the merge is
# confirmed: the pull request's change broke a package its selection left out. One that passes at
# the merge is a flake of the main run; one that fails at the base was broken before the pull
# request (preexisting); one that cannot be rerun (no merge commit, not an ancestor of the failed
# commit, an unusable go test capture, no time left) stays unconfirmed. Nothing here guesses: a
# verdict other than confirmed never says the selection was right, only that this run did not
# prove it wrong.
# Environment: MISS (miss.json), SOURCE (a checkout of main holding HEAD_SHA and its history),
# RUN_ID, HEAD_SHA, FORWARD_EPOCH, RUNNER_TEMP, GITHUB_OUTPUT, GITHUB_STEP_SUMMARY; GO (default
# go), GO_TEST_FLAGS (extra go test flags, e.g. "-race -exec /path/test-confine", split on
# spaces), TEST_TIMEOUT (default 30m), MAX_PACKAGES (default 5, over every pull request),
# BUDGET_SECONDS (default 10800: a package whose three reruns cannot start within it stays
# unconfirmed, and with timeout(1) on PATH each rerun is killed when the budget ends, so the step
# writes check.json instead of outliving its deadline; that package stays unconfirmed). Writes $RUNNER_TEMP/main-miss/check.json (corvint-main-miss-check/0) and the step
# outputs `check` (its artifact name, ci-forward-MAINCHECK-r<run>-<sha>-e<epoch>-c<confirmed>-
# k<flake>-u<unconfirmed>-pr<N>[_<N>...]), `confirmed` (1 when any pull request is confirmed) and
# miss1..miss3: one ci-forward-MISS-pr<N>-<merge>-e<epoch>-f1 row name per confirmed pull request,
# its document at $RUNNER_TEMP/main-miss/miss<K>/row.json, so bar.jq counts a confirmed miss as a
# MISS row of the epoch (AFP-V0-029) and of the AFP-V0-030 window. A fourth confirmed pull request
# and beyond are in check.json and the summary only.
set -u
dir="$RUNNER_TEMP/main-miss"
mkdir -p "$dir/confirm"
say() { printf '%s\n' "$1" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"; }
hex40() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
digits() { printf '%s' "$1" | grep -Eqx '[0-9]+'; }
GO=${GO:-go}
GO_TEST_FLAGS=${GO_TEST_FLAGS:-}
TEST_TIMEOUT=${TEST_TIMEOUT:-30m}
MAX_PACKAGES=${MAX_PACKAGES:-5}
BUDGET_SECONDS=${BUDGET_SECONDS:-10800}
started=$(date +%s)
elapsed() { echo $(( $(date +%s) - started )); }
# CIN-V0-7922: reruns are bounded by the time left in BUDGET_SECONDS where timeout(1) exists; on a
# host without it they run unbounded, as the step's own timeout-minutes is then the only bound.
bounded=""
! command -v timeout > /dev/null 2>&1 || bounded=1

if ! digits "$RUN_ID" || ! hex40 "$HEAD_SHA" || ! digits "$FORWARD_EPOCH" || [ ! -f "$MISS" ] ||
  ! git -C "$SOURCE" rev-parse --git-dir > /dev/null 2>&1; then
  say "main miss confirmation: run $RUN_ID at $HEAD_SHA with $MISS in $SOURCE is not a well-formed input; nothing confirmed"
  exit 0
fi
if ! jq -e '.profile == "corvint-main-miss/0" and (.misses | type) == "array" and (.misses | length) > 0' "$MISS" > /dev/null 2>&1; then
  say "main miss confirmation: $MISS is not a corvint-main-miss/0 document naming a miss; nothing confirmed"
  exit 0
fi
git -C "$SOURCE" cat-file -e "$HEAD_SHA^{commit}" 2> /dev/null || {
  say "main miss confirmation: $SOURCE does not hold the failed commit $HEAD_SHA; nothing confirmed"
  exit 0
}
go_version=$("$GO" version 2> /dev/null | head -n 1) || go_version=""
clean() {
  git -C "$SOURCE" worktree remove --force "$dir/at-base" 2> /dev/null || true
  git -C "$SOURCE" worktree remove --force "$dir/at-merge" 2> /dev/null || true
  rm -rf "$dir/at-base" "$dir/at-merge"
}
# rerun WORKTREE DIR PKG CAPTURE: one go test of the package, as the shard ran it; prints pass,
# fail (a package-level fail or a build failure of the package, with a nonzero exit), absent (no
# such directory), timeout (the run did not end within what is left of BUDGET_SECONDS, or none was
# left) or unusable (anything else: a compile error elsewhere, a pattern that matched no package, a
# pass with a nonzero exit).
rerun() {
  if [ ! -d "$1/$2" ]; then
    echo absent
    return
  fi
  status=0
  if [ -n "$bounded" ]; then
    left=$((BUDGET_SECONDS - $(elapsed)))
    if [ "$left" -le 0 ]; then
      echo timeout
      return
    fi
    # timeout(1) signals its whole process group, so the test binary and its children end too; a
    # run that ignores TERM is killed 30 s later (exit 137).
    # shellcheck disable=SC2086
    (cd "$1" && CORVINT_CONFINE_ROOT="$PWD" timeout -k 30 "$left" "$GO" test $GO_TEST_FLAGS -json -count=1 -timeout "$TEST_TIMEOUT" "./$2") > "$4" 2>&1 < /dev/null || status=$?
    if [ "$status" -eq 124 ] || { [ "$status" -eq 137 ] && [ "$(elapsed)" -ge "$BUDGET_SECONDS" ]; }; then
      echo timeout
      return
    fi
  else
    # shellcheck disable=SC2086
    (cd "$1" && CORVINT_CONFINE_ROOT="$PWD" "$GO" test $GO_TEST_FLAGS -json -count=1 -timeout "$TEST_TIMEOUT" "./$2") > "$4" 2>&1 < /dev/null || status=$?
  fi
  jq -R -s -r --arg pkg "$3" --argjson status "$status" '
    [split("\n")[] | fromjson? // empty | select(type == "object")] as $ev
    | any($ev[]; .Action == "pass" and .Package == $pkg and ((.Test // "") == "")) as $passed
    | (any($ev[]; .Action == "fail" and .Package == $pkg and ((.Test // "") == ""))
       or any($ev[]; .Action == "build-fail" and ((.ImportPath // "") | startswith($pkg)))) as $failed
    | if $status == 0 and $passed and ($failed | not) then "pass"
      elif $status != 0 and $failed then "fail"
      else "unusable" end' "$4" 2> /dev/null || echo unusable
}
n=0
: > "$dir/results.jsonl"
jq -c '.misses[] | select((.pr | type) == "number")' "$MISS" | while read -r miss; do
  pr=$(printf '%s' "$miss" | jq -r '.pr')
  head=$(printf '%s' "$miss" | jq -r '.head // ""')
  merge=$(printf '%s' "$miss" | jq -r '.merge // ""')
  base="" reason=""
  : > "$dir/confirm/pkgs-$pr.jsonl"
  if ! hex40 "$merge"; then
    reason="no merge commit is named for #$pr"
  elif ! git -C "$SOURCE" cat-file -e "$merge^{commit}" 2> /dev/null; then
    reason="merge commit $merge of #$pr is not in $SOURCE"
  elif ! git -C "$SOURCE" merge-base --is-ancestor "$merge" "$HEAD_SHA" 2> /dev/null; then
    reason="merge commit $merge of #$pr is not an ancestor of the failed commit $HEAD_SHA"
  elif ! base=$(git -C "$SOURCE" rev-parse --verify -q "$merge^1") || ! hex40 "$base"; then
    reason="merge commit $merge of #$pr has no first parent"
  fi
  module=""
  [ -z "$base" ] || module=$(git -C "$SOURCE" show "$merge:go.mod" 2> /dev/null | sed -n 's/^module[[:space:]]\{1,\}//p' | head -n 1)
  if [ -n "$base" ] && [ -z "$module" ]; then
    reason="merge commit $merge of #$pr has no readable module path"
    base=""
  fi
  if [ -n "$base" ]; then
    clean
    if ! git -C "$SOURCE" worktree add -q --detach "$dir/at-base" "$base" 2> /dev/null ||
      ! git -C "$SOURCE" worktree add -q --detach "$dir/at-merge" "$merge" 2> /dev/null; then
      reason="the worktrees of #$pr at $base and $merge could not be added"
      base=""
    fi
  fi
  for pkg in $(printf '%s' "$miss" | jq -r '.packages[]? | select(type == "string")'); do
    verdict=unconfirmed why="" b="" m1="" m2=""
    if [ -n "$reason" ]; then
      why=$reason
    elif [ "$n" -ge "$MAX_PACKAGES" ]; then
      why="more than $MAX_PACKAGES packages suspected in this run"
    elif [ "$(elapsed)" -gt "$BUDGET_SECONDS" ]; then
      why="no time left for three reruns within $BUDGET_SECONDS s"
    else
      n=$((n + 1))
      case "$pkg" in
      "$module") pkgdir=. ;;
      "$module"/*) pkgdir=${pkg#"$module"/} ;;
      *) pkgdir="" ;;
      esac
      if [ -z "$pkgdir" ]; then
        why="$pkg is outside module $module"
      else
        b=$(rerun "$dir/at-base" "$pkgdir" "$pkg" "$dir/confirm/$pr-$n-base.json")
        case "$b" in
        fail) verdict=preexisting why="the package already fails at the base $base" ;;
        unusable) why="the base rerun at $base is unusable (compile error elsewhere, no package or a pass with a nonzero exit)" ;;
        timeout) why="the base rerun at $base did not end within the $BUDGET_SECONDS s budget" ;;
        pass | absent)
          m1=$(rerun "$dir/at-merge" "$pkgdir" "$pkg" "$dir/confirm/$pr-$n-merge1.json")
          case "$m1" in
          pass) verdict=flake why="the package passed at the merge commit $merge" ;;
          fail)
            m2=$(rerun "$dir/at-merge" "$pkgdir" "$pkg" "$dir/confirm/$pr-$n-merge2.json")
            case "$m2" in
            pass) verdict=flake why="the package passed on its second rerun at the merge commit $merge" ;;
            fail) verdict=confirmed why="the package passes at the base $base and failed twice at the merge commit $merge" ;;
            timeout) why="the second merge rerun at $merge did not end within the $BUDGET_SECONDS s budget" ;;
            *) why="the second merge rerun at $merge is $m2" ;;
            esac
            ;;
          timeout) why="the merge rerun at $merge did not end within the $BUDGET_SECONDS s budget" ;;
          *) why="the merge rerun at $merge is $m1" ;;
          esac
          [ "$b" != absent ] || why="$why; the package is absent at the base, so the base counts as a pass"
          ;;
        esac
      fi
    fi
    jq -n -c --arg pkg "$pkg" --arg verdict "$verdict" --arg why "$why" --arg b "$b" --arg m1 "$m1" --arg m2 "$m2" \
      '{package: $pkg, verdict: $verdict, why: $why,
        base: (if $b == "" then null else $b end),
        merge: [$m1, $m2 | select(. != "")]}' >> "$dir/confirm/pkgs-$pr.jsonl"
    say "CIN-V0-7922: #$pr $pkg: $verdict ($why)"
  done
  clean
  jq -s -c --argjson pr "$pr" --arg head "$head" --arg merge "$merge" --arg base "$base" \
    '{pr: $pr, head: (if $head == "" then null else $head end), merge: (if $merge == "" then null else $merge end),
      base: (if $base == "" then null else $base end), packages: .,
      verdict: (if any(.[]; .verdict == "confirmed") then "CONFIRMED"
                elif length > 0 and all(.[]; .verdict == "flake") then "FLAKE"
                else "UNCONFIRMED" end)}' "$dir/confirm/pkgs-$pr.jsonl" >> "$dir/results.jsonl"
done
jq -s -c --arg runId "$RUN_ID" --arg sha "$HEAD_SHA" --arg epoch "$FORWARD_EPOCH" --arg go "$go_version" \
  --arg flags "$GO_TEST_FLAGS" --arg timeout "$TEST_TIMEOUT" --slurpfile miss "$MISS" '
  sort_by(.pr) as $r
  | {profile: "corvint-main-miss-check/0", runId: ($runId | tonumber), sha: $sha, epoch: ($epoch | tonumber),
     failed: ($miss[0].failed // []),
     runner: {go: $go, goTestFlags: $flags, timeout: $timeout},
     results: $r,
     confirmed: [$r[] | select(.verdict == "CONFIRMED") | .pr],
     flake: [$r[] | select(.verdict == "FLAKE") | .pr],
     unconfirmed: [$r[] | select(.verdict == "UNCONFIRMED") | .pr]}
  | .name = "ci-forward-MAINCHECK-r\(.runId)-\(.sha)-e\(.epoch)-c\(.confirmed | length)-k\(.flake | length)-u\(.unconfirmed | length)-pr\([$r[:10][] | .pr | tostring] | join("_"))"' \
  "$dir/results.jsonl" > "$dir/check.json"
echo "check=$(jq -r '.name' "$dir/check.json")" >> "$GITHUB_OUTPUT"
k=0
for pr in $(jq -r '.confirmed[]' "$dir/check.json"); do
  k=$((k + 1))
  [ "$k" -le 3 ] || break
  merge=$(jq -r --argjson pr "$pr" '.results[] | select(.pr == $pr) | .merge' "$dir/check.json")
  mkdir -p "$dir/miss$k"
  jq -c --argjson pr "$pr" --argjson epoch "$FORWARD_EPOCH" --argjson run "$RUN_ID" '.results[] | select(.pr == $pr)
    | {profile: "corvint-main-miss-row/0", class: "MISS", epoch: $epoch, pr: .pr, sha: .merge, head: .head, base: .base,
       runId: $run, missed: [.packages[] | select(.verdict == "confirmed") | .package], confirmation: .}' \
    "$dir/check.json" > "$dir/miss$k/row.json"
  echo "miss$k=ci-forward-MISS-pr$pr-$merge-e$FORWARD_EPOCH-f1" >> "$GITHUB_OUTPUT"
done
if [ "$k" -gt 0 ]; then
  echo "confirmed=1" >> "$GITHUB_OUTPUT"
fi
jq -r '"main miss check `\(.name)`: \(.confirmed | length) confirmed" + (if (.confirmed | length) > 0 then " (" + (.confirmed | map("#\(.)") | join(", ")) + ")" else "" end)
  + ", \(.flake | length) flake, \(.unconfirmed | length) unconfirmed, over \(.results | length) suspected pull request(s)"' "$dir/check.json" |
  while read -r line; do say "$line"; done
