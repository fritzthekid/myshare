#!/bin/bash
# Watches a directory for new image files (e.g. dropped via scp), shows
# each new file on the framebuffer for TIMEOUT seconds, then clears the
# display again (black image).

set -euo pipefail

WATCH_DIR="${WATCH_DIR:-$HOME/fb-drop}"
FB_DEVICE="${FB_DEVICE:-/dev/fb1}"
TIMEOUT="${TIMEOUT:-60}"
BLACK_PNG="${BLACK_PNG:-$HOME/.fb-image-watcher/black.png}"

mkdir -p "$WATCH_DIR" "$(dirname "$BLACK_PNG")"

# Clean starting state: kill orphaned fbi processes from a previous run
# (e.g. after a crash/restart) before starting new ones.
pkill -x fbi 2>/dev/null || true

if [ ! -f "$BLACK_PNG" ]; then
    RES=$(fbset -fb "$FB_DEVICE" -s | awk '/geometry/ {print $2"x"$3}')
    RES="${RES:-480x320}"
    convert -size "$RES" xc:black "$BLACK_PNG"
fi

CURRENT_FILE=/tmp/fb-image-watcher/current-file
GEN_FILE=/tmp/fb-image-watcher.pid

# "setsid cmd &" often yields in $! only the PID of the short-lived setsid
# wrapper (which forks internally because the background job is already a
# process group leader) instead of the actual fbi process's PID - a "kill
# $!" based on that would therefore never hit the real fbi. Hence a blanket
# "pkill -x fbi" here instead (on this dedicated display Pi, fbi runs
# exclusively for this purpose).
clear_screen() {
    pkill -x fbi 2>/dev/null || true
    fbi -d "$FB_DEVICE" -T 1 -noverbose -once "$BLACK_PNG"
}

show_image() {
    local file="$1"
    local token
    token=$(date +%s%N)

    pkill -x fbi 2>/dev/null || true
    mkdir -p "$(dirname "$CURRENT_FILE")"
    cp "$file" "$CURRENT_FILE"

    setsid fbi -d "$FB_DEVICE" -T 1 -noverbose -a "$CURRENT_FILE" &
    echo "$token" > "$GEN_FILE"
    (
        sleep "$TIMEOUT"
        # $GEN_FILE gets overwritten on every new image; only clear if no
        # newer image has arrived in the meantime.
        if [ "$(cat "$GEN_FILE" 2>/dev/null)" = "$token" ]; then
            clear_screen
        fi
    ) &
}

echo "Watching $WATCH_DIR (display on $FB_DEVICE, timeout ${TIMEOUT}s)..."

inotifywait -m -e close_write --format '%f' "$WATCH_DIR" | while read -r NAME; do
    FILE="$WATCH_DIR/$NAME"
    [ -f "$FILE" ] || continue
    show_image "$FILE"
    # sleep 1
    rm -f "$FILE"
done
