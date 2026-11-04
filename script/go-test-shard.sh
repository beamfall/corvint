#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/go-test-shard.sh [--list-only] PACKAGE INDEX COUNT
# DTL-V0-007: a complete foreground partition of one package's ordinary Go tests.
# Example: script/go-test-shard.sh ./cmd/corvint/ 1 8
# Run each INDEX 1..COUNT against unchanged inputs; every name belongs to exactly one shard.
set -uo pipefail
list_only=0
if [[ ${1-} == --help || ${1-} == -h ]]; then sed -n '2,6p' "$0"; exit 0; fi
if [[ ${1-} == --list-only ]]; then list_only=1; shift; fi
if [[ $# != 3 || ! $1 =~ ^\./[A-Za-z0-9_./-]+/?$ || $1 == *..* || ! $2 =~ ^[1-9][0-9]*$ || ! $3 =~ ^[1-9][0-9]*$ ]] || (($2 > $3 || $3 > 64)); then
  echo 'go-test-shard: REFUSE invalid-arguments; fix: use script/go-test-shard.sh [--list-only] ./PACKAGE/ INDEX COUNT (1 <= INDEX <= COUNT <= 64)' >&2
  exit 2
fi
pkg=$1 index=$2 count=$3
here=$(cd "$(dirname "$0")" && pwd) || exit 2
# Inherited GOFLAGS (environment or go env file) must not change which names are listed or run.
# Both calls neutralize every selection flag with an explicit command-line value, which Go lets
# override GOFLAGS; a build or output setting (-tags, -mod, -race, ...) still applies. Any other
# inherited flag is refused before anything runs.
neutral=(-skip= -short=false -c=false -n=false -fuzz= -exec=)
goflags=$(GOTOOLCHAIN=local go env GOFLAGS) || { echo 'go-test-shard: REFUSE goflags-unreadable; fix: make GOTOOLCHAIN=local go env GOFLAGS succeed, then rerun' >&2; exit 2; }
read -ra words <<< "$goflags"
for word in ${words[@]+"${words[@]}"}; do
  name=${word#-}
  name=${name#-}
  name=${name%%=*}
  case $name in
    tags | mod | modfile | modcacherw | race | msan | asan | cover | covermode | coverpkg | trimpath | buildvcs | ldflags | gcflags | asmflags | p | pgo | overlay | compiler | installsuffix | linkshared | buildmode | work | x | v | vet | count | timeout | parallel | shuffle | failfast) ;;
    skip | list | short | c | n | fuzz | exec | run | test.skip | test.list | test.short | test.run | test.count | test.timeout | test.v | test.parallel | test.shuffle | test.failfast) ;;
    *)
      printf 'go-test-shard: REFUSE inherited-goflags-unsupported %s; fix: unset GOFLAGS or remove %s from it (go env -u GOFLAGS for a go env file value), keeping build settings such as -tags, -mod and -race\n' "$word" "$word" >&2
      exit 2
      ;;
  esac
done
inventory=$("$here/heavy.sh" env GOTOOLCHAIN=local go test -count=1 -timeout 30m "${neutral[@]}" -list '^(Test|Example|Fuzz)' "$pkg") || exit $?
names=$(printf '%s\n' "$inventory" | LC_ALL=C sort -u | awk '/^(Test|Example|Fuzz)[^[:space:]]*$/ { print }')
[[ -n $names ]] || { echo 'go-test-shard: REFUSE empty-inventory; fix: run the package whole and inspect go test -list output' >&2; exit 2; }
# Test identifiers contain no regex metacharacters; reject unknown identifier shapes.
pattern=$(printf '%s\n' "$names" | perl -CSD -e '
  use strict; use warnings;
  my ($index,$count)=@ARGV; my @names=<STDIN>; chomp @names;
  die "invalid test identifier\n" if grep { !/\A(?:Test|Example|Fuzz)\w*\z/u } @names;
  my @selected=map { $names[$_] } grep { $_ % $count == $index-1 } 0..$#names;
  print @selected ? "^(".join("|",@selected).")\$" : "^\$";
' "$index" "$count") || exit 2
printf 'go-test-shard: package=%s shard=%s/%s inventory=%s pattern=%s\n' "$pkg" "$index" "$count" "$(wc -l <<< "$names" | tr -d ' ')" "$pattern" >&2
if ((list_only)); then printf '%s\n' "$pattern"; exit 0; fi
# ^$ is Go's explicit no-match pattern; an empty shard is observable.
[[ $pattern != '^$' ]] || { echo 'go-test-shard: no selected tests'; exit 0; }
exec "$here/heavy.sh" env GOTOOLCHAIN=local go test -count=1 -timeout 30m "${neutral[@]}" -list= -run "$pattern" "$pkg"
