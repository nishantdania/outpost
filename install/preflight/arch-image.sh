command -v pacstrap >/dev/null && return

gum log --level info "Installing Arch image build tools..."
if ((EUID == 0)); then
    pacman -S --needed --noconfirm arch-install-scripts e2fsprogs
else
    sudo pacman -S --needed --noconfirm arch-install-scripts e2fsprogs
fi
