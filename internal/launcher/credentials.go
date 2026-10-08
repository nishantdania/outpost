package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/vmapi"
)

func (r *FirecrackerRuntime) installCredentials(ctx context.Context, paths vmPaths, spec vmapi.VMSpec) error {
	if spec.EgressCA == "" {
		return nil
	}
	for _, dir := range []string{"/usr/local/share/ca-certificates", "/etc/profile.d", "/etc/systemd/system/ssh.service.d"} {
		// debugfs mkdir reports an existing directory without changing it; subsequent
		// stat/write checks still validate each required file.
		if _, err := r.run(ctx, "debugfs", "-w", "-R", "mkdir "+dir, paths.stateDisk); err != nil {
			return err
		}
	}
	shell := "#!/bin/sh\nset -a\n. /etc/outpost-credentials.env\nset +a\nexport NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/outpost-egress.crt\n"
	files := map[string]string{
		"/etc/outpost-credentials.env":                        spec.CredentialEnv,
		"/etc/profile.d/outpost-credentials.sh":               shell,
		"/usr/local/share/ca-certificates/outpost-egress.crt": spec.EgressCA,
		// Trust is ready before SSH startup, which is the launcher readiness gate.
		"/etc/systemd/system/ssh.service.d/outpost-egress.conf": "[Service]\nExecStartPre=/usr/sbin/update-ca-certificates\n",
	}
	for guest, content := range files {
		file := filepath.Join(paths.stateDir, "egress-"+filepath.Base(guest))
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			return err
		}
		if err := r.removeGuestIdentity(ctx, paths.stateDisk, guest); err != nil {
			os.Remove(file)
			return err
		}
		_, err := r.run(ctx, "debugfs", "-w", "-R", "write "+file+" "+guest, paths.stateDisk)
		os.Remove(file)
		if err != nil {
			return err
		}
		if err := r.inode(ctx, paths.stateDisk, guest, "0100644"); err != nil {
			return err
		}
		info, err := r.guestStat(ctx, paths.stateDisk, guest)
		if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(content)) {
			return fmt.Errorf("credential guest material installation failed")
		}
	}
	return nil
}

// One atomic nftables transaction. No permissive NAT fallback for managed VMs.
func managedRules(m manifest, dns string) string {
	// Allocation and DNS values are validated before interpolation. Named slots
	// keep the confinement rules auditable without a positional argument list.
	return strings.NewReplacer(
		"{{table}}", networkTable(m), "{{tap}}", m.Tap,
		"{{guest}}", m.GuestIP, "{{gateway}}", m.Gateway,
		"{{port}}", strconv.Itoa(credentials.Port), "{{dns}}", dns,
	).Replace(`table inet {{table}} {
 chain prerouting {
  type nat hook prerouting priority dstnat; policy accept;
  iifname "{{tap}}" ip saddr {{guest}} tcp dport 443 dnat ip to {{gateway}}:{{port}}
 }
 chain input {
  type filter hook input priority -10; policy accept;
  ip daddr {{gateway}} tcp dport {{port}} iifname != "{{tap}}" drop
  iifname "{{tap}}" meta nfproto ipv6 drop
  iifname "{{tap}}" ip saddr != {{guest}} drop
  iifname "{{tap}}" ip daddr {{gateway}} tcp dport {{port}} accept
  iifname "{{tap}}" ip daddr {{dns}} udp dport 53 accept
  iifname "{{tap}}" ct state established,related accept
  iifname "{{tap}}" drop
 }
 chain forward {
  type filter hook forward priority -10; policy accept;
  iifname "{{tap}}" meta nfproto ipv6 drop
  iifname "{{tap}}" ip saddr != {{guest}} drop
  iifname "{{tap}}" ip daddr {{dns}} udp dport 53 accept
  oifname "{{tap}}" ct state established,related accept
  iifname "{{tap}}" drop
  oifname "{{tap}}" drop
 }
 chain postrouting {
  type nat hook postrouting priority srcnat; policy accept;
  ip saddr {{guest}} ip daddr {{dns}} udp dport 53 masquerade
 }
}
`)
}
func (r *FirecrackerRuntime) managedNetworkUp(ctx context.Context, m manifest) error {
	path := filepath.Join(r.vmPaths(m.Spec.ID).stateDir, "egress.nft")
	if err := os.WriteFile(path, []byte(managedRules(m, r.config.DNS)), 0600); err != nil {
		return err
	}
	defer os.Remove(path)
	if _, err := r.run(ctx, "nft", "-f", path); err != nil {
		return err
	}
	return nil
}
