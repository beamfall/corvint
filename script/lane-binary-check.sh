#!/usr/bin/env bash
# usage: script/lane-binary-check.sh [--rebuild] [--corvint PATH] [--against REF] DIR
#
# ASD-V0-755 (V1-3761, V1-4044): judges the installed `corvint` (PATH, or --corvint PATH) against
# the commit checked out in DIR, a Corvint checkout or lane worktree, with the rule the binary uses
# for itself (AFP-V0-846): the same commit, or no non-test Go source, go.mod or go.sum change
# between the binary's stamped commit and DIR's HEAD, is FRESH. It prints one line naming the
# installed build and commit and the expected build (the first-parent commit count of HEAD, the
# number a release or `-X main.build` stamp carries) and commit:
#   lane-binary-check: FRESH corvint build N (commit C) for build M (commit H)            exit 0
#   lane-binary-check: STALE corvint build N (commit C) for build M (commit H): K Go
#     source paths differ (a, b, c ...); rebuild: script/lane-binary-check.sh --rebuild DIR   exit 1
#   lane-binary-check: UNVERIFIED ... (no stamped commit, the commit is absent in DIR, or the  exit 1
#     diff from it fails)
# A DIR whose go.mod is not the Corvint module is not judged (exit 0, no output), so a fixture or
# foreign repository never fails lane setup. --rebuild (ASD-V0-756) builds ./cmd/corvint from DIR
# with the expected build stamp into a temporary directory, refuses a dirty DIR (the build would
# record vcs.modified=true), installs it over the PATH binary with script/install-local-bin.sh (one
# `corvint.corvint-prev-<n>` rollback; `install-local-bin.sh --rollback PATH` restores it) and
# judges again. Bad arguments or a missing corvint exit 2. GO names the go command (default go).
# --against REF selects the expected revision instead of HEAD; origin/main is the lane-start
# host check. --rebuild with that option requires DIR's clean HEAD to be REF.
# Example: script/lane-binary-check.sh --against origin/main .
set -uo pipefail
export LC_ALL=C
# Keep the recovery beside each reason for the flow-guide check, and print it on its own line.
refuse() { printf 'lane-binary-check: REFUSE %s\nfix: %s\n' "${1%%; fix:*}" "${1#*; fix: }" >&2; exit 2; }
rebuild=0 corvint='' dir='' against=HEAD
while (($#)); do
  case $1 in
    --rebuild) rebuild=1; shift ;;
    --against) [[ $# -ge 2 && -n $2 ]] || refuse "--against needs a value; fix: pass --against origin/main"; against=$2; shift 2 ;;
    --corvint) [[ $# -ge 2 && -n $2 ]] || refuse "--corvint needs a value; fix: pass --corvint PATH with the path to an executable corvint"; corvint=$2; shift 2 ;;
    -h | --help) sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) refuse "unknown-option $1; fix: use script/lane-binary-check.sh [--rebuild] [--corvint PATH] DIR" ;;
    *) [[ -z $dir ]] || refuse "usage: script/lane-binary-check.sh [--rebuild] [--corvint PATH] DIR; fix: pass exactly one checkout DIR and only the documented options"; dir=$1; shift ;;
  esac
done
[[ -n $dir ]] || refuse "usage: script/lane-binary-check.sh [--rebuild] [--corvint PATH] DIR; fix: pass exactly one checkout DIR and only the documented options"
here=$(cd "$(dirname "$0")" && pwd) || exit 2
dir=$(cd "$dir" 2> /dev/null && pwd) || refuse "not-a-directory $dir; fix: pass an existing checkout directory as DIR"
git -C "$dir" rev-parse --git-dir > /dev/null 2>&1 || refuse "not-a-repository $dir; fix: pass a Git checkout as DIR"
module=$(sed -n 's/^module[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' "$dir/go.mod" 2> /dev/null | head -1)
[[ $module == github.com/Beamfall/corvint ]] || exit 0
if [[ -z $corvint ]]; then
  corvint=$(command -v corvint) || refuse "corvint-not-found on PATH; fix: install corvint on PATH or pass --corvint PATH to an installed executable"
fi
[[ -x $corvint ]] || refuse "corvint-not-executable $corvint; fix: pass --corvint PATH to an executable corvint, or correct its execute permission"
head=$(git -C "$dir" rev-parse --verify -q "$against^{commit}") || refuse "no-head $dir ref=$against; fix: fetch origin and name an available --against ref"
expected="build $(git -C "$dir" rev-list --first-parent --count "$head") (commit ${head:0:12})"

# judge prints the verdict line and returns 0 for FRESH, 1 otherwise.
judge() {
  local line installed commit paths count shown
  line=$("$corvint" --version 2> /dev/null | head -1)
  # The commit member is read on its own: a plain `go build` states `(commit SHA)` without a build
  # number (CRB-V0-003), and a `, modified` build states no clean commit, so it is UNVERIFIED.
  commit=$(sed -n 's/.*[(,] *commit \([0-9a-f]\{40\}\)).*/\1/p' <<< "$line")
  installed=$(sed -n 's/.*(\(build [0-9]*\), commit [0-9a-f]\{40\}).*/\1/p' <<< "$line")
  installed=${installed:-unstamped build}
  if [[ -z $commit ]]; then
    printf 'lane-binary-check: UNVERIFIED corvint %s states no build commit (%s) for %s; rebuild: script/lane-binary-check.sh --rebuild %s\n' "$corvint" "${line:-no --version output}" "$expected" "$dir"
    return 1
  fi
  installed="$installed (commit ${commit:0:12})"
  if [[ $commit == "$head" ]]; then
    printf 'lane-binary-check: FRESH corvint %s for %s\n' "$installed" "$expected"
    return 0
  fi
  if ! git -C "$dir" cat-file -e "$commit^{commit}" 2> /dev/null; then
    printf 'lane-binary-check: UNVERIFIED corvint %s for %s: commit %s is absent in %s (fetch, or rebuild: script/lane-binary-check.sh --rebuild %s)\n' "$installed" "$expected" "${commit:0:12}" "$dir" "$dir"
    return 1
  fi
  # A failed diff (for example an offline partial clone missing the commit's tree) is no evidence.
  if ! paths=$(git -C "$dir" diff --name-only --no-renames "$commit" "$head" -- ':(glob)**/*.go' ':(exclude,glob)**/*_test.go' ':(exclude,glob)**/testdata/**' go.mod go.sum 2> /dev/null); then
    printf 'lane-binary-check: UNVERIFIED corvint %s for %s: git diff from commit %s failed in %s (fetch, or rebuild: script/lane-binary-check.sh --rebuild %s)\n' "$installed" "$expected" "${commit:0:12}" "$dir" "$dir"
    return 1
  fi
  if [[ -z $paths ]]; then
    printf 'lane-binary-check: FRESH corvint %s for %s (no Go source differs)\n' "$installed" "$expected"
    return 0
  fi
  count=$(wc -l <<< "$paths" | tr -d ' ')
  shown=$(head -3 <<< "$paths" | paste -sd, - | sed 's/,/, /g')
  ((count > 3)) && shown="$shown (+$((count - 3)) more)"
  printf 'lane-binary-check: STALE corvint %s for %s: %s Go source paths differ (%s); rebuild: script/lane-binary-check.sh --rebuild %s\n' "$installed" "$expected" "$count" "$shown" "$dir"
  return 1
}

judge && exit 0
((rebuild)) || exit 1
[[ $(git -C "$dir" rev-parse HEAD) == "$head" ]] || refuse "rebuild-ref-not-head $against; fix: run --rebuild in a clean checkout at $against, then retry lane setup"
[[ -z $(git -C "$dir" status --porcelain --untracked-files=normal) ]] || refuse "dirty-checkout $dir (the build would record vcs.modified=true); fix: commit or move aside the checkout changes, then rerun with --rebuild"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/lane-binary-check.XXXXXX") || refuse "temp-dir-failed; fix: set TMPDIR to a writable directory with free space, then rerun with --rebuild"
trap '/bin/rm -rf "$tmp"' EXIT INT TERM
build=$(git -C "$dir" rev-list --first-parent --count HEAD)
(cd "$dir" && GOTOOLCHAIN=local "${GO:-go}" build -o "$tmp/corvint" -ldflags "-X main.build=$build" ./cmd/corvint) || refuse "build-failed in $dir; fix: resolve the Go build error above (GO selects the toolchain), then rerun with --rebuild"
"$here/install-local-bin.sh" "$tmp/corvint" "$corvint" || exit 2
judge
