#!/usr/bin/env bash
# ASD-V0-750 (V1-3812): macOS ships Bash 3.2.57 as /bin/bash, and the Linux lane hosts run Bash 5,
# so a Bash 4 construct passes every Linux check and breaks the first Mac run. This doc gate scans
# every tracked shell script (a *.sh file or a file whose first line names bash) for the Bash 4+
# constructs below and fails naming file:line. Quoted text and comments are not code: the code
# view of a line drops both, and the expansion rules read a view that keeps double-quoted text.
# A line that must keep a construct, for example a fixture that feeds one to a refusal test, ends
# with `# bash32: allow`. A scan that cannot read every script refuses; it never prints ok.
# Not detected: an empty array expanded under `set -u`, which needs data flow; use the
# ${a[@]+"${a[@]}"} idiom (ASD-V0-750 non-goal).
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
BEGIN {
  SQ = sprintf("%c", 39)
  nr = split(ENVIRON["BASH32_RULES"], rl, "\n")
  for (k = 1; k <= nr; k++) { split(rl[k], part, "\t"); lab[k] = part[1]; vw[k] = part[2]; re[k] = part[3] }
  fails = 0; scanned = 0
  for (a = 1; a < ARGC; a++) {
    f = ARGV[a]; st = 0; term = ""; ln = 0
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
files=()
oldifs=$IFS
IFS=$'\n'
set -f
for f in $all; do
  [[ -f $f && ! -L $f ]] || continue
  case $f in
    # The rule table above names every construct, so the gate does not scan itself.
    script/check-bash32.sh) ;;
    *.sh) files+=("$f") ;;
    *) head -n 1 -- "$f" 2> /dev/null | grep -Eq '^#!.*([/ ]bash)([[:space:]]|$)' && files+=("$f") ;;
  esac
done
set +f
IFS=$oldifs

if ((${#files[@]} == 0)); then
  echo "check-bash32: ok, 0 scripts"
  exit 0
fi
refuse() {
  echo "check-bash32: FAIL incomplete scan: $1; refusing rather than reporting ok (ASD-V0-750)" >&2
  exit 2
}
out=$(BASH32_RULES=$rules awk "$prog" "${files[@]}") || refuse "the scanner exited $?"
summary=${out##*$'\n'}
case $summary in
  "#END "*) ;;
  *) refuse "the scanner printed no summary" ;;
esac
# shellcheck disable=SC2086
set -- $summary
fails=$2
if [[ $3 != "${#files[@]}" ]]; then
  refuse "scanned $3 of ${#files[@]} scripts"
fi
if ((fails)); then
  printf '%s\n' "${out%$'\n'*}"
  echo "check-bash32: $fails Bash 4+ construct(s) in ${#files[@]} scripts; rewrite them for Bash 3.2 (ASD-V0-750)" >&2
  exit 1
fi
echo "check-bash32: ok, ${#files[@]} scripts"
