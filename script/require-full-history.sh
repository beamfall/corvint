#!/usr/bin/env bash
# usage: script/require-full-history.sh NAME [DIR]
#
# ASD-V0-072 (V1-3230): refuse, before any range is computed, when the repository holding DIR
# (default the working directory) is a shallow clone. Its grafted history can hide a base commit
# or the merge base of a range, so merge-base, `corvint affected` and the dogfood ranges would fail
# or mislead before anything said the clone is shallow. On refusal it prints
# `NAME: REFUSE shallow-repository ...` with the `git fetch --unshallow origin` recovery and exits
# 2; a full clone, or a directory Git cannot inspect (left to the caller's own checks), exits 0.
set -uo pipefail
name=${1:?usage: require-full-history.sh NAME [DIR]}
dir=${2:-.}
shallow=$(git -C "$dir" rev-parse --is-shallow-repository 2> /dev/null) || exit 0
[[ $shallow == true ]] || exit 0
top=$(git -C "$dir" rev-parse --show-toplevel 2> /dev/null) || top=$dir
printf '%s: REFUSE shallow-repository: %s is a shallow clone, so a base commit or merge base may lie past its shallow boundary; run `git -C %q fetch --unshallow origin` (or reclone without --depth), then rerun\n' "$name" "$top" "$top" >&2
exit 2
