#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/requirement-id-reservations.sh reserve PREFIX [LO HI] OWNER [--base REF]
#        script/requirement-id-reservations.sh release OWNER [PREFIX [LO HI]]
#        script/requirement-id-reservations.sh check PREFIX [LO HI] [--owner OWNER]
#        script/requirement-id-reservations.sh next PREFIX SIZE [--base REF] [--owner OWNER]
#        script/requirement-id-reservations.sh list
#
# The requirement-ID reservation ledger (LVT-V0-003, LVT-V0-004; V1-4046). An orchestrator that
# hands a lane a block (PREFIX LO HI, LO and HI 0..9999) or a whole new prefix (PREFIX alone)
# reserves it here first, so a second charter cannot hand out the same IDs before the first lane
# commits a spec row that `make requirement-ids-free` could see.
#
# The ledger is one private file shared by every worktree of the checkout:
# $(git rev-parse --git-common-dir)/corvint/requirement-id-reservations.tsv, or
# REQUIREMENT_ID_RESERVATIONS when set (one file several clones on a host can share). It holds a
# `# requirement-id-reservations/1` header and one `PREFIX<TAB>LO<TAB>HI<TAB>OWNER<TAB>UTC` row per
# reservation; LO and HI are `-` for a whole prefix. Two reservations overlap when they name the same
# prefix and their ranges intersect, or when either is a whole prefix of the other's family (the
# prefix up to a trailing -V<n>).
#
#   reserve  refuses `reserved ...` (exit 1) when another owner's reservation overlaps, and refuses
#            (exit 1, the checks' own `used` lines) unless `script/requirement-ids-free.sh` finds the
#            IDs unused at REF (default origin/main) and `script/requirement-id-collisions.sh` finds
#            them undefined on every open ref; then appends the row under the ledger lock and prints
#            `reserved-ok ...`. The same owner reserving the same IDs again prints `already-reserved`.
#   release  removes OWNER's rows (only those naming PREFIX [LO HI] when given), printing
#            `released N`.
#   check    prints `reserved PREFIX-LO..PREFIX-HI OWNER UTC` for each reservation of another owner
#            (any owner without --owner) overlapping the IDs and exits 1; otherwise one `unreserved`
#            line and exit 0. An absent ledger holds no reservations. It writes nothing.
#   list     prints the rows.
#
# A malformed argument, row or ledger, or a held lock, exits 2 naming it and its fix. reserve
# and release write only the ledger and its lock directory.
set -uo pipefail
export LC_ALL=C

me=requirement-id-reservations
header='# requirement-id-reservations/1'
usage='usage: script/requirement-id-reservations.sh reserve PREFIX [LO HI] OWNER [--base REF] | release OWNER [PREFIX [LO HI]] | check PREFIX [LO HI] [--owner OWNER] | next PREFIX SIZE [--base REF] [--owner OWNER] | list'
refuse() { printf '%s: REFUSE %s\n%s\n' "$me" "$1" "$usage" >&2; exit 2; }

if [[ ${1:-} == --help ]]; then
  printf '%s\n' "$usage" \
    'Next finds the lowest free SIZE-ID block (SIZE 1..10000), without reserving it.' \
    'Reserve requires PREFIX and OWNER; optional LO HI select a block (0..9999).' \
    'The ledger is shared across linked worktrees; REQUIREMENT_ID_RESERVATIONS overrides its path.' \
    'Example: script/requirement-id-reservations.sh reserve AFP-V0 2600 2619 lane-a' \
    'Example: make requirement-ids-release OWNER=lane-a'
  exit 0
fi

here=$(cd "$(dirname "$0")" && pwd) || exit 2
top=$(git rev-parse --show-toplevel 2> /dev/null) && cd "$top" || refuse "not-a-checkout $PWD: run it inside a Git checkout"
if [[ -n ${REQUIREMENT_ID_RESERVATIONS-} ]]; then
  ledger=$REQUIREMENT_ID_RESERVATIONS
else
  common=$(git rev-parse --path-format=absolute --git-common-dir 2> /dev/null) || refuse "git-dir-unavailable: run it inside a Git checkout"
  ledger=$common/corvint/requirement-id-reservations.tsv
fi
[[ $ledger == /* ]] || ledger=$top/$ledger

valid_prefix() { [[ $1 =~ ^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$ ]] || refuse "bad-prefix $1: want upper-case letters and digits joined by -, such as DCW-V0"; }
valid_owner() { [[ $1 =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || refuse "bad-owner $1: want a lane name of letters, digits, '.', '_' or '-' (at most 64)"; }
valid_range() {
  [[ $1 =~ ^[0-9]{1,4}$ && $2 =~ ^[0-9]{1,4}$ ]] && ((10#$1 <= 10#$2)) || refuse "bad-range $1..$2: want LO and HI of 1 to 4 digits with LO <= HI"
}
# spell PREFIX LO HI: the human form of one reservation.
spell() { if [[ $2 == - ]]; then printf 'prefix %s' "$1"; else printf '%s-%03d..%s-%03d' "$1" "$((10#$2))" "$1" "$((10#$3))"; fi; }

# rows prints the ledger's rows, refusing a malformed one; an absent ledger has none.
rows() {
  [[ -e $ledger || -L $ledger ]] || return 0
  [[ ! -L $ledger ]] || refuse "ledger-unreadable $ledger: use a regular ledger in place of the symlink"
  [[ -f $ledger && -r $ledger ]] || refuse "ledger-unreadable $ledger: use a readable regular file or move it aside"
  awk -F'\t' -v header="$header" -v path="$ledger" '
    NR == 1 { if ($0 != header) { printf "line 1 is not %s\n", header > "/dev/stderr"; bad = 1; exit } next }
    $0 == "" { next }
    NF != 5 || length($4) > 64 || $5 !~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z$/ || $1 !~ /^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$/ || $4 !~ /^[A-Za-z0-9][A-Za-z0-9._-]*$/ ||
      !(($2 == "-" && $3 == "-") || ($2 ~ /^(0|[1-9][0-9]*)$/ && $3 ~ /^(0|[1-9][0-9]*)$/ && $2 + 0 <= $3 + 0 && $3 + 0 <= 9999)) {
      printf "line %d is malformed: %s\n", NR, $0 > "/dev/stderr"; bad = 1; exit
    }
    { print }
    END { if (NR == 0) { print "empty ledger" > "/dev/stderr"; bad = 1 } exit bad }' "$ledger" || refuse "ledger-malformed $ledger: fix: repair or remove the named line (each row is PREFIX, LO, HI, OWNER, UTC separated by tabs)"
}

# overlapping PREFIX LO HI EXCLUDE_OWNER: the rows of other owners that overlap the IDs.
overlapping() {
  rows | awk -F'\t' -v prefix="$1" -v lo="$2" -v hi="$3" -v owner="$4" '
    function stem(p) { sub(/-V[0-9]+$/, "", p); return p }
    $4 == owner { next }
    $2 == "-" || lo == "-" { if (stem($1) == stem(prefix)) print; next }
    $1 == prefix && $2 + 0 <= hi + 0 && lo + 0 <= $3 + 0 { print }'
}

lock=$ledger.lock
locked=
unlock() { [[ -z $locked ]] || rmdir "$lock" 2> /dev/null; }
take_lock() {
  mkdir -p "${ledger%/*}" || refuse "ledger-dir-unwritable ${ledger%/*}: set REQUIREMENT_ID_RESERVATIONS to a writable file"
  if mkdir "$lock" 2> /dev/null; then
    locked=1
    trap unlock EXIT
    trap 'unlock; exit 129' HUP
    trap 'unlock; exit 130' INT
    trap 'unlock; exit 143' TERM
    return 0
  fi
  refuse "ledger-locked $lock: another reserve or release holds it; when no such command runs, remove the directory (rmdir $lock) and rerun"
}

# write_rows FILE: replaces the ledger with the header and FILE's rows, atomically.
write_rows() {
  local tmp
  tmp=$(mktemp "$ledger.XXXXXX") || refuse "ledger-unwritable $ledger: set REQUIREMENT_ID_RESERVATIONS to a writable file"
  { printf '%s\n' "$header"; cat "$1"; } > "$tmp" && mv -f "$tmp" "$ledger" || {
    /bin/rm -f "$tmp"
    refuse "ledger-unwritable $ledger: set REQUIREMENT_ID_RESERVATIONS to a writable file"
  }
}

(($# >= 1)) || refuse bad-arguments
command=$1
shift
case $command in
  list)
    (($# == 0)) || refuse bad-arguments
    rows
    ;;
  check)
    owner=
    args=()
    while (($#)); do
      case $1 in
        --owner) (($# >= 2)) || refuse missing-owner; owner=$2; valid_owner "$owner"; shift 2 ;;
        *) args+=("$1"); shift ;;
      esac
    done
    case ${#args[@]} in
      1) prefix=${args[0]} lo=- hi=- ;;
      3) prefix=${args[0]} lo=${args[1]} hi=${args[2]}; valid_range "$lo" "$hi" ;;
      *) refuse bad-arguments ;;
    esac
    valid_prefix "$prefix"
    found=$(overlapping "$prefix" "$lo" "$hi" "$owner") || exit 2
    if [[ -n $found ]]; then
      while IFS=$'\t' read -r p a b o when; do
        printf 'reserved %s %s %s\n' "$(spell "$p" "$a" "$b")" "$o" "$when"
      done <<< "$found"
      printf '%s: %s overlaps a reservation in %s; pick a free block, or pass OWNER=<lane> (--owner) when the reservation is your own\n' "$me" "$(spell "$prefix" "$lo" "$hi")" "$ledger" >&2
      exit 1
    fi
    printf 'unreserved %s in %s\n' "$(spell "$prefix" "$lo" "$hi")" "$ledger"
    ;;
  next)
    (($# >= 2)) || refuse bad-arguments
    prefix=$1 size=$2 base=origin/main owner=
    shift 2
    valid_prefix "$prefix"
    [[ $size =~ ^[0-9]{1,5}$ ]] && ((10#$size >= 1 && 10#$size <= 10000)) ||
      refuse "bad-size $size: want SIZE in 1..10000"
    size=$((10#$size))
    while (($#)); do
      case $1 in
        --base) (($# >= 2)) || refuse missing-base; base=$2; shift 2 ;;
        --owner) (($# >= 2)) || refuse missing-owner; owner=$2; valid_owner "$owner"; shift 2 ;;
        *) refuse "bad-argument $1: use next PREFIX SIZE [--base REF] [--owner OWNER]" ;;
      esac
    done
    sha=$(git rev-parse --verify -q --end-of-options "$base^{commit}") ||
      refuse "missing-base $base: use a commit available in this checkout"
    # Full-range queries enumerate each occupied token/definition in two bounded scans.
    # Neither an unavailable Git read nor an invalid ledger can become a free answer.
    status=0
    used_base=$("$here/requirement-ids-free.sh" "$prefix" 0 9999 "$sha") || status=$?
    ((status <= 1)) || exit "$status"
    status=0
    used_refs=$("$here/requirement-id-collisions.sh" --block "$prefix" 0 9999 --base "$sha") || status=$?
    ((status <= 1)) || exit "$status"
    reservations=$(rows) || exit 2
    found=$(printf '%s\n' "$used_base" "$used_refs" | awk -v prefix="$prefix" -v size="$size" -v owner="$owner" -v ledger="$reservations" '
      function stem(p) { sub(/-V[0-9]+$/, "", p); return p }
      BEGIN {
        n = split(ledger, lines, "\n")
        for (i = 1; i <= n; i++) {
          split(lines[i], row, "\t")
          if (row[4] == owner) continue
          if (row[2] == "-" && stem(row[1]) == stem(prefix)) {
            for (j = 0; j <= 9999; j++) taken[j] = 1
          } else if (row[1] == prefix) {
            for (j = row[2] + 0; j <= row[3] + 0; j++) taken[j] = 1
          }
        }
      }
      $1 == "used" { number = $2; sub("^" prefix "-", "", number); taken[number + 0] = 1 }
      END {
        for (start = 0; start + size <= 10000; start++) {
          for (j = start; j < start + size; j++) if (j in taken) break
          if (j == start + size) { print start, start + size - 1; exit }
          start = j
        }
      }') || exit 2
    if [[ -z $found ]]; then
      printf 'no-free-block %s size=%d at %s\n' "$prefix" "$size" "$sha"
      exit 1
    fi
    read -r lo hi <<< "$found"
    printf 'next-free %s at %s; reserve it before assigning it\n' "$(spell "$prefix" "$lo" "$hi")" "$sha"
    ;;
  reserve)
    base=origin/main
    args=()
    while (($#)); do
      case $1 in
        --base) (($# >= 2)) || refuse missing-base; base=$2; shift 2 ;;
        *) args+=("$1"); shift ;;
      esac
    done
    case ${#args[@]} in
      2) prefix=${args[0]} lo=- hi=- owner=${args[1]} ;;
      4) prefix=${args[0]} lo=${args[1]} hi=${args[2]} owner=${args[3]}; valid_range "$lo" "$hi" ;;
      *) refuse bad-arguments ;;
    esac
    valid_prefix "$prefix"
    valid_owner "$owner"
    [[ $lo == - ]] || lo=$((10#$lo)) hi=$((10#$hi))
    take_lock
    found=$(overlapping "$prefix" "$lo" "$hi" "$owner") || exit 2
    if [[ -n $found ]]; then
      while IFS=$'\t' read -r p a b o when; do
        printf 'reserved %s %s %s\n' "$(spell "$p" "$a" "$b")" "$o" "$when"
      done <<< "$found"
      printf '%s: %s is already reserved by another lane; pick a free block\n' "$me" "$(spell "$prefix" "$lo" "$hi")" >&2
      exit 1
    fi
    if rows | awk -F'\t' -v p="$prefix" -v a="$lo" -v b="$hi" -v o="$owner" '$1 == p && $2 == a && $3 == b && $4 == o { found = 1 } END { exit !found }'; then
      printf 'already-reserved %s %s in %s\n' "$(spell "$prefix" "$lo" "$hi")" "$owner" "$ledger"
      exit 0
    fi
    # Reserving IDs that are already in use would hand them out twice: the tree and open-ref checks
    # run under the lock, so no other reserve slips in between.
    if [[ $lo == - ]]; then
      "$here/requirement-ids-free.sh" "$prefix" "$base" || exit $?
      "$here/requirement-id-collisions.sh" --family "$prefix" --base "$base" || exit $?
    else
      "$here/requirement-ids-free.sh" "$prefix" "$lo" "$hi" "$base" || exit $?
      "$here/requirement-id-collisions.sh" --block "$prefix" "$lo" "$hi" --base "$base" || exit $?
    fi
    work=$(mktemp "${TMPDIR:-/tmp}/$me.XXXXXX") || refuse "scratch-unavailable: set TMPDIR to a writable directory"
    trap '/bin/rm -f "$work"; unlock' EXIT
    { rows || exit 2; printf '%s\t%s\t%s\t%s\t%s\n' "$prefix" "$lo" "$hi" "$owner" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; } > "$work" || exit 2
    write_rows "$work"
    printf 'reserved-ok %s %s in %s\n' "$(spell "$prefix" "$lo" "$hi")" "$owner" "$ledger"
    ;;
  release)
    case $# in
      1) owner=$1 prefix= lo= hi= ;;
      2) owner=$1 prefix=$2 lo=- hi=- ;;
      4) owner=$1 prefix=$2 lo=$3 hi=$4; valid_range "$lo" "$hi"; lo=$((10#$lo)) hi=$((10#$hi)) ;;
      *) refuse bad-arguments ;;
    esac
    valid_owner "$owner"
    [[ -z $prefix ]] || valid_prefix "$prefix"
    take_lock
    work=$(mktemp "${TMPDIR:-/tmp}/$me.XXXXXX") || refuse "scratch-unavailable: set TMPDIR to a writable directory"
    trap '/bin/rm -f "$work"; unlock' EXIT
    all=$(rows) || exit 2
    keep=$(awk -F'\t' -v o="$owner" -v p="$prefix" -v a="$lo" -v b="$hi" '
      $4 == o && (p == "" || ($1 == p && $2 == a && $3 == b)) { next } { print }' <<< "$all")
    released=$(($(grep -c . <<< "$all") - $(grep -c . <<< "$keep")))
    [[ -z $keep ]] || printf '%s\n' "$keep" > "$work"
    if ((released > 0)); then
      write_rows "$work"
    fi
    printf 'released %d\n' "$released"
    ;;
  *) refuse "unknown-command $command: use list, check, next, reserve or release (--help)" ;;
esac
