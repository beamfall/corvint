#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-047: append the forward-row bar count to the step summary from the
# repository's newest artifact names (at most BAR_PAGES pages of 100), with gh
# and jq only. Environment: GH_TOKEN, REPO, FORWARD_EPOCH, GITHUB_STEP_SUMMARY.
set -u
BAR_PAGES=20
here=$(cd "$(dirname "$0")" && pwd)
names="${RUNNER_TEMP:-/tmp}/forward-row-names"
: > "$names"
page=1 read=0 limit=""
while [ "$page" -le "$BAR_PAGES" ]; do
  # One "EXPIRED NAME" line per artifact, so a page full of expired artifacts still pages on.
  if ! got=$(gh api "repos/$REPO/actions/artifacts?per_page=100&page=$page" --jq '.artifacts[] | "\(.expired) \(.name)"'); then
    limit="artifact page $page is unreadable; the count covers the pages before it"
    break
  fi
  read=$page
  printf '%s\n' "$got" | sed -n 's/^false //p' >> "$names"
  [ "$(printf '%s\n' "$got" | grep -c .)" -ge 100 ] || break
  page=$((page + 1))
done
{
  jq -R -s -r --arg epoch "$FORWARD_EPOCH" -f "$here/bar.jq" < "$names" ||
    echo "AFP-V0-029 bar count unavailable: the artifact names could not be classified."
  echo ""
  echo "Window: $read page(s) of the newest repository artifacts; an older row outside it is not counted."
  [ -z "$limit" ] || echo "Limit: $limit."
} >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
