# Sourced by the shell tests that assert with ripgrep; not executable on its own (DCW-V0-811).
#
# test_rg_fallback: when rg is on PATH it does nothing. Otherwise it writes a stand-in `rg` over GNU
# grep into a new temporary directory, prepends that directory to PATH (so fake binaries the test
# writes find it too) and sets rg_fallback_dir for the caller's cleanup. The stand-in admits only
# the subset these tests use, short flags drawn from F, x, q and c, an optional `--`, one pattern and
# at most one file, reading a non-fixed pattern as a PCRE (grep -P), as ripgrep reads its regex
# syntax for these patterns. Any other argument exits 2 naming it, so a new flag fails loudly
# instead of being misread. One known difference: -c prints 0 for no match where ripgrep prints
# nothing; both exit 1. It returns 1, writing nothing, when grep has no -P (BSD grep), and the
# caller keeps its unsupported-environment-missing-rg refusal.
rg_fallback_dir=
test_rg_fallback() {
  command -v rg > /dev/null 2>&1 && return 0
  printf 'x\n' | grep -qP '^x$' 2> /dev/null || return 1
  rg_fallback_dir=$(mktemp -d "${TMPDIR:-/tmp}/corvint-rg-fallback.XXXXXX") || return 1
  test_rg_write_stand_in "$rg_fallback_dir" || return 1
  PATH="$rg_fallback_dir:$PATH"
  export PATH
  printf '%s: NOTE rg not on PATH; asserting with the grep -P stand-in at %s (DCW-V0-811)\n' "${0##*/}" "$rg_fallback_dir/rg" >&2
}

# test_rg_write_stand_in DIR writes the stand-in as DIR/rg; script/test-rg-fallback_test.sh calls it
# directly, so the stand-in is checked on hosts that have ripgrep too.
test_rg_write_stand_in() {
  cat > "$1/rg" << 'SHIM'
#!/usr/bin/env bash
# Test-only ripgrep stand-in written by script/test-rg-fallback.sh (DCW-V0-811).
flags=() fixed=0
while (($#)); do
  case $1 in
    --) shift; break ;;
    -[Fxqc]*)
      [[ $1 =~ ^-[Fxqc]+$ ]] || { printf 'rg stand-in: unsupported argument %q\n' "$1" >&2; exit 2; }
      [[ $1 == *F* ]] && fixed=1
      flags+=("$1"); shift ;;
    -*) printf 'rg stand-in: unsupported argument %q\n' "$1" >&2; exit 2 ;;
    *) break ;;
  esac
done
(($# == 1 || $# == 2)) || { printf 'rg stand-in: expected PATTERN [FILE], got %d arguments\n' "$#" >&2; exit 2; }
if ((fixed)); then exec grep "${flags[@]}" -- "$@"; fi
exec grep -P "${flags[@]}" -- "$@"
SHIM
  chmod +x "$1/rg"
}
