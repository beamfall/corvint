#!/usr/bin/env bash
# usage: script/install-cem-verifiers.sh [--bin-dir DIR] [--keep N]
#        script/install-cem-verifiers.sh --check [--bin-dir DIR]
#
# Rebuilds and installs the native verifier `corvint` and the portable consumer `cem01-go` together
# from this checkout's HEAD (TCB-V0-046, V1-1761), so the cem-verdict gate's two legs share their
# source. It refuses a checkout with any uncommitted or untracked file (Go then records
# vcs.modified=true, which the gate refuses), builds both into a temporary
# directory for this host's GOHOSTOS/GOHOSTARCH, requires each build to record that target,
# vcs.revision equal to HEAD and vcs.modified=false, then
# installs each over DIR/<tool> (default ~/.local/bin) with script/install-local-bin.sh (one
# rollback backup each; more with --keep N) and re-reads the installed pair.
# --check builds and writes nothing: it reads the build information the installed pair records and
# prints MATCH <revision> (exit 0), or MISMATCH or UNRECORDED with each tool's build (exit 1).
# Bad arguments, a missing tool or a refused install exit 2. GO names the go command (default go).
set -uo pipefail
export LC_ALL=C

bin_dir=${HOME:-}/.local/bin
keep=1
check=0
while (($# > 0)); do
  case $1 in
    --bin-dir) bin_dir=${2-}; shift 2 || shift ;;
    --keep) keep=${2-}; shift 2 || { keep=''; shift; } ;;
    --check) check=1; shift ;;
    -h | --help) sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'install-cem-verifiers: REFUSE unknown-argument %s\n' "$1" >&2; exit 2 ;;
  esac
done
refuse() { printf 'install-cem-verifiers: REFUSE %s\n' "$*" >&2; exit 2; }
go=${GO:-go}
script=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$script/.." && pwd)
[[ -n $bin_dir ]] || refuse "usage: --bin-dir DIR"
[[ $keep =~ ^[1-9][0-9]*$ ]] || refuse "invalid-keep ${keep:-<empty>} (want 1 or more)"
tools=(corvint cem01-go)

# setting prints the value of one build setting from `go version -m` output, or nothing.
setting() {
  printf '%s\n' "$1" | awk -F'\t' -v key="$2=" '$2 == "build" && index($3, key) == 1 { print substr($3, length(key) + 1) }'
}

# build_of prints "<vcs.revision> <vcs.modified>" from FILE's Go build information, with "-" for a
# setting the build does not record.
build_of() {
  local info rev modified
  info=$("$go" version -m "$1" 2> /dev/null) || { printf -- '- -\n'; return; }
  rev=$(setting "$info" vcs.revision)
  modified=$(setting "$info" vcs.modified)
  printf '%s %s\n' "${rev:--}" "${modified:--}"
}

# target_of prints "<GOOS>/<GOARCH>" from FILE's Go build information.
target_of() {
  local info
  info=$("$go" version -m "$1" 2> /dev/null) || { printf -- '-/-\n'; return; }
  printf '%s/%s\n' "$(setting "$info" GOOS)" "$(setting "$info" GOARCH)"
}

# check_pair compares the builds of DIR/corvint and DIR/cem01-go the way the cem-verdict gate does
# (TCB-V0-045): both must record the same revision and an unmodified tree.
check_pair() {
  local dir=$1 tool line rev modified first='' status=MATCH report=''
  for tool in "${tools[@]}"; do
    [[ -f $dir/$tool ]] || refuse "missing-tool $dir/$tool"
    line=$(build_of "$dir/$tool")
    rev=${line% *}
    modified=${line#* }
    report+=" $tool=$rev"
    [[ $modified == true ]] && report+="+modified"
    if [[ $rev == - || $modified == - ]]; then
      status=UNRECORDED
    elif [[ $status == MATCH ]]; then
      [[ $modified == false && (-z $first || $first == "$rev") ]] || status=MISMATCH
      first=$rev
    fi
  done
  if [[ $status == MATCH ]]; then
    printf 'MATCH %s\n' "$first"
    return 0
  fi
  printf '%s%s\n' "$status" "$report"
  return 1
}

if ((check)); then
  check_pair "$bin_dir"
  exit
fi

[[ -d $bin_dir ]] || refuse "bin-dir-missing $bin_dir"
# The same status Go reads for vcs.modified: any modified, staged or untracked file.
dirty=$(git -C "$repo" status --porcelain) || refuse "git-status-failed $repo"
[[ -z $dirty ]] || refuse "uncommitted-changes $repo (commit, stash or remove them first)"
head=$(git -C "$repo" rev-parse HEAD) || refuse "git-head-failed $repo"
count=$(git -C "$repo" rev-list --count --first-parent HEAD) || refuse "git-head-failed $repo"
# Build for this host whatever GOOS/GOARCH the environment or go env sets: `go version -m` reads a
# foreign binary without running it, so a cross build would otherwise install and still MATCH.
host=$("$go" env GOHOSTOS GOHOSTARCH) || refuse "go-env-failed"
hostos=${host%%$'\n'*}
hostarch=${host#*$'\n'}
[[ -n $hostos && -n $hostarch && $hostarch != "$host" ]] || refuse "go-env-failed GOHOSTOS/GOHOSTARCH"
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-install-cem-verifiers.XXXXXX") || refuse "temp-dir-failed"
trap '/bin/rm -rf "$work"' EXIT
(cd "$repo" && GOOS=$hostos GOARCH=$hostarch GOTOOLCHAIN=local "$go" build -trimpath -ldflags "-X main.build=$count" -o "$work/corvint" ./cmd/corvint) ||
  refuse "build-failed corvint"
(cd "$repo/interop/cem01-go" && GOOS=$hostos GOARCH=$hostarch GOTOOLCHAIN=local "$go" build -trimpath -o "$work/cem01-go" .) ||
  refuse "build-failed cem01-go"
for tool in "${tools[@]}"; do
  line=$(build_of "$work/$tool")
  [[ $line == "$head false" ]] || refuse "build-not-stamped $tool records '$line', want '$head false'"
  line=$(target_of "$work/$tool")
  [[ $line == "$hostos/$hostarch" ]] || refuse "build-not-host $tool targets '$line', want '$hostos/$hostarch'"
done
for tool in "${tools[@]}"; do
  "$script/install-local-bin.sh" --keep "$keep" "$work/$tool" "$bin_dir/$tool" || refuse "install-failed $tool"
done
check_pair "$bin_dir" || refuse "installed-pair-mismatch $bin_dir"
