#!/usr/bin/env bash
# Shares a file via one of the already set-up CUPS queues - same call as
# sendfile, but without the calling user needing access to the Nextcloud
# credentials themselves (those live exclusively with the CUPS backend,
# see /etc/myshare/backend.env). Requires deploy/install.sh to have
# already run on this machine.
#
# Usage:
#   scripts/share.sh path/to/test.zip
#   scripts/share.sh path/to/test.zip "Other Title.zip"
#   cat test.zip | scripts/share.sh - test.zip
#   PRINTER=CloudWeb scripts/share.sh path/to/test.zip   # a different queue
set -euo pipefail

PRINTER="${PRINTER:-CloudToRaspi}"

if [ $# -lt 1 ]; then
  echo "Usage: $0 <path|-> [title]" >&2
  exit 1
fi

PATH_ARG="$1"

if [ "$PATH_ARG" = "-" ]; then
  TITLE="${2:-file}"
  exec lp -d "$PRINTER" -t "$TITLE"
fi

TITLE="${2:-$(basename "$PATH_ARG")}"
exec lp -d "$PRINTER" -t "$TITLE" "$PATH_ARG"
