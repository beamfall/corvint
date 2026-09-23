#!/bin/sh
set -eu

# RCB-V0-006: the offline verifier reports a matching bundle as PASS and names the exact file of a
# tampered, missing, or extra receipt. Bundles are written by hand in the manifest's frozen
# line-oriented shape, so the property under test is the verifier alone.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
verify="$source_root/script/verify-receipt-bundle.sh"
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-receipt-bundle-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum < "$1"; else shasum -a 256 < "$1"; fi | awk '{print $1}'; }

base=$(printf 'a%.0s' 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40)
target=$(printf '%s' "$base" | tr a b)

# bundle NAME writes a bundle with a present cem and gate receipt and prints its directory.
bundle() {
    dir="$test_root/$1"
    mkdir -p "$dir/receipts"
    printf '{"spec":"cem/0.2"}\n' > "$dir/receipts/cem.json"
    printf 'corvint-gate-receipt/0 a b c\n' > "$dir/receipts/gate-receipt.txt"
    {
        printf '{"profile":"corvint-receipt-bundle/0","base":"%s","target":"%s","receipts":[\n' "$base" "$target"
        printf '{"kind":"cem","state":"present","file":"receipts/cem.json","sha256":"%s","source":"x","base":"a","target":"b","notRunOrNotProduced":[]},\n' "$(sha "$dir/receipts/cem.json")"
        printf '{"kind":"witness","state":"absent","source":"--witness","reason":"not-supplied"},\n'
        printf '{"kind":"dogfood","state":"absent","source":".corvint/dogfood-report.json","reason":"not-found"},\n'
        printf '{"kind":"gate-receipt","state":"present","file":"receipts/gate-receipt.txt","sha256":"%s","source":"y","target":"b","notRunOrNotProduced":[]}\n' "$(sha "$dir/receipts/gate-receipt.txt")"
        printf ']}\n'
    } > "$dir/manifest.json"
    printf '%s\n' "$dir"
}

# restructure NAME SED_SCRIPT writes a bundle whose manifest SED_SCRIPT rewrites and checks that the
# verifier refuses it as unusable.
restructure() {
    dir=$(bundle "$1")
    sed "$2" "$dir/manifest.json" > "$dir/rewritten"
    mv "$dir/rewritten" "$dir/manifest.json"
    code=0
    sh "$verify" "$dir" >/dev/null 2>&1 || code=$?
    [ "$code" -eq 2 ] || fail "$1: exit $code, want 2"
}

# run DIR WANT_EXIT runs the verifier and prints its output.
run() {
    code=0
    output=$(sh "$verify" "$1") || code=$?
    [ "$code" -eq "$2" ] || fail "exit $code, want $2: $output"
    printf '%s\n' "$output"
}

match=$(bundle match)
out=$(run "$match" 0)
printf '%s\n' "$out" | grep -qx "manifest sha256 $(sha "$match/manifest.json")" || fail "match: manifest digest: $out"
[ "$(printf '%s\n' "$out" | grep -c '^MATCH ')" -eq 2 ] || fail "match: two MATCH lines: $out"
[ "$(printf '%s\n' "$out" | tail -n 1)" = PASS ] || fail "match: PASS: $out"

tampered=$(bundle tampered)
expected=$(sha "$tampered/receipts/cem.json")
printf 'x' >> "$tampered/receipts/cem.json"
out=$(run "$tampered" 1)
printf '%s\n' "$out" | grep -qx "MISMATCH receipts/cem.json expected=$expected actual=$(sha "$tampered/receipts/cem.json")" ||
    fail "tampered: exact mismatch: $out"
printf '%s\n' "$out" | grep -qx 'MATCH receipts/gate-receipt.txt .*' || fail "tampered: untouched file still matches: $out"
[ "$(printf '%s\n' "$out" | tail -n 1)" = FAIL ] || fail "tampered: FAIL: $out"

missing=$(bundle missing)
rm "$missing/receipts/gate-receipt.txt"
out=$(run "$missing" 1)
printf '%s\n' "$out" | grep -qx 'MISSING receipts/gate-receipt.txt' || fail "missing: $out"

linked=$(bundle linked)
mv "$linked/receipts/cem.json" "$test_root/outside.json"
ln -s "$test_root/outside.json" "$linked/receipts/cem.json"
out=$(run "$linked" 1)
printf '%s\n' "$out" | grep -qx 'MISSING receipts/cem.json' || fail "symlink is not a bundle file: $out"

extra=$(bundle extra)
printf 'unlisted\n' > "$extra/receipts/notes.txt"
out=$(run "$extra" 1)
printf '%s\n' "$out" | grep -qx 'EXTRA receipts/notes.txt' || fail "extra: $out"
[ "$(printf '%s\n' "$out" | grep -c '^MATCH ')" -eq 2 ] || fail "extra: listed files still match: $out"

unusable="$test_root/unusable"
mkdir -p "$unusable"
printf '{"profile":"other/0"}\n' > "$unusable/manifest.json"
run "$unusable" 2 >/dev/null 2>&1 || fail "a foreign manifest is unusable"
run "$test_root/absent" 2 >/dev/null 2>&1 || fail "an absent manifest is unusable"

zero=0000000000000000000000000000000000000000000000000000000000000000
restructure duplicated "3s/.*/{\"kind\":\"cem\",\"state\":\"present\",\"file\":\"receipts\/cem.json\",\"sha256\":\"$zero\",\"source\":\"x\"},/"
restructure reordered '3{h;d;};4{G;s/},\n\(.*\),$/},\n\1,/;}'
restructure traversal 's|"file":"receipts/gate-receipt.txt"|"file":"receipts/../../outside.txt"|'
restructure "absent cem" '2s/.*/{"kind":"cem","state":"absent","source":"--map","reason":"not-found"},/'
restructure "no closing line" '$d'
restructure crlf 's/$/\r/'
restructure "50-hex base" "1s/\"base\":\"$base/&aaaaaaaaaa/"

echo "verify-receipt-bundle tests: PASS"
