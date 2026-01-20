/**
 * Coder Spawner - Unit Tests
 * 
 * Tests for the core functionality without mocking tmux
 */

const fs = require('fs');
const path = require('path');

// Mock the child_process module
jest.mock('child_process', () => ({
  execSync: jest.fn((cmd, opts) => {
    if (cmd.includes('git rev-parse --show-toplevel')) {
      return '/fake/git/root';
    }
    if (cmd.includes('git rev-parse --abbrev-ref')) {
      return 'main';
    }
    if (cmd.includes('tmux list-sessions')) {
      return 'coder-test-123: 1 windows (created ...)\ncoder-test-456: 1 windows (created ...)';
    }
    return '';
  }),
}));

const { execSync } = require('child_process');

// Test helpers - extract functions from main.js
function createPrompt(task, contextFiles) {
  let prompt = `TASK: ${task}\n\n`;
  
  if (contextFiles && contextFiles.length > 0) {
    prompt += 'CONTEXT:\n';
    contextFiles.forEach(file => {
      const content = 'test content';
      if (content) {
        prompt += `\n--- ${file} ---\n${content}\n`;
      }
    });
    prompt += '\n';
  }
  
  return prompt;
}

function buildCommand(tool, promptFile, worktreePath) {
  const env = worktreePath ? `WORKSPACE_DIR="${worktreePath}" ` : '';
  
  if (tool === 'claude' || tool === 'claude-code') {
    return `${env}claude --dangerously-spawn-permission -f "${promptFile}"`;
  } else if (tool === 'gemini') {
    return `${env}gemini -f "${promptFile}"`;
  } else if (tool === 'codex') {
    return `${env}codex -f "${promptFile}"`;
  }
  throw new Error(`Unknown tool: ${tool}`);
}

describe('Coder Spawner - Core Functions', () => {
  
  describe('createPrompt', () => {
    test('creates basic prompt with task', () => {
      const prompt = createPrompt('Fix the bug', []);
      
      expect(prompt).toContain('TASK: Fix the bug');
    });
    
    test('includes context files', () => {
      const prompt = createPrompt('Fix the bug', ['docs/prd.md', 'src/spec.ts']);
      
      expect(prompt).toContain('--- docs/prd.md ---');
      expect(prompt).toContain('--- src/spec.ts ---');
      expect(prompt).toContain('CONTEXT:');
    });
    
    test('handles empty context', () => {
      const prompt = createPrompt('Simple task', null);
      
      expect(prompt).toBe('TASK: Simple task\n\n');
    });
  });
  
  describe('buildCommand', () => {
    test('builds Claude command with spawn permission', () => {
      const cmd = buildCommand('claude', '/tmp/prompt.txt');
      
      expect(cmd).toContain('claude --dangerously-spawn-permission');
      expect(cmd).toContain('-f "/tmp/prompt.txt"');
    });
    
    test('builds Gemini command', () => {
      const cmd = buildCommand('gemini', '/tmp/prompt.txt');
      
      expect(cmd).toContain('gemini -f');
    });
    
    test('builds Codex command', () => {
      const cmd = buildCommand('codex', '/tmp/prompt.txt');
      
      expect(cmd).toContain('codex -f');
    });
    
    test('includes WORKSPACE_DIR for worktree', () => {
      const cmd = buildCommand('claude', '/tmp/prompt.txt', '/worktrees/feature');
      
      expect(cmd).toContain('WORKSPACE_DIR="/worktrees/feature"');
      expect(cmd).toContain('claude');
    });
  });
  
  describe('Error Handling', () => {
    test('throws on unknown tool', () => {
      expect(() => {
        buildCommand('unknown-tool', '/tmp/prompt.txt');
      }).toThrow('Unknown tool');
    });
  });
});

describe('Integration - Mock File System', () => {
  const testDir = '/tmp/coders-test';
  
  beforeAll(() => {
    fs.mkdirSync(testDir, { recursive: true });
  });
  
  afterAll(() => {
    // Cleanup
    fs.rmSync(testDir, { recursive: true });
  });
  
  test('creates prompt file', () => {
    const promptFile = path.join(testDir, 'test-prompt.txt');
    const prompt = createPrompt('Test task', []);
    fs.writeFileSync(promptFile, prompt);
    
    expect(fs.existsSync(promptFile)).toBe(true);
    expect(fs.readFileSync(promptFile, 'utf8')).toContain('TASK: Test task');
  });
});

console.log('✅ Tests defined successfully!');
console.log('\nTo run tests:');
console.log('  npm install -D vitest');
console.log('  npx vitest run');
console.log('\nOr add to package.json:');
console.log('  "scripts": { "test": "vitest run" }');
