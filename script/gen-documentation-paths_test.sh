#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# ASD-V0-105 (V1-3174): script/gen-documentation-paths.sh writes the AFP-V0-160 declaration from
# the tracked top-level docs/*.md files and the declared directories, and --check names the fix
# when the committed declaration is stale. It runs in a fixture repository, not this checkout.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-documentation-paths-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

mkdir -p "$test_root/script" "$test_root/.corvint" "$test_root/docs/specs" "$test_root/docs/plans"
cp "$source_root/script/gen-documentation-paths.sh" "$test_root/script/"
printf 'b\n' >"$test_root/docs/b.md"
printf 'a\n' >"$test_root/docs/A.md"
printf 's\n' >"$test_root/docs/specs/s.md"
printf 'p\n' >"$test_root/docs/plans/p.md"
printf 'x\n' >"$test_root/docs/notes.txt"
git -C "$test_root" init -q
git -C "$test_root" add -A
tool() { (cd "$test_root" && script/gen-documentation-paths.sh "$@"); }
cases=0

# 1. The generator writes the byte-ordered declaration: top-level .md files and the declared
# directories, never a nested file, a non-.md file or an undeclared directory.
tool || fail "generate failed"
want='{"profile":"corvint-documentation-paths/0","paths":["docs/A.md","docs/b.md","docs/build-log/","docs/decisions/","docs/specs/"]}'
[ "$(cat "$test_root/.corvint/documentation-paths.json")" = "$want" ] ||
    fail "declaration $(cat "$test_root/.corvint/documentation-paths.json"), want $want"
tool --check || fail "check refused a fresh declaration"
cases=$((cases + 1))

# 2. A staged new guide makes the committed declaration stale; the refusal names the fix and
# regenerating clears it. An untracked guide is not declared until it is staged.
printf 'c\n' >"$test_root/docs/C.md"
tool --check || fail "check counted an untracked guide"
git -C "$test_root" add docs/C.md
if output=$(tool --check 2>&1); then fail "check passed a stale declaration"; fi
case $output in *'stale'*'make documentation-paths'*) ;; *) fail "refusal did not name the fix: $output" ;; esac
tool && tool --check || fail "regenerating did not clear the stale declaration"
grep -q '"docs/C.md"' "$test_root/.corvint/documentation-paths.json" || fail "staged guide not declared"
cases=$((cases + 1))

# 3. A declared guide deleted from the tree fails the check, naming the missing path.
git -C "$test_root" rm -qf docs/b.md
if output=$(tool --check 2>&1); then fail "check passed a declaration naming a deleted guide"; fi
case $output in *'missing: docs/b.md'*) ;; *) fail "refusal did not name the missing path: $output" ;; esac
tool && tool --check || fail "regenerating did not drop the deleted guide"
cases=$((cases + 1))

# 3b. A hand edit or a missing declaration is stale too.
printf '{"profile":"corvint-documentation-paths/0","paths":["docs/"]}\n' >"$test_root/.corvint/documentation-paths.json"
if tool --check 2>/dev/null; then fail "check passed a hand-edited declaration"; fi
rm "$test_root/.corvint/documentation-paths.json"
if tool --check 2>/dev/null; then fail "check passed a missing declaration"; fi
cases=$((cases + 1))

# 4. A guide name with a space, a quote or a non-ASCII byte is declared raw, not Git-quoted.
odd=$(printf 'docs/Odd "name" \303\251.md')
printf 'o\n' >"$test_root/$odd"
git -C "$test_root" add -- "$odd"
tool || fail "generate failed on an unusual name"
jq -e --arg p "$odd" '.paths | index($p)' "$test_root/.corvint/documentation-paths.json" >/dev/null ||
    fail "unusual name not declared raw: $(cat "$test_root/.corvint/documentation-paths.json")"
cases=$((cases + 1))

# 4b. A guide name with a backslash, which the AFP-V0-160 entry grammar refuses, is refused by
# both modes, so --check cannot pass a declaration the planner would reject.
back='docs/a\b.md'
printf 'b\n' >"$test_root/$back"
git -C "$test_root" add -- "$back"
if output=$(tool 2>&1); then fail "generate accepted a backslash name"; fi
case $output in *'docs/a\b.md holds a backslash'*) ;; *) fail "backslash refusal did not name the path: $output" ;; esac
if tool --check 2>/dev/null; then fail "check passed a backslash name"; fi
git -C "$test_root" rm -qf --cached -- "$back" && rm -f "$test_root/$back"
cases=$((cases + 1))

# 5. An unknown argument is a usage refusal.
status=0
tool --nope >/dev/null 2>&1 || status=$?
[ "$status" -eq 2 ] || fail "unknown argument exited $status, want 2"
cases=$((cases + 1))

printf 'gen-documentation-paths_test: %d cases passed\n' "$cases"
