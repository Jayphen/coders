package notify

import (
	"runtime"
	"testing"
	"time"
)

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `Hello World`,
			expected: `Hello World`,
		},
		{
			input:    `Hello "World"`,
			expected: `Hello \"World\"`,
		},
		{
			input:    `Hello\nWorld`,
			expected: `Hello\\nWorld`,
		},
		{
			input:    `C:\Users\test`,
			expected: `C:\\Users\\test`,
		},
		{
			input:    `Line1\nLine2\tTabbed`,
			expected: `Line1\\nLine2\\tTabbed`,
		},
		{
			input:    `Quote: " Backslash: \`,
			expected: `Quote: \" Backslash: \\`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeAppleScript(tt.input)
			if result != tt.expected {
				t.Errorf("escapeAppleScript(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSend(t *testing.T) {
	// Test that Send doesn't panic and returns immediately (non-blocking)
	t.Run("non-blocking", func(t *testing.T) {
		start := time.Now()
		Send("Test Title", "Test Message")
		elapsed := time.Since(start)

		// Should return almost immediately (< 10ms) since it's non-blocking
		if elapsed > 10*time.Millisecond {
			t.Errorf("Send() took %v, expected < 10ms (non-blocking)", elapsed)
		}
	})

	// Test with various message contents
	t.Run("special characters", func(t *testing.T) {
		// Should not panic with special characters
		Send("Test", `Message with "quotes" and \backslashes\`)
		Send("Test", "Message\nwith\nnewlines")
		Send("Test", "Message\twith\ttabs")
		// Give goroutine time to execute
		time.Sleep(100 * time.Millisecond)
	})

	// Test empty strings
	t.Run("empty strings", func(t *testing.T) {
		Send("", "")
		Send("Title", "")
		Send("", "Message")
		// Give goroutine time to execute
		time.Sleep(100 * time.Millisecond)
	})

	// Platform-specific test
	t.Run("platform detection", func(t *testing.T) {
		switch runtime.GOOS {
		case "darwin":
			t.Log("Testing on macOS - should use osascript")
		case "linux":
			t.Log("Testing on Linux - should use notify-send")
		default:
			t.Logf("Testing on unsupported platform: %s - should fail silently", runtime.GOOS)
		}

		Send("Platform Test", "Testing on "+runtime.GOOS)
		// Give goroutine time to execute
		time.Sleep(100 * time.Millisecond)
	})
}

func TestSendConcurrent(t *testing.T) {
	// Test that multiple concurrent sends don't cause issues
	t.Run("concurrent", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			go Send("Concurrent Test", "Test message")
		}
		// Give goroutines time to execute
		time.Sleep(200 * time.Millisecond)
	})
}

// BenchmarkSend measures the overhead of the Send function
func BenchmarkSend(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Send("Benchmark", "Test message")
	}
	// Give goroutines time to finish
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkEscapeAppleScript measures the performance of string escaping
func BenchmarkEscapeAppleScript(b *testing.B) {
	testStrings := []string{
		"Simple message",
		`Message with "quotes"`,
		"Message\nwith\nnewlines\tand\ttabs",
		`Complex: "quotes" \backslashes\ \n newlines`,
	}

	for _, s := range testStrings {
		b.Run(s, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				escapeAppleScript(s)
			}
		})
	}
}
