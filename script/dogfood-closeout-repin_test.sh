#!/usr/bin/env bash
# ASD-V0-568 (V1-4101): the closeout repin step commits a purely mechanical repin and refreshes the
# plan before batch-precheck, so the aff-w16 shape (main moved a cited span; the precheck's repin
# rewrote it and then failed the seal) passes in one run; a failed repin still fails with its
# recovery, commits nothing and restores HEAD, the index and the tree, even when it staged or
# committed its rewrite. The repinner is one of a fixed set run by its literal path; any other value
# is refused before anything runs. Temporary Git fixtures only.
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
# The repinners run by their literal paths from the repository root (ASD-V0-568), so each case writes
# its fake there, untracked so that no commit holds it; a fake run with other arguments exits 9.
mkdir script || exit 2
mechanical='script/batch-regen.sh --mechanical'
# fake PATH ARG BODY writes the repinner at PATH, which exits 9 unless run with exactly ARG.
fake() { printf '#!/bin/sh\n[ "$*" = %s ] || exit 9\n%s\n' "$2" "$3" > "$1" && chmod +x "$1"; }
# A fake repin that moves the citation, and a fake corvint whose refresh appends a row.
fake script/batch-regen.sh --mechanical 'printf "see spec.md:4\n" > doc.md'
printf '#!/bin/sh\n[ "$1 $2 $3" = "dogfood propose-citations --refresh" ] || exit 9\ncat "$4"; printf "2\\tdoc.md\\t1:1\\trepin\\n"\n' > "$tmp/corvint" && chmod +x "$tmp/corvint"

# 1. A mechanical repin is committed alone and the plan is refreshed, keeping the previous plan.
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical CORVINT_BIN=$tmp/corvint "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "mechanical repin: rc=$? $out"
[[ $(git rev-list --count "$base..HEAD") == 1 ]] || fail "the repin was not one commit: $out"
[[ $(git log -1 --format=%s) == 'chore(repin): mechanical repin before the closeout (V1-4101)' ]] || fail "commit subject"
[[ $(git diff --name-only HEAD~1 HEAD) == doc.md ]] || fail "the commit holds more than the repinned file"
git diff --quiet HEAD -- || fail "the tree is dirty after the repin commit"
grep -q 'repin$' "$tmp/plan.tsv" || fail "the plan was not refreshed: $(cat "$tmp/plan.tsv")"
[[ $(cat "$tmp/plan.tsv.pre-repin") == $'1\tdoc.md\t1:1\timplementation' ]] || fail "the previous plan was not kept"
grep -q '^dogfood-closeout-repin: committed ' <<< "$out" || fail "no committed line: $out"

# 2. A second run has nothing to repin and changes nothing.
head=$(git rev-parse HEAD)
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical CORVINT_BIN=$tmp/corvint "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "no-op run: rc=$?"
[[ $(git rev-parse HEAD) == "$head" && $out == 'dogfood-closeout-repin: nothing to repin' ]] || fail "no-op run changed something: $out"

# 3. A failed repin exits 1 with its output and recovery, commits nothing and restores the tree.
fake script/batch-regen.sh --mechanical 'printf "x\n" >> spec.md; echo "line-citations: changed span spec.md:2" >&2; exit 1'
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical "$script" "$base" "$tmp/plan.tsv" 2>&1)
rc=$?
[[ $rc == 1 && $(git rev-parse HEAD) == "$head" ]] || fail "failed repin: rc=$rc $out"
grep -qF '  | line-citations: changed span spec.md:2' <<< "$out" && grep -qF "fix: run $mechanical, fix what it names" <<< "$out" || fail "failed repin output: $out"
git diff --quiet HEAD -- || fail "the failed repin left tracked changes"

# 3b. A failed repin that staged its rewrite, as check-line-citations.sh --repin does, or committed
# it, leaves HEAD, the index and the tree at the starting commit.
fake script/batch-regen.sh --mechanical 'printf "see spec.md:9\n" > doc.md && git add doc.md; exit 1'
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical "$script" "$base" "$tmp/plan.tsv" 2>&1)
rc=$?
[[ $rc == 1 && $(git rev-parse HEAD) == "$head" ]] || fail "staged failed repin: rc=$rc $out"
git diff --quiet HEAD -- && git diff --cached --quiet HEAD -- || fail "the staged failed repin left changes: $(git status --short)"
fake script/batch-regen.sh --mechanical 'printf "see spec.md:8\n" > doc.md && git commit -q -am partial; exit 1'
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical "$script" "$base" "$tmp/plan.tsv" 2>&1)
rc=$?
[[ $rc == 1 && $(git rev-parse HEAD) == "$head" ]] || fail "committed failed repin: rc=$rc $out"
git diff --quiet HEAD -- && git diff --cached --quiet HEAD -- || fail "the committed failed repin left changes: $(git status --short)"

# 3c. A restore that fails exits 1 naming the starting commit and the recovery.
fake script/batch-regen.sh --mechanical 'printf "see spec.md:7\n" > doc.md && git add doc.md && : > "$(git rev-parse --git-dir)/index.lock"; exit 1'
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical "$script" "$base" "$tmp/plan.tsv" 2>&1)
rc=$?
[[ $rc == 1 ]] || fail "failed restore: rc=$rc $out"
grep -qF "dogfood-closeout-repin: FAIL could not restore HEAD, the index and the tracked tree to $head" <<< "$out" &&
  grep -qF "fix: git reset -q $head && git checkout -q -- :/" <<< "$out" || fail "failed restore output: $out"
/bin/rm -f .git/index.lock && git reset -q --hard "$head" || exit 2

# 4. Uncommitted tracked changes skip the step; batch-precheck names them.
printf 'dirty\n' >> spec.md
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "dirty tree: rc=$?"
grep -q 'skipped: tracked changes are uncommitted' <<< "$out" && [[ $(git rev-parse HEAD) == "$head" ]] || fail "dirty tree: $out"
git checkout -q -- spec.md

# 5. An unset repin command skips the step.
out=$(env -u DOGFOOD_CLOSEOUT_REGEN "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "unset command: rc=$?"
grep -q 'skipped' <<< "$out" && [[ $(git rev-parse HEAD) == "$head" ]] || fail "unset command: $out"

# 5b. A value outside the fixed set exits 2 before anything runs, naming the allowed values: a
# computed path, and an allowed script with other arguments.
printf '#!/bin/sh\n: > "%s/ran"\n' "$tmp" > "$tmp/other" && chmod +x "$tmp/other"
fake script/batch-regen.sh --mechanical ': > "$0.ran"'
for other in "$tmp/other" 'script/batch-regen.sh' 'script/batch-regen.sh --mechanical --no-commit'; do
  out=$(DOGFOOD_CLOSEOUT_REGEN=$other "$script" "$base" "$tmp/plan.tsv" 2>&1)
  rc=$?
  [[ $rc == 2 && $(git rev-parse HEAD) == "$head" && ! -e $tmp/ran && ! -e script/batch-regen.sh.ran ]] || fail "refused value $other: rc=$rc $out"
  grep -qF "dogfood-closeout-repin: REFUSE DOGFOOD_CLOSEOUT_REGEN=$other is not an allowed repinner" <<< "$out" &&
    grep -qF "fix: set DOGFOOD_CLOSEOUT_REGEN to 'script/batch-regen.sh --mechanical' (the default) or 'script/check-line-citations.sh --repin'" <<< "$out" || fail "refused value output: $out"
done

# 5c. The line-citation repinner is the other allowed value; it runs by its own literal path.
fake script/check-line-citations.sh --repin 'printf "see spec.md:5\n" > doc.md'
out=$(DOGFOOD_CLOSEOUT_REGEN='script/check-line-citations.sh --repin' CORVINT_BIN=$tmp/corvint "$script" "$base" "$tmp/plan.tsv" 2>&1) || fail "line-citation repin: rc=$? $out"
[[ $(git rev-list --count "$head..HEAD") == 1 && $(git show HEAD:doc.md) == 'see spec.md:5' ]] || fail "line-citation repin: $out"
head=$(git rev-parse HEAD)

# 5d. ASD-V0-3731 (review): the plan refresh fails once after the repin commit. The rerun has nothing
# left to repin, yet still runs the pending refresh and reports success only after it; a third run is
# a plain no-op.
# The flaky refresh fails on its first call only.
printf '#!/bin/sh\n[ "$1 $2 $3" = "dogfood propose-citations --refresh" ] || exit 9\necho call >> "%s/refresh.calls"\n[ "$(wc -l < "%s/refresh.calls")" -ne 1 ] || { echo "refresh: boom" >&2; exit 1; }\ncat "$4"; printf "3\\tdoc.md\\t1:1\\tretried\\n"\n' "$tmp" "$tmp" > "$tmp/corvint-flaky" && chmod +x "$tmp/corvint-flaky"
printf '1\tdoc.md\t1:1\timplementation\n' > "$tmp/plan-retry.tsv"
fake script/batch-regen.sh --mechanical 'printf "see spec.md:6\n" > doc.md'
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical CORVINT_BIN=$tmp/corvint-flaky "$script" "$base" "$tmp/plan-retry.tsv" 2>&1)
rc=$?
[[ $rc == 1 && $(git rev-list --count "$head..HEAD") == 1 ]] || fail "failed refresh: rc=$rc $out"
grep -qF 'dogfood-closeout-repin: FAIL the plan refresh failed; the repin is committed' <<< "$out" || fail "failed refresh output: $out"
[[ $(cat "$tmp/plan-retry.tsv") == $'1\tdoc.md\t1:1\timplementation' ]] || fail "the failed refresh changed the plan"
head=$(git rev-parse HEAD)
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical CORVINT_BIN=$tmp/corvint-flaky "$script" "$base" "$tmp/plan-retry.tsv" 2>&1)
rc=$?
[[ $rc == 0 && $(git rev-parse HEAD) == "$head" ]] || fail "refresh retry: rc=$rc $out"
[[ $(wc -l < "$tmp/refresh.calls") == 2 ]] || fail "the rerun did not retry the refresh: $out"
grep -q 'retried$' "$tmp/plan-retry.tsv" && [[ $(cat "$tmp/plan-retry.tsv.pre-repin") == $'1\tdoc.md\t1:1\timplementation' ]] || fail "the retried refresh did not install the plan: $out"
grep -qF "dogfood-closeout-repin: nothing to repin; retrying the plan refresh the repin commit $head left pending" <<< "$out" &&
  [[ $(tail -n 1 <<< "$out") == "dogfood-closeout-repin: refreshed $tmp/plan-retry.tsv for the repin hunks (previous plan: $tmp/plan-retry.tsv.pre-repin)" ]] || fail "refresh retry output: $out"
out=$(DOGFOOD_CLOSEOUT_REGEN=$mechanical CORVINT_BIN=$tmp/corvint-flaky "$script" "$base" "$tmp/plan-retry.tsv" 2>&1) || fail "after the retry: rc=$?"
[[ $out == 'dogfood-closeout-repin: nothing to repin' && $(wc -l < "$tmp/refresh.calls") == 2 ]] || fail "the retried refresh stayed pending: $out"

# 6. Usage.
"$script" > /dev/null 2>&1
[[ $? == 2 ]] || fail "no BASE is not a usage error"
printf 'dogfood-closeout-repin_test: ok\n'
