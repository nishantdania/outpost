#!/usr/bin/env bash
set -euo pipefail

handoff_work=
server_uninstaller_b64='IyEvYmluL3NoCnNldCAtZXUKWyAiJCMiIC1lcSAwIF0gfHwgZXhpdCAxCnJvb3Q9JHtBUktfVU5JTlNUQUxMX1JPT1Q6LX0KdGVzdF9tb2RlPSR7QVJLX1VOSU5TVEFMTF9URVNUOi0wfQpbIC16ICIkcm9vdCIgXSB8fCBbICIkdGVzdF9tb2RlIiA9IDEgXSB8fCBleGl0IDEKY2FzZSAiJHJvb3QiIGluICcnfC8qKSA6OzsgKikgZXhpdCAxOzsgZXNhYwpbICIkdGVzdF9tb2RlIiA9IDEgXSB8fCBbICIkKGlkIC11KSIgPSAwIF0gfHwgZXhpdCAxCmxvZygpIHsgWyAteiAiJHtBUktfVU5JTlNUQUxMX1RFU1RfTE9HOi19IiBdIHx8IHByaW50ZiAnJXNcbicgIiQxIiA+PiAiJEFSS19VTklOU1RBTExfVEVTVF9MT0ciOyB9CnJlbW92ZSgpIHsgaWYgWyAtZSAiJHJvb3QkMSIgXSB8fCBbIC1MICIkcm9vdCQxIiBdOyB0aGVuIHJtIC1yZiAtLSAiJHJvb3QkMSI7IGZpOyB9CmluYWN0aXZlKCkgeyBbICIkKHN5c3RlbWN0bCBzaG93IC1wIEFjdGl2ZVN0YXRlIC0tdmFsdWUgIiQxLnNlcnZpY2UiKSIgPSBpbmFjdGl2ZSBdOyB9CnN0b3AoKSB7IHN5c3RlbWN0bCBzdG9wICIkMS5zZXJ2aWNlIiB8fCBbICIkKHN5c3RlbWN0bCBzaG93IC1wIExvYWRTdGF0ZSAtLXZhbHVlICIkMS5zZXJ2aWNlIikiID0gbm90LWZvdW5kIF07IGluYWN0aXZlICIkMSI7IH0KcmVhZHkoKSB7CiBbICIkdGVzdF9tb2RlIiA9IDEgXSAmJiBbICIke0FSS19VTklOU1RBTExfVEVTVF9SRUFEWTotfSIgPSAxIF0gJiYgcmV0dXJuCiBweXRob24zIC0gIiRyb290L3J1bi9hcmsvdm0tbGF1bmNoZXIuc29jayIgIiR7QVJLX1VOSU5TVEFMTF9SRUFEWV9USU1FT1VUOi0zMH0iIDw8J1BZJwppbXBvcnQgc29ja2V0LHN5cyx0aW1lCnBhdGgsdGltZW91dD1zeXMuYXJndlsxXSxpbnQoc3lzLmFyZ3ZbMl0pOyBlbmQ9dGltZS5tb25vdG9uaWMoKSt0aW1lb3V0CndoaWxlIHRpbWUubW9ub3RvbmljKCk8ZW5kOgogdHJ5OgogIHM9c29ja2V0LnNvY2tldChzb2NrZXQuQUZfVU5JWCk7IHMuc2V0dGltZW91dCguMik7IHMuY29ubmVjdChwYXRoKTsgcy5jbG9zZSgpOyByYWlzZSBTeXN0ZW1FeGl0KDApCiBleGNlcHQgT1NFcnJvcjogdGltZS5zbGVlcCguMSkKcmFpc2UgU3lzdGVtRXhpdCgxKQpQWQp9CmhhbmRvZmY9Jyc7IGNyZWF0ZWRfYXJrZF91c2VyPScnOyBjcmVhdGVkX2Fya3ZtX3VzZXI9Jyc7IGNyZWF0ZWRfYXJrZF9ncm91cD0nJzsgY3JlYXRlZF9hcmt2bV9ncm91cD0nJzsgY3JlYXRlZF9zdWJ1aWQ9Jyc7IGNyZWF0ZWRfc3ViZ2lkPScnOyBpcF9wcmV2aW91cz0nJzsgY2hhbmdlZF9pcD0nJzsgY3JlYXRlZF9zeXNjdGw9JycKZmlsZT0kcm9vdC9ldGMvYXJrZC91bmluc3RhbGwuZW52CmlmIFsgLWYgIiRmaWxlIiBdICYmIFsgISAtTCAiJGZpbGUiIF07IHRoZW4KIFsgIiR0ZXN0X21vZGUiID0gMSBdIHx8IFsgIiQoc3RhdCAtYyAldTolYSAiJGZpbGUiKSIgPSAwOjYwMCBdIHx8IGV4aXQgMQogeyBJRlM9IHJlYWQgLXIgb25lOyBJRlM9IHJlYWQgLXIgdHdvOyBJRlM9IHJlYWQgLXIgdGhyZWU7IElGUz0gcmVhZCAtciBmb3VyOyBJRlM9IHJlYWQgLXIgZml2ZTsgSUZTPSByZWFkIC1yIHNpeDsgSUZTPSByZWFkIC1yIHNldmVuOyBJRlM9IHJlYWQgLXIgZWlnaHQ7IElGUz0gcmVhZCAtciBuaW5lOyBJRlM9IHJlYWQgLXIgdGVuOyBJRlM9IHJlYWQgLXIgZXh0cmEgfHwgOjsgfSA8ICIkZmlsZSIKIFsgLXogIiRleHRyYSIgXSB8fCBleGl0IDEKIHNldCAtLSAiJG9uZSIgIiR0d28iICIkdGhyZWUiICIkZm91ciIgIiRmaXZlIiAiJHNpeCIgIiRzZXZlbiIgIiRlaWdodCIgIiRuaW5lIiAiJHRlbiIKIGNhc2UgIiQxOiQyOiQzOiQ0OiQ1OiQ2OiQ3OiQ4OiQ5OiR7MTB9IiBpbgogIEFSS19IQU5ET0ZGPSo6QVJLX0NSRUFURURfQVJLRF9VU0VSPVswMV06QVJLX0NSRUFURURfQVJLVk1fVVNFUj1bMDFdOkFSS19DUkVBVEVEX0FSS0RfR1JPVVA9WzAxXTpBUktfQ1JFQVRFRF9BUktWTV9HUk9VUD1bMDFdOkFSS19DUkVBVEVEX0FSS0RfU1VCVUlEPVswMV06QVJLX0NSRUFURURfQVJLRF9TVUJHSUQ9WzAxXTpBUktfSVBfRk9SV0FSRF9QUkVWSU9VUz1bMDFdOkFSS19DSEFOR0VEX0lQX0ZPUldBUkQ9WzAxXTpBUktfQ1JFQVRFRF9TWVNDVEw9WzAxXSkgOjs7ICopIGV4aXQgMTs7CiBlc2FjCiBoYW5kb2ZmPSR7MSNBUktfSEFORE9GRj19CiBpZiBbICIkdGVzdF9tb2RlIiA9IDEgXTsgdGhlbgogIGNhc2UgIiRoYW5kb2ZmIiBpbiAvKi8uY29uZmlnL2Fyay9zZXJ2ZXIuZW52KSA6OzsgKikgZXhpdCAxOzsgZXNhYwogZWxzZQogIGNhc2UgIiRoYW5kb2ZmIiBpbgogICAvcm9vdC8uY29uZmlnL2Fyay9zZXJ2ZXIuZW52KSA6IDs7CiAgIC9ob21lLyovLmNvbmZpZy9hcmsvc2VydmVyLmVudikKICAgIHVzZXI9JHtoYW5kb2ZmIy9ob21lL307IHVzZXI9JHt1c2VyJSUvKn0KICAgIGNhc2UgIiR1c2VyIiBpbiAnJ3wqWyFBLVphLXowLTlfLi1dKikgZXhpdCAxOzsgZXNhYwogICAgWyAiJGhhbmRvZmYiID0gIi9ob21lLyR1c2VyLy5jb25maWcvYXJrL3NlcnZlci5lbnYiIF0gfHwgZXhpdCAxCiAgICA7OwogICAqKSBleGl0IDEgOzsKICBlc2FjCiBmaQogY3JlYXRlZF9hcmtkX3VzZXI9JHsyIyo9fTsgY3JlYXRlZF9hcmt2bV91c2VyPSR7MyMqPX07IGNyZWF0ZWRfYXJrZF9ncm91cD0kezQjKj19OyBjcmVhdGVkX2Fya3ZtX2dyb3VwPSR7NSMqPX07IGNyZWF0ZWRfc3VidWlkPSR7NiMqPX07IGNyZWF0ZWRfc3ViZ2lkPSR7NyMqPX07IGlwX3ByZXZpb3VzPSR7OCMqPX07IGNoYW5nZWRfaXA9JHs5Iyo9fTsgY3JlYXRlZF9zeXNjdGw9JHsxMCMqPX0KZmkKbGF1bmNoZXJfbG9hZD0kKHN5c3RlbWN0bCBzaG93IC1wIExvYWRTdGF0ZSAtLXZhbHVlIGFyay12bS1sYXVuY2hlci5zZXJ2aWNlKQpsb2cgc3RvcC1hcmtkOyBzdG9wIGFya2QgfHwgZXhpdCAxCmxvZyBzdG9wLWxhdW5jaGVyOyBzdG9wIGFyay12bS1sYXVuY2hlciB8fCBleGl0IDEKaWYgWyAiJGxhdW5jaGVyX2xvYWQiICE9IG5vdC1mb3VuZCBdOyB0aGVuCiBsb2cgcmVjb25jaWxlLWxhdW5jaGVyOyBzeXN0ZW1jdGwgc3RhcnQgYXJrLXZtLWxhdW5jaGVyLnNlcnZpY2UgJiYgc3lzdGVtY3RsIGlzLWFjdGl2ZSAtLXF1aWV0IGFyay12bS1sYXVuY2hlci5zZXJ2aWNlICYmIHJlYWR5IHx8IGV4aXQgMQogbG9nIHN0b3AtcmVjb25jaWxlZC1sYXVuY2hlcjsgc3RvcCBhcmstdm0tbGF1bmNoZXIgfHwgZXhpdCAxCmZpCnN5c3RlbWN0bCBkaXNhYmxlIGFya2Quc2VydmljZSB8fCBbICIkKHN5c3RlbWN0bCBzaG93IC1wIExvYWRTdGF0ZSAtLXZhbHVlIGFya2Quc2VydmljZSkiID0gbm90LWZvdW5kIF0gfHwgZXhpdCAxCnN5c3RlbWN0bCBkaXNhYmxlIGFyay12bS1sYXVuY2hlci5zZXJ2aWNlIHx8IFsgIiRsYXVuY2hlcl9sb2FkIiA9IG5vdC1mb3VuZCBdIHx8IGV4aXQgMQpbIC16ICIkaGFuZG9mZiIgXSB8fCByZW1vdmUgIiRoYW5kb2ZmIgpbICIkY3JlYXRlZF9zeXNjdGwiICE9IDEgXSB8fCByZW1vdmUgL2V0Yy9zeXNjdGwuZC85OS1hcmsuY29uZgpyZW1vdmUgL2V0Yy9zeXN0ZW1kL3N5c3RlbS9hcmtkLnNlcnZpY2UKcmVtb3ZlIC9ldGMvc3lzdGVtZC9zeXN0ZW0vYXJrLXZtLWxhdW5jaGVyLnNlcnZpY2UKcmVtb3ZlIC91c3IvbG9jYWwvYmluL2FyawpyZW1vdmUgL3Vzci9sb2NhbC9saWIvYXJrCnJlbW92ZSAvZXRjL2Fya2QKcmVtb3ZlIC92YXIvbGliL2Fya2QKcmVtb3ZlIC9zcnYvYXJrL3N0YXRlCnJlbW92ZSAvc3J2L2Fyay9qYWlsZXIKcm1kaXIgLS0gIiRyb290L3Nydi9hcmsiIDI+L2Rldi9udWxsIHx8IDoKcmVtb3ZlIC9ydW4vYXJrCmlmIFsgIiR0ZXN0X21vZGUiICE9IDEgXTsgdGhlbgogWyAiJGNyZWF0ZWRfc3VidWlkIiAhPSAxIF0gfHwgc2VkIC1pICcvXmFya2Q6L2QnIC9ldGMvc3VidWlkCiBbICIkY3JlYXRlZF9zdWJnaWQiICE9IDEgXSB8fCBzZWQgLWkgJy9eYXJrZDovZCcgL2V0Yy9zdWJnaWQKIFsgIiRjcmVhdGVkX2Fya2RfdXNlciIgIT0gMSBdIHx8IHVzZXJkZWwgYXJrZAogWyAiJGNyZWF0ZWRfYXJrdm1fdXNlciIgIT0gMSBdIHx8IHVzZXJkZWwgYXJrdm0KIFsgIiRjcmVhdGVkX2Fya2RfZ3JvdXAiICE9IDEgXSB8fCBncm91cGRlbCBhcmtkCiBbICIkY3JlYXRlZF9hcmt2bV9ncm91cCIgIT0gMSBdIHx8IGdyb3VwZGVsIGFya3ZtCiBpZiBbICIkY2hhbmdlZF9pcCIgPSAxIF0gJiYgISBncmVwIC1ScXMgJ15bWzpzcGFjZTpdXSpuZXQuaXB2NC5pcF9mb3J3YXJkW1s6c3BhY2U6XV0qPVtbOnNwYWNlOl1dKjFbWzpzcGFjZTpdXSokJyAvZXRjL3N5c2N0bC5jb25mIC9ldGMvc3lzY3RsLmQgMj4vZGV2L251bGw7IHRoZW4gc3lzY3RsIC13ICJuZXQuaXB2NC5pcF9mb3J3YXJkPSRpcF9wcmV2aW91cyIgPi9kZXYvbnVsbDsgZmkKZmkKc3lzdGVtY3RsIGRhZW1vbi1yZWxvYWQKbG9nIGNvbXBsZXRlCg=='
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
 local home bin lib work actual client_sha uninstaller_sha
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
 cat > "$work/ark-uninstall" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
fail() { printf '%s\n' "$1" >&2; exit 1; }
[[ ( $# -eq 1 && $1 == uninstall ) || ( $# -eq 2 && $1 == uninstall && $2 == --yes ) ]] || fail 'usage: ark uninstall [--yes]'
[[ ${ARK_UNINSTALL_CONFIG:-} == "$HOME/.config/ark/server.env" ]] || fail 'refusing to remove an unexpected Ark configuration path'
if [[ $# -eq 1 ]]; then
 [[ -r /dev/tty ]] || fail 'a terminal is required; use ark uninstall --yes for noninteractive use'
 printf "Type uninstall to permanently remove Ark and all VMs: " >/dev/tty
 IFS= read -r answer </dev/tty || fail 'unable to read confirmation'
 [[ $answer == uninstall ]] || fail 'uninstall cancelled'
fi
if [[ ${ARK_UNINSTALL_LOCAL:-0} == 1 ]]; then
 sudo /usr/local/lib/ark/ark-uninstall-server
else
 ssh -tt -- "$ARK_UNINSTALL_JUMP" 'sudo /usr/local/lib/ark/ark-uninstall-server'
fi
rm -f -- "$ARK_UNINSTALL_CONFIG" "$ARK_UNINSTALL_BIN" "$ARK_UNINSTALL_SELF"
rm -f -- "$HOME/.config/ark/known_hosts" "$HOME/.config/ark/keys/id_ed25519" "$HOME/.config/ark/keys/id_ed25519.pub"
for client in "$ARK_UNINSTALL_LIB"/ark-*; do
 [[ -f $client && ! -L $client && ${client##*/} =~ ^ark-[0-9]+\.[0-9]+\.[0-9]+$ ]] && rm -f -- "$client"
done
rm -f -- "$ARK_UNINSTALL_LIB/ark-uninstall"
rmdir -- "$ARK_UNINSTALL_LIB" "$HOME/.config/ark/keys" "$HOME/.config/ark" 2>/dev/null || :
EOF
 install -m 0700 "$work/ark-uninstall" "$lib/ark-uninstall"
 uninstaller_sha=$(sha256sum "$lib/ark-uninstall" | awk '{print $1}')
 cat > "$work/ark-wrapper" <<EOF
#!/usr/bin/env bash
set -euo pipefail
config=\${ARK_CONFIG_FILE:-\$HOME/.config/ark/server.env}
real=\${ARK_REAL_BINARY:-$lib/ark-$version}
uninstaller=$lib/ark-uninstall
fail() { printf '%s\\n' "\$1" >&2; exit 1; }
if [[ ( \$# -eq 1 && \$1 == uninstall ) || ( \$# -eq 2 && \$1 == uninstall && \$2 == --yes ) ]]; then
 [[ -e \$config ]] || fail 'invalid Ark configuration'
fi
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
 local_config=0
 if [[ \$count -eq 2 && -z \$jump ]]; then local_config=1; elif [[ \$count -eq 3 && \$jump =~ ^[A-Za-z_][A-Za-z0-9_-]{0,31}@[A-Za-z0-9][A-Za-z0-9.-]{0,251}\$ && \$jump != *..* ]]; then :; else fail 'invalid Ark configuration'; fi
 [[ \$token =~ ^[A-Za-z0-9+/=]{20,256}\$ ]] && valid_server "\$server" || fail 'invalid Ark configuration'
 [[ -n \${ARK_SERVER:-} ]] || export ARK_SERVER=\$server
 [[ -n \${ARK_TOKEN:-} ]] || export ARK_TOKEN=\$token
 [[ \$local_config == 1 || -n \${ARK_SSH_PROXY_JUMP:-} ]] || export ARK_SSH_PROXY_JUMP=\$jump
 if [[ ( \$# -eq 1 && \$1 == uninstall ) || ( \$# -eq 2 && \$1 == uninstall && \$2 == --yes ) ]]; then
  [[ \$config == "\$HOME/.config/ark/server.env" ]] || fail 'refusing to remove an unexpected Ark configuration path'
  [[ -f \$uninstaller && ! -L \$uninstaller ]] || fail 'Ark uninstaller is not installed'
  [[ \$(sha256sum "\$uninstaller" | awk '{print \$1}') == $uninstaller_sha ]] || fail 'Ark uninstaller checksum mismatch'
  export ARK_UNINSTALL_CONFIG=\$config ARK_UNINSTALL_JUMP=\$jump ARK_UNINSTALL_LOCAL=\$local_config ARK_UNINSTALL_BIN=$bin/ark ARK_UNINSTALL_LIB=$lib ARK_UNINSTALL_SELF=\$uninstaller
  exec "\$uninstaller" "\$@"
 fi
fi
[[ -f \$real && ! -L \$real ]] || fail 'Ark client is not installed'
[[ \$(sha256sum "\$real" | awk '{print \$1}') == $client_sha ]] || fail 'Ark client checksum mismatch'
exec "\$real" "\$@"
EOF
 install -m 0755 "$work/ark-wrapper" "$bin/ark"
 printf 'Installed Ark %s\n' "$version"
)
install_server_packages() {
 local manager=${ARK_INSTALL_PACKAGE_MANAGER:-} command missing=()
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
 if [[ ${ARK_INSTALL_TEST:-0} == 1 ]]; then
  [[ $handoff == "$HOME/.config/ark/server.env" ]] || fail 'invalid client handoff path'
 else
  account_home=$(getent passwd "$(id -u)" | awk -F: 'NR == 1 && NF == 7 { print $6 }')
  [[ $account_home =~ ^(/home/[A-Za-z0-9_.-]+|/root)$ && $handoff == "$account_home/.config/ark/server.env" ]] || fail 'invalid client handoff path'
 fi
}
load_uninstall_marker() {
 local marker
 if ! sudo test -e /etc/arkd/uninstall.env; then return 1; fi
 [[ $(sudo stat -c '%F:%a:%u:%g' /etc/arkd/uninstall.env) == 'regular file:600:0:0' ]] || fail 'invalid Ark uninstall marker'
 marker=$(sudo cat /etc/arkd/uninstall.env) || fail 'invalid Ark uninstall marker'
 mapfile -t marker_lines <<< "$marker"
 [[ ${#marker_lines[@]} == 10 ]] || fail 'invalid Ark uninstall marker'
 [[ ${marker_lines[0]} =~ ^ARK_HANDOFF=(/home/[A-Za-z0-9_.-]+|/root)/\.config/ark/server\.env$ && ${marker_lines[1]} =~ ^ARK_CREATED_ARKD_USER=[01]$ && ${marker_lines[2]} =~ ^ARK_CREATED_ARKVM_USER=[01]$ && ${marker_lines[3]} =~ ^ARK_CREATED_ARKD_GROUP=[01]$ && ${marker_lines[4]} =~ ^ARK_CREATED_ARKVM_GROUP=[01]$ && ${marker_lines[5]} =~ ^ARK_CREATED_ARKD_SUBUID=[01]$ && ${marker_lines[6]} =~ ^ARK_CREATED_ARKD_SUBGID=[01]$ && ${marker_lines[7]} =~ ^ARK_IP_FORWARD_PREVIOUS=[01]$ && ${marker_lines[8]} =~ ^ARK_CHANGED_IP_FORWARD=[01]$ && ${marker_lines[9]} =~ ^ARK_CREATED_SYSCTL=[01]$ ]] || fail 'invalid Ark uninstall marker'
 created_arkd_user=${marker_lines[1]#*=}; created_arkvm_user=${marker_lines[2]#*=}; created_arkd_group=${marker_lines[3]#*=}; created_arkvm_group=${marker_lines[4]#*=}; created_subuid=${marker_lines[5]#*=}; created_subgid=${marker_lines[6]#*=}; ip_previous=${marker_lines[7]#*=}; changed_ip=${marker_lines[8]#*=}; created_sysctl=${marker_lines[9]#*=}
 return 0
}
install_server_helper() {
 local handoff
 handoff=${ARK_CLIENT_HANDOFF:-$HOME/.config/ark/server.env}
 validate_handoff "$handoff"
 printf '%s' "$server_uninstaller_b64" | base64 -d | sudo install -o root -g root -m 0700 /dev/stdin /usr/local/lib/ark/ark-uninstall-server
 printf 'ARK_HANDOFF=%s\nARK_CREATED_ARKD_USER=%s\nARK_CREATED_ARKVM_USER=%s\nARK_CREATED_ARKD_GROUP=%s\nARK_CREATED_ARKVM_GROUP=%s\nARK_CREATED_ARKD_SUBUID=%s\nARK_CREATED_ARKD_SUBGID=%s\nARK_IP_FORWARD_PREVIOUS=%s\nARK_CHANGED_IP_FORWARD=%s\nARK_CREATED_SYSCTL=%s\n' "$handoff" "$created_arkd_user" "$created_arkvm_user" "$created_arkd_group" "$created_arkvm_group" "$created_subuid" "$created_subgid" "$ip_previous" "$changed_ip" "$created_sysctl" | sudo tee /etc/arkd/uninstall.env >/dev/null
 sudo chown root:root /etc/arkd/uninstall.env
 sudo chmod 0600 /etc/arkd/uninstall.env
}
server_install() (
 local work ip uplink actual handoff created_arkd_user created_arkvm_user created_arkd_group created_arkvm_group created_subuid created_subgid ip_previous changed_ip created_sysctl
 [[ $(uname -s) == Linux ]] || fail 'Linux is required'
 [[ $(uname -m) == x86_64 ]] || fail 'x86-64 is required'
 command -v sudo >/dev/null 2>&1 || fail 'sudo is required'
 command -v tailscale >/dev/null 2>&1 || fail 'Tailscale is required'
 handoff=${ARK_CLIENT_HANDOFF:-$HOME/.config/ark/server.env}
 validate_handoff "$handoff"
 load_uninstall_marker || true
 install_server_packages
 work=$(mktemp -d "${TMPDIR:-/tmp}/ark-server-install.XXXXXX")
 trap 'rm -rf "$work"' EXIT HUP INT TERM
 release_metadata "$work"
 ip=$(tailscale ip -4 | awk 'NR == 1 { print }')
 valid_ipv4 "$ip" || fail 'a Tailscale IPv4 address is required'
 uplink=$(ip route show default | awk 'NR == 1 { print $5 }')
 [[ $uplink =~ ^[A-Za-z0-9_.-]{1,15}$ ]] || fail 'a default network uplink is required'
 if [[ ${ARK_INSTALL_TEST:-0} != 1 ]]; then
  [[ -r /dev/kvm && -c /dev/net/tun && -c /dev/userfaultfd && -f /sys/fs/cgroup/cgroup.controllers ]] || fail 'KVM, TUN, userfaultfd, and cgroup v2 are required'
 fi
 if [[ -z ${created_arkd_user:-} ]]; then
  created_arkd_user=0; created_arkvm_user=0; created_arkd_group=0; created_arkvm_group=0; created_subuid=0; created_subgid=0
  id arkd >/dev/null 2>&1 || created_arkd_user=1
  id arkvm >/dev/null 2>&1 || created_arkvm_user=1
  getent group arkd >/dev/null || created_arkd_group=1
  getent group arkvm >/dev/null || created_arkvm_group=1
  grep -q '^arkd:' /etc/subuid 2>/dev/null || created_subuid=1
  grep -q '^arkd:' /etc/subgid 2>/dev/null || created_subgid=1
  ip_previous=$(sysctl -n net.ipv4.ip_forward); [[ $ip_previous =~ ^[01]$ ]] || fail 'invalid IPv4 forwarding state'
  changed_ip=0; [[ $ip_previous == 1 ]] || changed_ip=1; created_sysctl=1
 fi
 if [[ -e /etc/sysctl.d/99-ark.conf || -L /etc/sysctl.d/99-ark.conf ]]; then
  [[ -f /etc/sysctl.d/99-ark.conf && ! -L /etc/sysctl.d/99-ark.conf && $(cat /etc/sysctl.d/99-ark.conf) == net.ipv4.ip_forward=1 ]] || fail 'Ark sysctl path contains unexpected content'
 else
  printf '%s\n' 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-ark.conf >/dev/null
 fi
 sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null
 actual=$(sha256sum "$work/install-server" | awk '{print $1}')
 [[ $actual == "$installer_sha" ]] || fail 'server installer checksum mismatch'
 sudo env ARK_VERSION="$version" ARK_UPLINK="$uplink" ARK_LISTEN="$ip:17890" ARK_SERVER="http://$ip:17890" ARK_CLIENT_HANDOFF="$handoff" ARK_RELEASE_URL="$release" ARK_ASSETS_MANIFEST_SHA256="$manifest_sha" sh "$work/install-server"
 install_server_helper
)
main() {
 local mode=${ARK_INSTALL_MODE:-} target=${ARK_INSTALL_TARGET:-} work server token config_dir config_tmp
 case $mode in ''|client|server) ;; *) fail 'invalid ARK_INSTALL_MODE' ;; esac
 if [[ $mode == server || $target == local ]]; then client_install; server_install; return; fi
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
 printf -v remote_script 'set -o pipefail; curl --fail --location --proto %q --tlsv1.2 %q | ARK_INSTALL_MODE=server ARK_RELEASE_URL=%q ARK_RELEASE_VERSION=%q ARK_ASSETS_MANIFEST_SHA256=%q ARK_INSTALL_SERVER_SHA256=%q bash' '=https' 'https://raw.githubusercontent.com/nishantdania/ark/refs/heads/main/install.sh' "$release" "$version" "$manifest_sha" "$installer_sha"
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
