#!/bin/sh
set -eu

# Fixture cases for script/analyzer-schema.sh (V1-2130, V1-1732; IDX-SNAP-V0-060, IDX-SNAP-V0-061):
# a clean tree checks; an input edit is refused until one `bump` rewrites both pins; a second edit
# on the same branch keeps the branch's number; a base that took the same number with another
# digest is refused with the rebump recovery and `bump` moves past it; an unresolvable base leaves
# the collision check NOT_RUN and makes `bump` refuse. With an output pin (V1-2289,
# IDX-SNAP-V0-107) a perf-only edit keeps the base number, a changed output bumps, the base number
# with another output digest is refused, and a missing output digest makes `bump` refuse. The
# fixture's digest must also equal the one TestAnalyzerSchemaInputs computes, which the real
# repository's pin proves on every gate.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-analyzer-schema-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

mkdir -p "$test_root/script" "$test_root/internal/contextindex"
cp "$source_root/script/analyzer-schema.sh" "$test_root/script/"
printf '#!/bin/sh\necho repinned >>repin.log\n' >"$test_root/script/repin-use-case-receipts.sh"
chmod +x "$test_root/script/repin-use-case-receipts.sh"
printf 'module example.test/fixture\n' >"$test_root/go.mod"
printf 'package contextindex\n\nconst analyzerSchemaID = "corvint-analyzer/7"\n' >"$test_root/internal/contextindex/analyzer_schema.go"
cat >"$test_root/internal/contextindex/analyzer_schema_test.go" <<'EOF'
package contextindex

func TestAnalyzerSchemaInputs() {
		const auditedSchema = "corvint-analyzer/7"
		const auditedSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		for _, directory := range []string{"contextindex"} {
		}
}
EOF
printf 'repin.log\n' >"$test_root/.gitignore"

git -C "$test_root" init -q -b main
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

run() {
    status=0
    output=$(cd "$test_root" && script/analyzer-schema.sh "$@" 2>&1) || status=$?
}
commit() { git -C "$test_root" add -A && git -C "$test_root" commit -q -m "$1"; }
pinned() { sed -n 's/^const analyzerSchemaID = "\(.*\)"$/\1/p' "$test_root/internal/contextindex/analyzer_schema.go"; }

# The placeholder digest is stale: check refuses, bump against main (no other bump) keeps 7 -> 8.
commit seed
run check --base main
[ "$status" -eq 1 ] || fail "stale placeholder digest passed check"
case $output in *'run script/analyzer-schema.sh bump'*) ;; *) fail "drift refusal does not name the recovery: $output" ;; esac
run bump --base main
[ "$status" -eq 0 ] || fail "bump on a stale seed failed: $output"
[ "$(pinned)" = corvint-analyzer/8 ] || fail "seed bump did not move 7 -> 8: $(pinned)"
grep -q 'const auditedSchema = "corvint-analyzer/8"' "$test_root/internal/contextindex/analyzer_schema_test.go" || fail "test schema pin not rewritten"
[ -s "$test_root/repin.log" ] || fail "bump did not repin use-case receipts"
run check --base main
[ "$status" -eq 0 ] || fail "check after bump failed: $output"
commit "bump to 8"

# A branch edits an input: one bump to 9; a second edit on the same branch keeps 9.
git -C "$test_root" checkout -q -b feature
printf '// edit one\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
run check --base main
[ "$status" -eq 1 ] || fail "input edit passed check"
run bump --base main
[ "$(pinned)" = corvint-analyzer/9 ] || fail "branch bump did not move 8 -> 9: $(pinned)"
commit "feature bump"
printf '// edit two\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
run bump --base main
[ "$(pinned)" = corvint-analyzer/9 ] || fail "second edit re-bumped the branch: $(pinned)"
run check --base main
[ "$status" -eq 0 ] || fail "check on a once-bumped branch failed: $output"
commit "feature second edit"

# A peer branch lands 9 with another digest on main: the feature branch reuses 9 and is refused.
git -C "$test_root" checkout -q main
printf '// peer edit\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
run bump --base main
[ "$(pinned)" = corvint-analyzer/9 ] || fail "peer bump did not move 8 -> 9: $(pinned)"
commit "peer bump"
git -C "$test_root" checkout -q feature
run check --base main
[ "$status" -eq 1 ] || fail "colliding schema number passed check: $output"
case $output in *'Rebump to corvint-analyzer/10'*) ;; *) fail "collision refusal does not name the rebump: $output" ;; esac
run bump --base main
[ "$(pinned)" = corvint-analyzer/10 ] || fail "bump did not move past the base number: $(pinned)"
run check --base main
[ "$status" -eq 0 ] || fail "check after rebump failed: $output"

# A branch that changes nothing analyzer-related is never refused, even behind a newer base.
git -C "$test_root" checkout -q -f main
git -C "$test_root" checkout -q -b docs-only HEAD~1
printf 'notes\n' >"$test_root/NOTES"
commit "docs only"
run check --base main
[ "$status" -eq 0 ] || fail "branch with unchanged pins refused: $output"

# No base: the collision check is reported NOT_RUN and bump refuses to guess.
run check --base no-such-ref
[ "$status" -eq 0 ] || fail "unresolvable base failed a clean check: $output"
case $output in *'NOT_RUN'*) ;; *) fail "unresolvable base not reported NOT_RUN: $output" ;; esac
run bump --base no-such-ref
[ "$status" -eq 1 ] || fail "bump without a base did not refuse"

# Output pin (V1-2289, IDX-SNAP-V0-107). The fixture's output command reads output.digest, which
# stands in for the digest TestAnalyzerSchemaOutput prints for the golden corpus.
digest_a=$(printf 'a%.0s' $(seq 64))
digest_b=$(printf 'b%.0s' $(seq 64))
produce() { printf 'ok\nanalyzer output SHA256=%s\n' "$1" >"$test_root/output.digest"; }
output_pin() { sed -n 's/^[[:space:]]*const auditedOutputSHA256 = "\(.*\)"$/\1/p' "$test_root/internal/contextindex/analyzer_schema_test.go"; }
CORVINT_ANALYZER_OUTPUT_COMMAND='cat output.digest'
export CORVINT_ANALYZER_OUTPUT_COMMAND
git -C "$test_root" checkout -q -f main
printf 'output.digest\n' >>"$test_root/.gitignore"
perl -0pi -e "s/^(\\s*const auditedSHA256 = \"[0-9a-f]{64}\"\\n)/\${1}\\t\\tconst auditedOutputSHA256 = \"$digest_a\"\\n/m or die" "$test_root/internal/contextindex/analyzer_schema_test.go"
commit "pin output"
produce "$digest_a"
base_number=$(pinned)

# A perf-only input edit whose golden output is unchanged keeps the base number.
git -C "$test_root" checkout -q -b perf
printf '// faster, same output\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
run check --base main
[ "$status" -eq 1 ] || fail "perf edit with a stale input digest passed check"
run bump --base main
[ "$status" -eq 0 ] || fail "perf bump failed: $output"
[ "$(pinned)" = "$base_number" ] || fail "perf-only edit bumped $base_number -> $(pinned)"
[ "$(output_pin)" = "$digest_a" ] || fail "perf bump moved the output pin: $(output_pin)"
run check --base main
[ "$status" -eq 0 ] || fail "check refused a perf-only edit with the same output: $output"
case $output in *'no bump needed'*) ;; *) fail "perf-only check did not report the output equivalence: $output" ;; esac

# Claiming the base number with another output digest is refused.
perl -0pi -e "s/$digest_a/$digest_b/ or die" "$test_root/internal/contextindex/analyzer_schema_test.go"
run check --base main
[ "$status" -eq 1 ] || fail "same number with a different output digest passed check: $output"
case $output in *'Rebump to corvint-analyzer/'*) ;; *) fail "output collision refusal does not name the rebump: $output" ;; esac

# An input edit that changes the output bumps once and pins the new output digest.
git -C "$test_root" checkout -q -f main
git -C "$test_root" checkout -q -b output-change
printf '// new facts\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
produce "$digest_b"
run bump --base main
[ "$status" -eq 0 ] || fail "output-changing bump failed: $output"
[ "$(pinned)" = "corvint-analyzer/$((${base_number#corvint-analyzer/} + 1))" ] || fail "output change did not bump past $base_number: $(pinned)"
[ "$(output_pin)" = "$digest_b" ] || fail "output change did not repin the output digest: $(output_pin)"
run check --base main
[ "$status" -eq 0 ] || fail "check after an output bump failed: $output"

# An output command that reports no digest makes bump refuse instead of guessing.
printf '// another edit\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
printf 'build failed\n' >"$test_root/output.digest"
run bump --base main
[ "$status" -ne 0 ] || fail "bump without an output digest did not refuse"
case $output in *'reported no output digest'*) ;; *) fail "missing output digest not reported: $output" ;; esac

printf 'ok analyzer-schema\n'
