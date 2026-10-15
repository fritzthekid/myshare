#!/bin/bash
# Watches a directory for new image files (e.g. dropped via scp), shows
# each new file on the framebuffer for TIMEOUT seconds, then clears the
# display again (black image).

set -euo pipefail

# Deliberately not home-relative by default (the service runs as root,
# and the delivering user may differ from whoever installed the
# service) - still overridable via the environment like FB_DEVICE/
# TIMEOUT/BLACK_PNG below.
FB_IMAGE_WATCH_DIR=/tmp/fb-image-watcher
mkdir -p "$FB_IMAGE_WATCH_DIR"
WATCH_DIR="${WATCH_DIR:-$FB_IMAGE_WATCH_DIR/fb-drop}"
WATCH_TMP_DIR="$FB_IMAGE_WATCH_DIR/tmp"
mkdir -p "$WATCH_DIR"
# Needs the execute bit too, not just rw, or a non-root user can't
# traverse into/write into this directory via scp.
chmod 777 "$WATCH_DIR"
mkdir -p "$WATCH_TMP_DIR"

# Which fb number the SPI TFT gets can vary (the vc4/HDMI driver can also
# claim fb0 or fb1, depending on probe timing) - if nothing shows up,
# check with `dmesg | grep -i fb_ili9486` instead of assuming /dev/fb1.
FB_DEVICE="${FB_DEVICE:-/dev/fb1}"
TIMEOUT="${TIMEOUT:-60}"
BLACK_PNG="$WATCH_TMP_DIR"/black.png

# Some SPI TFT overlays (e.g. waveshare35a) drive the panel with inverted
# colors compared to others (e.g. tft35a) - detect which overlay is
# active from the device tree instead of hardcoding one display model.
DISPLAY_TYPE=$(ls /proc/device-tree/soc/spi@7e204000/ 2>/dev/null | grep -iE '^(tft35a|waveshare35a)@' | head -1 | cut -d@ -f1)
if [ "$DISPLAY_TYPE" = "waveshare35a" ]; then
    INVERT=1
else
    INVERT=0
fi

# Clean starting state: kill orphaned fbi processes from a previous run
# (e.g. after a crash/restart) before starting new ones.
pkill -x fbi 2>/dev/null || true

# Regenerated on every start (not just once) since INVERT can differ
# between runs if the SD card/service config moves to a different Pi.
RES=$(fbset -fb "$FB_DEVICE" -s | awk '/geometry/ {print $2"x"$3}')
RES="${RES:-480x320}"
if [ "$INVERT" = "1" ]; then
    convert -size "$RES" xc:white "$BLACK_PNG"
else
    convert -size "$RES" xc:black "$BLACK_PNG"
fi

CURRENT_FILE="$WATCH_TMP_DIR"/current-file
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

    if [ "$INVERT" = "1" ]; then
        convert "$file" -background white -alpha remove -alpha off -negate "$CURRENT_FILE"
    else
        cp "$file" "$CURRENT_FILE"
    fi

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
