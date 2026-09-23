#!/bin/sh
# Install-lifecycle regression for the native Core (SOP-V0-001..006,
# docs/specs/stable-operations-v0.md). Against one release archive or one built binary, in a
# private temporary directory, it performs: verified install into store A, first index and read
# on a fixture repository, upgrade into store B, rollback to A, uninstall with repository and
# `.corvint` retention, backup and restore of `.corvint`, and corrupted-snapshot recovery. Local
# evidence only: it never publishes, signs, tags, or writes into the repository it is run from.
#
# Inputs (environment):
#   CORVINT_LIFECYCLE_ARCHIVE         a release `corvint_<goos>_<goarch>.tar.gz` whose root member
#                                     directory holds `corvint` and `SHA256SUMS` (the layout
#                                     conformance/release-artifact-v0 assembles), or
#   CORVINT_LIFECYCLE_BINARY          a built `corvint` executable; the script writes its own
#                                     SHA256SUMS for it. One of the two is required.
#   CORVINT_LIFECYCLE_UPGRADE_BINARY  the executable installed as the upgrade (default: the same
#                                     bytes as the first install, reported as a same-bytes upgrade).
#                                     Its packet is compared to the one it builds from a cold index
#                                     of a clone at the same commit, since a newer release may
#                                     change the packet wire; `packet=changed` reports that.
#   CORVINT_LIFECYCLE_REPORT          a file the step report is also written to (default: none).
set -eu

usage() {
  echo "usage: CORVINT_LIFECYCLE_ARCHIVE=<tar.gz> | CORVINT_LIFECYCLE_BINARY=<corvint> script/check-install-lifecycle.sh" >&2
  exit 2
}

archive=${CORVINT_LIFECYCLE_ARCHIVE:-}
binary=${CORVINT_LIFECYCLE_BINARY:-}
upgrade=${CORVINT_LIFECYCLE_UPGRADE_BINARY:-}
report=${CORVINT_LIFECYCLE_REPORT:-}
test -n "$archive" || test -n "$binary" || usage
test "$#" -eq 0 || usage

work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-install-lifecycle.XXXXXX")
cleanup() { rm -rf "$work"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
work=$(cd "$work" && pwd -P)

lines=""
say() {
  printf '%s\n' "$1"
  lines="$lines$1
"
}
fail() {
  say "step $1: FAIL $2"
  say "SUMMARY status=FAIL step=$1"
  write_report
  exit 1
}
write_report() {
  test -n "$report" || return 0
  printf '%s' "$lines" > "$report"
}

# sha256 of one file, portable across the shasum (Darwin) and sha256sum (Linux) hosts.
digest() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

# verify_sums DIR: every `<sha256>  <name>` row of DIR/SHA256SUMS must match the file beside it.
verify_sums() {
  test -f "$1/SHA256SUMS" || return 1
  while read -r sum name; do
    test -n "$name" || return 1
    test -f "$1/$name" || return 1
    test "$(digest "$1/$name")" = "$sum" || return 1
  done < "$1/SHA256SUMS"
}

# install_from_archive DIR: extract the archive's single root directory into DIR.
install_from_archive() {
  mkdir -p "$1/extract"
  tar -xzf "$archive" -C "$1/extract" || return 1
  root=$(ls "$1/extract")
  test "$(printf '%s\n' "$root" | wc -l | tr -d ' ')" = 1 || return 1
  test -d "$1/extract/$root" || return 1
  test -f "$1/extract/$root/corvint" || return 1
  cp "$1/extract/$root/corvint" "$1/corvint"
  cp "$1/extract/$root/SHA256SUMS" "$1/SHA256SUMS" 2>/dev/null || return 1
  rm -rf "$1/extract"
}

# install_from_binary DIR SOURCE: copy SOURCE and write the checksum manifest the install verifies.
install_from_binary() {
  cp "$2" "$1/corvint"
  printf '%s  corvint\n' "$(digest "$2")" > "$1/SHA256SUMS"
}

# install DIR SOURCE_BINARY: a verified install; SOURCE_BINARY empty selects the archive.
install() {
  mkdir -p "$1"
  if test -n "$2"; then
    install_from_binary "$1" "$2" || return 1
  else
    install_from_archive "$1" || return 1
  fi
  chmod 0755 "$1/corvint"
  verify_sums "$1" || { echo "checksum mismatch in $1" >&2; return 1; }
  "$1/corvint" --version > "$1/version.txt" || return 1
}

# fixture: a committed Go module the packet verbs can read.
make_fixture() {
  mkdir -p "$1"
  (
    cd "$1"
    git init -q
    git config user.email lifecycle@example.invalid
    git config user.name lifecycle
    printf 'module example.com/lifecycle\n\ngo 1.27\n' > go.mod
    printf 'package main\n\nfunc main() {}\n' > main.go
    printf 'package main\n\nimport "testing"\n\nfunc TestMain2(t *testing.T) {}\n' > main_test.go
    printf '# Agents\n\nGate: run the repository gate before commit.\n' > AGENTS.md
    git add -A
    git commit -q -m fixture
  )
}

# read_packet STORE OUT [ROOT]: one read verb whose bytes are compared across every later step.
read_packet() {
  "$1/corvint" --root "${3:-$fixture}" context --task "Identify the active work queue" --limit 1 > "$2"
}

# index STORE MODE OUT: `corvint index` (MODE=full) or `corvint index --if-stale` (MODE=if-stale).
index() {
  if test "$2" = if-stale; then
    "$1/corvint" --root "$fixture" index --if-stale > "$3"
  else
    "$1/corvint" --root "$fixture" index > "$3"
  fi
}

json_field() { sed -n "s/.*\"$2\":\([^,}]*\).*/\1/p" "$1" | head -n 1 | tr -d '"'; }
snapshot_path() { ls "$fixture/.corvint/index"/*.gob; }
file_size() { wc -c < "$1" | tr -d ' '; }

store="$work/store"
fixture="$work/fixture"
say "platform $(uname -sm)"
test -z "$archive" || say "archive $archive"
test -z "$binary" || say "binary $binary"

# 1. Fresh install into A with checksum verification.
install "$store/a" "$binary" || fail install-a "install or checksum verification failed"
version_a=$(cat "$store/a/version.txt")
say "step install-a: ok version=$version_a"

# 2. First index and first read on the fixture.
make_fixture "$fixture" || fail first-index "fixture repository was not created"
index "$store/a" full "$work/index1.json" || fail first-index "corvint index failed"
test "$(json_field "$work/index1.json" ok)" = true || fail first-index "index did not report ok"
snapshot=$(snapshot_path) || fail first-index "no snapshot under .corvint/index"
snapshot_size=$(file_size "$snapshot")
read_packet "$store/a" "$work/packet1.json" || fail first-index "read verb failed"
test -s "$work/packet1.json" || fail first-index "read verb produced no packet"
say "step first-index: ok snapshot=$(basename "$snapshot") bytes=$snapshot_size"

# 3. Upgrade into B; A stays in place. A same-bytes upgrade reads the first packet's bytes; a
#    distinct upgrade reads the bytes B builds from a cold index of a clone at the same commit.
upgrade_source=${upgrade:-$binary}
install "$store/b" "$upgrade_source" || fail upgrade-b "install or checksum verification failed"
version_b=$(cat "$store/b/version.txt")
index "$store/b" if-stale "$work/index-b.json" || fail upgrade-b "corvint index --if-stale failed"
read_packet "$store/b" "$work/packet-b.json" || fail upgrade-b "read verb failed"
if test -n "$upgrade"; then
  git clone -q "$fixture" "$work/fixture-cold" || fail upgrade-b "cold fixture clone failed"
  "$store/b/corvint" --root "$work/fixture-cold" index > "$work/index-cold.json" || fail upgrade-b "cold corvint index failed"
  read_packet "$store/b" "$work/packet-cold.json" "$work/fixture-cold" || fail upgrade-b "cold read verb failed"
  cmp -s "$work/packet-cold.json" "$work/packet-b.json" || fail upgrade-b "packet bytes differ from the upgrade's cold-index packet"
  packet=identical
  cmp -s "$work/packet1.json" "$work/packet-b.json" || packet=changed
  say "step upgrade-b: ok version=$version_b packet=$packet"
else
  cmp -s "$work/packet1.json" "$work/packet-b.json" || fail upgrade-b "packet bytes changed across the upgrade"
  say "step upgrade-b: ok version=$version_b same-bytes"
fi

# 4. Rollback: A still runs and still produces the same packet.
test -x "$store/a/corvint" || fail rollback-a "store A was disturbed by the upgrade"
index "$store/a" if-stale "$work/index-a2.json" || fail rollback-a "corvint index --if-stale failed"
read_packet "$store/a" "$work/packet-a2.json" || fail rollback-a "read verb failed"
cmp -s "$work/packet1.json" "$work/packet-a2.json" || fail rollback-a "packet bytes changed after rollback"
say "step rollback-a: ok"

# 5. Uninstall both stores: repository files and .corvint survive, the tree is unchanged.
rm -rf "$store/b" "$store/a"
test ! -e "$store/a/corvint" || fail uninstall "store A still present"
test -d "$fixture/.corvint/index" || fail uninstall ".corvint/index was removed by uninstall"
(cd "$fixture" && git diff --quiet HEAD && test -z "$(git status --porcelain)") || fail uninstall "fixture repository changed"
say "step uninstall: ok"

# 6. Backup .corvint, remove it, restore it; the restored snapshot is fresh and reads identically.
install "$store/a" "$binary" || fail backup-restore "reinstall failed"
(cd "$fixture" && tar -czf "$work/corvint-backup.tar.gz" .corvint) || fail backup-restore "backup failed"
rm -rf "$fixture/.corvint"
(cd "$fixture" && tar -xzf "$work/corvint-backup.tar.gz") || fail backup-restore "restore failed"
index "$store/a" if-stale "$work/index-restored.json" || fail backup-restore "corvint index --if-stale failed"
test "$(json_field "$work/index-restored.json" state)" = fresh || fail backup-restore "restored snapshot was not fresh"
read_packet "$store/a" "$work/packet-restored.json" || fail backup-restore "read verb failed"
cmp -s "$work/packet1.json" "$work/packet-restored.json" || fail backup-restore "packet bytes changed after restore"
say "step backup-restore: ok"

# 7. Corrupted snapshot: a read verb treats it as a miss, changes no byte of the packet and does
#    not rewrite it (read non-mutation); `corvint index --if-stale` reports it stale and rebuilds
#    a snapshot of the original size that the next `--if-stale` reports fresh.
corrupt_and_recover() {
  case $1 in
    truncate) : > "$snapshot" ;;
    overwrite) printf 'corrupt' | dd of="$snapshot" bs=1 seek=100 conv=notrunc 2>/dev/null ;;
  esac
  damaged_size=$(file_size "$snapshot")
  read_packet "$store/a" "$work/packet-$1.json" || fail "corrupt-$1" "read verb failed on a damaged snapshot"
  cmp -s "$work/packet1.json" "$work/packet-$1.json" || fail "corrupt-$1" "packet bytes changed on a damaged snapshot"
  test "$(file_size "$snapshot")" = "$damaged_size" || fail "corrupt-$1" "a read verb rewrote the snapshot"
  index "$store/a" if-stale "$work/index-$1.json" || fail "corrupt-$1" "corvint index --if-stale failed"
  test "$(json_field "$work/index-$1.json" mutates)" = true || fail "corrupt-$1" "index --if-stale did not rebuild the damaged snapshot"
  test "$(file_size "$snapshot")" = "$snapshot_size" || fail "corrupt-$1" "rebuilt snapshot size differs from the original"
  index "$store/a" if-stale "$work/index-$1-again.json" || fail "corrupt-$1" "second index --if-stale failed"
  test "$(json_field "$work/index-$1-again.json" state)" = fresh || fail "corrupt-$1" "rebuilt snapshot was not fresh"
  say "step corrupt-$1: ok read=miss-unchanged rebuild=mutates"
}
corrupt_and_recover truncate
corrupt_and_recover overwrite

say "SUMMARY status=PASS version_a=$(printf '%s' "$version_a" | tr ' ' '_') version_b=$(printf '%s' "$version_b" | tr ' ' '_')"
write_report
