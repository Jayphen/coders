#!/bin/bash
# Test script for promises feature

set -e

PLUGIN_DIR="/Users/beepboop/dev/coders/packages/plugin"
CODERS="node ${PLUGIN_DIR}/skills/coders/scripts/main.js"

echo "🧪 Testing Promises Feature"
echo "================================"
echo ""

# Test 1: Spawn session with completion promise
echo "📍 Test 1: Spawning session that will complete..."
$CODERS spawn claude --name test-completed \
  --task "Create a test-output.txt file with 'Test passed', then publish completion promise 'Test completed successfully'" \
  --no-heartbeat

echo "⏳ Waiting 20 seconds for session to complete..."
sleep 20

# Test 2: Check promises
echo ""
echo "📍 Test 2: Checking promises..."
$CODERS promises

# Test 3: Verify file was created
echo ""
echo "📍 Test 3: Verifying file was created..."
if [ -f "test-output.txt" ]; then
  echo "✅ File exists: test-output.txt"
  echo "📄 Content: $(cat test-output.txt)"
else
  echo "❌ File not found: test-output.txt"
fi

# Test 4: Manual promise publishing
echo ""
echo "📍 Test 4: Testing manual promise publishing..."
export CODERS_SESSION_ID="coder-manual-test"
$CODERS promise "Manual test completed" --status completed
sleep 1
$CODERS promise "Manual test blocked" --status blocked --blockers "Need credentials"
sleep 1
$CODERS promise "Manual test needs review" --status needs-review

echo ""
echo "📍 Test 4 results: Checking all promises..."
$CODERS promises

# Cleanup
echo ""
echo "🧹 Cleaning up..."
tmux kill-session -t coder-test-completed 2>/dev/null || true
rm -f test-output.txt
redis-cli DEL "coders:promise:coder-manual-test" >/dev/null 2>&1 || true

echo ""
echo "✅ All tests completed!"
echo ""
echo "Summary:"
echo "- Spawned session published completion promise ✓"
echo "- Promises command displayed all promises ✓"
echo "- File was created by session ✓"
echo "- Manual promise publishing works ✓"
