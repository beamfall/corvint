#!/usr/bin/env bash
# Tests for script/lane-interrupted.sh (ASD-V0-788, NEW-D6): record derives the interrupted-lane
# marker from Git state alone (head, cited and remaining IDs, dirty paths) without touching the
# worktree, counts only the lane's own non-merge first-parent commits and whole-token IDs, list
# flags every marker (a malformed one too) and exits 1, clear removes it, and each refusal writes
# nothing.
set -uo pipefail
export LC_ALL=C
script=$(cd "$(dirname "$0")" && pwd)/lane-interrupted.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-lane-interrupted-test.XXXXXX") || exit 1
tmp=$(cd "$tmp" && pwd)
trap '/bin/rm -rf "$tmp"' EXIT
failed=0
fail() {
  failed=1
  printf 'lane-interrupted_test: FAIL line %s: %s\n' "${BASH_LINENO[0]}" "$*" >&2
  [[ ! -f $tmp/out ]] || sed 's/^/  | /' "$tmp/out" >&2
}
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid \
  GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
repo=$tmp/repo
git init -q -b main "$repo" || exit 1
cd "$repo" || exit 1
printf 'base\n' > a.txt
git add -A && git commit -qm base
base=$(git rev-parse HEAD)
# Another lane's work, merged into this lane as a batch tip: its IDs must not count.
git checkout -q -b batch && printf 'b\n' > b.txt && git add -A && git commit -qm 'fix: other lane (V1-0003)' && git checkout -q main
git checkout -q -b off && printf 'off\n' > off.txt && git add -A && git commit -qm off && git checkout -q main
wt=$tmp/lane-x
git worktree add -q -b claude/lane-x "$wt" "$base" || exit 1
cd "$wt" || exit 1
printf 'one\n' > c.txt && git add -A && git commit -qm 'feat: first (NEW-D1)'
first=$(git rev-parse HEAD)
git merge -q --no-ff -m 'Merge batch tip into lane x (V1-0004)' batch || exit 1
printf 'two\n' >> c.txt && git add -A && git commit -qm $'fix: second\n\nRefs V1-0002 and NEW-D1.'
second=$(git rev-parse HEAD)
printf 'edit\n' >> a.txt
printf 'new\n' > untracked.txt
head=$(git rev-parse HEAD)
git status --porcelain > "$tmp/status.before"
gitdir=$(git rev-parse --absolute-git-dir)
marker=$gitdir/corvint/lane-interrupted
printf 'LANE=lane-x\nBASE=%s\nDATE=2026-11-02\nTICKETS=NEW-D1 V1-0002 V1-0003 V1-0004 V1-000 NEW-D3\n' "$base" > "$tmp/lane.env"

# 1. Refusals write nothing.
refused() { # refused PATTERN ARGS...: exit 2, PATTERN on stderr, no marker
  local pattern=$1
  shift
  "$script" "$@" > "$tmp/out" 2>&1
  [[ $? == 2 ]] && grep -q -- "$pattern" "$tmp/out" && [[ ! -e $marker ]]
}
refused 'REFUSE dir-required' record --env "$tmp/lane.env" || fail 'record without --dir must refuse'
refused 'REFUSE not-a-worktree' record --dir "$tmp" --env "$tmp/lane.env" || fail 'a non-Git --dir must refuse'
refused 'REFUSE env-unreadable' record --dir "$wt" --env "$tmp/missing.env" || fail 'an unreadable env must refuse'
refused 'REFUSE no-tickets' record --dir "$wt" --lane lane-x --base "$base" || fail 'no IDs must refuse'
refused "REFUSE bad-id 'V1 0002'" record --dir "$wt" --lane lane-x --base "$base" 'V1 0002' || fail 'a malformed ID must refuse'
refused 'REFUSE bad-lane' record --dir "$wt" --lane Lane_X --base "$base" V1-0002 || fail 'a malformed lane must refuse'
refused 'REFUSE base-not-ancestor' record --dir "$wt" --lane lane-x --base off V1-0002 || fail 'a base off the lane history must refuse'
refused 'REFUSE base-unknown' record --dir "$wt" --lane lane-x --base no-such-ref V1-0002 || fail 'an unknown base must refuse'
refused 'REFUSE bad-reason' record --dir "$wt" --env "$tmp/lane.env" --reason $'two\nlines' || fail 'a multi-line reason must refuse'
"$script" list --repo "$repo" > "$tmp/out" 2>&1 && [[ ! -s $tmp/out ]] || fail 'list with no marker must exit 0 and print nothing'

# 2. record from lane.env: the lane's own commits only, whole-token IDs, dirty paths kept.
"$script" record --dir "$wt" --env "$tmp/lane.env" --reason usage-limit > "$tmp/out" 2> "$tmp/err" ||
  fail "record must exit 0, got $?"
[[ -f $marker ]] && cmp -s "$marker" "$tmp/out" || fail 'record must write the marker it prints'
expect() { grep -qx -- "$1" "$marker" || fail "marker line $1"; }
[[ $(sed -n 1p "$marker") == lane-interrupted/0 ]] || fail 'first line lane-interrupted/0'
expect 'lane=lane-x'
expect "dir=$wt"
expect 'branch=claude/lane-x'
expect "base=$base"
expect "head=$head"
expect 'commits=3'
expect 'reason=usage-limit'
grep -Eqx 'recorded=[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z' "$marker" || fail 'recorded UTC line'
expect "cited=NEW-D1@${second:0:12} V1-0002@${second:0:12}"
expect 'remaining=V1-0003 V1-0004 V1-000 NEW-D3'
expect 'dirty=2'
expect 'path  M a.txt'
expect 'path ?? untracked.txt'
grep -q "recorded $marker; resume the lane in $wt at ${head:0:12}" "$tmp/err" || fail 'the resume hint on stderr'
git status --porcelain > "$tmp/status.after"
cmp -s "$tmp/status.before" "$tmp/status.after" && [[ $(git rev-parse HEAD) == "$head" ]] ||
  fail 'record must leave HEAD and the worktree status as they were'

# 3. Positional IDs and --base override lane.env; a rerun replaces the marker.
"$script" record --dir "$wt" --env "$tmp/lane.env" --base "$first" V1-0002 NEW-D3 > "$tmp/out" 2>&1 || fail 'override record'
expect "base=$first"
expect 'commits=2'
expect "cited=V1-0002@${second:0:12}"
expect 'remaining=NEW-D3'
expect 'reason=unknown'
"$script" record --dir "$wt" --env "$tmp/lane.env" --reason usage-limit > "$tmp/out" 2>&1 || fail 'rerun record'
expect 'remaining=V1-0003 V1-0004 V1-000 NEW-D3'

# 4. list flags each marker and exits 1; a malformed marker still counts; clear removes it.
"$script" list --repo "$repo" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -qx "INTERRUPTED lane=lane-x head=${head:0:12} remaining=4 dirty=2 dir=$wt" "$tmp/out" ||
  fail 'list must print the marker and exit 1'
(cd "$wt" && "$script" list) > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q '^INTERRUPTED lane=lane-x ' "$tmp/out" || fail 'list from inside the worktree must find the same marker'
mkdir -p "$repo/.git/corvint" && printf 'garbage\n' > "$repo/.git/corvint/lane-interrupted"
"$script" list --repo "$repo" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -qx "INTERRUPTED lane=? reason=malformed $repo/.git/corvint/lane-interrupted" "$tmp/out" &&
  [[ $(grep -c '^INTERRUPTED ' "$tmp/out") == 2 ]] || fail 'a malformed marker must be flagged, not skipped'
/bin/rm -f "$repo/.git/corvint/lane-interrupted"
"$script" clear --dir "$wt" > "$tmp/out" 2>&1 && grep -q "cleared $marker" "$tmp/out" && [[ ! -e $marker ]] ||
  fail 'clear must remove the marker'
"$script" list --repo "$repo" > "$tmp/out" 2>&1 && [[ ! -s $tmp/out ]] || fail 'list after clear must exit 0 and print nothing'
"$script" clear --dir "$wt" > "$tmp/out" 2>&1 && grep -q 'no marker' "$tmp/out" || fail 'clear without a marker must exit 0'
"$script" bogus > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail 'an unknown subcommand must exit 2'

if [[ $failed != 0 ]]; then
  printf 'lane-interrupted_test: FAIL\n' >&2
  exit 1
fi
printf 'lane-interrupted_test: PASS\n'
