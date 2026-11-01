#!/bin/sh
set -eu

# DCG-V0-682 (V1-3746): two branches repin one citation to different line numbers after each moves
# the cited code. The merge stops on the document; the resolver keeps one side, stages it and runs
# --repin, which moves the pin to the merged position. A conflict that also changes prose is
# refused and left unmerged; a diff3 base section resolves the same way as a two-part block.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-line-citations-resolve-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
g() { git -C "$test_root" -c user.name=t -c user.email=t@example.invalid "$@"; }

mkdir -p "$test_root/script" "$test_root/docs/specs" "$test_root/internal/demo"
cp "$source_root/script/check-line-citations.sh" "$source_root/script/line-citations-resolve.sh" "$test_root/script/"
g init -q -b main
: > "$test_root/script/line-citation-legacy-documents.txt"
: > "$test_root/script/line-citation-unpinned-inventory.txt"
i=1
while [ "$i" -le 10 ]; do printf 'line %s of the demo\n' "$i"; i=$((i + 1)); done > "$test_root/internal/demo/demo.go"
g add -A
pin() { (cd "$test_root" && script/check-line-citations.sh --hash "internal/demo/demo.go:$1"); }
# shellcheck disable=SC2016 # the backticks are Markdown literals
row() { printf '# Demo\n\n| Row | Evidence |\n|---|---|\n| %s | `%s` |\n' "$1" "$2" > "$test_root/docs/specs/demo-v0.md"; }
row one "$(pin 8)"
g add -A
g commit -q -m base

# Branch a inserts one line at the top, so line 8 moves to 9; branch b inserts two after line 4,
# so it moves to 10. Each repins its own copy of the row.
g checkout -q -b a
{ printf 'inserted on a\n'; cat "$test_root/internal/demo/demo.go"; } > "$test_root/demo.tmp"
/bin/mv -f "$test_root/demo.tmp" "$test_root/internal/demo/demo.go"
g add -A
row one "$(pin 9)"
g add -A
g commit -q -m a
g checkout -q -b b main
awk 'NR == 5 { print "inserted on b"; print "inserted on b too" } { print }' "$test_root/internal/demo/demo.go" > "$test_root/demo.tmp"
/bin/mv -f "$test_root/demo.tmp" "$test_root/internal/demo/demo.go"
g add -A
row one "$(pin 10)"
g add -A
g commit -q -m b
g branch -q b-prose
g checkout -q a

# 1. A conflict that differs only in the pinned span resolves, stages and repins to line 11.
if g merge -q --no-edit b > /dev/null 2>&1; then fail "the fixture merge did not conflict"; fi
rc=0
out=$(cd "$test_root" && script/line-citations-resolve.sh 2>&1) || rc=$?
[ "$rc" = 0 ] || fail "a line-number-only conflict was not resolved (rc $rc): $out"
case $out in *'docs/specs/demo-v0.md: resolved 1 block(s)'*) ;; *) fail "the resolution was not named: $out" ;; esac
[ -z "$(g diff --name-only --diff-filter=U)" ] || fail "the document is still unmerged: $(g status --short)"
grep -q 'internal/demo/demo.go:11@' "$test_root/docs/specs/demo-v0.md" ||
  fail "--repin did not move the pin to the merged line: $(cat "$test_root/docs/specs/demo-v0.md")"
(cd "$test_root" && script/check-line-citations.sh > /dev/null 2>&1) || fail "the resolved merge fails the gate"
g merge --abort

# 2. The same conflict with a diff3 base section resolves the same way.
if g -c merge.conflictStyle=diff3 merge -q --no-edit b > /dev/null 2>&1; then fail "the diff3 merge did not conflict"; fi
grep -q '^|||||||' "$test_root/docs/specs/demo-v0.md" || fail "the fixture conflict has no diff3 base section"
(cd "$test_root" && script/line-citations-resolve.sh > /dev/null 2>&1) || fail "a diff3 line-number-only conflict was not resolved"
grep -q 'internal/demo/demo.go:11@' "$test_root/docs/specs/demo-v0.md" || fail "the diff3 resolution did not repin"
g merge --abort

# 3. A side that also changes prose is refused, named by line, and left unmerged and unstaged.
g checkout -q b-prose
sed 's/| one |/| one, reworded |/' "$test_root/docs/specs/demo-v0.md" > "$test_root/doc.tmp"
/bin/mv -f "$test_root/doc.tmp" "$test_root/docs/specs/demo-v0.md"
g commit -q -am prose
g checkout -q a
if g merge -q --no-edit b-prose > /dev/null 2>&1; then fail "the prose merge did not conflict"; fi
rc=0
out=$(cd "$test_root" && script/line-citations-resolve.sh 2>&1) || rc=$?
[ "$rc" = 1 ] || fail "a prose conflict was not refused (rc $rc): $out"
case $out in *'docs/specs/demo-v0.md:5: the two sides differ in more than pinned citation line numbers'*) ;; *) fail "the refusal did not name the block: $out" ;; esac
[ "$(g diff --name-only --diff-filter=U)" = docs/specs/demo-v0.md ] || fail "a refused document was staged"
grep -q '^<<<<<<<' "$test_root/docs/specs/demo-v0.md" || fail "a refused document was rewritten"
g merge --abort

# 4. With nothing unmerged it says so and succeeds.
out=$(cd "$test_root" && script/line-citations-resolve.sh 2>&1) || fail "an idle run failed: $out"
case $out in *'no unmerged Markdown file'*) ;; *) fail "an idle run was not named: $out" ;; esac

printf 'line-citations-resolve_test.sh: ok\n'
