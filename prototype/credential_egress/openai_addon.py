"""Run only with the dedicated launcher; never enable mitmproxy flow dumps/UI."""
from pathlib import Path
import json
import sqlite3
import subprocess
import sys

from mitmproxy import ctx, http

sys.path.insert(0, str(Path(__file__).resolve().parent))
from openai_policy import Denied, HOST, MAX_REQUEST, MAX_RESPONSE, OpenAIGrant, RedactedStream


class OpenAIEgress:
    def load(self, loader):
        loader.add_option("openai_grant", str, "", "Private host-only OpenAI grant JSON file")

    def configure(self, updated):
        if "openai_grant" in updated:
            self.grant = OpenAIGrant(ctx.options.openai_grant)

    def reject(self, flow, message, status=403):
        flow.request.headers.pop("Authorization", None)
        body = json.dumps({"error": {"message": "Credential egress: " + message, "type": "permission_error", "code": "outpost_egress_denied"}}).encode()
        flow.response = http.Response.make(status, body, {"Content-Type": "application/json", "Cache-Control": "no-store"})

    def details(self, flow):
        request = flow.request
        if flow.client_conn.sni != HOST or request.host_header not in (HOST, HOST + ":443"):
            raise Denied("TLS SNI and HTTP authority must both be api.openai.com")
        return (flow.client_conn.peername[0], request.method, request.host, request.port, request.scheme, request.path, request.headers.get("Authorization", ""))

    def requestheaders(self, flow):
        try:
            self.grant.validate_request(*self.details(flow))
            if int(flow.request.headers.get("Content-Length", "0")) > MAX_REQUEST:
                raise Denied("Request exceeds 256 KiB")
            if flow.request.headers.get("Content-Encoding", "identity") != "identity":
                raise Denied("Compressed provider requests are unsupported")
        except Denied as error:
            self.reject(flow, str(error))
        except (OSError, ValueError, KeyError, sqlite3.Error, subprocess.SubprocessError):
            self.reject(flow, "Cannot validate active grant")

    def request(self, flow):
        if flow.response:
            return
        try:
            request_id = self.grant.authorize(*self.details(flow), flow.request.raw_content or b"")
            flow.metadata["openai_request_id"] = request_id
            # Do not forward guest-selected organization/project/auth/cookie headers.
            flow.request.headers.clear()
            flow.request.headers["Host"] = HOST
            flow.request.headers["Content-Type"] = "application/json"
            flow.request.headers["Content-Length"] = str(len(flow.request.raw_content or b""))
            flow.request.headers["Accept-Encoding"] = "identity"
            flow.request.headers["Authorization"] = "Bearer " + self.grant.key
        except Denied as error:
            self.reject(flow, str(error))
        except (OSError, ValueError, KeyError, sqlite3.Error, subprocess.SubprocessError):
            self.reject(flow, "Cannot validate active grant")

    def responseheaders(self, flow):
        if "openai_request_id" not in flow.metadata:
            return
        try:
            self.grant.active_vm(flow.client_conn.peername[0], inspect_vm=False)
            if 300 <= flow.response.status_code < 400:
                raise Denied("Provider redirects are not permitted")
            if flow.response.headers.get("Content-Encoding", "identity") != "identity":
                raise Denied("Compressed provider responses are unsupported")
            if int(flow.response.headers.get("Content-Length", "0")) > MAX_RESPONSE:
                raise Denied("Provider response exceeds 2 MiB")
            for name in list(flow.response.headers):
                if name.lower() in ("authorization", "proxy-authorization", "set-cookie"):
                    del flow.response.headers[name]
                else:
                    flow.response.headers[name] = flow.response.headers[name].replace(self.grant.key, "[REDACTED]")
            flow.response.headers.pop("Content-Length", None)
            flow.response.stream = RedactedStream(self.grant.key, still_active=lambda: self.grant.active_vm(flow.client_conn.peername[0], inspect_vm=False))
        except Denied as error:
            self.reject(flow, str(error), status=502)
        except (OSError, ValueError, KeyError, sqlite3.Error, subprocess.SubprocessError):
            self.reject(flow, "Cannot validate active grant", status=502)

    def response(self, flow):
        if "openai_request_id" in flow.metadata:
            self.grant.finish(flow.metadata["openai_request_id"], flow.response.status_code)
            if flow.response.raw_content:
                flow.response.content = flow.response.content.replace(self.grant.key.encode(), b"[REDACTED]")
        flow.request.headers.pop("Authorization", None)

    def error(self, flow):
        flow.request.headers.pop("Authorization", None)
        if "openai_request_id" in flow.metadata:
            self.grant.finish(flow.metadata["openai_request_id"], 0)


addons = [OpenAIEgress()]
