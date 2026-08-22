# Outpost

Outpost is a Go control plane for Firecracker VMs on remote Linux servers. `outpost` is the public CLI. `outpostd` is an implementation detail running on a server reachable over Tailscale or through local transport.

## Code guide

Keep package boundaries simple and domain-focused. Do not add generic `api`, `utils`, or `commands` packages. Prefer the standard library and avoid unnecessary dependencies and comments. When adding files, state, services, privileged helpers, networking resources, or downloaded assets, update `outpost uninstall` so it removes them cleanly.

## Development loop

Do not stop after editing or unit testing. Unless the user explicitly says otherwise, complete the full build, release, deployment, and end-to-end validation loop.

1. Format and validate:

   ```bash
   gofmt -w cmd internal
   go test ./...
   go vet ./...
   go build ./cmd/...
   ```

2. Commit the change and push `main`.

3. Increment only the patch version. Determine the next tag from the latest existing `v0.0.x` tag, then create and push an annotated tag:

   ```bash
   git tag -a v0.0.X -m v0.0.X
   git push origin v0.0.X
   ```

4. Wait for the `Release` GitHub Actions workflow and fail the task if the release fails:

   ```bash
   gh run list --workflow Release --limit 1
   gh run watch <run-id> --exit-status
   ```

5. Deploy the release:

   ```bash
   outpost update
   outpost version
   ```

   `outpost update` updates the current CLI and selected daemon. A CLI installed alongside the daemon is a separate installation; update or reinstall it when testing server-local CLI behavior.

6. Exercise the changed behavior against a real Firecracker VM. Test the successful path, relevant failure behavior, and persistence or cleanup where applicable. Do not claim completion based only on Go tests.

7. Delete temporary Outposts and other test resources. Do not delete user-owned Outposts.

## Test environment

The established test server is `nishant@fortytwo`. The normal client points at its Tailscale daemon endpoint. The server also has a colocated client configured with:

```json
{
  "daemon_url": "http://localhost:8080",
  "ssh_host": "local"
}
```

Use unique, clearly temporary names for test Outposts. A basic remote smoke test is:

```bash
outpost version
outpost create <temporary-name>
outpost list
outpost exec <temporary-name> 'echo ok'
outpost delete <temporary-name>
```

For lifecycle changes, also test stop, start, daemon restart reconciliation, SSH reachability, and cleanup as relevant. For transport changes, test both the normal remote client and the colocated server client. For setup or privileged networking changes, use the existing interactive tmux/SSH workflow and ask for a sudo password only when the prompt is actually waiting.

## Release policy

GitHub Actions builds archives and checksums and publishes the GitHub Release from pushed tags. Patch releases are the only release type for now. Every release must be followed by version verification and a real end-to-end test of the changed behavior.
