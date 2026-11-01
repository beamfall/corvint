#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# CIN-V0-010 (V1-3748): pin-lag.sh MAIN TOOL_PIN SHADOW_PIN decides whether the
# CORVINT_PR_TOOL_SOURCE and CORVINT_TEST_SHADOW_SOURCE pins build the selection
# code of MAIN, a commit of main. The narrowing gate runs the copy read from MAIN
# itself, in a checkout holding MAIN's history. Exit 0: each pin is in MAIN's
# history and its selection paths equal MAIN's. Exit 1: a pin lags or is not in
# MAIN's history; one ::error annotation names it. Exit 2: a malformed or
# unreadable pin or MAIN. Any nonzero exit leaves the shard FULL.
set -u
if [ $# -ne 3 ]; then
  echo "usage: pin-lag.sh MAIN TOOL_PIN SHADOW_PIN" >&2
  exit 2
fi
main=$1 tool=$2 shadow=$3
unreadable() {
  echo "::error title=CI pin lag unreadable (CIN-V0-010)::$1; this shard runs FULL"
  exit 2
}
head=$(git rev-parse --verify --quiet "$main^{commit}") || unreadable "MAIN $main is not a commit"
# The selection rules the pinned driver, selector, narrow.sh and shadow planner carry; their
# tests and fixtures do not change the built tools, and package-costs.json moves shard costs only.
set -- ':(glob)internal/gateselect/**' ':(glob)internal/liveverify/affected/**' \
  ':(glob)internal/sessiontest/**' ':(glob)tools/corvint-pr-tests/**' ':(glob)tools/gate-affected-select/**' \
  ':(glob).github/cishards/**' ':(glob)cmd/corvint/affected*.go' ':(glob)cmd/corvint/test*.go' \
  ':(glob).github/forward-rows/narrow.sh' ':(glob).github/forward-rows/narrow.jq' \
  ':(exclude,glob)**/*_test.go' ':(exclude,glob)**/testdata/**' ':(exclude).github/cishards/package-costs.json'
lag=0
for pair in "CORVINT_PR_TOOL_SOURCE $tool" "CORVINT_TEST_SHADOW_SOURCE $shadow"; do
  name=${pair%% *} pin=${pair#* }
  case $pin in
    *[!0-9a-f]* | '') unreadable "$name '$pin' is not a full commit id" ;;
  esac
  [ ${#pin} -eq 40 ] || unreadable "$name '$pin' is not a full commit id"
  git cat-file -e "$pin^{commit}" 2> /dev/null || unreadable "$name $pin is not a commit of this checkout"
  git merge-base --is-ancestor "$pin" "$head"
  case $? in
    0) ;;
    1)
      echo "::error title=CI pin lags selection code (CIN-V0-010)::$name $pin is not in the history of main $head; repin it to a reviewed main commit; this shard runs FULL"
      lag=1
      continue
      ;;
    *) unreadable "the ancestry of $name $pin is unreadable" ;;
  esac
  git diff --quiet "$pin" "$head" -- "$@"
  case $? in
    0) continue ;;
    1) ;;
    *) unreadable "the selection paths of $name $pin are unreadable" ;;
  esac
  count=$(git rev-list --count --no-merges "$pin..$head" -- "$@") || unreadable "the history of $name $pin is unreadable"
  first=$(git log --no-merges --format=%h "$pin..$head" -- "$@" | head -n 10 | tr '\n' ' ')
  echo "::error title=CI pin lags selection code (CIN-V0-010)::$name $pin builds older selection code than main $head ($count commit(s): ${first% }); repin it to a reviewed main commit; this shard runs FULL"
  lag=1
done
exit "$lag"
