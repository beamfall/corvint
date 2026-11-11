#!/bin/sh
set -eu

# Cases, on a fixture repository: a clause body plus its table row passes, a row whose clause
# body was deleted fails and is named, a prose range such as `PUB-V0-011..015`. does not count as a
# second definition of the first ID in it, a spec deleted from the worktree alone is still read,
# and an operational `git ls-files` failure refuses. The third case matters for the regex, because
# the unbalanced-backtick form failed the real corpus on exactly that line. The fourth pins the
# enumeration to the Git index: `make gate` must measure what a fresh clone contains, not what the
# dirty worktree happens to hold. V1-1062 adds the unquoted prose range and a real duplicate;
# RDG-V0-021 adds the outside-Requirements ratchet and its stale-baseline refusal; RDG-V0-022 the
# definition-shaped near miss; RDG-V0-023 the wrapped-line citation; RDG-V0-9980 the ids cited in Go
# files, a Makefile and AGENTS.md with their frozen baseline.

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

# RDG-V0-023 (V1-2403): a citation that starts a wrapped paragraph or list-item line is no
# definition; a bulleted definition right after a paragraph line still is, so it still duplicates.
# shellcheck disable=SC2016
printf '# Alpha\n\n- `ALPHA-V0-001`: the clause.\n\nThis paragraph cites\n`ALPHA-V0-001`. It wraps.\n\n- an item that cites\n  `ALPHA-V0-001`: wrapped.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
run > /dev/null || fail "a citation starting a wrapped line must not count as a definition"

# shellcheck disable=SC2016
printf '# Alpha\n\n- `ALPHA-V0-001`: the clause.\n\nA paragraph line.\n- `ALPHA-V0-001`: a real second definition.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a bulleted duplicate after a paragraph line should fail"; fi
case $out in
    *"duplicate definition: ALPHA-V0-001 at docs/specs/alpha.md:3 and docs/specs/alpha.md:6"*) ;;
    *) fail "the bulleted duplicate should be named, got: $out" ;;
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
# A four-backtick block holding a three-backtick line stays open until a run of at least four
# backticks closes it, as OCM's fence reader does: the `## Amendment` and the definition inside it
# are code, not a heading that ends the section, and a later tilde run does not close it either.
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n````markdown\n```\n## Amendment\n~~~~\n- `ALPHA-V0-009`: example.\n````\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
out=$(run) || fail "a three-backtick line inside a four-backtick fence ended the fence, got: $out"
# The same document with a three-backtick fence closes at the inner ``` line, so the heading after
# it is real and both later definitions sit outside the section.
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n```markdown\n```\n## Amendment\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a heading after a closed three-backtick fence should end the section"; fi
case $out in
    *"docs/specs/alpha.md ALPHA-V0-001"*) ;;
    *) fail "the definitions after the closed fence should be named, got: $out" ;;
esac
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n' \
    > "$test_root/docs/specs/alpha.md"
stage

# RDG-V0-022 (V1-2168): `- `ID` (V1-x): text` is definition-shaped but no definition, so it fails
# naming the line and the accepted forms; the same shape for an id defined elsewhere passes.
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002` (V1-2168): misshapen.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a definition-shaped line with a note before its colon should fail"; fi
case $out in
    *"look like definitions"*'- `ID`: (note) text'*"docs/specs/alpha.md:7 ALPHA-V0-002"*) ;;
    *) fail "the misshapen definition should be named with the accepted forms, got: $out" ;;
esac
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n\nALPHA-V0-002 (V1-2168): a traceability line.\n' \
    > "$test_root/docs/specs/alpha.md"
stage
out=$(run) || fail "a traceability line for a defined id should pass, got: $out"
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n\n- `ALPHA-V0-002`: later.\n' \
    > "$test_root/docs/specs/alpha.md"
stage

rm "$test_root/docs/specs/alpha.md"
run > /dev/null || fail "a spec deleted from the worktree alone must still be read from the index"

# RDG-V0-741 (V1-3699): the table-row-only failure names the regeneration fix.
saved_tsv=$(git -C "$test_root" show :docs/specs/REQUIREMENTS.tsv)
saved_alpha=$(git -C "$test_root" show :docs/specs/alpha.md)
printf 'id\tfile\tline\ttitle\nALPHA-V0-001\tdocs/specs/alpha.md\t5\tthe clause\nALPHA-V0-009\tdocs/specs/alpha.md\t7\tgone\n' \
    > "$test_root/docs/specs/REQUIREMENTS.tsv"
# shellcheck disable=SC2016
printf '# Alpha\n\n## Requirements\n\n- `ALPHA-V0-001`: the clause.\n' > "$test_root/docs/specs/alpha.md"
stage
if out=$(run); then fail "a stale REQUIREMENTS.tsv row should fail"; fi
case $out in
    *'ALPHA-V0-009'*'stage the spec, then run make spec-requirements to regenerate docs/specs/REQUIREMENTS.tsv'*) ;;
    *) fail "the table-row-only failure should name make spec-requirements, got: $out" ;;
esac
printf '%s\n' "$saved_tsv" > "$test_root/docs/specs/REQUIREMENTS.tsv"
printf '%s\n' "$saved_alpha" > "$test_root/docs/specs/alpha.md"
stage

# RDG-V0-9980 (V1-5324): an id cited in a Go file, a Makefile or AGENTS.md that no spec defines
# fails, naming the id, its first citing line and the three remedies, unless the frozen baseline
# or the reviewed allowlist lists it; a baseline or allowlist line that is now defined or no longer
# cited fails as stale, and an allowlist line with no reason fails. A definition in a retired spec
# counts as defined and is only counted. A cited id ends at its first three-digit group.
mkdir -p "$test_root/cmd/zeta"
printf 'package main\n\n// ZETA-V0-001: an undefined requirement.\nfunc main() {}\n' > "$test_root/cmd/zeta/main.go"
stage
if out=$(run); then fail "an undefined id cited in a Go file should fail"; fi
case $out in
    *"cited in Go code, a Makefile or AGENTS.md"*"define the requirement"*"fix the citation"*"script/requirement-citations-allowlist.tsv"*"ZETA-V0-001 cmd/zeta/main.go:3"*) ;;
    *) fail "the undefined Go citation should be named with the three remedies, got: $out" ;;
esac
# shellcheck disable=SC2016
printf 'package main\n\n// ALPHA-V0-001 and `ALPHA-V0-002` are defined; ZETA-V0-0015 has four digits.\nfunc main() {}\n' \
    > "$test_root/cmd/zeta/main.go"
stage
out=$(run) || fail "a Go citation of defined ids, and a four-digit id, should pass, got: $out"
# A range, a suffix and a slash pair read as the first id alone, so nothing is invented from them.
printf 'package main

// ALPHA-V0-001..003, ALPHA-V0-001-002, ALPHA-V0-001/002 and ALPHA-V0-001-foo.
func main() {}
' \
    > "$test_root/cmd/zeta/main.go"
stage
out=$(run) || fail "a range, suffix or slash pair after a defined id should read as that id alone, got: $out"

printf 'gate: ## ZETA-V0-002 builds the thing\n\ttrue\n' > "$test_root/Makefile"
stage
if out=$(run); then fail "an undefined id cited in the Makefile should fail"; fi
case $out in
    *"ZETA-V0-002 Makefile:1"*) ;;
    *) fail "the undefined Makefile citation should be named, got: $out" ;;
esac
rm "$test_root/Makefile"

printf '# Agents\n\nThe release rule is ZETA-V0-003.\n' > "$test_root/AGENTS.md"
stage
if out=$(run); then fail "an undefined id cited in AGENTS.md should fail"; fi
case $out in
    *"ZETA-V0-003 AGENTS.md:3"*) ;;
    *) fail "the undefined AGENTS.md citation should be named, got: $out" ;;
esac

printf '# Undefined requirement ids cited outside specs (RDG-V0-9980). Remove lines only.\nZETA-V0-003\n' \
    > "$test_root/script/requirement-citations-undefined.txt"
stage
out=$(run) || fail "a baselined undefined citation should pass, got: $out"

printf '# Agents\n\nThe release rule is ALPHA-V0-001.\n' > "$test_root/AGENTS.md"
stage
if out=$(run); then fail "a baseline line no longer cited should fail as stale"; fi
case $out in
    *"script/requirement-citations-undefined.txt"*"stale"*"ZETA-V0-003"*) ;;
    *) fail "the stale baseline line should be named, got: $out" ;;
esac
printf '# Undefined requirement ids cited outside specs (RDG-V0-9980). Remove lines only.\nALPHA-V0-001\n' \
    > "$test_root/script/requirement-citations-undefined.txt"
stage
if out=$(run); then fail "a baseline line whose id is now defined should fail as stale"; fi
case $out in
    *"stale"*"ALPHA-V0-001"*) ;;
    *) fail "the defined baseline line should be named, got: $out" ;;
esac
rm "$test_root/script/requirement-citations-undefined.txt"

# A definition in a retired spec counts as defined and is reported as a count only; a testdata
# citation is not read.
# shellcheck disable=SC2016
printf '# Old\n\nDelivery status: retired\n\n## Requirements\n\n- `OLD-V0-001`: removed behaviour.\n' \
    > "$test_root/docs/specs/old.md"
printf '# Agents\n\nStill cites OLD-V0-001.\n' > "$test_root/AGENTS.md"
mkdir -p "$test_root/cmd/zeta/testdata"
printf 'package fixture\n\n// ZETA-V0-009 is fixture text.\n' > "$test_root/cmd/zeta/testdata/fixture.go"
stage
out=$(run) || fail "an id defined in a retired spec should pass, and testdata should not be read, got: $out"
case $out in
    *"1 cited id(s) are defined only in a retired spec"*) ;;
    *) fail "the retired-only citation should be counted, got: $out" ;;
esac

# The reviewed allowlist: a listed token passes, a line with no reason fails, a line whose token is
# no longer cited or is now defined fails as stale, and a baseline line for a listed token is stale.
printf '# Agents\n\nThe digest is SHA-256.\n' > "$test_root/AGENTS.md"
stage
if out=$(run); then fail "an unlisted SHA-256 token should fail"; fi
printf '# reviewed\nSHA-256\thash algorithm name, not a requirement id\n' > "$test_root/script/requirement-citations-allowlist.tsv"
stage
out=$(run) || fail "an allowlisted token should pass, got: $out"
printf '# reviewed\nSHA-256\n' > "$test_root/script/requirement-citations-allowlist.tsv"
stage
if out=$(run); then fail "an allowlist line with no reason should fail"; fi
case $out in
    *"give no reason"*"script/requirement-citations-allowlist.tsv:2 SHA-256"*) ;;
    *) fail "the reasonless allowlist line should be named, got: $out" ;;
esac
printf '# reviewed\nSHA-256\t \n' > "$test_root/script/requirement-citations-allowlist.tsv"
stage
if out=$(run); then fail "an allowlist line with a blank reason should fail"; fi
printf '# reviewed\nSHA-256\thash algorithm name, not a requirement id\n' > "$test_root/script/requirement-citations-allowlist.tsv"
printf 'SHA-256\n' > "$test_root/script/requirement-citations-undefined.txt"
stage
if out=$(run); then fail "a baseline line for an allowlisted token should fail as stale"; fi
case $out in
    *"requirement-citations-undefined.txt lines are stale"*"SHA-256"*) ;;
    *) fail "the allowlisted baseline line should be named, got: $out" ;;
esac
rm "$test_root/script/requirement-citations-undefined.txt"
printf '# Agents\n\nNo token here.\n' > "$test_root/AGENTS.md"
stage
if out=$(run); then fail "an allowlist line no longer cited should fail as stale"; fi
case $out in
    *"requirement-citations-allowlist.tsv lines are stale"*"SHA-256"*) ;;
    *) fail "the stale allowlist line should be named, got: $out" ;;
esac
printf '# reviewed\nALPHA-V0-001\ta defined id is no token\n' > "$test_root/script/requirement-citations-allowlist.tsv"
printf '# Agents\n\nCites ALPHA-V0-001.\n' > "$test_root/AGENTS.md"
stage
if out=$(run); then fail "an allowlist line whose id is defined should fail as stale"; fi
case $out in
    *"requirement-citations-allowlist.tsv lines are stale"*"ALPHA-V0-001"*) ;;
    *) fail "the defined allowlist line should be named, got: $out" ;;
esac
rm -r "$test_root/docs/specs/old.md" "$test_root/AGENTS.md" "$test_root/cmd" "$test_root/script/requirement-citations-allowlist.tsv"
stage
run > /dev/null || fail "the fixture should pass again once the citations are gone"

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

# SRG-V0-966 (V1-3660): an ID restated under a Rollback heading, bulleted or not, is no
# definition for the generator or this check. The fixture is generated with the real generator
# and then checked, so both doc-gate steps see the same fixture: no out-of-section refusal
# (SRG-V0-891, RDG-V0-021), no duplicate, and the row keeps its Requirements definition.
rbk=$test_root/rbk
mkdir -p "$rbk/script" "$rbk/docs/specs"
cp "$source_root/script/check-requirement-definitions.sh" "$source_root/script/gen-spec-requirements.sh" "$rbk/script/"
git -C "$rbk" init -q
# shellcheck disable=SC2016
printf '# Rollback fixture\n\n## Requirements\n\n- `RBK-V0-001`: the definition.\n\n### Rollback\n\nRBK-V0-001: revert inside the section.\n\n## Rollback\n\n- `RBK-V0-001`: revert the commit that added it.\nRBK-V0-001: and drop its test.\n' > "$rbk/docs/specs/rbk.md"
git -C "$rbk" add -A
(cd "$rbk" && script/gen-spec-requirements.sh > docs/specs/REQUIREMENTS.tsv) 2> "$rbk/gen.err" ||
    fail "the generator refused a rollback restatement: $(cat "$rbk/gen.err")"
grep -q '^RBK-V0-001	docs/specs/rbk.md	the definition\.' "$rbk/docs/specs/REQUIREMENTS.tsv" ||
    fail "a rollback restatement displaced the definition: $(cat "$rbk/docs/specs/REQUIREMENTS.tsv")"
git -C "$rbk" add -A
rbk_out=$(cd "$rbk" && script/check-requirement-definitions.sh 2>&1) ||
    fail "the definition check refused a rollback restatement: $rbk_out"

printf 'ok\n'
