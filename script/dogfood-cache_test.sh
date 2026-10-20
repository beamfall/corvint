#!/usr/bin/env bash
# DCW-V0-038: observe every wrapper build environment without invoking Go or writing caches.
# Bash 3.2 does not apply errexit to [[ ]] in loops; assertions need explicit exits.
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd)
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-cache-test.XXXXXX")
cleanup() {
  find "$root" -type f -exec /bin/rm -f {} +
  find "$root" -depth -type d -exec rmdir {} +
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$root/repo/script" "$root/bin"
/bin/cp -f "$source_root"/script/dogfood-{change,check,seal,batch-check,build-corvint}.sh "$root/repo/script/"
/bin/cp -f "$source_root/Makefile" "$root/repo/"
printf 'test-version\n' > "$root/repo/VERSION"
printf 'base-source\n' > "$root/repo/source-marker"
git -C "$root/repo" init -q
git -C "$root/repo" add .
git -C "$root/repo" -c user.name=test -c user.email=test@example.invalid commit -qm fixture
base=$(git -C "$root/repo" rev-parse HEAD)
printf 'tree-source\n' > "$root/repo/source-marker"
cat > "$root/bin/corvint" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ ${1-} == --version ]]; then
  printf 'Corvint %s (build 0)\n' "${TEST_VERSION:-test-version}"
else
  printf '%s\n' "$*" >> "$TEST_FLOW_LOG"
fi
EOF
cat > "$root/bin/go" <<'EOF'
#!/usr/bin/env bash
set -eu
[[ $1 == build && $2 == -o && ${!#} == ./cmd/corvint ]] || exit 1
printf '%s\t%s\t%s\t%s\n' "$GOCACHE" "$GOTOOLCHAIN" "$(cat source-marker)" "$*" >> "$TEST_BUILD_LOG"
[[ ${TEST_FAIL_SOURCE-} != "$(cat source-marker)" ]] || exit 1
/bin/cp -f "$TEST_FAKE_CORVINT" "$3"
EOF
chmod +x "$root/bin/"* "$root/repo/script/"*.sh
export TEST_FAKE_CORVINT="$root/bin/corvint" TEST_BUILD_LOG="$root/build.log" TEST_FLOW_LOG="$root/flow.log"
export PATH="$root/bin:$PATH"
unset CORVINT_BIN CORVINT_GOCACHE GOCACHE TEST_FAIL_SOURCE TEST_VERSION
cd "$root/repo"

# A path with spaces also checks that the selected value remains one environment value.
for route in direct make; do
  for selection in default caller corvint both empty-corvint empty-all make-argument; do
    unset CORVINT_GOCACHE GOCACHE
    args=()
    expected=/tmp/corvint-go-build-cache
    case $selection in
      caller|empty-corvint) export GOCACHE="$root/caller cache"; expected=$GOCACHE ;;
      corvint|both) export CORVINT_GOCACHE="$root/corvint cache"; expected=$CORVINT_GOCACHE ;;
      make-argument)
        [[ $route == make ]] || continue
        export GOCACHE="$root/caller cache"
        args=("CORVINT_GOCACHE=$root/argument cache"); expected="$root/argument cache" ;;
    esac
    [[ $selection != both ]] || export GOCACHE="$root/caller cache"
    [[ $selection != empty-corvint && $selection != empty-all ]] || export CORVINT_GOCACHE=
    [[ $selection != empty-all ]] || export GOCACHE=
    for action in change check seal batch-check; do
      : > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
      status=0
      if [[ $route == direct ]]; then
        bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
      else
        make -s "dogfood-$action" "BASE=$base" ${args[@]+"${args[@]}"} > "$root/output" 2>&1 || status=$?
      fi
      if [[ $action == seal ]]; then
        # No CEM exists: prove seal reaches check, then refuses before a commit or rename.
        [[ $status == 2 ]] || exit 1
        rg -q '^dogfood-seal: REFUSE nothing-to-seal$' "$root/output" || exit 1
      else
        [[ $status == 0 ]] || exit 1
      fi
      count=0
      while IFS=$'\t' read -r cache toolchain source argv; do
        [[ $cache == "$expected" && $toolchain == local ]] || {
          printf '%s/%s/%s: expected cache=%s toolchain=local; got cache=%s toolchain=%s\n' \
            "$route" "$selection" "$action" "$expected" "$cache" "$toolchain" >&2
          exit 1
        }
        count=$((count + 1))
        if [[ $count == 1 ]]; then [[ $source == tree-source ]] || exit 1; else [[ $source == base-source ]] || exit 1; fi
        [[ $argv == *' -trimpath ./cmd/corvint' ]] || exit 1
      done < "$TEST_BUILD_LOG"
      if [[ $action == change ]]; then
        [[ $count == 1 ]] || exit 1
        rg -q "^dogfood change --corvint-bin .* $base$" "$TEST_FLOW_LOG" || exit 1
      elif [[ $action == batch-check ]]; then
        [[ $count == 1 ]] || exit 1
        rg -q "^dogfood batch-check --verifier .* $base$" "$TEST_FLOW_LOG" || exit 1
      else
        [[ $count == 2 ]] || exit 1
        rg -q "^dogfood check --base-verifier .* --tree-verifier .* $base$" "$TEST_FLOW_LOG" || exit 1
      fi
    done
  done
done

# Existing version refusals happen before builds; a valid override bypasses only change's build.
export CORVINT_GOCACHE="$root/private cache" CORVINT_BIN="$TEST_FAKE_CORVINT"
for action in change check; do
  : > "$TEST_BUILD_LOG"
  status=0
  TEST_VERSION=wrong bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
  [[ $status == 2 && ! -s $TEST_BUILD_LOG ]] || exit 1
  rg -q "^dogfood-$action: REFUSE corvint-version-mismatch expected=test-version$" "$root/output" || exit 1
  bash "script/dogfood-$action.sh" "$base"
  if [[ $action == change ]]; then [[ ! -s $TEST_BUILD_LOG ]] || exit 1; else
    [[ $(wc -l < "$TEST_BUILD_LOG") -eq 2 ]] || exit 1
    rg -q -- "--override-verifier $CORVINT_BIN $base$" "$TEST_FLOW_LOG" || exit 1
  fi
done
unset CORVINT_BIN
for action in change check; do
  for source in tree-source base-source; do
    [[ $action != change || $source != base-source ]] || continue
    : > "$TEST_BUILD_LOG"; : > "$TEST_FLOW_LOG"
    status=0
    TEST_FAIL_SOURCE=$source bash "script/dogfood-$action.sh" "$base" > "$root/output" 2>&1 || status=$?
    [[ $status == 2 && ! -s $TEST_FLOW_LOG ]] || exit 1
    reason=current-tree-corvint-build-failed
    [[ $source != base-source ]] || reason=base-corvint-build-failed
    rg -q "^dogfood-$action: REFUSE $reason$" "$root/output" || exit 1
  done
done
# The stub never creates even the selected caches, and no real Go build runs in this test.
[[ ! -e "$root/caller cache" && ! -e "$root/corvint cache" && ! -e "$root/private cache" && ! -e "$root/argument cache" ]] || exit 1
printf 'DCW-V0-038 dogfood cache selection: PASS\n'
