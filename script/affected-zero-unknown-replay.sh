#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# AZU-V0-009: replay exact frozen merge trees, with fail-closed safety receipts.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
exec "$root/script/heavy.sh" go run "$root/tools/affected-zero-unknown-replay" --root "$root" "$@"
