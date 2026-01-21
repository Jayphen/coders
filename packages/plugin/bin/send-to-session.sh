#!/bin/bash
# Helper script to send messages to coder sessions
# Usage: ./send-to-session.sh SESSION_ID "your message"

SESSION_ID="$1"
MESSAGE="$2"

if [ -z "$SESSION_ID" ] || [ -z "$MESSAGE" ]; then
  echo "Usage: $0 SESSION_ID \"message\""
  exit 1
fi

# Send the message text
tmux send-keys -t "$SESSION_ID" "$MESSAGE"

# Small delay to let TUI process the input
sleep 0.5

# Send Enter to submit
tmux send-keys -t "$SESSION_ID" C-m

echo "✓ Message sent to $SESSION_ID"
