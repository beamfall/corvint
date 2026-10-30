#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-530 (V1-2682 option A): the weekly CI flake report. Lists the AFP-V0-441
# `ci-flaky-<event>-<shard>` artifacts of the window with gh, downloads each archive,
# aggregates them with the ci-flake-report tool (TOOL, built from this commit), appends
# the report to the job summary and writes it into the one open `ci-flake-report` issue,
# creating and pinning it only when none exists. Every FLAKY record it could not
# download is named in the report; a listing failure fails the run instead of
# reporting a quiet week. It never reruns a test and changes no gate.
#
# Environment: GH_TOKEN, REPO, RUN_URL, TOOL; optional FLAKE_REPORT_NOW (RFC3339
# window end, for tests), FLAKE_REPORT_SHARDS (artifact names per event, default 16),
# GITHUB_STEP_SUMMARY.
set -eu
: "${GH_TOKEN:?}" "${REPO:?}" "${TOOL:?}"
run_url=${RUN_URL:-}
shards=${FLAKE_REPORT_SHARDS:-16}
label=ci-flake-report
title="CI flake report (weekly, AFP-V0-530)"
die() {
  echo "::error title=flake report::$1"
  exit 1
}
dir=$(mktemp -d)
trap 'rm -rf -- "$dir"' EXIT INT TERM
set --
[ -z "${FLAKE_REPORT_NOW:-}" ] || set -- -now "$FLAKE_REPORT_NOW"

# One exact-name query per retained artifact name: the ci-flaky names are bounded
# (two events, one per shard index), so this costs at most 2 * shards small pages.
: > "$dir/listing.json"
for event in pull_request merge_group; do
  s=0
  while [ "$s" -lt "$shards" ]; do
    gh api --paginate "repos/$REPO/actions/artifacts?name=ci-flaky-$event-$s&per_page=100" >> "$dir/listing.json" ||
      die "listing ci-flaky-$event-$s failed; the window is not observed"
    s=$((s + 1))
  done
done
"$TOOL" "$@" select "$dir/listing.json" > "$dir/selected.tsv" || die "selecting the window's records failed"

mkdir -p "$dir/artifacts"
tab=$(printf '\t')
while IFS="$tab" read -r id name created run; do
  [ -n "$id" ] || continue
  gh api "repos/$REPO/actions/artifacts/$id/zip" > "$dir/artifacts/$id.zip" ||
    { rm -f -- "$dir/artifacts/$id.zip"; echo "::warning title=flake report::artifact $id ($name, run $run, $created) not downloaded"; }
done < "$dir/selected.tsv"

"$TOOL" "$@" report -artifacts "$dir/artifacts" -run-url "$run_url" "$dir/listing.json" > "$dir/body.md" ||
  die "building the report failed"
[ -z "${GITHUB_STEP_SUMMARY:-}" ] || cat "$dir/body.md" >> "$GITHUB_STEP_SUMMARY"

# One issue, updated in place: the oldest open issue (not a pull request) with the label.
# Create the label only when missing; an existing label's metadata is never rewritten.
gh api "repos/$REPO/labels/$label" > /dev/null 2>&1 ||
  gh label create "$label" --repo "$REPO" --color 5319E7 \
    --description "Weekly CI flake report, updated in place (AFP-V0-530)" > /dev/null ||
  echo "::warning title=flake report::label $label not ensured"
number=$(gh api "repos/$REPO/issues?labels=$label&state=open&sort=created&direction=asc&per_page=100" \
  --jq 'map(select(.pull_request | not)) | .[0].number // empty') || die "listing the $label issues failed"
if [ -n "$number" ]; then
  gh issue edit "$number" --repo "$REPO" --body-file "$dir/body.md" > /dev/null ||
    die "updating issue #$number failed"
  said="updated issue #$number in place"
else
  url=$(gh issue create --repo "$REPO" --title "$title" --label "$label" --body-file "$dir/body.md") ||
    die "creating the $label issue failed"
  number=${url##*/}
  if gh issue pin "$number" --repo "$REPO" > /dev/null 2>&1; then
    said="opened and pinned issue #$number ($url)"
  else
    said="opened issue #$number ($url); pinning refused (at most three issues can be pinned), pin it by hand"
  fi
fi
echo "flake report: $said"
[ -z "${GITHUB_STEP_SUMMARY:-}" ] || printf '\nflake report: %s\n' "$said" >> "$GITHUB_STEP_SUMMARY"
