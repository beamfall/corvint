#!/usr/bin/env bash
# usage: script/requirement-ids-free.sh PREFIX LO HI [REF]
#        script/requirement-ids-free.sh PREFIX [REF]
#
# Reports whether the requirement IDs PREFIX-LO..PREFIX-HI (three-digit numbers) are free in REF's
# tree (default origin/main), so a lane charter never names a block already in use (ASD-V0-960,
# V1-3472). Each ID some tracked file at REF names prints as `used ID PATH:LINE` (its first
# occurrence) and the exit is 1; otherwise one `free PREFIX-LO..PREFIX-HI at REF (SHA)` line and
# exit 0. A malformed input or a REF that is not a commit exits 2 naming it. It reads only Git
# objects: the worktree, the index and unpushed branches other than REF are not consulted.
#
# With no range, PREFIX is a new family a charter wants to claim (ASD-V0-962, V1-3472): any row of
# REF's docs/specs/REQUIREMENTS.tsv in that family refuses it with `used-prefix PREFIX` and exit 1,
# naming its row count, its first ID and spec, and the next free prefix (the family's last letter
# advanced until neither REQUIREMENTS.tsv nor any tracked file names it); otherwise `free-prefix`.
set -uo pipefail
export LC_ALL=C

usage='usage: script/requirement-ids-free.sh PREFIX LO HI [REF] (for example DCW-V0 960 979 origin/main), or PREFIX [REF] for a new prefix'
if (($# < 1 || $# > 4)); then
  printf 'requirement-ids-free: REFUSE bad-arguments\n%s\n' "$usage" >&2
  exit 2
fi
prefix=$1
[[ $prefix =~ ^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$ ]] || {
  printf 'requirement-ids-free: REFUSE bad-prefix %q: a requirement prefix is upper-case letters and digits joined by -, such as DCW-V0\n' "$prefix" >&2
  exit 2
}
if (($# <= 2)); then
  ref=${2:-origin/main}
  sha=$(git rev-parse --verify -q --end-of-options "$ref^{commit}") || {
    printf 'requirement-ids-free: REFUSE ref-not-a-commit %q: fetch it (git fetch origin) or name a commit\n' "$ref" >&2
    exit 2
  }
  rows=$(git show "$sha:docs/specs/REQUIREMENTS.tsv" 2> /dev/null) || {
    printf 'requirement-ids-free: REFUSE requirements-index-unavailable: %s has no docs/specs/REQUIREMENTS.tsv\n' "$ref" >&2
    exit 2
  }
  # family is the prefix up to a trailing -V<n> version; a new prefix must not reuse any version of it.
  family=$prefix version=
  [[ $prefix =~ ^(.*)(-V[0-9]+)$ ]] && family=${BASH_REMATCH[1]} version=${BASH_REMATCH[2]}
  in_family() { awk -F'\t' -v family="$1-" 'index($1, family) == 1 && substr($1, length(family) + 1) ~ /^(V[0-9]+-)?[0-9]+$/' <<< "$rows"; }
  used=$(in_family "$family")
  if [[ -z $used ]]; then
    printf 'free-prefix %s at %s (%s)\n' "$prefix" "$ref" "$sha"
    exit 0
  fi
  next=
  stem=${family%?} last=${family: -1}
  for letter in {A..Z}; do
    [[ $letter > $last ]] || continue
    candidate=$stem$letter
    [[ -z $(in_family "$candidate") ]] || continue
    git grep -q -I -E "(^|[^A-Za-z0-9])$candidate-(V[0-9]+-)?[0-9]{3}" "$sha" -- . 2> /dev/null && continue
    next=$candidate$version
    break
  done
  first=$(head -1 <<< "$used")
  printf 'used-prefix %s: %d requirement rows in docs/specs/REQUIREMENTS.tsv at %s, first %s in %s; next free prefix: %s\n' \
    "$prefix" "$(wc -l <<< "$used" | tr -d ' ')" "$ref" "$(cut -f1 <<< "$first")" "$(cut -f2 <<< "$first")" "${next:-NONE (no later letter is free; choose another family)}"
  exit 1
fi
lo=$2 hi=$3 ref=${4:-origin/main}
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
