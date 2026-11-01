# Sourced by Corvint's dogfood wrappers (DCW-V0-022, DCW-V0-074); not executable on its own.
# Builds ./cmd/corvint once per recorded input key and reuses that build while the key matches.
# The caller sets `repo` (the repository top level) and `go_cache` before calling.
#
# The key names the build: this scheme, the build flags, `go env` for the toolchain and target,
# and the Git objects of every build input. Every build runs with -trimpath and -buildvcs=false,
# so its bytes depend on those inputs alone. Any failure while computing, reading or storing a key
# falls back to the plain build: a reuse is never required for the wrapper to succeed.
# ASD-V0-070 (V1-3233): with no HOME the reuse cache is off; the plain build still runs.
# DCW-V0-943 (V1-3627): the cache is per user and host, outside every repository, so worktrees on one
# BASE share the base verifier. It keeps the CORVINT_DOGFOOD_BIN_CACHE_MAX (default 16) most
# recently stored or reused entries; concurrent wrappers need no lock, since every store renames
# private temporary files into place and every reuse verifies the bytes it copied.
# dogfood_bin_outcome names how the last dogfood_build_*_corvint call got its binary: reuse or
# build (DCW-V0-942).
dogfood_bin_outcome=
dogfood_bin_cache=${CORVINT_DOGFOOD_BIN_CACHE:-${XDG_CACHE_HOME:-${HOME:+$HOME/.cache}}}
dogfood_bin_cache=${dogfood_bin_cache:+${CORVINT_DOGFOOD_BIN_CACHE:-$dogfood_bin_cache/corvint-dogfood-bin}}

# dogfood_require_full_history NAME: ASD-V0-072 (V1-3230). A shallow clone can hide the base or the
# merge base of the dogfood range, so the wrapper refuses it up front, naming the recovery; a
# directory Git cannot inspect is left to the wrapper's own checks.
dogfood_require_full_history() {
  local shallow
  shallow=$(git -C "$repo" rev-parse --is-shallow-repository 2> /dev/null) || return 0
  [[ $shallow == true ]] || return 0
  printf '%s: REFUSE shallow-repository: %s is a shallow clone, so a base commit or merge base may lie past its shallow boundary; run `git -C %q fetch --unshallow origin` (or reclone without --depth), then rerun\n' "$1" "$repo" "$repo" >&2
  return 2
}

# GOFLAGS comes from `go env`, so flags persisted with `go env -w` are kept like a plain build.
dogfood_goflags=$(GOTOOLCHAIN=local go env GOFLAGS 2> /dev/null) || dogfood_goflags=
dogfood_goflags="${dogfood_goflags:+$dogfood_goflags }-buildvcs=false"

# dogfood_version_matches OUTPUT EXPECTED: OUTPUT is one `corvint --version` line naming VERSION
# EXPECTED or its development form (1.0.0-rc.3.dev, 1.0.0-dev), read by its prefix: the
# parenthesised members (build, commit, modified) may grow and are not matched (PUB-V0-053).
dogfood_version_matches() {
  local line=$1 expected=$2 development
  if [[ $expected == *-* ]]; then development="$expected.dev"; else development="$expected-dev"; fi
  [[ $line != *$'\n'* && $line =~ ^"Corvint "("$expected"|"$development")" ("[^\(\)]+")"$ ]]
}

dogfood_go() {
  GOCACHE="$go_cache" GOTOOLCHAIN=local GOFLAGS="$dogfood_goflags" go "$@"
}

# dogfood_build DIR OUT builds ./cmd/corvint in DIR into OUT with the one flag set every dogfood
# wrapper uses. A failed build keeps the full compiler output in a log file and prints its last
# lines and the log's path on stderr (V1-1893); a successful build removes the log.
dogfood_build() {
  local dir=$1 out=$2 log
  log=$(mktemp "${TMPDIR:-/tmp}/corvint-dogfood-build.XXXXXX") || return 1
  if (cd "$dir" && dogfood_go build -o "$out" -trimpath ./cmd/corvint) > "$log" 2>&1; then
    /bin/rm -f "$log"
    return 0
  fi
  {
    printf 'dogfood: go build ./cmd/corvint failed; last lines:\n'
    tail -n 20 "$log"
    printf 'dogfood: full build log: %s\n' "$log"
  } >&2
  return 1
}

# dogfood_corvint_key REV [SOURCE] prints the key's object ID, or fails.
# Without SOURCE the build source is an archive of REV, and REV's root tree names every input.
# With SOURCE (the worktree, checked out at REV) the inputs are go.mod, go.sum, go.work, vendor,
# VERSION and the directory of every main-module package `go list -deps ./cmd/corvint` reports;
# the key names REV's objects at those paths and fails unless the paths are clean in SOURCE,
# counting staged, untracked and ignored files.
dogfood_corvint_key() {
  local rev=$1 source=${2-} environment listing line paths=() objects optional status workspace
  environment=$(cd "${source:-$repo}" && dogfood_go env GOVERSION GOOS GOARCH CGO_ENABLED GOFLAGS \
    GOEXPERIMENT GOAMD64 GOARM64 GOARM GO386 GOMIPS GOMIPS64 GOPPC64 GORISCV64 GOWASM GOFIPS140 \
    CC CXX AR PKG_CONFIG CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_FFLAGS CGO_LDFLAGS 2> /dev/null) || return 1
  # A workspace file outside the keyed objects (an ancestor go.work or GOWORK) leaves no key.
  workspace=$(cd "${source:-$repo}" && dogfood_go env GOWORK 2> /dev/null) || return 1
  [[ -z $workspace || $workspace == off || ( -n $source && $workspace == "$source/go.work" ) ]] || return 1
  if [[ -z $source ]]; then
    objects=$(git -C "$repo" rev-parse --verify -q "$rev^{tree}") || return 1
  else
    listing=$(cd "$source" && dogfood_go list -deps \
      -f '{{if not .Standard}}{{.Module.Main}} {{.Module.Path}} {{.ImportPath}}{{end}}' ./cmd/corvint 2> /dev/null) ||
      return 1
    paths=(VERSION)
    while IFS= read -r line; do
      [[ -n $line ]] || continue
      set -- $line
      [[ $# == 3 && $1 == true && $3 == "$2"/* ]] || return 1
      paths+=("${3#"$2"/}")
    done <<< "$listing"
    ((${#paths[@]} > 1)) || return 1
    objects=$(git -C "$source" rev-parse "${paths[@]/#/$rev:}" 2> /dev/null) || return 1
    optional=$(git -C "$source" ls-tree "$rev" -- go.mod go.sum go.work vendor) || return 1
    [[ $optional == *$'\tgo.mod'* ]] || return 1
    status=$(git -C "$source" status --porcelain --ignored --untracked-files=all -- \
      "${paths[@]}" go.mod go.sum go.work vendor) || return 1
    [[ -z $status ]] || return 1
    objects+=$'\n'"$optional"$'\n'"${paths[*]}"
  fi
  printf 'corvint-dogfood-bin/1\nbuild -trimpath ./cmd/corvint\n%s\n%s\n' "$environment" "$objects" |
    git -C "$repo" hash-object --stdin
}

# dogfood_corvint_reuse OUT KEY copies the verified cached build for KEY to OUT, or fails.
dogfood_corvint_reuse() {
  local out=$1 key=$2 entry="$dogfood_bin_cache/$2" expected temporary
  [[ -n $dogfood_bin_cache && -n $key && -f $entry/corvint && -f $entry/corvint.oid ]] || return 1
  expected=$(cat "$entry/corvint.oid" 2> /dev/null) || return 1
  temporary=$(mktemp "$out.XXXXXX") || return 1
  if /bin/cp -f "$entry/corvint" "$temporary" && chmod 755 "$temporary" &&
    [[ -n $expected && $(git -C "$repo" hash-object "$temporary") == "$expected" ]] &&
    /bin/mv -f "$temporary" "$out"; then
    touch -- "$entry" 2> /dev/null || :
    return 0
  fi
  /bin/rm -f "$temporary"
  return 1
}

# dogfood_corvint_store OUT KEY records OUT as the build for KEY; a failure only skips the record.
dogfood_corvint_store() {
  local out=$1 key=$2 entry="$dogfood_bin_cache/$2" binary record
  [[ -n $dogfood_bin_cache && -n $key ]] || return 0
  mkdir -p "$entry" 2> /dev/null || return 0
  binary=$(mktemp "$entry/.corvint.XXXXXX") || return 0
  record=$(mktemp "$entry/.corvint.oid.XXXXXX") || { /bin/rm -f "$binary"; return 0; }
  if /bin/cp -f "$out" "$binary" && chmod 755 "$binary" &&
    git -C "$repo" hash-object "$binary" > "$record" &&
    /bin/mv -f "$binary" "$entry/corvint" && /bin/mv -f "$record" "$entry/corvint.oid"; then
    touch -- "$entry" 2> /dev/null || :
    dogfood_corvint_prune
    return 0
  fi
  /bin/rm -f "$binary" "$record"
  return 0
}

# dogfood_corvint_prune removes every keyed entry past the CORVINT_DOGFOOD_BIN_CACHE_MAX most
# recent (DCW-V0-943). An entry another wrapper is copying or storing may go; that wrapper's
# verification or rename then fails and it builds instead. A failure only skips the prune.
dogfood_corvint_prune() {
  local max=${CORVINT_DOGFOOD_BIN_CACHE_MAX:-16} name n=0
  [[ -n $dogfood_bin_cache && $max =~ ^[1-9][0-9]*$ ]] || return 0
  while IFS= read -r name; do
    [[ $name =~ ^[0-9a-f]{40,64}$ ]] || continue
    n=$((n + 1))
    ((n > max)) || continue
    /bin/rm -rf -- "${dogfood_bin_cache:?}/$name" 2> /dev/null || :
  done < <(ls -t -- "$dogfood_bin_cache" 2> /dev/null)
  return 0
}

# dogfood_build_tree_corvint OUT builds the worktree's corvint into OUT, reusing a recorded build
# when HEAD's inputs are clean and keyed. The key is recomputed after a build and recorded only
# when it is unchanged, so an input edited during the build is never recorded. Fails on a failed
# build.
dogfood_build_tree_corvint() {
  local out=$1 key
  key=$(dogfood_corvint_key HEAD "$repo") || key=
  dogfood_bin_outcome=reuse
  dogfood_corvint_reuse "$out" "$key" && return 0
  dogfood_bin_outcome=build
  dogfood_build "$repo" "$out" || return 1
  [[ -n $key && $(dogfood_corvint_key HEAD "$repo") == "$key" ]] || return 0
  dogfood_corvint_store "$out" "$key"
}

# dogfood_build_base_corvint OUT BASE builds an archive of commit BASE into OUT, reusing a
# recorded build of the same root tree without extracting it. Exit 2: the archive could not be
# extracted; exit 1: the build failed.
dogfood_build_base_corvint() {
  local out=$1 base=$2 key status=0
  key=$(dogfood_corvint_key "$base") || key=
  dogfood_bin_outcome=reuse
  dogfood_corvint_reuse "$out" "$key" && return 0
  dogfood_bin_outcome=build
  dogfood_base_tree=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-base.XXXXXX") || return 2
  trap 'rm -rf "$dogfood_base_tree"' EXIT
  if ! git -C "$repo" archive --format=tar "$base" | tar -xf - -C "$dogfood_base_tree"; then
    status=2
  elif ! dogfood_build "$dogfood_base_tree" "$out"; then
    status=1
  fi
  rm -rf "$dogfood_base_tree"
  trap - EXIT
  ((status == 0)) || return "$status"
  dogfood_corvint_store "$out" "$key"
}

# dogfood_build_fix PREFIX tree|base prints the recovery line after a build-failed refusal
# (DCW-V0-332, V1-1929). A failing tree of a batch usually means its sealed constituents do not
# compose; a failing base cannot carry a seal.
dogfood_build_fix() {
  if [[ $2 == base ]]; then
    printf '%s: fix: BASE does not build, so no change can be sealed on it; rerun with the last building origin/main commit as BASE, or repair main first (docs/DOGFOOD.md section 4)\n' "$1" >&2
  else
    printf '%s: fix: repair the compile error above and rerun; in a merge batch whose constituents build alone, they do not compose: rebase the later constituent'"'"'s work commits onto the batch head, fix it there, reseal it with BASE=<batch head>, then merge it (docs/DOGFOOD.md "Non-composing constituents")\n' "$1" >&2
  fi
}
