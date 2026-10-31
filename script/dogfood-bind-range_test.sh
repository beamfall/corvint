#!/usr/bin/env bash
# DOGFOOD-011 (DOGFOOD-BIND-001 to -008) and DOGFOOD-012 regressions for
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
source "$source_root/script/dogfood-fixture-scripts.sh"
dogfood_fixture_copy "$source_root" "$repo/script" dogfood-bind-range.sh
# The check notes come from the binary itself (DCW-V0-020); the fixture has no cmd/corvint to build.
(cd "$source_root" && GOCACHE=${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache GOTOOLCHAIN=local go build -o "$test_root/driver" ./cmd/corvint)
# DOGFOOD-BIND-008: the guarding-tests count comes from the real Go rule, built once here.
(cd "$source_root" && GOCACHE=${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache GOTOOLCHAIN=local go build -o "$test_root/guard-hunks" ./tools/dogfood-guard-hunks)
printf '0.4.0a4\n' > "$repo/VERSION"
printf 'one\ntwo\n' > "$repo/doc.md"

# The fake models the two properties the coordinator relies on: prepare writes the fixed map in
# its --root, and status accepts only a target whose committed sidecar equals that map. The map
# has DOGFOOD_TEST_HUNKS unknown hunks (default 1) in the compact cem/1.0 producer layout
# (CEM-V1-140); DOGFOOD_TEST_LEGACY=1 writes a retired cem/0.2 layout the plan check must refuse.
# DOGFOOD_TEST_GO_PATH names every hunk's path instead of the default escaped non-Go path.
cat > "$test_root/bin/corvint" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ $1 == --version ]]; then printf 'Corvint 0.4.0a4 (build 12)\n'; exit 0; fi
root=$2 action="$3 $4"
printf '%s\n' "$action" >> "$DOGFOOD_TEST_LOG"
case $action in
  'cem prepare')
    mkdir -p "$root/.corvint"
    if [[ ${DOGFOOD_TEST_LEGACY:-} == 1 ]]; then
      printf '{\n  "baseRevision": "%s",\n  "hunks": [\n    {\n      "disposition": "unknown",\n      "id": "hunk:test:1"\n    }\n  ],\n  "spec": "cem/0.2"\n}\n' \
        "$6" > "$root/.corvint/change.cem.json"
      exit 0
    fi
    {
      printf '{"baseRevision":"%s","hunks":[' "$6"
      for ((hunk = 1; hunk <= ${DOGFOOD_TEST_HUNKS:-1}; hunk++)); do
        [[ $hunk == 1 ]] || printf ','
        printf '{"basis":[],"disposition":"unknown","id":"hunk:test:%d","path":"%s","reason":"no-evidence"}' "$hunk" \
          "${DOGFOOD_TEST_GO_PATH:-a\\\"disposition\\\":\\\"x}"
      done
      printf '],"spec":"cem/1.0"}\n'
    } > "$root/.corvint/change.cem.json"
    ;;
  'cem cite')
    if [[ ${DOGFOOD_TEST_CITE:-} == fail ]]; then
      printf '{"code": "unknown-hunk-id", "error": "no such hunk", "ok": false}\n' >&2
      exit 2
    fi
    printf "cite %s %s %s\n" "$8" "${11}" "${12}" >> "$DOGFOOD_TEST_LOG"
    # A cite or mark adds a member, so the sidecar stays one JSON object, as a real map does.
    sed 's/}$/,"cited'"$8"'":true}/' "$root/.corvint/change.cem.json" > "$root/.corvint/edit.json"
    mv "$root/.corvint/edit.json" "$root/.corvint/change.cem.json"
    ;;
  'cem mark')
    sed 's/}$/,"marked'"$8 ${10} ${12}"'":true}/' "$root/.corvint/change.cem.json" > "$root/.corvint/edit.json"
    mv "$root/.corvint/edit.json" "$root/.corvint/change.cem.json"
    ;;
  'cem status')
    printf 'max-unknown %s\n' "${12}" >> "$DOGFOOD_TEST_LOG"
    [[ ${15:-} != --artifacts ]] || printf 'artifacts %s\n' "${16}" >> "$DOGFOOD_TEST_LOG"
    git -C "$root" show "${10}:.corvint/change.cem.json" | cmp -s - "$root/.corvint/change.cem.json" ||
      { printf '{"code": "excluded-artifact-mismatch", "ok": false}\n' >&2; exit 2; }
    if [[ ${DOGFOOD_TEST_STATUS:-} == policy ]]; then
      printf '{"counts":{"mechanical":0,"supported":0,"total":2,"unknown":2},"map":{"filler":"%s"},"ok":false,"policyIssues":[{"code":"unknown-ceiling","detail":"unknown 2 > 0"}],"policyState":"over-ceiling","state":"incomplete","tool":"corvint"}\n' "$(printf 'x%.0s' {1..4096})"
      exit 1
    fi
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
    CORVINT_GUARD_HUNKS_BIN="$test_root/guard-hunks" \
    GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t \
    GIT_COMMITTER_EMAIL=t@example.invalid "$@")
}
git -C "$repo" init -q -b main
b0=$(work b0)
work c1 >/dev/null
mkdir -p "$repo/.corvint"
printf '{"baseRevision":"%s","hunks":[],"spec":"cem/1.0"}\n' "$b0" > "$repo/.corvint/change.cem.json"
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
rg -q "^dogfood-check: NOTE unbound-commits count=2 window=$b0\\.\\.$g2 newest=[0-9a-f]{40} oldest=[0-9a-f]{40} before-base list=.*/dogfood-check\\.unbound-commits\$" <<< "$before"
if rg -q '^  unbound '; then exit 1; fi <<< "$before"
if rg -q retroactive; then exit 1; fi <<< "$before"

# DOGFOOD-011: a citation or policy failure publishes no binding and leaves no private worktree.
status=0
output=$(bind DOGFOOD_TEST_CITE=fail DOGFOOD_CITATIONS="$test_root/citations.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-cite unknown-hunk-id'
status=0
output=$(bind DOGFOOD_TEST_STATUS=incomplete script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
rg -q '^dogfood-bind-range: FAIL cem-policy$' <<< "$output"
if rg -q 'PASS|next:'; then exit 1; fi <<< "$output"
test "$(git -C "$repo" worktree list | wc -l)" -eq 1
if compgen -G "$evidence/dogfood-bind-range.*" >/dev/null; then exit 1; fi

# V1-0228 / DCW-V0-019 / CEM-V1-140: a citation plan written for another map is refused before any
# cite, as in the bind loop: an unknown hunk named by no plan, or an ordinal above the hunk count.
# The compact cem/1.0 hunks are counted exactly (an escaped "disposition" inside a string is not a
# hunk), and a map that is not cem/1.0 is refused as a mismatch (CEM-V1-215).
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
# The legacy plan names its hunk by ID, so only the cem/1.0 requirement refuses it.
printf 'hunk:test:1\tdoc.md\t1:1\tspecification\n' > "$test_root/citations-id.tsv"
status=0
output=$(bind DOGFOOD_TEST_LEGACY=1 DOGFOOD_CITATIONS="$test_root/citations-id.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-cite citation-plan-map-mismatch'
if rg -q '^cem cite$' "$test_root/corvint.log"; then exit 1; fi

# DOGFOOD-011: a passing binding commit is the target's tree plus only the cited sidecar, carries
# the retroactive trailer, abstains from pre-change context, never writes the main worktree, and
# passes the absolute private worktree root as the stable verifier's artifact root.
sidecar_before=$(git -C "$repo" hash-object .corvint/change.cem.json)
output=$(bind DOGFOOD_CITATIONS="$test_root/citations.tsv" script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
binding=$(printf '%s\n' "$output" | sed -n "s/^dogfood-bind-range: PASS retroactive binding=\\([0-9a-f]\\{40\\}\\) range=$s1\\.\\.$g2\$/\\1/p")
test -n "$binding"
rg -q '^artifacts /.*/dogfood-bind-range\.[^/]*/worktree$' "$test_root/corvint.log"
for step in prechange-query prechange-impact local-outcome ocm-aggregate; do
  rg -q "^dogfood-bind-range: NOTE $step NOT_PRODUCED retroactive-binding\$" <<< "$output"
done
# DOGFOOD-BIND-008: a map with no hunk the guarding-test search would select prints no guarding NOTE.
if rg -q 'guarding-tests'; then exit 1; fi <<< "$output"
test "$(git -C "$repo" rev-parse "$binding^@")" = "$g2"
test "$(git -C "$repo" diff --name-only "$g2" "$binding")" = .corvint/change.cem.json
bound_cem=$(git -C "$repo" show "$binding:.corvint/change.cem.json")
rg -q "^\{\"baseRevision\":\"$s1\"," <<< "$bound_cem"
rg -qF '"cited1":true' <<< "$bound_cem"
prepared="$evidence/bind-range.${s1:0:12}..${g2:0:12}.cem.json"
rg -q "^\{\"baseRevision\":\"$s1\"," "$prepared"
if rg -q cited "$prepared"; then exit 1; fi
test "$(git -C "$repo" log -1 --format='%(trailers:key=Corvint-Dogfood-Binding,valueonly)' "$binding")" = retroactive
test "$(git -C "$repo" hash-object .corvint/change.cem.json)" = "$sidecar_before"
test -z "$(git -C "$repo" status --porcelain --untracked-files=all)"
test "$(git -C "$repo" worktree list | wc -l)" -eq 1

# DOGFOOD-BIND-008 / DCW-V0-069: a bound map holding non-test Go hunks prints the single-shot path's
# guarding-tests NOTE with the hunk count and the binding to verify; a count that cannot be produced
# stays a visible NOTE and never fails the binding.
printf '1\tdoc.md\t1:1\tspecification\n2\tdoc.md\t2:2\tspecification\n' > "$test_root/citations-two.tsv"
output=$(bind DOGFOOD_TEST_HUNKS=2 DOGFOOD_TEST_GO_PATH=pkg/x.go DOGFOOD_CITATIONS="$test_root/citations-two.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
guarded=$(printf '%s\n' "$output" | sed -n 's/^dogfood-bind-range: PASS retroactive binding=\([0-9a-f]\{40\}\) .*/\1/p')
test -n "$guarded"
test "$(rg -c '^dogfood-bind-range: NOTE guarding-tests ' <<< "$output")" = 1
rg -qF "dogfood-bind-range: NOTE guarding-tests NOT_PRODUCED guarding-tests-not-run hunks=2: to cite an observed guarding test, check out the binding with git worktree add --detach DIR $guarded, run corvint --root DIR cem verify --map .corvint/change.cem.json --expected-base $s1 --target $guarded --artifacts DIR --guarding-tests with DIR absolute," <<< "$output"
output=$(bind CORVINT_GUARD_HUNKS_BIN=false DOGFOOD_TEST_HUNKS=2 DOGFOOD_TEST_GO_PATH=pkg/x.go \
  DOGFOOD_CITATIONS="$test_root/citations-two.tsv" script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
rg -q '^dogfood-bind-range: NOTE guarding-tests NOT_PRODUCED guarding-tests-count-unavailable$' <<< "$output"
rg -q '^dogfood-bind-range: PASS retroactive binding=' <<< "$output"

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
rg -q '^dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=2 reason=no-evidence detail=intent-added-inside-the-range$' <<< "$output"
marked=$(printf '%s\n' "$output" | sed -n 's/^dogfood-bind-range: PASS retroactive binding=\([0-9a-f]\{40\}\) .*/\1/p')
marked_cem=$(git -C "$repo" show "$marked:.corvint/change.cem.json")
rg -qF '"marked2 unknown no-evidence":true' <<< "$marked_cem"
marked_message=$(git -C "$repo" log -1 --format=%B "$marked")
rg -q '^NOT_PRODUCED hunk=2 reason=no-evidence detail=intent-added-inside-the-range$' <<< "$marked_message"
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
rg -q "^dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=2 reason=no-evidence detail=$pin\$" <<< "$output"

# DCW-V0-825 (V1-3366): a policy failure prints only the counts, the policy issues, the state and
# the published status file, never the whole status document.
status=0
output=$(bind DOGFOOD_TEST_STATUS=policy script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = "dogfood-bind-range: NOTE cem-cite NOT_PRODUCED citation-plan-not-provided
dogfood-bind-range: cem-policy counts={\"mechanical\":0,\"supported\":0,\"total\":2,\"unknown\":2} policyIssues=[{\"code\":\"unknown-ceiling\",\"detail\":\"unknown 2 > 0\"}] state=incomplete
dogfood-bind-range: cem-policy status=$evidence/bind-range.${s1:0:12}..${g2:0:12}.cem-status.json
dogfood-bind-range: FAIL cem-policy"
test "$(wc -c < "$evidence/bind-range.${s1:0:12}..${g2:0:12}.cem-status.json")" -gt 4096

# DCW-V0-827 (V1-3368): a citation row's span is `--lines S:E` or, as `bytes:S:E`, `--bytes S:E`.
printf '1\tdoc.md\t1:1\tspecification\n2\tdoc.md\tbytes:0:3\tspecification\n' > "$test_root/citations-forms.tsv"
: > "$test_root/corvint.log"
output=$(bind DOGFOOD_TEST_HUNKS=2 DOGFOOD_CITATIONS="$test_root/citations-forms.tsv" \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1)
rg -q '^dogfood-bind-range: PASS retroactive binding=' <<< "$output"
test "$(rg '^cite ' "$test_root/corvint.log")" = 'cite 1 --lines 1:1
cite 2 --bytes 0:3'
printf '1\tdoc.md\tbytes:0\tspecification\n' > "$test_root/citations-bad-bytes.tsv"
: > "$test_root/corvint.log"
bind DOGFOOD_CITATIONS="$test_root/citations-bad-bytes.tsv" script/dogfood-bind-range.sh "$s1" "$g2" >/dev/null 2>&1
test "$(rg '^cite ' "$test_root/corvint.log")" = 'cite 1 --lines bytes:0'

# DCW-V0-824 (V1-3365): DOGFOOD_CITATIONS=auto-inherit cites the helper's plan rows and keeps every
# uninherited hunk a visible unknown with a bounded NOTE naming the private list.
cat > "$test_root/inherit-stub" <<'EOF'
#!/usr/bin/env bash
printf '1\tdoc.md\tbytes:0:3\tspecification\n' > "$5"
printf '2\thunk:test:2\tdoc.md\tno-constituent-hunk\n' > "$6"
printf 'hunks=2 inherited=1 rows=1 uninherited=1 constituents=1\n'
EOF
chmod +x "$test_root/inherit-stub"
: > "$test_root/corvint.log"
status=0
output=$(bind CORVINT_INHERIT_CITATIONS_BIN="$test_root/inherit-stub" DOGFOOD_TEST_HUNKS=2 DOGFOOD_CITATIONS=auto-inherit \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 0
inherited_plan="$evidence/bind-range.${s1:0:12}..${g2:0:12}.inherited.tsv"
rg -qF "dogfood-bind-range: NOTE cem-cite inherited hunks=2 inherited=1 rows=1 uninherited=1 constituents=1 plan=$inherited_plan" <<< "$output"
rg -qF "dogfood-bind-range: NOTE cem-cite NOT_PRODUCED uninherited count=1 list=$evidence/bind-range.${s1:0:12}..${g2:0:12}.uninherited.tsv" <<< "$output"
test "$(rg '^cite ' "$test_root/corvint.log")" = 'cite 1 --bytes 0:3'
status=0
output=$(bind CORVINT_INHERIT_CITATIONS_BIN=false DOGFOOD_CITATIONS=auto-inherit \
  script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
test "$status" = 1
test "$output" = 'dogfood-bind-range: FAIL cem-cite inherit-failed exit-1'

# DCW-V0-830 (V1-3371): --prepare-only and DOGFOOD_CITATIONS=none write the prepared map, print its
# path and hunk count, exit 0, and run no cite, binding commit or status.
for mode in flag env; do
  : > "$test_root/corvint.log"
  heads_before=$(git -C "$repo" for-each-ref --format='%(objectname) %(refname)' | sort)
  status=0
  if [[ $mode == flag ]]; then
    output=$(bind DOGFOOD_TEST_HUNKS=3 script/dogfood-bind-range.sh --prepare-only "$s1" "$g2" 2>&1) || status=$?
  else
    output=$(bind DOGFOOD_TEST_HUNKS=3 DOGFOOD_CITATIONS=none script/dogfood-bind-range.sh "$s1" "$g2" 2>&1) || status=$?
  fi
  test "$status" = 0
  prepared="$evidence/bind-range.${s1:0:12}..${g2:0:12}.cem.json"
  test "$output" = "dogfood-bind-range: PASS prepare-only map=$prepared hunks=3 range=$s1..$g2"
  test -s "$prepared"
  test "$(cat "$test_root/corvint.log")" = 'cem prepare'
  test "$(git -C "$repo" for-each-ref --format='%(objectname) %(refname)' | sort)" = "$heads_before"
done

# DCW-V0-826 (V1-3367): no child reads the caller's stdin, so a while-read loop binds every piece
# even when a git on PATH drains its stdin.
mkdir -p "$test_root/shim"
printf '#!/usr/bin/env bash\ncat >/dev/null\nexec %q "$@"\n' "$(command -v git)" > "$test_root/shim/git"
chmod +x "$test_root/shim/git"
bound=0
while read -r from to; do
  output=$(bind PATH="$test_root/shim:$PATH" script/dogfood-bind-range.sh "$from" "$to" 2>&1)
  rg -q "^dogfood-bind-range: PASS retroactive binding=[0-9a-f]{40} range=$from\\.\\.$to\$" <<< "$output"
  bound=$((bound + 1))
done <<< "$s1 $g1
$g1 $g2
$g2 $c3"
test "$bound" = 3

# DOGFOOD-012: once merged, the check names the range as retroactively bound, apart from the
# still-unbound commit after it.
git -C "$repo" -c user.name=t -c user.email=t@example.invalid merge -q --no-ff -s ours -m bind "$binding"
merge=$(git -C "$repo" rev-parse HEAD)
work c4 >/dev/null
after=$(cd "$repo" && "$test_root/driver" dogfood check "$merge" 2>&1) || :
# DCW-V0-343: one NOTE line per window; the private list file names every commit.
rg -q "^dogfood-check: NOTE unbound-commits count=1 window=$b0\\.\\.$merge newest=$c3 oldest=$c3 before-base list=.*/dogfood-check\\.unbound-commits\$" <<< "$after"
test "$(cat "$(printf '%s\n' "$after" | sed -n 's/^dogfood-check: NOTE unbound-commits .* list=//p')")" = "unbound $c3"
rg -q "^dogfood-check: NOTE retroactive-bound-commits count=3 window=$b0\\.\\.$merge newest=[0-9a-f]{40} oldest=[0-9a-f]{40} before-base list=.*/dogfood-check\\.retroactive-bound-commits\$" <<< "$after"
if rg -q '^  (unbound|retroactive) '; then exit 1; fi <<< "$after"
test "$(sort "$(printf '%s\n' "$after" | sed -n 's/^dogfood-check: NOTE retroactive-bound-commits .* list=//p')")" = \
  "$(printf 'retroactive %s binding=%s\n' "$g1" "$binding" "$g2" "$binding" "$binding" "$binding" | sort)"
