# Archive: implemented requirements

**Please file new requirements as a GitHub issue** (template
"Requirement / Feature", see `.github/ISSUE_TEMPLATE/requirement.md`) -
status (open/closed) and linking to the implementing PR are built in
there, instead of maintaining that manually here.

This file is now only an archive for larger, already-implemented topics -
as a reference for what a well-formulated requirement looks like in
hindsight (the template above uses exactly this format: goal, acceptance
criteria, decisions, non-goals). The two entries below were written
retroactively this way, after the actual (much more loosely worded)
requirement had already been implemented.

## Printer driver output: CloudWeb instead of Raspberry Pi

GitHub issue: [#4](https://github.com/fritzthekid/myshare/issues/4) (closed)

**Goal:** Any device on the LAN - especially a phone, simply in the
browser - should be able to serve as a display for QR code + link,
without that device itself uploading anything or needing credentials
(pure viewer, unlike `webshare`). Replaces the previous Raspberry Pi
framebuffer as the output path.

**Acceptance criteria:**
- Given a print job/`sendfile`/`webshare` call runs successfully and the
  output hook points to `push-to-cloudweb.sh`, when a `cloudweb` display
  page is open, then QR code + link appear there within 1 second, without
  a manual reload.
- Given nothing has ever been pushed, when the display page is opened,
  then it shows a placeholder ("Waiting for the next print job …")
  instead of an error.
- Given any device on the same LAN (including a phone browser), when it
  opens `http://<host>:40080/`, then it sees the same display - without
  login, app, or installation.

**Decisions:**
- Language: Go, no TypeScript/Node - one binary, no extra dependency,
  can reuse existing internal packages.
- Port configurable via `CLOUDWEB_LISTEN`, default `:40080`.
- Security model: `POST /push` only accepts requests from `localhost`
  (driver/hook run on the same host as `cloudweb`); the display page
  itself remains reachable on the LAN without access protection.
- Runs as a systemd service with `DynamicUser=yes` (no root needed, since
  there's no file access/no Nextcloud credentials).
- No screen wake lock - the display device's standard lock timeout is
  enough.

**Non-goals:**
- No login/access protection on the display page itself.
- No history of multiple QR codes - only the latest one counts.
- No installable PWA/no share target in the phone's menu.

## Password protection for CloudWeb shares (`scripts/share-to-web.sh`)

GitHub issue: [#5](https://github.com/fritzthekid/myshare/issues/5) (closed)

**Goal:** For larger/more sensitive files (e.g. ZIP archives), a
`CloudWeb` share should additionally be protectable with a password,
without the recipient needing to remember two independent passwords.

**Acceptance criteria:**
- Given `scripts/share-to-web.sh <file> <name>` is called, when the share
  is created, then Nextcloud has a share password set that is exactly
  `<name>` + a displayed 16-character Base32 string (`crypt`) - e.g.
  `hashlib.sha256(b"password")` → `base64.b32encode(...)[:16]` ==
  `L2EERGG2FACHCUOQ`.
- Given the same `<name>` is used for two different shares, then the two
  `crypt` values differ (no reusable password).
- Given an attacker only knows `<name>` (e.g. overheard), then they can't
  compute `crypt` themselves (missing knowledge of the print server's MAC
  address as a silent second factor).
- Given `scripts/share-to-web.sh <file>` is called without `<name>`, then
  the share remains completely unprotected by a password, as before.
- Given the recipient knows `<name>` and sees `<crypt>`, when they type
  `<name><crypt>` (no separator) as the Nextcloud share password, then
  they can download the file.

**Decisions:**
- Password source: Nextcloud's native OCS share API (`password` field on
  share creation) - no own file encryption.
- Handoff path CLI → backend: CUPS job option `-o nc-password-seed=<name>`
  (not the job title, which is already used for the filename).
- Derivation: `crypt = base32(sha256(name + MAC address + unique
  remote path))[:16]`; the actual password is `name + crypt` -
  explicitly **not** the hardened seed itself plus `crypt` (that
  wouldn't be typeable by the recipient, since they don't know the MAC
  address/remote path).
- MAC address: that of a LAN interface of the print server, read
  automatically - not that of a target device, not passed manually.

**Non-goals:**
- No second, separately typed password.
- No client-side file encryption.
- No change to `sendfile`/`webshare` - only the `CloudWeb` queue is
  affected.
