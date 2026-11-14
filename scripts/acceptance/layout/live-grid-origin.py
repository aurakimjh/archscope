"""Deterministic synthetic HTTP fixture for native live grid/search acceptance."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit, parse_qs
import json


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def respond(self):
        index = int(parse_qs(urlsplit(self.path).query)["slot"][0])
        length = int(self.headers.get("Content-Length", "0"))
        if length:
            self.rfile.read(length)
        body = json.dumps({"marker": "live-grid-origin", "index": index}).encode()
        self.send_response([200, 404, 503][index % 3])
        self.send_header("Content-Type", ["application/json", "text/plain", "text/html"][index % 3])
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        if index < 40:
            self.send_header("X-Grid-Proof", "header-needle")
        if index < 30:
            self.send_header("Set-Cookie", "proof=cookie-needle; Path=/; SameSite=Lax")
        self.end_headers()
        self.wfile.write(body)

    do_GET = respond
    do_POST = respond

    def log_message(self, *_):
        pass


ThreadingHTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
