#!/usr/bin/env bash
# Builds all Linux binaries at the locations deploy/install.sh expects -
# one command instead of "go build -o X ./cmd/X" for each one individually
# (source of a recurring "install.sh installs a stale binary" bug).
#
# Usage: deploy/build.sh
# Afterwards: sudo ./deploy/install.sh
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go build -o backend ./cmd/backend
go build -o sendfile ./cmd/sendfile
go build -o webshare ./cmd/webshare
go build -o cloudweb ./cmd/cloudweb

echo "Done. Now: sudo ./deploy/install.sh"
