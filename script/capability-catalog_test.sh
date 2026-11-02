#!/usr/bin/env bash
# Temp-fixture test for make capability-catalog (V1-3522, ASD-V0-918): each snapshot is renamed into
# place only after its generator succeeded. A failing generator leaves catalog.json byte-identical,
# leaves no .tmp file and makes the target exit nonzero; a succeeding one replaces every snapshot.
# The Makefile runs in a scratch directory with CAPCATALOG_RUN standing in for go run, so no
# repository file is touched.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/capability-catalog-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
fail() { echo "capability-catalog_test: FAIL $*" >&2; exit 1; }

cat > "$tmp/gen" <<'STUB'
#!/bin/sh
# Stand-in generator: prints a partial line, then fails when GEN_FAIL names its package.
printf 'partial %s\n' "$1"
[ "${GEN_FAIL:-}" = "$1" ] && exit 1
printf 'generated %s %s\n' "$2" "$3"
STUB
chmod +x "$tmp/gen"
w=$tmp/w
mkdir -p "$w/internal/capcatalog/trees"
printf '{"old":"catalog"}\n' > "$w/internal/capcatalog/catalog.json"
printf '{"old":"tree"}\n' > "$w/internal/capcatalog/trees/corvint.json"
cp "$w/internal/capcatalog/catalog.json" "$tmp/catalog.before"
leftover() { find "$w" -name '*.tmp'; }
run() { make -s -C "$w" -f "$repo/Makefile" capability-catalog CAPCATALOG_RUN="$tmp/gen" >"$tmp/out" 2>&1; }

if GEN_FAIL=./cmd/corvint run; then fail "a failing generator exited 0: $(cat "$tmp/out")"; fi
cmp -s "$tmp/catalog.before" "$w/internal/capcatalog/catalog.json" ||
  fail "a failing generator changed catalog.json: $(cat "$w/internal/capcatalog/catalog.json")"
[ -z "$(leftover)" ] || fail "a failing generator left $(leftover)"
grep -q '"old":"tree"' "$w/internal/capcatalog/trees/corvint.json" ||
  fail "a failing first generator still rewrote a later snapshot"

run || fail "a succeeding generator exited nonzero: $(cat "$tmp/out")"
grep -qx 'generated help --json' "$w/internal/capcatalog/catalog.json" ||
  fail "catalog.json was not replaced: $(cat "$w/internal/capcatalog/catalog.json")"
grep -qx 'generated help --tree' "$w/internal/capcatalog/trees/corvint.json" ||
  fail "trees/corvint.json was not replaced"
[ "$(find "$w/internal/capcatalog/trees" -name '*.json' | wc -l)" -eq 15 ] ||
  fail "not all 15 trees snapshots were written"
[ -z "$(leftover)" ] || fail "a succeeding run left $(leftover)"
echo "capability-catalog_test: PASS"
