package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Jayphen/coders/internal/tmux"
)

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach [session]",
		Short: "Attach to a coder session",
		Long: `Attach to a coder session by name or partial match.

If inside tmux, switches to the session. If outside, attaches directly.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runAttach,
	}
}

func runAttach(cmd *cobra.Command, args []string) error {
	sessions, err := tmux.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no coder sessions found")
	}

	var sessionName string

	if len(args) == 0 {
		// No argument - attach to first active session or show list
		for _, s := range sessions {
			if !s.HasPromise {
				sessionName = s.Name
				break
			}
		}
		if sessionName == "" {
			sessionName = sessions[0].Name
		}
	} else {
		// Find session by name or partial match
		query := args[0]

		// First try exact match
		for _, s := range sessions {
			if s.Name == query || s.Name == tmux.SessionPrefix+query {
				sessionName = s.Name
				break
			}
		}

		// Then try partial match
		if sessionName == "" {
			for _, s := range sessions {
				if strings.Contains(s.Name, query) {
					sessionName = s.Name
					break
				}
			}
		}

		if sessionName == "" {
			return fmt.Errorf("no session matching '%s' found", query)
		}
	}

	fmt.Printf("Attaching to %s...\n", sessionName)
	return tmux.AttachSession(sessionName)
}
