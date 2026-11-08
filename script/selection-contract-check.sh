#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Replay both actual consumers over the pinned selection-baseline corpus.
# Usage: script/selection-contract-check.sh (from the repository root).
# It shares selection-baseline-check's input decision: a diff disjoint from the
# three binaries' compilation closure and the driver's reads skips (AFP-V0-9501).
# The result cache keys SELECTION_CONTRACT_CHECK, so only records that ran the
# parity assertions can be reused; replay uses the default parallelism.
set -eu
d=$(mktemp -d)
trap 'rm -rf "$d"' EXIT HUP INT TERM
GOTOOLCHAIN=local go build -trimpath -buildvcs=false -o "$d/" ./tools/selection-baseline
decision=$(SELECTION_BASELINE_INPUTS_BASE="${SELECTION_BASELINE_INPUTS_BASE-}" "$d/selection-baseline")
case "$decision" in
'skip '*) echo "selection-contract-check skipped: ${decision#skip }"; exit 0 ;;
*) echo "selection-contract-check runs: ${decision#run }" ;;
esac
# The replay must not see the decision input, or it answers the decision again.
unset SELECTION_BASELINE_INPUTS_BASE
GOTOOLCHAIN=local go build -trimpath -buildvcs=false -o "$d/" ./cmd/corvint ./tools/gate-affected-select
SELECTION_CONTRACT_CHECK=1 \
    "$d/selection-baseline" --corvint "$d/corvint" --selector "$d/gate-affected-select"
