#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-800: print the effective base of EVENT_BASE HEAD TESTED_MERGE.
# No output on refusal; callers must keep full CI / incomplete shadow evidence.
set -eu
export GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/dev/null
git() { command git -c advice.graftFileDeprecated=false "$@"; }
refuse() { echo "$1" >&2; exit 1; }
hex() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
[ "$#" = 3 ] || refuse pr-base-invalid-oid
hex "$1" && hex "$2" && hex "$3" || refuse pr-base-invalid-oid
[ "$(git rev-parse HEAD)" = "$3" ] || refuse pr-base-head-drift
parents=$(git show -s --format=%P "$3") || refuse pr-base-parents-unreadable
event=$1 head=$2
set -- $parents
[ "$#" = 2 ] || refuse pr-base-parent-count
[ "$2" = "$head" ] || refuse pr-base-head-mismatch
git merge-base --is-ancestor "$event" "$1" || refuse pr-base-not-ancestor
printf '%s\n' "$1"
