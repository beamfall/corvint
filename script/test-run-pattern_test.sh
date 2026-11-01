#!/usr/bin/env bash
# ASD-V0-1025..1027 (V1-3892): script/test-run-pattern.sh selects the tests that read a changed file
# or name a changed requirement ID, including a test outside the changed requirement family that
# reads the file through a helper, keeps a key in TestMain or a top-level declaration fail-closed as
# a whole-package run, keys a shared base name by its parent directory, and refuses a bad BASE.
# Fixture repositories only.
set -uo pipefail
export LC_ALL=C

here=$(cd "$(dirname "$0")" && pwd)
script=$here/test-run-pattern.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-test-run-pattern-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
failed=0
fail() {
  printf 'test-run-pattern_test: FAIL %s\n' "$*" >&2
  failed=1
}
r=$tmp/repo
g() { git -C "$r" -c user.name=t -c user.email=t@t "$@"; }

git init -q "$r" || exit 1
mkdir -p "$r/docs/specs" "$r/internal/a" "$r/internal/b" "$r/internal/c"
printf -- '- `XYZ-V0-1020`: old text\n' > "$r/docs/specs/x.md"
printf 'one\n' > "$r/Makefile"
cat > "$r/internal/a/a_test.go" << 'EOF'
package a

import "testing"

// readMakefile is a helper the targets test reads the changed file through.
func readMakefile(t *testing.T) string {
	return string(mustRead(t, "../../Makefile"))
}

var specPath = filepath.Join("..", "..", "docs", "specs", "x.md")

// TestTargets is in another requirement family; it reads the file only through the helper.
func TestTargets(t *testing.T) {
	_ = readMakefile(t)
}

func TestSpec(t *testing.T) { _ = specPath }

// TestByDoc covers XYZ-V0-1020 in its doc comment.
func TestByDoc(t *testing.T) {
}

func TestXYZV01020Name(t *testing.T) {
}

func TestUnrelated(t *testing.T) {
	_ = "Makefile.bak"
}

func mustRead(t *testing.T, p string) []byte { return nil }
EOF
cat > "$r/internal/b/b_test.go" << 'EOF'
package b

import "testing"

func TestMain(m *testing.M) {
	load("Makefile")
}

func TestB(t *testing.T) {}
EOF
cat > "$r/internal/c/c_test.go" << 'EOF'
package c

import "testing"

func TestC(t *testing.T) {}
EOF
mkdir -p "$r/tools/one" "$r/tools/two" "$r/internal/d"
printf 'package main\n' > "$r/tools/one/main.go"
printf 'package main\n' > "$r/tools/two/main.go"
cat > "$r/internal/d/d_test.go" << 'EOF'
package d

import "testing"

func TestOne(t *testing.T) { _ = filepath.Join(root, "tools", "one", "main.go") }

func TestTwo(t *testing.T) { _ = "../../tools/two/main.go" }

func TestAnyMain(t *testing.T) { _ = "main.go" }
EOF
g add -A && g commit -q -m base
base=$(g rev-parse HEAD)

# No change: no key, no selection.
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == 'test-run-pattern: SUMMARY keys=paths:0,ids:0 packages=0 tests=0 whole=0' ]] || fail "no change: rc=$rc out=$out"

printf 'two\n' > "$r/Makefile"
printf -- '- `XYZ-V0-1020`: new text\n' > "$r/docs/specs/x.md"
g commit -q -am change
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
want="run: script/heavy.sh go test -count=1 -run '^(TestByDoc|TestSpec|TestTargets|TestXYZV01020Name)\$' ./internal/a
  why TestByDoc internal/a/a_test.go:19 XYZ-V0-1020
  why TestSpec internal/a/a_test.go:10 x.md via specPath
  why TestTargets internal/a/a_test.go:7 Makefile via readMakefile
  why TestXYZV01020Name internal/a/a_test.go:23 XYZV01020
whole: ./internal/b reason=TestMain-reads-key internal/b/b_test.go:6 Makefile
test-run-pattern: SUMMARY keys=paths:2,ids:1 packages=2 tests=4 whole=1"
[[ $rc == 0 && $out == "$want" ]] || fail "selection: rc=$rc out=$out"

# A key in a top-level declaration the parser cannot attribute runs the package whole.
printf 'package c\n\ntype fixture struct{ path string `json:"Makefile"` }\n' > "$r/internal/c/d_test.go"
g add -A && g commit -q -m tag
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *'whole: ./internal/c reason=top-level internal/c/d_test.go:3 Makefile'* ]] || fail "top-level key: rc=$rc out=$out"

# A base name two tracked files share needs its parent directory: only TestOne reads tools/one.
printf 'package main // changed\n' > "$r/tools/one/main.go"
g commit -q -am shared
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"run: script/heavy.sh go test -count=1 -run '^(TestOne)\$' ./internal/d"* ]] || fail "shared base name: rc=$rc out=$out"

out=$(cd "$r" && "$script" no-such-rev 2>&1); rc=$?
[[ $rc == 2 && $out == *'REFUSE unknown-base no-such-rev; fix:'* ]] || fail "bad base: rc=$rc out=$out"
out=$(cd "$r" && "$script" 2>&1); rc=$?
[[ $rc == 2 && $out == *'REFUSE usage:'* ]] || fail "usage: rc=$rc out=$out"

[[ $failed == 0 ]] || exit 1
echo "test-run-pattern_test.sh: ok"
