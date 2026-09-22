#!/bin/sh
# Runs the bounded native-bridge pre-review matrix against one clean immutable HEAD.
set -u

failure_output_limit=12288
lock_type=corvint-native-bridge-pre-review/v1
run_dir=
commands=
lock_dir=
lock_acquired=0
stage_pid=
finished=0

stop_stage() {
	if [ -z "$stage_pid" ]; then
		return
	fi
	kill -TERM "$stage_pid" 2>/dev/null || true
	wait "$stage_pid" 2>/dev/null || true
	stage_pid=
}

cleanup() {
	status=$?
	trap - EXIT INT TERM
	stop_stage
	if [ -n "$run_dir" ]; then
		rm -rf "$run_dir"
	fi
	if [ "$lock_acquired" -eq 1 ]; then
		rm -f "$lock_dir/owner"
		rmdir "$lock_dir" 2>/dev/null || true
	fi
	exit "$status"
}

on_signal() {
	stop_stage
	exit "$1"
}

trap cleanup EXIT
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

root=$(git rev-parse --show-toplevel) || exit 1
cd "$root" || exit 1

if [ -n "$(git status --porcelain --untracked-files=all)" ]; then
	printf '%s\n' 'native-bridge pre-review refuses a dirty worktree' >&2
	exit 1
fi

head=$(git rev-parse HEAD) || exit 1
tree=$(git rev-parse HEAD^{tree}) || exit 1
receipt_dir=$(git rev-parse --git-path corvint/native-bridge-pre-review) || exit 1
lock_dir="$receipt_dir.lock"
umask 077
mkdir -p "$receipt_dir" || exit 1
if ! mkdir "$lock_dir"; then
	printf '%s\n' "native-bridge pre-review already running for this worktree: $lock_dir" >&2
	exit 1
fi
lock_acquired=1
printf 'type=%s\nhead=%s\ntree=%s\nworktree=%s\npid=%s\n' "$lock_type" "$head" "$tree" "$root" "$$" > "$lock_dir/owner" || exit 1

tmp_root=${TMPDIR:-/tmp}
run_dir=$(mktemp -d "$tmp_root/corvint-native-bridge-pre-review.XXXXXX") || exit 1
run_id=${run_dir##*.}
receipt="$receipt_dir/$head.json"
failure_dir="$receipt_dir/failures/$head"
cache_dir="$run_dir/go-build-cache"
commands="$run_dir/commands.json"
first=1

printf '{"schema":"corvint-native-bridge-pre-review/v1","head":"%s","tree":"%s","cem_ocm":"NOT_RUN","controlled_concurrency":{"go_flags":"-p=1","gomaxprocs":2},"commands":[' "$head" "$tree" > "$commands"

append_result() {
	name=$1
	command=$2
	result=$3
	status=$4
	output=$5
	failure_artifact=${6:-}
	failure_artifact_sha256=${7:-}
	digest=$(shasum -a 256 "$output" | awk '{print $1}') || exit 1
	if [ "$first" -eq 0 ]; then
		printf ',' >> "$commands"
	fi
	first=0
	printf '{"name":"%s","command":"%s","result":"%s","exit_status":%s,"output_sha256":"sha256:%s"' "$name" "$command" "$result" "$status" "$digest" >> "$commands"
	if [ -n "$failure_artifact" ]; then
		printf ',"failure_artifact":{"path":"%s","sha256":"sha256:%s"}' "$failure_artifact" "$failure_artifact_sha256" >> "$commands"
	fi
	printf '}' >> "$commands"
}

finish() {
	result=$1
	if [ "$finished" -eq 1 ]; then
		return
	fi
	finished=1
	printf '],"result":"%s"}\n' "$result" >> "$commands"
	mv "$commands" "$receipt" || exit 1
	cat "$receipt"
}

sanitize_output() {
	dd if="$1" bs="$failure_output_limit" count=1 2>/dev/null |
		LC_ALL=C tr -c '\11\12\15\40-\176' '?' |
		sed -E \
			-e 's/([Aa]uthorization:[[:space:]]*).*/\1[REDACTED]/' \
			-e 's/([Pp]assword|[Pp]ass|[Tt]oken|[Ss]ecret)[[:space:]]*[=:][[:space:]]*[^[:space:]]+/\1=[REDACTED]/g' \
			-e 's#(https?://)[^/@[:space:]]+@#\1[REDACTED]@#g'
}

retain_failure() {
	stage=$1
	command=$2
	status=$3
	output=$4
	output_bytes=$(wc -c < "$output" | tr -d '[:space:]') || exit 1
	if [ "$output_bytes" -gt "$failure_output_limit" ]; then
		captured_bytes=$failure_output_limit
		truncated=true
	else
		captured_bytes=$output_bytes
		truncated=false
	fi
	mkdir -p "$failure_dir" || exit 1
	failure_artifact_relative="failures/$head/$run_id-$stage.txt"
	failure_artifact="$receipt_dir/$failure_artifact_relative"
	failure_artifact_tmp="$failure_dir/.$run_id-$stage.tmp"
	output_digest=$(shasum -a 256 "$output" | awk '{print $1}') || exit 1
	{
		printf 'schema=corvint-native-bridge-pre-review-failure/v1\n'
		printf 'head=%s\n' "$head"
		printf 'tree=%s\n' "$tree"
		printf 'stage=%s\n' "$stage"
		printf 'command=%s\n' "$command"
		printf 'exit_status=%s\n' "$status"
		printf 'output_sha256=sha256:%s\n' "$output_digest"
		printf 'output_bytes=%s\n' "$output_bytes"
		printf 'captured_bytes=%s\n' "$captured_bytes"
		printf 'truncated=%s\n' "$truncated"
		printf 'sanitization=ascii-printable-secret-redaction/v1\n---\n'
		sanitize_output "$output"
		if [ "$truncated" = true ]; then
			printf '\n[output truncated after %s bytes]\n' "$failure_output_limit"
		fi
	} > "$failure_artifact_tmp" || exit 1
	mv "$failure_artifact_tmp" "$failure_artifact" || exit 1
	failure_artifact_sha256=$(shasum -a 256 "$failure_artifact" | awk '{print $1}') || exit 1
}

run_stage() {
	name=$1
	fault_stage=${CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE:-}
	if [ "$fault_stage" = "$name" ]; then
		printf '%s\n' 'injected native bridge pre-review failure'
		fault_output_text=${CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_OUTPUT_TEXT:-}
		if [ -n "$fault_output_text" ]; then
			printf '%s\n' "$fault_output_text"
		fi
		fault_output_bytes=${CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_OUTPUT_BYTES:-0}
		case "$fault_output_bytes" in
			*[!0-9]*) printf '%s\n' 'invalid injected output byte count' >&2; return 98 ;;
		esac
		if [ "$fault_output_bytes" -gt 65536 ]; then
			printf '%s\n' 'injected output byte count exceeds limit' >&2
			return 98
		fi
		i=0
		while [ "$i" -lt "$fault_output_bytes" ]; do
			printf x
			i=$((i + 1))
		done
		fault_sleep_seconds=${CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_SLEEP_SECONDS:-0}
		case "$fault_sleep_seconds" in
			*[!0-9]*) printf '%s\n' 'invalid injected sleep seconds' >&2; return 98 ;;
		esac
		if [ "$fault_sleep_seconds" -gt 60 ]; then
			printf '%s\n' 'injected sleep seconds exceeds limit' >&2
			return 98
		fi
		if [ "$fault_sleep_seconds" -gt 0 ]; then
			sleep "$fault_sleep_seconds" &
			stage_pid=$!
			printf '\nchild_pid=%s\n' "$stage_pid"
			wait "$stage_pid"
			stage_pid=
		fi
		return 97
	fi
	shift
	"$@" &
	stage_pid=$!
	wait "$stage_pid"
	status=$?
	stage_pid=
	return "$status"
}

run() {
	name=$1
	command=$2
	shift 2
	output="$run_dir/$name.out"
	if run_stage "$name" "$@" > "$output" 2>&1; then
		append_result "$name" "$command" PASS 0 "$output"
		return 0
	else
		status=$?
	fi
	retain_failure "$name" "$command" "$status" "$output"
	append_result "$name" "$command" FAIL "$status" "$output" "$failure_artifact_relative" "$failure_artifact_sha256"
	finish FAIL
	printf '%s\n' "native-bridge pre-review stage $name failed; diagnostics=$failure_artifact" >&2
	exit 1
}

go_env="GOCACHE=$cache_dir GOTOOLCHAIN=local GOFLAGS=-p=1 GOMAXPROCS=2"

run diff-before 'git diff --check' git diff --check
run package-and-clis "$go_env go test -count=1 -timeout=180s ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java" env "GOCACHE=$cache_dir" GOTOOLCHAIN=local GOFLAGS=-p=1 GOMAXPROCS=2 go test -count=1 -timeout=180s ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java
run focused-race "$go_env go test -race -count=1 -timeout=180s ./internal/analyzernativebridge" env "GOCACHE=$cache_dir" GOTOOLCHAIN=local GOFLAGS=-p=1 GOMAXPROCS=2 go test -race -count=1 -timeout=180s ./internal/analyzernativebridge
run vet "$go_env go vet ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java" env "GOCACHE=$cache_dir" GOTOOLCHAIN=local GOFLAGS=-p=1 GOMAXPROCS=2 go vet ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java

for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/amd64; do
	goos=${target%/*}
	goarch=${target#*/}
	run "compile-$goos-$goarch" "CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch $go_env go test -exec=/usr/bin/true -run ^$ ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java" env CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" "GOCACHE=$cache_dir" GOTOOLCHAIN=local GOFLAGS=-p=1 GOMAXPROCS=2 go test -exec=/usr/bin/true -run '^$' ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java
done

if [ "$(git rev-parse HEAD)" != "$head" ] || [ "$(git rev-parse HEAD^{tree})" != "$tree" ] || [ -n "$(git status --porcelain --untracked-files=all)" ]; then
	output="$run_dir/snapshot-stability.out"
	printf '%s\n' 'native-bridge pre-review detected HEAD/tree/worktree drift' > "$output"
	retain_failure snapshot-stability 'git rev-parse HEAD; git status --porcelain --untracked-files=all' 1 "$output"
	append_result snapshot-stability 'git rev-parse HEAD; git status --porcelain --untracked-files=all' FAIL 1 "$output" "$failure_artifact_relative" "$failure_artifact_sha256"
	finish FAIL
	exit 1
fi

run diff-after 'git diff --check' git diff --check
finish PASS
