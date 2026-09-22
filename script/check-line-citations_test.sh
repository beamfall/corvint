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

write_source() {
  printf '%s\n' 'package demo' '' "func Cited() int { return $1 }" 'func After() {}' \
    > "$test_root/internal/demo/demo.go"
  stage internal/demo/demo.go
}

cite() {
  # shellcheck disable=SC2016
  printf '# Fixture\n\nThe claim rests on `%s`.\n' "$1" > "$test_root/docs/agent-memory/fixture.md"
  stage docs/agent-memory/fixture.md
}

run() {
  status=0
  output=$(cd "$test_root" && script/check-line-citations.sh 2>&1) || status=$?
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
cite "internal/demo/demo.go:3"
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
cite "internal/demo/ghost.go:3"
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
cite "Makefile:2"
run >/dev/null || fail "a citation into a tracked extensionless path did not resolve"
cite "Makefile:3"
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

# 11. A line citation into a prepended backlog file (BUILD-LOG.md, agent-memory/*.md) is
#    unstable by construction and must carry a content anchor; a bare line number is
#    rejected, and the same citation once pinned passes.
printf '%s\n' '# Other backlog file' '' 'A stable-looking line.' 'A second line.' \
  > "$test_root/docs/agent-memory/other.md"
stage docs/agent-memory/other.md
cite "docs/agent-memory/other.md:3"
output=$(run) && fail "an unpinned citation into a prepended agent-memory file passed"
case "$output" in
  *"is prepended"*) ;;
  *) fail "the prepended-doc failure did not explain why: $output" ;;
esac
pin=$(cd "$test_root" && script/check-line-citations.sh --hash docs/agent-memory/other.md:3)
cite "$pin"
run >/dev/null || fail "a pinned citation into a prepended agent-memory file still failed"

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
cite "internal/demo/demo.go:3"
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
cite "NOTES.md:2"
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
cite "internal/demo/demo.go:3"
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
cite "internal/demo/demo.go:3"
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
run || fail "a pinned citation in a decisions document did not pass"

echo "check-line-citations_test.sh: ok"
