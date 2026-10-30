#!/bin/sh
set -eu

# Cases on a fixture repository: a clean tracked docs/decisions set passes, and a
# `git ls-files` failure is refused loudly instead of being swallowed by the pipe and
# reported as zero collisions found (plain `sh` has no `pipefail`, so a pipeline's exit
# status is its last command's, not the enumeration's); missing and duplicate index rows
# fail; and parallel README appends union-merge cleanly under .gitattributes (DNG-V0-009).

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-decision-numbers-test.XXXXXX")
fakebin=$(mktemp -d "${TMPDIR:-/tmp}/corvint-decision-numbers-fakebin.XXXXXX")
cleanup() { rm -rf "$test_root" "$fakebin"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/decisions"
cp "$source_root/script/check-decision-numbers.sh" "$test_root/script/"

git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-decision-numbers.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

readme="$test_root/docs/decisions/README.md"
write_readme() {
    printf '# Decision records index\n\n' > "$readme"
    printf '| [`0002-second-2026-09-01.md`](0002-second-2026-09-01.md) | accepted | fixture |\n' >> "$readme"
    printf '| [`0001-first-2026-09-01.md`](0001-first-2026-09-01.md) | accepted | fixture |\n' >> "$readme"
}

# 1. A clean tracked set with no collisions, fully and uniquely indexed, passes and says nothing.
printf '# one\n' > "$test_root/docs/decisions/0001-first-2026-09-01.md"
printf '# two\n' > "$test_root/docs/decisions/0002-second-2026-09-01.md"
write_readme
git -C "$test_root" add -A
git -C "$test_root" commit -qm fixture
output=$(run) || fail "a clean tracked set did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 2. A `git ls-files` failure is refused loudly, never reported as a passing check.
cat > "$fakebin/git" <<'EOF'
#!/bin/sh
if [ "$1" = "ls-files" ]; then
    echo "fatal: fake git ls-files failure" >&2
    exit 128
fi
exec /usr/bin/git "$@"
EOF
chmod +x "$fakebin/git"

status=0
output=$(cd "$test_root" && PATH="$fakebin:$PATH" script/check-decision-numbers.sh 2>&1) || status=$?
test "$status" -ne 0 || fail "a git ls-files failure passed the gate: $output"
case $output in
    *ls-files*) ;;
    *) fail "the failure did not name git ls-files: $output" ;;
esac

# 3. A README index missing a tracked decision file's row is refused as missing.
printf '| [`0002-second-2026-09-01.md`](0002-second-2026-09-01.md) | accepted | fixture |\n' > "$readme"
status=0
output=$(run) || status=$?
test "$status" -ne 0 || fail "a missing index row passed the gate: $output"
case $output in
    *missing*0001-first-2026-09-01.md*) ;;
    *) fail "the failure did not name the missing row: $output" ;;
esac

# 4. A README index row cited twice for the same file is refused as a duplicate.
write_readme
printf '| [`0001-first-2026-09-01.md`](0001-first-2026-09-01.md) | accepted | fixture |\n' >> "$readme"
status=0
output=$(run) || status=$?
test "$status" -ne 0 || fail "a duplicate index row passed the gate: $output"
case $output in
    *duplicate*0001-first-2026-09-01.md*) ;;
    *) fail "the failure did not name the duplicate row: $output" ;;
esac

# Restore a clean README so any later case reusing this fixture starts from a passing state.
write_readme

# 5. DNG-V0-009: with the repository's .gitattributes, two branches that each add a decision
# file, its README row at the same place and a build-log entry merge with no conflict, and the
# merged index passes the gate.
cp "$source_root/.gitattributes" "$test_root/.gitattributes"
git -C "$test_root" add -A
git -C "$test_root" commit -qm attributes
start=$(git -C "$test_root" rev-parse HEAD)
add_decision() { # branch file build-log-entry
    git -C "$test_root" checkout -q -b "$1" "$start"
    printf '# %s\n' "$2" > "$test_root/docs/decisions/$2"
    mkdir -p "$test_root/docs/build-log"
    printf '# %s\n' "$3" > "$test_root/docs/build-log/$3"
    { sed -n '1,2p' "$readme"
      printf '| [`%s`](%s) | accepted | fixture |\n' "$2" "$2"
      sed '1,2d' "$readme"; } > "$readme.new"
    mv -f "$readme.new" "$readme"
    git -C "$test_root" add -A
    git -C "$test_root" commit -qm "$1"
}
add_decision lane-a 0003-third-2026-10-24.md 2026-10-24-third.md
add_decision lane-b 0004-fourth-2026-10-24.md 2026-10-24-fourth.md
git -C "$test_root" checkout -q lane-a
git -C "$test_root" merge -q --no-edit lane-b >/dev/null 2>&1 || fail "parallel decision rows did not merge cleanly: $(git -C "$test_root" status --short)"
grep -q '0003-third-2026-10-24.md' "$readme" && grep -q '0004-fourth-2026-10-24.md' "$readme" ||
    fail "the union merge dropped a row: $(cat "$readme")"
output=$(run) || fail "the union-merged index did not pass: $output"

# 6. A union merge of two branches that each claimed the same decision number keeps both rows,
# and the gate refuses the colliding number rather than accepting it silently (V1-0265).
add_decision lane-c 0005-fifth-a-2026-10-24.md 2026-10-24-fifth-a.md
add_decision lane-d 0005-fifth-b-2026-10-24.md 2026-10-24-fifth-b.md
git -C "$test_root" checkout -q lane-c
git -C "$test_root" merge -q --no-edit lane-d >/dev/null 2>&1 || fail "colliding decision rows did not union-merge: $(git -C "$test_root" status --short)"
status=0
output=$(run) || status=$?
test "$status" -ne 0 || fail "a union-merged number collision passed the gate: $output"
case $output in
    *0005-fifth-a-2026-10-24.md*0005-fifth-b-2026-10-24.md*) ;;
    *) fail "the failure did not name both colliding files: $output" ;;
esac

printf 'check-decision-numbers_test: 6 cases passed\n'
