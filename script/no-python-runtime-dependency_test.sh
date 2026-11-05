#!/bin/sh
# GOC-V0-008 falsifying assertion: Corvint's own build, tests, hooks, packaging and default
# execution paths (Makefile, script/*.sh, integrations/**/*.mjs and integrations/**/*.json)
# must carry no literal Python-interpreter invocation. Three things are never flagged: the
# CORVINT_TEST_EXTERNAL_PYTEST opt-in, which lets an explicitly requested command run against a
# user's own Python project, the `corvint-analyzer-python` Go tool family (its name embeds
# "python" but it is Go source that analyzes Python as fixture data, not a Python interpreter
# dependency of Corvint's own build/tests/hooks/packaging), and this check's own name, which the
# Makefile's gate step must spell. In Makefile and script/*.sh, full-line # comments other than
# #! lines are prose and are skipped too (V1-3842); in .mjs a leading # begins a private field, so
# nothing there is skipped. One file is exempt by exact repository path: the named
# external-conformance script script/spec-toolchain-conformance.sh (STA-V0-011), which installs
# third-party Spec Kit into a throwaway venv to compare upstream's view with Corvint's. It is not
# Corvint's build/test/hook/packaging path, so the check also fails if any gate step or other
# scanned path reaches it (GOC-V0-011).
set -eu

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
pattern='\<python3?\>|\<pip\>|\<pytest\>'
opt_in='CORVINT_TEST_EXTERNAL_PYTEST'
names='analyzer-python|no-python-runtime-dependency'
external_conformance='script/spec-toolchain-conformance.sh'

# unexempted keeps each matched line that still names an interpreter after the opt-in lines are
# dropped and the two allowed names are masked, so an allowed name elsewhere on a line or in its
# path never hides a real invocation. Paths are repository-relative so the checkout path cannot
# match or exempt anything.
# A full-line # comment other than a #! interpreter line is prose, never an invocation (V1-3842),
# but only in the Makefile or a .sh file (or a pathless self-test line): in JavaScript a leading #
# starts a private field, which can run code. The opt-in drop is textual: any line naming the token
# is dropped, whatever else it runs.
unexempted() {
  grep -vE '^(([^:]*/)?Makefile:|[^:]*\.sh:)?[0-9]+:[[:space:]]*#([^!]|$)' | grep -v "$opt_in" | sed -E "s/$names/[allowed-name]/g" | grep -E "$pattern"
}

# mask_fixture_data blanks the quoted arguments of a printf or echo command (V1-3971): text a shell
# test writes into a fixture is data, not an invocation. It reads the shell words of each line: a
# printf or echo in command position (line start, after | ; & ( : or a reserved word such as then)
# has its single-quoted arguments and its double-quoted ones without $( or a backquote replaced by
# empty quotes, up to the next unquoted | ; & or ). An unquoted name, a later command on the line and
# a command substitution inside double quotes are left as they are, so a real call still matches.
# A single-quoted word whose text parses as a JSON array or object, or as a comma-separated run of
# them after an optional leading comma (array elements spliced after a quoted prefix), is a JSON
# literal, such as a language list, and is blanked the same way (GOC-V0-081). It stays scanned when
# its command names a shell, eval or a -c flag anywhere (the text may run even when it parses as
# JSON), and when it is a bare word in command position, such as a continuation line whose earlier
# words grep did not show.
mask_fixture_data() {
  awk '
    function jws() { while (jp <= jn && substr(js, jp, 1) ~ /[ \t]/) jp++ }
    function jstr(  c) {
      if (substr(js, jp, 1) != "\"") return 0
      for (jp++; jp <= jn; jp++) {
        c = substr(js, jp, 1)
        if (c == "\\") { jp++; continue }
        if (c == "\"") { jp++; return 1 }
      }
      return 0
    }
    function jval(  c) {
      jws(); c = substr(js, jp, 1)
      if (c == "\"") return jstr()
      if (c == "{" || c == "[") {
        jp++; jws()
        if (substr(js, jp, 1) == (c == "{" ? "}" : "]")) { jp++; return 1 }
        for (;;) {
          if (c == "{") { jws(); if (!jstr()) return 0; jws(); if (substr(js, jp, 1) != ":") return 0; jp++ }
          if (!jval()) return 0
          jws()
          if (substr(js, jp, 1) == (c == "{" ? "}" : "]")) { jp++; return 1 }
          if (substr(js, jp, 1) != ",") return 0
          jp++
        }
      }
      if (match(substr(js, jp), /^(true|false|null|-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?)/)) { jp += RLENGTH; return 1 }
      return 0
    }
    # isjson: s, after one optional leading comma, is one or more comma-separated JSON arrays or
    # objects and nothing else.
    function isjson(s) {
      sub(/^,/, "", s)
      js = s; jn = length(s); jp = 1
      for (;;) {
        jws(); if (substr(js, jp, 1) !~ /^[[{]$/ || !jval()) return 0
        jws(); if (jp > jn) return 1
        if (substr(js, jp, 1) != ",") return 0
        jp++
      }
    }
    function endword() {
      if (word == "") return
      if (word ~ /^(-[A-Za-z]*c|eval)$/ || word ~ /(^|\/)(sh|bash|dash|zsh|ksh)$/) runs = 1
      if (cmdpos && (word == "printf" || word == "echo")) args = 1
      cmdpos = word ~ /^(then|do|else|elif|if|while|until|!|time)$/
      word = ""
    }
    {
      line = $0; n = length(line); out = ""; word = ""; cmdpos = 1; args = 0; runs = 0
      for (i = 1; i <= n; i++) {
        c = substr(line, i, 1)
        if (c == "\\") { out = out substr(line, i, 2); word = word "x"; i++; continue }
        if (c == "\047") {
          j = index(substr(line, i + 1), "\047")
          if (j == 0) { out = out substr(line, i); break }
          s = substr(line, i + 1, j - 1)
          data = args || (!runs && !(cmdpos && word == "") && isjson(s))
          out = out (data ? "\047\047" : substr(line, i, j + 1)); i += j; word = word "x"; continue
        }
        if (c == "\"") {
          for (j = i + 1; j <= n && substr(line, j, 1) != "\""; j++) if (substr(line, j, 1) == "\\") j++
          s = substr(line, i, j - i + 1)
          out = out ((args && s !~ /\$\(|`/) ? "\"\"" : s); i = j; word = word "x"; continue
        }
        if (c ~ /[ \t|;&():]/) {
          endword()
          if (c ~ /[|;&(:]/) { cmdpos = 1; args = 0; runs = 0 } else if (c == ")") args = 0
        } else word = word c
        out = out c
      }
      print out
    }'
}

# mask_language_names blanks a JSON string that names Python as a language, not a program
# (GOC-V0-062, V1-4531), in two contexts only: an entry of the list a language field holds, a
# JSON key or shell variable whose name ends in langs or languages, such as
# FRONTIER_OTHER_LANGS='["dotnet","python"]' or "languages": ["go", "python"], and a
# language-frontier "detail":"python:REASON". The list ends at its first "]", so an argv later on
# the same line is still read. "python" anywhere else, such as the argv ["python","go"] or
# ["python","build.py"], "python3", an image tag such as "python:alpine" outside a "detail"
# field, and an unquoted name still match. The list marker has no "]", which would end the list.
langlist="((LANGS|LANGUAGES|[Ll]angs|[Ll]anguages)\"? *[:=] *'?\\[[^]]*)\"python\""
mask_language_names() {
  sed -E -e ':list' -e "s/$langlist/\1\"<language>\"/" -e 't list' \
    -e 's/("detail" *: *)"python:([a-z][a-z0-9-]*)"/\1"[language]:\2"/g'
}

# scan reads grep -Hn lines (path:line:text) and drops only those whose path field is exactly the
# external-conformance script, so a renamed copy, a longer path or a line elsewhere naming it is
# still scanned. Quoted printf and echo fixture data (V1-3971) and JSON language names
# (GOC-V0-062) are masked first.
scan() {
  awk -v p="$external_conformance" 'index($0, p ":") != 1 || substr($0, length(p) + 2) !~ /^[0-9]+:/' | mask_fixture_data |
    mask_language_names | unexempted
}

# reaches_external reads Makefile text and names each way a gate step or another target reaches the
# exempt script: it may only be the recipe of its own target, which nothing lists as a step or
# prerequisite.
reaches_external() {
  awk -v p="${external_conformance##*/}" -v t=spec-toolchain-conformance '
    # make logical lines: GATE_STEPS and prerequisite lists continue one entry per line (STR-V0-920).
    { joined = joined $0 }
    /\\$/ { sub(/\\$/, " ", joined); next }
    { $0 = joined; joined = "" }
    /^GATE_STEPS *=/ { for (i = 3; i <= NF; i++) if ($i == t) print "GATE_STEPS lists " t; next }
    /^[^\t#][^:=]*:/ {
      target = substr($0, 1, index($0, ":") - 1); n = split(substr($0, index($0, ":") + 1), deps, /[ \t|]+/)
      for (i = 1; i <= n; i++) if (deps[i] == t && target != ".PHONY") print target " depends on " t
    }
    /^\t/ && index($0, p) && target != t { print "recipe of " target " runs " p }
  '
}

# GOC-V0-008 covers maintained adapters, not installed third-party dependency trees.
# Prune only a directory named node_modules; adjacent owned files remain scan targets.
integration_targets() {
  find "$1" -type d -name node_modules -prune -o -type f \( -name '*.mjs' -o -name '*.json' \) -print
}

cd "$source_root"
targets=$(
  printf '%s\n' Makefile
  find script -maxdepth 1 -type f -name '*.sh' ! -name "$(basename "$0")"
  integration_targets integrations 2>/dev/null
)

violations=$(printf '%s\n' "$targets" | xargs grep -HnE "$pattern" 2>/dev/null | scan || true)
via=$(
  reaches_external < Makefile
  printf '%s\n' "$targets" | grep -vx -e Makefile -e "$external_conformance" |
    xargs grep -Hn "${external_conformance##*/}" 2>/dev/null | grep -v '^[^:]*:[0-9]*: *#' || true
)
if [ -n "$via" ]; then
  printf 'GOC-V0-011: the external-conformance exemption is reachable from a scanned path:\n%s\n' "$via" >&2
  exit 1
fi
if [ -n "$violations" ]; then
  printf 'GOC-V0-008: a build/test/hook/packaging path names a Python interpreter:\n%s\n' "$violations" >&2
  exit 1
fi

# Falsifying self-test: prove the scan genuinely fires on a real invocation and genuinely
# allows every carved-out exception, so this is not a vacuous always-pass grep.
fixture=$(mktemp "${TMPDIR:-/tmp}/corvint-no-python-dep.XXXXXX")
fixture_tree=$(mktemp -d "${TMPDIR:-/tmp}/corvint-no-python-targets.XXXXXX")
trap 'rm -f "$fixture"; rm -rf "$fixture_tree"' EXIT

# The installed SDK example that triggered the host verifier must be outside the
# maintained scan, at any nesting depth. Similar names and adjacent owned code
# must still be selected and fail on an interpreter invocation.
mkdir -p "$fixture_tree/node_modules/sdk" "$fixture_tree/owned/node_modules/sdk" \
  "$fixture_tree/owned/node_modules-extra"
for path in node_modules/sdk/example.mjs owned/node_modules/sdk/example.json \
  owned/runner.mjs owned/config.json owned/node_modules-extra/runner.mjs; do
  printf 'python3 build.py\n' > "$fixture_tree/$path"
done
selected=$(integration_targets "$fixture_tree" | LC_ALL=C sort)
expected=$(printf '%s\n' "$fixture_tree/owned/config.json" \
  "$fixture_tree/owned/node_modules-extra/runner.mjs" "$fixture_tree/owned/runner.mjs" | LC_ALL=C sort)
if [ "$selected" != "$expected" ]; then
  echo "self-test: integration targets lost maintained files or included installed dependencies" >&2
  exit 1
fi
printf '%s\n' "$selected" | while IFS= read -r path; do
  if [ -z "$(grep -HnE "$pattern" "$path" | scan)" ]; then
    echo "self-test: an owned integration interpreter invocation was not detected" >&2
    exit 1
  fi
done

printf 'python3 build.py\n' > "$fixture"
if ! grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: scan failed to detect a genuine python3 invocation" >&2
  exit 1
fi

# shellcheck disable=SC2016
printf 'if [ -n "$CORVINT_TEST_EXTERNAL_PYTEST" ]; then python3 -m pytest; fi\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: the CORVINT_TEST_EXTERNAL_PYTEST opt-in path was not allowed" >&2
  exit 1
fi

printf 'go build -o corvint-analyzer-python ./cmd/corvint-analyzer-python\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: the corvint-analyzer-python Go tool name was not allowed" >&2
  exit 1
fi

printf 'no-python-runtime-dependency-test:\n\t@script/no-python-runtime-dependency_test.sh\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: this check's own Makefile gate step was not allowed" >&2
  exit 1
fi

printf '\t@script/no-python-runtime-dependency_test.sh && python3 -c 1\n' > "$fixture"
if ! grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: an allowed name on the line hid a genuine python3 invocation" >&2
  exit 1
fi

# GOC-V0-081 (V1-3842): the check runs in doc-gates, which every pull request runs, so a scanned
# path that gains a Python call cannot merge unseen; a single-quoted JSON literal is data.
# shellcheck disable=SC2016
for line in "x:1:LANGS='[\"cfamily\",\"python\"]'" "x:1:plan \"\$f\"',{\"detail\":\"python:dynamic-import\"}' golden"; do
  if [ -n "$(printf '%s\n' "$line" | scan)" ]; then
    echo "self-test: a single-quoted JSON literal naming a language was reported: $line" >&2
    exit 1
  fi
done
if [ -z "$(printf '%s\n' "x:1:LANGS='[\"a\"]'; python3 -c 1" | scan)" ]; then
  echo "self-test: a call after a single-quoted JSON literal was not reported" >&2
  exit 1
fi
# Shell code is never JSON data: a braced group is not valid JSON, a word that parses as JSON still
# runs when its command names a shell, eval or -c, and a bare word in command position may continue
# an earlier line's sh -c that grep did not show.
# shellcheck disable=SC2016
for line in "x:1:sh -c '{ python3 -c \"print(1)\"; }'" "x:1:sh -c '[\"\$(python3 -c 1)\"]'" \
  "x:1:bash -lc '{ pip install x; }'" "x:1:eval '[\"\$(pytest)\"]'" "x:1:X='{ python3 -c 1; }'" \
  "x:1:bash -c -- '[\"\$(python3 -c 1)\"]'" "x:1:/bin/sh -e '[\"\$(pip install x)\"]'" \
  "x:2:  '[\"\$(python3 -c 1)\"]' \\"; do
  if [ -z "$(printf '%s\n' "$line" | scan)" ]; then
    echo "self-test: executable shell code was masked as a JSON literal: $line" >&2
    exit 1
  fi
done
if ! grep -E '^DOC_GATE_TARGETS :=.* no-python-runtime-dependency-test( |$)' Makefile >/dev/null; then
  echo "self-test: doc-gates (DOC_GATE_TARGETS) does not run no-python-runtime-dependency-test" >&2
  exit 1
fi

# The external-conformance exemption is the exact path only: its own line is allowed, while the
# same invocation in another script, a longer path or a Makefile recipe naming it still fails.
fires() { printf '%s\n' "$1" | scan >/dev/null; }
if fires "$external_conformance:32:python=\${SPEC_TOOLCHAIN_PYTHON:-python3}"; then
  echo "self-test: the external-conformance script was not exempt by its exact path" >&2
  exit 1
fi
for line in "script/build.sh:32:python=\${SPEC_TOOLCHAIN_PYTHON:-python3}" \
  "$external_conformance.bak:1:python3 -m venv v" "x$external_conformance:1:pip install x" \
  "Makefile:282:	@$external_conformance && python3 -c 1"; do
  if ! fires "$line"; then
    echo "self-test: the external-conformance exemption hid a genuine invocation: $line" >&2
    exit 1
  fi
done

# V1-3842: shell tests are scan targets, and a python3 call added to one fails the scan like any
# other scanned script.
if ! printf '%s\n' "$targets" | grep -qx script/check-bash32_test.sh; then
  echo "self-test: script/*_test.sh shell tests are not scan targets" >&2
  exit 1
fi
if ! fires 'script/example_test.sh:12:  out=$(python3 -c "print(1)")'; then
  echo "self-test: a python3 call in a script/*_test.sh shell test was not detected" >&2
  exit 1
fi

# V1-3842: a full-line comment naming an interpreter is skipped; a #! interpreter line, a trailing
# comment after a call and an indented call are not. The opt-in drop is textual, so a line that only
# names the token in a comment passes; this pins that limit, which the GOC-V0-008 trace states.
for line in "script/x.sh:3:# python3 is deliberately not required" "script/x.sh:4:  # pip install is never run" "Makefile:9:#"; do
  if fires "$line"; then
    echo "self-test: a full-line comment was flagged: $line" >&2
    exit 1
  fi
done
# In .mjs a leading # is a private field, not a comment, so the same shape is still scanned there.
for line in "script/x.sh:1:#!/usr/bin/env python3" "script/x.sh:5:make x # python3" "script/x.sh:6:  python3 -c 1" \
  'integrations/x/hook.mjs:7:  #result = execFileSync("python3", ["-c", "1"]);' \
  'integrations/x/hook.sh.mjs:8:#run = () => execFileSync("pip", ["install", "x"]);'; do
  if ! fires "$line"; then
    echo "self-test: a genuine invocation was hidden as a comment: $line" >&2
    exit 1
  fi
done
if fires "script/x.sh:7:python3 -c 1 # $opt_in"; then
  echo "self-test: the textual opt-in drop changed; update the GOC-V0-008 trace" >&2
  exit 1
fi

# Reaching the exempt script from a gate step, a prerequisite or another recipe is detected; its
# own non-gate target is not.
printf 'GATE_STEPS = go-test\nspec-toolchain-conformance:\n\t@%s\n' "$external_conformance" > "$fixture"
if [ -n "$(reaches_external < "$fixture")" ]; then
  echo "self-test: the external-conformance target itself was reported as reaching the exemption" >&2
  exit 1
fi
for makefile in 'GATE_STEPS = go-test spec-toolchain-conformance' 'go-test: build spec-toolchain-conformance' \
  "$(printf 'go-test:\n\t@%s' "$external_conformance")" \
  "$(printf 'GATE_STEPS = \\\n  go-test \\\n  spec-toolchain-conformance \\\n  # end')" \
  "$(printf 'shell-test: \\\n  build \\\n  spec-toolchain-conformance \\\n  # end')"; do
  printf '%s\n' "$makefile" > "$fixture"
  if [ -z "$(reaches_external < "$fixture")" ]; then
    echo "self-test: a gate path reaching the external-conformance script was not detected: $makefile" >&2
    exit 1
  fi
done

# V1-3971: an interpreter name only inside a quoted printf or echo argument is fixture data; a real
# call (bare, after a pipe, after && or ;, inside $(...), unquoted after echo) is still reported.
# shellcheck disable=SC2016
for line in "script/x_test.sh:3:printf 'python3 -c 1\\n' > \"\$f\"" \
  "script/x_test.sh:4:  printf '%s\\n' 'pip install x' \"pytest -q\" >> \"\$f\"" \
  "script/x_test.sh:5:if true; then echo \"python3 build.py\"; fi" \
  "script/x_test.sh:6:out=\$(printf 'python3 -c 1' | wc -c)"; do
  if fires "$line"; then
    echo "self-test: quoted printf/echo fixture data was flagged: $line" >&2
    exit 1
  fi
  # The mutant that drops the quoting check (no mask_fixture_data stage) flags it, so the
  # cases above are not vacuous.
  if ! printf '%s\n' "$line" | unexempted >/dev/null; then
    echo "self-test: the fixture-data case does not depend on the quoting check: $line" >&2
    exit 1
  fi
done
# shellcheck disable=SC2016
for line in "script/x_test.sh:7:python3 -c 1" "script/x_test.sh:8:printf 'x' | python3 -" \
  "script/x_test.sh:9:printf 'x' && python3 -c 1" "script/x_test.sh:10:printf 'x'; pip install y" \
  "script/x_test.sh:11:out=\$(python3 -c 1)" "script/x_test.sh:12:printf '%s' \"\$(python3 -c 1)\"" \
  "script/x_test.sh:13:echo python3 -c 1" "script/x_test.sh:14:printf 'x' | sh -c 'pytest'" \
  "script/x_test.sh:15:echo 'x' | python3"; do
  if ! fires "$line"; then
    echo "self-test: a genuine invocation was hidden as fixture data: $line" >&2
    exit 1
  fi
done

# GOC-V0-062 (V1-4531): Python named as a language in a language field's list or a
# language:reason detail is data, while a JSON argv, even one whose other entries are language
# names, or a line that also runs an interpreter is still reported.
# shellcheck disable=SC2016
for line in "script/x.sh:440:FRONTIER_OTHER_LANGS=[\"cfamily\",\"dotnet\",\"python\",\"ruby\"]" \
  "integrations/x/report.json:3:  {\"reason\":\"LANGUAGE_FRONTIER\",\"detail\":\"python:dynamic-import\"}" \
  "integrations/x/langs.json:2:  \"languages\": [\"go\", \"python\"]"; do
  if fires "$line"; then
    echo "self-test: a JSON language name was flagged: $line" >&2
    exit 1
  fi
  # Without the mask_language_names stage the line is flagged, so the case is not vacuous.
  if ! printf '%s\n' "$line" | mask_fixture_data | unexempted >/dev/null; then
    echo "self-test: the language-name case does not depend on the language mask: $line" >&2
    exit 1
  fi
done
# shellcheck disable=SC2016
for line in "integrations/x/hooks.json:3:  \"command\": [\"python3\", \"hook.py\"]" \
  "integrations/x/hooks.json:7:  [\"python\",\"build.py\"]" "integrations/x/hooks.json:4:  \"command\": \"python3 hook.py\"" \
  "script/x.sh:18:langs='[\"ruby\",\"python\"]'; python3 -c 1" "integrations/x/hooks.json:8:  [\"python\",\"-c\",\"go\"]" \
  "integrations/x/x.json:2:  \"cmd\": \"python:3\"" "integrations/x/devcontainer.json:2:  \"image\": \"python:alpine\"" \
  "integrations/x/hooks.json:5:  {\"command\":[\"python\",\"go\"]}" \
  "integrations/x/hooks.json:6:  {\"languages\":[\"go\"],\"command\":[\"python\",\"go\"]}"; do
  if ! fires "$line"; then
    echo "self-test: a genuine invocation was hidden as a language name: $line" >&2
    exit 1
  fi
done

printf 'no-python-runtime-dependency: build/test/hook/packaging paths name no Python interpreter\n'
