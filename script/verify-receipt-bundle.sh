#!/bin/sh
# RCB-V0-006: recompute every digest a `corvint cem export` receipt bundle lists, from the bundle
# alone, with no Corvint binary and no network. Prints one line per listed receipt file (MATCH,
# MISMATCH with both digests, or MISSING), one EXTRA line per unlisted file, then PASS or FAIL.
# Exit 0 on PASS, 1 on FAIL, 2 when the bundle has no usable manifest: anything but a header line,
# exactly the cem, witness, dogfood and gate-receipt lines in that order with the cem present, and
# the closing line. Each present receipt must name its one fixed file.
set -euf

usage() { echo "usage: script/verify-receipt-bundle.sh BUNDLE_DIR" >&2; exit 2; }
[ "$#" -eq 1 ] || usage
bundle=${1%/}
manifest="$bundle/manifest.json"
malformed() { echo "verify-receipt-bundle: $manifest $1" >&2; exit 2; }

if command -v sha256sum >/dev/null 2>&1; then
    sha256_of() { sha256sum < "$1" | awk '{print $1}'; }
else
    sha256_of() { shasum -a 256 < "$1" | awk '{print $1}'; }
fi

if [ -L "$manifest" ] || [ ! -f "$manifest" ]; then
    malformed "is not a regular file"
fi
[ "$(wc -l < "$manifest" | tr -d ' ')" -eq 6 ] && tail -c 1 "$manifest" | od -An -tx1 | grep -q 0a ||
    malformed "is not six LF-terminated lines"
sed -n 1p "$manifest" |
    grep -Eqx '\{"profile":"corvint-receipt-bundle/0","base":"[0-9a-f]{40,64}","target":"[0-9a-f]{40,64}","receipts":\[' ||
    malformed "does not open with a corvint-receipt-bundle/0 header line"
[ "$(sed -n 6p "$manifest")" = ']}' ] || malformed "does not end with the closing line"

pairs=''
n=2
for entry in cem:receipts/cem.json witness:receipts/witness.json dogfood:receipts/dogfood-report.json \
    gate-receipt:receipts/gate-receipt.txt; do
    kind=${entry%%:*}
    path=${entry#*:}
    line=$(sed -n "${n}p" "$manifest")
    end='},'
    [ "$n" -eq 5 ] && end='}'
    case $line in
        *"$end") ;;
        *) malformed "line $n does not end with $end" ;;
    esac
    case $line in
        "{\"kind\":\"$kind\",\"state\":\"present\",\"file\":\"$path\",\"sha256\":\""*) ;;
        "{\"kind\":\"$kind\",\"state\":\"absent\",\"source\":"*)
            [ "$kind" != cem ] || malformed "line 2 lists the required cem receipt as absent"
            n=$((n + 1))
            continue ;;
        *) malformed "line $n is not the $kind receipt naming $path" ;;
    esac
    rest=${line#*\"sha256\":\"}
    expected=${rest%%\"*}
    printf '%s\n' "$expected" | grep -Eqx '[0-9a-f]{64}' || malformed "line $n has no sha256 digest"
    pairs="$pairs$path $expected
"
    n=$((n + 1))
done
printf 'manifest sha256 %s\n' "$(sha256_of "$manifest")"
status=0

for path in $(printf '%s' "$pairs" | awk '{ print $1 }'); do
    expected=$(printf '%s' "$pairs" | awk -v p="$path" '$1 == p { print $2 }')
    file="$bundle/$path"
    if [ -L "$file" ] || [ ! -f "$file" ]; then
        echo "MISSING $path"
        status=1
        continue
    fi
    actual=$(sha256_of "$file")
    if [ "$actual" = "$expected" ]; then
        echo "MATCH $path $actual"
    else
        echo "MISMATCH $path expected=$expected actual=$actual"
        status=1
    fi
done

for file in $(cd "$bundle" && find . ! -type d | sed 's|^\./||' | LC_ALL=C sort); do
    [ "$file" = manifest.json ] && continue
    printf '%s' "$pairs" | awk -v p="$file" '$1 == p { found = 1 } END { exit !found }' && continue
    echo "EXTRA $file"
    status=1
done

if [ "$status" -eq 0 ]; then echo PASS; else echo FAIL; fi
exit "$status"
