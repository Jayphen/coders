package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/notify"
	"github.com/Jayphen/coders/internal/tmux"
)

func newTestNotifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "test-notify",
		Short:  "Test notification systems (OS-native and tmux)",
		Hidden: true, // Hide from main help (dev/test command)
		RunE:   runTestNotify,
	}
	return cmd
}

func runTestNotify(cmd *cobra.Command, args []string) error {
	fmt.Println("\n\033[34m🔔 Testing Notification Systems\033[0m")

	// Test 1: OS-native notification
	fmt.Println("\n📱 Sending OS notification...")
	notify.Send("Coders Notification Test", "This is a test OS notification from the coders system")
	fmt.Println("   ✅ OS notification sent (check your system notifications)")
	time.Sleep(2 * time.Second)

	// Test 2: tmux display-message notification (if in tmux)
	fmt.Println("\n💬 Sending tmux notification...")
	if !tmux.IsInsideTmux() {
		fmt.Println("   ⚠️  Not inside tmux - skipping tmux notification test")
		fmt.Println("   💡 Run this test from within a tmux session to test tmux notifications")
	} else {
		sessionName, err := tmux.GetCurrentSession()
		if err != nil {
			fmt.Printf("   ❌ Failed to get current session: %v\n", err)
		} else {
			fmt.Printf("   📍 Current tmux session: %s\n", sessionName)
			err = tmux.SendDisplayMessage(sessionName, "Coders: Test notification from notification system")
			if err != nil {
				fmt.Printf("   ❌ Failed to send tmux notification: %v\n", err)
			} else {
				fmt.Println("   ✅ tmux notification sent (check your tmux status bar)")
			}
		}
	}

	// Test 3: Simulate loop completion notification
	fmt.Println("\n🔄 Simulating loop completion notification...")
	notify.Send("Loop completed", "Test loop finished successfully with 3 tasks")
	fmt.Println("   ✅ Loop completion OS notification sent")

	if tmux.IsInsideTmux() {
		sessionName, _ := tmux.GetCurrentSession()
		_ = tmux.SendDisplayMessage(sessionName, "Loop test-loop completed: 3 tasks")
		fmt.Println("   ✅ Loop completion tmux notification sent")
	}

	time.Sleep(2 * time.Second)
	fmt.Println("\n\033[32m✅ Notification test complete!\033[0m")
	fmt.Println("\nWhat you should have seen:")
	fmt.Println("  1. An OS notification (macOS: top-right corner)")
	fmt.Println("  2. A tmux status bar message (if inside tmux)")
	fmt.Println("  3. A loop completion OS notification")
	fmt.Println("  4. A loop completion tmux message (if inside tmux)")
	fmt.Println()

	return nil
}
