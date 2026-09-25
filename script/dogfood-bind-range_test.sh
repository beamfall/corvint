#!/usr/bin/env bash
# DOGFOOD-011 (DOGFOOD-BIND-001 to -007) and DOGFOOD-012 regressions for
# script/dogfood-bind-range.sh and the retroactive-bound note of `corvint dogfood check`,
# on a fake corvint.
set -euo pipefail

# This wrapper asserts on captured output with ripgrep (rg); without it every rg call below fails
# as a bare "command not found" partway through, which reads as a real regression instead of a
# missing prerequisite. Fail closed here, before any fixture is built.
if ! command -v rg >/dev/null 2>&1; then
    printf 'dogfood-bind-range_test: REFUSE unsupported-environment-missing-rg\n' >&2
    exit 1
fi

source_root=$(cd "$(dirname "$0")/.." && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-bind-range-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
repo="$test_root/repo"
mkdir -p "$repo/script" "$test_root/bin"
cp "$source_root/script/dogfood-bind-range.sh" "$repo/script/"
# The check notes come from the binary itself (DCW-V0-020); the fixture has no cmd/corvint to build.
(cd "$source_root" && GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go build -o "$test_root/driver" ./cmd/corvint)
printf '0.4.0a4\n' > "$repo/VERSION"
printf 'one\ntwo\n' > "$repo/doc.md"

# The fake models the two properties the coordinator relies on: prepare writes the fixed map in
# its --root, and status accepts only a target whose committed sidecar equals that map. The map
# has DOGFOOD_TEST_HUNKS unknown hunks (default 1) in the indent-2 layout the plan check reads.
cat > "$test_root/bin/corvint" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ $1 == --version ]]; then printf 'Corvint 0.4.0a4 (build 12)\n'; exit 0; fi
root=$2 action="$3 $4"
printf '%s\n' "$action" >> "$DOGFOOD_TEST_LOG"
case $action in
  'cem prepare')
    mkdir -p "$root/.corvint"
    {
      printf '{\n  "baseRevision": "%s",\n  "hunks": [\n' "$6"
      for ((hunk = 1; hunk <= ${DOGFOOD_TEST_HUNKS:-1}; hunk++)); do
        printf '    {\n      "disposition": "unknown",\n      "id": "hunk:test:%d"\n    },\n' "$hunk"
      done
      printf '  ],\n  "spec": "cem/0.2"\n}\n'
    } > "$root/.corvint/change.cem.json"
    ;;
  'cem cite')
    if [[ ${DOGFOOD_TEST_CITE:-} == fail ]]; then
      printf '{"code": "unknown-hunk-id", "error": "no such hunk", "ok": false}\n' >&2
      exit 2
    fi
    printf 'cited %s\n' "$8" >> "$root/.corvint/change.cem.json"
    ;;
  'cem mark')
    printf 'marked %s %s %s\n' "$8" "${10}" "${12}" >> "$root/.corvint/change.cem.json"
    ;;
  'cem status')
    printf 'max-unknown %s\n' "${12}" >> "$DOGFOOD_TEST_LOG"
    git -C "$root" show "${10}:.corvint/change.cem.json" | cmp -s - "$root/.corvint/change.cem.json" ||
      { printf '{"code": "excluded-artifact-mismatch", "ok": false}\n' >&2; exit 2; }
    [[ ${DOGFOOD_TEST_STATUS:-} != incomplete ]] || { printf '{"ok":false,"state":"incomplete"}\n'; exit 1; }
    printf '{"ok":true,"state":"ready-for-ci"}\n'
    ;;
esac
EOF
chmod +x "$test_root/bin/corvint"
: > "$test_root/corvint.log"
printf '1\tdoc.md\t1:1\tspecification\n' > "$test_root/citations.tsv"

git_commit() {
  git -C "$repo" -c user.name=t -c user.email=t@example.invalid add -A
  git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -qm "$1"
  git -C "$repo" rev-parse HEAD
}
work() { printf '%s\n' "$1" >> "$repo/work.txt"; git_commit "$1"; }
bind() {
  (cd "$repo" && env CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t \
    GIT_COMMITTER_EMAIL=t@example.invalid "$@")
}
git -C "$repo" init -q -b main
b0=$(work b0)
work c1 >/dev/null
mkdir -p "$repo/.corvint"
printf '{\n  "baseRevision": "%s",\n  "spec": "cem/0.2"\n}\n' "$b0" > "$repo/.corvint/change.cem.json"
s1=$(git_commit s1)
g1=$(work g1)
g2=$(work g2)
c3=$(work c3)
evidence=$(git -C "$repo" rev-parse --absolute-git-dir)/corvint

# DOGFOOD-011: a range that is empty, reversed or not landed on HEAD is refused before any work.
for case in "$s1 $s1 empty-range" "$g2 $g1 base-not-ancestor-of-target"; do
  read -r from to reason <<< "$case"
  status=0
  output=$(bind script/dogfood-bind-range.sh "$from" "$to" 2>&1) || status=$?
  test "$status" = 2
  test "$output" = "dogfood-bind-range: REFUSE $reason"
done
unlanded=$(git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit-tree "$g2^{tree}" -p "$g2" -m side </dev/null)
status=0
output=$(bind script/dogfood-bind-range.sh "$s1" "$unlanded" 2>&1) || status=$?
test "$status" = 2
test "$output" = 'dogfood-bind-range: REFUSE target-not-landed'

# DOGFOOD-012: before a retroactive binding the gap is reported as unbound.
before=$(cd "$repo" && "$test_root/driver" dogfood check "$g2" 2>&1) || :
printf '%s\n' "$before" | rg -q "^dogfood-check: NOTE unbound-commits count=2 window=$b0\\.\\.$g2\$"
if printf '%s\n' "$before" | rg -q retroactive; then exit 1; fi

# DOGFOOD-011: a citation or policy failure publishes no binding and leaves no private worktree.
status=0
output=$(bind DOGFOOD_TEST_CITE=fail DOGFOOD_CITATIONS="$test_root/citations.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-cite unknown-hunk-id'
status=0
output=$(bind DOGFOOD_TEST_STATUS=incomplete script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
printf '%s\n' "$output" | rg -q '^dogfood-bind-range: FAIL cem-policy$'
if printf '%s\n' "$output" | rg -q 'PASS|next:'; then exit 1; fi
test "$(git -C "$repo" worktree list | wc -l)" -eq 1
if compgen -G "$evidence/dogfood-bind-range.*" >/dev/null; then exit 1; fi

# V1-0228 / DCW-V0-019: a citation plan written for another map is refused before any cite, as in
# the bind loop: an unknown hunk named by no plan, or an ordinal above the hunk count.
: > "$test_root/corvint.log"
status=0
output=$(bind DOGFOOD_TEST_HUNKS=2 DOGFOOD_CITATIONS="$test_root/citations.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-cite citation-plan-map-mismatch'
{ cat "$test_root/citations.tsv"; printf '2\tdoc.md\t2:2\tspecification\n'; } > "$test_root/citations-stale.tsv"
status=0
output=$(bind DOGFOOD_CITATIONS="$test_root/citations-stale.tsv" script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-cite citation-plan-map-mismatch'
if rg -q '^cem cite$' "$test_root/corvint.log"; then exit 1; fi

# DOGFOOD-011: a passing binding commit is the target's tree plus only the cited sidecar, carries
# the retroactive trailer, abstains from pre-change context, and never writes the main worktree.
sidecar_before=$(git -C "$repo" hash-object .corvint/change.cem.json)
output=$(bind DOGFOOD_CITATIONS="$test_root/citations.tsv" script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
binding=$(printf '%s\n' "$output" | sed -n "s/^dogfood-bind-range: PASS retroactive binding=\\([0-9a-f]\\{40\\}\\) range=$s1\\.\\.$g2\$/\\1/p")
test -n "$binding"
for step in prechange-query prechange-impact local-outcome ocm-aggregate; do
  printf '%s\n' "$output" | rg -q "^dogfood-bind-range: NOTE $step NOT_PRODUCED retroactive-binding\$"
done
test "$(git -C "$repo" rev-parse "$binding^@")" = "$g2"
test "$(git -C "$repo" diff --name-only "$g2" "$binding")" = .corvint/change.cem.json
git -C "$repo" show "$binding:.corvint/change.cem.json" | rg -q "^  \"baseRevision\": \"$s1\",\$"
git -C "$repo" show "$binding:.corvint/change.cem.json" | rg -q '^cited 1$'
prepared="$evidence/bind-range.${s1:0:12}..${g2:0:12}.cem.json"
rg -q "^  \"baseRevision\": \"$s1\",\$" "$prepared"
if rg -q cited "$prepared"; then exit 1; fi
test "$(git -C "$repo" log -1 --format='%(trailers:key=Corvint-Dogfood-Binding,valueonly)' "$binding")" = retroactive
test "$(git -C "$repo" hash-object .corvint/change.cem.json)" = "$sidecar_before"
test -z "$(git -C "$repo" status --porcelain --untracked-files=all)"
test "$(git -C "$repo" worktree list | wc -l)" -eq 1

# DOGFOOD-BIND-007: an explicit unknown plan marks each hunk, raises the unknown ceiling to exactly
# its row count, and keeps every NOT_PRODUCED reason in the output and the binding message.
printf '2\tfree-text\tdetail\n' > "$test_root/unknown-bad.tsv"
status=0
output=$(bind DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_UNKNOWN="$test_root/unknown-bad.tsv" script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-mark invalid-unknown-plan'
printf '2\tno-evidence\tintent-added-inside-the-range\n' > "$test_root/unknown.tsv"
: > "$test_root/corvint.log"
output=$(bind DOGFOOD_TEST_HUNKS=2 DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_UNKNOWN="$test_root/unknown.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
printf '%s\n' "$output" | rg -q '^dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=2 reason=no-evidence detail=intent-added-inside-the-range$'
marked=$(printf '%s\n' "$output" | sed -n 's/^dogfood-bind-range: PASS retroactive binding=\([0-9a-f]\{40\}\) .*/\1/p')
git -C "$repo" show "$marked:.corvint/change.cem.json" | rg -q '^marked 2 unknown no-evidence$'
git -C "$repo" log -1 --format=%B "$marked" | rg -q '^NOT_PRODUCED hunk=2 reason=no-evidence detail=intent-added-inside-the-range$'
test "$(git -C "$repo" log -1 --format='%(trailers:key=Corvint-Dogfood-Binding,valueonly)' "$marked")" = retroactive
rg -q '^max-unknown 1$' "$test_root/corvint.log"

# DOGFOOD-BIND-007 (decision 0165): a removed-intent pin must name a blob span in BASE's tree.
target_only=$(git -C "$repo" rev-parse "$g2:work.txt")
printf '2\tno-evidence\tremoved-intent.%s.0-3\n' "$target_only" > "$test_root/unknown-pin-bad.tsv"
status=0
output=$(bind DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_UNKNOWN="$test_root/unknown-pin-bad.tsv" script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-mark invalid-unknown-plan'
pin="removed-intent.$(git -C "$repo" rev-parse "$s1:work.txt").0-3"
printf '2\tno-evidence\t%s\n' "$pin" > "$test_root/unknown-pin.tsv"
output=$(bind DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_UNKNOWN="$test_root/unknown-pin.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
printf '%s\n' "$output" | rg -q "^dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=2 reason=no-evidence detail=$pin\$"

# DOGFOOD-012: once merged, the check names the range as retroactively bound, apart from the
# still-unbound commit after it.
git -C "$repo" -c user.name=t -c user.email=t@example.invalid merge -q --no-ff -s ours -m bind "$binding"
merge=$(git -C "$repo" rev-parse HEAD)
work c4 >/dev/null
after=$(cd "$repo" && "$test_root/driver" dogfood check "$merge" 2>&1) || :
printf '%s\n' "$after" | rg -q "^dogfood-check: NOTE unbound-commits count=1 window=$b0\\.\\.$merge\$"
test "$(printf '%s\n' "$after" | rg '^  unbound ')" = "  unbound $c3"
printf '%s\n' "$after" | rg -q "^dogfood-check: NOTE retroactive-bound-commits count=3 window=$b0\\.\\.$merge\$"
test "$(printf '%s\n' "$after" | rg '^  retroactive ' | sort)" = \
  "$(printf '  retroactive %s binding=%s\n' "$g1" "$binding" "$g2" "$binding" "$binding" "$binding" | sort)"
