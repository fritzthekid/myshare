---
title: Software details
---

[← Back to overview](index.md)

# Software details

## Why a CUPS "backend" instead of a "filter"?

CUPS converts print data through a filter chain into a format the
"printer" understands, then hands the result to the backend, which sends
it to the target device. The printer queue is set up as a standard PDF
queue (the same technique `cups-pdf` uses), so the backend already
receives a finished PDF - no PostScript/PCL interpretation needed.

## Package structure

| Package | Purpose |
|-------|-------|
| `internal/config` | Configuration exclusively via environment variables |
| `internal/filenames` | Unique, filesystem-safe filenames from title + timestamp |
| `internal/nextcloud` | WebDAV upload + public, expiring share link via the OCS API |
| `internal/sharepassword` | Derivation of the optional share password + its short display form |
| `internal/qrview` | QR code PNG from the share link |
| `internal/output` | Default output: link in `tmp/links.log` + QR PNG in `tmp/`, plus an optional external display hook |
| `cmd/backend` | CUPS backend entry point |
| `cmd/sendfile` | CLI entry point for arbitrary files (not a print job) |
| `cmd/webshare` | HTTP entry point for arbitrary files, e.g. from a phone |
| `cmd/cloudweb` | Standalone web display for the most recently shared QR code + link (also a Windows service) |
| `cmd/fileshare` | Windows Explorer "Send to" entry point |

## Protocols used

- **WebDAV** (`PUT`) to upload the file into Nextcloud.
- **Nextcloud OCS Share API** (`shareType=3`, public link) to create an
  expiring, optionally password-protected share link.
- Plain HTTP (`cloudweb`'s `POST /push`, restricted to `localhost`) to hand
  a rendered QR code over to the browser-facing display service.

## Requirements and tests

Every requirement is tracked with its own ID and mapped to the automated
test(s) that cover it in
[REQUIREMENTS.md](https://github.com/fritzthekid/myshare/blob/main/REQUIREMENTS.md).
Run the full suite with:

```bash
go test ./...
```

## License

GNU Affero General Public License v3.0 (or later) - see
[LICENSE](https://github.com/fritzthekid/myshare/blob/main/LICENSE). Third-party
libraries (each permissively licensed) are listed with their full license
text in
[THIRD_PARTY_NOTICES.md](https://github.com/fritzthekid/myshare/blob/main/THIRD_PARTY_NOTICES.md).
