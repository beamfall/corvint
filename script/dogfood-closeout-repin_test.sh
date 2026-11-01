#!/usr/bin/env bash
# ASD-V0-568 (V1-4101): the closeout repin step commits a purely mechanical repin and refreshes the
# plan before batch-precheck, so the aff-w16 shape (main moved a cited span; the precheck's repin
# rewrote it and then failed the seal) passes in one run; a failed repin still fails with its
# recovery and commits nothing. Temporary Git fixtures only.
set -uo pipefail
script=$(cd "$(dirname "$0")" && pwd)/dogfood-closeout-repin.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/closeout-repin-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'dogfood-closeout-repin_test: FAIL %s\n' "$1" >&2; exit 1; }
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
repo=$tmp/repo
git init -q "$repo" && cd "$repo" || exit 2
git config core.hooksPath /dev/null
printf 'see spec.md:3\n' > doc.md
printf 'a\nb\nc\n' > spec.md
git add . && git commit -q -m base || exit 2
base=$(git rev-parse HEAD)
printf '1\tdoc.md\t1:1\timplementation\n' > "$tmp/plan.tsv"
# A fake repin that moves the citation, and a fake corvint whose refresh appends a row.
printf '#!/bin/sh\nprintf "see spec.md:4\\n" > doc.md\n' > "$tmp/regen" && chmod +x "$tmp/regen"
printf '#!/bin/sh\n[ "$1 $2 $3" = "dogfood propose-citations --refresh" ] || exit 9\ncat "$4"; printf "2\\tdoc.md\\t1:1\\trepin\\n"\n' > "$tmp/corvint" && chmod +x "$tmp/corvint"

# 1. A mechanical repin is committed alone and the plan is refreshed, keeping the previous plan.
out=$(DOGFOOD_CLOSEOUT_REGEN=$tmp/regen CORVINT_BIN=$tmp/corvint "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "mechanical repin: rc=$? $out"
[[ $(git rev-list --count "$base..HEAD") == 1 ]] || fail "the repin was not one commit: $out"
[[ $(git log -1 --format=%s) == 'chore(repin): mechanical repin before the closeout (V1-4101)' ]] || fail "commit subject"
[[ $(git diff --name-only HEAD~1 HEAD) == doc.md ]] || fail "the commit holds more than the repinned file"
git diff --quiet HEAD -- || fail "the tree is dirty after the repin commit"
grep -q 'repin$' "$tmp/plan.tsv" || fail "the plan was not refreshed: $(cat "$tmp/plan.tsv")"
[[ $(cat "$tmp/plan.tsv.pre-repin") == $'1\tdoc.md\t1:1\timplementation' ]] || fail "the previous plan was not kept"
grep -q '^dogfood-closeout-repin: committed ' <<< "$out" || fail "no committed line: $out"

# 2. A second run has nothing to repin and changes nothing.
head=$(git rev-parse HEAD)
out=$(DOGFOOD_CLOSEOUT_REGEN=$tmp/regen CORVINT_BIN=$tmp/corvint "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "no-op run: rc=$?"
[[ $(git rev-parse HEAD) == "$head" && $out == 'dogfood-closeout-repin: nothing to repin' ]] || fail "no-op run changed something: $out"

# 3. A failed repin exits 1 with its output and recovery, commits nothing and restores the tree.
printf '#!/bin/sh\nprintf "x\\n" >> spec.md\necho "line-citations: changed span spec.md:2" >&2\nexit 1\n' > "$tmp/bad" && chmod +x "$tmp/bad"
out=$(DOGFOOD_CLOSEOUT_REGEN=$tmp/bad "$script" "$base" "$tmp/plan.tsv" 2>&1)
rc=$?
[[ $rc == 1 && $(git rev-parse HEAD) == "$head" ]] || fail "failed repin: rc=$rc $out"
grep -qF '  | line-citations: changed span spec.md:2' <<< "$out" && grep -q '^fix: run script/batch-regen.sh --mechanical' <<< "$out" || fail "failed repin output: $out"
git diff --quiet HEAD -- || fail "the failed repin left tracked changes"

# 4. Uncommitted tracked changes skip the step; batch-precheck names them.
printf 'dirty\n' >> spec.md
out=$(DOGFOOD_CLOSEOUT_REGEN=$tmp/regen "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "dirty tree: rc=$?"
grep -q 'skipped: tracked changes are uncommitted' <<< "$out" && [[ $(git rev-parse HEAD) == "$head" ]] || fail "dirty tree: $out"
git checkout -q -- spec.md

# 5. Usage.
"$script" > /dev/null 2>&1
[[ $? == 2 ]] || fail "no BASE is not a usage error"
printf 'dogfood-closeout-repin_test: ok\n'
