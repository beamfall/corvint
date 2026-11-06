#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# CIN-V0-4752 (V1-4750): the cadence-plan step of ci.yml and go-macos.yml. Runs the classifier
# beside it (the trusted event-base copy) over the checkout in ./source and writes `cadence` once
# to $GITHUB_OUTPUT: DEFER only when the receipt has the fixed schema, says DEFER and echoes exactly
# this event's base, head and tested commit; anything else is FULL. It always exits zero, so a
# failure here can only leave the campaigns running.
#
# Inputs: BASE, HEAD, CI_CADENCE_FORCE and the GITHUB_* runner variables.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
receipt=${RUNNER_TEMP:-/tmp}/ci-cadence.txt
cadence=FULL
if bash "$here/classify.sh" --root source --event "${GITHUB_EVENT_NAME:-}" \
  --base "${BASE:-}" --head "${HEAD:-}" --target "${GITHUB_SHA:-}" > "$receipt" 2> "$receipt.err"; then
  want=$(printf 'profile=corvint-ci-cadence/0\ncadence=DEFER\nclass=\nbase=%s\nhead=%s\ntarget=%s\nhelper=\nreason=routine-change' \
    "${BASE:-}" "${HEAD:-}" "${GITHUB_SHA:-}")
  # Blank the two free fields (class, helper) and compare every other line exactly.
  got=$(sed -e 's/^class=.*/class=/' -e 's/^helper=.*/helper=/' "$receipt")
  if [ "$got" = "$want" ] && grep -Eqx 'class=(DOCS|TESTS|DOCS\+TESTS)' "$receipt" &&
    grep -Eqx 'helper=sha256:[0-9a-f]{64}' "$receipt"; then
    cadence=DEFER
  fi
else
  echo "classifier exited nonzero: $(head -c 400 "$receipt.err")" >> "$receipt"
fi
echo "cadence=$cadence" >> "${GITHUB_OUTPUT:-/dev/null}"
{
  echo "### CI cadence (CIN-V0-4752): $cadence"
  if [ "$cadence" = DEFER ]; then
    echo 'Routine change: the macOS shards and, in a pull request, the selection-baseline replay and the archive gate are NOT_RUN here; the merge queue runs the replay and the archive gate, and the nightly release gates run all of them in full. Set CI_CADENCE_FORCE=FULL to run them on every change.'
  fi
  echo '```'
  cat "$receipt"
  echo '```'
} >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
exit 0
