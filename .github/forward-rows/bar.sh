#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: append the forward-row bar count to the step summary from the
# repository's newest artifact names (at most BAR_PAGES pages of 100), with gh
# and jq only. Environment: GH_TOKEN, REPO, FORWARD_EPOCH, GITHUB_STEP_SUMMARY.
# This summary is advisory; the trusted verdict of AFP-V0-097 (verdict.sh) is
# what a pull request's narrowing reads.
set -u
BAR_PAGES=20
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=names.sh
. "$here/names.sh"
names="${RUNNER_TEMP:-/tmp}/forward-row-names"
list_names "$BAR_PAGES" "$names"
{
  jq -R -s -r --arg epoch "$FORWARD_EPOCH" --arg exhausted "$exhausted" --rawfile reclassified "$here/infra-reclassified.txt" -f "$here/bar.jq" < "$names" ||
    echo "AFP-V0-029 bar count unavailable: the artifact names could not be classified."
  echo ""
  if [ -n "$exhausted" ]; then
    echo "Window: $pages_read page(s), every repository artifact."
  else
    echo "Window: $pages_read page(s) of the newest repository artifacts; an older row outside it is not counted."
  fi
  [ -z "$limit" ] || echo "Limit: $limit."
} >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
