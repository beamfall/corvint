# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
# Sourced by the CEM workflow recipes; not run directly.
#
# run_step NAME ARGV... runs one child in its own process group, with standard output in
# "$recipe_out/NAME.out" and standard error in "$recipe_out/NAME.stderr". The group is killed
# after RECIPE_TIMEOUT seconds (default 600; the step then returns 124), and on EXIT, HUP, INT
# or TERM of the recipe, so no child outlives it. Each step line and the final outcome line go
# to standard output.

set -m
recipe_timeout=${RECIPE_TIMEOUT:-600}
child_pid=
watch_pid=

stop_children() {
  if [ -n "$watch_pid" ]; then kill -TERM -- "-$watch_pid" 2>/dev/null; fi
  if [ -n "$child_pid" ]; then kill -TERM -- "-$child_pid" 2>/dev/null; fi
  child_pid=
  watch_pid=
}
trap stop_children EXIT
trap 'stop_children; exit 129' HUP
trap 'stop_children; exit 130' INT
trap 'stop_children; exit 143' TERM

operational() {
  printf 'outcome=operational reason=%s\n' "$1"
  exit 2
}

case $recipe_timeout in
  ''|0|*[!0123456789]*) operational 'RECIPE_TIMEOUT must be a positive whole number of seconds' ;;
esac

# The output directory holds the retained receipts, refusals included. It must sit outside the
# repository, because an untracked file there would dirty the worktree the commands read.
recipe_out=${RECIPE_OUT:-}
if [ -z "$recipe_out" ]; then
  recipe_out=$(mktemp -d "${TMPDIR:-/tmp}/corvint-recipe.XXXXXX") || operational 'cannot create output directory'
fi
mkdir -p "$recipe_out" || operational 'cannot create RECIPE_OUT'
recipe_out=$(CDPATH='' cd -- "$recipe_out" && pwd -P)

run_step() {
  local name=$1 status=0
  shift
  rm -f "$recipe_out/$name.timeout"
  "$@" > "$recipe_out/$name.out" 2> "$recipe_out/$name.stderr" &
  child_pid=$!
  (
    elapsed=0
    while [ "$elapsed" -lt "$recipe_timeout" ]; do
      sleep 1
      kill -0 "$child_pid" 2>/dev/null || exit 0
      elapsed=$((elapsed + 1))
    done
    : > "$recipe_out/$name.timeout"
    kill -TERM -- "-$child_pid" 2>/dev/null
  ) &
  watch_pid=$!
  wait "$child_pid" 2>/dev/null || status=$?
  kill -TERM -- "-$watch_pid" 2>/dev/null
  wait "$watch_pid" 2>/dev/null
  child_pid=
  watch_pid=
  if [ -e "$recipe_out/$name.timeout" ]; then status=124; fi
  printf 'step=%s exit=%s\n' "$name" "$status"
  return "$status"
}
