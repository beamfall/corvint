#!/bin/sh
set -eu

# Diagnostic coverage ratchet (docs/specs/diagnostic-repair-contract-v0.md, DRC-V0-011/012).
#
# The gate is a Go test because a multi-line diagnostic.Refusal composite literal cannot be
# checked for its keys without a parser (decision 0201). TestDiagnosticCoverageRatchet parses
# non-test Go under cmd/ and internal/, fails any refusal literal without a subject, without a
# registered supported_fixes list or terminal reason, or any DRC-V0-012 message parse, and
# requires the covered-site count to equal script/diagnostic-coverage.count, which may not be
# below seven. Raise the recorded count in the same change that converts a site.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^TestDiagnosticCoverageRatchet$' ./internal/diagnostic
