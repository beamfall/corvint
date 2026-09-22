#!/bin/sh
# fake-agent.sh proves the cem-trial pipeline without a model. It reads the
# prompt as its only argument, cites the first hunk's own source file, adds the
# stem-convention test file when the working copy holds one, and lists every
# remaining hunk as unknown. It reads only its working directory.
set -eu
prompt=${1:-}
first=$(printf '%s\n' "$prompt" | sed -n 's/^1\. \([^ ]*\) @@.*/\1/p' | head -n 1)
last=$(printf '%s\n' "$prompt" | sed -n 's/^\([0-9][0-9]*\)\. .* @@.*/\1/p' | tail -n 1)
[ -n "$first" ] || first="README.md"
[ -n "$last" ] || last=1
guess=""
case "$first" in
  *.go) guess="${first%.go}_test.go" ;;
  *.tsx) guess="${first%.tsx}.test.tsx" ;;
  *.ts) guess="${first%.ts}.test.ts" ;;
  *.swift) guess="${first%.swift}Tests.swift" ;;
  *.py) guess="${first%.py}_test.py" ;;
esac
printf 'I read the base revision and cited what I could.\n\n```json\n'
printf '{"citations":[{"hunk":"1","path":"%s","lines":"1:2","relation":"implementation","confidence":"likely"}' "$first"
if [ -n "$guess" ] && [ -f "$guess" ]; then
  printf ',{"hunk":"1","path":"%s","lines":"1:2","relation":"test-claim","confidence":"likely"}' "$guess"
fi
printf '],"unknown":['
n=2
sep=""
while [ "$n" -le "$last" ]; do
  printf '%s"%s"' "$sep" "$n"
  sep=","
  n=$((n + 1))
done
printf ']}\n```\n'
