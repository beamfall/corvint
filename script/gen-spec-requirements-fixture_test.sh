#!/bin/sh
set -eu

# SRG-V0-681 (V1-3388): the fixture layout documented in the header of gen-spec-requirements.sh is
# enough. One staged spec with the two header lines, a one-sentence digest Claim and a requirement
# generates REQUIREMENTS.tsv; --readme without a staged README.md fails naming the layout pointer;
# adding the documented README table row, whose description starts with the claim, lets --write
# regenerate all three files; a spec without the header lines is refused naming the spec.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-gen-spec-requirements-fixture-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
stage() { git -C "$test_root" add -A; }
gen() { (cd "$test_root" && script/gen-spec-requirements.sh "$@" 2>&1); }

mkdir -p "$test_root/script" "$test_root/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$test_root/script/"
git -C "$test_root" init -q -b main
grep -q '^# Fixture layout (SRG-V0-681' "$test_root/script/gen-spec-requirements.sh" ||
  fail "the fixture layout is not documented in the script header"

# shellcheck disable=SC2016 # the backticks are Markdown literals
printf '# Fix\n\nIntent status: proposed\nDelivery status: not-started\n\n## Agent digest\n- Claim: Fix is a fixture.\n\n## Requirements\n\n- `FIX-V0-001`: The only clause.\n' \
  > "$test_root/docs/specs/fix-v0.md"
stage
out=$(gen) || fail "the documented spec layout did not generate REQUIREMENTS.tsv: $out"
case $out in *'FIX-V0-001	docs/specs/fix-v0.md	The only clause.'*) ;; *) fail "the clause row is missing: $out" ;; esac

rc=0
out=$(gen --readme) || rc=$?
[ "$rc" != 0 ] || fail "--readme passed without a staged README.md: $out"
case $out in
  *'docs/specs/README.md is not in the Git index; --readme, --all and --write need it staged (the fixture layout is in the header of script/gen-spec-requirements.sh)'*) ;;
  *) fail "the missing README.md was not named with the layout: $out" ;;
esac

printf '# Specs\n\n| Scope | Contract | Intent | Delivery | Description |\n|---|---|---|---|---|\n| Fix | [fix-v0.md](fix-v0.md) | x | y | Old claim. More prose. |\n' \
  > "$test_root/docs/specs/README.md"
: > "$test_root/docs/specs/REQUIREMENTS.tsv"
printf '[]\n' > "$test_root/docs/specs/INDEX.json"
stage
out=$(gen --write) || fail "--write failed on the documented layout: $out"
stage
out=$(gen --check) || fail "--check failed after --write: $out"
grep -qx '| Fix | \[fix-v0.md\](fix-v0.md) | proposed | not-started | Fix is a fixture. More prose. |' "$test_root/docs/specs/README.md" ||
  fail "--write did not project the header cells and the claim: $(cat "$test_root/docs/specs/README.md")"

# shellcheck disable=SC2016 # the backticks are Markdown literals
printf '# Bare\n\n## Agent digest\n- Claim: Bare has no header.\n\n## Requirements\n\n- `BAR-V0-001`: A clause.\n' \
  > "$test_root/docs/specs/bare-v0.md"
stage
rc=0
out=$(gen --index) || rc=$?
[ "$rc" != 0 ] || fail "a spec without Intent and Delivery status lines passed: $out"
case $out in *'docs/specs/bare-v0.md: declare Intent status and Delivery status before regenerating'*) ;; *) fail "the bare spec was not named: $out" ;; esac

printf 'gen-spec-requirements-fixture_test.sh: ok\n'
