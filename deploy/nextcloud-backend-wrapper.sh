#!/bin/sh
# Called by cupsd as /usr/lib/cups/backend/nextcloud. CUPS starts backends
# with a minimal environment, so the NC_* variables have to be reloaded
# from a root-only readable file before the actual (compiled) backend runs.
set -a
[ -r /etc/myshare/backend.env ] && . /etc/myshare/backend.env
set +a

# CUPS sets $PRINTER to the name of the queue the job was printed through -
# this lets multiple queues using the same backend code be permanently
# routed to different display targets, instead of sharing a single global
# hook in backend.env. Without a match, the MYSHARE_DISPLAY_HOOK default
# from backend.env applies (if set).
case "$PRINTER" in
  CloudToRaspi)
    export MYSHARE_DISPLAY_HOOK=/usr/local/lib/myshare/toraspi.sh
    ;;
  CloudWeb)
    export MYSHARE_DISPLAY_HOOK=/usr/local/lib/myshare/push-to-cloudweb.sh
    ;;
esac

exec /usr/local/lib/myshare/backend "$@"
