package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Jayphen/coders/internal/types"
)

// TestNewModel tests the initialization of a new TUI model.
func TestNewModel(t *testing.T) {
	version := "1.0.0"
	model := NewModel(version)

	if model.version != version {
		t.Errorf("version = %q, want %q", model.version, version)
	}

	if !model.loading {
		t.Error("expected loading to be true on new model")
	}

	if model.selectedIndex != 0 {
		t.Errorf("selectedIndex = %d, want 0", model.selectedIndex)
	}

	if model.previewLines != defaultPreviewLines {
		t.Errorf("previewLines = %d, want %d", model.previewLines, defaultPreviewLines)
	}

	if model.spawnMode {
		t.Error("expected spawnMode to be false on new model")
	}

	if model.confirmKill {
		t.Error("expected confirmKill to be false on new model")
	}

	if model.previewFocus {
		t.Error("expected previewFocus to be false on new model")
	}
}

// TestKeyboardNavigation tests up/down and j/k navigation keys.
func TestKeyboardNavigation(t *testing.T) {
	tests := []struct {
		name           string
		sessions       []types.Session
		initialIndex   int
		key            string
		expectedIndex  int
		shouldChange   bool
	}{
		{
			name: "down arrow moves selection down",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
				{Name: "coder-claude-task3", Tool: "claude"},
			},
			initialIndex:  0,
			key:           "down",
			expectedIndex: 1,
			shouldChange:  true,
		},
		{
			name: "up arrow moves selection up",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
				{Name: "coder-claude-task3", Tool: "claude"},
			},
			initialIndex:  2,
			key:           "up",
			expectedIndex: 1,
			shouldChange:  true,
		},
		{
			name: "j key moves selection down",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
			},
			initialIndex:  0,
			key:           "j",
			expectedIndex: 1,
			shouldChange:  true,
		},
		{
			name: "k key moves selection up",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
			},
			initialIndex:  1,
			key:           "k",
			expectedIndex: 0,
			shouldChange:  true,
		},
		{
			name: "down arrow at bottom stays at bottom",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
			},
			initialIndex:  1,
			key:           "down",
			expectedIndex: 1,
			shouldChange:  false,
		},
		{
			name: "up arrow at top stays at top",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
			},
			initialIndex:  0,
			key:           "up",
			expectedIndex: 0,
			shouldChange:  false,
		},
		{
			name:          "navigation with no sessions does nothing",
			sessions:      []types.Session{},
			initialIndex:  0,
			key:           "down",
			expectedIndex: 0,
			shouldChange:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.sessions = tt.sessions
			model.selectedIndex = tt.initialIndex
			model.loading = false

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if tt.key == "up" {
				msg = tea.KeyMsg{Type: tea.KeyUp}
			} else if tt.key == "down" {
				msg = tea.KeyMsg{Type: tea.KeyDown}
			}

			updatedModel, _ := model.Update(msg)
			newModel := updatedModel.(*Model)

			if newModel.selectedIndex != tt.expectedIndex {
				t.Errorf("selectedIndex = %d, want %d", newModel.selectedIndex, tt.expectedIndex)
			}

			if tt.shouldChange && newModel.selectedIndex == tt.initialIndex {
				t.Error("expected selection to change but it didn't")
			}
		})
	}
}

// TestSpawnDialog tests the spawn dialog interaction.
func TestSpawnDialog(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		inSpawnMode     bool
		expectedMode    bool
		shouldTrigger   bool
		description     string
	}{
		{
			name:          "s key opens spawn dialog",
			key:           "s",
			inSpawnMode:   false,
			expectedMode:  true,
			shouldTrigger: true,
			description:   "pressing 's' should open spawn mode",
		},
		{
			name:          "esc closes spawn dialog",
			key:           "esc",
			inSpawnMode:   true,
			expectedMode:  false,
			shouldTrigger: true,
			description:   "pressing 'esc' should close spawn mode",
		},
		{
			name:          "enter in spawn mode closes dialog",
			key:           "enter",
			inSpawnMode:   true,
			expectedMode:  false,
			shouldTrigger: true,
			description:   "pressing 'enter' should submit and close spawn mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.spawnMode = tt.inSpawnMode
			model.loading = false
			model.sessions = []types.Session{
				{Name: "coder-claude-test", Tool: "claude"},
			}

			var msg tea.KeyMsg
			switch tt.key {
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updatedModel, _ := model.Update(msg)
			newModel := updatedModel.(*Model)

			if newModel.spawnMode != tt.expectedMode {
				t.Errorf("%s: spawnMode = %v, want %v", tt.description, newModel.spawnMode, tt.expectedMode)
			}
		})
	}
}

// TestSpawnInput tests text input in spawn dialog.
func TestSpawnInput(t *testing.T) {
	model := NewModel("test")
	model.spawnMode = true
	model.loading = false
	model.spawnInput.Focus()

	// Set the text directly using SetValue since we can't simulate individual keypresses
	// in a way that bubbles textinput will accept without a full terminal
	testInput := "claude --task \"Fix bug\""
	model.spawnInput.SetValue(testInput)

	// The text input should contain the set text
	value := model.spawnInput.Value()
	if value != testInput {
		t.Errorf("spawnInput.Value() = %q, want %q", value, testInput)
	}

	// Test that esc clears the input
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := model.Update(msg)
	newModel := updatedModel.(*Model)

	if newModel.spawnInput.Value() != "" {
		t.Errorf("spawnInput after esc = %q, want empty string", newModel.spawnInput.Value())
	}
}

// TestConfirmKillDialog tests the kill confirmation dialog.
func TestConfirmKillDialog(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		inConfirm     bool
		shouldConfirm bool
		shouldCancel  bool
	}{
		{
			name:          "y key confirms kill",
			key:           "y",
			inConfirm:     true,
			shouldConfirm: true,
			shouldCancel:  false,
		},
		{
			name:          "Y key confirms kill",
			key:           "Y",
			inConfirm:     true,
			shouldConfirm: true,
			shouldCancel:  false,
		},
		{
			name:          "n key cancels kill",
			key:           "n",
			inConfirm:     true,
			shouldConfirm: false,
			shouldCancel:  true,
		},
		{
			name:          "N key cancels kill",
			key:           "N",
			inConfirm:     true,
			shouldConfirm: false,
			shouldCancel:  true,
		},
		{
			name:          "esc cancels kill",
			key:           "esc",
			inConfirm:     true,
			shouldConfirm: false,
			shouldCancel:  true,
		},
		{
			name:          "enter cancels kill",
			key:           "enter",
			inConfirm:     true,
			shouldConfirm: false,
			shouldCancel:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.confirmKill = tt.inConfirm
			model.loading = false
			// Add a completed session so there's something to kill
			model.sessions = []types.Session{
				{
					Name:       "coder-claude-test",
					Tool:       "claude",
					HasPromise: true,
					Promise: &types.CoderPromise{
						Status: types.PromiseCompleted,
					},
				},
			}

			var msg tea.KeyMsg
			switch tt.key {
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updatedModel, cmd := model.Update(msg)
			newModel := updatedModel.(*Model)

			// After any key, confirmKill should be false
			if newModel.confirmKill {
				t.Error("confirmKill should be false after handling key")
			}

			// Check if command was returned (indicates action)
			hasCmd := cmd != nil
			if tt.shouldConfirm && !hasCmd {
				t.Error("expected command to be returned for confirmation")
			}
			if tt.shouldCancel && hasCmd && newModel.statusMessage != "Cancelled" {
				t.Errorf("expected cancel message, got %q", newModel.statusMessage)
			}
		})
	}
}

// TestPreviewFocus tests tab key for preview focus.
func TestPreviewFocus(t *testing.T) {
	tests := []struct {
		name           string
		initialFocus   bool
		key            string
		expectedFocus  bool
		description    string
	}{
		{
			name:          "tab from main view focuses preview",
			initialFocus:  false,
			key:           "tab",
			expectedFocus: true,
			description:   "tab should focus preview input",
		},
		{
			name:          "tab from preview unfocuses",
			initialFocus:  true,
			key:           "tab",
			expectedFocus: false,
			description:   "tab in preview should unfocus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.previewFocus = tt.initialFocus
			model.loading = false
			model.sessions = []types.Session{
				{Name: "coder-claude-test", Tool: "claude"},
			}

			msg := tea.KeyMsg{Type: tea.KeyTab}
			updatedModel, _ := model.Update(msg)
			newModel := updatedModel.(*Model)

			if newModel.previewFocus != tt.expectedFocus {
				t.Errorf("%s: previewFocus = %v, want %v", tt.description, newModel.previewFocus, tt.expectedFocus)
			}
		})
	}
}

// TestSessionListRendering tests the rendering of session lists.
func TestSessionListRendering(t *testing.T) {
	tests := []struct {
		name     string
		sessions []types.Session
		expected []string // strings that should appear in output
		notExpected []string // strings that should not appear
	}{
		{
			name:     "empty session list",
			sessions: []types.Session{},
			expected: []string{"No active coder sessions"},
		},
		{
			name: "active sessions",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude", Task: "task1"},
				{Name: "coder-gemini-task2", Tool: "gemini", Task: "task2"},
			},
			expected: []string{"Active", "claude", "gemini"},
		},
		{
			name: "completed sessions",
			sessions: []types.Session{
				{
					Name:       "coder-claude-task1",
					Tool:       "claude",
					Task:       "task1",
					HasPromise: true,
					Promise: &types.CoderPromise{
						Status:  types.PromiseCompleted,
						Summary: "Task completed",
					},
				},
			},
			expected: []string{"Completed", "✓"},
		},
		{
			name: "orchestrator session",
			sessions: []types.Session{
				{
					Name:           "coder-orchestrator",
					Tool:           "unknown",
					Task:           "orchestrator",
					IsOrchestrator: true,
				},
			},
			expected: []string{"orchestrator", "🎯"},
		},
		{
			name: "mixed active and completed",
			sessions: []types.Session{
				{Name: "coder-claude-active", Tool: "claude", Task: "active"},
				{
					Name:       "coder-claude-done",
					Tool:       "claude",
					Task:       "done",
					HasPromise: true,
					Promise: &types.CoderPromise{
						Status:  types.PromiseCompleted,
						Summary: "Done",
					},
				},
			},
			expected: []string{"Active", "Completed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.sessions = tt.sessions
			model.loading = false

			output := model.renderSessionList()

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q\noutput: %s", expected, output)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("expected output to NOT contain %q\noutput: %s", notExpected, output)
				}
			}
		})
	}
}

// TestSessionRowRendering tests individual session row rendering.
func TestSessionRowRendering(t *testing.T) {
	tests := []struct {
		name     string
		session  types.Session
		index    int
		selected bool
		expected []string
	}{
		{
			name: "active claude session",
			session: types.Session{
				Name: "coder-claude-implement-auth",
				Tool: "claude",
				Task: "implement-auth",
			},
			index:    0,
			selected: true,
			expected: []string{"claude", "implement-auth"},
		},
		{
			name: "completed gemini session",
			session: types.Session{
				Name:       "coder-gemini-fix-bug",
				Tool:       "gemini",
				Task:       "fix-bug",
				HasPromise: true,
				Promise: &types.CoderPromise{
					Status:  types.PromiseCompleted,
					Summary: "Bug fixed",
				},
			},
			index:    0,
			selected: false,
			expected: []string{"gemini", "✓"},
		},
		{
			name: "blocked session",
			session: types.Session{
				Name:       "coder-claude-blocked",
				Tool:       "claude",
				Task:       "blocked",
				HasPromise: true,
				Promise: &types.CoderPromise{
					Status:   types.PromiseBlocked,
					Summary:  "Blocked",
					Blockers: []string{"issue-123"},
				},
			},
			index:    0,
			selected: false,
			expected: []string{"claude", "!"},
		},
		{
			name: "orchestrator session",
			session: types.Session{
				Name:           "coder-orchestrator",
				Tool:           "unknown",
				Task:           "orchestrator",
				IsOrchestrator: true,
			},
			index:    0,
			selected: false,
			expected: []string{"orchestrator", "🎯"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.sessions = []types.Session{tt.session}
			model.selectedIndex = 0
			model.loading = false

			output := model.renderSessionRow(tt.index)

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("expected row to contain %q\nrow: %s", expected, output)
				}
			}
		})
	}
}

// TestSessionMessages tests message handling for sessions.
func TestSessionMessages(t *testing.T) {
	tests := []struct {
		name            string
		msg             tea.Msg
		initialLoading  bool
		expectedLoading bool
		checkSessions   bool
	}{
		{
			name: "sessionsMsg sets sessions and stops loading",
			msg: sessionsMsg([]types.Session{
				{Name: "coder-claude-task", Tool: "claude"},
			}),
			initialLoading:  true,
			expectedLoading: false,
			checkSessions:   true,
		},
		{
			name:            "errMsg stops loading",
			msg:             errMsg(tea.ErrProgramKilled),
			initialLoading:  true,
			expectedLoading: false,
			checkSessions:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.loading = tt.initialLoading

			updatedModel, _ := model.Update(tt.msg)
			newModel := updatedModel.(*Model)

			if newModel.loading != tt.expectedLoading {
				t.Errorf("loading = %v, want %v", newModel.loading, tt.expectedLoading)
			}

			if tt.checkSessions {
				if msg, ok := tt.msg.(sessionsMsg); ok {
					if len(newModel.sessions) != len(msg) {
						t.Errorf("sessions length = %d, want %d", len(newModel.sessions), len(msg))
					}
				}
			}
		})
	}
}

// TestWindowSizeMessage tests window size updates.
func TestWindowSizeMessage(t *testing.T) {
	model := NewModel("test")

	width := 120
	height := 40
	msg := tea.WindowSizeMsg{Width: width, Height: height}

	updatedModel, _ := model.Update(msg)
	newModel := updatedModel.(*Model)

	if newModel.width != width {
		t.Errorf("width = %d, want %d", newModel.width, width)
	}

	if newModel.height != height {
		t.Errorf("height = %d, want %d", newModel.height, height)
	}
}

// TestSpawnModeConflicts tests that spawn mode prevents other actions.
func TestSpawnModeConflicts(t *testing.T) {
	model := NewModel("test")
	model.spawnMode = true
	model.sessions = []types.Session{
		{Name: "coder-claude-task1", Tool: "claude"},
		{Name: "coder-claude-task2", Tool: "claude"},
	}
	model.selectedIndex = 0
	model.loading = false

	// Try to navigate while in spawn mode - should not work
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := model.Update(msg)
	newModel := updatedModel.(*Model)

	// Selection should not change because we're in spawn mode
	if newModel.selectedIndex != model.selectedIndex {
		t.Error("selection should not change while in spawn mode")
	}
}

// TestViewCaching tests that the view caching mechanism works.
func TestViewCaching(t *testing.T) {
	model := NewModel("test")
	model.sessions = []types.Session{
		{Name: "coder-claude-task", Tool: "claude"},
	}
	model.loading = false
	model.width = 80
	model.height = 24

	// First render
	view1 := model.View()
	if view1 == "" {
		t.Error("first view should not be empty")
	}

	// Second render with no state changes should use cache
	view2 := model.View()
	if view2 != view1 {
		t.Error("cached view should match first view")
	}

	// Change state - cache should invalidate
	model.selectedIndex = 0
	model.viewStateDirty = true
	view3 := model.View()
	// View might be the same or different depending on content, but it should render
	if view3 == "" {
		t.Error("view after state change should not be empty")
	}
}

// TestStatusMessage tests status message setting and display.
func TestStatusMessage(t *testing.T) {
	model := NewModel("test")
	model.loading = false

	statusMsg := "Test status message"
	model.setStatus(statusMsg)

	if model.statusMessage != statusMsg {
		t.Errorf("statusMessage = %q, want %q", model.statusMessage, statusMsg)
	}

	if model.statusExpiry.IsZero() {
		t.Error("statusExpiry should be set")
	}

	if model.statusExpiry.Before(time.Now()) {
		t.Error("statusExpiry should be in the future")
	}
}

// TestCountCompleted tests counting completed sessions.
func TestCountCompleted(t *testing.T) {
	tests := []struct {
		name     string
		sessions []types.Session
		expected int
	}{
		{
			name:     "no sessions",
			sessions: []types.Session{},
			expected: 0,
		},
		{
			name: "no completed sessions",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
			},
			expected: 0,
		},
		{
			name: "all completed sessions",
			sessions: []types.Session{
				{
					Name:       "coder-claude-task1",
					Tool:       "claude",
					HasPromise: true,
					Promise:    &types.CoderPromise{Status: types.PromiseCompleted},
				},
				{
					Name:       "coder-claude-task2",
					Tool:       "claude",
					HasPromise: true,
					Promise:    &types.CoderPromise{Status: types.PromiseCompleted},
				},
			},
			expected: 2,
		},
		{
			name: "mixed sessions",
			sessions: []types.Session{
				{Name: "coder-claude-active", Tool: "claude"},
				{
					Name:       "coder-claude-done",
					Tool:       "claude",
					HasPromise: true,
					Promise:    &types.CoderPromise{Status: types.PromiseCompleted},
				},
				{Name: "coder-gemini-active", Tool: "gemini"},
			},
			expected: 1,
		},
		{
			name: "orchestrator not counted",
			sessions: []types.Session{
				{
					Name:           "coder-orchestrator",
					Tool:           "unknown",
					HasPromise:     true,
					IsOrchestrator: true,
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.sessions = tt.sessions

			count := model.countCompleted()
			if count != tt.expected {
				t.Errorf("countCompleted() = %d, want %d", count, tt.expected)
			}
		})
	}
}

// TestSelectedSession tests getting the currently selected session.
func TestSelectedSession(t *testing.T) {
	tests := []struct {
		name          string
		sessions      []types.Session
		selectedIndex int
		expectNil     bool
	}{
		{
			name:          "no sessions",
			sessions:      []types.Session{},
			selectedIndex: 0,
			expectNil:     true,
		},
		{
			name: "valid selection",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
				{Name: "coder-claude-task2", Tool: "claude"},
			},
			selectedIndex: 1,
			expectNil:     false,
		},
		{
			name: "index out of bounds",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
			},
			selectedIndex: 5,
			expectNil:     true,
		},
		{
			name: "negative index",
			sessions: []types.Session{
				{Name: "coder-claude-task1", Tool: "claude"},
			},
			selectedIndex: -1,
			expectNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.sessions = tt.sessions
			model.selectedIndex = tt.selectedIndex

			session := model.selectedSession()

			if tt.expectNil && session != nil {
				t.Error("expected nil session but got non-nil")
			}

			if !tt.expectNil && session == nil {
				t.Error("expected non-nil session but got nil")
			}

			if !tt.expectNil && session != nil {
				expectedSession := &tt.sessions[tt.selectedIndex]
				if session.Name != expectedSession.Name {
					t.Errorf("session.Name = %q, want %q", session.Name, expectedSession.Name)
				}
			}
		})
	}
}

// TestParseSpawnArgs tests the spawn argument parsing.
func TestParseSpawnArgs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		expectErr bool
	}{
		{
			name:     "simple args",
			input:    "claude --task fix-bug",
			expected: []string{"claude", "--task", "fix-bug"},
		},
		{
			name:     "args with double quotes",
			input:    `claude --task "Fix the bug"`,
			expected: []string{"claude", "--task", "Fix the bug"},
		},
		{
			name:     "args with single quotes",
			input:    `claude --task 'Fix the bug'`,
			expected: []string{"claude", "--task", "Fix the bug"},
		},
		{
			name:     "args with escaped quotes",
			input:    `claude --task "Fix \"critical\" bug"`,
			expected: []string{"claude", "--task", `Fix "critical" bug`},
		},
		{
			name:      "unterminated double quote",
			input:     `claude --task "Fix bug`,
			expectErr: true,
		},
		{
			name:      "unterminated single quote",
			input:     `claude --task 'Fix bug`,
			expectErr: true,
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "complex command",
			input:    `gemini --task "Add tests" --priority high --file "src/main.go"`,
			expected: []string{"gemini", "--task", "Add tests", "--priority", "high", "--file", "src/main.go"},
		},
		{
			name:     "args with tabs and newlines",
			input:    "claude\t--task\nfix-bug",
			expected: []string{"claude", "--task", "fix-bug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSpawnArgs(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("result length = %d, want %d\nresult: %v\nexpected: %v", len(result), len(tt.expected), result, tt.expected)
				return
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("arg[%d] = %q, want %q", i, arg, tt.expected[i])
				}
			}
		})
	}
}

// TestTextInputFocus tests that text inputs are properly focused.
func TestTextInputFocus(t *testing.T) {
	model := NewModel("test")

	// Initially, neither input should be focused
	if model.spawnInput.Focused() {
		t.Error("spawnInput should not be focused initially")
	}
	if model.previewInput.Focused() {
		t.Error("previewInput should not be focused initially")
	}

	// Open spawn mode
	model.spawnMode = true
	model.spawnInput.Focus()

	if !model.spawnInput.Focused() {
		t.Error("spawnInput should be focused when spawn mode is active")
	}

	// Switch to preview focus
	model.spawnMode = false
	model.spawnInput.Blur()
	model.previewFocus = true
	model.previewInput.Focus()

	if !model.previewInput.Focused() {
		t.Error("previewInput should be focused when preview focus is active")
	}
	if model.spawnInput.Focused() {
		t.Error("spawnInput should not be focused when not in spawn mode")
	}
}

// TestPreviewInputInFocus tests sending text to sessions via preview input.
func TestPreviewInputInFocus(t *testing.T) {
	model := NewModel("test")
	model.previewFocus = true
	model.previewInput.Focus()
	model.sessions = []types.Session{
		{Name: "coder-claude-test", Tool: "claude"},
	}
	model.selectedIndex = 0

	// Type some text
	testText := "test message"
	model.previewInput.SetValue(testText)

	// Press enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(msg)
	newModel := updatedModel.(*Model)

	// Input should be cleared after sending
	if newModel.previewInput.Value() != "" {
		t.Errorf("previewInput should be cleared after enter, got %q", newModel.previewInput.Value())
	}

	// Status message should indicate success or failure
	if newModel.statusMessage == "" {
		t.Error("expected status message after sending text")
	}
}

// TestPreviewMessage tests preview message handling.
func TestPreviewMessage(t *testing.T) {
	model := NewModel("test")
	model.sessions = []types.Session{
		{Name: "coder-claude-test", Tool: "claude"},
	}
	model.selectedIndex = 0
	model.previewSession = "coder-claude-test"
	model.previewLoading = true

	testOutput := "test output\nline 2\nline 3"
	msg := previewMsg{
		session: "coder-claude-test",
		output:  testOutput,
		err:     nil,
	}

	updatedModel, _ := model.Update(msg)
	newModel := updatedModel.(*Model)

	if newModel.previewLoading {
		t.Error("previewLoading should be false after receiving preview message")
	}

	if newModel.preview != testOutput {
		t.Errorf("preview = %q, want %q", newModel.preview, testOutput)
	}

	if newModel.previewErr != nil {
		t.Errorf("previewErr should be nil, got %v", newModel.previewErr)
	}

	// Test error case
	testErr := errMsg(tea.ErrProgramKilled)
	msg2 := previewMsg{
		session: "coder-claude-test",
		err:     testErr,
	}

	updatedModel2, _ := model.Update(msg2)
	newModel2 := updatedModel2.(*Model)

	if newModel2.previewErr == nil {
		t.Error("previewErr should be set when preview message contains error")
	}

	if newModel2.preview != "" {
		t.Error("preview should be empty when error occurs")
	}
}

// TestKeyQuitBehavior tests that q and ctrl+c quit the application.
func TestKeyQuitBehavior(t *testing.T) {
	tests := []struct {
		name       string
		key        tea.KeyMsg
		shouldQuit bool
	}{
		{
			name:       "q key quits",
			key:        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			shouldQuit: true,
		},
		{
			name:       "ctrl+c quits",
			key:        tea.KeyMsg{Type: tea.KeyCtrlC},
			shouldQuit: true,
		},
		{
			name:       "other keys don't quit",
			key:        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")},
			shouldQuit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("test")
			model.loading = false

			_, cmd := model.Update(tt.key)

			hasQuitCmd := cmd != nil && cmd() == tea.Quit()

			if tt.shouldQuit && !hasQuitCmd {
				t.Error("expected quit command but didn't get one")
			}

			if !tt.shouldQuit && hasQuitCmd {
				t.Error("got unexpected quit command")
			}
		})
	}
}
