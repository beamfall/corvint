#!/bin/sh
set -eu

# Covers SRG-V0-001 (cases 1-3) and SRG-V0-005 (case 4). SRG-V0-001: the generated index must be
# the one a fresh clone reproduces, so the enumeration is the Git index and nothing the worktree
# alone says can change the output. The three cases are the drift directions reproduced in
# `docs/specs/spec-requirement-index-generator-v0.md`: an untracked spec, a tracked spec
# deleted from the worktree alone, and an unstaged edit to a tracked spec.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-requirements-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$test_root/script/"
git -C "$test_root" init -q -b main

stage() { git -C "$test_root" -c user.name=t -c user.email=t@example.invalid add -- "$1"; }

write_spec() {
  # shellcheck disable=SC2016
  printf '# %s\n\n- `%s`: %s\n' "$1" "$2" "$3" > "$test_root/docs/specs/$1.md"
}

run() { (cd "$test_root" && script/gen-spec-requirements.sh); }

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

write_spec alpha ALPHA-V0-001 'the tracked clause'
stage docs/specs/alpha.md
baseline=$(run)

case "$baseline" in
  *'ALPHA-V0-001	docs/specs/alpha.md	3	the tracked clause'*) ;;
  *) fail "baseline did not index the tracked spec: $baseline" ;;
esac

# 1. An untracked spec is not in a fresh clone, so it must contribute no row.
write_spec beta BETA-V0-001 'the untracked clause'
actual=$(run)
test "$actual" = "$baseline" || fail "an untracked spec changed the index: $actual"
rm "$test_root/docs/specs/beta.md"

# 2. A tracked spec deleted from the worktree alone is still in the index, so its rows stay.
rm "$test_root/docs/specs/alpha.md"
actual=$(run)
test "$actual" = "$baseline" || fail "a worktree-only deletion dropped tracked rows: $actual"
write_spec alpha ALPHA-V0-001 'the tracked clause'

# 3. An unstaged edit is not published yet; `git add` is what changes the index.
write_spec alpha ALPHA-V0-001 'the unstaged clause'
actual=$(run)
test "$actual" = "$baseline" || fail "an unstaged edit changed the index: $actual"
stage docs/specs/alpha.md
actual=$(run)
test "$actual" != "$baseline" || fail "a staged edit did not change the index"

# docs/specs/README.md is the index of specs, not a spec, and is excluded even when tracked.
write_spec README README-V0-001 'the excluded clause'
stage docs/specs/README.md
case "$(run)" in
  *README-V0-001*) fail "docs/specs/README.md was indexed" ;;
esac

# 4. SRG-V0-005 truncates titles to 80 characters, not 80 bytes: a multibyte title keeps all 80
# characters and never ends in a split UTF-8 sequence.
long=$(printf '\303\251%.0s' $(seq 1 90))
write_spec gamma GAMMA-V0-001 "$long"
stage docs/specs/gamma.md
want=$(printf '\303\251%.0s' $(seq 1 80))
actual=$(run | grep '^GAMMA-V0-001	' | cut -f4)
test "$actual" = "$want" || fail "a multibyte title was not truncated to 80 characters: $actual"

printf 'ok\n'
