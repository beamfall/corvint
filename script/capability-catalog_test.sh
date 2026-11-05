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
# Every snapshot the recipe names, counted from the Makefile so a new tree line needs no edit here.
want=$(grep -c 'capcatalog_write,.*,internal/capcatalog/trees/' "$repo/Makefile")
[ "$(find "$w/internal/capcatalog/trees" -name '*.json' | wc -l)" -eq "$want" ] ||
  fail "not all $want trees snapshots were written"
[ -z "$(leftover)" ] || fail "a succeeding run left $(leftover)"

# CAP-V0-9300: one regeneration must refresh the canonical folded tree before
# deriving compatibility shim trees, and derive documentation last. Model a
# changed folded help source with a deliberately stale initial snapshot.
cat > "$tmp/gen" <<'STUB'
#!/bin/sh
case "$1" in
  ./cmd/corvint-test) cat folded-source ;;
  ./tools/cli-discovery-gen)
    if [ "${2:-}" = --docs ]; then
      cat internal/capcatalog/trees/corvint-tests-accept.json > docs.txt
    else
      for shim in corvint-tests-accept corvint-test-runner corvint-behavior-falsify corvint-playwright-minimize; do
        cp internal/capcatalog/trees/corvint-test.json "internal/capcatalog/trees/$shim.json"
      done
    fi ;;
  *) printf 'generated %s %s\n' "$2" "$3" ;;
esac
STUB
printf 'new folded help\n' > "$w/folded-source"
printf 'stale folded help\n' > "$w/internal/capcatalog/trees/corvint-test.json"
run || fail "folded regeneration failed: $(cat "$tmp/out")"
for shim in corvint-tests-accept corvint-test-runner corvint-behavior-falsify corvint-playwright-minimize; do
  cmp -s "$w/folded-source" "$w/internal/capcatalog/trees/$shim.json" ||
    fail "$shim consumed stale folded help"
done
cmp -s "$w/folded-source" "$w/docs.txt" || fail "documentation consumed stale shim help"
cp -R "$w" "$tmp/once"
run || fail "second folded regeneration failed: $(cat "$tmp/out")"
diff -r "$tmp/once" "$w" || fail "one regeneration did not reach a stable result"
echo "capability-catalog_test: PASS"
