#!/usr/bin/env bash

set -euo pipefail

component="outpost"
if [[ "${1:-}" == "--daemon" ]]; then
  component="outpostd"
elif [[ $# -ne 0 ]]; then
  echo "usage: install.sh [--daemon]" >&2
  exit 2
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [[ "$component" == "outpostd" && "$os" != "linux" ]]; then
  echo "outpostd is currently available for Linux only" >&2
  exit 1
fi

archive="${component}_${os}_${arch}.tar.gz"
base_url="https://github.com/nishantdania/outpost/releases/latest/download"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

curl -fsSL "$base_url/checksums.txt" -o "$tmpdir/checksums.txt"
curl -fsSL "$base_url/$archive" -o "$tmpdir/$archive"
expected="$(awk -v name="$archive" '$2 == name { print $1 }' "$tmpdir/checksums.txt")"
if [[ -z "$expected" ]]; then
  echo "checksum not found for $archive" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmpdir/$archive" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$tmpdir/$archive" | awk '{print $1}')"
fi
if [[ "$actual" != "$expected" ]]; then
  echo "checksum verification failed" >&2
  exit 1
fi

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
install_dir="${OUTPOST_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$install_dir"
install -m 0755 "$tmpdir/$component" "$install_dir/$component"
printf 'Installed %s to %s\n' "$component" "$install_dir/$component"

if [[ "$component" == "outpostd" ]]; then
  unit_dir="$HOME/.config/systemd/user"
  mkdir -p "$unit_dir"
  cat > "$unit_dir/outpostd.service" <<EOF
[Unit]
Description=Outpost daemon

[Service]
ExecStart=$install_dir/outpostd
KillMode=process
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
EOF
  systemctl --user daemon-reload
  printf 'Created %s/outpostd.service\n' "$unit_dir"
fi
