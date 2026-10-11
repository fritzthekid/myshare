#!/usr/bin/env bash
# Output hook (see internal/output.Hook / MYSHARE_DISPLAY_HOOK): called by
# backend/sendfile/webshare after every success as
# "push-to-cloudweb.sh <qr-png-path> <link> [password]" and sends the
# result via HTTP to a locally running cloudweb server (see cmd/cloudweb)
# instead of the Raspberry Pi framebuffer (raspi/scripts/toraspi.sh). The
# third argument (if present) is the short display form (crypt, see
# internal/sharepassword, scripts/share-to-web.sh) derived by cmd/backend
# from "nc-password-seed=..." - not the full share password.
#
# CLOUDWEB_URL: base URL of the server, default http://127.0.0.1:40080
#   (must run on the same host - the push endpoint only accepts requests
#   from localhost).
# CLOUDWEB_PASSWORD: fallback if no third argument was passed (e.g. for
#   manual calls without cmd/backend).
set -euo pipefail

if [ $# -lt 2 ]; then
  cat >&2 <<USAGE
Usage: $0 <qr-png-file> <link> [password]

This is the internal output hook (see MYSHARE_DISPLAY_HOOK) - not an
upload tool. It expects an already-finished QR code PNG (e.g. a file from
/var/lib/myshare/tmp/*.png) and only forwards it to cloudweb, it doesn't
upload anything to Nextcloud itself.

To share + display an arbitrary file instead:
  PRINTER=CloudWeb scripts/share.sh <file>
  scripts/share-to-web.sh <file> <name>   # additionally password-protected
USAGE
  exit 1
fi

PNG="$1"
LINK="$2"
PASSWORD="${3:-${CLOUDWEB_PASSWORD:-}}"

CLOUDWEB_URL="${CLOUDWEB_URL:-http://127.0.0.1:40080}"

curl -sf -X POST "$CLOUDWEB_URL/push" \
  -F "file=@${PNG}" \
  -F "link=${LINK}" \
  ${PASSWORD:+-F "password=${PASSWORD}"}
