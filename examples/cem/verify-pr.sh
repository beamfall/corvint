#!/usr/bin/env sh
# Verify a checked-in CEM sidecar without executing repository or map content.
#
# Required environment:
#   CEM_BASE_SHA       exact 40- or 64-hex base commit
#   CEM_HEAD_SHA       exact 40- or 64-hex PR-head commit
# Optional environment:
#   CEM_MAP_PATH       map path in the head tree (default .corvint/change.cem.json)
#   CEM_REPOSITORY     trusted checkout containing the base (default $PWD)
#   CEM_HEAD_REPOSITORY separate checkout containing CEM_HEAD_SHA (optional)
#   CEM_PROFILE        expected map profile: cem/0.1 (default) or cem/0.2
#   CEM_MAX_UNKNOWN    accepted unknown hunks (default 0)
#   CEM_MAX_MECHANICAL accepted mechanical hunks (default 0)
#   CORVINT_BIN        executable Corvint CLI (default corvint)

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
  exit "${2:-3}"
}

is_oid() {
  case "$1" in
    ''|*[!0123456789abcdef]*) return 1 ;;
  esac
  length=${#1}
  [ "$length" -eq 40 ] || [ "$length" -eq 64 ]
}

is_map_path() {
  case "$1" in
    ''|/*|.|..|../*|*/..|*/../*|*'//'*|*'/./'*|*[!ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._/-]*)
      return 1
      ;;
  esac
  return 0
}

is_count() {
  case "$1" in
    ''|*[!0123456789]*) return 1 ;;
  esac
  return 0
}

base_sha=${CEM_BASE_SHA:-}
head_sha=${CEM_HEAD_SHA:-}
map_path=${CEM_MAP_PATH:-.corvint/change.cem.json}
cem_profile=${CEM_PROFILE:-cem/0.1}
base_repo=${CEM_REPOSITORY:-$PWD}
head_repo=${CEM_HEAD_REPOSITORY:-}
corvint_bin=${CORVINT_BIN-corvint}
max_unknown=${CEM_MAX_UNKNOWN:-0}
max_mechanical=${CEM_MAX_MECHANICAL:-0}

is_oid "$base_sha" || fail 'CEM_BASE_SHA must be an exact lowercase 40- or 64-hex commit ID.'
is_oid "$head_sha" || fail 'CEM_HEAD_SHA must be an exact lowercase 40- or 64-hex commit ID.'
is_map_path "$map_path" || fail 'CEM_MAP_PATH must be a normalized repository-relative path.'
case "$cem_profile" in
  cem/0.1) ;;
  cem/0.2) [ "$map_path" = .corvint/change.cem.json ] || \
    fail 'cem/0.2 requires CEM_MAP_PATH=.corvint/change.cem.json.' ;;
  *) fail 'CEM_PROFILE must be cem/0.1 or cem/0.2.' ;;
esac
is_count "$max_unknown" || fail 'CEM_MAX_UNKNOWN must be a non-negative integer.'
is_count "$max_mechanical" || fail 'CEM_MAX_MECHANICAL must be a non-negative integer.'
[ "$(git -C "$base_repo" rev-parse --is-inside-work-tree 2>/dev/null || true)" = true ] || \
  fail "CEM_REPOSITORY is not a Git checkout: $base_repo"

base_repo=$(CDPATH= cd -- "$base_repo" && pwd -P)
if [ -n "$head_repo" ]; then
  [ "$(git -C "$head_repo" rev-parse --is-inside-work-tree 2>/dev/null || true)" = true ] || \
    fail "CEM_HEAD_REPOSITORY is not a Git checkout: $head_repo"
  head_repo=$(CDPATH= cd -- "$head_repo" && pwd -P)
  # Fetch only the declared object from a local checkout. This reads Git objects;
  # it does not run hooks, repository scripts, map values, or an external service.
  git -C "$base_repo" fetch --no-tags --depth=1 "$head_repo" "$head_sha" >/dev/null 2>&1 || \
    fail 'cannot import declared PR-head commit from CEM_HEAD_REPOSITORY.'
fi

actual_base=$(git -C "$base_repo" rev-parse --verify "$base_sha^{commit}" 2>/dev/null) || \
  fail 'CEM_BASE_SHA is not an available commit in the trusted checkout.'
actual_head=$(git -C "$base_repo" rev-parse --verify "$head_sha^{commit}" 2>/dev/null) || \
  fail 'CEM_HEAD_SHA is not an available commit in the trusted checkout.'
[ "$actual_base" = "$base_sha" ] || fail 'CEM_BASE_SHA did not resolve byte-for-byte.'
[ "$actual_head" = "$head_sha" ] || fail 'CEM_HEAD_SHA did not resolve byte-for-byte.'

work_dir=$(mktemp -d "$base_repo/.corvint-cem.XXXXXX") || fail 'cannot create private temporary directory.'
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
map_file=$work_dir/change.cem.json
patch_file=$work_dir/change.patch
work_relative=${work_dir#"$base_repo"/}
map_input=$work_relative/change.cem.json
patch_input=$work_relative/change.patch
max_map_bytes=4194304
max_patch_blocks=16384

# Read the map as data from the PR head. The CEM sidecar itself is deliberately
# excluded from the patch: otherwise a map would recursively need to cite its
# own creation. All other textual hunks remain mandatory CEM subjects.
map_size=$(git -C "$base_repo" cat-file -s "$head_sha:$map_path" 2>/dev/null) || \
  fail "CEM map is absent or unreadable at $head_sha:$map_path"
is_count "$map_size" || fail 'CEM map size is not a non-negative integer.'
[ "$map_size" -le "$max_map_bytes" ] || fail 'CEM map exceeds the 4 MiB limit.'
git -C "$base_repo" show "$head_sha:$map_path" >"$map_file" 2>/dev/null || \
  fail "CEM map is absent or unreadable at $head_sha:$map_path"
if [ "$cem_profile" = cem/0.1 ]; then
  ( ulimit -f "$max_patch_blocks"
    git -C "$base_repo" -c core.quotePath=false -c diff.algorithm=myers -c diff.context=3 \
      diff --binary --full-index --no-color --no-ext-diff --no-textconv --no-renames \
      --no-indent-heuristic --diff-algorithm=myers --unified=3 \
      --src-prefix=a/ --dst-prefix=b/ --ignore-submodules=none \
      "$base_sha" "$head_sha" -- . ":(exclude)$map_path" >"$patch_file"
  ) || fail 'cannot derive the exact base-to-head patch.'
  [ "$(wc -c <"$patch_file" | tr -d ' ')" -le 8388608 ] || \
    fail 'CEM patch exceeds the 8 MiB limit.'
  "$corvint_bin" --root "$base_repo" cem verify \
    --map "$map_input" --patch "$patch_input" --target "$head_sha" \
    --expected-base "$base_sha" \
    --max-unknown "$max_unknown" --max-mechanical "$max_mechanical"
else
  "$corvint_bin" --root "$base_repo" cem verify \
    --map "$map_input" --target "$head_sha" --expected-base "$base_sha" \
    --max-unknown "$max_unknown" --max-mechanical "$max_mechanical"
fi
