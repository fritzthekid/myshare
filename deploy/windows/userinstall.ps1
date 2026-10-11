<#
.SYNOPSIS
  Sets up the Explorer "Send to" entry "CloudWeb Share".
.DESCRIPTION
  Deliberately separate from install.ps1 (which needs admin rights): an
  elevated PowerShell may run under a different user profile (%APPDATA%)
  than the user who later actually uses "Send to" - observed live (see
  GitHub issue #7). Therefore run without admin rights, in a plain
  PowerShell under your own login:

    .\userinstall.ps1

  Requires install.ps1 to have already run (once, as Administrator) -
  C:\ProgramData\myshare\fileshare.exe must exist.
#>

$ErrorActionPreference = "Stop"

$FileshareExe = "C:\ProgramData\myshare\fileshare.exe"
if (-not (Test-Path $FileshareExe)) {
    throw "$FileshareExe is missing - run install.ps1 as Administrator first."
}

$sendTo = Join-Path $env:USERPROFILE "AppData\Roaming\Microsoft\Windows\SendTo"
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut((Join-Path $sendTo "CloudWeb Share.lnk"))
$shortcut.TargetPath = $FileshareExe
$shortcut.Save()

Write-Host "Done: 'Send to' -> 'CloudWeb Share' should now appear in the Explorer context menu."
