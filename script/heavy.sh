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
#
# A slot is an flock(2) on HEAVY_LOCK_DIR/slotN held by this wrapper while the command runs. The
# kernel releases it when the holder exits for any reason, so a dead owner never strands a slot and
# no reclaim step can race a live owner. The wrapper waits quietly, prints one line to stderr when
# admitted (slot and wait seconds), forwards INT/TERM/HUP to the command and exits with its status.
set -uo pipefail

if [[ $# -eq 0 || $1 == -h || $1 == --help ]]; then
  sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
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
lock_dir=${HEAVY_LOCK_DIR:-/tmp/corvint-heavy-$(id -u)}
if ! mkdir -p "$lock_dir" 2> /dev/null; then
  printf 'heavy.sh: REFUSE lock-dir-unavailable %s\n' "$lock_dir" >&2
  exit 2
fi
case " ${GOFLAGS-} " in
  *" -p="* | *" -p "*) ;;
  *) export GOFLAGS="${GOFLAGS:+$GOFLAGS }-p=$par" ;;
esac

exec perl -e '
use strict;
use warnings;
use Fcntl qw(:flock);
use POSIX ();
my ($dir, $n, @cmd) = @ARGV;
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
my $pid = fork;
die "heavy.sh: fork failed: $!\n" unless defined $pid;
if ($pid == 0) {
    close $lock;
    no warnings "exec";
    exec { $cmd[0] } @cmd or print STDERR "heavy.sh: cannot run $cmd[0]: $!\n";
    POSIX::_exit(127);
}
$SIG{$_} = do { my $s = $_; sub { kill $s, $pid } } for qw(INT TERM HUP);
my $got;
do { $got = waitpid($pid, 0) } until $got == $pid || $got == -1;
my $status = $?;
exit($status & 127 ? 128 + ($status & 127) : $status >> 8);
' "$lock_dir" "$slots" "$@"
