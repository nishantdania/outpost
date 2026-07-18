#!/usr/bin/env bash

set -euo pipefail

export OUTPOST_PATH="${OUTPOST_PATH:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)}"
export OUTPOST_INSTALL="$OUTPOST_PATH/install"
export PATH="$OUTPOST_PATH/bin:$PATH"

source "$OUTPOST_INSTALL/preflight/all.sh"
source "$OUTPOST_INSTALL/helpers/all.sh"
source "$OUTPOST_INSTALL/post-install/all.sh"
