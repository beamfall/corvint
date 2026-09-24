#!/bin/sh
set -eu

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-release-checklist-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
repository="$test_root/repo"
mkdir -p "$repository/script"
cp "$source_root/script/release-checklist" "$repository/script/"
printf '0.5.0a1\n' > "$repository/VERSION"
git -C "$repository" init -q -b main
git -C "$repository" -c user.name=test -c user.email=test@example.invalid add .
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm fixture
real_git=$(command -v git)

checklist() {
  set +e
  "$repository/script/release-checklist" > "$test_root/report"
  status=$?
  set -e
  test "$status" -eq 1
}
row() { awk -F '\t' -v key="$1" '$2 == key { print $1 }' "$test_root/report"; }
reason() { awk -F '\t' -v key="$1" '$2 == key { print $4 }' "$test_root/report"; }

# GOC-V0-005: a cancelled measurement is never promoted to a current PASS.
checklist
test "$(row native-runtime)" = PASS
test "$(row native-performance)" = NOT_RUN
test "$(row go-archive)" = NOT_RUN
test "$(row full-gate)" = NOT_RUN
test "$(reason full-gate)" = "full gate receipt is absent"
test "$(row tag)" = NOT_RUN
test "$(row publication)" = NOT_RUN
test "$(row promotion)" = NOT_RUN
test -z "$(row packet-5)"
test -z "$(row wheel-integrity)"
# ARTIFACT-RDY-V0-001: exactly seven rows, in order.
test "$(cut -f 2 "$test_root/report" | tr "\n" " ")" = "native-runtime native-performance go-archive full-gate tag publication promotion "

# GOC-V0-005 falsifying assertion: the row must say the measurement was cancelled, not
# merely absent -- a wording regression that silently reworded this into a routine "not yet
# measured" NOT_RUN (dropping the owner-cancellation fact) would pass every check above.
test "$(reason native-performance)" = "paired measurement cancelled by owner; native performance unmeasured"

# GOC-V0-005 falsifying assertion: the checklist script must never itself claim a relative
# speed comparison (a "faster"/"Nx" claim in the reason text, or a NOT_RUN row quietly
# replaced by a PASS carrying such wording) -- that would resurrect the retired paired
# native/legacy performance measurement GOC-V0-005 forbids.
if grep -Eiq 'faster|slower|speedup|relative.speed|[0-9](\.[0-9]+)?x[^[:alnum:]]|x-?faster' "$source_root/script/release-checklist"; then
  printf 'release-checklist source names a relative-speed comparison; GOC-V0-005 retires paired performance measurement\n' >&2
  exit 1
fi

# GOC-V0-001 is decided from the Git index, not the worktree: an untracked stray legacy file
# passes (it was never staged), a staged-but-uncommitted one fails (the index is the
# reference, not HEAD), a committed one still fails, and one only removed from the worktree
# without `git rm` still fails too -- the masking shape a worktree-`test -f` check would miss.
for legacy in pyproject.toml setup.py src/corvint_cli.py; do
  mkdir -p "$(dirname "$repository/$legacy")"
  printf 'legacy\n' > "$repository/$legacy"
  checklist
  test "$(row native-runtime)" = PASS
  git -C "$repository" add "$legacy"
  checklist
  test "$(row native-runtime)" = FAIL
  git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm "add $legacy"
  checklist
  test "$(row native-runtime)" = FAIL
  rm "$repository/$legacy"
  checklist
  test "$(row native-runtime)" = FAIL
  git -C "$repository" rm -q --cached "$legacy"
  git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm "remove $legacy"
  checklist
  test "$(row native-runtime)" = PASS
done

# GOC-V0-010: full-gate is PASS only for a canonical receipt bound to HEAD's commit and tree and to
# the current archive witness's digest. A stale receipt or a different or missing witness is
# NOT_RUN; a malformed or symlinked receipt is FAIL.
git_dir="$repository/.git"
mkdir -p "$git_dir/corvint"
printf 'witness\n' > "$git_dir/corvint/release-go-archive-report.json"
full_gate_receipt() {
  printf 'corvint-gate-receipt/0 %s %s %s\n' "$(git -C "$repository" rev-parse HEAD)" \
    "$(git -C "$repository" rev-parse 'HEAD^{tree}')" \
    "$(shasum -a 256 "$git_dir/corvint/release-go-archive-report.json" | awk '{print $1}')"
}
full_gate_receipt > "$git_dir/corvint/release-gate-receipt"
checklist
test "$(row full-gate)" = PASS
printf 'witness rerecorded\n' > "$git_dir/corvint/release-go-archive-report.json"
checklist
test "$(row full-gate)" = NOT_RUN
test "$(reason full-gate)" = "full gate receipt names another archive witness"
rm "$git_dir/corvint/release-go-archive-report.json"
checklist
test "$(row full-gate)" = NOT_RUN
printf 'witness\n' > "$git_dir/corvint/release-go-archive-report.json"
printf 'receipt\n' > "$repository/receipt-change"
git -C "$repository" add receipt-change
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm receipt-change
checklist
test "$(row full-gate)" = NOT_RUN
test "$(reason full-gate)" = "full gate receipt is not bound to HEAD"
full_gate_receipt > "$git_dir/corvint/release-gate-receipt"
checklist
test "$(row full-gate)" = PASS
full_gate_receipt | sed 's/$/ extra/' > "$git_dir/corvint/release-gate-receipt"
checklist
test "$(row full-gate)" = FAIL
full_gate_receipt | sed 's/receipt\/0/receipt\/1/' > "$git_dir/corvint/release-gate-receipt"
checklist
test "$(row full-gate)" = FAIL
# A canonical line followed by bytes without a final LF is still one `wc -l` line: noncanonical.
{ full_gate_receipt; printf 'trailing'; } > "$git_dir/corvint/release-gate-receipt"
checklist
test "$(row full-gate)" = FAIL
full_gate_receipt > "$test_root/receipt-target"
ln -sf "$test_root/receipt-target" "$git_dir/corvint/release-gate-receipt"
checklist
test "$(row full-gate)" = FAIL
rm -rf "$git_dir/corvint"

# An operational failure while inspecting the index is an invalid environment, never proof that
# the retired legacy paths are absent.
mkdir -p "$test_root/failing-git"
cat > "$test_root/failing-git/git" <<'SH'
#!/bin/sh
for arg do
  if [ "$arg" = ls-files ]; then
    exit 128
  fi
done
exec "$REAL_GIT" "$@"
SH
chmod +x "$test_root/failing-git/git"
set +e
failure=$(REAL_GIT="$real_git" PATH="$test_root/failing-git:$PATH" \
  "$repository/script/release-checklist" 2>&1)
status=$?
set -e
test "$status" -eq 2
case $failure in
  *"release-checklist: git ls-files failed"*) ;;
  *) printf 'missing git failure diagnostic: %s\n' "$failure" >&2; exit 1 ;;
esac

# ARTIFACT-RDY-V0-001: a Git read failure while resolving the expected tag is an invalid
# environment, never the ordinary "no such tag" NOT_RUN that `rev-parse --verify --quiet`'s exit 1
# reports; any other nonzero exit (128 here, standing in for a corrupt object database) must be
# diagnosed and exit 2.
mkdir -p "$test_root/failing-tag-git"
cat > "$test_root/failing-tag-git/git" <<'SH'
#!/bin/sh
for arg do
  case "$arg" in
    refs/tags/*) exit 128 ;;
  esac
done
exec "$REAL_GIT" "$@"
SH
chmod +x "$test_root/failing-tag-git/git"
set +e
failure=$(REAL_GIT="$real_git" PATH="$test_root/failing-tag-git:$PATH" \
  "$repository/script/release-checklist" 2>&1)
status=$?
set -e
test "$status" -eq 2
case $failure in
  *"release-checklist: git rev-parse failed for refs/tags/v0.5.0a1"*) ;;
  *) printf 'missing tag-resolution failure diagnostic: %s\n' "$failure" >&2; exit 1 ;;
esac

# ARTIFACT-RDY-V0-003: only a tag counts; a branch spelled like the expected tag is not one.
git -C "$repository" branch v0.5.0a1
checklist
test "$(row tag)" = NOT_RUN
git -C "$repository" branch -q -D v0.5.0a1

# Decision 0379: the tag names the notes commit whose only parent is the gated HEAD and whose only
# changes are docs Markdown including docs/RELEASE-NOTES.md. A tag on HEAD itself is not that shape.
git -C "$repository" tag v0.5.0a1
checklist
test "$(row tag)" = FAIL
git -C "$repository" tag -d v0.5.0a1 >/dev/null
gated=$(git -C "$repository" rev-parse HEAD)
notes_commit() {
  git -C "$repository" checkout -q --detach "$gated"
  for path in "$@"; do
    mkdir -p "$repository/$(dirname "$path")"
    printf 'notes\n' >> "$repository/$path"
    git -C "$repository" add "$path"
  done
  git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm notes
  git -C "$repository" tag -f v0.5.0a1 >/dev/null
  git -C "$repository" checkout -q --detach "$gated"
}
notes_commit docs/RELEASE-NOTES.md docs/decisions/0001-note.md
checklist
test "$(row tag)" = PASS
notes_commit docs/decisions/0001-note.md
checklist
test "$(row tag)" = FAIL
test "$(reason tag)" = "expected release tag commit does not change docs/RELEASE-NOTES.md"
notes_commit docs/RELEASE-NOTES.md change
checklist
test "$(row tag)" = FAIL
test "$(reason tag)" = "expected release tag commit changes a path outside docs Markdown"
notes_commit docs/RELEASE-NOTES.md
git -C "$repository" checkout -q --detach v0.5.0a1
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -q --allow-empty -m later
git -C "$repository" tag -f v0.5.0a1 >/dev/null
git -C "$repository" checkout -q --detach "$gated"
checklist
test "$(row tag)" = FAIL
test "$(reason tag)" = "expected release tag is not a single-parent child of HEAD"
notes_commit docs/RELEASE-NOTES.md
git -C "$repository" checkout -q main
printf 'next\n' > "$repository/change"
git -C "$repository" add change
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm next
checklist
test "$(row tag)" = FAIL
# ARTIFACT-RDY-V0-009: a committed receipt the reader cannot evaluate (this fixture has no Go module)
# is FAIL, never PASS; an uncommitted receipt is not read at all.
mkdir -p "$repository/docs/releases/v0.5.0a1"
printf '{}\n' > "$repository/docs/releases/v0.5.0a1/publication-receipt.json"
checklist
test "$(row publication)" = NOT_RUN
git -C "$repository" add docs
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm receipt
checklist
test "$(row publication)" = FAIL
printf 'bad version\n' > "$repository/VERSION"
checklist
test "$(row tag)" = FAIL
test "$(row publication)" = NOT_RUN

# Readiness is read-only even with retained historical results and a dirty version file.
mkdir -p "$repository/conformance/perf-v0/results/historical"
printf '{"slicePerformanceStatus":"NOT_RUN"}\n' > "$repository/conformance/perf-v0/results/historical/report.json"
before=$(git -C "$repository" status --porcelain=v1 -uall)
prior=$(cat "$repository/conformance/perf-v0/results/historical/report.json")
checklist
test "$before" = "$(git -C "$repository" status --porcelain=v1 -uall)"
test "$prior" = "$(cat "$repository/conformance/perf-v0/results/historical/report.json")"
# ARTIFACT-RDY-V0-001 (ticket V1-0189): --pre-promotion prints the same seven rows and exits on the
# candidate rows only. NOT_RUN on native-performance, tag, publication and promotion does not lower
# the exit; a FAIL anywhere or a candidate row short of PASS still exits 1; any other argument is a
# usage error. A fake `go` stands in for the archive-status reader this fixture cannot build.
pre_promotion() {
  set +e
  "$repository/script/release-checklist" --pre-promotion > "$test_root/pre-report"
  pre_status=$?
  set -e
}
set +e
"$repository/script/release-checklist" --candidate > /dev/null 2>&1
status=$?
set -e
test "$status" -eq 2
set +e
"$repository/script/release-checklist" --pre-promotion extra > /dev/null 2>&1
status=$?
set -e
test "$status" -eq 2
printf '0.5.0a9\n' > "$repository/VERSION"
checklist
pre_promotion
test "$pre_status" -eq 1
cmp -s "$test_root/report" "$test_root/pre-report"
test "$(row go-archive)" = NOT_RUN
test "$(row tag)" = NOT_RUN
mkdir -p "$test_root/fake-go"
cat > "$test_root/fake-go/go" <<'SH'
#!/bin/sh
for arg do
  case $arg in
    archive-status) printf '%s\n' "${FAKE_ARCHIVE_STATUS:-PASS}"; exit 0 ;;
    publication-status) printf 'STALE\n'; exit 0 ;;
  esac
done
exit 1
SH
chmod +x "$test_root/fake-go/go"
mkdir -p "$git_dir/corvint"
printf 'witness\n' > "$git_dir/corvint/release-go-archive-report.json"
full_gate_receipt > "$git_dir/corvint/release-gate-receipt"
set +e
PATH="$test_root/fake-go:$PATH" "$repository/script/release-checklist" > "$test_root/report"
status=$?
PATH="$test_root/fake-go:$PATH" "$repository/script/release-checklist" --pre-promotion > "$test_root/pre-report"
pre_status=$?
set -e
test "$status" -eq 1
test "$pre_status" -eq 0
cmp -s "$test_root/report" "$test_root/pre-report"
test "$(row native-runtime)" = PASS
test "$(row native-performance)" = NOT_RUN
test "$(row go-archive)" = PASS
test "$(row full-gate)" = PASS
test "$(row tag)" = NOT_RUN
test "$(row publication)" = NOT_RUN
test "$(row promotion)" = NOT_RUN
# Each candidate row alone short of PASS still exits 1: a stale archive witness with a PASS full
# gate, then a PASS archive witness with no full-gate receipt.
set +e
FAKE_ARCHIVE_STATUS=STALE PATH="$test_root/fake-go:$PATH" "$repository/script/release-checklist" --pre-promotion > "$test_root/pre-report"
pre_status=$?
set -e
test "$pre_status" -eq 1
test "$(awk -F '\t' '$1 != "PASS" && $1 != "NOT_RUN" { print $2 }' "$test_root/pre-report")" = ""
test "$(awk -F '\t' '$2 == "go-archive" || $2 == "full-gate" { print $1 }' "$test_root/pre-report" | tr "\n" " ")" = "NOT_RUN PASS "
mv "$git_dir/corvint/release-gate-receipt" "$test_root/gate-receipt"
set +e
PATH="$test_root/fake-go:$PATH" "$repository/script/release-checklist" --pre-promotion > "$test_root/pre-report"
pre_status=$?
set -e
test "$pre_status" -eq 1
test "$(awk -F '\t' '$1 != "PASS" && $1 != "NOT_RUN" { print $2 }' "$test_root/pre-report")" = ""
test "$(awk -F '\t' '$2 == "go-archive" || $2 == "full-gate" { print $1 }' "$test_root/pre-report" | tr "\n" " ")" = "PASS NOT_RUN "
mv "$test_root/gate-receipt" "$git_dir/corvint/release-gate-receipt"
# VERSION names the tag already placed on an earlier commit: the tag row is the only FAIL and every
# candidate row is PASS, so the FAIL alone keeps the exit at 1.
printf '0.5.0a1\n' > "$repository/VERSION"
set +e
PATH="$test_root/fake-go:$PATH" "$repository/script/release-checklist" --pre-promotion > "$test_root/pre-report"
pre_status=$?
set -e
test "$pre_status" -eq 1
test "$(awk -F '\t' '$1 == "FAIL" { print $2 }' "$test_root/pre-report")" = tag
test "$(awk -F '\t' '$2 == "native-runtime" || $2 == "go-archive" || $2 == "full-gate" { print $1 }' "$test_root/pre-report" | tr "\n" " ")" = "PASS PASS PASS "
rm -rf "$git_dir/corvint"
printf 'release-checklist: native boundary, unmeasured status, full-gate receipt, tag binding, receipt row, pre-promotion exit and nonmutation pass\n'
