#!/usr/bin/env bash
# usage: script/go-cache-trim.sh [--apply] [--max-gib N] [--min-age-hours H] [--work-age-hours W]
#                                [--min-free-pct P] [--cache DIR]... [--temp-root DIR]...
#
# Bounds the Go build caches Corvint uses and removes stale `go-build*` work directories
# (V1-0360, ASD-V0-021, docs/LANE-ORCHESTRATION.md section 6). Dry-run is the default; only
# --apply deletes.
#
# Caches: every --cache DIR, or by default `go env GOCACHE`, ${XDG_CACHE_HOME:-$HOME/.cache}/
# corvint-go-build-cache and corvint-work-queue-gocache-<uid>, and the legacy /tmp copies of both. A directory is trimmed only when it holds Go's cache
# README marker; anything else is KEEP not-a-go-cache and is never touched. Within a cache only
# the entry files in its two-hex-digit subdirectories are candidates, oldest modification time
# first, and only entries older than --min-age-hours (default 12, the post-batch rule of ASD-V0-038;
# Go refreshes an entry's time when a build uses it and it is over an hour old), until the cache's entry bytes are at most
# --max-gib (default 20) GiB. Without --max-gib, a cache's bound is also at most 40% of the size of
# the file system holding it (ASD-V0-900): a 20 GiB bound over a 16 GiB /tmp tmpfs bounded nothing
# and the shared cache filled the disk every lane builds on. Go treats a removed entry as a cache
# miss and rebuilds it.
#
# Work directories: each `go-build*` directory directly under every --temp-root (default
# $TMPDIR, /tmp and ${XDG_CACHE_HOME:-$HOME/.cache}/corvint-tmp, the TMPDIR the heavy runner gives a
# local run on a tmpfs /tmp, ASD-V0-063) last modified more than --work-age-hours (default 24) ago, unless a running
# process names it in its arguments (KEEP in-use) or the process list cannot be read (KEEP
# ps-unavailable).
#
# Output lines:
#   KEEP <reason> cache|workdir <path>
#   WOULD-TRIM|TRIMMED cache <dir> entries=N bytes=B kept-bytes=B
#   WOULD-REMOVE|REMOVED|REMOVE-FAILED workdir <path>
#   SUMMARY caches=N trim-bytes=B workdirs=N applied=<yes|no> free-before=<KiB> free-after=<KiB|->
#   WARN low-disk free-pct=N min-free-pct=P path=/tmp
#       (ASD-V0-038) when the free share of the /tmp file system, after an --apply or else before,
#       is below --min-free-pct (default 20; 0 never warns): trimming alone did not restore the
#       host's free-disk floor, so other scratch (lane checkouts, scratch-sweep) needs clearing.
set -uo pipefail
export LC_ALL=C

apply=0
max_gib=20
max_explicit=0
min_age_hours=12
min_free_pct=20
work_age_hours=24
caches=()
temp_roots=()
refuse() { printf 'go-cache-trim: REFUSE %s\n' "$*" >&2; exit 2; }
number() { [[ ${2-} =~ ^[0-9]+$ ]] || refuse "invalid-$1 ${2-}"; }
while [[ $# -gt 0 ]]; do
  case $1 in
    --apply) apply=1; shift ;;
    --max-gib) number max-gib "${2-}"; max_gib=$2; max_explicit=1; shift 2 ;;
    --min-age-hours) number min-age-hours "${2-}"; min_age_hours=$2; shift 2 ;;
    --work-age-hours) number work-age-hours "${2-}"; work_age_hours=$2; shift 2 ;;
    --min-free-pct) number min-free-pct "${2-}"; min_free_pct=$2; shift 2 ;;
    --cache) [[ -n ${2-} ]] || refuse "cache-required"; caches+=("$2"); shift 2 ;;
    --temp-root) [[ -n ${2-} ]] || refuse "temp-root-required"; temp_roots+=("$2"); shift 2 ;;
    -h | --help) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) refuse "unknown-argument $1" ;;
  esac
done
((work_age_hours > 0)) || refuse "invalid-work-age-hours 0"
if ((${#caches[@]} == 0)); then
  default_cache=$(GOTOOLCHAIN=local go env GOCACHE 2> /dev/null) || default_cache=
  [[ -n $default_cache && $default_cache != off ]] && caches+=("$default_cache")
  user_cache_root=${XDG_CACHE_HOME:-$HOME/.cache}
  caches+=("$user_cache_root/corvint-go-build-cache" "$user_cache_root/corvint-work-queue-gocache-$(id -u)"
    /tmp/corvint-go-build-cache "/tmp/corvint-work-queue-gocache-$(id -u)")
  # ASD-V0-914 (V1-3796): the Makefile and heavy.sh default cache follows TMPDIR.
  tmp_cache=${TMPDIR:-/tmp}
  tmp_cache=${tmp_cache%/}/corvint-go-build-cache
  [[ $tmp_cache == /tmp/corvint-go-build-cache ]] || caches+=("$tmp_cache")
fi
if ((${#temp_roots[@]} == 0)); then
  temp_roots=("${TMPDIR:-/tmp}" /tmp "${XDG_CACHE_HOME:-$HOME/.cache}/corvint-tmp")
fi

free_kib() { df -Pk "$1" 2> /dev/null | awk 'NR == 2 { print $4 }'; }
# free_pct DIR: the available share of DIR's file system, whole percent (available over used plus
# available, as df's capacity column counts it); empty when df cannot say.
free_pct() { df -Pk "$1" 2> /dev/null | awk 'NR == 2 && $3 + $4 > 0 { printf "%d\n", 100 * $4 / ($3 + $4) }'; }
# fs_bytes DIR: the size of DIR's file system in bytes (used plus available); empty when df cannot say.
fs_bytes() { df -Pk "$1" 2> /dev/null | awk 'NR == 2 && $3 + $4 > 0 { printf "%.0f\n", ($3 + $4) * 1024 }'; }
free_before=$(free_kib /tmp)
max_bytes=$((max_gib * 1024 * 1024 * 1024))
now=$(date +%s)
cutoff=$((now - min_age_hours * 3600))
if stat -f '%m' / > /dev/null 2>&1; then stat_format=(-f '%m %z %N'); else stat_format=(-c '%Y %s %n'); fi

nl=$'\n'
ncaches=0
trim_total=0
seen=" "
for cache in "${caches[@]}"; do
  [[ -d $cache ]] || continue
  real=$(cd "$cache" 2> /dev/null && pwd -P) || { printf 'KEEP unreadable cache %s\n' "$cache"; continue; }
  [[ $seen == *" $real "* ]] && continue
  seen+="$real "
  if ! grep -qs 'This directory holds cached build artifacts from the Go build system' "$real/README"; then
    printf 'KEEP not-a-go-cache cache %s\n' "$real"
    continue
  fi
  ncaches=$((ncaches + 1))
  listing=$(mktemp "${TMPDIR:-/tmp}/corvint-go-cache-trim.XXXXXX") || refuse "temporary-file-unavailable"
  # Entries: oldest first, as "mtime size path".
  # Names holding a newline are never listed, so each "mtime size path" line is one whole file.
  find "$real" -mindepth 2 -maxdepth 2 -type f -path "$real/[0-9a-f][0-9a-f]/*" ! -name "*$nl*" -exec stat "${stat_format[@]}" {} + 2> /dev/null |
    sort -n -k1,1 > "$listing"
  bound=$max_bytes
  if ((!max_explicit)); then
    size=$(fs_bytes "$real")
    if [[ -n $size ]] && ((size * 40 / 100 < bound)); then bound=$((size * 40 / 100)); fi
  fi
  plan=$(awk -v max="$bound" -v cutoff="$cutoff" '
    { mtime[NR] = $1; size[NR] = $2; total += $2 }
    END {
      kept = total
      for (i = 1; i <= NR && kept > max; i++) {
        if (mtime[i] >= cutoff) break
        kept -= size[i]; n++; bytes += size[i]
      }
      printf "%d %.0f %.0f\n", n, bytes, kept
    }' "$listing")
  read -r entries bytes kept <<< "$plan"
  if ((entries == 0)); then
    printf 'KEEP within-bound cache %s bytes=%s\n' "$real" "$kept"
  elif ((apply)); then
    head -n "$entries" "$listing" | cut -d' ' -f3- | tr '\n' '\0' | xargs -0 /bin/rm -f
    printf 'TRIMMED cache %s entries=%s bytes=%s kept-bytes=%s\n' "$real" "$entries" "$bytes" "$kept"
  else
    printf 'WOULD-TRIM cache %s entries=%s bytes=%s kept-bytes=%s\n' "$real" "$entries" "$bytes" "$kept"
  fi
  trim_total=$((trim_total + bytes))
  /bin/rm -f "$listing"
done

nwork=0
snapshot() { processes=$(ps -axww -o args= 2> /dev/null) && processes+=$'\n' || processes=; }
# in_use NAME: 0 when a running process names the work directory, or the process list is unreadable.
in_use() {
  [[ -z $processes || $processes == *"/$1/"* || $processes == *"/$1 "* || $processes == *"/$1"$'\n'* ]]
}
snapshot
seen=" "
for temp_root in "${temp_roots[@]}"; do
  [[ -d $temp_root ]] || continue
  root_real=$(cd "$temp_root" 2> /dev/null && pwd -P) || continue
  [[ $root_real != / ]] || continue
  while IFS= read -r -d '' dir; do
    [[ -n $dir ]] || continue
    if [[ $dir == *"$nl"* ]]; then
      printf 'KEEP newline-in-name workdir %q\n' "$dir"
      continue
    fi
    [[ $seen == *" $dir "* ]] && continue
    seen+="$dir "
    name=${dir##*/}
    if [[ -z $processes ]]; then
      printf 'KEEP ps-unavailable workdir %s\n' "$dir"
      continue
    fi
    if in_use "$name"; then
      printf 'KEEP in-use workdir %s\n' "$dir"
      continue
    fi
    if ((apply)); then
      # Re-read the process list just before removal; a process starting after this check is not seen.
      snapshot
      if in_use "$name"; then
        printf 'KEEP in-use workdir %s\n' "$dir"
        continue
      fi
      nwork=$((nwork + 1))
      if /bin/rm -rf "$dir"; then printf 'REMOVED workdir %s\n' "$dir"; else printf 'REMOVE-FAILED workdir %s\n' "$dir"; fi
    else
      nwork=$((nwork + 1))
      printf 'WOULD-REMOVE workdir %s\n' "$dir"
    fi
  done < <(find "$root_real" -mindepth 1 -maxdepth 1 -type d -name 'go-build*' -mmin "+$((work_age_hours * 60))" -print0 2> /dev/null)
done

free_after=-
((apply)) && free_after=$(free_kib /tmp)
printf 'SUMMARY caches=%d trim-bytes=%s workdirs=%d applied=%s free-before=%s free-after=%s\n' \
  "$ncaches" "$trim_total" "$nwork" "$( ((apply)) && echo yes || echo no)" "${free_before:--}" "${free_after:--}"
pct=$(free_pct /tmp)
if [[ -n $pct ]] && ((pct < min_free_pct)); then
  printf 'WARN low-disk free-pct=%s min-free-pct=%s path=/tmp\n' "$pct" "$min_free_pct"
fi
