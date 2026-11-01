#!/bin/sh
set -eu

# Fixture cases for script/analyzer-schema.sh (V1-2130, V1-1732, V1-2289, V1-3574; IDX-SNAP-V0-060,
# IDX-SNAP-V0-061, IDX-SNAP-V0-107, IDX-SNAP-V0-955). The output pin is the one content pin: two
# branches that each edit an audited input with the same golden output merge without a conflict
# and pass check, which names the changed inputs on stderr; an output change is bumped once and
# kept on a second edit; a base that took the same number with another output digest is refused
# with the rebump recovery and `bump` moves past it; a perf-only edit whose output equals the
# base's keeps the base number; an unresolvable base leaves the collision check NOT_RUN and makes
# `bump` refuse; an output command that reports no digest makes `bump` refuse. The fixture's
# output command reads output.digest, standing in for TestAnalyzerSchemaOutput.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-analyzer-schema-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

digest_a=$(printf 'a%.0s' $(seq 64))
digest_b=$(printf 'b%.0s' $(seq 64))
digest_c=$(printf 'c%.0s' $(seq 64))

mkdir -p "$test_root/script" "$test_root/internal/contextindex"
cp "$source_root/script/analyzer-schema.sh" "$test_root/script/"
printf '#!/bin/sh\necho repinned >>repin.log\n' >"$test_root/script/repin-use-case-receipts.sh"
chmod +x "$test_root/script/repin-use-case-receipts.sh"
printf 'module example.test/fixture\n' >"$test_root/go.mod"
printf 'package contextindex\n\nconst analyzerSchemaID = "corvint-analyzer/7"\n' >"$test_root/internal/contextindex/analyzer_schema.go"
printf 'package contextindex\n\nfunc receipt() {}\n' >"$test_root/internal/contextindex/receipt.go"
cat >"$test_root/internal/contextindex/analyzer_schema_test.go" <<EOT
package contextindex

func TestAnalyzerSchemaInputs() {
		const auditedSchema = "corvint-analyzer/7"
		for _, directory := range []string{"contextindex"} {
		}
}

func TestAnalyzerSchemaOutput() {
		const auditedOutputSHA256 = "$digest_a"
}
EOT
printf 'repin.log\noutput.digest\n' >"$test_root/.gitignore"

git -C "$test_root" init -q -b main
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

CORVINT_ANALYZER_OUTPUT_COMMAND='cat output.digest'
export CORVINT_ANALYZER_OUTPUT_COMMAND
produce() { printf 'ok\nanalyzer output SHA256=%s\n' "$1" >"$test_root/output.digest"; }
run() {
    status=0
    output=$(cd "$test_root" && script/analyzer-schema.sh "$@" 2>&1) || status=$?
}
commit() { git -C "$test_root" add -A && git -C "$test_root" commit -q -m "$1"; }
pinned() { sed -n 's/^const analyzerSchemaID = "\(.*\)"$/\1/p' "$test_root/internal/contextindex/analyzer_schema.go"; }
output_pin() { sed -n 's/^[[:space:]]*const auditedOutputSHA256 = "\(.*\)"$/\1/p' "$test_root/internal/contextindex/analyzer_schema_test.go"; }

commit seed
produce "$digest_a"
run check --base main
[ "$status" -eq 0 ] || fail "clean seed failed check: $output"
run bump --base main
[ "$status" -eq 0 ] || fail "bump on a clean seed failed: $output"
case $output in *'nothing to bump'*) ;; *) fail "clean seed bumped: $output" ;; esac

# AC1 (V1-3574): two branches each edit receipt.go with an unchanged output; neither rewrites a
# pin, both pass check naming the changed input, and they merge without a conflict.
for side in left right; do
    git -C "$test_root" checkout -q -b "$side" main
    printf '// %s edit\n' "$side" >>"$test_root/internal/contextindex/receipt.go"
    run check --base main
    [ "$status" -eq 0 ] || fail "$side output-neutral edit refused: $output"
    case $output in *'analyzer inputs changed since'*'internal/contextindex/receipt.go'*) ;; *) fail "$side changed input not named: $output" ;; esac
    run bump --base main
    case $output in *'nothing to bump'*) ;; *) fail "$side output-neutral edit bumped: $output" ;; esac
    git -C "$test_root" diff --quiet -- internal/contextindex/analyzer_schema.go internal/contextindex/analyzer_schema_test.go || fail "$side rewrote a pin"
    commit "$side receipt edit"
done
git -C "$test_root" checkout -q left
git -C "$test_root" merge -q --no-edit right >/dev/null 2>&1 && fail "fixture receipt edits were expected to conflict only in receipt.go"
conflicted=$(git -C "$test_root" diff --name-only --diff-filter=U)
[ "$conflicted" = internal/contextindex/receipt.go ] || fail "merge conflicted outside receipt.go: $conflicted"
git -C "$test_root" merge --abort

# An output-changing edit bumps once; a second edit on the same branch keeps the number.
git -C "$test_root" checkout -q -b feature main
printf '// new facts\n' >>"$test_root/internal/contextindex/receipt.go"
produce "$digest_b"
run bump --base main
[ "$status" -eq 0 ] || fail "output bump failed: $output"
[ "$(pinned)" = corvint-analyzer/8 ] || fail "output change did not move 7 -> 8: $(pinned)"
[ "$(output_pin)" = "$digest_b" ] || fail "output pin not rewritten: $(output_pin)"
grep -q 'const auditedSchema = "corvint-analyzer/8"' "$test_root/internal/contextindex/analyzer_schema_test.go" || fail "test schema pin not rewritten"
[ -s "$test_root/repin.log" ] || fail "bump did not repin use-case receipts"
commit "feature bump"
printf '// more facts\n' >>"$test_root/internal/contextindex/receipt.go"
produce "$digest_c"
run bump --base main
[ "$(pinned)" = corvint-analyzer/8 ] || fail "second edit re-bumped the branch: $(pinned)"
run check --base main
[ "$status" -eq 0 ] || fail "check on a once-bumped branch failed: $output"
commit "feature second edit"

# A peer lands 8 with another output on main: the feature branch reuses 8 and is refused.
git -C "$test_root" checkout -q main
printf '// peer facts\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
produce "$digest_b"
run bump --base main
[ "$(pinned)" = corvint-analyzer/8 ] || fail "peer bump did not move 7 -> 8: $(pinned)"
commit "peer bump"
git -C "$test_root" checkout -q feature
run check --base main
[ "$status" -eq 1 ] || fail "colliding schema number passed check: $output"
case $output in *'Rebump to corvint-analyzer/9'*) ;; *) fail "collision refusal does not name the rebump: $output" ;; esac
produce "$digest_c"
run bump --base main
[ "$(pinned)" = corvint-analyzer/9 ] || fail "bump did not move past the base number: $(pinned)"
run check --base main
[ "$status" -eq 0 ] || fail "check after rebump failed: $output"

# A perf-only edit whose output equals the base's keeps the base number.
git -C "$test_root" checkout -q -f main
git -C "$test_root" checkout -q -b perf
printf '// faster, same output\n' >>"$test_root/internal/contextindex/receipt.go"
produce "$digest_b"
run bump --base main
[ "$(pinned)" = corvint-analyzer/8 ] || fail "perf-only edit bumped: $(pinned)"
[ "$(output_pin)" = "$digest_b" ] || fail "perf bump moved the output pin: $(output_pin)"

# A branch that changes nothing analyzer-related is never refused, even behind a newer base.
git -C "$test_root" checkout -q -f main
git -C "$test_root" checkout -q -b docs-only HEAD~1
printf 'notes\n' >"$test_root/NOTES"
commit "docs only"
run check --base main
[ "$status" -eq 0 ] || fail "branch with unchanged pins refused: $output"

# No base: the collision check is reported NOT_RUN and bump refuses to guess.
run check --base no-such-ref
[ "$status" -eq 0 ] || fail "unresolvable base failed check: $output"
case $output in *'NOT_RUN'*) ;; *) fail "unresolvable base not reported NOT_RUN: $output" ;; esac
run bump --base no-such-ref
[ "$status" -eq 1 ] || fail "bump without a base did not refuse"

# An output command that reports no digest makes bump refuse instead of guessing.
printf 'build failed\n' >"$test_root/output.digest"
run bump --base main
[ "$status" -ne 0 ] || fail "bump without an output digest did not refuse"
case $output in *'reported no output digest'*) ;; *) fail "missing output digest not reported: $output" ;; esac

printf 'ok analyzer-schema\n'
