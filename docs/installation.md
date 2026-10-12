---
title: Installation
---

[← Back to overview](index.md)

# Installation

All variants need a Nextcloud instance and a dedicated user with an **app
password** (not the account password) that has write access to a target
folder. Configuration everywhere is via environment variables (`NC_*`) -
see the
[configuration table in the README](https://github.com/fritzthekid/myshare/blob/main/README.md#configuration-environment-variables).

## Linux: CUPS backend + queues

Builds the Go binaries and sets up two CUPS queues, `CloudToRaspi` (output
on a Raspberry Pi display) and `CloudWeb` (output in a browser):

```bash
deploy/build.sh
sudo ./deploy/install.sh
sudo "$EDITOR" /etc/myshare/backend.env   # fill in NC_BASE_URL, NC_USERNAME, NC_PASSWORD
sudo systemctl restart cups
```

Both queues use the same backend code - CUPS passes the queue name to the
wrapper script, which picks the matching display hook automatically.
`install.sh` is idempotent, so rerunning it after a rebuild is enough for
updates. See the
[README section "Integration into CUPS"](https://github.com/fritzthekid/myshare/blob/main/README.md#integration-into-cups)
for the full breakdown of what gets installed where.

## Raspberry Pi display (`CloudToRaspi`)

On the machine running the CUPS backend, `deploy/install.sh` installs
`raspi/scripts/toraspi.sh` as the display hook for the `CloudToRaspi`
queue; it composes the QR code + link into an image and sends it to the Pi
over `ssh` (passwordless root SSH access to the Pi is required, since CUPS
runs the backend as root).

On the Raspberry Pi itself, two pieces receive and show that image:

- `raspi/bin/pipetodisp.sh` - receives the image over stdin and drops it
  into a watch directory.
- `raspi/bin/fb-image-watcher.sh`, installed as the systemd service
  `raspi/services/fb-image-watcher.service` - watches that directory and
  shows new images on the framebuffer for a limited time.

Which `/dev/fb*` device is the actual display can vary by boot/probe
timing - check with `dmesg | grep -i fb_ili9486` on the Pi if nothing
shows up, instead of assuming the default is correct.

## `cloudweb` (browser display, no extra device needed)

`cloudweb` is a small, standalone web server (no Nextcloud credentials
needed) that shows the most recently pushed QR code + link and updates
itself automatically. `deploy/install.sh` installs it as the systemd
service `myshare-cloudweb` if it was built; print to the `CloudWeb` queue
to send results there. Can also be run standalone:

```bash
go build -o cloudweb ./cmd/cloudweb
./cloudweb
# -> leave http://<this-machine>:40080/ open in a browser (e.g. on a phone)
```

## Windows (`cloudweb` service + "Send to")

See
[doc/windows_requirement.md](https://github.com/fritzthekid/myshare/blob/main/doc/windows_requirement.md)
for the full guide. In short: `deploy/windows/build.sh` cross-compiles
everything (no Windows machine needed to build) into `dist/windows/`,
packaged as `cloudweb.zip` with a step-by-step `HowToInstall.txt` inside.
An **Administrator** PowerShell runs `install.ps1` (sets up the printer and
the `cloudweb` Windows service), a **normal** (non-elevated) PowerShell
runs `userinstall.ps1` (sets up the Explorer "Send to" entry).

## Android / Termux (`webshare`)

Sharing from a phone without a reachable server and without relying on
Nextcloud's CORS configuration works by compiling `webshare` natively in
Termux on the phone itself - see
[doc/android.md](https://github.com/fritzthekid/myshare/blob/main/doc/android.md)
for the full guide.
