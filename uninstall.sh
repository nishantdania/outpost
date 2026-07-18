#!/usr/bin/env bash

set -euo pipefail

readonly OUTPOST_PATH="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly OUTPOST_INSTALLED_PATH="${OUTPOST_INSTALLED_PATH:-$HOME/.local/share/outpost}"

[ "$#" -eq 0 ] || {
    echo "Usage: uninstall.sh" >&2
    exit 2
}

[ "$OUTPOST_PATH" = "$OUTPOST_INSTALLED_PATH" ] || {
    echo "Error: uninstall.sh only removes the installed checkout at $OUTPOST_INSTALLED_PATH." >&2
    exit 1
}

source "$OUTPOST_PATH/helpers/all.sh"

gum confirm "Remove Outpost at $OUTPOST_PATH and image assets at $OUTPOST_DEFAULT_IMAGE_DIR?" || exit 0
rm -rf "$OUTPOST_DEFAULT_IMAGE_DIR"
rm -rf "$OUTPOST_PATH"
outpost_info "Removed Outpost"
