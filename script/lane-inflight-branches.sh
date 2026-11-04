#!/usr/bin/env bash
# lane-inflight-branches.sh: make unmerged batch branches visible to remote lane hosts (ASD-V0-770,
# V1-3867). A host's checkout fetched only origin, so a lane there could not see a sealed lane's
# unmerged branch on another host and rebuilt tickets already delivered in flight. The orchestrator
# publishes the in-flight branches to each host as refs/remotes/inflight/BRANCH; screening
# (`git log --all --grep=ID`) then finds them.
#
#   script/lane-inflight-branches.sh publish --remote URL BRANCH...
#   script/lane-inflight-branches.sh check [--repo DIR] BRANCH...
#
# publish pushes each local refs/heads/BRANCH to URL as refs/remotes/inflight/BRANCH, never forced
# (a non-fast-forward is refused by git and reported). check resolves each BRANCH in DIR as
# refs/heads/BRANCH, refs/remotes/inflight/BRANCH or refs/remotes/origin/BRANCH and prints
# `VISIBLE BRANCH REF SHA` or `MISSING BRANCH; run: ...publish --remote <this host> BRANCH`.
# Exit 0 when every branch is published or visible, 1 otherwise, 2 on a usage error.
set -uo pipefail
export LC_ALL=C
usage() { printf 'usage: script/lane-inflight-branches.sh publish --remote URL BRANCH... | check [--repo DIR] BRANCH...\n' >&2; exit 2; }
g() { env -u GIT_CONFIG_GLOBAL git "$@"; }
(($#)) || usage
mode=$1; shift
remote='' repo=.
while (($#)); do
  case $1 in
    --remote) [[ $mode == publish && $# -ge 2 && -n $2 ]] || usage; remote=$2; shift 2 ;;
    --repo) [[ $mode == check && $# -ge 2 && -n $2 ]] || usage; repo=$2; shift 2 ;;
    -*) usage ;;
    *) break ;;
  esac
done
(($#)) || usage
rc=0
case $mode in
  publish)
    [[ -n $remote ]] || usage
    for b in "$@"; do
      g check-ref-format "refs/heads/$b" || { printf 'REFUSED %s: not a branch name\n' "$b"; rc=1; continue; }
      if g push -q "$remote" "refs/heads/$b:refs/remotes/inflight/$b" < /dev/null; then
        printf 'PUBLISHED %s %s\n' "$b" "$(g rev-parse "refs/heads/$b")"
      else
        printf 'FAILED %s: push to %s refused; rebase is not forced, publish the new head under a new name\n' "$b" "$remote"; rc=1
      fi
    done ;;
  check)
    for b in "$@"; do
      found=''
      for r in "refs/heads/$b" "refs/remotes/inflight/$b" "refs/remotes/origin/$b"; do
        sha=$(g -C "$repo" rev-parse --verify -q "$r^{commit}" 2> /dev/null) && { found=$r; break; }
      done
      if [[ -n $found ]]; then printf 'VISIBLE %s %s %s\n' "$b" "$found" "$sha"; else
        printf 'MISSING %s; run on the orchestrator: script/lane-inflight-branches.sh publish --remote <this host repo> %s\n' "$b" "$b"; rc=1
      fi
    done ;;
  *) usage ;;
esac
exit "$rc"
