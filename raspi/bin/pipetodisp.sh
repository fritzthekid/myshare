#!/bin/bash
set -euo pipefail

# Deliberately not home-relative by default (the systemd service runs as
# root, and the delivering user may differ from whoever installed the
# service) - still overridable via the environment, same as
# fb-image-watcher.sh, and must point at the same directory as that
# script's WATCH_DIR for the two to actually talk to each other.
FB_IMAGE_WATCH_DIR=/tmp/fb-image-watcher
WATCH_DIR="${WATCH_DIR:-$FB_IMAGE_WATCH_DIR/fb-drop}"
BLACK_PNG="${BLACK_PNG:-$FB_IMAGE_WATCH_DIR/tmp/black.png}"

mkdir -p "$WATCH_DIR"
cat > "$WATCH_DIR/pic.png"

TIMEOUT=1

if [[ $# > 0 ]]; then
   TIMEOUT=$1
fi

setsid bash -c "
if [[  $# > 0 ]]; then
   sleep $TIMEOUT
   cp '$BLACK_PNG' '$WATCH_DIR/'
fi
" < /dev/null > /dev/null 2>&1 &
disown
