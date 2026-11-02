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

[[ $failed == 0 ]] || exit 1
echo "review-head-check_test.sh: ok"
