# Outpost

Outpost is a Go control plane for creating and managing Firecracker-backed compute on remote servers. `outpost` is the client CLI; `outpostd` runs on a Linux server reachable over Tailscale. The project is building the client-to-daemon control plane, persistent state, and eventual VM lifecycle management.

## Code guide

Keep boundaries simple. Use focused files inside domain packages; do not add generic `api`, `utils`, or `commands` packages. When adding files, state, services, or downloaded assets, update `outpost uninstall` so it cleanly removes them.

## Test

```bash
go test ./...
go vet ./...
go build ./cmd/...
```

For remote testing, ask the author which server to use. Install the latest CLI and daemon release, configure the client daemon URL and daemon listen address, then test:

```bash
outpost version
outpost create test
outpost list
outpost delete <id>
```

## Release

Releases are GitHub Actions builds triggered by pushed tags. For now, increment only the patch version (`v0.0.7` → `v0.0.8`).

```bash
scripts/release.sh v0.0.8
git tag -a v0.0.8 -m "v0.0.8"
git push origin v0.0.8
```

Wait for the `Release` GitHub Actions workflow to complete. It builds archives, checksums, and publishes the GitHub Release. Verify the installed release with `outpost version` and `outpost update`.
