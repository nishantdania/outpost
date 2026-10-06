import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import time
import unittest
from unittest import mock

from mitmproxy.test import tflow

from network import rules
from openai_addon import OpenAIEgress
from openai_policy import Denied, HOST, OpenAIGrant, RedactedStream


class OpenAIEgressTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        key = self.root / "host-key"
        key.write_text("synthetic-host-only-openai-key")
        key.chmod(0o600)
        self.config = {"grant_id": "0123456789abcdef0123456789abcdef", "vm_name": "example-vm",
                       "vm_id": "vm-1", "guest_ip": "172.30.20.2", "listen_host": "172.30.20.1", "listen_port": 18443,
                       "key_file": str(key), "placeholder": "outpost-placeholder-test", "models": ["gpt-4.1-nano"],
                       "max_requests": 3, "max_output_tokens": 128, "expires_at": time.time() + 1000}
        path = self.root / "grant.json"
        path.write_text(json.dumps(self.config))
        path.chmod(0o600)
        self.grant = OpenAIGrant(path, inspect_vm=lambda name: {"id": "vm-1", "guest_ip": "172.30.20.2", "status": "running"})

    def tearDown(self):
        self.grant.usage.close()
        self.temp.cleanup()

    def args(self, **changes):
        args = {"source_ip": "172.30.20.2", "method": "POST", "host": HOST, "port": 443, "scheme": "https", "path": "/v1/responses", "authorization": "Bearer outpost-placeholder-test", "body": json.dumps({"model": "gpt-4.1-nano", "input": "hello", "max_output_tokens": 64}).encode()}
        args.update(changes)
        return args

    def flow(self):
        flow = tflow.tflow()
        flow.client_conn.peername = ("172.30.20.2", 12345)
        flow.client_conn.sni = HOST
        flow.request.host = HOST
        flow.request.port = 443
        flow.request.scheme = "https"
        flow.request.method = "POST"
        flow.request.path = "/v1/responses"
        flow.request.headers["Host"] = HOST
        flow.request.headers["Authorization"] = "Bearer outpost-placeholder-test"
        flow.request.content = self.args()["body"]
        return flow

    def addon(self):
        addon = OpenAIEgress()
        addon.grant = self.grant
        return addon

    def test_valid_request_and_persistent_budget(self):
        for _ in range(3):
            self.assertIsInstance(self.grant.authorize(**self.args()), int)
        with self.assertRaises(Denied):
            self.grant.authorize(**self.args())
        # A restart cannot reset quota: usage is tied to host-generated grant_id.
        second = OpenAIGrant(self.root / "grant.json", inspect_vm=self.grant.inspect_vm)
        try:
            with self.assertRaises(Denied):
                second.authorize(**self.args())
        finally:
            second.usage.close()

    def test_rejects_wrong_source_destination_header_operation_and_limits(self):
        for change in ({"source_ip": "172.30.20.3"}, {"host": "evil.example"}, {"port": 8443}, {"scheme": "http"},
                       {"path": "/v1/files"}, {"path": "/v1/responses?redirect=1"}, {"method": "DELETE"},
                       {"authorization": "Bearer different-placeholder"}, {"authorization": "Bearer non-ascii-é"},
                       {"body": b"x" * 262145},
                       {"body": b'{"model":"other","max_output_tokens":64}'},
                       {"body": b'{"model":"gpt-4.1-nano","max_output_tokens":10000}'},
                       {"body": b'{"model":"gpt-4.1-nano","max_output_tokens":64,"tools":[{}]}'},
                       {"body": b'[]'}, {"body": b'not-json'}):
            with self.subTest(change=change.keys()), self.assertRaises(Denied):
                self.grant.authorize(**self.args(**change))
        self.assertEqual(0, self.grant.usage.execute("SELECT count(*) FROM requests").fetchone()[0])

    def test_revocation_expiry_vm_identity_and_running_state(self):
        (self.root / "REVOKED").touch()
        with self.assertRaises(Denied):
            self.grant.authorize(**self.args())
        (self.root / "REVOKED").unlink()
        self.grant.config["expires_at"] = 0
        with self.assertRaises(Denied):
            self.grant.authorize(**self.args())
        self.grant.config["expires_at"] = time.time() + 1000
        self.grant.inspect_vm = lambda name: {"id": "reused-ip", "guest_ip": "172.30.20.2", "status": "running"}
        with self.assertRaises(Denied):
            self.grant.authorize(**self.args())
        self.grant.inspect_vm = lambda name: {"id": "vm-1", "guest_ip": "172.30.20.2", "status": "stopped"}
        with self.assertRaises(Denied):
            self.grant.authorize(**self.args())
        self.grant.inspect_vm = lambda name: {"id": "vm-1", "guest_ip": "172.30.20.3", "status": "running"}
        with self.assertRaises(Denied):
            self.grant.authorize(**self.args())

    def test_configure_grants_existing_vm_without_copying_key_or_provisioning(self):
        import configure
        output = self.root / "new-grant"
        vm = {"id": "vm-1", "guest_ip": "172.30.20.2", "status": "running"}
        argv = ["configure.py", "--vm", "example-vm", "--key-file", str(self.root / "host-key"), "--output", str(output)]
        stdout = io.StringIO()
        with mock.patch("sys.argv", argv), mock.patch("configure.subprocess.run", return_value=mock.Mock(stdout=json.dumps(vm).encode())) as run, contextlib.redirect_stdout(stdout):
            configure.main()
        self.assertEqual(["outpost", "inspect", "example-vm", "-o", "json"], run.call_args.args[0])
        self.assertEqual(1, run.call_count)
        config = json.loads((output / "grant.json").read_text())
        self.assertEqual("vm-1", config["vm_id"])
        self.assertEqual("example-vm", config["vm_name"])
        self.assertNotIn("gateway_state", config)
        self.assertNotIn(self.grant.key, (output / "grant.json").read_text() + stdout.getvalue())
        self.assertEqual(0o600, (output / "grant.json").stat().st_mode & 0o777)
        self.assertEqual(0o700, output.stat().st_mode & 0o777)

    def test_configure_rejects_stopped_vm_without_creating_grant(self):
        import configure
        output = self.root / "stopped-grant"
        vm = {"id": "vm-1", "guest_ip": "172.30.20.2", "status": "stopped"}
        argv = ["configure.py", "--vm", "example-vm", "--key-file", str(self.root / "host-key"), "--output", str(output)]
        with mock.patch("sys.argv", argv), mock.patch("configure.subprocess.run", return_value=mock.Mock(stdout=json.dumps(vm).encode())), contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            configure.main()
        self.assertFalse(output.exists())

    def test_host_key_file_permissions_enforced(self):
        (self.root / "host-key").chmod(0o644)
        with self.assertRaises(Denied):
            OpenAIGrant(self.root / "grant.json")

    def test_addon_injects_only_after_approval_and_removes_guest_auth_and_cookies(self):
        flow = self.flow()
        flow.request.headers["Cookie"] = "personal-cookie"
        flow.request.headers["OpenAI-Project"] = "unapproved-project"
        addon = self.addon()
        addon.requestheaders(flow)
        addon.request(flow)
        self.assertEqual("Bearer " + self.grant.key, flow.request.headers["Authorization"])
        self.assertNotIn("Cookie", flow.request.headers)
        self.assertNotIn("OpenAI-Project", flow.request.headers)
        self.assertIn("openai_request_id", flow.metadata)

    def test_wrong_http_authority_or_sni_is_rejected_even_with_fixed_reverse_upstream(self):
        for attr in ("authority", "sni"):
            flow = self.flow()
            if attr == "authority":
                flow.request.headers["Host"] = "evil.example"
            else:
                flow.client_conn.sni = "evil.example"
            addon = self.addon()
            addon.requestheaders(flow)
            addon.request(flow)
            self.assertEqual(403, flow.response.status_code)
            self.assertNotIn("Authorization", flow.request.headers)
            self.assertNotIn("openai_request_id", flow.metadata)

    def test_redirect_is_blocked_and_key_is_not_forwarded_or_returned(self):
        flow = self.flow()
        addon = self.addon()
        addon.request(flow)
        from mitmproxy import http
        flow.response = http.Response.make(302, b"", {"Location": "https://evil.example"})
        addon.responseheaders(flow)
        self.assertEqual(502, flow.response.status_code)
        self.assertNotIn("Location", flow.response.headers)
        self.assertNotIn("Authorization", flow.request.headers)
        self.assertNotIn(self.grant.key.encode(), flow.response.content)

    def test_response_streaming_redacts_split_key_and_limits_output(self):
        secret = "synthetic-secret"
        content = b"before synthetic-secret after synthetic-secret end"
        for size in range(1, 30):
            stream = RedactedStream(secret)
            output = b"".join(stream(content[i:i + size]) for i in range(0, len(content), size)) + stream(b"")
            self.assertEqual(b"before [REDACTED] after [REDACTED] end", output)
        stream = RedactedStream(secret, limit=2)
        with self.assertRaises(Denied):
            stream(b"abc")

    def test_stream_revocation_rechecked(self):
        stream = RedactedStream(self.grant.key, still_active=lambda: self.grant.active_vm("172.30.20.2", inspect_vm=False))
        stream(b"hello")
        (self.root / "REVOKED").touch()
        with self.assertRaises(Denied):
            stream(b"next")

    def test_host_rules_scope_only_assigned_tap_and_block_bypass(self):
        table, text = rules(self.config, "outpost0001")
        self.assertEqual("outpost_openai_0123456789abcdef0123456789abcdef", table)
        self.assertIn('iifname "outpost0001" ip saddr 172.30.20.2 tcp dport 443 redirect to :18443', text)
        self.assertIn('iifname "outpost0001" drop', text)
        self.assertIn('ct direction reply ct state established,related accept', text)
        other = self.config | {"grant_id": "a" * 32}
        self.assertNotEqual(table, rules(other, "outpost0001")[0])
        self.assertNotIn("flush", text)
        with self.assertRaises(ValueError):
            rules(self.config, "enp16s0")


if __name__ == "__main__":
    unittest.main()
