command -v gum >/dev/null && return

[ -f /etc/arch-release ] || {
    echo "Error: Outpost currently supports Arch Linux only." >&2
    exit 1
}

echo "Installing Gum..."
if ((EUID == 0)); then
    pacman -S --needed --noconfirm gum
else
    sudo pacman -S --needed --noconfirm gum
fi
