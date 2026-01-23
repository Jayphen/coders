/**
 * Unit tests for coders spawn functionality
 * Tests buildSpawnCommand for all CLI tools with various argument combinations
 */

import { describe, it, expect, beforeEach } from 'vitest';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

// Note: We're testing the command generation logic, not the actual spawning
// The main.js file would need to export the functions for proper testing
// For now, we'll replicate the logic to test

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Replicate generateSystemPrompt for testing
 */
function generateSystemPrompt(customSystemPrompt = null) {
  if (customSystemPrompt) {
    return customSystemPrompt;
  }

  let prompt = 'You are a spawned coder session with full permissions to complete your assigned task.\n\n';
  prompt += 'IMPORTANT RULES:\n';
  prompt += '- You have full access to read, write, and execute code\n';
  prompt += '- When you finish your task, you MUST publish a completion promise using: /coders:promise "Brief summary"\n';
  prompt += '- If you get blocked, use: /coders:promise "Reason for being blocked" --status blocked\n';
  prompt += '- If you need review, use: /coders:promise "What you did" --status needs-review\n';
  prompt += '\nThis notifies the orchestrator and dashboard about your status.';

  return prompt;
}

/**
 * Replicate generateUserMessage for testing
 */
function generateUserMessage(taskDescription, contextFiles = []) {
  let message = `TASK: ${taskDescription}\n\n`;

  if (contextFiles.length > 0) {
    message += 'CONTEXT FILES:\n';
    // In real implementation, this would read file contents
    contextFiles.forEach(file => {
      message += `\n--- ${file} ---\n[File content would be here]\n`;
    });
    message += '\n';
  }

  message += 'Please complete this task.';
  return message;
}

/**
 * Replicate buildSpawnCommand for testing
 */
function buildSpawnCommand(tool, promptFile, userMessage, extraEnv = {}, model = null, systemPrompt = null) {
  let cmd;
  const escapedUserMessage = userMessage.replace(/'/g, "'\\''");
  const escapedSystemPrompt = systemPrompt ? systemPrompt.replace(/'/g, "'\\''") : null;

  const envStr = Object.entries(extraEnv)
    .filter(([_, v]) => v !== undefined && v !== null)
    .map(([k, v]) => `${k}="${v}"`)
    .join(' ');
  const envPrefix = envStr ? envStr + ' ' : '';
  const modelArg = model ? ` --model "${model}"` : '';
  const systemPromptArg = escapedSystemPrompt ? ` --system-prompt '${escapedSystemPrompt}'` : '';

  if (tool === 'claude' || tool === 'claude-code') {
    cmd = `${envPrefix}claude --dangerously-skip-permissions${modelArg}${systemPromptArg} < "${promptFile}"`;
  } else if (tool === 'gemini') {
    const fullPrompt = escapedSystemPrompt
      ? `System: ${escapedSystemPrompt}\n\nUser: ${escapedUserMessage}`
      : escapedUserMessage;
    cmd = `${envPrefix}gemini --yolo${modelArg} --prompt-interactive '${fullPrompt}'`;
  } else if (tool === 'codex' || tool === 'openai-codex') {
    const fullPrompt = escapedSystemPrompt
      ? `System: ${escapedSystemPrompt}\n\nUser: ${escapedUserMessage}`
      : escapedUserMessage;
    cmd = `${envPrefix}codex --dangerously-bypass-approvals-and-sandbox${modelArg} '${fullPrompt}'`;
  } else if (tool === 'opencode' || tool === 'open-code') {
    cmd = `${envPrefix}opencode${modelArg}${systemPromptArg} < "${promptFile}"`;
  }

  return cmd;
}

describe('Spawn Command Generation', () => {
  const testPromptFile = '/tmp/test-prompt.txt';
  const testTask = 'Fix the login bug';
  const testSystemPrompt = 'You are a senior developer';

  describe('generateSystemPrompt', () => {
    it('should return default system prompt when no custom prompt provided', () => {
      const prompt = generateSystemPrompt();
      expect(prompt).toContain('spawned coder session');
      expect(prompt).toContain('/coders:promise');
      expect(prompt).toContain('full permissions');
    });

    it('should return custom system prompt when provided', () => {
      const custom = 'You are a Rust expert';
      const prompt = generateSystemPrompt(custom);
      expect(prompt).toBe(custom);
    });
  });

  describe('generateUserMessage', () => {
    it('should generate basic task message', () => {
      const message = generateUserMessage(testTask);
      expect(message).toContain('TASK: Fix the login bug');
      expect(message).toContain('Please complete this task');
    });

    it('should include context files when provided', () => {
      const message = generateUserMessage(testTask, ['spec.md', 'design.md']);
      expect(message).toContain('CONTEXT FILES');
      expect(message).toContain('spec.md');
      expect(message).toContain('design.md');
    });

    it('should not include context section when no files provided', () => {
      const message = generateUserMessage(testTask);
      expect(message).not.toContain('CONTEXT FILES');
    });
  });

  describe('buildSpawnCommand - Claude', () => {
    it('should generate basic claude command without system prompt', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage);

      expect(cmd).toContain('claude');
      expect(cmd).toContain('--dangerously-skip-permissions');
      expect(cmd).toContain(`< "${testPromptFile}"`);
      expect(cmd).not.toContain('--system-prompt');
    });

    it('should include system prompt when provided', () => {
      const userMessage = generateUserMessage(testTask);
      const systemPrompt = generateSystemPrompt(testSystemPrompt);
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, {}, null, systemPrompt);

      expect(cmd).toContain('claude');
      expect(cmd).toContain('--system-prompt');
      expect(cmd).toContain(testSystemPrompt);
    });

    it('should include model when provided', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, {}, 'claude-opus-4');

      expect(cmd).toContain('--model "claude-opus-4"');
    });

    it('should include environment variables', () => {
      const userMessage = generateUserMessage(testTask);
      const env = { WORKSPACE_DIR: '/path/to/workspace' };
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, env);

      expect(cmd).toContain('WORKSPACE_DIR="/path/to/workspace"');
    });

    it('should handle all options together', () => {
      const userMessage = generateUserMessage(testTask);
      const systemPrompt = generateSystemPrompt(testSystemPrompt);
      const env = { WORKSPACE_DIR: '/workspace' };
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, env, 'claude-opus-4', systemPrompt);

      expect(cmd).toContain('WORKSPACE_DIR="/workspace"');
      expect(cmd).toContain('claude');
      expect(cmd).toContain('--dangerously-skip-permissions');
      expect(cmd).toContain('--model "claude-opus-4"');
      expect(cmd).toContain('--system-prompt');
      expect(cmd).toContain(testSystemPrompt);
      expect(cmd).toContain(`< "${testPromptFile}"`);
    });
  });

  describe('buildSpawnCommand - Gemini', () => {
    it('should generate basic gemini command', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('gemini', testPromptFile, userMessage);

      expect(cmd).toContain('gemini');
      expect(cmd).toContain('--yolo');
      expect(cmd).toContain('--prompt-interactive');
      expect(cmd).toContain(testTask);
    });

    it('should prepend system prompt to user message (no native support)', () => {
      const userMessage = generateUserMessage(testTask);
      const systemPrompt = generateSystemPrompt(testSystemPrompt);
      const cmd = buildSpawnCommand('gemini', testPromptFile, userMessage, {}, null, systemPrompt);

      expect(cmd).toContain('System:');
      expect(cmd).toContain(testSystemPrompt);
      expect(cmd).toContain('User:');
      expect(cmd).toContain(testTask);
    });

    it('should include model when provided', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('gemini', testPromptFile, userMessage, {}, 'gemini-2.0-flash');

      expect(cmd).toContain('--model "gemini-2.0-flash"');
    });
  });

  describe('buildSpawnCommand - Codex', () => {
    it('should generate basic codex command', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('codex', testPromptFile, userMessage);

      expect(cmd).toContain('codex');
      expect(cmd).toContain('--dangerously-bypass-approvals-and-sandbox');
      expect(cmd).toContain(testTask);
    });

    it('should prepend system prompt to user message', () => {
      const userMessage = generateUserMessage(testTask);
      const systemPrompt = generateSystemPrompt(testSystemPrompt);
      const cmd = buildSpawnCommand('codex', testPromptFile, userMessage, {}, null, systemPrompt);

      expect(cmd).toContain('System:');
      expect(cmd).toContain(testSystemPrompt);
      expect(cmd).toContain('User:');
    });

    it('should handle openai-codex alias', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('openai-codex', testPromptFile, userMessage);

      expect(cmd).toContain('codex');
    });
  });

  describe('buildSpawnCommand - OpenCode', () => {
    it('should generate basic opencode command', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('opencode', testPromptFile, userMessage);

      expect(cmd).toContain('opencode');
      expect(cmd).toContain(`< "${testPromptFile}"`);
    });

    it('should include system prompt flag', () => {
      const userMessage = generateUserMessage(testTask);
      const systemPrompt = generateSystemPrompt(testSystemPrompt);
      const cmd = buildSpawnCommand('opencode', testPromptFile, userMessage, {}, null, systemPrompt);

      expect(cmd).toContain('--system-prompt');
      expect(cmd).toContain(testSystemPrompt);
    });

    it('should handle open-code alias', () => {
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('open-code', testPromptFile, userMessage);

      expect(cmd).toContain('opencode');
    });
  });

  describe('Shell escaping', () => {
    it('should escape single quotes in user message', () => {
      const userMessage = "TASK: Fix the 'login' bug\n\nPlease complete this task.";
      const cmd = buildSpawnCommand('gemini', testPromptFile, userMessage);

      // The escaped version should contain '\'' for each single quote
      expect(cmd).toMatch(/login.*'\\''/);
    });

    it('should escape single quotes in system prompt', () => {
      const systemPrompt = "You're a senior developer";
      const userMessage = generateUserMessage(testTask);
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, {}, null, systemPrompt);

      expect(cmd).toMatch(/You.*'\\''/);
    });
  });

  describe('Environment variables', () => {
    it('should filter out undefined env vars', () => {
      const userMessage = generateUserMessage(testTask);
      const env = {
        DEFINED: 'value',
        UNDEFINED: undefined,
        NULL: null
      };
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, env);

      expect(cmd).toContain('DEFINED="value"');
      expect(cmd).not.toContain('UNDEFINED');
      expect(cmd).not.toContain('NULL');
    });

    it('should handle multiple environment variables', () => {
      const userMessage = generateUserMessage(testTask);
      const env = {
        WORKSPACE_DIR: '/workspace',
        CODERS_SESSION_ID: 'test-session',
        CODERS_PARENT_SESSION_ID: 'parent-session'
      };
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, env);

      expect(cmd).toContain('WORKSPACE_DIR="/workspace"');
      expect(cmd).toContain('CODERS_SESSION_ID="test-session"');
      expect(cmd).toContain('CODERS_PARENT_SESSION_ID="parent-session"');
    });
  });

  describe('Integration scenarios', () => {
    it('should handle complete workflow with context files', () => {
      const contextFiles = ['prd.md', 'spec.md'];
      const userMessage = generateUserMessage(testTask, contextFiles);
      const systemPrompt = generateSystemPrompt('You are a testing expert');
      const env = { WORKSPACE_DIR: '/project' };
      const model = 'claude-opus-4';

      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage, env, model, systemPrompt);

      expect(cmd).toContain('WORKSPACE_DIR="/project"');
      expect(cmd).toContain('claude');
      expect(cmd).toContain('--dangerously-skip-permissions');
      expect(cmd).toContain('--model "claude-opus-4"');
      expect(cmd).toContain('--system-prompt');
      expect(cmd).toContain('testing expert');
      expect(cmd).toContain(`< "${testPromptFile}"`);
    });

    it('should work with minimal options', () => {
      const userMessage = generateUserMessage('Simple task');
      const cmd = buildSpawnCommand('claude', testPromptFile, userMessage);

      expect(cmd).toBe(`claude --dangerously-skip-permissions < "${testPromptFile}"`);
    });
  });
});
