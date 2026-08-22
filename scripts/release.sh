#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/release.sh <version>" >&2
  exit 2
fi

version="$1"
outdir="dist/$version"
rm -rf "$outdir"
mkdir -p "$outdir"

build() {
  local binary="$1"
  local os="$2"
  local arch="$3"
  GOOS="$os" GOARCH="$arch" go build -ldflags "-X main.version=$version" -o "$outdir/$binary" "./cmd/$binary"
  tar -C "$outdir" -czf "$outdir/${binary}_${os}_${arch}.tar.gz" "$binary"
  rm "$outdir/$binary"
}

build outpost linux amd64
build outpost linux arm64
build outpost darwin amd64
build outpost darwin arm64
build outpostd linux amd64
build outpostd linux arm64

(
  cd "$outdir"
  sha256sum *.tar.gz > checksums.txt
)
