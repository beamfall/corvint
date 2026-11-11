#!/usr/bin/env bash
# ASD-V0-1025..1027 (V1-3892): script/test-run-pattern.sh selects the tests that read a changed file
# or name a changed requirement ID, including a test outside the changed requirement family that
# reads the file through a helper, keeps a key in TestMain or a top-level declaration fail-closed as
# a whole-package run, keys a shared base name by its parent directory, and refuses a bad BASE.
# ASD-V0-3832 (V1-5007): a changed non-test .go file runs its package whole (fault injection: the
# edit breaks an existing test), and a _test.go-only edit narrows to the changed tests.
# ASD-V0-5133: removed lines are read from BASE: a removed init, var or helper declaration and a
# deleted test file run the package whole, a removed test or benchmark selects nothing of its own,
# a removed line inside a surviving test or helper narrows; an untracked test file's uncovered line
# runs whole. ASD-V0-5134: diff.noprefix, diff.mnemonicPrefix and a path with a space and a
# non-ASCII byte change nothing. ASD-V0-5135: a changed .s file, another non-Go file in a package
# directory with tests, and go.mod run whole; testdata and a testless directory do not.
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
printf 'module example.test/fixture\n\ngo 1.22\n' > "$r/go.mod"
mkdir -p "$r/internal/e/testdata" "$r/internal/f"
cat > "$r/internal/e/lib.go" << 'EOF'
package e

// Answer is the value the existing test pins.
func Answer() int { return 1 }
EOF
cat > "$r/internal/e/lib_test.go" << 'EOF'
package e

import "testing"

func TestAnswer(t *testing.T) {
	if Answer() != 1 {
		t.Fatalf("Answer() = %d", Answer())
	}
}

func TestOther(t *testing.T) {}
EOF
printf 'package x\n' > "$r/internal/e/testdata/x.go"
cat > "$r/internal/f/f_test.go" << 'EOF'
package f

import (
	"testing"
)

type fixture struct{ name string }

func setup(t *testing.T) fixture {
	return fixture{name: "one"}
}

func TestOne(t *testing.T) {
	_ = 1
}

// TestTwo uses the helper.
func TestTwo(t *testing.T) {
	_ = setup(t)
}
EOF
cat > "$r/internal/f/main_test.go" << 'EOF'
package f

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
EOF
g add -A && g commit -q -m base
base=$(g rev-parse HEAD)

# No change: no key, no selection.
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == 'test-run-pattern: SUMMARY keys=paths:0,ids:0 packages=0 tests=0 whole=0 sources=0' ]] || fail "no change: rc=$rc out=$out"

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
test-run-pattern: SUMMARY keys=paths:2,ids:1 packages=2 tests=4 whole=1 sources=0"
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

# ASD-V0-3832 (V1-5007): a changed non-test .go file runs its package whole (source-edit), in every
# state of the change; a .go file under testdata is data, not source. The fault injection: the edit
# breaks TestAnswer, which only a whole-package run (no -run narrowing) finds before commit.
source_whole="whole: ./internal/e reason=source-edit internal/e/lib.go"
printf 'package e\n\n// Answer is the value the existing test pins.\nfunc Answer() int { return 2 }\n' > "$r/internal/e/lib.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
# sources counts tools/one/main.go, committed above, as well: a changed source of a testless package.
[[ $rc == 0 && $out == *"$source_whole"* && $out == *" sources=2" ]] || fail "unstaged source edit: rc=$rc out=$out"
# The fixture is a real regression: TestAnswer fails under the edited lib.go, so the printed whole
# run finds it (run by hand; this self-test runs no toolchain, docs/SHELL-TEST-REGISTRATION.md).
g add internal/e/lib.go
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"$source_whole"* ]] || fail "staged source edit: rc=$rc out=$out"
g commit -q -m source
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"$source_whole"* ]] || fail "committed source edit: rc=$rc out=$out"
printf 'package e\n\nfunc Extra() int { return 3 }\n' > "$r/internal/e/extra.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"whole: ./internal/e reason=source-edit internal/e/extra.go"* && $out == *" sources=3" ]] || fail "untracked source file: rc=$rc out=$out"
/bin/rm -f "$r/internal/e/extra.go"
printf 'package x // data\n' > "$r/internal/e/testdata/x.go"
g commit -q -am testdata
printf 'package y\n' > "$r/tools/one/y.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out != *"internal/e/testdata"* && $out != *"tools/one reason="* && $out == *" sources=3" ]] || fail "testdata and testless package: rc=$rc out=$out"
/bin/rm -f "$r/tools/one/y.go"

# ASD-V0-3832: a _test.go-only edit narrows to the changed tests: the edited test, a test that
# names an edited helper, a new test; an import-only edit selects nothing; a changed TestMain or a
# changed type declaration runs the package whole.
perl -0pi -e 's/func TestOne\(t \*testing.T\) \{\n\t_ = 1\n/func TestOne(t *testing.T) {\n\t_ = 2\n/' "$r/internal/f/f_test.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"run: script/heavy.sh go test -count=1 -run '^(TestOne)\$' ./internal/f
  why TestOne internal/f/f_test.go:14 changed"* ]] || fail "edited test narrows: rc=$rc out=$out"
g checkout -q -- internal/f
perl -0pi -e 's/return fixture\{name: "one"\}/return fixture{name: "two"}/' "$r/internal/f/f_test.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"run: script/heavy.sh go test -count=1 -run '^(TestTwo)\$' ./internal/f
  why TestTwo internal/f/f_test.go:10 changed via setup"* ]] || fail "edited helper selects its tests: rc=$rc out=$out"
g checkout -q -- internal/f
printf '\nfunc TestThree(t *testing.T) {}\n' >> "$r/internal/f/f_test.go"
g add -A && g commit -q -m three
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"run: script/heavy.sh go test -count=1 -run '^(TestThree)\$' ./internal/f
  why TestThree internal/f/f_test.go:22 changed"* ]] || fail "new test selected: rc=$rc out=$out"
cat > "$r/internal/f/g_test.go" << 'EOF'
package f

import "testing"

// TestNew is in an untracked file.
func TestNew(t *testing.T) {}
EOF
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"-run '^(TestNew|TestThree)\$' ./internal/f"* && $out == *"why TestNew internal/f/g_test.go:5 changed"* ]] || fail "untracked test file: rc=$rc out=$out"
/bin/rm -f "$r/internal/f/g_test.go"
perl -0pi -e 's/import \(\n\t"testing"\n\)/import (\n\t"os"\n\t"testing"\n)/' "$r/internal/f/f_test.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"-run '^(TestThree)\$' ./internal/f"* && $out != *"why TestOne internal/f"* && $out != *"whole: ./internal/f"* ]] || fail "import-only edit selects nothing new: rc=$rc out=$out"
g checkout -q -- internal/f
perl -0pi -e 's/type fixture struct\{ name string \}/type fixture struct{ name, kind string }/' "$r/internal/f/f_test.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"whole: ./internal/f reason=changed-top-level internal/f/f_test.go:7"* ]] || fail "changed type runs whole: rc=$rc out=$out"
g checkout -q -- internal/f
perl -0pi -e 's/os.Exit\(m.Run\(\)\)/os.Exit(m.Run() \& 1)/' "$r/internal/f/main_test.go"
out=$(cd "$r" && "$script" "$base" 2>&1); rc=$?
[[ $rc == 0 && $out == *"whole: ./internal/f reason=TestMain-changed internal/f/main_test.go:9 changed"* ]] || fail "changed TestMain runs whole: rc=$rc out=$out"
g checkout -q -- internal/f

# ASD-V0-5133 to ASD-V0-5135 on a second fixture whose BASE already holds the declarations the
# cases remove.
r2=$tmp/repo2
g2() { git -C "$r2" -c user.name=t -c user.email=t@t "$@"; }
git init -q "$r2" || exit 1
mkdir -p "$r2/internal/g" "$r2/internal/h/testdata" "$r2/internal/f" "$r2/tools/x"
printf 'module example.test/two\n\ngo 1.22\n' > "$r2/go.mod"
printf 'package g\n' > "$r2/internal/g/g.go"
cat > "$r2/internal/g/g_test.go" << 'EOF'
package g

import "testing"

var counter int

func init() { counter = 1 }

func helper() int {
	return counter
}

func TestG1(t *testing.T) {
	if helper() != 1 {
		t.Fatal("init did not run")
	}
}

func TestG2(t *testing.T) {
	_ = 2
}

func BenchmarkG(b *testing.B) {
	for range b.N {
	}
}
EOF
printf 'package g\n\nimport "testing"\n\nfunc TestOther(t *testing.T) {}\n' > "$r2/internal/g/other_test.go"
printf 'package h\n' > "$r2/internal/h/h.go"
printf 'package h\n\nimport "testing"\n\nfunc TestH(t *testing.T) {}\n' > "$r2/internal/h/h_test.go"
printf 'data\n' > "$r2/internal/h/testdata/x.txt"
printf 'package f\n\nimport "testing"\n\nfunc TestOne(t *testing.T) {\n\t_ = 1\n}\n\nfunc TestTwo(t *testing.T) {}\n' > "$r2/internal/f/f_test.go"
printf 'notes\n' > "$r2/tools/x/notes.txt"
g2 add -A && g2 commit -q -m base
base2=$(g2 rev-parse HEAD)
run2() { out=$(cd "$r2" && "$script" "$base2" 2>&1); rc=$?; }
reset2() { g2 reset -q --hard "$base2" && g2 clean -qfd; }

# A removed init, var or helper declaration runs the package whole, named with BASE's line.
perl -0pi -e 's/func init\(\) \{ counter = 1 \}\n\n//' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"whole: ./internal/g reason=removed-init internal/g/g_test.go:7"* && $out != *"run:"* ]] || fail "removed init runs whole: rc=$rc out=$out"
reset2
perl -0pi -e 's/var counter int\n\n//' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"whole: ./internal/g reason=removed-counter internal/g/g_test.go:5"* ]] || fail "removed var runs whole: rc=$rc out=$out"
reset2
perl -0pi -e 's/func helper\(\) int \{\n\treturn counter\n\}\n\n//' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"whole: ./internal/g reason=removed-helper internal/g/g_test.go:9"* ]] || fail "removed helper runs whole: rc=$rc out=$out"
reset2
# A removed test or benchmark selects nothing of its own; a removed line inside a surviving
# helper or test narrows as an edit does.
perl -0pi -e 's/func TestG2\(t \*testing.T\) \{\n\t_ = 2\n\}\n\n//' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out != *"internal/g"* && $out == *" whole=0 "* ]] || fail "removed test selects nothing: rc=$rc out=$out"
reset2
perl -0pi -e 's/\nfunc BenchmarkG\(b \*testing.B\) \{\n\tfor range b.N \{\n\t\}\n\}\n//' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out != *"internal/g"* ]] || fail "removed benchmark selects nothing: rc=$rc out=$out"
reset2
perl -0pi -e 's/\n\treturn counter\n/\n/' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"run: script/heavy.sh go test -count=1 -run '^(TestG1)\$' ./internal/g
  why TestG1 internal/g/g_test.go:9 changed via helper"* ]] || fail "removed helper line narrows: rc=$rc out=$out"
reset2
perl -0pi -e 's/\n\t\tt.Fatal\("init did not run"\)\n/\n/' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"run: script/heavy.sh go test -count=1 -run '^(TestG1)\$' ./internal/g
  why TestG1 internal/g/g_test.go:14 changed"* ]] || fail "removed test line narrows: rc=$rc out=$out"
reset2
# A deleted test file runs the package whole, unstaged, staged and committed.
deleted_whole="whole: ./internal/g reason=deleted-test-file internal/g/g_test.go"
/bin/rm -f "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"$deleted_whole"* ]] || fail "deleted test file (unstaged): rc=$rc out=$out"
g2 rm -q internal/g/g_test.go
run2
[[ $rc == 0 && $out == *"$deleted_whole"* ]] || fail "deleted test file (staged): rc=$rc out=$out"
g2 commit -q -m delete
run2
[[ $rc == 0 && $out == *"$deleted_whole"* ]] || fail "deleted test file (committed): rc=$rc out=$out"
reset2
# An uncovered line of an untracked test file (a type declaration) runs the package whole.
printf 'package g\n\nimport "testing"\n\ntype fixture struct{ a int }\n\nfunc TestNew(t *testing.T) {}\n' > "$r2/internal/g/new_test.go"
run2
[[ $rc == 0 && $out == *"whole: ./internal/g reason=changed-top-level internal/g/new_test.go:5"* && $out != *"isn't numeric"* ]] || fail "untracked file type runs whole: rc=$rc out=$out"
reset2

# ASD-V0-5134: the same hunks under diff.noprefix and diff.mnemonicPrefix, and a test file whose
# path holds a space and a non-ASCII byte (quoted by Git's default core.quotepath).
g2 config diff.noprefix true
g2 config diff.mnemonicPrefix true
perl -0pi -e 's/\n\t\tt.Fatal\("init did not run"\)\n/\n/' "$r2/internal/g/g_test.go"
run2
[[ $rc == 0 && $out == *"why TestG1 internal/g/g_test.go:14 changed"* ]] || fail "noprefix hunks: rc=$rc out=$out"
reset2
printf 'package g\n\nimport "testing"\n\nfunc TestCafe(t *testing.T) {}\n' > "$r2/internal/g/caf é_test.go"
run2
[[ $rc == 0 && $out == *"-run '^(TestCafe)\$' ./internal/g"* && $out == *"why TestCafe internal/g/caf é_test.go:5 changed"* ]] || fail "untracked quoted path: rc=$rc out=$out"
g2 add -A && g2 commit -q -m cafe
printf 'package g\n\nimport "testing"\n\nfunc TestCafe(t *testing.T) { _ = 1 }\n' > "$r2/internal/g/caf é_test.go"
run2
[[ $rc == 0 && $out == *"why TestCafe internal/g/caf é_test.go:5 changed"* && $out == *" whole=0 "* ]] || fail "edited quoted path: rc=$rc out=$out"
reset2

# ASD-V0-5135: non-Go package inputs.
printf 'TEXT\n' > "$r2/internal/h/asm.s"
run2
[[ $rc == 0 && $out == *"whole: ./internal/h reason=source-edit internal/h/asm.s"* ]] || fail "changed .s runs whole: rc=$rc out=$out"
reset2
printf 'fixture\n' > "$r2/internal/h/fixture.txt"
printf 'changed\n' > "$r2/internal/h/testdata/x.txt"
printf 'changed\n' > "$r2/tools/x/notes.txt"
testless_whole="whole: ./tools/x"
run2
[[ $rc == 0 && $out == *"whole: ./internal/h reason=package-file internal/h/fixture.txt"* && $out != *"$testless_whole"* && $out == *" packages=1 "* ]] || fail "package file runs whole, testdata and testless do not: rc=$rc out=$out"
reset2
printf 'changed\n' > "$r2/internal/h/testdata/x.txt"
run2
[[ $rc == 0 && $out == *" packages=0 "* ]] || fail "testdata alone selects nothing: rc=$rc out=$out"
reset2
perl -0pi -e 's/\t_ = 1\n/\t_ = 2\n/' "$r2/internal/f/f_test.go"
run2
[[ $rc == 0 && $out == *"-run '^(TestOne)\$' ./internal/f"* ]] || fail "narrowed control before go.mod: rc=$rc out=$out"
printf '\n// toolchain note\n' >> "$r2/go.mod"
run2
[[ $rc == 0 && $out == *"whole: ./internal/f reason=module-edit go.mod"* && $out != *"run:"* && $out == *" packages=1 "* ]] || fail "go.mod turns the narrowed package whole: rc=$rc out=$out"
g2 checkout -q -- internal/f
run2
[[ $rc == 0 && $out == *" packages=0 "* ]] || fail "go.mod alone selects nothing: rc=$rc out=$out"
reset2

out=$(cd "$r" && "$script" no-such-rev 2>&1); rc=$?
[[ $rc == 2 && $out == *'REFUSE unknown-base no-such-rev; fix:'* ]] || fail "bad base: rc=$rc out=$out"
out=$(cd "$r" && "$script" 2>&1); rc=$?
[[ $rc == 2 && $out == *'REFUSE usage:'* ]] || fail "usage: rc=$rc out=$out"

[[ $failed == 0 ]] || exit 1
echo "test-run-pattern_test.sh: ok"
