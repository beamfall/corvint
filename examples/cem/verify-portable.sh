#!/usr/bin/env sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Verify a checked-in cem/0.1 sidecar with the portable, digest-pinned verifier
# (interop/cem01-go `ci`), without executing repository or map content.
#
# Required environment:
#   CEM_BASE_SHA        exact 40- or 64-hex base commit
#   CEM_HEAD_SHA        exact 40- or 64-hex PR-head commit
#   CEM_VERIFIER        the fetched verifier executable
#   CEM_VERIFIER_SHA256 pinned lowercase SHA-256 of that executable
# Optional environment:
#   CEM_MAP_PATH        map path in the head tree (default .corvint/change.cem.json)
#   CEM_REPOSITORY      trusted checkout containing the base (default $PWD)
#   CEM_HEAD_REPOSITORY separate checkout containing CEM_HEAD_SHA (optional)
#   CEM_PROFILE         cem/0.1 (default) runs `ci`; cem/1.0 explicitly selects the stable branch
#   CEM_ARTIFACTS       absolute artifact root directory, required with CEM_PROFILE=cem/1.0
#
# Standard output is the verifier's one-line cem-ci-report/0 JSON. The exit status is the
# verifier's: 0 accepted, 1 rejected, 2 operational, 3 missing evidence, 4 unsupported profile,
# 5 repository mismatch. Before the verifier runs, invalid input or a digest mismatch exits 2
# with one stderr line and no report; a mismatched executable is never run. A committed cem/1.0
# map exits 4 with code use-verify-stable-canonical: rerun with CEM_PROFILE=cem/1.0.
#
# Stable branch (CEM_PROFILE=cem/1.0, CEM-V1-PSR-004): the committed map at CEM_HEAD_SHA:CEM_MAP_PATH
# is read as Git object data into a private file and checked by `verify-stable-canonical` against
# CEM_BASE_SHA, CEM_HEAD_SHA and CEM_ARTIFACTS. Standard output is then the verifier's one-line
# cem-stable-verification/1 result and the exit status is its own: 0 accept, 1 reject, 2
# unsupported. An absent map exits 3 and an absent head commit exits 5, each with one stderr line
# and no result. Acceptance is structural and reference integrity only; a completion claim still
# needs the native Tasks gate.

set -eu
umask 077
unset GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_ATTR_SOURCE GIT_CEILING_DIRECTORIES \
  GIT_COMMON_DIR GIT_CONFIG_COUNT GIT_CONFIG_PARAMETERS GIT_DIFF_OPTS GIT_DIR \
  GIT_DISCOVERY_ACROSS_FILESYSTEM GIT_EDITOR GIT_EXEC_PATH GIT_EXTERNAL_DIFF \
  GIT_GLOB_PATHSPECS GIT_ICASE_PATHSPECS GIT_INDEX_FILE GIT_LITERAL_PATHSPECS \
  GIT_NAMESPACE GIT_NOGLOB_PATHSPECS GIT_OBJECT_DIRECTORY GIT_PAGER \
  GIT_PROXY_COMMAND GIT_SEQUENCE_EDITOR GIT_SHALLOW_FILE GIT_SSH GIT_SSH_COMMAND \
  GIT_TRACE GIT_TRACE2 GIT_TRACE2_EVENT GIT_TRACE2_PERF GIT_TRACE_CURL \
  GIT_TRACE_CURL_NO_DATA GIT_TRACE_PACKET GIT_TRACE_PERFORMANCE GIT_TRACE_REDACT \
  GIT_TRACE_SETUP GIT_TRACE_SHALLOW GIT_WORK_TREE
export GIT_NO_REPLACE_OBJECTS=1 GIT_NO_LAZY_FETCH=1 GIT_ATTR_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
export GCM_INTERACTIVE=Never GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null

fail() {
  printf '%s\n' "cem-ci: $1" >&2
  exit 2
}

is_oid() {
  case "$1" in
    ''|*[!0123456789abcdef]*) return 1 ;;
  esac
  length=${#1}
  [ "$length" -eq 40 ] || [ "$length" -eq 64 ]
}

is_digest() {
  case "$1" in
    ''|*[!0123456789abcdef]*) return 1 ;;
  esac
  [ "${#1}" -eq 64 ]
}

is_map_path() {
  case "$1" in
    ''|/*|.|..|../*|*/..|*/../*|*'//'*|*'/./'*|*[!ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._/-]*)
      return 1
      ;;
  esac
  return 0
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum <"$1")
  elif command -v shasum >/dev/null 2>&1; then
    digest=$(shasum -a 256 <"$1")
  else
    return 1
  fi
  printf '%s\n' "${digest%% *}"
}

base_sha=${CEM_BASE_SHA:-}
head_sha=${CEM_HEAD_SHA:-}
map_path=${CEM_MAP_PATH:-.corvint/change.cem.json}
base_repo=${CEM_REPOSITORY:-$PWD}
head_repo=${CEM_HEAD_REPOSITORY:-}
verifier=${CEM_VERIFIER:-}
pinned=${CEM_VERIFIER_SHA256:-}
profile=${CEM_PROFILE:-cem/0.1}
artifacts=${CEM_ARTIFACTS:-}

is_oid "$base_sha" || fail 'CEM_BASE_SHA must be an exact lowercase 40- or 64-hex commit ID.'
is_oid "$head_sha" || fail 'CEM_HEAD_SHA must be an exact lowercase 40- or 64-hex commit ID.'
is_map_path "$map_path" || fail 'CEM_MAP_PATH must be a normalized repository-relative path.'
case "$profile" in
  cem/0.1) ;;
  cem/1.0)
    case "$artifacts" in /*) ;; *) fail 'CEM_ARTIFACTS must be an absolute directory for CEM_PROFILE=cem/1.0.' ;; esac
    [ -d "$artifacts" ] || fail 'CEM_ARTIFACTS is not a directory.'
    ;;
  *) fail 'CEM_PROFILE must be cem/0.1 or cem/1.0.' ;;
esac
is_digest "$pinned" || fail 'CEM_VERIFIER_SHA256 must be a lowercase 64-hex SHA-256.'
[ -f "$verifier" ] && [ ! -L "$verifier" ] || fail 'CEM_VERIFIER must name a regular executable file.'
[ "$(git -C "$base_repo" rev-parse --is-inside-work-tree 2>/dev/null || true)" = true ] || \
  fail "CEM_REPOSITORY is not a Git checkout: $base_repo"

base_repo=$(CDPATH='' cd -- "$base_repo" && pwd -P)
if [ -n "$head_repo" ]; then
  [ "$(git -C "$head_repo" rev-parse --is-inside-work-tree 2>/dev/null || true)" = true ] || \
    fail "CEM_HEAD_REPOSITORY is not a Git checkout: $head_repo"
  head_repo=$(CDPATH='' cd -- "$head_repo" && pwd -P)
  # Fetch only the declared object from a local checkout. This reads Git objects;
  # it does not run hooks, repository scripts, map values, or an external service.
  git -C "$base_repo" fetch --no-tags --depth=1 "$head_repo" "$head_sha" >/dev/null 2>&1 || \
    fail 'cannot import declared PR-head commit from CEM_HEAD_REPOSITORY.'
fi

# Copy, hash and run one private file, so the checked bytes are the executed bytes.
work_dir=$(mktemp -d) || fail 'cannot create private temporary directory.'
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
cp "$verifier" "$work_dir/verifier" || fail 'cannot copy CEM_VERIFIER.'
chmod 700 "$work_dir/verifier"
actual=$(sha256_of "$work_dir/verifier") || fail 'no sha256sum or shasum is available.'
[ "$actual" = "$pinned" ] || fail 'CEM_VERIFIER does not match CEM_VERIFIER_SHA256; not executed.'

status=0
if [ "$profile" = cem/1.0 ]; then
  # Read the committed map as object data only: no checkout, filter, hook or map value runs.
  if ! git -C "$base_repo" cat-file -e "$head_sha^{commit}" 2>/dev/null; then
    printf '%s\n' 'cem-ci: CEM_HEAD_SHA is not in the repository.' >&2
    exit 5
  fi
  # A directory selector would make ls-tree list the directory's children instead of one map.
  case "$map_path" in */) fail 'CEM_MAP_PATH must name a file, not a directory.' ;; esac
  entry=$(git -C "$base_repo" ls-tree --full-tree "$head_sha" -- "$map_path") || \
    fail 'cannot read the PR-head tree.'
  if [ -z "$entry" ]; then
    printf '%s\n' 'cem-ci: no map at CEM_MAP_PATH in CEM_HEAD_SHA.' >&2
    exit 3
  fi
  # Exactly one line "<mode> blob <oid><TAB><path>" whose path is the requested path.
  tab=$(printf '\t')
  meta=${entry%%"$tab"*}
  entry_path=${entry#*"$tab"}
  mode=${meta%% *}
  rest=${meta#* }
  kind=${rest%% *}
  oid=${rest#* }
  if [ "$entry_path" != "$map_path" ] || [ "$kind" != blob ] || ! is_oid "$oid"; then
    fail 'CEM_MAP_PATH in CEM_HEAD_SHA is not a regular file.'
  fi
  case "$mode" in 100644 | 100755) ;; *) fail 'CEM_MAP_PATH in CEM_HEAD_SHA is not a regular file.' ;; esac
  size=$(git -C "$base_repo" cat-file -s "$oid") || fail 'cannot read the map size.'
  [ "$size" -le 4194304 ] || fail 'the map exceeds 4 MiB.'
  git -C "$base_repo" cat-file blob "$oid" >"$work_dir/change.cem.json" || fail 'cannot read the map.'
  "$work_dir/verifier" verify-stable-canonical --repository "$base_repo" \
    --map "$work_dir/change.cem.json" --expected-base "$base_sha" --target "$head_sha" \
    --artifacts "$artifacts" || status=$?
  exit "$status"
fi
"$work_dir/verifier" ci --repository "$base_repo" --base "$base_sha" --head "$head_sha" \
  --map "$map_path" || status=$?
exit "$status"
