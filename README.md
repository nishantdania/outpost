# Outpost

Outpost runs persistent Firecracker VMs on a remote Linux server.

## Requirements

- An x86-64 Arch, Ubuntu, or Debian server with KVM, TUN/TAP, cgroup v2, and `/dev/userfaultfd`
- Tailscale connectivity between your computer and server
- SSH and interactive sudo access to the server

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/nishantdania/outpost/refs/heads/main/install.sh | bash
```

The installer asks for `user@server` to install both the client and remote server, `local` to install a server on this machine, or nothing for the client only. It installs `outpost` in `~/.local/bin`; after a remote install, `outpost list` works immediately. On Arch, required packages must already be installed; Outpost never upgrades the system. Re-run the installer before using `outpost uninstall`; it confirms `uninstall` (or accepts `--yes`) and removes the configured server and local Outpost files.

## Usage

```bash
outpost create dev
outpost list
outpost ssh dev
outpost exec dev -- uname -a
outpost copy ./file dev:/root/file
outpost sync ./project/ dev:/root/project/
outpost stop dev
outpost start dev
outpost delete dev
outpost image list
outpost image build -t coding:latest ./images/coding
outpost create coding --image coding:latest --cpus 4 --memory 8G --disk 32G
outpost uninstall
```
