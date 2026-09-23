#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Recipe 3: verify portable change evidence in CI with the digest-pinned verifier. Runs
# examples/cem/verify-portable.sh (CEM-PILOT-020..023) under a wall-clock bound, keeps its
# cem-ci-report/0 line, and names the recovery for a non-accepted verdict. It needs no hosted
# Corvint service, no LLM and no network.
#
# Required environment (passed through to verify-portable.sh):
#   CEM_BASE_SHA, CEM_HEAD_SHA   exact 40- or 64-hex commits
#   CEM_VERIFIER                 the fetched interop/cem01-go executable
#   CEM_VERIFIER_SHA256          its pinned lowercase SHA-256
# Optional environment:
#   CEM_REPOSITORY, CEM_HEAD_REPOSITORY, CEM_MAP_PATH   as for verify-portable.sh
#   RECIPE_OUT       directory for the retained report (default: a new temp dir)
#   RECIPE_TIMEOUT   wall-clock bound in seconds (default 600)
#
# Exit status is the verifier's: 0 accepted, 1 rejected, 2 operational (also a digest mismatch,
# bad input, or the bound expiring), 3 missing evidence, 4 unsupported profile, 5 repository
# mismatch. The report is RECIPE_OUT/verify.out; nothing is published.

set -u
# shellcheck source-path=SCRIPTDIR source=bounded.sh
. "$(dirname "$0")/bounded.sh"

recovery=(
  'none'
  'rejected: read drift and code in the report, re-cite or re-prepare the map, commit, rerun'
  'operational: read verify.stderr; check the pins, SHAs and repository paths'
  'missing-evidence: commit a cem/0.1 map, or cite or mark every unknown hunk, then rerun'
  'unsupported-profile: this verifier reads cem/0.1 only; use verify-pr.sh for cem/0.2'
  'repository-mismatch: fetch both commits and prepare the map against the declared base'
)

status=0
run_step verify "$(dirname "$0")/../verify-portable.sh" || status=$?
verdict=$(grep -o '"verdict":"[a-z-]*"' "$recipe_out/verify.out" | cut -d'"' -f4)
if [ "$status" -eq 124 ]; then
  printf 'outcome=operational timeout=verify out=%s\n' "$recipe_out"
  exit 2
fi
if [ "$status" -gt 5 ]; then status=2; fi
[ -n "$verdict" ] || status=2
printf 'outcome=%s exit=%s recovery=%s out=%s\n' "${verdict:-operational}" "$status" \
  "${recovery[$status]}" "$recipe_out"
exit "$status"
