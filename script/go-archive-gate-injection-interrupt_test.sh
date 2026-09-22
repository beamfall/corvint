#!/bin/sh
set -eu

# GAG-V0-006 evidence integrity: `script/go-archive-gate-injection_test.sh` is evidence only through
# its exit status, so an operator interrupt must end it with status 130 and remove its temporary
# directory. A trap that cleaned up and returned let the harness continue past a command whose
# failure it tolerates (`run_gate`, or the `wait` in case 4) into checks over the removed directory,
# where an empty command substitution passes: interrupted during case 4's drain, it printed its final
# pass line and exited 0. A stub `go` stands in for the toolchain and blocks in the gate's first
# archive call, so the interrupt lands inside `run_gate` without a real build.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-injection-interrupt-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

mkdir "$scratch/bin" "$scratch/tmp"
cat > "$scratch/bin/go" <<'STUB'
#!/bin/sh
: > "$CORVINT_INTERRUPT_TEST_MARKER"
exec sleep 60
STUB
chmod 0755 "$scratch/bin/go"

set -m
CORVINT_INTERRUPT_TEST_MARKER="$scratch/blocked" PATH="$scratch/bin:$PATH" TMPDIR="$scratch/tmp" \
    "$source_root/script/go-archive-gate-injection_test.sh" > "$scratch/out" 2>&1 &
harness=$!
set +m
attempt=0
until [ -f "$scratch/blocked" ]; do
    [ "$attempt" -lt 600 ] || { cat "$scratch/out" >&2; fail "the harness never reached the gate's archive call"; }
    sleep 0.1
    attempt=$((attempt + 1))
done
kill -INT -- "-$harness"
status=0
wait "$harness" || status=$?
[ "$status" -eq 130 ] || { cat "$scratch/out" >&2; fail "an interrupted harness exited $status, want 130"; }
[ -z "$(find "$scratch/tmp" -mindepth 1 -maxdepth 1)" ] || fail "an interrupted harness left its temporary directory"

printf 'go-archive-gate-injection-interrupt: an interrupted harness exits 130 and removes its temporary directory\n'
