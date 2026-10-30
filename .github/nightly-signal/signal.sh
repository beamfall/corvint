#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# SOP-V0-013: give a failed scheduled release-gates run on main one deduplicated,
# visible signal. The release-gates nightly-failure-signal job runs this copy from
# main's own commit with gh and jq only. It reads the attempt's jobs, then comments
# on the oldest open issue labelled `nightly-triage`, or opens one when none is
# open, naming the failing jobs, their failing steps and the run URL.
# Environment: GH_TOKEN, REPO, RUN_ID, RUN_ATTEMPT, RUN_URL, SHA and optional
# GITHUB_STEP_SUMMARY. A read or write that fails exits 1, so the signal job fails
# visibly in the run instead of passing silently.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
label=nightly-triage
title="Nightly release gates failing on main"
die() {
  printf 'nightly signal: %s\n' "$1" >&2
  exit 1
}
printf '%s' "$RUN_ID" | grep -Eqx '[0-9]+' || die "RUN_ID $RUN_ID is not a run id"
printf '%s' "$RUN_ATTEMPT" | grep -Eqx '[0-9]+' || die "RUN_ATTEMPT $RUN_ATTEMPT is not an attempt number"
printf '%s' "$SHA" | grep -Eqx '[0-9a-f]{40}' || die "SHA $SHA is not a full commit"
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
gh run view "$RUN_ID" --repo "$REPO" --attempt "$RUN_ATTEMPT" --json jobs > "$dir/jobs.json" ||
  die "cannot read the jobs of run $RUN_ID attempt $RUN_ATTEMPT"
jq -r --arg url "$RUN_URL" --arg sha "$SHA" -f "$here/body.jq" "$dir/jobs.json" > "$dir/body.md" ||
  die "cannot summarize the jobs of run $RUN_ID"
gh label create "$label" --repo "$REPO" --force --color B60205 \
  --description "A scheduled release-gates run on main failed" > /dev/null ||
  die "cannot ensure the $label label"
# The oldest open issue first; the REST list also returns pull requests, which never count.
number=$(gh api "repos/$REPO/issues?labels=$label&state=open&sort=created&direction=asc&per_page=100" \
  --jq 'map(select(.pull_request | not)) | .[0].number // empty') || die "cannot list open $label issues"
if [ -n "$number" ]; then
  gh issue comment "$number" --repo "$REPO" --body-file "$dir/body.md" > /dev/null ||
    die "cannot comment on issue #$number"
  said="commented on #$number"
else
  url=$(gh issue create --repo "$REPO" --title "$title" --label "$label" --body-file "$dir/body.md") ||
    die "cannot open the $label issue"
  said="opened $url"
fi
{
  printf 'nightly signal: %s\n\n' "$said"
  cat "$dir/body.md"
} >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
