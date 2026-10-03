from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import threading
import unittest

spec = importlib.util.spec_from_file_location("demo_pdv", Path(__file__).resolve().parents[1] / "demo_pdv.py")
demo = importlib.util.module_from_spec(spec)
spec.loader.exec_module(demo)


@contextmanager
def http_reply(status, body, extra=None):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(status)
            for key, value in (extra or {}).items():
                self.send_header(key, value)
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server.server_port
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


class DemoTests(unittest.TestCase):
    def test_occupied_port_is_refused_without_stopping_its_listener(self):
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            listener.listen()
            with self.assertRaises(demo.DemoError):
                demo.check_port(listener.getsockname()[1])
            self.assertEqual(listener.getsockopt(socket.SOL_SOCKET, socket.SO_ACCEPTCONN), 1)

    def test_bad_port_and_external_paths_are_refused(self):
        for port in (0, -1, 65536):
            with self.assertRaises(demo.DemoError):
                demo.check_port(port)
        for path in ("http://external", "//external:8181/", "/../login"):
            with self.assertRaises(demo.DemoError):
                demo.API(1).request(path)

    def test_redirect_cannot_contact_a_second_endpoint(self):
        with http_reply(302, b"", {"Location": "http://127.0.0.1:1/target"}) as port:
            with self.assertRaisesRegex(demo.DemoError, "redirecionamento"):
                demo.API(port).request("/health")

    def test_error_bodies_are_not_printed_in_exception(self):
        with http_reply(403, b'private fixture password token') as port:
            with self.assertRaises(demo.DemoError) as caught:
                demo.API(port).request("/health")
            self.assertNotIn("private fixture", str(caught.exception))
            self.assertIn("HTTP 403", str(caught.exception))

    def test_html_arrays_and_large_bodies_are_refused(self):
        for body in (b"<html>index</html>", b"[]", b"x" * 131073):
            with http_reply(200, body) as port:
                with self.assertRaises(demo.DemoError):
                    demo.API(port).request("/health")

    def test_json_health_passes(self):
        with http_reply(200, b'{"status":"local"}') as port:
            self.assertEqual(demo.API(port).request("/health"), {"status": "local"})

    def test_foreign_context_prevents_contract_and_stock_writes(self):
        class API:
            def __init__(self):
                self.calls = []

            def request(self, path, token=None, body=None, method=None):
                self.calls.append(path)
                if path == "/login":
                    return {"token": "fixture"}
                if path == "/me":
                    return {"tenant_id": "foreign", "store_id": "store", "device_id": "device", "identity_id": "actor"}
                if path == "/logout":
                    return None
                self.fail = "unexpected mutation"
                raise AssertionError(self.fail)

        api = API()
        with self.assertRaises(demo.DemoError):
            demo.seed(api, "actor", "fixture", {"tenant_id": "tenant", "store_id": "store", "device_id": "device"}, {})
        self.assertEqual(api.calls, ["/login", "/me", "/logout"])

    def test_private_metadata_is_exclusive_and_private(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "demo.json"
            demo.private_file(path, {"owner_id": "actor"})
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(FileExistsError):
                demo.private_file(path, {"owner_id": "foreign"})
            self.assertEqual(json.loads(path.read_text()), {"owner_id": "actor"})

    def test_cleanup_stops_only_the_process_it_started(self):
        owned = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
        other = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
        try:
            demo.stop_owned(owned)
            self.assertIsNotNone(owned.poll())
            self.assertIsNone(other.poll())
        finally:
            demo.stop_owned(other)


if __name__ == "__main__":
    unittest.main()
