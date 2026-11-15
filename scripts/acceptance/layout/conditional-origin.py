"""Owned loopback Docker fixture for form/multipart and response boundaries."""
from email import policy
from email.parser import BytesParser
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qsl, urlsplit
import base64
import gzip
import hashlib
import json
import threading
import time

events = []
lock = threading.Lock()

class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def respond(self):
        path = urlsplit(self.path).path
        if path == "/stats":
            with lock:
                payload = json.dumps(events, ensure_ascii=False).encode()
        else:
            body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
            content_type = self.headers.get("Content-Type", "")
            event = {"path": path, "method": self.command, "contentType": content_type,
                     "bodyBytes": len(body), "bodySha256": hashlib.sha256(body).hexdigest()}
            if content_type.startswith("application/x-www-form-urlencoded"):
                event["fields"] = parse_qsl(body.decode("utf-8"), keep_blank_values=True)
            elif content_type.startswith("multipart/form-data"):
                message = BytesParser(policy=policy.default).parsebytes(
                    ("Content-Type: " + content_type + "\r\nMIME-Version: 1.0\r\n\r\n").encode() + body)
                event["parts"] = []
                for part in message.iter_parts():
                    value = part.get_payload(decode=True)
                    event["parts"].append({"name": part.get_param("name", header="content-disposition"),
                        "filename": part.get_filename(), "contentType": part.get_content_type(),
                        "bytes": len(value), "sha256": hashlib.sha256(value).hexdigest(),
                        "base64": base64.b64encode(value).decode()})
            with lock:
                events.append(event)
            if path == "/delay":
                time.sleep(2)
            payload = json.dumps({"marker": "windows-conditional-origin", **event}, ensure_ascii=False).encode()
            if path == "/gzip":
                payload = gzip.compress(b'{"marker":"gzip-boundary"}', mtime=0)
            elif path == "/bad-gzip":
                payload = b"invalid-gzip-fixture"
            elif path == "/encoded":
                payload = b"not-a-decoded-br-body"
            elif path == "/large":
                payload = b"builder-large-prefix:" + b"x" * (1048576 + 37)
            elif path == "/short":
                payload = b"short"
            elif path == "/error":
                payload = b'{"marker":"deliberate-503"}'
        self.send_response(503 if path == "/error" else 200)
        self.send_header("Content-Type", "application/json" if path not in ("/large", "/short", "/encoded") else "text/plain")
        if path in ("/gzip", "/bad-gzip"):
            self.send_header("Content-Encoding", "gzip")
        if path == "/encoded":
            self.send_header("Content-Encoding", "br")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(payload) + 7 if path == "/short" else len(payload)))
        if path == "/short":
            self.send_header("Connection", "close")
        self.end_headers()
        try:
            self.wfile.write(payload)
            self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError):
            pass
        if path == "/short":
            self.close_connection = True

    do_GET = do_POST = do_PATCH = respond
    def log_message(self, *_):
        pass

ThreadingHTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
