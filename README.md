# Outpost

Outpost creates and manages Outposts through a daemon running on your network.

## Install

Install the CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/nishantdania/outpost/main/install.sh | bash
```

Install the daemon on a Linux server:

```bash
curl -fsSL https://raw.githubusercontent.com/nishantdania/outpost/main/install.sh | bash -s -- --daemon
```

Binaries are installed to `~/.local/bin` by default.

## Configure

On the machine running the CLI, create `~/.config/outpost/config.json`:

```json
{
  "daemon_url": "http://server.your-tailnet.ts.net:8080"
}
```

On the machine running the daemon, create `~/.config/outpost/daemon.json`:

```json
{
  "listen_addr": ":8080"
}
```

## Run

Start the daemon:

```bash
outpostd
```

Use the CLI:

```bash
outpost
outpost create
```

The initial `create` command returns `Hello, World!`.

## Release

Create release archives and checksums:

```bash
mise exec go -- scripts/release.sh v0.0.1
```

Publish the contents of `dist/v0.0.1` as a GitHub Release tagged `v0.0.1`.
