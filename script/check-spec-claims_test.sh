#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# LVT-V0-010: unstaged and untracked long Claims fail early, including Unicode characters.
set -eu
source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/spec-claims-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$scratch/script" "$scratch/docs/specs"
cp "$source_root/script/check-spec-claims.sh" "$scratch/script/"
git -C "$scratch" init -q
cd "$scratch"
claim() { perl -CS -e 'print "# Spec\n\n## Agent digest\n- Claim: ", "a" x $ARGV[0], ".\n"' "$1" > docs/specs/one.md; }
claim 159
git add docs/specs/one.md
script/check-spec-claims.sh
claim 160
if script/check-spec-claims.sh > out 2>&1; then echo 'FAIL: unstaged long Claim passed' >&2; exit 1; fi
grep -q 'docs/specs/one.md:4: Agent digest Claim is 161 characters' out
mv docs/specs/one.md docs/specs/untracked.md
if script/check-spec-claims.sh > out 2>&1; then echo 'FAIL: untracked long Claim passed' >&2; exit 1; fi
grep -q 'docs/specs/untracked.md:4:' out
perl -CS -e 'print "## Agent digest\n- Claim: ", chr(0xE9) x 159, ".\n\n## Body\n- Claim: ", "x" x 200, ".\n"' > docs/specs/untracked.md
script/check-spec-claims.sh
# LVT-V0-010: indented headings and Claim bullets count, as the generator reads them.
perl -CS -e 'print "# Spec\n\n   ## Agent digest  \n  - Claim: ", "b" x 170, ".\n- Status: proposed\n"' > docs/specs/untracked.md
if script/check-spec-claims.sh > out 2>&1; then echo 'FAIL: indented long Claim passed' >&2; exit 1; fi
grep -q 'docs/specs/untracked.md:4: Agent digest Claim is 171 characters' out
perl -CS -e 'print "# Spec\n\n\t## Agent digest\n\t- Claim: ", "c" x 159, ".\n"' > docs/specs/untracked.md
script/check-spec-claims.sh
# A Claim after the digest's first blank line is not the generator's Claim.
perl -CS -e 'print "## Agent digest\n- Claim: Short.\n\n- Claim: ", "d" x 200, ".\n"' > docs/specs/untracked.md
script/check-spec-claims.sh
printf 'check-spec-claims_test.sh: ok\n'
