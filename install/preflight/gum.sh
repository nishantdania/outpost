command -v gum >/dev/null && return

echo "Installing Gum..."
if ((EUID == 0)); then
    pacman -S --needed --noconfirm gum
else
    sudo pacman -S --needed --noconfirm gum
fi
