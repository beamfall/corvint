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
# paths they never run, and scripts reached through other variables. NAME is the full file name, so
# $script/exists.sh.bak names exists.sh.bak. Exit 0 clean, 1 on a missing script, 2 outside a Git
# checkout or when the tracked tests cannot be listed or a scanned file cannot be read.
set -uo pipefail
export LC_ALL=C
top=$(git rev-parse --show-toplevel 2> /dev/null) || { printf 'check-test-script-refs: not inside a Git checkout\n' >&2; exit 2; }
cd "$top" || exit 2
bad=0 refs=0
# A script name is the full file name after the slash (so exists.sh.bak is not read as exists.sh)
# that ends in .sh or .py or carries one of them before a further suffix; trailing dots are prose.
script_name() {
  ref=$1
  while [[ $ref == *. ]]; do ref=${ref%.}; done
  [[ ${ref##*/} == *.sh || ${ref##*/} == *.py || ${ref##*/} == *.sh.* || ${ref##*/} == *.py.* ]]
}
# matches FILE ERE: the -no matches of ERE in FILE, failing closed on a grep error (exit 2).
matches() {
  hits=$(grep -noE "$2" "$1")
  case $? in
    0 | 1) ;;
    *) printf 'check-test-script-refs: cannot read %s\n' "$1" >&2; exit 2 ;;
  esac
}
# The tracked tests, NUL-separated (no core.quotePath quoting, so script/café_test.sh is scanned),
# listed to a file so a failed git ls-files exits 2 instead of scanning nothing and passing.
list=$(mktemp "${TMPDIR:-/tmp}/check-test-script-refs.XXXXXX") || { printf 'check-test-script-refs: mktemp failed\n' >&2; exit 2; }
trap '/bin/rm -f "$list"' EXIT
git ls-files -z -- '*_test.sh' > "$list" || { printf 'check-test-script-refs: git ls-files failed, so the tracked tests cannot be listed\n' >&2; exit 2; }
while IFS= read -r -d '' f; do
  [[ $f =~ (^|/)testdata/ ]] && continue
  [[ -f $f ]] || continue
  # shellcheck disable=SC2016 # the pattern matches a literal $script in the scanned file
  matches "$f" '\$(\{script\}|script|\{source_root\}/script|source_root/script)/[A-Za-z0-9._-]+'
  while IFS=: read -r line ref; do
    [[ -n $ref ]] && script_name "$ref" || continue
    refs=$((refs + 1))
    name=${ref##*/}
    [[ -e script/$name ]] && continue
    printf 'check-test-script-refs: FAIL %s:%s names %s, but script/%s does not exist\n' "$f" "$line" "$ref" "$name"
    bad=1
  done <<< "$hits"
done < "$list"
if [[ -f Makefile ]]; then
  matches Makefile '(^|[^A-Za-z0-9_./-])(\./)?script/[A-Za-z0-9._-]+'
  while IFS=: read -r line ref; do
    [[ -n $ref ]] && script_name "$ref" || continue
    refs=$((refs + 1))
    ref=${ref#"${ref%%script/*}"}
    [[ -e $ref ]] && continue
    printf 'check-test-script-refs: FAIL Makefile:%s names %s, which does not exist\n' "$line" "$ref"
    bad=1
  done <<< "$hits"
fi
((bad)) && exit 1
printf 'check-test-script-refs: PASS %d script references resolve\n' "$refs"
