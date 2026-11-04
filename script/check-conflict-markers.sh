#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# DCI-V0-7902: inspect staged text, so a dirty worktree cannot hide a committed marker.
set -euo pipefail
export LC_ALL=C
cd "$(git rev-parse --show-toplevel)"
allow=script/conflict-marker-fixtures.txt
allowed=()
if git cat-file -e ":$allow" 2>/dev/null; then
  while IFS= read -r path || [[ -n $path ]]; do
    [[ -z $path || $path == \#* ]] && continue
    case /$path/ in
      */../*|*/./*|//*|*\**|*\?*|*\[*) echo "conflict-markers: invalid fixture path: $path" >&2; exit 2 ;;
    esac
    if [[ $(git cat-file -t ":$path" 2>/dev/null) != blob ]]; then
      echo "conflict-markers: allowlist requires a tracked fixture file: $path" >&2
      exit 2
    fi
    case /$path in
      */testdata/*|*/fixtures/*) allowed+=("$path") ;;
      *) echo "conflict-markers: allowlist requires an exact fixture path: $path" >&2; exit 2 ;;
    esac
  done < <(git show ":$allow")
fi
paths=('docs/' '*.go' '*.json' 'script/' '*.sh' '*.bash' '*.cjs' '*.mjs' '*.js' '*.py' '*.ps1')
for fixture in ${allowed[@]+"${allowed[@]}"}; do
  paths+=(":(exclude,literal)$fixture")
done
rc=0
git grep --cached -n -E '^(<<<<<<< |=======$|>>>>>>> )' -- "${paths[@]}" || rc=$?
if ((rc == 0)); then
  echo 'conflict-markers: FAIL: resolve the staged conflict markers above' >&2
  exit 1
fi
if ((rc != 1)); then exit "$rc"; fi
echo 'conflict-markers: PASS'
