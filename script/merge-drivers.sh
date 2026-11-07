#!/usr/bin/env bash
# usage: script/merge-drivers.sh [--install]
#
# ASD-V0-571 (V1-4167): a custom merge driver named in .gitattributes (`merge=NAME`, other than
# Git's built-in text, binary and union) runs only in a checkout whose Git config defines
# `merge.NAME.driver`; an unset driver silently falls back to the ordinary text merge. This check
# lists each such NAME and exits 1 when one is unset, naming the install command. The repository's
# convention is that driver NAME is `script/merge-NAME.sh %O %A %B %L`. --install sets each unset
# driver whose script exists in this checkout's local config (it never overwrites a set driver) and
# then checks again; script/lane-setup.sh runs it for every new lane. Exit 2 outside a checkout.
set -uo pipefail
export LC_ALL=C
me=merge-drivers
install=0
case ${1-} in
  '') ;;
  --install) install=1 ;;
  *) printf 'usage: script/merge-drivers.sh [--install]\n' >&2; exit 2 ;;
esac
top=$(git rev-parse --show-toplevel 2> /dev/null) || { printf '%s: REFUSE not inside a Git checkout\n' "$me" >&2; exit 2; }
names=$(cat "$top/.gitattributes" 2> /dev/null | awk '
  /^[[:space:]]*#/ { next }
  { for (i = 2; i <= NF; i++) if ($i ~ /^merge=/) { n = substr($i, 7); if (n != "text" && n != "binary" && n != "union") print n } }' | sort -u)
missing=0
for name in $names; do
  driver="script/merge-$name.sh %O %A %B %L"
  if [[ -n $(git -C "$top" config --get "merge.$name.driver") ]]; then
    printf '%s: ok merge=%s\n' "$me" "$name"
    continue
  fi
  if ((install)) && [[ -x $top/script/merge-$name.sh ]]; then
    git -C "$top" config "merge.$name.driver" "$driver" || { printf '%s: could not set merge.%s.driver\n' "$me" "$name" >&2; exit 1; }
    printf '%s: installed merge=%s (%s)\n' "$me" "$name" "$driver"
    continue
  fi
  if [[ -x $top/script/merge-$name.sh ]]; then
    printf '%s: MISSING merge=%s: .gitattributes names it but merge.%s.driver is unset, so Git falls back to a text merge; fix: script/merge-drivers.sh --install (or git config merge.%s.driver '\''%s'\'')\n' "$me" "$name" "$name" "$name" "$driver" >&2
  else
    printf '%s: MISSING merge=%s: .gitattributes names it but neither merge.%s.driver nor script/merge-%s.sh exists; fix: add the driver script or remove the attribute\n' "$me" "$name" "$name" "$name" >&2
  fi
  missing=1
done
exit "$missing"
