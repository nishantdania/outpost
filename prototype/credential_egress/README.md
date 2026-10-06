# OpenAI credentialed network egress (prototype)

A host-side TLS interceptor replaces a VM's placeholder bearer credential with a
private host-held OpenAI key. Normal SDKs keep using `https://api.openai.com/v1`.
This directory contains only grant policy, interception, host firewall setup and
unit tests. It does **not** create/start VMs, install application runtimes, provide an
MCP development gateway or depend on its session database, or include application
smoke scripts. Use an existing running Outpost VM.

## Request and response path

```text
Guest SDK -> api.openai.com:443 (placeholder Authorization header)
          -> host per-TAP nftables redirect
          -> host TLS proxy (validate grant, replace header)
          -> api.openai.com:443 (real Authorization header, verified TLS)
          <- OpenAI response / SSE
          <- host proxy (bounded streaming, literal-key redaction)
          <- unchanged guest SDK
```

1. **Host routing:** `network.py` adds one grant-specific nftables table scoped to
   the assigned Outpost TAP. It redirects guest IPv4 TCP/443 to the proxy, drops
   other guest forwarding/input including IPv6, and permits DNS only to
   1.1.1.1/8.8.8.8. It never flushes unrelated firewall tables. Host-initiated SSH
   reply traffic remains permitted; incoming forwarded app previews are not enabled.
2. **Two TLS connections:** the proxy terminates guest TLS with a development CA,
   then connects to OpenAI with upstream certificate verification enabled. Only the
   CA's **public certificate** enters the guest; the signing key remains on the host.
   Installing trust is guest runtime configuration, not an HTTP-client patch.
3. **Policy and replacement:** exact HTTPS authority/SNI/host/port, operation,
   placeholder, source VM IP, live Outpost VM ID/IP/status, expiry and revocation
   are checked before a persistent request-budget reservation. Only then is the
   host key inserted. Guest auth/project/organization/cookie headers are cleared.
   No arbitrary forward destination or redirect following is supported.
4. **Streaming back:** the ordinary OpenAI response flows back to the ordinary SDK.
   Response size is bounded, literal credential echoes are redacted across chunk
   boundaries, sensitive response headers are removed, and authorization is cleared
   from completed/error flow objects. No UI, flow dumps or prompt/body logging.

## Setup (host administration)

Requires Linux nftables/iproute2, local Outpost CLI access, and Python 3.12+.
The proxy runs as an **unprivileged user**; firewall apply/remove needs host sudo.

```bash
cd prototype/credential_egress
python3 -m venv /private/path/egress-venv
/private/path/egress-venv/bin/pip install -r requirements.txt
```

Put a development-only OpenAI project key in a host-owned mode-0600 file under a
private directory. Do not paste it into agent tool arguments, logs, source control,
VM configuration or environment. Use provider-side spending limits as well.

```bash
python3 configure.py --vm EXISTING_VM \
  --key-file /private/path/openai-key --output /private/path/new-grant \
  --model gpt-4.1-nano --requests 8 --seconds 1800
/private/path/egress-venv/bin/python run_proxy.py --grant /private/path/new-grant/grant.json
```

The printed placeholder is intentionally nonsecret. The grant binds actual VM ID/IP
and name, allowed model, quota and expiry; it contains a key **path**, not its value.
The launcher creates a private CA directory and binds to the assigned VM's host-side
/30 gateway address (port 18443 by default), with a fixed OpenAI upstream.

Copy **only** `ca/mitmproxy-ca-cert.pem` to the guest's trust store. For Ubuntu:

```bash
outpost copy /private/path/new-grant/ca/mitmproxy-ca-cert.pem \
  EXISTING_VM:/usr/local/share/ca-certificates/outpost-openai-egress.crt
outpost exec EXISTING_VM -- update-ca-certificates
```

Never copy `mitmproxy-ca.pem`: it contains the private signing key. Configure the
application's ordinary API-key field with the placeholder, not the real key. Other
runtimes may require `SSL_CERT_FILE`, `NODE_EXTRA_CA_CERTS` or
`REQUESTS_CA_BUNDLE` pointing to guest public CA trust; pinned clients need separate
handling. No HTTP_PROXY/HTTPS_PROXY or base-URL override is required.

Inspect/apply the per-TAP host rules while the VM is running:

```bash
python3 network.py --grant /private/path/new-grant/grant.json print
sudo python3 network.py --grant /private/path/new-grant/grant.json apply
```

**Existing host firewall rules still apply.** An accept in one nftables base chain
cannot override a drop in another. If UFW/another firewall blocks the proxy, its
operator must add a permit scoped to the assigned TAP, guest source IP, gateway
IP and proxy TCP port. This prototype does not weaken that firewall automatically.
Configure guest DNS to an allowed public resolver if necessary.

## Limits and cleanup

- Exact GET `/v1/models`, POST `/v1/responses` and POST `/v1/chat/completions` only.
  POST needs an approved model and explicit output-token limit <=128; body <=256 KiB;
  response <=2 MiB. Tools, query variants, redirects and compression are rejected.
- SQLite reservations precede forwarding and survive proxy restart. Failed requests
  consume quota. Grant expiry is <=30 minutes; `REVOKED` rejects new injection and
  is rechecked during streaming. VM identity/status is inspected for each request,
  not each chunk. Already submitted operations cannot be undone; this is not a
  guarantee of upstream cancellation or a monetary budget.
- Guest root is untrusted; firewall rules must live on the host. The host proxy and
  Outpost administrator are trusted. Placeholder possession alone is not authority.
  VM IP provenance must be enforced by host networking; this is not cryptographic
  guest authentication. Credential hiding does not prevent permitted spending or
  disclosure of provider-returned data. DNS allowance is not a non-exfiltration proof.
- Literal redaction is defense in depth, not protection against encoded/partial
  credential echoes. No universal TLS compatibility or production-hardening claim.

```bash
touch /private/path/new-grant/REVOKED
sudo python3 network.py --grant /private/path/new-grant/grant.json remove
# Stop the proxy; remove the guest public CA and refresh guest trust.
outpost exec EXISTING_VM -- rm /usr/local/share/ca-certificates/outpost-openai-egress.crt
outpost exec EXISTING_VM -- update-ca-certificates --fresh
```

Remove the table before deleting/recreating the VM. Remove any separately added
firewall permit. Retain/delete the host key file according to the operator's needs;
this prototype never deletes it. Revocation does not stop/delete the VM.

## Verification

```bash
/private/path/egress-venv/bin/python -m unittest -v test_openai_egress.py
```

Unit tests cover policy, persisted quotas, source/destination/SNI/authority checks,
VM identity/liveness, private-file handling, header replacement, redirect rejection,
stream redaction/revocation, grant creation without VM lifecycle operations and
scoped firewall generation.

Earlier local experiments verified real OpenAI streaming, unchanged application
clients, CA-verification failure, wrong-placeholder rejection and revocation using
SSH/guest-NAT demonstration routing. That demo and its application smoke harness
are intentionally **not included here**. This extraction changes grants to bind
an existing VM directly rather than a development-session database; it has been
unit-tested, not live re-verified. Privileged nftables application, existing-firewall
interaction and bypass/private-network negative tests remain **unverified** because
host sudo was unavailable. Do not claim host-enforced isolation until those tests run.
