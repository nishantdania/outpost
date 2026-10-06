#!/usr/bin/env python3
"""Launch a fixed-upstream OpenAI TLS interceptor as the current unprivileged user."""
import argparse
import ipaddress
import os
from pathlib import Path
import sys

from openai_policy import OpenAIGrant


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--grant", required=True)
    args = parser.parse_args()
    grant = OpenAIGrant(args.grant)
    config = grant.config
    grant.active_vm(config["guest_ip"])
    address = ipaddress.IPv4Address(config["listen_host"])
    if address != ipaddress.IPv4Address(config["guest_ip"]) - 1:
        parser.error("Listener must be the assigned Outpost /30 gateway address")
    if not 1024 <= config["listen_port"] <= 65535:
        parser.error("Invalid unprivileged listener port")
    os.umask(0o077)
    ca = Path(args.grant).resolve().parent / "ca"
    ca.mkdir(mode=0o700, exist_ok=True)
    from mitmproxy.certs import CertStore
    CertStore.from_store(ca, "mitmproxy", 2048)
    binary = Path(sys.executable).parent / "mitmdump"
    command = [str(binary), "-q", "--mode", "reverse:https://api.openai.com:443",
                     "--listen-host", str(address), "--listen-port", str(config["listen_port"]),
                     "--set", "confdir=" + str(ca), "--set", "openai_grant=" + str(Path(args.grant).resolve()),
                     "--set", "upstream_cert=false", "--set", "keep_host_header=true",
                     "--set", "connection_strategy=lazy", "--set", "ssl_insecure=false",
                     "--set", "flow_detail=0", "--set", "termlog_verbosity=error",
                     "--set", "body_size_limit=256k", "--set", "websocket=false",
                     "-s", str(Path(__file__).with_name("openai_addon.py"))]
    os.execv(binary, command)


if __name__ == "__main__":
    main()
