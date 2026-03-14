#!/usr/bin/env python3
"""System test: starts test-server and key-rest daemon, registers all credentials,
and tests all 26 services through the Python key_rest.requests client."""

import json
import os
import re
import signal
import socket
import subprocess
import sys
import tempfile
import time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "clients", "python"))

from key_rest.requests import get, post  # noqa: E402


def find_free_port():
    s = socket.socket()
    s.bind(("", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def parse_credentials(output):
    creds = {}
    in_creds = False
    for line in output.splitlines():
        if line.startswith("=== Test Credentials"):
            in_creds = True
            continue
        if line.startswith("===="):
            break
        if in_creds:
            fields = line.split()
            if len(fields) >= 3:
                key = f"{fields[0]}/{fields[1]}"
                val = fields[-1]
                creds[key] = val
    return creds


def main():
    work = tempfile.mkdtemp()
    test_server_proc = None
    daemon_started = False
    passed = 0
    failed = 0
    total = 0

    try:
        # --- Build ---
        print("=== Building ===")
        subprocess.run(
            ["go", "build", "-o", os.path.join(work, "test-server"), "./test-server/"],
            cwd=ROOT, check=True, capture_output=True,
        )
        subprocess.run(
            ["go", "build", "-o", os.path.join(work, "key-rest"), "./cmd/key-rest/"],
            cwd=ROOT, check=True, capture_output=True,
        )

        key_rest = os.path.join(work, "key-rest")
        test_server = os.path.join(work, "test-server")
        port = find_free_port()
        cert = os.path.join(work, "cert.pem")
        key_file = os.path.join(work, "key.pem")

        # --- Start test-server ---
        print(f"=== Starting test-server on port {port} ===")
        test_server_proc = subprocess.Popen(
            [test_server, "-port", str(port), "-cert", cert, "-key", key_file, "-gen-cert"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )

        # Wait for credentials
        output = ""
        deadline = time.time() + 10
        while time.time() < deadline:
            line = test_server_proc.stdout.readline().decode()
            if not line:
                break
            output += line
            if "========================" in line and "Test Credentials" not in line:
                break

        if "========================" not in output:
            print("FATAL: test-server did not produce credentials")
            sys.exit(1)

        creds = parse_credentials(output)
        print(f"Parsed {len(creds)} credentials")

        # --- Register keys ---
        print("=== Registering keys ===")
        env = os.environ.copy()
        env["KEY_REST_DIR"] = os.path.join(work, "keyrest-data")
        env["KEY_REST_SOCKET"] = os.path.join(env["KEY_REST_DIR"], "key-rest.sock")
        os.makedirs(env["KEY_REST_DIR"], exist_ok=True)

        passphrase = "system-test-passphrase"
        base = f"https://localhost:{port}"

        def add_key(uri, url_prefix, *extra_args):
            inp = f"{passphrase}\n{creds[uri]}\n".encode()
            subprocess.run(
                [key_rest, "add"] + list(extra_args) + [f"user1/{uri}", url_prefix],
                input=inp, env=env, capture_output=True, check=True,
            )

        # Bearer token services
        for uri, path in [
            ("openai/api-key", "/openai/"), ("mistral/api-key", "/mistral/"),
            ("groq/api-key", "/groq/"), ("xai/api-key", "/xai/"),
            ("perplexity/api-key", "/perplexity/"), ("deepseek/api-key", "/deepseek/"),
            ("openrouter/api-key", "/openrouter/"), ("github/token", "/github/"),
            ("matrix/access-token", "/matrix/"), ("slack/bot-token", "/slack/"),
            ("sentry/auth-token", "/sentry/"), ("line/channel-access-token", "/line/"),
            ("notion/api-key", "/notion/"), ("discord/bot-token", "/discord/"),
            ("linear/api-key", "/linear/"),
        ]:
            add_key(uri, base + path)

        # Custom header services
        for uri, path in [
            ("anthropic/api-key", "/anthropic/"), ("exa/api-key", "/exa/"),
            ("brave/api-key", "/brave/"), ("gitlab/token", "/gitlab/"),
            ("bing/api-key", "/bing/"),
        ]:
            add_key(uri, base + path)

        # Query parameter services
        for uri, path in [
            ("gemini/api-key", "/gemini/"), ("google-search/api-key", "/google-search/"),
            ("trello/api-key", "/trello/"), ("trello/token", "/trello/"),
        ]:
            add_key(uri, base + path, "--allow-url")

        # Body field services
        add_key("tavily/api-key", base + "/tavily/", "--allow-body")

        # Path embedding services
        add_key("telegram/bot-token", base + "/telegram/", "--allow-url")

        # Basic auth
        add_key("atlassian/email", base + "/atlassian/")
        add_key("atlassian/token", base + "/atlassian/")

        # Trust test-server cert (must be before daemon start)
        env["SSL_CERT_FILE"] = cert

        # --- Start daemon ---
        print("=== Starting key-rest daemon ===")
        subprocess.run(
            [key_rest, "start"],
            input=f"{passphrase}\n".encode(),
            env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True,
        )
        daemon_started = True
        time.sleep(3)

        # Verify daemon is running
        result = subprocess.run(
            [key_rest, "status"], env=env, capture_output=True, text=True,
        )
        if "running" not in result.stdout:
            print("FATAL: daemon failed to start")
            sys.exit(1)

        socket_path = env["KEY_REST_SOCKET"]

        # --- Test helpers ---
        def run_test(name, method, url, headers=None, body=None):
            nonlocal total, passed, failed
            total += 1
            try:
                if method == "GET":
                    resp = get(url, headers=headers, socket_path=socket_path)
                else:
                    resp = post(url, headers=headers, data=body, socket_path=socket_path)
                if resp.status_code == 200:
                    # Verify valid JSON
                    resp.json()
                    print(f"  PASS  {name}")
                    passed += 1
                else:
                    print(f"  FAIL  {name} (status {resp.status_code})")
                    failed += 1
            except Exception as e:
                print(f"  FAIL  {name}")
                print(f"        {e}")
                failed += 1

        print()
        print("=== Testing all 26 services ===")
        print()

        # --- Bearer token services (13) ---
        run_test("openai", "POST", f"{base}/openai/v1/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/openai/api-key", "Content-Type": "application/json"},
                 '{"model":"gpt-4o"}')
        run_test("mistral", "POST", f"{base}/mistral/v1/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/mistral/api-key", "Content-Type": "application/json"},
                 '{"model":"mistral-large-latest"}')
        run_test("groq", "POST", f"{base}/groq/openai/v1/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/groq/api-key", "Content-Type": "application/json"},
                 '{"model":"llama-3.3-70b-versatile"}')
        run_test("xai", "POST", f"{base}/xai/v1/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/xai/api-key", "Content-Type": "application/json"},
                 '{"model":"grok-3"}')
        run_test("perplexity", "POST", f"{base}/perplexity/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/perplexity/api-key", "Content-Type": "application/json"},
                 '{"model":"sonar"}')
        run_test("deepseek", "POST", f"{base}/deepseek/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/deepseek/api-key", "Content-Type": "application/json"},
                 '{"model":"deepseek-chat"}')
        run_test("openrouter", "POST", f"{base}/openrouter/api/v1/chat/completions",
                 {"Authorization": "Bearer key-rest://user1/openrouter/api-key", "Content-Type": "application/json"},
                 '{"model":"anthropic/claude-sonnet-4-20250514"}')
        run_test("github", "GET", f"{base}/github/user/repos",
                 {"Authorization": "Bearer key-rest://user1/github/token"})
        run_test("matrix", "POST", f"{base}/matrix/_matrix/client/v3/rooms/ROOM/send/m.room.message",
                 {"Authorization": "Bearer key-rest://user1/matrix/access-token", "Content-Type": "application/json"},
                 '{"msgtype":"m.text","body":"test"}')
        run_test("slack", "POST", f"{base}/slack/api/chat.postMessage",
                 {"Authorization": "Bearer key-rest://user1/slack/bot-token", "Content-Type": "application/json"},
                 '{"channel":"C01234567","text":"test"}')
        run_test("sentry", "GET", f"{base}/sentry/api/0/projects/",
                 {"Authorization": "Bearer key-rest://user1/sentry/auth-token"})
        run_test("line", "POST", f"{base}/line/v2/bot/message/push",
                 {"Authorization": "Bearer key-rest://user1/line/channel-access-token", "Content-Type": "application/json"},
                 '{"to":"U123","messages":[{"type":"text","text":"test"}]}')
        run_test("notion", "POST", f"{base}/notion/v1/databases/DB/query",
                 {"Authorization": "Bearer key-rest://user1/notion/api-key", "Content-Type": "application/json"},
                 '{}')

        # --- Custom header services (5) ---
        run_test("anthropic", "POST", f"{base}/anthropic/v1/messages",
                 {"X-Api-Key": "key-rest://user1/anthropic/api-key", "Content-Type": "application/json"},
                 '{"model":"claude-sonnet-4-20250514","max_tokens":1}')
        run_test("exa", "POST", f"{base}/exa/search",
                 {"X-Api-Key": "key-rest://user1/exa/api-key", "Content-Type": "application/json"},
                 '{"query":"test","type":"neural"}')
        run_test("brave", "GET", f"{base}/brave/res/v1/web/search?q=test",
                 {"X-Subscription-Token": "key-rest://user1/brave/api-key"})
        run_test("gitlab", "GET", f"{base}/gitlab/api/v4/projects",
                 {"Private-Token": "key-rest://user1/gitlab/token"})
        run_test("bing", "GET", f"{base}/bing/v7.0/search?q=test",
                 {"Ocp-Apim-Subscription-Key": "key-rest://user1/bing/api-key"})

        # --- Prefix/raw token services (2) ---
        run_test("discord", "POST", f"{base}/discord/api/v10/channels/CH/messages",
                 {"Authorization": "Bot key-rest://user1/discord/bot-token", "Content-Type": "application/json"},
                 '{"content":"test"}')
        run_test("linear", "POST", f"{base}/linear/graphql",
                 {"Authorization": "key-rest://user1/linear/api-key", "Content-Type": "application/json"},
                 '{"query":"{ issues { nodes { id title } } }"}')

        # --- Query parameter services (3) ---
        run_test("gemini", "POST",
                 f"{base}/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key",
                 {"Content-Type": "application/json"},
                 '{"contents":[{"parts":[{"text":"test"}]}]}')
        run_test("google-search", "GET",
                 f"{base}/google-search/customsearch/v1?key=key-rest://user1/google-search/api-key")
        run_test("trello", "GET",
                 f"{base}/trello/1/members/me/boards?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token")

        # --- Body field service (1) ---
        run_test("tavily", "POST", f"{base}/tavily/search",
                 {"Content-Type": "application/json"},
                 '{"api_key":"key-rest://user1/tavily/api-key","query":"test","search_depth":"basic"}')

        # --- Path embedding service (1) ---
        run_test("telegram", "POST",
                 f"{base}/telegram/bot{{{{key-rest://user1/telegram/bot-token}}}}/sendMessage",
                 {"Content-Type": "application/json"},
                 '{"chat_id":123456789,"text":"test"}')

        # --- Basic auth service (1) ---
        run_test("atlassian", "GET",
                 f"{base}/atlassian/2.0/repositories/ws/repo/pullrequests",
                 {"Authorization": 'Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}'})

        # --- Response masking test ---
        print()
        print("=== Testing response masking ===")
        print()
        total += 1
        echo_key_value = creds["openai/api-key"]
        # Register a key for the echo endpoint
        subprocess.run(
            [key_rest, "add", "user1/echo/key", base + "/echo/"],
            input=f"{passphrase}\n{echo_key_value}\n".encode(),
            env=env, capture_output=True, check=True,
        )
        try:
            resp = get(
                f"{base}/echo/test",
                headers={"Authorization": "Bearer key-rest://user1/echo/key"},
                socket_path=socket_path,
            )
            body = resp.text
            if echo_key_value in body:
                print("  FAIL  response-masking (credential leaked in response body)")
                failed += 1
            elif "key-rest://" not in body:
                print("  FAIL  response-masking (credential not reverse-substituted)")
                failed += 1
            else:
                print("  PASS  response-masking")
                passed += 1
        except Exception as e:
            print(f"  FAIL  response-masking")
            print(f"        {e}")
            failed += 1

        # --- Summary ---
        print()
        print(f"=== Results: {passed}/{total} passed, {failed} failed ===")

        if failed > 0:
            sys.exit(1)

    finally:
        # Cleanup
        if daemon_started:
            subprocess.run([key_rest, "stop"], env=env, capture_output=True)
        if test_server_proc:
            test_server_proc.terminate()
            test_server_proc.wait()
        import shutil
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
