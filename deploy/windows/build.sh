#!/usr/bin/env bash
# Cross-compiles cloudweb.exe + fileshare.exe for Windows (amd64), from
# Linux or macOS - no Windows toolchain needed, pure Go, no cgo.
# See doc/windows_requirement.md.
#
# Usage: deploy/windows/build.sh [output-directory]
# Default output directory: dist/windows
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir="${1:-"$repo_root/dist/windows"}"

mkdir -p "$out_dir"
cd "$repo_root"

export GOOS=windows GOARCH=amd64 CGO_ENABLED=0

go build -o "$out_dir/cloudweb.exe" ./cmd/cloudweb
go build -o "$out_dir/fileshare.exe" ./cmd/fileshare
cp deploy/windows/install.ps1 "$out_dir/"
cp deploy/windows/userinstall.ps1 "$out_dir/"
cp deploy/windows/backend.env.example "$out_dir/"
cp deploy/windows/HowToInstall.txt "$out_dir/"

# Package into a ZIP - otherwise Windows SmartScreen warns on every
# individually downloaded .exe; as a ZIP, only once on extraction.
rm -f "$out_dir/cloudweb.zip"
zip -j "$out_dir/cloudweb.zip" \
    "$out_dir"/cloudweb.exe \
    "$out_dir"/fileshare.exe \
    "$out_dir"/install.ps1 \
    "$out_dir"/userinstall.ps1 \
    "$out_dir"/backend.env.example \
    "$out_dir"/HowToInstall.txt

echo "Done: $out_dir"
ls -la "$out_dir"
