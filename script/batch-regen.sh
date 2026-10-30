#!/usr/bin/env bash
# script/batch-regen.sh (ASD-V0-033, V1-2899; ASD-V0-036, V1-2900): regenerate, in one fixed order
# and one commit, every derived file that two lanes of one batch can each leave stale for the
# other, so the integrator fixes cross-lane drift once instead of once per CI round.
# `make batch-regen` runs it.
#
#   script/batch-regen.sh [--no-commit]   run every step; commit the result as one commit
#   script/batch-regen.sh --mechanical    only the mechanical repins (host package versions,
#                                         moved line citations and use-case receipts) on the
#                                         worktree, never staging or committing;
#                                         script/batch-precheck.sh runs this first
#   script/batch-regen.sh --golden-tests  print the "<package> <test>" goldens it rewrites
#
# Order: spec-requirements (REQUIREMENTS.tsv, INDEX.json, README.md), capability-catalog
# (internal/capcatalog), analyzer-schema (bump only when `script/analyzer-schema.sh check` fails),
# the GOLDEN_TESTS goldens (CORVINT_UPDATE_GOLDEN=1, always local: HEAVY_REMOTE=0), host package
# version bumps, the line-citation repin (`script/check-line-citations.sh --repin`), and last the
# use-case receipt repin, because receipts pin the other outputs.
# Each step's changes are staged before the next, since the spec and citation generators read the
# Git index. A failing step stops the run with its changes staged and nothing committed.
#
# The full run refuses a checkout with tracked changes, so its commit holds only regenerated
# files, and it is idempotent: a second run on its own commit changes nothing and commits nothing.
# A host package whose version file already differs from HEAD in the worktree is not bumped
# again, because host-package-versions-check reads committed history only.
#
# The line-citation repin rewrites only citations whose cited lines moved unchanged; a changed
# span needs a human read before its anchor is rewritten, so it stays a batch-precheck failure
# (line-citations-check). Not regenerated: tests that pin their expectation in Go source
# (TestAffectedReceiptMembersAreClosedAndByteStable, the spec enumeration tests): batch-precheck
# runs them, and the fix is a code edit.
set -uo pipefail
export LC_ALL=C

root=${BATCH_REGEN_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)} || exit 2
cd "$root" || exit 2
heavy=${BATCH_REGEN_HEAVY-script/heavy.sh}
# ASD-V0-070 (V1-3233): with no HOME the go test step refuses (exit 3) instead of aborting on it.
cache_home=${XDG_CACHE_HOME:-${HOME:+$HOME/.cache}}
gocache=${CORVINT_GOCACHE:-${cache_home:+$cache_home/corvint-go-build-cache}}

# Every test that rewrites its golden under CORVINT_UPDATE_GOLDEN=1. batch-precheck --sync-check
# fails when a test that reads that variable is missing here.
GOLDEN_TESTS=(
  "./cmd/corvint TestCoreVerbsEmitTheFrozenProfiles"
  "./cmd/corvint TestAFUV1024StrictAndCoverageBytesUnchanged"
  "./cmd/corvint TestAFUV1040SelectionCorpusReport"
)

goldens() {
  local pkgs=() names='' line pkg
  for line in "${GOLDEN_TESTS[@]}"; do
    pkg=${line%% *}
    case " ${pkgs[*]-} " in *" $pkg "*) ;; *) pkgs+=("$pkg") ;; esac
    names+="${names:+|}${line#* }"
  done
  [[ -n $gocache ]] || { printf 'batch-regen: REFUSE home-unset: HOME, XDG_CACHE_HOME and CORVINT_GOCACHE are all unset; set HOME (or CORVINT_GOCACHE) and rerun\n' >&2; exit 3; }
  env CORVINT_UPDATE_GOLDEN=1 HEAVY_REMOTE=0 HEAVY_LOCAL_REASON=golden-regen $heavy \
    env GOCACHE="$gocache" GOTOOLCHAIN=local go test -count=1 -run "^($names)\$" "${pkgs[@]}"
}

analyzer() {
  script/analyzer-schema.sh check > /dev/null 2>&1 && return 0
  script/analyzer-schema.sh bump
}

# host_packages bumps each package host-package-versions-check names, unless one of its version
# files already differs from HEAD (an uncommitted bump the committed-history check cannot see).
host_packages() {
  local out pkgs pkg path
  out=$(script/check-host-package-versions.sh 2>&1) && return 0
  pkgs=$(printf '%s\n' "$out" | sed -n 's/^host package versions: \([a-z0-9-]*\): .*run make host-package-version-bump PKG=.*/\1/p' | sort -u)
  [[ -n $pkgs ]] || { printf '%s\n' "$out" >&2; return 1; }
  for pkg in $pkgs; do
    while read -r path; do
      git diff --quiet HEAD -- "$path" || { printf 'batch-regen: %s already bumped in the worktree (%s)\n' "$pkg" "$path"; continue 2; }
    done < <(printf '%s\n' "$out" | sed -n "s/^host package versions: $pkg: \\([^ ]*\\) last bumped at .*/\\1/p")
    script/host-package-version.sh bump "$pkg" || return 1
  done
}

citations() { script/check-line-citations.sh --repin; }

receipts() { script/repin-use-case-receipts.sh; }

# Steps as "name<TAB>command"; BATCH_REGEN_STEPS replaces them (tests).
STEPS=(
  "spec-requirements	make -s spec-requirements"
  "capability-catalog	make -s capability-catalog"
  "analyzer-schema	analyzer"
  "goldens	goldens"
  "host-package-versions	host_packages"
  "line-citations	citations"
  "use-case-receipts	receipts"
)
MECHANICAL=(
  "host-package-versions	host_packages"
  "line-citations	citations"
  "use-case-receipts	receipts"
)
if [[ -n ${BATCH_REGEN_STEPS:-} ]]; then
  STEPS=()
  while IFS= read -r line; do [[ -n $line ]] && STEPS+=("$line"); done <<< "$BATCH_REGEN_STEPS"
  MECHANICAL=("${STEPS[@]}")
fi

# stage adds every tracked change and every untracked file that was not there before the run.
stage() {
  local f
  git add -u || return 1
  while IFS= read -r f; do
    [[ -n $f ]] && ! grep -qxF -- "$f" "$untracked_before" && { git add -- "$f" || return 1; }
  done < <(git ls-files --others --exclude-standard)
  return 0
}

run_step() {
  local name=$1 cmd=$2 t0=$SECONDS
  if eval "$cmd"; then
    printf 'batch-regen: PASS %s (%ss)\n' "$name" $((SECONDS - t0))
  else
    printf 'batch-regen: FAIL %s (%ss)\n' "$name" $((SECONDS - t0))
    return 1
  fi
}

mode=full commit=1
case ${1:-} in
  --golden-tests) printf '%s\n' "${GOLDEN_TESTS[@]}"; exit 0 ;;
  --mechanical) mode=mechanical ;;
  --no-commit) commit=0 ;;
  '') ;;
  *) printf 'usage: script/batch-regen.sh [--no-commit|--mechanical|--golden-tests]\n' >&2; exit 2 ;;
esac

if [[ $mode == mechanical ]]; then
  for step in "${MECHANICAL[@]}"; do
    run_step "${step%%	*}" "${step#*	}" || exit 1
  done
  exit 0
fi

if ! git diff --quiet HEAD -- || ! git diff --cached --quiet HEAD --; then
  printf 'batch-regen: refused: commit or stash tracked changes first, so the regen commit holds only derived files\n' >&2
  exit 2
fi
untracked_before=$(mktemp "${TMPDIR:-/tmp}/batch-regen.XXXXXX") || exit 2
trap '/bin/rm -f "$untracked_before"' EXIT
git ls-files --others --exclude-standard > "$untracked_before"
start=$SECONDS
for step in "${STEPS[@]}"; do
  run_step "${step%%	*}" "${step#*	}" < /dev/null || { stage; printf 'batch-regen: stopped; changes so far are staged, nothing committed\n' >&2; exit 1; }
  stage || exit 1
done
if git diff --cached --quiet HEAD --; then
  printf 'batch-regen: nothing to regenerate (%ss)\n' $((SECONDS - start))
  exit 0
fi
git diff --cached --name-only | sed 's/^/batch-regen: regenerated /'
if ((commit)); then
  git commit -q -m 'chore(batch): regenerate derived files (make batch-regen, ASD-V0-033)' || exit 1
  printf 'batch-regen: committed %s (%ss)\n' "$(git rev-parse --short HEAD)" $((SECONDS - start))
else
  printf 'batch-regen: staged, not committed (%ss)\n' $((SECONDS - start))
fi
