#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$tmp/bin"
cat > "$tmp/bin/git" <<'SH'
#!/bin/sh
printf 'spawned\n' >> "$RUNTIME_ENV_MARKER"
exit 91
SH
chmod +x "$tmp/bin/git"
export RUNTIME_ENV_MARKER="$tmp/marker"
for helper in dogfood-change dogfood-check dogfood-bind-range; do
  for setting in current empty; do
    unset CORVINT_BIN
    case "$setting" in
      current) export CORVINT_BIN=tool;;
      empty) export CORVINT_BIN=;;
    esac
    PATH="$tmp/bin:$PATH" bash "$root/script/$helper.sh" baseline target > "$tmp/out" 2>&1 && exit 1
    test -e "$tmp/marker"
    rm "$tmp/marker"
  done
done
unset CORVINT_BIN
bash "$root/script/dogfood-change_test.sh"
bash "$root/script/dogfood-bind-range_test.sh"
sh "$root/script/cem-verify-pr_test.sh"
echo 'runtime environment helpers: ok'
