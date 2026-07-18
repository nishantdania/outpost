command -v firecracker >/dev/null && return

echo "Installing Firecracker..."
if ((EUID == 0)); then
    pacman -S --needed --noconfirm firecracker
else
    sudo pacman -S --needed --noconfirm firecracker
fi
