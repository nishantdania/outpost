# Ark

Ark runs persistent Firecracker VMs on a remote Linux server.

## Requirements

- An x86-64 Ubuntu or Debian server with KVM, TUN/TAP, cgroup v2, and `/dev/userfaultfd`
- Tailscale connectivity between your computer and server
- SSH and interactive sudo access to the server

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/nishantdania/ark/main/install.sh | bash
```

The installer asks for `user@server` to install both the client and remote server, `local` to install a server on this machine, or nothing for the client only. It installs `ark` in `~/.local/bin`; after a remote install, `ark list` works immediately.

## Usage

```bash
ark create dev
ark list
ark ssh dev
ark exec dev -- uname -a
ark copy ./file dev:/root/file
ark sync ./project/ dev:/root/project/
ark stop dev
ark start dev
ark delete dev
ark image list
ark image build -t coding:latest ./images/coding
ark create coding --image coding:latest --cpus 4 --memory 8G --disk 32G
```
