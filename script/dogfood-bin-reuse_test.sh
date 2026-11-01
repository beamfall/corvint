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
source "$source_root/script/dogfood-fixture-scripts.sh"
dogfood_fixture_copy "$source_root" "$repo/script" dogfood-change.sh dogfood-check.sh
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
printf '%s\n' "$$" > "$TEST_PID_LOG"
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
export TEST_FAKE_CORVINT="$root/bin/corvint" TEST_BUILD_LOG="$root/build.log" TEST_FLOW_LOG="$root/flow.log" TEST_PID_LOG="$root/pid.log"
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
# expect_builds_with NAME=VALUE ACTION COUNT CASE runs expect_builds with NAME=VALUE exported for that
# run only. Bash in POSIX mode (run as sh, as macOS /bin/sh is) keeps a prefix assignment to a
# function call after it returns, so `NAME=VALUE expect_builds` would leak into every later case
# (V1-2557).
expect_builds_with() {
  (export "$1"; shift; expect_builds "$@") || exit 1
}
tree_bin=.git/corvint/corvint

expect_builds change 1 'first change'
expect_builds change 0 'unchanged change'
grep -q '^# test-version$' "$tree_bin" || exit 1
# The base verifier is keyed by the base commit's root tree; the tree verifier is already recorded.
expect_builds check 1 'first check'
expect_builds check 0 'unchanged check'

# DCW-V0-942 (V1-3625): the check wrapper writes its setup and build rows, with reuse or build, bound
# to the process ID the exec'd check runs as.
# Bash before 5 (macOS /bin/bash 3.2) has no EPOCHREALTIME, so its wrapper writes no build file.
# expect_build_rows GITDIR TREE BASE checks the wrapper's build file under GITDIR, or its absence
# under such a bash.
has_epoch=$(bash -c 'printf %s "${EPOCHREALTIME:+yes}"')
expect_build_rows() {
  local file="$1/corvint/dogfood-check.build.$(cat "$TEST_PID_LOG").tsv"
  if [[ -z $has_epoch ]]; then
    [[ ! -e $file ]] || { printf 'a bash without EPOCHREALTIME wrote build timings at %s\n' "$file" >&2; exit 1; }
    return 0
  fi
  [[ -f $file ]] || { printf 'no build timings at %s\n' "$file" >&2; exit 1; }
  awk -v pid="$(cat "$TEST_PID_LOG")" -v tree="$2" -v base="$3" -F '\t' '
    NR == 1 { ok = $0 == "corvint-dogfood-check-build/0" }
    NR == 2 { ok = ok && $0 == "pid " pid }
    NR == 3 { ok = ok && $0 ~ /^started [0-9]+\.[0-9]+$/ }
    NR == 4 { ok = ok && $1 == "setup" && $2 == "-" && $3 ~ /^[0-9]+\.[0-9]{3}$/ }
    NR == 5 { ok = ok && $1 == "build-tree" && $2 == tree && $3 ~ /^[0-9]+\.[0-9]{3}$/ }
    NR == 6 { ok = ok && $1 == "build-base" && $2 == base && $3 ~ /^[0-9]+\.[0-9]{3}$/ }
    END { exit !(ok && NR == 6) }' "$file" || { printf 'bad build timings (want %s/%s):\n' "$2" "$3" >&2; cat "$file" >&2; exit 1; }
}
expect_build_rows .git reuse reuse
# The same on any host: a wrapper whose bash lacks EPOCHREALTIME (simulated by unsetting it, which
# drops its special meaning, as Bash 3.2 never had it) reuses both builds and writes no build file.
/bin/rm -f .git/corvint/dogfood-check.build.*.tsv
: > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
bash -c 'unset EPOCHREALTIME; . script/dogfood-check.sh "$1"' script/dogfood-check.sh "$base" > "$root/output" 2>&1 ||
  { cat "$root/output" >&2; exit 1; }
[[ ! -s $TEST_BUILD_LOG && -s $TEST_FLOW_LOG ]] || { printf 'no-EPOCHREALTIME check rebuilt or did not run\n' >&2; exit 1; }
for leftover in .git/corvint/dogfood-check.build.*.tsv; do
  [[ ! -e $leftover ]] || { printf 'a wrapper without EPOCHREALTIME wrote %s\n' "$leftover" >&2; exit 1; }
done
/bin/rm -rf "$CORVINT_DOGFOOD_BIN_CACHE"
expect_builds check 2 'cold check'
expect_build_rows .git build build
# DCW-V0-943 (V1-3627): a second worktree on the same BASE reuses the base verifier the first built.
git worktree add -q --detach "$root/second" HEAD
(cd "$root/second" && expect_builds check 0 'second worktree') || exit 1
expect_build_rows "$repo/.git/worktrees/second" reuse reuse
git worktree remove --force "$root/second"
# Untracked and ignored files outside the build inputs do not matter.
printf 'edited\n' > docs/notes.md
mkdir -p out && printf 'x\n' > out/ignored
expect_builds change 0 'dirty non-input'
git checkout -q -- docs/notes.md

# A modified, staged, untracked or ignored build input always rebuilds, and never records.
printf 'package lib // edited\n' > internal/lib/lib.go
expect_builds change 1 'modified input'
expect_builds change 1 'modified input again'
grep -q '^# package lib // edited$' "$tree_bin" || exit 1
git add internal/lib/lib.go
expect_builds change 1 'staged input'
git reset -q && git checkout -q -- internal/lib/lib.go
expect_builds change 0 'restored input'
grep -qx '# package lib' "$tree_bin" || exit 1
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
grep -q '^# test-version-3$' "$tree_bin" && grep -q '^# package lib // committed$' "$tree_bin" || exit 1
expect_builds_with TEST_GOVERSION=go1.27.3 change 1 'new Go version'
grep -q '^# go1.27.3$' "$tree_bin" || exit 1
expect_builds_with TEST_GOVERSION=go1.27.3 change 0 'new Go version again'
[[ -z ${TEST_GOVERSION+x} ]] || { printf 'TEST_GOVERSION leaked past its case\n' >&2; exit 1; }
expect_builds change 0 'previous Go version'
grep -q '^# go1.27.2$' "$tree_bin" || exit 1
# A doc-only commit keeps the build inputs, so it reuses.
printf 'more\n' >> docs/notes.md
git -c user.name=test -c user.email=test@example.invalid commit -qam docs
expect_builds change 0 'doc-only commit'

# The C toolchain and its flags are key inputs.
expect_builds_with TEST_CC=clang change 1 'new C compiler'
expect_builds_with TEST_CC=clang change 0 'new C compiler again'
# A workspace file outside the keyed objects leaves no key, so it always builds.
expect_builds_with TEST_GOWORK="$root/go.work" change 1 'outside workspace'
expect_builds_with TEST_GOWORK="$root/go.work" change 1 'outside workspace again'

# Any failure to compute or verify the key falls back to a build.
expect_builds_with TEST_LIST_FAIL=1 change 1 'key unavailable'
for entry in "$CORVINT_DOGFOOD_BIN_CACHE"/*/corvint; do printf 'tampered\n' >> "$entry"; done
expect_builds change 1 'tampered record'
expect_builds check 1 'tampered base record'
expect_builds check 0 'rerecorded'
git rm -q go.mod
git -c user.name=test -c user.email=test@example.invalid commit -qm no-module
expect_builds change 1 'no go.mod'
expect_builds change 1 'no go.mod again'

# DCW-V0-943: two concurrent writers over eight keys, with a bound of two, leave only verified
# entries; a later store brings the cache back within the bound.
cache="$root/bounded"
store_loop() {
  (
    set +e
    repo=$repo go_cache=$root/go-cache CORVINT_DOGFOOD_BIN_CACHE=$cache CORVINT_DOGFOOD_BIN_CACHE_MAX=2
    source "$source_root/script/dogfood-build-corvint.sh"
    for ((i = 0; i < 16; i++)); do
      key=$(printf '%040d' $(((i + $1) % 8)))
      printf 'binary %s %s\n' "$key" "$1" > "$root/out.$1"
      dogfood_corvint_store "$root/out.$1" "$key"
      dogfood_corvint_reuse "$root/reused.$1" "$key" || :
    done
  )
}
store_loop 0 &
writer_a=$!
store_loop 1 &
writer_b=$!
wait "$writer_a" && wait "$writer_b" || { printf 'a concurrent writer failed\n' >&2; exit 1; }
for entry in "$cache"/*; do
  [[ -f $entry/corvint && -f $entry/corvint.oid ]] || continue
  [[ $(git hash-object "$entry/corvint") == "$(cat "$entry/corvint.oid")" ]] ||
    { printf 'unverified entry after concurrent writers: %s\n' "$entry" >&2; exit 1; }
done
entries=$(find "$cache" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
((entries <= 4)) || { printf 'the bound of 2 left %s entries with two writers\n' "$entries" >&2; exit 1; }
store_loop 2
entries=$(find "$cache" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
((entries <= 2)) || { printf 'the bound of 2 left %s entries after a serial store\n' "$entries" >&2; exit 1; }
printf 'DCW-V0-074 dogfood build reuse: PASS\n'
