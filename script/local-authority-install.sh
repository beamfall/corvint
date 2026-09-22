#!/bin/sh
# No sudo invocation, daemon, account creation or admission is implicit.
set -eu
if [ "$#" -ne 4 ]; then
  echo 'usage: local-authority-install.sh REVIEWED_BINARY BUNDLE MANIFEST_SHA256 BINARY_SHA256' >&2
  exit 2
fi
# The initial operator bootstrap must copy this script and the reviewed binary
# into a root-owned path and verify that protected copy before execution.
protected() {
  checked=$1
  while :; do
    [ ! -L "$checked" ] || return 1
    [ "$(/usr/bin/stat -f %u "$checked")" = 0 ] || return 1
    permissions=$(/usr/bin/stat -f %Lp "$checked")
    case "$permissions" in 755|555|444|700|644|500|400) ;; *) return 1 ;; esac
    [ "$(/bin/ls -lde "$checked" | /usr/bin/awk 'END {print NR}')" = 1 ] || return 1
    [ "$checked" = / ] && break
    checked=$(/usr/bin/dirname "$checked")
  done
}
case "$0" in /Library/CorvintAuthority/bootstrap/*|/Library/CorvintAuthority/versions/*) ;; *) echo 'script must be independently bootstrapped into protected storage' >&2; exit 1 ;; esac
protected "$0" || { echo 'unprotected script' >&2; exit 1; }
runner=$1
bundle=$2
manifest=$3
binary_hash=$4
case "$runner" in /Library/CorvintAuthority/bootstrap/*/local-authority|/Library/CorvintAuthority/versions/*/local-authority) ;; *) echo 'runner is outside protected storage' >&2; exit 1 ;; esac
protected "$runner" || { echo 'unprotected runner' >&2; exit 1; }
actual=$(/usr/bin/shasum -a 256 "$runner" | /usr/bin/awk '{print $1}')
[ "$actual" = "$binary_hash" ] || { echo 'reviewed binary mismatch' >&2; exit 1; }
exec "$runner" install-release "$bundle" "$manifest"
