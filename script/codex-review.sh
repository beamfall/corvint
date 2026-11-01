#!/usr/bin/env bash
# usage: script/codex-review.sh --preflight
#        script/codex-review.sh [CODEX_EXEC_ARG...]
#
# ASD-V0-064 (V1-3219): the lane's independent Codex review with a login preflight. It checks
# `codex login status` exactly once before any review call; when the CLI is missing or not logged in
# it prints one `codex-review: NOT_AVAILABLE <reason>` line naming the fix and exits 3 at once, so a
# lane records the review as NOT_RUN instead of retrying "401 Not logged in" until it times out.
#   --preflight  only the check, for lane setup: prints `codex-review: OK logged-in` and exits 0.
#   otherwise    runs `codex exec -m CODEX_REVIEW_MODEL -s read-only CODEX_EXEC_ARG...` (the prompt
#                as an argument or on stdin) and exits with its status. ASD-V0-1020 (V1-3959): its
#                first stdout line pins the reviewed checkout, `codex-review: reviewed-head=SHA
#                tree=SHA dirty=yes|no` (or `reviewed-head=NOT_PRODUCED not-a-git-checkout`), so
#                the saved review names the head its findings are about (script/review-head-check.sh).
#   CODEX_BIN          the codex executable (default: codex from PATH)
#   CODEX_REVIEW_MODEL  the review model (default gpt-6.1-sol)
#   CODEX_REVIEW_HEAVY  the host-queue wrapper the review runs through as WRAPPER --admit-only CODEX...
#                       (ASD-V0-930, V1-3626; default the heavy.sh beside this script, which then
#                       never reads the prompt; empty runs codex directly). The preflight takes no slot.
# Exit: 0 ok (or the review's own status), 2 usage, 3 NOT_AVAILABLE.
set -uo pipefail

if [[ ${1-} == -h || ${1-} == --help ]]; then
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
fi
codex=${CODEX_BIN:-codex}
model=${CODEX_REVIEW_MODEL:-gpt-6.1-sol}
fix="run 'codex login' (or 'codex login --with-api-key' on a headless host) and rerun; until then record the review as NOT_RUN"

if ! resolved=$(type -P -- "$codex"); then
  printf 'codex-review: NOT_AVAILABLE codex-not-installed (%s not found); install the Codex CLI, or record the review as NOT_RUN\n' "$codex" >&2
  exit 3
fi
# One check, never retried: a logged-out CLI answers every review call with 401.
status=$("$resolved" login status 2>&1 < /dev/null)
rc=$?
first=${status%%$'\n'*}
if [[ $rc != 0 ]] || grep -qi 'not logged in' <<< "$status"; then
  printf 'codex-review: NOT_AVAILABLE not-logged-in (codex login status exit %s: %s); fix: %s\n' "$rc" "$first" "$fix" >&2
  exit 3
fi
if [[ ${1-} == --preflight ]]; then
  [[ $# == 1 ]] || { printf 'codex-review: usage: codex-review.sh --preflight\n' >&2; exit 2; }
  printf 'codex-review: OK logged-in (%s)\n' "$first"
  exit 0
fi
heavy=${CODEX_REVIEW_HEAVY-$(dirname "$0")/heavy.sh}
# ASD-V0-1020: pin the reviewed head before the review starts; the review reads this checkout.
if head=$(git rev-parse --verify -q 'HEAD^{commit}' 2> /dev/null); then
  dirty=no
  git diff --quiet HEAD -- 2> /dev/null || dirty=yes
  printf 'codex-review: reviewed-head=%s tree=%s dirty=%s\n' "$head" "$(git rev-parse 'HEAD^{tree}')" "$dirty"
else
  printf 'codex-review: reviewed-head=NOT_PRODUCED not-a-git-checkout\n'
fi
exec ${heavy:+"$heavy" --admit-only} "$resolved" exec -m "$model" -s read-only "$@"
