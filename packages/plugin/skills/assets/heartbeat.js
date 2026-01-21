#!/usr/bin/env node

/**
 * Redis Heartbeat Script
 *
 * Publishes heartbeats to Redis for dashboard monitoring.
 * Run this in the background within each coder session.
 */

const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const HEARTBEAT_CHANNEL = 'coders:heartbeats';
const PANE_KEY_PREFIX = 'coders:pane:';
const INTERVAL_MS = 30000; // 30 seconds
const RECONNECT_BASE_MS = 500;
const RECONNECT_MAX_MS = 30000;

const sessionId = process.env.SESSION_ID || process.argv[2];
const paneId = process.env.PANE_ID || `pane-${Date.now()}`;
const parentSessionId = process.env.CODERS_PARENT_SESSION_ID || null;

if (!sessionId) {
  console.error('Usage: heartbeat.js <session-id>');
  process.exit(1);
}

let client;
let connectPromise = null;
let reconnectTimer = null;
let reconnectAttempts = 0;
let isShuttingDown = false;

function getReconnectDelay(attempt) {
  const jitter = Math.floor(Math.random() * 250);
  return Math.min(RECONNECT_BASE_MS * Math.pow(2, attempt), RECONNECT_MAX_MS) + jitter;
}

function scheduleReconnect(reason) {
  if (isShuttingDown || reconnectTimer) return;
  const delay = getReconnectDelay(reconnectAttempts++);
  console.warn(`[Heartbeat] Redis disconnected (${reason}). Reconnecting in ${delay}ms`);
  reconnectTimer = setTimeout(async () => {
    reconnectTimer = null;
    if (isShuttingDown) return;
    try {
      await connect();
    } catch (err) {
      scheduleReconnect('retry');
    }
  }, delay);
}

async function connect() {
  if (client?.isOpen) return;
  if (connectPromise) return connectPromise;

  connectPromise = (async () => {
    try {
      // Use dynamic import to avoid top-level import errors
      const { createClient } = await import('redis');
      if (client && client.isOpen) {
        await client.quit();
      }
      client = createClient({ url: REDIS_URL });
      client.on('error', (err) => console.error('[Heartbeat] Redis error:', err.message));
      client.on('end', () => {
        if (isShuttingDown) return;
        scheduleReconnect('end');
      });
      client.on('ready', () => {
        reconnectAttempts = 0;
      });
      await client.connect();
      console.log(`[Heartbeat] Connected to Redis for session: ${sessionId}`);
    } catch (err) {
      console.error('[Heartbeat] Failed to connect to redis:', err.message);
      scheduleReconnect('connect-failed');
      throw err;
    }
  })().finally(() => {
    connectPromise = null;
  });

  return connectPromise;
}

async function ensureConnected() {
  if (client?.isOpen) return true;
  try {
    await connect();
    return client?.isOpen || false;
  } catch {
    return false;
  }
}

async function publishHeartbeat() {
  if (!(await ensureConnected())) return;

  const data = {
    paneId,
    sessionId,
    timestamp: Date.now(),
    parentSessionId
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
  await connect().catch(() => {});
  await publishHeartbeat();

  // Then publish every interval
  setInterval(publishHeartbeat, INTERVAL_MS);

  console.log(`[Heartbeat] Publishing every ${INTERVAL_MS / 1000}s`);
}

// Handle shutdown gracefully
process.on('SIGINT', async () => {
  console.log('\n[Heartbeat] Shutting down...');
  isShuttingDown = true;
  if (reconnectTimer) clearTimeout(reconnectTimer);
  if (client?.isOpen) await client.quit();
  process.exit(0);
});

start().catch(console.error);
