#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# ACR-V1-006, ACR-V1-007, ACR-V1-013 to ACR-V1-015: tests for script/acceptance-log-audit.sh
# over a fixture tree.

here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
audit=$here/acceptance-log-audit.sh
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-acceptance-log-audit-test.XXXXXX")
trap 'rm -rf "$root"' EXIT INT TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

mkdir -p "$root/docs/specs/openspec/changes/one-change" \
  "$root/docs/specs/openspec/changes/archive/2026-11-10-old-change"
log=$root/docs/specs/openspec/acceptance-log.tsv
index=$root/docs/specs/REQUIREMENTS.tsv
plan=$root/docs/specs/PRODUCT-PLAN-1.0.md

write_index() {
  printf 'id\tfile\ttitle\tacceptance\tacceptedBy\n' > "$index"
  printf 'PROD-A-001\tdocs/specs/PRODUCT-PLAN-1.0.md\tt\towner\tdecision 0001\n' >> "$index"
  printf 'PROD-A-002\tdocs/specs/PRODUCT-PLAN-1.0.md\tt\tstanding\t2026-10-11\n' >> "$index"
  printf 'PROD-A-003\tdocs/specs/PRODUCT-PLAN-1.0.md\tt\tunrecorded\t-\n' >> "$index"
  printf 'OTH-V0-001\tdocs/specs/other.md\tt\tunrecorded\t-\n' >> "$index"
}
write_log() {
  printf 'id\tkind\tmode\tdate\townerRef\trationale\n' > "$log"
  printf 'PROD-A-001\tprod\towner\t2026-11-10\tdecision 0001\tr\n' >> "$log"
  printf 'PROD-A-002\tprod\tauto\t2026-11-10\tstanding\tr\n' >> "$log"
  printf 'PROD-A-003\tprod\tpending-owner\t2026-11-10\t-\tr\n' >> "$log"
  printf 'O1\to-row\towner\t2026-11-10\tdecision 0001\tr\n' >> "$log"
  printf 'one-change\tchange\tauto\t2026-11-10\tstanding\tr\n' >> "$log"
  printf 'old-change\tchange\tauto\t2026-11-10\tstanding\tr\n' >> "$log"
}
readme=$root/docs/specs/openspec/README.md
write_folders() {
  printf '# Change\n\nIntent status: accepted under the standing auto-accept\n' > "$root/docs/specs/openspec/changes/one-change/proposal.md"
  printf '# Change\n\nIntent status: accepted under the standing auto-accept\n' > "$root/docs/specs/openspec/changes/archive/2026-11-10-old-change/proposal.md"
  printf '# Change proposals\n\n| Change | Purpose | PROD requirements | Status |\n|---|---|---|---|\n| `one-change` | p | PROD-A-001 | auto-accepted |\n' > "$readme"
}
printf '# Plan\n\n| O-row | Text |\n|---|---|\n| O1 | first |\n' > "$plan"
write_index
write_log
write_folders
clean='total	rows 6	missing 0	unknown 0	duplicate 0	mismatch 0	change-missing 0	header 0	index 0'

# 1. A log that agrees with the index, the plan, the proposal headers and the README index has
# no finding, strict or not. An archived folder needs no README index row.
out=$("$audit" --strict "$root") || fail "a clean log exited $?: $out"
test "$out" = "$clean" || fail "a clean log printed: $out"

# 2. ACR-V1-006: a PROD requirement and a plan O-row with no row are named.
printf 'PROD-A-004\tdocs/specs/PRODUCT-PLAN-1.0.md\tt\tunrecorded\t-\n' >> "$index"
printf '| O2 | second |\n' >> "$plan"
out=$("$audit" "$root") || fail "the report mode exited $? on a missing row"
case $out in
  *'MISSING_ROW	PROD-A-004	prod'*'MISSING_ROW	O2	o-row'*'missing 2	unknown 0	duplicate 0	mismatch 0'*) ;;
  *) fail "missing rows printed: $out" ;;
esac
status=0
"$audit" --strict "$root" > /dev/null || status=$?
test "$status" = 1 || fail "--strict exited $status on a missing row, want 1"
write_index
printf '# Plan\n\n| O1 | first |\n' > "$plan"

# 3. ACR-V1-007: a requirement marked standing in its spec and owner in the log names both modes.
sed 's/^PROD-A-002	prod	auto/PROD-A-002	prod	owner/' "$log" > "$log.new" && mv "$log.new" "$log"
out=$("$audit" "$root") || fail "the report mode exited $? on a mismatch"
case $out in
  *'MODE_MISMATCH	PROD-A-002	log owner	spec standing'*'mismatch 1'*) ;;
  *) fail "a mode mismatch printed: $out" ;;
esac
# ACR-V1-008: pending-owner agrees only with no acceptance marker.
write_log
sed 's/^PROD-A-001	prod	owner/PROD-A-001	prod	pending-owner/' "$log" > "$log.new" && mv "$log.new" "$log"
out=$("$audit" "$root")
case $out in
  *'MODE_MISMATCH	PROD-A-001	log pending-owner	spec owner'*) ;;
  *) fail "a pending-owner row over an owner marker printed: $out" ;;
esac

# 4. Unknown and duplicate rows are named.
write_log
printf 'PROD-A-009\tprod\towner\t2026-11-10\tx\tr\n' >> "$log"
printf 'O9\to-row\towner\t2026-11-10\tx\tr\n' >> "$log"
printf 'no-such-change\tchange\tauto\t2026-11-10\tx\tr\n' >> "$log"
printf 'O1\to-row\towner\t2026-11-10\tx\tr\n' >> "$log"
# A folder nested in a change, such as its specs/, is no change.
mkdir -p "$root/docs/specs/openspec/changes/one-change/specs"
printf 'specs\tchange\tauto\t2026-11-10\tx\tr\n' >> "$log"
out=$("$audit" "$root")
case $out in
  *'DUPLICATE_ROW	O1	o-row'*'UNKNOWN_ROW	PROD-A-009	prod'*'UNKNOWN_ROW	O9	o-row'*'UNKNOWN_ROW	no-such-change	change'*'UNKNOWN_ROW	specs	change'*'unknown 4	duplicate 1'*) ;;
  *) fail "unknown and duplicate rows printed: $out" ;;
esac

# 6. ACR-V1-013: a change folder, active or archived, with no change row is named.
write_log
mkdir -p "$root/docs/specs/openspec/changes/new-change"
printf 'Intent status: proposed (only the owner can accept it)\n' > "$root/docs/specs/openspec/changes/new-change/proposal.md"
printf '| `new-change` | p | PROD-A-003 | waiting for owner |\n' >> "$readme"
sed '/^old-change	/d' "$log" > "$log.new" && mv "$log.new" "$log"
out=$("$audit" "$root")
case $out in
  *'MISSING_CHANGE_ROW	2026-11-10-old-change'*'MISSING_CHANGE_ROW	new-change'*'change-missing 2	header 0	index 0') ;;
  *) fail "folders with no change row printed: $out" ;;
esac
status=0
"$audit" --strict "$root" > /dev/null || status=$?
test "$status" = 1 || fail "--strict exited $status on a missing change row, want 1"
write_log
printf 'new-change\tchange\tpending-owner\t2026-11-11\tG14\tr\n' >> "$log"
out=$("$audit" --strict "$root") || fail "a pending-owner folder with a proposed header exited $?: $out"
test "$out" = 'total	rows 7	missing 0	unknown 0	duplicate 0	mismatch 0	change-missing 0	header 0	index 0' || fail "a pending-owner folder printed: $out"

# 7. ACR-V1-014: a proposal header that disagrees with the row, or has no status, is named.
printf 'Intent status: proposed (written 2026-11-09)\n' > "$root/docs/specs/openspec/changes/one-change/proposal.md"
printf 'Intent status: accepted by the owner\n' > "$root/docs/specs/openspec/changes/new-change/proposal.md"
printf '# no status line\n' > "$root/docs/specs/openspec/changes/archive/2026-11-10-old-change/proposal.md"
out=$("$audit" "$root")
case $out in
  *'HEADER_MISMATCH	2026-11-10-old-change	log auto	header (none)'*'HEADER_MISMATCH	new-change	log pending-owner	header accepted by the owner'*'HEADER_MISMATCH	one-change	log auto	header proposed (written 2026-11-09)'*'header 3	index 0') ;;
  *) fail "header mismatches printed: $out" ;;
esac
rm "$root/docs/specs/openspec/changes/new-change/proposal.md"
out=$("$audit" "$root")
case $out in
  *'HEADER_MISMATCH	new-change	log pending-owner	header (none)'*) ;;
  *) fail "a folder with no proposal.md printed: $out" ;;
esac

# 8. ACR-V1-015: a README index row that is missing or names another status is named.
write_folders
printf 'Intent status: proposed (only the owner can accept it)\n' > "$root/docs/specs/openspec/changes/new-change/proposal.md"
printf '| `new-change` | p | PROD-A-003 | proposed |\n' >> "$readme"
out=$("$audit" "$root")
case $out in
  *'INDEX_MISMATCH	new-change	log pending-owner	index proposed	want waiting for owner'*'index 1') ;;
  *) fail "an index row with another status printed: $out" ;;
esac
write_folders
out=$("$audit" "$root")
case $out in
  *'INDEX_MISMATCH	new-change	log pending-owner	index (none)	want waiting for owner'*'index 1') ;;
  *) fail "a folder with no index row printed: $out" ;;
esac
rm -r "$root/docs/specs/openspec/changes/new-change"
write_log

# 5. A malformed log, a closed-set violation and an unreadable file exit 2.
write_log
printf 'PROD-A-001\tprod\taccepted\t2026-11-10\tx\tr\n' >> "$log"
status=0
"$audit" "$root" > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "a mode outside the closed set exited $status, want 2"
for row in 'PROD-A-001\tprod\towner\t\tx\tr' 'PROD-A-001\tprod\towner\t2026-11-10\t\tr' \
  'PROD-A-001\tprod\towner\t2026-11-10\tx\t'; do
  printf 'id\tkind\tmode\tdate\townerRef\trationale\n' > "$log"
  printf "$row\n" >> "$log"
  status=0
  "$audit" "$root" > /dev/null 2>&1 || status=$?
  test "$status" = 2 || fail "a row with an empty cell exited $status, want 2"
done
printf 'id\tmode\n' > "$log"
status=0
"$audit" "$root" > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "a log with another header exited $status, want 2"
rm "$log"
status=0
"$audit" "$root" > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "an absent log exited $status, want 2"

printf 'ok\n'
