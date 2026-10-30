#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-480: write the shadow comparison tiers of BASE..TARGET to OUT for selection.jq.
# Usage: tiers.sh BASE TARGET OUT, run in the tested checkout's module root. Only git and a
# static `go list` with GOPROXY=off run: no network, no build, no test. OUT is always
# written: {tiers, tierIncomplete, millis} (millis is the wall time of this script, null
# where `date` has no millisecond format). A step that fails names
# the failure in tierIncomplete and leaves tiers null, which the forward row reports as
# unmeasured; it never decides, executes or narrows anything.
set -u
base=$1 target=$2 out=$3
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d) || {
  printf '{"tiers":null,"tierIncomplete":["mktemp failed"],"millis":null}\n' > "$out"
  exit 0
}
trap 'rm -rf "$work"' EXIT
# Milliseconds since the epoch: the first 13 digits of seconds plus nanoseconds, since some
# `date` implementations print every nanosecond digit for %3N; empty where %N is unsupported.
now() {
  t=$(date +%s%N 2> /dev/null) || t=""
  case "$t" in '' | *[!0-9]*) t="" ;; esac
  [ "${#t}" -ge 13 ] || t=""
  printf '%.13s' "$t"
}
start=$(now)
fail() {
  jq -n -c --arg reason "$1" '{tiers: null, tierIncomplete: [$reason], millis: null}' > "$out"
  exit 0
}
git diff --name-only --no-renames "$base" "$target" -- > "$work/changed" 2> "$work/err" ||
  fail "git diff $base $target failed: $(head -c 200 "$work/err" | tr '\n' ' ')"
# A deleted file is absent from the target's go list, so its embedding is judged from the
# target's embed patterns (tiers.jq).
git diff --name-only --no-renames --diff-filter=D "$base" "$target" -- > "$work/deleted" 2> "$work/err" ||
  fail "git diff --diff-filter=D $base $target failed: $(head -c 200 "$work/err" | tr '\n' ' ')"
# AFP-V0-520: the target's documentation declaration (AFP-V0-160), empty when it has none. A change
# to the declaration is itself a non-Go change, so the target's and the base's agree whenever it
# exempts a path.
: > "$work/documentation"
if git cat-file -e "$target:.corvint/documentation-paths.json" 2> /dev/null; then
  git cat-file blob "$target:.corvint/documentation-paths.json" > "$work/documentation" 2> "$work/err" ||
    fail "git cat-file of the documentation declaration failed: $(head -c 200 "$work/err" | tr '\n' ' ')"
fi
module=$(GOPROXY=off GOFLAGS='' go list -m 2> "$work/err") ||
  fail "go list -m failed: $(head -c 200 "$work/err" | tr '\n' ' ')"
GOPROXY=off GOFLAGS='' go list -e \
  -json=ImportPath,Imports,TestImports,XTestImports,EmbedFiles,TestEmbedFiles,XTestEmbedFiles,EmbedPatterns,TestEmbedPatterns,XTestEmbedPatterns ./... \
  > "$work/packages.json" 2> "$work/err" ||
  fail "go list ./... failed: $(head -c 200 "$work/err" | tr '\n' ' ')"
jq -n -c --rawfile changed "$work/changed" --rawfile deleted "$work/deleted" --rawfile documentation "$work/documentation" \
  --slurpfile pkgs "$work/packages.json" --arg module "$module" \
  -f "$here/tiers.jq" > "$work/tiers.json" 2> "$work/err" ||
  fail "tiers.jq failed: $(head -c 200 "$work/err" | tr '\n' ' ')"
end=$(now)
jq -c --arg start "$start" --arg end "$end" \
  '. + {millis: (if $start != "" and $end != "" then ($end | tonumber) - ($start | tonumber) else null end)}' \
  "$work/tiers.json" > "$out" || fail "the tier document could not be written"
