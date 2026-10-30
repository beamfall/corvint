#!/bin/sh
# Resolve the CCF-V1-007 N-1 release for the Release gates workflow (decision 2026-10-30, V1-3177).
# usage: core-n1-source.sh [--replay]
# Reads SHA, PREVIOUS_RELEASE_TAG, PREVIOUS_RELEASE_OID and GITHUB_REPOSITORY; gh must be
# authenticated. N-1 is script/select-previous-release.sh's tag, and it must be a published (not
# draft) release of GITHUB_REPOSITORY. Exit 0 prints that tag; exit 3 is FIRST_RELEASE: no tag
# precedes SHA and no published release of the repository could be its predecessor (stdout empty,
# the reason on stderr and in GITHUB_STEP_SUMMARY when set); exit 2 is any refusal, so a predecessor
# that exists but cannot be selected or verified fails closed. --replay runs
# `make core-n1-replay CORE_N1_TAG=<tag>`, or reports FIRST_RELEASE not applicable and exits 0.
set -euf
export LC_ALL=C GIT_NO_REPLACE_OBJECTS=1
refuse() { printf 'core-n1-source: %s\n' "$*" >&2; exit 2; }
replay=false
case $# in
  0) ;;
  1) [ "$1" = --replay ] || refuse 'usage: core-n1-source.sh [--replay]'; replay=true ;;
  *) refuse 'usage: core-n1-source.sh [--replay]' ;;
esac
repo=${GITHUB_REPOSITORY-}
printf '%s\n' "$repo" | grep -Eqx '[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+' || refuse 'GITHUB_REPOSITORY must be OWNER/NAME'
candidate=${SHA-}
script_dir=$(cd "$(dirname "$0")" && pwd -P)
code=0
tag=$("$script_dir/select-previous-release.sh" "$candidate" "${PREVIOUS_RELEASE_TAG-}" "${PREVIOUS_RELEASE_OID-}") || code=$?
case $code in
  0)
    # A tag alone is not a release: require the published release of that exact tag.
    view=$(gh release view "$tag" --repo "$repo" --json tagName,isDraft --jq '"\(.tagName) \(.isDraft)"') ||
      refuse "N-1 tag $tag has no readable published release in $repo"
    [ "$view" = "$tag false" ] || refuse "N-1 tag $tag is not a published release in $repo ($view)"
    printf 'core-n1-source: N-1 %s is a published release of %s\n' "$tag" "$repo" >&2
    if [ "$replay" = true ]; then exec make core-n1-replay CORE_N1_TAG="$tag"; fi
    printf '%s\n' "$tag"
    exit 0
    ;;
  3) ;;
  *) refuse "previous release selection refused (exit $code)" ;;
esac
# No reachable tag. Every published release must then be the candidate itself or a descendant;
# any other published release is a predecessor this run cannot verify, so it fails closed.
published=$(gh release list --repo "$repo" --exclude-drafts --limit 1000 --json tagName --jq '.[].tagName') ||
  refuse "cannot list published releases of $repo"
for release in $published; do
  peeled=$(git rev-parse -q --verify "refs/tags/$release^{commit}") ||
    refuse "published release $release of $repo has no local tag; supply previous_release_tag and previous_release_oid"
  [ "$peeled" = "$candidate" ] && continue
  status=0
  git merge-base --is-ancestor "$candidate" "$peeled" || status=$?
  case $status in
    0) continue ;;
    1) refuse "published release $release of $repo is not a reachable predecessor of $candidate; supply previous_release_tag and previous_release_oid" ;;
    *) refuse "ancestry check of $release failed (exit $status)" ;;
  esac
done
reason="FIRST_RELEASE: no published release of $repo precedes $candidate; the N-1 checks are not applicable"
printf 'core-n1-source: %s\n' "$reason" >&2
if [ -n "${GITHUB_STEP_SUMMARY-}" ]; then printf -- '- N-1 %s\n' "$reason" >>"$GITHUB_STEP_SUMMARY"; fi
if [ "$replay" = true ]; then
  printf '::notice title=core-n1-replay::%s\n' "$reason"
  printf 'core-n1-replay: NOT_APPLICABLE %s\n' "$reason"
  exit 0
fi
exit 3
