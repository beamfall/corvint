#!/bin/sh
set -eu

# GOC-V0-010: `make gate` clears the private full-gate receipt before its first step and records it
# only after every step passed, from a clean worktree whose archive witness is PASS for HEAD. The
# archive status command is stubbed on PATH because the property under test is the receipt's
# binding and refusals, not the archive semantics release-artifact-integrity-v0.md owns.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-gate-receipt-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

repository="$test_root/repo"
mkdir -p "$repository/script" "$test_root/bin"
cp "$source_root/script/gate-receipt" "$repository/script/"
printf 'fixture\n' > "$repository/README"
git -C "$repository" init -q -b main
git -C "$repository" -c user.name=test -c user.email=test@example.invalid add .
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm fixture
witness="$repository/.git/corvint/release-go-archive-report.json"
receipt="$repository/.git/corvint/release-gate-receipt"
revision=$(git -C "$repository" rev-parse 'HEAD^{commit}')
tree=$(git -C "$repository" rev-parse 'HEAD^{tree}')

# Stands in for `go run ./conformance/release-artifact-v0 archive-status`: the fixture witness is
# one "revision tree verdict" line.
cat > "$test_root/bin/go" <<'STUB'
#!/bin/sh
set -eu
[ "$3" = archive-status ] || exit 2
if [ -n "${CORVINT_FIXTURE_STATUS_WAIT:-}" ]; then
    printf '%s\n' "$$" > "$CORVINT_FIXTURE_STATUS_WAIT"
    while [ ! -e "$CORVINT_FIXTURE_STATUS_WAIT.release" ]; do sleep 0.05; done
fi
shift 3
path=; revision=; tree=
while [ $# -gt 0 ]; do
    case $1 in
    --witness) path=$2; shift 2 ;;
    --revision) revision=$2; shift 2 ;;
    --tree) tree=$2; shift 2 ;;
    *) shift ;;
    esac
done
[ -f "$path" ] || { printf 'INVALID\n'; exit 1; }
read -r recorded_revision recorded_tree verdict < "$path"
if [ "$recorded_revision" = "$revision" ] && [ "$recorded_tree" = "$tree" ]; then
    printf '%s\n' "$verdict"
else
    printf 'STALE\n'
fi
STUB
chmod +x "$test_root/bin/go"

run() {
    set +e
    (cd "$repository" && PATH="$test_root/bin:$PATH" script/gate-receipt "$@") > "$test_root/out" 2>&1
    status=$?
    set -e
}
write_witness() {
    mkdir -p "$(dirname "$witness")"
    printf '%s %s %s\n' "$1" "$tree" "$2" > "$witness"
}

# GOC-V0-010 binding: a PASS witness for HEAD from a clean worktree records the canonical receipt.
write_witness "$revision" PASS
run clear
run record
[ "$status" -eq 0 ] || fail "record refused a clean PASS: $(cat "$test_root/out")"
digest=$(shasum -a 256 "$witness" | awk '{print $1}')
[ "$(cat "$receipt")" = "corvint-gate-receipt/0 $revision $tree $digest" ] || fail "receipt is not canonical: $(cat "$receipt")"
[ "$(wc -l < "$receipt" | tr -d ' ')" = 1 ] || fail "receipt is not one LF-terminated line"
mode=$(stat -f '%Lp' "$receipt" 2>/dev/null || stat -c '%a' "$receipt")
[ "$mode" = 600 ] || fail "receipt mode is $mode"

# GOC-V0-010 clear: the gate's first step removes an earlier receipt, so a failing run leaves none.
run clear
[ "$status" -eq 0 ] || fail "clear failed"
[ ! -e "$receipt" ] || fail "clear left the receipt"

# GOC-V0-010 refusals: each leaves no receipt, including one an earlier record wrote.
refuses() {
    expected=$1
    run record
    [ "$status" -ne 0 ] || fail "record accepted: $expected"
    [ ! -e "$receipt" ] || fail "record left a receipt: $expected"
    grep -q "gate-receipt: REFUSE $expected" "$test_root/out" || fail "missing reason $expected: $(cat "$test_root/out")"
}
run record
[ -f "$receipt" ] || fail "re-record failed"
printf 'untracked\n' > "$repository/stray"
refuses worktree-not-clean
rm "$repository/stray"
write_witness 0000000000000000000000000000000000000000 PASS
refuses archive-witness-not-pass
write_witness "$revision" FAIL
refuses archive-witness-not-pass
rm "$witness"
refuses archive-witness-absent

# GOC-V0-010 run binding: the receipt names the HEAD the gate started at from a clean worktree. A
# worktree dirty when the gate started, or a commit made while it ran, refuses even though HEAD's
# witness is PASS and the worktree is clean at the end.
printf 'untracked\n' > "$repository/stray"
run clear
rm "$repository/stray"
write_witness "$revision" PASS
refuses worktree-not-clean-at-start
run clear
printf 'moved\n' > "$repository/README"
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qam moved
revision=$(git -C "$repository" rev-parse 'HEAD^{commit}')
tree=$(git -C "$repository" rev-parse 'HEAD^{tree}')
write_witness "$revision" PASS
refuses head-moved-during-gate
run clear
run record
[ "$status" -eq 0 ] || fail "record refused a gate started at the new HEAD: $(cat "$test_root/out")"
# An unreadable worktree state is not a clean one: a corrupt index makes `git status` fail.
cp "$repository/.git/index" "$test_root/index"
printf 'corrupt' > "$repository/.git/index"
refuses worktree-not-clean
cp "$test_root/index" "$repository/.git/index"

# A change `git status` does not report is still a change: a tracked file edited under
# skip-worktree or assume-unchanged, or an ignored `.go` file the gate would compile, refuses.
printf 'hidden\n' > "$repository/README"
git -C "$repository" update-index --skip-worktree README
refuses worktree-not-clean
git -C "$repository" update-index --no-skip-worktree README
git -C "$repository" update-index --assume-unchanged README
refuses worktree-not-clean
git -C "$repository" update-index --no-assume-unchanged README
git -C "$repository" checkout -q -- README
mkdir -p "$repository/build" "$repository/.cache"
printf 'build/\n.cache/\n' > "$repository/.git/info/exclude"
printf 'package cache\n' > "$repository/.cache/cache.go"
run clear
run record
[ "$status" -eq 0 ] || fail "record refused an ignored .go file under a directory Go skips: $(cat "$test_root/out")"
printf 'package build\n' > "$repository/build/build.go"
refuses worktree-not-clean
rm -r "$repository/build" "$repository/.cache"

# GOC-V0-010 signal: TERM to record alone while its status check still reports PASS exits 143 and
# leaves no receipt, instead of resuming after cleanup to record one.
write_witness "$revision" PASS
run clear
status_wait="$test_root/waiting-status"
(cd "$repository" && PATH="$test_root/bin:$PATH" CORVINT_FIXTURE_STATUS_WAIT="$status_wait" \
    exec script/gate-receipt record) > "$test_root/out" 2>&1 &
record_pid=$!
attempt=0
while [ ! -s "$status_wait" ] && [ "$attempt" -lt 600 ]; do
    sleep 0.05
    attempt=$((attempt + 1))
done
[ -s "$status_wait" ] || fail "signal fixture did not reach the status check"
kill -TERM "$record_pid"
: > "$status_wait.release"
status=0
wait "$record_pid" || status=$?
[ "$status" -eq 143 ] || fail "a record signalled during a passing status check exited $status, not 143"
[ ! -e "$receipt" ] || fail "a signalled record left a receipt"

# GOC-V0-010 wiring: the clear is the gate's first prerequisite and the record is its recipe.
awk '
    /^gate:/ { split($0, parts, " "); first = parts[2]; ingate = 1; next }
    ingate && /^\t/ { recipe = recipe $0 "\n"; next }
    ingate { ingate = 0 }
    END {
        if (first != "gate-receipt-clear") exit 1
        if (recipe != "\t@script/gate-receipt record\n") exit 1
    }
' "$source_root/Makefile" || fail "Makefile gate does not clear first and record last"

# GOC-V0-010 ordering: under `make -j` no gate step starts before the clear finished, and a step run
# on its own does not clear. The real Makefile's graph runs with every recipe replaced by a logger;
# the ledger is off so its wrappers run the logger recipes and record nothing.
order_log="$test_root/order.log"
cat > "$test_root/order.mk" <<'MK'
gate-receipt-clear:
	@sleep 1; echo clear >> "$(ORDER_LOG)"
$(GATE_STEPS):
	@echo step >> "$(ORDER_LOG)"
gate:
	@echo record >> "$(ORDER_LOG)"
MK
CORVINT_GATE_LEDGER=off make -s -C "$source_root" -f "$source_root/Makefile" -f "$test_root/order.mk" -j8 gate ORDER_LOG="$order_log" 2>/dev/null ||
    fail "ordering probe make failed"
[ "$(sed -n 1p "$order_log")" = clear ] || fail "a gate step started before gate-receipt-clear finished"
[ "$(sed -n '$p' "$order_log")" = record ] || fail "gate recorded before its last step"
rm "$order_log"
make -s -C "$source_root" -f "$source_root/Makefile" -f "$test_root/order.mk" -j8 go-version ORDER_LOG="$order_log" 2>/dev/null ||
    fail "standalone probe make failed"
[ "$(cat "$order_log")" = step ] || fail "a standalone gate step ran gate-receipt-clear"

printf 'gate-receipt: binding, clear, refusals and gate wiring pass\n'
