#!/usr/bin/env bash
# Tests for script/batch-precheck.sh (ASD-V0-029, V1-2551; ASD-V0-032, V1-2895; ASD-V0-034 to 036,
# V1-2899/V1-2900) and script/batch-regen.sh (ASD-V0-033): the sync check against this checkout's
# CI workflow and against fixtures, the doc-gate steps derived from the fixture workflow, the
# parallel runner and its table on stub make/go/gofmt commands, the HEAD-bound receipt and its push
# guard, the regen commit, its idempotence and its refusals, the --base classification, and the
# ledger reuse of ASD-V0-042 (V1-2985), the drift and catalog selection of ASD-V0-050 and
# ASD-V0-051 (V1-3134, V1-3136), the one rerun of a failed step (ASD-V0-046, V1-3122) and the
# sealed CEM name check (ASD-V0-109, V1-3267).
# ASD-V0-923 (V1-3341): only bash outside POSIX mode runs this file. Under `sh` (dash, or bash in
# POSIX mode as macOS sh is) a `VAR=x func` assignment persists after the call and case 3 failed
# falsely, so this POSIX-parseable guard refuses first.
if [ -z "${BASH_VERSION-}" ] || case :${SHELLOPTS-}: in *:posix:*) true ;; *) false ;; esac; then
  printf 'batch-precheck_test: REFUSE not-bash: run it as script/batch-precheck_test.sh or with bash, not sh\n' >&2
  exit 2
fi
set -uo pipefail
export LC_ALL=C
script=$(cd "$(dirname "$0")" && pwd)
pre=$script/batch-precheck.sh
regen=$script/batch-regen.sh
# ASD-V0-894 (V1-3387): the test strips a trailing slash from TMPDIR itself (macOS's default ends in
# `/`), so every path the precheck and its steps derive from it matches the resolved scratch path.
while [[ ${TMPDIR-} == ?*/ ]]; do TMPDIR=${TMPDIR%/}; done
[[ -z ${TMPDIR-} ]] || export TMPDIR
# ASD-V0-923: the guard above refuses dash and bash in POSIX mode.
for sh_mode in sh 'bash --posix'; do
  command -v "${sh_mode%% *}" > /dev/null 2>&1 || continue
  out=$($sh_mode "$script/batch-precheck_test.sh" 2>&1)
  rc=$?
  [[ $rc == 2 && $out == *'REFUSE not-bash'* ]] || { printf 'batch-precheck_test: FAIL %s did not refuse (exit %s): %s\n' "$sh_mode" "$rc" "$out" >&2; exit 1; }
done
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-batch-precheck-test.XXXXXX") ||
  { printf 'batch-precheck_test: FAIL mktemp under %s: %s\n' "${TMPDIR:-/tmp}" "$(df -Pk "${TMPDIR:-/tmp}" 2>&1 | tail -n 1)" >&2; exit 1; }
# batch-precheck resolves the step TMPDIR with `cd && pwd`; macOS TMPDIR ends in `/`, so the
# mktemp path holds `//` until it is resolved the same way (V1-3055).
tmp=$(cd "$tmp" && pwd)
# V1-3205: every failure names its case (the numbered group and the failing line), the last output
# of the command under test and the free space of TMPDIR, whose exhaustion once failed this test
# with an empty log; an exit that bypassed fail says so.
# ASD-V0-780 (V1-3858): fail records the case in $tmp/failures (so a failure inside a subshell
# counts too) and returns; the run continues and finish lists every failed case and exits 1.
failed=0
# ASD-V0-913 (V1-3805): all scratch lives under TMPDIR. A TMPDIR below the floor, or a failure whose
# output names ENOSPC, is a host-resource refusal (exit 3) naming the directory and the KiB needed,
# not an ordinary FAIL: a nearly full /tmp tmpfs once failed group 9 whatever the change.
need_kib=${BATCH_PRECHECK_TEST_MIN_KIB:-65536}
tmp_avail() { df -Pk "${TMPDIR:-/tmp}" 2> /dev/null | awk 'NR == 2 { print $4 }'; }
host_refuse() {
  failed=1
  printf 'batch-precheck_test: REFUSE host-resource: TMPDIR %s has %sKiB free; this test needs at least %sKiB there (%s): free space or set TMPDIR to a directory with room\n' \
    "${TMPDIR:-/tmp}" "$(tmp_avail)" "$need_kib" "$1" >&2
  exit 3
}
fail() {
  local line=${BASH_LINENO[0]} group
  group=$(awk -v n="$line" 'NR > n { exit } /^# [0-9]+[a-z]?\. / { g = $0 } END { print substr(g, 3, 70) }' "$script/batch-precheck_test.sh")
  failed=1
  if [[ -z ${BATCH_PRECHECK_TEST_NO_HOST_CHECK:-} ]] && { [[ $* == *[Nn]'o space left on device'* ]] || grep -qi 'no space left on device' "$tmp/out" 2> /dev/null; }; then
    avail=$(tmp_avail)
    [[ $avail =~ ^[0-9]+$ ]] && ((avail < need_kib)) && host_refuse "line $line hit no space left on device"
  fi
  printf 'case "%s" line %s: %s\n' "$group" "$line" "$*" >>"$tmp/failures"
  {
    printf 'batch-precheck_test: FAIL case "%s" line %s: %s\n' "$group" "$line" "$*"
    [[ -s $tmp/out ]] && { printf 'batch-precheck_test: last output (%s), last 40 lines:\n' "$tmp/out"; tail -n 40 "$tmp/out" | sed 's/^/  | /'; }
    printf 'batch-precheck_test: TMPDIR %s: %s\n' "${TMPDIR:-/tmp}" "$(df -Pk "${TMPDIR:-/tmp}" 2>&1 | tail -n 1)"
  } >&2
  return 1
}
finish() {
  if [[ -s $tmp/failures ]]; then
    failed=1
    printf 'batch-precheck_test: FAIL %s case(s):\n' "$(wc -l <"$tmp/failures" | tr -d ' ')" >&2
    sed 's/^/  - /' "$tmp/failures" >&2
    exit 1
  fi
  printf 'batch-precheck_test: PASS\n'
  exit 0
}
on_exit() {
  local rc=$?
  [[ -s $tmp/failures ]] && failed=1
  ((rc == 0 || failed)) || printf 'batch-precheck_test: FAIL exit %s without a FAIL line (TMPDIR %s: %s)\n' "$rc" "${TMPDIR:-/tmp}" "$(df -Pk "${TMPDIR:-/tmp}" 2>&1 | tail -n 1)" >&2
  /bin/rm -rf "$tmp"
}
trap on_exit EXIT
avail=$(tmp_avail)
[[ $avail =~ ^[0-9]+$ ]] && ((avail < need_kib)) && host_refuse "checked before the first case"
# ASD-V0-913: a floor above the free space refuses with exit 3, naming the directory and the KiB.
out=$(BATCH_PRECHECK_TEST_MIN_KIB=999999999999 bash "$script/batch-precheck_test.sh" 2>&1)
rc=$?
[[ $rc == 3 && $out == *"REFUSE host-resource: TMPDIR ${TMPDIR:-/tmp} has "*"KiB free; this test needs at least 999999999999KiB there"* ]] ||
  fail "a short TMPDIR was not a host-resource refusal (exit $rc): $out"
# The precheck's step TMPDIR (ASD-V0-055) stays inside this test's scratch unless a case sets it.
export BATCH_PRECHECK_TMPDIR=$tmp/step-tmp
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
# Every group but the ledger-reuse one runs cold; that group turns the ledger on explicitly.
export CORVINT_GATE_LEDGER=off
# ASD-V0-781 (V1-4016): Go fails every build when an exported GOTMPDIR is missing, so the test
# creates it before any case runs go.
[[ -z ${GOTMPDIR-} ]] || mkdir -p "$GOTMPDIR" || fail "cannot create GOTMPDIR $GOTMPDIR"
if [[ ${BATCH_PRECHECK_TEST_SEED-} == 1 ]]; then
# 0a. ASD-V0-780 seeded failure one: the run must go on to the next case.
  false || fail "seeded failure one"
# 0b. ASD-V0-780 seeded failure two, inside a subshell.
  (false || fail "seeded failure two")
  finish
fi
# 0c. ASD-V0-780: a failing case does not stop the run; the summary names every failed case.
BATCH_PRECHECK_TEST_SEED=1 "$script/batch-precheck_test.sh" > "$tmp/out" 2>&1
seed_rc=$?
[[ $seed_rc == 1 ]] || fail "seeded run exited $seed_rc, want 1: $(cat "$tmp/out")"
grep -q '^batch-precheck_test: FAIL 2 case(s):$' "$tmp/out" || fail "seeded run did not count two failed cases: $(cat "$tmp/out")"
grep -q '^  - case "0a\. ASD-V0-780 seeded failure one.*seeded failure one$' "$tmp/out" &&
  grep -q '^  - case "0b\. ASD-V0-780 seeded failure two.*seeded failure two$' "$tmp/out" ||
  fail "seeded summary did not name both cases: $(cat "$tmp/out")"
: > "$tmp/out"

# 1. The real workflow: every doc-gates command is a make target or go run, every gofmt set is
# mirrored, every drift prefix names a test and every golden writer is listed. This is the check
# that fails when CI gains a step batch-precheck cannot run.
(cd "$script/.." && "$pre" --sync-check) > "$tmp/out" 2>&1 || fail "checkout out of sync: $(cat "$tmp/out")"
(cd "$script/.." && "$pre" --list) > "$tmp/out" 2>&1 || fail "checkout --list: $(cat "$tmp/out")"
# V1-3232: the shared doc-gates target runs as its Makefile targets and go run, in parallel.
grep -qx 'make line-citations-check' "$tmp/out" && grep -qx 'go run ./conformance/use-cases-v0' "$tmp/out" ||
  fail "derived doc-gates not expanded into their targets: $(cat "$tmp/out")"
grep -qx 'make doc-gates' "$tmp/out" && fail "doc-gates still runs as one step: $(cat "$tmp/out")"
for t in $(sed -n 's/^DOC_GATE_TARGETS := //p' "$script/../Makefile"); do
  grep -qx "make $t" "$tmp/out" || fail "doc-gates target $t has no step: $(cat "$tmp/out")"
done
grep -q '^batch-drift (' "$tmp/out" || fail "no batch-drift step: $(cat "$tmp/out")"
grep -qx 'batch-drift TestCorvintBundledBinariesBuildTwiceIdentically' "$tmp/out" &&
  grep -qx 'batch-drift TestCurrentTasksSourceArchiveBuildsOffline' "$tmp/out" || fail "the @own drift tests are not their own steps: $(cat "$tmp/out")"
grep -qx 'make host-adapter-node-test' "$tmp/out" || fail "no host-adapter-node-test step: $(cat "$tmp/out")"

# 1c. DCW-V0-399 (V1-3218): the lane's pre-bind `make doc-gates` reaches ./internal/specindex,
# whose TestIndexCoversSpecsAndHeaders refuses an Agent digest Status that drifts from the header.
(cd "$script/.." && make -n --no-print-directory HEAVY_ADMIT= doc-gates) > "$tmp/out" 2>&1 || fail "make -n doc-gates: $(cat "$tmp/out")"
grep -q '\./internal/specindex' "$tmp/out" || fail "doc-gates no longer runs ./internal/specindex"
grep -q 'TestIndexCoversSpecsAndHeaders' "$script/../internal/specindex/index_test.go" || fail "specindex header/digest check missing"
grep -q 'make doc-gates' "$script/../docs/DOGFOOD.md" || fail "lane recipe no longer runs make doc-gates before the bind"

# 1b. ASD-V0-044 (V1-3062): that step catches a Pi adapter version bump that leaves the
# docs/specs/pi-workflow-v0.md capability tuple behind. On a copy of the adapter, its spec and the
# native literal, the tuple set to the package version passes PWV-V0-011 and a stale version
# fails it.
pi=$tmp/pi-drift
mkdir -p "$pi/integrations" "$pi/docs/specs" "$pi/cmd/corvint"
/bin/cp -R "$script/../integrations/pi" "$pi/integrations/pi" || fail "copy Pi adapter"
/bin/cp -f "$script/../integrations/compatibility.json" "$pi/integrations/" || fail "copy compatibility matrix"
/bin/cp -f "$script/../cmd/corvint/pi_adapter.go" "$pi/cmd/corvint/" || fail "copy native adapter literal"
pi_version=$(sed -n 's/^  "version": "\(.*\)",$/\1/p' "$pi/integrations/pi/package.json")
[[ -n $pi_version ]] || fail "Pi package version not found"
pi_tuple() { sed "s/\"tuple\": {\"adapterVersion\": \"[^\"]*\"/\"tuple\": {\"adapterVersion\": \"$1\"/" "$script/../docs/specs/pi-workflow-v0.md" > "$pi/docs/specs/pi-workflow-v0.md"; }
pi_tuple "$pi_version"
# The Pi capability test needs node; CI has it. A lane host without it (Unraid) skips this case by
# name instead of failing the whole suite.
if ! command -v node > /dev/null 2>&1; then
  printf 'batch-precheck_test: SKIP case 1b: node is not on PATH, so the Pi tuple test is NOT_RUN\n'
else
node --test "$pi/integrations/pi/capabilities.test.mjs" > "$tmp/out" 2>&1 || fail "matching Pi tuple failed: $(tail -40 "$tmp/out")"
pi_tuple "0.0.0-stale"
node --test "$pi/integrations/pi/capabilities.test.mjs" > "$tmp/out" 2>&1 && fail "a stale Pi spec tuple passed"
grep -q '^not ok .* PWV-V0-011 registered tools' "$tmp/out" || fail "stale tuple did not fail PWV-V0-011: $(tail -40 "$tmp/out")"
fi

# 2. A fixture repository, workflow and packages.
repo=$tmp/repo
mkdir -p "$repo/.github/workflows" "$repo/pkg" "$repo/interop/cem01-go"
git init -q "$repo" || fail "git init"
printf 'package pkg\n\nimport "testing"\n\nfunc TestDriftOne(t *testing.T) {}\n' > "$repo/pkg/a_test.go"
cat > "$tmp/regen-stub" <<'EOF'
#!/bin/sh
case $1 in
  --golden-tests) printf '%s\n' ${STUB_GOLDENS:-} | sed 's/:/ /' ;;
  --mechanical) [ -n "${STUB_REPIN:-}" ] && printf repinned >> derived.txt; exit 0 ;;
esac
EOF
chmod +x "$tmp/regen-stub"
write_ci() {
  cat > "$repo/.github/workflows/ci.yml" <<EOF
jobs:
  doc-gates:
    env:
      GOPROXY: off
      GOFLAGS: ""
    steps:
      - name: Documentation gate steps
        run: make -s spec-requirements-check $1
      - name: Receipts
        run: |
          # a comment
          make -s use-case-receipts-check \\
            use-case-receipts-test
          go run ${2:-./conformance/use-cases-v0}
${4:-}
  go-static:
    steps:
      - run: |
          test -z "\$(gofmt -l ${3:-cmd internal tools .github/cishards .github/testconfine})"
  go-interop:
    steps:
      - run: |
          test -z "\$(gofmt -l .)"
  other:
    steps:
      - run: make -s not-a-doc-gate
EOF
}
fixture() { BATCH_PRECHECK_ROOT=$repo BATCH_PRECHECK_REGEN=$tmp/regen-stub BATCH_PRECHECK_DRIFT=${drift:-"./pkg TestDrift"} "$pre" "$@" > "$tmp/out" 2>&1; }

write_ci ""
fixture --sync-check || fail "matching fixture refused: $(cat "$tmp/out")"
fixture --list || fail "--list: $(cat "$tmp/out")"
for want in 'make spec-requirements-check' 'make use-case-receipts-check' 'make use-case-receipts-test' 'go run ./conformance/use-cases-v0'; do
  grep -qxF "$want" "$tmp/out" || fail "derived step $want missing: $(cat "$tmp/out")"
done
grep -q 'not-a-doc-gate' "$tmp/out" && fail "a step of another job was derived: $(cat "$tmp/out")"
# AFP-V0-671: the doc-gates-reuse decision job and the doc-gates-record job sit beside doc-gates,
# which gains job-level needs/if and a step-level if per step; the twin still mirrors exactly the
# doc-gates commands, and never a command of the decision or record job.
cat > "$repo/.github/workflows/ci.yml" <<'EOF'
jobs:
  doc-gates-reuse:
    if: github.event_name == 'merge_group' || github.event_name == 'push'
    steps:
      - name: Find
        id: find
        run: bash .github/gate-reuse/find.sh
  doc-gates:
    needs: doc-gates-reuse
    if: ${{ !cancelled() }}
    env:
      GOPROXY: off
      GOFLAGS: ""
    steps:
      - name: Documentation gate steps
        if: needs.doc-gates-reuse.outputs.mode != 'REUSE'
        run: make -s spec-requirements-check
      - name: Receipts
        if: needs.doc-gates-reuse.outputs.mode != 'REUSE'
        run: |
          make -s use-case-receipts-check use-case-receipts-test
          go run ./conformance/use-cases-v0
  doc-gates-record:
    needs: doc-gates
    steps:
      - run: printf record
  go-static:
    steps:
      - run: |
          test -z "$(gofmt -l cmd internal tools .github/cishards .github/testconfine)"
  go-interop:
    steps:
      - run: |
          test -z "$(gofmt -l .)"
EOF
fixture --sync-check || fail "doc-gates with reuse wiring refused: $(cat "$tmp/out")"
fixture --list || fail "--list with reuse wiring: $(cat "$tmp/out")"
for want in 'make spec-requirements-check' 'make use-case-receipts-check' 'make use-case-receipts-test' 'go run ./conformance/use-cases-v0'; do
  grep -qxF "$want" "$tmp/out" || fail "derived step $want missing with reuse wiring: $(cat "$tmp/out")"
done
grep -qE 'find\.sh|printf record' "$tmp/out" && fail "a reuse or record command was derived: $(cat "$tmp/out")"
# A new doc-gates make target or go run is picked up without editing batch-precheck.
write_ci "new-drift-check" "./tools/new-gen"
fixture --sync-check || fail "new make target or go run refused: $(cat "$tmp/out")"
fixture --list
grep -qxF 'make new-drift-check' "$tmp/out" || fail "new make target not derived: $(cat "$tmp/out")"
grep -qxF 'go run ./tools/new-gen' "$tmp/out" || fail "new go run not derived: $(cat "$tmp/out")"
# A doc-gates command that is neither fails the sync check.
write_ci "" "" "" "      - run: script/new-gate.sh --check"
fixture --sync-check && fail "unknown doc-gates command accepted"
grep -q 'doc-gates runs `script/new-gate.sh --check`, which batch-precheck cannot mirror' "$tmp/out" || fail "unknown: $(cat "$tmp/out")"
write_ci "VAR=1"
fixture --sync-check && fail "make with a variable accepted"
# Shell syntax in a doc-gates command never reaches a step.
write_ci 'nonexistent;true'
fixture --sync-check && fail "make with a shell operator accepted"
grep -q 'cannot mirror' "$tmp/out" || fail "shell operator: $(cat "$tmp/out")"
write_ci "" './tools/x$(touch pwned)'
fixture --sync-check && fail "go run with a substitution accepted"
[[ -e $repo/pwned ]] && fail "a substitution in ci.yml ran"
# A step form the parser cannot classify fails closed instead of being dropped (V1-2900 review).
for form in '      - {run: "make must-fail"}' \
  $'      - name: continued\n        run: make -s spec-requirements-check\n          must-fail' \
  '      - "run": make must-fail' \
  $'      - run: make -s use-case-receipts-check\n        working-directory: sub' \
  $'      - run:\n          make must-fail'; do
  write_ci "" "" "" "$form"
  fixture --sync-check && fail "unclassifiable step form accepted: $form"
  grep -q 'which batch-precheck cannot mirror' "$tmp/out" || fail "unclassifiable form $form: $(cat "$tmp/out")"
done
write_ci "" "" "" '      - {run: "make must-fail"}'
fixture --list
grep -q 'must-fail' "$tmp/out" && fail "a flow-mapping step was derived: $(cat "$tmp/out")"
write_ci "" "" "cmd internal"
fixture --sync-check && fail "changed gofmt set accepted"
grep -q 'go-static runs gofmt -l cmd internal in \.' "$tmp/out" || fail "gofmt: $(cat "$tmp/out")"
write_ci ""
drift="./pkg TestRenamedAway" fixture --sync-check && fail "stale drift prefix accepted"
grep -q 'drift test prefix TestRenamedAway names no test in ./pkg' "$tmp/out" || fail "drift: $(cat "$tmp/out")"
# A declared drift input must be @go, @root or a tracked path (ASD-V0-050).
git -C "$repo" add pkg/a_test.go
drift="./pkg TestDriftOne @go pkg/" fixture --sync-check || fail "declared @go and a tracked directory refused: $(cat "$tmp/out")"
drift="./pkg TestDriftOne docs/missing.md" fixture --sync-check && fail "a declared input naming no tracked path accepted"
grep -q 'drift test ./pkg TestDriftOne declares input docs/missing.md, which is not @go, @root, @own, @prebind or a tracked path' "$tmp/out" || fail "missing input: $(cat "$tmp/out")"
drift="./pkg TestDriftOne @bogus" fixture --sync-check && fail "an unknown @ token accepted"
# A test that rewrites a golden must be both a drift test and a batch-regen golden; a helper that
# only names the variable in a message is not attributed to the test before it.
mkdir -p "$repo/internal/g"
cat > "$repo/internal/g/g_test.go" <<'EOF'
package g

import (
	"os"
	"testing"
)

func TestGoldenWriter(t *testing.T) { _ = os.Getenv("CORVINT_UPDATE_GOLDEN") }

func helper(t *testing.T) { t.Fatal("rerun with CORVINT_UPDATE_GOLDEN=1") }
EOF
git -C "$repo" add internal/g/g_test.go
drift="./pkg TestDriftOne" fixture --sync-check && fail "golden writer outside DRIFT_TESTS accepted"
grep -q './internal/g TestGoldenWriter reads CORVINT_UPDATE_GOLDEN but is not in DRIFT_TESTS' "$tmp/out" || fail "golden drift: $(cat "$tmp/out")"
grep -q 'TestGoldenWriter reads CORVINT_UPDATE_GOLDEN but .* --golden-tests does not name it' "$tmp/out" || fail "golden regen: $(cat "$tmp/out")"
drift="./pkg TestDriftOne
./internal/g TestGoldenWriter" STUB_GOLDENS="./internal/g:TestGoldenWriter" fixture --sync-check || fail "listed golden writer refused: $(cat "$tmp/out")"
grep -q helper "$tmp/out" && fail "helper attributed: $(cat "$tmp/out")"
export STUB_GOLDENS="./internal/g:TestGoldenWriter"

# 3. The runner on stubs: a failing make target, an unformatted file and a drift package that runs
# no test fail the run; every other step still runs, in parallel, under the doc-gates env.
bin=$tmp/bin
mkdir -p "$bin"
cat > "$bin/make" <<EOF
#!/bin/sh
echo "make \$* GOPROXY=\${GOPROXY-unset}" >> $tmp/calls
mkdir -p $tmp/running && : > $tmp/running/\$2
case "\$2" in
  test-commands-check) [ -n "\${STUB_FAIL:-}" ] && { rm -f $tmp/running/\$2; echo boom; exit 1; } ;;
  slow-*) sleep 3 ;;
  host-adapter-node-test) if [ -n "\${STUB_SERIAL_PROBE:-}" ]; then
      sleep 1; ls $tmp/running | grep -vx host-adapter-node-test >> $tmp/overlap; fi ;;
esac
rm -f $tmp/running/\$2
exit 0
EOF
# The stub node makes the host-adapter-node-test step run on a host without node (ASD-V0-973).
printf '#!/bin/sh\nexit 0\n' > "$bin/node"
cat > "$bin/gofmt" <<EOF
#!/bin/sh
echo "gofmt \$*" >> $tmp/calls
[ "\$2" = . ] && [ -n "\${STUB_FAIL:-}" ] && echo bad.go
exit 0
EOF
cat > "$bin/go" <<EOF
#!/bin/sh
echo "go \$*" >> $tmp/calls
case "\$*" in *TestVacant*) echo "ok pkg2 [no tests to run]" ;; *) echo ok ;; esac
exit 0
EOF
chmod +x "$bin"/*
mkdir -p "$repo/pkg2"
printf 'package pkg2\n\nimport "testing"\n\nfunc TestVacant(t *testing.T) {}\n' > "$repo/pkg2/a_test.go"
write_ci "test-commands-check slow-a slow-b"
git -C "$repo" add -A && git -C "$repo" commit -q -m fixture || fail "fixture commit"
receipt=$(git -C "$repo" rev-parse --path-format=absolute --git-path batch-precheck-receipt)
# The caller's GOPROXY is unset so the stub make sees only what the precheck applies: release-gates
# exports GOPROXY=off, which would mask the doc-gates env check below (V1-3055).
run() { (unset GOPROXY; PATH=$bin:$PATH BATCH_PRECHECK_HEAVY='' drift=${drift:-"./pkg TestDriftOne
./internal/g TestGoldenWriter"} fixture "$@"); }

# ASD-V0-894 (V1-3387): the step count comes from the fixture's own --list, so moving a step in or
# out of the list changes no literal below; the table adds the repin row to the listed steps.
run --list || fail "fixture --list: $(cat "$tmp/out")"
nsteps=$(grep -c . "$tmp/out")
((nsteps > 3)) || fail "fixture --list printed $nsteps steps: $(cat "$tmp/out")"
rows=$((nsteps + 1))
: > "$tmp/calls"
STUB_FAIL=1 drift="./pkg TestDriftOne
./internal/g TestGoldenWriter
./pkg2 TestVacant" run
[[ $? == 1 ]] || fail "failing run did not exit 1: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | sync-check$' "$tmp/out" || fail "sync row: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | repin (mechanical)$' "$tmp/out" || fail "repin row: $(cat "$tmp/out")"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | make test-commands-check$' "$tmp/out" || fail "make row: $(cat "$tmp/out")"
grep -q '^  | boom' "$tmp/out" || fail "failure output not shown: $(cat "$tmp/out")"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | gofmt -l \. (interop/cem01-go)$' "$tmp/out" || fail "gofmt row: $(cat "$tmp/out")"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | batch-drift (3 packages)$' "$tmp/out" || fail "no tests to run passed: $(cat "$tmp/out")"
# One go test per attempt: the failed drift step reruns once (ASD-V0-046).
[[ $(grep -c '^go test' "$tmp/calls") == 2 && $(grep '^go test' "$tmp/calls" | sort -u | wc -l | tr -d ' ') == 1 ]] ||
  fail "drift was not one go test per attempt: $(cat "$tmp/calls")"
grep -q -- '-run ^(TestDriftOne|TestGoldenWriter|TestVacant) ./pkg ./internal/g ./pkg2$' "$tmp/calls" || fail "drift union: $(cat "$tmp/calls")"
grep -q '^make -s use-case-receipts-test GOPROXY=off$' "$tmp/calls" || fail "doc-gates env not applied: $(cat "$tmp/calls")"
grep -q '^make -s capability-catalog-check GOPROXY=unset$' "$tmp/calls" || fail "extra target: $(cat "$tmp/calls")"
grep -q '^make -s host-adapter-node-test GOPROXY=unset$' "$tmp/calls" || fail "host-adapter-node-test not run: $(cat "$tmp/calls")"
# ASD-V0-640 (V1-3744): with no base the lane shell tests run on the merged tree.
grep -q '^make -s lane-scripts-test GOPROXY=unset$' "$tmp/calls" || fail "lane-scripts-test not run without a base: $(cat "$tmp/calls")"
grep -q '^batch-precheck: lane-scripts-test runs (no --base)$' "$tmp/out" || fail "lane-scripts-test reason: $(cat "$tmp/out")"
grep -q "^batch-precheck: $((rows - 3)) passed, 3 failed in " "$tmp/out" || fail "summary: $(cat "$tmp/out")"
grep -q '^batch-precheck: failed: ' "$tmp/out" || fail "failed list: $(cat "$tmp/out")"
[[ -f $receipt ]] && fail "a failing run wrote a receipt"
# Parallel: the two 3-second steps overlap, so the wall time is below the summed time.
wall=$(sed -n 's/.* in \([0-9]*\)s wall, \([0-9]*\)s summed$/\1 \2/p' "$tmp/out")
read -r w s <<< "$wall"
((w < s)) || fail "steps did not run in parallel (wall $w, summed $s)"
# ASD-V0-047 (V1-3142): host-adapter-node-test runs alone after the parallel steps, so no other
# step is running while it does, even beside the two 3-second steps; the others stay parallel.
: > "$tmp/overlap"
STUB_SERIAL_PROBE=1 run
[[ $? == 0 ]] || fail "serial probe run failed: $(cat "$tmp/out")"
[[ -s $tmp/overlap ]] && fail "host-adapter-node-test ran beside: $(sort -u "$tmp/overlap" | paste -sd ' ' -)"
grep -q '^batch-precheck: | PASS | *[0-9]*s | make host-adapter-node-test$' "$tmp/out" || fail "serial row: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | make lane-scripts-test$' "$tmp/out" || fail "lane-scripts-test serial row: $(cat "$tmp/out")"
read -r w s <<< "$(sed -n 's/.* in \([0-9]*\)s wall, \([0-9]*\)s summed$/\1 \2/p' "$tmp/out")"
((w < s)) || fail "the other steps no longer ran in parallel (wall $w, summed $s)"
/bin/rm -f "$receipt"

# 4. A passing run on a clean checkout writes the receipt for HEAD; the guard accepts HEAD, then
# refuses once HEAD moves, and a run on a dirty checkout writes none.
head=$(git -C "$repo" rev-parse HEAD)
(cd "$repo" && "$pre" --verify-receipt) > "$tmp/out" 2>&1 && fail "guard passed with no receipt"
grep -q 'refused: no precheck receipt' "$tmp/out" || fail "no receipt: $(cat "$tmp/out")"
run || fail "passing run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: receipt for $head written" "$tmp/out" || fail "receipt not written: $(cat "$tmp/out")"
grep -qx "steps $rows" "$receipt" && grep -qx "passed $rows" "$receipt" || fail "receipt counts: $(cat "$receipt")"
[[ $(head -n 1 "$receipt") == corvint-batch-precheck-receipt/0 ]] || fail "receipt profile: $(cat "$receipt")"
(cd "$repo" && "$pre" --verify-receipt) > "$tmp/out" 2>&1 || fail "guard refused HEAD: $(cat "$tmp/out")"
(cd "$repo" && "$pre" --verify-receipt "$head") > /dev/null 2>&1 || fail "guard refused an explicit REV"
# 4a. ASD-V0-973 (V1-3801, V1-3241): with node absent, host-adapter-node-test does not run; its row
# is NOT_AVAILABLE naming the tool, it is neither passed nor failed, and the receipt records it as
# a not-run line the guard accepts and prints. A receipt claiming more steps than it accounts for
# is refused.
/bin/rm -f "$receipt"
: > "$tmp/calls"
BATCH_PRECHECK_NODE=corvint-absent-node-probe run || fail "absent-node run failed: $(cat "$tmp/out")"
grep -q 'host-adapter-node-test' "$tmp/calls" && fail "absent-node run still ran the node step: $(cat "$tmp/calls")"
grep -q '^batch-precheck: | NOT_AVAILABLE | *[0-9]*s | make host-adapter-node-test: corvint-absent-node-probe is not on PATH, so the node half of host-adapter-test did not run; install Node.js to run it$' "$tmp/out" ||
  fail "absent-node row: $(cat "$tmp/out")"
grep -q "^batch-precheck: $((rows - 1)) passed, 0 failed, 1 not run in " "$tmp/out" || fail "absent-node summary: $(cat "$tmp/out")"
grep -qx "steps $rows" "$receipt" && grep -qx "passed $((rows - 1))" "$receipt" &&
  grep -qx 'not-run host-adapter-node-test corvint-absent-node-probe is not on PATH, so the node half of host-adapter-test did not run; install Node.js to run it' "$receipt" ||
  fail "absent-node receipt: $(cat "$receipt" 2> /dev/null)"
(cd "$repo" && "$pre" --verify-receipt) > "$tmp/out" 2>&1 || fail "guard refused a not-run receipt: $(cat "$tmp/out")"
grep -q '^batch-precheck: receipt records NOT_RUN host-adapter-node-test corvint-absent-node-probe is not on PATH' "$tmp/out" || fail "guard did not print the not-run step: $(cat "$tmp/out")"
sed -i.bak '/^not-run /d' "$receipt" && /bin/rm -f "$receipt.bak"
(cd "$repo" && "$pre" --verify-receipt) > /dev/null 2>&1 && fail "guard accepted a receipt whose steps exceed passed and not-run"
# ASD-V0-837 (V1-3550): the pre-bind run of dogfood-closeout leaves out the batch-drift go test and
# writes no receipt, so the receipt and each drift step come from the one run on the sealed head.
/bin/rm -f "$receipt"
: > "$tmp/calls"
BATCH_PRECHECK_SKIP_DRIFT=1 run || fail "skip-drift run failed: $(cat "$tmp/out")"
grep -q '^go test' "$tmp/calls" && fail "skip-drift ran the drift go test: $(cat "$tmp/calls")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | batch-drift (skipped before the bind)$' "$tmp/out" || fail "skip-drift row: $(cat "$tmp/out")"
grep -q '^batch-precheck: no receipt: BATCH_PRECHECK_SKIP_DRIFT=1' "$tmp/out" || fail "skip-drift receipt line: $(cat "$tmp/out")"
[[ -f $receipt ]] && fail "a skip-drift run wrote a receipt"
run || fail "passing run after skip-drift failed: $(cat "$tmp/out")"
# BATCH_PRECHECK_HEAVY is one quoted path: a space does not split it and $(...) never runs.
hv="$tmp/h d/heavy\$(touch pwned)"
mkdir -p "$tmp/h d"
printf '#!/bin/sh\necho heavy >> "%s/calls"\nexec "$@"\n' "$tmp" > "$hv"
chmod +x "$hv"
: > "$tmp/calls"
PATH=$bin:$PATH BATCH_PRECHECK_HEAVY=$hv drift="./pkg TestDriftOne
./internal/g TestGoldenWriter" fixture || fail "quoted heavy run failed: $(cat "$tmp/out")"
grep -qx heavy "$tmp/calls" || fail "heavy path with a space not run: $(cat "$tmp/calls")"
[[ -e $repo/pwned || -e $script/pwned || -e pwned ]] && fail "a substitution in BATCH_PRECHECK_HEAVY ran"
# 3b. ASD-V0-941 (V1-3037): BATCH_PRECHECK_GO_WRAPPER, one quoted path, wraps every make and go run
# step; unset, none is wrapped. Two concurrent prechecks whose wrapper takes one shared lock never
# run two wrapped steps at once.
gw="$tmp/g w/wrap\$(touch pwned)"
mkdir -p "$tmp/g w"
# The lock is flock(2), not mkdir: uutils mkdir (0.8.0) lets two racing calls both succeed.
cat > "$gw" <<'WRAP'
#!/usr/bin/env perl
use strict;
use warnings;
use Fcntl qw(:flock);
my $tmp = $ENV{GW_TMP};
open(my $lock, ">>", "$tmp/gw.lock") or die "gw: lock: $!\n";
flock($lock, LOCK_EX) or die "gw: flock: $!\n";
sub note { open(my $w, ">>", "$tmp/wrapped") or die "gw: log: $!\n"; print $w "@_\n"; close $w }
note("start @ARGV");
my $status = system { $ARGV[0] } @ARGV;
note("end");
exit($status == -1 ? 127 : $status & 127 ? 128 + ($status & 127) : $status >> 8);
WRAP
export GW_TMP=$tmp
chmod +x "$gw"
: > "$tmp/calls"; : > "$tmp/wrapped"
run || fail "unwrapped run failed: $(cat "$tmp/out")"
[[ -s $tmp/wrapped ]] && fail "an unset BATCH_PRECHECK_GO_WRAPPER wrapped a step: $(cat "$tmp/wrapped")"
wrapped_targets() { sed -nE 's/^start .* (make -s|go run) ([^ ]*)$/\1 \2/p' "$tmp/wrapped" | tr -d "'" | sort; }
called_targets() { sed -nE 's/^(make -s|go run) ([^ ]*).*$/\1 \2/p' "$tmp/calls" | sort; }
: > "$tmp/calls"
(export BATCH_PRECHECK_GO_WRAPPER=$gw; run) || fail "wrapped run failed: $(cat "$tmp/out")"
[[ -n $(called_targets) && $(wrapped_targets) == "$(called_targets)" ]] ||
  fail "not every make or go run step was wrapped: wrapped $(wrapped_targets | paste -sd ' ' -), called $(called_targets | paste -sd ' ' -)"
[[ -e $repo/pwned || -e $script/pwned || -e pwned || -e "$tmp/g w/pwned" ]] && fail "a substitution in BATCH_PRECHECK_GO_WRAPPER ran"
/bin/rm -rf "$tmp/repo2" && /bin/cp -R "$repo" "$tmp/repo2" || fail "second fixture copy"
: > "$tmp/wrapped"
run2() {
  (unset GOPROXY; export PATH=$bin:$PATH BATCH_PRECHECK_HEAVY='' BATCH_PRECHECK_GO_WRAPPER=$gw BATCH_PRECHECK_ROOT=$1 \
    BATCH_PRECHECK_REGEN=$tmp/regen-stub BATCH_PRECHECK_DRIFT="./pkg TestDriftOne
./internal/g TestGoldenWriter"; "$pre" > "$2" 2>&1)
}
run2 "$repo" "$tmp/out1" & p1=$!
run2 "$tmp/repo2" "$tmp/out2" & p2=$!
wait "$p1" || fail "first concurrent wrapped run failed: $(cat "$tmp/out1")"
wait "$p2" || fail "second concurrent wrapped run failed: $(cat "$tmp/out2")"
[[ $(grep -c '^start ' "$tmp/wrapped") -ge 2 && -z $(awk '{ k = $1 } k == last { print NR; exit } { last = k }' "$tmp/wrapped") ]] ||
  fail "concurrent prechecks ran two wrapped steps at once: $(cat "$tmp/wrapped")"
git -C "$repo" commit -q --allow-empty -m moved
(cd "$repo" && "$pre" --verify-receipt) > "$tmp/out" 2>&1 && fail "guard passed after HEAD moved"
grep -q "refused: the precheck receipt names $head, not " "$tmp/out" || fail "mismatch: $(cat "$tmp/out")"
(cd "$repo" && "$pre" --verify-receipt no-such-rev) > /dev/null 2>&1
[[ $? == 2 ]] || fail "unknown REV did not exit 2"
printf 'dirty\n' >> "$repo/pkg/a_test.go"
run || fail "dirty run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: no receipt' "$tmp/out" || fail "dirty run: $(cat "$tmp/out")"
[[ -f $receipt ]] && fail "a dirty run wrote a receipt"
git -C "$repo" checkout -q -- pkg/a_test.go
printf 'package pkg\n' > "$repo/pkg/untracked.go"
run || fail "untracked run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: no receipt' "$tmp/out" || fail "untracked run: $(cat "$tmp/out")"
[[ -f $receipt ]] && fail "a run with an untracked file wrote a receipt"
/bin/rm -f "$repo/pkg/untracked.go"
# An ignored file a go test can read (a _test.go with init, a fixture in a package directory, a
# dot or underscore fixture there or under testdata) means the checked tree is not HEAD, so no
# receipt; an ignored file under a dot directory outside any package or testdata does not.
printf 'local_test.go\n*.local\n.cache/\n_cases/\n.fixture\n' > "$repo/.gitignore"
mkdir -p "$repo/pkg/testdata" && printf 'tracked\n' > "$repo/pkg/testdata/tracked.txt"
git -C "$repo" add .gitignore pkg/testdata && git -C "$repo" commit -q -m ignore || fail "ignore commit"
for f in pkg/local_test.go pkg/fixture.local pkg/testdata/_cases/input.json pkg/.fixture; do
  mkdir -p "$(dirname "$repo/$f")"
  printf 'package pkg\n\nfunc init() {}\n' > "$repo/$f"
  run || fail "ignored $f run failed: $(cat "$tmp/out")"
  grep -q '^batch-precheck: no receipt' "$tmp/out" || fail "ignored $f: $(cat "$tmp/out")"
  grep -q "^  $f\$" "$tmp/out" || fail "ignored $f not named: $(cat "$tmp/out")"
  [[ -f $receipt ]] && fail "a run with ignored $f wrote a receipt"
  /bin/rm -f "$repo/$f"
done
/bin/rm -rf "$repo/pkg/testdata/_cases"
mkdir -p "$repo/.cache" && printf 'package x\n' > "$repo/.cache/x.go"
run || fail "dot-dir ignored run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: receipt for ' "$tmp/out" || fail "dot-dir ignored file blocked the receipt: $(cat "$tmp/out")"
/bin/rm -rf "$repo/.cache" "$receipt"
# A malformed or partly failing receipt naming HEAD is refused.
git -C "$repo" rev-parse HEAD > "$tmp/h"
printf 'corvint-batch-precheck-receipt/0\nhead %s\n' "$(cat "$tmp/h")" > "$receipt"
(cd "$repo" && "$pre" --verify-receipt) > "$tmp/out" 2>&1 && fail "truncated receipt accepted"
grep -q 'not a well-formed passing precheck receipt' "$tmp/out" || fail "truncated: $(cat "$tmp/out")"
printf 'corvint-batch-precheck-receipt/0\nhead %s\nsteps 3\npassed 2\nfinished x\n' "$(cat "$tmp/h")" > "$receipt"
(cd "$repo" && "$pre" --verify-receipt) > /dev/null 2>&1 && fail "receipt with a failed step accepted"
/bin/rm -f "$receipt"
# A mechanical repin that changes a tracked file on a clean checkout fails the run.
printf 'v0\n' > "$repo/derived.txt"
git -C "$repo" add derived.txt && git -C "$repo" commit -q -m derived
STUB_REPIN=1 run && fail "a repin that changed a file passed"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | repin (mechanical)$' "$tmp/out" || fail "repin fail row: $(cat "$tmp/out")"
grep -q 'mechanical repins changed tracked files' "$tmp/out" || fail "repin message: $(cat "$tmp/out")"
git -C "$repo" checkout -q -- derived.txt

# 4b. Ledger reuse (ASD-V0-042, V1-2985): a rerun on identical inputs reuses every step's recorded
# pass, reports it and still writes the receipt; a tree change reruns the whole-tree steps while
# the gofmt steps (keyed on *.go) still reuse; a changed GO* variable, a staged index that differs
# from the worktree and CORVINT_GATE_LEDGER=off reuse nothing; a failure is never reused.
(cd "$script/.." && GOTOOLCHAIN=local go build -o "$tmp/gate-ledger" ./tools/gate-ledger) > "$tmp/out" 2>&1 || fail "gate-ledger build: $(cat "$tmp/out")"
lrun() { CORVINT_GATE_LEDGER='' CORVINT_GATE_LEDGER_DIR=$tmp/ledger BATCH_PRECHECK_LEDGER=$tmp/gate-ledger run; }
# ASD-V0-641 (V1-3745): the print-only rows come from --list-print-only, so no count below names a
# step; under BATCH_PRECHECK_SKIP_DRIFT=1 the skipped drift row is one of them.
run --list-print-only || fail "--list-print-only: $(cat "$tmp/out")"
nprint=$(grep -c . "$tmp/out")
BATCH_PRECHECK_SKIP_DRIFT=1 run --list-print-only || fail "skip-drift --list-print-only: $(cat "$tmp/out")"
nprint_skip=$(grep -c . "$tmp/out")
grep -qxF 'batch-drift (skipped before the bind)' "$tmp/out" && ((nprint_skip == nprint + 1)) ||
  fail "--list-print-only missed the skipped drift row ($nprint, $nprint_skip): $(cat "$tmp/out")"
lrun || fail "first ledger run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: ledger reuse on ($tmp/gate-ledger)$" "$tmp/out" || fail "ledger not on: $(cat "$tmp/out")"
grep -q 'REUSE' "$tmp/out" && fail "a first run reused: $(cat "$tmp/out")"
: > "$tmp/calls"
lrun || fail "reuse run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: | REUSE | *[0-9]*s | make test-commands-check$' "$tmp/out" || fail "make step not reused: $(cat "$tmp/out")"
grep -q '^batch-precheck: | REUSE | *[0-9]*s | batch-drift (2 packages)$' "$tmp/out" || fail "drift not reused: $(cat "$tmp/out")"
grep -q '^batch-precheck: REUSE make slow-a ([0-9]*s): .*recorded ' "$tmp/out" || fail "reuse line: $(cat "$tmp/out")"
grep -q "^batch-precheck: $rows passed, 0 failed in " "$tmp/out" || fail "reuse summary: $(cat "$tmp/out")"
# Only the repin row and the print-only rows run; every other listed step reuses.
[[ $(grep -c '^batch-precheck: | PASS |' "$tmp/out") == $((1 + nprint)) ]] || fail "a listed step did not reuse: $(cat "$tmp/out")"
grep -q "^batch-precheck: $((nsteps - nprint)) of $nsteps steps reused a ledger pass on identical inputs" "$tmp/out" || fail "reuse count: $(cat "$tmp/out")"
grep -q '^batch-precheck: receipt for ' "$tmp/out" && grep -qx "passed $rows" "$receipt" || fail "reuse receipt: $(cat "$tmp/out")"
grep -q '^make \|^gofmt \|^go test' "$tmp/calls" && fail "a reused step ran: $(cat "$tmp/calls")"
# ASD-V0-892 (V1-3572): a print-only row (here the drift step skipped before the bind) never goes
# through the ledger, so it reruns as a PASS while every other step reuses.
BATCH_PRECHECK_SKIP_DRIFT=1 lrun || fail "skip-drift reuse run failed: $(cat "$tmp/out")"
[[ $(grep -c '^batch-precheck: | PASS |' "$tmp/out") == $((1 + nprint_skip)) ]] && grep -q '^batch-precheck: | PASS | *[0-9]*s | batch-drift (skipped before the bind)$' "$tmp/out" ||
  fail "print-only row went through the ledger: $(cat "$tmp/out")"
grep -q "^batch-precheck: $((nsteps - nprint_skip)) of $nsteps steps reused a ledger pass on identical inputs" "$tmp/out" || fail "print-only reuse count: $(cat "$tmp/out")"
grep -q '^make \|^gofmt \|^go test' "$tmp/calls" && fail "a reused step ran: $(cat "$tmp/calls")"
printf 'note\n' > "$repo/notes.txt"
lrun || fail "tree-change run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | make test-commands-check$' "$tmp/out" || fail "tree change reused a tree step: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | batch-drift (2 packages)$' "$tmp/out" || fail "tree change reused drift: $(cat "$tmp/out")"
grep -q '^batch-precheck: | REUSE | *[0-9]*s | gofmt -l \. (interop/cem01-go)$' "$tmp/out" || fail "gofmt not reused outside *.go: $(cat "$tmp/out")"
GOFLAGS=-mod=mod lrun || fail "GOFLAGS run failed: $(cat "$tmp/out")"
grep -q 'REUSE' "$tmp/out" && fail "a changed GO variable reused: $(cat "$tmp/out")"
git -C "$repo" add notes.txt && printf 'more\n' >> "$repo/notes.txt"
lrun || fail "staged run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: ledger reuse off (the index differs from the worktree); every step runs$' "$tmp/out" || fail "staged index: $(cat "$tmp/out")"
grep -q 'REUSE' "$tmp/out" && fail "a staged index reused: $(cat "$tmp/out")"
git -C "$repo" reset -q -- notes.txt
CORVINT_GATE_LEDGER=off CORVINT_GATE_LEDGER_DIR=$tmp/ledger BATCH_PRECHECK_LEDGER=$tmp/gate-ledger run || fail "off run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: ledger reuse off (CORVINT_GATE_LEDGER=off); every step runs$' "$tmp/out" || fail "off: $(cat "$tmp/out")"
grep -q 'REUSE' "$tmp/out" && fail "CORVINT_GATE_LEDGER=off reused: $(cat "$tmp/out")"
printf 'other\n' > "$repo/notes.txt"
STUB_FAIL=1 lrun && fail "a failing ledger run passed"
STUB_FAIL=1 lrun && fail "a failure was reused"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | make test-commands-check$' "$tmp/out" || fail "failure rerun: $(cat "$tmp/out")"
/bin/rm -f "$repo/notes.txt" "$receipt"

# ASD-V0-922 (V1-3342): the full batch-regen run regenerates the documentation-paths declaration
# after the spec index and repins changed line-citation spans against the batch base.
want_steps=$("$regen" --list | cut -f1 | tr '\n' ' ')
[[ $want_steps == 'spec-requirements documentation-paths capability-catalog analyzer-schema goldens host-package-versions line-citations use-case-receipts ' ]] ||
  fail "batch-regen --list steps: $want_steps"
"$regen" --list | grep -qx $'documentation-paths\tmake -s documentation-paths' && "$regen" --list | grep -qx $'line-citations\tchanged_citations' &&
  grep -q '^changed_citations() { script/check-line-citations.sh --repin --changed --base "${BATCH_REGEN_BASE:-origin/main}"; }$' "$regen" ||
  fail "batch-regen does not regenerate documentation-paths or repin changed citations against the base"

# 4c. ASD-V0-641 (V1-3745): the test-script-refs step fails when a shell test names a script that is
# not in the tree, naming the file, line and path, and passes once the script exists.
mkdir -p "$repo/script"
# The reference is assembled, so this file does not itself name a missing script.
gone='$'script/gone.sh
printf '#!/bin/sh\nscript=$(cd "$(dirname "$0")" && pwd)\n"%s" --check\n' "$gone" > "$repo/script/caller_test.sh"
git -C "$repo" add script/caller_test.sh && git -C "$repo" commit -q -m caller || fail "caller commit"
run && fail "a test naming a missing script passed: $(cat "$tmp/out")"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | test-script-refs$' "$tmp/out" || fail "test-script-refs row: $(cat "$tmp/out")"
grep -qF "check-test-script-refs: FAIL script/caller_test.sh:3 names $gone, but script/gone.sh does not exist" "$tmp/out" || fail "missing script not named: $(cat "$tmp/out")"
printf '#!/bin/sh\n' > "$repo/script/gone.sh"
git -C "$repo" add script/gone.sh && git -C "$repo" commit -q -m gone || fail "gone commit"
run || fail "an existing script failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | test-script-refs$' "$tmp/out" || fail "test-script-refs pass row: $(cat "$tmp/out")"
git -C "$repo" rm -q -r script && git -C "$repo" commit -q -m 'drop caller' || fail "caller cleanup"
/bin/rm -f "$receipt"

# 4d. ASD-V0-642 (V1-3764): the dr-numbers step fails when two merged lanes both defined one DR-
# number, naming the number and both lines, and passes once the later entry is renumbered. A tree
# without a register passes (every earlier case).
mkdir -p "$repo/conformance"
printf '# Register\n\n### DR-0046 \xe2\x80\x94 a\n\n### DR-0047 \xe2\x80\x94 lane one\n\n### DR-0047 \xe2\x80\x94 lane two\n' > "$repo/conformance/divergence-register.md"
git -C "$repo" add conformance && git -C "$repo" commit -q -m register || fail "register commit"
run && fail "a duplicate DR number passed: $(cat "$tmp/out")"
grep -q '^batch-precheck: | FAIL | *[0-9]*s | dr-numbers$' "$tmp/out" || fail "dr-numbers row: $(cat "$tmp/out")"
grep -qF 'check-dr-numbers: FAIL DR-0047 is defined 2 times (conformance/divergence-register.md:5, conformance/divergence-register.md:7)' "$tmp/out" ||
  fail "duplicate DR number not named: $(cat "$tmp/out")"
sed -i.bak '7s/DR-0047/DR-0048/' "$repo/conformance/divergence-register.md" && /bin/rm -f "$repo/conformance/divergence-register.md.bak"
git -C "$repo" commit -q -am renumber || fail "renumber commit"
run || fail "distinct DR numbers failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: | PASS | *[0-9]*s | dr-numbers$' "$tmp/out" || fail "dr-numbers pass row: $(cat "$tmp/out")"
git -C "$repo" rm -q -r conformance && git -C "$repo" commit -q -m 'drop register' || fail "register cleanup"
/bin/rm -f "$receipt"

# 5. batch-regen on stub steps: one commit of the regenerated files, nothing on a second run, a
# refusal on tracked changes, and no commit when a step fails.
rg() { BATCH_REGEN_ROOT=$repo BATCH_REGEN_STEPS=$1 "$regen" > "$tmp/out" 2>&1; }
steps='gen-a	printf "a\n" > derived.txt
gen-b	printf "b\n" > new-derived.txt'
base=$(git -C "$repo" rev-parse HEAD)
rg "$steps" || fail "regen failed: $(cat "$tmp/out")"
[[ $(git -C "$repo" rev-list --count "$base..HEAD") == 1 ]] || fail "regen did not make one commit: $(cat "$tmp/out")"
[[ $(git -C "$repo" show --name-only --format= HEAD | sort | tr '\n' ' ') == 'derived.txt new-derived.txt ' ]] || fail "regen commit files: $(git -C "$repo" show --stat HEAD)"
grep -q '^batch-regen: PASS gen-b' "$tmp/out" || fail "regen step line: $(cat "$tmp/out")"
again=$(git -C "$repo" rev-parse HEAD)
rg "$steps" || fail "second regen failed: $(cat "$tmp/out")"
grep -q '^batch-regen: nothing to regenerate' "$tmp/out" || fail "second regen not idempotent: $(cat "$tmp/out")"
[[ $(git -C "$repo" rev-parse HEAD) == "$again" ]] || fail "second regen committed"
# ASD-V0-928 (V1-3747): a --changed re-anchor is named for review in the output and the commit body.
rg 'gen-e	printf "e\n" > derived.txt; echo "docs/x.md:3  a.go:1-2 changed since origin/main; --changed re-anchored it as a.go:4-5 (read those lines: the anchor now asserts this content)"'
grep -qxF 'batch-regen: REVIEW docs/x.md:3  a.go:1-2 changed since origin/main; --changed re-anchored it as a.go:4-5 (read those lines: the anchor now asserts this content)' "$tmp/out" ||
  fail "a --changed re-anchor was not named for review: $(cat "$tmp/out")"
git -C "$repo" log -1 --format=%B | grep -qxF 'Re-anchored citations (read each; ASD-V0-928):' &&
  git -C "$repo" log -1 --format=%B | grep -qF -- 'a.go:1-2 changed since origin/main; --changed re-anchored it as a.go:4-5' ||
  fail "the regen commit body does not record the re-anchor: $(git -C "$repo" log -1 --format=%B)"
git -C "$repo" log -1 --format=%s | grep -qxF 'chore(batch): regenerate derived files (make batch-regen, ASD-V0-033)' || fail "regen subject changed"
again=$(git -C "$repo" rev-parse HEAD)
printf 'x\n' >> "$repo/derived.txt"
rg "$steps"
[[ $? == 2 ]] || fail "regen on tracked changes did not exit 2: $(cat "$tmp/out")"
git -C "$repo" checkout -q -- derived.txt
rg 'gen-c	printf "c\n" > derived.txt
broken	false
gen-d	printf "d\n" > derived.txt'
[[ $? == 1 ]] || fail "failing regen step did not exit 1: $(cat "$tmp/out")"
[[ $(git -C "$repo" rev-parse HEAD) == "$again" ]] || fail "failing regen committed"
[[ $(cat "$repo/derived.txt") == c ]] || fail "regen ran a step after the failure"
git -C "$repo" diff --cached --quiet && fail "failing regen did not leave its changes staged"
mkdir -p "$repo/script"
printf '#!/bin/sh\nexit 0\n' > "$repo/script/check-host-package-versions.sh"
printf '#!/bin/sh\necho "citations $*" >> "%s/rcalls"\n' "$tmp" > "$repo/script/check-line-citations.sh"
printf '#!/bin/sh\necho receipts >> "%s/rcalls"\n' "$tmp" > "$repo/script/repin-use-case-receipts.sh"
chmod +x "$repo"/script/*.sh
: > "$tmp/rcalls"
BATCH_REGEN_ROOT=$repo "$regen" --mechanical > "$tmp/out" 2>&1 || fail "mechanical regen failed: $(cat "$tmp/out")"
[[ $(cat "$tmp/rcalls") == $'citations --repin\nreceipts' ]] || fail "mechanical regen line-citation repin: $(cat "$tmp/rcalls")"
grep -q '^batch-regen: PASS line-citations' "$tmp/out" || fail "mechanical line-citations row: $(cat "$tmp/out")"
/bin/rm -rf "$repo/script"
"$regen" --golden-tests | grep -qx './cmd/corvint TestCoreVerbsEmitTheFrozenProfiles' || fail "regen goldens list"

# 6. --base (ASD-V0-032, V1-2895): a failure the base shows with the same findings is a base-present
# WARN that does not fail the run; a new finding in a step the base also fails, a step only the head
# fails, and an unresolvable base are introduced FAILs.
write_ci "test-read-scopes-check test-commands-check"
crepo=$tmp/crepo
mkdir -p "$crepo/.github/workflows" "$crepo/pkg" "$crepo/interop/cem01-go"
cp "$repo/.github/workflows/ci.yml" "$crepo/.github/workflows/ci.yml"
cp "$repo/pkg/a_test.go" "$crepo/pkg/a_test.go"
cbin=$tmp/cbin
mkdir -p "$cbin"
# The stub make fails a target when fail-<target> exists in the current checkout, printing it, the
# checkout path and a varying timing, which the comparison must ignore. It finds the checkout through
# git, so an exported GIT_DIR/GIT_WORK_TREE that leaked into the base run would read the head.
printf '#!/bin/sh\ntop=$(git rev-parse --show-toplevel)\n[ -f "$top/fail-$2" ] || exit 0\necho "heavy.sh: running as $$"; cat "$top/fail-$2"; echo "ok  $(pwd) 0.$$s"; exit 1\n' > "$cbin/make"
printf '#!/bin/sh\nexit 0\n' > "$cbin/gofmt"
printf '#!/bin/sh\necho ok\n' > "$cbin/go"
chmod +x "$cbin"/*
printf 'shell_test.go: undeclared read scope\ncmd/v1schema.go\n' > "$crepo/fail-test-read-scopes-check"
printf 'unlisted flag --old\n' > "$crepo/fail-test-commands-check"
git -C "$crepo" init -q && git -C "$crepo" add -A &&
  git -C "$crepo" -c user.name=t -c user.email=t@t commit -qm base || fail "fixture commit"
basesha=$(git -C "$crepo" rev-parse HEAD)
printf 'unlisted flag --new\n' >> "$crepo/fail-test-commands-check"
printf 'catalog drift\n' > "$crepo/fail-capabilities-check"
crun() { PATH=$cbin:$PATH BATCH_PRECHECK_ROOT=$crepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_DRIFT="./pkg TestDrift" "$pre" "$@" > "$tmp/out" 2>&1; }
crun --base "$basesha"
[[ $? == 1 ]] || fail "introduced failures passed with --base: $(cat "$tmp/out")"
grep -q "^batch-precheck: WARN make test-read-scopes-check .* base-present: fails the same way at $basesha" "$tmp/out" ||
  fail "base-present not warned: $(cat "$tmp/out")"
grep -q '^batch-precheck: FAIL make test-commands-check .* introduced' "$tmp/out" || fail "new finding hidden: $(cat "$tmp/out")"
grep -q '^batch-precheck: FAIL make capabilities-check .* introduced' "$tmp/out" || fail "head-only failure: $(cat "$tmp/out")"
grep -q '^batch-precheck: failed: make capabilities-check,make test-commands-check$' "$tmp/out" || fail "failed list: $(cat "$tmp/out")"
grep -q "^batch-precheck: WARNING 1 base-present failure(s) at $basesha, not counted: make test-read-scopes-check$" "$tmp/out" ||
  fail "warning summary: $(cat "$tmp/out")"
[[ $(git -C "$crepo" worktree list --porcelain | grep -c '^worktree ') == 1 ]] || fail "base worktree left behind"
# Only base-present failures: the run passes and still warns.
printf 'unlisted flag --old\n' > "$crepo/fail-test-commands-check"
/bin/rm -f "$crepo/fail-capabilities-check"
crun --base "$basesha" || fail "base-present only failed the run: $(cat "$tmp/out")"
grep -q '^batch-precheck: WARN make test-commands-check' "$tmp/out" || fail "second warn: $(cat "$tmp/out")"
grep -q '^batch-precheck: WARNING 2 base-present' "$tmp/out" || fail "two warnings: $(cat "$tmp/out")"
# On this clean checkout a base-present WARN still writes no receipt: not every step passed.
grep -q '^batch-precheck: no receipt: 2 step(s) failed, though only base-present$' "$tmp/out" || fail "warn receipt: $(cat "$tmp/out")"
[[ -f $crepo/.git/batch-precheck-receipt ]] && fail "a run with base-present failures wrote a receipt"
# A new finding that differs from a base one only by digits is introduced, not a timing.
printf 'cmd/v2schema.go\n' >> "$crepo/fail-test-read-scopes-check"
crun --base "$basesha" && fail "v2schema hidden as a timing"
grep -q '^batch-precheck: FAIL make test-read-scopes-check .* introduced' "$tmp/out" || fail "digit finding: $(cat "$tmp/out")"
# With GIT_DIR/GIT_WORK_TREE exported, a head-only failure is still introduced.
printf 'catalog drift\n' > "$crepo/fail-capabilities-check"
GIT_DIR=$crepo/.git GIT_WORK_TREE=$crepo crun --base "$basesha" && fail "GIT_DIR run passed"
grep -q '^batch-precheck: FAIL make capabilities-check .* introduced' "$tmp/out" || fail "GIT_DIR leak: $(cat "$tmp/out")"
grep -q '^batch-precheck: WARN make test-commands-check' "$tmp/out" || fail "GIT_DIR base-present: $(cat "$tmp/out")"
[[ $(git -C "$crepo" worktree list --porcelain | grep -c '^worktree ') == 1 ]] || fail "base worktree left behind"
# Without --base the same failures still fail; an unresolvable base classifies nothing as base-present.
crun && fail "no-base run passed with failures"
crun --base no-such-rev && fail "bad base passed"
grep -q '^batch-precheck: FAIL make test-read-scopes-check .* introduced: base no-such-rev is not a commit' "$tmp/out" ||
  fail "bad base reason: $(cat "$tmp/out")"
crun --base > /dev/null 2>&1
[[ $? == 2 ]] || fail "--base without a value did not exit 2"

# Only volatile tokens are erased: a changed number in a diagnostic is introduced, while a changed
# timing on a go test result line, a temporary path and a pid are not.
printf 'timeout limit 2s exceeded\n--- FAIL: TestX (0.01s)\nread /tmp/TestX111/001/f: denied\npid 41 exited\n' > "$crepo/fail-test-commands-check"
/bin/rm -f "$crepo/fail-capabilities-check" "$crepo/fail-test-read-scopes-check"
git -C "$crepo" add -A && git -C "$crepo" -c user.name=t -c user.email=t@t commit -qm base2 || fail "base2 commit"
base2=$(git -C "$crepo" rev-parse HEAD)
printf 'timeout limit 2s exceeded\n--- FAIL: TestX (0.73s)\nread /tmp/TestX999/001/f: denied\npid 97 exited\n' > "$crepo/fail-test-commands-check"
crun --base "$base2" || fail "volatile tokens made a base-present failure introduced: $(cat "$tmp/out")"
grep -q '^batch-precheck: WARN make test-commands-check' "$tmp/out" || fail "volatile warn: $(cat "$tmp/out")"
printf 'timeout limit 3s exceeded\n--- FAIL: TestX (0.01s)\n' > "$crepo/fail-test-commands-check"
crun --base "$base2" && fail "a changed diagnostic number was classified base-present"
grep -q '^batch-precheck: FAIL make test-commands-check .* introduced' "$tmp/out" || fail "diagnostic number: $(cat "$tmp/out")"

# 7. Drift and catalog selection under a base (ASD-V0-050, ASD-V0-051; V1-3134, V1-3136), on a
# real Go fixture module, a stub planner and a stub make: a diff touching no drift or catalog input
# runs no go test and skips the catalog check, printing the skipped set; a changed golden runs its
# package and fails on the real drift; a changed declared spec runs its test without the plan; a
# changed catalog input runs the catalog check, which still fails on drift; an UNKNOWN, failed,
# malformed or missing plan, a new frontier kind, a quoted path and a missing base run everything,
# as do a plan bound to another base, an unmappable or empty selection under a Go change; a quoted
# module directive still maps.
srepo=$tmp/srepo
mkdir -p "$srepo/.github/workflows" "$srepo/interop/cem01-go" "$srepo/conformance/use-cases-v0" "$srepo/golden/testdata" "$srepo/other" "$srepo/docs/specs"
cp "$repo/.github/workflows/ci.yml" "$srepo/.github/workflows/ci.yml"
printf 'module example.test/sel\n\ngo 1.21\n' > "$srepo/go.mod"
printf 'package main\n\nfunc main() {}\n' > "$srepo/conformance/use-cases-v0/main.go"
cat > "$srepo/golden/golden_test.go" <<'EOF2'
package golden

import (
	"os"
	"strings"
	"testing"
)

func TestGoldenDrift(t *testing.T) {
	b, err := os.ReadFile("testdata/want.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != "v1" {
		t.Fatalf("golden drift: want v1, got %s", got)
	}
}

func TestFreeze(t *testing.T) {}
EOF2
printf 'v1\n' > "$srepo/golden/testdata/want.txt"
printf 'package other\n\nimport "testing"\n\nfunc TestOtherDrift(t *testing.T) {}\n' > "$srepo/other/other_test.go"
printf '# freeze\n' > "$srepo/docs/specs/freeze.md"
printf '# guide\n' > "$srepo/docs/guide.md"
printf 'capability-catalog-check:\n\t$(GO_TEST_COMMAND) -run x ./golden\n' > "$srepo/Makefile"
git -C "$srepo" init -q && git -C "$srepo" add -A && git -C "$srepo" commit -qm base || fail "selection fixture commit"
sbase=$(git -C "$srepo" rev-parse HEAD)
sbin=$tmp/sbin
mkdir -p "$sbin"
printf '#!/bin/sh\necho "make $*" >> "%s/scalls"\n[ "$2" = capability-catalog-check ] && [ -n "${STUB_CATALOG_FAIL:-}" ] && grep -q changed docs/guide.md && { echo "catalog drift"; exit 1; }\nexit 0\n' "$tmp" > "$sbin/make"
printf '#!/bin/sh\nexit 0\n' > "$sbin/gofmt"
printf '#!/bin/sh\necho "planner $*" >> "%s/scalls"\n[ -z "${STUB_PLAN_RC:-}" ] || { echo "{\\"ok\\":false,\\"code\\":\\"boom\\"}"; exit "$STUB_PLAN_RC"; }\ncat "%s/plan.json"\n' "$tmp" "$tmp" > "$tmp/planner"
chmod +x "$sbin"/* "$tmp/planner"
frontier='{"reason":"LANGUAGE_FRONTIER","detail":"go:build-constraint-variants"},{"reason":"LANGUAGE_FRONTIER","detail":"shell:foreign-invocation-unmodelled"}'
# plan_sel UNKNOWNS SELECTED writes a runnable affected-plan/1 bound to $pbase (default the
# fixture base) with the raw selected entries; plan UNKNOWNS PKG... selects each PKG.
plan_sel() {
  printf '{"ok":true,"profile":"affected-plan/1","range":{"base":"%s"},"summary":{"line":"scope=UNKNOWN"},"provider":{"go":{"state":"RUNNABLE"}},"plan":{"selected":[%s],"unknown":[%s]}}\n' "${pbase:-$sbase}" "$2" "$1" > "$tmp/plan.json"
}
plan() {
  local sel='' p
  for p in "${@:2}"; do sel+="${sel:+,}{\"unitId\":\"go:example.test/sel/$p\",\"witness\":{\"kind\":\"UNBOUNDED_READER\"}}"; done
  plan_sel "$1" "$sel"
}
sdrift='./golden TestGoldenDrift @go
./other TestOtherDrift
./golden TestFreeze @go docs/specs/freeze.md'
srun() { (unset GOFLAGS; PATH=$sbin:$PATH BATCH_PRECHECK_ROOT=$srepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_PLANNER=${planner-$tmp/planner} BATCH_PRECHECK_DRIFT=$sdrift "$pre" "$@") > "$tmp/out" 2>&1; }
row_is() { grep -q "^batch-precheck: | $1 | *[0-9]*s | $2\$" "$tmp/out" || fail "row $1 $2: $(cat "$tmp/out")"; }
# A path outside every drift and catalog input: nothing runs, the skipped set is printed.
plan "$frontier" golden
mkdir -p "$srepo/notes" && printf 'n\n' > "$srepo/notes/a.txt"
: > "$tmp/scalls"
STUB_CATALOG_FAIL=1 srun --base "$sbase" || fail "an unreachable diff failed: $(cat "$tmp/out")"
grep -qx "planner affected --base $sbase" "$tmp/scalls" || fail "planner not asked with the full base: $(cat "$tmp/scalls")"
grep -q "^batch-precheck: batch-drift runs 0 of 3 drift tests for the diff since $sbase (1 changed paths)\$" "$tmp/out" || fail "zero selection line: $(cat "$tmp/out")"
grep -qx 'batch-precheck:   skipped ./golden TestGoldenDrift (no Go input and no declared input changed)' "$tmp/out" || fail "skipped golden: $(cat "$tmp/out")"
grep -qx 'batch-precheck:   skipped ./other TestOtherDrift (the plan does not select its package)' "$tmp/out" || fail "skipped other: $(cat "$tmp/out")"
grep -qx 'batch-precheck:   skipped ./golden TestFreeze (no Go input and no declared input changed)' "$tmp/out" || fail "skipped freeze: $(cat "$tmp/out")"
grep -q "^batch-precheck: capability-catalog-check skipped: no catalog input changed since $sbase" "$tmp/out" || fail "catalog not skipped: $(cat "$tmp/out")"
row_is PASS 'batch-drift (0 of 3 tests)'
row_is PASS 'make capability-catalog-check'
grep -q 'capability-catalog-check' "$tmp/scalls" && fail "a skipped catalog check ran: $(cat "$tmp/scalls")"
# ASD-V0-912 (V1-3804): the non-Go frontier details every real plan carries (the TypeScript,
# C-family, .NET, Python, Ruby and Swift adapters' own gaps) do not widen a doc-only diff.
plan "$frontier"',{"reason":"LANGUAGE_FRONTIER","detail":"typescript:path-alias-unresolved"},{"reason":"LANGUAGE_FRONTIER","detail":"cfamily:macro-expansion"},{"reason":"LANGUAGE_FRONTIER","detail":"dotnet:reflection"},{"reason":"LANGUAGE_FRONTIER","detail":"python:dynamic-import"},{"reason":"LANGUAGE_FRONTIER","detail":"ruby:metaprogramming"},{"reason":"LANGUAGE_FRONTIER","detail":"swift:macro"}' golden
srun --base "$sbase" || fail "a doc-only diff with non-Go frontier details failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: batch-drift runs 0 of 3 drift tests for the diff since $sbase" "$tmp/out" ||
  fail "non-Go frontier details widened a doc-only diff: $(cat "$tmp/out")"
plan "$frontier" golden
# ASD-V0-892 (V1-3572, V1-3818): the skipped catalog check is a print-only row, which never goes
# through the ledger: on a rerun on identical inputs it is a PASS while the listed steps reuse.
sledger() { CORVINT_GATE_LEDGER='' CORVINT_GATE_LEDGER_DIR=$tmp/sledger BATCH_PRECHECK_LEDGER=$tmp/gate-ledger srun "$@"; }
sledger --base "$sbase" || fail "first selection ledger run failed: $(cat "$tmp/out")"
sledger --base "$sbase" || fail "selection ledger rerun failed: $(cat "$tmp/out")"
row_is PASS 'make capability-catalog-check'
row_is REUSE 'sync-check'
grep -q '^batch-precheck: | REUSE | *[0-9]*s | make capability-catalog-check$' "$tmp/out" && fail "print-only row went through the ledger: $(cat "$tmp/out")"
# ASD-V0-640 (V1-3744): no script/, Makefile, docs/DOGFOOD.md or benchmarks/retained-data.tsv path
# changed, so lane-scripts-test is a print-only skip; a new file under script/ runs it, and so does a
# change to the retained-data manifest alone, which the ASD-V0-646 group validates.
grep -q "^batch-precheck: lane-scripts-test skipped: no lane-scripts-test input (script/, Makefile, docs/DOGFOOD.md, benchmarks/retained-data.tsv) changed since $sbase\$" "$tmp/out" || fail "lane-scripts-test not skipped: $(cat "$tmp/out")"
row_is PASS 'make lane-scripts-test'
grep -q 'lane-scripts-test' "$tmp/scalls" && fail "a skipped lane-scripts-test ran: $(cat "$tmp/scalls")"
mkdir -p "$srepo/script" && printf '#!/bin/sh\n' > "$srepo/script/new.sh"
: > "$tmp/scalls"
srun --base "$sbase" || fail "a script/ change failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: lane-scripts-test runs (a script/, Makefile, docs/DOGFOOD.md or benchmarks/retained-data.tsv path changed since $sbase)\$" "$tmp/out" || fail "lane-scripts-test reason: $(cat "$tmp/out")"
grep -qx 'make -s lane-scripts-test' "$tmp/scalls" || fail "lane-scripts-test not run for a script/ change: $(cat "$tmp/scalls")"
/bin/rm -rf "$srepo/script"
mkdir -p "$srepo/benchmarks" && printf 'id\n' > "$srepo/benchmarks/retained-data.tsv"
: > "$tmp/scalls"
srun --base "$sbase" || fail "a retained-data manifest change failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: lane-scripts-test runs (a script/, Makefile, docs/DOGFOOD.md or benchmarks/retained-data.tsv path changed since $sbase)\$" "$tmp/out" ||
  fail "lane-scripts-test skipped for a manifest-only change: $(cat "$tmp/out")"
grep -qx 'make -s lane-scripts-test' "$tmp/scalls" || fail "lane-scripts-test not run for a manifest-only change: $(cat "$tmp/scalls")"
/bin/rm -rf "$srepo/benchmarks"
# A changed golden (a non-Go file in a Go package directory) runs that package and fails on the
# real drift, introduced since the base.
printf 'v2\n' > "$srepo/golden/testdata/want.txt"
srun --base "$sbase" && fail "real golden drift passed"
grep -q "^batch-precheck: batch-drift runs 2 of 3 drift tests" "$tmp/out" || fail "golden selection: $(cat "$tmp/out")"
row_is FAIL 'batch-drift (1 packages, 2 of 3 tests)'
grep -q 'golden drift: want v1, got v2' "$tmp/out" || fail "drift output not shown: $(cat "$tmp/out")"
grep -q '^batch-precheck: FAIL batch-drift .* introduced' "$tmp/out" || fail "drift not introduced: $(cat "$tmp/out")"
grep -q '^batch-precheck: capability-catalog-check runs (a catalog input changed' "$tmp/out" || fail "catalog not run on a Go input: $(cat "$tmp/out")"
git -C "$srepo" checkout -q -- golden/testdata/want.txt
# A changed declared spec runs its test even when the plan selects nothing; a nested docs path is
# not a top-level docs/*.md catalog input.
plan "$frontier"
printf 'changed\n' >> "$srepo/docs/specs/freeze.md"
srun --base "$sbase" || fail "declared spec run failed: $(cat "$tmp/out")"
row_is PASS 'batch-drift (1 packages, 1 of 3 tests)'
grep -q '^batch-precheck: capability-catalog-check skipped' "$tmp/out" || fail "nested docs path ran the catalog: $(cat "$tmp/out")"
git -C "$srepo" checkout -q -- docs/specs/freeze.md
# A changed top-level doc runs the catalog check, which still fails on drift.
printf 'changed\n' >> "$srepo/docs/guide.md"
: > "$tmp/scalls"
STUB_CATALOG_FAIL=1 srun --base "$sbase" && fail "catalog drift passed: $(cat "$tmp/out")"
row_is FAIL 'make capability-catalog-check'
grep -q '^batch-precheck: FAIL make capability-catalog-check .* introduced' "$tmp/out" || fail "catalog drift not introduced: $(cat "$tmp/out")"
grep -q '^make -s capability-catalog-check$' "$tmp/scalls" || fail "catalog not run: $(cat "$tmp/scalls")"
grep -q '^  | catalog drift' "$tmp/out" || fail "catalog output: $(cat "$tmp/out")"
git -C "$srepo" checkout -q -- docs/guide.md
# Everything runs, naming why, for an UNKNOWN plan, a new frontier kind, a failing, malformed or
# missing planner, a path that needs quoting and no base.
full() {
  grep -q "^batch-precheck: batch-drift runs every drift test and capability-catalog-check runs (.*$1" "$tmp/out" || fail "full union for $1: $(cat "$tmp/out")"
  row_is PASS 'batch-drift (2 packages)'
  grep -q '^make -s capability-catalog-check$' "$tmp/scalls" || fail "catalog skipped for $1: $(cat "$tmp/scalls")"
}
run_full() { : > "$tmp/scalls"; srun "$@" || fail "full run failed: $(cat "$tmp/out")"; }
plan '{"reason":"UNOWNED_DIRTY_PATH","detail":"notes/a.txt"}' golden
run_full --base "$sbase"; full 'the plan has 1 unknowns, first: UNOWNED_DIRTY_PATH notes/a.txt; all in '
# ASD-V0-893 (V1-3260): many unknowns print a count, the first and the retained file, not the list.
many=$(for i in $(seq 1 300); do printf ',{"reason":"LANGUAGE_FRONTIER","detail":"go:fake-frontier %s"}' "$i"; done)
plan "$frontier$many" golden
run_full --base "$sbase"; full 'the plan has 300 unknowns, first: LANGUAGE_FRONTIER go:fake-frontier 1; all in '
ufile=$(git -C "$srepo" rev-parse --path-format=absolute --git-path batch-precheck-unknowns.txt)
[[ $(grep -c . "$ufile") == 300 ]] && grep -qx 'LANGUAGE_FRONTIER go:fake-frontier 299' "$ufile" &&
  grep -qF "all in $ufile)" "$tmp/out" || fail "unknowns file $ufile: $(head -c 300 "$ufile" 2>&1)"
(($(awk '{ if (length > m) m = length } END { print m + 0 }' "$tmp/out") < 600)) || fail "an output line still carries the unknown list"
plan "$frontier"',{"reason":"LANGUAGE_FRONTIER","detail":"go:cgo"}' golden
run_full --base "$sbase"; full 'LANGUAGE_FRONTIER go:cgo'
plan "$frontier" golden
STUB_PLAN_RC=3 run_full --base "$sbase"; full 'affected exited 3'
printf '{"ok":true,"profile":"affected-plan/2"}\n' > "$tmp/plan.json"
run_full --base "$sbase"; full 'the plan is not a runnable affected-plan/1'
plan "$frontier" golden
planner=$tmp/no-such-planner run_full --base "$sbase"; full 'no-such-planner affected exited 127'
# A plan bound to another base, an entry without a string unitId, a Go unit outside the module, and
# an empty selection while a Go input changed each run everything (the review's P1 cases).
pbase=0123456789abcdef0123456789abcdef01234567 plan "$frontier" golden
run_full --base "$sbase"; full "the plan is bound to base 0123456789abcdef0123456789abcdef01234567, not $sbase"
plan_sel "$frontier" '{}'
run_full --base "$sbase"; full 'the plan selection cannot be mapped: an entry without a string unitId'
plan_sel "$frontier" '{"unitId":7},{"unitId":"go:example.test/sel/golden"}'
run_full --base "$sbase"; full 'the plan selection cannot be mapped: an entry without a string unitId'
plan_sel "$frontier" '{"unitId":"go:example.test/elsewhere/x"},{"unitId":"go:example.test/sel/golden"}'
run_full --base "$sbase"; full 'go unit go:example.test/elsewhere/x is outside module example.test/sel'
plan "$frontier"
printf 'package other\n' > "$srepo/other/extra.go"
run_full --base "$sbase"; full 'a Go input changed but the plan selects no Go package'
/bin/rm -f "$srepo/other/extra.go"
# A quoted module directive (P2) still maps the plan's units into the module.
printf 'module "example.test/sel"\n\ngo 1.21\n' > "$srepo/go.mod"
plan "$frontier" golden
srun --base "$sbase" || fail "quoted module run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: batch-drift runs 2 of 3 drift tests' "$tmp/out" || fail "quoted module selection: $(cat "$tmp/out")"
grep -qx 'batch-precheck:   skipped ./other TestOtherDrift (the plan does not select its package)' "$tmp/out" || fail "quoted module skip: $(cat "$tmp/out")"
git -C "$srepo" checkout -q -- go.mod
printf 'q\n' > "$srepo/notes/a\"b.txt"
run_full --base "$sbase"; full 'a changed path needs quoting'
/bin/rm -f "$srepo/notes/a\"b.txt"
run_full; full 'no --base or BATCH_PRECHECK_BASE'
/bin/rm -rf "$srepo/notes"
# ASD-V0-904 (V1-3441): a docs-only diff under a planner that cannot narrow the run still skips each
# @go entry, naming why, unless a declared input of it changed (TestFreeze's spec); a Go change runs
# them again.
printf 'changed\n' >> "$srepo/docs/specs/freeze.md"
STUB_PLAN_RC=3 srun --base "$sbase" || fail "docs-only fallback run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: batch-drift runs every drift test and capability-catalog-check runs (.*affected exited 3' "$tmp/out" || fail "docs-only fallback reason: $(cat "$tmp/out")"
grep -qx "batch-precheck:   NOT_RUN ./golden TestGoldenDrift (docs-only diff: no Go input changed since $sbase)" "$tmp/out" || fail "docs-only golden not skipped: $(cat "$tmp/out")"
grep -q 'NOT_RUN ./golden TestFreeze' "$tmp/out" && fail "docs-only diff skipped TestFreeze, whose declared input changed: $(cat "$tmp/out")"
row_is PASS 'batch-drift (2 packages, 2 of 3 tests)'
printf 'package golden\n' > "$srepo/golden/extra.go"
: > "$tmp/scalls"
STUB_PLAN_RC=3 srun --base "$sbase" || fail "Go fallback run failed: $(cat "$tmp/out")"
grep -q 'NOT_RUN' "$tmp/out" && fail "a Go change skipped an @go entry: $(cat "$tmp/out")"
row_is PASS 'batch-drift (2 packages)'
/bin/rm -f "$srepo/golden/extra.go"
# A docs change beside many non-documentation paths is not docs-only: the check must not read a
# pipe whose writer a SIGPIPE killed under pipefail as "no such path" (review of V1-3441).
mkdir -p "$srepo/notes"
(cd "$srepo/notes" && seq -f 'n%05g.txt' 1 10000 | xargs touch)
: > "$tmp/scalls"
STUB_PLAN_RC=3 srun --base "$sbase" || fail "many-path fallback run failed: $(cat "$tmp/out")"
grep -q 'NOT_RUN' "$tmp/out" && fail "10000 non-documentation paths skipped an @go entry: $(cat "$tmp/out")"
/bin/rm -rf "$srepo/notes"
git -C "$srepo" checkout -q -- docs/specs/freeze.md

# 8. ASD-V0-046 (V1-3122): a failed step reruns once before it is classified. A pass on the rerun
# is FLAKY (counted passed, both attempts named, attempt 1's failing test named); a second failure
# names both attempts and the failing go subtest and node test names.
frepo=$tmp/frepo fbin=$tmp/fbin
mkdir -p "$frepo/.github/workflows" "$frepo/pkg" "$frepo/interop/cem01-go" "$fbin" "$tmp/fmark"
cp "$repo/.github/workflows/ci.yml" "$frepo/.github/workflows/ci.yml"
cp "$repo/pkg/a_test.go" "$frepo/pkg/a_test.go"
cat > "$fbin/make" <<'EOF'
#!/bin/sh
top=$(git rev-parse --show-toplevel)
if [ -f "$top/flaky-$2" ] && [ ! -f "$FLAKY_MARK/$2" ]; then
  : > "$FLAKY_MARK/$2"; printf -- '--- FAIL: TestFlaky (0.01s)\n    --- FAIL: TestFlaky/sub (0.01s)\n'; exit 1
fi
if [ -f "$top/hostflaky-$2" ] && [ ! -f "$FLAKY_MARK/host-$2" ]; then
  : > "$FLAKY_MARK/host-$2"; printf 'write /x/sort1: No space left on device\n'; exit 1
fi
[ -f "$top/fail-$2" ] || exit 0
[ -s "$top/fail-$2" ] && cat "$top/fail-$2"
printf -- '--- FAIL: TestHard (0.01s)\nnot ok 3 - node case\n'; exit 1
EOF
printf '#!/bin/sh\nexit 0\n' > "$fbin/gofmt"
printf '#!/bin/sh\necho ok\n' > "$fbin/go"
chmod +x "$fbin"/*
: > "$frepo/flaky-test-read-scopes-check"
: > "$frepo/fail-test-commands-check"
git -C "$frepo" init -q && git -C "$frepo" add -A &&
  git -C "$frepo" -c user.name=t -c user.email=t@t commit -qm base || fail "flaky fixture commit"
frun() { PATH=$fbin:$PATH FLAKY_MARK=$tmp/fmark BATCH_PRECHECK_ROOT=$frepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_DRIFT="./pkg TestDrift" "$pre" "$@" > "$tmp/out" 2>&1; }
frun
[[ $? == 1 ]] || fail "a hard failure passed after its rerun: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: FLAKY make test-read-scopes-check: attempt 1 failed rc=1 \([0-9]+s\), failing: TestFlaky, TestFlaky/sub; attempt 2 passed \([0-9]+s\)$' "$tmp/out" ||
  fail "flaky step not classified with both attempts: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: FAIL make test-commands-check: attempt 1 rc=1 \([0-9]+s\), failing: TestHard, node case; attempt 2 rc=1 \([0-9]+s\), failing: TestHard, node case$' "$tmp/out" ||
  fail "hard failure not rerun with both attempts: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: \| FLAKY \| +[0-9]+s \| make test-read-scopes-check$' "$tmp/out" || fail "no FLAKY row: $(cat "$tmp/out")"
grep -qx '  | attempt 1: --- FAIL: TestFlaky (0.01s)' "$tmp/out" || fail "FLAKY attempt 1 output not shown (ASD-V0-055): $(cat "$tmp/out")"
grep -q '^batch-precheck: 1 step(s) FLAKY, passed only on their one rerun: make test-read-scopes-check$' "$tmp/out" || fail "no FLAKY summary: $(cat "$tmp/out")"
grep -q '^batch-precheck: failing tests: TestHard, node case$' "$tmp/out" || fail "failing test names not printed: $(cat "$tmp/out")"
grep -q '^batch-precheck: failed: make test-commands-check$' "$tmp/out" || fail "flaky step counted failed: $(cat "$tmp/out")"
/bin/rm -f "$tmp/fmark"/*
git -C "$frepo" rm -q fail-test-commands-check && git -C "$frepo" -c user.name=t -c user.email=t@t commit -qm pass || fail "flaky pass commit"
frun || fail "a flaky-only run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: FLAKY make test-read-scopes-check' "$tmp/out" || fail "flaky-only run: $(cat "$tmp/out")"

# 9. ASD-V0-054: a failure naming an exhausted host resource is a host-resource FAIL, never
# introduced, and is not rerun at the base (whose own run here passes, so a comparison would say
# introduced); a FLAKY pass names the resource; a TMPDIR below the free-space floor warns first.
fbase=$(git -C "$frepo" rev-parse HEAD)
printf 'compile: writing output: write $WORK/b1/_pkg_.a: no space left on device\n' > "$frepo/fail-test-commands-check"
: > "$frepo/hostflaky-capabilities-check"
git -C "$frepo" add -A && git -C "$frepo" -c user.name=t -c user.email=t@t commit -qm host || fail "host fixture commit"
/bin/rm -f "$tmp/fmark"/*
BATCH_PRECHECK_MIN_TMP_KIB=999999999999 frun --base "$fbase"
[[ $? == 1 ]] || fail "a host-resource failure passed: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: WARN low-tmp-space avail=[0-9]+KiB under .*set TMPDIR to a directory with room$' "$tmp/out" ||
  fail "low TMPDIR space not warned: $(cat "$tmp/out")"
grep -Fq "batch-precheck: FAIL make test-commands-check: attempt 1 rc=1" "$tmp/out" || fail "host failure not rerun: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: FAIL make test-commands-check \([0-9]+s\) host-resource: compile: writing output: write \$WORK/b1/_pkg_\.a: no space left on device; not compared with '"$fbase"': free disk or memory' "$tmp/out" ||
  fail "host-resource failure not named: $(cat "$tmp/out")"
grep -q 'make test-commands-check .* introduced' "$tmp/out" && fail "host-resource failure called introduced: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: FLAKY make capabilities-check: attempt 1 failed rc=1 \([0-9]+s\), host-resource: write /x/sort1: No space left on device; attempt 2 passed' "$tmp/out" ||
  fail "flaky host-resource pass not named: $(cat "$tmp/out")"
# A floor of 1 KiB, so the case does not depend on the host's free space (/ had 3.8 GB on Unraid).
BATCH_PRECHECK_MIN_TMP_KIB=1 frun --base "$fbase"
grep -q 'low-tmp-space' "$tmp/out" && fail "low-tmp-space warned above the floor: $(cat "$tmp/out")"

# 10. ASD-V0-055 (V1-3205, V1-3206): step scratch stays off a RAM-backed TMPDIR. BATCH_PRECHECK_TMPDIR
# is every step's TMPDIR; unset, a tmpfs TMPDIR (Linux /dev/shm here) becomes
# XDG_CACHE_HOME/corvint-tmp, named in one line; a disk TMPDIR is kept.
printf '#!/bin/sh\nprintf "%%s\\n" "$TMPDIR" >> "%s/step-tmpdirs"\nexit 0\n' "$tmp" > "$fbin/make"
: > "$tmp/step-tmpdirs"
frun || fail "BATCH_PRECHECK_TMPDIR run failed: $(cat "$tmp/out")"
# A mismatch prints both values byte-visibly (V1-3464): equal-looking paths differed by `//`.
step_tmp_diff() { diff <(printf '%s\n' "$1" | od -c) <(sort -u "$tmp/step-tmpdirs" | od -c) | head -n 20; }
[[ $(sort -u "$tmp/step-tmpdirs") == "$tmp/step-tmp" ]] ||
  fail "steps did not use BATCH_PRECHECK_TMPDIR: expected $tmp/step-tmp, got $(sort -u "$tmp/step-tmpdirs"); od -c diff (< expected, > got): $(step_tmp_diff "$tmp/step-tmp")"
grep -q 'RAM-backed' "$tmp/out" && fail "an explicit BATCH_PRECHECK_TMPDIR was replaced: $(cat "$tmp/out")"
if [[ $(uname -s) == Linux && $(stat -f -c %T /dev/shm 2> /dev/null) == tmpfs ]]; then
  : > "$tmp/step-tmpdirs"
  (unset BATCH_PRECHECK_TMPDIR; TMPDIR=/dev/shm XDG_CACHE_HOME=$tmp/xdg frun) || fail "tmpfs TMPDIR run failed: $(cat "$tmp/out")"
  grep -qx "batch-precheck: TMPDIR /dev/shm is RAM-backed; steps use $tmp/xdg/corvint-tmp (BATCH_PRECHECK_TMPDIR chooses another)" "$tmp/out" ||
    fail "tmpfs TMPDIR not replaced: $(cat "$tmp/out")"
  [[ $(sort -u "$tmp/step-tmpdirs") == "$tmp/xdg/corvint-tmp" ]] || fail "steps kept the tmpfs TMPDIR: $(sort -u "$tmp/step-tmpdirs")"
  [[ $(stat -c %a "$tmp/xdg/corvint-tmp") == 700 ]] || fail "step TMPDIR mode $(stat -c %a "$tmp/xdg/corvint-tmp")"
  : > "$tmp/step-tmpdirs"
  (unset BATCH_PRECHECK_TMPDIR XDG_CACHE_HOME HOME; TMPDIR=/dev/shm CORVINT_GOCACHE=$tmp/gocache frun) || fail "a run without HOME failed: $(cat "$tmp/out")"
  [[ $(sort -u "$tmp/step-tmpdirs") == /dev/shm ]] || fail "without HOME the TMPDIR was not kept: $(sort -u "$tmp/step-tmpdirs")"
  : > "$tmp/step-tmpdirs"
  mkdir -p "$tmp/disk-tmp"
  if [[ $(stat -f -c %T "$tmp/disk-tmp") != tmpfs ]]; then
    (unset BATCH_PRECHECK_TMPDIR; TMPDIR=$tmp/disk-tmp XDG_CACHE_HOME=$tmp/xdg2 frun) || fail "disk TMPDIR run failed: $(cat "$tmp/out")"
    [[ $(sort -u "$tmp/step-tmpdirs") == "$tmp/disk-tmp" ]] || fail "a disk TMPDIR was replaced: $(sort -u "$tmp/step-tmpdirs")"
  fi
fi

# 10b. ASD-V0-781 (V1-4016): an exported GOTMPDIR that does not exist is created before
# any step runs, so a go step does not fail with "GOTMPDIR does not exist".
/bin/cp -f "$fbin/make" "$tmp/make.saved"
printf '#!/bin/sh\n[ -d "$GOTMPDIR" ] || { echo "make stub: GOTMPDIR $GOTMPDIR missing" >&2; exit 1; }\nexit 0\n' > "$fbin/make"
GOTMPDIR=$tmp/gotmp/missing frun || fail "a missing GOTMPDIR failed the run: $(cat "$tmp/out")"
[[ -d $tmp/gotmp/missing ]] || fail "missing GOTMPDIR was not created"
/bin/cp -f "$tmp/make.saved" "$fbin/make"

# 13. ASD-V0-109 (V1-3267): a sealed CEM the branch adds must be named for a commit in HEAD's
# history. A resolvable name passes; an unresolvable one fails naming the file and the reseal
# recovery; an unreachable name the base already holds is not judged; with no base and no
# origin/main the check is skipped with its reason.
srepo=$tmp/srepo
mkdir -p "$srepo/.github/workflows" "$srepo/pkg" "$srepo/interop/cem01-go" "$srepo/.corvint/changes"
cp "$repo/.github/workflows/ci.yml" "$srepo/.github/workflows/ci.yml"
cp "$repo/pkg/a_test.go" "$srepo/pkg/a_test.go"
inherited=1111111111111111111111111111111111111111
: > "$srepo/.corvint/changes/$inherited.cem.json"
git -C "$srepo" init -q && git -C "$srepo" add -A && git -C "$srepo" commit -qm base || fail "seal fixture base commit"
sbase=$(git -C "$srepo" rev-parse HEAD)
srun() { PATH=$fbin:$PATH FLAKY_MARK=$tmp/fmark BATCH_PRECHECK_ROOT=$srepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_PLANNER= BATCH_PRECHECK_DRIFT="./pkg TestDrift" "$pre" "$@" > "$tmp/out" 2>&1; }
srun || fail "seal fixture without a base failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: SKIP sealed-cem-names (no --base and no merge base with origin/main)$' "$tmp/out" || fail "no-base seal check not skipped: $(cat "$tmp/out")"
: > "$srepo/.corvint/changes/$sbase.cem.json"
git -C "$srepo" add -A && git -C "$srepo" commit -qm "seal a real commit" || fail "resolvable seal commit"
srun --base "$sbase" || fail "a resolvable seal name failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: PASS sealed-cem-names$' "$tmp/out" || fail "resolvable seal not checked: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: \| PASS \| +[0-9]+s \| sealed-cem-names$' "$tmp/out" || fail "no sealed-cem-names row: $(cat "$tmp/out")"
ghost=87a5ef5d00000000000000000000000000000000
: > "$srepo/.corvint/changes/$ghost.cem.json"
git -C "$srepo" add -A && git -C "$srepo" commit -qm "seal from another lineage" || fail "unresolvable seal commit"
srun --base "$sbase"
[[ $? == 1 ]] || fail "an unresolvable seal name passed: $(cat "$tmp/out")"
grep -q "^  | sealed-cem-names: .corvint/changes/$ghost.cem.json names $ghost, which is not a commit in HEAD's history" "$tmp/out" ||
  fail "refusal does not name the file: $(cat "$tmp/out")"
grep -q "^  | sealed-cem-names: recover: git rm each named tracked file and commit$" "$tmp/out" &&
  grep -q "^  | sealed-cem-names: recover: then rebind and reseal this branch against its base: make dogfood-finish LANE=1 BASE=$sbase " "$tmp/out" ||
  fail "refusal does not name the reseal recovery: $(cat "$tmp/out")"
grep -q 'rm each named untracked file' "$tmp/out" && fail "an untracked recovery for a committed seal: $(cat "$tmp/out")"
grep -q "$sbase.cem.json names\|$inherited" "$tmp/out" && fail "a resolvable or base-held seal was refused: $(cat "$tmp/out")"
grep -q '^batch-precheck: failed: sealed-cem-names$' "$tmp/out" || fail "sealed-cem-names not in the failed list: $(cat "$tmp/out")"
git -C "$srepo" rm -q ".corvint/changes/$ghost.cem.json" && : > "$srepo/.corvint/changes/q\"uote.cem.json" &&
  git -C "$srepo" add -A && git -C "$srepo" commit -qm "quoted seal name" || fail "quoted seal commit"
srun --base "$sbase"
[[ $? == 1 ]] && grep -q 'sealed-cem-names: ".corvint/changes/q\\"uote.cem.json" needs quoting' "$tmp/out" || fail "a quoted seal name was not refused: $(cat "$tmp/out")"
# A quoted name that is not a sealed CEM is not judged.
git -C "$srepo" rm -q ".corvint/changes/q\"uote.cem.json" && : > "$srepo/.corvint/changes/review\"notes.txt" &&
  git -C "$srepo" add -A && git -C "$srepo" commit -qm "quoted note" || fail "quoted note commit"
srun --base "$sbase" || fail "a quoted non-CEM name failed sealed-cem-names: $(cat "$tmp/out")"
grep -q '^batch-precheck: PASS sealed-cem-names$' "$tmp/out" || fail "quoted non-CEM name: $(cat "$tmp/out")"
# An untracked stale seal names the rm recovery, not git rm.
: > "$srepo/.corvint/changes/$ghost.cem.json"
srun --base "$sbase"
[[ $? == 1 ]] || fail "an untracked unresolvable seal passed: $(cat "$tmp/out")"
grep -q "names $ghost, which is not a commit in HEAD's history (a seal made on another lineage, such as before a rewrite; untracked)" "$tmp/out" &&
  grep -q '^  | sealed-cem-names: recover: rm each named untracked file$' "$tmp/out" ||
  fail "untracked seal recovery not named: $(cat "$tmp/out")"
grep -q 'git rm each named tracked file' "$tmp/out" && fail "git rm recovery for an untracked seal: $(cat "$tmp/out")"
/bin/rm -f "$srepo/.corvint/changes/$ghost.cem.json"

"$pre" --bogus > /dev/null 2>&1
[[ $? == 2 ]] || fail "bad argument did not exit 2"
"$regen" --bogus > /dev/null 2>&1
[[ $? == 2 ]] || fail "regen bad argument did not exit 2"

# 8b. V1-3232: a Makefile whose doc-gates recipe has its two known lines expands into one step per
# target plus the go run; any other recipe keeps `make doc-gates` as one step. An `@own` drift entry
# is its own go test step beside the union of the others.
erepo=$tmp/erepo
mkdir -p "$erepo"
cp -R "$repo/.github" "$erepo/.github" && cp -R "$repo/pkg" "$erepo/pkg" || fail "expansion fixture"
printf 'jobs:\n  doc-gates:\n    steps:\n      - run: make doc-gates\n' > "$erepo/.github/workflows/ci.yml"
printf 'DOC_GATE_TARGETS := alpha-check beta-test\ndoc-gates:\n\t@$(MAKE) --no-print-directory --keep-going $(DOC_GATE_TARGETS)\n\t@GOTOOLCHAIN=local go run ./conformance/use-cases-v0\n' > "$erepo/Makefile"
git -C "$erepo" init -q && git -C "$erepo" add -A && git -C "$erepo" commit -qm e || fail "expansion fixture commit"
elist() { BATCH_PRECHECK_ROOT=$erepo BATCH_PRECHECK_REGEN= BATCH_PRECHECK_DRIFT="./pkg TestDrift
./pkg TestOwn @own" "$pre" --list; }
elist > "$tmp/out" 2>&1 || fail "expansion --list: $(cat "$tmp/out")"
grep -qx 'make alpha-check' "$tmp/out" && grep -qx 'make beta-test' "$tmp/out" && grep -qx 'go run ./conformance/use-cases-v0' "$tmp/out" ||
  fail "doc-gates not expanded: $(cat "$tmp/out")"
grep -qx 'make doc-gates' "$tmp/out" && fail "an expanded doc-gates also ran whole: $(cat "$tmp/out")"
grep -qx 'batch-drift (1 packages)' "$tmp/out" && grep -qx 'batch-drift TestOwn' "$tmp/out" || fail "@own not split: $(cat "$tmp/out")"
# ASD-V0-930: the recipe admitted through the host queue ($(HEAVY_ADMIT) prefixes) still expands.
printf 'DOC_GATE_TARGETS := alpha-check beta-test\ndoc-gates:\n\t@$(HEAVY_ADMIT) $(MAKE) --no-print-directory --keep-going $(DOC_GATE_TARGETS)\n\t@$(HEAVY_ADMIT) env GOTOOLCHAIN=local go run ./conformance/use-cases-v0\n' > "$erepo/Makefile"
elist > "$tmp/out" 2>&1 || fail "admitted expansion --list: $(cat "$tmp/out")"
grep -q 'no longer matches doc_gate_expansion' "$tmp/out" && fail "a matching doc-gates recipe was reported as drifted: $(cat "$tmp/out")"
grep -qx 'make alpha-check' "$tmp/out" && grep -qx 'go run ./conformance/use-cases-v0' "$tmp/out" && ! grep -qx 'make doc-gates' "$tmp/out" ||
  fail "the admitted doc-gates recipe was not expanded: $(cat "$tmp/out")"
# ASD-V0-782: the first line run through script/doc-gates-summary.sh still expands.
printf 'DOC_GATE_TARGETS := alpha-check beta-test\ndoc-gates:\n\t@script/doc-gates-summary.sh $(HEAVY_ADMIT) $(MAKE) --no-print-directory --keep-going $(DOC_GATE_TARGETS)\n\t@$(HEAVY_ADMIT) env GOTOOLCHAIN=local go run ./conformance/use-cases-v0\n' > "$erepo/Makefile"
elist > "$tmp/out" 2>&1 || fail "summarized expansion --list: $(cat "$tmp/out")"
grep -qx 'make beta-test' "$tmp/out" && ! grep -qx 'make doc-gates' "$tmp/out" || fail "the summarized doc-gates recipe was not expanded: $(cat "$tmp/out")"
# ASD-V0-782 (V1-3884): doc-gates-summary.sh passes output through and, on failure, ends naming each
# failed doc gate in either make's format (GNU 4 `[Makefile:N: T]`, 3.81 `[T]`) and a rerun command.
dgs=$tmp/dgs
mkdir -p "$dgs"
printf '#!/bin/sh\necho gate-out\necho "make[1]: *** [Makefile:9: beta-test] Error 1" >&2\necho "make: *** [alpha.check] Error 2" >&2\necho "make[2]: *** [Makefile:3: inner] Error 1" >&2\nexit 2\n' > "$dgs/make" && chmod +x "$dgs/make" || fail "summary stub"
"$script/doc-gates-summary.sh" "$dgs/make" --keep-going alpha.check beta-test gamma-check > "$dgs/out" 2> "$dgs/err"
[[ $? == 2 ]] || fail "doc-gates-summary did not keep the exit status: $(cat "$dgs/err")"
[[ $(cat "$dgs/out") == gate-out ]] || fail "doc-gates-summary changed stdout: $(cat "$dgs/out")"
[[ $(tail -n 2 "$dgs/err") == 'doc-gates-summary: FAIL 2 doc gate(s): alpha.check beta-test
doc-gates-summary: rerun one alone with: make -s alpha.check' ]] || fail "doc-gates-summary did not end naming the failed gates: $(cat "$dgs/err")"
grep -q 'Makefile:9: beta-test' "$dgs/err" || fail "doc-gates-summary dropped make's stderr: $(cat "$dgs/err")"
printf '#!/bin/sh\necho boom >&2\nexit 3\n' > "$dgs/make"
"$script/doc-gates-summary.sh" "$dgs/make" alpha.check 2> "$dgs/err"
[[ $? == 3 && $(tail -n 1 "$dgs/err") == 'doc-gates-summary: FAIL exit 3 naming no doc gate; read the output above' ]] || fail "unnamed failure: $(cat "$dgs/err")"
"$script/doc-gates-summary.sh" true alpha.check > "$dgs/out" 2>&1 && [[ ! -s $dgs/out ]] || fail "a passing run printed a summary: $(cat "$dgs/out")"
# V1-4137, V1-3734: a `## help` comment on the doc-gates target line still expands.
printf 'DOC_GATE_TARGETS := alpha-check beta-test\ndoc-gates: ## Run the doc gates\n\t@$(MAKE) --no-print-directory --keep-going $(DOC_GATE_TARGETS)\n\t@GOTOOLCHAIN=local go run ./conformance/use-cases-v0\n' > "$erepo/Makefile"
elist > "$tmp/out" 2>&1 || fail "help-comment expansion --list: $(cat "$tmp/out")"
grep -qx 'make alpha-check' "$tmp/out" && grep -qx 'make beta-test' "$tmp/out" && ! grep -qx 'make doc-gates' "$tmp/out" ||
  fail "the doc-gates target with a ## help comment was not expanded: $(cat "$tmp/out")"
printf 'DOC_GATE_TARGETS := alpha-check\ndoc-gates:\n\t@$(MAKE) --no-print-directory $(DOC_GATE_TARGETS)\n\t@echo extra\n' > "$erepo/Makefile"
elist > "$tmp/out" 2>&1 || fail "unexpanded --list: $(cat "$tmp/out")"
grep -qx 'make doc-gates' "$tmp/out" && ! grep -qx 'make alpha-check' "$tmp/out" || fail "an unknown doc-gates recipe was expanded: $(cat "$tmp/out")"
# ASD-V0-911 (V1-3789): that drifted recipe names doc_gate_expansion; the known recipe above did not.
grep -q 'doc-gates recipe no longer matches doc_gate_expansion in script/batch-precheck.sh' "$tmp/out" ||
  fail "a drifted doc-gates recipe gave no pointer to doc_gate_expansion: $(cat "$tmp/out")"

# 9. ASD-V0-070 (V1-3233): under an empty environment (env -i) a run and --list refuse with exit 3
# naming the unset HOME, a bad argument still exits 2, and --sync-check needs no cache.
(cd "$script/.." && env -i "$pre") > "$tmp/out" 2>&1
[[ $? == 3 ]] || fail "env -i run did not exit 3: $(cat "$tmp/out")"
grep -q '^batch-precheck: REFUSE home-unset: HOME, XDG_CACHE_HOME and CORVINT_GOCACHE are all unset' "$tmp/out" || fail "env -i refusal: $(cat "$tmp/out")"
grep -q 'unbound variable' "$tmp/out" && fail "env -i aborted on an unbound variable: $(cat "$tmp/out")"
(cd "$script/.." && env -i "$pre" --list) > "$tmp/out" 2>&1
[[ $? == 3 ]] || fail "env -i --list did not exit 3: $(cat "$tmp/out")"
(cd "$script/.." && env -i "$pre" --bogus) > /dev/null 2>&1
[[ $? == 2 ]] || fail "env -i bad argument did not exit 2"
# ASD-V0-913: the cases that get past the refusal keep their scratch under this test's TMPDIR.
(cd "$script/.." && env -i TMPDIR="$tmp" "$pre" --sync-check) > "$tmp/out" 2>&1 || fail "env -i --sync-check: $(cat "$tmp/out")"
(cd "$script/.." && env -i TMPDIR="$tmp" CORVINT_GOCACHE=$tmp/gc "$pre" --list) > /dev/null 2>&1 || fail "env -i with CORVINT_GOCACHE --list failed"

# 10. ASD-V0-831 (V1-3337): ai-coauthor-trailers is the one AI-trailer step (ASD-V0-071's
# ai-trailers step is retired; make ai-coauthor-trailers-test covers the check itself). A --base run
# with an AI trailer since the base fails it as introduced, never base-present.
trepo=$tmp/trepo
git init -q "$trepo" && git -C "$trepo" commit -q --allow-empty -m base || fail "trailer fixture"
tbase=$(git -C "$trepo" rev-parse HEAD)
# V1-3492: the listing derives its doc-gates rows from the fixture's own workflow, so the fixture
# carries one (without it awk printed `can't open file .github/workflows/ci.yml` on a pass).
mkdir -p "$trepo/.github/workflows" && cp "$repo/.github/workflows/ci.yml" "$trepo/.github/workflows/ci.yml" || fail "trailer fixture workflow"
(cd "$trepo" && BATCH_PRECHECK_ROOT=$trepo BATCH_PRECHECK_BASE=$tbase "$pre" --list) > "$tmp/out" 2> "$tmp/list.err" || fail "trailer --list: $(cat "$tmp/out")"
[[ ! -s $tmp/list.err ]] || fail "the trailer fixture listing wrote to stderr: $(cat "$tmp/list.err")"
[[ $(grep -c '^ai-coauthor-trailers (' "$tmp/out") == 1 ]] || fail "not one ai-coauthor-trailers step: $(cat "$tmp/out")"
grep -qx 'ai-trailers' "$tmp/out" && fail "the retired ai-trailers step is still listed"
ai_line=$(printf 'Co-Authored-By: Claude Opus 5.5 <noreply@%s>' anthropic.com)
gclone=$tmp/gclone
git clone -q "$frepo" "$gclone" 2> /dev/null && rm -f "$gclone/flaky-test-read-scopes-check" &&
  git -C "$gclone" commit -qam 'no flaky' && gbase=$(git -C "$gclone" rev-parse HEAD) &&
  git -C "$gclone" commit -q --allow-empty -m 'feat: g' -m "$ai_line" || fail "trailer precheck fixture"
mkdir -p "$tmp/gmark"
(cd "$gclone" && PATH=$fbin:$PATH FLAKY_MARK=$tmp/gmark BATCH_PRECHECK_ROOT=$gclone BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= \
  BATCH_PRECHECK_PLANNER= BATCH_PRECHECK_DRIFT="./pkg TestDrift" "$pre" --base "$gbase") > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "a --base run with an AI trailer did not fail: $(cat "$tmp/out")"
grep -Eq '^batch-precheck: \| FAIL \| +[0-9]+s \| ai-coauthor-trailers \([0-9a-f]+\)$' "$tmp/out" ||
  fail "ai-coauthor-trailers not an introduced FAIL: $(cat "$tmp/out")"
grep -q 'WARN ai-coauthor-trailers' "$tmp/out" && fail "ai-coauthor-trailers was downgraded to base-present: $(cat "$tmp/out")"

# 10c. ASD-V0-784 (V1-3504): the merge-conflict-comments step refuses a commit since the batch base
# whose message keeps git's "# Conflicts:" block, naming the commit and the strip recovery; a clean
# range passes, a base that does not resolve exits 2, and no base without origin/main is a skip.
[[ $(grep -c '^merge-conflict-comments (' <(cd "$trepo" && BATCH_PRECHECK_ROOT=$trepo BATCH_PRECHECK_BASE=$tbase "$pre" --list 2> /dev/null)) == 1 ]] ||
  fail "not one merge-conflict-comments step in the listing"
crepo=$tmp/crepo
git init -q "$crepo" && git -C "$crepo" commit -q --allow-empty -m base && cbase=$(git -C "$crepo" rev-parse HEAD) &&
  git -C "$crepo" commit -q --allow-empty -m 'feat: clean' || fail "conflict-comments fixture"
(cd "$crepo" && "$pre" --conflict-comments "$cbase") > "$tmp/out" 2>&1 || fail "a clean range failed conflict-comments: $(cat "$tmp/out")"
(cd "$crepo" && "$pre" --conflict-comments) > "$tmp/out" 2>&1 || fail "no base without origin/main did not skip: $(cat "$tmp/out")"
grep -q '^merge-conflict-comments: skipped: no batch base' "$tmp/out" || fail "no skip line: $(cat "$tmp/out")"
(cd "$crepo" && "$pre" --conflict-comments no-such-base) > /dev/null 2>&1
[[ $? == 2 ]] || fail "an unresolved base did not exit 2"
git -C "$crepo" commit -q --allow-empty --cleanup=verbatim -m "Merge branch 'lane' into batch" -m "# Conflicts:
#	script/x.sh" && csha=$(git -C "$crepo" rev-parse HEAD) || fail "conflict-comments dirty commit"
(cd "$crepo" && "$pre" --conflict-comments "$cbase") > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "a kept # Conflicts: block did not fail: $(cat "$tmp/out")"
grep -q "^merge-conflict-comments: $csha keeps a \"# Conflicts:\" block" "$tmp/out" || fail "the dirty commit is not named: $(cat "$tmp/out")"
grep -q 'cleanup=strip' "$tmp/out" || fail "no strip recovery named: $(cat "$tmp/out")"

# 11. ASD-V0-073 (V1-3238): the legal-file-pins step refuses a legal file whose manifest sha256 is
# stale, naming the file and the manifest, and a missing one; matching pins pass.
lrepo=$tmp/lrepo
mkdir -p "$lrepo/conformance/release-artifact-v0"
printf 'Apache\n' > "$lrepo/LICENSE"
printf 'terms\n' > "$lrepo/LICENSING.md"
pin() { perl -MDigest::SHA=sha256_hex -e 'local $/; open my $f, "<", $ARGV[0] or die; print sha256_hex(<$f>)' "$lrepo/$1"; }
printf '{"legalFiles": [{"path": "LICENSE", "sha256": "%s"}, {"path": "LICENSING.md", "sha256": "%s"}]}\n' "$(pin LICENSE)" "$(pin LICENSING.md)" \
  > "$lrepo/conformance/release-artifact-v0/manifest.json"
(cd "$lrepo" && "$script/check-legal-file-pins.sh") > "$tmp/out" 2>&1 || fail "matching legal pins failed: $(cat "$tmp/out")"
printf 'edited terms\n' > "$lrepo/LICENSING.md"
(cd "$lrepo" && "$script/check-legal-file-pins.sh") > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "a stale legal pin passed: $(cat "$tmp/out")"
grep -q '^check-legal-file-pins: FAIL legal-file-altered LICENSING.md sha256 [0-9a-f]\{64\}, conformance/release-artifact-v0/manifest.json pins ' "$tmp/out" ||
  fail "stale pin not named: $(cat "$tmp/out")"
rm -f "$lrepo/LICENSE"
(cd "$lrepo" && "$script/check-legal-file-pins.sh") > "$tmp/out" 2>&1
grep -q '^check-legal-file-pins: FAIL legal-file-missing LICENSE' "$tmp/out" || fail "missing legal file not named: $(cat "$tmp/out")"
(cd "$tmp" && "$script/check-legal-file-pins.sh") > "$tmp/out" 2>&1 || fail "a checkout without the manifest failed: $(cat "$tmp/out")"
(cd "$script/.." && "$script/check-legal-file-pins.sh") > "$tmp/out" 2>&1 || fail "this checkout's legal pins are stale: $(cat "$tmp/out")"
# ASD-V0-584 (V1-3374): the same refusal runs in make doc-gates, before bind and seal: the
# legal-file-pins-check target is a doc gate, and on the fixture it refuses an edited LICENSING.md.
grep -qE '^DOC_GATE_TARGETS := (.* )?legal-file-pins-check( |$)' "$script/../Makefile" || fail "legal-file-pins-check is not a doc gate"
printf 'Apache\n' > "$lrepo/LICENSE"
mkdir -p "$lrepo/script" && /bin/cp -f "$script/check-legal-file-pins.sh" "$lrepo/script/"
(cd "$lrepo" && make -s -f "$script/../Makefile" legal-file-pins-check) > "$tmp/out" 2>&1 &&
  fail "make legal-file-pins-check passed an edited LICENSING.md: $(cat "$tmp/out")"
grep -q '^check-legal-file-pins: FAIL legal-file-altered LICENSING.md ' "$tmp/out" ||
  fail "make legal-file-pins-check did not name the edited LICENSING.md: $(cat "$tmp/out")"

# 12. ASD-V0-072 (V1-3230): a --base run in a shallow clone refuses with the unshallow recovery
# before any step runs.
srepo2=$tmp/shallow
git clone -q --depth 1 "file://$trepo" "$srepo2" 2> /dev/null || fail "shallow clone"
(cd "$srepo2" && "$pre" --base HEAD) > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail "a shallow --base run did not exit 2: $(cat "$tmp/out")"
grep -q '^batch-precheck: REFUSE shallow-repository: .* fetch --unshallow origin' "$tmp/out" || fail "shallow refusal: $(cat "$tmp/out")"
"$script/require-full-history.sh" probe "$trepo" || fail "a full clone was refused"
# The dogfood range wrappers refuse the same clone before resolving their base.
mkdir -p "$srepo2/script" && cp "$script/dogfood-change.sh" "$script/dogfood-check.sh" "$script/dogfood-bind-range.sh" "$script/dogfood-build-corvint.sh" "$srepo2/script/" ||
  fail "copy dogfood wrappers"
for w in dogfood-change dogfood-check dogfood-bind-range; do
  "$srepo2/script/$w.sh" HEAD HEAD > "$tmp/out" 2>&1
  [[ $? == 2 ]] && grep -q "^$w: REFUSE shallow-repository: .* fetch --unshallow origin" "$tmp/out" || fail "$w in a shallow clone: $(cat "$tmp/out")"
done

# 13. ASD-V0-582 (V1-3364): the binary-blobs-check doc gate (a batch-precheck step by group 1)
# refuses a committed binary blob outside a declared fixture path and a tools/ main whose root
# build output is not ignored; a testdata/ fixture and an ignored tool output pass.
brepo=$tmp/brepo
mkdir -p "$brepo/tools/kept" "$brepo/pkg/testdata"
git -C "$brepo" init -q -b main
printf 'package main\n\nfunc main() {}\n' > "$brepo/tools/kept/main.go"
printf '/kept\n' > "$brepo/.gitignore"
printf 'fixture\0bytes\n' > "$brepo/pkg/testdata/blob.bin"
git -C "$brepo" add -A && git -C "$brepo" commit -qm base
(cd "$brepo" && "$script/check-binary-blobs.sh") > "$tmp/out" 2>&1 || fail "a fixture blob or ignored tool failed: $(cat "$tmp/out")"
printf '\177ELF\2\1\1\0\0\0' > "$brepo/kept-build"
chmod +x "$brepo/kept-build"
git -C "$brepo" add kept-build
(cd "$brepo" && "$script/check-binary-blobs.sh") > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q '^check-binary-blobs: FAIL binary-blob kept-build mode=100755: ' "$tmp/out" ||
  fail "a committed executable blob was not refused: $(cat "$tmp/out")"
git -C "$brepo" rm -q --cached kept-build
mkdir -p "$brepo/tools/loose"
printf 'package main\n\nfunc main() {}\n' > "$brepo/tools/loose/main.go"
git -C "$brepo" add tools/loose
(cd "$brepo" && "$script/check-binary-blobs.sh") > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -qx 'check-binary-blobs: FAIL tool-output-not-ignored loose: .*add /loose to the root .gitignore' "$tmp/out" ||
  fail "an unignored tool output was not refused: $(cat "$tmp/out")"
grep -q kept "$tmp/out" && fail "an ignored tool output was refused: $(cat "$tmp/out")"
# ASD-V0-845 (V1-3500): under the repository's own root .gitignore a throwaway tools/ main and its
# built root binary pass with no line of their own, LICENSE, Makefile and VERSION stay addable, and a
# tracked extension-less root file the generic rule ignores is refused with its re-include line.
git -C "$brepo" rm -q -r --cached tools/loose
cp "$script/../.gitignore" "$brepo/.gitignore"
mkdir -p "$brepo/tools/throwaway"
printf 'package main\n\nfunc main() {}\n' > "$brepo/tools/throwaway/main.go"
printf '\177ELF\2\1\1\0\0\0' > "$brepo/throwaway"
printf 'x\n' > "$brepo/LICENSE"; printf 'x\n' > "$brepo/Makefile"; printf 'x\n' > "$brepo/VERSION"
git -C "$brepo" add -A
git -C "$brepo" ls-files --error-unmatch throwaway > /dev/null 2>&1 && fail "git add -A added the root tool binary"
for f in LICENSE Makefile VERSION tools/throwaway/main.go .gitignore; do
  git -C "$brepo" ls-files --error-unmatch "$f" > /dev/null 2>&1 || fail "git add -A skipped $f"
done
(cd "$brepo" && "$script/check-binary-blobs.sh") > "$tmp/out" 2>&1 || fail "a new tool under the generic ignore failed: $(cat "$tmp/out")"
printf 'x\n' > "$brepo/NOTICE"
git -C "$brepo" add -f NOTICE
(cd "$brepo" && "$script/check-binary-blobs.sh") > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q '^check-binary-blobs: FAIL tracked-path-ignored NOTICE: .*add !/NOTICE' "$tmp/out" ||
  fail "a tracked ignored root file was not refused: $(cat "$tmp/out")"

# ASD-V0-846 (V1-2432): the capability catalog drift check is a doc gate, so the local doc gates catch
# a new command, binary or MCP tool that has no docs/CAPABILITIES.md row.
grep -qE '^DOC_GATE_TARGETS := (.* )?capabilities-check( |$)' "$script/../Makefile" || fail "capabilities-check is not a doc gate"

# ASD-V0-720 (V1-3847): fixed-tmp-paths-check is a doc gate; it refuses a test that writes under
# the fixed /tmp or exports TMPDIR=/tmp, and passes t.TempDir() and a `// fixed-tmp:` line or the
# line after one, for a quoted or raw literal and for a call whose argument is on the next line.
grep -qE '^DOC_GATE_TARGETS := (.* )?fixed-tmp-paths-check( |$)' "$script/../Makefile" || fail "fixed-tmp-paths-check is not a doc gate"
trepo=$tmp/tmprepo
mkdir -p "$trepo/pkg"
git -C "$trepo" init -q -b main
printf '%s\n' 'package pkg' '' 'func a(t *testing.T) {' '	_, _ = os.MkdirTemp(t.TempDir(), "x")' \
  '	_, _ = os.MkdirTemp("/tmp", "sock-") // fixed-tmp: sun_path' '	_ = "TMPDIR=/tmp"' \
  '	// fixed-tmp: sun_path' '	_, _ = os.MkdirTemp(' '		`/tmp`, "sock-")' '}' > "$trepo/pkg/ok_test.go"
git -C "$trepo" add -A
(cd "$trepo" && "$script/check-fixed-tmp-paths.sh") > "$tmp/out" 2>&1 || fail "allowed /tmp lines failed: $(cat "$tmp/out")"
printf '%s\n' 'package pkg' '' 'func b(t *testing.T) {' '	_, _ = os.MkdirTemp("/tmp", "x-")' \
  '	command.Env = []string{"HOME=" + home, "TMPDIR=/tmp"}' '	_ = os.WriteFile(`/tmp/x`, nil, 0o600)' \
  '	_, _ = os.MkdirTemp(' '		"/tmp", "y-")' '}' > "$trepo/pkg/bad_test.go"
git -C "$trepo" add -A
(cd "$trepo" && "$script/check-fixed-tmp-paths.sh") > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "a fixed /tmp write passed: $(cat "$tmp/out")"
grep -q '^check-fixed-tmp-paths: FAIL pkg/bad_test.go:4: ' "$tmp/out" &&
  grep -q '^check-fixed-tmp-paths: FAIL pkg/bad_test.go:5: ' "$tmp/out" &&
  grep -q '^check-fixed-tmp-paths: FAIL pkg/bad_test.go:6: ' "$tmp/out" &&
  grep -q '^check-fixed-tmp-paths: FAIL pkg/bad_test.go:7: ' "$tmp/out" &&
  [[ $(grep -c FAIL "$tmp/out") == 4 ]] || fail "the fixed /tmp lines were not named exactly: $(cat "$tmp/out")"

# 14. ASD-V0-583 (V1-3373): license-headers-check and mkfifo-build-tags-check are doc gates, and the
# Mkfifo check refuses a file calling syscall.Mkfifo under //go:build unix (which includes solaris)
# or with no constraint at all, while an explicit GOOS list, unix && !solaris and a GOOS file-name
# suffix pass.
for t in license-headers-check mkfifo-build-tags-check; do
  grep -qE "^DOC_GATE_TARGETS := (.* )?$t( |\$)" "$script/../Makefile" || fail "$t is not a doc gate"
done
frepo=$tmp/frepo
mkdir -p "$frepo/pkg"
git -C "$frepo" init -q -b main
fifo_file() { printf '%spackage pkg\n\nimport "syscall"\n\nvar _ = syscall.Mkfifo\n' "$2" > "$frepo/pkg/$1"; }
fifo_file listed_test.go $'//go:build darwin || linux\n\n'
fifo_file excluded_test.go $'//go:build unix && !solaris\n\n'
fifo_file suffix_linux_test.go ''
git -C "$frepo" add -A
(cd "$frepo" && "$script/check-mkfifo-build-tags.sh") > "$tmp/out" 2>&1 || fail "allowed Mkfifo constraints failed: $(cat "$tmp/out")"
fifo_file unix_test.go $'//go:build unix\n\n'
fifo_file untagged_test.go ''
git -C "$frepo" add -A
(cd "$frepo" && "$script/check-mkfifo-build-tags.sh") > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "a solaris-reachable Mkfifo file passed: $(cat "$tmp/out")"
grep -q '^check-mkfifo-build-tags: FAIL pkg/unix_test.go: calls syscall.Mkfifo under //go:build unix, ' "$tmp/out" &&
  grep -q '^check-mkfifo-build-tags: FAIL pkg/untagged_test.go: calls syscall.Mkfifo with no //go:build constraint' "$tmp/out" ||
  fail "the unix-tagged and untagged Mkfifo files were not both named: $(cat "$tmp/out")"
[[ $(grep -c FAIL "$tmp/out") == 2 ]] || fail "an allowed Mkfifo file was refused: $(cat "$tmp/out")"

# 15. ASD-V0-943, ASD-V0-944, DCW-V0-944 (V1-3550): the closeout's sealed-head run selects since the
# first run's receipt, and selection-baseline-check is skipped when none of its inputs changed, with
# or without a usable plan.
grep -A2 '^dogfood-closeout-precheck:$' "$script/../Makefile" | grep -qxF $'\t@BATCH_PRECHECK_SINCE_RECEIPT=1 $(MAKE) --no-print-directory batch-precheck' ||
  fail "dogfood-closeout-precheck does not select since the receipt (DCW-V0-923)"
rrepo=$tmp/rrepo
git clone -q "$srepo" "$rrepo" || fail "since-receipt fixture clone"
git -C "$rrepo" checkout -q --detach "$sbase" && mkdir -p "$rrepo/interop/cem01-go"
printf 'capability-catalog-check:\n\t$(GO_TEST_COMMAND) -run x ./golden\nselection-baseline-check:\n\t@d=x && \\\n\tgo build ./other ./golden\n' > "$rrepo/Makefile"
sed -i.bak 's/make -s spec-requirements-check/make -s spec-requirements-check selection-baseline-check/' "$rrepo/.github/workflows/ci.yml" && /bin/rm -f "$rrepo/.github/workflows/ci.yml.bak"
git -C "$rrepo" commit -qam 'selection-baseline recipe' || fail "since-receipt fixture commit"
rbase=$(git -C "$rrepo" rev-parse HEAD)
rrun() { (unset GOFLAGS; cd "$rrepo" && PATH=$sbin:$PATH BATCH_PRECHECK_ROOT=$rrepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_PLANNER=$tmp/planner BATCH_PRECHECK_DRIFT=$sdrift "$pre" "$@") > "$tmp/out" 2>&1; }
# A doc-only change skips selection-baseline-check; a Go change in a recipe package runs it.
mkdir -p "$rrepo/notes" && printf 'n\n' > "$rrepo/notes/a.txt"
git -C "$rrepo" add -A && git -C "$rrepo" commit -qm notes
work=$(git -C "$rrepo" rev-parse HEAD)
pbase=$rbase plan "$frontier" golden
: > "$tmp/scalls"
rrun --base "$rbase" || fail "doc-only run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: selection-baseline-check skipped: no selection-baseline input changed since $rbase" "$tmp/out" || fail "selection-baseline not skipped: $(cat "$tmp/out")"
grep -q 'selection-baseline-check' "$tmp/scalls" && fail "a skipped selection-baseline-check ran: $(cat "$tmp/scalls")"
grep -q "^batch-precheck: receipt for $work written" "$tmp/out" || fail "doc-only run wrote no receipt: $(cat "$tmp/out")"
printf 'package golden\n' > "$rrepo/golden/extra.go"
: > "$tmp/scalls"
rrun --base "$rbase" || fail "Go change run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: selection-baseline-check runs (a selection-baseline input changed' "$tmp/out" || fail "selection-baseline skipped on a Go change: $(cat "$tmp/out")"
grep -q '^make -s selection-baseline-check$' "$tmp/scalls" || fail "selection-baseline-check did not run: $(cat "$tmp/scalls")"
# A planner that cannot narrow still lets the diff decide: a Go change runs it, a doc-only one skips it.
: > "$tmp/scalls"
STUB_PLAN_RC=3 rrun --base "$rbase" || fail "unusable-plan Go change run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: selection-baseline-check runs (a Go input changed and the plan is not used)' "$tmp/out" || fail "unusable plan skipped a Go change: $(cat "$tmp/out")"
grep -q '^make -s selection-baseline-check$' "$tmp/scalls" || fail "selection-baseline-check did not run without a plan: $(cat "$tmp/scalls")"
/bin/rm -f "$rrepo/golden/extra.go"
: > "$tmp/scalls"
STUB_PLAN_RC=3 rrun --base "$rbase" || fail "unusable-plan doc-only run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: selection-baseline-check skipped: no Go or selection-baseline input changed since $rbase (the plan is not used: " "$tmp/out" || fail "unusable plan ran selection-baseline on a doc-only diff: $(cat "$tmp/out")"
grep -q 'selection-baseline-check' "$tmp/scalls" && fail "a skipped selection-baseline-check ran without a plan: $(cat "$tmp/scalls")"
printf 'x\n' > "$rrepo/top.txt"
STUB_PLAN_RC=3 rrun --base "$rbase" || fail "unusable-plan top-level run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: selection-baseline-check runs (a top-level file changed and the plan is not used)' "$tmp/out" || fail "a top-level change skipped selection-baseline: $(cat "$tmp/out")"
/bin/rm -f "$rrepo/top.txt"
# Restore the receipt for the work head, then commit only change evidence on top, as bind and seal do.
rrun --base "$rbase" || fail "work head rerun failed: $(cat "$tmp/out")"
mkdir -p "$rrepo/.corvint/changes" && printf '{}\n' > "$rrepo/.corvint/changes/$work.cem.json"
git -C "$rrepo" add -A && git -C "$rrepo" commit -qm 'chore: seal change evidence'
sealed=$(git -C "$rrepo" rev-parse HEAD)
pbase=$work plan "$frontier"
: > "$tmp/scalls"
BATCH_PRECHECK_SINCE_RECEIPT=1 rrun --base "$rbase" || fail "since-receipt run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: since-receipt base $work: " "$tmp/out" || fail "since-receipt base not used: $(cat "$tmp/out")"
grep -qx "planner affected --base $work" "$tmp/scalls" || fail "planner not asked since the receipt: $(cat "$tmp/scalls")"
row_is PASS 'batch-drift (0 of 3 tests)'
grep -q '^batch-precheck: capability-catalog-check skipped' "$tmp/out" && grep -q '^batch-precheck: selection-baseline-check skipped' "$tmp/out" ||
  fail "catalog or selection-baseline reran after a seal: $(cat "$tmp/out")"
grep -qE '^make -s (capability-catalog|selection-baseline)-check$' "$tmp/scalls" && fail "a skipped step ran: $(cat "$tmp/scalls")"
grep -q "^batch-precheck: receipt for $sealed written" "$tmp/out" || fail "sealed head got no receipt: $(cat "$tmp/out")"
# A receipt for the clean HEAD runs nothing; a dirty checkout, or no receipt, runs with the given base.
: > "$tmp/scalls"
BATCH_PRECHECK_SINCE_RECEIPT=1 rrun --base "$rbase" || fail "receipt-at-HEAD run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: SKIP receipt already names HEAD $sealed and the checkout is clean" "$tmp/out" || fail "receipt at HEAD reran: $(cat "$tmp/out")"
[[ ! -s $tmp/scalls ]] || fail "receipt at HEAD ran steps: $(cat "$tmp/scalls")"
[[ $(awk '$1 == "head" { print $2 }' "$(git -C "$rrepo" rev-parse --path-format=absolute --git-path batch-precheck-receipt)") == "$sealed" ]] || fail "the receipt for HEAD was not kept"
printf 'dirty\n' >> "$rrepo/notes/a.txt"
pbase=$rbase plan "$frontier" golden
BATCH_PRECHECK_SINCE_RECEIPT=1 rrun --base "$rbase" || fail "dirty since-receipt run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: since-receipt not used: the receipt names HEAD but the checkout is not clean' "$tmp/out" || fail "dirty checkout used the receipt: $(cat "$tmp/out")"
git -C "$rrepo" checkout -q -- notes/a.txt
BATCH_PRECHECK_SINCE_RECEIPT=1 rrun --base "$rbase" || fail "no-receipt run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: since-receipt not used: no well-formed passing receipt' "$tmp/out" || fail "a missing receipt was used: $(cat "$tmp/out")"
grep -q "^batch-precheck: batch-drift runs .* for the diff since $rbase" "$tmp/out" || fail "no-receipt run did not keep its base: $(cat "$tmp/out")"

# 16. ICL-V0-020, ICL-V0-021 (V1-1629): the pre-bind run (BATCH_PRECHECK_SKIP_DRIFT=1) runs the
# @prebind drift entries, which no bind or seal can reach, so a failure stops the closeout before
# the bind; it records their passes for its head. The run on the sealed head carries each recorded
# pass only while it still binds: change evidence alone changed since, the plan of those paths
# selects no carried package and has no unknown but the frontier, and the salt is unchanged; any
# other case prints why and runs the entry again.
prepo=$tmp/prepo
git clone -q "$srepo" "$prepo" && git -C "$prepo" reset -q --hard "$sbase" && mkdir -p "$prepo/interop/cem01-go" || fail "pre-bind fixture clone"
pdrift='./golden TestGoldenDrift @go
./other TestOtherDrift @prebind'
prun() { (unset GOFLAGS; PATH=$sbin:$PATH BATCH_PRECHECK_ROOT=$prepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_PLANNER=$tmp/planner BATCH_PRECHECK_DRIFT=$pdrift "$pre" "$@") > "$tmp/out" 2>&1; }
prec=$(git -C "$prepo" rev-parse --path-format=absolute --git-path batch-precheck-prebind)
preceipt=$(git -C "$prepo" rev-parse --path-format=absolute --git-path batch-precheck-receipt)
phead=$(git -C "$prepo" rev-parse HEAD)
BATCH_PRECHECK_SKIP_DRIFT=1 prun || fail "pre-bind run failed: $(cat "$tmp/out")"
row_is PASS 'batch-drift (pre-bind, 1 packages, 1 of 2 tests)'
row_is PASS 'batch-drift (skipped before the bind)'
[[ -f $preceipt ]] && fail "a pre-bind run wrote a receipt"
[[ $(sed -n 1p "$prec") == corvint-batch-precheck-prebind/1 && $(sed -n 2p "$prec") == "head $phead" ]] || fail "pre-bind record: $(cat "$prec" 2> /dev/null)"
grep -qxF 'entry ./other TestOtherDrift @prebind' "$prec" && ! grep -q 'TestGoldenDrift' "$prec" || fail "pre-bind record entries: $(cat "$prec")"
# A bind commits change evidence alone; the plan of that path selects ./golden only: ./other's pass carries.
mkdir -p "$prepo/.corvint" && printf '{}\n' > "$prepo/.corvint/change.cem.json"
git -C "$prepo" add -A && git -C "$prepo" commit -qm 'chore: bind change evidence' || fail "bind commit"
plan "$frontier" golden
: > "$tmp/scalls"
prun || fail "sealed-head run failed: $(cat "$tmp/out")"
grep -qx 'planner affected --path .corvint/change.cem.json --format json' "$tmp/scalls" || fail "carry plan not asked for the evidence paths: $(cat "$tmp/scalls")"
grep -q "^batch-precheck: carried 1 pre-bind drift pass(es) from ${phead:0:12}: " "$tmp/out" || fail "carry line: $(cat "$tmp/out")"
grep -qx 'batch-precheck:   carried ./other TestOtherDrift @prebind' "$tmp/out" || fail "carried entry: $(cat "$tmp/out")"
row_is PASS "batch-drift (1 pre-bind passes carried from ${phead:0:12})"
row_is PASS 'batch-drift (1 packages, 1 of 2 tests)'
[[ -f $preceipt ]] || fail "a carried run wrote no receipt: $(cat "$tmp/out")"
# The plan of the evidence selects the entry's package: it runs again, named.
plan "$frontier" golden other
prun || fail "selected-package run failed: $(cat "$tmp/out")"
grep -qx 'batch-precheck:   runs again ./other TestOtherDrift (the plan of the evidence paths selects ./other)' "$tmp/out" || fail "runs-again line: $(cat "$tmp/out")"
row_is PASS 'batch-drift (2 packages)'
# An unknown beyond the frontier, a failed plan and a changed environment carry nothing.
plan '{"reason":"UNRESOLVED_READER","detail":"x"}' golden
prun || fail "unknown-plan run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: no pre-bind carry: the plan of the evidence paths has the unknown UNRESOLVED_READER x; ' "$tmp/out" || fail "unknown carried: $(cat "$tmp/out")"
row_is PASS 'batch-drift (2 packages)'
plan "$frontier,{\"reason\":\"LANGUAGE_FRONTIER\",\"detail\":\"typescript:e2e-runtime-dependency:a.ts\"}" golden
prun && grep -q '^batch-precheck: carried 1 pre-bind' "$tmp/out" || fail "a TypeScript frontier unknown stopped the carry: $(cat "$tmp/out")"
plan "$frontier" golden
STUB_PLAN_RC=4 prun || fail "failed-plan run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: no pre-bind carry: .*planner affected --path exited 4' "$tmp/out" || fail "failed plan carried: $(cat "$tmp/out")"
CORVINT_PREBIND_PROBE=1 prun || fail "changed-env run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: no pre-bind carry: the go command or the Go, cgo or Corvint environment changed; ' "$tmp/out" || fail "changed env carried: $(cat "$tmp/out")"
row_is PASS 'batch-drift (2 packages)'
# A dirty tree and a commit beyond change evidence carry nothing.
printf 'x\n' > "$prepo/other/note.txt"
prun || fail "dirty run failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: no pre-bind carry: the checkout has uncommitted' "$tmp/out" || fail "dirty tree carried: $(cat "$tmp/out")"
git -C "$prepo" add -A && git -C "$prepo" commit -qm fix || fail "fix commit"
prun || fail "fix run failed: $(cat "$tmp/out")"
grep -q "^batch-precheck: no pre-bind carry: ${phead:0:12}..HEAD changes other/note.txt, which is not change evidence; " "$tmp/out" || fail "a source change carried: $(cat "$tmp/out")"
row_is PASS 'batch-drift (2 packages)'
# A failing @prebind entry fails the pre-bind run, before any bind, and removes the record.
printf 'v2\n' > "$prepo/golden/testdata/want.txt"
git -C "$prepo" add -A && git -C "$prepo" commit -qm drift || fail "drift commit"
pdrift='./golden TestGoldenDrift @go @prebind
./other TestOtherDrift @prebind'
BATCH_PRECHECK_SKIP_DRIFT=1 prun && fail "a failing pre-bind entry passed the pre-bind run"
row_is FAIL 'batch-drift (pre-bind, 2 packages)'
grep -q 'batch-drift (skipped before the bind)' "$tmp/out" && fail "a skip row with nothing held: $(cat "$tmp/out")"
[[ -f $prec ]] && fail "a failed pre-bind run left a record: $(cat "$prec")"
# ASD-V0-825 (V1-4069): under LANE=1 an @own entry runs only when a companion input changed; the
# receipt of such a run is marked, and a since-receipt run without LANE does not narrow from it.
odrift='./golden TestGoldenDrift @go
./golden TestFreeze @own @go'
orun() { (unset GOFLAGS; cd "$rrepo" && PATH=$sbin:$PATH BATCH_PRECHECK_ROOT=$rrepo BATCH_PRECHECK_HEAVY= BATCH_PRECHECK_REGEN= BATCH_PRECHECK_PLANNER=$tmp/planner BATCH_PRECHECK_DRIFT=$odrift "$pre" "$@") > "$tmp/out" 2>&1; }
printf 'package golden\n' > "$rrepo/golden/extra.go"
git -C "$rrepo" add -A && git -C "$rrepo" commit -qm 'lane go change'
ohead=$(git -C "$rrepo" rev-parse HEAD)
pbase=$rbase plan "$frontier" golden
LANE=1 orun --base "$rbase" || fail "lane run failed: $(cat "$tmp/out")"
grep -qx "batch-precheck:   skipped ./golden TestFreeze (lane closeout: no companion input changed since $rbase; the batch precheck runs it, ASD-V0-825)" "$tmp/out" ||
  fail "the lane run did not skip the @own entry: $(cat "$tmp/out")"
grep -q '| batch-drift TestFreeze$' "$tmp/out" && fail "the lane run ran the @own entry: $(cat "$tmp/out")"
row_is PASS 'batch-drift (1 packages, 1 of 2 tests)'
olane="$(git -C "$rrepo" rev-parse --path-format=absolute --git-path batch-precheck-receipt).lane"
[[ $(cat "$olane" 2> /dev/null) == "$ohead" ]] || fail "the lane receipt carries no .lane mark for $ohead"
# The batch run (no LANE) runs the @own entry and does not narrow from the lane receipt.
mkdir -p "$rrepo/.corvint/changes" && printf '{}\n' > "$rrepo/.corvint/changes/$ohead.cem.json"
git -C "$rrepo" add -A && git -C "$rrepo" commit -qm 'chore: seal lane change evidence'
BATCH_PRECHECK_SINCE_RECEIPT=1 orun --base "$rbase" || fail "batch run over a lane receipt failed: $(cat "$tmp/out")"
grep -q '^batch-precheck: since-receipt not used: the receipt is from a lane closeout that skipped the @own companion builds (ASD-V0-825)$' "$tmp/out" ||
  fail "a batch run narrowed from a lane receipt: $(cat "$tmp/out")"
row_is PASS 'batch-drift TestFreeze'
[[ ! -e $olane ]] || fail "a batch receipt kept the .lane mark"
# A changed companion input runs the @own entry in a lane closeout too.
mkdir -p "$rrepo/internal/companionrelease" && printf 'x\n' > "$rrepo/internal/companionrelease/note.txt"
LANE=1 orun --base "$rbase" || fail "lane run with a companion input failed: $(cat "$tmp/out")"
row_is PASS 'batch-drift TestFreeze'
/bin/rm -rf "$rrepo/internal"

finish
