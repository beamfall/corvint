#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# PRS-V1-1900 (V1-0374): script/release-candidate-bytes.sh binds the linux-core archives to the
# dispatched candidate's SHA256SUMS digest, refuses any other bytes, says SOURCE_ONLY without one,
# and records commit, tree, archive, binary, runner and report digests; release-gates.yml validates
# the candidate_sums_sha256 input and passes it to the script in linux-core.
#
# The workflow binding lands in its own .github change (ASD-V0-706). Until it lands the workflow is
# byte-identical to the pre-V1-0374 file pinned below, and only then are the workflow cases skipped,
# with the reason printed. Any other workflow bytes must carry the whole binding, so the binding
# cannot go missing silently once both changes are in.
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
script=$source_root/script/release-candidate-bytes.sh
workflow=$source_root/.github/workflows/release-gates.yml
# sha256 of release-gates.yml before the V1-0374 .github change, repinned for the go1.27.4 setup-go
# pin (V1-5408); it was 35be7f1e at b55635262.
pre_binding_workflow_sha256=3a101693f17a9515e1456b54068f662b7cd33b22633431d53401fa84617bcd8d
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-candidate-bytes-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
# The fixture commit must not depend on the caller's Git configuration (signing, hooks).
GIT_CONFIG_GLOBAL=/dev/null
GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM
cases=0
ok() { cases=$((cases + 1)); }
fail() { printf 'release-candidate-bytes_test: FAIL %s\n' "$1" >&2; exit 1; }
sha256() {
  local out
  if command -v sha256sum > /dev/null 2>&1; then out=$(sha256sum < "$1"); else out=$(shasum -a 256 < "$1"); fi
  printf '%s\n' "${out%% *}"
}

repo=$tmp/repo
git init -q "$repo"
printf 'source\n' > "$repo/a"
git -C "$repo" add a
git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -qm candidate
commit=$(git -C "$repo" rev-parse HEAD)
tree=$(git -C "$repo" rev-parse 'HEAD^{tree}')
core=$tmp/core
mkdir -p "$core"
printf 'linux archive bytes\n' > "$core/corvint_linux_amd64.tar.gz"
printf 'darwin archive bytes\n' > "$core/corvint_darwin_arm64.tar.gz"
(cd "$core" && for f in corvint_darwin_arm64.tar.gz corvint_linux_amd64.tar.gz; do printf '%s  %s\n' "$(sha256 "$f")" "$f"; done) > "$core/SHA256SUMS"
printf 'corvint binary\n' > "$tmp/corvint"
printf 'SUMMARY\tstatus=PASS\n' > "$tmp/cli.txt"
sums_sha=$(sha256 "$core/SHA256SUMS")
upper=$(printf '%s\n' "$sums_sha" | tr a-f A-F)
nl='
'
two_lines=$sums_sha$nl$sums_sha
export RUNNER_LABEL=ubuntu-24.04 ImageVersion=fixture-image GITHUB_SERVER_URL=https://example.invalid
export GITHUB_REPOSITORY=beamfall/corvint GITHUB_RUN_ID=7 GITHUB_RUN_ATTEMPT=2

run() { (cd "$repo" && "$script" "$@") > "$tmp/out" 2> "$tmp/err"; }

# Without --expect: the full identity record and SOURCE_ONLY, never CANDIDATE_BYTES.
run --sums "$core/SHA256SUMS" --binary "$tmp/corvint" || fail "SOURCE_ONLY run failed: $(cat "$tmp/err")"
for line in "commit=$commit" "tree=$tree" "sha256sums_sha256=$sums_sha" \
  "archive=corvint_linux_amd64.tar.gz sha256=$(sha256 "$core/corvint_linux_amd64.tar.gz")" \
  "binary_sha256=$(sha256 "$tmp/corvint")" 'runner=ubuntu-24.04 image=fixture-image' \
  'run=https://example.invalid/beamfall/corvint/actions/runs/7/attempts/2'; do
  grep -Fqx "$line" "$tmp/out" || fail "identity record lacks '$line': $(cat "$tmp/out")"
done
tail -n 1 "$tmp/out" | grep -q '^verdict=SOURCE_ONLY ' || fail "no SOURCE_ONLY verdict: $(cat "$tmp/out")"
if grep -q CANDIDATE_BYTES "$tmp/out"; then fail "a run without --expect said CANDIDATE_BYTES: $(cat "$tmp/out")"; fi
ok

# The candidate's digest: CANDIDATE_BYTES, with the identity record and each report's digest.
run --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha" --report "$tmp/cli.txt" ||
  fail "matching candidate refused: $(cat "$tmp/err")"
for line in "commit=$commit" "tree=$tree" "sha256sums_sha256=$sums_sha" \
  "archive=corvint_linux_amd64.tar.gz sha256=$(sha256 "$core/corvint_linux_amd64.tar.gz")" \
  "binary_sha256=$(sha256 "$tmp/corvint")" 'runner=ubuntu-24.04 image=fixture-image' \
  'run=https://example.invalid/beamfall/corvint/actions/runs/7/attempts/2' \
  "report $(sha256 "$tmp/cli.txt") $tmp/cli.txt"; do
  grep -Fqx "$line" "$tmp/out" || fail "CANDIDATE_BYTES record lacks '$line': $(cat "$tmp/out")"
done
tail -n 1 "$tmp/out" | grep -q "^verdict=CANDIDATE_BYTES .*$sums_sha" || fail "no CANDIDATE_BYTES verdict: $(cat "$tmp/out")"
ok

# Another digest: exit 1, NOT_CANDIDATE, and the refusal names both digests and its recovery.
other=$(printf '%064d' 0)
rc=0; run --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$other" || rc=$?
[ "$rc" = 1 ] || fail "a different candidate digest exited $rc, want 1"
grep -Fqx 'verdict=NOT_CANDIDATE' "$tmp/out" || fail "no NOT_CANDIDATE verdict: $(cat "$tmp/out")"
if grep -q CANDIDATE_BYTES "$tmp/out"; then fail "a different digest said CANDIDATE_BYTES: $(cat "$tmp/out")"; fi
grep -Fq "REFUSE candidate-mismatch: the rebuilt SHA256SUMS is $sums_sha, the candidate's is $other" "$tmp/err" ||
  fail "mismatch refusal does not name both digests: $(cat "$tmp/err")"
grep -Fq '; fix: ' "$tmp/err" || fail "mismatch refusal names no recovery (ASD-V0-997): $(cat "$tmp/err")"
ok

# Refusals: each exits as wanted, names its code and its recovery (ASD-V0-997), and never says
# CANDIDATE_BYTES.
expect_refusal() {
  local want_rc=$1 code=$2 rc=0
  shift 2
  run "$@" || rc=$?
  [ "$rc" = "$want_rc" ] && grep -Fq "REFUSE $code" "$tmp/err" || fail "want exit $want_rc REFUSE $code for $*, got $rc: $(cat "$tmp/err")"
  grep -Fq "REFUSE $code" "$tmp/err" && grep -Fq '; fix: ' "$tmp/err" || fail "REFUSE $code names no recovery (ASD-V0-997): $(cat "$tmp/err")"
  if grep -q CANDIDATE_BYTES "$tmp/out"; then fail "REFUSE $code still said CANDIDATE_BYTES: $(cat "$tmp/out")"; fi
  ok
}
expect_refusal 2 expect-malformed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$upper"
expect_refusal 2 expect-malformed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "${sums_sha:0:63}"
expect_refusal 2 expect-malformed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha$sums_sha"
expect_refusal 2 expect-malformed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$two_lines"
expect_refusal 2 expect-malformed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha$nl"
expect_refusal 2 sums-unreadable --sums "$tmp/absent/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha"
expect_refusal 2 report-missing --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --report "$tmp/absent.txt"
expect_refusal 2 binary-unreadable --sums "$core/SHA256SUMS" --binary "$tmp/absent"
cp "$core/corvint_linux_amd64.tar.gz" "$tmp/linux.orig"
printf 'tampered\n' >> "$core/corvint_linux_amd64.tar.gz"
expect_refusal 1 archive-row-mismatch --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha"
expect_refusal 1 archive-row-mismatch --sums "$core/SHA256SUMS" --binary "$tmp/corvint"
cp "$tmp/linux.orig" "$core/corvint_linux_amd64.tar.gz"
mv "$core/corvint_linux_amd64.tar.gz" "$tmp/linux.moved"
expect_refusal 2 archive-missing --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha"
mv "$tmp/linux.moved" "$core/corvint_linux_amd64.tar.gz"
cp "$core/SHA256SUMS" "$tmp/sums.orig"
grep -v corvint_linux_amd64 "$tmp/sums.orig" > "$core/SHA256SUMS"
expect_refusal 2 archive-row-missing --sums "$core/SHA256SUMS" --binary "$tmp/corvint"
cp "$tmp/sums.orig" "$core/SHA256SUMS"
rc=0; "$script" --sums "$core/SHA256SUMS" > /dev/null 2>&1 || rc=$?
[ "$rc" = 2 ] || fail "a missing --binary exited $rc, want 2"
ok

# A hash that fails or prints no 64-hex digest refuses (exit 2) before any record line, so no
# digest is ever printed empty and no verdict is reached, SOURCE_ONLY included. A fake sha256sum
# first on PATH breaks the hash of one input, chosen by its content, and delegates the rest.
fake=$tmp/fakebin
mkdir -p "$fake"
if real=$(command -v sha256sum); then real_cmd="'$real'"; else real_cmd="'$(command -v shasum)' -a 256"; fi
cat > "$fake/sha256sum" <<EOF
#!/usr/bin/env bash
in=\$(mktemp '$tmp/fake-sha.XXXXXX')
cat > "\$in"
if grep -Fq -- "\$FAKE_SHA_MATCH" "\$in"; then
  rm -f "\$in"
  case \$FAKE_SHA_MODE in
    fail) exit 1 ;;
    garbage) printf 'not-a-digest  -\n' ;;
    short) printf 'abc123  -\n' ;;
  esac
  exit 0
fi
$real_cmd < "\$in"; rc=\$?; rm -f "\$in"; exit \$rc
EOF
chmod +x "$fake/sha256sum"
with_fake_sha() { (export FAKE_SHA_MODE=$1 FAKE_SHA_MATCH=$2 PATH="$fake:$PATH"; shift 2; run "$@"); }
# Control: the fake delegates every other input, so the refusals below are the broken hash's.
with_fake_sha fail no-such-content --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --report "$tmp/cli.txt" ||
  fail "the delegating fake sha256sum broke a clean run: $(cat "$tmp/err")"
tail -n 1 "$tmp/out" | grep -q '^verdict=SOURCE_ONLY ' || fail "no SOURCE_ONLY verdict with the fake: $(cat "$tmp/out")"
ok
hash_refusal() {
  local mode=$1 match=$2 code=$3 rc=0
  shift 3
  with_fake_sha "$mode" "$match" "$@" || rc=$?
  [ "$rc" = 2 ] && grep -Fq "REFUSE $code" "$tmp/err" || fail "want exit 2 REFUSE $code for a $mode hash of '$match', got $rc: $(cat "$tmp/err")"
  grep -Fq '; fix: ' "$tmp/err" || fail "REFUSE $code names no recovery (ASD-V0-997): $(cat "$tmp/err")"
  [ ! -s "$tmp/out" ] || fail "REFUSE $code still printed a record: $(cat "$tmp/out")"
  ok
}
for mode in fail garbage short; do
  hash_refusal "$mode" 'linux archive bytes' archive-hash-failed --sums "$core/SHA256SUMS" --binary "$tmp/corvint"
  hash_refusal "$mode" corvint_linux_amd64.tar.gz sums-hash-failed --sums "$core/SHA256SUMS" --binary "$tmp/corvint"
  hash_refusal "$mode" 'corvint binary' binary-hash-failed --sums "$core/SHA256SUMS" --binary "$tmp/corvint"
  hash_refusal "$mode" 'corvint binary' binary-hash-failed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --expect "$sums_sha"
  hash_refusal "$mode" SUMMARY report-hash-failed --sums "$core/SHA256SUMS" --binary "$tmp/corvint" --report "$tmp/cli.txt"
done

# The workflow: the dispatch input exists, Check inputs refuses a malformed digest and admits an
# empty or well-formed one, and linux-core passes it to this script and indexes its reports.
if [ "$(sha256 "$workflow")" = "$pre_binding_workflow_sha256" ]; then
  printf 'release-candidate-bytes_test: SKIP workflow cases: release-gates.yml is still the pre-V1-0374 file (sha256 %s); the .github change of V1-0374 that adds candidate_sums_sha256 and the linux-core bind and index steps has not landed\n' \
    "$pre_binding_workflow_sha256"
  printf 'release-candidate-bytes_test: PASS %s cases, workflow cases SKIPPED\n' "$cases"
  exit 0
fi
grep -Eq '^      candidate_sums_sha256:$' "$workflow" || fail "release-gates.yml has no candidate_sums_sha256 input"
grep -Fq 'CANDIDATE_SUMS_SHA256: ${{ inputs.candidate_sums_sha256 }}' "$workflow" || fail "the input is not in the workflow env"
ok
awk '/^      - &check-inputs$/ { step = 1; next } step && /^        run: \|$/ { copy = 1; next }
     copy && !/^          / { exit } copy { sub(/^          /, ""); print }' "$workflow" > "$tmp/check-inputs"
grep -q CANDIDATE_SUMS_SHA256 "$tmp/check-inputs" || fail "Check inputs does not validate the candidate digest: $(cat "$tmp/check-inputs")"
check_inputs() { SHA=$commit RELEASE=v1.0.0-rc.3 CANDIDATE_SUMS_SHA256=$1 bash -e "$tmp/check-inputs" > /dev/null 2>&1; }
check_inputs '' || fail "Check inputs refused an empty candidate digest"
check_inputs "$sums_sha" || fail "Check inputs refused a well-formed candidate digest"
ok
for bad in "$upper" "${sums_sha:0:63}" "$sums_sha$sums_sha" "$two_lines" "$sums_sha$nl"; do
  if check_inputs "$bad"; then fail "Check inputs admitted candidate digest '$bad'"; fi
  ok
done
awk '/^  linux-core:$/ { job = 1 } job && /^  [a-z-]+:$/ && !/^  linux-core:$/ { exit } job { print }' "$workflow" > "$tmp/linux-core"
# step NAME_PREFIX: one linux-core step, from its `- name:` line to the next step.
step() {
  awk -v want="      - name: $1" 'index($0, want) == 1 { copy = 1; print; next }
    copy && /^      - / { exit } copy { print }' "$tmp/linux-core"
}
call='script/release-candidate-bytes.sh --sums "$RUNNER_TEMP/release/core/SHA256SUMS" --binary "$RUNNER_TEMP/current/corvint" ${CANDIDATE_SUMS_SHA256:+--expect "$CANDIDATE_SUMS_SHA256"}'
step 'Bind the archives to the release candidate' > "$tmp/bind"
grep -Fqx '        id: bind' "$tmp/bind" || fail "linux-core has no bind step: $(cat "$tmp/linux-core")"
grep -Fq "\"\$RUNNER_TEMP/run-gate\" candidate-identity $call || code=\$?" "$tmp/bind" ||
  fail "the bind step does not run script/release-candidate-bytes.sh with the candidate digest as --expect: $(cat "$tmp/bind")"
grep -Fqx '          exit "$code"' "$tmp/bind" || fail "the bind step does not fail the job with the script's exit code: $(cat "$tmp/bind")"
ok
step 'Index the retained reports' > "$tmp/index"
grep -Fq "\"\$RUNNER_TEMP/run-gate\" report-index $call \"\${reports[@]}\"" "$tmp/index" ||
  fail "the index step does not run script/release-candidate-bytes.sh with the candidate digest: $(cat "$tmp/index")"
grep -Fq -- 'reports+=(--report "$report")' "$tmp/index" || fail "the index step does not index the retained reports: $(cat "$tmp/index")"
ok

printf 'release-candidate-bytes_test: PASS %s cases\n' "$cases"
