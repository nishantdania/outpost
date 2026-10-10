# Snapshot-inherited host credentials

A snapshot can carry a **credential profile**. Every VM created from it pins a
copy of that profile. The guest receives credential references, not the real
keys. Outpost's host-side HTTPS proxy substitutes a reference only in an
explicitly authorized destination/header/operation.

```text
Snapshot disk + host-side profile
                |
                +--> fork A: same references, independent disk, A's TAP authorization
                +--> fork B: same references, independent disk, B's TAP authorization

Guest HTTPS :443 -> host nftables DNAT -> outpostd TLS proxy
              -> authorized header substitution -> verified provider HTTPS
```

This is provider-independent: Bearer, raw custom headers and Basic auth are
configuration, not provider adapters. The proxy does not inspect model names,
tool schemas or provider payload semantics. Ordinary provider URLs are retained.

## Enable on the server

Upgrade **both outpostd and the VM launcher**. This feature is opt-in; ordinary
VMs keep their existing networking behavior. No local container build is needed.

Provision two private directories owned by the daemon user:

```sh
sudo install -d -m 700 -o outpostd -g outpostd \
  /var/lib/outpostd/credentials /var/lib/outpostd/egress
# Copy a private key file, not a command-line literal or guest .env:
sudo install -m 600 -o outpostd -g outpostd \
  /path/to/private/development-key /var/lib/outpostd/credentials/llm-development
```

Files must be regular, single-link, daemon-owned, mode 600 or stricter. Symlinks,
path traversal, public files, oversized values and header-injection characters
are rejected. Key values are bounded printable ASCII (4 KiB); one terminal
newline is stripped. The filename is a logical secret name, not an API key.
The daemon and server administrator are trusted. Snapshot/profile management uses
Outpost's existing administrator bearer-token API; never give that token to the
guest. Administrators must bind credentials only to trusted destinations.

Add these non-secret settings through a systemd drop-in:

```ini
[Service]
Environment=OUTPOST_CREDENTIAL_STORE=/var/lib/outpostd/credentials
Environment=OUTPOST_EGRESS_STATE=/var/lib/outpostd/egress
```

Then reload/restart the daemon. Equivalent daemon flags are `--credential-store`
and `--egress-state`. On first startup, Outpost generates a persistent CA in the
private egress state directory. Its signing key stays on the host. Back up that
state privately; deleting/replacing it does not automatically repair existing
VM trust. An invalid existing CA fails startup rather than silently rotating.

The TLS listener uses IPv4 port **18443**. It checks the live VM's database state,
source IP, and local gateway address on TLS handshake and on each HTTP request.
Managed per-TAP input rules prevent another interface from impersonating that VM.
Do not expose or forward this listener as a general-purpose public proxy.

**Existing host firewall drops still apply.** With UFW or another firewall, permit
only the managed TAP's assigned source IP to its own gateway TCP/18443 and its
configured DNS resolver UDP/53 route. There is no automatic broad UFW route allow
for managed VMs. Keep host-initiated SSH/application ingress possible through
your existing rules. An accept in Outpost's nftables chain cannot override a drop
in another base chain.

## Put configuration on a snapshot

Create a JSON file containing **bindings only**, never actual key values:

```json
{
  "allowed_hosts": ["rubygems.org"],
  "credentials": [
    {
      "env": "OPENAI_API_KEY",
      "secret": "llm-development",
      "host": "api.openai.com",
      "header": "Authorization",
      "encoding": "bearer",
      "operations": [{"method": "POST", "path": "/v1/responses"}]
    }
  ]
}
```

The OpenAI hostname above is just example data. An Anthropic binding can use
`X-Api-Key` / `raw`; a Git HTTPS binding can use `Authorization` / `basic` plus
`username`. Raw auth can specify a literal `prefix` (for example `"token "` or
`"Bot "`) without adding a provider adapter. Header names are case-insensitive. Hosts are exact lowercase names,
not wildcards. Each env name must map to one named secret; it may be repeated
across hosts/encodings (for example one GitHub token for API Bearer and Git Basic
auth). Multiple keys can share a host/auth slot; the exact guest reference selects
one binding and its operation scope. Duplicate ambiguous bindings are rejected.

`operations` is optional: omitting it permits any non-tunnel HTTP operation at
that credential's host. With operations specified, method and escaped URL path
must match exactly; the query string is preserved. Prefer explicit operations
and least-privilege development credentials, particularly for infrastructure APIs.
Network permission is separate from key scopes: omitting `allowed_hosts` (or
setting it to `null`) permits public HTTPS, so ordinary package/CDN traffic does
not require maintaining a provider-specific network manifest. An explicit array
restricts additional credential-free HTTPS to those names; `[]` permits only
credential hosts. For restricted profiles, name all required download/CDN hosts.
Redirects do not implicitly expand an explicit allowlist. No additional host gains
credential access merely because public HTTPS is permitted.

```sh
outpost stop template
outpost snapshot create template --name prepared --credentials profile.json
outpost image inspect prepared --output json
# Choose a disk at least as large as the saved template:
outpost create feature-a --image prepared --disk 32G
outpost create feature-b --image prepared --disk 32G
```

Host secret availability is checked before publication/creation. Snapshot metadata
and image inspection expose the profile, never key values. The snapshot API accepts
an optional JSON profile body (maximum 64 KiB). Omitting `--credentials` when
snapshotting a managed VM inherits its pinned profile.

A disk digest's profile is immutable. Saving identical disk bytes with a different
profile returns a conflict without changing the tag; aliases must not silently
change authorization. Make a new prepared disk snapshot when changing bindings.
Existing forks keep their pinned profile even when an image tag moves. Removing
the source VM does not remove the snapshot or its profile.

**Do not copy real keys into the template first.** This feature cannot erase
credentials that were already in its filesystem, application database or logs.
Existing private snapshots/clones still contain any previously copied keys and
must be retired or repaired deliberately.

## Guest setup and application environment

At VM creation the launcher installs:

- `/etc/outpost-credentials.env`: public references under the configured env names;
- `/etc/profile.d/outpost-credentials.sh`: exports those references for login shells;
- `/usr/local/share/ca-certificates/outpost-egress.crt`: public CA certificate only;
- an SSH service drop-in that runs `update-ca-certificates` before readiness.

Credential egress is authorized only once the VM reaches `running`. Enabled
services making external calls during early boot must defer/retry until readiness.

The image must support systemd's `ssh.service` and `/usr/sbin/update-ca-certificates`
(as the default Ubuntu image does). Missing guest installation fails creation;
there is no unmediated fallback. Node login shells additionally receive
`NODE_EXTRA_CA_CERTS`. Runtimes with their own CA bundles may need explicit public
CA configuration. Certificate-pinned clients cannot transparently use this path.

Login shells load the profile automatically. A non-login application process
should source it explicitly or use the env file as a systemd `EnvironmentFile`:

```sh
outpost exec feature-a -- bash -lc 'curl -fsS \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  --data-binary @request.json https://api.openai.com/v1/responses'
```

For Rails/Bundler launched through `runuser` or tmux, source
`/etc/profile.d/outpost-credentials.sh` in the application's launch command before
loading non-secret dotenv configuration. An old `.env` can otherwise override the
reference; remove external real keys from it deliberately.

References remain stable across forks so references in persisted application data
keep working. Their secrecy is **not** the authorization boundary. A reference
from another VM does not unlock a profile that this VM lacks. Real values are
read from host files on each authorized request, so rotation does not require
rewriting snapshots, guest environments or persisted reference tokens. Stop/delete
and lifecycle epoch changes revoke further requests/stream output. Already sent
provider operations cannot be undone.

## Scope and limits

- Managed guests get mediated public HTTPS (optionally hostname-restricted),
  configured resolver UDP/53, and replies to
  host-initiated traffic. Other forwarded egress, guest-initiated host-service
  access, IPv6, alternate ports and QUIC are blocked. The resolver is restricted,
  but ordinary DNS queries can still be an exfiltration channel for guest data.
- Upstream resolution happens on the host; private/metadata/mixed-address answers
  are rejected and the checked IPv4 address is pinned. Upstream TLS verification
  stays enabled. IPv6-only upstreams are currently unsupported.
- HTTP/1.1 only initially; no CONNECT/raw tunnel, WebSocket, HTTP/2 or HTTP/3 path.
  The proxy does not follow redirects or retry provider operations.
- Bounds: 64 concurrent HTTP/handshake operations, 8 MiB request bodies, 64 MiB
  responses, two minutes per HTTP request, and finite header/connection timeouts.
  SSE frames stream with bounded buffering. Compressed credential-authenticated
  responses are rejected rather than bypassing reflection filtering; requests ask
  for identity. Ordinary credential-free HTTPS retains cookies, compression and
  caching headers, so public browsing/downloads remain ordinary HTTP.
- Literal secret echoes in headers/bodies, Basic values and supported base64 forms
  are redacted, including split chunks. Cookies from credential-authenticated
  responses are stripped. This is defense in
  depth, **not** a guarantee against an upstream deliberately transforming a key.
  Credential destinations and operation shapes must be trusted.
- Body/query credentials, OAuth refresh token lifecycle, SSH private keys, local
  HMAC/SigV4 signing, JWT/encryption keys and database passwords are not generic
  header substitution. Keep per-VM local secrets or introduce explicit protocol
  brokers separately. Do not use placeholder keys for local cryptography.
- This delegates API use; it is not a monetary budget or an exfiltration-proof VM.
  Use provider-side spend limits and development-scoped accounts.

## Verification

Run `go test ./...` and focused race tests. The test suite covers snapshot/API
inheritance, stable references and host-only key material, credential file safety,
real verified TLS for Bearer/raw/Basic, denied authority/auth/operation cases,
upstream/guest certificate failures, streaming literal redaction, public-IP checks,
and real ext4 guest installation.

`TestManagedNetworkNamespace` uses a disposable unprivileged Linux user/network
namespace and veth guest peer. It actually applies nftables and verifies transparent
TCP/443 routing, host/alternate-port/IPv6 blocks, spoofed-source and cross-interface
blocks, configured resolver UDP access/other UDP denial, and no upstream fallback
after proxy death. A live synthetic upstream in a separate namespace verifies
forwarded traffic, not just connections to local host services. It skips when isolated net-admin
is unavailable. It does not prove interaction with the server's existing UFW rules
or a real Firecracker VM. No real credentials are used in tests.

Before deploying broadly, verify an actual fork through its TAP with real SDKs and
existing host firewall, then the application's normal end-to-end smoke path.
