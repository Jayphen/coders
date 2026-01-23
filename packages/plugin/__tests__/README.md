# Coders Spawn Tests

Comprehensive unit tests for the coders spawn functionality.

## Running Tests

```bash
# Run all tests
npm test

# Run tests in watch mode (re-run on file changes)
npm run test:watch

# Run tests with UI interface
npm run test:ui

# Run tests with coverage report
npm run test:coverage
```

## Test Coverage

The test suite covers:

### Core Functions
- **`generateSystemPrompt()`** - System prompt generation with default and custom prompts
- **`generateUserMessage()`** - User message generation with task and context files
- **`buildSpawnCommand()`** - Command generation for all CLI tools

### CLI Tools Tested
- ✅ **Claude** - Native `--system-prompt` support
- ✅ **Gemini** - System prompt prepended to user message
- ✅ **Codex** - System prompt prepended to user message
- ✅ **OpenCode** - Native `--system-prompt` support

### Argument Combinations
- Basic commands (tool only)
- With system prompt (default and custom)
- With model selection
- With environment variables
- With all options combined

### Edge Cases
- Shell escaping (single quotes in prompts)
- Environment variable filtering (undefined/null handling)
- Tool aliases (claude-code, openai-codex, open-code)
- Context files handling
- Minimal vs maximal option combinations

## Test Structure

```
__tests__/
├── spawn.test.js    - Main test suite (25 tests)
└── README.md        - This file
```

## Implementation Notes

The tests replicate the core logic from `skills/coders/scripts/main.js` to verify:

1. **System Prompt Strategy**: Persistent role/behavior instructions
2. **User Message Strategy**: Specific task descriptions
3. **Hybrid Approach**: Combines both for optimal spawning

### Command Format

**Claude/OpenCode:**
```bash
claude --dangerously-skip-permissions --model "model-id" --system-prompt 'prompt' < "/tmp/file.txt"
```

**Gemini/Codex:**
```bash
gemini --yolo --model "model-id" --prompt-interactive 'System: prompt\n\nUser: task'
```

## Adding New Tests

To add more test cases:

1. Add test to appropriate `describe` block in `spawn.test.js`
2. Use `it('should ...', () => { ... })` format
3. Test both successful scenarios and edge cases
4. Run `npm test` to verify

## Future Enhancements

Potential areas for additional testing:
- Integration tests with actual tmux sessions
- Error handling and validation
- File I/O for prompt files
- Session name generation
- Worktree creation
- Path resolution with zoxide
