#!/bin/sh
set -eu
# Opt-in only: run via `make sql-native-ratchets`, never `make gate` — it builds cmd/corvint
# twice and runs 30 benchmark samples, far too slow for the gate. See
# docs/specs/sql-native-ratchet-gate-v0.md. The toolchain root is discovered, but its identity is
# pinned by content (SNR-V0-003/SNR-V0-004, decision 0104): the go binary digest and the whole-root
# receipt below, never the path or the version string alone.

causal_parent=a7db6200685c1ea7d4afc1798fb376ccc724acd1
go_root=$(GOTOOLCHAIN=local go env GOROOT)
go_bin="$go_root/bin/go"
go_sha=71c4991041d8e44975c882e4f72005719c958013d3340dc665a3808b72ddf702
receipt_sha=f8337b642a5ffaf4ee44e29fec2b8053450a46c549e7d61187a7897487eb3da6
tree='{"Entries":17277,"SHA256":"94b2c0ed86c9f62518348cc7fc3e4442e4e40a0194d10cfc8f6bc1be98447c33"}'
lock="$(git rev-parse --git-dir)/sql-native-ratchets.lock"
if ! mkdir "$lock" 2>/dev/null; then
	printf '%s\n' 'sql-native-ratchets status=REJECTED reason=CONCURRENT_RUN' >&2
	exit 75
fi
ratchet_root=
cleanup() { test -z "$ratchet_root" || { chmod -R u+w "$ratchet_root" 2>/dev/null || :; rm -rf "$ratchet_root"; }; rmdir "$lock" 2>/dev/null || :; }
trap cleanup EXIT
trap 'cleanup; exit 143' HUP INT TERM
ratchet_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-sql-native.XXXXXX")

case ${SQL_NATIVE_RATCHET_LOCK_TEST_HOLD:-} in
"") ;;
spin) while :; do :; done ;;
*) printf '%s\n' 'sql-native-ratchets status=REJECTED reason=INVALID_LOCK_TEST' >&2; exit 64 ;;
esac

test -n "$go_root"
test -x "$go_bin"
test "$(shasum -a 256 "$go_bin" | awk '{print $1}')" = "$go_sha"
test "$(GOTOOLCHAIN=local "$go_bin" env GOVERSION)" = go1.27.0
source_bytes=$(find experimental/analyzers/sqlnative cmd/corvint-analyzer-sql-native -name '*.go' ! -name '*_test.go' -type f -exec wc -c {} \; | awk '{total+=$1}END{print total}')
test "$source_bytes" -le 65536

run_go() {
	dir=$1 state=$2; shift 2
	mkdir -p "$state/home" "$state/gopath" "$state/modcache" "$state/cache"
	(cd "$dir" && env -i PATH=/usr/bin:/bin HOME="$state/home" GOPATH="$state/gopath" GOMODCACHE="$state/modcache" GOCACHE="$state/cache" GOROOT="$go_root" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local CGO_ENABLED=0 GOMAXPROCS=1 "$go_bin" "$@")
}

current_dir=$(pwd)
# SNR-V0-007 (decision 0226): Core without the candidate is HEAD with the candidate's two source
# trees excluded. A Core that imports the candidate cannot build from it, and any other difference
# from the current tree refuses at the binary or dependency comparison.
core_base=$(git rev-parse --verify 'HEAD^{commit}')
mkdir "$ratchet_root/core-base" "$ratchet_root/causal-parent"
git archive "$core_base" -- . ':(exclude)experimental/analyzers/sqlnative' ':(exclude)cmd/corvint-analyzer-sql-native' > "$ratchet_root/core-base.tar"
tar -x -C "$ratchet_root/core-base" < "$ratchet_root/core-base.tar"
# SNR-V0-013: the causal parent is never substituted. When its object is absent the parent half
# of the comparison abstains and the report says so; every other check still runs.
causal_available=false
if git cat-file -e "$causal_parent^{commit}" 2>/dev/null; then
	causal_available=true
	git archive "$causal_parent" > "$ratchet_root/causal-parent.tar"
	tar -x -C "$ratchet_root/causal-parent" < "$ratchet_root/causal-parent.tar"
fi
run_go "$current_dir" "$ratchet_root/receipt-build" build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$ratchet_root/toolchain-receipt" ./cmd/corvint-go-toolchain-receipt
test "$(shasum -a 256 "$ratchet_root/toolchain-receipt" | awk '{print $1}')" = "$receipt_sha"
check_toolchain() { test "$("$ratchet_root/toolchain-receipt" "$go_root")" = "$tree"; }
check_toolchain

build() {
	dir=$1 state=$2 output=$3 package=$4
	check_toolchain
	run_go "$dir" "$state" build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$output" "$package"
	check_toolchain
}

build "$current_dir" "$ratchet_root/candidate-build" "$ratchet_root/candidate" ./cmd/corvint-analyzer-sql-native
binary_bytes=$(wc -c < "$ratchet_root/candidate" | tr -d ' ')
test "$binary_bytes" -le 6291456
mkdir "$ratchet_root/core-a" "$ratchet_root/core-b"
core_a="$ratchet_root/core-a/corvint" core_b="$ratchet_root/core-b/corvint"
test "${#core_a}" = "${#core_b}"
build "$current_dir" "$ratchet_root/core-current" "$core_a" ./cmd/corvint
build "$ratchet_root/core-base" "$ratchet_root/core-base-build" "$core_b" ./cmd/corvint
cmp "$core_a" "$core_b"
check_toolchain
run_go "$current_dir" "$ratchet_root/deps-current" list -deps -f '{{.ImportPath}}' ./cmd/corvint > "$ratchet_root/core-a/deps.raw"
LC_ALL=C sort "$ratchet_root/core-a/deps.raw" > "$ratchet_root/core-a/deps"
check_toolchain
check_toolchain
run_go "$ratchet_root/core-base" "$ratchet_root/deps-base" list -deps -f '{{.ImportPath}}' ./cmd/corvint > "$ratchet_root/core-b/deps.raw"
LC_ALL=C sort "$ratchet_root/core-b/deps.raw" > "$ratchet_root/core-b/deps"
check_toolchain
cmp "$ratchet_root/core-a/deps" "$ratchet_root/core-b/deps"

for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do
	os=${target%/*} arch=${target#*/}
	check_toolchain
	mkdir -p "$ratchet_root/$os-$arch/home" "$ratchet_root/$os-$arch/gopath" "$ratchet_root/$os-$arch/modcache" "$ratchet_root/$os-$arch/cache"
	env -i PATH=/usr/bin:/bin HOME="$ratchet_root/$os-$arch/home" GOPATH="$ratchet_root/$os-$arch/gopath" GOMODCACHE="$ratchet_root/$os-$arch/modcache" GOCACHE="$ratchet_root/$os-$arch/cache" GOROOT="$go_root" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "$go_bin" build -trimpath -buildvcs=false -o "$ratchet_root/$os-$arch/analyzer" ./cmd/corvint-analyzer-sql-native
	check_toolchain
done

measure() {
	dir=$1 state=$2 benchmark=$3 log=$4
	for sample in $(seq 1 10); do
		check_toolchain
		result=$(run_go "$dir" "$state-$sample" test -run '^$' -bench "^$benchmark$" -benchmem -count=1 -benchtime=100x ./experimental/analyzers/sqlnative)
		check_toolchain
		line=$(printf '%s\n' "$result" | awk -v name="$benchmark" 'index($1,name)==1{print;exit}')
		bytes=$(printf '%s\n' "$line" | awk '{print $(NF-3)+0}')
		allocs=$(printf '%s\n' "$line" | awk '{print $(NF-1)+0}')
		test -n "$bytes"; test -n "$allocs"
		printf '%s %s %s\n' "$sample" "$bytes" "$allocs" >> "$log"
	done
}

candidate_log="$ratchet_root/candidate.log"
check_toolchain
run_go "$current_dir" "$ratchet_root/complexity" test -count=1 -run '^TestSQLite351NestedWithDelimiterWorkRatchet$' ./experimental/analyzers/sqlnative
check_toolchain
measure "$current_dir" "$ratchet_root/candidate-sample" BenchmarkAnalyzeCandidate "$candidate_log"
# shellcheck disable=SC2046
set -- $(awk 'NR==1{mb=$2;ma=$3}{if($2>mb)mb=$2;if($3>ma)ma=$3}END{if(NR!=10)exit 1;print mb,ma}' "$candidate_log")
candidate_max_bytes=$1 candidate_max_allocs=$2
test "$candidate_max_bytes" -le 98304
test "$candidate_max_allocs" -le 400

fixture_sha=$(shasum -a 256 experimental/analyzers/sqlnative/causal_benchmark_test.go | awk '{print $1}')
causal_candidate_log="$ratchet_root/causal-candidate.log" causal_parent_log="$ratchet_root/causal-parent.log"
measure "$current_dir" "$ratchet_root/causal-candidate-sample" BenchmarkAnalyzeCausalSQL "$causal_candidate_log"
# shellcheck disable=SC2046
set -- $(awk 'NR==1{mb=$2;ma=$3}{if($2>mb)mb=$2;if($3>ma)ma=$3}END{if(NR!=10)exit 1;print mb,ma}' "$causal_candidate_log")
causal_candidate_max_bytes=$1 causal_candidate_max_allocs=$2
test "$causal_candidate_max_allocs" -le 400
causal_parent_min_bytes=NOT_RUN causal_parent_min_allocs=NOT_RUN causal=NOT_RUN-causal-parent-unavailable
if $causal_available; then
	cp experimental/analyzers/sqlnative/causal_benchmark_test.go "$ratchet_root/causal-parent/experimental/analyzers/sqlnative/causal_benchmark_test.go"
	measure "$ratchet_root/causal-parent" "$ratchet_root/causal-parent-sample" BenchmarkAnalyzeCausalSQL "$causal_parent_log"
	# shellcheck disable=SC2046
	set -- $(awk 'NR==1{mb=$2;ma=$3}{if($2<mb)mb=$2;if($3<ma)ma=$3}END{if(NR!=10)exit 1;print mb,ma}' "$causal_parent_log")
	causal_parent_min_bytes=$1 causal_parent_min_allocs=$2
	test "$causal_parent_min_allocs" -gt 400
	causal=identical-workload-lifecycle-parent-fails-alloc-cap
fi

printf 'sql-native-ratchets core_base=%s causal_parent=%s fixture_sha256=%s source_bytes=%s binary_bytes=%s goroot_entries=17277 goroot_sha256=94b2c0ed86c9f62518348cc7fc3e4442e4e40a0194d10cfc8f6bc1be98447c33 core_binary=identical core_deps=identical complexity=delimiter-pairs-linear-old-parenAt-model-fails candidate_max_b_op=%s candidate_max_allocs_op=%s causal_candidate_max_b_op=%s causal_candidate_max_allocs_op=%s causal_parent_min_b_op=%s causal_parent_min_allocs_op=%s causal=%s latency=NOT_RUN\n' "$core_base" "$causal_parent" "$fixture_sha" "$source_bytes" "$binary_bytes" "$candidate_max_bytes" "$candidate_max_allocs" "$causal_candidate_max_bytes" "$causal_candidate_max_allocs" "$causal_parent_min_bytes" "$causal_parent_min_allocs" "$causal"
