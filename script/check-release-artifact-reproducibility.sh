#!/bin/sh
# Reproducible-build gate for the Go candidate (GPK-V0-018 / GPK-V0-019).
# Builds every declared target twice in one profile and proves the outputs are
# byte-identical. Local evidence only: it never publishes, signs, or uploads.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
output=${CORVINT_RELEASE_ARTIFACT_OUTPUT:-/tmp/corvint-release-artifact}

case $output in
  "$root"|"$root"/*)
    echo "check-release-artifact-reproducibility: output must be outside $root" >&2
    exit 2
    ;;
esac

mkdir -p "$output"
GOTOOLCHAIN=local exec go run ./conformance/release-artifact-v0 \
  --root "$root" \
  --manifest "$root/conformance/release-artifact-v0/manifest.json" \
  --output "$output" \
  --report "$output/report.json"
