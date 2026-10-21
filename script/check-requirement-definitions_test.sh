#!/bin/sh
set -eu

# Cases, on a fixture repository: a clause body plus its table row passes, a row whose clause
# body was deleted fails and is named, a prose range such as `PUB-V0-011..015`. does not count as a
# second definition of the first ID in it, a spec deleted from the worktree alone is still read,
# and an operational `git ls-files` failure refuses. The third case matters for the regex, because
# the unbalanced-backtick form failed the real corpus on exactly that line. The fourth pins the
# enumeration to the Git index: `make gate` must measure what a fresh clone contains, not what the
# dirty worktree happens to hold. V1-1062 adds the unquoted prose range and a real duplicate;
# RDG-V0-021 adds the outside-Requirements ratchet and its stale-baseline refusal.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-requirement-definitions-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/specs"
cp "$source_root/script/check-requirement-definitions.sh" "$test_root/script/"
git -C "$test_root" init -q
git -C "$test_root" config user.email test@example.com
git -C "$test_root" config user.name test

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

stage() { git -C "$test_root" add -A; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-requirement-definitions.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

printf 'id\tfile\tline\ttitle\nALPHA-V0-001\tdocs/specs/alpha.md\t3\tthe clause\n' \
    > "$test_root/docs/specs/REQUIREMENTS.tsv"
# shellcheck disable=SC2016
printf '# Alpha\n\n- `ALPHA-V0-001`: the clause.\n' > "$test_root/docs/specs/alpha.md"
stage
run > /dev/null || fail "a defined requirement should pass"

printf '# Alpha\n\nno clause body here.\n' > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a row with no clause body should fail"; fi
case $out in
    *ALPHA-V0-001*) ;;
    *) fail "the undefined id should be named, got: $out" ;;
esac

# shellcheck disable=SC2016
printf '# Alpha\n\n- `ALPHA-V0-001`: the clause.\n\n`ALPHA-V0-001..003`. A prose range.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
run > /dev/null || fail "a prose range must not count as a second definition"

# V1-1062: a prose line that starts with an unquoted ID range is not a definition.
# shellcheck disable=SC2016
printf '# Alpha\n\n- `ALPHA-V0-001`: the clause.\n\nALPHA-V0-001..003 are additive prose.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
run > /dev/null || fail "a prose line starting with an ID range must not count as a definition"

# shellcheck disable=SC2016
printf '# Alpha\n\n- `ALPHA-V0-001`: the clause.\n\nALPHA-V0-001. A real second definition.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a real duplicate definition should fail"; fi
case $out in
    *"duplicate definition: ALPHA-V0-001"*) ;;
    *) fail "the duplicate should be named, got: $out" ;;
esac

# RDG-V0-021: an OCM-form definition after the single Requirements section fails unless the
# frozen baseline lists it, and a baseline row that no longer applies fails too.
printf 'id\tfile\tline\ttitle\nALPHA-V0-001\tdocs/specs/alpha.md\t5\tthe clause\nALPHA-V0-002\tdocs/specs/alpha.md\t9\tlater\n' \
    > "$test_root/docs/specs/REQUIREMENTS.tsv"
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n\n## Amendment\n\n- `ALPHA-V0-002`: later.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a definition outside ## Requirements should fail"; fi
case $out in
    *"outside the spec's ## Requirements section"*"docs/specs/alpha.md ALPHA-V0-002"*) ;;
    *) fail "the outside definition should be named, got: $out" ;;
esac
printf '# frozen\ndocs/specs/alpha.md ALPHA-V0-002\n' > "$test_root/script/requirement-definitions-outside-section.txt"
stage
run > /dev/null || fail "a baselined outside definition should pass"
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n\n## Amendment\n' \
    > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a stale baseline row should fail"; fi
case $out in
    *"delete from script/requirement-definitions-outside-section.txt"*"docs/specs/alpha.md ALPHA-V0-002"*) ;;
    *) fail "the stale baseline row should be named, got: $out" ;;
esac
rm "$test_root/script/requirement-definitions-outside-section.txt"
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n```text\n## Not a heading\n```\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
run > /dev/null || fail "a fenced level-two heading must not end the Requirements section"

rm "$test_root/docs/specs/alpha.md"
run > /dev/null || fail "a spec deleted from the worktree alone must still be read from the index"

# A Git operational failure is not evidence that the tracked TSV is absent.
real_git=$(command -v git)
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
status=0
out=$(cd "$test_root" && REAL_GIT="$real_git" PATH="$test_root/failing-git:$PATH" \
    script/check-requirement-definitions.sh 2>&1) || status=$?
test "$status" -ne 0 || fail "git ls-files failure should refuse"
case $out in
    *"requirement definitions: git ls-files failed"*) ;;
    *) fail "git ls-files failure should be diagnosed, got: $out" ;;
esac

printf 'ok\n'
