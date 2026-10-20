#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-047: append the forward-row bar count to the step summary from the
# repository's newest artifact names (at most BAR_PAGES pages of 100), with gh
# and jq only. Environment: GH_TOKEN, REPO, FORWARD_EPOCH, GITHUB_STEP_SUMMARY.
# Only an artifact of a run on `main` of this repository counts: the forward-rows
# job runs there, while a pull-request or queue run can upload any name it likes.
set -u
BAR_PAGES=20
here=$(cd "$(dirname "$0")" && pwd)
names="${RUNNER_TEMP:-/tmp}/forward-row-names"
: > "$names"
page=1 read=0 limit="" exhausted=""
while [ "$page" -le "$BAR_PAGES" ]; do
  # One "STATE NAME" line per artifact, so a page of expired or other-branch artifacts still pages on.
  if ! got=$(gh api "repos/$REPO/actions/artifacts?per_page=100&page=$page" --jq '.artifacts[]
      | (if .expired then "expired"
         elif .workflow_run.head_branch == "main" and .workflow_run.repository_id != null
              and .workflow_run.head_repository_id == .workflow_run.repository_id then "main"
         else "other" end) + " " + .name'); then
    limit="artifact page $page is unreadable; the count covers the pages before it"
    break
  fi
  read=$page
  printf '%s\n' "$got" | sed -n 's/^main //p' >> "$names"
  # A short page is the end of the listing: every artifact was read (owner, 2026-10-20; V1-0812).
  [ "$(printf '%s\n' "$got" | grep -c .)" -ge 100 ] || {
    exhausted=1
    break
  }
  page=$((page + 1))
done
{
  jq -R -s -r --arg epoch "$FORWARD_EPOCH" --arg exhausted "$exhausted" -f "$here/bar.jq" < "$names" ||
    echo "AFP-V0-029 bar count unavailable: the artifact names could not be classified."
  echo ""
  if [ -n "$exhausted" ]; then
    echo "Window: $read page(s), every repository artifact."
  else
    echo "Window: $read page(s) of the newest repository artifacts; an older row outside it is not counted."
  fi
  [ -z "$limit" ] || echo "Limit: $limit."
} >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
