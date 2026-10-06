#!/usr/bin/env python3
"""Generate/apply only a per-VM nftables table; never flush existing host rules."""
import argparse
import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess


def table_name(config):
    grant_id = config["grant_id"]
    if not isinstance(grant_id, str) or not re.fullmatch(r"[0-9a-f]{32}", grant_id):
        raise ValueError("Expected a host-generated grant ID")
    return "outpost_openai_" + grant_id


def rules(config, interface):
    guest = ipaddress.IPv4Address(config["guest_ip"])
    host = ipaddress.IPv4Address(config["listen_host"])
    port = config["listen_port"]
    if host != guest - 1 or type(port) is not int or not 1024 <= port <= 65535:
        raise ValueError("Invalid Outpost /30 listener")
    if not re.fullmatch(r"outpost[0-9a-f]+", interface):
        raise ValueError("Expected an Outpost TAP interface")
    table = table_name(config)
    text = f"""table inet {table} {{
  chain intercept {{
    type nat hook prerouting priority -110; policy accept;
    iifname "{interface}" ip saddr {guest} tcp dport 443 redirect to :{port}
  }}
  chain guest_input {{
    type filter hook input priority -20; policy accept;
    iifname "{interface}" ct direction reply ct state established,related accept
    iifname "{interface}" ip saddr {guest} ip daddr {host} tcp dport {port} accept
    iifname "{interface}" drop
  }}
  chain guest_forward {{
    type filter hook forward priority -20; policy accept;
    iifname "{interface}" ip saddr {guest} ip daddr {{ 1.1.1.1, 8.8.8.8 }} udp dport 53 accept
    iifname "{interface}" ip saddr {guest} ip daddr {{ 1.1.1.1, 8.8.8.8 }} tcp dport 53 accept
    iifname "{interface}" drop
    oifname "{interface}" ct state established,related accept
    oifname "{interface}" drop
  }}
}}
"""
    return table, text


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--grant", required=True)
    parser.add_argument("action", choices=("print", "apply", "remove"))
    args = parser.parse_args()
    config = json.loads(Path(args.grant).read_text())
    guest = str(ipaddress.IPv4Address(config["guest_ip"]))
    table = table_name(config)
    if args.action == "remove":
        if os.geteuid() != 0:
            parser.error("Host nftables removal requires sudo")
        subprocess.run(["nft", "delete", "table", "inet", table], check=True)
        return
    route = json.loads(subprocess.check_output(["ip", "-j", "route", "get", guest]))[0]
    table, text = rules(config, route["dev"])
    if args.action == "print":
        print(text, end="")
        return
    if os.geteuid() != 0:
        parser.error("Host nftables apply/remove requires sudo; print is unprivileged")
    exists = subprocess.run(["nft", "list", "table", "inet", table], capture_output=True)
    if exists.returncode == 0:
        parser.error("Table already exists; inspect or remove this experiment's table first")
    # Validate the complete transaction before atomically installing it.
    subprocess.run(["nft", "--check", "--file", "-"], input=text.encode(), check=True)
    subprocess.run(["nft", "--file", "-"], input=text.encode(), check=True)
    print("Installed only " + table + "; remove it before destroying/resuming the VM")


if __name__ == "__main__":
    main()
