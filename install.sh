#!/usr/bin/env bash
set -euo pipefail

handoff_work=
fail() { printf '%s\n' "$1" >&2; exit 1; }
usage() { printf '%s\n' 'Enter user@server for a remote server, local for this machine, or leave blank for client only.' >&2; }
https_url() { [[ $1 =~ ^https://[A-Za-z0-9.-]+(/[A-Za-z0-9._~/%+-]*)?$ ]]; }
ssh_target() { [[ $1 =~ ^[A-Za-z_][A-Za-z0-9_-]{0,31}@[A-Za-z0-9][A-Za-z0-9.-]{0,251}$ && $1 != *..* && $1 != *.-* && $1 != *-. ]]; }
valid_ipv4() {
 local IFS=. a b c d extra octet
 read -r a b c d extra <<< "$1"
 [[ -z ${extra:-} && -n ${a:-} && -n ${b:-} && -n ${c:-} && -n ${d:-} ]] || return 1
 for octet in "$a" "$b" "$c" "$d"; do [[ $octet =~ ^[0-9]{1,3}$ ]] && ((10#$octet <= 255)) || return 1; done
}
valid_server() {
 local address host port
 [[ $1 =~ ^https?://[^/]+$ ]] || return 1
 address=${1#*://}; host=${address%:*}; port=${address##*:}
 [[ $host != "$address" && $port =~ ^[1-9][0-9]{0,4}$ ]] && ((10#$port <= 65535)) && valid_ipv4 "$host"
}
valid_token() { [[ $1 =~ ^[A-Za-z0-9+/=]{20,256}$ ]]; }
config_value() {
 local key=$1 file=$2 line name value found=
 while IFS= read -r line || [[ -n $line ]]; do
  [[ $line == *=* ]] || return 1
  name=${line%%=*}; value=${line#*=}
  [[ $name == "$key" ]] || continue
  [[ -z ${found:-} ]] || return 1
  found=$value
 done < "$file"
 [[ -n ${found:-} ]] || return 1
 printf '%s' "$found"
}
valid_config() {
 local file=$1 server token lines
 [[ -f $file && ! -L $file ]] || return 1
 lines=$(wc -l < "$file")
 [[ $lines -eq 2 ]] || return 1
 server=$(config_value ARK_SERVER "$file") || return 1
 token=$(config_value ARK_TOKEN "$file") || return 1
 valid_server "$server" && valid_token "$token"
}
release_metadata() {
 local work=$1 api tag
 if [[ -n ${ARK_RELEASE_URL:-} || -n ${ARK_RELEASE_VERSION:-} || -n ${ARK_ASSETS_MANIFEST_SHA256:-} || -n ${ARK_INSTALL_SERVER_SHA256:-} ]]; then
  [[ -n ${ARK_RELEASE_URL:-} && -n ${ARK_RELEASE_VERSION:-} && -n ${ARK_ASSETS_MANIFEST_SHA256:-} && -n ${ARK_INSTALL_SERVER_SHA256:-} ]] || fail 'release automation values must be complete'
  release=$ARK_RELEASE_URL
  version=$ARK_RELEASE_VERSION
  manifest_sha=$ARK_ASSETS_MANIFEST_SHA256
  installer_sha=$ARK_INSTALL_SERVER_SHA256
 else
  api=https://github.com/nishantdania/ark/releases/latest
  tag=$(curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error --output /dev/null --write-out '%{url_effective}' "$api") || fail 'unable to resolve latest release'
  [[ $tag =~ ^https://github\.com/nishantdania/ark/releases/tag/(v[0-9]+\.[0-9]+\.[0-9]+)$ ]] || fail 'invalid latest release metadata'
  tag=${BASH_REMATCH[1]}
  version=${tag#v}
  release=https://github.com/nishantdania/ark/releases/download/$tag
 fi
 https_url "$release" || fail 'release URL must use a safe HTTPS URL'
 [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'invalid release version'
 [[ ${manifest_sha:-} =~ ^[a-f0-9]{64}$ || -z ${manifest_sha:-} ]] || fail 'invalid manifest checksum'
 [[ ${installer_sha:-} =~ ^[a-f0-9]{64}$ || -z ${installer_sha:-} ]] || fail 'invalid installer checksum'
 curl --fail --location --proto '=https' --tlsv1.2 -o "$work/checksums.txt" "$release/checksums.txt"
 curl --fail --location --proto '=https' --tlsv1.2 -o "$work/assets.json" "$release/assets.json"
 curl --fail --location --proto '=https' --tlsv1.2 -o "$work/install-server" "$release/install-server"
 local listed_assets listed_installer actual
 listed_assets=$(awk '$2 == "assets.json" { print $1 }' "$work/checksums.txt")
 listed_installer=$(awk '$2 == "install-server" { print $1 }' "$work/checksums.txt")
 [[ $listed_assets =~ ^[a-f0-9]{64}$ && $listed_installer =~ ^[a-f0-9]{64}$ ]] || fail 'invalid release checksums'
 actual=$(sha256sum "$work/assets.json" | awk '{print $1}')
 [[ $actual == "$listed_assets" ]] || fail 'assets manifest checksum mismatch'
 actual=$(sha256sum "$work/install-server" | awk '{print $1}')
 [[ $actual == "$listed_installer" ]] || fail 'server installer checksum mismatch'
 [[ -z ${manifest_sha:-} || $manifest_sha == "$listed_assets" ]] || fail 'assets manifest checksum mismatch'
 [[ -z ${installer_sha:-} || $installer_sha == "$listed_installer" ]] || fail 'server installer checksum mismatch'
 manifest_sha=$listed_assets
 installer_sha=$listed_installer
}
client_install() (
 local home bin lib work actual client_sha
 home=${HOME:?HOME is required}
 bin=${ARK_INSTALL_BIN_DIR:-$home/.local/bin}
 lib=${ARK_INSTALL_LIB_DIR:-$home/.local/lib/ark}
 [[ $bin == /* && $lib == /* ]] || fail 'install paths must be absolute'
 work=$(mktemp -d "${TMPDIR:-/tmp}/ark-client-install.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 client_sha=$(awk '$2 == "ark" { print $1 }' "$work/checksums.txt")
 [[ $client_sha =~ ^[a-f0-9]{64}$ ]] || fail 'invalid client release checksum'
 curl --fail --location --proto '=https' --tlsv1.2 -o "$work/ark" "$release/ark"
 actual=$(sha256sum "$work/ark" | awk '{print $1}')
 [[ $actual == "$client_sha" ]] || fail 'client checksum mismatch'
 install -d -m 0755 "$bin" "$lib"
 install -m 0755 "$work/ark" "$lib/ark-$version"
 cat > "$work/ark-wrapper" <<EOF
#!/usr/bin/env bash
set -euo pipefail
config=\${ARK_CONFIG_FILE:-\$HOME/.config/ark/server.env}
real=\${ARK_REAL_BINARY:-\$HOME/.local/lib/ark/ark-$version}
fail() { printf '%s\\n' "\$1" >&2; exit 1; }
[[ -f \$real && ! -L \$real ]] || fail 'Ark client is not installed'
[[ \$(sha256sum "\$real" | awk '{print \$1}') == $client_sha ]] || fail 'Ark client checksum mismatch'
if [[ -e \$config ]]; then
 [[ -f \$config && ! -L \$config ]] || fail 'invalid Ark configuration'
 mode=\$(stat -c %a "\$config")
 [[ \$mode =~ ^[0-7]00\$ ]] || fail 'Ark configuration must not be accessible by group or others'
 [[ \$(stat -c %u "\$config") == \$(id -u) ]] || fail 'Ark configuration must be owned by the current user'
 server= token= jump= count=0
 while IFS= read -r line || [[ -n \$line ]]; do
  [[ \$line == *=* ]] || fail 'invalid Ark configuration'
  key=\${line%%=*}; value=\${line#*=}; count=\$((count + 1))
  case \$key in
   ARK_SERVER) [[ -z \$server ]] || fail 'invalid Ark configuration'; server=\$value ;;
   ARK_TOKEN) [[ -z \$token ]] || fail 'invalid Ark configuration'; token=\$value ;;
   ARK_SSH_PROXY_JUMP) [[ -z \$jump ]] || fail 'invalid Ark configuration'; jump=\$value ;;
   *) fail 'invalid Ark configuration' ;;
  esac
 done < "\$config"
 valid_ipv4() { local IFS=. a b c d extra octet; read -r a b c d extra <<< "\$1"; [[ -z \${extra:-} && -n \${a:-} && -n \${b:-} && -n \${c:-} && -n \${d:-} ]] || return 1; for octet in "\$a" "\$b" "\$c" "\$d"; do [[ \$octet =~ ^[0-9]{1,3}\$ ]] && ((10#\$octet <= 255)) || return 1; done; }
 valid_server() { local address host port; [[ \$1 =~ ^https?://[^/]+\$ ]] || return 1; address=\${1#*://}; host=\${address%:*}; port=\${address##*:}; [[ \$host != "\$address" && \$port =~ ^[1-9][0-9]{0,4}\$ ]] && ((10#\$port <= 65535)) && valid_ipv4 "\$host"; }
 [[ \$count -eq 3 && \$token =~ ^[A-Za-z0-9+/=]{20,256}\$ && \$jump =~ ^[A-Za-z_][A-Za-z0-9_-]{0,31}@[A-Za-z0-9][A-Za-z0-9.-]{0,251}\$ && \$jump != *..* ]] && valid_server "\$server" || fail 'invalid Ark configuration'
 [[ -n \${ARK_SERVER:-} ]] || export ARK_SERVER=\$server
 [[ -n \${ARK_TOKEN:-} ]] || export ARK_TOKEN=\$token
 [[ -n \${ARK_SSH_PROXY_JUMP:-} ]] || export ARK_SSH_PROXY_JUMP=\$jump
fi
exec "\$real" "\$@"
EOF
 install -m 0755 "$work/ark-wrapper" "$bin/ark"
 printf 'Installed Ark %s\n' "$version"
)
install_server_packages() {
 local manager=${ARK_INSTALL_PACKAGE_MANAGER:-}
 if [[ -z $manager ]]; then
  if command -v apt-get >/dev/null 2>&1; then manager=apt; elif command -v pacman >/dev/null 2>&1; then manager=pacman; else fail 'apt-get or pacman is required'; fi
 fi
 case $manager in
  apt)
   sudo apt-get update
   sudo apt-get install -y curl zstd iproute2 nftables e2fsprogs openssh-client rsync podman uidmap fuse-overlayfs python3
   ;;
  pacman)
   sudo pacman -S --needed --noconfirm curl zstd iproute2 nftables e2fsprogs openssh rsync podman shadow fuse-overlayfs python
   ;;
  *) fail 'invalid package manager' ;;
 esac
}
server_install() (
 local work ip uplink actual
 [[ $(uname -s) == Linux ]] || fail 'Linux is required'
 [[ $(uname -m) == x86_64 ]] || fail 'x86-64 is required'
 command -v sudo >/dev/null 2>&1 || fail 'sudo is required'
 command -v tailscale >/dev/null 2>&1 || fail 'Tailscale is required'
 work=$(mktemp -d "${TMPDIR:-/tmp}/ark-server-install.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 install_server_packages
 ip=$(tailscale ip -4 | awk 'NR == 1 { print }')
 valid_ipv4 "$ip" || fail 'a Tailscale IPv4 address is required'
 uplink=$(ip route show default | awk 'NR == 1 { print $5 }')
 [[ $uplink =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || fail 'a default network uplink is required'
 if [[ ${ARK_INSTALL_TEST:-0} != 1 ]]; then
  [[ -r /dev/kvm && -c /dev/net/tun && -c /dev/userfaultfd && -f /sys/fs/cgroup/cgroup.controllers ]] || fail 'KVM, TUN, userfaultfd, and cgroup v2 are required'
 fi
 printf '%s\n' 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-ark.conf >/dev/null
 sudo sysctl --system >/dev/null
 actual=$(sha256sum "$work/install-server" | awk '{print $1}')
 [[ $actual == "$installer_sha" ]] || fail 'server installer checksum mismatch'
 sudo env ARK_VERSION="$version" ARK_UPLINK="$uplink" ARK_LISTEN="$ip:17890" ARK_SERVER="http://$ip:17890" ARK_RELEASE_URL="$release" ARK_ASSETS_MANIFEST_SHA256="$manifest_sha" sh "$work/install-server"
)
main() {
 local mode=${ARK_INSTALL_MODE:-} target=${ARK_INSTALL_TARGET:-} work server token config_dir config_tmp
 case $mode in ''|client|server) ;; *) fail 'invalid ARK_INSTALL_MODE' ;; esac
 if [[ $mode == server || $target == local ]]; then server_install; return; fi
 if [[ -z $target && $mode != client ]]; then
  [[ -r /dev/tty ]] || fail 'no terminal available; set ARK_INSTALL_MODE=client, server, or ARK_INSTALL_TARGET for automation'
  printf 'Ark server (user@server, local, or blank for client only): ' >/dev/tty
  IFS= read -r target </dev/tty || fail 'unable to read server target'
 fi
 if [[ -z $target ]]; then client_install; return; fi
 ssh_target "$target" || { usage; fail 'invalid SSH target'; }
 work=$(mktemp -d "${TMPDIR:-/tmp}/ark-release-metadata.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 rm -rf "$work"
 trap - EXIT HUP INT TERM
 client_install
 work=$(mktemp -d "${TMPDIR:-/tmp}/ark-handoff.XXXXXX")
 handoff_work=$work
 trap 'rm -rf "$handoff_work"' EXIT HUP INT TERM
 local remote_script remote_command ssh_stdin
 printf -v remote_script 'set -o pipefail; curl --fail --location --proto %q --tlsv1.2 %q | ARK_INSTALL_MODE=server ARK_RELEASE_URL=%q ARK_RELEASE_VERSION=%q ARK_ASSETS_MANIFEST_SHA256=%q ARK_INSTALL_SERVER_SHA256=%q bash' '=https' 'https://raw.githubusercontent.com/nishantdania/ark/main/install.sh' "$release" "$version" "$manifest_sha" "$installer_sha"
 printf -v remote_command 'bash -c %q' "$remote_script"
 ssh_stdin=${ARK_INSTALL_SSH_STDIN:-/dev/tty}
 [[ $ssh_stdin == /dev/tty || ${ARK_INSTALL_TEST:-0} == 1 ]] || fail 'SSH input must be /dev/tty'
 ssh -tt "$target" "$remote_command" < "$ssh_stdin"
 scp "$target:~/.config/ark/server.env" "$work/server.env"
 valid_config "$work/server.env" || fail 'server returned invalid configuration'
 server=$(config_value ARK_SERVER "$work/server.env")
 token=$(config_value ARK_TOKEN "$work/server.env")
 config_dir=${ARK_CONFIG_DIR:-$HOME/.config/ark}
 [[ $config_dir == /* ]] || fail 'ARK_CONFIG_DIR must be absolute'
 mkdir -p "$config_dir"
 chmod 0700 "$config_dir"
 config_tmp=$(mktemp "$config_dir/.server.env.XXXXXX")
 printf 'ARK_SERVER=%s\nARK_TOKEN=%s\nARK_SSH_PROXY_JUMP=%s\n' "$server" "$token" "$target" > "$config_tmp"
 chmod 0600 "$config_tmp"
 mv -f "$config_tmp" "$config_dir/server.env"
 printf 'Ark is ready. Run ark list.\n'
}
main
