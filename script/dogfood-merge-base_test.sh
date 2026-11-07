#!/usr/bin/env bash
# DWC-V0-7601 (V1-4721, V1-0527): script/dogfood-merge-base.sh derives a merge-forward lane's BASE
# as the merged mainline commit, lists the conflicted paths, keeps the BASE across later bind and
# seal commits, and refuses a missing, octopus, foreign or non-mainline merge. make
# BASE=merge-forward passes the derived SHA to a dogfood target, and a refused lookup stops make
# before any target runs.
set -euo pipefail
export LC_ALL=C

source_root=$(cd "$(dirname "$0")/.." && pwd)
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-merge-base-test.XXXXXX")
trap '/bin/rm -rf "$root"' EXIT
fail() {
  printf 'dogfood-merge-base_test: FAIL line=%s\n' "$1" >&2
  exit 1
}
script=$source_root/script/dogfood-merge-base.sh

git init -q -b main "$root/repo"
cd "$root/repo"
git config user.email test@example.invalid
git config user.name test
printf 'a\n' > shared.txt
git add shared.txt
git commit -q -m base
# The lane has no merge yet.
git checkout -q -b lane
status=0
"$script" > "$root/out" 2> "$root/err" || status=$?
((status == 2)) && grep -q '^dogfood-merge-base: REFUSE no-merge-forward$' "$root/err" || fail "$LINENO"
printf 'lane\n' > shared.txt
git commit -q -am lane
# The mainline moves on and conflicts with the lane.
git checkout -q main
printf 'main\n' > shared.txt
printf 'm\n' > main.txt
git add main.txt
git commit -q -am main
main=$(git rev-parse HEAD)
git update-ref refs/remotes/origin/main "$main"
git checkout -q lane
status=0
"$script" > /dev/null 2> "$root/err" || status=$?
((status == 2)) || fail "$LINENO"
# Merge the mainline forward and resolve the conflict by hand.
git merge -q origin/main > /dev/null 2>&1 || true
printf 'lane and main\n' > shared.txt
git add shared.txt
git commit -q --no-edit
merge=$(git rev-parse HEAD)
[[ $("$script") == "$main" ]] || fail "$LINENO"
[[ $("$script" --paths) == shared.txt ]] || fail "$LINENO"
[[ $("$script" "$merge") == "$main" ]] || fail "$LINENO"
# A failing git show --remerge-diff is refused, not printed as a merge with no conflicted path.
mkdir "$root/shim"
printf '#!/bin/sh\n[ "$1" != show ] || { echo "fatal: injected show failure" >&2; exit 128; }\nexec '"'%s'"' "$@"\n' "$(command -v git)" > "$root/shim/git"
chmod +x "$root/shim/git"
status=0
PATH="$root/shim:$PATH" "$script" --paths > "$root/out" 2> "$root/err" || status=$?
((status == 2)) && [[ ! -s $root/out ]] && grep -q '^dogfood-merge-base: REFUSE remerge-diff-failed$' "$root/err" &&
  grep -q 'exited 128' "$root/err" || fail "$LINENO"
# A later bind or seal commit (one parent) does not move the BASE.
printf 'bind\n' > bind.txt
git add bind.txt
git commit -q -m bind
[[ $("$script") == "$main" ]] || fail "$LINENO"
# make BASE=merge-forward hands the derived SHA to the target.
printf 'include %s/Makefile\nprobe:\n\t@printf "%%s\\n" "$(DOGFOOD_BASE)"\n' "$source_root" > "$root/probe.mk"
mkdir -p script
cp "$script" script/dogfood-merge-base.sh
[[ $(make -s -f "$root/probe.mk" probe BASE=merge-forward 2> /dev/null) == "$main" ]] || fail "$LINENO"
[[ $(make -s -f "$root/probe.mk" probe BASE="$main" 2> /dev/null) == "$main" ]] || fail "$LINENO"
/bin/rm -rf script
# A merged branch that is not on the mainline is refused.
git checkout -q -b other "$main~1"
printf 'o\n' > other.txt
git add other.txt
git commit -q -m other
git checkout -q lane
git merge -q --no-edit other > /dev/null
status=0
"$script" > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q '^dogfood-merge-base: REFUSE merged-parent-not-mainline$' "$root/err" || fail "$LINENO"
# So is a merge outside HEAD, and a missing mainline ref.
status=0
"$script" other > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q 'REFUSE merge-not-two-parent' "$root/err" || fail "$LINENO"
status=0
DOGFOOD_MAINLINE_REF=refs/remotes/nowhere/main "$script" "$merge" > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q 'REFUSE mainline-ref-missing' "$root/err" || fail "$LINENO"
git checkout -q -b elsewhere "$main"
status=0
"$script" "$merge" > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q 'REFUSE merge-not-in-head' "$root/err" || fail "$LINENO"
# A refused BASE=merge-forward stops make before any target runs. An empty DOGFOOD_BASE would read
# as an omitted BASE, which dogfood-check takes as the bound base and dogfood-closeout-precheck as
# no base. Stubs log what each target runs; the probe drops any BASE that a calling make passes down.
mkdir -p script
cp "$script" script/dogfood-merge-base.sh
cat > "$root/stub" << 'EOF'
#!/bin/sh
printf '%s %s base=%s\n' "${0##*/}" "$*" "${SPEC_REQUIREMENTS_BASE-}" >> "$DOGFOOD_MERGE_BASE_TEST_RAN"
[ "${0##*/}" != gen-spec-requirements.sh ]
EOF
for stub in script/batch-precheck.sh script/dogfood-check.sh script/gen-spec-requirements.sh; do
  cp "$root/stub" "$stub"
  chmod +x "$stub"
done
export DOGFOOD_MERGE_BASE_TEST_RAN=$root/ran
probe() {
  env -u BASE -u MAKEFLAGS -u MFLAGS -u MAKELEVEL -u SPEC_REQUIREMENTS_BASE make -s HEAVY_ADMIT= HEAVY_NICE=0 "$@"
}
for target in dogfood-check dogfood-closeout-precheck; do
  status=0
  probe -f "$root/probe.mk" "$target" BASE=merge-forward > "$root/out" 2> "$root/err" || status=$?
  ((status != 0)) && [[ ! -e $root/ran ]] && grep -q '^dogfood-merge-base: REFUSE no-merge-forward$' "$root/err" &&
    grep -q 'BASE=merge-forward refused' "$root/err" || fail "$LINENO"
done
# An omitted or explicit BASE keeps its meaning.
probe -f "$root/probe.mk" dogfood-check > /dev/null
probe -f "$root/probe.mk" dogfood-closeout-precheck > /dev/null
probe -f "$root/probe.mk" dogfood-check BASE="$main" > /dev/null
probe -f "$root/probe.mk" dogfood-closeout-precheck BASE="$main" > /dev/null
[[ $(cat "$root/ran") == "dogfood-check.sh --bound-base base=
batch-precheck.sh  base=
dogfood-check.sh $main base=
batch-precheck.sh --base $main base=" ]] || fail "$LINENO"
# dogfood-closeout hands its spec checks the derived SHA, not the word merge-forward. Its sub-make
# reads the makefile of the working directory, so the probe is a GNUmakefile there.
git checkout -q --detach "$merge"
printf 'include %s/Makefile\n' "$source_root" > GNUmakefile
/bin/rm -f "$root/ran"
status=0
probe dogfood-closeout BASE=merge-forward > /dev/null 2>&1 || status=$?
((status != 0)) && [[ $(cat "$root/ran") == "gen-spec-requirements.sh --capacity base=$main" ]] || fail "$LINENO"
/bin/rm -rf script GNUmakefile
printf 'DWC-V0-7601 dogfood merge-forward BASE: PASS\n'
