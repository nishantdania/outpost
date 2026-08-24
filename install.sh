#!/usr/bin/env bash
set -euo pipefail

handoff_work=
server_uninstaller_b64='IyEvYmluL3NoCnNldCAtZXUKWyAiJCMiIC1lcSAwIF0gfHwgZXhpdCAxCnJvb3Q9JHtPVVRQT1NUX1VOSU5TVEFMTF9ST09UOi19CnRlc3RfbW9kZT0ke09VVFBPU1RfVU5JTlNUQUxMX1RFU1Q6LTB9ClsgLXogIiRyb290IiBdIHx8IFsgIiR0ZXN0X21vZGUiID0gMSBdIHx8IGV4aXQgMQpjYXNlICIkcm9vdCIgaW4gJyd8LyopIDo7OyAqKSBleGl0IDE7OyBlc2FjClsgIiR0ZXN0X21vZGUiID0gMSBdIHx8IFsgIiQoaWQgLXUpIiA9IDAgXSB8fCBleGl0IDEKbG9nKCkgeyBbIC16ICIke09VVFBPU1RfVU5JTlNUQUxMX1RFU1RfTE9HOi19IiBdIHx8IHByaW50ZiAnJXNcbicgIiQxIiA+PiAiJE9VVFBPU1RfVU5JTlNUQUxMX1RFU1RfTE9HIjsgfQpyZW1vdmUoKSB7IGlmIFsgLWUgIiRyb290JDEiIF0gfHwgWyAtTCAiJHJvb3QkMSIgXTsgdGhlbiBybSAtcmYgLS0gIiRyb290JDEiOyBmaTsgfQppbmFjdGl2ZSgpIHsgWyAiJChzeXN0ZW1jdGwgc2hvdyAtcCBBY3RpdmVTdGF0ZSAtLXZhbHVlICIkMS5zZXJ2aWNlIikiID0gaW5hY3RpdmUgXTsgfQpzdG9wKCkgeyBzeXN0ZW1jdGwgc3RvcCAiJDEuc2VydmljZSIgfHwgWyAiJChzeXN0ZW1jdGwgc2hvdyAtcCBMb2FkU3RhdGUgLS12YWx1ZSAiJDEuc2VydmljZSIpIiA9IG5vdC1mb3VuZCBdOyBpbmFjdGl2ZSAiJDEiOyB9CnJlYWR5KCkgewogWyAiJHRlc3RfbW9kZSIgPSAxIF0gJiYgWyAiJHtPVVRQT1NUX1VOSU5TVEFMTF9URVNUX1JFQURZOi19IiA9IDEgXSAmJiByZXR1cm4KIHB5dGhvbjMgLSAiJHJvb3QvcnVuL291dHBvc3Qvdm0tbGF1bmNoZXIuc29jayIgIiR7T1VUUE9TVF9VTklOU1RBTExfUkVBRFlfVElNRU9VVDotMzB9IiA8PCdQWScKaW1wb3J0IHNvY2tldCxzeXMsdGltZQpwYXRoLHRpbWVvdXQ9c3lzLmFyZ3ZbMV0saW50KHN5cy5hcmd2WzJdKTsgZW5kPXRpbWUubW9ub3RvbmljKCkrdGltZW91dAp3aGlsZSB0aW1lLm1vbm90b25pYygpPGVuZDoKIHRyeToKICBzPXNvY2tldC5zb2NrZXQoc29ja2V0LkFGX1VOSVgpOyBzLnNldHRpbWVvdXQoLjIpOyBzLmNvbm5lY3QocGF0aCk7IHMuY2xvc2UoKTsgcmFpc2UgU3lzdGVtRXhpdCgwKQogZXhjZXB0IE9TRXJyb3I6IHRpbWUuc2xlZXAoLjEpCnJhaXNlIFN5c3RlbUV4aXQoMSkKUFkKfQpoYW5kb2ZmPScnOyBjcmVhdGVkX291dHBvc3RkX3VzZXI9Jyc7IGNyZWF0ZWRfb3V0cG9zdHZtX3VzZXI9Jyc7IGNyZWF0ZWRfb3V0cG9zdGRfZ3JvdXA9Jyc7IGNyZWF0ZWRfb3V0cG9zdHZtX2dyb3VwPScnOyBjcmVhdGVkX3N1YnVpZD0nJzsgY3JlYXRlZF9zdWJnaWQ9Jyc7IGlwX3ByZXZpb3VzPScnOyBjaGFuZ2VkX2lwPScnOyBjcmVhdGVkX3N5c2N0bD0nJwpmaWxlPSRyb290L2V0Yy9vdXRwb3N0ZC91bmluc3RhbGwuZW52CmlmIFsgLWYgIiRmaWxlIiBdICYmIFsgISAtTCAiJGZpbGUiIF07IHRoZW4KIFsgIiR0ZXN0X21vZGUiID0gMSBdIHx8IFsgIiQoc3RhdCAtYyAldTolYSAiJGZpbGUiKSIgPSAwOjYwMCBdIHx8IGV4aXQgMQogeyBJRlM9IHJlYWQgLXIgb25lOyBJRlM9IHJlYWQgLXIgdHdvOyBJRlM9IHJlYWQgLXIgdGhyZWU7IElGUz0gcmVhZCAtciBmb3VyOyBJRlM9IHJlYWQgLXIgZml2ZTsgSUZTPSByZWFkIC1yIHNpeDsgSUZTPSByZWFkIC1yIHNldmVuOyBJRlM9IHJlYWQgLXIgZWlnaHQ7IElGUz0gcmVhZCAtciBuaW5lOyBJRlM9IHJlYWQgLXIgdGVuOyBJRlM9IHJlYWQgLXIgZXh0cmEgfHwgOjsgfSA8ICIkZmlsZSIKIFsgLXogIiRleHRyYSIgXSB8fCBleGl0IDEKIHNldCAtLSAiJG9uZSIgIiR0d28iICIkdGhyZWUiICIkZm91ciIgIiRmaXZlIiAiJHNpeCIgIiRzZXZlbiIgIiRlaWdodCIgIiRuaW5lIiAiJHRlbiIKIGNhc2UgIiQxOiQyOiQzOiQ0OiQ1OiQ2OiQ3OiQ4OiQ5OiR7MTB9IiBpbgogIE9VVFBPU1RfSEFORE9GRj0qOk9VVFBPU1RfQ1JFQVRFRF9PVVRQT1NURF9VU0VSPVswMV06T1VUUE9TVF9DUkVBVEVEX09VVFBPU1RWTV9VU0VSPVswMV06T1VUUE9TVF9DUkVBVEVEX09VVFBPU1REX0dST1VQPVswMV06T1VUUE9TVF9DUkVBVEVEX09VVFBPU1RWTV9HUk9VUD1bMDFdOk9VVFBPU1RfQ1JFQVRFRF9PVVRQT1NURF9TVUJVSUQ9WzAxXTpPVVRQT1NUX0NSRUFURURfT1VUUE9TVERfU1VCR0lEPVswMV06T1VUUE9TVF9JUF9GT1JXQVJEX1BSRVZJT1VTPVswMV06T1VUUE9TVF9DSEFOR0VEX0lQX0ZPUldBUkQ9WzAxXTpPVVRQT1NUX0NSRUFURURfU1lTQ1RMPVswMV0pIDo7OyAqKSBleGl0IDE7OwogZXNhYwogaGFuZG9mZj0kezEjT1VUUE9TVF9IQU5ET0ZGPX0KIGlmIFsgIiR0ZXN0X21vZGUiID0gMSBdOyB0aGVuCiAgY2FzZSAiJGhhbmRvZmYiIGluIC8qLy5jb25maWcvb3V0cG9zdC9zZXJ2ZXIuZW52KSA6OzsgKikgZXhpdCAxOzsgZXNhYwogZWxzZQogIGNhc2UgIiRoYW5kb2ZmIiBpbgogICAvcm9vdC8uY29uZmlnL291dHBvc3Qvc2VydmVyLmVudikgOiA7OwogICAvaG9tZS8qLy5jb25maWcvb3V0cG9zdC9zZXJ2ZXIuZW52KQogICAgdXNlcj0ke2hhbmRvZmYjL2hvbWUvfTsgdXNlcj0ke3VzZXIlJS8qfQogICAgY2FzZSAiJHVzZXIiIGluICcnfCpbIUEtWmEtejAtOV8uLV0qKSBleGl0IDE7OyBlc2FjCiAgICBbICIkaGFuZG9mZiIgPSAiL2hvbWUvJHVzZXIvLmNvbmZpZy9vdXRwb3N0L3NlcnZlci5lbnYiIF0gfHwgZXhpdCAxCiAgICA7OwogICAqKSBleGl0IDEgOzsKICBlc2FjCiBmaQogY3JlYXRlZF9vdXRwb3N0ZF91c2VyPSR7MiMqPX07IGNyZWF0ZWRfb3V0cG9zdHZtX3VzZXI9JHszIyo9fTsgY3JlYXRlZF9vdXRwb3N0ZF9ncm91cD0kezQjKj19OyBjcmVhdGVkX291dHBvc3R2bV9ncm91cD0kezUjKj19OyBjcmVhdGVkX3N1YnVpZD0kezYjKj19OyBjcmVhdGVkX3N1YmdpZD0kezcjKj19OyBpcF9wcmV2aW91cz0kezgjKj19OyBjaGFuZ2VkX2lwPSR7OSMqPX07IGNyZWF0ZWRfc3lzY3RsPSR7MTAjKj19CmZpCmxhdW5jaGVyX2xvYWQ9JChzeXN0ZW1jdGwgc2hvdyAtcCBMb2FkU3RhdGUgLS12YWx1ZSBvdXRwb3N0LXZtLWxhdW5jaGVyLnNlcnZpY2UpCmxvZyBzdG9wLW91dHBvc3RkOyBzdG9wIG91dHBvc3RkIHx8IGV4aXQgMQpsb2cgc3RvcC1sYXVuY2hlcjsgc3RvcCBvdXRwb3N0LXZtLWxhdW5jaGVyIHx8IGV4aXQgMQppZiBbICIkbGF1bmNoZXJfbG9hZCIgIT0gbm90LWZvdW5kIF07IHRoZW4KIGxvZyByZWNvbmNpbGUtbGF1bmNoZXI7IHN5c3RlbWN0bCBzdGFydCBvdXRwb3N0LXZtLWxhdW5jaGVyLnNlcnZpY2UgJiYgc3lzdGVtY3RsIGlzLWFjdGl2ZSAtLXF1aWV0IG91dHBvc3Qtdm0tbGF1bmNoZXIuc2VydmljZSAmJiByZWFkeSB8fCBleGl0IDEKIGxvZyBzdG9wLXJlY29uY2lsZWQtbGF1bmNoZXI7IHN0b3Agb3V0cG9zdC12bS1sYXVuY2hlciB8fCBleGl0IDEKZmkKc3lzdGVtY3RsIGRpc2FibGUgb3V0cG9zdGQuc2VydmljZSB8fCBbICIkKHN5c3RlbWN0bCBzaG93IC1wIExvYWRTdGF0ZSAtLXZhbHVlIG91dHBvc3RkLnNlcnZpY2UpIiA9IG5vdC1mb3VuZCBdIHx8IGV4aXQgMQpzeXN0ZW1jdGwgZGlzYWJsZSBvdXRwb3N0LXZtLWxhdW5jaGVyLnNlcnZpY2UgfHwgWyAiJGxhdW5jaGVyX2xvYWQiID0gbm90LWZvdW5kIF0gfHwgZXhpdCAxClsgLXogIiRoYW5kb2ZmIiBdIHx8IHJlbW92ZSAiJGhhbmRvZmYiClsgIiRjcmVhdGVkX3N5c2N0bCIgIT0gMSBdIHx8IHJlbW92ZSAvZXRjL3N5c2N0bC5kLzk5LW91dHBvc3QuY29uZgpyZW1vdmUgL2V0Yy9zeXN0ZW1kL3N5c3RlbS9vdXRwb3N0ZC5zZXJ2aWNlCnJlbW92ZSAvZXRjL3N5c3RlbWQvc3lzdGVtL291dHBvc3Qtdm0tbGF1bmNoZXIuc2VydmljZQpyZW1vdmUgL3Vzci9sb2NhbC9iaW4vb3V0cG9zdApyZW1vdmUgL3Vzci9sb2NhbC9saWIvb3V0cG9zdApyZW1vdmUgL2V0Yy9vdXRwb3N0ZApyZW1vdmUgL3Zhci9saWIvb3V0cG9zdGQKcmVtb3ZlIC9zcnYvb3V0cG9zdC9zdGF0ZQpyZW1vdmUgL3Nydi9vdXRwb3N0L2phaWxlcgpybWRpciAtLSAiJHJvb3Qvc3J2L291dHBvc3QiIDI+L2Rldi9udWxsIHx8IDoKcmVtb3ZlIC9ydW4vb3V0cG9zdAppZiBbICIkdGVzdF9tb2RlIiAhPSAxIF07IHRoZW4KIFsgIiRjcmVhdGVkX3N1YnVpZCIgIT0gMSBdIHx8IHNlZCAtaSAnL15vdXRwb3N0ZDovZCcgL2V0Yy9zdWJ1aWQKIFsgIiRjcmVhdGVkX3N1YmdpZCIgIT0gMSBdIHx8IHNlZCAtaSAnL15vdXRwb3N0ZDovZCcgL2V0Yy9zdWJnaWQKIFsgIiRjcmVhdGVkX291dHBvc3RkX3VzZXIiICE9IDEgXSB8fCB1c2VyZGVsIG91dHBvc3RkCiBbICIkY3JlYXRlZF9vdXRwb3N0dm1fdXNlciIgIT0gMSBdIHx8IHVzZXJkZWwgb3V0cG9zdHZtCiBbICIkY3JlYXRlZF9vdXRwb3N0ZF9ncm91cCIgIT0gMSBdIHx8IGdyb3VwZGVsIG91dHBvc3RkCiBbICIkY3JlYXRlZF9vdXRwb3N0dm1fZ3JvdXAiICE9IDEgXSB8fCBncm91cGRlbCBvdXRwb3N0dm0KIGlmIFsgIiRjaGFuZ2VkX2lwIiA9IDEgXSAmJiAhIGdyZXAgLVJxcyAnXltbOnNwYWNlOl1dKm5ldC5pcHY0LmlwX2ZvcndhcmRbWzpzcGFjZTpdXSo9W1s6c3BhY2U6XV0qMVtbOnNwYWNlOl1dKiQnIC9ldGMvc3lzY3RsLmNvbmYgL2V0Yy9zeXNjdGwuZCAyPi9kZXYvbnVsbDsgdGhlbiBzeXNjdGwgLXcgIm5ldC5pcHY0LmlwX2ZvcndhcmQ9JGlwX3ByZXZpb3VzIiA+L2Rldi9udWxsOyBmaQpmaQpzeXN0ZW1jdGwgZGFlbW9uLXJlbG9hZApsb2cgY29tcGxldGUK'
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
 server=$(config_value OUTPOST_SERVER "$file") || return 1
 token=$(config_value OUTPOST_TOKEN "$file") || return 1
 valid_server "$server" && valid_token "$token"
}
release_metadata() {
 local work=$1 api tag
 if [[ -n ${OUTPOST_RELEASE_URL:-} || -n ${OUTPOST_RELEASE_VERSION:-} || -n ${OUTPOST_ASSETS_MANIFEST_SHA256:-} || -n ${OUTPOST_INSTALL_SERVER_SHA256:-} ]]; then
  [[ -n ${OUTPOST_RELEASE_URL:-} && -n ${OUTPOST_RELEASE_VERSION:-} && -n ${OUTPOST_ASSETS_MANIFEST_SHA256:-} && -n ${OUTPOST_INSTALL_SERVER_SHA256:-} ]] || fail 'release automation values must be complete'
  release=$OUTPOST_RELEASE_URL
  version=$OUTPOST_RELEASE_VERSION
  manifest_sha=$OUTPOST_ASSETS_MANIFEST_SHA256
  installer_sha=$OUTPOST_INSTALL_SERVER_SHA256
 else
  api=https://github.com/nishantdania/outpost/releases/latest
  tag=$(curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error --output /dev/null --write-out '%{url_effective}' "$api") || fail 'unable to resolve latest release'
  [[ $tag =~ ^https://github\.com/nishantdania/outpost/releases/tag/(v[0-9]+\.[0-9]+\.[0-9]+)$ ]] || fail 'invalid latest release metadata'
  tag=${BASH_REMATCH[1]}
  version=${tag#v}
  release=https://github.com/nishantdania/outpost/releases/download/$tag
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
 local home bin lib work actual client_sha uninstaller_sha
 home=${HOME:?HOME is required}
 bin=${OUTPOST_INSTALL_BIN_DIR:-$home/.local/bin}
 lib=${OUTPOST_INSTALL_LIB_DIR:-$home/.local/lib/outpost}
 [[ $bin == /* && $lib == /* ]] || fail 'install paths must be absolute'
 work=$(mktemp -d "${TMPDIR:-/tmp}/outpost-client-install.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 client_sha=$(awk '$2 == "outpost" { print $1 }' "$work/checksums.txt")
 [[ $client_sha =~ ^[a-f0-9]{64}$ ]] || fail 'invalid client release checksum'
 curl --fail --location --proto '=https' --tlsv1.2 -o "$work/outpost" "$release/outpost"
 actual=$(sha256sum "$work/outpost" | awk '{print $1}')
 [[ $actual == "$client_sha" ]] || fail 'client checksum mismatch'
 install -d -m 0755 "$bin" "$lib"
 install -m 0755 "$work/outpost" "$lib/outpost-$version"
 cat > "$work/outpost-uninstall" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
fail() { printf '%s\n' "$1" >&2; exit 1; }
[[ ( $# -eq 1 && $1 == uninstall ) || ( $# -eq 2 && $1 == uninstall && $2 == --yes ) ]] || fail 'usage: outpost uninstall [--yes]'
[[ ${OUTPOST_UNINSTALL_CONFIG:-} == "$HOME/.config/outpost/server.env" ]] || fail 'refusing to remove an unexpected Outpost configuration path'
if [[ $# -eq 1 ]]; then
 [[ -r /dev/tty ]] || fail 'a terminal is required; use outpost uninstall --yes for noninteractive use'
 printf "Type uninstall to permanently remove Outpost and all VMs: " >/dev/tty
 IFS= read -r answer </dev/tty || fail 'unable to read confirmation'
 [[ $answer == uninstall ]] || fail 'uninstall cancelled'
fi
if [[ ${OUTPOST_UNINSTALL_LOCAL:-0} == 1 ]]; then
 sudo /usr/local/lib/outpost/outpost-uninstall-server
else
 ssh -tt -- "$OUTPOST_UNINSTALL_JUMP" 'sudo /usr/local/lib/outpost/outpost-uninstall-server'
fi
rm -f -- "$OUTPOST_UNINSTALL_CONFIG" "$OUTPOST_UNINSTALL_BIN" "$OUTPOST_UNINSTALL_SELF"
rm -f -- "$HOME/.config/outpost/known_hosts" "$HOME/.config/outpost/keys/id_ed25519" "$HOME/.config/outpost/keys/id_ed25519.pub"
for client in "$OUTPOST_UNINSTALL_LIB"/outpost-*; do
 [[ -f $client && ! -L $client && ${client##*/} =~ ^outpost-[0-9]+\.[0-9]+\.[0-9]+$ ]] && rm -f -- "$client"
done
rm -f -- "$OUTPOST_UNINSTALL_LIB/outpost-uninstall"
rmdir -- "$OUTPOST_UNINSTALL_LIB" "$HOME/.config/outpost/keys" "$HOME/.config/outpost" 2>/dev/null || :
EOF
 install -m 0700 "$work/outpost-uninstall" "$lib/outpost-uninstall"
 uninstaller_sha=$(sha256sum "$lib/outpost-uninstall" | awk '{print $1}')
 cat > "$work/outpost-wrapper" <<EOF
#!/usr/bin/env bash
set -euo pipefail
config=\${OUTPOST_CONFIG_FILE:-\$HOME/.config/outpost/server.env}
real=\${OUTPOST_REAL_BINARY:-$lib/outpost-$version}
uninstaller=$lib/outpost-uninstall
fail() { printf '%s\\n' "\$1" >&2; exit 1; }
if [[ ( \$# -eq 1 && \$1 == uninstall ) || ( \$# -eq 2 && \$1 == uninstall && \$2 == --yes ) ]]; then
 [[ -e \$config ]] || fail 'invalid Outpost configuration'
fi
if [[ -e \$config ]]; then
 [[ -f \$config && ! -L \$config ]] || fail 'invalid Outpost configuration'
 mode=\$(stat -c %a "\$config")
 [[ \$mode =~ ^[0-7]00\$ ]] || fail 'Outpost configuration must not be accessible by group or others'
 [[ \$(stat -c %u "\$config") == \$(id -u) ]] || fail 'Outpost configuration must be owned by the current user'
 server= token= jump= count=0
 while IFS= read -r line || [[ -n \$line ]]; do
  [[ \$line == *=* ]] || fail 'invalid Outpost configuration'
  key=\${line%%=*}; value=\${line#*=}; count=\$((count + 1))
  case \$key in
   OUTPOST_SERVER) [[ -z \$server ]] || fail 'invalid Outpost configuration'; server=\$value ;;
   OUTPOST_TOKEN) [[ -z \$token ]] || fail 'invalid Outpost configuration'; token=\$value ;;
   OUTPOST_SSH_PROXY_JUMP) [[ -z \$jump ]] || fail 'invalid Outpost configuration'; jump=\$value ;;
   *) fail 'invalid Outpost configuration' ;;
  esac
 done < "\$config"
 valid_ipv4() { local IFS=. a b c d extra octet; read -r a b c d extra <<< "\$1"; [[ -z \${extra:-} && -n \${a:-} && -n \${b:-} && -n \${c:-} && -n \${d:-} ]] || return 1; for octet in "\$a" "\$b" "\$c" "\$d"; do [[ \$octet =~ ^[0-9]{1,3}\$ ]] && ((10#\$octet <= 255)) || return 1; done; }
 valid_server() { local address host port; [[ \$1 =~ ^https?://[^/]+\$ ]] || return 1; address=\${1#*://}; host=\${address%:*}; port=\${address##*:}; [[ \$host != "\$address" && \$port =~ ^[1-9][0-9]{0,4}\$ ]] && ((10#\$port <= 65535)) && valid_ipv4 "\$host"; }
 local_config=0
 if [[ \$count -eq 2 && -z \$jump ]]; then local_config=1; elif [[ \$count -eq 3 && \$jump =~ ^[A-Za-z_][A-Za-z0-9_-]{0,31}@[A-Za-z0-9][A-Za-z0-9.-]{0,251}\$ && \$jump != *..* ]]; then :; else fail 'invalid Outpost configuration'; fi
 [[ \$token =~ ^[A-Za-z0-9+/=]{20,256}\$ ]] && valid_server "\$server" || fail 'invalid Outpost configuration'
 [[ -n \${OUTPOST_SERVER:-} ]] || export OUTPOST_SERVER=\$server
 [[ -n \${OUTPOST_TOKEN:-} ]] || export OUTPOST_TOKEN=\$token
 [[ \$local_config == 1 || -n \${OUTPOST_SSH_PROXY_JUMP:-} ]] || export OUTPOST_SSH_PROXY_JUMP=\$jump
 if [[ ( \$# -eq 1 && \$1 == uninstall ) || ( \$# -eq 2 && \$1 == uninstall && \$2 == --yes ) ]]; then
  [[ \$config == "\$HOME/.config/outpost/server.env" ]] || fail 'refusing to remove an unexpected Outpost configuration path'
  [[ -f \$uninstaller && ! -L \$uninstaller ]] || fail 'Outpost uninstaller is not installed'
  [[ \$(sha256sum "\$uninstaller" | awk '{print \$1}') == $uninstaller_sha ]] || fail 'Outpost uninstaller checksum mismatch'
  export OUTPOST_UNINSTALL_CONFIG=\$config OUTPOST_UNINSTALL_JUMP=\$jump OUTPOST_UNINSTALL_LOCAL=\$local_config OUTPOST_UNINSTALL_BIN=$bin/outpost OUTPOST_UNINSTALL_LIB=$lib OUTPOST_UNINSTALL_SELF=\$uninstaller
  exec "\$uninstaller" "\$@"
 fi
fi
[[ -f \$real && ! -L \$real ]] || fail 'Outpost client is not installed'
[[ \$(sha256sum "\$real" | awk '{print \$1}') == $client_sha ]] || fail 'Outpost client checksum mismatch'
exec "\$real" "\$@"
EOF
 install -m 0755 "$work/outpost-wrapper" "$bin/outpost"
 printf 'Installed Outpost %s\n' "$version"
)
install_server_packages() {
 local manager=${OUTPOST_INSTALL_PACKAGE_MANAGER:-} command missing=()
 if [[ -z $manager ]]; then
  if command -v apt-get >/dev/null 2>&1; then manager=apt; elif command -v pacman >/dev/null 2>&1; then manager=pacman; else fail 'apt-get or pacman is required'; fi
 fi
 case $manager in
  apt)
   sudo apt-get update
   sudo apt-get install -y curl zstd iproute2 nftables e2fsprogs openssh-client rsync podman uidmap fuse-overlayfs python3
   ;;
  pacman)
   for command in curl zstd ip nft mkfs.ext4 e2fsck resize2fs ssh ssh-keygen rsync podman newuidmap newgidmap fuse-overlayfs python3; do
    command -v "$command" >/dev/null 2>&1 || missing+=("$command")
   done
   if command -v python3 >/dev/null 2>&1 && ! python3 -c 'import hashlib, json' >/dev/null 2>&1; then missing+=(python3-unusable); fi
   ((${#missing[@]} == 0)) || fail "missing or unusable Arch dependencies: ${missing[*]}; install them during your normal system upgrade"
   ;;
  *) fail 'invalid package manager' ;;
 esac
}
validate_handoff() {
 local handoff=$1 account_home
 if [[ ${OUTPOST_INSTALL_TEST:-0} == 1 ]]; then
  [[ $handoff == "$HOME/.config/outpost/server.env" ]] || fail 'invalid client handoff path'
 else
  account_home=$(getent passwd "$(id -u)" | awk -F: 'NR == 1 && NF == 7 { print $6 }')
  [[ $account_home =~ ^(/home/[A-Za-z0-9_.-]+|/root)$ && $handoff == "$account_home/.config/outpost/server.env" ]] || fail 'invalid client handoff path'
 fi
}
load_uninstall_marker() {
 local marker
 if ! sudo test -e /etc/outpostd/uninstall.env; then return 1; fi
 [[ $(sudo stat -c '%F:%a:%u:%g' /etc/outpostd/uninstall.env) == 'regular file:600:0:0' ]] || fail 'invalid Outpost uninstall marker'
 marker=$(sudo cat /etc/outpostd/uninstall.env) || fail 'invalid Outpost uninstall marker'
 mapfile -t marker_lines <<< "$marker"
 [[ ${#marker_lines[@]} == 10 ]] || fail 'invalid Outpost uninstall marker'
 [[ ${marker_lines[0]} =~ ^OUTPOST_HANDOFF=(/home/[A-Za-z0-9_.-]+|/root)/\.config/outpost/server\.env$ && ${marker_lines[1]} =~ ^OUTPOST_CREATED_OUTPOSTD_USER=[01]$ && ${marker_lines[2]} =~ ^OUTPOST_CREATED_OUTPOSTVM_USER=[01]$ && ${marker_lines[3]} =~ ^OUTPOST_CREATED_OUTPOSTD_GROUP=[01]$ && ${marker_lines[4]} =~ ^OUTPOST_CREATED_OUTPOSTVM_GROUP=[01]$ && ${marker_lines[5]} =~ ^OUTPOST_CREATED_OUTPOSTD_SUBUID=[01]$ && ${marker_lines[6]} =~ ^OUTPOST_CREATED_OUTPOSTD_SUBGID=[01]$ && ${marker_lines[7]} =~ ^OUTPOST_IP_FORWARD_PREVIOUS=[01]$ && ${marker_lines[8]} =~ ^OUTPOST_CHANGED_IP_FORWARD=[01]$ && ${marker_lines[9]} =~ ^OUTPOST_CREATED_SYSCTL=[01]$ ]] || fail 'invalid Outpost uninstall marker'
 created_outpostd_user=${marker_lines[1]#*=}; created_outpostvm_user=${marker_lines[2]#*=}; created_outpostd_group=${marker_lines[3]#*=}; created_outpostvm_group=${marker_lines[4]#*=}; created_subuid=${marker_lines[5]#*=}; created_subgid=${marker_lines[6]#*=}; ip_previous=${marker_lines[7]#*=}; changed_ip=${marker_lines[8]#*=}; created_sysctl=${marker_lines[9]#*=}
 return 0
}
install_server_helper() {
 local handoff
 handoff=${OUTPOST_CLIENT_HANDOFF:-$HOME/.config/outpost/server.env}
 validate_handoff "$handoff"
 printf '%s' "$server_uninstaller_b64" | base64 -d | sudo install -o root -g root -m 0700 /dev/stdin /usr/local/lib/outpost/outpost-uninstall-server
 printf 'OUTPOST_HANDOFF=%s\nOUTPOST_CREATED_OUTPOSTD_USER=%s\nOUTPOST_CREATED_OUTPOSTVM_USER=%s\nOUTPOST_CREATED_OUTPOSTD_GROUP=%s\nOUTPOST_CREATED_OUTPOSTVM_GROUP=%s\nOUTPOST_CREATED_OUTPOSTD_SUBUID=%s\nOUTPOST_CREATED_OUTPOSTD_SUBGID=%s\nOUTPOST_IP_FORWARD_PREVIOUS=%s\nOUTPOST_CHANGED_IP_FORWARD=%s\nOUTPOST_CREATED_SYSCTL=%s\n' "$handoff" "$created_outpostd_user" "$created_outpostvm_user" "$created_outpostd_group" "$created_outpostvm_group" "$created_subuid" "$created_subgid" "$ip_previous" "$changed_ip" "$created_sysctl" | sudo tee /etc/outpostd/uninstall.env >/dev/null
 sudo chown root:root /etc/outpostd/uninstall.env
 sudo chmod 0600 /etc/outpostd/uninstall.env
}
server_install() (
 local work ip uplink actual handoff created_outpostd_user created_outpostvm_user created_outpostd_group created_outpostvm_group created_subuid created_subgid ip_previous changed_ip created_sysctl
 [[ $(uname -s) == Linux ]] || fail 'Linux is required'
 [[ $(uname -m) == x86_64 ]] || fail 'x86-64 is required'
 command -v sudo >/dev/null 2>&1 || fail 'sudo is required'
 command -v tailscale >/dev/null 2>&1 || fail 'Tailscale is required'
 handoff=${OUTPOST_CLIENT_HANDOFF:-$HOME/.config/outpost/server.env}
 validate_handoff "$handoff"
 load_uninstall_marker || true
 install_server_packages
 work=$(mktemp -d "${TMPDIR:-/tmp}/outpost-server-install.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 ip=$(tailscale ip -4 | awk 'NR == 1 { print }')
 valid_ipv4 "$ip" || fail 'a Tailscale IPv4 address is required'
 uplink=$(ip route show default | awk 'NR == 1 { print $5 }')
 [[ $uplink =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || fail 'a default network uplink is required'
 if [[ ${OUTPOST_INSTALL_TEST:-0} != 1 ]]; then
  [[ -r /dev/kvm && -c /dev/net/tun && -c /dev/userfaultfd && -f /sys/fs/cgroup/cgroup.controllers ]] || fail 'KVM, TUN, userfaultfd, and cgroup v2 are required'
 fi
 if [[ -z ${created_outpostd_user:-} ]]; then
  created_outpostd_user=0; created_outpostvm_user=0; created_outpostd_group=0; created_outpostvm_group=0; created_subuid=0; created_subgid=0
  id outpostd >/dev/null 2>&1 || created_outpostd_user=1
  id outpostvm >/dev/null 2>&1 || created_outpostvm_user=1
  getent group outpostd >/dev/null || created_outpostd_group=1
  getent group outpostvm >/dev/null || created_outpostvm_group=1
  grep -q '^outpostd:' /etc/subuid 2>/dev/null || created_subuid=1
  grep -q '^outpostd:' /etc/subgid 2>/dev/null || created_subgid=1
  ip_previous=$(sysctl -n net.ipv4.ip_forward); [[ $ip_previous =~ ^[01]$ ]] || fail 'invalid IPv4 forwarding state'
  changed_ip=0; [[ $ip_previous == 1 ]] || changed_ip=1; created_sysctl=1
 fi
 if [[ -e /etc/sysctl.d/99-outpost.conf || -L /etc/sysctl.d/99-outpost.conf ]]; then
  [[ -f /etc/sysctl.d/99-outpost.conf && ! -L /etc/sysctl.d/99-outpost.conf && $(cat /etc/sysctl.d/99-outpost.conf) == net.ipv4.ip_forward=1 ]] || fail 'Outpost sysctl path contains unexpected content'
 else
  printf '%s\n' 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-outpost.conf >/dev/null
 fi
 sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null
 actual=$(sha256sum "$work/install-server" | awk '{print $1}')
 [[ $actual == "$installer_sha" ]] || fail 'server installer checksum mismatch'
 sudo env OUTPOST_VERSION="$version" OUTPOST_UPLINK="$uplink" OUTPOST_LISTEN="$ip:17890" OUTPOST_SERVER="http://$ip:17890" OUTPOST_CLIENT_HANDOFF="$handoff" OUTPOST_RELEASE_URL="$release" OUTPOST_ASSETS_MANIFEST_SHA256="$manifest_sha" sh "$work/install-server"
 install_server_helper
)
main() {
 local mode=${OUTPOST_INSTALL_MODE:-} target=${OUTPOST_INSTALL_TARGET:-} work server token config_dir config_tmp
 case $mode in ''|client|server) ;; *) fail 'invalid OUTPOST_INSTALL_MODE' ;; esac
 if [[ $mode == server || $target == local ]]; then client_install; server_install; return; fi
 if [[ -z $target && $mode != client ]]; then
  [[ -r /dev/tty ]] || fail 'no terminal available; set OUTPOST_INSTALL_MODE=client, server, or OUTPOST_INSTALL_TARGET for automation'
  printf 'Outpost server (user@server, local, or blank for client only): ' >/dev/tty
  IFS= read -r target </dev/tty || fail 'unable to read server target'
 fi
 if [[ -z $target ]]; then client_install; return; fi
 ssh_target "$target" || { usage; fail 'invalid SSH target'; }
 work=$(mktemp -d "${TMPDIR:-/tmp}/outpost-release-metadata.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 rm -rf "$work"
 trap - EXIT HUP INT TERM
 client_install
 work=$(mktemp -d "${TMPDIR:-/tmp}/outpost-handoff.XXXXXX")
 handoff_work=$work
 trap 'rm -rf "$handoff_work"' EXIT HUP INT TERM
 local remote_script remote_command ssh_stdin
 printf -v remote_script 'set -o pipefail; curl --fail --location --proto %q --tlsv1.2 %q | OUTPOST_INSTALL_MODE=server OUTPOST_RELEASE_URL=%q OUTPOST_RELEASE_VERSION=%q OUTPOST_ASSETS_MANIFEST_SHA256=%q OUTPOST_INSTALL_SERVER_SHA256=%q bash' '=https' 'https://raw.githubusercontent.com/nishantdania/outpost/refs/heads/main/install.sh' "$release" "$version" "$manifest_sha" "$installer_sha"
 printf -v remote_command 'bash -c %q' "$remote_script"
 ssh_stdin=${OUTPOST_INSTALL_SSH_STDIN:-/dev/tty}
 [[ $ssh_stdin == /dev/tty || ${OUTPOST_INSTALL_TEST:-0} == 1 ]] || fail 'SSH input must be /dev/tty'
 ssh -tt "$target" "$remote_command" < "$ssh_stdin"
 scp "$target:~/.config/outpost/server.env" "$work/server.env"
 valid_config "$work/server.env" || fail 'server returned invalid configuration'
 server=$(config_value OUTPOST_SERVER "$work/server.env")
 token=$(config_value OUTPOST_TOKEN "$work/server.env")
 config_dir=${OUTPOST_CONFIG_DIR:-$HOME/.config/outpost}
 [[ $config_dir == /* ]] || fail 'OUTPOST_CONFIG_DIR must be absolute'
 mkdir -p "$config_dir"
 chmod 0700 "$config_dir"
 config_tmp=$(mktemp "$config_dir/.server.env.XXXXXX")
 printf 'OUTPOST_SERVER=%s\nOUTPOST_TOKEN=%s\nOUTPOST_SSH_PROXY_JUMP=%s\n' "$server" "$token" "$target" > "$config_tmp"
 chmod 0600 "$config_tmp"
 mv -f "$config_tmp" "$config_dir/server.env"
 printf 'Outpost is ready. Run outpost list.\n'
}
main
