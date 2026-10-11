#!/usr/bin/env bash
# Generates a personalized copy of web/share.html with baked-in Nextcloud
# credentials, so no input is needed anymore on the phone: copy one file to
# the phone, open it, done.
#
# Recommendation: use a dedicated app password for this (Nextcloud web UI
# -> Settings -> Security -> "Create new app password", e.g. named
# "phone-share"), separate from the CUPS backend's app password -
# independently revocable if the phone is lost. Even better: a dedicated
# Nextcloud user restricted to one folder instead of the main account (as
# the existing "printer" user already demonstrates for the CUPS backend
# part).
#
# Usage:
#   ./deploy/gen-share-page.sh /etc/myshare/backend.env > my-phone.html
# or with your own values in the environment:
#   NC_BASE_URL=... NC_USERNAME=... NC_PASSWORD=... ./deploy/gen-share-page.sh > my-phone.html
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE="$REPO_ROOT/web/share.html"

if [ "${1:-}" ]; then
  if [ ! -r "$1" ]; then
    echo "Env file not readable: $1" >&2
    exit 1
  fi
  set -a
  # shellcheck disable=SC1090
  . "$1"
  set +a
fi

: "${NC_BASE_URL:?NC_BASE_URL missing (pass an env file as an argument or export the variables)}"
: "${NC_USERNAME:?NC_USERNAME missing}"
: "${NC_PASSWORD:?NC_PASSWORD missing}"
TARGET_DIR="${NC_TARGET_DIR:-/PrinterUploads}"
EXPIRE_DAYS="${NC_LINK_EXPIRE_DAYS:-1}"

if ! [[ "$EXPIRE_DAYS" =~ ^[0-9]+$ ]]; then
  echo "NC_LINK_EXPIRE_DAYS must be a number, but is: $EXPIRE_DAYS" >&2
  exit 1
fi

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

CONFIG_JSON=$(printf '{"baseUrl":"%s","username":"%s","password":"%s","targetDir":"%s","linkExpireDays":%s}' \
  "$(json_escape "${NC_BASE_URL%/}")" \
  "$(json_escape "$NC_USERNAME")" \
  "$(json_escape "$NC_PASSWORD")" \
  "$(json_escape "$TARGET_DIR")" \
  "$EXPIRE_DAYS")

# Inject CONFIG_JSON via ENVIRON instead of "awk -v": "-v" assignments
# interpret backslash escapes (a POSIX awk quirk) and would undo the \" and
# \\ we just produced - ENVIRON values, on the other hand, stay unchanged.
MYSHARE_CFG_JSON="$CONFIG_JSON" awk '
  /<!--MYSHARE_CONFIG-->/ { print "<script>window.MYSHARE_CONFIG = " ENVIRON["MYSHARE_CFG_JSON"] ";</script>"; next }
  { print }
' "$TEMPLATE"
