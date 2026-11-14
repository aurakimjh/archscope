"""Loopback-bound Docker synthetic Builder/timing fixture; no external traffic."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit, parse_qs
import gzip, hashlib, json, struct, threading, time, zlib

events = []
lock = threading.Lock()
class Handler(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    def respond(self):
        target = urlsplit(self.path)
        if target.path == '/stats':
            with lock: payload = json.dumps(events).encode()
        else:
            body = self.rfile.read(int(self.headers.get('Content-Length', '0')))
            item = {'path': target.path, 'method': self.command,
                    'contentType': self.headers.get('Content-Type', ''),
                    'bodyBytes': len(body), 'bodySha256': hashlib.sha256(body).hexdigest()}
            with lock: events.append(item)
            if target.path == '/slow': time.sleep(min(15, float(parse_qs(target.query).get('seconds', ['5'])[0])))
            if target.path == '/redirect':
                self.send_response(302); self.send_header('Location', '/must-not-follow')
                self.send_header('Content-Length', '0'); self.end_headers(); return
            payload = json.dumps({'marker': 'remaining-origin', **item}).encode()
            if target.path == '/large': payload = b'capture-body-prefix:' + b'x' * 300000
            if target.path == '/html': payload = b'<html><body><h1>Inert fixture</h1><script>document.title="BODY_SCRIPT_EXECUTED"</script></body></html>'
            if target.path == '/gzip': payload = gzip.compress(b'{"marker":"compressed-body-proof"}', mtime=0)
            if target.path == '/image':
                def chunk(kind, value): return struct.pack('>I', len(value)) + kind + value + struct.pack('>I', zlib.crc32(kind + value))
                payload = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', 1, 1, 8, 2, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(b'\x00\xff\x00\x00')) + chunk(b'IEND', b'')
        self.send_response(200)
        self.send_header('Content-Type', 'image/png' if target.path == '/image' else 'text/html' if target.path == '/html' else 'text/plain' if target.path == '/large' else 'application/json')
        if target.path == '/gzip': self.send_header('Content-Encoding', 'gzip')
        if target.path == '/body':
            self.send_header('Set-Cookie', 'same=first; Path=/; SameSite=Lax')
            self.send_header('Set-Cookie', 'same=second; Path=/; HttpOnly')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('Content-Length', str(len(payload)))
        self.end_headers()
        try: self.wfile.write(payload)
        except (BrokenPipeError, ConnectionResetError): pass
    do_GET = do_POST = do_PATCH = respond
    def log_message(self, *_): pass
ThreadingHTTPServer(('0.0.0.0', 8000), Handler).serve_forever()
