#!/usr/bin/env bash
# usage: script/heavy.sh COMMAND [ARG...]
#
# Runs one heavy Go command (go test, go build ./..., go vet, a make target that builds) under a
# host-wide counting semaphore, so concurrent lanes queue instead of thrashing the host
# (ASD-V0-014, docs/LANE-ORCHESTRATION.md).
#
#   HEAVY_SLOTS     concurrent heavy commands admitted host-wide; default max(1, CPUs/4)
#   HEAVY_P         Go build/test parallelism (-p) exported through GOFLAGS unless GOFLAGS already
#                   sets -p; default max(1, CPUs/HEAVY_SLOTS). An explicit -p argument still wins.
#   HEAVY_LOCK_DIR  directory holding the slot lock files; default /tmp/corvint-heavy-<uid>.
#                   Every lane on the host must use the same directory and the same HEAVY_SLOTS.
#   HEAVY_NICE      scheduling niceness the command runs at (0..19); default 10, so hook adapters
#                   keep their CPU share under lane test load (AHI-199). 0 keeps the caller's
#                   priority; a caller already nicer than HEAVY_NICE is never raised.
#
# Pinned Go: the repository root's go.mod (else the nearest one) names the pinned release in its
# toolchain line, else its go line. When the go on PATH reports another release and the pinned one
# is installed in $HOME/sdk/goX or the module cache (golang.org/toolchain@v0.0.1-goX.OS-ARCH), its
# bin goes first on PATH, so GOTOOLCHAIN=local commands run it; otherwise one warning line names
# the go that will run.
#
# A slot is an flock(2) on HEAVY_LOCK_DIR/slotN. The command inherits the locked descriptor, so
# the slot stays held while any workload process that kept it runs, even if the wrapper is killed.
# The kernel releases it when the last holder exits, so a dead owner never strands a slot and no
# reclaim step can race a live owner. The command runs in its own process group. The wrapper waits
# quietly, prints one line to stderr when admitted (slot and wait seconds), forwards INT/TERM/HUP
# to the command's whole process group and exits with the command's status.
set -uo pipefail

if [[ $# -eq 0 || $1 == -h || $1 == --help ]]; then
  sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'
  [[ $# -eq 0 ]] && exit 2
  exit 0
fi

cpus=$(getconf _NPROCESSORS_ONLN 2> /dev/null || sysctl -n hw.ncpu 2> /dev/null || echo 4)
[[ $cpus =~ ^[1-9][0-9]*$ ]] || cpus=4
slots=${HEAVY_SLOTS:-$((cpus / 4 > 0 ? cpus / 4 : 1))}
if ! [[ $slots =~ ^[1-9][0-9]*$ ]] || ((slots > 64)); then
  printf 'heavy.sh: REFUSE invalid-slots HEAVY_SLOTS=%s (want 1..64)\n' "$slots" >&2
  exit 2
fi
par=${HEAVY_P:-$((cpus / slots > 0 ? cpus / slots : 1))}
if ! [[ $par =~ ^[1-9][0-9]*$ ]] || ((par > cpus)); then
  printf 'heavy.sh: REFUSE invalid-parallelism HEAVY_P=%s (want 1..%s)\n' "$par" "$cpus" >&2
  exit 2
fi
nice_level=${HEAVY_NICE:-10}
if ! [[ $nice_level =~ ^(0|[1-9][0-9]?)$ ]] || ((nice_level > 19)); then
  printf 'heavy.sh: REFUSE invalid-nice HEAVY_NICE=%s (want 0..19)\n' "$nice_level" >&2
  exit 2
fi
lock_dir=${HEAVY_LOCK_DIR:-/tmp/corvint-heavy-$(id -u)}
if ! mkdir -p "$lock_dir" 2> /dev/null; then
  printf 'heavy.sh: REFUSE lock-dir-unavailable %s\n' "$lock_dir" >&2
  exit 2
fi
pin_go() {
  local d want have cache os arch c
  d=$(git rev-parse --show-toplevel 2> /dev/null)
  if [[ -z $d || ! -f $d/go.mod ]]; then
    d=$PWD
    until [[ -f $d/go.mod || $d == / ]]; do d=$(dirname "$d"); done
  fi
  [[ -f $d/go.mod ]] && command -v go > /dev/null || return 0
  want=$(sed -n 's/^toolchain go\([0-9][0-9.]*\)[[:space:]]*$/\1/p' "$d/go.mod" | head -n 1)
  [[ -n $want ]] || want=$(sed -n 's/^go \([0-9][0-9.]*\)[[:space:]]*$/\1/p' "$d/go.mod" | head -n 1)
  [[ $want =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 0 # a language-only line pins no release
  { read -r have; read -r cache; read -r os; read -r arch; } < <(GOTOOLCHAIN=local go env GOVERSION GOMODCACHE GOOS GOARCH 2> /dev/null)
  [[ ${have-} == "go$want" ]] && return 0
  for c in "$HOME/sdk/go$want/bin" "${cache-}/golang.org/toolchain@v0.0.1-go$want.${os-}-${arch-}/bin"; do
    if [[ -x $c/go && $(GOTOOLCHAIN=local "$c/go" env GOVERSION 2> /dev/null) == "go$want" ]]; then
      export PATH="$c:$PATH"
      return 0
    fi
  done
  printf 'heavy.sh: pinned go%s not installed; running %s\n' "$want" "${have:-an unknown go}" >&2
}
pin_go
case " ${GOFLAGS-} " in
  *" -p="* | *" -p "*) ;;
  *) export GOFLAGS="${GOFLAGS:+$GOFLAGS }-p=$par" ;;
esac

exec perl -e '
use strict;
use warnings;
use Fcntl qw(:flock F_SETFD);
use POSIX qw(setpgid);
my ($dir, $n, $nice, @cmd) = @ARGV;
my $t0 = time;
my ($lock, $slot);
until ($lock) {
    for my $i (1 .. $n) {
        open(my $fh, ">>", "$dir/slot$i") or die "heavy.sh: REFUSE lock-open-failed $dir/slot$i: $!\n";
        if (flock($fh, LOCK_EX | LOCK_NB)) { ($lock, $slot) = ($fh, $i); last }
        close $fh;
    }
    select(undef, undef, undef, 1) unless $lock;
}
truncate($lock, 0);
syswrite($lock, "$$\n");
printf STDERR "heavy.sh: slot %d/%d after %ds\n", $slot, $n, time - $t0;
# The command inherits the locked descriptor, so the slot stays held until every process that
# kept it has exited, even when the wrapper itself is killed.
fcntl($lock, F_SETFD, 0) or die "heavy.sh: fcntl failed: $!\n";
my $pid = fork;
die "heavy.sh: fork failed: $!\n" unless defined $pid;
if ($pid == 0) {
    setpgid(0, 0);
    setpriority(0, 0, $nice) if $nice > getpriority(0, 0);
    no warnings "exec";
    exec { $cmd[0] } @cmd or print STDERR "heavy.sh: cannot run $cmd[0]: $!\n";
    POSIX::_exit(127);
}
setpgid($pid, $pid);
# The command runs in its own process group; a forwarded signal reaches the whole group.
$SIG{$_} = do { my $s = $_; sub { kill $s, -$pid } } for qw(INT TERM HUP);
my $got;
do { $got = waitpid($pid, 0) } until $got == $pid || ($got == -1 && !$!{EINTR});
my $status = $?;
exit($status & 127 ? 128 + ($status & 127) : $status >> 8);
' "$lock_dir" "$slots" "$nice_level" "$@"
