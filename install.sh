#!/usr/bin/env bash
# Install a local CLI and, optionally, one launcher on this host or over SSH.
set -euo pipefail
fail() { printf '%s\n' "$*" >&2; exit 1; }
tty_available() { ( : </dev/tty ) 2>/dev/null; }
for command in curl python3 sha256sum install; do command -v "$command" >/dev/null || fail "missing $command"; done
[[ $(uname -s):$(uname -m) == Linux:x86_64 ]] || fail 'Linux x86-64 is required'
[[ $# -le 1 ]] || fail 'usage: install.sh [local|user@host]'
target=${1:-${OUTPOST_INSTALL_TARGET:-}}
if [[ $# == 0 && -z $target ]] && tty_available; then
 printf 'Launcher target: local, user@host, or empty for CLI only: ' >/dev/tty
 IFS= read -r target </dev/tty || :
fi
[[ -z $target || $target == local || $target =~ ^[A-Za-z_][A-Za-z0-9_-]{0,31}@[A-Za-z0-9][A-Za-z0-9.-]{0,251}$ ]] || fail 'invalid launcher target'
admin_user=${OUTPOST_ADMIN_USER:-}
if [[ -n $target && ! ${OUTPOST_ADMIN_USER+x} ]] && tty_available; then
 if [[ $target == local ]]; then candidate=${SUDO_USER:-$(id -un)}; else candidate=${target%%@*}; fi
 printf 'Grant %s privileged Outpost management access on the VM host? [y/N]: ' "$candidate" >/dev/tty
 IFS= read -r answer </dev/tty || answer=
 case "$answer" in y|Y|yes|YES) admin_user=$candidate ;; esac
fi
[[ -z $admin_user || $admin_user =~ ^[A-Za-z_][A-Za-z0-9_-]{0,31}$ ]] || fail 'invalid administrator username'
[[ -n $target || -z $admin_user ]] || fail 'administrator access requires a launcher target'
if [[ $target == local ]]; then
 command -v sudo >/dev/null || fail 'local launcher installation requires sudo'
 sudo -v || fail 'sudo authentication failed; nothing installed'
elif [[ -n $target ]]; then
 command -v ssh >/dev/null || fail 'remote installation requires OpenSSH'
fi
version=${OUTPOST_VERSION:-}
if [[ -z $version ]]; then
 version=$(curl --fail --location --proto '=https' --tlsv1.2 -sS https://api.github.com/repos/nishantdania/outpost/releases/latest | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"].removeprefix("v"))')
fi
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'invalid release version'
base=${OUTPOST_RELEASE_URL:-https://github.com/nishantdania/outpost/releases/download/v$version}
[[ $base =~ ^https://[A-Za-z0-9._~:/%+-]+$ ]] || fail 'invalid HTTPS release URL'
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl --fail --location --proto '=https' --tlsv1.2 -sS -o "$work/checksums.txt" "$base/checksums.txt"
curl --fail --location --proto '=https' --tlsv1.2 -sS -o "$work/outpost" "$base/outpost"
curl --fail --location --proto '=https' --tlsv1.2 -sS -o "$work/assets.json" "$base/assets.json"
checksum() {
 local value
 value=$(awk -v file="$1" '$2 == file {print $1}' "$work/checksums.txt")
 [[ $value =~ ^[a-f0-9]{64}$ ]] || fail "missing or ambiguous checksum for $1"
 printf '%s' "$value"
}
[[ $(sha256sum "$work/outpost" | awk '{print $1}') == "$(checksum outpost)" ]] || fail 'client checksum mismatch'
[[ $(sha256sum "$work/assets.json" | awk '{print $1}') == "$(checksum assets.json)" ]] || fail 'manifest checksum mismatch'
install_client() {
 install -d -m 0755 "$HOME/.local/bin"
 install -m 0755 "$work/outpost" "$HOME/.local/bin/.outpost.new"
 mv "$HOME/.local/bin/.outpost.new" "$HOME/.local/bin/outpost"
 printf 'Installed CLI %s in ~/.local/bin/outpost. It connects to a local launcher socket.\n' "$version"
}
if [[ -z $target ]]; then
 install_client
 exit 0
fi
installer_sha=$(checksum install-server)
manifest_sha=$(checksum assets.json)
# Values interpolated into this remote shell command are restricted above to
# URL-safe characters, numeric semver, and hex digests. No credentials/handoff.
script="set -eu
umask 077
work=\$(mktemp -d)
trap 'rm -rf \"\$work\"' EXIT
curl --fail --location --proto '=https' --tlsv1.2 -sS -o \"\$work/install-server\" '$base/install-server'
printf '%s  %s\\n' '$installer_sha' \"\$work/install-server\" | sha256sum -c -
sudo env OUTPOST_VERSION='$version' OUTPOST_RELEASE_URL='$base' OUTPOST_ASSETS_MANIFEST_SHA256='$manifest_sha' OUTPOST_ADMIN_USER='$admin_user' sh \"\$work/install-server\""
if [[ $target == local ]]; then
 sh -c "$script"
 printf 'Use sudo outpost list, or grant trusted users membership in the outpost group.\n'
else
 command -v ssh >/dev/null || fail 'remote installation requires OpenSSH'
 ssh -tt -- "$target" "$script" </dev/tty
 printf 'Use the CLI on the host, for example: ssh -t %s sudo outpost list\n' "$target"
fi
# Do not replace a working client when server installation fails.
install_client
