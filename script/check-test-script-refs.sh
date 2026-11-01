#!/usr/bin/env bash
# check-test-script-refs.sh: fail when a shell test or a Makefile line names a script that is not
# in the tree (ASD-V0-641, V1-3745). A lane that deletes or renames a script on main leaves another
# lane's test calling it; this names the file, line and missing path before the batch merges.
#
#   script/check-test-script-refs.sh        check the checkout at the current directory's top level
#
# Scanned: every tracked *_test.sh outside a testdata/ directory, for $script/NAME, ${script}/NAME,
# $source_root/script/NAME and ${source_root}/script/NAME where NAME ends in .sh or .py (the forms
# the repository's shell tests use for the real script directory), and every Makefile line naming
# script/NAME.sh or script/NAME.py. Not scanned: Go test files, whose fixture strings name script
# paths they never run, and scripts reached through other variables. Exit 0 clean, 1 on a missing
# script, 2 outside a Git checkout.
set -uo pipefail
export LC_ALL=C
top=$(git rev-parse --show-toplevel 2> /dev/null) || { printf 'check-test-script-refs: not inside a Git checkout\n' >&2; exit 2; }
cd "$top" || exit 2
bad=0 refs=0
while IFS= read -r f; do
  [[ -f $f ]] || continue
  while IFS=: read -r line ref; do
    refs=$((refs + 1))
    name=${ref##*/}
    [[ -e script/$name ]] && continue
    printf 'check-test-script-refs: FAIL %s:%s names %s, but script/%s does not exist\n' "$f" "$line" "$ref" "$name"
    bad=1
  done < <(grep -noE '\$(\{script\}|script|\{source_root\}/script|source_root/script)/[A-Za-z0-9._-]+\.(sh|py)\b' "$f")
done < <(git ls-files -- '*_test.sh' | grep -v '\(^\|/\)testdata/')
if [[ -f Makefile ]]; then
  while IFS=: read -r line ref; do
    refs=$((refs + 1))
    ref=${ref#"${ref%%script/*}"}
    [[ -e $ref ]] && continue
    printf 'check-test-script-refs: FAIL Makefile:%s names %s, which does not exist\n' "$line" "$ref"
    bad=1
  done < <(grep -noE '(^|[^A-Za-z0-9_./-])(\./)?script/[A-Za-z0-9._-]+\.(sh|py)\b' Makefile)
fi
((bad)) && exit 1
printf 'check-test-script-refs: PASS %d script references resolve\n' "$refs"
