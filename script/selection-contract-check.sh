#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Replay both actual consumers over the pinned selection-baseline corpus.
# Usage: script/selection-contract-check.sh (from the repository root).
# This gate never reads old baseline result-cache entries: those lack parity.
set -eu
d=$(mktemp -d)
trap 'rm -rf "$d"' EXIT HUP INT TERM
GOTOOLCHAIN=local go build -trimpath -buildvcs=false -o "$d/" \
    ./cmd/corvint ./tools/gate-affected-select ./tools/selection-baseline
SELECTION_CONTRACT_CHECK=1 SELECTION_BASELINE_CACHE=off SELECTION_BASELINE_JOBS=1 \
    "$d/selection-baseline" --corvint "$d/corvint" --selector "$d/gate-affected-select"
