#!/bin/sh
set -eu

# Covers, on a fixture repository built from the documented minimal layout: DCG-V0-683 (V1-3388),
# a missing list file failing with its name and the layout pointer, and the empty layout passing;
# DCG-V0-681 (V1-3746), a repeated pinned table row failing naming both lines while a distinct
# pinned row and a repeated unpinned row pass; DCG-V0-680 (V1-3678), a failure naming the Git index
# copy when nothing is unstaged, the unstaged note replacing that line when something is, and an
# untracked scanned document named in a passing run.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-line-citations-sources-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
stage() { git -C "$test_root" add -A; }
run() { (cd "$test_root" && script/check-line-citations.sh 2>&1); }

mkdir -p "$test_root/script" "$test_root/docs/specs" "$test_root/internal/demo"
cp "$source_root/script/check-line-citations.sh" "$test_root/script/"
git -C "$test_root" init -q -b main
stage

# DCG-V0-683: each required list file is named when it is missing, with the layout pointer.
for list in line-citation-legacy-documents.txt line-citation-unpinned-inventory.txt; do
  rc=0
  out=$(run) || rc=$?
  [ "$rc" != 0 ] || fail "a fixture without script/$list passed: $out"
  case $out in
    *"script/$list: not readable from the Git index; stage it, even empty, in a fixture repository (the fixture layout is in the header of script/check-line-citations.sh)"*) ;;
    *) fail "the missing script/$list was not named with the layout: $out" ;;
  esac
  : > "$test_root/script/$list"
  stage
done
out=$(run) || fail "the minimal fixture layout failed: $out"
grep -q '^# Fixture layout (DCG-V0-683' "$test_root/script/check-line-citations.sh" ||
  fail "the fixture layout is not documented in the script header"

printf 'package demo\n\nfunc Demo() int {\n\treturn 1\n}\n' > "$test_root/internal/demo/demo.go"
stage
pin=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:4)
doc="$test_root/docs/specs/demo-v0.md"
table() { printf '# Demo\n\n| Row | Evidence |\n|---|---|\n'; for r in "$@"; do printf '%s\n' "$r"; done; }
table "| one | \`$pin\` |" > "$doc"
stage
out=$(run) || fail "a pinned row failed: $out"

# DCG-V0-681: the repeated pinned row fails naming both lines; distinct and unpinned rows pass.
table "| one | \`$pin\` |" "| one | \`$pin\` |" > "$doc"
stage
rc=0
out=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a repeated pinned row passed (rc $rc): $out"
case $out in
  *'docs/specs/demo-v0.md:6  table row repeats docs/specs/demo-v0.md:5 byte for byte, pinned citations included; delete one copy'*) ;;
  *) fail "the repeated row was not named: $out" ;;
esac
table "| one | \`$pin\` |" "| two | \`$pin\` |" "| plain | prose |" "| plain | prose |" > "$doc"
stage
out=$(run) || fail "distinct pinned rows or a repeated unpinned row failed: $out"

# DCG-V0-680: with no unstaged edit, a failure names the Git index copy it judged.
table "| one | \`$pin\` |" "| past | \`internal/demo/demo.go:99@00000000\` |" > "$doc"
stage
rc=0
out=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a citation past the end passed (rc $rc): $out"
case $out in
  *'line citations: note: checked the Git index copy of each file read; the worktree holds no unstaged edit to any of them'*) ;;
  *) fail "a failure did not name the Git index copy: $out" ;;
esac
# An unstaged edit replaces that line with the note naming the file.
printf '// unstaged\n' >> "$test_root/internal/demo/demo.go"
rc=0
out=$(run) || rc=$?
[ "$rc" = 1 ] || fail "the failure changed with an unstaged edit (rc $rc): $out"
case $out in
  *'checked the Git index copy'*) fail "the index-copy line repeated the unstaged note: $out" ;;
  *'this gate reads the Git index, so unstaged edits to 1 file(s) it read were not checked'*'internal/demo/demo.go'*) ;;
  *) fail "the unstaged edit was not named: $out" ;;
esac
git -C "$test_root" checkout -q -- internal/demo/demo.go

# An untracked document in the scanned set is named in a passing run and changes nothing.
table "| one | \`$pin\` |" > "$doc"
stage
# shellcheck disable=SC2016 # the backticks are Markdown literals
printf '# Draft\n\nCites `internal/demo/demo.go:99`.\n' > "$test_root/docs/specs/draft-v0.md"
printf 'not scanned\n' > "$test_root/internal/demo/notes.md"
out=$(run) || fail "an untracked document changed a passing result: $out"
case $out in
  *'line citations: note: 1 untracked document(s) in the scanned set were not checked (git add them to include them):'*'  docs/specs/draft-v0.md'*) ;;
  *) fail "the untracked document was not named: $out" ;;
esac
case $out in *notes.md*) fail "an untracked file outside the scanned set was named: $out" ;; esac

printf 'check-line-citations-sources_test.sh: ok\n'
