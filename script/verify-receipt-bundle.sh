#!/bin/sh
# RCB-V0-006: recompute every digest a `corvint cem export` receipt bundle lists, from the bundle
# alone, with no Corvint binary and no network. Prints one line per listed receipt file (MATCH,
# MISMATCH with both digests, or MISSING), one EXTRA line per unlisted file, then PASS or FAIL.
# Exit 0 on PASS, 1 on FAIL, 2 when the bundle has no usable manifest.
set -euf

usage() { echo "usage: script/verify-receipt-bundle.sh BUNDLE_DIR" >&2; exit 2; }
[ "$#" -eq 1 ] || usage
bundle=${1%/}
manifest="$bundle/manifest.json"

if command -v sha256sum >/dev/null 2>&1; then
    hash() { sha256sum < "$1" | awk '{print $1}'; }
else
    hash() { shasum -a 256 < "$1" | awk '{print $1}'; }
fi

if [ -L "$manifest" ] || [ ! -f "$manifest" ]; then
    echo "verify-receipt-bundle: $manifest is not a regular file" >&2
    exit 2
fi
if ! head -n 1 "$manifest" | grep -q '^{"profile":"corvint-receipt-bundle/0",'; then
    echo "verify-receipt-bundle: $manifest is not a corvint-receipt-bundle/0 manifest" >&2
    exit 2
fi
printf 'manifest sha256 %s\n' "$(hash "$manifest")"

pairs=$(sed -n -E 's/.*"file":"(receipts\/[a-z][a-z-]*\.[a-z]+)","sha256":"([0-9a-f]{64})".*/\1 \2/p' "$manifest")
listed=$(grep -c '"state":"present"' "$manifest" || true)
status=0
if [ "$(printf '%s' "$pairs" | grep -c . || true)" -ne "$listed" ]; then
    echo "MALFORMED manifest.json lists $listed present receipts but not one file and digest for each"
    status=1
fi

for path in $(printf '%s\n' "$pairs" | awk 'NF { print $1 }'); do
    expected=$(printf '%s\n' "$pairs" | awk -v p="$path" '$1 == p { print $2; exit }')
    file="$bundle/$path"
    if [ -L "$file" ] || [ ! -f "$file" ]; then
        echo "MISSING $path"
        status=1
        continue
    fi
    actual=$(hash "$file")
    if [ "$actual" = "$expected" ]; then
        echo "MATCH $path $actual"
    else
        echo "MISMATCH $path expected=$expected actual=$actual"
        status=1
    fi
done

for file in $(cd "$bundle" && find . ! -type d | sed 's|^\./||' | LC_ALL=C sort); do
    [ "$file" = manifest.json ] && continue
    printf '%s\n' "$pairs" | awk -v p="$file" '$1 == p { found = 1 } END { exit !found }' && continue
    echo "EXTRA $file"
    status=1
done

if [ "$status" -eq 0 ]; then echo PASS; else echo FAIL; fi
exit "$status"
