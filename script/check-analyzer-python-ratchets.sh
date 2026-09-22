#!/bin/sh
set -eu

root=${1:?repository root required}
binary=${2:?stripped analyzer binary required}
test -d "$root"
test -f "$root/internal/analyzerpython/analyzer.go"
test ! -L "$root/internal/analyzerpython/analyzer.go"
test -f "$root/cmd/corvint-analyzer-python/main.go"
test ! -L "$root/cmd/corvint-analyzer-python/main.go"
test -f "$binary"
test ! -L "$binary"
source_bytes=$(wc -c < "$root/internal/analyzerpython/analyzer.go")
command_bytes=$(wc -c < "$root/cmd/corvint-analyzer-python/main.go")
binary_bytes=$(wc -c < "$binary")

test "$source_bytes" -le 65536
test "$command_bytes" -le 4096
test "$binary_bytes" -le 6291456
