#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# ACR-V1-001: compare docs/specs/PRODUCT-PLAN-1.0.md with the plan blob the acceptance record
# names. The comparison is over words, so re-wrapping a paragraph is no difference, and it
# ignores three things that change without changing what was accepted:
#   - the acceptance marker of a PROD requirement line, `(proposed YYYY-MM-DD)` or
#     `(accepted, decision NNNN; V1-NNNN)` (owner decision H17, 2026-11-10);
#   - the line numbers of a `path:LINE@hash` citation, which a repin moves while the hash holds;
#   - the record's own path, which changed when the proposed record took its decision number.
# Any other difference prints STALE_PLAN with the first differing words and exits 1. A record
# with no blob, or a blob this clone does not hold, exits 2: the pin is then not verified.
# usage: script/acceptance-plan-pin-check.sh [ROOT [RECORD]]

test $# -le 2 || { printf 'usage: %s [ROOT [RECORD]]\n' "$0" >&2; exit 2; }
root=${1:-$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)}
record=${2:-docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md}
plan=$root/docs/specs/PRODUCT-PLAN-1.0.md
die() { printf 'acceptance-plan-pin-check: %s\n' "$1" >&2; exit 2; }
test -r "$plan" || die "cannot read $plan"
test -r "$root/$record" || die "cannot read $root/$record"
blob=$(tr '\n' ' ' < "$root/$record" | tr -s ' ' \
  | sed -n 's/.*plan file is blob `\([0-9a-f]\{40\}\)`.*/\1/p')
test -n "$blob" || die "$record names no plan blob"
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-plan-pin.XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM
git -C "$root" cat-file blob "$blob" > "$work/pinned" 2> /dev/null \
  || die "blob $blob is not in this clone; the pin is NOT_VERIFIED"

words() {
  RECORD=$record perl -0777 -pe '
    s/\s+/ /g;
    s/(`PROD-[A-Z]+-\d+`: )\((?:proposed \d{4}-\d\d-\d\d|accepted, decision \d{4}; V1-\d+)\) /$1/g;
    s/:\d+(?:-\d+)?(\@[0-9a-f]{8})/:LINE$1/g;
    s{docs/decisions/PROPOSED-accept-product-plan-1\.0\.md}{RECORD}g;
    s{\Q$ENV{RECORD}\E}{RECORD}g;
    s/ /\n/g;' "$1"
}
words "$work/pinned" > "$work/a"
words "$plan" > "$work/b"
if cmp -s "$work/a" "$work/b"; then
  printf 'acceptance-plan-pin-check: ok (plan matches blob %s)\n' "$blob"
  exit 0
fi
printf 'STALE_PLAN\t%s\tblob %s\n' "$record" "$blob"
diff "$work/a" "$work/b" | grep '^[<>]' | head -20 || true
exit 1
