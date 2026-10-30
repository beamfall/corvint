#!/bin/sh
set -eu

# SRG-V0-041: resolves a merge whose only remaining conflicts are in the generated spec index
# files (docs/specs/REQUIREMENTS.tsv, INDEX.json, README.md), as when two branches append adjacent
# requirement rows or each add a new spec. It refuses, naming them, while any other path is
# unmerged: resolve and stage a conflicted spec first. Each conflicted generated file restarts
# from this side's version (`--ours`), then all three are regenerated from the staged specs, as
# `make spec-requirements` does, and staged. A new spec the other side added gets the INDEX.json
# entry and README.md row its header derives; hand-set fields of that entry are re-applied by hand.

generated='docs/specs/REQUIREMENTS.tsv docs/specs/INDEX.json docs/specs/README.md'
unmerged=$(git diff --name-only --diff-filter=U)
test -n "$unmerged" || { echo "spec-requirements-resolve: no unmerged path" >&2; exit 1; }
other=
for path in $unmerged; do
  case " $generated " in
    *" $path "*) ;;
    *) other="$other $path" ;;
  esac
done
if test -n "$other"; then
  echo "spec-requirements-resolve: resolve and stage these first:$other" >&2
  exit 1
fi
for path in $unmerged; do
  git checkout -q --ours -- "$path"
  git add -- "$path"
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
script/gen-spec-requirements.sh --all "$tmp"
for file in REQUIREMENTS.tsv INDEX.json README.md; do
  cp "$tmp/$file" "docs/specs/$file"
done
git add -- $generated
echo "spec-requirements-resolve: regenerated and staged $generated"
