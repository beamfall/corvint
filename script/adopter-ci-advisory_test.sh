#!/usr/bin/env bash
# Tests for script/adopter-ci-advisory.sh (ADS-V0-650, V1-3732), with a stand-in corvint binary
# packed the way a release archive is: corvint_<platform>/{corvint,SHA256SUMS} plus the release
# SHA256SUMS beside the archive.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
script=$here/adopter-ci-advisory.sh
root=$(mktemp -d "${TMPDIR:-/tmp}/adopter-ci-advisory-test.XXXXXX")
trap 'rm -rf "$root"' EXIT INT TERM
fail() { echo "FAIL: $*" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then sum() { sha256sum "$@"; }; else sum() { shasum -a 256 "$@"; }; fi
platform=linux_amd64
base=0123456789abcdef0123456789abcdef01234567

# pack DIR [TAMPER]: build DIR/corvint_linux_amd64.tar.gz and DIR/SHA256SUMS. With TAMPER the
# binary changes after its inner checksum was written, and the outer checksum still matches.
pack() {
  local dir=$1 tamper=${2:-}
  local stage=$dir/stage/corvint_$platform
  mkdir -p "$stage"
  cat > "$stage/corvint" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >> "${FAKE_LOG:?}"
case "$*" in
  --version) echo "Corvint 1.0.0-rc.3 (build 9)" ;;
  *"adopt --setup"*) [ "${FAKE_FAIL:-}" = adopt ] && { echo boom >&2; exit 2; }; echo '{"ok":true,"tool":"adopt"}' ;;
  *"affected --base"*) echo '{"profile":"affected-plan/0"}' ;;
  *) exit 3 ;;
esac
FAKE
  chmod +x "$stage/corvint"
  (cd "$stage" && sum corvint > SHA256SUMS)
  if [ -n "$tamper" ]; then echo '# tampered' >> "$stage/corvint"; fi
  tar -czf "$dir/corvint_$platform.tar.gz" -C "$dir/stage" "corvint_$platform"
  (cd "$dir" && sum "corvint_$platform.tar.gz" > SHA256SUMS && echo "$(printf '%064d' 0)  corvint_darwin_arm64.tar.gz" >> SHA256SUMS)
}

# run NAME RELEASE_DIR [BASE]: run the script into $root/NAME/out, with stdout, stderr and rc kept.
run() {
  local name=$1
  shift
  mkdir -p "$root/$name"
  local rc=0
  FAKE_LOG=$root/$name/fake.log CORVINT_PLATFORM=$platform CORVINT_REPO=$root \
    CORVINT_ADVISORY_OUT=$root/$name/out GITHUB_PATH=$root/$name/path GITHUB_STEP_SUMMARY=$root/$name/summary \
    "$script" "$@" > "$root/$name/stdout" 2> "$root/$name/stderr" || rc=$?
  echo "$rc" > "$root/$name/rc"
}
rc() { cat "$root/$1/rc"; }

mkdir -p "$root/good" "$root/tampered"
pack "$root/good"
pack "$root/tampered" tamper

# 1. Verified install, both advisory steps run against the repository, outputs kept.
run ok "$root/good" "$base"
[ "$(rc ok)" = 0 ] || fail "verified run exited $(rc ok): $(cat "$root/ok/stderr")"
grep -q 'Corvint 1.0.0-rc.3' "$root/ok/out/version.txt" || fail "version not retained"
grep -qx -- "--root $root adopt --setup" "$root/ok/fake.log" || fail "adopt --setup not run: $(cat "$root/ok/fake.log")"
grep -qx -- "--root $root affected --base $base" "$root/ok/fake.log" || fail "affected not run"
[ "$(cat "$root/ok/out/adopt-setup.exit")" = 0 ] && [ "$(cat "$root/ok/out/affected.exit")" = 0 ] || fail "exit codes not retained"
grep -q '"tool":"adopt"' "$root/ok/out/adopt-setup.json" || fail "adopt output not retained"
grep -q "corvint_$platform\$" "$root/ok/path" || fail "GITHUB_PATH not extended"
grep -q 'Corvint advisory' "$root/ok/summary" || fail "step summary not written"

# 2. An advisory failure warns and still exits 0.
FAKE_FAIL=adopt run advisory "$root/good" "$base"
[ "$(rc advisory)" = 0 ] || fail "advisory failure changed the exit to $(rc advisory)"
[ "$(cat "$root/advisory/out/adopt-setup.exit")" = 2 ] || fail "advisory exit code not retained"
grep -q '::warning' "$root/advisory/stderr" || fail "advisory failure printed no warning"
grep -q 'affected: exit 0' "$root/advisory/stdout" || fail "a failed step stopped the next one"

# 3. Outer checksum mismatch: refused before extraction, nothing runs.
mkdir -p "$root/outer"
cp "$root/good/corvint_$platform.tar.gz" "$root/outer/"
echo "$(printf '%064d' 1)  corvint_$platform.tar.gz" > "$root/outer/SHA256SUMS"
run outer "$root/outer" "$base"
[ "$(rc outer)" = 1 ] || fail "outer mismatch exited $(rc outer)"
grep -q 'nothing was extracted' "$root/outer/stderr" || fail "outer mismatch reason missing"
[ ! -e "$root/outer/fake.log" ] || fail "a binary ran after an outer mismatch"
[ -z "$(find "$root/outer/out" -name corvint -type f)" ] || fail "archive extracted after an outer mismatch"

# 4. Inner checksum mismatch: refused before the binary runs.
run inner "$root/tampered" "$base"
[ "$(rc inner)" = 1 ] || fail "inner mismatch exited $(rc inner)"
grep -q 'no binary was run' "$root/inner/stderr" || fail "inner mismatch reason missing"
[ ! -e "$root/inner/fake.log" ] || fail "a binary ran after an inner mismatch"

# 5. No line for this platform, or two, is refused.
mkdir -p "$root/missing"
cp "$root/good/corvint_$platform.tar.gz" "$root/missing/"
grep -v "corvint_$platform" "$root/good/SHA256SUMS" > "$root/missing/SHA256SUMS"
run missing "$root/missing"
[ "$(rc missing)" = 1 ] && grep -q '0 times' "$root/missing/stderr" || fail "missing checksum line not refused"
cat "$root/good/SHA256SUMS" "$root/good/SHA256SUMS" > "$root/missing/SHA256SUMS"
run twice "$root/missing"
[ "$(rc twice)" = 1 ] && grep -q '2 times' "$root/twice/stderr" || fail "duplicate checksum line not refused"

# 6. Without BASE, affected is skipped and says so; a short BASE is a usage error.
run nobase "$root/good"
[ "$(rc nobase)" = 0 ] && grep -q 'affected: skipped, no BASE' "$root/nobase/stdout" || fail "no-BASE run did not skip affected"
! grep -q affected "$root/nobase/fake.log" || fail "affected ran without BASE"
run shortbase "$root/good" abc123
[ "$(rc shortbase)" = 1 ] && [ ! -e "$root/shortbase/fake.log" ] || fail "short BASE not refused"

echo "adopter-ci-advisory_test: ok"
