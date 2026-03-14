#!/usr/bin/env bash
# System test: starts test-server and key-rest daemon, registers all credentials,
# and tests all 26 services through key-rest-curl.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'cleanup' EXIT

PASS="system-test-passphrase"
PORT=""
TEST_SERVER_PID=""
DAEMON_PID=""
PASSED=0
FAILED=0
TOTAL=0

cleanup() {
  [ -n "$DAEMON_PID" ] && kill "$DAEMON_PID" 2>/dev/null && wait "$DAEMON_PID" 2>/dev/null || true
  [ -n "$TEST_SERVER_PID" ] && kill "$TEST_SERVER_PID" 2>/dev/null && wait "$TEST_SERVER_PID" 2>/dev/null || true
  rm -rf "$WORK"
}

# --- Build ---

echo "=== Building ==="
(cd "$ROOT" && go build -o "$WORK/test-server" ./test-server/)
(cd "$ROOT" && go build -o "$WORK/key-rest" ./cmd/key-rest/)

# --- Find free port ---

PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()')

# --- Start test-server ---

echo "=== Starting test-server on port $PORT ==="
CERT="$WORK/cert.pem"
KEY="$WORK/key.pem"
"$WORK/test-server" -port "$PORT" -cert "$CERT" -key "$KEY" -gen-cert \
  >"$WORK/test-server.out" 2>"$WORK/test-server.err" &
TEST_SERVER_PID=$!

# Wait for credentials output
for i in $(seq 1 50); do
  if grep -q "========================" "$WORK/test-server.out" 2>/dev/null; then
    break
  fi
  sleep 0.1
done

if ! grep -q "========================" "$WORK/test-server.out" 2>/dev/null; then
  echo "FATAL: test-server did not produce credentials"
  cat "$WORK/test-server.err"
  exit 1
fi

# --- Parse credentials ---

declare -A CREDS
while IFS= read -r line; do
  # Format: "  service label  value"
  fields=($line)
  if [ "${#fields[@]}" -ge 3 ]; then
    key="${fields[0]}/${fields[1]}"
    val="${fields[${#fields[@]}-1]}"
    CREDS["$key"]="$val"
  fi
done < <(sed -n '/=== Test Credentials ===/,/========================/{ /===/d; p; }' "$WORK/test-server.out")

echo "Parsed ${#CREDS[@]} credentials"

# --- Start daemon ---

echo "=== Starting daemon ==="
export KEY_REST_DIR="$WORK/keyrest-data"
export KEY_REST_SOCKET="$KEY_REST_DIR/key-rest.sock"
mkdir -p "$KEY_REST_DIR"

# Register all keys (daemon not running, passphrase piped each time)
echo "=== Registering keys ==="

add_key() {
  local uri="$1" url_prefix="$2"
  shift 2
  printf '%s\n%s\n' "$PASS" "${CREDS[$uri]}" | \
    "$WORK/key-rest" add "$@" "user1/$uri" "$url_prefix" 2>/dev/null
}

BASE="https://localhost:$PORT"

# Bearer token services
add_key "openai/api-key"     "$BASE/openai/"
add_key "mistral/api-key"    "$BASE/mistral/"
add_key "groq/api-key"       "$BASE/groq/"
add_key "xai/api-key"        "$BASE/xai/"
add_key "perplexity/api-key" "$BASE/perplexity/"
add_key "deepseek/api-key"   "$BASE/deepseek/"
add_key "openrouter/api-key" "$BASE/openrouter/"
add_key "github/token"       "$BASE/github/"
add_key "matrix/access-token" "$BASE/matrix/"
add_key "slack/bot-token"    "$BASE/slack/"
add_key "sentry/auth-token"  "$BASE/sentry/"
add_key "line/channel-access-token" "$BASE/line/"
add_key "notion/api-key"     "$BASE/notion/"
add_key "discord/bot-token"  "$BASE/discord/"
add_key "linear/api-key"     "$BASE/linear/"

# Custom header services
add_key "anthropic/api-key"  "$BASE/anthropic/"
add_key "exa/api-key"        "$BASE/exa/"
add_key "brave/api-key"      "$BASE/brave/"
add_key "gitlab/token"       "$BASE/gitlab/"
add_key "bing/api-key"       "$BASE/bing/"

# Query parameter services
add_key "gemini/api-key"       "$BASE/gemini/"       --allow-url
add_key "google-search/api-key" "$BASE/google-search/" --allow-url
add_key "trello/api-key"       "$BASE/trello/"       --allow-url
add_key "trello/token"         "$BASE/trello/"       --allow-url

# Body field services
add_key "tavily/api-key"       "$BASE/tavily/"       --allow-body

# Path embedding services
add_key "telegram/bot-token"   "$BASE/telegram/"     --allow-url

# Basic auth
add_key "atlassian/email"      "$BASE/atlassian/"
add_key "atlassian/token"      "$BASE/atlassian/"

# Trust test-server cert (must be before daemon start so child inherits it)
export SSL_CERT_FILE="$CERT"

echo "=== Starting key-rest daemon ==="
printf '%s\n' "$PASS" | "$WORK/key-rest" start 2>/dev/null
sleep 3

# Verify daemon is running
if ! "$WORK/key-rest" status 2>/dev/null | grep -q running; then
  echo "FATAL: daemon failed to start"
  exit 1
fi

# --- key-rest-curl wrapper ---

CURL="$ROOT/clients/curl/key-rest-curl"

run_test() {
  local name="$1"
  shift
  TOTAL=$((TOTAL + 1))

  local output
  if output=$("$@" 2>&1); then
    # Check for non-empty response (not an error)
    if [ -n "$output" ]; then
      echo "  PASS  $name"
      PASSED=$((PASSED + 1))
    else
      echo "  FAIL  $name (empty response)"
      FAILED=$((FAILED + 1))
    fi
  else
    echo "  FAIL  $name"
    echo "        $output" | head -3
    FAILED=$((FAILED + 1))
  fi
}

echo ""
echo "=== Testing all 26 services ==="
echo ""

# --- Bearer token services (13) ---

run_test "openai" "$CURL" "$BASE/openai/v1/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/openai/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o"}'

run_test "mistral" "$CURL" "$BASE/mistral/v1/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/mistral/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"mistral-large-latest"}'

run_test "groq" "$CURL" "$BASE/groq/openai/v1/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/groq/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-3.3-70b-versatile"}'

run_test "xai" "$CURL" "$BASE/xai/v1/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/xai/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"grok-3"}'

run_test "perplexity" "$CURL" "$BASE/perplexity/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/perplexity/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"sonar"}'

run_test "deepseek" "$CURL" "$BASE/deepseek/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/deepseek/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat"}'

run_test "openrouter" "$CURL" "$BASE/openrouter/api/v1/chat/completions" \
  -H "Authorization: Bearer key-rest://user1/openrouter/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"anthropic/claude-sonnet-4-20250514"}'

run_test "github" "$CURL" "$BASE/github/user/repos" \
  -H "Authorization: Bearer key-rest://user1/github/token"

run_test "matrix" "$CURL" "$BASE/matrix/_matrix/client/v3/rooms/ROOM/send/m.room.message" \
  -H "Authorization: Bearer key-rest://user1/matrix/access-token" \
  -H "Content-Type: application/json" \
  -d '{"msgtype":"m.text","body":"test"}'

run_test "slack" "$CURL" "$BASE/slack/api/chat.postMessage" \
  -H "Authorization: Bearer key-rest://user1/slack/bot-token" \
  -H "Content-Type: application/json" \
  -d '{"channel":"C01234567","text":"test"}'

run_test "sentry" "$CURL" "$BASE/sentry/api/0/projects/" \
  -H "Authorization: Bearer key-rest://user1/sentry/auth-token"

run_test "line" "$CURL" "$BASE/line/v2/bot/message/push" \
  -H "Authorization: Bearer key-rest://user1/line/channel-access-token" \
  -H "Content-Type: application/json" \
  -d '{"to":"U123","messages":[{"type":"text","text":"test"}]}'

run_test "notion" "$CURL" "$BASE/notion/v1/databases/DB/query" \
  -H "Authorization: Bearer key-rest://user1/notion/api-key" \
  -H "Content-Type: application/json" \
  -d '{}'

# --- Custom header services (5) ---

run_test "anthropic" "$CURL" "$BASE/anthropic/v1/messages" \
  -H "X-Api-Key: key-rest://user1/anthropic/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","max_tokens":1}'

run_test "exa" "$CURL" "$BASE/exa/search" \
  -H "X-Api-Key: key-rest://user1/exa/api-key" \
  -H "Content-Type: application/json" \
  -d '{"query":"test","type":"neural"}'

run_test "brave" "$CURL" "$BASE/brave/res/v1/web/search?q=test" \
  -H "X-Subscription-Token: key-rest://user1/brave/api-key"

run_test "gitlab" "$CURL" "$BASE/gitlab/api/v4/projects" \
  -H "Private-Token: key-rest://user1/gitlab/token"

run_test "bing" "$CURL" "$BASE/bing/v7.0/search?q=test" \
  -H "Ocp-Apim-Subscription-Key: key-rest://user1/bing/api-key"

# --- Prefix/raw token services (2) ---

run_test "discord" "$CURL" "$BASE/discord/api/v10/channels/CH/messages" \
  -H "Authorization: Bot key-rest://user1/discord/bot-token" \
  -H "Content-Type: application/json" \
  -d '{"content":"test"}'

run_test "linear" "$CURL" "$BASE/linear/graphql" \
  -H "Authorization: key-rest://user1/linear/api-key" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ issues { nodes { id title } } }"}'

# --- Query parameter services (3) ---

run_test "gemini" "$CURL" "$BASE/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"test"}]}]}'

run_test "google-search" "$CURL" "$BASE/google-search/customsearch/v1?key=key-rest://user1/google-search/api-key"

run_test "trello" "$CURL" "$BASE/trello/1/members/me/boards?key=key-rest://user1/trello/api-key\&token=key-rest://user1/trello/token"

# --- Body field service (1) ---

run_test "tavily" "$CURL" "$BASE/tavily/search" \
  -H "Content-Type: application/json" \
  -d '{"api_key":"key-rest://user1/tavily/api-key","query":"test","search_depth":"basic"}'

# --- Path embedding service (1) ---

run_test "telegram" "$CURL" "$BASE/telegram/bot{{key-rest://user1/telegram/bot-token}}/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{"chat_id":123456789,"text":"test"}'

# --- Basic auth service (1) ---

run_test "atlassian" "$CURL" "$BASE/atlassian/2.0/repositories/ws/repo/pullrequests" \
  -H 'Authorization: Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}'

# --- Summary ---

echo ""
echo "=== Results: $PASSED/$TOTAL passed, $FAILED failed ==="

if [ "$FAILED" -gt 0 ]; then
  exit 1
fi
