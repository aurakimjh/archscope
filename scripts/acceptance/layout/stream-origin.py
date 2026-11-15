"""Owned loopback-only Docker fixture for encoding/cap/stream acceptance."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import gzip
import json
import threading
import time
import urllib.parse
import zlib

events = []
lock = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def event(self, stage):
        with lock:
            events.append({"path": self.path, "stage": stage, "method": self.command,
                           "acceptEncoding": self.headers.get("Accept-Encoding", "")})

    def chunk(self, payload):
        self.wfile.write(f"{len(payload):x}\r\n".encode() + payload + b"\r\n")
        self.wfile.flush()

    def do_GET(self):
        path = urllib.parse.urlsplit(self.path).path
        if path == "/stats":
            with lock:
                payload = json.dumps(events).encode()
            self.send_response(200)
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            return
        self.event("start")
        marker = b'{"marker":"identity-proof"}'
        if path in ("/hold", "/hold-gzip", "/timeout", "/stream"):
            compressed = path == "/hold-gzip"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Transfer-Encoding", "chunked")
            if compressed:
                self.send_header("Content-Encoding", "gzip")
            self.end_headers()
            compressor = zlib.compressobj(wbits=31) if compressed else None
            try:
                if path == "/stream":
                    for data in (b"first\n", b"second\n", b"third\n"):
                        self.chunk(data)
                        time.sleep(0.05)
                    self.wfile.write(b"0\r\n\r\n")
                    self.wfile.flush()
                    self.event("complete")
                    return
                prefix = b"prefix\n"
                self.chunk(compressor.compress(prefix) + compressor.flush(zlib.Z_SYNC_FLUSH)
                           if compressed else prefix)
                self.event("prefix")
                for _ in range(20):
                    time.sleep(0.5)
                    data = b"heartbeat\n"
                    self.chunk(compressor.compress(data) + compressor.flush(zlib.Z_SYNC_FLUSH)
                               if compressed else data)
                if compressed:
                    self.chunk(compressor.flush())
                self.wfile.write(b"0\r\n\r\n")
                self.wfile.flush()
            except (BrokenPipeError, ConnectionResetError):
                self.event("client-closed")
            return
        headers = []
        mime = "application/json"
        payload = marker
        declared = None
        if path == "/identity":
            headers = [("Content-Encoding", "IdEnTiTy")]
        elif path == "/identity-short":
            payload, declared = b"short", 20
            headers = [("Content-Encoding", "identity"), ("Connection", "close")]
        elif path in ("/x-gzip", "/stacked"):
            payload = gzip.compress(marker, mtime=0)
            headers = [("Content-Encoding", "x-gzip" if path == "/x-gzip" else "gzip")]
            if path == "/stacked":
                headers.append(("Content-Encoding", "br"))
        elif path in ("/binary", "/binary-gzip", "/binary-exact"):
            mime = "application/octet-stream"
            payload = b"\xff\x00" * ((1048576 if path == "/binary-exact" else 1048610) // 2)
            if path == "/binary-gzip":
                payload = gzip.compress(payload, mtime=0)
                headers = [("Content-Encoding", "gzip")]
        elif path == "/head":
            declared = 999
        else:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", mime)
        self.send_header("Cache-Control", "no-store")
        for name, value in headers:
            self.send_header(name, value)
        self.send_header("Content-Length", str(declared if declared is not None else len(payload)))
        self.end_headers()
        try:
            if self.command != "HEAD":
                self.wfile.write(payload)
                self.wfile.flush()
            self.event("sent")
        except (BrokenPipeError, ConnectionResetError):
            self.event("client-closed")
        if path == "/identity-short":
            self.close_connection = True

    do_HEAD = do_GET

    def log_message(self, *_):
        pass


ThreadingHTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
