#!/usr/bin/env bash
# usage: check-goos-filenames.sh [REPO]
# Fails when a tracked Go file under cmd/ or internal/ carries a GOOS/GOARCH filename suffix that
# Go treats as a build constraint for a platform Corvint does not build for (for example
# `validity_js.go` builds only for GOOS=js, so the code silently disappears from every real build).
# Suffixes for darwin, linux, windows, amd64 and arm64 are intended platform files. Any other
# constrained name needs a line in script/goos-filename-allowlist.txt (V1-1910).
set -uo pipefail
export LC_ALL=C
repo=${1:-$(cd "$(dirname "$0")/.." && pwd)}
allowlist="$repo/script/goos-filename-allowlist.txt"
# Go's knownOS and knownArch lists (go/build/syslist.go).
known_os=" aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos "
known_arch=" 386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm "
allowed=" darwin linux windows amd64 arm64 "

files=$(git -C "$repo" ls-files -- 'cmd/*.go' 'internal/*.go') || { echo "check-goos-filenames: REFUSE git-ls-files-failed" >&2; exit 2; }
allowed_paths=
[[ -f $allowlist ]] && allowed_paths=$(grep -v '^[[:space:]]*\(#\|$\)' "$allowlist")
bad=0
while IFS= read -r path; do
  [[ -n $path ]] || continue
  name=${path##*/}
  name=${name%.go}
  name=${name%_test}
  [[ $name == *_* ]] || continue
  name=${name#*_}            # Go ignores the first element.
  IFS=_ read -r -a parts <<< "_$name"
  n=${#parts[@]}
  last=${parts[n - 1]}
  constraint=
  if ((n >= 3)) && [[ $known_os == *" ${parts[n - 2]} "* && $known_arch == *" $last "* ]]; then
    constraint="${parts[n - 2]} $last"
  elif [[ $known_os == *" $last "* || $known_arch == *" $last "* ]]; then
    constraint=$last
  fi
  [[ -n $constraint ]] || continue
  ok=1
  for element in $constraint; do [[ $allowed == *" $element "* ]] || ok=0; done
  ((ok)) && continue
  grep -qxF -- "$path" <<< "$allowed_paths" && continue
  suggestion=${path%.go}
  suggestion=${suggestion%_test}
  test_suffix=
  [[ ${path%.go} == *_test ]] && test_suffix=_test
  suggestion="${suggestion%_*}_${last}ts${test_suffix}.go"
  printf 'check-goos-filenames: %s builds only for %s (Go filename build constraint); rename it (for example %s) or list it in script/goos-filename-allowlist.txt\n' \
    "$path" "${constraint// //}" "$suggestion" >&2
  bad=1
done <<< "$files"
((bad == 0)) || exit 1
echo "check-goos-filenames: ok"
