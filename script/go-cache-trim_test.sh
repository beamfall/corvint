#!/usr/bin/env bash
# Temp-fixture tests for script/go-cache-trim.sh (V1-0360, ASD-V0-021).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
tool="$here/go-cache-trim.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/go-cache-trim-test.XXXXXX")
holder=
cleanup() {
  [[ -n $holder ]] && kill "$holder" 2> /dev/null || true
  /bin/rm -rf "$tmp"
}
trap cleanup EXIT INT TERM
fail() { printf 'go-cache-trim_test: FAIL %s\n' "$*" >&2; exit 1; }
expect() { grep -qF -- "$1" "$2" || { cat "$2" >&2; fail "missing: $1"; }; }
absent() { ! grep -qF -- "$1" "$2" || { cat "$2" >&2; fail "unexpected: $1"; }; }

old=$(date -v-10d +%Y%m%d%H%M 2> /dev/null || date -d '10 days ago' +%Y%m%d%H%M)
older=$(date -v-20d +%Y%m%d%H%M 2> /dev/null || date -d '20 days ago' +%Y%m%d%H%M)
mib() { dd if=/dev/zero of="$1" bs=1048576 count="$2" 2> /dev/null; }

# A Go cache: README marker, two old 1 MiB entries (one older), one fresh 1 MiB entry.
cache="$tmp/cache"
mkdir -p "$cache/0a" "$cache/1b" "$cache/2c"
echo 'This directory holds cached build artifacts from the Go build system.' > "$cache/README"
mib "$cache/0a/oldest-a" 1 && touch -t "$older" "$cache/0a/oldest-a"
mib "$cache/1b/old-d" 1 && touch -t "$old" "$cache/1b/old-d"
mib "$cache/2c/fresh-a" 1
# A directory without the marker is never touched.
other="$tmp/not-cache"
mkdir -p "$other/0a" && mib "$other/0a/x" 1 && touch -t "$older" "$other/0a/x"

# Work dirs (named with $$ so a host process cannot name them): stale, fresh, stale-but-in-use, and a non-go-build name.
root="$tmp/root"
mkdir -p "$root/go-build111-$$" "$root/go-build222-$$" "$root/go-build333-$$" "$root/other-old"
touch -t "$older" "$root/go-build111-$$" "$root/go-build333-$$" "$root/other-old"
# A work dir whose name holds a newline must never split into another path (here ./src of the cwd).
nlname=$'go-build444\nsrc'
mkdir -p "$root/$nlname" "$tmp/cwd/src" && touch -t "$older" "$root/$nlname"
# A cache entry whose name holds a newline is never listed, so ./victim of the cwd is never removed.
mib "$cache/0a/x"$'\n'"victim" 0 && touch -t "$older" "$cache/0a/x"$'\n'"victim" && touch "$tmp/cwd/victim"
sh -c 'sleep 60; :' "$root/go-build333-$$/b001/exe" &
holder=$!

# 1. Dry run (default) with a 1 MiB bound: plans the two old entries, deletes nothing.
"$tool" --cache "$cache" --cache "$other" --temp-root "$root" --max-gib 0 > "$tmp/dry" 2>&1 || fail "dry run exited $?"
real_cache=$(cd "$cache" && pwd -P)
real_root=$(cd "$root" && pwd -P)
expect "WOULD-TRIM cache $real_cache entries=2 bytes=2097152 kept-bytes=1048576" "$tmp/dry"
expect "KEEP not-a-go-cache cache $(cd "$other" && pwd -P)" "$tmp/dry"
expect "WOULD-REMOVE workdir $real_root/go-build111-$$" "$tmp/dry"
expect "KEEP in-use workdir $real_root/go-build333-$$" "$tmp/dry"
absent "go-build222-$$" "$tmp/dry"
absent "other-old" "$tmp/dry"
expect "SUMMARY caches=1 trim-bytes=2097152 workdirs=1 applied=no" "$tmp/dry"
[[ -f $cache/0a/oldest-a && -f $cache/1b/old-d && -d $root/go-build111-$$ ]] || fail "dry run deleted something"

# 2. A bound the cache already meets keeps everything.
"$tool" --cache "$cache" --temp-root "$tmp/none" --max-gib 1 > "$tmp/within" 2>&1
expect "KEEP within-bound cache $real_cache bytes=3145728" "$tmp/within"

# 3. --min-age-hours protects recent entries: a 30-day minimum leaves nothing to trim.
"$tool" --cache "$cache" --temp-root "$tmp/none" --max-gib 0 --min-age-hours 720 > "$tmp/young" 2>&1
expect "KEEP within-bound cache $real_cache bytes=3145728" "$tmp/young"

# 4. --apply trims oldest-first old entries only, removes only the stale unused work dir.
(cd "$tmp/cwd" && "$tool" --cache "$cache" --cache "$other" --temp-root "$root" --max-gib 0 --apply) > "$tmp/apply" 2>&1
expect "TRIMMED cache $real_cache entries=2 bytes=2097152 kept-bytes=1048576" "$tmp/apply"
expect "REMOVED workdir $real_root/go-build111-$$" "$tmp/apply"
expect "applied=yes" "$tmp/apply"
[[ ! -e $cache/0a/oldest-a && ! -e $cache/1b/old-d ]] || fail "old entries survived --apply"
[[ -f $cache/2c/fresh-a && -f $cache/README && -f $other/0a/x ]] || fail "--apply removed a protected file"
[[ -d $tmp/cwd/src && -f $tmp/cwd/victim && -d $root/$nlname ]] || fail "a newline-named path removed another path"
expect "KEEP newline-in-name workdir" "$tmp/apply"
[[ ! -e $root/go-build111-$$ && -d $root/go-build222-$$ && -d $root/go-build333-$$ && -d $root/other-old ]] || fail "--apply work dir set wrong"

# 5. ASD-V0-038: a free share of /tmp below --min-free-pct warns; 0 never warns.
"$tool" --cache "$cache" --temp-root "$tmp/none" --min-free-pct 100 > "$tmp/lowdisk" 2>&1
expect "WARN low-disk free-pct=" "$tmp/lowdisk"
expect " min-free-pct=100 path=/tmp" "$tmp/lowdisk"
"$tool" --cache "$cache" --temp-root "$tmp/none" --min-free-pct 0 > "$tmp/nowarn" 2>&1
absent "WARN low-disk" "$tmp/nowarn"
# The post-batch default keeps entries younger than 12 hours: a 6-hour-old entry stays.
mkdir -p "$cache/3d" && mib "$cache/3d/six-hours" 1 &&
  touch -t "$(date -v-6H +%Y%m%d%H%M 2> /dev/null || date -d '6 hours ago' +%Y%m%d%H%M)" "$cache/3d/six-hours"
"$tool" --cache "$cache" --temp-root "$tmp/none" --max-gib 0 --apply > "$tmp/twelve" 2>&1
[[ -f $cache/3d/six-hours ]] || fail "the default trim removed an entry younger than 12 hours"

# 6. Invalid arguments refuse with exit 2.
set +e
"$tool" --max-gib x > /dev/null 2>&1; rc1=$?
"$tool" --bogus > /dev/null 2>&1; rc2=$?
"$tool" --min-free-pct x > /dev/null 2>&1; rc3=$?
set -e
((rc1 == 2 && rc2 == 2 && rc3 == 2)) || fail "refusal exit codes $rc1 $rc2 $rc3"

# 7. ASD-V0-900: without --max-gib the bound is at most 40% of the cache's file system. A fake df
# reports a 5 MiB file system, so three old 1 MiB entries trim to 2 MiB; an explicit --max-gib
# keeps its own bound.
small="$tmp/small-cache"
mkdir -p "$small/0a" "$tmp/fakebin"
echo 'This directory holds cached build artifacts from the Go build system.' > "$small/README"
for e in a b c; do mib "$small/0a/old-$e" 1 && touch -t "$old" "$small/0a/old-$e"; done
touch -t "$older" "$small/0a/old-a"
printf '#!/bin/sh\nprintf "Filesystem 1024-blocks Used Available Capacity Mounted\\nfake 5120 4096 1024 80%%%% /\\n"\n' > "$tmp/fakebin/df"
chmod +x "$tmp/fakebin/df"
real_small=$(cd "$small" && pwd -P)
PATH="$tmp/fakebin:$PATH" "$tool" --cache "$small" --temp-root "$tmp/none" > "$tmp/fs-bound" 2>&1
expect "WOULD-TRIM cache $real_small entries=1 bytes=1048576 kept-bytes=2097152" "$tmp/fs-bound"
PATH="$tmp/fakebin:$PATH" "$tool" --cache "$small" --temp-root "$tmp/none" --max-gib 1 > "$tmp/fs-explicit" 2>&1
expect "KEEP within-bound cache $real_small bytes=3145728" "$tmp/fs-explicit"

# 8. ASD-V0-063 (V1-2904, V1-3824): without --temp-root the work-directory roots include
# ${XDG_CACHE_HOME:-$HOME/.cache}/corvint-tmp, where script/heavy.sh puts a tmpfs host's TMPDIR.
mkdir -p "$tmp/xdg/corvint-tmp/go-build555-$$" && touch -t "$older" "$tmp/xdg/corvint-tmp/go-build555-$$"
XDG_CACHE_HOME="$tmp/xdg" "$tool" --cache "$tmp/none" > "$tmp/xdg-root" 2>&1
expect "WOULD-REMOVE workdir $(cd "$tmp/xdg/corvint-tmp" && pwd -P)/go-build555-$$" "$tmp/xdg-root"
[[ -d "$tmp/xdg/corvint-tmp/go-build555-$$" ]] || fail "dry run removed the corvint-tmp work dir"

echo "go-cache-trim_test: ok"
