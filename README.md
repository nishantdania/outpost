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

## Hostnames and one shared gateway

Register an HTTP hostname against a **VM name and port**, not its current IP:

```bash
outpost host main-dev.example.com dev:3000
outpost host memory-main-dev.example.com dev:3101
outpost hosts
outpost hosts --output json
outpost unhost main-dev.example.com
```

Mappings persist in outpostd's database and bind to the VM's immutable ID.
Registering an identical mapping is idempotent. A conflicting VM or port is
refused: explicitly `unhost` the existing mapping before replacing it. Names
are case-insensitive fully qualified ASCII DNS names; use punycode for IDNs.
Wildcard, URL, IP-address and hostname-with-port registrations are rejected.

A stopped/starting/failed VM's mapping remains registered but serves **503**.
When it starts, the current guest IP is resolved automatically for new requests.
Deleting the VM removes its mappings; creating another VM with the same name
never inherits them. Unknown or removed hostnames serve **404**. Existing
connections, including WebSockets, continue until the application/network closes
them; removing a mapping prevents new requests rather than forcibly killing
in-flight requests. Only registered HTTP upstream ports are reachable through
this gateway; it is not an arbitrary TCP proxy.

Start **one gateway process** in tmux on the machine where your tunnel connector
runs:

```bash
tmux new-session -d -s outpost-gateway 'outpost gateway'
# Optional different loopback port:
outpost gateway --listen 127.0.0.1:18080
```

It listens on **http://127.0.0.1:17891** by default. Bind addresses must be
loopback IPs. The gateway uses the installed client's saved server/token settings
(or normal `--server`/`OUTPOST_TOKEN` configuration) to resolve the authenticated
host registry. The management API/token is never exposed or forwarded to guests.
Deploy matching CLI and outpostd versions before using these commands; the server
adds the `hosts` table automatically when opening its database.

The gateway machine must be able to reach the VM guest IPs. This works directly
on the VM host; a desktop client needs routing to the guest subnet (for example,
Tailscale subnet routing). A Tailscale connection to the control server alone
is not necessarily a route to its VM network. The gateway does not create that
network route or use SSH forwarding implicitly.

### Bring your own tunnel provider

Point Cloudflare Tunnel, ngrok, or another trusted **local** reverse proxy at
`http://127.0.0.1:17891`, preserving the original public **Host** header. TLS,
DNS, wildcard-domain setup and provider credentials stay outside Outpost. Do not
point a public tunnel at the management API on port 17890.

For a Cloudflare named tunnel, an ingress rule can send all first-level
hostnames for a domain to the gateway:

```yaml
ingress:
  - hostname: "*.example.com"
    service: http://127.0.0.1:17891
  - service: http_status:404
```

Configure the corresponding wildcard DNS route using Cloudflare's tools. The
wildcard delivers traffic to the gateway; **only explicitly registered hosts
are forwarded to VMs**. Existing exact DNS records take precedence over wildcard
DNS, so migrate only records you own. Single-level hostnames fit the provider's
usual `*.example.com` certificate; nested names need appropriate TLS coverage.
For ngrok, use endpoint/domain settings that deliver the registered public host
names to the same gateway; custom/wildcard domains depend on your provider plan.

The gateway uses Go's standard reverse proxy, preserves request paths, query
strings and public Host headers, streams HTTP uploads/SSE, and supports protocol
upgrades such as WebSockets. Forwarded HTTPS/client-IP metadata is accepted only
from a loopback connector; incoming `X-Forwarded-Host` cannot select a route.
The control server is consulted for each new request, so route changes require
neither a gateway restart nor generated configuration reloads. If the registry
is unavailable, requests fail closed with 503; unreachable guests return 502.
Stopping the gateway affects all its routes, not the VMs themselves.

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

## Host-held credentials inherited from snapshots

Save reusable credential bindings alongside a snapshot without copying real keys
into its disk:

```bash
outpost snapshot create dev --name dev-ready --credentials profile.json
outpost create feature --image dev-ready --disk 32G
```

The server holds the real keys. Forks inherit stable references and the public CA;
a VM-scoped host HTTPS proxy substitutes authorized Bearer, custom-header, or
Basic credentials at the networking boundary. The feature is opt-in and requires
server configuration. See [snapshot credential egress](docs/credential-egress.md)
for setup, profile format, guest environment, limits, and verification.

## Agent skill

A skill for agents to use Outpost with tmux is available at [outpost-work](https://github.com/nishantdania/dotfiles/tree/fc641d49c64a38de0fdddc76ba24d86262693875/.pi/skills/outpost-work).
