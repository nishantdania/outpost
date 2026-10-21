# Outpost

Outpost runs persistent Firecracker VMs on a Linux host. One launcher process
owns VM management, images and SQLite; the CLI talks directly to its Unix socket.

```text
outpost CLI → /run/outpost/vm-launcher.sock → launcher → Firecracker VMs
                                             ├─ SQLite
                                             └─ disk/image files
```

The local protocol uses HTTP framing over Unix sockets, **not a TCP connection**.
There is no API daemon, metadata worker, bearer token or built-in remote façade.

Start with [an SSH workstation](#start-an-ssh-workstation), the
[SSH image recipe](#example-build-an-ssh-workstation-image), or
[host diagnostics](#diagnose-the-host-and-launcher). See
[Domain boundaries](#domain-boundaries) for internal architecture and workflow ownership.

## Requirements

- An x86-64 Arch, Ubuntu, or Debian server with KVM, TUN/TAP, cgroup v2, and `/dev/userfaultfd`
- IPv4 forwarding enabled on the VM host
- Root/sudo for installation and launcher operation
- Local socket access for the CLI; SSH is optional external transport for remote administration

The guest kernel is supplied separately from disk images: Firecracker boots
`/usr/local/lib/outpost/launcher/vmlinux` and mounts the selected disk as root.
An imported disk's own kernel/bootloader is not used. Releases build the kernel
from pinned Linux source and a Firecracker configuration; see the
[kernel build and validation guide](kernel/README.md). Building is a release-time
workflow, not a VM-host installation requirement.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/nishantdania/outpost/refs/heads/main/install.sh | bash
```

The installer asks for `local`, `user@server`, or nothing for CLI-only installation.
The client is installed in `~/.local/bin`; a server installation also installs the
CLI in `/usr/local/bin` and enables only `outpost-vm-launcher.service`.
The default guest disk is distributed as a complete compressed filesystem image;
the installer downloads, verifies and decompresses it. Installing Outpost and
running the default image require **neither Docker nor Podman**. Container tools
are only needed if you choose an image-building recipe that uses them.

On Arch,
install dependencies during your normal system upgrade; the installer never runs
an Arch upgrade. Ubuntu/Debian dependencies are installed with apt.

Run `sudo outpost list` **on the VM host**. During interactive server setup,
the installer offers to grant the installing user privileged management access
without sudo. This is opt-in and adds the selected account to the `outpost` group
during the same installation. Start a new login session or run `newgrp outpost`
to activate membership in your shell.

For noninteractive setup, explicitly set `OUTPOST_ADMIN_USER` to an existing
account on the VM host (also supported by `scripts/install-server`):

```bash
OUTPOST_ADMIN_USER=alice bash install.sh local
```

Leave it unset or empty to keep sudo-only access; no user is automatically added.
The socket is `root:outpost`, mode `0660`, under a protected directory.
**Socket-group membership grants privileged VM-host management access.** It does
not grant direct access to the root-private database or disks.

The CLI and launcher always use `/run/outpost/vm-launcher.sock`; there is no
socket-path flag or environment override. There are no `--server`/`--token` flags
or remote connection profiles. For remote use, run the host's CLI through an
external tool:

```bash
ssh -t hostuser@server sudo outpost list
ssh -t hostuser@server sudo outpost doctor
```

There is no automatic migration from the retired daemon installation layout.
Deploy this launcher-only layout explicitly; do not expect an old API client,
old daemon-owned database path or runtime JSON manifests to be supported. Startup
refuses untracked VM disk/jail directories rather than silently adopting or deleting
them; preserve old state until you have deliberately retired or moved that deployment.

Installation validates and stages release assets before stopping an existing
launcher. Upgrading stops its VMs; start them again explicitly after the upgrade.
The installer checks the local API before reporting success and does not replace
the workstation CLI if server installation fails. Retired daemon installations
must be removed explicitly with their matching uninstaller first.

To uninstall a server, run `sudo /usr/local/lib/outpost/uninstall-server` on that
host. It confirms `uninstall` (or accepts `--yes`), stops/reconciles the launcher,
and removes **all VMs, images and metadata**. It does not create or verify a backup.
If startup or shutdown fails, it refuses to delete state. Cleanup can be retried
after a partial removal; absent accounts/groups are not errors. The `outpost` group and sandbox
`outpostvm` account/group are left for administrator review. Remove a CLI-only
installation with `rm ~/.local/bin/outpost`.

## Usage

The core command surface is deliberately small:

```bash
outpost image import ubuntu ./ubuntu.ext4
outpost run dev --image ubuntu --cpus 2 --memory 4G
outpost stop dev
outpost start dev
outpost list
outpost inspect dev
outpost stop dev
outpost snapshot dev --tag dev-base
outpost run another --image dev-base
outpost rm dev
outpost image rm dev-base
```

`run` creates and starts a VM; `start` boots an existing VM. `rm` removes a VM
and its writable disk, not its source image or saved snapshots. `image rm TAG`
removes an image tag, not VMs made from it; `image gc` reclaims untagged,
unreferenced image bytes. `create`, `delete`/`remove`, and `image remove`/`delete`
remain compatibility aliases.

### Start an SSH workstation

The normal interactive workflow is **prepare an SSH-ready disk → import → create
→ connect with OpenSSH**. The bundled `default` disk is not configured for login.
Follow the [SSH image recipe](#example-build-an-ssh-workstation-image) below to
produce `workstation.ext4` containing your public key. It uses standard external
image tools, not an Outpost image builder. If you build elsewhere, transfer the
finished disk to the VM host with your normal file-transfer tool.

On the **VM host**:

```bash
sudo outpost doctor
sudo outpost image import -t workstation:latest ./workstation.ext4
sudo outpost create dev --image workstation:latest --cpus 2 --memory 4G --disk 8G
sudo outpost inspect dev --output json
```

Take `guest_ip` from the inspection output. From the **machine holding your private
SSH key**, connect through the VM host:

```bash
guest_ip=172.30.0.2  # Replace with the actual guest_ip, not this example address.
ssh -i ~/.ssh/id_ed25519 -J hostuser@server dev@"$guest_ip"
```

Replace the identity-file path and `hostuser@server` with your own. The recipe
creates the guest login `dev`; the Outpost VM name and guest username are unrelated.
If your machine already routes to the guest subnet, omit `-J`. Verify host keys
using your normal SSH policy; do not disable host-key checking. The recipe logs
the guest's Ed25519 host-key fingerprint to its boot console for verification.

A running VM process does not mean SSH is ready: allow time for the guest to boot.
Once connected, install tools and work normally. This recipe puts only the public
login key in the image; keep the private key on your SSH client. Outpost does not
sanitize imported bytes, so do not put private client credentials into your disk.

### Manage the VM

On the VM host, use `sudo outpost` or a trusted account with socket-group access:

```bash
sudo outpost list
sudo outpost inspect dev --output json
# First stop guest applications cleanly through your external guest access.
sudo outpost stop dev
sudo outpost start dev
# Destructive: permanently removes this VM's writable disk.
sudo outpost delete dev
sudo outpost image list
```

See [snapshots](#save-a-configured-vm-as-an-image) to save a stopped workstation for
reuse. Use `outpost --help` and `outpost COMMAND --help` for command options.

### Defaults and data lifetime

`create` starts the VM immediately. Unless overridden, it uses the bundled
`default` disk, **2 vCPUs, 4 GiB RAM and an 8 GiB disk file**. The bundled disk is
not SSH-ready; for an interactive workstation, use an image containing an enabled
SSH server, a login user and your authorized public key. Outpost does not add them.

| Operation | Effect on your data |
| --- | --- |
| `stop` | Terminates Firecracker; keeps the VM disk and configuration. Shut down guest applications cleanly first. |
| `start` | Boots the same writable VM disk with its existing configuration. |
| `delete` | Removes that VM and its writable disk, jail and networking. Its source image and saved snapshots are not deleted. |
| `image remove TAG` | Removes the tag, not VMs created from it. |
| `image gc` | Removes eligible untagged images; VM references protect images still in use. |
| Server uninstall | Removes all Outpost VMs, image bytes and metadata on that host. |

Disk-file capacity is not filesystem capacity: selecting `--disk 32G` does not
resize the filesystem inside the guest.

## Guest access

Outpost does not provision SSH keys, host keys, users or guest credentials. There
are no `ssh`, `exec`, `copy` or `sync` helper commands or SSH-specific flags. Use
OpenSSH, scp, rsync, configuration management, or your guest's own access mechanism
outside Outpost. Install and configure any required guest services in your image
or through your external provisioning process.

For example, with an SSH-enabled guest you prepared yourself and a trusted
`hostuser` that has launcher-socket access (replace `guestuser` with `dev` for the
workstation recipe):

```bash
guest_ip=$(ssh hostuser@server outpost inspect dev --output json | jq -r .guest_ip)
ssh -J hostuser@server guestuser@"$guest_ip"
# Run a command or transfer files using your normal SSH configuration:
ssh -J hostuser@server guestuser@"$guest_ip" uname -a
scp -o ProxyJump=hostuser@server ./file guestuser@"$guest_ip":/path/to/file
```

Use your own keys, SSH config and host-key verification. `ProxyJump` is optional
when your machine already has a route to the guest network. Run `outpost inspect`
locally instead when the CLI is on the VM host; neither the management socket nor
an SSH session alone automatically routes your local machine to the guest network.

The built-in `default` disk is a minimal bootable base, not an automatically
accessible workstation: its recipe does not install SSH or configure login
credentials. Use a custom image for your application or preferred access setup.
Existing guest disks retain their existing credentials; removing these features
does not revoke access inside already-created VMs.

## Diagnose the host and launcher

Start with the normal CLI check on the VM host:

```bash
sudo outpost doctor
sudo outpost doctor --json
```

**Today this checks launcher connectivity by listing VMs; it is not a full host
preflight and does not prove that a guest boots or accepts SSH.** The fuller host
checks already exist on the installed launcher binary. With the default installed
paths, run them as root and load the uplink chosen by the installer:

```bash
sudo sh -c '
  . /etc/outpost/launcher.env
  exec /usr/local/lib/outpost/launcher/outpost-vm-launcher \
    --doctor --doctor-online --uplink="$OUTPOST_UPLINK"
'
```

For JSON output add `--doctor-json`. If the service is stopped, omit
`--doctor-online` to check installed prerequisites without requiring an active
service/socket. If you changed launcher paths or its socket group, pass those
same settings to this diagnostic invocation.

The host checks cover KVM/TUN/userfaultfd devices, cgroup v2, IPv4 forwarding,
`ip`/`nft` availability, the uplink, accounts/groups, storage-directory permissions,
state-to-jail hardlink support, installed asset checksums, and—with online checks
selected—the socket's ownership/permissions and systemd service state. They do not
install dependencies, change system settings, validate guest contents, check SSH
readiness, or directly validate the database schema. Startup/schema errors appear
in the service journal.

The installer checks host prerequisites too. On Arch, install `curl`, `zstd`,
`iproute2`, `nftables` and `python` through your normal full system upgrade, with the
usual systemd/coreutils/shadow base tools present. On Ubuntu/Debian the installer
installs its package dependencies. Enable IPv4 forwarding before installation;
Outpost does not silently change your host's forwarding policy. Provision KVM and
nested-virtualization support through your host/provider when applicable.

### When a check fails

```bash
sudo systemctl status outpost-vm-launcher.service --no-pager
sudo journalctl -u outpost-vm-launcher.service -b -n 100 --no-pager
```

| Symptom | Next step |
| --- | --- |
| Socket permission denied | Use sudo, or explicitly grant a trusted user `outpost` group membership and start a new login session. Do not make the socket world-writable or expose the database. |
| Socket missing/refused | Run the host checks and inspect the service journal; the launcher may have failed startup. |
| Unsupported database schema | Preserve the old deployment. There is no automatic conversion; do not delete metadata or change schema markers to bypass the check. See [One metadata store](#one-metadata-store). |
| KVM/device/forwarding/cgroup failure | Fix the host prerequisite identified by the diagnostic; a stored image cannot compensate for missing host virtualization support. |
| VM is running but SSH is unavailable | Check guest boot/network/sshd configuration and allow boot time. A process-status check is not guest readiness. |
| SSH reports `Permission denied (publickey)` | Check the guest username and that the matching public key was included in the image. The launcher cannot provision or repair credentials. |
| VM operation leaves status `failed` | Read `failure` in `inspect` and the logs. After diagnosing it, delete the failed VM to retry cleanup; deletion permanently removes its writable disk. Do not manually remove live VM resources. |

For guest boot output and the SSH recipe's host-key fingerprint, use the VM ID
from `inspect` and the configured state directory (default shown). This convenience
command uses external `jq`; alternatively copy `id` from the JSON output:

```bash
vm_id=$(sudo outpost inspect dev --output json | jq -r .id)
sudo tail -n 100 "/srv/outpost/state/vms/$vm_id/firecracker.log"
```

No built-in command logs into a guest or repairs its filesystem. Host diagnostics
and guest configuration are separate responsibilities.

## Public routing with Caddy and Cloudflared

Outpost manages VMs, not public hostnames or HTTP routing. This example exposes
an app in VM `dev` as `https://dev.example.com`, using a VM-name alias in Caddy:

```text
Browser → Cloudflare Tunnel → Caddy → dev.outpost.internal:3000
```

Run Caddy and Cloudflared **on the VM host** for direct access to the guest network.
Install them separately and use the host's Outpost CLI. To use a different
machine, first provide a route to the guest subnet; management access alone is
not sufficient. Replace `dev.example.com` with a hostname in a domain you
manage through Cloudflare.

### 1. Start the guest app and resolve its VM name

Use an image containing your app and its boot/access configuration, imported and
tagged as `my-app:latest` before following this example.

```bash
outpost create dev --image my-app:latest
outpost inspect dev --output json
```

Run your app inside `dev`, listening on **`0.0.0.0:3000`**, not just the guest's
loopback interface. Outpost reports the VM process as running before guest services
are necessarily ready, so wait for your app to start.

Take `guest_ip` from the inspection output. For example, if it is `172.30.0.2`, add
this entry to **`/etc/hosts` on the Caddy machine** with `sudoedit /etc/hosts`:

```text
172.30.0.2 dev.outpost.internal
```

Confirm name resolution and app connectivity before setting up the tunnel:

```bash
getent hosts dev.outpost.internal
curl --fail http://dev.outpost.internal:3000/
```

This alias is managed by you, not automatically created by Outpost. Existing DNS
software can provide the same name instead of a hosts-file entry.

### 2. Run Caddy locally

Save this as `Caddyfile`:

```caddyfile
{
    auto_https off
}

http://dev.example.com:17891 {
    bind 127.0.0.1
    reverse_proxy dev.outpost.internal:3000
}

http://:17891 {
    bind 127.0.0.1
    respond "Unknown host" 404
}
```

Caddy selects the upstream using the public Host header. The guest alias keeps VM
IPs out of the Caddyfile; the catch-all rejects unconfigured public hostnames.
Cloudflare handles public HTTPS, while this local listener uses loopback HTTP.

```bash
caddy validate --config ./Caddyfile --adapter caddyfile
caddy run --config ./Caddyfile --adapter caddyfile
```

Keep Caddy running. In another terminal, check the route before exposing it:

```bash
curl --fail -H 'Host: dev.example.com' http://127.0.0.1:17891/
```

### 3. Point a Cloudflare named tunnel at Caddy

Create a named tunnel and its public DNS record:

```bash
cloudflared tunnel login
cloudflared tunnel create outpost-apps
cloudflared tunnel route dns outpost-apps dev.example.com
```

Save this as `cloudflared.yml`, replacing `TUNNEL_UUID` and `credentials-file` with
the UUID and absolute credentials-file path printed by `tunnel create`:

```yaml
tunnel: TUNNEL_UUID
credentials-file: /absolute/path/to/TUNNEL_UUID.json

ingress:
  - hostname: dev.example.com
    service: http://127.0.0.1:17891
  - service: http_status:404
```

Do not override `httpHostHeader`: Caddy needs the original public Host header.
Start the tunnel while Caddy is running:

```bash
cloudflared tunnel --config ./cloudflared.yml ingress validate
cloudflared tunnel --config ./cloudflared.yml run outpost-apps
```

Visit `https://dev.example.com`. This publishes the app; use Cloudflare Access or
application authentication if it should be private. Stopping Caddy or Cloudflared
stops public routing, not the VM. For persistent operation, run both with your
normal service manager.

### Maintaining routes

For another VM or app, add its local DNS/hosts alias, an explicit Caddy site, and a
Cloudflared ingress rule before the catch-all. Validate and reload the Caddyfile
with `caddy reload --config ./Caddyfile --adapter caddyfile`; restart the tunnel
process after changing its ingress configuration.

Use `outpost list --output json` to inspect current VM names and addresses. Update
aliases when addresses change and remove aliases and routes when VMs are deleted,
so stale names cannot point to a reused address. There is no built-in alias-sync
service. The Outpost management socket is privileged and must never be exposed
through a public tunnel or an unauthenticated network bridge.

The former `host`, `hosts`, `unhost`, and `gateway` commands and hostname API are
removed. The current database baseline has no hostname registry; older databases
are rejected, not automatically converted.

## Import an image built by another tool

Build disks with Nix, mkosi, Packer, Docker/Podman plus filesystem tools, or whatever
suits your guest. Outpost stores the bytes and runs the VM; it does not build or
inspect the guest OS.

```bash
outpost image import -t workstation:latest ./rootfs.ext4
# Or stream from stdin:
outpost image import -t workstation:latest - < ./rootfs.ext4
outpost create workstation --image workstation:latest --disk 32G
```

Imports stream directly into content-addressed storage, preserving every logical
byte and zero-filled regions as sparse holes. There is no guest validation,
image-size cap, format detection, conversion, or sanitization. A successful import
means the bytes were stored, not that the image will boot. There is no `image build`,
`image validate`, `--format`, or Outpost-specific conversion tool.

### What makes a disk bootable here?

Accepting an import and booting an OS are different things. The current launch
configuration is one writable virtio root disk, an externally supplied x86-64 Linux
kernel, no supplied initramfs, and one virtio network interface. The disk's own
kernel, bootloader and container `CMD`/`ENTRYPOINT` are not used.

The concrete target for the recipe below is:

| Part | Required by this recipe |
| --- | --- |
| Disk layout | A raw, unpartitioned ext4 filesystem, mounted as the root disk `/dev/vda`; not a whole installer/partitioned boot image. |
| Userspace | x86-64 Linux executables and their libraries, including a working `/sbin/init`. The supplied kernel must support the filesystem and virtio devices without relying on a guest initramfs. |
| Boot | systemd from `systemd-sysv`; systemd is this recipe's choice, not an import-validation requirement. |
| Networking | The launcher supplies `ip=GUEST_IP::GATEWAY:255.255.255.252::eth0:off` on the kernel command line. The recipe enables systemd's network generator and networkd to consume it. Outpost does not run DHCP. |
| DNS | A resolver file supplied by the image producer; Outpost never writes it. |
| SSH access | An enabled SSH server, login user and authorized public key, supplied by the image producer. Host keys are generated inside the guest on first boot. |

An ISO, OCI archive, qcow2 or VDI is not directly usable as this root filesystem.
Converting a partitioned image to raw alone does not make its layout match this
configuration. Other guest setups may work if built for the same launch
configuration; Outpost does not inspect or certify them.

**Running means the Firecracker process is running**, not that the kernel mounted
root, init succeeded or SSH is ready. SSH, systemd and rsync are not core guest
requirements; they are choices for the environment you build.

All VM disks, including the bundled `default`, are copied unchanged, then extended
to the requested capacity if needed. The requested disk must fit the source;
Outpost will not truncate guest data. It never grows or repairs a filesystem,
writes guest DNS, injects credentials, or resets machine identity. Configure these
in your image or guest, including any first-boot filesystem growth and identity
initialization. Extending the disk file alone does not increase filesystem space.

### Example: build an SSH workstation image

This is an external image-producer recipe for an x86-64 Linux build machine with
Podman, Skopeo, umoci and e2fsprogs. Run it in a fresh build directory. Container
and filesystem tools are not Outpost runtime dependencies.

**Validation status:** this configuration has been built and booted on a real
Firecracker host with the pinned kernel. Guest networking, verified-key SSH,
scp/rsync, stop/start persistence, stopped-disk snapshots and independent clones
were exercised successfully. Image export/population used Skopeo, umoci and
e2fsprogs inside a rootless producer container rather than host sudo. This is a
validation of that setup, not a guarantee for every host or future package input.

#### 1. Supply your public key and guest configuration

```bash
umask 077
mkdir workstation-image
cd workstation-image
cp ~/.ssh/id_ed25519.pub authorized_keys
```

Use the public key corresponding to the private key you will use to log in. If the
key is on another machine, transfer only its public half to this directory. Never
copy a private key into the image or build context.

Save `sshd.conf`:

```text
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
AllowUsers dev
```

Save `guest-ssh-hostkeys.service`:

```ini
[Unit]
Description=Generate missing guest SSH host keys
Before=ssh.service

[Service]
Type=oneshot
ExecStart=/usr/bin/ssh-keygen -A
ExecStart=/usr/bin/ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
StandardOutput=journal+console

[Install]
RequiredBy=ssh.service
```

Save `Dockerfile`:

```dockerfile
FROM ubuntu:24.04
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      systemd-sysv udev openssh-server rsync sudo iproute2 ca-certificates e2fsprogs \
 && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --user-group --shell /bin/bash dev \
 && usermod --password '*' dev \
 && install -d -m 0700 -o dev -g dev /home/dev/.ssh
COPY authorized_keys /home/dev/.ssh/authorized_keys
COPY sshd.conf /etc/ssh/sshd_config.d/00-workstation.conf
COPY guest-ssh-hostkeys.service /etc/systemd/system/guest-ssh-hostkeys.service
RUN chown dev:dev /home/dev/.ssh/authorized_keys \
 && chmod 0600 /home/dev/.ssh/authorized_keys \
 && printf '%s\n' 'dev ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dev \
 && chmod 0440 /etc/sudoers.d/dev \
 && systemctl disable ssh.socket \
 && systemctl enable ssh.service guest-ssh-hostkeys.service \
      systemd-network-generator.service systemd-networkd.service \
 && rm -f /etc/ssh/ssh_host_* /var/lib/dbus/machine-id /var/lib/systemd/random-seed \
 && truncate -s 0 /etc/machine-id
```

The `dev` user has passwordless sudo **inside this guest**. Anyone holding an
authorized key gets guest-administrator access; use a different policy if that
is not appropriate. The image disables password login and contains no private
login or SSH host keys. These are producer choices, not launcher provisioning.
The Ubuntu tag is not pinned; pin image/package inputs yourself if you need
reproducible builds. This image is separate from the bundled minimal `default`.

#### 2. Export and populate a raw root filesystem

```bash
podman build -t workstation:latest .
podman save --format oci-archive --output workstation.oci.tar workstation:latest
skopeo copy oci-archive:workstation.oci.tar oci:workstation-layout:latest
sudo umoci unpack --image workstation-layout:latest workstation-bundle

# Replace container runtime files in the producer's unpacked root tree.
sudo rm -f workstation-bundle/rootfs/.dockerenv \
  workstation-bundle/rootfs/run/.containerenv \
  workstation-bundle/rootfs/etc/hostname \
  workstation-bundle/rootfs/etc/hosts \
  workstation-bundle/rootfs/etc/resolv.conf
printf '127.0.0.1 localhost\n::1 localhost\n' \
  | sudo tee workstation-bundle/rootfs/etc/hosts >/dev/null
# Choose a DNS resolver reachable from your guest network.
printf 'nameserver 1.1.1.1\n' \
  | sudo tee workstation-bundle/rootfs/etc/resolv.conf >/dev/null
sudo chmod 0644 workstation-bundle/rootfs/etc/hosts \
  workstation-bundle/rootfs/etc/resolv.conf

truncate -s 8G workstation.ext4
sudo mkfs.ext4 -F -d workstation-bundle/rootfs workstation.ext4
```

Using sudo for unpacking/population preserves guest numeric ownership, including
`dev`'s home and authorized-key permissions. The filesystem itself is 8 GiB; choose
a larger size here if you want more space without a later guest-side resize.
Do not rerun `mkfs.ext4` on a disk containing data you want to keep.

#### 3. Import, boot and connect

Transfer `workstation.ext4` to the VM host if needed, then follow
[Start an SSH workstation](#start-an-ssh-workstation). The login is `dev`, using
your existing private key; allow first-boot key generation and networking to finish.
The guest generates missing host keys itself and retains them on later boots.
Check its logged fingerprint through the trusted host's boot log when establishing
SSH trust. No private login key needs to be installed on the VM host for ProxyJump.

The guest includes `e2fsprogs` for optional filesystem administration. If you
create a VM with a larger disk, explicitly grow this recipe's ext4 filesystem
inside the guest:

```bash
sudo resize2fs /dev/vda
df -h /
```

The kernel built by the current release recipe enables
`CONFIG_BLK_DEV_WRITE_MOUNTED`; online growth from 8 to 12 GiB and then to 16 GiB
in a snapshot clone has been tested on a real host. The older prebuilt kernel
blocks this operation with `Device or resource busy`. Upgrade the host's Outpost
kernel and stop/start the guest before retrying, or prepare a larger filesystem
offline before import. See the [kernel guide](kernel/README.md) for the build and
security trade-off. Outpost never resizes guest filesystems automatically. The
launcher also does not set the guest hostname to the VM name.

For the separate pinned, release-time minimal-disk recipe, see
`images/default/build.sh`. This is a maintainer/development workflow, not an
installation step: normal installations use the already-built default disk.
To rebuild it with rootless Podman on a development machine:

```bash
OUTPOST_VERSION=0.0.0 OCI_ENGINE=podman \
  SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) images/default/build.sh
```

Docker is the builder's default engine; omit `OCI_ENGINE=podman` to use it.

**Imported images retain application data, credentials, and machine identity.**
Prepare them for reuse yourself; Outpost does not sanitize them. Snapshots of an
already-booted workstation retain its existing host keys as well as authorized
users; the example's first-boot unit does not rotate keys that already exist.

## Save a configured VM as an image

A snapshot saves a **stopped VM's disk** as a reusable Outpost image. It does not
capture live memory or running processes. Create a VM, configure it manually,
then save the environment for reuse:

```bash
# Skip creation if you are using dev from the quickstart.
outpost create dev --image workstation:latest --cpus 4 --memory 8G --disk 32G
outpost inspect dev --output json
# Use your externally configured guest access to install tools and configure it.
# Shut down app/database services cleanly before leaving the guest.
outpost stop dev
outpost snapshot dev --tag dev-ready

outpost create another-dev --image dev-ready --cpus 8 --memory 16G --disk 32G
```

The new VM gets an independent writable disk and host-side networking allocation.
Its CPU and memory are chosen at creation, not inherited from the source VM.
Specify a disk at least as large as the saved disk (creation defaults to 8 GiB).
A larger disk is supported, but filesystem growth inside the guest is your
responsibility. Snapshot-based VMs retain the source's guest identity and credentials.

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

**Snapshots contain the source disk's application data, secrets, and identity.**
VMs created from them retain SSH keys, authorized users, and application credentials.
Enabled services may start as soon as the new VM boots. Prepare identities for
reuse yourself and disable services with unwanted side effects before saving.

The current `stop` terminates Firecracker rather than gracefully shutting down
the guest. Snapshotting copies the stopped disk byte-for-byte, without filesystem
recovery or repair. Shut down guest applications cleanly first. Snapshots stay on
the same host and do not protect against host-storage loss. No running VM is
automatically stopped. Large disks can
take time to copy and hash; this is not an instant copy-on-write snapshot.
Deploy matching versions of the CLI and launcher for VM management and snapshots.

## Domain boundaries

Outpost consumes compatible disks. Image construction and format adaptation live
outside the core CLI/API. One `internal/manager` implements VM lifecycle,
image import/removal and disk snapshots, without workflow/facade wrappers.
All durable VM and image metadata lives in one
SQLite database; disk bytes remain ordinary files. Schema initialization and
future migrations are centralized.

| Package | Owns | Must not own |
| --- | --- | --- |
| `internal/artifact` | Content-addressed disk bytes: sparse publication, opening, verification, removal, abandoned-publication reconciliation | Image tags, SQLite, container conversion, VM state |
| `internal/catalog` | Digests, tags, image metadata, reference checks and garbage-collection eligibility | Disk I/O, Podman, VM execution |
| `internal/manager` | VM lifecycle, image pinning/import/removal/GC, stopped-disk snapshots, failure recording | HTTP framing, SQL implementation, privileged host execution |
| `internal/outpost` | Canonical VM records, launcher runtime records, shared SQLite setup/migrations | Resolving mutable image tags during persistence, privileged host operations |
| `internal/launcher` | Host execution, VM disks/jails/networking, local socket serving | A second metadata store or persisted VM specification |
| `internal/api` | Small shared JSON structs and socket location; no generated API or codegen tools | Runtime operations |
| `internal/client` / `internal/httpapi` | Thin client and explicit standard-library HTTP routes over a local Unix socket | TCP serving, bearer authentication, a separate daemon |

### One metadata store

The launcher owns `/var/lib/outpost/outpost.db` (or its configured `--database`
path). It is the only process opening SQLite. The CLI accesses metadata through
launcher operations, never by opening the file itself. SQLite is a local file,
not a separately installed database server.

- `outposts` holds VM names/IDs, the immutable image digest and resource configuration,
  guest IP, desired state, observed lifecycle status, failures and timestamps.
- `vm_runtime` references the VM by ID and holds only launcher bookkeeping: TAP,
  gateway, PID/start time and firewall cleanup information. It does not duplicate
  the VM specification or guest IP. Unique allocations and a foreign key prevent
  conflicting resources and removal of a VM record before runtime cleanup.
- `images` and `image_tags` hold image metadata and tags in that same database.
  A VM pins its image until its record is actually deleted, including when a
  failed deletion leaves privileged resources to clean up.

The launcher runs as root. Its database and image directories are root-private,
so it never opens a database controlled by an unprivileged daemon. VM operations,
image workflows, snapshot exports and database calls are ordinary in-process calls.
There are no internal RPC clients, subprocess workers, runtime JSON writes, or
legacy runtime-manifest readers. VM processes remain sandboxed as `outpostvm`.

The schema starts from `internal/outpost/migrations/0001_initial.sql`, containing
only the current VM, runtime and image tables. There are no historical SSH-column
or hostname-table creation/drop steps.

The baseline has its own SQLite `application_id` and a contiguous migration ledger.
Only an empty, unclaimed database is initialized. Existing databases from the
retired migration chain—including the earlier nine-migration refactor schema—are
**unsupported and rejected without conversion or reset**. Databases with an unknown
application identity, an incomplete ledger, or migrations newer than this launcher
are also rejected. Reopening a current database preserves its records.

For a new deployment, use a fresh database and clean, dedicated VM-state, image
and jailer locations. Deliberately retire the old manager first; only one launcher
may manage a host. Do not simply delete an old database: its VM resources and
image references may still be needed, and startup refuses untracked VM directories.
There is no bundled conversion tool. Preserve the old deployment until you have
deliberately retired or manually converted it.

Future schema changes append consecutively numbered migrations to this baseline;
do not edit/reuse applied migration versions or stamp an old database with the new
identity to bypass validation.

All VM disks, images and metadata are local to this host. **If the host's storage
is lost, that data is lost.** Outpost provides no backup, replication or disaster
recovery workflow. Local snapshots are reusable disk images, not off-host backups.

### Image creation

```text
External disk bytes ────────────────────┐
                                        ├→ manager.ImportImage → artifact.Publish
Stopped Outpost VM → snapshot exporter ─┘                      → catalog.PutImage
```

Publication streams into a temporary artifact, then registers its digest. There is
no separate import staging or guest-validation step. Published disk paths remain
`<image-store>/<sha256 hex>/rootfs.ext4`; this byte layout is unchanged, but existing
databases and their catalog records are not automatically converted.
The built-in `default` image remains a special, separately provisioned disk.

### VM lifecycle

```text
reference → catalog resolution → persist VM with immutable digest
          → manager.Create → manager.Start
```

The manager serializes image resolution plus VM reference insertion against removal
and GC. The lock is released before launcher operations; the persisted VM reference
then protects the image. VM persistence accepts only a digest or the built-in
`default` ID, never a mutable tag. The launcher reads the canonical specification
from SQLite and rejects mismatching create requests; it never persists a copy.
The launcher opens artifact bytes through `internal/artifact`, requiring regular
files owned by its process with safe permissions and no unsafe links. It also
verifies content-addressed disks against their recorded digest.

The catalog shares the VM repository's SQLite connection so reference eligibility
queries see the VM records. This is a persistence relationship, not permission for
the catalog to execute VMs. Migrations remain centralized in `internal/outpost`.

### Snapshots

```text
stopped VM → launcher.OpenSnapshot → exported disk stream
           → manager.ImportImage → artifact.Publish → catalog.PutImage
```

The manager validates the stopped state, opens a runtime export, publishes it
through the same path as imports, and closes the export even on failure.
It never inspects or repairs the guest filesystem: installed
applications, guest data, and sparse disk tails survive unchanged. The launcher remains responsible for
excluding concurrent runtime operations while creating its export.

### Consistency and recovery

Publication precedes registration. If registration fails, unregistered bytes are
eligible for startup reconciliation. Catalog removal precedes byte deletion;
unregistered bytes left by a deletion failure are likewise reconciled on restart.
Artifact reconciliation only touches abandoned publications and unregistered
digest directories.

Launcher startup reconciles database runtime records against verified processes,
cleans up dead-process networking/jails, and marks interrupted provisioning,
stopping or deletion operations failed instead of leaving them permanently stuck.
VM list/inspect refresh stable running/stopped states from the launcher before
reading SQLite, so a dead process is not reported running just because of an old
row. Reads of VM status therefore require the launcher to be available. Lifecycle
transitions never overwrite launcher-owned IP allocations, including on failure.
As with import, “running” means a verified Firecracker process, not guest-service
or SSH readiness.

Run one launcher per VM host; separate databases are not separate host-network
namespaces. The launcher acquires exclusive state/image directory locks before
opening SQLite or reconciling files, rejecting a competing manager for those stores.
The publication/pin lock itself is in-process. Startup reconciliation runs before
serving requests, with no concurrent importers, publishers, or removers.

Transport-facing `outpost.Image` and image error/validation names remain aliases or
forwarders to catalog definitions, keeping image metadata representations stable.
The disk-only CLI/API deliberately removes container-build and OCI-import behavior;
use matching CLI/launcher releases.

## Development checks

Production checks use Go 1.27.0:

```bash
go test ./...
go test -race ./...
go build ./...
go vet ./...
git diff --check
```

Regression tests cover lifecycle transitions and failures, image reference protection,
tag replacement, disk-snapshot byte preservation and cleanup, restart recovery,
request validation, and client/handler round trips over a real Unix socket using
a simulated runtime. These do not replace a real KVM boot/network test.
Release-time image construction still uses filesystem tools, but the installed VM
manager does not require them or modify guest filesystems.

## Agent skill

A skill for agents to use Outpost with tmux is available at [outpost-work](https://github.com/nishantdania/dotfiles/tree/fc641d49c64a38de0fdddc76ba24d86262693875/.pi/skills/outpost-work).
