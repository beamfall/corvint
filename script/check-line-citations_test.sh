#!/bin/sh
set -eu

# Covers the content anchor (a pinned citation fails when the cited lines change, an unpinned one
# keeps the pre-anchor behavior, and --hash round-trips into a passing pin) and the index
# resolution (DCG-V0-013): the fixture is a Git repository, so a citation resolves through the
# index and nothing the worktree alone says can change the result.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-line-citations-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/specs" "$test_root/docs/agent-memory" \
  "$test_root/docs/decisions" "$test_root/internal/demo"
cp "$source_root/script/check-line-citations.sh" "$test_root/script/"
git -C "$test_root" init -q -b main

# The gate resolves through the Git index, so a fixture file counts only once it is staged.
stage() { git -C "$test_root" -c user.name=t -c user.email=t@example.invalid add -- "$1"; }

# The fixture document is legacy; every other scanned fixture document is deliberately new.
printf '%s\n' 'docs/agent-memory/fixture.md' > "$test_root/script/line-citation-legacy-documents.txt"
stage script/line-citation-legacy-documents.txt

# DCG-V0-041: the unpinned citations the fixture document may carry. An unused row fails, so
# cite() rewrites the inventory with exactly the rows each case names.
inventory() {
  printf '# fixture inventory\n' > "$test_root/script/line-citation-unpinned-inventory.txt"
  for citation in "$@"; do
    printf 'docs/agent-memory/fixture.md\t%s\n' "$citation"
  done >> "$test_root/script/line-citation-unpinned-inventory.txt"
  stage script/line-citation-unpinned-inventory.txt
}
inventory

write_source() {
  printf '%s\n' 'package demo' '' "func Cited() int { return $1 }" 'func After() {}' \
    > "$test_root/internal/demo/demo.go"
  stage internal/demo/demo.go
}

# cite TEXT [ROW...]: the fixture document cites TEXT, and the inventory lists exactly ROW...
cite() {
  # shellcheck disable=SC2016
  printf '# Fixture\n\nThe claim rests on `%s`.\n' "$1" > "$test_root/docs/agent-memory/fixture.md"
  stage docs/agent-memory/fixture.md
  shift
  inventory "$@"
}

run() {
  status=0
  output=$(cd "$test_root" && script/check-line-citations.sh "$@" 2>&1) || status=$?
  printf '%s' "$output"
  return $status
}

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

write_source 1
anchor=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:3)
test "$anchor" = "internal/demo/demo.go:3@$(printf '%s' "$anchor" | sed 's/.*@//')" ||
  fail "--hash did not print a citation with an appended anchor: $anchor"

# 1. --hash round-trips: the anchor it prints is one the gate accepts.
cite "$anchor"
run >/dev/null || fail "the anchor --hash printed did not pass the gate"

# 2. Trailing whitespace is not cited content, so it must not break a pin.
sed -i.bak 's/func Cited() int { return 1 }/func Cited() int { return 1 }   /' "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
run >/dev/null || fail "trailing whitespace broke an otherwise matching anchor"
write_source 1

# 3. A changed cited line fails the pin and reports both hashes.
write_source 2
output=$(run) && fail "a changed cited line left the anchor passing"
case "$output" in
  *"cited content changed"*"@$(printf '%s' "$anchor" | sed 's/.*@//')"*) ;;
  *) fail "drift failure did not name the pinned anchor: $output" ;;
esac

# 4. The same change is invisible without a pin: unpinned citations keep the old behavior.
cite "internal/demo/demo.go:3" internal/demo/demo.go:3
run >/dev/null || fail "an unpinned citation failed after the cited line changed"

# 5. A range anchor covers every line in the range, not just its first.
range=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:3-4)
cite "$range"
run >/dev/null || fail "a freshly printed range anchor did not pass"
printf '%s\n' 'package demo' '' 'func Cited() int { return 2 }' 'func Renamed() {}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
run >/dev/null && fail "a change to the last line of a cited range left the anchor passing"

# 6. A pin may be written at full digest length, not only at the eight --hash prints.
write_source 1
full=$(cd "$test_root" && perl -MDigest::SHA=sha256_hex -e 'print sha256_hex("func Cited() int { return 1 }")')
cite "internal/demo/demo.go:3@$full"
run >/dev/null || fail "a full-length pin did not compare equal to the same content"

# 7. A file that exists on disk but is untracked does not resolve: that is the shape that let a
# citation into an author-only file pass the gate and dangle for everyone who clones.
printf '%s\n' 'package ghost' '' 'func Ghost() {}' > "$test_root/internal/demo/ghost.go"
cite "internal/demo/ghost.go:3" internal/demo/ghost.go:3
output=$(run) && fail "a citation into an untracked file resolved"
case "$output" in
  *"no such file in the Git index: internal/demo/ghost.go"*) ;;
  *) fail "the untracked-file failure did not name the index: $output" ;;
esac
stage internal/demo/ghost.go
run >/dev/null || fail "staging the cited file did not make the citation resolve"

# 8. A dirty worktree cannot change the result in either direction. The cited line is past the
# end of the worktree copy and the file is then removed from the worktree entirely; the index
# still carries three lines, so the gate must keep passing both times.
printf '%s\n' 'package ghost' > "$test_root/internal/demo/ghost.go"
run >/dev/null || fail "an unstaged truncation of the cited file changed the result"
rm "$test_root/internal/demo/ghost.go"
run >/dev/null || fail "deleting the cited file from the worktree alone changed the result"
git -C "$test_root" checkout -- internal/demo/ghost.go

# 9. DCG-V0-015: a tracked root-level extensionless path is a citation and is checked like any
# other: a real Makefile line passes and a blank Makefile line is refused.
printf '%s\n' 'all:' 'echo build' '' > "$test_root/Makefile"
stage Makefile
cite "Makefile:2" Makefile:2
run >/dev/null || fail "a citation into a tracked extensionless path did not resolve"
cite "Makefile:3" Makefile:3
output=$(run) && fail "a blank line in an extensionless path passed"
case "$output" in
  *"Makefile:3 is a blank or bracket-only line"*) ;;
  *) fail "the extensionless failure did not name the trivial line: $output" ;;
esac
cite "Makefile:9"
run >/dev/null && fail "a line past the end of an extensionless path passed"

# 10. An extensionless token the index does not track is prose, not a citation, and must not
# fail the gate — matching untracked paths broadly would turn backticked prose into failures.
cite "notafile:3"
run >/dev/null || fail "an untracked extensionless token was treated as a citation"

# 11. A line citation into a log or backlog file (BUILD-LOG.md, agent-memory/*.md) is
#    a position, not an entry, and must carry a content anchor; a bare line number is
#    rejected, and the same citation once pinned passes.
printf '%s\n' '# Other backlog file' '' 'A stable-looking line.' 'A second line.' \
  > "$test_root/docs/agent-memory/other.md"
stage docs/agent-memory/other.md
cite "docs/agent-memory/other.md:3"
output=$(run) && fail "an unpinned citation into an agent-memory backlog file passed"
case "$output" in
  *"is a log or backlog file"*) ;;
  *) fail "the log-or-backlog failure did not explain why: $output" ;;
esac
pin=$(cd "$test_root" && script/check-line-citations.sh --hash docs/agent-memory/other.md:3)
cite "$pin"
run >/dev/null || fail "a pinned citation into an agent-memory backlog file still failed"

# 12. A leading-slash absolute path is a citation too, not invisible prose: it never resolves
#     through the Git index (an absolute host path is never tracked), so it must fail loudly
#     instead of being silently skipped -- that silence is what let a scratch-directory citation
#     land in a spec unnoticed.
cite "/private/tmp/corvint-scratch-xyz/ghost.md:3"
output=$(run) && fail "a leading-slash absolute-path citation resolved"
case "$output" in
  *"no such file in the Git index: /private/tmp/corvint-scratch-xyz/ghost.md"*) ;;
  *) fail "the absolute-path failure did not name the index: $output" ;;
esac

# 13. Index reads share one `git cat-file --batch` co-process; when it exits non-zero the gate must
#     fail with that exit status instead of treating the missing bytes as an empty file.
cite "internal/demo/demo.go:3" internal/demo/demo.go:3
mkdir -p "$test_root/fakegit"
real_git=$(command -v git)
# shellcheck disable=SC2016
printf '#!/bin/sh\n[ "$1 $2" = "cat-file --batch" ] && exit 7\nexec "%s" "$@"\n' "$real_git" \
  > "$test_root/fakegit/git"
chmod +x "$test_root/fakegit/git"
status=0
output=$(cd "$test_root" && PATH="$test_root/fakegit:$PATH" script/check-line-citations.sh 2>&1) || status=$?
[ "$status" -eq 7 ] || fail "a failing git cat-file --batch exited $status, not 7: $output"
case "$output" in
  *"git cat-file --batch failed: exit 7"*) ;;
  *) fail "the batch failure did not report its exit status: $output" ;;
esac

# 14. A reversed range names no lines: a pin on it would hash the empty span and never detect
#     drift, so the gate and --hash both refuse it with a message distinct from past-the-end.
cite "internal/demo/demo.go:4-3"
output=$(run) && fail "a reversed range passed"
case "$output" in
  *"range end 3 is before its start 4"*) ;;
  *) fail "the reversed-range failure did not name both endpoints: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:4-3) >/dev/null 2>&1 \
  && fail "--hash printed an anchor for a reversed range"

# 15. A basename the index tracks at the repository root names exactly one file, so it is a
#     full path: `NOTES.md:N` is range-checked, and a basename tracked nowhere at the root
#     (`demo.go:9`) stays unchecked prose.
printf '%s\n' '# Notes' 'A root-level line.' > "$test_root/NOTES.md"
stage NOTES.md
cite "NOTES.md:2" NOTES.md:2
run >/dev/null || fail "a root-file citation on a real line failed"
cite "NOTES.md:9"
output=$(run) && fail "a root-file citation past the end passed"
case "$output" in
  *"line 9 is past the end of NOTES.md (2 lines)"*) ;;
  *) fail "the root-file failure did not name the root path: $output" ;;
esac
cite "demo.go:9"
run >/dev/null || fail "a bare basename not tracked at the root was checked"

# 16. A token that begins as a citation to a tracked path but has an invalid suffix is not
#     prose: report the whole malformed token instead of silently skipping it.
cite "internal/demo/demo.go:3-4@internal/demo/demo.go:3-4@7782963d"
output=$(run) && fail "a malformed citation-like token was silently skipped"
case "$output" in
  *"internal/demo/demo.go:3-4@internal/demo/demo.go:3-4@7782963d"*"malformed citation-like token"*) ;;
  *) fail "the malformed-token failure was unclear: $output" ;;
esac

# 17. The committed ceiling is a ratchet: one otherwise-valid unpinned citation fails when the
#     measured count exceeds it, and the diagnostic reports both values.
write_source 1
cite "internal/demo/demo.go:3" internal/demo/demo.go:3
# shellcheck disable=SC2016
sed -i.bak 's/my \$unpinned_ceiling = [0-9][0-9]*;/my $unpinned_ceiling = 0;/' \
  "$test_root/script/check-line-citations.sh"
stage script/check-line-citations.sh
output=$(run) && fail "an unpinned count above the committed ceiling passed"
case "$output" in
  *"unpinned citation count 1 exceeds committed ceiling 0"*) ;;
  *) fail "the unpinned-ceiling failure did not report both counts: $output" ;;
esac

# 18. A scanned document absent from the index-pinned legacy allowlist requires anchors for every
#     citation. An unstaged allowlist edit cannot grandfather it; the same citation pinned passes.
cp "$source_root/script/check-line-citations.sh" "$test_root/script/"
stage script/check-line-citations.sh
# shellcheck disable=SC2016
printf '# New fixture\n\nThe claim rests on `%s`.\n' 'internal/demo/demo.go:3' \
  > "$test_root/docs/specs/new.md"
stage docs/specs/new.md
printf '%s\n' 'docs/specs/new.md' >> "$test_root/script/line-citation-legacy-documents.txt"
output=$(run) && fail "an unpinned citation in a new document passed"
case "$output" in
  *"docs/specs/new.md:3"*"outside the legacy allowlist require an @<hex> content anchor"*) ;;
  *) fail "the new-document failure did not require an anchor: $output" ;;
esac
new_pin=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:3)
# shellcheck disable=SC2016
printf '# New fixture\n\nThe claim rests on `%s`.\n' "$new_pin" > "$test_root/docs/specs/new.md"
stage docs/specs/new.md
run >/dev/null || fail "a pinned citation in a new document did not pass"

# 19. Line numbers are 1-based: `:0` names no line, so neither the gate nor --hash may resolve it
#     (a zero index would otherwise read the file's last line and pass).
cite "internal/demo/demo.go:0"
output=$(run) && fail "a citation of line 0 passed"
case "$output" in
  *"line 0 is before the start of internal/demo/demo.go"*) ;;
  *) fail "the line-0 failure was unclear: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:0) >/dev/null 2>&1 \
  && fail "--hash printed an anchor for line 0"

# 20. DCG-V0-001: docs/decisions/*.md is a scanned document class, not exempt prose. A new
#     decision fixture is outside the legacy allowlist, so its unpinned citation must require an
#     anchor exactly as a new spec document's would, and the same citation pinned then passes.
cite "internal/demo/demo.go:3" internal/demo/demo.go:3
# shellcheck disable=SC2016
printf '# Decision 9001 - fixture\n\nThe claim rests on `%s`.\n' 'internal/demo/demo.go:3' \
  > "$test_root/docs/decisions/9001-fixture.md"
stage docs/decisions/9001-fixture.md
output=$(run) && fail "an unpinned citation in a decisions document passed"
case "$output" in
  *"docs/decisions/9001-fixture.md:3"*"outside the legacy allowlist require an @<hex> content anchor"*) ;;
  *) fail "the decisions-document failure did not require an anchor: $output" ;;
esac
decision_pin=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:3)
# shellcheck disable=SC2016
printf '# Decision 9001 - fixture\n\nThe claim rests on `%s`.\n' "$decision_pin" \
  > "$test_root/docs/decisions/9001-fixture.md"
stage docs/decisions/9001-fixture.md
run >/dev/null || fail "a pinned citation in a decisions document did not pass"

# 21. DCG-V0-021: an unstaged edit to a cited file or a scanned document still cannot change the
#     result, but the gate names the file instead of passing silently; an unstaged file the gate
#     never read is not named, and staging the edit clears the note.
(cd "$test_root" && git checkout -- script/line-citation-legacy-documents.txt)
printf '%s\n' 'package other' > "$test_root/internal/demo/other.go"
stage internal/demo/other.go
printf '%s\n' 'package other' '' > "$test_root/internal/demo/other.go"
printf '%s\n' '// moved' >> "$test_root/internal/demo/demo.go"
output=$(run) || fail "an unstaged edit to a cited file changed the result"
case $output in
*'unstaged edits to 1 file(s) it read were not checked'*'internal/demo/demo.go'*) ;;
*) fail "an unstaged edit to a cited file was not named: $output" ;;
esac
case $output in
*other.go*) fail "an unstaged file the gate never read was named: $output" ;;
esac
stage internal/demo/demo.go
stage internal/demo/other.go
output=$(run) || fail "staging the edit changed the result"
case $output in
*'unstaged edits'*) fail "the note survived staging: $output" ;;
esac
# A scanned document edited but not staged is named too, and the note leaves a failing run's
# exit status and findings intact.
cite "internal/demo/demo.go:99"
printf '%s\n' 'An unstaged sentence.' >> "$test_root/docs/agent-memory/fixture.md"
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "beside an unstaged edit, a citation past the end exited $rc, not 1: $output"
case $output in
*'1 citation(s) do not resolve'*'unstaged edits to 1 file(s) it read were not checked'*'docs/agent-memory/fixture.md'*) ;;
*) fail "an unstaged edit to a scanned document was not named beside the failure: $output" ;;
esac

# 22. DCG-V0-001: skipped basenames are visible without affecting pass/fail. DCG-V0-046 (V1-2821):
#     by default they are one counted note line; --verbose lists each one.
cite 'demo.go:999'
output=$(run) || fail "a skipped basename changed a passing result"
case $output in
*'skipped `demo.go:999`'*) fail "the default output listed a skip instead of counting it: $output" ;;
*'note: skipped 1 citation(s) whose basename is not tracked at repository root (--verbose lists them)'*) ;;
*) fail "skipped basename was not counted: $output" ;;
esac
output=$(run --verbose) || fail "--verbose changed a passing result"
case $output in
*'docs/agent-memory/fixture.md:3: skipped `demo.go:999`: basename is not tracked at repository root'*) ;;
*) fail "skipped basename was not explained under --verbose: $output" ;;
esac
# A tracked root basename still receives the normal checks, with no skip note.
printf '%s\n' '# Root' > "$test_root/ROOT.md"
stage ROOT.md
cite 'ROOT.md:999'
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a tracked root basename was skipped: $output"
output=$(run --verbose) || rc=$?
case $output in *'skipped `ROOT.md:999`'*|*'note: skipped'*) fail "checked root basename reported skipped" ;; esac
# Notes also leave existing failures intact.
cite 'demo.go:999` and `internal/demo/demo.go:999'
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "skip note changed a failing result: $output"
case $output in
*'1 citation(s) do not resolve'*'note: skipped 1 citation(s)'*) ;;
*) fail "the failure did not print before the skip count: $output" ;;
esac
rc=0
output=$(run --verbose) || rc=$?
[ "$rc" = 1 ] || fail "--verbose changed a failing result: $output"
case $output in
*'1 citation(s) do not resolve'*'skipped `demo.go:999`'*) ;;
*) fail "--verbose did not list the skip after the failure: $output" ;;
esac

# 23. DCG-V0-001 (V1-1480): docs/RELEASE-NOTES-*.md is a scanned document class, so a pinned
#     citation in release notes fails once the cited lines change, naming the release notes.
cite "internal/demo/demo.go:3" internal/demo/demo.go:3
write_source 1
notes_pin=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:3)
# shellcheck disable=SC2016
printf '# Release notes\n\nThe claim rests on `%s`.\n' "$notes_pin" \
  > "$test_root/docs/RELEASE-NOTES-fixture.md"
stage docs/RELEASE-NOTES-fixture.md
run >/dev/null || fail "a pinned citation in release notes did not pass"
write_source 2
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a drifted release-notes citation exited $rc, not 1: $output"
case $output in
*'docs/RELEASE-NOTES-fixture.md:3'*'cited content changed'*) ;;
*) fail "the release-notes drift was not reported: $output" ;;
esac

# 24. DCG-V0-022 (V1-1530, V1-2139): a pinned two-line span whose text moved unchanged is named at
#     its new position, and --repin rewrites the document to it; changed or repeated text still fails.
#     Earlier fixture documents are dropped so the fixture document is the only one citing demo.go.
git -C "$test_root" rm -r -q --cached docs
rm -rf "$test_root/docs"
mkdir -p "$test_root/docs/agent-memory"
write_source 1
span=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:3-4)
cite "$span"
pin=${span#*@}
printf '%s\n' 'package demo' '' '// one' '// two' 'func Cited() int { return 1 }' 'func After() {}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a moved pinned citation exited $rc, not 1: $output"
case $output in
*"cited content moved unchanged to \`internal/demo/demo.go:5-6@$pin\`"*'--repin'*) ;;
*) fail "the unique move was not named with its new lines: $output" ;;
esac
# DCW-V0-353 (V1-2836): a failing run prints the failures, then the exact repin command, then the
# notes. A skipped basename and an unstaged edit add notes, which must follow, never precede.
cp "$test_root/docs/agent-memory/fixture.md" "$test_root/fixture.md.keep"
# shellcheck disable=SC2016
printf '# Fixture\n\nThe claim rests on `%s` and `demo.go:999`.\n' "$span" > "$test_root/docs/agent-memory/fixture.md"
stage docs/agent-memory/fixture.md
printf '// unstaged\n' >> "$test_root/internal/demo/demo.go"
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "the ordered failing run exited $rc, not 1: $output"
expected_head="line citations: 1 citation(s) do not resolve:
  docs/agent-memory/fixture.md:3  \`$span\`  cited content moved unchanged to \`internal/demo/demo.go:5-6@$pin\` (script/check-line-citations.sh --repin rewrites it)
line citations: fix: 1 citation(s) moved unchanged; run script/check-line-citations.sh --repin (it stages the documents it rewrites), then rerun make line-citations-check
line citations: note: skipped 1 citation(s) whose basename is not tracked at repository root (--verbose lists them)
line citations: note: this gate reads the Git index, so unstaged edits to 1 file(s) it read were not checked (git add them to include them):
  internal/demo/demo.go"
test "$output" = "$expected_head" || fail "failures, repin command and notes are out of order:
$output"
git -C "$test_root" checkout -q -- internal/demo/demo.go
/bin/mv -f "$test_root/fixture.md.keep" "$test_root/docs/agent-memory/fixture.md"
stage docs/agent-memory/fixture.md
printf 'dirty\n' >> "$test_root/docs/agent-memory/fixture.md"
rc=0
output=$(cd "$test_root" && script/check-line-citations.sh --repin 2>&1) || rc=$?
[ "$rc" = 1 ] || fail "--repin rewrote a document whose worktree differs from the index: $output"
case $output in *'differs from the index'*) ;; *) fail "--repin refusal not named: $output" ;; esac
cite "$span"
out=$(cd "$test_root" && script/check-line-citations.sh --repin 2>&1) || fail "--repin failed on a unique move: $out"
grep -q "internal/demo/demo.go:5-6@$pin" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin did not rewrite the citation to lines 5-6"
# DCG-V0-048 (V1-3228): --repin staged its rewrite, so the index-reading check passes unstaged.
case $out in *'staged them (git add -- docs/agent-memory/fixture.md)'*) ;; *) fail "--repin did not name what it staged: $out" ;; esac
[ -z "$(git -C "$test_root" diff --name-only -- docs/agent-memory/fixture.md)" ] || fail "--repin left its rewrite unstaged"
run >/dev/null || fail "the repinned citation did not pass without manual staging"
# 24b. DCG-V0-048: a failed `git add` (index.lock held) exits non-zero naming the command; a pass
#      that still rewrote at the fifth pass exits non-zero as not converged; a document whose name
#      is a glob stages only itself, never an unstaged sibling the glob would match.
cite "$span"
: > "$test_root/.git/index.lock"
rc=0
out=$(cd "$test_root" && script/check-line-citations.sh --repin 2>&1) || rc=$?
rm -f "$test_root/.git/index.lock"
[ "$rc" = 1 ] || fail "--repin exited $rc after a failed git add: $out"
case $out in *'could not stage them; run `git add -- docs/agent-memory/fixture.md`'*) ;; *) fail "failed staging not named: $out" ;; esac
cite "$span"
rc=0
out=$(cd "$test_root" && CHECK_LINE_CITATIONS_REPIN_PASS=5 script/check-line-citations.sh --repin 2>&1) || rc=$?
[ "$rc" = 1 ] || fail "--repin exited $rc when its fifth pass still rewrote: $out"
case $out in *'did not converge'*) ;; *) fail "non-convergence not named: $out" ;; esac
cite "$span"
printf 'sibling\n' > "$test_root/docs/agent-memory/f.md"
printf '# Glob\n\nAlso `%s`.\n' "$span" > "$test_root/docs/agent-memory/[fx].md"
stage docs/agent-memory/f.md
git -C "$test_root" --literal-pathspecs add -- 'docs/agent-memory/[fx].md'
printf 'unstaged sibling edit\n' >> "$test_root/docs/agent-memory/f.md"
out=$(cd "$test_root" && script/check-line-citations.sh --repin 2>&1) || fail "--repin of a glob-named document failed: $out"
git -C "$test_root" diff --name-only | grep -qx 'docs/agent-memory/f.md' || fail "--repin staged a sibling its glob-named document matches"
[ -z "$(git -C "$test_root" --literal-pathspecs diff --name-only -- 'docs/agent-memory/[fx].md')" ] || fail "--repin left the glob-named rewrite unstaged"
git -C "$test_root" --literal-pathspecs rm -q -f --cached -- 'docs/agent-memory/f.md' 'docs/agent-memory/[fx].md'
rm -f "$test_root/docs/agent-memory/f.md" "$test_root/docs/agent-memory/[fx].md"

# 25. Moved text that now appears twice is ambiguous and is not relocated.
cite "$span"
printf '%s\n' 'package demo' '' '// x' 'func Cited() int { return 1 }' 'func After() {}' \
  'func Cited() int { return 1 }' 'func After() {}' > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "an ambiguous move exited $rc, not 1: $output"
case $output in
*'cited content changed'*"appears at 2 positions (\`internal/demo/demo.go:4-5@$pin\`, \`internal/demo/demo.go:6-7@$pin\`)"*) ;;
*) fail "the ambiguous move was relocated or its candidate positions not named: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin with nothing to move failed"
grep -q "internal/demo/demo.go:3-4@$pin" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin rewrote an ambiguous citation"

# 26. Changed text is not a move.
write_source 2
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "changed text exited $rc, not 1: $output"
case $output in
*'moved unchanged'*) fail "changed text was reported as a move: $output" ;;
*'cited content changed'*) ;;
*) fail "changed text was not reported: $output" ;;
esac

# 28. A unique match of one short line is not move evidence: the cited `return nil` changed and
#     another `return nil` became the only match, so the citation stays unresolved and --repin
#     leaves it (DCG-V0-044 relocates only a distinctive lone line).
printf '%s\n' 'package demo' '' 'func A() error {' '	return nil' '}' > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
single=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:4)
cite "$single"
printf '%s\n' 'package demo' '' 'func A() error {' '	return errA' '}' 'func B() error {' '	return nil' '}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a one-line unique match exited $rc, not 1: $output"
case $output in
*'moved unchanged'*) fail "a one-line unique match was relocated: $output" ;;
*'cited content changed'*'appears once at `internal/demo/demo.go:7@'*'not relocated'*) ;;
*) fail "the one-line match was not named as unrelocated: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin with a one-line match failed"
grep -q "$single" "$test_root/docs/agent-memory/fixture.md" || fail "--repin rewrote a one-line match"

# 28b. DCW-V0-354 (V1-2852) with the DCG-V0-044 threshold: one cited line of 29 non-whitespace
#      characters (at least 20, below the lane's former 32) whose text is byte-identical at exactly
#      one new position moved, so it is named as a move and --repin rewrites it; the same line at
#      two new positions is ambiguous and refused naming both candidate positions.
long='	return errors.New("bad header")'
printf '%s\n' 'package demo' '' 'func A() error {' "$long" '}' > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
single=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:4)
one_pin=${single#*@}
cite "$single"
printf '%s\n' 'package demo' '' '// inserted' '// above' 'func A() error {' "$long" '}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a moved distinctive one-line citation exited $rc, not 1: $output"
case $output in
*"cited content moved unchanged to \`internal/demo/demo.go:6@$one_pin\`"*) ;;
*) fail "the distinctive one-line move was not named with its new line: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin failed on a one-line move"
grep -q "internal/demo/demo.go:6@$one_pin" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin did not rewrite the one-line citation to line 6"
stage docs/agent-memory/fixture.md
run >/dev/null || fail "the repinned one-line citation did not pass"
cite "$single"
printf '%s\n' 'package demo' '' 'func A() error {' '	return nil' "$long" '}' 'func B() error {' "$long" '}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "an ambiguous one-line move exited $rc, not 1: $output"
case $output in
*'moved unchanged'*) fail "an ambiguous one-line move was relocated: $output" ;;
*"appears at 2 positions (\`internal/demo/demo.go:5@$one_pin\`, \`internal/demo/demo.go:8@$one_pin\`)"*) ;;
*) fail "the ambiguous one-line move did not name its candidates: $output" ;;
esac

# 29. --repin rewrites the reported occurrence only: two identical `:3-4@PIN` continuations on one
#     line resolve to different files, and only the one whose file moved is rewritten.
write_source 1
cp "$test_root/internal/demo/demo.go" "$test_root/internal/demo/copy.go"
stage internal/demo/copy.go
head_demo=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:1)
head_copy=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/copy.go:1)
# shellcheck disable=SC2016
printf '# Fixture\n\nSee `%s` `:3-4@%s` and `%s` `:3-4@%s`.\n' "$head_demo" "$pin" "$head_copy" "$pin" \
  > "$test_root/docs/agent-memory/fixture.md"
stage docs/agent-memory/fixture.md
run >/dev/null || fail "the two-continuation fixture did not pass before the move"
printf '%s\n' 'package demo' '' '// one' '// two' 'func Cited() int { return 1 }' 'func After() {}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin of one continuation failed"
grep -qF "\`$head_demo\` \`:5-6@$pin\` and \`$head_copy\` \`:3-4@$pin\`" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin did not rewrite exactly the moved continuation: $(cat "$test_root/docs/agent-memory/fixture.md")"
stage docs/agent-memory/fixture.md
run >/dev/null || fail "the repinned continuation fixture did not pass"
git -C "$test_root" rm -q --cached internal/demo/copy.go
rm -f "$test_root/internal/demo/copy.go"

# 31. DCG-V0-044 (V1-2814): a pinned one-line span whose distinctive line (at least 20
#     non-whitespace characters) moved unchanged and now matches exactly one line is relocated,
#     and --repin rewrites it and reports the move; the same line appearing twice, or no longer
#     appearing at all, still fails naming the citation and is not rewritten.
lone='	message := "single-line citation moved by an unrelated edit"'
printf '%s\n' 'package demo' '' 'func Lone() {' "$lone" '}' > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
single=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:4)
lone_pin=${single##*@}
cite "$single"
run >/dev/null || fail "the one-line fixture did not pass before the move"
printf '%s\n' 'package demo' '' '// inserted' '// above' 'func Lone() {' "$lone" '}' > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a moved one-line span exited $rc, not 1: $output"
case $output in
*"docs/agent-memory/fixture.md:3"*"$single"*'cited content moved unchanged to `internal/demo/demo.go:6@'"$lone_pin"*) ;;
*) fail "the distinctive one-line move was not named at its new position: $output" ;;
esac
repinned=$(cd "$test_root" && script/check-line-citations.sh --repin 2>/dev/null) || fail "--repin of a one-line move failed"
case $repinned in
*"docs/agent-memory/fixture.md:3"*"$single"*' -> '*"internal/demo/demo.go:6@$lone_pin"*) ;;
*) fail "--repin did not report the one-line relocation: $repinned" ;;
esac
grep -q "internal/demo/demo.go:6@$lone_pin" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin did not rewrite the one-line move"
stage docs/agent-memory/fixture.md
run >/dev/null || fail "the repinned one-line citation did not pass"
# Ambiguous: the distinctive line now appears twice, so neither copy is chosen.
cite "$single"
printf '%s\n' 'package demo' '' 'func Lone() {' '	_ = 0' "$lone" '}' 'func Twin() {' "$lone" '}' \
  > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "an ambiguous one-line move exited $rc, not 1: $output"
case $output in
*'moved unchanged'*) fail "an ambiguous one-line move was relocated: $output" ;;
*"$single"*'cited content changed'*'appears at 2 positions'*) ;;
*) fail "the ambiguous one-line move was not named: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin with an ambiguous one-line match failed"
grep -q "$single" "$test_root/docs/agent-memory/fixture.md" || fail "--repin rewrote an ambiguous one-line match"
# Missing: the line changed, so there is nothing to move to.
printf '%s\n' 'package demo' '' 'func Lone() {' '	message := "the cited line was edited"' '}' > "$test_root/internal/demo/demo.go"
stage internal/demo/demo.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a missing one-line match exited $rc, not 1: $output"
case $output in
*'moved unchanged'*|*'appears'*) fail "a missing one-line match was reported as a move: $output" ;;
*"$single"*'cited content changed'*) ;;
*) fail "the missing one-line match was not named: $output" ;;
esac
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin with a missing one-line match failed"
grep -q "$single" "$test_root/docs/agent-memory/fixture.md" || fail "--repin rewrote a missing one-line match"

# 32. DCG-V0-045 (V1-2820): lines inserted above cited spans move every one by the same offset.
#     The hash search alone cannot relocate a repeated span or a short lone line, but --repin
#     proves the move from the diff between HEAD and the index and rewrites both; a span whose
#     text changed is not rewritten, and --repin prints the span the diff maps it to with that
#     span's current anchor. An unstaged cited file is named with its exact `git add`, and a
#     --base that names no commit refuses.
printf '%s\n' 'package demo' '' 'func Shift() int {' '	if err != nil {' '		return nil' '	}' \
  '	if err != nil {' '		return nil' '	}' '	x := 1' '	return x' '}' > "$test_root/internal/demo/shift.go"
stage internal/demo/shift.go
pinned=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/shift.go:4-6 internal/demo/shift.go:5 internal/demo/shift.go:10-11)
block=$(printf '%s\n' "$pinned" | sed -n 1p)
short=$(printf '%s\n' "$pinned" | sed -n 2p)
body=$(printf '%s\n' "$pinned" | sed -n 3p)
cite "$block\` and \`$short\` and \`$body"
run >/dev/null || fail "the shift fixture did not pass before the insertion"
git -C "$test_root" -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false \
  -c core.hooksPath=/dev/null commit -q -m base
printf '%s\n' 'package demo' '' '// one' '// two' '// three' 'func Shift() int {' '	if err != nil {' \
  '		return nil' '	}' '	if err != nil {' '		return nil' '	}' '	x := 2' '	return x' '}' \
  > "$test_root/internal/demo/shift.go"
stage internal/demo/shift.go
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "spans shifted by an insertion exited $rc, not 1: $output"
case $output in
*'moved unchanged'*) fail "the hash search alone relocated a repeated or short span: $output" ;;
*"$block"*'appears at 2 positions'*) ;;
*) fail "the repeated span was not reported as ambiguous before --repin: $output" ;;
esac
rc=0
repinned=$(cd "$test_root" && script/check-line-citations.sh --repin 2>/dev/null) || rc=$?
[ "$rc" = 0 ] || fail "--repin of diff-proven moves exited $rc: $repinned"
moved_block="internal/demo/shift.go:7-9@${block##*@}"
moved_short="internal/demo/shift.go:8@${short##*@}"
suggested=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/shift.go:13-14)
case $repinned in
*"$block"*" -> \`$moved_block\`"*) ;;
*) fail "--repin did not relocate the repeated span by the diff: $repinned" ;;
esac
case $repinned in
*"$short"*" -> \`$moved_short\`"*) ;;
*) fail "--repin did not relocate the short lone line by the diff: $repinned" ;;
esac
case $repinned in
*"$body"*' changed since HEAD; the diff maps it to `'"$suggested"'`'*) ;;
*) fail "--repin did not suggest the changed span's mapped lines and anchor: $repinned" ;;
esac
grep -q "$moved_block" "$test_root/docs/agent-memory/fixture.md" &&
  grep -q "$moved_short" "$test_root/docs/agent-memory/fixture.md" &&
  grep -q "$body" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin did not rewrite exactly the moved spans: $(cat "$test_root/docs/agent-memory/fixture.md")"
stage docs/agent-memory/fixture.md
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "the changed span passed after --repin: $output"
case $output in
*'1 citation(s) do not resolve'*"$body"*'cited content changed'*) ;;
*) fail "after --repin, anything but the changed span failed: $output" ;;
esac
printf '%s\n' '// unstaged' >> "$test_root/internal/demo/shift.go"
output=$(cd "$test_root" && script/check-line-citations.sh --repin 2>&1) || true
case $output in
*'run `git add -- internal/demo/shift.go`, then rerun --repin'*) ;;
*) fail "--repin did not name the staging step for an unstaged cited file: $output" ;;
esac
git -C "$test_root" checkout -q -- internal/demo/shift.go
rc=0
output=$(cd "$test_root" && script/check-line-citations.sh --repin --base no-such-rev 2>&1) || rc=$?
[ "$rc" != 0 ] || fail "--repin --base with no commit exited 0: $output"
case $output in *'--base no-such-rev does not name a commit'*) ;; *) fail "the bad --base was not named: $output" ;; esac
git -C "$test_root" rm -q --cached internal/demo/shift.go
rm -f "$test_root/internal/demo/shift.go"
cite "$single"

# 33. DCG-V0-047 (V1-3124), DCG-V0-048 (V1-3228): a transitive chain repins in one --repin call
#     with no hand-run --hash and no manual staging. The fixture pins a spec line that itself cites
#     a moved source line; --repin moves the spec's token, which changes the spec line, stages the
#     spec, and its second pass re-anchors the fixture's citation because the line changed only
#     inside its citation tokens, a `:N@pin` continuation among them.
mkdir -p "$test_root/docs/specs"
printf '%s\n' 'package demo' '' 'func ChainTarget() int { return 42 }' 'func ChainOther() int { return 7 }' \
  > "$test_root/internal/demo/chain.go"
stage internal/demo/chain.go
inner=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/chain.go:3)
other=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/chain.go:4)
printf '# Chain\n\nThe target is `%s`, then `:4@%s` today.\n' "$inner" "${other##*@}" > "$test_root/docs/specs/chain.md"
stage docs/specs/chain.md
outer=$(cd "$test_root" && script/check-line-citations.sh --hash docs/specs/chain.md:3)
cite "$outer"
run >/dev/null || fail "the chain fixture did not pass before the move: $(run)"
git -C "$test_root" -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false \
  -c core.hooksPath=/dev/null commit -q -m chain
printf '%s\n' 'package demo' '' '// one' '// two' 'func ChainTarget() int { return 42 }' 'func ChainOther() int { return 7 }' \
  > "$test_root/internal/demo/chain.go"
stage internal/demo/chain.go
rc=0
repinned=$(cd "$test_root" && script/check-line-citations.sh --repin 2>&1) || rc=$?
[ "$rc" = 0 ] || fail "--repin of the chain exited $rc: $repinned"
grep -q "internal/demo/chain.go:5@${inner##*@}" "$test_root/docs/specs/chain.md" || fail "--repin did not move the inner citation"
grep -q "\`:6@${other##*@}\`" "$test_root/docs/specs/chain.md" ||
  fail "--repin did not move the inner continuation: $(cat "$test_root/docs/specs/chain.md")"
reanchored=$(cd "$test_root" && script/check-line-citations.sh --hash docs/specs/chain.md:3)
case $repinned in
*'--repin pass 2 rereads the staged rewrite'*"$outer"*" -> \`$reanchored\`"*) ;;
*) fail "--repin did not re-anchor the transitive citation to $reanchored in its second pass: $repinned" ;;
esac
[ -z "$(git -C "$test_root" diff --name-only)" ] || fail "--repin left a rewrite unstaged: $(git -C "$test_root" diff --name-only)"
run >/dev/null || fail "the re-anchored chain did not pass without manual staging: $(run)"
git -C "$test_root" rm -q --cached internal/demo/chain.go docs/specs/chain.md
rm -f "$test_root/internal/demo/chain.go" "$test_root/docs/specs/chain.md"
cite "$single"

# 33b. DCG-V0-047: a backticked `name:number` that names no file is prose, not a citation. When
#      the inner citation moves and `retry:3` becomes `retry:0` on the same line, --repin still
#      refuses to re-anchor the outer citation, because the line changed outside its citations.
printf '%s\n' 'package demo' '' 'func ProseTarget() int { return 42 }' > "$test_root/internal/demo/prose.go"
stage internal/demo/prose.go
inner=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/prose.go:3)
printf '# Prose\n\nThe target is `%s` with `retry:3` today.\n' "$inner" > "$test_root/docs/specs/prose.md"
stage docs/specs/prose.md
outer=$(cd "$test_root" && script/check-line-citations.sh --hash docs/specs/prose.md:3)
cite "$outer"
run >/dev/null || fail "the prose fixture did not pass before the move: $(run)"
git -C "$test_root" -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false \
  -c core.hooksPath=/dev/null commit -q -m prose
printf '%s\n' 'package demo' '' '// one' '// two' 'func ProseTarget() int { return 42 }' > "$test_root/internal/demo/prose.go"
stage internal/demo/prose.go
perl -0pi -e 's/`retry:3`/`retry:0`/' "$test_root/docs/specs/prose.md"
stage docs/specs/prose.md
# One --repin moves the inner citation and stages it (DCG-V0-048); its second pass must refuse.
(cd "$test_root" && script/check-line-citations.sh --repin >/dev/null 2>&1) || fail "--repin of the prose chain exited non-zero"
grep -q "internal/demo/prose.go:5@${inner##*@}\` with \`retry:0\`" "$test_root/docs/specs/prose.md" ||
  fail "the prose fixture was not set up: $(cat "$test_root/docs/specs/prose.md")"
grep -qF "$outer" "$test_root/docs/agent-memory/fixture.md" ||
  fail "--repin re-anchored a citation whose line changed a prose name:number: $(cat "$test_root/docs/agent-memory/fixture.md")"
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "a citation whose line changed a prose name:number passed: $output"
case $output in
*"$outer"*'cited content changed'*) ;;
*) fail "the prose change was not reported as changed content: $output" ;;
esac
git -C "$test_root" rm -q --cached internal/demo/prose.go docs/specs/prose.md
rm -f "$test_root/internal/demo/prose.go" "$test_root/docs/specs/prose.md"
cite "$single"

# 27. DCG-V0-041 (V1-1952): an unpinned legacy citation edited to another non-blank line has no
#     inventory row, so it fails with a --hash hint even though the line it now names exists;
#     the untouched row keeps passing, and a second copy of an inventoried citation fails.
write_source 1
cite "internal/demo/demo.go:4" internal/demo/demo.go:3
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "an edited unpinned legacy citation onto a non-blank line exited $rc, not 1: $output"
case $output in
*'unpinned citation is new or edited'*'--hash internal/demo/demo.go:4'*) ;;
*) fail "the edited unpinned citation was not named with a pin hint: $output" ;;
esac
cite "internal/demo/demo.go:3" internal/demo/demo.go:3
run >/dev/null || fail "an inventoried unpinned citation failed"
cite 'internal/demo/demo.go:3` and `internal/demo/demo.go:3' internal/demo/demo.go:3
output=$(run) && fail "a second copy of a once-inventoried unpinned citation passed: $output"
# The inventory only shrinks: a row no citation matches fails, so a retired row cannot admit a
# later citation with the same key.
cite "internal/demo/demo.go:3" internal/demo/demo.go:3 internal/demo/demo.go:4
rc=0
output=$(run) || rc=$?
[ "$rc" = 1 ] || fail "an unused inventory row exited $rc, not 1: $output"
case $output in
*'line-citation-unpinned-inventory.txt'*'internal/demo/demo.go:4'*'match no unpinned citation'*) ;;
*) fail "the unused inventory row was not named: $output" ;;
esac

# 30. DCG-V0-043 (V1-2152): --hash on a path with unmerged index entries and no stage 0 exits
#     non-zero and names the unmerged entry, rather than the bare `git cat-file` failure.
blob=$(printf 'x\ny\n' | git -C "$test_root" hash-object -w --stdin)
git -C "$test_root" rm -q --cached internal/demo/demo.go
printf '100644 %s 1\tinternal/demo/demo.go\n100644 %s 2\tinternal/demo/demo.go\n' "$blob" "$blob" |
  git -C "$test_root" update-index --index-info
rc=0
output=$(cd "$test_root" && script/check-line-citations.sh --hash internal/demo/demo.go:1 2>&1) || rc=$?
[ "$rc" != 0 ] || fail "--hash on an unmerged path exited 0: $output"
case $output in
*'internal/demo/demo.go is unmerged in the Git index (stages 1,2, no stage 0)'*) ;;
*) fail "--hash on an unmerged path did not name the unmerged entry: $output" ;;
esac

echo "check-line-citations_test.sh: ok"
