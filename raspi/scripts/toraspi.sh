#!/bin/bash
# Output hook for backend/sendfile (see internal/output.Hook, activated via
# MYSHARE_DISPLAY_HOOK=/path/to/toraspi.sh in backend.env).
#
# Called by the Go code as: toraspi.sh <qr-png-path> <link>
#
# Composes the QR code with the link as text next to it and sends the
# result via ssh to a Raspberry Pi's display (raspi/bin/pipetodisp.sh +
# raspi/services/fb-image-watcher.service there).
set -euo pipefail

PNG="$1"
LINK="$2"

RASPI_HOST="${RASPI_HOST:-raspi.local}"
RASPI_DISPLAY_TIMEOUT="${RASPI_DISPLAY_TIMEOUT:-10}"
OUT="$(dirname "$PNG")/output.png"

TEXT="${LINK#https://}"
TEXT="${TEXT#http://}"
TEXT="$(printf '%s' "$TEXT" | fold -w 24 -s)"

convert "$PNG" \( -size 180x320 -gravity center -background black -fill white -pointsize 20 label:"$TEXT" \) +append "$OUT"
ssh "$RASPI_HOST" pipetodisp.sh "$RASPI_DISPLAY_TIMEOUT" < "$OUT"
