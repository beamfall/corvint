#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# release-candidate-bytes.sh binds the Release gates linux-core job to one release candidate's bytes
# (PRS-V1-1900, decision 0420 item 2, V1-0374). The job rebuilds the archives at the dispatched sha
# with conformance/release-artifact-v0, which is reproducible (GOC-V0-006). With --expect, the
# sha256 of the rebuilt SHA256SUMS must equal the candidate's, so every archive the job qualifies is
# byte-identical to the candidate's; a different digest refuses (exit 1) and names both. Without
# --expect the record says SOURCE_ONLY: evidence for the sha's source, never for a release.
#
# usage: release-candidate-bytes.sh --sums SHA256SUMS --binary CORVINT [--expect SHA256] [--report FILE]...
#
# It prints one identity record: the checked-out commit and tree, the SHA256SUMS digest, the
# linux/amd64 archive row (which must match the archive beside SHA256SUMS), the extracted binary's
# digest, the runner identity (RUNNER_LABEL, ImageVersion, the run URL from GITHUB_SERVER_URL,
# GITHUB_REPOSITORY, GITHUB_RUN_ID and GITHUB_RUN_ATTEMPT), one `report SHA256 PATH` line per
# --report file, and last `verdict=CANDIDATE_BYTES` or `verdict=SOURCE_ONLY`. Exit 2 is a usage
# error or an unreadable input.
set -euo pipefail

usage() { printf 'usage: %s --sums SHA256SUMS --binary CORVINT [--expect SHA256] [--report FILE]...\n' "$0" >&2; exit 2; }
refuse() { printf 'release-candidate-bytes: REFUSE %s\n' "$1" >&2; exit "${2:-2}"; }

sums='' binary='' expect='' reports=()
while [ $# -gt 0 ]; do
  case $1 in
    --sums) [ $# -ge 2 ] || usage; sums=$2; shift 2 ;;
    --binary) [ $# -ge 2 ] || usage; binary=$2; shift 2 ;;
    --expect) [ $# -ge 2 ] || usage; expect=$2; shift 2 ;;
    --report) [ $# -ge 2 ] || usage; reports+=("$2"); shift 2 ;;
    *) usage ;;
  esac
done
[ -n "$sums" ] && [ -n "$binary" ] || usage
case $expect in
  "") ;;
  *[!0-9a-f]*) refuse "expect-malformed: --expect must be the 64 lowercase hex sha256 of the candidate's SHA256SUMS; fix: pass the digest field of shasum -a 256 SHA256SUMS" ;;
  *) [ "${#expect}" -eq 64 ] || refuse "expect-malformed: --expect must be the 64 lowercase hex sha256 of the candidate's SHA256SUMS; fix: pass the digest field of shasum -a 256 SHA256SUMS" ;;
esac

sha256() {
  local out
  if command -v sha256sum > /dev/null 2>&1; then out=$(sha256sum < "$1"); else out=$(shasum -a 256 < "$1"); fi
  printf '%s\n' "${out%% *}"
}
[ -f "$sums" ] && [ -r "$sums" ] || refuse "sums-unreadable: $sums; fix: run conformance/release-artifact-v0 archive first and pass its SHA256SUMS"
[ -f "$binary" ] && [ -r "$binary" ] || refuse "binary-unreadable: $binary; fix: extract corvint from corvint_linux_amd64.tar.gz and pass its path"

archive_name=corvint_linux_amd64.tar.gz
row=$(awk -v name="$archive_name" '$2 == name || $2 == "*" name { print $1 }' "$sums")
[ "$(printf '%s\n' "$row" | grep -c .)" = 1 ] || refuse "archive-row-missing: $sums has no single $archive_name row; fix: rebuild the archives, the producer writes one row per target"
archive=$(dirname "$sums")/$archive_name
[ -f "$archive" ] || refuse "archive-missing: $archive; fix: keep the archive beside its SHA256SUMS"
archive_sha=$(sha256 "$archive")
[ "$archive_sha" = "$row" ] || refuse "archive-row-mismatch: $archive is $archive_sha, SHA256SUMS says $row; fix: rebuild the archives into a new directory" 1

commit=$(git rev-parse --verify -q HEAD) || refuse "no-checkout: HEAD does not name a commit; fix: run it from the checkout of the dispatched sha"
tree=$(git rev-parse --verify -q 'HEAD^{tree}') || refuse "no-checkout: HEAD has no tree; fix: run it from the checkout of the dispatched sha"
sums_sha=$(sha256 "$sums")
printf 'commit=%s\n' "$commit"
printf 'tree=%s\n' "$tree"
printf 'sha256sums_sha256=%s\n' "$sums_sha"
printf 'archive=%s sha256=%s\n' "$archive_name" "$archive_sha"
printf 'binary_sha256=%s\n' "$(sha256 "$binary")"
printf 'runner=%s image=%s\n' "${RUNNER_LABEL:-unknown}" "${ImageVersion:-unknown}"
printf 'run=%s/%s/actions/runs/%s/attempts/%s\n' "${GITHUB_SERVER_URL:-unknown}" "${GITHUB_REPOSITORY:-unknown}" \
  "${GITHUB_RUN_ID:-unknown}" "${GITHUB_RUN_ATTEMPT:-unknown}"
for report in ${reports[@]+"${reports[@]}"}; do
  [ -f "$report" ] || refuse "report-missing: $report; fix: pass only reports that exist"
  printf 'report %s %s\n' "$(sha256 "$report")" "$report"
done
if [ -z "$expect" ]; then
  printf 'verdict=SOURCE_ONLY no candidate_sums_sha256 was dispatched; this is evidence for the source at %s, not for a release candidate\n' "$commit"
  exit 0
fi
if [ "$sums_sha" != "$expect" ]; then
  printf 'verdict=NOT_CANDIDATE\n'
  refuse "candidate-mismatch: the rebuilt SHA256SUMS is $sums_sha, the candidate's is $expect; these archives are not the candidate's bytes and qualify nothing; fix: dispatch at the candidate's commit, or correct candidate_sums_sha256" 1
fi
printf 'verdict=CANDIDATE_BYTES the rebuilt SHA256SUMS equals the candidate'"'"'s %s\n' "$expect"
