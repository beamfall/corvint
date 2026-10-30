#!/bin/sh
set -eu

# Every Go file this repository tracks must be gofmt-clean. `go vet` is already in
# `make gate` and says nothing about formatting, so an unformatted file has been
# able to land and sit: `internal/contextindex/blob_shards_open_test.go` did, and
# was noticed only when an unrelated change happened to run `gofmt -l`.
#
# Scope is the tracked set, not a directory walk, for two reasons. A walk from the
# repository root descends into `.corvint-benchmark-cache/` and
# `extensions/vscode/node_modules/`, which hold thousands of Go files this
# repository neither owns nor formats; and the thing being gated is what gets
# committed, which is exactly what `git ls-files` names. The tracked set spans
# every module — the root, `interop/cem01-go`, `benchmarks/snapshot-reader` — with
# no list of modules to fall out of date.
#
# The formatter is the pinned toolchain's, not whichever `gofmt` is first on PATH,
# because gofmt's output changes between Go releases and a gate that depends on
# the developer's PATH is not a gate.
#
# Bytes come from the Git index, not the worktree. `git ls-files` alone still names the
# tracked set correctly, but handing gofmt those worktree paths means it reads whatever the
# worktree currently holds there: a tracked file with unstaged edits is judged on the edit,
# and a tracked file deleted from the worktree alone makes `xargs` fail instead of the gate
# reporting on it. `git write-tree` snapshots the current index (not HEAD, so a staged-but-
# uncommitted fix already counts) into a tree object, and `git archive` extracts exactly that
# tree to a scratch directory; gofmt then runs once against the tracked Go paths inside it, so
# the batched, self-naming `-l` behaviour is unchanged and every path it prints is still the
# real relative path the index carries.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
cd "$root"

goroot=$(GOTOOLCHAIN=local go env GOROOT)
gofmt="$goroot/bin/gofmt"
if [ ! -x "$gofmt" ]; then
    printf 'go format: no gofmt at %s (is the pinned Go toolchain installed?)\n' "$gofmt" >&2
    exit 1
fi

unmerged=$(git ls-files --unmerged)
if [ -n "$unmerged" ]; then
    printf 'go format: index has unmerged entries; resolve the conflict before running this gate\n' >&2
    exit 1
fi

tree=$(git write-tree)
workdir=$(mktemp -d "${TMPDIR:-/tmp}/corvint-go-format.XXXXXX")
listing=$(mktemp "${TMPDIR:-/tmp}/corvint-go-format-listing.XXXXXX")
trap 'rm -rf "$workdir" "$listing"' EXIT
git archive --format=tar "$tree" | tar -x -C "$workdir"

# The enumeration is captured and checked on its own: plain `sh` has no `pipefail`, so piping a
# failed `git ls-files` into `xargs` would hand gofmt no paths and report a clean tracked set.
if ! git ls-files -z -- '*.go' >"$listing"; then
    printf 'go format: git ls-files failed to enumerate the tracked Go files\n' >&2
    exit 1
fi

if ! unformatted=$(cd "$workdir" && xargs -0 "$gofmt" -l <"$listing" 2>&1); then
    printf 'go format: gofmt could not read the tracked Go files:\n%s\n' "$unformatted" >&2
    exit 1
fi

if [ -n "$unformatted" ]; then
    count=$(printf '%s\n' "$unformatted" | wc -l | tr -d ' ')
    printf 'go format: %s tracked Go file(s) are not gofmt-clean:\n' "$count" >&2
    printf '%s\n' "$unformatted" | while IFS= read -r path; do
        printf '  %s\n' "$path" >&2
    done
    printf "run: '%s' -w <file>\n" "$gofmt" >&2
    exit 1
fi
