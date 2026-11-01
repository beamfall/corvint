#!/bin/sh
set -eu

# Fixture cases for script/analyzer-schema.sh (V1-2130, V1-1732, V1-2289, V1-3574, V1-2976,
# V1-2356; IDX-SNAP-V0-060, IDX-SNAP-V0-061, IDX-SNAP-V0-107, IDX-SNAP-V0-890 to IDX-SNAP-V0-892).
# The pins are the schema number and the golden output (pack and shard digests). An input edit
# with unchanged output moves no pin, so two such branches merge without a conflict; check notes the
# changed input and check --output decides; an output or shard change bumps once; a second change on
# the branch keeps its number; a base that took the same number with other output is refused with
# the rebump recovery and bump moves past it; a base from before the shard pin compares the pack
# only; an unresolvable base leaves the collision check NOT_RUN and makes bump refuse; a missing
# digest refuses; the default output command runs through script/heavy.sh.

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
shard_1=$(printf '1%.0s' $(seq 64))
shard_2=$(printf '2%.0s' $(seq 64))
schema_test=$test_root/internal/contextindex/analyzer_schema_test.go

mkdir -p "$test_root/script" "$test_root/internal/contextindex"
cp "$source_root/script/analyzer-schema.sh" "$source_root/script/analyzer-inputs.sh" "$test_root/script/"
printf '#!/bin/sh\necho repinned >>repin.log\n' >"$test_root/script/repin-use-case-receipts.sh"
chmod +x "$test_root/script/repin-use-case-receipts.sh"
printf 'module example.test/fixture\n' >"$test_root/go.mod"
printf 'package contextindex\n\nconst analyzerSchemaID = "corvint-analyzer/7"\n' >"$test_root/internal/contextindex/analyzer_schema.go"
printf 'package contextindex\n\nfunc One() {}\n\n\n\n\nfunc Two() {}\n' >"$test_root/internal/contextindex/receipt.go"
cat >"$schema_test" <<EOF
package contextindex

func TestAnalyzerSchemaInputs() {
		for _, directory := range []string{"contextindex"} {
		}
}

func TestAnalyzerSchemaOutput() {
		const auditedSchema = "corvint-analyzer/7"
		const auditedOutputSHA256 = "$digest_a"
		const auditedShardSHA256 = "$shard_1"
}
EOF
printf 'repin.log\noutput.digest\n' >"$test_root/.gitignore"

git -C "$test_root" init -q -b main
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

run() {
    status=0
    output=$(cd "$test_root" && script/analyzer-schema.sh "$@" 2>&1) || status=$?
}
commit() { git -C "$test_root" add -A && git -C "$test_root" commit -q -m "$1"; }
pinned() { sed -n 's/^const analyzerSchemaID = "\(.*\)"$/\1/p' "$test_root/internal/contextindex/analyzer_schema.go"; }
output_pin() { sed -n 's/^[[:space:]]*const auditedOutputSHA256 = "\(.*\)"$/\1/p' "$schema_test"; }
shard_pin() { sed -n 's/^[[:space:]]*const auditedShardSHA256 = "\(.*\)"$/\1/p' "$schema_test"; }
# The fixture's output command reads output.digest, which stands in for the two digests
# TestAnalyzerSchemaOutput prints for the golden corpus.
produce() { printf 'ok\nanalyzer output SHA256=%s\nanalyzer shard SHA256=%s\n' "$1" "$2" >"$test_root/output.digest"; }
CORVINT_ANALYZER_OUTPUT_COMMAND='cat output.digest'
export CORVINT_ANALYZER_OUTPUT_COMMAND

# A seed whose pins match its output checks, and bump finds nothing to do.
commit seed
produce "$digest_a" "$shard_1"
run check --output --base main
[ "$status" -eq 0 ] || fail "matching seed failed check --output: $output"
run bump --base main
[ "$status" -eq 0 ] || fail "bump on a matching seed failed: $output"
case $output in *'nothing to bump'*) ;; *) fail "matching seed bump: $output" ;; esac
[ -s "$test_root/repin.log" ] || fail "bump did not repin use-case receipts"
rm -f "$test_root/repin.log"

# IDX-SNAP-V0-890 (V1-3574): two branches that each edit receipt.go with unchanged output move no
# pin, check notes the changed input, and the branches merge without a conflict.
git -C "$test_root" checkout -q -b edit-one
sed 's/^func One() {}$/func One() { _ = 1 }/' "$test_root/internal/contextindex/receipt.go" >"$test_root/r" && mv "$test_root/r" "$test_root/internal/contextindex/receipt.go"
run check --base main
[ "$status" -eq 0 ] || fail "output-neutral input edit failed check: $output"
case $output in *'NOTE 1 analyzer input(s) changed since '*', first internal/contextindex/receipt.go: '*'check --output decides'*) ;; *) fail "input edit not noted: $output" ;; esac
run check --output --base main
[ "$status" -eq 0 ] || fail "output-neutral input edit failed check --output: $output"
run bump --base main
case $output in *'nothing to bump'*) ;; *) fail "output-neutral bump moved a pin: $output" ;; esac
git -C "$test_root" diff --quiet -- internal/contextindex/analyzer_schema.go internal/contextindex/analyzer_schema_test.go || fail "output-neutral bump rewrote a pin file"
commit "edit one"
git -C "$test_root" checkout -q -b edit-two main
sed 's/^func Two() {}$/func Two() { _ = 2 }/' "$test_root/internal/contextindex/receipt.go" >"$test_root/r" && mv "$test_root/r" "$test_root/internal/contextindex/receipt.go"
commit "edit two"
git -C "$test_root" merge -q --no-edit edit-one >/dev/null 2>&1 || fail "two output-neutral receipt.go edits conflicted: $(git -C "$test_root" diff --name-only --diff-filter=U)"
run check --output --base main
[ "$status" -eq 0 ] || fail "merged output-neutral edits failed check --output: $output"

# An output change is refused by check --output naming the recovery; one bump moves 7 -> 8 and
# pins the new output; a second change on the branch keeps 8.
git -C "$test_root" checkout -q -b output-change main
printf '// new facts\n' >>"$test_root/internal/contextindex/receipt.go"
produce "$digest_b" "$shard_1"
run check --base main
[ "$status" -eq 0 ] || fail "static check refused an unpinned output change: $output"
run check --output --base main
[ "$status" -eq 1 ] || fail "output change passed check --output: $output"
case $output in *'drifted from its pins'*'run script/analyzer-schema.sh bump'*) ;; *) fail "output drift does not name the recovery: $output" ;; esac
run bump --base main
[ "$status" -eq 0 ] || fail "output-changing bump failed: $output"
[ "$(pinned)" = corvint-analyzer/8 ] || fail "output change did not bump 7 -> 8: $(pinned)"
[ "$(output_pin)" = "$digest_b" ] || fail "output change did not repin the output digest: $(output_pin)"
grep -q 'const auditedSchema = "corvint-analyzer/8"' "$schema_test" || fail "test schema pin not rewritten"
run check --output --base main
[ "$status" -eq 0 ] || fail "check after an output bump failed: $output"
commit "output bump"
printf '// more facts\n' >>"$test_root/internal/contextindex/receipt.go"
produce "$digest_c" "$shard_1"
run bump --base main
[ "$(pinned)" = corvint-analyzer/8 ] || fail "second output change re-bumped the branch: $(pinned)"
commit "output second change"

# IDX-SNAP-V0-892: a shard-only change bumps too.
git -C "$test_root" checkout -q -b shard-change main
printf '// shard facts\n' >>"$test_root/internal/contextindex/receipt.go"
produce "$digest_a" "$shard_2"
run check --output --base main
[ "$status" -eq 1 ] || fail "shard-only change passed check --output: $output"
run bump --base main
[ "$(pinned)" = corvint-analyzer/8 ] && [ "$(shard_pin)" = "$shard_2" ] || fail "shard-only change: $(pinned) $(shard_pin)"
git -C "$test_root" checkout -q -f main

# A peer branch lands 8 with output b on main: the output-change branch (8, output c) is refused
# with the rebump recovery, and bump moves past it.
git -C "$test_root" checkout -q -b peer main
printf '// peer facts\n' >>"$test_root/internal/contextindex/analyzer_schema.go"
produce "$digest_b" "$shard_1"
run bump --base main
commit "peer bump"
git -C "$test_root" checkout -q main
git -C "$test_root" merge -q --ff-only peer
git -C "$test_root" checkout -q output-change
produce "$digest_c" "$shard_1"
run check --base main
[ "$status" -eq 1 ] || fail "colliding schema number passed check: $output"
case $output in *'Rebump to corvint-analyzer/9'*) ;; *) fail "collision refusal does not name the rebump: $output" ;; esac
run bump --base main
[ "$(pinned)" = corvint-analyzer/9 ] || fail "bump did not move past the base number: $(pinned)"
run check --base main
[ "$status" -eq 0 ] || fail "check after rebump failed: $output"
git -C "$test_root" checkout -q -f main

# A branch that changes nothing analyzer-related is never refused, even behind a newer base.
git -C "$test_root" checkout -q -b docs-only HEAD~1
printf 'notes\n' >"$test_root/NOTES"
commit "docs only"
run check --base main
[ "$status" -eq 0 ] || fail "branch with unchanged pins refused: $output"
case $output in *NOTE*) fail "docs-only branch noted an input change: $output" ;; esac
git -C "$test_root" checkout -q -f main

# A base from before the shard pin compares the pack only: adding the shard pin needs no bump.
git -C "$test_root" checkout -q -b old-base main
perl -0pi -e 's/^\s*const auditedShardSHA256 = "[0-9a-f]{64}"\n//m or die' "$schema_test"
commit "drop the shard pin"
git -C "$test_root" checkout -q -b add-shard old-base
perl -0pi -e "s/(const auditedOutputSHA256 = \"[0-9a-f]{64}\"\\n)/\${1}\\t\\tconst auditedShardSHA256 = \"$shard_1\"\\n/ or die" "$schema_test"
produce "$digest_b" "$shard_1"
run check --output --base old-base
[ "$status" -eq 0 ] || fail "adding the shard pin to an unchanged pack was refused: $output"
case $output in *'no bump needed'*) ;; *) fail "shard-pin addition not reported equivalent: $output" ;; esac
git -C "$test_root" checkout -q -f main

# No base: the collision check is reported NOT_RUN and bump refuses to guess.
produce "$digest_b" "$shard_1"
run check --base no-such-ref
[ "$status" -eq 0 ] || fail "unresolvable base failed a clean check: $output"
case $output in *'NOT_RUN'*) ;; *) fail "unresolvable base not reported NOT_RUN: $output" ;; esac
run bump --base no-such-ref
[ "$status" -eq 1 ] || fail "bump without a base did not refuse"

# An output command that reports no digest makes bump and check --output refuse.
printf 'build failed\n' >"$test_root/output.digest"
run bump --base main
[ "$status" -ne 0 ] || fail "bump without an output digest did not refuse"
case $output in *'reported no output digest'*) ;; *) fail "missing output digest not reported: $output" ;; esac
run check --output --base main
[ "$status" -ne 0 ] || fail "check --output without an output digest did not refuse"

# IDX-SNAP-V0-892 (V1-2356): the default output command runs the Go test through script/heavy.sh.
cat >"$test_root/script/heavy.sh" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>heavy.log
printf 'analyzer output SHA256=%s\nanalyzer shard SHA256=%s\n' "$digest_b" "$shard_1"
EOF
chmod +x "$test_root/script/heavy.sh"
printf 'heavy.log\n' >>"$test_root/.gitignore"
(
    unset CORVINT_ANALYZER_OUTPUT_COMMAND
    run check --output --base main
    [ "$status" -eq 0 ] || fail "check --output through heavy.sh failed: $output"
    grep -qx 'env GOTOOLCHAIN=local go test -count=1 -run ^TestAnalyzerSchemaOutput$ -v ./internal/contextindex/' "$test_root/heavy.log" ||
        fail "the output test did not run through script/heavy.sh: $(cat "$test_root/heavy.log" 2>&1)"
)

printf 'ok analyzer-schema\n'
