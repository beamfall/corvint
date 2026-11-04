#!/usr/bin/env bash
# usage: script/github-split-check.sh [BASE]
#
# A code-lane range never carries a .github change (ASD-V0-706, V1-3292): a .github path forces
# every PR shard FULL, so it ships on its own side branch and PR. For BASE..HEAD (default BASE
# origin/main; an empty BASE means origin/main, and a checkout where that does not resolve skips,
# saying so) it reads the non-merge commits. A range whose
# commits touch only .github/ paths, or no .github/ path, passes with one `github-split-check: ok`
# line. A range with both prints `refuse github-in-code-range SHA SUBJECT` for each commit that
# touches .github/ and exits 1 with the side-branch recovery. Exit 2 on a usage error, a BASE
# that is not a commit, or a log that cannot be read or classified, including one with fewer or more
# commit headers than `git rev-list --count` reports (never a silent ok). It reads
# Git objects only.
set -uo pipefail
export LC_ALL=C

me=github-split-check
usage='usage: script/github-split-check.sh [BASE]'
refuse() { printf '%s: REFUSE %s\n%s\n' "$me" "$1" "$usage" >&2; exit 2; }
(($# <= 1)) || refuse "bad-arguments (fix: pass at most one BASE)"
base=${1:-}
if [[ -z $base ]]; then
  # The batch default (make batch-precheck): a checkout without origin/main skips, as the trailer check does.
  git rev-parse --verify -q origin/main^{commit} > /dev/null || {
    printf '%s: skipped: no base was given and origin/main does not resolve here; run make batch-precheck DOGFOOD_BASE=<sha> to check the batch range\n' "$me"
    exit 0
  }
  base=origin/main
fi
base_sha=$(git rev-parse --verify -q "$base^{commit}") || refuse "unknown-base $base (fix: git fetch -q origin main, then rerun)"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-github-split.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
# One NUL-delimited walk (-z, so Git never display-quotes a non-ASCII or control-character path):
# a `\036SHA SUBJECT` record per commit, then its paths; the first path after a header carries one
# leading newline.
git log --no-merges -z --format='%x1e%H %s' --name-only "$base_sha..HEAD" > "$tmp/log" || refuse "git-log-failed $base (fix: run it inside the lane checkout)"
parse_failed() { refuse "parse-failed $base (fix: check free space in ${TMPDIR:-/tmp} and that git log -z runs here, then rerun)"; }
hdr='' after=0 marked=0 n_gh=0 n_other=0 n_headers=0
while IFS= read -r -d '' rec || [[ -n $rec ]]; do
  if ((after)); then rec=${rec#$'\n'}; after=0; fi
  if [[ $rec == $'\036'* ]]; then hdr=${rec:1}; after=1; marked=0; n_headers=$((n_headers + 1)); continue; fi
  [[ -n $rec ]] || continue
  [[ -n $hdr ]] || parse_failed
  if [[ $rec == .github/* ]]; then
    ((marked)) && continue
    printf '%s\n' "$hdr" >> "$tmp/gh" || parse_failed
    marked=1 n_gh=$((n_gh + 1))
  else
    n_other=$((n_other + 1))
  fi
done < "$tmp/log"
# Fail closed: every commit in the range must have been read as a header.
n_commits=$(git rev-list --no-merges --count "$base_sha..HEAD") || refuse "git-log-failed $base (fix: run it inside the lane checkout)"
((n_headers == n_commits)) || parse_failed

if ((n_gh == 0 || n_other == 0)); then
  printf '%s: ok github-commits=%s base=%s\n' "$me" "$n_gh" "$base_sha"
  exit 0
fi
while IFS= read -r line; do
  printf 'refuse github-in-code-range %s\n' "$line"
done < "$tmp/gh"
printf '%s: FAIL github-commits=%s base=%s\n  fix: move each named commit to its own side branch (git branch <lane>-github %s, then git cherry-pick SHA there) and rebuild the lane branch without it; the .github change ships as its own PR (docs/LANE-ORCHESTRATION.md section 16)\n' "$me" "$n_gh" "$base_sha" "${base_sha:0:12}"
exit 1
