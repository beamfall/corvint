#!/usr/bin/env bash
# Temp-fixture test for script/pre-push-dogfood-seal.sh (DCW-V0-348, V1-2805, V1-1773): real
# `git push` runs through the documented hook install, with a stub batch check that passes only
# while a fixture "sealed" file names the pushed HEAD.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
hook="$here/pre-push-dogfood-seal.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/pre-push-seal-test.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
trap '/bin/rm -rf "$tmp"' EXIT INT TERM
fail() { printf 'pre-push-dogfood-seal_test: FAIL %s\n' "$*" >&2; exit 1; }
export GIT_CONFIG_NOSYSTEM=1 HOME="$tmp/home" GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid \
  GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
mkdir -p "$HOME"
git config --global init.defaultBranch main
q() { git "$@" > /dev/null 2>&1; }

q init --bare "$tmp/origin.git"
work="$tmp/work"
q clone "$tmp/origin.git" "$work"
mkdir -p "$work/script"
/bin/cp -f "$hook" "$work/script/pre-push-dogfood-seal.sh"
# The stub batch check logs its base and passes only when sealed.txt names HEAD, standing in for
# the corvint dogfood batch-check that fails unbound-commits for a commit after the seal.
cat > "$work/script/dogfood-batch-check.sh" << 'EOF'
#!/usr/bin/env bash
echo "batch-check base=$1" >> batch.log
[[ $(cat sealed.txt 2> /dev/null) == $(git rev-parse HEAD) ]] && { echo "dogfood-batch-check: PASS"; exit 0; }
echo "dogfood-batch-check: FAIL unbound-commits count=1"
exit 1
EOF
chmod +x "$work/script/dogfood-batch-check.sh"
hooks=$(git -C "$work" rev-parse --path-format=absolute --git-path hooks)
mkdir -p "$hooks"
printf '#!/bin/sh\nexec script/pre-push-dogfood-seal.sh "$@"\n' > "$hooks/pre-push"
chmod +x "$hooks/pre-push"
printf 'batch.log\nsealed.txt\nscript/\n' > "$work/.gitignore"
echo base > "$work/a.txt"
q -C "$work" add -A && q -C "$work" commit -m base
# The first push has no origin/main yet: no merge base, so it is refused, and --no-verify seeds it.
if git -C "$work" push origin main > "$tmp/out0" 2>&1; then fail "push without a merge base was accepted"; fi
grep -q 'REFUSE dogfood-seal-not-checked refs/heads/main has no merge base with origin/main' "$tmp/out0" || { cat "$tmp/out0" >&2; fail "no merge-base refusal"; }
q -C "$work" push --no-verify origin main || fail "seed push failed"
base=$(git -C "$work" rev-parse HEAD)
q -C "$work" checkout -b batch

# 1. A batch head with no sealed CEM is refused and does not reach the remote (V1-2805).
echo one >> "$work/a.txt" && q -C "$work" commit -am one
if git -C "$work" push origin batch > "$tmp/out1" 2>&1; then fail "unsealed push was accepted"; fi
grep -q "REFUSE dogfood-seal-missing range=$base\.\.$(git -C "$work" rev-parse HEAD)" "$tmp/out1" || { cat "$tmp/out1" >&2; fail "no dogfood-seal-missing refusal"; }
grep -q 'unbound-commits count=1' "$tmp/out1" || { cat "$tmp/out1" >&2; fail "batch check output not shown"; }
grep -qx "batch-check base=$base" "$work/batch.log" || fail "batch check did not receive the merge base"
git -C "$tmp/origin.git" rev-parse -q --verify refs/heads/batch > /dev/null && fail "refused branch reached the remote"

# 2. Once sealed, the same head pushes.
git -C "$work" rev-parse HEAD > "$work/sealed.txt"
git -C "$work" push origin batch > "$tmp/out2" 2>&1 || { cat "$tmp/out2" >&2; fail "sealed push refused"; }

# 3. A commit after the seal is refused (V1-1773).
echo two >> "$work/a.txt" && q -C "$work" commit -am after-seal
if git -C "$work" push origin batch > "$tmp/out3" 2>&1; then fail "post-seal commit push was accepted"; fi
grep -q 'REFUSE dogfood-seal-missing' "$tmp/out3" || { cat "$tmp/out3" >&2; fail "post-seal commit not refused"; }
[[ $(git -C "$tmp/origin.git" rev-parse batch) == $(cat "$work/sealed.txt") ]] || fail "post-seal commit reached the remote"
git -C "$work" rev-parse HEAD > "$work/sealed.txt"

# 4. Uncommitted tracked changes are refused before the check runs.
runs=$(wc -l < "$work/batch.log")
echo dirty >> "$work/a.txt"
if git -C "$work" push origin batch > "$tmp/out4" 2>&1; then fail "dirty push was accepted"; fi
grep -q 'REFUSE dogfood-seal-not-checked the working tree has uncommitted tracked changes' "$tmp/out4" || { cat "$tmp/out4" >&2; fail "no dirty-tree refusal"; }
[[ $(wc -l < "$work/batch.log") == "$runs" ]] || fail "the check ran on a dirty tree"
q -C "$work" checkout -- a.txt

# 5. A branch that is not the checked-out HEAD is refused, not checked against the wrong tree.
q -C "$work" branch side
q -C "$work" checkout side && echo side >> "$work/a.txt" && q -C "$work" commit -am side && q -C "$work" checkout batch
if git -C "$work" push origin side > "$tmp/out5" 2>&1; then fail "non-HEAD push was accepted"; fi
grep -q 'REFUSE dogfood-seal-not-checked refs/heads/side is not the checked-out HEAD' "$tmp/out5" || { cat "$tmp/out5" >&2; fail "no non-HEAD refusal"; }

# 6. The sealed head pushes; a tag, a deletion and a branch with no new commit skip the check.
git -C "$work" push origin batch > "$tmp/out6" 2>&1 || { cat "$tmp/out6" >&2; fail "resealed push refused"; }
runs=$(wc -l < "$work/batch.log")
q -C "$work" tag v1
git -C "$work" push origin v1 > "$tmp/out7" 2>&1 || { cat "$tmp/out7" >&2; fail "tag push refused"; }
git -C "$work" push origin "$base:refs/heads/at-base" > "$tmp/out8" 2>&1 || { cat "$tmp/out8" >&2; fail "push of the base refused"; }
git -C "$work" push origin --delete at-base > "$tmp/out9" 2>&1 || { cat "$tmp/out9" >&2; fail "deletion refused"; }
[[ $(wc -l < "$work/batch.log") == "$runs" ]] || fail "a tag, base or deletion push ran the check"

echo "pre-push-dogfood-seal_test: ok"
