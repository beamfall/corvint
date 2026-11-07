#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/dogfood-retire-traces.sh
#
# The one rebind step after a lane reset its own unpushed bind or seal commit (DCW-V0-2142,
# V1-4442): it quarantines, byte-preserved, every worktree-local trace that names a commit HEAD no
# longer reaches, which `make dogfood-inputs-check` reports as `NOTE stranded trace <sha>`. It runs
# `corvint migrate-traces --dry-run` with this tree's build (CORVINT_BIN when set) and applies that
# plan with its own digest only when the plan holds stranded traces alone, as dogfood finish-lane
# and dogfood change do before they prepare (DCW-V0-406). A plan that would also rekey a reachable
# legacy trace is refused naming the manual dry run and apply. Exit 0: nothing was stranded, or
# every stranded trace was quarantined; 2: a refusal or a failed step.
set -uo pipefail
export LC_ALL=C
me=dogfood-retire-traces
(($# == 0)) || { printf '%s: REFUSE bad-arguments\nusage: script/dogfood-retire-traces.sh (or make dogfood-retire-traces); it takes no arguments\n' "$me" >&2; exit 2; }
corvint_bin=${CORVINT_BIN-}
# Both invocations run from the repository root, so resolve a PATH-only name and make a relative
# path absolute against the caller's directory first, as dogfood-finish.sh does (DCW-V0-2142).
if [[ -n $corvint_bin ]]; then
  [[ $corvint_bin == */* ]] || corvint_bin=$(type -P -- "$corvint_bin") || {
    printf '%s: REFUSE corvint-bin-not-found: CORVINT_BIN=%s is not an executable on PATH; nothing was moved\n' "$me" "$CORVINT_BIN" >&2
    printf 'fix: set CORVINT_BIN to a path of a current build of this tree, or unset it to build one, then rerun make %s\n' "$me" >&2
    exit 2
  }
  [[ $corvint_bin == /* ]] || corvint_bin=$PWD/$corvint_bin
fi
repo=$(cd "$(dirname "$0")/.." && pwd)
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
if [[ -z $corvint_bin ]]; then
  [[ -n ${CORVINT_GOCACHE:-}${GOCACHE:-}${XDG_CACHE_HOME:-}${HOME:-} ]] ||
    { printf '%s: REFUSE home-unset: set HOME and rerun\n' "$me" >&2; exit 2; }
  go_cache=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}
  # shellcheck source=script/dogfood-build-corvint.sh
  source "$repo/script/dogfood-build-corvint.sh" || exit 2
  mkdir -p "$git_dir/corvint" || exit 2
  corvint_bin=$git_dir/corvint/corvint
  dogfood_build_tree_corvint "$corvint_bin" || {
    printf '%s: REFUSE current-tree-corvint-build-failed\n' "$me" >&2
    dogfood_build_fix "$me" tree
    exit 2
  }
fi
plan=$(cd "$repo" && "$corvint_bin" migrate-traces --dry-run) || {
  printf '%s: REFUSE dry-run-failed: %s migrate-traces --dry-run exited nonzero; nothing was moved\n%s\n' "$me" "$corvint_bin" "$plan" >&2
  printf 'fix: repair the trace store that dry run names, then rerun make %s\n' "$me" >&2
  exit 2
}
# One field per line: the digest, the stranded count and revisions, and the legacy entry count.
fields=$(perl -MJSON::PP -e '
  my $p = eval { JSON::PP->new->decode(join "", <STDIN>) } or exit 2;
  ref $p->{stranded_revisions} eq "ARRAY" && ref $p->{entries} eq "ARRAY" && ($p->{plan_digest} // "") =~ /^[0-9a-f]{64}$/ or exit 2;
  print join("\n", $p->{plan_digest}, scalar @{$p->{entries}}, @{$p->{stranded_revisions}}), "\n";' <<< "$plan") || {
  printf '%s: REFUSE dry-run-unreadable: %s migrate-traces --dry-run printed no plan_digest, entries and stranded_revisions; nothing was moved\n' "$me" "$corvint_bin" >&2
  printf 'fix: run corvint migrate-traces --dry-run with a current build of this tree (set CORVINT_BIN to one), then rerun make %s\n' "$me" >&2
  exit 2
}
digest=$(sed -n 1p <<< "$fields")
legacy=$(sed -n 2p <<< "$fields")
stranded=$(sed -n '3,$p' <<< "$fields")
if [[ -z $stranded ]]; then
  printf '%s: NOTHING stranded: every worktree-local trace names a commit HEAD reaches; nothing was moved\n' "$me"
  exit 0
fi
if ((legacy > 0)); then
  printf '%s: REFUSE plan-rekeys-legacy-traces stranded=%d legacy=%d: the migration plan also rekeys reachable tree-keyed legacy traces, so it is not applied automatically (DCW-V0-406); nothing was moved\n' \
    "$me" "$(wc -l <<< "$stranded" | tr -d ' ')" "$legacy" >&2
  printf 'fix: inspect corvint migrate-traces --dry-run, then corvint migrate-traces --apply --plan-digest %s\n' "$digest" >&2
  exit 2
fi
(cd "$repo" && "$corvint_bin" migrate-traces --apply --plan-digest "$digest" > /dev/null) || {
  printf '%s: REFUSE apply-failed: migrate-traces --apply --plan-digest %s exited nonzero (the store changed since the dry run, or the apply failed)\n' "$me" "$digest" >&2
  printf 'fix: rerun make %s, which takes a fresh dry run\n' "$me" >&2
  exit 2
}
while IFS= read -r revision; do
  printf '%s: RETIRE stale trace %s quarantined byte-preserved to .context-corvint/legacy-traces/ by migrate-traces --apply --plan-digest %s (DCW-V0-2142, LTPM-V0-012)\n' "$me" "$revision" "$digest"
done <<< "$stranded"
printf '%s: rerun the dogfood command you ran (make dogfood-inputs-check or make dogfood-change) with the same inputs\n' "$me"
