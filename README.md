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
  "default_host": "fortytwo",
  "hosts": {
    "fortytwo": {
      "daemon_url": "http://server.your-tailnet.ts.net:8080",
      "ssh_host": "user@server"
    },
    "local": {
      "daemon_url": "http://localhost:8080",
      "ssh_host": "local"
    }
  }
}
```

Create `~/.config/outpost/daemon.json` on the server:

```json
{
  "listen_addr": ":8080",
  "firecracker_version": "v1.10.1"
}
```

The default host is used automatically. Select another with `outpost --host local <command>` or `OUTPOST_HOST=local`. The `local` SSH transport bypasses the remote SSH hop when CLI and daemon share a machine.

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
outpost create build --cpus 4 --memory 8G --disk 32G
outpost list
outpost ssh dev
outpost exec dev 'uname -a'
printf 'input' | outpost exec -i dev 'cat'
outpost copy ./file dev:/root/file
outpost copy host:~/.pi/agent/auth.json dev:/root/.pi/agent/auth.json --mode 600
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
