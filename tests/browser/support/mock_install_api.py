import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_OPTIONS(self):
        self.send_response(204)
        self._headers()
        self.end_headers()

    def do_GET(self):
        payload = None
        if self.path == "/api/v1/install/status":
            payload = {"installed": False, "version": "browser-smoke", "mode": "docker"}
        elif self.path == "/api/v1/install/readiness":
            payload = {
                "infrastructure": [
                    {"id": identifier, "name": identifier, "status": "pass", "message": "ready", "required": True}
                    for identifier in ("file_permissions", "environment", "postgres", "redis")
                ],
                "capabilities": [],
            }
        if payload is None:
            self.send_error(404)
            return
        encoded = json.dumps({"code": 0, "message": "ok", "data": payload}).encode()
        self.send_response(200)
        self._headers()
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def _headers(self):
        self.send_header("Access-Control-Allow-Origin", "http://127.0.0.1:3000")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.send_header("Access-Control-Allow-Methods", "GET,OPTIONS")

    def log_message(self, *_):
        return


ThreadingHTTPServer(("127.0.0.1", 8080), Handler).serve_forever()
