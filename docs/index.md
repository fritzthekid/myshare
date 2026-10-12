---
layout: default
title: Overview
---

<section>
<nav>
<ul>
  <li><a href="#sec-what">What is myshare</a></li>
  <li><a href="#sec-next">Where to go next</a></li>
  <li><a href="#sec-license">Source and license</a></li>
  <hr>
  <li><a href="installation.html">Installation</a></li>
  <li><a href="usage.html">Usage</a></li>
  <li><a href="software-details.html">Software details</a></li>
</ul>
</nav>

<article markdown="1">

# myshare

## What is myshare {#sec-what}

**myshare** shares a file via [Nextcloud](https://nextcloud.com/) and shows
the resulting public link as a QR code - so whoever needs the file can just
scan it with their phone, instead of typing a link or waiting for an email.

It started as a CUPS backend (a "virtual printer" that uploads the PDF
instead of printing it on paper), and grew into several interchangeable
ways to trigger the same upload+share+QR-code flow:

- **Print** to a special CUPS queue (from any application with a print
  dialog).
- **Run a command** (`sendfile`, `scripts/share.sh`) for an arbitrary file,
  no printing involved.
- **Upload from a phone** (`webshare`, incl. a Termux build for Android).
- **Drag a file onto "Send to"** on Windows (`fileshare.exe`).

The resulting QR code + link can show up on an attached Raspberry Pi
display, in any browser via `cloudweb`, or just in the terminal log -
whichever fits the situation.

## Where to go next {#sec-next}

<div class="info-box" markdown="1">
- **[Installation](installation.html)** - set up the CUPS queues, the
  Raspberry Pi display, `cloudweb`, or the Windows service.
- **[Usage](usage.html)** - share a file and see the result, including
  password-protected shares.
- **[Software details](software-details.html)** - package structure,
  protocols used, and where the automated tests live.
</div>

## Source and license {#sec-license}

The full source, the requirement → test mapping, and the issue tracker
live at [github.com/fritzthekid/myshare](https://github.com/fritzthekid/myshare).

myshare is licensed under the GNU Affero General Public License v3.0 (or
later) - see
[LICENSE](https://github.com/fritzthekid/myshare/blob/main/LICENSE).

</article>
</section>
