#!/bin/sh
# Select the published lifecycle predecessor; stdout is its tag, stderr its Git identity.
# An explicit preserved tag need not be an ancestor. The operator owns that predecessor choice.
set -eu
export LC_ALL=C
refuse() { printf 'previous-release: %s\n' "$*" >&2; exit 2; }
[ "$#" -eq 1 ] || [ "$#" -eq 3 ] || refuse 'usage: select-previous-release.sh CANDIDATE [TAG DIRECT_OID]'
candidate=$1
tag=${2-}
expected=${3-}
root=$(pwd -P)
git_read() {
  env -i PATH="$PATH" LC_ALL=C GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
    GIT_CONFIG_SYSTEM=/dev/null GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/dev/null \
    GIT_NO_LAZY_FETCH=1 GIT_OPTIONAL_LOCKS=0 GIT_TERMINAL_PROMPT=0 \
    git -C "$root" -c advice.graftFileDeprecated=false -c core.fsmonitor=false "$@"
}
full_oid() {
  [ "${#1}" -eq 40 ] || return 1
  case $1 in *[!0-9a-f]*) return 1 ;; esac
}
valid_tag() {
  case $1 in v[0-9]*) ;; *) return 1 ;; esac
  case $1 in *[!0-9A-Za-z.+_-]*) return 1 ;; esac
  git_read check-ref-format "refs/tags/$1"
}
full_oid "$candidate" || refuse 'candidate must be a full lowercase 40-hex commit OID'
kind=$(git_read cat-file -t "$candidate") || refuse 'candidate object unavailable'
[ "$kind" = commit ] || refuse 'candidate must name a commit object directly'
if [ -n "$tag" ] || [ -n "$expected" ]; then
  [ -n "$tag" ] && [ -n "$expected" ] || refuse 'explicit tag and direct OID must be supplied together'
  full_oid "$expected" || refuse 'direct OID must be full lowercase 40-hex'
  mode=explicit
else
  mode=reachable
  # Capture Git status before choosing the first row; never hide errors behind a pipeline.
  choices=$(git_read tag --merged "$candidate" --no-contains "$candidate" --list 'v[0-9]*' --sort=-v:refname) || refuse 'reachable tag enumeration failed'
  [ -n "$choices" ] || refuse 'no reachable previous release; supply an explicit tag and direct OID'
  # Validate the highest-ranked row, never silently fall back after a malformed selection.
  tag=$(printf '%s\n' "$choices" | sed -n '1p')
fi
valid_tag "$tag" || refuse 'invalid release tag spelling'
direct=$(git_read rev-parse --verify "refs/tags/$tag") || refuse 'exact tag ref is missing or unreadable'
full_oid "$direct" || refuse 'tag ref did not resolve to a full object OID'
if [ "$mode" = explicit ]; then
  [ "$direct" = "$expected" ] || refuse 'tag direct OID differs from the supplied identity'
fi
peeled=$(git_read rev-parse --verify "refs/tags/$tag^{commit}") || refuse 'release tag must peel to a commit'
full_oid "$peeled" || refuse 'tag commit did not resolve to a full object OID'
[ "$peeled" != "$candidate" ] || refuse 'release tag names the candidate itself'
if git_read merge-base --is-ancestor "$candidate" "$peeled"; then
  refuse 'release tag descends from the candidate'
else
  code=$?
  [ "$code" -eq 1 ] || refuse "ancestry check failed (exit $code)"
fi
printf 'previous-release: mode=%s candidate=%s tag=%s direct=%s commit=%s\n' \
  "$mode" "$candidate" "$tag" "$direct" "$peeled" >&2
printf '%s\n' "$tag"
