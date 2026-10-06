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

The installer asks for `user@server` to install both the client and remote server, `local` to install a server on this machine, or nothing for the client only. It installs `outpost` in `~/.local/bin`; after a remote install, `outpost list` works immediately. On Arch, required packages must already be installed; Outpost never upgrades the system. Re-run the installer before using `outpost uninstall`; it confirms `uninstall` (or accepts `--yes`) and removes the configured server and local Outpost files. If the server no longer exists or cannot be reached, `outpost uninstall --client-only` removes only the local client files.

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

## Save a configured VM as an image

A snapshot saves a **stopped VM's disk** as a reusable Outpost image. It does not
capture live memory or running processes. Create a VM, configure it manually,
then save the environment for reuse:

```bash
outpost create dev --cpus 4 --memory 8G --disk 32G
outpost ssh dev
# Install tools and configure the environment inside the VM.
# Shut down app/database services cleanly before leaving the guest.
outpost stop dev
outpost snapshot create dev --name dev-ready

outpost create another-dev --image dev-ready --cpus 8 --memory 16G --disk 32G
```

The new VM gets an independent writable disk, networking, machine identity, and
SSH host keys. Its CPU and memory are chosen at creation, not inherited from the
source VM. Specify a disk at least as large as the saved disk (creation defaults
to 8 GiB); a larger disk is supported, but shrinking is not.

The original VM can be restarted or deleted without changing the saved image.
Snapshots use the existing image catalog:

```bash
outpost image list
outpost image inspect dev-ready
outpost image remove dev-ready
outpost image gc
```

Saving to an existing tag replaces its target. Existing VMs pin their source
image digest, so moving a tag does not change them. Removing a tag does not
remove a VM; untagged images still used by VMs are protected from removal/GC.

**Snapshots contain the source disk's application data and secrets.** Only the
built-in root SSH authorized key is replaced when creating a new VM; other user
keys and application credentials remain. Enabled services may start as soon as
the new VM boots. Use development data and disable services with unwanted side
effects before saving or reusing an environment.

The current `stop` terminates Firecracker rather than gracefully shutting down
the guest. Snapshotting copies the stopped disk and runs filesystem recovery on
that copy, never on the original. This is not a substitute for clean database
shutdown or backups. No running VM is automatically stopped. Large disks can
take time to copy and hash; this is not an instant copy-on-write snapshot.
Deploy matching versions of the CLI, outpostd, and VM launcher to use snapshots.

## Agent skill

A skill for agents to use Outpost with tmux is available at [outpost-work](https://github.com/nishantdania/dotfiles/tree/fc641d49c64a38de0fdddc76ba24d86262693875/.pi/skills/outpost-work).
