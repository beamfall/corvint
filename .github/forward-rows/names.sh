# SPDX-License-Identifier: AGPL-3.0-or-later
# shellcheck shell=sh
# AFP-V0-047/AFP-V0-097: list_names PAGES OUT [TIMES] writes the names of the repository's
# newest artifacts (at most PAGES pages of 100, newest first) that a run on `main`
# of this repository uploaded, one per line, to OUT. Sourced by bar.sh, verdict.sh
# and accept.sh; needs gh, GH_TOKEN and REPO. TIMES, when named, gets one
# "CREATED_AT NAME" line per listed name. Sets `pages_read`,
# `exhausted` (1 when a short page ended the listing: every artifact was read)
# and `limit` (why the listing stopped early, or empty).
# Only an artifact of a run on `main` of this repository counts: the forward-rows
# job runs there, while a pull-request or queue run can upload any name it likes.
# shellcheck disable=SC2034 # the callers read pages_read, exhausted and limit
list_names() {
  : > "$2"
  times=${3:-/dev/null}
  [ "$times" = /dev/null ] || : > "$times"
  page=1 pages_read=0 limit="" exhausted=""
  while [ "$page" -le "$1" ]; do
    # One "STATE NAME CREATED_AT" line per artifact, so a page of expired or other-branch artifacts still pages on.
    if ! got=$(gh api "repos/$REPO/actions/artifacts?per_page=100&page=$page" --jq '.artifacts[]
        | (if .expired then "expired"
           elif .workflow_run.head_branch == "main" and .workflow_run.repository_id != null
                and .workflow_run.head_repository_id == .workflow_run.repository_id then "main"
           else "other" end) + " " + .name + " " + (.created_at // "-")'); then
      limit="artifact page $page is unreadable; the count covers the pages before it"
      break
    fi
    pages_read=$page
    printf '%s\n' "$got" | sed -n 's/^main \(.*\) [^ ]*$/\1/p' >> "$2"
    printf '%s\n' "$got" | sed -n 's/^main \(.*\) \([^ ]*\)$/\2 \1/p' >> "$times"
    # A short page is the end of the listing: every artifact was read (owner, 2026-10-20; V1-0812).
    [ "$(printf '%s\n' "$got" | grep -c .)" -ge 100 ] || {
      exhausted=1
      break
    }
    page=$((page + 1))
  done
}
