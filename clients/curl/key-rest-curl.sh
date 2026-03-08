#!/usr/bin/env bash
# key-rest-curl: curl wrapper that sends requests through key-rest-daemon
# Usage: key-rest-curl [curl options] <url>

set -euo pipefail

SOCKET="${KEY_REST_SOCKET:-$HOME/.key-rest/key-rest.sock}"

if [ ! -S "$SOCKET" ]; then
  echo "error: key-rest-daemon is not running (socket not found: $SOCKET)" >&2
  exit 1
fi

# Parse curl arguments to build a JSON request
METHOD="GET"
URL=""
BODY=""
declare -A HEADERS

while [[ $# -gt 0 ]]; do
  case "$1" in
    -X|--request)
      METHOD="$2"
      shift 2
      ;;
    -H|--header)
      # Parse "Key: Value"
      header="$2"
      key="${header%%:*}"
      value="${header#*: }"
      HEADERS["$key"]="$value"
      shift 2
      ;;
    -d|--data|--data-raw)
      BODY="$2"
      if [ "$METHOD" = "GET" ]; then
        METHOD="POST"
      fi
      shift 2
      ;;
    -*)
      echo "warning: unsupported curl option: $1 (ignored)" >&2
      shift
      # Skip value if the option takes one
      if [[ $# -gt 0 && ! "$1" =~ ^- && -z "$URL" ]]; then
        shift
      fi
      ;;
    *)
      URL="$1"
      shift
      ;;
  esac
done

if [ -z "$URL" ]; then
  echo "error: no URL specified" >&2
  echo "Usage: key-rest-curl [curl options] <url>" >&2
  exit 1
fi

# Build JSON request
build_json() {
  local method="$1"
  local url="$2"
  local body="$3"

  # Build headers JSON object
  local headers_json="{"
  local first=true
  for key in "${!HEADERS[@]}"; do
    if [ "$first" = true ]; then
      first=false
    else
      headers_json+=","
    fi
    # Escape special characters in values
    local escaped_val
    escaped_val=$(printf '%s' "${HEADERS[$key]}" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()), end="")')
    local escaped_key
    escaped_key=$(printf '%s' "$key" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()), end="")')
    headers_json+="${escaped_key}:${escaped_val}"
  done
  headers_json+="}"

  local body_json="null"
  if [ -n "$body" ]; then
    body_json=$(printf '%s' "$body" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()), end="")')
  fi

  local url_json
  url_json=$(printf '%s' "$url" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()), end="")')
  local method_json
  method_json=$(printf '%s' "$method" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()), end="")')

  printf '{"type":"http","method":%s,"url":%s,"headers":%s,"body":%s}' \
    "$method_json" "$url_json" "$headers_json" "$body_json"
}

REQUEST=$(build_json "$METHOD" "$URL" "$BODY")

# Send request to daemon via Unix socket and read response
RESPONSE=$(printf '%s\n' "$REQUEST" | socat - UNIX-CONNECT:"$SOCKET")

# Check for error
if printf '%s' "$RESPONSE" | python3 -c 'import sys,json; d=json.load(sys.stdin); sys.exit(0 if "error" in d and d["error"] else 1)' 2>/dev/null; then
  printf '%s' "$RESPONSE" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(f"error: [{d[\"error\"][\"code\"]}] {d[\"error\"][\"message\"]}", file=sys.stderr)'
  exit 1
fi

# Output response body
printf '%s' "$RESPONSE" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("body",""))'
