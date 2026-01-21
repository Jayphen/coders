#!/usr/bin/env node

/**
 * Usage Monitor Script
 * 
 * Spawns a Claude session solely to track account-level usage stats.
 * Runs /usage periodically and publishes to Redis.
 */

const { spawn, execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const USAGE_CHANNEL = 'coders:usage-global';
const SESSION_NAME = 'coder-usage-monitor';
const CHECK_INTERVAL = 5 * 60 * 1000; // 5 minutes

// Parse arguments
const sessionId = process.argv[2] || SESSION_NAME;

let client;

async function connectRedis() {
  try {
    const { createClient } = await import('redis');
    client = createClient({ url: REDIS_URL });
    client.on('error', (err) => console.error('[Monitor] Redis error:', err.message));
    await client.connect();
    console.log('[Monitor] Connected to Redis');
    return true;
  } catch (err) {
    console.error('[Monitor] Failed to connect to Redis:', err.message);
    return false;
  }
}

function captureUsage() {
  try {
    // 1. Send /usage command
    execSync(`tmux send-keys -t "${sessionId}" "/usage" Enter`);
    
    // 2. Wait for UI to render (2 seconds)
    // We can't use await setTimeout in sync context easily without blocking event loop, 
    // but this is a dedicated process so blocking is fine-ish, or use setTimeout loop.
    
    setTimeout(() => {
      // 3. Capture pane
      const output = execSync(`tmux capture-pane -p -t "${sessionId}" -S -100 2>/dev/null`, { 
        encoding: 'utf8' 
      });

      // 4. Parse usage
      const stats = {};
      const fullText = output;
      
      const weeklyPercentMatch = fullText.match(/Current week (all models)\s*\n[█\s]*(\d+)%\s*used/);
      if (weeklyPercentMatch) {
        stats.weeklyLimitPercent = parseInt(weeklyPercentMatch[1], 10);
      }
      
      const weeklySonnetMatch = fullText.match(/Current week (Sonnet only)\s*\n[█\s]*(\d+)%\s*used/);
      if (weeklySonnetMatch) {
        stats.weeklySonnetPercent = parseInt(weeklySonnetMatch[1], 10);
      }

      console.log('[Monitor] Parsed stats:', stats);

      // 5. Publish to Redis
      if (client && client.isOpen && Object.keys(stats).length > 0) {
        client.publish(USAGE_CHANNEL, JSON.stringify(stats));
        // Also set a key for initial load
        client.set('coders:usage:global', JSON.stringify(stats));
      }

      // 6. Close usage overlay (Esc)
      execSync(`tmux send-keys -t "${sessionId}" Escape`);
      
    }, 3000); // 3s delay to ensure render
    
  } catch (e) {
    console.error('[Monitor] Error capturing usage:', e.message);
  }
}

async function start() {
  await connectRedis();

  console.log(`[Monitor] Starting usage checks every ${CHECK_INTERVAL/1000}s`);
  
  // Initial check after a delay to let Claude start
  setTimeout(captureUsage, 15000);
  
  // Periodic checks
  setInterval(captureUsage, CHECK_INTERVAL);
}

start();
