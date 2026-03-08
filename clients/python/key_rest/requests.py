"""requests-compatible interface for key-rest-daemon."""

import json
import os
import socket
from pathlib import Path
from typing import Any, Optional


DEFAULT_SOCKET = str(Path.home() / ".key-rest" / "key-rest.sock")


class Response:
    """A requests.Response-compatible response object."""

    def __init__(self, status_code: int, headers: dict, body: str):
        self.status_code = status_code
        self.headers = headers
        self.text = body
        self.content = body.encode("utf-8")
        self.ok = 200 <= status_code < 300

    def json(self) -> Any:
        return json.loads(self.text)

    def raise_for_status(self) -> None:
        if not self.ok:
            raise Exception(f"HTTP {self.status_code}")


class KeyRestError(Exception):
    """Error from key-rest-daemon."""

    def __init__(self, code: str, message: str):
        self.code = code
        super().__init__(f"[{code}] {message}")


def _send_to_daemon(request: dict, socket_path: str) -> dict:
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    try:
        sock.connect(socket_path)
        data = json.dumps(request) + "\n"
        sock.sendall(data.encode("utf-8"))

        buf = b""
        while b"\n" not in buf:
            chunk = sock.recv(65536)
            if not chunk:
                break
            buf += chunk
    finally:
        sock.close()

    line = buf.split(b"\n", 1)[0]
    return json.loads(line)


def _request(
    method: str,
    url: str,
    headers: Optional[dict] = None,
    params: Optional[dict] = None,
    data: Optional[str] = None,
    json_data: Any = None,
    socket_path: Optional[str] = None,
) -> Response:
    socket_path = socket_path or os.environ.get("KEY_REST_SOCKET", DEFAULT_SOCKET)

    # Append query params to URL
    if params:
        sep = "&" if "?" in url else "?"
        pairs = [f"{k}={v}" for k, v in params.items()]
        url = url + sep + "&".join(pairs)

    req_headers = dict(headers) if headers else {}

    body: Optional[str] = None
    if json_data is not None:
        body = json.dumps(json_data)
        if "Content-Type" not in req_headers:
            req_headers["Content-Type"] = "application/json"
    elif data is not None:
        body = data

    daemon_req = {
        "type": "http",
        "method": method,
        "url": url,
        "headers": req_headers,
        "body": body,
    }

    resp = _send_to_daemon(daemon_req, socket_path)

    if "error" in resp and resp["error"]:
        raise KeyRestError(resp["error"]["code"], resp["error"]["message"])

    return Response(
        status_code=resp.get("status", 200),
        headers=resp.get("headers", {}),
        body=resp.get("body", ""),
    )


def get(url: str, **kwargs: Any) -> Response:
    return _request("GET", url, **kwargs)


def post(url: str, **kwargs: Any) -> Response:
    return _request("POST", url, **kwargs)


def put(url: str, **kwargs: Any) -> Response:
    return _request("PUT", url, **kwargs)


def patch(url: str, **kwargs: Any) -> Response:
    return _request("PATCH", url, **kwargs)


def delete(url: str, **kwargs: Any) -> Response:
    return _request("DELETE", url, **kwargs)
