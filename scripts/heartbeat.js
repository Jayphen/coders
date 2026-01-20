#!/usr/bin/env node

/**
 * Redis Heartbeat Script
 *
 * Publishes heartbeats to Redis for dashboard monitoring.
 * Run this in the background within each coder session.
 */

import { createClient } from 'redis';

const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const HEARTBEAT_CHANNEL = 'coders:heartbeats';
const PANE_KEY_PREFIX = 'coders:pane:';
const INTERVAL_MS = 30000; // 30 seconds

const sessionId = process.env.SESSION_ID || process.argv[2];
const paneId = process.env.PANE_ID || `pane-${Date.now()}`;

if (!sessionId) {
  console.error('Usage: heartbeat.js <session-id>');
  process.exit(1);
}

let client;

async function connect() {
  client = createClient({ url: REDIS_URL });
  client.on('error', (err) => console.error('[Heartbeat] Redis error:', err.message));
  await client.connect();
  console.log(`[Heartbeat] Connected to Redis for session: ${sessionId}`);
}

async function publishHeartbeat() {
  if (!client?.isOpen) return;

  const data = {
    paneId,
    sessionId,
    timestamp: Date.now(),
    status: 'alive',
    lastActivity: 'working'
  };

  try {
    // Publish to channel
    await client.publish(HEARTBEAT_CHANNEL, JSON.stringify(data));

    // Set expiring key
    await client.set(`${PANE_KEY_PREFIX}${paneId}`, JSON.stringify(data), {
      EX: 150 // 2.5 min TTL
    });

    console.log(`[Heartbeat] Published at ${new Date().toLocaleTimeString()}`);
  } catch (e) {
    console.error('[Heartbeat] Publish failed:', e.message);
  }
}

async function start() {
  await connect();

  // Publish immediately
  await publishHeartbeat();

  // Then publish every interval
  setInterval(publishHeartbeat, INTERVAL_MS);

  console.log(`[Heartbeat] Publishing every ${INTERVAL_MS / 1000}s`);
}

// Handle shutdown gracefully
process.on('SIGINT', async () => {
  console.log('\n[Heartbeat] Shutting down...');
  if (client?.isOpen) await client.quit();
  process.exit(0);
});

start().catch(console.error);
