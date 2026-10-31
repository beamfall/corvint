#!/usr/bin/env bash
# usage: dogfood-bind-coverage.sh BASE TARGET
# DCW-V0-831 (V1-3372): lists, read-only, which change evidence covers each first-parent commit of
# the half-open range BASE..TARGET in the current repository; BASE itself is never listed. A CEM
# covers the half-open range (baseRevision, binding]: a sealed `.corvint/changes/<binding>.cem.json`
# in TARGET's tree, or a `.corvint/change.cem.json` written by a commit reachable from TARGET and not
# from BASE (a non-merge lane binding, or a retroactive binding merged with -s ours). One line per commit:
#   COMMIT<TAB>sealed:PATH        a sealed CEM's range holds COMMIT
#   COMMIT<TAB>bind:BINDING       an unsealed binding commit's CEM range holds COMMIT
#   COMMIT<TAB>seal:EVIDENCE      COMMIT only renames its first parent's sidecar, byte for byte, to
#                                 .corvint/changes/<parent>.cem.json and that parent is a binding
#                                 (docs/DOGFOOD.md: a seal commit is covered by its parent)
#   COMMIT<TAB>merged:EVIDENCE    a merge whose merged side holds a binding (first one shown) and
#                                 whose conflict resolution adds nothing outside CEM paths
#   COMMIT<TAB>no-change          COMMIT's tree equals its first parent's (a -s ours merge)
#   COMMIT<TAB>UNBOUND:resolution a merge whose result differs from every parent outside CEM paths
#                                 (`git diff-tree --cc`), which no constituent CEM covers
#   COMMIT<TAB>UNBOUND            nothing above
# then `dogfood-bind-coverage: commits=N covered=C no-change=M unbound=U`. It writes nothing and
# exits 0 whether or not a commit is unbound; 2 when BASE or TARGET is unusable.
set -uo pipefail
export LC_ALL=C
exec </dev/null

refuse() {
  printf 'dogfood-bind-coverage: REFUSE %s\n' "$1" >&2
  exit 2
}

[[ $# -eq 2 ]] || refuse 'usage: dogfood-bind-coverage.sh BASE TARGET'
git rev-parse --git-dir >/dev/null 2>&1 || refuse not-a-git-repository
base=$(git rev-parse --verify --quiet "$1^{commit}") || refuse base-unavailable
target=$(git rev-parse --verify --quiet "$2^{commit}") || refuse target-unavailable
git merge-base --is-ancestor "$base" "$target" || refuse base-not-ancestor-of-target

# Constituents as "BINDING<TAB>CEM-BASE<TAB>LABEL", sealed archives first.
constituents=()
cem_base() { git cat-file blob "$1" 2>/dev/null | head -c 65536 | sed -n 's/.*"baseRevision": *"\([0-9a-f]\{40\}\)".*/\1/p' | head -1; }
while IFS=$'\t' read -r meta path; do
  [[ -n $path ]] || continue
  binding=${path#.corvint/changes/}
  binding=${binding%.cem.json}
  [[ $binding =~ ^[0-9a-f]{40}$ ]] || continue
  from=$(cem_base "${meta##* }")
  [[ -n $from ]] && git cat-file -e "$binding^{commit}" 2>/dev/null || continue
  constituents+=("$binding"$'\t'"$from"$'\t'"sealed:$path")
done < <(git ls-tree -r "$target" -- .corvint/changes/)
while read -r binding; do
  [[ -n $binding ]] || continue
  from=$(cem_base "$binding:.corvint/change.cem.json")
  [[ -n $from ]] || continue
  constituents+=("$binding"$'\t'"$from"$'\t'"bind:$binding")
done < <(git log --full-history --no-merges --format=%H "$target" --not "$base" -- .corvint/change.cem.json)

in_range() { # COMMIT FROM BINDING: COMMIT is in (FROM, BINDING]
  git merge-base --is-ancestor "$1" "$3" && ! git merge-base --is-ancestor "$1" "$2"
}

# byte_identical_seal PARENT COMMIT: COMMIT's whole change is deleting PARENT's sidecar and adding
# that same blob as .corvint/changes/PARENT.cem.json; altered bytes are not a seal.
byte_identical_seal() {
  local sidecar raw
  sidecar=$(git rev-parse --verify --quiet "$1:.corvint/change.cem.json") || return 1
  raw=$(git diff-tree -r --no-renames --no-commit-id "$1" "$2" | awk '{print $5, $4, $6}' | LC_ALL=C sort)
  [[ $raw == "A $sidecar .corvint/changes/$1.cem.json"$'\n'"D 0000000000000000000000000000000000000000 .corvint/change.cem.json" ]]
}

commits=0 covered=0 unchanged=0 unbound=0
while read -r commit first second _; do
  [[ -n $commit ]] || continue
  commits=$((commits + 1))
  label=
  for entry in ${constituents[@]+"${constituents[@]}"}; do
    IFS=$'\t' read -r binding from name <<< "$entry"
    if in_range "$commit" "$from" "$binding"; then label=$name; break; fi
  done
  if [[ -z $label && -n $first && -z $second ]] && byte_identical_seal "$first" "$commit"; then
    for entry in ${constituents[@]+"${constituents[@]}"}; do
      IFS=$'\t' read -r binding from name <<< "$entry"
      if [[ $binding == "$first" ]]; then label="seal:$name"; break; fi
    done
  fi
  if [[ -z $label && -n $first && $(git rev-parse "$commit^{tree}") == $(git rev-parse "$first^{tree}") ]]; then
    label=no-change
  fi
  if [[ -z $label && -n $second ]] && [[ -n $(git diff-tree --cc --no-commit-id --name-only "$commit" -- . \
    ':(exclude).corvint/change.cem.json' ':(exclude).corvint/changes/') ]]; then
    label=UNBOUND:resolution
  fi
  if [[ -z $label && -n $second ]]; then
    for entry in ${constituents[@]+"${constituents[@]}"}; do
      IFS=$'\t' read -r binding from name <<< "$entry"
      if git merge-base --is-ancestor "$binding" "$commit" && ! git merge-base --is-ancestor "$binding" "$first"; then
        label="merged:$name"; break
      fi
    done
  fi
  case $label in
    '') label=UNBOUND; unbound=$((unbound + 1)) ;;
    UNBOUND:*) unbound=$((unbound + 1)) ;;
    no-change) unchanged=$((unchanged + 1)) ;;
    *) covered=$((covered + 1)) ;;
  esac
  printf '%s\t%s\n' "$commit" "$label"
done < <(git rev-list --first-parent --reverse --parents "$base..$target")
printf 'dogfood-bind-coverage: commits=%s covered=%s no-change=%s unbound=%s\n' "$commits" "$covered" "$unchanged" "$unbound"
