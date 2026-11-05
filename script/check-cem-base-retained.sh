#!/usr/bin/env bash
# usage: check-cem-base-retained.sh [--retained REF]... — refuse a committed map whose base no retained
# ref reaches (decision 0588, V1-3758).
#
# Historical labels preserve the original archive provenance. Archive references are not
# local-only tags: when their commit is unavailable, report NOT_RUN with the reason.
# Exact base/map binding is still mandatory; unlabelled unretained maps still refuse.
set -uo pipefail
export LC_ALL=C
# Counters, not ${#array[@]} or "${array[@]}" on an empty array: Bash before 4.4 treats an empty
# array as unset under `set -u`.
retained=()
retained_count=0
while (($#)); do
  case $1 in
    --retained) [[ $# -ge 2 && -n $2 ]] || { echo "check-cem-base-retained: --retained needs a ref" >&2; exit 2; }; retained+=("$2"); retained_count=$((retained_count + 1)); shift 2 ;;
    *) echo "check-cem-base-retained: unknown argument: $1" >&2; exit 2 ;;
  esac
done
((retained_count)) || retained=(refs/remotes/origin/main)
top=$(git rev-parse --show-toplevel 2> /dev/null) || { echo "check-cem-base-retained: not inside a Git work tree" >&2; exit 2; }
cd "$top" || exit 2
register=.corvint/cem-lineage.tsv

tips=()
tip_count=0
for ref in ${retained[@]+"${retained[@]}"}; do
  if tip=$(git rev-parse --verify -q "$ref^{commit}"); then tips+=("$tip"); tip_count=$((tip_count + 1)); fi
done
if ((tip_count == 0)); then
  echo "check-cem-base-retained: no default retained ref resolves here (${retained[*]-}); reachability NOT_RUN without a retained tip" >&2
fi

# Bash 3.2 (macOS /bin/bash) has no associative arrays: the register and the tracked targets are
# newline lists, "TARGET BASE" and "TARGET", looked up with grep -Fx.
labelled=
labelled_count=0
if [[ -f $register ]]; then
  while IFS=$'\t' read -r target base label ref; do
    [[ -z $target || $target == \#* ]] && continue
    case $label in
      archive-lineage | rebased-lane-base | base-unavailable) ;;
      *) echo "check-cem-base-retained: $register: $target has unknown label '$label'" >&2; exit 1 ;;
    esac
    [[ -n ${ref:-} ]] || { echo "check-cem-base-retained: $register: $target names no retained column" >&2; exit 1; }
    labelled+="$target $base $label $ref"$'\n'
    labelled_count=$((labelled_count + 1))
  done < "$register"
fi

maps=$(git ls-files -- '.corvint/changes/*.cem.json') || { echo "check-cem-base-retained: git ls-files failed" >&2; exit 2; }
refused=0
present=
while IFS= read -r map; do
  [[ -n $map ]] || continue
  target=${map#.corvint/changes/}
  target=${target%.cem.json}
  present+="$target"$'\n'
  base=$(grep -o '"baseRevision": *"[0-9a-f]*"' "$map" | head -1 | grep -o '[0-9a-f]\{40,64\}')
  if [[ -z $base ]]; then
    echo "check-cem-base-retained: $map names no baseRevision" >&2
    refused=1
    continue
  fi
  row=$(printf '%s' "$labelled" | grep -m1 "^$target " || true)
  if [[ -n $row ]]; then
    read -r labelled_target labelled_base labelled_label labelled_ref <<< "$row"
    if [[ $labelled_base != "$base" ]]; then
      echo "check-cem-base-retained: $register labels $target with base $labelled_base, but the map's base is $base" >&2
      refused=1
    fi
    # CEM-V1-8201: an archive locator survives a fresh clone without private tags.
    case $labelled_ref in
      beamfall/corvint-history@*)
        archive_commit=${labelled_ref#*@}
        if [[ $labelled_label != archive-lineage || ! $archive_commit =~ ^[0-9a-f]{40}$ ]]; then
          echo "check-cem-base-retained: $register: $target has invalid archive reference $labelled_ref" >&2
          refused=1
        elif ! git cat-file -e "$archive_commit^{commit}" 2> /dev/null || ! git cat-file -e "$base^{commit}" 2> /dev/null; then
          echo "check-cem-base-retained: NOT_RUN $target archive reachability: $labelled_ref or base $base absent locally" >&2
        elif ! git merge-base --is-ancestor "$base" "$archive_commit" 2> /dev/null; then
          echo "check-cem-base-retained: $register: $target base $base is not retained by $labelled_ref" >&2
          refused=1
        fi ;;
      none)
        echo "check-cem-base-retained: NOT_RUN $target retention: historical $labelled_label base $base has no retained locator" >&2 ;;
      refs/*)
        if ! labelled_tip=$(git rev-parse --verify -q "$labelled_ref^{commit}"); then
          echo "check-cem-base-retained: NOT_RUN $target retention: $labelled_ref absent locally" >&2
        elif ! git cat-file -e "$base^{commit}" 2> /dev/null; then
          echo "check-cem-base-retained: NOT_RUN $target reachability: base $base absent locally" >&2
        elif ! git merge-base --is-ancestor "$base" "$labelled_tip" 2> /dev/null; then
          echo "check-cem-base-retained: $register: $target base $base is not retained by $labelled_ref" >&2
          refused=1
        fi ;;
      *)
        echo "check-cem-base-retained: $register: $target has invalid retained reference $labelled_ref" >&2
        refused=1 ;;
    esac
    continue
  fi
  if ((tip_count == 0)); then
    echo "check-cem-base-retained: NOT_RUN $map reachability: no retained ref resolves locally" >&2
    continue
  fi
  reached=0
  if git cat-file -e "$base^{commit}" 2> /dev/null; then
    for tip in ${tips[@]+"${tips[@]}"}; do
      if git merge-base --is-ancestor "$base" "$tip" 2> /dev/null; then reached=1; break; fi
    done
  fi
  if ((!reached)); then
    echo "check-cem-base-retained: $map: base $base is not reachable from a retained ref (${retained[*]-})" >&2
    echo "  fix: rebind the change on a base that ${retained[0]} reaches (make dogfood-change BASE=\$(git rev-parse ${retained[0]})), or label the map in $register (decision 0588)" >&2
    refused=1
  fi
done <<< "$maps"
while IFS=' ' read -r target base label ref; do
  [[ -n $target ]] || continue
  if ! printf '%s' "$present" | grep -Fxq "$target"; then
    echo "check-cem-base-retained: $register labels $target, but .corvint/changes/$target.cem.json is not tracked" >&2
    refused=1
  fi
done <<< "$labelled"
((refused)) && exit 1
echo "check-cem-base-retained: every committed map's base is retained or labelled (NOT_RUN checks reported above) ($(wc -l <<< "$maps" | tr -d ' ') maps, $labelled_count labelled)"
