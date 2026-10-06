"""Host-only, VM-bound OpenAI grant for a local TLS interception experiment."""
import hmac
import ipaddress
import json
import os
from pathlib import Path
import sqlite3
import stat
import subprocess
import threading
import time

HOST = "api.openai.com"
OPERATIONS = {("GET", "/v1/models"), ("POST", "/v1/responses"), ("POST", "/v1/chat/completions")}
MAX_REQUEST = 256 * 1024
MAX_RESPONSE = 2 * 1024 * 1024


class Denied(Exception):
    pass


def private_file(path):
    path = Path(path)
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise Denied("Host credential/config file must be owned by the proxy user and mode 0600")
    return path


class OpenAIGrant:
    def __init__(self, config_file, inspect_vm=None):
        self.config_file = private_file(config_file)
        self.config = json.loads(self.config_file.read_text())
        self.key = private_file(self.config["key_file"]).read_text().strip()
        if not self.key or not self.key.isascii() or any(character.isspace() for character in self.key):
            raise Denied("Invalid host credential file")
        self.placeholder = self.config["placeholder"]
        if not self.placeholder.startswith("outpost-placeholder-") or hmac.compare_digest(self.key, self.placeholder):
            raise Denied("A distinct outpost-placeholder-* credential is required")
        ipaddress.IPv4Address(self.config["guest_ip"])
        if not 1 <= self.config["max_requests"] <= 100:
            raise Denied("Experiment request budget must be between 1 and 100")
        if not self.config["models"] or not all(isinstance(model, str) for model in self.config["models"]):
            raise Denied("Explicit model allowlist required")
        self.inspect_vm = inspect_vm or self.outpost_inspect
        self.lock = threading.Lock()
        self.usage = sqlite3.connect(self.config_file.parent / "openai-usage.sqlite3", check_same_thread=False)
        os.chmod(self.config_file.parent / "openai-usage.sqlite3", 0o600)
        self.usage.execute("CREATE TABLE IF NOT EXISTS requests (id INTEGER PRIMARY KEY, grant_id TEXT, created_at REAL, status INTEGER)")
        self.usage.commit()

    def outpost_inspect(self, name):
        result = subprocess.run([self.config.get("outpost", "outpost"), "inspect", name, "-o", "json"], capture_output=True, timeout=10)
        if result.returncode:
            raise Denied("Cannot confirm assigned VM")
        return json.loads(result.stdout)

    def active_vm(self, source_ip, inspect_vm=True):
        if source_ip != self.config["guest_ip"]:
            raise Denied("Request is not from the assigned VM")
        if (self.config_file.parent / "REVOKED").exists():
            raise Denied("Credential egress revoked locally")
        if self.config["expires_at"] <= time.time():
            raise Denied("Credential grant expired")
        if inspect_vm:
            vm = self.inspect_vm(self.config["vm_name"])
            if vm["id"] != self.config["vm_id"] or vm["guest_ip"] != self.config["guest_ip"] or vm["status"] != "running":
                raise Denied("VM identity no longer matches the grant")

    def validate_request(self, source_ip, method, host, port, scheme, path, authorization):
        self.active_vm(source_ip)
        if host != HOST or port != 443 or scheme != "https":
            raise Denied("Only HTTPS api.openai.com:443 is permitted")
        if (method, path) not in OPERATIONS:
            raise Denied("OpenAI operation is not permitted")
        if not isinstance(authorization, str) or not authorization.isascii() or not hmac.compare_digest(authorization, "Bearer " + self.placeholder):
            raise Denied("Expected this VM's placeholder Authorization header")

    def authorize(self, source_ip, method, host, port, scheme, path, authorization, body):
        self.validate_request(source_ip, method, host, port, scheme, path, authorization)
        if len(body) > MAX_REQUEST:
            raise Denied("Request exceeds 256 KiB")
        if method == "POST":
            try:
                payload = json.loads(body)
            except (ValueError, TypeError):
                raise Denied("Expected a JSON OpenAI request")
            if not isinstance(payload, dict) or payload.get("model") not in self.config["models"]:
                raise Denied("Model is not approved for this experiment")
            limit = payload.get("max_output_tokens") if path == "/v1/responses" else payload.get("max_completion_tokens", payload.get("max_tokens"))
            if type(limit) is not int or not 1 <= limit <= self.config.get("max_output_tokens", 128):
                raise Denied("Explicit bounded output token limit required")
            # Tools create authority beyond a deterministic text smoke experiment.
            if payload.get("tools"):
                raise Denied("Tools are not approved for this experiment")
        with self.lock:
            try:
                self.usage.execute("BEGIN IMMEDIATE")
                used = self.usage.execute("SELECT count(*) FROM requests WHERE grant_id=?", (self.config["grant_id"],)).fetchone()[0]
                if used >= self.config["max_requests"]:
                    raise Denied("Credential grant request budget exhausted")
                cursor = self.usage.execute("INSERT INTO requests(grant_id,created_at) VALUES(?,?)", (self.config["grant_id"], time.time()))
                self.usage.commit()
                return cursor.lastrowid
            except Exception:
                self.usage.rollback()
                raise

    def finish(self, request_id, status):
        with self.lock:
            self.usage.execute("UPDATE requests SET status=? WHERE id=?", (status, request_id))
            self.usage.commit()


class RedactedStream:
    """Bound streaming output and redact literal credential echoes across chunks."""
    def __init__(self, secret, limit=MAX_RESPONSE, still_active=None):
        self.secret = secret.encode()
        self.limit = limit
        self.total = 0
        self.pending = b""
        self.still_active = still_active

    def __call__(self, data):
        if self.still_active:
            self.still_active()
        self.total += len(data)
        if self.total > self.limit:
            raise Denied("Provider response exceeds experiment limit")
        self.pending += data
        if not data:
            result = self.pending.replace(self.secret, b"[REDACTED]")
            self.pending = b""
            return result
        # Emit only bytes which cannot start a credential spanning the next chunk.
        safe_end = max(0, len(self.pending) - len(self.secret) + 1)
        position, output = 0, bytearray()
        while position < safe_end:
            if self.pending.startswith(self.secret, position):
                output.extend(b"[REDACTED]")
                position += len(self.secret)
            else:
                output.append(self.pending[position])
                position += 1
        self.pending = self.pending[position:]
        return bytes(output)
