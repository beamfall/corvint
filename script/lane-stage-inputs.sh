#!/usr/bin/env bash
# lane-stage-inputs.sh: stage a lane's ticket bodies on its host before the lane session starts
# (ASD-V0-742, V1-3245). A lane host has no Tasks journal, so `ticket show` there refuses; a lane
# that starts before its bodies are staged works from the 120-character digest titles instead.
#
#   script/lane-stage-inputs.sh [--host HOST] LANE TICKET...
#
# Run on the host that owns the Tasks store. For each TICKET it exports `corvint-tasks ticket show`
# JSON as TICKET.json and `--format text` (TCL-V0-741) as TICKET.txt, and the whole queue as
# queue-snapshot.json (`queue snapshot`, the authority the lane's `queue parity` and agent-preflight
# tasks-store row compare against, TCL-V0-742), copies them into ~/lane-inputs/ticket-bodies-LANE/
# on HOST (over ssh; on this host without --host), then lists that directory on the target and
# requires every file to be there and non-empty. One line per finding:
#   EXPORT   ticket show or queue snapshot refused or printed nothing (nothing is copied)
#   MISSING  a file is absent or empty on the target after the copy
# Exit 0 with a PASS line, 1 on any finding, 2 on a usage error or a failed copy or listing. Do not
# start the lane session unless it exits 0. CORVINT_TASKS_BIN and LANE_SSH override the
# corvint-tasks and ssh commands (for tests).
set -uo pipefail
export LC_ALL=C
me='lane-stage-inputs'
usage() { printf 'usage: script/lane-stage-inputs.sh [--host HOST] LANE TICKET...\n' >&2; exit 2; }
refuse() { printf '%s: %s\n' "$me" "$*" >&2; exit 2; }
tasks=${CORVINT_TASKS_BIN:-corvint-tasks} ssh_cmd=${LANE_SSH:-ssh} host=''
while (($#)); do
  case $1 in
    --host) { (($# >= 2)) && [[ -n $2 ]]; } || usage; host=$2; shift 2 ;;
    -*) usage ;;
    *) break ;;
  esac
done
(($# >= 2)) || usage
lane=$1; shift
[[ $lane =~ ^[A-Za-z0-9._-]+$ ]] || refuse "lane name '$lane' is not valid; use letters, digits, dot, dash or underscore"
[[ -z $host || $host =~ ^[A-Za-z0-9._@-]+$ ]] || refuse "host '$host' is not valid; use an ssh host alias or user@host"
for id in "$@"; do
  [[ $id =~ ^[A-Za-z0-9._:-]+$ ]] || refuse "ticket '$id' is not valid; use a ticket ID such as V1-3245"
done
rel="lane-inputs/ticket-bodies-$lane"
# on_target runs one shell command on the target: over ssh with --host, here without.
on_target() { if [[ -n $host ]]; then "$ssh_cmd" "$host" "$1"; else bash -c "$1"; fi; }
stage=$(mktemp -d "${TMPDIR:-/tmp}/lane-stage-inputs.XXXXXX") || refuse 'mktemp failed'
trap '/bin/rm -rf "$stage"' EXIT
findings=0
for id in "$@"; do
  "$tasks" ticket show "$id" > "$stage/$id.json" 2> /dev/null && [[ -s $stage/$id.json ]] &&
    "$tasks" ticket show "$id" --format text > "$stage/$id.txt" 2> /dev/null && [[ -s $stage/$id.txt ]] || {
    printf '%s: EXPORT ticket=%s: corvint-tasks ticket show refused or printed nothing; check the ID and the store\n' "$me" "$id"
    findings=$((findings + 1))
  }
done
"$tasks" queue snapshot --out "$stage/queue-snapshot.json" > /dev/null 2>&1 && [[ -s $stage/queue-snapshot.json ]] || {
  printf '%s: EXPORT queue-snapshot.json: corvint-tasks queue snapshot refused or wrote nothing; check the store\n' "$me"
  findings=$((findings + 1))
}
((findings == 0)) || { printf '%s: FAIL %d exports refused; nothing was copied\n' "$me" "$findings"; exit 1; }
target=${host:-this host}
tar -C "$stage" -cf - . | on_target "mkdir -p \"\$HOME/$rel\" && tar -C \"\$HOME/$rel\" -xf -" ||
  refuse "copying to ~/$rel on $target failed"
# A directory absent after the copy lists nothing, so each of its files is reported MISSING.
listing=$(on_target "cd \"\$HOME/$rel\" 2> /dev/null || exit 0; find . -maxdepth 1 -type f -size +0c") ||
  refuse "listing ~/$rel on $target failed"
want=(queue-snapshot.json)
for id in "$@"; do want+=("$id.json" "$id.txt"); done
for f in ${want[@]+"${want[@]}"}; do
  grep -qxF "./$f" <<< "$listing" || {
    printf '%s: MISSING %s:~/%s/%s is absent or empty after the copy\n' "$me" "$target" "$rel" "$f"
    findings=$((findings + 1))
  }
done
((findings == 0)) || exit 1
printf '%s: PASS %d tickets staged and verified at %s:~/%s\n' "$me" "$#" "$target" "$rel"
