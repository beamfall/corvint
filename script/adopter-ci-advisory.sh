#!/usr/bin/env bash
# Adopter CI advisory step (ADS-V0-650, docs/AUTOMATION.md "Adopter CI recipe").
# usage: adopter-ci-advisory.sh RELEASE_DIR [BASE]
#
# RELEASE_DIR holds a downloaded Corvint release archive and the release SHA256SUMS. The script
# verifies the archive for this host before extracting it, verifies the archive's own SHA256SUMS,
# probes --version, then runs `adopt --setup` and, with a full 40-hex BASE, `affected --base BASE`
# against CORVINT_REPO (default: the current directory). Install or verification failures exit 1;
# the two Corvint steps are advisory: their output and exit code are kept under CORVINT_ADVISORY_OUT
# and a failure prints a warning but exits 0. It downloads nothing.
#
# Optional environment: CORVINT_PLATFORM (os_arch, default from uname), CORVINT_REPO,
# CORVINT_ADVISORY_OUT (default ${RUNNER_TEMP:-${TMPDIR:-/tmp}}/corvint-advisory), and the GitHub
# Actions files GITHUB_PATH and GITHUB_STEP_SUMMARY when set.
set -euo pipefail

die() { echo "adopter-ci-advisory: $*" >&2; exit 1; }
warn() { echo "::warning title=corvint advisory::$*" >&2; }

[ "$#" -ge 1 ] && [ "$#" -le 2 ] || die "usage: adopter-ci-advisory.sh RELEASE_DIR [BASE]"
release_dir=$(cd "$1" && pwd) || die "release directory $1 is not readable"
base=${2:-}
if [ -n "$base" ] && ! [[ "$base" =~ ^[0-9a-f]{40}$ ]]; then
  die "BASE must be a full 40-hex commit id"
fi
repo=$(cd "${CORVINT_REPO:-.}" && pwd) || die "repository ${CORVINT_REPO:-.} is not readable"

platform=${CORVINT_PLATFORM:-}
if [ -z "$platform" ]; then
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) die "unsupported host system $(uname -s); release targets are darwin and linux" ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) die "unsupported host CPU $(uname -m); release targets are amd64 and arm64" ;;
  esac
  platform=${os}_${arch}
fi
case "$platform" in
  darwin_amd64 | darwin_arm64 | linux_amd64 | linux_arm64) ;;
  *) die "unsupported platform $platform" ;;
esac
archive=corvint_${platform}.tar.gz

if command -v sha256sum >/dev/null 2>&1; then
  sha256check() { sha256sum -c "$1"; }
else
  sha256check() { shasum -a 256 -c "$1"; }
fi

out=${CORVINT_ADVISORY_OUT:-${RUNNER_TEMP:-${TMPDIR:-/tmp}}/corvint-advisory}
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d "$out/install.XXXXXX")

# 1. The release checksum names the archive exactly once and matches it, before any extraction.
[ -f "$release_dir/SHA256SUMS" ] || die "$release_dir/SHA256SUMS is missing"
awk -v name="$archive" '$2 == name || $2 == "*" name' "$release_dir/SHA256SUMS" > "$work/selected.sha256"
lines=$(wc -l < "$work/selected.sha256" | tr -d ' ')
[ "$lines" = 1 ] || die "SHA256SUMS names $archive $lines times; want exactly once"
(cd "$release_dir" && sha256check "$work/selected.sha256") >&2 ||
  die "$archive does not match the release SHA256SUMS; nothing was extracted"

# 2. Extract into a fresh directory and verify the archive's own checksums.
mkdir "$work/extract"
tar -xzf "$release_dir/$archive" -C "$work/extract"
bin_dir=$work/extract/corvint_${platform}
[ -f "$bin_dir/SHA256SUMS" ] || die "the archive has no corvint_${platform}/SHA256SUMS"
(cd "$bin_dir" && sha256check SHA256SUMS) >&2 ||
  die "the extracted files do not match the archive SHA256SUMS; no binary was run"
[ -x "$bin_dir/corvint" ] || die "the archive has no executable corvint"
"$bin_dir/corvint" --version > "$out/version.txt" || die "corvint --version failed"

# 3. Advisory steps: keep each output and exit code; a failure warns and continues.
advisory() {
  local name=$1
  shift
  local rc=0
  "$bin_dir/corvint" --root "$repo" "$@" > "$out/$name.json" 2> "$out/$name.stderr" || rc=$?
  echo "$rc" > "$out/$name.exit"
  if [ "$rc" -ne 0 ]; then
    warn "corvint $* exited $rc (advisory; see $out/$name.stderr)"
  fi
  echo "$name: exit $rc" >> "$out/summary.txt"
}
: > "$out/summary.txt"
echo "version: $(head -n 1 "$out/version.txt")" >> "$out/summary.txt"
advisory adopt-setup adopt --setup
if [ -n "$base" ]; then
  advisory affected affected --base "$base"
else
  echo "affected: skipped, no BASE" >> "$out/summary.txt"
fi

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "$bin_dir" >> "$GITHUB_PATH"
fi
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  { echo '### Corvint advisory'; echo; sed 's/^/    /' "$out/summary.txt"; } >> "$GITHUB_STEP_SUMMARY"
fi
cat "$out/summary.txt"
