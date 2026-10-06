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

## Prepare once, test branches in independent VMs

A **snapshot** is a stopped VM's disk saved as a content-addressed image. A **fork**
is a new VM with its own writable copy of that disk, networking, machine ID, and
SSH host keys. This does not capture live memory, and forks do not share a writable
filesystem with the baseline or each other.

### 1. Prepare an app baseline

```bash
outpost create my-app-base --cpus 4 --memory 8G --disk 32G
# Git must be installed in the guest for checkout/fork --repo.
outpost exec my-app-base -- sh -c 'apt-get update && apt-get install -y git'
outpost checkout my-app-base --repo ./my-app --branch main
outpost ssh my-app-base
# Inside the VM: cd /workspace, install your runtimes/dependencies/local services,
# and verify that your app runs. Setup is project-specific and stays manual.
```

Stop app/database services cleanly inside the guest, then save the baseline:

```bash
outpost stop my-app-base
outpost snapshot create my-app-base --name my-app-ready
```

The current `stop` terminates Firecracker; it is **not a graceful guest shutdown**.
The snapshot operation copies only a stopped disk and runs filesystem recovery on
that copy, without modifying the baseline. Filesystem recovery is not a substitute
for clean database shutdown or application backups. No running VM is auto-stopped.

### 2. Fork and test a feature branch

```bash
# Fetch/commit locally first; branch resolution does not fetch from remotes.
outpost fork my-app-ready --name test-feature \
  --repo ./my-app --branch feat/my-feature --cpus 4 --memory 8G
outpost ssh test-feature
# Inside: cd /workspace, update dependencies/migrate as needed, start the app.

# In another local terminal; then open http://127.0.0.1:3000 in your browser:
outpost forward test-feature 3000:3000
# Ctrl-C closes the tunnel. The forwarded port binds host loopback only.

outpost delete test-feature
```

`fork` also accepts any existing image, including `default`. Omit `--repo` to clone
just the prepared environment. `--path /your/app` selects the guest checkout path
(default `/workspace`). CPU/memory use the normal creation defaults unless supplied;
these are not inherited from the baseline. Disk size defaults to the snapshot's
logical size, at least 8 GiB; `--disk` can enlarge it but cannot shrink it.

Checkout transfers an exact committed SHA using a Git bundle, not the host working
tree, `.git/config`, credentials, or SSH agent. Uncommitted/untracked host files are
excluded. Source history can contain committed secrets. Submodule repositories and
Git LFS objects are not automatically transferred. The guest needs Git; project
runtimes are neither detected nor installed automatically.

An existing guest Git checkout is updated without `reset --hard` or `git clean`,
so installed ignored dependencies and local data remain. Tracked modifications are
rejected; conflicting untracked files are not overwritten. Keep the baseline
checkout clean and use the same app/repository and guest path when forking. Branches
may still need different dependencies or database migrations. A fork with failed
checkout is retained for inspection; the error identifies the VM to delete.

**Snapshots include all baseline disk data**, including any guest secrets, database
contents, remote configuration, and enabled services. Use development-only local
services/data; do not clone production credentials or external-service identities.
Only the built-in root SSH authorized key is replaced; keys/configuration for other
users remain. Forks still use ordinary Outpost networking, not a new network-isolation
boundary. Enabled app/worker services can start as soon as the fork boots, before
branch checkout; disable these in the baseline if that would have side effects.
Snapshotting/copying/hashing large disks can take time; this is not an instant
copy-on-write or live-memory clone.

Snapshots reuse image listing, inspection, removal, and GC:

```bash
outpost image list
outpost image inspect my-app-ready
outpost image remove my-app-ready    # removes this tag, not a live fork
outpost image gc                     # removes untagged, unused images
```

Saving an existing snapshot tag replaces its target. Forks pin the image digest,
so subsequent tag updates do not change existing VMs. The baseline can be restarted
or deleted after saving; the snapshot remains independent. Deploy matching updated
versions of the CLI, outpostd, and VM launcher to use these commands.

## Agent skill

A skill for agents to use Outpost with tmux is available at [outpost-work](https://github.com/nishantdania/dotfiles/tree/fc641d49c64a38de0fdddc76ba24d86262693875/.pi/skills/outpost-work).
