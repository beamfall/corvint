#!/usr/bin/env bash
# DCW-V0-074: the dogfood wrappers reuse a keyed corvint build only while every key input matches
# and the current tree's build inputs are clean; anything else rebuilds. A stub `go` stands in for
# the toolchain, so no real Go build runs and no cache outside this test's directory is written.
# Bash 3.2 does not apply errexit to [[ ]] in loops; assertions need explicit exits.
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd)
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-bin-reuse-test.XXXXXX")
trap '/bin/rm -rf "$root"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
repo="$root/repo"
mkdir -p "$repo/script" "$repo/cmd/corvint" "$repo/internal/lib" "$repo/docs" "$root/bin"
/bin/cp -f "$source_root"/script/dogfood-{change,check,build-corvint}.sh "$repo/script/"
printf 'module example.invalid/corvint\n' > "$repo/go.mod"
printf 'test-version\n' > "$repo/VERSION"
printf 'package main\n' > "$repo/cmd/corvint/main.go"
printf 'package lib\n' > "$repo/internal/lib/lib.go"
printf 'notes\n' > "$repo/docs/notes.md"
printf 'out/\n' > "$repo/.gitignore"
git -C "$repo" init -q
git -C "$repo" add .
git -C "$repo" -c user.name=test -c user.email=test@example.invalid commit -qm fixture
base=$(git -C "$repo" rev-parse HEAD)
cat > "$root/bin/corvint" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$TEST_FLOW_LOG"
EOF
# The stub's build output is the build's inputs, so a stale reuse would be visible in its bytes.
cat > "$root/bin/go" <<'EOF'
#!/usr/bin/env bash
set -eu
case $1 in
  env)
    shift
    for name in "$@"; do
      case $name in
        GOVERSION) printf '%s\n' "${TEST_GOVERSION:-go1.27.2}" ;;
        GOOS) printf 'linux\n' ;;
        GOARCH) printf 'amd64\n' ;;
        CGO_ENABLED) printf '0\n' ;;
        GOFLAGS) printf '%s\n' "${GOFLAGS-}" ;;
        GOWORK) printf '%s\n' "${TEST_GOWORK-}" ;;
        CC) printf '%s\n' "${TEST_CC-}" ;;
        *) printf '\n' ;;
      esac
    done ;;
  list)
    [[ ${TEST_LIST_FAIL-} != 1 ]] || exit 1
    printf 'true example.invalid/corvint example.invalid/corvint/cmd/corvint\n'
    printf 'true example.invalid/corvint example.invalid/corvint/internal/lib\n' ;;
  build)
    [[ $2 == -o && $4 == -trimpath && $5 == ./cmd/corvint && $GOFLAGS == *-buildvcs=false ]] || exit 1
    printf 'build\n' >> "$TEST_BUILD_LOG"
    { cat "$TEST_FAKE_CORVINT"; { cat VERSION internal/lib/lib.go; printf '%s\n' "${TEST_GOVERSION:-go1.27.2}"; } |
      sed 's/^/# /'; } > "$3"
    chmod +x "$3" ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$root/bin/"* "$repo/script/"*.sh
export TEST_FAKE_CORVINT="$root/bin/corvint" TEST_BUILD_LOG="$root/build.log" TEST_FLOW_LOG="$root/flow.log"
export PATH="$root/bin:$PATH" CORVINT_DOGFOOD_BIN_CACHE="$root/bin-cache"
unset CORVINT_BIN TEST_GOVERSION TEST_LIST_FAIL TEST_GOWORK TEST_CC GOFLAGS
cd "$repo"

# expect_builds ACTION COUNT CASE runs one wrapper and checks how many builds it ran.
expect_builds() {
  : > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
  bash "script/dogfood-$1.sh" "$base" > "$root/output" 2>&1 || { cat "$root/output" >&2; exit 1; }
  local builds
  builds=$(wc -l < "$TEST_BUILD_LOG" | tr -d ' ')
  [[ $builds == "$2" ]] || { printf '%s: expected %s builds, got %s\n' "$3" "$2" "$builds" >&2; exit 1; }
  [[ -s $TEST_FLOW_LOG ]] || { printf '%s: the flow did not run\n' "$3" >&2; exit 1; }
}
tree_bin=.git/corvint/corvint

expect_builds change 1 'first change'
expect_builds change 0 'unchanged change'
rg -q '^# test-version$' "$tree_bin" || exit 1
# The base verifier is keyed by the base commit's root tree; the tree verifier is already recorded.
expect_builds check 1 'first check'
expect_builds check 0 'unchanged check'
# Untracked and ignored files outside the build inputs do not matter.
printf 'edited\n' > docs/notes.md
mkdir -p out && printf 'x\n' > out/ignored
expect_builds change 0 'dirty non-input'
git checkout -q -- docs/notes.md

# A modified, staged, untracked or ignored build input always rebuilds, and never records.
printf 'package lib // edited\n' > internal/lib/lib.go
expect_builds change 1 'modified input'
expect_builds change 1 'modified input again'
rg -q '^# package lib // edited$' "$tree_bin" || exit 1
git add internal/lib/lib.go
expect_builds change 1 'staged input'
git reset -q && git checkout -q -- internal/lib/lib.go
expect_builds change 0 'restored input'
rg -qx '# package lib' "$tree_bin" || exit 1
printf 'package lib\n' > internal/lib/extra.go
expect_builds change 1 'untracked input'
/bin/rm -f internal/lib/extra.go
mkdir -p internal/lib/out && printf 'x\n' > internal/lib/out/ignored
expect_builds change 1 'ignored input'
/bin/rm -rf internal/lib/out
printf 'test-version-2\n' > VERSION
expect_builds change 1 'modified VERSION'
git checkout -q -- VERSION

# A committed input or VERSION change, or another Go version, is a new key.
printf 'package lib // committed\n' > internal/lib/lib.go
git -c user.name=test -c user.email=test@example.invalid commit -qam input
expect_builds change 1 'committed input'
expect_builds change 0 'committed input again'
printf 'test-version-3\n' > VERSION
git -c user.name=test -c user.email=test@example.invalid commit -qam version
expect_builds change 1 'committed VERSION'
rg -q '^# test-version-3$' "$tree_bin" && rg -q '^# package lib // committed$' "$tree_bin" || exit 1
TEST_GOVERSION=go1.27.3 expect_builds change 1 'new Go version'
rg -q '^# go1.27.3$' "$tree_bin" || exit 1
TEST_GOVERSION=go1.27.3 expect_builds change 0 'new Go version again'
expect_builds change 0 'previous Go version'
rg -q '^# go1.27.2$' "$tree_bin" || exit 1
# A doc-only commit keeps the build inputs, so it reuses.
printf 'more\n' >> docs/notes.md
git -c user.name=test -c user.email=test@example.invalid commit -qam docs
expect_builds change 0 'doc-only commit'

# The C toolchain and its flags are key inputs.
TEST_CC=clang expect_builds change 1 'new C compiler'
TEST_CC=clang expect_builds change 0 'new C compiler again'
# A workspace file outside the keyed objects leaves no key, so it always builds.
TEST_GOWORK="$root/go.work" expect_builds change 1 'outside workspace'
TEST_GOWORK="$root/go.work" expect_builds change 1 'outside workspace again'

# Any failure to compute or verify the key falls back to a build.
TEST_LIST_FAIL=1 expect_builds change 1 'key unavailable'
for entry in "$CORVINT_DOGFOOD_BIN_CACHE"/*/corvint; do printf 'tampered\n' >> "$entry"; done
expect_builds change 1 'tampered record'
expect_builds check 1 'tampered base record'
expect_builds check 0 'rerecorded'
git rm -q go.mod
git -c user.name=test -c user.email=test@example.invalid commit -qm no-module
expect_builds change 1 'no go.mod'
expect_builds change 1 'no go.mod again'
printf 'DCW-V0-074 dogfood build reuse: PASS\n'
