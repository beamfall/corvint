#!/usr/bin/env bash
# Covers SRG-V0-680 (V1-3769): script/spec-obligation-headroom.sh warns from 240 obligations,
# refuses over 256 with exit 1, counts as `ocm prepare` does (a fenced `##` line does not end the
# section, lines after the section and a spec with a second `## Requirements` heading do not
# count), reads the Git index, and the `spec-requirements` recipe runs it before the generator
# writes.
# shellcheck disable=SC2016 # backticks are Markdown literals here
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-headroom-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
repo="$test_root/repo"
mkdir -p "$repo/script" "$repo/docs/specs"
cp "$source_root/script/spec-obligation-headroom.sh" "$repo/script/"
git -C "$repo" init -q -b main

# spec NAME COUNT writes a spec whose ## Requirements section holds COUNT obligations, one of them
# in bold form and one after a fenced `##` line, plus an ID after the section that never counts,
# and stages it.
spec() {
  {
    printf '# %s\n\n## Requirements\n\n' "$1"
    for ((i = 1; i <= $2; i++)); do
      if [ "$i" = 2 ]; then
        printf '```text\n## Fenced, not a heading\n```\n'
      fi
      if [ "$i" = 1 ]; then
        printf -- '- **HDR-V0-%03d:** bold form.\n' "$i"
      else
        printf -- '- `HDR-V0-%03d`: requirement %d.\n' "$i" "$i"
      fi
    done
    printf '\n## Traceability\n\n- `OUT-V0-001`: after the section, not counted.\n'
  } > "$repo/docs/specs/$1.md"
  git -C "$repo" add -- "docs/specs/$1.md"
}
run() { (cd "$repo" && script/spec-obligation-headroom.sh 2>&1); }

spec small 239
output=$(run) || fail "239 obligations failed: $output"
[ -z "$output" ] || fail "239 obligations printed output: $output"

spec edge 240
output=$(run) || fail "240 obligations failed: $output"
case $output in
*'docs/specs/edge.md: warning: its ## Requirements section holds 240 of the 256 obligations'*'companion spec'*'(SRG-V0-680)'*) ;;
*) fail "240 obligations did not warn with the companion-spec route: $output" ;;
esac
case $output in *small.md*) fail "239 obligations warned: $output" ;; esac

spec full 256
output=$(run) || fail "256 obligations failed: $output"
case $output in *'full.md: warning: its ## Requirements section holds 256 of the 256'*) ;; *) fail "256 did not warn: $output" ;; esac

# An unstaged edit past the limit is not read: the generator and this check read the Git index.
spec over 257
git -C "$repo" rm -q --cached -- docs/specs/over.md
output=$(run) || fail "an untracked over-limit spec was read: $output"
git -C "$repo" add -- docs/specs/over.md
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "257 obligations exited $rc, not 1: $output"
case $output in
*'docs/specs/over.md: its ## Requirements section holds 257 obligations, over the 256'*'refused before writing'*'companion spec'*) ;;
*) fail "257 obligations were not refused naming the spec and the route: $output" ;;
esac

# A spec with two ## Requirements headings is not an OCM intent (`ocm prepare` refuses it for
# another reason), so it is not counted.
{ cat "$repo/docs/specs/over.md"; printf '\n## Requirements\n\n- `TWO-V0-001`: second section.\n'; } > "$repo/docs/specs/two.md"
git -C "$repo" rm -q --cached -- docs/specs/over.md
git -C "$repo" add -- docs/specs/two.md
output=$(run) || fail "a spec with two Requirements headings was counted: $output"
case $output in *two.md*) fail "a spec with two Requirements headings was named: $output" ;; esac

# The recipe runs the check before the generator writes, so an over-limit spec writes nothing.
recipe=$(sed -n '/^spec-requirements:/,/^$/p' "$source_root/Makefile")
check_line=$(printf '%s\n' "$recipe" | grep -n 'script/spec-obligation-headroom.sh' | cut -d: -f1)
write_line=$(printf '%s\n' "$recipe" | grep -n 'gen-spec-requirements.sh --write' | cut -d: -f1)
[ -n "$check_line" ] && [ -n "$write_line" ] && [ "$check_line" -lt "$write_line" ] ||
  fail "the spec-requirements recipe does not run the headroom check before --write: $recipe"

echo "spec-obligation-headroom tests passed"
