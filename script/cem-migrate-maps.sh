#!/bin/sh
set -eu

# Rewrite every committed change map to cem/1.0 with the converter (V1-1277, decision 0525 §3
# step 6, docs/specs/cem-stable-v1.md CEM-V1-200).
#
#   script/cem-migrate-maps.sh CORVINT [REPOSITORY]
#
# CORVINT is a corvint executable with `cem convert`; REPOSITORY (default: the current directory)
# is the checkout whose .corvint/changes/<bind>.cem.json maps are rewritten in place. Each map is
# converted with `corvint cem convert --in-place --repository REPOSITORY --map <abs>`; the bind
# commit is the file name. A map that is already cem/1.0 is refused `convert-source-is-stable`
# and stays byte-identical. Any other refusal stops the run at that map, printing the converter's
# report; maps converted before it stay rewritten, so rerun after `git checkout -- .corvint/changes`.
# The last line is "converted=N unchanged=N refused=N".
#
# The rewrite is not the check. `cem-convert-parity --migrated` rechecks every committed map
# against the converter's output over its provenance-named source blob.

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: script/cem-migrate-maps.sh CORVINT [REPOSITORY]" >&2
  exit 2
fi
corvint=$1
repository=$(cd "${2:-.}" && pwd -P)
converted=0
unchanged=0
for map in "$repository"/.corvint/changes/*.cem.json; do
  [ -f "$map" ] || continue
  set +e
  report=$("$corvint" cem convert --in-place --repository "$repository" --map "$map" </dev/null)
  rc=$?
  set -e
  case "$rc:$report" in
    0:*) converted=$((converted + 1)) ;;
    1:*'"convert-source-is-stable"'*) unchanged=$((unchanged + 1)) ;;
    *)
      printf '%s\n' "refused $map (exit $rc)" "$report" >&2
      echo "converted=$converted unchanged=$unchanged refused=1"
      exit 1
      ;;
  esac
done
echo "converted=$converted unchanged=$unchanged refused=0"
