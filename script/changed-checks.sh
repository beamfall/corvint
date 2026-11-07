#!/usr/bin/env bash
# usage: script/changed-checks.sh BASE
#
# ASD-V0-576 (V1-3390): the changed-files mode of three doc gates that otherwise surface only at
# closeout. From the paths changed since BASE (committed, staged, unstaged and untracked) it runs
#   line-citations-check     when a Markdown file changed or a changed path is cited as `PATH:N`
#                            in a tracked Markdown file;
#   unbounded-readers-check  when a path other than a Markdown file changed, or a path was added
#                            or deleted: the ratchet rebuilds the affected graph from tracked code
#                            in every indexed language and the .corvint read declarations, so a
#                            Go test or source edit or a read-scope declaration can move it;
#   test-read-scopes-check   when a *_test.go file, tools/test-read-scopes-check/ or an
#                            .corvint/test-read-scope* declaration changed, or a path was added
#                            or deleted;
# and prints one `skip` line for each check none of its inputs reach. CHANGED_CHECKS_MAKE replaces
# `make -s` (tests). Exit 1 when a run check fails, naming each; 2 on a usage error or bad BASE.
set -uo pipefail
export LC_ALL=C
me=changed-checks
[[ $# == 1 && -n $1 ]] || { printf 'usage: script/changed-checks.sh BASE\n' >&2; exit 2; }
base=$1
top=$(git rev-parse --show-toplevel 2> /dev/null) || { printf '%s: not inside a Git checkout\n' "$me" >&2; exit 2; }
cd "$top" || exit 2
git rev-parse -q --verify "$base^{commit}" > /dev/null || { printf '%s: BASE %s names no commit\n' "$me" "$base" >&2; exit 2; }
changed=$({ git diff --name-only "$base" --; git ls-files --others --exclude-standard; } | sort -u)
added=$({ git diff --name-only --diff-filter=AD "$base" --; git ls-files --others --exclude-standard; } | sort -u)
read -r -a run_make <<< "${CHANGED_CHECKS_MAKE:-make -s}"
[[ -z $changed ]] && { printf '%s: no path changed since %s\n' "$me" "$base"; exit 0; }

first() { grep -m1 -E "$1" <<< "$changed"; }
cited=
if ! cited=$(first '\.md$'); then
  patterns=$(sed 's/.*/`&:/' <<< "$changed")
  cited=$(git grep -h -o -F -f <(printf '%s\n' "$patterns") -- '*.md' 2> /dev/null | head -n 1 | sed 's/^`//; s/:$//')
fi
unbounded=$(grep -m1 -v -E '\.md$' <<< "$changed")
[[ -z $unbounded ]] && unbounded=$(head -n 1 <<< "$added")
scopes=$(first '_test\.go$|^tools/test-read-scopes-check/|^\.corvint/test-read-scope')
[[ -z $scopes ]] && scopes=$(head -n 1 <<< "$added")

failed=()
check() {
  local name=$1 reason=$2
  if [[ -z $reason ]]; then
    printf '%s: skip %s (none of its inputs changed)\n' "$me" "$name"
    return
  fi
  printf '%s: run %s (changed: %s)\n' "$me" "$name" "$reason"
  "${run_make[@]}" "$name" || failed+=("$name")
}
check line-citations-check "$cited"
check unbounded-readers-check "$unbounded"
check test-read-scopes-check "$scopes"
((${#failed[@]} == 0)) && { printf '%s: ok\n' "$me"; exit 0; }
printf '%s: FAILED %s\n' "$me" "${failed[*]}" >&2
exit 1
