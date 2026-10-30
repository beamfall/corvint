# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=sh
# AFP-V0-047/AFP-V0-097: list_names PAGES OUT [TIMES [STOP]] writes the names of the repository's
# newest artifacts (at most PAGES pages of 100, newest first) that a run on `main`
# of this repository uploaded, one per line, to OUT. Sourced by bar.sh, verdict.sh
# and accept.sh; needs gh, GH_TOKEN and REPO. TIMES, when named, gets one
# "CREATED_AT NAME" line per listed name. Sets `pages_read` (pages of 100),
# `exhausted` (1 when a short page ended the listing: every artifact was read)
# and `limit` (why the listing stopped early, or empty).
# CIN-V0-003 (V1-3091): STOP, when named, is a UTC time YYYY-MM-DDTHH:MM:SSZ; the listing ends
# after the first page that holds an artifact created at or before it and sets `reached` to 1,
# so a caller that needs only the artifacts since STOP reads only their pages.
# Only an artifact of a run on `main` of this repository counts: the forward-rows
# job runs there, while a pull-request or queue run can upload any name it likes.
# shellcheck disable=SC2034 # the callers read pages_read, exhausted, reached and limit
list_names() {
  : > "$2"
  times=${3:-/dev/null}
  stop=${4:-}
  [ "$times" = /dev/null ] || : > "$times"
  page=1 pages_read=0 limit="" exhausted="" reached=""
  while [ "$page" -le "$1" ]; do
    # AFP-V0-760 (V1-3084): an uncached page of 100 can come back truncated ("unexpected end
    # of JSON input") after about 8 s on every try, while the same artifacts read cleanly as
    # pages of 25. A page of 100 gets one read; when it fails it is read again as its four
    # pages of 25 (page P of 100 is pages 4P-3 to 4P of 25), each with the V1-2707 three
    # tries 5 s and 10 s apart; the last error stays in the limit.
    if list_names_read 100 "$page"; then
      list_names_keep "$2" "$times"
      list_names_reached
      # A short page is the end of the listing: every artifact was read (owner, 2026-10-20; V1-0812).
      [ "$(printf '%s\n' "$got" | grep -c .)" -ge 100 ] || exhausted=1
    else
      sub=$((page * 4 - 3))
      while [ "$sub" -le $((page * 4)) ]; do
        tries=1
        until list_names_read 25 "$sub"; do
          [ "$tries" -lt 3 ] || {
            limit="artifact page $page is unreadable: its page $sub of 25 failed after 3 tries ($(head -c 200 "${TMPDIR:-/tmp}/list-names.err" | tr '\n' ' ')); the count covers the artifacts before it"
            return 0
          }
          sleep $((tries * 5))
          tries=$((tries + 1))
        done
        list_names_keep "$2" "$times"
        list_names_reached
        [ -z "$reached" ] || break
        [ "$(printf '%s\n' "$got" | grep -c .)" -ge 25 ] || {
          exhausted=1
          break
        }
        sub=$((sub + 1))
      done
    fi
    pages_read=$page
    [ -z "$exhausted" ] && [ -z "$reached" ] || break
    page=$((page + 1))
  done
}

# list_names_read SIZE PAGE sets `got` to one "STATE NAME CREATED_AT" line per artifact of
# that page of SIZE, so a page of expired or other-branch artifacts still pages on, and
# fails on an unreadable page with its error in $TMPDIR/list-names.err.
list_names_read() {
  got=$(gh api "repos/$REPO/actions/artifacts?per_page=$1&page=$2" --jq '.artifacts[]
      | (if .expired then "expired"
         elif .workflow_run.head_branch == "main" and .workflow_run.repository_id != null
              and .workflow_run.head_repository_id == .workflow_run.repository_id then "main"
         else "other" end) + " " + .name + " " + (.created_at // "-")' 2> "${TMPDIR:-/tmp}/list-names.err")
}

# list_names_keep OUT TIMES appends the `main` artifacts of `got` to OUT and TIMES.
list_names_keep() {
  printf '%s\n' "$got" | sed -n 's/^main \(.*\) [^ ]*$/\1/p' >> "$1"
  printf '%s\n' "$got" | sed -n 's/^main \(.*\) \([^ ]*\)$/\2 \1/p' >> "$2"
}

# list_names_reached sets `reached` when STOP is named and `got` holds an artifact of any
# branch created at or before it (UTC times of one format compare as strings).
list_names_reached() {
  [ -z "$stop" ] || ! printf '%s\n' "$got" | awk -v s="$stop" '$NF != "-" && $NF <= s { f = 1 } END { exit !f }' || reached=1
}
