#!/usr/bin/env bash
# WQO-V0-921 (V1-2587): check-test-exec-writes refuses an executable os.WriteFile in a test file,
# single- or multi-line and wrapped in a FileMode conversion, and passes a non-executable mode,
# execfile.WriteFile, internal/execfile itself, testdata and a nested module.
set -euo pipefail
script=$(CDPATH='' cd -- "$(dirname "$0")" && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-test-exec-writes-test.XXXXXX")
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
repo=$tmp/repo
mkdir -p "$repo/pkg" "$repo/internal/execfile" "$repo/pkg/testdata" "$repo/nested/inner"
git -C "$repo" init -q -b main
run() { (cd "$repo" && "$script/check-test-exec-writes.sh") > "$tmp/out" 2>&1; }

cat > "$repo/pkg/ok_test.go" <<'GO'
package pkg

func helper() {
	_ = os.WriteFile("data", []byte("x)"), 0o644)
	_ = execfile.WriteFile("tool", []byte("#!/bin/sh\n"), 0o755)
	_ = os.WriteFile("other", nil, os.FileMode(0600))
}
GO
printf 'package execfile\n\nfunc x() { _ = os.WriteFile("a", nil, 0o755) }\n' > "$repo/internal/execfile/execfile_test.go"
printf 'package fixture\n\nfunc x() { _ = os.WriteFile("a", nil, 0755) }\n' > "$repo/pkg/testdata/fixture_test.go"
printf 'module example.com/nested\n' > "$repo/nested/go.mod"
printf 'package inner\n\nfunc x() { _ = os.WriteFile("a", nil, 0700) }\n' > "$repo/nested/inner/inner_test.go"
git -C "$repo" add -A
run || fail "allowed writes failed: $(cat "$tmp/out")"

cat > "$repo/pkg/bad_test.go" <<'GO'
package pkg

func bad() {
	_ = os.WriteFile("tool", []byte("#!/bin/sh\necho \"(\"\n"), 0o755)
	_ = os.WriteFile(
		"tool2",
		[]byte(`exit 0`),
		fs.FileMode(0700),
	)
}
GO
git -C "$repo" add -A
if run; then fail "an executable os.WriteFile passed"; fi
grep -q '^check-test-exec-writes: FAIL pkg/bad_test.go:4: os.WriteFile writes an executable (mode 0o755)' "$tmp/out" &&
  grep -q '^check-test-exec-writes: FAIL pkg/bad_test.go:5: os.WriteFile writes an executable (mode 0700)' "$tmp/out" ||
  fail "the executable writes were not named: $(cat "$tmp/out")"
test "$(grep -c FAIL "$tmp/out")" = 2 || fail "an allowed write was reported: $(cat "$tmp/out")"
echo "check-test-exec-writes_test.sh: ok"
