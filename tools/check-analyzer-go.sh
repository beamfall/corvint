#!/bin/sh
# Focused, offline verification for the standalone Go analyzer candidate.
# This script intentionally does not run a repository-wide gate.
set -eu

budget_tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-analyzer-go.XXXXXX")
base_root=$budget_tmp/integration-base
cleanup() {
	if [ -d "$base_root" ]; then
		git worktree remove "$base_root" >/dev/null 2>&1 || true
	fi
	rm -rf "$budget_tmp"
}
trap cleanup EXIT HUP INT TERM

go_root=/opt/homebrew/Cellar/go/1.27.0/libexec
go_tool=$go_root/bin/go
integration_parent=718dfc7db3e9162f045669309e2f28dc73487aff
go_sha256=71c4991041d8e44975c882e4f72005719c958013d3340dc665a3808b72ddf702
receipt_sha256=f8337b642a5ffaf4ee44e29fec2b8053450a46c549e7d61187a7897487eb3da6
tree_receipt='{"Entries":17277,"SHA256":"94b2c0ed86c9f62518348cc7fc3e4442e4e40a0194d10cfc8f6bc1be98447c33"}'

go_descriptor=$(stat -f '%d:%i:%p:%z:%m' "$go_tool")
toolchain_receipts=$budget_tmp/toolchain-receipts
check_tool_descriptor() {
	label=$1
	[ "$(shasum -a 256 "$go_tool" | awk '{print $1}')" = "$go_sha256" ] || {
	echo "unexpected Go executable receipt" >&2
	exit 1
}
	[ "$(stat -f '%d:%i:%p:%z:%m' "$go_tool")" = "$go_descriptor" ] || {
		echo "unexpected Go executable descriptor" >&2
		exit 1
}
	printf '%s descriptor=%s executable_sha256=%s\n' "$label" "$go_descriptor" "$go_sha256" >>"$toolchain_receipts"
}
check_tool_descriptor initial

# Every Go invocation uses a unique, empty private environment. GOROOT is the
# pinned immutable toolchain; HOME/GOPATH/module cache/config/cache cannot
# inherit caller state, and proxy/checksum lookup is disabled.
go_env() {
	label=$1
	shift
	state=$budget_tmp/env-$label
	mkdir -p "$state/home" "$state/gopath" "$state/modcache" "$state/cache"
	env -i \
		HOME="$state/home" PATH=/usr/bin:/bin GOROOT="$go_root" GOPATH="$state/gopath" \
		GOMODCACHE="$state/modcache" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off GONOSUMDB='*' \
		GOTOOLCHAIN=local GOMAXPROCS=1 CGO_ENABLED=0 GOCACHE="$state/cache" \
		"$@"
}
go_command() {
	label=$1
	shift
	go_env "$label" "$go_tool" "$@"
}
go_command_with_target() {
	label=$1
	target=$2
	shift 2
	go_env "$label" GOOS="${target%/*}" GOARCH="${target#*/}" "$go_tool" "$@"
}

# The receipt binary itself is built through the same private pinned command,
# then used immediately before and after every candidate/Core build and every
# benchmark. A toolchain-tree mismatch aborts without recording evidence.
check_tool_descriptor receipt-build-pre
go_command receipt-build build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$budget_tmp/toolchain-receipt" ./cmd/corvint-go-toolchain-receipt
check_tool_descriptor receipt-build-post
[ "$(shasum -a 256 "$budget_tmp/toolchain-receipt" | awk '{print $1}')" = "$receipt_sha256" ] || {
	echo "unexpected toolchain receipt verifier binary" >&2
	exit 1
}
check_toolchain() {
	label=$1
	check_tool_descriptor "$label"
	[ "$("$budget_tmp/toolchain-receipt" "$go_root")" = "$tree_receipt" ] || {
		echo "unexpected GOROOT receipt" >&2
		exit 1
	}
	printf '%s tree=%s\n' "$label" "$tree_receipt" >>"$toolchain_receipts"
}
bracket_go() {
	label=$1
	shift
	check_toolchain "$label-pre"
	go_command "$label" "$@"
	check_toolchain "$label-post"
}
bracket_target_go() {
	label=$1
	target=$2
	shift 2
	check_toolchain "$label-pre"
	go_command_with_target "$label" "$target" "$@"
	check_toolchain "$label-post"
}
bracket_go_in() {
	directory=$1
	label=$2
	shift 2
	check_toolchain "$label-pre"
	(
		cd "$directory"
		go_command "$label" "$@"
	)
	check_toolchain "$label-post"
}

bracket_go focused-test test -count=1 ./internal/analyzergo ./cmd/corvint-analyzer-go
bracket_go focused-vet vet ./internal/analyzergo ./cmd/corvint-analyzer-go
bracket_go focused-race test -race -count=1 ./internal/analyzergo ./cmd/corvint-analyzer-go
for target in linux/amd64 darwin/arm64 windows/amd64; do
	bracket_target_go "candidate-$target" "$target" build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$budget_tmp/${target%/*}-${target#*/}" ./cmd/corvint-analyzer-go
	bracket_target_go "library-test-$target" "$target" test -c -o "$budget_tmp/${target%/*}-${target#*/}-library.test" ./internal/analyzergo
	bracket_target_go "command-test-$target" "$target" test -c -o "$budget_tmp/${target%/*}-${target#*/}-command.test" ./cmd/corvint-analyzer-go
done
[ "$(shasum -a 256 "$budget_tmp/darwin-arm64" | awk '{print $1}')" = "23f86c1aa039595d19e88f1e0519f67b7db3784b133bfc65d8279fc2642d256c" ] || {
	echo "unexpected stripped analyzer binary" >&2
	exit 1
}

benchmark_samples_pass() {
	awk '
	function number(value) { return value ~ /^[0-9]+([.][0-9]+)?$/ }
	{
		if ($1 !~ /^sample=[0-9]$/ || NF != 9 || $2 !~ /^BenchmarkAnalyzeCandidate(-[0-9]+)?$/ || !number($3) || !number($4) || $5 != "ns/op" || !number($6) || $7 != "B/op" || !number($8) || $9 != "allocs/op") { bad = 1; next }
		sample = substr($1, 8)
		if (seen[sample]++) { bad = 1; next }
		count++
	}
	END {
		if (bad || count != 10) exit 1
		for (sample = 0; sample < 10; sample++) if (!(sample in seen)) exit 1
	}'
}
benchmark_row() {
	awk '/^BenchmarkAnalyzeCandidate(-[0-9]+)?[[:space:]]/ { print; count++ } END { if (count != 1) exit 1 }' "$1"
}
run_benchmark_samples() {
	name=$1
	directory=$2
	output=$3
	: >"$output"
	sample=0
	while [ "$sample" -lt 10 ]; do
		raw_output=$budget_tmp/$name-$sample.raw
		if [ "$directory" = . ]; then
			bracket_go "$name-$sample" test -run '^$' -bench '^BenchmarkAnalyzeCandidate$' -benchmem -count=1 ./internal/analyzergo >"$raw_output"
		else
			bracket_go_in "$directory" "$name-$sample" test -run '^$' -bench '^BenchmarkAnalyzeCandidate$' -benchmem -count=1 ./internal/analyzergo >"$raw_output"
		fi
		if ! row=$(benchmark_row "$raw_output"); then
			cat "$raw_output" >&2
			echo "missing, partial, duplicate, or malformed $name benchmark row" >&2
			return 1
		fi
		printf 'sample=%s %s\n' "$sample" "$row" >>"$output"
		sample=$((sample + 1))
	done
	benchmark_samples_pass <"$output"
}
benchmark_parser_valid=$budget_tmp/benchmark-parser-valid
: >"$benchmark_parser_valid"
sample=0
while [ "$sample" -lt 10 ]; do
	printf 'sample=%s BenchmarkAnalyzeCandidate-1 1 1 ns/op 1 B/op 1 allocs/op\n' "$sample" >>"$benchmark_parser_valid"
	sample=$((sample + 1))
done
benchmark_samples_pass <"$benchmark_parser_valid" || { echo "benchmark parser rejected ten valid samples" >&2; exit 1; }
if benchmark_samples_pass </dev/null; then echo "benchmark parser accepted zero samples" >&2; exit 1; fi
head -n 9 "$benchmark_parser_valid" | benchmark_samples_pass && { echo "benchmark parser accepted partial samples" >&2; exit 1; }
{ cat "$benchmark_parser_valid"; tail -n 1 "$benchmark_parser_valid"; } | benchmark_samples_pass && { echo "benchmark parser accepted duplicate samples" >&2; exit 1; }
printf 'sample=0 BenchmarkAnalyzeCandidate-1 malformed\n' | benchmark_samples_pass && { echo "benchmark parser accepted malformed samples" >&2; exit 1; }

# The exact integration parent predates this package, so the only permitted
# performance statement is the profile's parentless absolute ceiling: every
# one of the ten candidate samples must satisfy the accepted analyzergo
# allocation-ceiling row (12,000 B/op, 200 allocs/op). No baseline, no A/B.
run_benchmark_samples candidate . "$budget_tmp/candidate-bench"
awk '{ if ($6 > 12000 || $8 > 200) exit 1 }' "$budget_tmp/candidate-bench" || { echo "candidate allocation ceiling exceeded" >&2; exit 1; }
if ! grep -Fq 'const benchmarkFixtureSHA256 = "sha256:30e65aedcd3180e2bd573de479af77b4fb997296f69150efd308195034afc283"' internal/analyzergo/analyzer_test.go; then
	echo "analyzer benchmark fixture receipt changed" >&2
	exit 1
fi
if rg -n '"go/build"' internal/analyzergo --glob '!**/*_test.go'; then
	echo "analyzer must not import ambient go/build" >&2
	exit 1
fi

# Core proof is against the exact supplied integration parent, in a clean
# detached worktree. The candidate is unreferenced, so both dependency graph
# and stripped binary must remain byte-identical.
git worktree add --detach --quiet "$base_root" "$integration_parent"
go_command current-core-deps list -deps ./cmd/corvint | LC_ALL=C sort >"$budget_tmp/current-core-deps"
(
	cd "$base_root"
	go_command integration-parent-core-deps list -deps ./cmd/corvint | LC_ALL=C sort
) >"$budget_tmp/base-core-deps"
cmp "$budget_tmp/base-core-deps" "$budget_tmp/current-core-deps"
mkdir -p "$budget_tmp/core-head" "$budget_tmp/core-base"
bracket_go current-core-build build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$budget_tmp/core-head/corvint" ./cmd/corvint
bracket_go_in "$base_root" integration-parent-core-build build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$budget_tmp/core-base/corvint" ./cmd/corvint
cmp "$budget_tmp/core-base/corvint" "$budget_tmp/core-head/corvint"

source_bytes=$(find internal/analyzergo cmd/corvint-analyzer-go -name '*.go' ! -name '*_test.go' ! -path '*/testdata/*' -exec wc -c {} + | awk 'END { print $1 }')
[ "$source_bytes" -le 65536 ] || { echo "analyzer source budget exceeded: $source_bytes > 65536" >&2; exit 1; }
for target in linux/amd64 darwin/arm64 windows/amd64; do
	binary="$budget_tmp/${target%/*}-${target#*/}"
	binary_bytes=$(wc -c < "$binary")
	[ "$binary_bytes" -le 6291456 ] || { echo "analyzer binary budget exceeded: $binary_bytes > 6291456" >&2; exit 1; }
done

git diff --check
echo "corvint-analyzer-go focused verification passed; parent=$integration_parent source_bytes=$source_bytes"
