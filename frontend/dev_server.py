import http.server
import socketserver
import sys

PORT = 8080
DIRECTORY = "frontend"

if len(sys.argv) > 1:
    PORT = int(sys.argv[1])
if len(sys.argv) > 2:
    DIRECTORY = sys.argv[2]

class NoCacheHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Cache-Control", "no-cache, no-store, must-revalidate")
        self.send_header("Pragma", "no-cache")
        self.send_header("Expires", "0")
        super().end_headers()

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=DIRECTORY, **kwargs)

if __name__ == "__main__":
    with socketserver.TCPServer(("", PORT), NoCacheHTTPRequestHandler) as httpd:
        print(f"Serving HTTP on 0.0.0.0 port {PORT} (http://0.0.0.0:{PORT}/) ...")
        print(f"Serving directory: {DIRECTORY}")
        print("Cache-Control headers set to prevent caching.")
        httpd.serve_forever()
