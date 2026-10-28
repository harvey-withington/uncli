<#
.SYNOPSIS
Installs a UNCLI build on this PC and starts UNCLI again, from inside UNCLI
or away from the desk (decision 0024).

.DESCRIPTION
UNCLI runs its CLIs in a job that ends with it, and the installer quits
UNCLI before replacing it, so nothing started from a session survives the
install. This script copies the installer and itself to
%LOCALAPPDATA%\UNCLI\update and hands over to a Windows scheduled task
("UNCLI update", the user's own, no administrator), which runs outside
UNCLI:

  1. installs silently (per-user, so no UAC prompt);
  2. waits for every running UNCLI to exit;
  3. starts the newly installed UNCLI, and checks it stays up;
  4. if it doesn't, or the install failed, starts whichever UNCLI is
     installed (the per-user one, else an older one in Program Files),
     so a PC reached from a phone is never left without it.

Everything is logged to %LOCALAPPDATA%\UNCLI\update\update.log. Remote
access is a saved setting, so paired devices reconnect once UNCLI is back.

.PARAMETER Installer
The installer to run. Default: app\build\bin\uncli-amd64-installer.exe,
the one `wails build -nsis` (and publish-build.mjs) leaves.

.PARAMETER DryRun
Do everything except install, quit or start UNCLI: the task runs, logs
what it would do, and exits. To check the task works.

.EXAMPLE
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\install-build.ps1
#>
param(
  [string]$Installer = (Join-Path $PSScriptRoot '..\app\build\bin\uncli-amd64-installer.exe'),
  [switch]$DryRun,
  [switch]$Run # internal: set when the scheduled task runs this copy
)

$ErrorActionPreference = 'Stop'
$UpdateDir = Join-Path $env:LOCALAPPDATA 'UNCLI\update'
$Staged = Join-Path $UpdateDir 'uncli-setup.exe'
$StagedScript = Join-Path $UpdateDir 'install-build.ps1'
$LogFile = Join-Path $UpdateDir 'update.log'
$TaskName = 'UNCLI update'
$UserExe = Join-Path $env:LOCALAPPDATA 'Programs\UNCLI\uncli.exe'
$MachineExe = Join-Path $env:ProgramFiles 'Good Egg Software\UNCLI\uncli.exe'

function Log([string]$msg) {
  $line = '{0}  {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg
  Add-Content -Path $LogFile -Value $line -Encoding UTF8
  Write-Host $line
}

function Get-Uncli { @(Get-Process -Name 'uncli' -ErrorAction SilentlyContinue) }

# Wait-NoUncli waits up to $seconds for every UNCLI to exit; true once none runs.
function Wait-NoUncli([int]$seconds) {
  $deadline = (Get-Date).AddSeconds($seconds)
  while ((Get-Uncli).Count -gt 0) {
    if ((Get-Date) -gt $deadline) { return $false }
    Start-Sleep -Milliseconds 500
  }
  return $true
}

# Start-Uncli starts exe and checks it's still running 20 seconds later
# (UNCLI keeps running in the tray, so a live process means it's up).
function Start-Uncli([string]$exe) {
  if (-not (Test-Path $exe)) { Log "Not there: $exe"; return $false }
  Log "Starting $exe"
  try {
    Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe) | Out-Null
  } catch {
    Log "Couldn't start it: $($_.Exception.Message)"
    return $false
  }
  Start-Sleep -Seconds 20
  $up = @(Get-Uncli | Where-Object { $_.Path -eq $exe })
  if ($up.Count -gt 0) { Log "Running (pid $($up[0].Id))."; return $true }
  Log 'It exited again.'
  return $false
}

if ($Run) {
  # In the scheduled task, outside UNCLI.
  try {
    Log '--- update starts'
    Start-Sleep -Seconds 20 # let the session that asked finish saying so before UNCLI quits
    $before = Get-Uncli
    Log ('Running now: ' + (($before | ForEach-Object { "$($_.Id) $($_.Path)" }) -join '; '))
    if ($DryRun) {
      Log "Dry run: would install $Staged silently, wait for UNCLI to exit, then start $UserExe (or $MachineExe)."
      Log '--- dry run done'
      exit 0
    }

    $code = -1
    try {
      Log "Installing $Staged"
      $p = Start-Process -FilePath $Staged -ArgumentList '/S' -PassThru
      if ($p.WaitForExit(600000)) { $code = $p.ExitCode } else { Log 'The installer is still running after 10 minutes; going on.' }
      Log "Installer exit code: $code"
    } catch {
      Log "The installer couldn't run: $($_.Exception.Message)"
    }

    if (-not (Wait-NoUncli 90)) {
      # It didn't quit (or the install never started): it's still running, so
      # the PC is still reachable. Starting another would only hand over to it.
      $left = Get-Uncli
      Log ('UNCLI is still running, so it was left as it is: ' + (($left | ForEach-Object { "$($_.Id) $($_.Path)" }) -join '; '))
      Log '--- update done (not restarted)'
      exit 1
    }
    Log 'No UNCLI running.'

    $candidates = @()
    if ($code -eq 0) { $candidates += $UserExe }
    $candidates += $UserExe, $MachineExe
    $tried = @{}
    foreach ($exe in $candidates) {
      if ($tried.ContainsKey($exe)) { continue }
      $tried[$exe] = $true
      if ((Get-Uncli).Count -gt 0) { Log 'UNCLI is running.'; break }
      if (Start-Uncli $exe) { Log '--- update done'; exit 0 }
    }
    Log '--- update done, but no UNCLI could be started'
    exit 2
  } catch {
    Log "Failed: $($_.Exception.Message)"
    # Whatever went wrong, don't leave the PC without UNCLI.
    if ((Get-Uncli).Count -eq 0 -and -not $DryRun) {
      foreach ($exe in @($UserExe, $MachineExe)) { if (Start-Uncli $exe) { break } }
    }
    exit 3
  }
}

# Started by hand (or by a session): stage, then hand over to the task.
$Installer = [IO.Path]::GetFullPath($Installer)
if (-not (Test-Path $Installer)) { throw "No installer at $Installer. Build one first (npm run build, or node scripts/publish-build.mjs)." }
New-Item -ItemType Directory -Force -Path $UpdateDir | Out-Null
if (-not $DryRun) { Copy-Item -Path $Installer -Destination $Staged -Force }
Copy-Item -Path $PSCommandPath -Destination $StagedScript -Force
$hash = (Get-FileHash -Path $Installer -Algorithm SHA256).Hash.ToLower()

$taskArgs = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$StagedScript`" -Run"
if ($DryRun) { $taskArgs += ' -DryRun' }
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $taskArgs -WorkingDirectory $UpdateDir
$principal = New-ScheduledTaskPrincipal -UserId ([Security.Principal.WindowsIdentity]::GetCurrent().Name) -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit (New-TimeSpan -Minutes 30) -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName $TaskName -Action $action -Principal $principal -Settings $settings -Description 'Installs a UNCLI build and starts UNCLI again (scripts/install-build.ps1).' -Force | Out-Null
Start-ScheduledTask -TaskName $TaskName

$mode = if ($DryRun) { 'dry run' } else { 'install' }
Write-Host "Started the '$TaskName' task ($mode) for $Installer"
Write-Host "sha256 $hash"
Write-Host "UNCLI quits in about 20 seconds and comes back about half a minute after that. Log: $LogFile"
