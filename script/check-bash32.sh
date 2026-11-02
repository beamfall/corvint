#!/usr/bin/env bash
# ASD-V0-750 (V1-3812): macOS ships Bash 3.2.57 as /bin/bash, and the Linux lane hosts run Bash 5,
# so a Bash 4 construct passes every Linux check and breaks the first Mac run. This doc gate scans
# every tracked shell script (a *.sh file or a file whose first line names bash) for the Bash 4+
# constructs below and fails naming file:line. Quoted text and comments are not code: the code
# view of a line drops both, and the expansion rules read a view that keeps double-quoted text.
# A line that must keep a construct, for example a fixture that feeds one to a refusal test, ends
# with `# bash32: allow`. A scan that cannot read every script refuses; it never prints ok.
# ASD-V0-945 (V1-3632): under `set -u`, Bash before 4.4 aborts on "${a[@]}" or "${a[*]}" when
# the array is empty ("a[@]: unbound variable"). The empty-array-nounset rule flags such a plain
# expansion of an array the script initializes as NAME=() when the script sets -u and no
# ${#NAME[@]} guard precedes it; write ${a[@]+"${a[@]}"} or "${a[*]-}". It is a per-file
# heuristic, not data flow: an array filled before every use still needs the idiom or an allow.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# label<TAB>view<TAB>ERE, one Bash 4+ construct each. view `code` drops quoted text and comments;
# view `dq` keeps double-quoted text, where an expansion still runs. test-variable-set does not
# look inside a $(...) in the [[ ]], where -v is a command option. The gate runs under Bash 3.2.
W='(^|[^A-Za-z0-9_-])'
rules="associative-array	code	${W}(declare|local|typeset|readonly)[[:space:]]+-[A-Za-z]*A
nameref	code	${W}(declare|local|typeset)[[:space:]]+-[A-Za-z]*n[A-Za-z]*([[:space:]]|$)
mapfile	code	${W}(mapfile|readarray)([[:space:]]|$)
case-modification	dq	[$][{][A-Za-z_][A-Za-z0-9_]*([[][^]]*[]])?[,^]
parameter-transformation	dq	[$][{][A-Za-z_][A-Za-z0-9_]*([[][^]]*[]])?@[QEPAaUuLK][}]
negative-array-index	dq	[$][{][A-Za-z_][A-Za-z0-9_]*[[]-[0-9]
test-variable-set	code	[[][[]([^]$]|[]][^]]|[$][^(])*[[:space:]]-v([[:space:]]|$)
globstar	code	shopt[[:space:]]+-s[[:space:]]+([A-Za-z]+[[:space:]]+)*globstar
coproc	code	${W}coproc([[:space:]]|$)
pipe-stderr	code	[|]&
append-both	code	&>>
case-fallthrough	code	(;;&|[^;];&([[:space:]]|$))
automatic-fd	code	(^|[[:space:]])[{][A-Za-z_][A-Za-z0-9_]*[}][<>]
wait-next	code	${W}wait[[:space:]]+-n
brace-step	code	[{]-?[0-9]+[.][.]-?[0-9]+[.][.]-?[0-9]+[}]"

# PATH:NAME pairs exempt from empty-array-nounset, each provably non-empty where it expands and in
# a file this gate's owner cannot mark with an allow. script/heavy.sh:words: unwrap_shell expands
# words only after "$3" matched `go test` or `go vet`, so the parse holds at least two words.
nounset_exempt='script/heavy.sh:words'

# The scanner keeps quote state across lines, so a multi-line string stays text. A here-document
# body is scanned line by line with fresh quote state, so an apostrophe in its prose cannot hide
# the code after it. Every script is read with getline; an unreadable one exits 3.
# shellcheck disable=SC2016
prog='
function view(raw,   i, n, c, prev) {
  CODE = ""; DQ = ""; prev = ""; n = length(raw)
  for (i = 1; i <= n; i++) {
    c = substr(raw, i, 1)
    if (st == 1) { if (c == SQ) { st = 0; prev = c } continue }
    if (st == 3) { if (c == "\\") { i++; continue } if (c == SQ) { st = 0; prev = c } continue }
    if (st == 2) {
      if (c == "\\") { DQ = DQ c substr(raw, i + 1, 1); i++; continue }
      if (c == "\"") { st = 0; CODE = CODE "_"; prev = c; continue }
      DQ = DQ c; continue
    }
    if (c == "\\") { CODE = CODE "_"; DQ = DQ "_"; i++; prev = "_"; continue }
    if (c == SQ) { st = (prev == "$") ? 3 : 1; CODE = CODE "_"; DQ = DQ "_"; continue }
    if (c == "\"") { st = 2; continue }
    if (c == "#" && (prev == "" || prev ~ /[[:space:];|&(]/)) break
    CODE = CODE c; DQ = DQ c; prev = c
  }
}
# nounset applies empty-array-nounset (ASD-V0-945) to the current views: it records set -u,
# NAME=() initializers and ${#NAME[@]} guards, then reports a plain expansion of an empty-initialized,
# unguarded array. Expansions with an operator (${a[@]+...}, ${a[*]-}) are dropped first.
function nounset(raw,   s, m) {
  if (CODE ~ /(^|[;&|({[:space:]])set[[:space:]]+-[A-Za-z]*u/ || CODE ~ /(^|[;&|({[:space:]])set[[:space:]]+-o[[:space:]]+nounset/) setu = 1
  s = CODE
  while (match(s, /(^|[^A-Za-z0-9_])[A-Za-z_][A-Za-z0-9_]*=[(][)]/)) {
    m = substr(s, RSTART, RLENGTH); sub(/^[^A-Za-z_]/, "", m); sub(/=[(][)]$/, "", m); empty[m] = 1
    s = substr(s, RSTART + RLENGTH)
  }
  s = DQ
  while (match(s, /[$][{]#[A-Za-z_][A-Za-z0-9_]*[[][@*][]][}]/)) { guard[substr(s, RSTART + 3, RLENGTH - 7)] = 1; s = substr(s, RSTART + RLENGTH) }
  if (!setu || raw ~ /# bash32: allow$/) return
  s = DQ
  gsub(/[$][{][A-Za-z_][A-Za-z0-9_]*[[][@*][]][-+:=?][^}]*[}][}]?/, "_", s)
  while (match(s, /[$][{][A-Za-z_][A-Za-z0-9_]*[[][@*][]][}]/)) {
    m = substr(s, RSTART + 2, RLENGTH - 6)
    if ((m in empty) && !(m in guard) && !((f ":" m) in exempt)) {
      printf "check-bash32: FAIL empty-array-nounset %s:%d: %s is expanded under set -u and may be empty; Bash before 4.4 aborts; write ${%s[@]+\"${%s[@]}\"} or \"${%s[*]-}\"\n", f, ln, m, m, m, m
      fails++
    }
    s = substr(s, RSTART + RLENGTH)
  }
}
BEGIN {
  SQ = sprintf("%c", 39)
  nr = split(ENVIRON["BASH32_RULES"], rl, "\n")
  for (k = 1; k <= nr; k++) { split(rl[k], part, "\t"); lab[k] = part[1]; vw[k] = part[2]; re[k] = part[3] }
  ne = split(ENVIRON["BASH32_NOUNSET_EXEMPT"], ex, " ")
  for (k = 1; k <= ne; k++) exempt[ex[k]] = 1
  fails = 0; scanned = 0
  for (a = 1; a < ARGC; a++) {
    f = ARGV[a]; st = 0; term = ""; ln = 0; setu = 0; split("", empty); split("", guard)
    while ((r = (getline raw < f)) > 0) {
      ln++
      if (term != "") {
        t = raw; sub(/^\t+/, "", t)
        if (t == term) { term = ""; continue }
        st = 0; view(raw); st = 0
      } else {
        view(raw)
      }
      if (raw !~ /# bash32: allow$/) {
        for (k = 1; k <= nr; k++) {
          if (((vw[k] == "dq") ? DQ : CODE) ~ re[k]) {
            printf "check-bash32: FAIL %s %s:%d: Bash 4+ construct; macOS /bin/bash is 3.2\n", lab[k], f, ln
            fails++
          }
        }
      }
      if (term == "") nounset(raw)
      if (term == "" && st == 0 && CODE ~ /<</ && CODE !~ /<<</ && match(raw, /<<-?[[:space:]]*[^[:alnum:]_<()$;&|[:space:]]?[A-Za-z_][A-Za-z0-9_]*/)) {
        term = substr(raw, RSTART + 2, RLENGTH - 2); sub(/^[^A-Za-z_]*/, "", term)
      }
    }
    if (r < 0) { printf "check-bash32: FAIL cannot read %s; the scan is incomplete\n", f > "/dev/stderr"; exit 3 }
    close(f); scanned++
  }
  printf "#END %d %d\n", fails, scanned
  exit 0
}'

all=$(git -c core.quotePath=false ls-files)
cands=()
oldifs=$IFS
IFS=$'\n'
set -f
for f in $all; do
  [[ -f $f && ! -L $f ]] || continue
  # The rule table above names every construct, so the gate does not scan itself.
  [[ $f == script/check-bash32.sh ]] || cands+=("$f")
done
set +f
IFS=$oldifs
# ASD-V0-600 (V1-4100): one awk reads the first line of each candidate that is not a *.sh file,
# instead of a head|grep pair per tracked file (about 11000 of them, 40-85 s on a loaded lane
# host). A file it cannot read is not listed, as before; a failed finder refuses, never prints ok.
files=()
if ((${#cands[@]})); then
  # shellcheck disable=SC2016
  list=$(printf '%s\n' "${cands[@]}" | awk '
    /\.sh$/ { print; next }
    { f = $0; line = ""; if ((getline line < f) > 0 && line ~ /^#!.*([\/ ]bash)([[:space:]]|$)/) print f; close(f) }') || {
    echo "check-bash32: FAIL incomplete scan: the script finder exited $?; refusing rather than reporting ok (ASD-V0-750)" >&2
    exit 2
  }
  IFS=$'\n'
  set -f
  for f in $list; do files+=("$f"); done
  set +f
  IFS=$oldifs
fi

if ((${#files[@]} == 0)); then
  echo "check-bash32: ok, 0 scripts"
  exit 0
fi
# ASD-V0-601 (V1-4100): a scan whose inputs match the last passing scan in this worktree is not
# repeated. The key hashes the checker itself (rules and scanner), the awk and Bash identities and
# every listed script's path and content, so a changed, added or removed script, a changed checker
# or another awk rescans. Only a full ok scan writes the key, to a derived file in the git dir;
# CORVINT_GATE_LEDGER=off always rescans, and a key that cannot be computed rescans.
pass_file='' key=''
if [[ ${CORVINT_GATE_LEDGER:-} != off ]] && pass_file=$(git rev-parse --git-path bash32-check-pass 2> /dev/null); then
  key=$({
    printf 'bash32-check/1\nbash %s\n' "${BASH_VERSION-}"
    { awk --version 2>&1 < /dev/null || true; } | sed -n 1p
    git hash-object --no-filters -- script/check-bash32.sh
    printf '%s\n' "${files[@]}"
    printf '%s\n' "${files[@]}" | git hash-object --no-filters --stdin-paths
  } 2> /dev/null | git hash-object --stdin) || key=''
  if [[ -n $key && -f $pass_file && $(cat "$pass_file" 2> /dev/null) == "$key" ]]; then
    echo "check-bash32: ok, ${#files[@]} scripts (reused: the scripts, this checker, awk and bash match the last passing scan, key $key; CORVINT_GATE_LEDGER=off rescans)"
    exit 0
  fi
fi
refuse() {
  echo "check-bash32: FAIL incomplete scan: $1; refusing rather than reporting ok (ASD-V0-750)" >&2
  exit 2
}
out=$(BASH32_RULES=$rules BASH32_NOUNSET_EXEMPT=$nounset_exempt awk "$prog" "${files[@]}") || refuse "the scanner exited $?; fix: the awk error above names the cause, then rerun make bash32-check"
summary=${out##*$'\n'}
case $summary in
  "#END "*) ;;
  *) refuse "the scanner printed no summary; fix: rerun make bash32-check" ;;
esac
# shellcheck disable=SC2086
set -- $summary
fails=$2
if [[ $3 != "${#files[@]}" ]]; then
  refuse "scanned $3 of ${#files[@]} scripts; fix: make every listed script readable, then rerun make bash32-check"
fi
if ((fails)); then
  printf '%s\n' "${out%$'\n'*}"
  echo "check-bash32: $fails Bash 4+ construct(s) in ${#files[@]} scripts; rewrite them for Bash 3.2 (ASD-V0-750, ASD-V0-945)" >&2
  exit 1
fi
[[ -z $key || -z $pass_file ]] || printf '%s\n' "$key" > "$pass_file" 2> /dev/null || true
echo "check-bash32: ok, ${#files[@]} scripts"
