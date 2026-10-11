#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Installs the CloudWeb printer driver + service on Windows.
.DESCRIPTION
  See doc/anforderung_windows.md. Creates/idempotently updates:
    - C:\ProgramData\myshare - configuration (backend.env), binaries, and
      the fixed print target (incoming.pdf).
    - Printer "CloudWeb" using the existing inbox driver "Microsoft Print
      To PDF", bound to a fixed file port instead of PORTPROMPT: - the
      original "Microsoft Print to PDF" printer remains unchanged (verified
      live, see doc/anforderung_windows.md).
    - Windows service "CloudWeb" (cloudweb.exe: HTTP display + folder
      watcher on incoming.pdf), via golang.org/x/sys/windows/svc - no NSSM
      needed.
  Expects cloudweb.exe and fileshare.exe in the same directory as this
  script (e.g. after copying from a build artifact).

  Needs administrator rights (printer, service). The Explorer "Send to"
  entry is deliberately NOT set up by this script, but by the separate
  userinstall.ps1 (to be run without admin rights) - an elevated shell may
  run under a different user profile (%APPDATA%) than the user who later
  actually uses "Send to"; this was observed live (see GitHub issue #7).
#>

$ErrorActionPreference = "Stop"

$DataDir = "C:\ProgramData\myshare"
$PortPath = Join-Path $DataDir "incoming.pdf"
$EnvPath = Join-Path $DataDir "backend.env"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$CloudwebExe = Join-Path $DataDir "cloudweb.exe"
$FileshareExe = Join-Path $DataDir "fileshare.exe"

foreach ($exe in "cloudweb.exe", "fileshare.exe") {
    if (-not (Test-Path (Join-Path $ScriptDir $exe))) {
        throw "$exe is missing next to install.ps1 ($ScriptDir) - build it first with 'GOOS=windows GOARCH=amd64 go build'."
    }
}

New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

Write-Host "Copying binaries to $DataDir ..."
Copy-Item -Path (Join-Path $ScriptDir "cloudweb.exe") -Destination $CloudwebExe -Force
Copy-Item -Path (Join-Path $ScriptDir "fileshare.exe") -Destination $FileshareExe -Force

if (-not (Test-Path $EnvPath)) {
    Copy-Item -Path (Join-Path $ScriptDir "backend.env.example") -Destination $EnvPath
    Write-Warning "New configuration file created: $EnvPath - please fill in NC_BASE_URL/NC_USERNAME/NC_PASSWORD, then: Restart-Service CloudWeb"
}

Write-Host "Setting up printer 'CloudWeb' ..."
$existingPrinter = Get-Printer -Name "CloudWeb" -ErrorAction SilentlyContinue
if ($existingPrinter -and $existingPrinter.PortName -ne $PortPath) {
    # Still points to an old path (e.g. after a rename like
    # printtoqrview -> myshare) - remove the printer so it gets cleanly
    # rebound to $PortPath below, instead of silently staying on the old
    # port.
    Write-Warning "Printer 'CloudWeb' still pointed to $($existingPrinter.PortName) - switching to $PortPath."
    Remove-Printer -Name "CloudWeb"
    $existingPrinter = $null
}
if (-not (Get-PrinterPort -Name $PortPath -ErrorAction SilentlyContinue)) {
    Add-PrinterPort -Name $PortPath
}
if (-not $existingPrinter) {
    Add-Printer -Name "CloudWeb" -DriverName "Microsoft Print To PDF" -PortName $PortPath
}

Write-Host "Setting up service 'CloudWeb' ..."
$existing = Get-Service -Name "CloudWeb" -ErrorAction SilentlyContinue
if ($existing) {
    Stop-Service -Name "CloudWeb" -Force -ErrorAction SilentlyContinue
    sc.exe delete CloudWeb | Out-Null
    Start-Sleep -Seconds 1
}
New-Service -Name "CloudWeb" -BinaryPathName $CloudwebExe -DisplayName "CloudWeb (myshare)" -StartupType Automatic | Out-Null
Start-Service -Name "CloudWeb"

Write-Host ""
Write-Host "Done. To check:"
Write-Host "  Get-Service CloudWeb"
Write-Host "  Get-Printer | Select Name, DriverName, PortName"
Write-Host "  http://localhost:40080/"
Write-Host ""
Write-Host "Still to do: run userinstall.ps1 in a NON-elevated PowerShell"
Write-Host "(your own login), for the Explorer 'Send to' entry."
