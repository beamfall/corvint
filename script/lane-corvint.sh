#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/lane-corvint.sh
# DTL-V0-003: report installed freshness, then build and print this worktree's executable.
# Example: LANE_CORVINT=$(script/lane-corvint.sh); "$LANE_CORVINT" affected --worktree --path PATH
set -uo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd) || exit 2
"$repo/script/lane-binary-check.sh" --against origin/main "$repo" >&2 || :
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
out=$git_dir/corvint/lane-corvint
mkdir -p "${out%/*}" || exit 2
# Always rebuild from current bytes; Go's content-addressed build cache makes unchanged builds cheap.
(cd "$repo" && "$repo/script/heavy.sh" --admit-only env GOTOOLCHAIN=local go build -o "$out" ./cmd/corvint) >&2 || {
  echo 'lane-corvint: REFUSE worktree-build-failed; fix: repair the compile error above, then rerun script/lane-corvint.sh' >&2
  exit 2
}
printf '%s\n' "$out"
