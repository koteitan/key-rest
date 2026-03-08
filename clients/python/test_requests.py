"""Tests for key_rest.requests using a mock Unix socket daemon."""

import json
import os
import socket
import tempfile
import threading
import unittest

from key_rest.requests import get, post, KeyRestError, _request


class MockDaemon:
    """A mock key-rest-daemon that listens on a Unix socket."""

    def __init__(self, socket_path, handler):
        self.socket_path = socket_path
        self.handler = handler
        self.server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.server.bind(socket_path)
        self.server.listen(1)
        self.thread = threading.Thread(target=self._accept, daemon=True)

    def start(self):
        self.thread.start()

    def _accept(self):
        conn, _ = self.server.accept()
        try:
            buf = b""
            while b"\n" not in buf:
                chunk = conn.recv(65536)
                if not chunk:
                    break
                buf += chunk
            line = buf.split(b"\n", 1)[0]
            req = json.loads(line)
            resp = self.handler(req)
            conn.sendall((json.dumps(resp) + "\n").encode())
        finally:
            conn.close()

    def stop(self):
        self.server.close()


class TestRequests(unittest.TestCase):
    def test_get(self):
        with tempfile.TemporaryDirectory() as d:
            sock = os.path.join(d, "test.sock")

            def handler(req):
                self.assertEqual(req["method"], "GET")
                self.assertIn("key-rest://", req["headers"]["Authorization"])
                return {
                    "status": 200,
                    "statusText": "200 OK",
                    "headers": {"Content-Type": "application/json"},
                    "body": '{"ok":true}',
                }

            daemon = MockDaemon(sock, handler)
            daemon.start()
            try:
                resp = get(
                    "https://api.example.com/data",
                    headers={"Authorization": "Bearer key-rest://user1/test/key"},
                    socket_path=sock,
                )
                self.assertEqual(resp.status_code, 200)
                self.assertEqual(resp.json(), {"ok": True})
                self.assertTrue(resp.ok)
            finally:
                daemon.stop()

    def test_post_json(self):
        with tempfile.TemporaryDirectory() as d:
            sock = os.path.join(d, "test.sock")

            def handler(req):
                self.assertEqual(req["method"], "POST")
                self.assertEqual(json.loads(req["body"]), {"query": "test"})
                return {
                    "status": 200,
                    "body": '{"result":"ok"}',
                }

            daemon = MockDaemon(sock, handler)
            daemon.start()
            try:
                resp = post(
                    "https://api.example.com/search",
                    json_data={"query": "test"},
                    socket_path=sock,
                )
                self.assertEqual(resp.status_code, 200)
            finally:
                daemon.stop()

    def test_params(self):
        with tempfile.TemporaryDirectory() as d:
            sock = os.path.join(d, "test.sock")

            def handler(req):
                self.assertIn("key=val", req["url"])
                return {"status": 200, "body": ""}

            daemon = MockDaemon(sock, handler)
            daemon.start()
            try:
                resp = get(
                    "https://api.example.com/search",
                    params={"key": "val"},
                    socket_path=sock,
                )
                self.assertEqual(resp.status_code, 200)
            finally:
                daemon.stop()

    def test_error_response(self):
        with tempfile.TemporaryDirectory() as d:
            sock = os.path.join(d, "test.sock")

            def handler(req):
                return {
                    "error": {
                        "code": "KEY_NOT_FOUND",
                        "message": "key not found",
                    }
                }

            daemon = MockDaemon(sock, handler)
            daemon.start()
            try:
                with self.assertRaises(KeyRestError) as ctx:
                    get("https://api.example.com/", socket_path=sock)
                self.assertIn("KEY_NOT_FOUND", str(ctx.exception))
            finally:
                daemon.stop()

    def test_connection_error(self):
        with self.assertRaises(Exception):
            get("https://example.com/", socket_path="/nonexistent/socket.sock")


if __name__ == "__main__":
    unittest.main()
