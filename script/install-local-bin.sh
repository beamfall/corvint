#!/usr/bin/env bash
# usage: script/install-local-bin.sh [--keep N] SOURCE DEST
#        script/install-local-bin.sh [--keep N] --rollback DEST
#
# Installs a locally built executable over DEST with a bounded rollback set (ASD-V0-025, V1-1979).
# SOURCE is copied to a sibling temporary file in DEST's directory and renamed over DEST, so a
# running DEST is never overwritten in place. The replaced DEST is kept as DEST.corvint-prev-<n>
# (n increasing); afterwards only the newest N such backups are kept (default 1) and older ones
# are removed. --rollback renames the newest backup over DEST and keeps the replaced DEST as the
# next backup, so a second --rollback returns to it. Only files named exactly
# DEST.corvint-prev-<digits> are ever pruned; other backups (for example DEST.bak-*) are untouched.
# Prints INSTALLED/ROLLED-BACK, KEPT and REMOVED lines; refuses (exit 2) bad arguments, a missing
# source or backup, and a DEST that is not a regular file.
set -uo pipefail
export LC_ALL=C

keep=1
rollback=0
args=()
while (($# > 0)); do
  case $1 in
    --keep)
      keep=${2-}
      shift 2 || { keep=''; shift; }
      ;;
    --rollback) rollback=1; shift ;;
    -h | --help) sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    --) shift; args+=("$@"); break ;;
    -*) printf 'install-local-bin: REFUSE unknown-option %s\n' "$1" >&2; exit 2 ;;
    *) args+=("$1"); shift ;;
  esac
done
refuse() { printf 'install-local-bin: REFUSE %s\n' "$*" >&2; exit 2; }
[[ $keep =~ ^[1-9][0-9]*$ ]] || refuse "invalid-keep ${keep:-<empty>} (want 1 or more)"
if ((rollback)); then
  ((${#args[@]} == 1)) || refuse "usage: --rollback DEST"
  dest=${args[0]}
else
  ((${#args[@]} == 2)) || refuse "usage: SOURCE DEST"
  source=${args[0]}
  dest=${args[1]}
  [[ -f $source && -r $source ]] || refuse "source-unreadable $source"
fi
[[ ! -e $dest || (-f $dest && ! -L $dest) ]] || refuse "dest-not-regular-file $dest"
dir=$(dirname -- "$dest")
[[ -d $dir && -w $dir ]] || refuse "dest-dir-unwritable $dir"
prefix="$dest.corvint-prev-"

# backups prints the retained backup sequence numbers, ascending.
backups() {
  local path n
  for path in "$prefix"*; do
    n=${path#"$prefix"}
    [[ $n =~ ^[0-9]+$ && -f $path && ! -L $path ]] && printf '%s\n' "$((10#$n))"
  done | sort -n
}
newest=$(backups | tail -1)
next=$((${newest:-0} + 1))

if ((rollback)); then
  [[ -n $newest ]] || refuse "no-backup ${prefix}<n>"
  if [[ -e $dest ]]; then
    /bin/mv -f -- "$dest" "$prefix$next" || refuse "rename-failed $dest"
  fi
  /bin/mv -f -- "$prefix$newest" "$dest" || refuse "rename-failed $prefix$newest"
  printf 'ROLLED-BACK %s from %s\n' "$dest" "$prefix$newest"
else
  tmp=$(mktemp "$dest.install.XXXXXX") || refuse "temp-file-failed $dir"
  if ! /bin/cp -f -- "$source" "$tmp" || ! chmod 0755 "$tmp"; then
    /bin/rm -f -- "$tmp"
    refuse "copy-failed $source"
  fi
  if [[ -e $dest ]]; then
    # A hard link keeps DEST in place until the rename below replaces it.
    if ! ln -- "$dest" "$prefix$next" 2> /dev/null && ! /bin/cp -p -- "$dest" "$prefix$next"; then
      /bin/rm -f -- "$tmp"
      refuse "backup-failed $dest"
    fi
  fi
  /bin/mv -f -- "$tmp" "$dest" || { /bin/rm -f -- "$tmp"; refuse "rename-failed $dest"; }
  printf 'INSTALLED %s\n' "$dest"
fi

all=()
while IFS= read -r n; do all+=("$n"); done < <(backups)
drop=$((${#all[@]} - keep))
for i in "${!all[@]}"; do
  path=$prefix${all[$i]}
  if ((i < drop)); then
    /bin/rm -f -- "$path" && printf 'REMOVED %s\n' "$path"
  else
    printf 'KEPT %s\n' "$path"
  fi
done
exit 0
