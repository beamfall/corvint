#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Recipe 1: understand a proposed change (base..HEAD) before reviewing or extending it.
# Composes three read-only commands: `corvint impact --base`, `corvint affected --base` and
# `corvint context --task`. None writes repository, index or trace state.
#
# Required environment:
#   CORVINT_BASE   full commit ID the change starts from; HEAD is the change
#   CORVINT_TASK   one sentence naming the change, for `corvint context`
# Optional environment:
#   CORVINT_SUBJECT  repository-relative path the change centres on (context --subject)
#   CORVINT_ROOT     repository checkout (default $PWD); its worktree must be clean
#   CORVINT_BIN      Corvint executable (default corvint)
#   RECIPE_OUT       directory outside the repository for the receipts (default: a new temp dir)
#   RECIPE_TIMEOUT   per-step wall-clock bound in seconds (default 600)
#
# Output: step=NAME exit=N lines, then one outcome line. Exit 0 complete, 2 operational
# (bad recipe input or a step timed out), 3 incomplete: a step refused or impact is not READY.
# Every receipt and refusal stays in RECIPE_OUT as NAME.out and NAME.stderr.

set -u
# shellcheck source-path=SCRIPTDIR source=bounded.sh
. "$(dirname "$0")/bounded.sh"

base=${CORVINT_BASE:-}
task=${CORVINT_TASK:-}
subject=${CORVINT_SUBJECT:-}
root=${CORVINT_ROOT:-$PWD}
corvint=${CORVINT_BIN:-corvint}
[ -n "$base" ] || operational 'CORVINT_BASE is required'
[ -n "$task" ] || operational 'CORVINT_TASK is required'

finish() {
  printf 'outcome=%s %s out=%s\n' "$1" "$2" "$recipe_out"
  exit "$3"
}

step_or_stop() {
  local status=0
  run_step "$@" || status=$?
  if [ "$status" -eq 124 ]; then finish operational "timeout=$1" 2; fi
  if [ "$status" -ne 0 ]; then finish incomplete "refused=$1" 3; fi
}

subject_args=()
if [ -n "$subject" ]; then subject_args=(--subject "$subject"); fi

step_or_stop impact "$corvint" --root "$root" impact --base "$base"
step_or_stop affected "$corvint" --root "$root" affected --base "$base"
step_or_stop context "$corvint" --root "$root" context --task "$task" "${subject_args[@]+"${subject_args[@]}"}"

# The last upper-case "state" member of the compact impact receipt is context.state; any value
# other than READY (for example OUT_OF_SCOPE) is retained uncertainty, not a finished review.
state=$(grep -o '"state":"[A-Z_]*"' "$recipe_out/impact.out" | tail -n 1 | cut -d'"' -f4)
if [ "$state" != READY ]; then finish incomplete "impact-state=${state:-absent}" 3; fi
finish complete "impact-state=$state" 0
