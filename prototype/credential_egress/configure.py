#!/usr/bin/env python3
"""Host administration: grant bounded OpenAI egress to an existing Outpost VM."""
import argparse
import ipaddress
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

from openai_policy import private_file


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vm", required=True, help="Existing running Outpost VM name")
    parser.add_argument("--key-file", required=True)
    parser.add_argument("--output", required=True, help="New private grant directory")
    parser.add_argument("--model", default="gpt-4.1-nano")
    parser.add_argument("--port", type=int, default=18443)
    parser.add_argument("--requests", type=int, default=8)
    parser.add_argument("--seconds", type=int, default=1800)
    args = parser.parse_args()
    if not 1024 <= args.port <= 65535 or not 1 <= args.requests <= 100 or not 1 <= args.seconds <= 1800:
        parser.error("Invalid listener port, request limit or grant duration")
    key_file = private_file(args.key_file).resolve()
    result = subprocess.run(["outpost", "inspect", args.vm, "-o", "json"], capture_output=True, check=True, timeout=10)
    vm = json.loads(result.stdout)
    if vm["status"] != "running":
        parser.error("Select an existing running VM; this command never creates or starts one")
    guest_ip = ipaddress.IPv4Address(vm["guest_ip"])
    output = Path(args.output).resolve()
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    placeholder = "outpost-placeholder-" + secrets.token_hex(24)
    config = {"grant_id": secrets.token_hex(16), "vm_name": args.vm,
              "vm_id": vm["id"], "guest_ip": str(guest_ip),
              "listen_host": str(guest_ip - 1), "listen_port": args.port,
              "key_file": str(key_file), "placeholder": placeholder,
              "models": [args.model], "max_output_tokens": 128, "max_requests": args.requests,
              "expires_at": time.time() + args.seconds}
    path = output / "grant.json"
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as file:
        json.dump(config, file, indent=2)
    print(json.dumps({"grant_file": str(path), "guest_placeholder": placeholder, "model": args.model,
                      "expires_at": config["expires_at"], "note": "No real credential included in this output"}))


if __name__ == "__main__":
    main()
