# Release kernel

Outpost boots an external x86-64 Linux `vmlinux`; it does not boot the kernel or
bootloader inside an imported guest disk. The installer places it at
`/usr/local/lib/outpost/launcher/vmlinux`. All guests on that host use this kernel
on their next start. Already-running guests keep their current kernel.

## Build

From the repository root, with curl, Python 3, tar/gzip, coreutils and Docker or
rootless Podman available on the **development/release machine**:

```sh
OCI_ENGINE=podman OUTPOST_OUTPUT_DIR=dist/kernel scripts/build-kernel
# Or omit OCI_ENGINE to use Docker.
```

Compilers and build dependencies run in the container. VM hosts and end users do
not need a compiler, Docker or Podman: releases still ship a complete `vmlinux`.
`OUTPOST_BUILD_JOBS` optionally controls make parallelism.

Inputs are pinned in `deploy/toolchain.json`:

- Amazon Linux 6.18.41 source at commit
  `08639e3f54096d1f84cfea2731f55fa472d02945` (tag
  `kernel6.18-6.18.41-94.142.amzn2023`), with a pinned archive SHA-256. This is the
  vendor revision released before the baseline Firecracker CI build; using plain
  upstream Linux would discard Amazon-specific features such as SYSGENID.
- The original Firecracker CI kernel and SHA-256, used **only as the configuration
  baseline**, not as the final released binary.
- A fixed build epoch. The builder also fixes build user/host/version.
- A digest-pinned Ubuntu base and dated apt snapshot in `kernel/Dockerfile`.

The script verifies downloads, extracts the baseline's embedded configuration,
enables `CONFIG_BLK_DEV_WRITE_MOUNTED`, runs `olddefconfig`, and builds `vmlinux`.
It refuses unexpected feature changes: only the mounted-write option and inactive
Rust/pahole compiler-probe metadata may differ. Rust and debug information remain
disabled. `kernel-config.diff` records the full comparison for review.
Outputs are `vmlinux`, `vmlinux.config`, `baseline.config`, `kernel-config.diff` and
`kernel-checksums.txt`; downloads are cached under `kernel-input/`. The release
workflow builds this kernel before packaging it through `scripts/release-artifacts`.
The final binary's checksum is recorded in the normal release asset manifest.

The old kernel URL remains in the toolchain manifest as the verified **config
source**. It is deliberately not replaced with a made-up download URL for an
unpublished custom kernel.

## Why allow writes to mounted block devices?

The baseline disables `CONFIG_BLK_DEV_WRITE_MOUNTED`. With it disabled, the
`resize2fs` shipped by the documented Ubuntu guest cannot open the mounted root
block device for writing and fails with `Device or resource busy`. Allocating a
larger disk file alone does not increase filesystem capacity.

Enabling the option permits the block-device access needed by that online resize
workflow. It also removes a kernel safeguard against raw writes to mounted
filesystems: a privileged guest process can damage its own filesystem. This does
not grant new host-management access or expose another VM's disk. Do not treat it
as a reason to trust arbitrary guest-root tools.

Outpost still does **not** resize guest filesystems automatically, repair imported
bytes, or provision guest credentials. Administrators explicitly run filesystem
tools inside a compatible guest or prepare filesystem capacity before import.

## Validation before shipping

A build is not proof of guest compatibility. Test the candidate on a disposable
VM host using a complete local release bundle and the normal installer. Preserve
the previous kernel/bundle for rollback. Installation stops running guests; start
them explicitly after the update.

Required checks:

1. Inspect the embedded config and compare it with `baseline.config`.
2. Boot the minimal default image and an SSH-enabled ext4 image.
3. Verify network/SSH with strict host-key checking.
4. Create a VM with a disk larger than its source filesystem; run guest-side
   `sudo resize2fs /dev/vda` and verify both device and filesystem capacities.
5. Write a marker, stop/start, and verify the grown filesystem and marker survive.
6. Snapshot the stopped grown disk; boot a clone at least as large and verify its
   files, independent writes, networking and SSH.
7. Test host reinstall/recovery as needed, and clean up test resources.

The 2026-10-20 real-host pass verified default-image boot, strict-key SSH,
networking, online ext4 growth from 8 -> 12 GiB and then 12 -> 16 GiB in a
snapshot clone, stop/start persistence and independent clone writes. All test
VMs/images were cleaned up.

Fixed inputs and build identity make repeatable builds possible, but do not claim
bit-for-bit reproducibility until independently rebuilt outputs have been compared.
