#!/bin/sh
set -eu

# Covers SRG-V0-001 (cases 1-3), SRG-V0-005 (case 4), SRG-V0-002/003 (case 5), SRG-V0-009 (case 6),
# SRG-V0-012 (case 7) and SRG-V0-010 (case 8). SRG-V0-001: the generated index must be
# the one a fresh clone reproduces, so the enumeration is the Git index and nothing the worktree
# alone says can change the output. The three cases are the drift directions reproduced in
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
  *'ALPHA-V0-001	docs/specs/alpha.md	3	the tracked clause'*) ;;
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
actual=$(run | grep '^GAMMA-V0-001	' | cut -f4)
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
- ACC-007: (accepted) No anchor.
| ACC-008 | (accepted, decision 0478) a table row |
- ACC-009: (owner-approved standing exception, decision 0478; #12) Standing wins.
- ACC-010: ($late accepted, decision 0478) An anchor past the title cut.
- ACC-011: (not (yet) accepted, decision 0478) A negation across an inner parenthesis.
- ACC-012: (not an owner decision, 2026-10-09) A negated owner decision.
SPEC
stage docs/specs/accept.md
actual=$(run)
printf '%s\n' "$actual" | head -1 | grep -qx 'id	file	line	title	acceptance	acceptedBy' || fail "the header lacks the acceptance columns"
expect_mode() {
  row=$(printf '%s\n' "$actual" | grep "^$1	") || fail "$1 not indexed"
  got=$(printf '%s\n' "$row" | cut -f5-6)
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
test "$(printf '%s\n' "$actual" | grep '^ACC-010	' | cut -f4 | wc -m | tr -d ' ')" = 81 ||
  fail "ACC-010 title was not cut to 80 characters"
printf '%s\n' "$actual" | awk -F '\t' 'NF != 6 { exit 1 }' || fail "a row does not have six columns"

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
| One scope | [`one.md`](one.md) | proposed | not-started | A stale claim. Tail stays. |
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
| One scope | [`one.md`](one.md) | accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log) | implemented | One does the first thing. Tail stays. |
| One section | [`one.md`](one.md#requirements) | anchored | kept | Anchored rows stay. |
| Outside | [`notes`](../notes.md) | n/a | n/a | Non-spec rows stay. |
| Two spec | [`two.md`](two.md) | proposed | not-started | Two does the second thing. |

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
gen | grep -q '^ONE-V0-001	docs/specs/one.md	10	.*	standing	2026-10-11; V1-1347$' ||
  fail "the projected spec lost its standing row"

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
printf 'id\tfile\tline\ttitle\tacceptance\tacceptedBy\n\tdocs/specs/one.md\t10\tt\tstanding\t-\n' \
  > "$project/docs/specs/REQUIREMENTS.tsv"
status=0
(cd "$project" && script/spec-acceptance-audit.sh) > /dev/null 2>&1 || status=$?
test "$status" = 1 || fail "a six-column row with an empty id exited $status, want 1"

# A claim that is not one sentence cannot be cut out of a README description, so it is refused.
sed -i.bak 's/^- Claim: Two does the second thing\.$/- Claim: Two does. The second thing./' "$project/docs/specs/two.md"
git -C "$project" add -- docs/specs/two.md
if gen --index > /dev/null 2>&1; then fail "a two-sentence claim was accepted"; fi
if gen --bogus > /dev/null 2>&1; then fail "an unknown mode was accepted"; else test $? = 2 || fail "an unknown mode did not exit 2"; fi

printf 'ok\n'
