#!/usr/bin/env bash
# usage: batch-review-packet.sh BASE [HEAD]
# ASD-V0-043 (V1-2986): the bounded, read-only input for the batch reviewer of a merged range
# BASE..HEAD (docs/LANE-ORCHESTRATION.md section 8). It prints, in order:
#   - the reviewed revision: BASE, HEAD, HEAD's tree, the commit and first-parent merge counts and
#     the corvint version that produced the sections below;
#   - lane overlap: every path changed by more than one first-parent merge of the range, with the
#     merges that changed it (`git diff --no-renames --name-only M^1 M` per merge);
#   - `corvint impact --base BASE` for the range;
#   - `corvint dogfood batch-check BASE`: each constituent sealed CEM's report and its unresolved
#     obligations, or the refusal that names the missing seal.
# HEAD must be the checked-out commit, because impact and batch-check read the checkout's HEAD;
# nothing is checked out, fetched, indexed or written. Each section keeps its first
# BATCH_REVIEW_LINES lines (default 200) and names how many it elided; each corvint command runs
# under BATCH_REVIEW_TIMEOUT seconds (default 300) and its exit status is printed, so a refusal,
# timeout or missing corvint is visible in the packet rather than an empty section. CORVINT_BIN
# names the corvint executable (default: corvint from PATH).
set -uo pipefail
export LC_ALL=C

refuse() {
  printf 'batch-review-packet: REFUSE %s\n' "$*" >&2
  exit 2
}
lines=${BATCH_REVIEW_LINES:-200}
limit=${BATCH_REVIEW_TIMEOUT:-300}
corvint=${CORVINT_BIN:-corvint}
[[ $lines =~ ^[1-9][0-9]{0,4}$ ]] || refuse "invalid-lines BATCH_REVIEW_LINES=$lines (want 1..99999)"
[[ $limit =~ ^[1-9][0-9]{0,3}$ ]] || refuse "invalid-timeout BATCH_REVIEW_TIMEOUT=$limit (want 1..9999 seconds)"
[[ $# == 1 || $# == 2 ]] || refuse "usage: batch-review-packet.sh BASE [HEAD]"
top=$(git rev-parse --show-toplevel 2> /dev/null) || refuse "not-a-git-checkout $PWD"
cd "$top" || exit 2
"$(dirname "$0")/require-full-history.sh" batch-review-packet "$top" || exit 2
base=$(git rev-parse --verify -q "$1^{commit}") || refuse "unknown-base $1"
head=$(git rev-parse --verify -q "${2:-HEAD}^{commit}") || refuse "unknown-head ${2:-HEAD}"
checkout=$(git rev-parse --verify -q 'HEAD^{commit}') || refuse "no-checked-out-head"
[[ $head == "$checkout" ]] || refuse "head-not-checked-out head=$head checkout=$checkout (impact and batch-check read the checked-out HEAD)"
git merge-base --is-ancestor "$base" "$head" || refuse "base-not-ancestor base=$base head=$head"
[[ $corvint == */* ]] || corvint=$(type -P -- "$corvint") || corvint=

# bound: the first $lines lines of stdin, then how many were elided.
bound() {
  awk -v n="$lines" 'NR <= n { print; next }
    END { if (NR > n) printf "... %d more lines elided (BATCH_REVIEW_LINES=%d)\n", NR - n, n }'
}
# section TITLE COMMAND...: a corvint section with its bounded output and exit status.
section() {
  local rc
  printf '\n## %s\n' "$1"
  shift
  if [[ -z $corvint ]]; then
    printf 'NOT_PRODUCED corvint-unavailable (set CORVINT_BIN)\n'
    return
  fi
  perl -e 'alarm shift; exec @ARGV or exit 127' "$limit" "$corvint" "$@" < /dev/null 2>&1 | bound
  rc=${PIPESTATUS[0]}
  case $rc in
    0) printf 'exit=0\n' ;;
    142) printf 'exit=142 NOT_PRODUCED timeout after %ss (BATCH_REVIEW_TIMEOUT)\n' "$limit" ;;
    *) printf 'exit=%s\n' "$rc" ;;
  esac
}

merges=$(git rev-list --first-parent --merges "$base..$head")
version=NOT_PRODUCED
[[ -n $corvint ]] && version=$("$corvint" --version 2> /dev/null | head -n 1)
printf '# Batch review packet (ASD-V0-043)\n'
printf 'reviewed: base=%s head=%s tree=%s\n' "$base" "$head" "$(git rev-parse "$head^{tree}")"
printf 'commits=%s first-parent-merges=%s corvint=%s\n' "$(git rev-list --count "$base..$head")" \
  "$(grep -c . <<< "$merges")" "${version:-NOT_PRODUCED}"

printf '\n## Lane overlap: paths changed by more than one first-parent merge\n'
while read -r m; do
  [[ -n $m ]] || continue
  git diff --no-renames --name-only "$m^1" "$m" | sed "s/\$/	${m:0:12}/"
done <<< "$merges" | awk -F '\t' '{ n[$1]++; by[$1] = by[$1] " " $2 } END { for (p in n) if (n[p] > 1) printf "%s merges=%d%s\n", p, n[p], by[p] }' |
  sort | bound | grep . || printf 'none\n'

section "corvint impact --base $base" --root "$top" impact --base "$base"
section "corvint dogfood batch-check $base (constituent CEM reports and unresolved obligations)" \
  --root "$top" dogfood batch-check --verifier "$corvint" "$base"
