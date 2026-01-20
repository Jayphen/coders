#!/usr/bin/env node
/**
 * Simple Test Runner for Coder Spawner
 * 
 * Run basic tests without needing vitest
 */

const fs = require('fs');
const path = require('path');

let passed = 0;
let failed = 0;

function test(name, fn) {
  try {
    fn();
    console.log(`  ✅ ${name}`);
    passed++;
  } catch (e) {
    console.log(`  ❌ ${name}`);
    console.log(`     Error: ${e.message}`);
    failed++;
  }
}

function expect(actual) {
  return {
    toBe(expected) {
      if (actual !== expected) {
        throw new Error(`Expected "${expected}" but got "${actual}"`);
      }
    },
    toContain(substr) {
      if (!actual.includes(substr)) {
        throw new Error(`Expected to contain "${substr}" but got "${actual}"`);
      }
    },
    toBeTruthy() {
      if (!actual) {
        throw new Error(`Expected truthy but got "${actual}"`);
      }
    },
    toBeFalsy() {
      if (actual) {
        throw new Error(`Expected falsy but got "${actual}"`);
      }
    }
  };
}

console.log('\n🧪 Coder Spawner - Tests\n');

// Test helpers
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

console.log('createPrompt:');
test('creates basic prompt with task', () => {
  const prompt = createPrompt('Fix the bug', []);
  expect(prompt).toContain('TASK: Fix the bug');
});

test('includes context files', () => {
  const prompt = createPrompt('Fix the bug', ['docs/prd.md', 'src/spec.ts']);
  expect(prompt).toContain('--- docs/prd.md ---');
  expect(prompt).toContain('--- src/spec.ts ---');
});

test('handles empty context', () => {
  const prompt = createPrompt('Simple task', null);
  expect(prompt).toBe('TASK: Simple task\n\n');
});

console.log('\nbuildCommand:');
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
});

test('throws on unknown tool', () => {
  let threw = false;
  try {
    buildCommand('unknown-tool', '/tmp/prompt.txt');
  } catch (e) {
    threw = true;
    expect(e.message).toContain('Unknown tool');
  }
  if (!threw) throw new Error('Should have thrown');
});

console.log('\nFile System:');
test('creates and reads prompt file', () => {
  const testDir = '/tmp/coders-test-' + Date.now();
  fs.mkdirSync(testDir, { recursive: true });
  const promptFile = path.join(testDir, 'test-prompt.txt');
  const prompt = createPrompt('Test task', []);
  fs.writeFileSync(promptFile, prompt);
  expect(fs.existsSync(promptFile)).toBeTruthy();
  expect(fs.readFileSync(promptFile, 'utf8')).toContain('TASK: Test task');
  fs.rmSync(testDir, { recursive: true });
});

// Summary
console.log('\n' + '='.repeat(50));
console.log(`Results: ${passed} passed, ${failed} failed`);
console.log('='.repeat(50));

if (failed > 0) {
  process.exit(1);
}
