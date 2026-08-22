# Outpost

Outpost runs lightweight Firecracker VMs on a remote Linux server and manages them over Tailscale.

## Requirements

- A Linux server with KVM access
- Tailscale connectivity between client and server
- SSH and interactive sudo access to the server

## Install

On your computer:

```bash
curl -fsSL https://github.com/nishantdania/outpost/raw/main/install.sh | bash
```

On the server:

```bash
curl -fsSL https://github.com/nishantdania/outpost/raw/main/install.sh | bash -s -- --daemon
```

Binaries are installed in `~/.local/bin`.

## Configure

Create `~/.config/outpost/config.json` on your computer:

```json
{
  "daemon_url": "http://server.your-tailnet.ts.net:8080",
  "ssh_host": "user@server"
}
```

Create `~/.config/outpost/daemon.json` on the server:

```json
{
  "listen_addr": ":8080",
  "firecracker_version": "v1.10.1"
}
```

Enable the daemon:

```bash
systemctl --user enable --now outpostd.service
```

Prepare Firecracker, VM images, networking, and SSH access:

```bash
outpost setup
outpost doctor
```

## Usage

```bash
outpost create dev
outpost list
outpost ssh dev
outpost exec dev 'uname -a'
outpost stop <id>
outpost start <id>
outpost delete <id>
```

Maintenance commands:

```bash
outpost version
outpost update
outpost uninstall
```

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/...
```
