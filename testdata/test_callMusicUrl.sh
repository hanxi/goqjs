#!/bin/bash
# Test script for callMusicUrl via goqjs
# This script:
# 1. Builds goqjs
# 2. Loads the lx_prelude.js + 洛雪音乐源.js
# 3. Waits for 'inited' event to get available sources
# 4. Sends a callMusicUrl request

set -e

GOQJS_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT_PATH="/Users/hanxi/toy/mimusic/data/plugins_data/lxmusic/scripts/洛雪音乐源.js"
GOQJS_BIN="$GOQJS_DIR/goqjs"
OUTPUT_FILE="$GOQJS_DIR/test_output.log"

echo "=== Building goqjs ==="
cd "$GOQJS_DIR"
go build -o "$GOQJS_BIN" ./cmd/goqjs/

echo "=== Starting goqjs and loading script ==="

# Use a timeout wrapper to avoid hanging forever
# Step 1: Load the script and capture inited event to see sources
# Step 2: Send callMusicUrl

# Create a FIFO for communication
FIFO=$(mktemp -u)
mkfifo "$FIFO"

# Start goqjs in background, reading from FIFO
"$GOQJS_BIN" < "$FIFO" > "$OUTPUT_FILE" 2>&1 &
GOQJS_PID=$!

# Open FIFO for writing
exec 3>"$FIFO"

echo "=== Loading 洛雪音乐源.js ==="
# Send eval_file to load the script
echo '{"id":"1","type":"eval_file","path":"'"$SCRIPT_PATH"'"}' >&3

# Wait a bit for script to initialize
sleep 3

echo "=== Checking output so far ==="
cat "$OUTPUT_FILE"

echo ""
echo "=== Sending callMusicUrl request ==="
# Try with 'kw' source (酷我), a common source in lx scripts
# Use a simple songInfo
echo '{"id":"2","type":"callMusicUrl","source":"wy","songInfo":"{\"name\":\"海屿你\",\"singer\":\"马也_Crabbit\",\"songmid\":\"1973665667\",\"songId\":\"1973665667\"}","quality":"320k"}' >&3

# Wait for response
sleep 10

echo ""
echo "=== Final output ==="
cat "$OUTPUT_FILE"

# Cleanup
exec 3>&-
kill $GOQJS_PID 2>/dev/null || true
rm -f "$FIFO" "$GOQJS_BIN"

echo ""
echo "=== Test complete ==="
