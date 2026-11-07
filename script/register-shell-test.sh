#!/usr/bin/env bash
# usage: script/register-shell-test.sh script/NAME_test.sh [TARGET]
#        script/register-shell-test.sh --hint BASE
#
# ASD-V0-569 (V1-3319): one command registers a shell test in the Makefile's three lists
# (docs/SHELL-TEST-REGISTRATION.md items 1 and 2): it adds the TARGET rule (default NAME-test,
# recipe `@script/NAME_test.sh`) with its .PHONY line when no recipe runs the file, and inserts
# TARGET at its sorted place in `GATE_STEPS` (after the fixed first five steps) and in the
# `shell-test` prerequisites. It is idempotent: a second run changes nothing. Item 3, the
# unbounded-readers record, stays a reviewed hand edit; the command prints it as the next step.
# `make shell-test-register TEST=script/NAME_test.sh` runs it.
#
# --hint BASE (run by `make dogfood-inputs-check`) prints, for each script/*_test.sh added since
# BASE that no Makefile recipe runs or that GATE_STEPS or shell-test does not list, one
# `hint: unregistered-shell-test` line naming the file, this command and the doc. It always exits 0.
set -uo pipefail
export LC_ALL=C
makefile=${REGISTER_SHELL_TEST_MAKEFILE:-Makefile}
doc=docs/SHELL-TEST-REGISTRATION.md

recipe_target() { # FILE: the target whose recipe runs FILE, if any
  awk -v f="$1" '/^[A-Za-z0-9_.-]+:/ { t = $1; sub(/:.*/, "", t) } /^\t/ && index($0, f) { print t; exit }' "$makefile"
}
listed() { # LIST TARGET: whether TARGET is an entry of the list block LIST (GATE_STEPS or shell-test)
  awk -v list="$1" -v t="$2" '
    $0 == list " = \\" || $0 == list ": \\" { on = 1; next }
    on && /^  # end of / { exit 1 }
    on && $1 == t && $2 == "\\" { found = 1; exit 0 }
    END { exit !found }' "$makefile"
}

if [[ ${1-} == --hint ]]; then
  base=${2-}
  [[ -n $base ]] || exit 0
  added=$(git diff --diff-filter=A --name-only "$base" -- 'script/*_test.sh' 2> /dev/null; git ls-files --others --exclude-standard -- 'script/*_test.sh' 2> /dev/null)
  for file in $added; do
    target=$(recipe_target "$file")
    if [[ -z $target ]] || ! listed GATE_STEPS "$target" || ! listed shell-test "$target"; then
      printf 'hint: unregistered-shell-test %s: run make shell-test-register TEST=%s, then see %s item 3 (unbounded-readers)\n' "$file" "$file" "$doc" >&2
    fi
  done
  exit 0
fi

file=${1-}
if [[ ! $file =~ ^script/[A-Za-z0-9._-]+_test\.sh$ || ! -f $file ]]; then
  printf 'usage: script/register-shell-test.sh script/NAME_test.sh [TARGET] (an existing file)\n' >&2
  exit 2
fi
name=${file#script/}
name=${name%_test.sh}
target=$(recipe_target "$file")
[[ -n $target ]] || target=${2:-$name-test}
tmp=$(mktemp "${TMPDIR:-/tmp}/register-shell-test.XXXXXX") || exit 2
trap '/bin/rm -f "$tmp"' EXIT
changed=0
if [[ -z $(recipe_target "$file") ]]; then
  printf '\n.PHONY: %s\n%s:\n\t@%s\n' "$target" "$target" "$file" >> "$makefile"
  printf 'register-shell-test: added rule %s\n' "$target"
  changed=1
fi
# insert LIST SKIP: put "  TARGET \" at its sorted place in LIST, after its first SKIP entries.
insert() {
  awk -v list="$1" -v skip="$2" -v t="$target" '
    function put() { print "  " t " \\"; done = 1 }
    $0 == list " = \\" || $0 == list ": \\" { on = 1; n = 0; print; next }
    on && /^  # end of / { if (!done) put(); on = 0; print; next }
    on { n++; if (!done && n > skip && $1 > t) put(); print; next }
    { print }' "$makefile" > "$tmp" && /bin/cp -f "$tmp" "$makefile"
}
for list in GATE_STEPS shell-test; do
  listed "$list" "$target" && continue
  skip=0
  [[ $list == GATE_STEPS ]] && skip=5
  insert "$list" "$skip" || exit 2
  printf 'register-shell-test: listed %s in %s\n' "$target" "$list"
  changed=1
done
((changed)) || printf 'register-shell-test: %s is already registered as %s\n' "$file" "$target"
printf 'next: %s item 3: declare its reads in .corvint/test-read-scopes.json or .corvint/shell-read-scopes.json, or list it in .corvint/unbounded-readers.json; then make -s --keep-going unbounded-readers-check shell-test-registration-check\n' "$doc"
