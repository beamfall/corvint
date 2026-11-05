#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# CIN-V0-2304 / ASD-V0-036: exact mirror of the trusted CI batch check.
set -eu
case "$EVENT_NAME:$HEAD_REF" in
  pull_request:claude/batch-*) ;;
  *) echo "not a batch pull request; skipped"; exit 0 ;;
esac
base=$(git merge-base "$PR_BASE" "$PR_HEAD")
git worktree add --detach "$RUNNER_TEMP/trusted" "$PR_BASE"
(cd "$RUNNER_TEMP/trusted" && GOTOOLCHAIN=local go build -trimpath -buildvcs=false -o "$RUNNER_TEMP/corvint-trusted" ./cmd/corvint)
git worktree add --detach "$RUNNER_TEMP/batch-head" "$PR_HEAD"
cd "$RUNNER_TEMP/batch-head"
test "$(git rev-parse HEAD)" = "$PR_HEAD"
"$RUNNER_TEMP/corvint-trusted" dogfood batch-check --verifier "$RUNNER_TEMP/corvint-trusted" "$base"
