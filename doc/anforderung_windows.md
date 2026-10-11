---
name: Requirement / Feature
about: Port the printer driver (CloudWeb) to Windows
title: "Port the printer driver (CloudWeb) to Windows"
labels: "cloudweb_for_windows"
assignees: "claude"
---

<!--
Not every section is always needed - but where a decision has already been
made, it belongs under "Decisions" instead of staying open (otherwise it
becomes a follow-up question). See ISSUES.md for two worked examples.
-->

## Goal

The CloudWeb printer driver, including the http server, should be ported
to Windows. All features of the driver are carried over. The http server
is implemented as a service (Windows service). There's an installation
script to install both the driver and the service, and to start the
service. The share-to-web.sh functionality is also offered in this or a
similar form. If possible, integration into the file browser as a SendTo
option. The "Name" parameter now has to be prompted for. A file like the
current backend.env is created and read in.

### Summary:
- backend
- cloudweb driver
- fileshare (like share-to-web)
- Installation of the printer driver + service via PowerShell with admin rights.
- On SendTo (share-to-web), a name is prompted for; empty means no password.
- Service installation and operation should work via Get-Service, Start-Service, New-Service

<!-- One sentence: what is this good for (not just what should be built). -->

## Acceptance Criteria

- The printer driver actually appears as a selectable printer.
- The http server is active and delivers output
- The printer driver's output delivers to http://localhost:40080/ on
  printing (localhost is now a different machine)
  the QR code, the link (text), and last updated,
- When outputting files via the SendTo function,
  the QR code, the link (text), the crypt, and last updated.
- The printer driver and SendTo function write to the same target as in
  the Linux variant: http://localhost:40080/

**All criteria verified live in the Windows 10 VM** (2026-09-16):
- Printer "CloudWeb" appears selectable, original "Microsoft Print to PDF"
  remains unchanged alongside it.
- `cloudweb.exe` runs as an installed Windows service (`Start-Service
  CloudWeb`, created via `New-Service`) and processes print jobs (folder
  watcher on `incoming.pdf`) exactly like in the interactive test run.
- Printing via "CloudWeb" automatically generates a QR code + link at
  `http://localhost:40080/`, with no manual action needed.
- `fileshare.exe` (Explorer "Send to") uploads a selected file, asks for
  the optional name, additionally generates a password short form (crypt)
  for a non-empty name, and pushes both to the same display page - just
  like the printer.

<!--
Concrete, ideally as "Given ... when ... then ...".
Where a value should be computed exactly: example input/output as code
instead of prose, e.g.:

    hashlib.sha256(b"password") -> base64.b32encode(...)[:16] == "L2EERGG2FACHCUOQ"

- Given ..., when ..., then ...
-->


## Decisions

As in the original, as much code as possible should be reused. Use Go as the programming language for new components.

- Service capability of `cloudweb`: via `golang.org/x/sys/windows/svc`
  (official, lean Go package for the Windows Service Control Manager
  handshake) directly in the binary - no other language, no external
  wrapper like NSSM needed. This makes `New-Service`/`Start-Service`/
  `Get-Service` work as intended.
  **Implemented** in `cmd/cloudweb/service_windows.go`
  (`cmd/cloudweb/service_other.go` keeps the Linux/macOS behavior
  unchanged); separated via build tag, no impact on the existing Linux
  build (`go build ./...` on Linux remains unchanged and green).
  If `cloudweb.exe` runs interactively (not as a registered service,
  `svc.IsWindowsService()` detects that), it still starts server + watcher
  directly, blocking - makes testing easier without re-registering the
  service on every change.

- Configuration file (Windows equivalent of `EnvironmentFile=` under
  systemd): `C:\ProgramData\myshare\backend.env`, KEY=VALUE format like on
  Linux. Since `New-Service` doesn't know about an environment file,
  `cloudweb.exe` reads it in itself on startup (`internal/envfile`, new
  platform-neutral package with a Windows/Linux default path via build
  tag - a no-op on Linux, where systemd remains responsible).
  **Pitfall experienced live while testing:** Values in `backend.env` must
  **not** be in quotes (`NC_PASSWORD=abc`, not `NC_PASSWORD='abc'`) -
  neither `internal/envfile` nor systemd's `EnvironmentFile=` strip
  surrounding quotes, they'd otherwise become a literal part of the value
  (caused hours of "Login failed", because the entered app password never
  matched the actual one as a result).

- Printer driver mechanism: **a second, separate printer using the
  existing inbox driver "Microsoft Print To PDF"** - the original
  "Microsoft Print to PDF" printer remains unchanged (different name, own
  port, no registry/driver change to it). **Verified live in the Windows
  10 VM** (2026-09-16): printing via the new "CloudWeb" printer produces
  `C:\ProgramData\myshare\incoming.pdf` with no dialog at all;
  `Get-Printer` afterwards still shows "Microsoft Print to PDF"
  (`PORTPROMPT:`) unchanged, alongside "CloudWeb" (fixed file port).
  - The original printer uses the special port `PORTPROMPT:`, which opens
    the "save as" dialog on every print - that's why it can't/shouldn't be
    reused.
  - Instead, a **local file port with a fixed path** is created, e.g.
    `C:\ProgramData\myshare\incoming.pdf` (local ports on Windows are
    identified directly by their file path as the port name - no port
    monitor code needed). The spooler writes to exactly this file on
    every print job, silently, with no dialog at all.
  - A new printer (name **"CloudWeb"**, analogous to the Linux queue) is
    bound to this port using the same inbox driver, e.g.:
    ```powershell
    Add-PrinterPort -Name "C:\ProgramData\myshare\incoming.pdf"
    Add-Printer -Name "CloudWeb" -DriverName "Microsoft Print To PDF" `
      -PortName "C:\ProgramData\myshare\incoming.pdf"
    ```
  - The Windows service (`cloudweb.exe`, see below) contains a folder
    watcher (simple 1s polling via `time.Ticker`, deliberately without an
    extra dependency like `fsnotify` - for a single, fixed-name file this
    is entirely sufficient for the use case) on this fixed file; on
    change, it's **immediately renamed atomically** (to a timestamp
    filename in the same directory, `os.Rename`) and then processed just
    like the Linux backend (upload, share link, QR code, directly updating
    the display - no push via HTTP needed, since the watcher and display
    server run in the same process). The immediate rename prevents a
    second, quickly following print job from overwriting the file before
    it's processed. **Implemented** in `cmd/cloudweb/watcher_windows.go`.
  - **`cmd/backend` (the CUPS backend) is not a standalone program on
    Windows** - there's no Windows equivalent of the CUPS backend
    invocation (`job-id user title copies options [file]`) that Windows
    would automatically run on printing. Its role (upload + share link +
    QR code after a print job) is instead taken over directly by the
    folder watcher in `cloudweb.exe` (see above).
  - For `share-to-web` (SendTo with a name prompt), **no detour via the
    printer**: `cmd/fileshare` (new command, meant for Windows, but built
    on the same platform-neutral internal packages) uploads the file
    selected via Explorer directly itself (like `scripts/share-to-web.sh`),
    asking - if not passed as a second argument - interactively on the
    console for the optional `Name`, deriving crypt+password for a
    non-empty name exactly like `cmd/backend` does
    (`internal/sharepassword`, MAC address + remote path as a silent
    second factor), and then pushing the result via `POST /push` to the
    running `CloudWeb` service - protocol-compatible with
    `scripts/push-to-cloudweb.sh` (the same form fields `file`, `link`,
    `password`). Set up as an Explorer "Send to" entry (see
    `deploy/windows/install.ps1`).

<!--
Things that are already settled: language, which component takes on what,
which field/parameter is used for what, security model, etc.
-->

## Non-Goals

<!-- What is deliberately NOT required, in case the scope would otherwise be unclear. -->
- The CloudToRaspi printer driver is not required
- If the fileshare -> CloudWeb functionality works from the file browser, no additional script for output is expected.
- Windows' own "Microsoft Print to PDF" printer (driver entry, port
  `PORTPROMPT:`, behavior) is not changed, removed, or repurposed - it
  remains usable by the user unchanged, just as before the installation.
