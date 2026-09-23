#!/usr/bin/env bash
set -euo pipefail

# CEM-PILOT-024..027 on fixture repositories: each recipe under examples/cem/recipes completes
# on a supported fixture and, for missing, stale and unsupported evidence, exits non-zero with
# the refusal retained. The bounded runner kills a step at RECIPE_TIMEOUT and kills the child
# when the recipe is interrupted. CORVINT_BIN selects an existing Corvint CLI (for example the
# installed release); otherwise corvint is built from this checkout. The portable verifier is
# always built from this checkout's interop/cem01-go.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
recipes="$source_root/examples/cem/recipes"
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-cem-recipes-test.XXXXXX")
test_root=$(CDPATH='' cd -- "$test_root" && pwd -P)
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'cem-recipes: FAIL %s\n' "$1" >&2; exit 1; }

corvint_bin=${CORVINT_BIN-}
if [ -z "$corvint_bin" ]; then
  corvint_bin="$test_root/corvint"
  (cd "$source_root" && GOTOOLCHAIN=local go build -o "$corvint_bin" ./cmd/corvint)
fi
verifier="$test_root/cem01-go"
(cd "$source_root/interop/cem01-go" && GOTOOLCHAIN=local CGO_ENABLED=0 go build -trimpath -o "$verifier" .)
pin=$(shasum -a 256 "$verifier" | cut -d' ' -f1)
printf 'cem-recipes: corvint %s (%s)\n' "$("$corvint_bin" --version)" "$corvint_bin"

new_repo() {
  git init -q -b main "$1"
  git -C "$1" config user.email fixture@example.test
  git -C "$1" config user.name Fixture
}

# run NAME RECIPE [ENV=VALUE...]: runs one recipe, keeping its status and its stdout lines.
run() {
  local recipe=$2
  name=$1
  shift 2
  set +e
  env RECIPE_OUT="$test_root/out/$name" CORVINT_BIN="$corvint_bin" "$@" "$recipes/$recipe" \
    > "$test_root/$name.log" 2>&1
  status=$?
  set -e
  out="$test_root/out/$name"
}
expect() { # STATUS PATTERN
  [ "$status" -eq "$1" ] || { cat "$test_root/$name.log" >&2; fail "$name exit $status, want $1"; }
  grep -q -- "$2" "$test_root/$name.log" || { cat "$test_root/$name.log" >&2; fail "$name lacks $2"; }
}

# --- Go fixture for recipes 1 and 2 --------------------------------------------------------
repo="$test_root/repo"
new_repo "$repo"
mkdir -p "$repo/auth" "$repo/docs"
printf 'module example.test/fixture\n\ngo 1.22\n' > "$repo/go.mod"
printf 'package auth\n\n// Lifetime is the session token lifetime in seconds.\nconst Lifetime = 600\n' \
  > "$repo/auth/auth.go"
printf 'package auth\n\nimport "testing"\n\nfunc TestLifetime(t *testing.T) {\n\tif Lifetime <= 0 {\n\t\tt.Fatal("lifetime")\n\t}\n}\n' \
  > "$repo/auth/auth_test.go"
printf '# Session policy\n\nSession tokens expire after nine hundred seconds.\n' > "$repo/docs/policy.md"
git -C "$repo" add .
git -C "$repo" commit -qm base
base=$(git -C "$repo" rev-parse HEAD)
sed 's/600/900/' "$repo/auth/auth.go" > "$repo/auth/auth.go.new"
mv "$repo/auth/auth.go.new" "$repo/auth/auth.go"
git -C "$repo" commit -qam change
task='raise the session token lifetime to the documented policy'
understand=(CORVINT_ROOT="$repo" CORVINT_BASE="$base" CORVINT_TASK="$task" CORVINT_SUBJECT=auth/auth.go)

# Recipe 1: complete on a Go range, writing nothing into the repository.
run u-complete understand-change.sh "${understand[@]}"
expect 0 'outcome=complete impact-state=READY'
for step in impact affected context; do [ -s "$out/$step.out" ] || fail "u-complete $step.out empty"; done
[ -z "$(git -C "$repo" status --porcelain --ignored)" ] || fail 'understand wrote into the repository'

# Missing evidence: a base the repository does not hold is refused, and later steps do not run.
run u-missing understand-change.sh CORVINT_ROOT="$repo" CORVINT_BASE="$(printf 'ab%.0s' {1..20})" \
  CORVINT_TASK="$task"
expect 3 'outcome=incomplete refused=impact'
grep -q unsupported-impact-range "$out/impact.out" "$out/impact.stderr" || fail 'u-missing refusal not retained'
[ ! -e "$out/affected.out" ] || fail 'u-missing ran a step after the refusal'

# Stale evidence: an uncommitted edit means base..HEAD no longer describes the worktree.
printf '// pending\n' >> "$repo/auth/auth.go"
run u-stale understand-change.sh "${understand[@]}"
git -C "$repo" checkout -q -- auth/auth.go
expect 3 'outcome=incomplete refused=impact'
grep -q unsupported-impact-worktree "$out/impact.out" "$out/impact.stderr" || fail 'u-stale refusal not retained'

# Unsupported evidence: a range with no Go path is outside the native range profile.
git -C "$repo" checkout -q -b docs-only
printf 'Refresh tokens rotate on use.\n' >> "$repo/docs/policy.md"
git -C "$repo" commit -qam docs
run u-unsupported understand-change.sh CORVINT_ROOT="$repo" CORVINT_BASE="$(git -C "$repo" rev-parse HEAD~1)" \
  CORVINT_TASK="$task"
git -C "$repo" checkout -q main
expect 3 'outcome=incomplete impact-state=OUT_OF_SCOPE'

run u-no-task understand-change.sh CORVINT_ROOT="$repo" CORVINT_BASE="$base"
expect 2 'outcome=operational reason=CORVINT_TASK is required'

# Recipe 2: missing evidence first (the prepared hunk is unknown), then an uncommitted map,
# then complete; a later code commit makes the map stale and prepare refuses to replace it.
review=(CORVINT_ROOT="$repo" CORVINT_BASE="$base")
run r-missing review-change.sh "${review[@]}"
expect 3 'outcome=incomplete refused=status'
grep -q '"unknown":1' "$out/status.out" || fail 'r-missing status not retained'
"$corvint_bin" --root "$repo" cem cite --map .corvint/change.cem.json --hunk 1 \
  --evidence-path docs/policy.md --lines 3:3 --relation specification > /dev/null
run r-uncommitted review-change.sh "${review[@]}"
expect 3 'outcome=incomplete map-uncommitted=.corvint/change.cem.json'
git -C "$repo" add .corvint/change.cem.json
git -C "$repo" commit -qm 'change evidence'
run r-complete review-change.sh "${review[@]}"
expect 0 'outcome=complete report='
rendered=$(sed -n 's/.*outcome=complete report=\(.*\) out=.*/\1/p' "$test_root/$name.log")
[ -s "$rendered" ] && [ -s "$out/review.out" ] || fail 'r-complete report or review missing'
[ -z "$(git -C "$repo" status --porcelain)" ] || fail 'r-complete left the worktree dirty'

sed 's/900/901/' "$repo/auth/auth.go" > "$repo/auth/auth.go.new"
mv "$repo/auth/auth.go.new" "$repo/auth/auth.go"
git -C "$repo" commit -qam later
map_before=$(git -C "$repo" hash-object .corvint/change.cem.json)
run r-stale review-change.sh "${review[@]}"
expect 3 'outcome=incomplete refused=prepare'
grep -q 'different base or patch' "$out/prepare.out" "$out/prepare.stderr" || fail 'r-stale refusal not retained'
[ "$(git -C "$repo" hash-object .corvint/change.cem.json)" = "$map_before" ] || fail 'r-stale replaced the map'

sed 's#"spec": *"cem/0.2"#"spec":"cem/9.9"#' "$repo/.corvint/change.cem.json" > "$test_root/unsupported.json"
cp "$test_root/unsupported.json" "$repo/.corvint/change.cem.json"
run r-unsupported review-change.sh "${review[@]}"
expect 3 'outcome=incomplete refused=prepare'
grep -q 'not a valid CEM document' "$out/prepare.out" "$out/prepare.stderr" || fail 'r-unsupported refusal not retained'
git -C "$repo" checkout -q -- .corvint/change.cem.json

run r-bad-cap review-change.sh "${review[@]}" CEM_MAX_UNKNOWN=many
expect 2 'CEM_MAX_UNKNOWN must be a whole number'
[ ! -e "$out/prepare.out" ] || fail 'r-bad-cap ran prepare'

# --- Recipe 3: portable CI verification of a cem/0.1 map ------------------------------------
ci="$test_root/ci"
new_repo "$ci"
mkdir -p "$ci/docs" "$ci/.corvint"
printf 'one\n' > "$ci/app.txt"
printf '# Decision\n\nApp gains a second line.\n' > "$ci/docs/decision.md"
git -C "$ci" add .
git -C "$ci" commit -qm base
ci_base=$(git -C "$ci" rev-parse HEAD)
printf 'one\ntwo\n' > "$ci/app.txt"
git -C "$ci" commit -qam change
ci_change=$(git -C "$ci" rev-parse HEAD)
# The map is produced from exactly the docs/CEM-CI.md patch profile.
git -C "$ci" -c core.quotePath=false -c diff.algorithm=myers -c diff.context=3 \
  diff --binary --full-index --no-color --no-ext-diff --no-textconv --no-renames \
  --no-indent-heuristic --diff-algorithm=myers --unified=3 \
  --src-prefix=a/ --dst-prefix=b/ --ignore-submodules=none \
  "$ci_base" "$ci_change" -- . ':(exclude).corvint/change.cem.json' > "$test_root/ci.patch"
"$corvint_bin" --root "$ci" cem begin --patch "$test_root/ci.patch" --output .corvint/change.cem.json \
  --base "$ci_base" > /dev/null
"$corvint_bin" --root "$ci" cem cite --map .corvint/change.cem.json --hunk 1 \
  --evidence-path docs/decision.md --lines 3:3 --relation decision > /dev/null
git -C "$ci" add .corvint/change.cem.json
git -C "$ci" commit -qm 'change evidence'
ci_head=$(git -C "$ci" rev-parse HEAD)
pins=(CEM_REPOSITORY="$ci" CEM_BASE_SHA="$ci_base" CEM_VERIFIER="$verifier" CEM_VERIFIER_SHA256="$pin")

run c-accepted ci-verify.sh "${pins[@]}" CEM_HEAD_SHA="$ci_head"
expect 0 'outcome=accepted exit=0'
grep -q '"schema":"cem-ci-report/0"' "$out/verify.out" || fail 'c-accepted report not retained'

run c-missing ci-verify.sh "${pins[@]}" CEM_HEAD_SHA="$ci_change"
expect 3 'outcome=missing-evidence exit=3'
grep -q '"code":"map-absent"' "$out/verify.out" || fail 'c-missing report not retained'

# Stale: a later code commit leaves the committed map describing an older patch.
printf 'one\ntwo\nthree\n' > "$ci/app.txt"
git -C "$ci" commit -qam later
run c-stale ci-verify.sh "${pins[@]}" CEM_HEAD_SHA="$(git -C "$ci" rev-parse HEAD)"
expect 1 'outcome=rejected exit=1'

# Unsupported: a cem/0.2 map from `cem prepare` needs verify-pr.sh, not this verifier.
git -C "$ci" reset -q --hard "$ci_change"
"$corvint_bin" --root "$ci" cem prepare --base "$ci_base" --target HEAD > /dev/null
git -C "$ci" add .corvint/change.cem.json
git -C "$ci" commit -qm 'cem/0.2 evidence'
run c-unsupported ci-verify.sh "${pins[@]}" CEM_HEAD_SHA="$(git -C "$ci" rev-parse HEAD)"
expect 4 'outcome=unsupported-profile exit=4'

# A verifier whose digest differs from the pin is never executed.
printf '#!/bin/sh\n: > "%s/ran"\n' "$test_root" > "$test_root/impostor"
chmod +x "$test_root/impostor"
run c-digest ci-verify.sh "${pins[@]}" CEM_HEAD_SHA="$ci_head" CEM_VERIFIER="$test_root/impostor"
expect 2 'outcome=operational exit=2'
[ ! -e "$test_root/ran" ] || fail 'c-digest executed a mismatched verifier'
grep -q 'not executed' "$out/verify.stderr" || fail 'c-digest refusal not retained'

# --- Bounds: a hung step is killed at RECIPE_TIMEOUT, and interruption kills the child -------
printf '#!/bin/sh\necho $$ > "%s/stub.pid"\nexec sleep 60\n' "$test_root" > "$test_root/hang"
chmod +x "$test_root/hang"
run b-timeout understand-change.sh "${understand[@]}" CORVINT_BIN="$test_root/hang" RECIPE_TIMEOUT=1
expect 2 'outcome=operational timeout=impact'
! kill -0 "$(cat "$test_root/stub.pid")" 2>/dev/null || fail 'b-timeout left the step running'

# A step that ignores TERM is killed after the 2-second grace, so the bound still holds.
printf '#!/bin/sh
trap "" TERM
echo $$ > "%s/stub.pid"
exec sleep 60
' "$test_root" > "$test_root/stubborn"
chmod +x "$test_root/stubborn"
started=$SECONDS
run b-stubborn understand-change.sh "${understand[@]}" CORVINT_BIN="$test_root/stubborn" RECIPE_TIMEOUT=1
expect 2 'step=impact exit=124'
[ $((SECONDS - started)) -le 6 ] || fail "b-stubborn took $((SECONDS - started)) s, bound is 1 + 2 s grace"
! kill -0 "$(cat "$test_root/stub.pid")" 2>/dev/null || fail 'b-stubborn left the step running'

run b-out-reused understand-change.sh "${understand[@]}" RECIPE_OUT="$test_root/out/b-stubborn"
expect 2 'RECIPE_OUT must be a new or empty directory'

rm -f "$test_root/stub.pid"
env RECIPE_OUT="$test_root/out/b-interrupt" CORVINT_BIN="$test_root/stubborn" "${review[@]}" \
  "$recipes/review-change.sh" > "$test_root/b-interrupt.log" 2>&1 &
recipe_pid=$!
for _ in $(seq 50); do [ -s "$test_root/stub.pid" ] && break; sleep 0.1; done
[ -s "$test_root/stub.pid" ] || fail 'b-interrupt step never started'
kill -TERM "$recipe_pid"
set +e; wait "$recipe_pid"; status=$?; set -e
[ "$status" -eq 143 ] || fail "b-interrupt exit $status, want 143"
sleep 0.2
! kill -0 "$(cat "$test_root/stub.pid")" 2>/dev/null || fail 'b-interrupt left the step running'

echo "cem-recipes: ok"
