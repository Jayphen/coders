// Package notify provides OS-native notification functionality.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends an OS-native notification with the given title and message.
// It detects the platform and uses the appropriate notification command:
// - macOS: osascript (native AppleScript)
// - Linux: notify-send (libnotify)
//
// This function is non-blocking and fails silently if the notification
// command is not available on the system.
func Send(title, message string) {
	// Run notification in background goroutine to make it non-blocking
	go func() {
		var cmd *exec.Cmd

		switch runtime.GOOS {
		case "darwin":
			// macOS: Use osascript with AppleScript
			script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapeAppleScript(message), escapeAppleScript(title))
			cmd = exec.Command("osascript", "-e", script)

		case "linux":
			// Linux: Use notify-send
			cmd = exec.Command("notify-send", title, message)

		default:
			// Unsupported platform - fail silently
			return
		}

		// Execute the command and ignore errors (fail silently)
		// This prevents crashes if the notification command is not available
		_ = cmd.Run()
	}()
}

// escapeAppleScript escapes special characters for AppleScript strings.
// This prevents script injection and syntax errors.
func escapeAppleScript(s string) string {
	// Replace backslashes first, then quotes
	result := ""
	for _, ch := range s {
		switch ch {
		case '\\':
			result += "\\\\"
		case '"':
			result += "\\\""
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		case '\t':
			result += "\\t"
		default:
			result += string(ch)
		}
	}
	return result
}
