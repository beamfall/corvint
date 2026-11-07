#!/usr/bin/env bash
# ASD-V0-1021..1024 (V1-3959, V1-3668): script/review-head-check.sh classifies a pinned review's
# findings against a moved lane head, passes a head that changed only .corvint/, refuses an unpinned
# review and a reviewed head the lane head does not contain; script/dogfood-seal.sh with
# DOGFOOD_REVIEW refuses a stale review before its check runs. Fixture repositories only.
set -uo pipefail
export LC_ALL=C

here=$(cd "$(dirname "$0")" && pwd)
script=$here/review-head-check.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-review-head-check-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
failed=0
fail() {
  printf 'review-head-check_test: FAIL %s\n' "$*" >&2
  failed=1
}
g() { git -C "$tmp/repo" -c user.name=t -c user.email=t@t "$@"; }
check() { (cd "$tmp/repo" && "$script" "$@" 2>&1); }

git init -q "$tmp/repo" || exit 1
mkdir -p "$tmp/repo/script" "$tmp/repo/internal/x" "$tmp/repo/docs"
seq 1 20 | sed 's/^/line /' > "$tmp/repo/script/a.sh"
seq 1 20 | sed 's/^/code /' > "$tmp/repo/internal/x/b.go"
printf 'doc\n' > "$tmp/repo/docs/c.md"
g add -A && g commit -q -m reviewed
reviewed=$(g rev-parse HEAD)
# The reviewed head pinned the way script/codex-review.sh prints it (ASD-V0-1020).
cat > "$tmp/review.txt" << EOF
codex-review: reviewed-head=$reviewed tree=$(g rev-parse 'HEAD^{tree}') dirty=no
Findings:
- [P2] The tail refusal drops a line silently. \`script/a.sh:3\`
  It also reads \`internal/x/b.go:2-4\`.
- [P2] The range tie-break is unstable (internal/x/b.go:2-4).
- [P1] The guide is out of date: docs/c.md
- [P3] General naming nit, no location.
- [P2] Whole-file finding about script/a.sh.
EOF
grep -v '^codex-review:' "$tmp/review.txt" > "$tmp/unpinned.txt"

# HEAD changed only .corvint/: the review still covers it.
mkdir -p "$tmp/repo/.corvint" && printf '{}\n' > "$tmp/repo/.corvint/change.cem.json"
g add -A && g commit -q -m bind
out=$(check "$tmp/review.txt"); rc=$?
[[ $rc == 0 && $out == "review-head-check: CURRENT reviewed=$reviewed head=$(g rev-parse HEAD)" ]] ||
  fail "bind-only head: rc=$rc out=$out"

# A fix round moved line 3 of a.sh and line 15 of b.go.
sed -i.bak '3s/.*/line three fixed/' "$tmp/repo/script/a.sh" && sed -i.bak '15s/.*/code fifteen/' "$tmp/repo/internal/x/b.go"
/bin/rm -f "$tmp/repo/script/a.sh.bak" "$tmp/repo/internal/x/b.go.bak"
g add -A && g commit -q -m fix
out=$(check "$tmp/review.txt"); rc=$?
want="finding 1 POSSIBLY_FIXED cited=script/a.sh:3,internal/x/b.go:2-4 evidence=lines-changed,other-lines-changed :: - [P2] The tail refusal drops a line silently. \`script/a.sh:3\`
finding 2 OPEN cited=internal/x/b.go:2-4 evidence=other-lines-changed :: - [P2] The range tie-break is unstable (internal/x/b.go:2-4).
finding 3 OPEN cited=docs/c.md evidence=unchanged :: - [P1] The guide is out of date: docs/c.md
finding 4 UNKNOWN cited=none evidence=no-cited-path :: - [P3] General naming nit, no location.
finding 5 POSSIBLY_FIXED cited=script/a.sh evidence=path-changed :: - [P2] Whole-file finding about script/a.sh.
review-head-check: MOVED reviewed=$reviewed head=$(g rev-parse HEAD) changed=2 findings=5 possibly-fixed=2 open=2 unknown=1
  changed: internal/x/b.go
  changed: script/a.sh"
[[ $rc == 1 && $out == "$want"* && $out == *'fix: send only OPEN and UNKNOWN findings'* ]] || fail "moved head: rc=$rc out=$out"

# An inserted line after a cited line touches it.
printf 'inserted\n' >> "$tmp/repo/docs/c.md"
printf 'codex-review: reviewed-head=%s tree=x dirty=yes\n- [P2] see docs/c.md:1\n' "$(g rev-parse HEAD)" > "$tmp/dirty.txt"
g add -A && g commit -q -m insert
out=$(check "$tmp/dirty.txt"); rc=$?
[[ $rc == 1 && $out == *'NOTE the review read uncommitted changes'* && $out == *'finding 1 POSSIBLY_FIXED cited=docs/c.md:1 evidence=lines-changed'* ]] ||
  fail "insertion after a cited line: rc=$rc out=$out"

out=$(check "$tmp/unpinned.txt"); rc=$?
[[ $rc == 2 && $out == *'REFUSE unpinned-review'* && $out == *'fix: run the review through script/codex-review.sh'* ]] ||
  fail "unpinned review: rc=$rc out=$out"
out=$(check --reviewed-head "$reviewed" "$tmp/unpinned.txt"); rc=$?
[[ $rc == 1 && $out == *'possibly-fixed=3 open=1 unknown=1'* ]] || fail "--reviewed-head: rc=$rc out=$out"
printf 'codex-review: reviewed-head=NOT_PRODUCED not-a-git-checkout\n- [P2] x\n' > "$tmp/notproduced.txt"
out=$(check "$tmp/notproduced.txt"); rc=$?
[[ $rc == 2 && $out == *'REFUSE pin-not-produced'* ]] || fail "NOT_PRODUCED pin: rc=$rc out=$out"
# The lane head does not contain the reviewed head (the cem-u11 shape: a review of a later seal
# checked against an earlier lane head).
out=$(check "$tmp/dirty.txt" "$reviewed"); rc=$?
[[ $rc == 2 && $out == *'REFUSE reviewed-head-not-ancestor'* ]] || fail "not an ancestor: rc=$rc out=$out"
out=$(check "$tmp/missing.txt"); rc=$?
[[ $rc == 2 && $out == *'REFUSE unreadable-review'* ]] || fail "missing review: rc=$rc out=$out"

# ASD-V0-1024: dogfood-seal.sh with DOGFOOD_REVIEW runs the check first and refuses a moved head
# before dogfood-check runs; a current review lets the stub check run.
seal=$tmp/sealrepo
git init -q "$seal" && mkdir "$seal/script"
/bin/cp -f "$here/dogfood-seal.sh" "$here/review-head-check.sh" "$seal/script/"
printf '#!/usr/bin/env bash\necho check-ran >> "%s/check.log"\nexit 7\n' "$tmp" > "$seal/script/dogfood-check.sh"
chmod 0755 "$seal/script/"*.sh
printf 'x\n' > "$seal/f" && git -C "$seal" add -A && git -C "$seal" -c user.name=t -c user.email=t@t commit -q -m one
printf 'codex-review: reviewed-head=%s tree=x dirty=no\n' "$(git -C "$seal" rev-parse HEAD)" > "$tmp/seal-review.txt"
printf 'y\n' > "$seal/f" && git -C "$seal" -c user.name=t -c user.email=t@t commit -q -am two
: > "$tmp/check.log"
out=$(cd "$seal" && DOGFOOD_REVIEW=$tmp/seal-review.txt script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
[[ $rc == 2 && $out == *'dogfood-seal: REFUSE review-not-current'* && ! -s $tmp/check.log ]] ||
  fail "seal with a stale review: rc=$rc out=$out check=$(cat "$tmp/check.log")"
printf 'codex-review: reviewed-head=%s tree=x dirty=no\n' "$(git -C "$seal" rev-parse HEAD)" > "$tmp/seal-review.txt"
out=$(cd "$seal" && DOGFOOD_REVIEW=$tmp/seal-review.txt script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
[[ $rc == 7 && $out == *'review-head-check: CURRENT'* && $(cat "$tmp/check.log") == check-ran ]] ||
  fail "seal with a current review: rc=$rc out=$out"
: > "$tmp/check.log"
out=$(cd "$seal" && script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
[[ $rc == 7 && $out == *'dogfood-seal: NOTE review-before-seal NOT_OBSERVED'* ]] || fail "seal without a review: rc=$rc out=$out"

# DWC-V0-046: an orchestrator's external report binds exact HEAD plus verdict.
sealhead=$(git -C "$seal" rev-parse HEAD)
printf 'dogfood-review: reviewed-head=%s verdict=PASS\nExternal review report.\n' "$sealhead" > "$tmp/external-review.txt"
out=$(cd "$seal" && DOGFOOD_REVIEW=$tmp/external-review.txt script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
[[ $rc == 7 && $out == *"review-before-seal OBSERVED"* && $out == *"reviewed-head=$sealhead"* && $out == *'verdict=PASS'* ]] || fail "external review: rc=$rc out=$out"
printf 'dogfood-review: reviewed-head=%s verdict=FAIL\n' "$sealhead" > "$tmp/external-review.txt"
: > "$tmp/check.log"
out=$(cd "$seal" && DOGFOOD_REVIEW=$tmp/external-review.txt script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
[[ $rc == 2 && ! -s $tmp/check.log ]] || fail "nonpassing external review ran check: $out"
printf 'dogfood-review: reviewed-head=%s verdict=PASS\n' "$(git -C "$seal" rev-parse HEAD~1)" > "$tmp/external-review.txt"
out=$(cd "$seal" && DOGFOOD_REVIEW=$tmp/external-review.txt script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
[[ $rc == 2 && $out == *'external-review-head-mismatch'* && ! -s $tmp/check.log ]] || fail "wrong external head: $out"
# DWC-V0-046 review fix: a prose mention of the key is neither a second header nor a switch to
# the external format, in an external report or in a legacy one.
printf 'dogfood-review: reviewed-head=%s verdict=PASS\nThe parser recognizes `dogfood-review:` headers.\n' "$sealhead" > "$tmp/prose-review.txt"
out=$(cd "$seal" && "$script" "$tmp/prose-review.txt" 2>&1); rc=$?
[[ $rc == 0 && $out == "review-head-check: CURRENT reviewed=$sealhead head=$sealhead verdict=PASS" ]] || fail "external report with a prose mention: rc=$rc out=$out"
printf 'codex-review: reviewed-head=%s tree=x dirty=no\nThe parser recognizes `dogfood-review:` headers.\n' "$sealhead" > "$tmp/prose-review.txt"
out=$(cd "$seal" && "$script" "$tmp/prose-review.txt" 2>&1); rc=$?
[[ $rc == 0 && $out == "review-head-check: CURRENT reviewed=$sealhead head=$sealhead" ]] || fail "legacy report with a prose mention: rc=$rc out=$out"
printf 'dogfood-review: reviewed-head=%s verdict=PASS\ndogfood-review: reviewed-head=%s verdict=PASS\n' "$sealhead" "$sealhead" > "$tmp/prose-review.txt"
out=$(cd "$seal" && "$script" "$tmp/prose-review.txt" 2>&1); rc=$?
[[ $rc == 2 && $out == *'invalid-external-review'* ]] || fail "two header declarations accepted: rc=$rc out=$out"
# Finish a fixture seal, asserting durable review evidence and a rename-only tree.
mkdir -p "$seal/.corvint"
printf 'bound evidence\n' > "$seal/.corvint/change.cem.json"
git -C "$seal" add -A && git -C "$seal" -c user.name=t -c user.email=t@t commit -q -m bind
sealhead=$(git -C "$seal" rev-parse HEAD)
printf 'dogfood-review: reviewed-head=%s verdict=PASS\nExternal review.\n' "$sealhead" > "$tmp/external-review.txt"
printf '#!/usr/bin/env bash\nexit 0\n' > "$seal/script/dogfood-check.sh"
# Restore the stub in the tracked tree so the seal stays rename-only.
git -C "$seal" add script/dogfood-check.sh && git -C "$seal" -c user.name=t -c user.email=t@t commit -q -m check-stub
sealhead=$(git -C "$seal" rev-parse HEAD)
printf 'dogfood-review: reviewed-head=%s verdict=PASS\nExternal review.\n' "$sealhead" > "$tmp/external-review.txt"
# P2-1 / DWC-V0-046: an external header wrapping legacy output records only
# the validated external head, even when the nested pin names another commit.
printf 'codex-review: reviewed-head=%s tree=x dirty=no\n' "$(git -C "$seal" rev-parse HEAD~1)" >> "$tmp/external-review.txt"
out=$(cd "$seal" && GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t DOGFOOD_REVIEW=$tmp/external-review.txt script/dogfood-seal.sh HEAD~1 2>&1); rc=$?
message=$(git -C "$seal" log -1 --format=%B)
changes=$(git -C "$seal" diff --name-status -M HEAD^ HEAD)
[[ $rc == 0 && $out == *'dogfood-seal: PASS'* && $message == *"review-before-seal OBSERVED"* && $message == *"reviewed-head=$sealhead"* && $message == *'verdict=PASS' && $(printf '%s\n' "$message" | wc -l) -eq 3 && $changes == R100$'\t'*.cem.json$'\t'*.cem.json ]] || fail "durable external review seal: rc=$rc out=$out message=$message changes=$changes"

# DWC-V0-046: legacy reports allow a metadata-only descendant but carry no verdict.
legacyhead=$(git -C "$seal" rev-parse HEAD)
printf 'bound again\n' > "$seal/.corvint/change.cem.json"
git -C "$seal" add .corvint/change.cem.json && git -C "$seal" -c user.name=t -c user.email=t@t commit -q -m rebind
bindhead=$(git -C "$seal" rev-parse HEAD)
printf 'codex-review: reviewed-head=%s tree=x dirty=no\n' "$legacyhead" > "$tmp/legacy-review.txt"
cat "$tmp/legacy-review.txt" "$tmp/legacy-review.txt" > "$tmp/duplicate-legacy.txt"
out=$(cd "$seal" && DOGFOOD_REVIEW=$tmp/duplicate-legacy.txt script/dogfood-seal.sh "$legacyhead" 2>&1); rc=$?
[[ $rc == 2 && $out == *'invalid-legacy-review duplicate pins'* && $(git -C "$seal" rev-parse HEAD) == "$bindhead" ]] || fail "duplicate legacy pins: rc=$rc out=$out"
out=$(cd "$seal" && GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t DOGFOOD_REVIEW=$tmp/legacy-review.txt script/dogfood-seal.sh "$legacyhead" 2>&1); rc=$?
message=$(git -C "$seal" log -1 --format=%B)
changes=$(git -C "$seal" diff --name-status -M HEAD^ HEAD)
[[ $rc == 0 && $out == *'dogfood-seal: PASS'* && $message == *"reviewed-head=$legacyhead"* && $message == *'verdict=NOT_OBSERVED' && $(printf '%s\n' "$message" | wc -l) -eq 3 && $changes == R100$'\t'*.cem.json$'\t'*.cem.json ]] || fail "legacy descendant seal: rc=$rc out=$out message=$message changes=$changes"

[[ $failed == 0 ]] || exit 1
echo "review-head-check_test.sh: ok"
