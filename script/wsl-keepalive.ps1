# script/wsl-keepalive.ps1 (ASD-V0-039, V1-2888): keep one WSL distro running on a Windows host
# that serves remote heavy runs, so the distro does not stop when the last ssh session exits.
#
# Usage (on the Windows host, normally as a boot-triggered scheduled task; see
# docs/LANE-ORCHESTRATION.md "WSL keepalive" for the owner's install and rollback commands):
#   powershell -NoProfile -ExecutionPolicy Bypass -File wsl-keepalive.ps1 [-Distro Ubuntu]
#     [-RetrySeconds 30] [-Log PATH]
#
# The script is the Windows-side holder: it runs `wsl.exe -d DISTRO --exec /bin/sleep infinity`
# and, whenever that holder exits (a `wsl --shutdown`, a WSL update, the distro crashing), starts it
# again after -RetrySeconds, so an exit is recovered here rather than by the task scheduler's
# restart-on-failure, which a holder's ordinary exit does not trigger. A named mutex keeps one copy
# per distro per Windows session; a second copy exits 0 at once. Each start and exit is appended
# to -Log (default %LOCALAPPDATA%\corvint\wsl-keepalive-DISTRO.log) as one UTC line. It changes no
# WSL or Windows configuration and never stops a running distro.
#
# Exit status: 0 another copy already holds the distro; 2 an invalid argument. A running holder
# loop does not exit on its own.
param(
  [string]$Distro = "Ubuntu",
  [int]$RetrySeconds = 30,
  [string]$Log = ""
)
$ErrorActionPreference = "Stop"
if ($Distro -notmatch '^[A-Za-z0-9._-]{1,64}$' -or $RetrySeconds -lt 1 -or $RetrySeconds -gt 3600) {
  [Console]::Error.WriteLine("wsl-keepalive: REFUSE invalid-argument distro=$Distro retry=$RetrySeconds")
  exit 2
}
if ($Log -eq "") {
  $Log = Join-Path $env:LOCALAPPDATA "corvint\wsl-keepalive-$Distro.log"
}
$Log = [IO.Path]::GetFullPath($Log)   # a relative -Log has no parent directory to create
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Log) | Out-Null
function Write-Line([string]$text) {
  $stamp = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
  Add-Content -Path $Log -Value "$stamp $text"
}

$created = $false
$mutex = New-Object System.Threading.Mutex($true, "Local\corvint-wsl-keepalive-$Distro", [ref]$created)
if (-not $created) {
  Write-Line "already-held distro=$Distro pid=$PID"
  exit 0
}
$wsl = Join-Path $env:SystemRoot "System32\wsl.exe"
while ($true) {
  Write-Line "start distro=$Distro pid=$PID"
  $holder = Start-Process -FilePath $wsl -ArgumentList @("-d", $Distro, "--exec", "/bin/sleep", "infinity") -NoNewWindow -PassThru
  $holder.WaitForExit()
  Write-Line "exit distro=$Distro holder=$($holder.Id) code=$($holder.ExitCode) retry=${RetrySeconds}s"
  Start-Sleep -Seconds $RetrySeconds
}
