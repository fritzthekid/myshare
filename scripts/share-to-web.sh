#!/usr/bin/env bash
# Shares a file via the "CloudWeb" queue, optionally additionally protected
# with a password derived from <name> (+ this machine's MAC address, see
# internal/sharepassword, cmd/backend). Meant especially for larger/more
# sensitive files (e.g. ZIP archives), where the link/QR code alone
# shouldn't be considered sufficiently protected.
#
# IMPORTANT: Only the displayed short form (crypt) appears on the cloudweb
# display - <name> itself must be known to the recipient independently
# (e.g. agreed verbally) and typed in as "<name><crypt>" combined as the
# password when opening the share link.
#
# Usage:
#   scripts/share-to-web.sh path/to/test.zip           # without password protection
#   scripts/share-to-web.sh path/to/test.zip "name"    # with password protection
#   cat test.zip | scripts/share-to-web.sh - "name"
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <path|-> [name]" >&2
  exit 1
fi

PATH_ARG="$1"
NAME="${2:-}"

OPTS=()
if [ -n "$NAME" ]; then
  OPTS=(-o "nc-password-seed=${NAME}")
fi

if [ "$PATH_ARG" = "-" ]; then
  exec lp -d CloudWeb "${OPTS[@]}" -t "file"
fi

exec lp -d CloudWeb "${OPTS[@]}" -t "$(basename "$PATH_ARG")" "$PATH_ARG"
