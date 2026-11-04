#!/usr/bin/env bash
# usage: check-cem-base-retained.sh [--retained REF]... — refuse a committed map whose base no retained
# ref reaches (decision 0588, V1-3758).
#
# Every tracked archived map or unsealed sidecar names its baseRevision. Read the Git index so
# unstaged bytes cannot hide committed evidence. After branch deletion and gc a
# base that no retained ref reaches is gone, and the map's Git evidence becomes unverifiable. The check
# passes a map whose base is an ancestor of a retained ref (default refs/remotes/origin/main) or a real
# ref named by its exact `.corvint/cem-lineage.tsv` row. Historical target/base labels pinned at a retained tip
# remain disclosures with retention NOT_PROVED; a new label cannot bypass reachability (CEM-V1-7901).
# It refuses any other map, a register row whose
# map is gone or whose base differs, and an unknown label. With no retained ref resolvable here (a shallow
# or remote-less clone) it says it was skipped and passes: the retention of a base cannot be judged
# without the ref, and the skip is printed rather than read as a pass.
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
  echo "check-cem-base-retained: skipped: no retained ref resolves here (${retained[*]-}); base retention is NOT_RUN"
  exit 0
fi

# Bash 3.2 (macOS /bin/bash) has no associative arrays: the register and the tracked targets are
# newline lists, "TARGET BASE" and "TARGET", looked up with grep -Fx.
labelled=
labelled_refs=
labelled_count=0
if git cat-file -e ":$register" 2>/dev/null; then
  while IFS=$'\t' read -r target base label ref; do
    [[ -z $target || $target == \#* ]] && continue
    case $label in
      archive-lineage | rebased-lane-base | base-unavailable) ;;
      *) echo "check-cem-base-retained: $register: $target has unknown label '$label'" >&2; exit 1 ;;
    esac
    [[ -n ${ref:-} ]] || { echo "check-cem-base-retained: $register: $target names no retained column" >&2; exit 1; }
    labelled+="$target $base"$'\n'
    labelled_refs+="$target $ref"$'\n'
    labelled_count=$((labelled_count + 1))
  done < <(git show ":$register")
fi

maps=$(git ls-files -- '.corvint/changes/*.cem.json' '.corvint/change.cem.json') || { echo "check-cem-base-retained: git ls-files failed" >&2; exit 2; }
refused=0
present=
while IFS= read -r map; do
  [[ -n $map ]] || continue
  target=${map#.corvint/changes/}
  target=${target%.cem.json}
  present+="$target"$'\n'
  base=$(git show ":$map" | grep -o '"baseRevision": *"[0-9a-f]*"'  | head -1 | grep -o '[0-9a-f]\{40,64\}')
  if [[ -z $base ]]; then
    echo "check-cem-base-retained: $map names no baseRevision" >&2
    refused=1
    continue
  fi
  historic=0
  row=$(printf '%s' "$labelled" | grep -m1 "^$target " || true)
  if [[ -n $row ]]; then
    if [[ ${row#* } != "$base" ]]; then
      echo "check-cem-base-retained: $register labels $target with base ${row#* }, but the map's base is $base" >&2
      refused=1
      continue
    fi
    # CEM-V1-7901: only a target/base already labelled at a retained commit may
    # disclose legacy unretained evidence, even after its locator is repaired.
    # A new map/base pair needs a real ref.
    for tip in ${tips[@]+"${tips[@]}"}; do
      if git show "$tip:$register" 2>/dev/null | cut -f1,2 | grep -Fxq -- "$(printf '%s\t%s' "$target" "$base")"; then historic=1; break; fi
    done
  fi
  reached=0
  if git cat-file -e "$base^{commit}" 2> /dev/null; then
    for tip in ${tips[@]+"${tips[@]}"}; do
      if git merge-base --is-ancestor "$base" "$tip" 2> /dev/null; then reached=1; break; fi
    done
  fi
  if ((!reached)) && [[ -n $row ]]; then
    labelled_ref=$(printf '%s' "$labelled_refs" | grep -m1 "^$target " || true)
    labelled_ref=${labelled_ref#* }
    # A raw object ID is not a retained reference: it does not protect from gc.
    if [[ $labelled_ref == refs/* ]] && git show-ref --verify --quiet "$labelled_ref" &&
      git merge-base --is-ancestor "$base" "$labelled_ref" 2>/dev/null; then reached=1; fi
  fi
  if ((!reached && historic)); then
    echo "check-cem-base-retained: historical label $target base=$base retention NOT_PROVED"
    continue
  fi
  if ((!reached)); then
    echo "check-cem-base-retained: $map: base $base is not reachable from a retained ref (${retained[*]-})" >&2
    echo "  fix: rebind the change on a base that ${retained[0]} reaches (make dogfood-change BASE=\$(git rev-parse ${retained[0]})), or pin its base under a real retained ref in $register (CEM-V1-7901)" >&2
    refused=1
  fi
done <<< "$maps"
while IFS=' ' read -r target base; do
  [[ -n $target ]] || continue
  if ! printf '%s' "$present" | grep -Fxq "$target"; then
    echo "check-cem-base-retained: $register labels $target, but .corvint/changes/$target.cem.json is not tracked" >&2
    refused=1
  fi
done <<< "$labelled"
((refused)) && exit 1
echo "check-cem-base-retained: every committed map's base is retained or labelled ($(wc -l <<< "$maps" | tr -d ' ') maps, $labelled_count labelled)"
