package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func configureNetwork(ctx context.Context, temporary string) error {
	uid := os.Getuid()
	script := fmt.Sprintf(`#!/bin/sh
set -eu
ip link show outpost0 >/dev/null 2>&1 || ip link add outpost0 type bridge
ip addr replace 172.30.0.1/24 dev outpost0
ip link set outpost0 up
i=0
while [ "$i" -lt 16 ]; do
  tap="outpost-tap$i"
  ip link show "$tap" >/dev/null 2>&1 || ip tuntap add dev "$tap" mode tap user %d
  ip link set "$tap" master outpost0
  ip link set "$tap" up
  i=$((i+1))
done
sysctl -w net.ipv4.ip_forward=1 >/dev/null
iptables -C FORWARD -i outpost0 -j ACCEPT 2>/dev/null || iptables -A FORWARD -i outpost0 -j ACCEPT
iptables -C FORWARD -o outpost0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || iptables -A FORWARD -o outpost0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
iptables -t nat -C POSTROUTING -s 172.30.0.0/24 -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s 172.30.0.0/24 -j MASQUERADE
`, uid)
	stop := `#!/bin/sh
set -u
iptables -D FORWARD -i outpost0 -j ACCEPT 2>/dev/null || true
iptables -D FORWARD -o outpost0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || true
iptables -t nat -D POSTROUTING -s 172.30.0.0/24 -j MASQUERADE 2>/dev/null || true
i=0
while [ "$i" -lt 16 ]; do ip link del "outpost-tap$i" 2>/dev/null || true; i=$((i+1)); done
ip link del outpost0 2>/dev/null || true
`
	uninstall := `#!/bin/sh
set -u
systemctl disable --now outpost-network.service 2>/dev/null || true
rm -f /etc/systemd/system/outpost-network.service /usr/local/lib/outpost/network /usr/local/lib/outpost/network-stop /etc/sudoers.d/outpost
rm -f /usr/local/lib/outpost/uninstall-network
rmdir /usr/local/lib/outpost 2>/dev/null || true
systemctl daemon-reload
`
	sudoers := fmt.Sprintf("%s ALL=(root) NOPASSWD: /usr/local/lib/outpost/uninstall-network\n", os.Getenv("USER"))
	unit := `[Unit]
Description=Outpost VM networking
After=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/lib/outpost/network
ExecStop=/usr/local/lib/outpost/network-stop

[Install]
WantedBy=multi-user.target
`
	files := []struct {
		name, target, content string
		mode                  os.FileMode
	}{
		{"network", "/usr/local/lib/outpost/network", script, 0o755},
		{"network-stop", "/usr/local/lib/outpost/network-stop", stop, 0o755},
		{"outpost-network.service", "/etc/systemd/system/outpost-network.service", unit, 0o644},
		{"uninstall-network", "/usr/local/lib/outpost/uninstall-network", uninstall, 0o755},
		{"sudoers", "/etc/sudoers.d/outpost", sudoers, 0o440},
	}
	for _, file := range files {
		path := filepath.Join(temporary, file.name)
		if err := os.WriteFile(path, []byte(file.content), file.mode); err != nil {
			return err
		}
		if err := command(ctx, "sudo", "install", "-D", "-m", fmt.Sprintf("%o", file.mode), path, file.target); err != nil {
			return err
		}
	}
	if err := command(ctx, "sudo", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	return command(ctx, "sudo", "systemctl", "enable", "--now", "outpost-network.service")
}
