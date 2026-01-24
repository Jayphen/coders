# Proof of Concept: Raw Keystroke Forwarding in TUI Preview Mode

## Overview

This POC implements raw keystroke forwarding in the TUI preview mode, enabling Claude Code's native features like Tab completion, slash command autocomplete, and readline editing (Ctrl+A, Ctrl+E, etc.).

## How It Works

### Architecture

1. **New tmux function**: `SendRawKey()` in `internal/tmux/tmux.go`
   - Forwards individual keystrokes to tmux pane in real-time
   - Maps Bubbletea key events to tmux key names
   - Handles special keys (Tab, Enter, Ctrl combinations, etc.)
   - Uses tmux's `-l` flag for literal characters

2. **Model changes**: `internal/tui/model.go`
   - Added `passthroughMode bool` to track passthrough state
   - Added `lastEscTime time.Time` for double-Esc detection
   - Modified key handling to forward keys directly in passthrough mode

3. **View changes**: `internal/tui/views.go`
   - Yellow border when in passthrough mode
   - `[PASSTHROUGH]` indicator in preview header
   - Different help text showing exit methods

### User Experience

#### Entering Passthrough Mode
1. Press `Tab` to focus the preview pane (cyan border)
2. Press `Shift+Tab` to enable passthrough mode (border turns yellow)

#### Using Passthrough Mode
- **All keystrokes** are forwarded directly to the Claude Code session
- **Tab completion** works natively (Claude Code's autocomplete)
- **Ctrl+A, Ctrl+E, etc.** work for readline editing
- **Up/Down arrows** navigate command history
- **Slash commands** get autocomplete (`/commit`, `/help`, etc.)

#### Exiting Passthrough Mode
Two methods:
1. Press `Shift+Tab` again (toggle)
2. Press `Esc` twice quickly (within 500ms)

Note: `Ctrl+C` still quits the TUI entirely, even in passthrough mode.

### Visual Indicators

- **Normal preview focus**: Cyan border, shows "Send: " input field
- **Passthrough mode**: Yellow border, shows `[PASSTHROUGH]` tag and exit instructions
- **Status messages**: Appear at bottom when entering/exiting passthrough mode

## Implementation Details

### Key Mapping

The `SendRawKey()` function maps Bubbletea key strings to tmux key names:

```go
specialKeys := map[string]string{
    "enter":     "Enter",
    "tab":       "Tab",
    "esc":       "Escape",
    "backspace": "BSpace",
    "delete":    "DC",
    "up":        "Up",
    "down":      "Down",
    // ... etc
}
```

Ctrl combinations are handled with tmux's `C-` notation:
- `ctrl+a` → `C-a` (tmux notation for Ctrl+A)

### Double-Esc Detection

The implementation uses a simple time-based detection:
```go
if msg.String() == "esc" {
    now := time.Now()
    if !m.lastEscTime.IsZero() && now.Sub(m.lastEscTime) < 500*time.Millisecond {
        // Double-Esc detected - exit passthrough mode
        m.passthroughMode = false
    }
    m.lastEscTime = now
}
```

### Legacy Mode Preserved

The original buffered input mode is still available:
- When preview is focused but passthrough is NOT enabled
- Type text, press Enter to send
- Tab still unfocuses the preview

## Testing

To test the POC:

1. Build the binary:
   ```bash
   cd packages/go
   make build
   ```

2. Spawn a Claude session:
   ```bash
   ./bin/coders spawn claude --task "test task"
   ```

3. Launch the TUI:
   ```bash
   ./bin/coders tui
   ```

4. Test passthrough mode:
   - Press `Tab` to focus preview
   - Press `Shift+Tab` to enable passthrough
   - Try typing `/` and see slash command autocomplete
   - Press `Tab` to trigger file path autocomplete
   - Press `Esc Esc` to exit passthrough mode

## Tradeoffs

### Pros
- ✅ Native Claude Code features work (Tab completion, autocomplete, readline)
- ✅ Real-time keystroke forwarding
- ✅ User sees autocomplete suggestions immediately
- ✅ More powerful interaction model

### Cons
- ❌ Loses custom input field styling when in passthrough mode
- ❌ Focus/escape handling requires careful thought
- ❌ More like "inline attach" than "send a message"
- ❌ User needs to learn new keybindings (Shift+Tab, Esc Esc)

## Future Improvements

1. **Better escape mechanism**: Could use Ctrl+] (like tmux) instead of double-Esc
2. **Auto-passthrough**: Enter passthrough mode automatically on Tab focus
3. **Visual feedback**: Show which keys are being forwarded (debugging mode)
4. **Session state**: Remember passthrough preference per session
5. **Hybrid mode**: Allow switching between buffered and passthrough easily

## Files Changed

- `packages/go/internal/tmux/tmux.go` - Added `SendRawKey()` function
- `packages/go/internal/tui/model.go` - Added passthrough mode logic
- `packages/go/internal/tui/views.go` - Added visual indicators

## Related Issue

Implements: beads-jbn (Raw keystroke forwarding in TUI preview mode)
