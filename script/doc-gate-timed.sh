#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/doc-gate-timed.sh TARGET COMMAND [ARG...]
# DTL-V0-006: foreground timing, including failed gates; no cached or skipped checks.
set -uo pipefail
(($# >= 2)) || { echo 'usage: doc-gate-timed.sh TARGET COMMAND [ARG...]' >&2; exit 2; }
target=$1
shift
SECONDS=0
"$@"
rc=$?
printf 'doc-gate-timing: target=%s seconds=%s exit=%s\n' "$target" "$SECONDS" "$rc" >&2
exit "$rc"
