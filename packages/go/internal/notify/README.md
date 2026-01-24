# notify - OS-Native Notifications

This package provides cross-platform OS-native notification functionality for the coders system.

## Features

- **Platform Detection**: Automatically detects the OS using `runtime.GOOS`
- **macOS Support**: Uses `osascript` with AppleScript for native notifications
- **Linux Support**: Uses `notify-send` (libnotify) for desktop notifications
- **Non-Blocking**: All notifications are sent asynchronously in a goroutine
- **Fail-Silent**: If notification commands are not available, the package fails gracefully without errors

## Usage

```go
import "github.com/Jayphen/coders/internal/notify"

// Send a notification
notify.Send("Title", "Message content")

// Notifications are non-blocking and return immediately
fmt.Println("This prints immediately, notification sent in background")
```

## Platform Support

### macOS (darwin)
- Uses `osascript -e 'display notification "message" with title "title"'`
- Native system notification center
- No additional dependencies required

### Linux
- Uses `notify-send title message`
- Requires `libnotify-bin` package (usually pre-installed on desktop Linux)
- Works with most desktop environments (GNOME, KDE, XFCE, etc.)

### Other Platforms
- Unsupported platforms fail silently (no notification sent, no error)
- Safe to call on any platform

## Implementation Details

- Special characters in messages are properly escaped for AppleScript
- Notifications are fire-and-forget (no delivery confirmation)
- Command failures are silently ignored to prevent crashes
- Each notification runs in its own goroutine for maximum concurrency

## Testing

Run the tests:
```bash
make test
# or
go test ./internal/notify/...
```

Run the manual test program:
```bash
go run cmd/test-notification/main.go
```
