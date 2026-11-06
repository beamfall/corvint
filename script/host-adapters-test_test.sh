#!/usr/bin/env bash
# host-adapters-test_test.sh (V1-4147): cancelling script/host-adapters-test.sh reaches the whole
# child tree. A stub runner stands in for node and starts a grandchild; SIGTERM to the script must
# end that grandchild (at once when it honours TERM, after HOST_ADAPTERS_GRACE when it ignores TERM),
# exit 143 and remove the script's scratch directory. Relative overrides (HOST_ADAPTERS_NODE,
# CORVINT_TEST_NATIVE_FIXTURE, CORVINT_TEST_REAL_BINARY) resolve against the caller's directory and
# still name the same files after the script changes directory. Run by `make host-adapters-test-test`.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
script=$here/host-adapters-test.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/host-adapters-test-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() {
  printf 'host-adapters-test_test: FAIL: %s\n' "$*" >&2
  exit 1
}
alive() { ps -p "$1" -o stat= 2> /dev/null | grep -qv '^Z'; }

# cancel_case NAME IGNORE runs the script with a stub runner whose grandchild ignores TERM when
# IGNORE is 1, sends the script SIGTERM once the grandchild reports its PID, and checks the outcome.
cancel_case() {
  local name=$1 ignore=$2 scratch=$tmp/$1 pid gpid rc watchdog
  mkdir -p "$scratch/tmp"
  mkfifo "$scratch/ready"
  cat > "$scratch/runner" << EOF
#!/usr/bin/env bash
[[ $ignore == 1 ]] && trap '' TERM
sleep 300 &
printf '%s\n' "\$!" > "$scratch/ready"
wait
EOF
  chmod +x "$scratch/runner"
  exec 3<> "$scratch/ready"
  TMPDIR=$scratch/tmp HOST_ADAPTERS_NODE=$scratch/runner HOST_ADAPTERS_GRACE=1 \
    CORVINT_TEST_NATIVE_FIXTURE=/bin/true CORVINT_TEST_REAL_BINARY=/bin/true \
    "$script" > "$scratch/out" 2>&1 &
  pid=$!
  read -t 60 -r gpid <&3 || fail "$name: the stub runner never started: $(cat "$scratch/out")"
  exec 3<&-
  (
    sleep 60
    kill -KILL "$pid" 2> /dev/null
  ) &
  watchdog=$!
  kill -TERM "$pid"
  wait "$pid"
  rc=$?
  kill "$watchdog" 2> /dev/null
  [[ $rc == 143 ]] || fail "$name: exit $rc, want 143: $(cat "$scratch/out")"
  if alive "$gpid"; then
    kill -KILL "$gpid" 2> /dev/null
    fail "$name: the grandchild $gpid outlived the cancelled script"
  fi
  [[ -z $(ls -A "$scratch/tmp") ]] || fail "$name: scratch left behind: $(ls -A "$scratch/tmp")"
}

# relative_case NAME NODE runs the script from a caller directory holding a stub runner and two
# stub binaries, passing the binaries as relative paths and NODE as the runner override. The runner
# records whether both binary variables are absolute paths to existing files from its own directory.
relative_case() {
  local name=$1 node=$2 caller=$tmp/$1 rc
  mkdir -p "$caller/tmp"
  cat > "$caller/runner" << 'EOF'
#!/usr/bin/env bash
for v in "$CORVINT_TEST_NATIVE_FIXTURE" "$CORVINT_TEST_REAL_BINARY"; do
  [[ $v == /* && -x $v ]] || { printf 'runner: not an absolute existing file: %s (cwd %s)\n' "$v" "$PWD"; exit 9; }
done
printf 'runner: ok\n'
EOF
  chmod +x "$caller/runner"
  /bin/cp -f /bin/true "$caller/fixture"
  /bin/cp -f /bin/true "$caller/corvint"
  [[ $node == ./runner ]] || node=$caller/runner
  (
    cd "$caller" || exit 2
    TMPDIR=$caller/tmp HOST_ADAPTERS_NODE=$node \
      CORVINT_TEST_NATIVE_FIXTURE=./fixture CORVINT_TEST_REAL_BINARY=corvint \
      "$script" > "$caller/out" 2>&1
  )
  rc=$?
  [[ $rc == 0 ]] || fail "$name: exit $rc, want 0: $(cat "$caller/out")"
  grep -q '^runner: ok$' "$caller/out" || fail "$name: the runner did not confirm both binaries: $(cat "$caller/out")"
}

cancel_case honours-term 0
cancel_case ignores-term 1
relative_case relative-binaries absolute
relative_case relative-node ./runner
printf 'host-adapters-test_test: PASS\n'
