#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-531 (V1-2689): names why a go-product-shard failed, in its job summary.
#
#   failed.sh STREAM STATUS
#
# STREAM is the first attempt's go test -json stream, STATUS the exit status the
# shard is about to fail with; SHARD and SHARDS come from the environment. It
# appends a FAILED section to GITHUB_STEP_SUMMARY with one typed reason per failed
# package (test-failures with the failed tests named, build-failed, timeout,
# signal, panic or package-fail-no-test), the packages that never finished, and
# the stream's FAIL/panic/exit lines, bounded; and prints one ::error annotation.
# When no package reported a failure it says so: the Go step ended without a
# failure report (killed, out of memory, a runner signal or a truncated stream).
# It is advisory: it never changes the shard's status, and exits 0 on every
# path but a usage error.
set -eu
[ $# -eq 2 ] || { echo "usage: failed.sh STREAM STATUS" >&2; exit 2; }
stream=$1 status=$2
shard=${SHARD:-?} shards=${SHARDS:-?}
here=$(dirname "$0")
classified=$(jq -R -s -f "$here/failed.jq" "$stream" 2> /dev/null) ||
  classified='{"reasons":[],"unfinished":[],"unfinishedCount":0,"failLines":[],"failLineCount":0,"unreadable":true}'
reasons=$(printf '%s' "$classified" | jq -r '
  if .unreadable then "- the go test -json stream could not be read"
  elif (.reasons | length) == 0 then
    "- no package-level fail event: go test ended without a failure report (killed, out of memory, a runner signal or a truncated stream)"
  else .reasons[] | "- `\(.package)`: \(.reason)" +
    (if .testCount > 0 then ": " + (.tests | map("`" + . + "`") | join(", ")) +
      (if .testCount > (.tests | length) then " and \(.testCount - (.tests | length)) more" else "" end) else "" end)
  end')
summary_line=$(printf '%s' "$classified" | jq -r '
  if .unreadable then "stream unreadable"
  elif (.reasons | length) == 0 then "no package-level fail event"
  else [.reasons[] | "\(.package): \(.reason)"] | join("; ") end')
echo "::error title=go-product-shard $shard failed::exit $status; $summary_line"
[ -n "${GITHUB_STEP_SUMMARY:-}" ] || exit 0
{
  echo "### FAILED: go-product-shard $shard of $shards exited $status"
  echo
  echo "Typed reasons (AFP-V0-531, V1-2689):"
  echo
  echo "$reasons"
  printf '%s' "$classified" | jq -r 'select(.unfinishedCount > 0) |
    "\nUnfinished packages (\(.unfinishedCount)): " + (.unfinished | map("`" + . + "`") | join(", ")) +
    (if .unfinishedCount > (.unfinished | length) then " and \(.unfinishedCount - (.unfinished | length)) more" else "" end)'
  printf '%s' "$classified" | jq -r 'select(.failLineCount > 0) |
    "\nFAIL lines (\(.failLines | length) of \(.failLineCount)):\n\n```\n" + (.failLines | join("\n")) + "\n```"'
  echo
} >> "$GITHUB_STEP_SUMMARY"
exit 0
