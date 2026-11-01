#!/usr/bin/env bash
# ASD-V0-750 (V1-3812): macOS ships Bash 3.2.57 as /bin/bash, and the Linux lane hosts run Bash 5,
# so a Bash 4 construct passes every Linux check and breaks the first Mac run. This doc gate scans
# every tracked shell script (a *.sh file or a file whose first line names bash) for the Bash 4+
# constructs below and fails naming file:line. Comment lines are skipped. A line that must keep a
# construct, for example a fixture that feeds one to a refusal test, ends with `# bash32: allow`.
# Not detected: an empty array expanded under `set -u`, which needs data flow; use the
# ${a[@]+"${a[@]}"} idiom (ASD-V0-750 non-goal).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# label<TAB>ERE, one Bash 4+ construct each. The gate itself must run under Bash 3.2.
W='(^|[^A-Za-z0-9_-])'
# An operator is inert inside quotes, so the operator rules match only before any quote or #.
U="^[^\"'#]*"
rules="associative-array	${W}(declare|local|typeset|readonly)[[:space:]]+-[A-Za-z]*A
nameref	${W}(declare|local|typeset)[[:space:]]+-[A-Za-z]*n[A-Za-z]*[[:space:]]
mapfile	${W}(mapfile|readarray)([[:space:]]|$)
case-modification	\\$\\{[A-Za-z_][A-Za-z0-9_]*(\\[[^]]*\\])?(\\^|,)
parameter-transformation	\\$\\{[A-Za-z_][A-Za-z0-9_]*(\\[[^]]*\\])?@[QEPAaUuLK]\\}
negative-array-index	\\$\\{[A-Za-z_][A-Za-z0-9_]*\\[-[0-9]
test-variable-set	\\[\\[[[:space:]]+(![[:space:]]+)?-v[[:space:]]
globstar	shopt[[:space:]]+-s[[:space:]]+([A-Za-z]+[[:space:]]+)*globstar
coproc	${W}coproc([[:space:]]|$)
pipe-stderr	${U}[|]&
append-both	${U}&>>
case-fallthrough	${U}(;;&|[^;];&([[:space:]]|$))
automatic-fd	${W}(exec|[0-9A-Za-z_]+)[[:space:]]+[{][A-Za-z_][A-Za-z0-9_]*[}][<>]
wait-next	${W}wait[[:space:]]+-n
brace-step	[{]-?[0-9]+[.][.]-?[0-9]+[.][.]-?[0-9]+[}]"

files=()
while IFS= read -r -d '' f; do
  [[ -f $f && ! -L $f ]] || continue
  case $f in
    # The rule table above names every construct, so the gate does not scan itself.
    script/check-bash32.sh) ;;
    *.sh) files+=("$f") ;;
    *) head -n 1 -- "$f" 2> /dev/null | grep -Eq '^#!.*([/ ]bash)([[:space:]]|$)' && files+=("$f") ;;
  esac
done < <(git ls-files -z)

fails=0
for f in "${files[@]+"${files[@]}"}"; do
  while IFS=$'\t' read -r label re; do
    # A comment line, or a line marked `# bash32: allow`, is not scanned.
    hits=$(grep -nE -- "$re" "$f" | grep -Ev '^[0-9]+:[[:space:]]*#' | grep -v '# bash32: allow$' || true)
    [[ -n $hits ]] || continue
    while IFS= read -r hit; do
      printf 'check-bash32: FAIL %s %s:%s: Bash 4+ construct; macOS /bin/bash is 3.2\n' "$label" "$f" "${hit%%:*}"
      fails=$((fails + 1))
    done <<< "$hits"
  done <<< "$rules"
done
if ((fails)); then
  echo "check-bash32: $fails Bash 4+ construct(s) in ${#files[@]} scripts; rewrite them for Bash 3.2 (ASD-V0-750)" >&2
  exit 1
fi
echo "check-bash32: ok, ${#files[@]} scripts"
