#!/usr/bin/env bash
# script/wsl-keepalive-check.sh (ASD-V0-039, V1-2888): report whether a WSL distro on a Windows
# host has a Windows-side holder, the process that keeps it running after every ssh session exits.
#
# Usage: script/wsl-keepalive-check.sh [--host USER@HOST] [--distro NAME] [--task NAME]
#   --host    the Windows ssh target (default $HEAVY_KEEPALIVE_HOST, else russe@192.168.4.128)
#   --distro  the distro to check (default Ubuntu)
#   --task    the scheduled task expected to own the holder (default CorvintWslKeepAlive)
#
# It runs one read-only PowerShell query over ssh (Win32_Process and Get-ScheduledTask; nothing is
# started, stopped or changed) and prints:
#   holder pid=P parent=Q name=wsl.exe        one per `wsl.exe -d DISTRO ... sleep infinity`
#                                             holder (its own wsl.exe child is not counted)
#   task name=T state=S last-result=R          the scheduled task, or `task name=T state=missing`
#   wsl-keepalive-check: OK holders=N host=H distro=D
#   wsl-keepalive-check: FAIL no-holder host=H distro=D   (the distro stops when the last session
#                                             exits; install the task, docs/LANE-ORCHESTRATION.md)
#   wsl-keepalive-check: WARN task-not-running ...        holders exist but none is owned by the
#                                             task, so the next reboot leaves the distro unheld
# Exit status: 0 OK (WARN included), 1 no holder, 2 usage, 3 the host could not be queried (ssh
# failed, the answer was truncated, or the process query raised an error).
set -uo pipefail
export LC_ALL=C

host=${HEAVY_KEEPALIVE_HOST:-russe@192.168.4.128} distro=Ubuntu task=CorvintWslKeepAlive
while (($#)); do
  case $1 in
    --host | --distro | --task)
      [[ $# -ge 2 ]] || { printf 'wsl-keepalive-check: REFUSE missing-value %s\n' "$1" >&2; exit 2; }
      case $1 in --host) host=$2 ;; --distro) distro=$2 ;; --task) task=$2 ;; esac
      shift 2 ;;
    -h | --help) sed -n '2,22p' "$0"; exit 0 ;;
    *) printf 'wsl-keepalive-check: REFUSE unknown-argument %s\n' "$1" >&2; exit 2 ;;
  esac
done
[[ $distro =~ ^[A-Za-z0-9._-]{1,64}$ && $task =~ ^[A-Za-z0-9._\ -]{1,64}$ && -n $host ]] || {
  printf 'wsl-keepalive-check: REFUSE invalid-argument host=%s distro=%s task=%s\n' "$host" "$distro" "$task" >&2
  exit 2
}

# The query is sent as -EncodedCommand (UTF-16LE base64), so no quoting reaches cmd.exe.
ps1=$(cat <<'PS'
$ProgressPreference = "SilentlyContinue"; $ErrorActionPreference = "Stop"
$d = "@DISTRO@"; $n = "@TASK@"
try { $all = @(Get-CimInstance Win32_Process) } catch { "query-error"; exit 1 }
$h = @($all | Where-Object {
  $_.Name -eq "wsl.exe" -and $_.CommandLine -match ("-d\s+" + [regex]::Escape($d) + "\s") -and $_.CommandLine -match "sleep\s+infinity" })
$ids = @($h | ForEach-Object { $_.ProcessId })
$h | Where-Object { $ids -notcontains $_.ParentProcessId } |
  ForEach-Object { "holder pid={0} parent={1} name={2}" -f $_.ProcessId, $_.ParentProcessId, $_.Name }
$t = Get-ScheduledTask -TaskName $n -ErrorAction SilentlyContinue
if ($t) { $r = try { (Get-ScheduledTaskInfo -TaskName $n).LastTaskResult } catch { "unknown" }
  "task name={0} state={1} last-result={2}" -f ($n -replace " ", "_"), $t.State, $r }
else { "task name={0} state=missing" -f ($n -replace " ", "_") }
"end"
PS
)
ps1=${ps1//@DISTRO@/$distro} ps1=${ps1//@TASK@/$task}
enc=$(printf '%s' "$ps1" | iconv -f UTF-8 -t UTF-16LE | base64 | tr -d '\n') || exit 3
out=$(ssh -o BatchMode=yes -o ConnectTimeout=10 "$host" "powershell -NoProfile -NonInteractive -EncodedCommand $enc" < /dev/null 2> /dev/null | tr -d '\r')
rc=$?
lines=$(grep -E '^(holder|task) ' <<< "$out")
if grep -qx query-error <<< "$out"; then
  printf 'wsl-keepalive-check: FAIL query-error host=%s: the Win32_Process query raised an error\n' "$host" >&2
  exit 3
fi
if [[ $rc -ne 0 ]] || ! grep -qx end <<< "$out"; then
  printf 'wsl-keepalive-check: FAIL host-unreachable host=%s rc=%s\n' "$host" "$rc" >&2
  exit 3
fi
[[ -n $lines ]] && printf '%s\n' "$lines"
holders=$(grep -c '^holder ' <<< "$lines")
if [[ $holders -eq 0 ]]; then
  printf 'wsl-keepalive-check: FAIL no-holder host=%s distro=%s: the distro stops when the last session exits; install the task (docs/LANE-ORCHESTRATION.md "WSL keepalive")\n' "$host" "$distro"
  exit 1
fi
grep -q "^task .* state=Running" <<< "$lines" ||
  printf 'wsl-keepalive-check: WARN task-not-running host=%s task=%s: holders exist, but none survives a reboot\n' "$host" "$task"
printf 'wsl-keepalive-check: OK holders=%s host=%s distro=%s\n' "$holders" "$host" "$distro"
