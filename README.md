# myshare

CUPS backend (in Go) that uploads a print job as a PDF to a Nextcloud
instance, creates a public, expiring share link, and outputs it (for now)
as a log line + QR code. Alongside that, standalone ways to share
arbitrary files the same way - from the terminal, from a phone, or via
Explorer "Send to" on Windows (see below).

## Why "Backend" instead of "Filter"?

CUPS converts print data through a filter chain into a format the
"printer" understands, then hands the result to the backend, which sends
it to the target device. If you assign the printer a standard PDF queue
(exactly like `cups-pdf` does), the backend already receives a finished
PDF - so we don't have to do any PostScript/PCL interpretation ourselves.

## Structure

| Package | Purpose |
|-------|-------|
| `internal/config` | Configuration exclusively via environment variables (R6) |
| `internal/filenames` | Unique, filesystem-safe filenames from title + timestamp (R3) |
| `internal/nextcloud` | WebDAV upload (R4) + public, expiring share link via OCS API (R5) |
| `internal/qrview` | QR code PNG from the share link |
| `internal/output` | Preliminary output: link in `tmp/links.log` + QR PNG in `tmp/` (R7) |
| `cmd/backend` | CUPS backend entry point |
| `cmd/sendfile` | CLI entry point for arbitrary files (not a print job) |
| `cmd/webshare` | HTTP entry point for arbitrary files - e.g. from a phone via browser, without its own Nextcloud credentials |

## Configuration (environment variables)

| Variable | Required | Description |
|----------|---------|--------------|
| `NC_BASE_URL` | yes | e.g. `https://cloud.example.com` |
| `NC_USERNAME` | yes | Nextcloud user (for upload + sharing) |
| `NC_PASSWORD` | yes | Nextcloud **app password** (not the account password!) |
| `NC_TARGET_DIR` | no | Target folder, default `/PrinterUploads` |
| `NC_LINK_EXPIRE_DAYS` | no | Validity period of the share link in days, default `1` |

## Building

```bash
deploy/build.sh
```

Builds `backend`, `sendfile`, `webshare`, and `cloudweb` in one step
(avoids `install.sh` accidentally installing a stale binary because one of
them was forgotten). Individually also works:

```bash
go build -o backend ./cmd/backend
go build -o sendfile ./cmd/sendfile
go build -o webshare ./cmd/webshare
go build -o cloudweb ./cmd/cloudweb
```

## Testing locally

```bash
export NC_BASE_URL=https://cloud.example.com
export NC_USERNAME=change-cloud-user
export NC_PASSWORD=<app-password>
cat test-print.pdf | ./backend job1 alice "Test print" 1 ""
```

## Sharing arbitrary files (without a printer)

Besides the CUPS backend, there's `sendfile` for the case where you don't
have print data, but want to share an arbitrary file (e.g. a ZIP) directly
via the same path (Nextcloud upload + share link + QR code):

```bash
./sendfile path/to/test.zip
./sendfile path/to/test.zip "Other Title.zip"
cat test.zip | ./sendfile - test.zip
```

Without a label, the filename is taken from the path (incl. extension);
with stdin (`-`), the label has to provide the extension, otherwise `.bin`
is used. Uses the same configuration (`NC_*` environment variables) as the
backend.

## Sharing from a phone

Three code paths exist for this, each suited to different situations:

| | `webshare` on a server | `web/share.html` | `webshare` in Termux on the phone |
|---|---|---|---|
| Needs a reachable server | Yes (LAN/VPN) | No | No |
| Depends on Nextcloud's CORS configuration | No | Yes | No |

In practice, **`webshare` compiled natively in Termux** (third column) has
proven to be the way that works without a reachable server and without
CORS requirements on the Nextcloud instance - **see
[`doc/android.md`](doc/android.md) for the full guide.**

The other two code paths remain in the repo (other situations - e.g. an
actually reachable server, or a Nextcloud with CORS support - can make
them worthwhile), but aren't described further in this README:
`cmd/webshare/main.go` (HTTP server variant) and `web/share.html` +
`deploy/gen-share-page.sh` (purely client-side variant) are self-explanatorily
commented.

## Integration into CUPS

All the building blocks live in `deploy/`:

| File | Purpose |
|-------|-------|
| `deploy/install.sh` | Installs the binary + wrapper + PPD, sets up the queues (idempotent) |
| `deploy/nextcloud-backend-wrapper.sh` | Ends up as `/usr/lib/cups/backend/nextcloud`; reloads `NC_*` env vars (CUPS starts backends with a minimal environment) and chooses the display hook based on the queue |
| `deploy/backend.env.example` | Template for `/etc/myshare/backend.env` (credentials, `chmod 600 root:root`) |
| `deploy/cloudpdf.ppd` | PPD for a generic PDF passthrough queue (same technique as `cups-pdf`: `cupsFilter2` declares `application/pdf` as the final format, CUPS stops the filter chain there instead of rasterizing) - shared by both queues |
| `deploy/webshare.service` | systemd unit for `webshare` (see section "Sharing from a phone"), installed by `install.sh` if `webshare` was built |
| `deploy/cloudweb.service` | systemd unit for `cloudweb` (see section "Output on an external display"), installed by `install.sh` if `cloudweb` was built |

`install.sh` sets up **two** CUPS queues that use the same backend code -
only the queue name (passed by CUPS to the wrapper as `$PRINTER`) decides
which display hook runs automatically:

| Queue | Display target |
|---|---|
| `CloudToRaspi` | Raspberry Pi framebuffer (`raspi/scripts/toraspi.sh`) |
| `CloudWeb` | `cloudweb` web display (`scripts/push-to-cloudweb.sh`) |

Installation:

```bash
deploy/build.sh
sudo ./deploy/install.sh
# fill in credentials if not done yet:
sudo "$EDITOR" /etc/myshare/backend.env
sudo systemctl restart cups

lp -d CloudToRaspi test-print.pdf
lp -d CloudWeb test-print.pdf
tail -f /var/lib/myshare/tmp/links.log
```

`install.sh` also sets up `/usr/lib/cups/backend/nextcloud` with
`chmod 700 root:root` - only with this combination does `cupsd` run the
backend as root (see `man backend`); with laxer permissions it runs as an
unprivileged user (usually `lp`).

Called without arguments (e.g. by `lpinfo -v` for device discovery), the
backend responds with a CUPS-compliant discovery line instead of an error.

Rebuilding + `sudo ./deploy/install.sh` is enough for updates (the binary
is overwritten, the existing env file is preserved). An earlier single
queue named `CloudPDF` is automatically removed on the first run and
replaced by the two queues above.

### Sharing arbitrary files via the CUPS queue (`scripts/share.sh`)

Once the CUPS queue is set up, `scripts/share.sh` can serve as a lean
alternative to `sendfile`: same invocation, but without the calling user
needing access to the Nextcloud credentials themselves - those live
exclusively with the CUPS backend (`/etc/myshare/backend.env`), `lp` just
passes the file through:

```bash
scripts/share.sh path/to/test.zip
scripts/share.sh path/to/test.zip "Other Title.zip"
cat test.zip | scripts/share.sh - test.zip
```

Internally just a wrapper around `lp -d $PRINTER -t <title> [file]`
(default for `$PRINTER`: `CloudToRaspi`, switchable via the environment
variable `PRINTER=CloudWeb scripts/share.sh ...`). Only works on machines
with a configured queue (see above) - `sendfile` therefore remains the
independent variant (no CUPS needed), e.g. as the basis for `webshare` and
`web/share.html`.

`scripts/share-to-web.sh` is the same shorthand with `PRINTER=CloudWeb`
permanently preset (result appears in the browser instead of on the Pi).
Its second parameter is - unlike with `share.sh` - **not a title**, but an
optional name that serves as the basis for an additional share password
(`-o nc-password-seed=<name>`, see `internal/sharepassword`):

```bash
scripts/share-to-web.sh path/to/test.zip      # without password protection
scripts/share-to-web.sh path/to/test.zip Name # with password protection
```

Without a `name` argument, the share remains unprotected by a passphrase,
just like with `share.sh`.

## Output on an external display (optional)

After every successful run (backend **and** `sendfile`/`webshare`, since
they all use the same `internal/output.Default`), an external script can
additionally be called - e.g. to display the QR code on an attached
display. The hook is called as `<hook> <qr-png-path> <link>`, a failure
doesn't make the print job fail.

For the two CUPS queues (`CloudToRaspi`/`CloudWeb`, see "Integration into
CUPS"), the printed queue name alone decides which hook runs -
`deploy/nextcloud-backend-wrapper.sh` sets it automatically.
`MYSHARE_DISPLAY_HOOK=<path>` in `backend.env` remains the way to do this
for `sendfile`/`webshare` (there's no queue there) as well as a fallback
for other/future queues.

Included is `raspi/scripts/toraspi.sh` for output on a Raspberry Pi
display (queue `CloudToRaspi`):

| Directory | Runs on | Purpose |
|-------------|-----------|-------|
| `raspi/scripts/toraspi.sh` | this machine (as a hook) | Composes QR code + link text via ImageMagick, sends the result to the Pi via `ssh` |
| `raspi/bin/pipetodisp.sh` | Raspberry Pi | Receives the image via stdin, drops it into the watch directory |
| `raspi/bin/fb-image-watcher.sh` + `raspi/services/fb-image-watcher.service` | Raspberry Pi | systemd service, shows new images on the framebuffer for a limited time |

`deploy/install.sh` automatically installs `toraspi.sh` (if present) to
`/usr/local/lib/myshare/toraspi.sh`; activation + host/timeout via
`MYSHARE_DISPLAY_HOOK`, `RASPI_HOST`, `RASPI_DISPLAY_TIMEOUT` in
`backend.env` (template in `deploy/backend.env.example`). Requires:
`convert` (ImageMagick) as well as passwordless SSH access from root (CUPS
runs the backend as root) to the Pi.

See [`doc/raspi_setup.md`](doc/raspi_setup.md) for the full Raspberry Pi
side setup (display driver, verifying the display before installing
anything, the systemd service, and troubleshooting).

### Alternative: QR display in the browser instead of the Pi display (`cmd/cloudweb`, queue `CloudWeb`)

Needs no additional device: `cloudweb` is a small, standalone web server
(no Nextcloud access, no credentials needed) that displays the most
recently pushed QR code + link (+ optional password) and keeps itself
automatically up to date via polling (1x/second) - just leave it open in a
browser. That can be any device on the LAN, in particular also a phone: it
doesn't upload anything and needs no credentials, a pure viewer.

```bash
go build -o cloudweb ./cmd/cloudweb
./cloudweb
# -> leave http://<this-machine>:40080/ open in a browser (e.g. on a phone)
```

Print via the `CloudWeb` queue to send results there (instead of
`CloudToRaspi` for the Pi) - `MYSHARE_DISPLAY_HOOK` is set automatically
for this queue, see above. For `sendfile`/`webshare`, instead set it by
hand in `backend.env`:

```
MYSHARE_DISPLAY_HOOK=/usr/local/lib/myshare/push-to-cloudweb.sh
CLOUDWEB_URL=http://127.0.0.1:40080
```

The push endpoint (`POST /push`) only accepts requests from `localhost` -
the hook runs on the same host as `cloudweb` itself after all; the display
page remains normally reachable on the network. `deploy/install.sh`
automatically installs `cloudweb` (if built) as the systemd service
`myshare-cloudweb` (runs with `DynamicUser=yes`, needs no special
privileges unlike `backend`/`webshare`), as well as the hook script, to
`/usr/local/lib/myshare/push-to-cloudweb.sh`.

## Tests

```bash
go test ./...
```

See `REQUIREMENTS.md` for the requirement → test mapping.

## Windows

See `doc/windows_requirement.md` for goal/decisions (printer driver as a
second, separate printer on the "Microsoft Print To PDF" inbox driver,
`cloudweb` as a Windows service with a built-in folder watcher, `fileshare`
as an Explorer "Send to" tool).

Built via cross-compile - no Windows machine needed to build, pure Go
without cgo:

```bash
deploy/windows/build.sh
```

Lands in `dist/windows/` (`cloudweb.exe`, `fileshare.exe`, `install.ps1`,
`userinstall.ps1`, `backend.env.example`, `HowToInstall.txt`) - plus
packaged as `cloudweb.zip`, handy for downloading onto the Windows
machine (individual `.exe` downloads otherwise trigger a SmartScreen
warning every time). A detailed step-by-step guide is included as
`HowToInstall.txt` in the ZIP; briefly summarized:

In an **Administrator** PowerShell (printer + service, needs elevated
rights):

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
.\install.ps1
```

Then in a **normal** PowerShell (deliberately not elevated - an elevated
shell may run under a different user profile than the later "Send to"
user):

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
.\userinstall.ps1
```

sets up the Explorer "Send to" entry "CloudWeb Share".

Then fill in `C:\ProgramData\myshare\backend.env` with the real Nextcloud
credentials (values **without** quotes - neither `internal/envfile` nor
systemd's `EnvironmentFile=` on Linux strip them, they'd otherwise become
a literal part of the value) and `Restart-Service CloudWeb`.

## License

GNU Affero General Public License v3.0 (or later) - see [`LICENSE`](LICENSE).

Especially relevant for `cmd/cloudweb` and `cmd/webshare`: anyone running a
modified version of this project as a network service that third parties
interact with (e.g. whoever scans the QR code or uploads from a phone)
must - unlike with the plain GPL - offer those users access to the source
code of the running version (AGPL §13).

Third-party libraries included (each permissively licensed) are listed
with their full license text in [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
