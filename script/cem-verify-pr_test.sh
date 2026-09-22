#!/bin/sh
set -eu

# CEM-PILOT-009 on a fixture repository: examples/cem/verify-pr.sh passes a committed cem/0.2 map
# whose one hunk is cited, applies its unknown-hunk policy cap in both directions, refuses a map
# whose base is not the independently supplied base, and leaves no private work directory behind.
# CORVINT_BIN selects an existing Corvint CLI.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-cem-verify-pr-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

corvint_bin=${CORVINT_BIN-}
if [ -z "$corvint_bin" ]; then
  corvint_bin="$test_root/corvint"
  (cd "$source_root" && GOTOOLCHAIN=local go build -o "$corvint_bin" ./cmd/corvint)
fi

repository="$test_root/repo"
mkdir -p "$repository/docs"
git -C "$repository" init -q -b main
git -C "$repository" config user.email fixture@example.test
git -C "$repository" config user.name Fixture
printf 'one\n' > "$repository/app.txt"
printf '# Decision\n\nApp gains a second line.\n' > "$repository/docs/decision.md"
git -C "$repository" add .
git -C "$repository" commit -qm base
base=$(git -C "$repository" rev-parse HEAD)
printf 'one\ntwo\n' > "$repository/app.txt"
git -C "$repository" commit -qam target
target=$(git -C "$repository" rev-parse HEAD)

"$corvint_bin" --root "$repository" cem prepare --base "$base" --target HEAD > /dev/null
git -C "$repository" add .corvint/change.cem.json
git -C "$repository" commit -qm 'uncited candidate'
uncited=$(git -C "$repository" rev-parse HEAD)
"$corvint_bin" --root "$repository" cem cite --map .corvint/change.cem.json --hunk 1 \
  --evidence-path docs/decision.md --lines 3:3 --relation decision > /dev/null
git -C "$repository" commit -qam 'cited candidate'
cited=$(git -C "$repository" rev-parse HEAD)

verify() {
  set +e
  env CEM_REPOSITORY="$repository" CEM_PROFILE=cem/0.2 CORVINT_BIN="$corvint_bin" \
    CEM_BASE_SHA="$1" CEM_HEAD_SHA="$2" CEM_MAX_UNKNOWN="$3" \
    "$source_root/examples/cem/verify-pr.sh" > "$test_root/out" 2> "$test_root/err"
  status=$?
  set -e
}

verify "$base" "$cited" 0
test "$status" -eq 0 || { cat "$test_root/err" >&2; exit 1; }

# The unknown-hunk cap is applied, not implied: over the cap fails, at the cap passes.
verify "$base" "$uncited" 0
test "$status" -ne 0
grep -q '"code":"max-unknown-exceeded"' "$test_root/out"
verify "$base" "$uncited" 1
test "$status" -eq 0 || { cat "$test_root/err" >&2; exit 1; }

# A map prepared against `base` is refused when CI independently derives a different base.
verify "$target" "$cited" 0
test "$status" -ne 0
grep -q '"code":"base-revision-mismatch"' "$test_root/out"

# The private work directory is removed and nothing is published into the checkout.
test -z "$(find "$repository" -maxdepth 1 -name '.corvint-cem.*')"
test -z "$(git -C "$repository" status --porcelain)"

echo "cem-verify-pr: ok"
