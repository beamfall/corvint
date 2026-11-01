#!/usr/bin/env bash
# usage: check-windows-main.sh [MODULE_DIR]
# ASD-V0-974 (V1-3782): fails when a package main still builds for GOOS=windows but none of the
# files Go selects for windows declares func main, which `make windows-vet` reports late as
# "function main is undeclared in the main package". The usual cause is a new untagged file (for
# example help.go) beside a main.go whose `//go:build darwin || linux` excludes windows. It lists
# packages only (no type check), so it takes seconds. Exit 0 clean, 1 findings, 2 cannot run.
set -uo pipefail
export LC_ALL=C GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOFLAGS= GOWORK=off GOENV=off
dir=${1:-.}
listing=$(cd "$dir" && go list -e -f '{{if eq .Name "main"}}{{.Dir}}|{{join .GoFiles " "}}{{end}}' ./...) ||
  { echo "check-windows-main: REFUSE go-list-failed in $dir" >&2; exit 2; }
bad=0
while IFS='|' read -r pkg files; do
  [[ -n $pkg && -n $files ]] || continue
  found=0
  for f in $files; do
    if grep -qE '^func main\(\)' "$pkg/$f"; then found=1; break; fi
  done
  if (( !found )); then
    printf 'check-windows-main: %s builds for GOOS=windows from %s, but none declares func main; give those files the build constraint main.go carries, or add a windows main (ASD-V0-974)\n' "$pkg" "$files" >&2
    bad=1
  fi
done <<< "$listing"
exit $bad
