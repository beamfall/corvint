#!/usr/bin/env bash
# usage: script/heavy.sh [NAME=VALUE...] COMMAND [ARG...]
#        script/heavy.sh --print-lock-dir
#
# --print-lock-dir prints the slot lock directory a run would use (HEAVY_LOCK_DIR, else HEAVY_CONFIG,
# else the default below), exactly as given (a relative one is relative to the working directory),
# and exits 0 without creating it or taking a slot (DCW-V0-357).
#
# Runs one heavy Go command (go test, go build ./..., go vet, a make target that builds) under a
# host-wide counting semaphore, so concurrent lanes queue instead of thrashing the host
# (ASD-V0-014, docs/LANE-ORCHESTRATION.md). A `go test`, `go vet` or `make windows-vet` command may
# instead run on a configured remote Linux host (ASD-V0-028).
#
#   HEAVY_SLOTS     concurrent heavy commands admitted host-wide; default max(1, CPUs/4)
#   HEAVY_P         Go build/test parallelism (-p) exported through GOFLAGS unless GOFLAGS already
#                   sets -p; default max(1, CPUs/HEAVY_SLOTS). An explicit -p argument still wins.
#   HEAVY_LOCK_DIR  directory holding the slot lock files; default /tmp/corvint-heavy-<uid>.
#                   Every lane on the host must use the same directory and the same HEAVY_SLOTS.
#   HEAVY_NICE      scheduling niceness the command runs at (0..19); default 10, so hook adapters
#                   keep their CPU share under lane test load (AHI-199). 0 keeps the caller's
#                   priority; a caller already nicer than HEAVY_NICE is never raised.
#   HEAVY_CONFIG    optional KEY=VALUE file read for any HEAVY_* setting the environment leaves
#                   unset; default ${XDG_CONFIG_HOME:-$HOME/.config}/corvint/heavy.env.
#
# Lane policy (ASD-V0-030, ported from the lane prototype so its cutover loses nothing):
#   HEAVY_WHOLE_REASON   a `go test` without `-run` (a whole package) is refused with exit 2 unless
#                        this names why, for example "corvint selected the package whole" or Corvint's
#                        FALLBACK/UNKNOWN reason. Select units with `corvint test --base SHA --dry-run`.
#   HEAVY_SMALL_TEST_BYTES  a whole-package `go test` whose every package is a plain local directory
#                        under tools/ or conformance/ (no `...`) and whose *_test.go files total at
#                        most this many bytes runs without HEAVY_WHOLE_REASON, logged as
#                        `whole reason=small-package:NB` (ASD-V0-074, V1-3120); default 49152. On
#                        2026-10-30 the six such packages measured (4-45 KB of tests) each ran whole
#                        in 0.6-16 s; cmd/ and internal/ packages are never exempt. 0 disables it.
#   HEAVY_SELECTION      a `corvint test --base SHA --dry-run --manifest M` manifest of HEAD; a
#                        whole-package go test whose every ./REL package M selects whole needs no
#                        HEAVY_WHOLE_REASON, logged `whole reason=corvint-selection:HEX12` (ASD-V0-891).
#                        `script/heavy.sh --plan M` prints one such command per unit of M, runs none.
#                        A relative .git/REL resolves to Git's path (linked worktrees); when M
#                        admits nothing, one `HEAVY_SELECTION=M not admitted: REASON` line says why
#                        (ASD-V0-722). A `go test -c` only compiles and needs no reason (ASD-V0-721).
#   HEAVY_SELECTION_LOG  every `go test` appends `UTC selected|whole reason=Q|refused pwd=... cmd=...`
#                        here; default HEAVY_LOCK_DIR/selection.log.
#   HEAVY_LOCAL_REASON   with HEAVY_REMOTE=0, a go test, go vet or make windows-vet is refused with exit
#                        2 (logged as reason=refused-local) unless this names why it needs this host.
#   HEAVY_MAX_LOAD       a local run first waits while the integer 1-minute load average exceeds this
#                        (default 10), checking every 5 s, for at most HEAVY_LOAD_WAIT seconds
#                        (default 120); an unreadable load average does not wait.
#
# Run telemetry (ASD-V0-037, V1-2905). This file is the one runner: its remote half is the same file,
# shipped over ssh on every offload, so no copy lives on a remote host. Every run prints
# `heavy.sh: script sha256=HASH PATH` to stderr, and every selection-log line ends with
# `script=HASH12` (its first 12 hex digits), so a log line names the exact runner that wrote it.
# The selection log also gets, per run:
#   UTC offload host=H pack=Ns remote=Ns rc=N [receive=Ns wait=Ns prepare=Ns run=Ns worktree=W] ...
#       each offload attempt; the bracketed fields are the remote's phase line when it printed one.
#   UTC fallback host=H ...   an offload that stayed unavailable after its retry and ran locally.
#   UTC local wait=Ns run=Ns rc=N ...   each local run, after the command exits: slot wait, run
#       wall time, status.
#   UTC alert remote-unavailable streak=N host=H ...   when the last HEAVY_FALLBACK_ALERT (default
#       3; 0 disables) fallbacks happened with no successful offload between them (an offload with
#       any status other than 97 or ssh's 255 ends a streak); the same text goes to stderr.
#
# Remote offload (ASD-V0-028), only when HEAVY_REMOTE_HOST is set and the command contains `go test`,
# `go vet` or `make windows-vet`:
#   HEAVY_REMOTE_HOST     ssh destination; unset or empty means every command runs locally.
#   HEAVY_REMOTE          0 runs this command locally (darwin-specific tests); default 1.
#   Darwin-only files (V1-2552): before each offload of a go test or go vet, the package arguments
#   that are local paths (`./x`, `./x/...`, `.`) are scanned for Go files the Linux remote neither
#   builds nor runs: a `_darwin.go`, `_darwin_test.go` or `_darwin_<arch>` name, or a `//go:build`
#   line that requires darwin (`darwin` alone or joined only by `&&`). When any exist, one stderr
#   line and a `darwin-only` selection-log line name the count and the first file, so such a run is
#   never offloaded silently; HEAVY_REMOTE=0 with HEAVY_LOCAL_REASON runs them on this host.
#   HEAVY_REMOTE_RETRY    seconds before the one retry of an unavailable offload; default 15.
#   HEAVY_FALLBACK_LOG    file every local run of an offloadable command is appended to (UTC time,
#                         pwd, reason, command); default HEAVY_LOCK_DIR/fallback.log.
#   HEAVY_REMOTE_SHELL    remote command that runs bash on the script it reads from stdin; default
#                         `bash -s` (a Windows host with WSL: `wsl -d Ubuntu -- bash -s; exit $LASTEXITCODE`).
#   HEAVY_REMOTE_CLONE    remote clone of this repository's origin; default $HOME/corvint.
#   HEAVY_REMOTE_ROOT     remote scratch and lock root; default $HOME/.cache/corvint-heavy.
#   HEAVY_REMOTE_SLOTS    remote concurrent commands; default max(1, remote CPUs/4).
#   HEAVY_REMOTE_GOMAXPROCS  GOMAXPROCS the remote command runs with, so each slot's test binaries
#                         stay within their share of the remote CPUs (V1-1628); default the remote
#                         -p, max(1, remote CPUs/slots); 0 sets none. A GOMAXPROCS the remote
#                         environment file sets, or a leading GOMAXPROCS=N word, still wins.
#   HEAVY_REMOTE_ENV      remote file sourced first (for example one putting the pinned go on PATH).
#   HEAVY_REMOTE_TIMEOUT  bound on the remote command's run once it holds a remote slot, enforced with
#                         timeout(1), which the remote must have; the slot wait is not bounded; default 40m.
#   HEAVY_REMOTE_BUSY_PROCESS  process name (pgrep -x) whose presence on the remote makes the offload
#                         unavailable (exit 97, so the command runs locally after the retry); for a box
#                         that also hosts CI runners, Runner.Worker. Unset: never busy.
#   HEAVY_CI_REPO         owner/name of the GitHub repository whose Actions runs keep work off the
#                         remote (ASD-V0-040); default this checkout's origin when it is on github.com;
#                         `off` disables the guard.
#   HEAVY_CI_GH           the gh command that answers the query; default gh.
#   HEAVY_CI_STATUSES     run statuses that count as CI running; default "queued in_progress".
#   HEAVY_CI_TIMEOUT      seconds each query may take; default 20.
#   HEAVY_CI_GUARD        auto (default) skips the guard while the CI repository's Actions variable
#                         CORVINT_SELF_HOSTED reads `off` (CI on GitHub-hosted runners); on always
#                         guards; off never guards.
#   HEAVY_BUILD_OFFLOAD   0 keeps builds local; default 1 (ASD-V0-041).
#   HEAVY_REMOTE_BUILD_MAX_LOAD  the remote's 1-minute load above which a build stays local; default 16.
# CI guard (ASD-V0-040, owner rule of 2026-10-28): the remote's runners are the CI, so before any
# offload (test, vet or build) the wrapper asks `gh run list --repo HEAVY_CI_REPO --status S --limit 5
# --json databaseId --jq length` for each status S. Any run, a failed or non-numeric answer or a query
# that outlives HEAVY_CI_TIMEOUT keeps the work local: one stderr line, a `ci-running` fallback-log
# line and a `ci-busy` selection-log line (not a fallback for the alert streak), and no
# HEAVY_LOCAL_REASON is needed. With no repository (HEAVY_CI_REPO=off, or unset with a non-GitHub
# origin) nothing is queried. Under HEAVY_CI_GUARD=auto the wrapper first asks `gh variable get
# CORVINT_SELF_HOSTED --repo HEAVY_CI_REPO`, bounded by HEAVY_CI_TIMEOUT: the answer `off` means CI
# runs on GitHub-hosted runners, not the remote, so the run query is skipped and the work offloads;
# any other answer, a failure or a timeout leaves the guard in force (fail closed).
# Build offload (ASD-V0-041): `go build`, `make ... build` and `make ... tasks-build` cross-compile on
# the remote for this host's GOOS/GOARCH (Go's cross-build default CGO_ENABLED=0) in one try, only
# while the remote's load is at most HEAVY_REMOTE_BUILD_MAX_LOAD and no CI runs. Absolute outputs
# (-o P, -o=P, NAME_BIN=P) are built under the remote worktree and installed at P; relative ones
# land at the same checkout path. Every file the build wrote under the worktree is copied back (a
# tracked path only when it still holds its tested content; ignored outputs are overwritten, as a
# local build would). A busy, unavailable or failed copy-back (exit 96 or 97), an argument with a
# .. component or a file-writing GOFLAGS builds locally with one `build-local` log line each.
# The remote runs the command at this checkout's exact tree (HEAD as a bundle over its origin/main
# merge base, plus the uncommitted diff and untracked files under 50 MB) in a detached worktree
# of the remote slot (HEAVY_REMOTE_ROOT/wt/heavy-slotN, kept and reset on each run so the Go build
# cache hits; V1-2819), admitted through the same flock semaphore under HEAVY_REMOTE_ROOT, and
# prints `heavy.sh: phase receive= wait= prepare= run= worktree=` wall-clocks.
# Untracked files travel whenever `git ls-files -o --exclude-standard` lists any (V1-2802). The
# tested content is identified by the Git tree of the working tree (HEAD plus staged, unstaged and
# untracked non-ignored changes, written through a scratch index) and its count of paths differing
# from HEAD: the remote rebuilds that tree, refuses to run (exit 97) when its tree differs, and
# prints `running on HOST OS/ARCH GOVERSION at HEAD [+wip] tree=TREE dirty=N`. A local run of an
# offloadable command prints `running locally at HEAD tree=TREE dirty=N` (tree=unknown when Git
# cannot compute it).
# Any clone (ASD-V0-056, V1-3202): when origin is a local repository with an origin of its own (a
# plain `git clone` of a lane checkout), that origin's URL and that repository's origin/main stand in
# for origin and origin/main, one hop only. A checkout that cannot offload (no Git checkout, no
# origin, the base missing, or no history shared with it) prints `heavy.sh: offload unavailable from
# this checkout: REASON` once and runs locally, unretried.
# When the host is unreachable or the remote cannot reproduce the tree (exit 97, or ssh's 255), the
# offload is retried once after HEAVY_REMOTE_RETRY seconds; if that also fails, one stderr line says
# so and the command runs locally under the local semaphore. A failure to prepare the payload here is
# not retried. A TERM, INT or HUP to this wrapper during an offload stops the local ssh and exits
# 143, 130 or 129; the remote command keeps running until it ends or HEAVY_REMOTE_TIMEOUT stops it.
# Each local run of an offloadable command (unavailable, HEAVY_REMOTE=0, a file-writing flag) is
# appended to HEAVY_FALLBACK_LOG, so host load from local runs stays visible. Only the argv travels:
# set test environment with `env K=V` inside the command, or as leading K=V words, which become
# `env K=V ...` (ASD-V0-830). GOCACHE/GOMODCACHE assignments are
# dropped remotely, and a command with a file-writing flag (-o, -c, -outputdir, -coverprofile, the
# profiles, -trace, -test.*), as an argument, in a GOFLAGS= argument or in the exported GOFLAGS,
# always runs locally, because its files would stay on the remote.
# Checkout files a remote run writes (DCW-V0-359, V1-2858; for example goldens regenerated with
# -update) are copied back. After the command the remote compares its worktree with the tested tree
# (non-ignored paths, as the tree identity counts them); when they differ it keeps the written files,
# the deleted paths and the command's status in HEAVY_REMOTE_ROOT/out/ID.tar and exits 95, and the
# wrapper fetches that over a second ssh connection, which removes it. Only when every such local
# path still holds its tested content are the files installed (sibling copy, then rename) and the
# deletions made, one `heavy.sh: copied back PATH` line each, and the command's status returned;
# otherwise nothing is changed, the paths are named and the wrapper exits 96 asking for a local run.
# A run that writes no checkout file opens no second connection. Stale stores go after a day.
#
# Remote build cache (V1-2691): /tmp on a Linux host is often a small tmpfs and a Go build cache
# there filled it (ENOSPC). The remote command therefore gets CORVINT_GOCACHE (honoured by the
# Makefile's `?=`, so `make windows-vet` uses it) and GOCACHE (for a plain `go test`/`go vet`) set to
# ${HEAVY_REMOTE_GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache} on the remote disk,
# unless the remote environment (HEAVY_REMOTE_ENV) already exports them. HEAVY_REMOTE_GOCACHE is
# read on the remote. No remote command default touches /tmp for the Go cache; trim it with
# `make go-cache-trim` if it grows. The remote command's TMPDIR is a fresh directory under
# ${HEAVY_REMOTE_TMPDIR:-HEAVY_REMOTE_ROOT/tmp} (read on the remote), removed with everything in it
# when the command ends (V1-2563: the remote disk, as on the CI runners, not a tmpfs /tmp).
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
#
# The whole script is function definitions followed by one `main` line, so the remote bash parses
# it completely before main reads the offload payload that follows it on the same stdin.
set -uo pipefail

refuse() {
  printf 'heavy.sh: REFUSE %s\n' "$*" >&2
  printf 'heavy.sh: REFUSED %s (exit 2)\n' "${1%% *}" # stdout marker survives `| tail` (V1-3213)
  exit 2
}

# load_config FILE: KEY=VALUE lines (blank and # lines ignored; one level of quotes stripped) for
# HEAVY_* keys the environment leaves unset. Nothing in the file is executed.
load_config() {
  local file=$1 line key value
  [[ -r $file && -f $file ]] || return 0
  while IFS= read -r line || [[ -n $line ]]; do
    [[ $line =~ ^[[:space:]]*(#|$) ]] && continue
    [[ $line =~ ^[[:space:]]*(export[[:space:]]+)?(HEAVY_[A-Z_]+)=(.*)$ ]] || {
      printf 'heavy.sh: ignoring config line in %s: %s\n' "$file" "$line" >&2
      continue
    }
    key=${BASH_REMATCH[2]} value=${BASH_REMATCH[3]}
    [[ $value =~ ^\"(.*)\"$ || $value =~ ^\'(.*)\'$ ]] && value=${BASH_REMATCH[1]}
    [[ -n ${!key+set} ]] || export "$key=$value"
  done < "$file"
}

cpu_count() {
  local n
  n=$(getconf _NPROCESSORS_ONLN 2> /dev/null || sysctl -n hw.ncpu 2> /dev/null || echo 4)
  [[ $n =~ ^[1-9][0-9]*$ ]] || n=4
  printf '%s\n' "$n"
}

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
  for c in "${HOME:-/nonexistent}/sdk/go$want/bin" "${cache-}/golang.org/toolchain@v0.0.1-go$want.${os-}-${arch-}/bin"; do
    if [[ -x $c/go && $(GOTOOLCHAIN=local "$c/go" env GOVERSION 2> /dev/null) == "go$want" ]]; then
      export PATH="$c:$PATH"
      return 0
    fi
  done
  printf 'heavy.sh: pinned go%s not installed; running %s\n' "$want" "${have:-an unknown go}" >&2
}

# slot_limits CPUS: validated slots, parallelism and niceness into the globals slots, par, nice_level.
slot_limits() {
  local cpus=$1
  slots=${HEAVY_SLOTS:-$((cpus / 4 > 0 ? cpus / 4 : 1))}
  if ! [[ $slots =~ ^[1-9][0-9]*$ ]] || ((slots > 64)); then
    refuse "invalid-slots HEAVY_SLOTS=$slots (want 1..64)"
  fi
  par=${HEAVY_P:-$((cpus / slots > 0 ? cpus / slots : 1))}
  if ! [[ $par =~ ^[1-9][0-9]*$ ]] || ((par > cpus)); then
    refuse "invalid-parallelism HEAVY_P=$par (want 1..$cpus)"
  fi
  nice_level=${HEAVY_NICE:-10}
  if ! [[ $nice_level =~ ^(0|[1-9][0-9]?)$ ]] || ((nice_level > 19)); then
    refuse "invalid-nice HEAVY_NICE=$nice_level (want 0..19)"
  fi
}

bound_parallelism() {
  case " ${GOFLAGS-} " in
    *" -p="* | *" -p "*) ;;
    *) export GOFLAGS="${GOFLAGS:+$GOFLAGS }-p=$1" ;;
  esac
}

# The flock semaphore (see the header): perl -e "$slot_perl" LOCK_DIR SLOTS NICE COMMAND...
slot_perl='
use strict;
use warnings;
use Fcntl qw(:flock F_SETFD);
use POSIX qw(setpgid strftime);
use Time::HiRes qw(time);
use Cwd ();
use Config;
my ($dir, $n, $nice, @cmd) = @ARGV;
# ASD-V0-037: HEAVY_RUN_LOG names the selection log a local run appends its status line to, and
# HEAVY_RUN_TAG the suffix that line ends with; neither reaches the command.
my $log = delete $ENV{HEAVY_RUN_LOG} // "";
my $tag = delete $ENV{HEAVY_RUN_TAG} // "";
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
my $t1 = time;
printf STDERR "heavy.sh: slot %d/%d after %ds\n", $slot, $n, $t1 - $t0;
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
my $rc = $status & 127 ? 128 + ($status & 127) : $status >> 8;
# ASD-V0-940 (V1-3247): a non-zero exit names its cause, so it never reads as a silent wrapper failure.
if ($rc) {
    my $cause = "command";
    if ($status & 127) { my @sig = split " ", $Config{sig_name}; $cause = "signal-" . ($sig[$status & 127] // $status & 127) }
    elsif ($cmd[0] eq "timeout" && $rc == 124) { $cause = "timeout" }
    printf STDERR "heavy.sh: exit rc=%d cause=%s cmd=%s\n", $rc, $cause, $cmd[0] eq "timeout" ? $cmd[2] // "" : $cmd[0];
}
if (length $log) {
    my $line = sprintf "%s local wait=%.1fs run=%.1fs rc=%d pwd=%s cmd=%s%s\n",
        strftime("%Y-%m-%dT%H:%M:%SZ", gmtime), $t1 - $t0, time - $t1, $rc, $ENV{PWD} // Cwd::getcwd(), "@cmd", $tag;
    if (open(my $fh, ">>", $log)) { print $fh $line; close $fh }
    else { print STDERR "heavy.sh: cannot append to $log\n" }
}
exit $rc;
'

# origin_key URL: host-independent repository identity (owner/name) of a Git remote URL.
origin_key() {
  local u=${1%/}
  u=${u%.git}
  u=${u#*://}
  u=${u#*@}
  u=${u#*[:/]}
  printf '%s\n' "$u" | tr '[:upper:]' '[:lower:]'
}

# upstream TOP: the effective origin of the checkout at TOP (ASD-V0-056, V1-3202), as
# `URL<TAB>BASEREF`. A plain clone of a local checkout (`git clone ~/corvint scratch`) has a local
# path as its origin, whose `main` may be an unrelated lineage, so when origin is a local repository
# that itself has an origin, that repository's origin URL and its `origin/main` commit stand in, one
# hop only. Otherwise origin and `origin/main`. Fails, with the reason and its recovery on stdout,
# when TOP has no origin, or that repository has no origin/main or its commit is not in TOP.
upstream() {
  local top=$1 url path up ref
  url=$(git -C "$top" remote get-url origin 2> /dev/null) || { printf 'no origin remote\n'; return 1; }
  path=${url#file://}
  if [[ $path == /* || $path == .* ]] && [[ $path != *:* ]]; then
    [[ $path == /* ]] || path=$top/$path
    if up=$(git -C "$path" remote get-url origin 2> /dev/null) && [[ -n $up ]]; then
      ref=$(git -C "$path" rev-parse -q --verify 'refs/remotes/origin/main^{commit}' 2> /dev/null) ||
        { printf 'the source checkout %s has no origin/main; run git -C %s fetch origin\n' "$path" "$path"; return 1; }
      git -C "$top" cat-file -e "$ref^{commit}" 2> /dev/null ||
        { printf 'base %s (origin/main of %s) is not in this clone; run git fetch %s refs/remotes/origin/main\n' "$ref" "$path" "$path"; return 1; }
      printf '%s\t%s\n' "$up" "$ref"
      return 0
    fi
  fi
  printf '%s\torigin/main\n' "$url"
}

# tree_identity TOP INDEX: the content identity of TOP's working tree as Git would commit it (HEAD
# plus staged, unstaged and untracked non-ignored changes) into heavy_tree, and the number of paths
# where it differs from HEAD into heavy_dirty. INDEX is a scratch index file (a copy of the real
# one, so unchanged files are not rehashed); the real index is never written. Writes only blob and
# tree objects to the object store.
tree_identity() {
  local top=$1 index=$2 real
  heavy_tree= heavy_dirty=
  real=$(git -C "$top" rev-parse --path-format=absolute --git-path index 2> /dev/null) || return 1
  /bin/rm -f "$index"
  if [[ -f $real ]]; then /bin/cp -f "$real" "$index" || return 1; fi
  GIT_INDEX_FILE=$index git -C "$top" add -A > /dev/null 2>&1 || return 1
  heavy_tree=$(GIT_INDEX_FILE=$index git -C "$top" write-tree 2> /dev/null) || return 1
  heavy_dirty=$(git -C "$top" diff-tree -r --name-only -z HEAD "$heavy_tree" 2> /dev/null | tr -cd '\0' | wc -c | tr -d ' ')
  /bin/rm -f "$index"
  [[ -n $heavy_tree ]]
}

# offload_unavailable REASON: the one stderr line naming why this checkout cannot offload.
offload_unavailable() { printf 'heavy.sh: offload unavailable from this checkout: %s\n' "$1" >&2; }

# offload COMMAND...: run a go test, go vet or (with heavy_build set to the remote's maximum load) a
# build on HEAVY_REMOTE_HOST at this checkout's exact tree. Returns the command's status, or 97 when
# the remote cannot run it.
offload() {
  local top rel head base ref origin t size t0 t1 tree back wip=
  t0=$(heavy_now)
  heavy_noretry=1 # cleared once the payload is ready: only a remote failure is retried
  # ASD-V0-056 (V1-3202): a checkout that cannot offload says why, once, before the local run.
  top=$(git rev-parse --show-toplevel 2> /dev/null) || { offload_unavailable 'not a Git checkout'; return 97; }
  origin=$(upstream "$top") || { offload_unavailable "$origin"; return 97; }
  ref=${origin#*$'\t'} origin=${origin%%$'\t'*}
  rel=$(git -C "$PWD" rev-parse --show-prefix 2> /dev/null) || return 97
  rel=${rel%/}
  head=$(git -C "$top" rev-parse HEAD 2> /dev/null) || { offload_unavailable 'no HEAD commit'; return 97; }
  git -C "$top" cat-file -e "$ref^{commit}" 2> /dev/null ||
    { offload_unavailable "base $ref is not in this clone; run git fetch origin main"; return 97; }
  base=$(git -C "$top" merge-base HEAD "$ref" 2> /dev/null) ||
    { offload_unavailable "HEAD shares no history with $ref (a pre-snapshot lineage?); base the branch on origin/main"; return 97; }
  t=$(mktemp -d "${TMPDIR:-/tmp}/heavy-offload.XXXXXX") || return 97
  heavy_tmp=$t # removed by main, or by its EXIT trap when the wrapper is interrupted
  if [[ $head != "$base" ]]; then
    git -C "$top" bundle create -q "$t/head.bundle" HEAD "^$base" < /dev/null 2> /dev/null || return 97
  fi
  git -C "$top" diff --binary HEAD > "$t/wip.patch" 2> /dev/null || return 97
  local tarflags=() files=(meta.env wip.patch)
  tar --version 2> /dev/null | grep -q bsdtar && tarflags=(--no-xattrs --no-mac-metadata)
  # V1-2802: whether untracked files exist is read from the file list, never from the archive
  # size (a tar of one small file is the same 10240-byte record as an empty one). No list, no tar.
  git -C "$top" ls-files -o --exclude-standard -z > "$t/untracked.list" 2> /dev/null || return 97
  if [[ -s $t/untracked.list ]]; then
    (cd "$top" && COPYFILE_DISABLE=1 tar ${tarflags[@]+"${tarflags[@]}"} --null -T "$t/untracked.list" \
      -cf "$t/untracked.tar" 2> /dev/null) || return 97
    size=$(wc -c < "$t/untracked.tar" | tr -d ' ')
    if ((size >= 50000000)); then
      printf 'heavy.sh: untracked files over 50 MB; not offloading\n' >&2
      return 97
    fi
    files+=(untracked.tar)
    wip=1
  fi
  [[ -s $t/wip.patch ]] && wip=1
  # The tested tree's identity, taken after the payload so a later edit shows up as a mismatch
  # on the remote rather than as a wrong label.
  tree_identity "$top" "$t/index" || return 97
  tree=$heavy_tree
  local xgoos='' xgoarch=''
  if [[ -n ${heavy_build-} ]]; then
    { read -r xgoos; read -r xgoarch; } < <(GOTOOLCHAIN=local go env GOOS GOARCH 2> /dev/null)
    [[ -n $xgoos && -n $xgoarch ]] || return 97
  fi
  back=$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')
  [[ $back =~ ^[0-9a-f]{16}$ ]] || return 97
  {
    printf 'BASE=%q\nHEAD=%q\nRELDIR=%q\nWIP=%q\nORIGIN=%q\n' "$base" "$head" "${rel:-.}" "$wip" "$(origin_key "$origin")"
    printf 'TREE=%q\nDIRTY=%q\nBACK=%q\n' "$heavy_tree" "$heavy_dirty" "$back"
    printf 'CLONE=%q\nROOT=%q\nSLOTS=%q\nENVFILE=%q\nTIMEOUT=%q\n' "${HEAVY_REMOTE_CLONE-}" "${HEAVY_REMOTE_ROOT-}" \
      "${HEAVY_REMOTE_SLOTS-}" "${HEAVY_REMOTE_ENV-}" "${HEAVY_REMOTE_TIMEOUT:-40m}"
    printf 'BUSY=%q\nGMP=%q\n' "${HEAVY_REMOTE_BUSY_PROCESS-}" "${HEAVY_REMOTE_GOMAXPROCS-}"
    [[ -n ${heavy_build-} ]] && printf 'BUILD=1\nXGOOS=%q\nXGOARCH=%q\nMAXLOAD=%q\n' "$xgoos" "$xgoarch" "$heavy_build"
    printf 'CMD=('
    printf '%q ' "$@"
    printf ')\n'
  } > "$t/meta.env"
  [[ -f $t/head.bundle ]] && files+=(head.bundle)
  local self=${BASH_SOURCE[0]} rc phase=
  mkfifo "$t/stderr" || return 97
  heavy_noretry=
  t1=$(heavy_now)
  # ASD-V0-037: the remote's stderr passes through unchanged, and its phase line is also kept so the
  # selection log records the remote phases. Both run in the background so a signal trapped by main
  # interrupts the wait and stops them (heavy_abort); waiting for the filter makes the kept line
  # complete before it is read.
  perl -e '$| = 1; open(my $p, ">", shift) or die; while (<STDIN>) { print STDERR $_; print $p $_ if /^heavy\.sh: phase / }' \
    "$t/phase" < "$t/stderr" &
  heavy_filter=$!
  {
    printf '__heavy_serve=1\n'
    cat "$self"
    COPYFILE_DISABLE=1 tar ${tarflags[@]+"${tarflags[@]}"} -C "$t" -cf - "${files[@]}" | base64
  } | ${HEAVY_REMOTE_SSH:-ssh} -o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=30 \
    "$HEAVY_REMOTE_HOST" "${HEAVY_REMOTE_SHELL:-bash -s}" 2> "$t/stderr" &
  heavy_ssh=$!
  wait "$heavy_ssh"
  rc=$?
  heavy_ssh=
  wait "$heavy_filter"
  heavy_filter=
  [[ -s $t/phase ]] && phase=" $(sed -n '$s/^heavy\.sh: phase //p' "$t/phase")"
  # V1-2819: the local phases of every offload attempt, then the remote's own phase line.
  selection_log "$(date -u +%Y-%m-%dT%H:%M:%SZ) offload host=$HEAVY_REMOTE_HOST pack=$(heavy_secs "$t0" "$t1")s remote=$(heavy_secs "$t1" "$(heavy_now)")s rc=$rc$phase pwd=$PWD cmd=$*"
  ((rc == 255)) && return 97
  ((rc == 95)) && { copy_back "$top" "$tree" "$back" "$t"; return; }
  return "$rc"
}

# copy_back TOP TREE ID SCRATCH: fetch and remove the remote's store ID of the checkout files a run
# wrote (DCW-V0-359) and install them in TOP when each local path still holds its content in TREE.
# Returns the remote command's status, 95 when there is no store (the command's own status), or 96
# when nothing was installed.
copy_back() {
  local top=$1 tree=$2 id=$3 d=$4/back p want have conflicts=() status
  mkdir -p "$d/files" || return 96
  { printf '__heavy_fetch=%q\n__heavy_fetch_root=%q\n' "$id" "${HEAVY_REMOTE_ROOT-}"; cat "${BASH_SOURCE[0]}"; } |
    ${HEAVY_REMOTE_SSH:-ssh} -o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=30 \
      "$HEAVY_REMOTE_HOST" "${HEAVY_REMOTE_SHELL:-bash -s}" > "$d/store.b64"
  status=$?
  ((status == 3)) && return 95
  if ((status != 0)) || ! base64 -d < "$d/store.b64" | tar -x -C "$d" 2> /dev/null ||
    ! tar -x -C "$d/files" -f "$d/files.tar" 2> /dev/null || [[ ! $(cat "$d/rc" 2> /dev/null) =~ ^[0-9]{1,3}$ ]]; then
    printf 'heavy.sh: the remote run wrote checkout files that could not be fetched; nothing was copied back. Rerun locally: HEAVY_REMOTE=0 HEAVY_LOCAL_REASON=%q\n' 'writes checkout files' >&2
    return 96
  fi
  # Every written or deleted path must still hold its tested content here (absent when the tested
  # tree lacks it), so a local edit made during the run is never overwritten.
  while IFS= read -r -d '' p; do
    # A build's absolute output, or one absent from the tested tree (ignored or new), is overwritten,
    # as a local build would (ASD-V0-041).
    [[ -n ${heavy_build-} && $p == .rt-abs/* ]] && continue
    want=$(git -C "$top" rev-parse -q --verify "$tree:$p" 2> /dev/null) || want=
    [[ -n ${heavy_build-} && -z $want ]] && continue
    if [[ -e $top/$p || -L $top/$p ]]; then have=$(git -C "$top" hash-object -- "$p" 2> /dev/null) || have=unreadable; else have=; fi
    [[ $have == "$want" ]] || conflicts+=("$p")
  done < <(cat "$d/write.list" "$d/delete.list")
  if ((${#conflicts[@]})); then
    printf 'heavy.sh: the remote run wrote checkout files that changed here meanwhile; nothing was copied back: %s. Rerun locally: HEAVY_REMOTE=0 HEAVY_LOCAL_REASON=%q\n' "${conflicts[*]}" 'writes checkout files' >&2
    return 96
  fi
  local dst
  while IFS= read -r -d '' p; do
    dst=$(build_dest "$top" "$p") || return 96
    # A fresh mktemp sibling, never a predictable name an existing file or symlink could occupy.
    if ! { mkdir -p "$(dirname "$dst")" && tmp=$(mktemp "$dst.heavy-new.XXXXXX") &&
      /bin/cp -p "$d/files/$p" "$tmp" && /bin/mv -f "$tmp" "$dst"; }; then
      [[ -n ${tmp:-} ]] && /bin/rm -f "$tmp"
      printf 'heavy.sh: copying back %s failed; rerun locally: HEAVY_REMOTE=0 HEAVY_LOCAL_REASON=%q\n' "$p" 'writes checkout files' >&2
      return 96
    fi
    printf 'heavy.sh: copied back %s\n' "${dst#"$top/"}" >&2
  done < "$d/write.list"
  while IFS= read -r -d '' p; do
    /bin/rm -f "$top/$p" || return 96
    printf 'heavy.sh: copied back the deletion of %s\n' "$p" >&2
  done < "$d/delete.list"
  return "$(cat "$d/rc")"
}

# build_dest TOP PATH: where a copied-back PATH goes: TOP/PATH, or for a build's .rt-abs/K[/REST]
# the absolute output build_abs[K][/REST] (ASD-V0-041). Returns 1 for an unknown K.
build_dest() {
  local top=$1 p=$2 rest k
  if [[ -z ${heavy_build-} || $p != .rt-abs/* ]]; then
    printf '%s/%s\n' "$top" "$p"
    return 0
  fi
  rest=${p#.rt-abs/}
  k=${rest%%/*}
  [[ $k =~ ^[0-9]+$ && -n ${build_abs[k]-} ]] || return 1
  if [[ $rest == "$k" ]]; then printf '%s\n' "${build_abs[k]%/}"; else printf '%s/%s\n' "${build_abs[k]%/}" "${rest#*/}"; fi
}

# fetch: the remote half of copy_back. Prints the store as base64 and removes it; exit 3 = none.
fetch() {
  local root=${__heavy_fetch_root:-$HOME/.cache/corvint-heavy} f
  root=${root/#\~/$HOME}
  [[ $__heavy_fetch =~ ^[0-9a-f]{16}$ ]] || exit 3
  f=$root/out/$__heavy_fetch.tar
  [[ -f $f ]] || exit 3
  base64 < "$f" | tr -d '\n' && /bin/rm -f "$f"
}

# heavy_now: wall-clock seconds with millisecond precision.
heavy_now() {
  perl -MTime::HiRes=time -e 'printf "%.3f\n", time'
}

# heavy_secs FROM TO: TO - FROM in seconds, one decimal.
heavy_secs() {
  awk -v a="$1" -v b="$2" 'BEGIN { printf "%.1f", b - a }'
}

# worktree_slot DIR N: take the first free of N worktree locks DIR/slotI.lock on descriptor 7, which
# the command inherits so the lock is held until it exits, waiting while all N are held; set
# wt_slot to I. Returns 1 when a lock cannot be opened.
worktree_slot() {
  local dir=$1 n=$2 i
  mkdir -p "$dir" || return 1
  while :; do
    for ((i = 1; i <= n; i++)); do
      exec 7>> "$dir/slot$i.lock" || return 1
      # The flock belongs to the open file description bash keeps on descriptor 7, so it outlives perl.
      perl -MFcntl=:flock -e 'open(my $h, ">>&=", 7) or exit 2; exit(flock($h, LOCK_EX | LOCK_NB) ? 0 : 1)'
      case $? in
        0) wt_slot=$i && return 0 ;;
        1) exec 7>&- ;;
        *) return 1 ;;
      esac
    done
    sleep 1
  done
}

# serve: the remote half. Reads the payload from stdin, reproduces the tree in this slot's reused
# worktree of the remote clone, and runs the command under the remote flock semaphore. Exit 97 =
# the remote cannot run it (the caller then runs locally).
# shellcheck disable=SC1090,SC2153 # meta.env (written by offload) defines BASE, HEAD, ORIGIN, CMD...
serve() {
  local slots par nice_level a cmd=() t0 t1 t2 t3 t4 rc reused=reused common
  t0=$(heavy_now)
  infra() {
    printf 'heavy.sh: remote %s: %s\n' "$(command -p hostname)" "$*" >&2
    exit 97
  }
  root=$HOME/.cache/corvint-heavy
  id=$(date +%s)-$$
  # TSP-V0-004: the default PATH keeps the fresh payload directory provable (this file sets PATH).
  w=$(command -p mktemp -d "${TMPDIR:-/tmp}/corvint-heavy-in.XXXXXX") || infra "mktemp failed"
  # shellcheck disable=SC2016 # expanded when the trap runs
  trap 'git -C "${clone:-/nonexistent}" update-ref -d "refs/heavy/$id" > /dev/null 2>&1
    [[ -n ${run_tmp-} ]] && { chmod -R u+rwx "$run_tmp"; /bin/rm -rf "$run_tmp"; } > /dev/null 2>&1
    /bin/rm -rf "$w"' EXIT
  trap 'exit 130' INT TERM HUP
  command -p base64 -d | command -p tar -x -C "$w" || infra "payload decode failed"
  # shellcheck disable=SC1091 # generated by offload
  . "$w/meta.env" || infra "metadata unreadable"
  [[ -n $ENVFILE ]] && { . "${ENVFILE/#\~/$HOME}" || infra "env file $ENVFILE failed"; }
  # ASD-V0-037 (ported from the box prototype): while BUSY runs here (a CI job), the run goes local.
  if [[ -n $BUSY ]] && pgrep -x "$BUSY" > /dev/null 2>&1; then infra "busy: $BUSY is running"; fi
  # ASD-V0-041 (ported from the box prototype): a build runs here only while the box is quiet.
  if [[ -n ${BUILD-} ]]; then
    a=$(load1)
    [[ $a =~ ^[0-9]+$ ]] || infra "busy: load unreadable; building locally"
    ((a <= MAXLOAD)) || infra "busy: load $a above $MAXLOAD; building locally"
  fi
  clone=${CLONE:-$HOME/corvint}
  clone=${clone/#\~/$HOME}
  root=${ROOT:-$root}
  root=${root/#\~/$HOME}
  [[ -d $clone/.git || -f $clone/.git ]] || infra "no clone at $clone (set HEAVY_REMOTE_CLONE)"
  [[ $(origin_key "$(git -C "$clone" remote get-url origin 2> /dev/null)") == "$ORIGIN" ]] ||
    infra "clone $clone is not a clone of $ORIGIN"
  git -C "$clone" cat-file -e "$BASE^{commit}" 2> /dev/null || git -C "$clone" fetch -q origin < /dev/null ||
    infra "fetch origin failed"
  git -C "$clone" cat-file -e "$BASE^{commit}" 2> /dev/null || infra "base $BASE is not on origin"
  if [[ $HEAD != "$BASE" ]]; then
    git -C "$clone" fetch -q "$w/head.bundle" "HEAD:refs/heavy/$id" < /dev/null 2> /dev/null || infra "bundle fetch failed"
  fi
  HEAVY_SLOTS=${SLOTS-} HEAVY_NICE=5
  unset HEAVY_P
  slot_limits "$(cpu_count)"
  t1=$(heavy_now)
  # V1-2819: each of the $slots worktree locks owns one worktree at a fixed path, reset to the tested
  # tree on every run. Go keys compiled packages by their directory, so a fresh path per run would
  # rebuild every package of the module; a fixed one lets the remote build cache hit.
  worktree_slot "$root/wt" "$slots" || infra "worktree lock under $root/wt unavailable"
  t2=$(heavy_now)
  wt=$root/wt/heavy-slot$wt_slot
  common=$(git -C "$clone" rev-parse --path-format=absolute --git-common-dir 2> /dev/null)
  if ! {
    [[ -f $wt/.git && $(git -C "$wt" rev-parse --path-format=absolute --git-common-dir) == "$common" ]] &&
      git -C "$wt" checkout -q --detach --force "$HEAD" && git -C "$wt" clean -q -ffdx
  } < /dev/null > /dev/null 2>&1; then
    # Missing, from another clone, or left unusable by an interrupted run: recreate it.
    reused=created
    /bin/rm -rf "$wt"
    git -C "$clone" worktree add -q -f --detach "$wt" "$HEAD" < /dev/null > /dev/null 2>&1 || infra "worktree add failed"
  fi
  cd "$wt" || infra "worktree unreadable"
  if [[ -s $w/wip.patch ]]; then git apply --binary "$w/wip.patch" || infra "uncommitted diff did not apply"; fi
  if [[ -s $w/untracked.tar ]]; then tar -xf "$w/untracked.tar" || infra "untracked files did not unpack"; fi
  tree_identity "$wt" "$w/index" || infra "tree identity unavailable"
  [[ $heavy_tree == "$TREE" ]] || infra "reproduced tree $heavy_tree differs from the local tree $TREE"
  cd "$wt/$RELDIR" || infra "directory $RELDIR missing"
  mkdir -p "$root/slots" || infra "lock dir $root/slots unavailable"
  # V1-2563, V1-2691: the command's temporary files live in a per-run directory on the remote disk
  # (not a tmpfs /tmp), removed afterwards with whatever the tests leaked into it.
  mkdir -p "${HEAVY_REMOTE_TMPDIR:-$root/tmp}" || infra "temp dir ${HEAVY_REMOTE_TMPDIR:-$root/tmp} unavailable"
  run_tmp=$(mktemp -d "${HEAVY_REMOTE_TMPDIR:-$root/tmp}/run.XXXXXX") || infra "per-run temp dir unavailable"
  export TMPDIR=$run_tmp
  pin_go
  : "${CORVINT_GOCACHE:=${HEAVY_REMOTE_GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}"
  : "${GOCACHE:=$CORVINT_GOCACHE}"
  export CORVINT_GOCACHE GOCACHE
  mkdir -p "$GOCACHE" || infra "go cache $GOCACHE unavailable"
  bound_parallelism "$par"
  # V1-1628: -p bounds how many packages build or test at once, not the threads inside one test
  # binary, so a whole-package run on every slot would still contend for all the CPUs.
  if [[ -z ${GOMAXPROCS-} ]] && ((${GMP:-$par} > 0)); then export GOMAXPROCS=${GMP:-$par}; fi
  printf 'heavy.sh: remote bound -p=%s GOMAXPROCS=%s\n' "$par" "${GOMAXPROCS:-unset}" >&2
  printf 'heavy.sh: running on %s %s/%s %s at %s%s tree=%s dirty=%s\n' "$(hostname)" "$(go env GOOS 2> /dev/null)" \
    "$(go env GOARCH 2> /dev/null)" "$(go env GOVERSION 2> /dev/null)" "$HEAD" "${WIP:+ +wip}" "$TREE" "$DIRTY" >&2
  if [[ -n ${BUILD-} ]]; then
    # The caller's absolute outputs are built under $wt/.rt-abs (offload's @RT@) for its platform.
    mkdir -p "$wt/.rt-abs" || infra "build output dir unavailable"
    export GOOS=$XGOOS GOARCH=$XGOARCH
    printf 'heavy.sh: building for %s/%s\n' "$GOOS" "$GOARCH" >&2
  fi
  # A local GOCACHE/GOMODCACHE path means nothing here; the remote uses its own Go caches.
  for a in "${CMD[@]}"; do
    [[ -n ${BUILD-} ]] && a=${a//@RT@/$wt}
    case $a in GOCACHE=* | GOMODCACHE=*) ;; *) cmd+=("$a") ;; esac
  done
  command -v timeout > /dev/null || infra "timeout(1) missing; HEAVY_REMOTE_TIMEOUT cannot be enforced"
  : > "$w/marker" || infra "marker unwritable"
  t3=$(heavy_now)
  perl -e "$slot_perl" "$root/slots" "$slots" "$nice_level" timeout "$TIMEOUT" "${cmd[@]}" < /dev/null
  rc=$?
  t4=$(heavy_now)
  # run is the build and the test or vet together (Go does not report them apart).
  printf 'heavy.sh: phase receive=%ss wait=%ss prepare=%ss run=%ss worktree=%s/%s\n' "$(heavy_secs "$t0" "$t1")" \
    "$(heavy_secs "$t1" "$t2")" "$(heavy_secs "$t2" "$t3")" "$(heavy_secs "$t3" "$t4")" "heavy-slot$wt_slot" "$reused" >&2
  if [[ -n ${BUILD-} ]]; then keep_built "$rc" || return 96; else keep_written "$rc" || return 96; fi
  return "$rc"
}

# keep_written RC: in serve, after the command. When the worktree no longer matches the tested
# tree, store the written files, the deleted paths and RC under $root/out/$BACK.tar and set rc to
# 95 (DCW-V0-359). Returns 1, naming the paths, when they cannot be stored.
# shellcheck disable=SC2153 # BACK and TREE come from meta.env (serve)
keep_written() {
  local o=$w/back
  if ! tree_identity "$wt" "$w/index"; then
    printf 'heavy.sh: remote %s: no worktree identity after the run, so checkout files it wrote cannot be copied back. Rerun locally: HEAVY_REMOTE=0 HEAVY_LOCAL_REASON=%q\n' "$(hostname)" 'writes checkout files' >&2
    return 1
  fi
  [[ $heavy_tree == "$TREE" ]] && return 0
  mkdir -p "$o" "$root/out" || return 1
  find "$root/out" -name '*.tar*' -mmin +1440 -exec /bin/rm -f {} + 2> /dev/null
  git -C "$wt" diff-tree -r --no-renames -z --name-only --diff-filter=d "$TREE" "$heavy_tree" > "$o/write.list" &&
    git -C "$wt" diff-tree -r --no-renames -z --name-only --diff-filter=D "$TREE" "$heavy_tree" > "$o/delete.list" || return 1
  store_back "$1"
}

# keep_built RC: in serve, after a build (ASD-V0-041). When it passed, store every file it wrote
# under the worktree (newer than the run's marker; .git excluded) for copy-back, setting rc to 95.
keep_built() {
  local o=$w/back
  (($1 == 0)) || return 0
  mkdir -p "$o" "$root/out" || return 1
  find "$root/out" -name '*.tar*' -mmin +1440 -exec /bin/rm -f {} + 2> /dev/null
  (cd "$wt" && find . -path ./.git -prune -o -type f -newer "$w/marker" -print0) |
    perl -0 -pe 's#^\./##' > "$o/write.list" || return 1
  : > "$o/delete.list" || return 1
  [[ -s $o/write.list ]] || return 0
  store_back "$1"
}

# store_back RC: write the store $root/out/$BACK.tar of the files in $w/back/write.list (from the
# worktree), the paths in delete.list and RC, and set rc to 95. Returns 1, naming the paths, on failure.
# shellcheck disable=SC2153 # BACK comes from meta.env (serve)
store_back() {
  local o=$w/back n
  n=$(cat "$o/write.list" "$o/delete.list" | tr -cd '\0' | wc -c | tr -d ' ')
  printf '%s\n' "$1" > "$o/rc"
  if [[ ! $BACK =~ ^[0-9a-f]{16}$ ]] ||
    ! (cd "$wt" && tar --null -T "$o/write.list" -cf "$o/files.tar") ||
    ! tar -C "$o" -cf "$root/out/$BACK.tar.part" rc write.list delete.list files.tar ||
    ! /bin/mv -f "$root/out/$BACK.tar.part" "$root/out/$BACK.tar"; then
    printf 'heavy.sh: remote %s: the run wrote %s checkout files that stay on the remote: %s. Rerun locally: HEAVY_REMOTE=0 HEAVY_LOCAL_REASON=%q\n' \
      "$(hostname)" "$n" "$(cat "$o/write.list" "$o/delete.list" | tr '\0' ' ')" 'writes checkout files' >&2
    return 1
  fi
  rc=95
}

# local_identity COMMAND...: for an offloadable command run here inside a Git checkout, one stderr
# line naming the tested tree (tree_identity), or tree=unknown when it cannot be computed. It is
# taken before slot admission, so an edit made while waiting is not reflected.
local_identity() {
  local top head idx
  offloadable "$@" || return 0
  top=$(git rev-parse --show-toplevel 2> /dev/null) || return 0
  head=$(git -C "$top" rev-parse HEAD 2> /dev/null) || head=unknown
  idx=$(mktemp "${TMPDIR:-/tmp}/heavy-index.XXXXXX") && tree_identity "$top" "$idx" || heavy_tree=unknown heavy_dirty=unknown
  [[ -n ${idx-} ]] && /bin/rm -f "$idx"
  printf 'heavy.sh: running locally at %s tree=%s dirty=%s\n' "$head" "$heavy_tree" "$heavy_dirty" >&2
}

# offloadable ARG...: true for a command kind the remote runs (go test, go vet, make windows-vet).
offloadable() {
  case " $* " in
    *" go test "* | *" go vet "* | *" make windows-vet "*) return 0 ;;
  esac
  return 1
}

# buildable ARG...: true for a build the remote may cross-compile (ASD-V0-041).
buildable() {
  local a
  [[ " $* " == *" go build "* ]] && return 0
  [[ " $* " == *" make "* ]] || return 1
  for a; do
    [[ $a == build || $a == tasks-build ]] && return 0
  done
  return 1
}

# build_map COMMAND...: the build command into build_cmd with each absolute output (-o P, -o=P,
# NAME_BIN=P) replaced by @RT@/.rt-abs/K (serve puts the remote worktree for @RT@) and P into
# build_abs[K]; relative outputs keep their checkout path. Returns 1 for an argument with a ..
# component, whose meaning the remote worktree cannot keep.
build_map() {
  local a prev='' k slash
  build_cmd=() build_abs=()
  for a in "$@"; do
    [[ $a =~ (^|[/=])\.\.(/|$) ]] && return 1
    k=${#build_abs[@]} slash=
    [[ $a == */ ]] && slash=/
    if [[ ($prev == -o || $prev == --o) && $a == /?* ]]; then
      build_abs+=("$a")
      a=@RT@/.rt-abs/$k$slash
    elif [[ $a =~ ^(--?o=|[A-Z_]*BIN=)(/.+)$ ]]; then
      build_abs+=("${BASH_REMATCH[2]}")
      a=${BASH_REMATCH[1]}@RT@/.rt-abs/$k$slash
    fi
    build_cmd+=("$a")
    prev=$a
  done
}

# ci_repo: the repository whose Actions runs keep work local (ASD-V0-040): HEAVY_CI_REPO, else this
# checkout's origin when it is on github.com; nothing for HEAVY_CI_REPO=off or another origin.
ci_repo() {
  local r=${HEAVY_CI_REPO-} u
  if [[ -z $r ]]; then
    u=$(upstream "$(git rev-parse --show-toplevel 2> /dev/null)") || return 0
    u=${u%%$'\t'*}
    case $u in *github.com[:/]*) r=$(origin_key "$u") ;; *) return 0 ;; esac
  fi
  [[ $r == off ]] || printf '%s\n' "$r"
}

# ci_gh LIMIT GH... -- ARGS...: run the gh query with its stdin closed and stderr dropped. perl's
# alarm survives the exec, so a hung query ends at LIMIT seconds and fails.
ci_gh() {
  local limit=$1
  shift
  perl -e 'alarm shift; exec @ARGV or exit 127' "$limit" "$@" < /dev/null 2> /dev/null
}

# ci_busy: true, with the evidence in ci_why, when an Actions run of the CI repository has one of
# HEAVY_CI_STATUSES or the query fails, outlives HEAVY_CI_TIMEOUT or answers a non-number
# (ASD-V0-040). False when no repository is configured, under HEAVY_CI_GUARD=off, or under
# HEAVY_CI_GUARD=auto while the repository's CORVINT_SELF_HOSTED variable reads off.
ci_busy() {
  local repo s n total=0 limit=${HEAVY_CI_TIMEOUT:-20} statuses=${HEAVY_CI_STATUSES:-queued in_progress} gh
  local guard=${HEAVY_CI_GUARD:-auto}
  ci_why=''
  case $guard in auto | on) ;; off) return 1 ;; *) refuse "invalid-ci-guard HEAVY_CI_GUARD=$guard (want auto, on or off)" ;; esac
  repo=$(ci_repo)
  [[ -n $repo ]] || return 1
  [[ $repo =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || refuse "invalid-ci-repo HEAVY_CI_REPO=$repo (want owner/name or off)"
  [[ $limit =~ ^[1-9][0-9]{0,2}$ ]] || refuse "invalid-ci-timeout HEAVY_CI_TIMEOUT=$limit (want 1..999 seconds)"
  [[ $statuses =~ ^[[:space:]]*[a-z_]+([[:space:]]+[a-z_]+)*[[:space:]]*$ ]] ||
    refuse "invalid-ci-statuses HEAVY_CI_STATUSES=$statuses (want run statuses such as queued in_progress)"
  read -ra gh <<< "${HEAVY_CI_GH:-gh}"
  ((${#gh[@]})) || gh=(gh)
  # CI on GitHub-hosted runners does not use the remote; any other answer keeps the guard.
  [[ $guard == auto && $(ci_gh "$limit" "${gh[@]}" variable get CORVINT_SELF_HOSTED --repo "$repo") == off ]] && return 1
  for s in $statuses; do
    if ! n=$(ci_gh "$limit" "${gh[@]}" run list --repo "$repo" --status "$s" --limit 5 --json databaseId --jq length) ||
      [[ ! $n =~ ^[0-9]+$ ]]; then
      ci_why="repo=$repo status=$s query-failed"
      return 0
    fi
    total=$((total + n))
  done
  ((total > 0)) || return 1
  ci_why="repo=$repo runs=$total"
}

# ci_local COMMAND...: report and log a command kept local because CI runs (ASD-V0-040).
ci_local() {
  printf 'heavy.sh: CI is running or unknown (%s); not using %s, running locally\n' "$ci_why" "$HEAVY_REMOTE_HOST" >&2
  fallback_log "ci-running $ci_why" "$@"
  selection_log "$(date -u +%Y-%m-%dT%H:%M:%SZ) ci-busy host=$HEAVY_REMOTE_HOST $ci_why pwd=$PWD cmd=$*"
}

# build_local REASON COMMAND...: report and log a build kept local (ASD-V0-041).
build_local() {
  local why=$1
  shift
  printf 'heavy.sh: building locally (%s)\n' "$why" >&2
  fallback_log "build-local $why" "$@"
  selection_log "$(date -u +%Y-%m-%dT%H:%M:%SZ) build-local host=$HEAVY_REMOTE_HOST reason=$(printf %q "$why") pwd=$PWD cmd=$*"
}

# offload_traps: during an offload, an INT/TERM/HUP stops ssh and removes the payload.
offload_traps() {
  trap '/bin/rm -rf "${heavy_tmp-}"' EXIT
  trap 'heavy_abort 130' INT
  trap 'heavy_abort 143' TERM
  trap 'heavy_abort 129' HUP
}

# build_offload COMMAND...: one try at the build on the remote (ASD-V0-041). Returns 0 with the
# build's status in rc, or 1 when the build runs locally.
build_offload() {
  local max=${HEAVY_REMOTE_BUILD_MAX_LOAD:-16} a flags=()
  [[ $max =~ ^[0-9]{1,6}$ ]] || refuse "invalid-build-max-load HEAVY_REMOTE_BUILD_MAX_LOAD=$max (want 0..999999)"
  for a in "$@" "GOFLAGS=${GOFLAGS-}"; do [[ $a == GOFLAGS=* ]] && flags+=("$a"); done
  if writes_files "${flags[@]}"; then
    build_local "GOFLAGS writes files" "$@"
    return 1
  fi
  if ! build_map "$@"; then
    build_local "an argument has a .. component" "$@"
    return 1
  fi
  if ci_busy; then
    ci_local "$@"
    return 1
  fi
  offload_traps
  heavy_build=$max
  try_offload "${build_cmd[@]}"
  heavy_build=
  trap - EXIT INT TERM HUP
  case $rc in
    96 | 97)
      build_local "remote busy or unavailable (rc=$rc)" "$@"
      return 1
      ;;
  esac
}

# fallback_log REASON COMMAND...: append one line for a local run of an offloadable command.
fallback_log() {
  local reason=$1
  shift
  printf '%s pwd=%s reason=%s cmd=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$PWD" "$reason" "$*" \
    >> "${HEAVY_FALLBACK_LOG:-$lock_dir/fallback.log}" 2> /dev/null ||
    printf 'heavy.sh: cannot append to %s\n' "${HEAVY_FALLBACK_LOG:-$lock_dir/fallback.log}" >&2
}

# try_offload COMMAND...: offload, removing its scratch directory. Sets rc.
try_offload() {
  offload "$@"
  rc=$?
  /bin/rm -rf "${heavy_tmp-}"
}

# heavy_abort STATUS: the offload's INT/TERM/HUP handler. Stops ssh, removes the payload, exits.
heavy_abort() {
  [[ -n ${heavy_ssh-} ]] && kill "$heavy_ssh" 2> /dev/null
  [[ -n ${heavy_filter-} ]] && kill "$heavy_filter" 2> /dev/null
  /bin/rm -rf "${heavy_tmp-}"
  trap - EXIT
  exit "$1"
}

# writes_flag ARG: true for a go flag that writes a file the caller would read afterwards (a
# profile, a coverage file, a test binary).
writes_flag() {
  local f=$1
  [[ $f == --* ]] && f=${f#-}
  case $f in -o | -o=* | -outputdir* | -coverprofile* | -cpuprofile* | -memprofile* | -blockprofile* | \
    -mutexprofile* | -trace | -trace=* | -test.* | -c) return 0 ;;
  esac
  return 1
}

# writes_files ARG...: true when an argument, or a flag inside a GOFLAGS= argument, writes a file;
# such a command runs locally.
writes_files() {
  local a f flags
  for a in "$@"; do
    if [[ $a == GOFLAGS=* ]]; then
      read -ra flags <<< "${a#GOFLAGS=}"
      for f in ${flags[@]+"${flags[@]}"}; do writes_flag "$f" && return 0; done
    fi
    writes_flag "$a" && return 0
  done
  return 1
}

# darwin_only COMMAND...: print the Go files under the command's local package arguments that only
# build for darwin (V1-2552), at most one path per line; prints nothing on any error.
darwin_only() {
  local a d depth
  for a in "$@"; do
    case $a in . | ./*) ;; *) continue ;; esac
    d=${a%/...} depth=(-maxdepth 1)
    [[ $d != "$a" ]] && depth=()
    [[ -d $d ]] || continue
    find "$d" ${depth[@]+"${depth[@]}"} -type f -name '*.go' 2> /dev/null
  done | sort -u | while IFS= read -r a; do
    case ${a##*/} in
      *_darwin.go | *_darwin_test.go | *_darwin_*.go) printf '%s\n' "$a"; continue ;;
    esac
    grep -qE '^//go:build darwin( *&&.*)?$' "$a" 2> /dev/null && ! grep -qE '^//go:build .*\|\|' "$a" 2> /dev/null && printf '%s\n' "$a"
  done
}

# darwin_notice COMMAND...: before an offload, name the darwin-only files the remote will skip.
darwin_notice() {
  local files n
  files=$(darwin_only "$@")
  [[ -n $files ]] || return 0
  n=$(grep -c . <<< "$files")
  printf 'heavy.sh: offloading to %s, which neither builds nor runs %s darwin-only file(s), first %s; HEAVY_REMOTE=0 with HEAVY_LOCAL_REASON runs them here\n' \
    "$HEAVY_REMOTE_HOST" "$n" "$(head -n 1 <<< "$files")" >&2
  selection_log "$(date -u +%Y-%m-%dT%H:%M:%SZ) darwin-only host=$HEAVY_REMOTE_HOST n=$n first=$(head -n 1 <<< "$files") pwd=$PWD cmd=$*"
}

# selection_log LINE: append LINE and this runner's script=HASH12 to the selection log, or say it
# could not.
selection_log() {
  printf '%s script=%s\n' "$1" "${heavy_script:0:12}" >> "${HEAVY_SELECTION_LOG:-$lock_dir/selection.log}" 2> /dev/null ||
    printf 'heavy.sh: cannot append to %s\n' "${HEAVY_SELECTION_LOG:-$lock_dir/selection.log}" >&2
}

# small_package_bytes COMMAND...: print the total *_test.go bytes of the go test's packages and
# succeed when every package is a plain local directory under tools/ or conformance/ and the total
# is at most HEAVY_SMALL_TEST_BYTES (ASD-V0-074, V1-3120). Flags are skipped, with the value of a
# flag that takes one; any other package form (an import path, `...`, a file) fails.
small_package_bytes() {
  local limit=${HEAVY_SMALL_TEST_BYTES:-49152} seen=0 skip=0 prev_go=0 arg dir total=0 n=0 f size
  [[ $limit =~ ^[0-9]+$ ]] && ((limit > 0)) || return 1
  for arg in "$@"; do
    if ((!seen)); then [[ $arg == test && $prev_go == 1 ]] && seen=1; [[ $arg == go ]] && prev_go=1 || prev_go=0; continue; fi
    if ((skip)); then skip=0; continue; fi
    case $arg in
      -run | -skip | -timeout | -count | -p | -parallel | -cpu | -tags | -bench | -benchtime | -shuffle | -vet | -exec | -ldflags | -gcflags | -asmflags | -mod | -o | -coverpkg | -covermode | -list) skip=1 ;;
      -C | -C=*) return 1 ;;
      -*) ;;
      *)
        dir=${arg#./}
        [[ $arg == ./* && $dir != *...* && $dir != *..* && ($dir == tools/* || $dir == conformance/*) && -d $dir ]] || return 1
        n=$((n + 1))
        for f in "$dir"/*_test.go; do
          [[ -f $f ]] || continue
          size=$(wc -c < "$f" | tr -d ' ') || return 1
          total=$((total + size))
        done
        ;;
    esac
  done
  ((n > 0 && total <= limit)) || return 1
  printf '%s\n' "$total"
}

# run_selects_all COMMAND...: whether a -run value of the command matches every test (ASD-V0-860):
# its part before the first /, less one leading ^, is empty, ., .*, .+, Test, Test.* or Test.+.
run_selects_all() {
  local value next=""
  for arg in "$@"; do
    if [[ -n $next ]]; then
      value=$arg next=""
    else
      case $arg in
        -run | --run) next=1; continue ;;
        -run=* | --run=*) value=${arg#*=} ;;
        *) continue ;;
      esac
    fi
    value=${value%%/*}
    value=${value#^}
    case $value in '' | . | '.*' | '.+' | Test | 'Test.*' | 'Test.+') return 0 ;; esac
  done
  return 1
}

# manifest_perl: reads a corvint-test-manifest/0 (ARGV[0]) for manifest_admits and print_plan
# (ASD-V0-891). Mode "admit" (ARGV[1]) succeeds, printing the planDigest's first 12 hex digits, when
# source.head is ARGV[2] and every remaining package (an import path) is a unit with slice "whole",
# or, when the -skip pattern ARGV[3] is not empty, a unit of any slice with that -skip pattern. It
# fails with one `heavy.sh: HEAVY_SELECTION=PATH not admitted: REASON` stderr line (ASD-V0-722).
# Mode "plan" prints one heavy.sh command per go: unit under module ARGV[2], each slice with its own
# -run/-skip pattern and the repository's -timeout 30m (GO_TEST_TIMEOUT), or a skipped line.
manifest_perl='
use strict; use warnings; use JSON::PP;
my ($path, $mode, $key, $skip, @pkgs) = @ARGV;
sub nope { print STDERR $mode eq "admit" ? "heavy.sh: HEAVY_SELECTION=$ARGV[0] not admitted: $_[0]\n" : "heavy.sh: REFUSE $_[0]\n"; exit $_[1] }
open(my $fh, "<", $path) or nope("manifest-unreadable $path", 2);
my $m = eval { local $/; JSON::PP->new->decode(<$fh>) };
ref $m eq "HASH" && ($m->{profile} // "") eq "corvint-test-manifest/0" && ref $m->{units} eq "ARRAY"
  or nope("manifest-invalid $path (want corvint-test-manifest/0)", 2);
my $digest = ($m->{planDigest} // "") =~ /^sha256:([0-9a-f]{12})/ ? $1 : "unknown";
if ($mode eq "admit") {
  my $head = ref $m->{source} eq "HASH" ? $m->{source}{head} // "" : "";
  $head eq $key or nope("source.head $head is not HEAD $key (regenerate the manifest after this commit)", 1);
  my %admit = map { ($_->{unit} // "") => 1 } grep {
    $skip eq "" ? ($_->{slice} // "") eq "whole" : ($_->{flag} // "") eq "-skip" && ($_->{pattern} // "") eq $skip
  } @{ $m->{units} };
  $admit{"go:$_"} or nope($skip eq "" ? "unit go:$_ is not selected whole" : "unit go:$_ has no -skip $skip slice", 1) for @pkgs;
  print "$digest\n"; exit 0;
}
sub sq { (my $v = shift) =~ s/\x27/\x27\\\x27\x27/g; "\x27$v\x27" } # shell single quotes
my $n = 0;
for my $u (@{ $m->{units} }) {
  my $unit = $u->{unit} // "";
  next unless $unit =~ /^go:(.+)$/;
  my $ip = $1;
  my $rel = $ip eq $key ? "." : index($ip, "$key/") == 0 ? "./" . substr($ip, length($key) + 1) : undef;
  if (!defined $rel) { print "# skipped $unit: outside $key\n"; next }
  my ($slice, $flag, $pattern, $sel) = ($u->{slice} // "", $u->{flag} // "", $u->{pattern} // "", "");
  if ($slice ne "whole") {
    if (($flag ne "-run" && $flag ne "-skip") || $pattern eq "") { print "# skipped $unit: slice $slice has no -run/-skip pattern\n"; next }
    $sel = " $flag " . sq($pattern);
  }
  print "HEAVY_SELECTION=", sq($path), " script/heavy.sh go test -count=1 -timeout 30m$sel $rel\n"; $n++;
}
print STDERR "heavy.sh: plan $n units from $path (planDigest $digest)\n";
'

# module_path: the module path in ./go.mod, or nothing.
module_path() {
  awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' go.mod 2> /dev/null
}

# not_admitted REASON: the one ASD-V0-722 stderr line naming why HEAVY_SELECTION admits nothing.
not_admitted() {
  printf 'heavy.sh: HEAVY_SELECTION=%s not admitted: %s\n' "$HEAVY_SELECTION" "$1" >&2
}

# selection_manifest PATH: PATH, or, when PATH is an unreadable relative .git/REL (a linked
# worktree's .git is a file), Git's own path for REL (ASD-V0-722).
selection_manifest() {
  local resolved
  if [[ ! -r $1 && $1 == .git/?* ]] && resolved=$(git rev-parse --path-format=absolute --git-path "${1#.git/}" 2> /dev/null); then
    printf '%s\n' "$resolved"
  else
    printf '%s\n' "$1"
  fi
}

# compile_only COMMAND...: succeeds when the go test arguments (up to -args) include -c, which
# builds the test binary and runs no test (ASD-V0-721).
compile_only() {
  local seen=0 prev_go=0 arg
  for arg in "$@"; do
    if ((!seen)); then [[ $arg == test && $prev_go == 1 ]] && seen=1; [[ $arg == go ]] && prev_go=1 || prev_go=0; continue; fi
    case $arg in
      -args | --args) return 1 ;;
      -c | -c=true | --c | --c=true) return 0 ;;
    esac
  done
  return 1
}

# manifest_admits COMMAND...: ASD-V0-891. Succeeds, printing the manifest's planDigest prefix, when
# HEAVY_SELECTION is a manifest of HEAD that selects every package argument whole, or, with one
# -skip PATTERN, lists that -skip slice for each; each argument must be ./REL (or .) with no ..., ..
# or -C. Other flags are skipped as in small_package_bytes.
manifest_admits() {
  local seen=0 skip=0 prev_go=0 arg dir mod head manifest pkgs=() skips=()
  mod=$(module_path) && [[ -n $mod ]] || { not_admitted "./go.mod names no module"; return 1; }
  head=$(git rev-parse HEAD 2> /dev/null) && [[ -n $head ]] || { not_admitted "git rev-parse HEAD failed"; return 1; }
  for arg in "$@"; do
    if ((!seen)); then [[ $arg == test && $prev_go == 1 ]] && seen=1; [[ $arg == go ]] && prev_go=1 || prev_go=0; continue; fi
    if ((skip)); then ((skip == 2)) && skips+=("$arg"); skip=0; continue; fi
    case $arg in
      -skip) skip=2 ;;
      -skip=*) skips+=("${arg#-skip=}") ;;
      -run | -timeout | -count | -p | -parallel | -cpu | -tags | -bench | -benchtime | -shuffle | -vet | -exec | -ldflags | -gcflags | -asmflags | -mod | -o | -coverpkg | -covermode | -list) skip=1 ;;
      -C | -C=*) not_admitted "argument $arg is not a ./REL directory"; return 1 ;;
      -*) ;;
      .) pkgs+=("$mod") ;;
      *)
        dir=${arg#./}
        dir=${dir%/}
        [[ $arg == ./* && -n $dir && $dir != *...* && $dir != *..* && -d $dir ]] || { not_admitted "argument $arg is not a ./REL directory"; return 1; }
        pkgs+=("$mod/$dir")
        ;;
    esac
  done
  ((${#pkgs[@]})) || { not_admitted "no package argument"; return 1; }
  ((${#skips[@]} < 2)) || { not_admitted "more than one -skip"; return 1; }
  manifest=$(selection_manifest "$HEAVY_SELECTION")
  perl -e "$manifest_perl" "$manifest" admit "$head" "${skips[0]-}" "${pkgs[@]}"
}

# print_plan MANIFEST: ASD-V0-891 --plan; prints the per-package heavy.sh commands, runs nothing.
print_plan() {
  local mod
  mod=$(module_path) && [[ -n $mod ]] || refuse "no-module ./go.mod names no module; run --plan from the module root"
  perl -e "$manifest_perl" "$(selection_manifest "$1")" plan "$mod"
}

# selection_policy COMMAND...: log every go test and refuse a whole-package one (no -run, or a -run
# that matches every test) without HEAVY_WHOLE_REASON (ASD-V0-030, ASD-V0-860). Returns 1 on refusal.
selection_policy() {
  local now
  case " $* " in *" go test "*) ;; *) return 0 ;; esac
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if compile_only "$@"; then
    selection_log "$now whole reason=compile-only pwd=$PWD cmd=$*"
    return 0
  fi
  case " $* " in
    *" -run "* | *" -run="* | *" --run "* | *" --run="*)
      if ! run_selects_all "$@"; then
        selection_log "$now selected pwd=$PWD cmd=$*"
        return 0
      fi
      ;;
  esac
  local small digest
  if [[ -z ${HEAVY_WHOLE_REASON-} ]] && small=$(small_package_bytes "$@"); then
    selection_log "$now whole reason=small-package:${small}B pwd=$PWD cmd=$*"
    return 0
  fi
  if [[ -z ${HEAVY_WHOLE_REASON-} && -n ${HEAVY_SELECTION-} ]] && digest=$(manifest_admits "$@"); then
    selection_log "$now whole reason=corvint-selection:$digest pwd=$PWD cmd=$*"
    return 0
  fi
  if [[ -z ${HEAVY_WHOLE_REASON-} ]]; then
    printf '%s\n' "heavy.sh: refused: whole-package go test without HEAVY_WHOLE_REASON. Run only Corvint's selection: corvint test --base <origin/main sha> --dry-run --manifest M, then script/heavy.sh --plan M prints one command per package, each with HEAVY_SELECTION=M, which admits a package the manifest selected whole (ASD-V0-891). If Corvint reported UNKNOWN/FALLBACK for it, rerun with HEAVY_WHOLE_REASON='<that reason>'. A tools/ or conformance/ package with at most ${HEAVY_SMALL_TEST_BYTES:-49152} bytes of tests needs no reason." >&2
    selection_log "$now refused pwd=$PWD cmd=$*"
    return 1
  fi
  selection_log "$now whole reason=$(printf %q "$HEAVY_WHOLE_REASON") pwd=$PWD cmd=$*"
}

# local_policy COMMAND...: refuse an offloadable command under HEAVY_REMOTE=0 without
# HEAVY_LOCAL_REASON (ASD-V0-030). Returns 1 on refusal.
local_policy() {
  [[ ${HEAVY_REMOTE:-1} == 0 ]] && offloadable "$@" || return 0
  [[ -n ${HEAVY_LOCAL_REASON-} ]] && return 0
  printf '%s\n' "heavy.sh: refused: HEAVY_REMOTE=0 without HEAVY_LOCAL_REASON. Drop HEAVY_REMOTE=0 so the run offloads (flake reruns and -count=N included). Only a test that needs this host may run locally: rerun with HEAVY_LOCAL_REASON='<why it needs this host>'." >&2
  fallback_log refused-local "$@"
  return 1
}

# load1: the 1-minute load average as an integer (sysctl, else /proc/loadavg); empty when unreadable.
load1() {
  local l
  l=$(sysctl -n vm.loadavg 2> /dev/null | awk '{ print int($2) }')
  [[ -n $l ]] || l=$(awk '{ print int($1) }' /proc/loadavg 2> /dev/null)
  printf '%s' "$l"
}

# load_wait: before a local run, wait while the load exceeds HEAVY_MAX_LOAD, for at most
# HEAVY_LOAD_WAIT seconds (ASD-V0-030).
load_wait() {
  local max=${HEAVY_MAX_LOAD:-10} limit=${HEAVY_LOAD_WAIT:-120} t0=$SECONDS l
  [[ $max =~ ^[0-9]{1,6}$ ]] || refuse "invalid-max-load HEAVY_MAX_LOAD=$max (want 0..999999)"
  [[ $limit =~ ^[0-9]{1,5}$ ]] || refuse "invalid-load-wait HEAVY_LOAD_WAIT=$limit (want 0..99999 seconds)"
  while l=$(load1) && [[ $l =~ ^[0-9]+$ ]] && ((l > max && SECONDS - t0 < limit)); do
    # Never sleep past the bound (V1-3232): a 1-second bound waits 1 second, not one 5-second poll.
    sleep $((limit - (SECONDS - t0) < 5 ? limit - (SECONDS - t0) : 5))
  done
  ((SECONDS > t0)) && printf 'heavy.sh: waited %ss for host load %s (HEAVY_MAX_LOAD=%s)\n' "$((SECONDS - t0))" "$(load1)" "$max" >&2
  return 0
}

# script_hash FILE: FILE's SHA-256 in hex, or unknown.
script_hash() {
  local h
  h=$({ shasum -a 256 "$1" || sha256sum "$1"; } 2> /dev/null | awk 'NR == 1 { print $1 }')
  [[ $h =~ ^[0-9a-f]{64}$ ]] || h=unknown
  printf '%s' "$h"
}

# fallback_alert: after a fallback to a local run, alert when the last HEAVY_FALLBACK_ALERT
# fallbacks for this host had no successful offload between them (ASD-V0-037).
fallback_alert() {
  local n
  ((fallback_limit > 0)) || return 0
  n=$(tail -n 2000 "${HEAVY_SELECTION_LOG:-$lock_dir/selection.log}" 2> /dev/null | awk -v h="host=$HEAVY_REMOTE_HOST" '
    $3 != h { next }
    $2 == "fallback" { n++; next }
    $2 == "offload" {
      rc = ""
      for (i = 4; i <= NF; i++) if ($i ~ /^rc=/) { rc = substr($i, 4); break }
      if (rc != 97 && rc != 255) n = 0
    }
    END { print n + 0 }')
  ((n >= fallback_limit)) || return 0
  printf 'heavy.sh: ALERT remote-unavailable streak=%s host=%s: the last %s offloads ran locally; check the host (for a WSL box, script/wsl-keepalive-check.sh)\n' \
    "$n" "$HEAVY_REMOTE_HOST" "$n" >&2
  selection_log "$(date -u +%Y-%m-%dT%H:%M:%SZ) alert remote-unavailable streak=$n host=$HEAVY_REMOTE_HOST pwd=$PWD"
}

main() {
  if [[ -n ${__heavy_serve-} ]]; then
    serve
    return
  fi
  if [[ -n ${__heavy_fetch-} ]]; then
    fetch
    return
  fi
  if [[ $# -eq 0 || $1 == -h || $1 == --help ]]; then
    awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "${BASH_SOURCE[0]}"
    [[ $# -eq 0 ]] && return 2
    return 0
  fi
  # ASD-V0-070 (V1-3233): with no HOME and no XDG_CONFIG_HOME there is no default config file.
  local config_home=${XDG_CONFIG_HOME:-${HOME:+$HOME/.config}}
  load_config "${HEAVY_CONFIG:-${config_home:+$config_home/corvint/heavy.env}}"
  local cpus slots par nice_level lock_dir rc retry
  lock_dir=${HEAVY_LOCK_DIR:-/tmp/corvint-heavy-$(id -u)}
  if [[ $1 == --plan ]]; then
    [[ $# -eq 2 ]] || refuse "invalid-arguments --plan takes one manifest path"
    print_plan "$2"
    return
  fi
  if [[ $1 == --print-lock-dir ]]; then
    [[ $# -eq 1 ]] || refuse "invalid-arguments --print-lock-dir takes no command"
    printf '%s\n' "$lock_dir"
    return 0
  fi
  # ASD-V0-830 (V1-3256): leading NAME=VALUE words are environment, as for env(1), so
  # `heavy.sh GOTOOLCHAIN=local go test ...` runs `env GOTOOLCHAIN=local go test ...` locally and
  # remotely instead of failing with `cannot run` after the load wait.
  [[ $1 =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] && set -- env "$@"
  heavy_script=$(script_hash "${BASH_SOURCE[0]}")
  printf 'heavy.sh: script sha256=%s %s\n' "$heavy_script" "${BASH_SOURCE[0]}" >&2
  cpus=$(cpu_count)
  slot_limits "$cpus"
  mkdir -p "$lock_dir" 2> /dev/null || refuse "lock-dir-unavailable $lock_dir"
  # ASD-V0-890 (V1-3506): evaluate both reason rules, then refuse once naming every missing reason.
  local missing=()
  selection_policy "$@" || missing+=(whole-reason-missing)
  local_policy "$@" || missing+=(local-reason-missing)
  if ((${#missing[@]})); then
    ((${#missing[@]} > 1)) && printf 'heavy.sh: refused: set both HEAVY_WHOLE_REASON and HEAVY_LOCAL_REASON\n' >&2
    printf 'heavy.sh: REFUSED %s (exit 2)\n' "$(IFS=+; printf '%s' "${missing[*]}")" # stdout marker (V1-3213)
    heavy_cause=refused
    return 2
  fi
  if [[ -n ${HEAVY_REMOTE_HOST-} ]] && offloadable "$@"; then
    if [[ ${HEAVY_REMOTE:-1} == 0 ]]; then
      fallback_log "HEAVY_REMOTE=0 local_reason=$(printf %q "$HEAVY_LOCAL_REASON")" "$@"
    elif writes_files "$@" "GOFLAGS=${GOFLAGS-}"; then
      fallback_log writes-files "$@"
    elif ci_busy; then
      ci_local "$@"
    else
      retry=${HEAVY_REMOTE_RETRY:-15}
      [[ $retry =~ ^[0-9]{1,3}$ ]] || refuse "invalid-retry HEAVY_REMOTE_RETRY=$retry (want 0..999 seconds)"
      fallback_limit=${HEAVY_FALLBACK_ALERT:-3}
      [[ $fallback_limit =~ ^[0-9]{1,3}$ ]] && ((10#$fallback_limit <= 100)) || refuse "invalid-fallback-alert HEAVY_FALLBACK_ALERT=$fallback_limit (want 0..100)"
      [[ ${HEAVY_REMOTE_GOMAXPROCS:-0} =~ ^(0|[1-9][0-9]{0,3})$ ]] || refuse "invalid-remote-gomaxprocs HEAVY_REMOTE_GOMAXPROCS=$HEAVY_REMOTE_GOMAXPROCS (want 0..9999)"
      darwin_notice "$@"
      offload_traps
      try_offload "$@"
      if ((rc == 97)) && [[ -z ${heavy_noretry-} ]]; then
        printf 'heavy.sh: offload to %s unavailable; retrying in %ss\n' "$HEAVY_REMOTE_HOST" "$retry" >&2
        sleep "$retry" &
        wait $!
        try_offload "$@"
      fi
      trap - EXIT INT TERM HUP
      ((rc == 97)) || { heavy_cause="offload host=$HEAVY_REMOTE_HOST"; return "$rc"; }
      printf 'heavy.sh: offload to %s unavailable; running locally (logged)\n' "$HEAVY_REMOTE_HOST" >&2
      fallback_log offload-unavailable "$@"
      selection_log "$(date -u +%Y-%m-%dT%H:%M:%SZ) fallback host=$HEAVY_REMOTE_HOST pwd=$PWD cmd=$*"
      fallback_alert
    fi
  elif [[ -n ${HEAVY_REMOTE_HOST-} && ${HEAVY_REMOTE:-1} != 0 && ${HEAVY_BUILD_OFFLOAD:-1} != 0 ]] && buildable "$@"; then
    build_offload "$@" && { heavy_cause="offload host=$HEAVY_REMOTE_HOST"; return "$rc"; }
  fi
  load_wait
  pin_go
  bound_parallelism "$par"
  local_identity "$@"
  # ASD-V0-038 (V1-2904): one Go build cache per host. Unless the caller sets GOCACHE, a local run
  # uses the cache the Makefile uses (CORVINT_GOCACHE, default /tmp/corvint-go-build-cache), so a
  # plain go test and a make target do not fill two caches.
  : "${GOCACHE:=${CORVINT_GOCACHE:-/tmp/corvint-go-build-cache}}"
  export GOCACHE
  # exec, as before: killing the wrapper kills the semaphore holder while the workload keeps the
  # inherited slot (ASD-V0-014). The holder appends the run's status line (ASD-V0-037).
  HEAVY_RUN_LOG=${HEAVY_SELECTION_LOG:-$lock_dir/selection.log} HEAVY_RUN_TAG=" script=${heavy_script:0:12}" \
    exec perl -e "$slot_perl" "$lock_dir" "$slots" "$nice_level" "$@"
}

# heavy_exit RC: ASD-V0-940. The calling shell names the cause of a non-zero main; the remote half
# leaves that to its slot holder, whose line is relayed.
heavy_exit() {
  if (($1 != 0)) && [[ -z ${__heavy_serve-}${__heavy_fetch-} ]]; then
    printf 'heavy.sh: exit rc=%d cause=%s\n' "$1" "${heavy_cause:-wrapper}" >&2
  fi
  exit "$1"
}

main "$@"; heavy_exit $?
