#!/usr/bin/env bash
# usage: dogfood-change-runs.sh [SINCE]
# ASD-V0-955 (V1-1629): per-lane dogfood-change run counts from the repository-wide ledger that
# script/dogfood-change.sh appends to, which survives git worktree remove. SINCE is a UTC time
# (YYYY-MM-DD or YYYY-MM-DDTHH:MM:SSZ); rows before it are left out. Reads only; exits 2 on a bad SINCE.
set -uo pipefail
export LC_ALL=C

since=${1-}
if [[ -n $since && ! $since =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}(T[0-9]{2}:[0-9]{2}:[0-9]{2}Z)?$ ]]; then
  printf 'dogfood-change-runs: REFUSE bad-since SINCE=%s: pass a UTC time, for example SINCE=2026-11-01 or SINCE=2026-11-01T03:00:00Z\n' "$since" >&2
  exit 2
fi
common=$(git rev-parse --path-format=absolute --git-common-dir) || exit 2
log="$common/corvint/dogfood-change-runs-all.tsv"
[[ -r $log ]] || log_rows=/dev/null
# A five-column row (ASD-V0-955) is one run; its lane is the branch, or the worktree when detached.
# The ISO times compare as strings.
awk -F '\t' -v since="${since:-}" -v path="$log" '
  NF >= 5 && ($1 "") >= since { lane = ($4 == "DETACHED") ? $5 : $4; n[lane]++; total++ }
  END {
    k = 0
    for (lane in n) { k++; c[k] = n[lane]; name[k] = lane }
    for (i = 2; i <= k; i++) for (j = i; j > 1 && name[j - 1] > name[j]; j--) {
      t = name[j]; name[j] = name[j - 1]; name[j - 1] = t; t = c[j]; c[j] = c[j - 1]; c[j - 1] = t
    }
    for (i = 1; i <= k; i++) { printf "run-count %s %d\n", name[i], c[i]; s[i] = c[i] }
    for (i = 2; i <= k; i++) for (j = i; j > 1 && s[j - 1] > s[j]; j--) { t = s[j]; s[j] = s[j - 1]; s[j - 1] = t }
    if (k == 0) median = "NOT_OBSERVED"
    else if (k % 2) median = s[(k + 1) / 2]
    else median = sprintf("%g", (s[k / 2] + s[k / 2 + 1]) / 2)
    printf "dogfood-change-runs: lanes=%d median=%s max=%d total=%d since=%s log=%s\n", k, median, (k ? s[k] : 0), total, (since == "" ? "ALL" : since), path
  }' "${log_rows:-$log}"
