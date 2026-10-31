#!/usr/bin/env bash
# usage: script/requirement-ids-free.sh PREFIX LO HI [REF]
#
# Reports whether the requirement IDs PREFIX-LO..PREFIX-HI (three-digit numbers) are free in REF's
# tree (default origin/main), so a lane charter never names a block already in use (ASD-V0-960,
# V1-3472). Each ID some tracked file at REF names prints as `used ID PATH:LINE` (its first
# occurrence) and the exit is 1; otherwise one `free PREFIX-LO..PREFIX-HI at REF (SHA)` line and
# exit 0. A malformed input or a REF that is not a commit exits 2 naming it. It reads only Git
# objects: the worktree, the index and unpushed branches other than REF are not consulted.
set -uo pipefail
export LC_ALL=C

usage='usage: script/requirement-ids-free.sh PREFIX LO HI [REF] (for example DCW-V0 960 979 origin/main)'
if (($# < 3 || $# > 4)); then
  printf 'requirement-ids-free: REFUSE bad-arguments\n%s\n' "$usage" >&2
  exit 2
fi
prefix=$1 lo=$2 hi=$3 ref=${4:-origin/main}
[[ $prefix =~ ^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$ ]] || {
  printf 'requirement-ids-free: REFUSE bad-prefix %q: a requirement prefix is upper-case letters and digits joined by -, such as DCW-V0\n' "$prefix" >&2
  exit 2
}
[[ $lo =~ ^[0-9]{1,3}$ && $hi =~ ^[0-9]{1,3}$ ]] && ((10#$lo <= 10#$hi)) || {
  printf 'requirement-ids-free: REFUSE bad-range %q..%q: LO and HI are numbers 0..999 with LO <= HI\n' "$lo" "$hi" >&2
  exit 2
}
sha=$(git rev-parse --verify -q --end-of-options "$ref^{commit}") || {
  printf 'requirement-ids-free: REFUSE ref-not-a-commit %q: fetch it (git fetch origin) or name a commit\n' "$ref" >&2
  exit 2
}
lo=$((10#$lo)) hi=$((10#$hi))
# git grep prints SHA:PATH:LINE:MATCH; a path may hold ':', so the line and match are the last two
# fields. The leading byte keeps XDCW-V0-960 from counting as DCW-V0-960; four digits never match.
# git grep exits 1 when nothing matches, which is a free range, not a failure.
matches=$(mktemp "${TMPDIR:-/tmp}/requirement-ids-free.XXXXXX") || exit 2
trap '/bin/rm -f "$matches"' EXIT
git grep -I -n -o -E "(^|[^A-Za-z0-9])$prefix-[0-9]+" "$sha" -- . > "$matches"
status=$?
if ((status > 1)); then
  printf 'requirement-ids-free: REFUSE search-failed git-grep-status=%d\n' "$status" >&2
  exit 2
fi
used=$(awk -v prefix="$prefix-" -v lo="$lo" -v hi="$hi" -v sha="$sha:" '
    {
      record = substr($0, length(sha) + 1)
      match_at = match(record, ":[0-9]+:[^:]*$")
      path = substr(record, 1, match_at - 1)
      rest = substr(record, match_at + 1)
      line = substr(rest, 1, index(rest, ":") - 1)
      id = substr(rest, index(rest, ":") + 1)
      id = substr(id, index(id, prefix))
      number = substr(id, length(prefix) + 1)
      if (length(number) != 3 || number + 0 < lo || number + 0 > hi || seen[id]++) next
      print "used " id " " path ":" line
    }' "$matches" | sort) || exit 2
range=$(printf '%s-%03d..%s-%03d' "$prefix" "$lo" "$prefix" "$hi")
if [[ -n $used ]]; then
  printf '%s\n' "$used"
  printf 'requirement-ids-free: %d of %d IDs in %s are used at %s (%s); pick a free block\n' \
    "$(printf '%s\n' "$used" | wc -l | tr -d ' ')" $((hi - lo + 1)) "$range" "$ref" "${sha:0:12}" >&2
  exit 1
fi
printf 'free %s at %s (%s)\n' "$range" "$ref" "$sha"
