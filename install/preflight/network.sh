command -v dnsmasq >/dev/null && return

gum log --level info "Installing Firecracker network tools..."
if ((EUID == 0)); then
    pacman -S --needed --noconfirm dnsmasq nftables iproute2
else
    sudo pacman -S --needed --noconfirm dnsmasq nftables iproute2
fi
