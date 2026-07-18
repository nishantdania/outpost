#!/usr/bin/env bash

set -euo pipefail

export OUTPOST_ONLINE_INSTALL=true
OUTPOST_REF="${OUTPOST_REF:-main}"
OUTPOST_REPOSITORY="${OUTPOST_REPOSITORY:-nishantdania/outpost}"
OUTPOST_PATH="${OUTPOST_PATH:-$HOME/.local/share/outpost}"

command -v git >/dev/null || sudo pacman -S --needed --noconfirm git

echo "Cloning Outpost from: https://github.com/${OUTPOST_REPOSITORY}.git"
echo "Using branch: $OUTPOST_REF"
rm -rf "$OUTPOST_PATH"
git clone --branch "$OUTPOST_REF" "https://github.com/${OUTPOST_REPOSITORY}.git" "$OUTPOST_PATH"

source "$OUTPOST_PATH/install.sh"
