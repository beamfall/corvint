#!/usr/bin/env bash
# Covers SRG-V0-764 (V1-3389): the next free ID comes from the highest staged definition, not the
# last line of a spec, and each out-of-order definition is named by file and line.
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-requirement-next-id-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
mkdir -p "$test_root/script" "$test_root/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$source_root/script/requirement-next-id.sh" "$test_root/script/"
git -C "$test_root" init -q -b main
# shellcheck disable=SC2016
printf -- '- `ORD-V0-001`: one.\n- `ORD-V0-007`: seven.\n- `ORD-V0-003`: three, after seven.\n- `ORD-V0-008`: eight.\n- `ORD-V0-005`: five, after eight.\n' > "$test_root/docs/specs/ord.md"
# shellcheck disable=SC2016
printf -- '- `ORD-V0-002`: two, in another spec.\n- `ORDX-V0-050`: another prefix.\n' > "$test_root/docs/specs/other.md"
git -C "$test_root" add -- docs/specs
out=$(cd "$test_root" && script/requirement-next-id.sh ORD-V0)
want='next ORD-V0-009 after ORD-V0-008 docs/specs/ord.md:4
out-of-order ORD-V0-003 docs/specs/ord.md:3 after ORD-V0-007 line 2
out-of-order ORD-V0-005 docs/specs/ord.md:5 after ORD-V0-008 line 4
out-of-order=2'
[[ $out == "$want" ]] || fail "unexpected report: $out"
out=$(cd "$test_root" && script/requirement-next-id.sh NEW-V0)
[[ $out == $'next NEW-V0-001 none-defined\nout-of-order=0' ]] || fail "an unused prefix printed: $out"
if (cd "$test_root" && script/requirement-next-id.sh bad 2> /dev/null); then fail "a bad prefix was accepted"; else test $? = 2 || fail "a bad prefix did not exit 2"; fi
printf 'ok\n'
