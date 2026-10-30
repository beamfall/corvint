# Quiet benchmark host: WSL2 Ubuntu (V1-2442)

Performance and latency targets that need a quiet host run here, not on a lane host. The lane Mac
runs at load average 25 to 60, and CI runners are shared, so their numbers show shape only. The
measured results are in build-log entries. This page describes how the host is set up and how a run
is made, so that the next run is comparable.

## Rules

- **WSL2 numbers are their own baseline.** Label every number `WSL2/Linux`. Never compare one with a
  macOS, native Windows or CI number, and never use one as a gate baseline for another platform.
  You may compare two WSL2 runs from this host, binaries built on it, and alternating pairs within
  one run.
- **The clone lives on the WSL ext4 filesystem** (`~/bench`, on `/dev/sdd`), never under `/mnt/c`.
  Set `TMPDIR` to `~/bench/tmp`: the distro's `/tmp` is a 16 GB tmpfs, and the store harnesses
  create their fixtures under `os.MkdirTemp("")`.
- **No native Windows workload runs during a benchmark:** no Windows tests, builds or indexers
  started by an agent. The owner's own desktop use is outside our control. The idle check records
  Windows CPU, and a run that started on a busy host is marked.
- **Windows system settings are not changed** (power plan, WSL memory or CPU limits, Defender)
  without the owner's approval. The idle check records the power plan as it is.
- **One measurement at a time.** The runner is serial; do not start a second one beside it.

## Host

| Item | Value (2026-10-23) |
| --- | --- |
| Access | `ssh russe@192.168.4.128` (key auth); the login shell is PowerShell; `wsl -d Ubuntu -- bash -lc '...'` |
| CPU | AMD Ryzen 9 5950X, 16 cores / 32 threads; WSL2 sees 32 logical CPUs |
| Memory | 64 GB host; WSL2 default cap, 31 GiB visible |
| OS | Windows 10.0.26200.9168; WSL 2.7.13.0; kernel `6.18.33.2-microsoft-standard-WSL2`; Ubuntu 26.04 LTS |
| Power | Windows plan `Winhance Power Plan` (GUID `57696e68-616e-6365-506f-776572000000`); WSL2 does not expose `cpufreq`, so there is no Linux governor |
| Go | go1.27.3 linux/amd64 at `~/sdk/go1.27.3` and `/usr/local/go` (both byte-identical to the official `go1.27.3.linux-amd64.tar.gz`, sha256 `65efbc80…9b40`, verified 2026-10-25 for V1-2556). The distro `go` on a non-login shell's `PATH` is apt `/usr/bin/go` (go1.26.8); `/etc/profile.d/corvint-go.sh` prepends `/usr/local/go/bin` for login shells only. The 2026-10-23 inventory recorded go1.27.2. |

"Load at or below the core count" (decision 0549's bound) means a 1-minute load at or below 16 here.
The idle check's own bar is much lower than that.

## Setup

1. **Go 1.27.3** (decision 0557; the 2026-10-23 inventory above records the go1.27.2 install that
   preceded it). Download `go1.27.3.linux-amd64.tar.gz` from `https://go.dev/dl/`. Check it against
   the sha256 listed at `https://go.dev/dl/?mode=json&include=all`
   (`65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40`) with `sha256sum -c`, then
   extract it to `~/sdk/go1.27.3`. Runs set `PATH=$HOME/sdk/go1.27.3/bin:$PATH GOTOOLCHAIN=local`.
   A `go` earlier on `PATH` that reports go1.27.2 (for example `/usr/local/go`) refuses to build the
   `go 1.27.3` module directive.
   The heavy.sh offload runner (`/private/tmp/cem10-build/heavy.sh` on the Mac; `~/bench/rt.sh` here)
   sources `~/bench/go.env`, which is exactly that `PATH`/`GOTOOLCHAIN` line, so an offloaded `go test`
   and every `go` subprocess the tests start resolve to go1.27.3 regardless of the login-shell
   `PATH` (V1-2556; the runner banner prints the `go version` it resolved).
2. **Checkout.** `~/corvint` is the owner's clone of `beamfall/corvint`, with the Windows git
   credential manager configured. Each measurement uses a detached worktree at a named `origin/main`
   sha, for example `git -C ~/corvint worktree add --detach ~/bench/corvint <sha>`. Comparison
   binaries get their own worktrees, `~/bench/wt-<name>`.
3. **Idle check.** `~/bench/idle-check.sh`, given below, runs before every step. It prints one JSON
   line and exits 1 when the host is not quiet: WSL load1 above 1.0 or Windows CPU above 10%,
   averaged over three one-second `typeperf` samples. It records the WSL kernel, the CPU count, the
   load averages, the run queue, the governor (or `NOT_EXPOSED_IN_WSL2`), the Windows power plan, the
   Windows CPU, the free memory and the Go version.
4. **Runner.** Keep one serial runner script that builds the test binaries once with `go test -c`
   and then runs each step after an idle check. Write the outputs to `~/bench/results/`. Start it
   from the Mac over one SSH session that stays open, as a background job. If the last `wsl.exe`
   client exits, WSL may stop the distro, so a detached `nohup` alone is not enough.

```bash
#!/usr/bin/env bash
# Quiet-host idle check for WSL2 benchmark runs (V1-2442). Prints one JSON line and
# exits 1 when the host is not quiet. Run immediately before each measurement.
set -u
max_load=${IDLE_MAX_LOAD1:-1.0}      # WSL 1-minute load average ceiling
max_win=${IDLE_MAX_WIN_CPU:-10}      # Windows total CPU % ceiling (3 one-second samples)
read -r l1 l5 l15 rq _ < /proc/loadavg
cpus=$(nproc)
gov=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || echo NOT_EXPOSED_IN_WSL2)
plan=$(/mnt/c/Windows/System32/powercfg.exe /getactivescheme 2>/dev/null | tr -d '\r' | sed 's/^Power Scheme GUID: //')
win=$(/mnt/c/Windows/System32/typeperf.exe '\Processor(_Total)\% Processor Time' -sc 3 2>/dev/null | tr -d '\r' |
  awk -F'","' 'NR>2 && NF>1 {gsub(/"/,"",$2); s+=$2; n++} END {if (n) printf "%.1f", s/n; else print "NOT_OBSERVED"}')
mem=$(awk '/MemAvailable/ {printf "%d", $2/1024}' /proc/meminfo)
quiet=true
awk -v a="$l1" -v b="$max_load" 'BEGIN {exit !(a <= b)}' || quiet=false
[ "$win" = NOT_OBSERVED ] || awk -v a="$win" -v b="$max_win" 'BEGIN {exit !(a <= b)}' || quiet=false
printf '{"at":"%s","host":"%s","platform":"WSL2/Linux","kernel":"%s","cpus":%s,"load1":%s,"load5":%s,"load15":%s,"runQueue":"%s","governor":"%s","windowsPowerPlan":"%s","windowsCpuPct":"%s","memAvailableMiB":%s,"go":"%s","quiet":%s}\n' \
  "$(date -u +%FT%TZ)" "$(hostname)" "$(uname -r)" "$cpus" "$l1" "$l5" "$l15" "$rq" "$gov" "$plan" "$win" "$mem" \
  "$(go version 2>/dev/null | awk '{print $3}')" "$quiet"
$quiet
```

## Running a measurement

Each opt-in harness is a test binary built at the named sha. Run it from its package directory, with
`-test.count=1 -test.timeout=0 -test.v`, and with its report path set:

| Ticket | Harness | Settings used |
| --- | --- | --- |
| V1-2205 / V1-2203 (B1) | `internal/tasks/store` `TestCALV0118_WriterHoldProfile` | `CORVINT_TASKS_WRITER_HOLD_PROFILE=<report>`, `CORVINT_TASKS_HISTORY_SIZES=3500,13500,40000`, `CORVINT_TASKS_HISTORY_TICKETS=880`, `CORVINT_TASKS_WRITER_HOLD_REPS=5`, three runs |
| V1-2205 / V1-2385 (B2) | `internal/tasks/store` `TestCALV0263_LeaseGateLoad` | `CORVINT_TASKS_LEASE_GATE_LOAD=<report>`, `_RECEIPTS=40000`, `_WRITERS=15`, `_SECONDS=120`, three runs |
| V1-1569 | `internal/tasks/store` `TestPSRV0055_PoolAcquireLoad` | `CORVINT_TASKS_POOL_ACQUIRE_LOAD=<report>`, defaults: 45,000 receipts, 8 acquirers, 600 s, 300 s grace |
| V1-2300 | `corvint --root <corpus> index` | Ten rounds, each running base `ef2a01db5`, seal `9080ab596` and main, in turn. Before each run the corpus's shared snapshot store `<git-common-dir>/corvint/index` is removed. The corpus is a clone at `ef2a01db5`. Timing is `/usr/bin/time` `%e %U %S`. |
| V1-2347 | `internal/opencodequalification` `sampleBaseline` | Five 60 s windows of the production AHI-172 probe sampler, driven by a scratch in-package test that is not committed |
| V1-2399 | `internal/procgroup` `TestPGOV0022*` and `cmd/corvint` `TestSourceViewCancellationTerminatesGitProcessGroup` | `-test.count=5` and `-test.count=10`; the source-view test also runs at the pre-fix main (`c7505ee38^1`) |

The page cache stays warm. Dropping it needs root, which the runner does not use. Record medians and
spread across repetitions, and keep each step's idle-check JSON with its result.
