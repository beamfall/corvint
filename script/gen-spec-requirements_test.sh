#!/bin/sh
set -eu

# Covers SRG-V0-001 (cases 1-3), SRG-V0-005 (case 4), SRG-V0-002/003 (case 5), SRG-V0-009 (case 6),
# SRG-V0-012 and SRG-V0-043 (case 7), SRG-V0-010 (case 8), SRG-V0-015 (case 9), SRG-V0-018 (case 10)
# SRG-V0-042/044/046 (case 11), SRG-V0-547/548 (case 12), SRG-V0-980/950 (case 13), SRG-V0-951
# (case 14), SRG-V0-952/953 (case 15) and SRG-V0-954 (case 16).
# SRG-V0-001: the generated index must be the one a fresh clone reproduces, so the enumeration is
# the Git index and nothing the worktree alone says can change the output. The three cases are the drift directions reproduced in
# `docs/specs/spec-requirement-index-generator-v0.md`: an untracked spec, a tracked spec
# deleted from the worktree alone, and an unstaged edit to a tracked spec.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-requirements-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$test_root/script/"
git -C "$test_root" init -q -b main

stage() { git -C "$test_root" -c user.name=t -c user.email=t@example.invalid add -- "$1"; }

write_spec() {
  # shellcheck disable=SC2016
  printf '# %s\n\n- `%s`: %s\n' "$1" "$2" "$3" > "$test_root/docs/specs/$1.md"
}

run() { (cd "$test_root" && script/gen-spec-requirements.sh); }

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

write_spec alpha ALPHA-V0-001 'the tracked clause'
stage docs/specs/alpha.md
baseline=$(run)

case "$baseline" in
  *'ALPHA-V0-001	docs/specs/alpha.md	the tracked clause'*) ;;
  *) fail "baseline did not index the tracked spec: $baseline" ;;
esac

# 1. An untracked spec is not in a fresh clone, so it must contribute no row.
write_spec beta BETA-V0-001 'the untracked clause'
actual=$(run)
test "$actual" = "$baseline" || fail "an untracked spec changed the index: $actual"
rm "$test_root/docs/specs/beta.md"

# 2. A tracked spec deleted from the worktree alone is still in the index, so its rows stay.
rm "$test_root/docs/specs/alpha.md"
actual=$(run)
test "$actual" = "$baseline" || fail "a worktree-only deletion dropped tracked rows: $actual"
write_spec alpha ALPHA-V0-001 'the tracked clause'

# 3. An unstaged edit is not published yet; `git add` is what changes the index.
write_spec alpha ALPHA-V0-001 'the unstaged clause'
actual=$(run)
test "$actual" = "$baseline" || fail "an unstaged edit changed the index: $actual"
stage docs/specs/alpha.md
actual=$(run)
test "$actual" != "$baseline" || fail "a staged edit did not change the index"

# docs/specs/README.md is the index of specs, not a spec, and is excluded even when tracked.
write_spec README README-V0-001 'the excluded clause'
stage docs/specs/README.md
case "$(run)" in
  *README-V0-001*) fail "docs/specs/README.md was indexed" ;;
esac

# 4. SRG-V0-005 truncates titles to 80 characters, not 80 bytes: a multibyte title keeps all 80
# characters and never ends in a split UTF-8 sequence.
long=$(printf '\303\251%.0s' $(seq 1 90))
write_spec gamma GAMMA-V0-001 "$long"
stage docs/specs/gamma.md
want=$(printf '\303\251%.0s' $(seq 1 80))
actual=$(run | grep '^GAMMA-V0-001	' | cut -f3)
test "$actual" = "$want" || fail "a multibyte title was not truncated to 80 characters: $actual"

# 5. SRG-V0-002/003: token syntax is not a definition boundary. Plain definitions
# start paragraphs; marked definitions and table rows retain their existing ranks.
cat > "$test_root/docs/specs/boundary.md" <<'SPEC'
PLAIN-001. A plain definition at the start of the file.
The acceptance evidence restores the exact old
SHA-256. Negative checksum cases are fixtures.
The next wrapped sentence mentions
WRAP-001: another prose token, even with a colon.
  INDENT-001. An indented continuation is prose too.
- `MARK-001`: A marked definition immediately after prose.
* **BOLD-001.** A bold bullet definition.
`CODE-001`: A backtick definition.
**BOLD-002:** A bold definition.

PLAIN-002: A plain definition at a paragraph boundary.
| TABLE-001 | table fallback |
| MARK-001 | table must not displace its definition |
- SHORT-01: Wrong suffix length.
- LONG-0001: Wrong suffix length.
SPEC
stage docs/specs/boundary.md
actual=$(run)
for id in SHA-256 WRAP-001 INDENT-001 SHORT-01 LONG-0001; do
  if printf '%s\n' "$actual" | grep -q "^$id[[:space:]]"; then
    fail "prose or invalid token indexed: $id"
  fi
done
for id in PLAIN-001 PLAIN-002 MARK-001 BOLD-001 BOLD-002 CODE-001 TABLE-001; do
  printf '%s\n' "$actual" | grep -q "^$id[[:space:]]" || fail "definition lost: $id"
done
printf '%s\n' "$actual" | grep '^MARK-001' | grep -q 'A marked definition' || fail "table displaced definition"
test "$(run)" = "$actual" || fail "regeneration was not deterministic"


# 6. SRG-V0-009: the acceptance mode comes from the untruncated leading parenthetical of a rank 0
# definition. `standing` wins over every other word, `owner` needs an affirmative word and a
# decision number or date, a negated "not (yet) accepted" or "not an owner decision" does not
# count, and everything else, including a table row, is `unrecorded` with acceptedBy `-`.
late=$(printf 'x%.0s' $(seq 1 90))
cat > "$test_root/docs/specs/accept.md" <<SPEC
- ACC-001: (accepted under the owner's standing auto-accept (2026-10-11); V1-1347) A standing line.
- ACC-002: (accepted, decision 0478) An owner line.
- ACC-003: (V1-0309, owner decision 2026-10-09) An owner line by date.
- ACC-004: (proposed; not accepted by decision 0478) A negated line.
- ACC-005: (accepted, decision 0478 An unbalanced marker runs to the end of the line.
- ACC-006: No marker.
- ACC-007: (proposed) No acceptance word.
| ACC-008 | (accepted, decision 0478) a table row |
- ACC-009: (owner-approved standing exception, decision 0478; #12) Standing wins.
- ACC-010: ($late accepted, decision 0478) An anchor past the title cut.
- ACC-011: (not (yet) accepted, decision 0478) A negation across an inner parenthesis.
- ACC-012: (not an owner decision, 2026-10-09) A negated owner decision.
SPEC
stage docs/specs/accept.md
actual=$(run)
printf '%s\n' "$actual" | head -1 | grep -qx 'id	file	title	acceptance	acceptedBy' || fail "the header is not the committed layout"
expect_mode() {
  row=$(printf '%s\n' "$actual" | grep "^$1	") || fail "$1 not indexed"
  got=$(printf '%s\n' "$row" | cut -f4-5)
  test "$got" = "$2	$3" || fail "$1 acceptance is '$got', want '$2	$3'"
}
expect_mode ACC-001 standing '2026-10-11; V1-1347'
expect_mode ACC-002 owner 'decision 0478'
expect_mode ACC-003 owner 'V1-0309; 2026-10-09'
expect_mode ACC-004 unrecorded -
expect_mode ACC-005 owner 'decision 0478'
expect_mode ACC-006 unrecorded -
expect_mode ACC-007 unrecorded -
expect_mode ACC-008 unrecorded -
expect_mode ACC-009 standing 'decision 0478; #12'
expect_mode ACC-010 owner 'decision 0478'
expect_mode ACC-011 unrecorded -
expect_mode ACC-012 unrecorded -
test "$(printf '%s\n' "$actual" | grep '^ACC-010	' | cut -f3 | wc -m | tr -d ' ')" = 81 ||
  fail "ACC-010 title was not cut to 80 characters"
printf '%s\n' "$actual" | awk -F '\t' 'NF != 5 { exit 1 }' || fail "a row does not have five columns"

# 7. SRG-V0-012: --index and --readme project the header intent, delivery and digest claim onto
# the staged INDEX.json and README.md, keep every other field and cell byte for byte, append a
# spec that has no entry or row, and reach a fixed point once their output is staged.
project=$test_root/project
mkdir -p "$project/script" "$project/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$project/script/"
git -C "$project" init -q -b main
cat > "$project/docs/specs/one.md" <<'SPEC'
# One spec

Intent status: accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log)
Delivery status: implemented

## Agent digest
- Claim: One does the first thing.
- Status: accepted / implemented

- ONE-V0-001: (accepted under the owner's standing auto-accept (2026-10-11); V1-1347) The first clause.
SPEC
cat > "$project/docs/specs/two.md" <<'SPEC'
# Two spec

- **Intent status:** proposed
- **Delivery status:** not-started
- **Requirement prefix:** `TWO-V0`

## Agent digest
- Claim: Two does the second thing.

- TWO-V0-001: The second clause mentions a standing approval (standing approval) but no marker.
- TWO-V0-002: A trailing marker is not a leading one, so this clause
  stays unrecorded. (accepted under the owner's standing auto-accept (2026-10-11); V1-9001)
SPEC
cat > "$project/docs/specs/INDEX.json" <<'JSON'
[
  {
    "path": "docs/specs/one.md",
    "title": "Index-owned title",
    "reqPrefix": "ONE-V0",
    "intent": "proposed",
    "delivery": "not-started",
    "claim": "A stale claim.",
    "dependsOn": [
      "docs/specs/two.md"
    ],
    "supersededBy": null,
    "closableToday": true,
    "implementation": []
  }
]
JSON
cat > "$project/docs/specs/README.md" <<'MD'
# Specs

Prose stays.

| Scope | Contract | Intent | Delivery | Binding evidence or next gate |
| --- | --- | --- | --- | --- |
| One scope | [`one.md`](one.md) ONE-V0-001..009 | proposed | not-started | A stale claim. Tail stays. |
| One section | [`one.md`](one.md#requirements) | anchored | kept | Anchored rows stay. |
| Outside | [`notes`](../notes.md) | n/a | n/a | Non-spec rows stay. |

Trailing prose stays.
MD
git -C "$project" add -- docs/specs
gen() { (cd "$project" && script/gen-spec-requirements.sh "$@"); }
cat > "$test_root/want-index" <<'JSON'
[
  {
    "path": "docs/specs/one.md",
    "title": "Index-owned title",
    "reqPrefix": "ONE-V0",
    "intent": "accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log)",
    "delivery": "implemented",
    "claim": "One does the first thing.",
    "dependsOn": [
      "docs/specs/two.md"
    ],
    "supersededBy": null,
    "closableToday": true,
    "implementation": []
  },
  {
    "path": "docs/specs/two.md",
    "title": "Two spec",
    "reqPrefix": "TWO-V0",
    "intent": "proposed",
    "delivery": "not-started",
    "claim": "Two does the second thing.",
    "dependsOn": [],
    "supersededBy": null,
    "closableToday": false,
    "implementation": []
  }
]
JSON
cat > "$test_root/want-readme" <<'MD'
# Specs

Prose stays.

| Scope | Contract | Intent | Delivery | Binding evidence or next gate |
| --- | --- | --- | --- | --- |
| One scope | [`one.md`](one.md) ONE-V0-001 | accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log) | implemented | One does the first thing. Tail stays. |
| One section | [`one.md`](one.md#requirements) | anchored | kept | Anchored rows stay. |
| Outside | [`notes`](../notes.md) | n/a | n/a | Non-spec rows stay. |
| Two spec | [`two.md`](two.md) TWO-V0-001..002 | proposed | not-started | Two does the second thing. |

Trailing prose stays.
MD
gen --index > "$test_root/got-index" || fail "--index failed"
cmp "$test_root/want-index" "$test_root/got-index" || fail "--index did not project the headers"
gen --readme > "$test_root/got-readme" || fail "--readme failed"
cmp "$test_root/want-readme" "$test_root/got-readme" || fail "--readme did not project the headers"
cp "$test_root/got-index" "$project/docs/specs/INDEX.json"
cp "$test_root/got-readme" "$project/docs/specs/README.md"
git -C "$project" add -- docs/specs
gen --index | cmp - "$project/docs/specs/INDEX.json" || fail "--index is not a fixed point"
gen --readme | cmp - "$project/docs/specs/README.md" || fail "--readme is not a fixed point"
gen --lines | grep -q '^ONE-V0-001	docs/specs/one.md	10	.*	standing	2026-10-11; V1-1347$' ||
  fail "the projected spec lost its standing row"
# SRG-V0-043: the Contract-cell range follows the spec's ids, and a range whose prefix the spec
# never defines is refused rather than kept.
sed 's/(one\.md) ONE-V0-001 /(one.md) ONE-V0-001..003; extra /' "$project/docs/specs/README.md" > "$test_root/readme-range"
cp "$test_root/readme-range" "$project/docs/specs/README.md"
git -C "$project" add -- docs/specs/README.md
gen --readme | grep -qF '| [`one.md`](one.md) ONE-V0-001; extra |' || fail "--readme kept a stale range before a ';'"
sed 's/(one\.md) ONE-V0-001\.\.003; extra /(one.md) NONE-V0-001 /' "$test_root/readme-range" > "$project/docs/specs/README.md"
git -C "$project" add -- docs/specs/README.md
range_err=$(gen --readme 2>&1 > /dev/null) && fail "a range with an undefined prefix was accepted"
case $range_err in *"docs/specs/one.md: its README row shows a NONE-V0- range but the spec defines no NONE-V0- id"*) ;; *) fail "the range refusal printed: $range_err" ;; esac
cp "$test_root/got-readme" "$project/docs/specs/README.md"
git -C "$project" add -- docs/specs/README.md

# 8. SRG-V0-010: the audit lists standing requirements by spec, names a standing marker that is
# not leading as UNPARSED_STANDING (but not a clause that only mentions a standing approval), and
# reports a validated spec that still holds a standing requirement as NOT_PROMOTABLE, without failing.
cp "$source_root/script/spec-acceptance-audit.sh" "$project/script/"
sed -i.bak "s/^Delivery status: implemented\$/Delivery status: validated/" "$project/docs/specs/one.md"
git -C "$project" add -- docs/specs/one.md
gen > "$project/docs/specs/REQUIREMENTS.tsv"
gen --index > "$project/docs/specs/INDEX.json"
audit=$(cd "$project" && script/spec-acceptance-audit.sh) || fail "the audit failed"
want_audit=$(printf '%s\n' 'standing requirements by spec' 'docs/specs/one.md	1' \
  '  ONE-V0-001	docs/specs/one.md:10	2026-10-11; V1-1347' \
  'UNPARSED_STANDING	TWO-V0-002	docs/specs/two.md:11' \
  'NOT_PROMOTABLE	docs/specs/one.md	delivery validated	1 standing' \
  'total	1 standing in 1 specs	owner 0	standing 1	unrecorded 2	UNPARSED_STANDING 1')
test "$audit" = "$want_audit" || fail "the audit printed: $audit"
case $(cd "$project" && script/spec-acceptance-audit.sh --acceptance unrecorded) in
  *'  TWO-V0-001'*NOT_PROMOTABLE*) ;;
  *) fail "--acceptance unrecorded did not list TWO-V0-001 and keep NOT_PROMOTABLE" ;;
esac
status=0
(cd "$project" && script/spec-acceptance-audit.sh --acceptance accepted) > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "a mode outside the closed set exited $status, want 2"
printf 'id\ttitle\n' > "$project/docs/specs/REQUIREMENTS.tsv"
status=0
(cd "$project" && script/spec-acceptance-audit.sh) > /dev/null 2>&1 || status=$?
test "$status" = 1 || fail "a TSV without the acceptance columns exited $status, want 1"
printf 'id\tfile\ttitle\tacceptance\tacceptedBy\n\tdocs/specs/one.md\tt\tstanding\t-\n' \
  > "$project/docs/specs/REQUIREMENTS.tsv"
status=0
(cd "$project" && script/spec-acceptance-audit.sh) > /dev/null 2>&1 || status=$?
test "$status" = 1 || fail "a five-column row with an empty id exited $status, want 1"
# A stale row that names another file than the staged specs define the id in is refused, not
# joined to that other file's definition line (SRG-V0-017).
gen | sed 's|^ONE-V0-001	docs/specs/one.md	|ONE-V0-001	docs/specs/two.md	|' \
  > "$project/docs/specs/REQUIREMENTS.tsv"
grep -q '^ONE-V0-001	docs/specs/two.md	' "$project/docs/specs/REQUIREMENTS.tsv" || fail "the stale-file fixture was not written"
status=0
stale=$(cd "$project" && script/spec-acceptance-audit.sh 2>&1) || status=$?
test "$status" = 1 || fail "a row naming another file than its staged definition exited $status, want 1: $stale"
case $stale in
  *'ONE-V0-001 is defined in docs/specs/one.md, not docs/specs/two.md'*) ;;
  *) fail "the stale-file refusal printed: $stale" ;;
esac

# 9. SRG-V0-015: the committed index has no line column and orders its rows by file, then by
# definition position, so an edit that only shifts a spec's lines leaves it byte for byte; `--lines`
# is the derived locator view, ordered by id, with the line column (SRG-V0-017).
order() {
  printf '# Order\n\n%s- `ORD-V0-002`: Defined first.\n- `ORD-V0-001`: Defined second.\n' "$1" \
    > "$test_root/docs/specs/order.md"
  stage docs/specs/order.md
}
lines() { (cd "$test_root" && script/gen-spec-requirements.sh --lines); }
order ''
before=$(run)
test "$(printf '%s\n' "$before" | grep '^ORD-' | cut -f1 | tr '\n' ' ')" = 'ORD-V0-002 ORD-V0-001 ' ||
  fail "rows of one spec are not in definition order: $before"
printf '%s\n' "$before" | tail -n +2 | cut -f2 | LC_ALL=C sort -c || fail "rows are not ordered by file"
lines | head -1 | grep -qx 'id	file	line	title	acceptance	acceptedBy' || fail "--lines lacks the line column"
test "$(lines | grep '^ORD-' | cut -f1,3 | tr '\n' ' ')" = 'ORD-V0-001	4 ORD-V0-002	3 ' ||
  fail "--lines is not the id-ordered locator view: $(lines | grep '^ORD-')"
order 'Prose added above both definitions.

More prose.

'
test "$(run)" = "$before" || fail "a line shift rewrote the committed index"
test "$(lines | grep '^ORD-V0-002' | cut -f3)" = 7 || fail "--lines did not follow the shift"
lines | tail -n +2 | cut -f1 | LC_ALL=C sort -c || fail "--lines is not ordered by id"

# 10. SRG-V0-018: one pipelined batch read serves every staged spec; a path the batch cannot frame
# (a newline) or serve as a blob (a gitlink) falls back to the one-shot read, which still reports
# and fails as the per-file read did.
nl_path="docs/specs/$(printf 'new\nline').md"
printf '# NL\n\n- `NL-V0-001`: Read by the one-shot path.\n' > "$test_root/$nl_path"
stage "$nl_path"
run | grep -q '^NL-V0-001	' || fail "a spec path holding a newline was not read"
run | grep -q '^ORD-V0-002	docs/specs/order.md	Defined first\.' || fail "the batch lost a spec beside the fallback"
git -C "$test_root" rm -q --cached -- "$nl_path"
rm -f "$test_root/$nl_path"
git -C "$test_root" update-index --add --cacheinfo 160000,0123456789012345678901234567890123456789,docs/specs/gitlink.md
if run > /dev/null 2>&1; then fail "a non-blob spec entry was read"; fi
git -C "$test_root" rm -q --cached -- docs/specs/gitlink.md

# SRG-V0-018: `--all DIR` writes the three committed files from one pass, each byte for byte what
# its own mode prints, and refuses a missing or absent directory with exit 2.
all_dir="$test_root/all"
mkdir "$all_dir"
gen --all "$all_dir" || fail "--all failed"
gen | cmp - "$all_dir/REQUIREMENTS.tsv" || fail "--all REQUIREMENTS.tsv differs from the default mode"
gen --index | cmp - "$all_dir/INDEX.json" || fail "--all INDEX.json differs from --index"
gen --readme | cmp - "$all_dir/README.md" || fail "--all README.md differs from --readme"
# DCW-V0-352 (V1-2835): `--all` names on stderr each docs/specs edit it ignored because it is not
# staged, an untracked spec included, and names nothing once the specs are staged; its output and
# exit status are unchanged either way.
warned=$(gen --all "$all_dir" 2>&1 >/dev/null) || fail "--all with staged specs failed: $warned"
test -z "$warned" || fail "--all warned with nothing unstaged: $warned"
printf '\nAn unstaged line.\n' >> "$project/docs/specs/one.md"
printf '# Zeta\n' > "$project/docs/specs/zeta.md"
warn_dir="$test_root/warn"
mkdir "$warn_dir"
warned=$(gen --all "$warn_dir" 2>&1 >/dev/null) || fail "--all beside unstaged edits failed: $warned"
expected="script/gen-spec-requirements.sh: WARN docs/specs/one.md is not staged, so this run ignored it; git add it, then rerun make spec-requirements
script/gen-spec-requirements.sh: WARN docs/specs/zeta.md is not staged, so this run ignored it; git add it, then rerun make spec-requirements"
test "$warned" = "$expected" || fail "--all did not name the unstaged edits: $warned"
for file in REQUIREMENTS.tsv INDEX.json README.md; do
  cmp "$all_dir/$file" "$warn_dir/$file" || fail "--all $file changed beside unstaged edits"
done
warned=$( (cd "$project" && GIT_INDEX_FILE="$(git rev-parse --absolute-git-dir)/index" \
  script/gen-spec-requirements.sh --all "$warn_dir") 2>&1 >/dev/null) || fail "--all with GIT_INDEX_FILE failed: $warned"
test -z "$warned" || fail "--all warned for a private GIT_INDEX_FILE: $warned"
git -C "$project" checkout -q -- docs/specs/one.md
rm "$project/docs/specs/zeta.md"
if gen --all > /dev/null 2>&1; then fail "--all without DIR was accepted"; else test $? = 2 || fail "--all without DIR did not exit 2"; fi
if gen --all "$test_root/absent" > /dev/null 2>&1; then fail "--all with an absent DIR was accepted"; else test $? = 2 || fail "--all with an absent DIR did not exit 2"; fi

# A claim that is not one sentence cannot be cut out of a README description, so it is refused.
sed -i.bak 's/^- Claim: Two does the second thing\.$/- Claim: Two does. The second thing./' "$project/docs/specs/two.md"
git -C "$project" add -- docs/specs/two.md
if gen --index > /dev/null 2>&1; then fail "a two-sentence claim was accepted"; fi
refused_dir="$test_root/refused"
mkdir "$refused_dir"
if gen --all "$refused_dir" > /dev/null 2>&1; then fail "--all accepted a two-sentence claim"; fi
test -z "$(ls -A "$refused_dir")" || fail "a refused --all wrote into DIR"
# SRG-V0-040: a claim over 160 characters is refused at generation with its path and length.
long_claim="Two $(printf 'x%.0s' $(seq 1 160))."
sed -i.bak "s/^- Claim: Two does. The second thing\.$/- Claim: $long_claim/" "$project/docs/specs/two.md"
git -C "$project" add -- docs/specs/two.md
long_err=$(gen --all "$refused_dir" 2>&1 > /dev/null) && fail "a 165-character claim was accepted"
case $long_err in *"docs/specs/two.md:8: the Agent digest Claim is 165 characters; the limit is 160"*) ;; *) fail "the long-claim refusal did not name the path and length: $long_err" ;; esac
test -z "$(ls -A "$refused_dir")" || fail "a refused long claim wrote into DIR"
for mode in '' --lines --index --readme; do
  # shellcheck disable=SC2086
  long_err=$(gen $mode 2>&1 > /dev/null) && fail "mode '${mode:-tsv}' accepted a 165-character claim"
  case $long_err in *"docs/specs/two.md:8: the Agent digest Claim is 165 characters; the limit is 160"*) ;; *) fail "mode '${mode:-tsv}' did not name the long claim: $long_err" ;; esac
done
if gen --bogus > /dev/null 2>&1; then fail "an unknown mode was accepted"; else test $? = 2 || fail "an unknown mode did not exit 2"; fi

# 11. SRG-V0-042, SRG-V0-044 and SRG-V0-046: an acceptance note the SRG-V0-009 reader would misread
# (including a trailing `Status: (accepted ...)` note and a note still open at the end of the
# definition line) is refused with the line,
# the id, the reason and the accepted forms; the exact forms and an adjective "Accepted" pass.
cat > "$test_root/docs/specs/near.md" <<'SPEC'
- NEAR-001: (accepted under the owner's standing auto-accept (2026-10-11); V1-1347) Exact standing.
- NEAR-002: (accepted, decision 0478; V1-0309) Exact owner.
- NEAR-003: Accepted closed inputs MUST stay typed.
- NEAR-004: (accepted, owner lane auto-acceptance 2026-10-11; V1-1463) Near miss: no standing.
- NEAR-005: (accepted, V1-1547) Near miss: no anchor.
- NEAR-006: accepted, decision 0506. Near miss: no parentheses.
- NEAR-007: Near miss: a trailing note. Status: (accepted, decision 0481; GitHub #715).
- NEAR-008: (accepted by the owner, amending NEAR-002 (owner chat,
  2026-10-27; V1-2763)) Near miss: the dated note wraps past the definition line.
- NEAR-009: (accepted by the owner, amending NEAR-002 (owner chat, 2026-10-27; V1-2763)) Nested dated note.
SPEC
stage docs/specs/near.md
near_err=$(run 2>&1 > /dev/null) && fail "near-miss acceptance notes were accepted"
for want in \
  'docs/specs/near.md:4: NEAR-004: an auto-acceptance note must name the standing approval' \
  'docs/specs/near.md:5: NEAR-005: the acceptance note records no decision number or date' \
  'docs/specs/near.md:6: NEAR-006: the acceptance note is not parenthesized, so it is not read' \
  'docs/specs/near.md:7: NEAR-007: the acceptance note trails the definition as `Status: (...)`, so it is not read' \
  'docs/specs/near.md:8: NEAR-008: the acceptance note is still open at the end of the definition line, and only that line is read' \
  '`(accepted by the owner, owner chat YYYY-MM-DD; V1-NNNN)`' \
  'add one, for example `(accepted, decision NNNN; V1-NNNN)` or `(accepted YYYY-MM-DD; V1-NNNN)`' \
  '`(accepted by the owner, amending X-V0-001 (owner chat, YYYY-MM-DD); V1-NNNN)`' \
  "(accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN)"; do
  case $near_err in *"$want"*) ;; *) fail "the near-miss refusal lacks '$want': $near_err" ;; esac
done
case $near_err in *NEAR-00[1239]*) fail "an exact or adjective form was refused: $near_err" ;; esac
sed -i.bak '4,9d' "$test_root/docs/specs/near.md"
stage docs/specs/near.md
run > /dev/null || fail "the exact acceptance forms were refused"
# SRG-V0-046: a dated owner-chat note inside a longer clause on the id's line classifies as `owner`.
lines | grep -q '^NEAR-009	docs/specs/near.md	4	.*	owner	2026-10-27; V1-2763$' ||
  fail "the nested dated note did not classify as owner: $(lines | grep '^NEAR-009')"

# 12. SRG-V0-547 (V1-3049): an id defined twice, across two specs or within one, is refused in
# every mode, naming every definition, instead of keeping one and dropping the other. SRG-V0-548:
# a wrapped citation that starts a continuation line is no second definition and ranks after the
# real one.
cat > "$test_root/docs/specs/dupa.md" <<'SPEC'
- `DUP-V0-001`: the first lane's text.
- `DUP-V0-002`: defined once in this spec.
- `DUP-V0-002`: and again in the same spec.
SPEC
cat > "$test_root/docs/specs/dupb.md" <<'SPEC'
- `DUP-V0-001`: the second lane's different text.
SPEC
stage docs/specs/dupa.md
stage docs/specs/dupb.md
for mode in '' --lines --index --readme; do
  # shellcheck disable=SC2086
  dup_err=$( (cd "$test_root" && script/gen-spec-requirements.sh $mode) 2>&1 > /dev/null) &&
    fail "mode '${mode:-tsv}' accepted a duplicate definition"
  for want in \
    'DUP-V0-001: defined more than once: docs/specs/dupa.md:1 docs/specs/dupb.md:1' \
    'DUP-V0-002: defined more than once: docs/specs/dupa.md:2 docs/specs/dupa.md:3' \
    'a requirement ID must have exactly one definition line across docs/specs'; do
    case $dup_err in *"$want"*) ;; *) fail "mode '${mode:-tsv}' duplicate refusal lacks '$want': $dup_err" ;; esac
  done
done
dup_dir="$test_root/dup-out"
mkdir "$dup_dir"
if (cd "$test_root" && script/gen-spec-requirements.sh --all "$dup_dir") > /dev/null 2>&1; then
  fail "--all accepted a duplicate definition"
fi
test -z "$(ls -A "$dup_dir")" || fail "a refused duplicate wrote into DIR"
git -C "$test_root" rm -q --cached -- docs/specs/dupb.md
cat > "$test_root/docs/specs/dupa.md" <<'SPEC'
- `DUP-V0-002`: amended by
  `DUP-V0-001`. A wrapped citation, not a definition.
- `DUP-V0-001`: the only definition.
SPEC
stage docs/specs/dupa.md
wrapped=$(run) || fail "a wrapped citation was refused as a duplicate definition"
printf '%s\n' "$wrapped" | grep -q '^DUP-V0-001	docs/specs/dupa.md	the only definition\.' ||
  fail "a wrapped citation displaced the definition: $(printf '%s\n' "$wrapped" | grep '^DUP-V0-001')"
git -C "$test_root" rm -q --cached -- docs/specs/dupa.md

# 13. SRG-V0-980 (V1-3458): the header Intent status and digest Status are frozen. A requirement ID
# the frozen row does not list, in either field, or a header grown past 160 characters and its
# frozen length, is refused with the repair; a frozen header, a shortened one and a short ID-free
# header with no frozen row pass.
long=$(printf 'accepted (decision 0001); FRZ-V0-001 accepted (decision 0002) %0100d' 0)
write_frozen() {
  # shellcheck disable=SC2016
  printf '# Frozen\n\nIntent status: %s\nDelivery status: experimental\n\n## Agent digest\n- Claim: The frozen fixture keeps its header.\n- Status: %s / experimental\n\n## Requirements\n\n- `FRZ-V0-001`: (accepted, decision 0002) The first clause.\n- `FRZ-V0-002`: (accepted under the owner'"'"'s standing auto-accept (2026-10-31); V1-3458) The second clause.\n' \
    "$1" "${2:-$1}" > "$test_root/docs/specs/frozen.md"
  stage docs/specs/frozen.md
}
printf 'path\tchars\tids\ndocs/specs/frozen.md\t%s\tFRZ-V0-001\n' "${#long}" > "$test_root/docs/specs/INTENT-STATUS-FROZEN.tsv"
stage docs/specs/INTENT-STATUS-FROZEN.tsv
write_frozen "$long"
run > /dev/null || fail "a frozen header was refused"
write_frozen 'accepted'
run > /dev/null || fail "a shortened header was refused"
write_frozen "$long; FRZ-V0-002 accepted (2026-10-31)" "$long"
frz_err=$(run 2>&1 > /dev/null) && fail "a new ID in the Intent status was accepted"
for want in \
  'docs/specs/frozen.md:3: its Intent status names FRZ-V0-002, which its frozen header text does not; this line is frozen (docs/specs/INTENT-STATUS-FROZEN.tsv), so FRZ-V0-002 belongs on its own definition line inside docs/specs/frozen.md'"'"'s ## Requirements section (lines 10..13)' \
  "docs/specs/frozen.md:3: its Intent status is $((${#long} + 34)) characters; the limit is its frozen length, ${#long}" \
  "a new requirement ID or range never goes there: it belongs on each requirement's own definition line" \
  "in the spec's acceptance and traceability rows, and in its build-log entry" \
  'acceptance and acceptedBy columns' '(SRG-V0-009, SRG-V0-981, SRG-V0-950)' '(SRG-V0-980)'; do
  case $frz_err in *"$want"*) ;; *) fail "the frozen-header refusal lacks '$want': $frz_err" ;; esac
done
case $frz_err in *FRZ-V0-001*) fail "a frozen ID was refused: $frz_err" ;; esac
write_frozen 'accepted' 'accepted; FRZ-V0-002 accepted'
frz_err=$(run 2>&1 > /dev/null) && fail "a new ID in the digest Status was accepted"
case $frz_err in
  *'docs/specs/frozen.md:8: its Agent digest Status names FRZ-V0-002, which its frozen header text does not; this line is frozen'*'(lines 10..13)'*) ;;
  *) fail "the digest refusal lacks the field and ID: $frz_err" ;;
esac
# A range or slash after a frozen ID names the IDs it reaches.
for segment in 'FRZ-V0-001..002' 'FRZ-V0-001/002'; do
  write_frozen "accepted; $segment accepted"
  frz_err=$(run 2>&1 > /dev/null) && fail "the segment $segment was accepted"
  case $frz_err in
    *'its Intent status names FRZ-V0-002'*) ;;
    *) fail "the segment $segment was not read as naming FRZ-V0-002: $frz_err" ;;
  esac
done
frz_dir="$test_root/frz-out"
mkdir "$frz_dir"
# V1-3471: the refusal must be the frozen-header one, not an unrelated fixture failure.
if frz_err=$( (cd "$test_root" && script/gen-spec-requirements.sh --all "$frz_dir") 2>&1 > /dev/null); then
  fail "--all accepted a new header segment"
fi
case $frz_err in
  *'docs/specs/frozen.md:3: its Intent status names FRZ-V0-002'*'(SRG-V0-980)'*) ;;
  *) fail "--all did not refuse with the frozen-header diagnostic: $frz_err" ;;
esac
test -z "$(ls -A "$frz_dir")" || fail "a refused header wrote into DIR"
git -C "$test_root" rm -q --cached -- docs/specs/INTENT-STATUS-FROZEN.tsv
write_frozen 'proposed'
run > /dev/null || fail "a short ID-free header without a frozen row was refused"
write_frozen "accepted $(printf '%0170d' 0)"
frz_err=$(run 2>&1 > /dev/null) && fail "a header over 160 characters without a frozen row was accepted"
case $frz_err in
  *'docs/specs/frozen.md:3: its Intent status is 179 characters; the limit is 160'*) ;;
  *) fail "the unfrozen length refusal lacks the lengths: $frz_err" ;;
esac
git -C "$test_root" rm -q --cached -- docs/specs/frozen.md

# 14. SRG-V0-891 (V1-3295): an OCM-form definition after the spec's one ## Requirements section is
# refused in every mode with its line and the section range, unless the frozen RDG-V0-021 baseline
# lists it; a fenced example and a spec with no Requirements section pass.
# shellcheck disable=SC2016
printf '# Placed\n\n## Requirements\n\n- `PLC-V0-001`: (proposed) Inside.\n\n## Later\n\n```\n- `PLC-V0-003`: (proposed) A fenced example.\n```\n\n- `PLC-V0-002`: (proposed) Outside.\n' > "$test_root/docs/specs/placed.md"
stage docs/specs/placed.md
plc_err=$(run 2>&1 > /dev/null) && fail "a definition outside ## Requirements was accepted"
case $plc_err in
  *'docs/specs/placed.md:13: PLC-V0-002 is defined outside the ## Requirements section (lines 3..6)'*'(SRG-V0-891, RDG-V0-021)'*) ;;
  *) fail "the misplaced-definition refusal lacks the line and range: $plc_err" ;;
esac
case $plc_err in *PLC-V0-001* | *PLC-V0-003*) fail "an inside or fenced definition was refused: $plc_err" ;; esac
plc_dir="$test_root/plc-out"
mkdir "$plc_dir"
(cd "$test_root" && script/gen-spec-requirements.sh --all "$plc_dir") > /dev/null 2>&1 && fail "--all accepted a misplaced definition"
mkdir -p "$test_root/script"
printf '# baseline\ndocs/specs/placed.md PLC-V0-002\n' > "$test_root/script/requirement-definitions-outside-section.txt"
stage script/requirement-definitions-outside-section.txt
run > /dev/null || fail "a baselined outside definition was refused"
git -C "$test_root" rm -q --cached -- script/requirement-definitions-outside-section.txt docs/specs/placed.md
# shellcheck disable=SC2016
printf '# Unsectioned\n\n- `PLC-V0-004`: (proposed) No Requirements section here.\n' > "$test_root/docs/specs/placed.md"
stage docs/specs/placed.md
run > /dev/null || fail "a spec without a Requirements section was refused"
git -C "$test_root" rm -q --cached -- docs/specs/placed.md

# 15. SRG-V0-951 (V1-3602): one run reports every refusal, not only the first: two malformed
# acceptance notes, a duplicate definition and an over-long claim, in every mode, with each
# category's repair once; --all writes nothing.
long_claim=$(printf 'The claim %0160d.' 0)
cat > "$test_root/docs/specs/multi.md" <<SPEC
# Multi

Intent status: proposed
Delivery status: experimental

## Agent digest
- Claim: $long_claim
- Status: proposed / experimental

- \`MUL-V0-001\`: accepted, decision 0001. The first malformed note.
- \`MUL-V0-002\`: approved by the owner. The second malformed note.
- \`MUL-V0-003\`: defined once.
- \`MUL-V0-003\`: and again.
SPEC
stage docs/specs/multi.md
for mode in '' --lines --index --readme; do
  # shellcheck disable=SC2086
  multi_err=$( (cd "$test_root" && script/gen-spec-requirements.sh $mode) 2>&1 > /dev/null) &&
    fail "mode '${mode:-tsv}' accepted the malformed spec"
  for want in \
    'docs/specs/multi.md:10: MUL-V0-001: the acceptance note is not parenthesized' \
    'docs/specs/multi.md:11: MUL-V0-002: the acceptance note is not parenthesized' \
    'MUL-V0-003: defined more than once: docs/specs/multi.md:12 docs/specs/multi.md:13' \
    'docs/specs/multi.md:7: the Agent digest Claim is 171 characters; the limit is 160' \
    '(SRG-V0-547)' '(SRG-V0-042)'; do
    case $multi_err in *"$want"*) ;; *) fail "mode '${mode:-tsv}' did not report '$want' in one run: $multi_err" ;; esac
  done
  test "$(printf '%s\n' "$multi_err" | grep -c 'SRG-V0-042')" = 1 || fail "the note repair was printed more than once: $multi_err"
done
multi_dir="$test_root/multi-out"
mkdir "$multi_dir"
if (cd "$test_root" && script/gen-spec-requirements.sh --all "$multi_dir") > /dev/null 2>&1; then
  fail "--all accepted the malformed spec"
fi
test -z "$(ls -A "$multi_dir")" || fail "a refused run wrote into DIR"
git -C "$test_root" rm -q --cached -- docs/specs/multi.md

# 16. SRG-V0-952 (V1-3563): an indented note paragraph that opens with its item's backticked ID
# continues the item and is no second definition; a real second definition is still refused.
# SRG-V0-953 (V1-3553): an acceptance note that starts on the continuation line is refused naming
# the line, the ID and the repair.
cat > "$test_root/docs/specs/para.md" <<'SPEC'
- `PAR-V0-001`: (accepted, decision 0001) The requirement.

  `PAR-V0-001`: acceptance note: the fixture test covers it.

  `PAR-V0-001`. A second note paragraph.
- `PAR-V0-002`: (accepted, decision 0001) The next requirement.
SPEC
stage docs/specs/para.md
para=$(run) || fail "an indented note paragraph opening with the ID was refused"
printf '%s\n' "$para" | grep -q '^PAR-V0-001	docs/specs/para.md	(accepted, decision 0001) The requirement\.	owner	decision 0001$' ||
  fail "the note paragraph displaced the definition: $(printf '%s\n' "$para" | grep '^PAR-V0-001')"
printf '\n`PAR-V0-001`: an unindented second definition.\n' >> "$test_root/docs/specs/para.md"
stage docs/specs/para.md
para_err=$(run 2>&1 > /dev/null) && fail "an unindented second definition was accepted"
case $para_err in
  *'PAR-V0-001: defined more than once: docs/specs/para.md:1 docs/specs/para.md:8'*) ;;
  *) fail "the unindented duplicate was not refused: $para_err" ;;
esac
cat > "$test_root/docs/specs/para.md" <<'SPEC'
- `PAR-V0-001`: The requirement wraps, and its note
  (accepted under the owner's standing auto-accept (2026-11-01); V1-3553) sits on line two.
- `PAR-V0-002`: (accepted, decision 0001) A note on the id's line
  (with a parenthetical continuation) passes.
- `PAR-V0-003`: A plain wrap
  (as an aside) passes.
- `PAR-V0-004`: A wrap whose note nests its anchor
  (by the owner (owner chat, 2026-10-27), accepted; V1-3553) before the word.
- `PAR-V0-005`: A wrap whose aside nests
  (see (the table) below) and names no acceptance.
SPEC
stage docs/specs/para.md
for mode in '' --lines --index; do
  # shellcheck disable=SC2086
  para_err=$( (cd "$test_root" && script/gen-spec-requirements.sh $mode) 2>&1 > /dev/null) &&
    fail "mode '${mode:-tsv}' accepted a note on the continuation line"
  for want in \
    'docs/specs/para.md:2: PAR-V0-001: the acceptance note starts on the continuation line' \
    'docs/specs/para.md:8: PAR-V0-004: the acceptance note starts on the continuation line' \
    "move the note to the start of the text on the id's line" '(SRG-V0-953)'; do
    case $para_err in *"$want"*) ;; *) fail "mode '${mode:-tsv}' continuation refusal lacks '$want': $para_err" ;; esac
  done
  case $para_err in *PAR-V0-00[235]*) fail "a note on the id's line or a plain aside was refused: $para_err" ;; esac
done
git -C "$test_root" rm -q --cached -- docs/specs/para.md

# 17. SRG-V0-954 (V1-3617, V1-3510): --check compares a fresh generation with docs/specs and names
# each stale file; --write refuses an unstaged or untracked spec, naming each and the git add
# command, and writes nothing, then writes once they are staged.
w16="$test_root/w16"
mkdir -p "$w16/script" "$w16/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$w16/script/"
git -C "$w16" init -q -b main
spec16() {
  # shellcheck disable=SC2016
  printf '# %s\n\nIntent status: proposed\nDelivery status: experimental\n\n## Agent digest\n- Claim: The %s fixture.\n- Status: proposed / experimental\n\n- `%s`: %s\n' \
    "$1" "$1" "$2" "$3" > "$w16/docs/specs/$1.md"
}
spec16 alpha ALPHA-V0-001 'the staged clause'
printf '| Scope | Contract | Intent | Delivery | Description |\n|---|---|---|---|---|\n' > "$w16/docs/specs/README.md"
git -C "$w16" add -- docs/specs
(cd "$w16" && script/gen-spec-requirements.sh --write) || fail "--write could not seed the outputs"
git -C "$w16" add -- docs/specs
(cd "$w16" && script/gen-spec-requirements.sh --check) || fail "--check refused fresh outputs"
printf 'stale\n' >> "$w16/docs/specs/REQUIREMENTS.tsv"
check_err=$( (cd "$w16" && script/gen-spec-requirements.sh --check) 2>&1) && fail "--check accepted a stale index"
case $check_err in
  *'STALE docs/specs/REQUIREMENTS.tsv: stage the changed spec, then run make spec-requirements'*) ;;
  *) fail "--check did not name the stale file: $check_err" ;;
esac
git -C "$w16" checkout -q -- docs/specs/REQUIREMENTS.tsv
before=$(cat "$w16/docs/specs/REQUIREMENTS.tsv")
spec16 alpha ALPHA-V0-001 'an unstaged edit'
spec16 gamma GAMMA-V0-001 'an untracked spec'
write_err=$( (cd "$w16" && script/gen-spec-requirements.sh --write) 2>&1) && fail "--write accepted unstaged specs"
for want in 'REFUSE unstaged-spec docs/specs/alpha.md' 'REFUSE unstaged-spec docs/specs/gamma.md' \
  'git add -- docs/specs/alpha.md docs/specs/gamma.md; then rerun make spec-requirements (SRG-V0-954)'; do
  case $write_err in *"$want"*) ;; *) fail "the --write refusal lacks '$want': $write_err" ;; esac
done
test "$(cat "$w16/docs/specs/REQUIREMENTS.tsv")" = "$before" || fail "a refused --write changed REQUIREMENTS.tsv"
git -C "$w16" add -- docs/specs/alpha.md docs/specs/gamma.md
(cd "$w16" && script/gen-spec-requirements.sh --write) || fail "--write refused staged specs"
grep -q '^GAMMA-V0-001	' "$w16/docs/specs/REQUIREMENTS.tsv" || fail "--write did not regenerate the index"
(cd "$w16" && script/gen-spec-requirements.sh --check 2>&1 | grep -q STALE) &&
  fail "--check named a file stale right after --write"
usage_err=$( (cd "$w16" && script/gen-spec-requirements.sh --check extra) 2>&1) && fail "--check accepted an argument"
case $usage_err in *'--check compares them with a fresh generation (make spec-requirements-check)'*) ;; *) fail "the usage text lacks --check: $usage_err" ;; esac

# 18. SRG-V0-966 (V1-3660): an ID restated at the start of a line under a Rollback heading is no
# definition: no duplicate refusal and no title from it; the next heading ends the section.
# SRG-V0-965 and SRG-V0-967: the spec-requirements recipe runs the obligation-bound check and
# prints a no-changes or updated line.
cat > "$test_root/docs/specs/rbk.md" <<'SPEC'
## Requirements

- `RBK-V0-001`: the definition.

### Rollback

RBK-V0-001: revert the commit that added it.

### Notes

- `RBK-V0-002`: defined after the rollback section ends.
SPEC
stage docs/specs/rbk.md
rbk=$(run) || fail "a rollback-section line was refused as a second definition"
printf '%s\n' "$rbk" | grep -q '^RBK-V0-001	docs/specs/rbk.md	the definition\.' ||
  fail "a rollback line displaced the definition: $(printf '%s\n' "$rbk" | grep '^RBK-V0-001')"
printf '%s\n' "$rbk" | grep -q '^RBK-V0-002	docs/specs/rbk.md	defined after' ||
  fail "the rollback section did not end at the next heading: $(printf '%s\n' "$rbk" | grep '^RBK-V0-002')"
git -C "$test_root" rm -q --cached -- docs/specs/rbk.md
# The no-changes or updated line is printed by the recipe's --write step (SRG-V0-967).
recipe=$(sed -n '/^spec-requirements:/,/^$/p' "$(dirname "$0")/../Makefile"; cat "$(dirname "$0")/gen-spec-requirements.sh")
for want in 'spec-requirements: no changes' 'spec-requirements: updated' 'TestSpecsStayWithinTheOCMObligationBound' 'SRG-V0-965'; do
  case $recipe in *"$want"*) ;; *) fail "the spec-requirements recipe lacks '$want'" ;; esac
done

# 60. SRG-V0-760 (V1-4000): a `- `ID` (note):` line inside ## Requirements whose id has no
# definition is refused in every mode, naming the line, instead of the id being dropped; a restated
# defined id and a fenced line are not refused.
cat > "$test_root/docs/specs/kept.md" <<'SPEC'
## Requirements

- `KPT-V0-001` (KEEP): the noted line defines nothing.
- `KPT-V0-002`: defined.
- `KPT-V0-002` (V1-0001): restated after its definition.

```text
- `KPT-V0-003` (fenced): an example.
```
SPEC
stage docs/specs/kept.md
for mode in '' --lines --index --readme; do
  # shellcheck disable=SC2086
  kpt_err=$( (cd "$test_root" && script/gen-spec-requirements.sh $mode) 2>&1 > /dev/null) &&
    fail "mode '${mode:-tsv}' dropped a noted id silently"
  case $kpt_err in
    *'docs/specs/kept.md:3: KPT-V0-001: the note sits before the colon'*'(SRG-V0-760)'*) ;;
    *) fail "mode '${mode:-tsv}' noted-id refusal lacks the line or rule: $kpt_err" ;;
  esac
  case $kpt_err in *KPT-V0-002*|*KPT-V0-003*) fail "mode '${mode:-tsv}' refused a defined or fenced id: $kpt_err" ;; esac
done
git -C "$test_root" rm -q --cached -- docs/specs/kept.md

# 61. SRG-V0-761 (V1-4001) and SRG-V0-762 (V1-2310): --readme replaces only the projected claim (the
# current Claim or the INDEX.json one) and refuses a hand-edited first sentence naming the row; a
# Contract cell that is the bare link gains the spec's range, before a `;` too, and is then stable.
p61=$test_root/p61
mkdir -p "$p61/script" "$p61/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$p61/script/"
git -C "$p61" init -q -b main
for name in sa sb; do
  upper=$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]')
  printf '# %s\n\nIntent status: proposed\nDelivery status: experimental\n\n## Agent digest\n- Claim: %s does the new thing.\n- Status: proposed / experimental\n\n## Requirements\n\n- `%s-V0-001`: one.\n- `%s-V0-003`: three.\n' \
    "$name" "$upper" "$upper" "$upper" > "$p61/docs/specs/$name.md"
done
cat > "$p61/docs/specs/INDEX.json" <<'JSON'
[
  {
    "path": "docs/specs/sa.md",
    "title": "sa",
    "reqPrefix": "SA-V0",
    "intent": "proposed",
    "delivery": "experimental",
    "claim": "SA did the old thing.",
    "dependsOn": [],
    "supersededBy": null,
    "closableToday": false,
    "implementation": []
  },
  {
    "path": "docs/specs/sb.md",
    "title": "sb",
    "reqPrefix": "",
    "intent": "proposed",
    "delivery": "experimental",
    "claim": "SB does the new thing.",
    "dependsOn": [],
    "supersededBy": null,
    "closableToday": false,
    "implementation": []
  }
]
JSON
cat > "$p61/docs/specs/README.md" <<'MD'
| Scope | Contract | Intent | Delivery | Binding evidence or next gate |
| --- | --- | --- | --- | --- |
| A | [`sa.md`](sa.md) | proposed | experimental | SA did the old thing. Tail stays. |
| B | [`sb.md`](sb.md); companion notes | proposed | experimental | SB does the new thing. Tail e.g. stays. |
MD
git -C "$p61" add -- docs/specs
g61() { (cd "$p61" && script/gen-spec-requirements.sh "$@"); }
cat > "$test_root/want61" <<'MD'
| Scope | Contract | Intent | Delivery | Binding evidence or next gate |
| --- | --- | --- | --- | --- |
| A | [`sa.md`](sa.md) SA-V0-001..003 | proposed | experimental | SA does the new thing. Tail stays. |
| B | [`sb.md`](sb.md) SB-V0-001..003; companion notes | proposed | experimental | SB does the new thing. Tail e.g. stays. |
MD
g61 --readme > "$test_root/got61" || fail "--readme refused the projected rows"
cmp "$test_root/want61" "$test_root/got61" || fail "--readme did not project the claim and range: $(cat "$test_root/got61")"
cp "$test_root/got61" "$p61/docs/specs/README.md"
git -C "$p61" add -- docs/specs/README.md
g61 --readme | cmp - "$p61/docs/specs/README.md" || fail "the inserted range is not a fixed point"
sed 's/| SA does the new thing\. Tail stays\. |/| A hand-written sentence. Tail stays. |/' "$test_root/got61" > "$p61/docs/specs/README.md"
git -C "$p61" add -- docs/specs/README.md
hand_err=$(g61 --readme 2>&1 > /dev/null) && fail "--readme rewrote a hand-edited first sentence"
case $hand_err in
  *'docs/specs/README.md:3: the docs/specs/sa.md row'*'(SRG-V0-761)'*) ;;
  *) fail "the hand-edit refusal lacks the row or rule: $hand_err" ;;
esac


# 19. SRG-V0-968 (V1-3661): a spec whose Agent digest has no Claim is refused in every mode, naming
# it, so no live spec's README summary is hand-maintained; a superseded digest (Successor and
# Why) and a spec with no digest at all are unaffected.
# shellcheck disable=SC2016
printf '# Claimless\n\n## Agent digest\n- Status: proposed\n\n## Requirements\n\n- `CLM-V0-001`: a requirement.\n' > "$test_root/docs/specs/claimless.md"
stage docs/specs/claimless.md
for mode in '' --lines --index --readme; do
  # shellcheck disable=SC2086
  clm_err=$( (cd "$test_root" && script/gen-spec-requirements.sh $mode) 2>&1 > /dev/null) &&
    fail "mode '${mode:-tsv}' accepted a digest without a Claim"
  case $clm_err in
    *'docs/specs/claimless.md: its ## Agent digest has no `- Claim:` line'*'(SRG-V0-968)'*) ;;
    *) fail "mode '${mode:-tsv}' claimless refusal lacks the spec or rule: $clm_err" ;;
  esac
done
git -C "$test_root" rm -q --cached -- docs/specs/claimless.md
# shellcheck disable=SC2016
printf '# Retired

## Agent digest
- Successor: `docs/specs/x.md` replaces it.
- Why: retired.

## Requirements

- `CLM-V0-002`: a retired requirement.
' > "$test_root/docs/specs/retired.md"
stage docs/specs/retired.md
run > /dev/null || fail "a superseded digest (Successor and Why) was refused for having no Claim"
git -C "$test_root" rm -q --cached -- docs/specs/retired.md

printf 'ok\n'
