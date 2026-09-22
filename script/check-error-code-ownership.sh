#!/bin/sh
set -eu

# Error-code ownership ratchet (docs/specs/error-code-ownership-gate-v0.md, ECO-V0).
#
# An emitted code is a kebab-case string literal (`[a-z][a-z0-9]*(-[a-z0-9]+)+`) in tracked,
# non-test Go under cmd/ and internal/, in one of six positions (ECO-V0-001):
#   1. the value of a `Code:` or `Reason:` composite-literal key;
#   2. the value of a `"code":` or `"reason":` map-literal key;
#   3. the argument of `errors.New(...)`;
#   4. the argument, at that parameter's index, of a call to any function, method or assigned
#      closure with a parameter named `code` or `reason`, discovered from the same sources; a
#      parameter after the first is matched only in files of the declaring directory;
#   5. the operand of a conversion to a `string`-based type whose name ends in `Error` or `Code`,
#      discovered from the same sources;
#   6. the value, at the same index, of a single-line `=`/`:=` assignment whose target list
#      names `code` or `reason`, such as `class, reason = "X", "code"`.
# A code is owned when some tracked docs/specs/*.md names it as a whole token (ECO-V0-002).
# script/error-code-ownership.allowlist holds exactly the codes that are emitted and unowned;
# the gate fails when the two sets differ in either direction, so the list only shrinks
# (ECO-V0-003). Everything is read from the Git index, never the worktree (ECO-V0-004).
#
#   script/check-error-code-ownership.sh          check the ratchet
#   script/check-error-code-ownership.sh --list   print unowned codes with their first site

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

mode=check
case ${1:-} in
"") ;;
--list) mode=list ;;
*) printf 'usage: %s [--list]\n' "$0" >&2; exit 64 ;;
esac

allowlist=script/error-code-ownership.allowlist
kebab='[a-z][a-z0-9]*(-[a-z0-9]+)+'
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-error-code-ownership.XXXXXX")
trap 'rm -rf "$work"' EXIT
trap 'rm -rf "$work"; exit 143' HUP INT TERM

# git grep exits 1 for "no match"; any other non-zero status is an enumeration failure that
# plain sh (no pipefail) would otherwise swallow, so each search is captured and checked alone.
search() {
    out=$1; shift
    status=0
    git grep --cached -I "$@" >"$out" || status=$?
    if [ "$status" -gt 1 ]; then
        printf 'error-code ownership: git grep failed (exit %s)\n' "$status" >&2
        exit 1
    fi
}

sources="cmd/*.go internal/*.go"
ident='[A-Za-z_][A-Za-z0-9_]*'
# One call argument: no top-level comma, at most one level of nested parentheses.
arg='([^,()]|\([^()]*\))*'
set -f
# shellcheck disable=SC2086
search "$work/constructors" -o -E \
    "(func (\([^)]*\) )?$ident|$ident :?= func)\((.*[(, ])?(code|reason)[ ,)].*" \
    -- $sources ':(exclude)*_test.go'
# shellcheck disable=SC2086
search "$work/types" -h -o -E "^type $ident(Error|Code) string" -- $sources ':(exclude)*_test.go'
set +f
# Each discovered signature yields "index<TAB>name<TAB>directory" for every parameter named
# code or reason, counting top-level commas in the parameter list. A named string type is
# index 0 everywhere.
sed -E 's/^([^:]*)\/[^/:]*:/\1	/; s/	func (\([^)]*\) )?/	/; s/ :?= func\(/(/' "$work/constructors" |
    awk -F'\t' '{
        name = substr($2, 1, index($2, "(") - 1)
        rest = substr($2, index($2, "(") + 1)
        depth = 0; n = 0; field = ""
        for (i = 1; i <= length(rest); i++) {
            c = substr(rest, i, 1)
            if (c == "(" || c == "[" || c == "{") depth++
            if ((c == ")" || c == "]" || c == "}") && depth == 0) break
            if (c == ")" || c == "]" || c == "}") depth--
            if (c == "," && depth == 0) { fields[n++] = field; field = ""; continue }
            field = field c
        }
        fields[n++] = field
        for (j = 0; j < n; j++) {
            split(fields[j], words, " ")
            if (words[1] == "code" || words[1] == "reason") printf "%d\t%s\t%s\n", j, name, $1
        }
    }' | LC_ALL=C sort -u >"$work/names"

# A first-parameter name or named type is matched in every source; a later-parameter name
# only in its declaring directory, where an unrelated same-named helper cannot collide.
first=$( (awk -F'\t' '$1 == 0 { print $2 }' "$work/names"; sed -E 's/^type //; s/ string$//' "$work/types") |
    LC_ALL=C sort -u | paste -sd'|' -)
test -n "$first" || first='errors\.New'
assignments=
for index in 0 1 2 3; do
    assignments="$assignments|([A-Za-z_][A-Za-z0-9_.]*[[:space:]]*,[[:space:]]*){$index}(code|reason)([[:space:]]*,[[:space:]]*[A-Za-z_][A-Za-z0-9_.]*)*[[:space:]]*:?=[[:space:]]*($arg,[[:space:]]*){$index}"
done

position="(^|[^A-Za-z0-9_])((Code|Reason)[[:space:]]*:[[:space:]]*|\"(code|reason)\"[[:space:]]*:[[:space:]]*|errors\.New\(|($first)\($assignments)\"$kebab\""
set -f
# shellcheck disable=SC2086
search "$work/sites" -n -o -E "$position" -- $sources ':(exclude)*_test.go'
set +f
# One "directory<TAB>pattern" line per declaring directory, alternating its later-parameter
# names grouped by index.
ARG=$arg awk -F'\t' '$1 > 0 {
        key = $3 "\t" $1
        group[key] = (key in group) ? group[key] "|" $2 : $2
    }
    END {
        for (key in group) {
            split(key, part, "\t")
            call = "(" group[key] ")\\((" ENVIRON["ARG"] ",[[:space:]]*){" part[2] "}"
            calls[part[1]] = (part[1] in calls) ? calls[part[1]] "|" call : call
        }
        for (directory in calls) printf "%s\t%s\n", directory, calls[directory]
    }' "$work/names" >"$work/directories"
while IFS='	' read -r directory calls; do
    search "$work/local-sites" -n -o -E "(^|[^A-Za-z0-9_])($calls)\"$kebab\"" \
        -- ":(glob)$directory/*.go" ':(exclude)*_test.go'
    cat "$work/local-sites" >>"$work/sites"
done <"$work/directories"
sed -E "s/^([^:]*):([0-9]+):.*\"($kebab)\"\$/\3	\1:\2/" "$work/sites" |
    LC_ALL=C sort -t'	' -k1,1 -k2,2 |
    awk -F'\t' '$1 != previous { print; previous = $1 }' >"$work/emitted"

search "$work/spec-words" -h -o -E '[a-z0-9-]+' -- ':(glob)docs/specs/*.md'
LC_ALL=C sort -u "$work/spec-words" >"$work/owned"

cut -f1 "$work/emitted" | LC_ALL=C comm -23 - "$work/owned" >"$work/unowned-codes"
LC_ALL=C join -t'	' "$work/unowned-codes" "$work/emitted" >"$work/unowned"

if [ "$mode" = list ]; then
    cat "$work/unowned"
    exit 0
fi

if ! git cat-file blob ":$allowlist" >"$work/allowlist" 2>/dev/null; then
    printf 'error-code ownership: %s is not in the Git index\n' "$allowlist" >&2
    exit 1
fi
if ! LC_ALL=C sort -u -c "$work/allowlist" 2>/dev/null; then
    printf 'error-code ownership: %s must be sorted (LC_ALL=C) with no duplicates\n' "$allowlist" >&2
    exit 1
fi

status=0
LC_ALL=C comm -23 "$work/unowned-codes" "$work/allowlist" >"$work/new"
if [ -s "$work/new" ]; then
    printf 'error-code ownership: emitted codes named by no docs/specs/*.md (add each to its owning spec):\n' >&2
    LC_ALL=C join -t'	' "$work/new" "$work/emitted" | sed 's/^/  /' >&2
    status=1
fi
LC_ALL=C comm -13 "$work/unowned-codes" "$work/allowlist" >"$work/stale"
if [ -s "$work/stale" ]; then
    printf 'error-code ownership: allowlisted codes now owned or no longer emitted (remove from %s):\n' "$allowlist" >&2
    sed 's/^/  /' "$work/stale" >&2
    status=1
fi
exit "$status"
