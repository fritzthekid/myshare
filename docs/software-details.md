---
layout: default
title: Software details
---

<section>
<nav>
<ul>
  <li><a href="index.html">← Overview</a></li>
  <hr>
  <li><a href="#sec-why-backend">Why a CUPS "backend"</a></li>
  <li><a href="#sec-packages">Package structure</a></li>
  <li><a href="#sec-protocols">Protocols used</a></li>
  <li><a href="#sec-tests">Requirements and tests</a></li>
  <li><a href="#sec-license">License</a></li>
  <hr>
  <li><a href="installation.html">Installation</a></li>
  <li><a href="usage.html">Usage</a></li>
</ul>
</nav>

<article markdown="1">

# Software details

## Why a CUPS "backend" instead of a "filter"? {#sec-why-backend}

CUPS converts print data through a filter chain into a format the
"printer" understands, then hands the result to the backend, which sends
it to the target device. The printer queue is set up as a standard PDF
queue (the same technique `cups-pdf` uses), so the backend already
receives a finished PDF - no PostScript/PCL interpretation needed.

## Package structure {#sec-packages}

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

## Protocols used {#sec-protocols}

- **WebDAV** (`PUT`) to upload the file into Nextcloud.
- **Nextcloud OCS Share API** (`shareType=3`, public link) to create an
  expiring, optionally password-protected share link.
- Plain HTTP (`cloudweb`'s `POST /push`, restricted to `localhost`) to
  hand a rendered QR code over to the browser-facing display service.

## Requirements and tests {#sec-tests}

Every requirement is tracked with its own ID and mapped to the automated
test(s) that cover it in
[REQUIREMENTS.md](https://github.com/fritzthekid/myshare/blob/main/REQUIREMENTS.md).
Run the full suite with:

```bash
go test ./...
```

## License {#sec-license}

GNU Affero General Public License v3.0 (or later) - see
[LICENSE](https://github.com/fritzthekid/myshare/blob/main/LICENSE).
Third-party libraries (each permissively licensed) are listed with their
full license text in
[THIRD_PARTY_NOTICES.md](https://github.com/fritzthekid/myshare/blob/main/THIRD_PARTY_NOTICES.md).

</article>
</section>
