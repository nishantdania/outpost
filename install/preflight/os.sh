[ -f /etc/arch-release ] || {
    echo "Error: Outpost currently supports Arch Linux only." >&2
    exit 1
}
