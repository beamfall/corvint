#!/usr/bin/env bash
# usage: script/requirement-id-collisions.sh [--base REF]
#        script/requirement-id-collisions.sh --block PREFIX LO HI [--base REF]
#
# Finds requirement IDs that two open lines of work define independently, before either merges
# (ASD-V0-950..952; V1-2909, V1-1009, V1-3614). Every other local branch and every other worktree's
# HEAD that is not yet merged into REF (default origin/main) and shares history with it is an open
# ref; an open ref's new definitions are the rows of its committed docs/specs/REQUIREMENTS.tsv whose
# ID REF's index lacks.
#
# With no --block, this checkout's new definitions are the rows the generator prints from its Git
# index (the staged specs), so a lane can run it before it commits. Each ID this checkout and an
# open ref both newly define with a different spec or title prints
# `collision ID here FILE "TITLE" vs REF_NAME (SHA) FILE "TITLE"` and the exit is 1; otherwise one
# `no-collision ... open refs` line and exit 0. The same row on both sides is the same change
# (a lane branch and the batch that merged it) and passes.
#
# With --block, PREFIX-LO..PREFIX-HI is a requirement-ID block a charter wants to hand out: each ID
# in it that an open ref newly defines prints `used ID REF_NAME (SHA) FILE` and the exit is 1;
# otherwise `free ... on open refs`. `make requirement-ids-free` runs it after the REF tree check.
#
# Malformed input, a REF that is not a commit or a generator failure exits 2. It reads only Git
# objects and the index; it writes nothing.
set -uo pipefail
export LC_ALL=C

me=requirement-id-collisions
usage='usage: script/requirement-id-collisions.sh [--base REF] | --block PREFIX LO HI [--base REF]'
refuse() { printf '%s: REFUSE %s\n%s\n' "$me" "$1" "$usage" >&2; exit 2; }

base=origin/main block=
while (($#)); do
  case $1 in
    --base) (($# >= 2)) || refuse missing-base; base=$2; shift 2 ;;
    --block)
      (($# >= 4)) || refuse bad-block
      prefix=$2 lo=$3 hi=$4 block=1
      shift 4
      ;;
    *) refuse "bad-argument $1" ;;
  esac
done
if [[ -n $block ]]; then
  [[ $prefix =~ ^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$ ]] || refuse "bad-prefix $prefix"
  [[ $lo =~ ^[0-9]{1,3}$ && $hi =~ ^[0-9]{1,3}$ ]] && ((10#$lo <= 10#$hi)) || refuse "bad-range $lo..$hi"
  lo=$((10#$lo)) hi=$((10#$hi))
fi
top=$(git rev-parse --show-toplevel 2> /dev/null) && cd "$top" || refuse "not-a-checkout $PWD"
base_sha=$(git rev-parse --verify -q --end-of-options "$base^{commit}") ||
  refuse "ref-not-a-commit $base: fetch it (git fetch origin) or name a commit"

work=$(mktemp -d "${TMPDIR:-/tmp}/$me.XXXXXX") || exit 2
trap '/bin/rm -rf "$work"' EXIT
git show "$base_sha:docs/specs/REQUIREMENTS.tsv" 2> /dev/null | cut -f1 > "$work/base.ids" ||
  refuse "requirements-index-unavailable at $base"

# new_rows prints `ID<TAB>FILE<TAB>TITLE` for each row of a REQUIREMENTS.tsv on stdin whose ID the
# base index lacks.
new_rows() { awk -F'\t' 'NR == FNR { known[$1] = 1; next } FNR > 1 && !($1 in known) { print $1 "\t" $2 "\t" $3 }' "$work/base.ids" -; }

# Open refs: every local branch tip and every worktree HEAD, deduplicated by commit, except this
# checkout's HEAD, commits already in the base, and commits with no history shared with it.
self=$(git rev-parse -q --verify HEAD 2> /dev/null || true)
{
  git for-each-ref --format='%(objectname) %(refname:short)' refs/heads
  git worktree list --porcelain | awk '/^worktree / { path = substr($0, 10) } /^HEAD / { print $2 " " path }'
} | awk '!seen[$1]++' > "$work/refs"
: > "$work/open"
open=0
while read -r sha name; do
  [[ $sha == "$self" ]] && continue
  git merge-base --is-ancestor "$sha" "$base_sha" 2> /dev/null && continue
  git merge-base "$sha" "$base_sha" > /dev/null 2>&1 || continue
  open=$((open + 1))
  git show "$sha:docs/specs/REQUIREMENTS.tsv" 2> /dev/null | new_rows |
    awk -F'\t' -v ref="$name (${sha:0:12})" '{ print $0 "\t" ref }' >> "$work/open"
done < "$work/refs"

if [[ -n $block ]]; then
  used=$(awk -F'\t' -v prefix="$prefix-" -v lo="$lo" -v hi="$hi" '
    index($1, prefix) == 1 {
      number = substr($1, length(prefix) + 1)
      if (number ~ /^[0-9][0-9][0-9]$/ && number + 0 >= lo && number + 0 <= hi) print "used " $1 " " $4 " " $2
    }' "$work/open" | sort -u)
  range=$(printf '%s-%03d..%s-%03d' "$prefix" "$lo" "$prefix" "$hi")
  if [[ -n $used ]]; then
    printf '%s\n' "$used"
    printf '%s: %s is partly defined on open refs not yet in %s; pick a free block\n' "$me" "$range" "$base" >&2
    exit 1
  fi
  printf 'free %s on %d open refs not yet in %s\n' "$range" "$open" "$base"
  exit 0
fi

script/gen-spec-requirements.sh > "$work/here.tsv" || { printf '%s: REFUSE generator-failed: fix the refusal above, then rerun\n' "$me" >&2; exit 2; }
new_rows < "$work/here.tsv" > "$work/here"
collisions=$(awk -F'\t' '
  NR == FNR { file[$1] = $2; title[$1] = $3; next }
  ($1 in file) && (file[$1] != $2 || title[$1] != $3) {
    printf "collision %s here %s \"%s\" vs %s %s \"%s\"\n", $1, file[$1], title[$1], $4, $2, $3
  }' "$work/here" "$work/open" | sort -u)
if [[ -n $collisions ]]; then
  printf '%s\n' "$collisions"
  printf '%s: renumber this checkout'"'"'s IDs (make requirement-ids-free PREFIX=... RANGE=... names a free block), or agree which side keeps each ID, before either merges (ASD-V0-950)\n' "$me" >&2
  exit 1
fi
printf 'no-collision %d new IDs here, %d open refs not yet in %s\n' "$(wc -l < "$work/here" | tr -d ' ')" "$open" "$base"
