#!/usr/bin/env node

/**
 * Redis Heartbeat Script
 *
 * Publishes heartbeats to Redis for dashboard monitoring.
 * Run this in the background within each coder session.
 */

const { execSync } = require('child_process');
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

function getUsageStats() {
  try {
    // Capture the last 100 lines of the pane
    // Use the session ID to target the pane (assuming one pane per session window for now)
    // or use the paneId if it corresponds to a tmux pane id (e.g. %1)
    // If paneId is just a random string, we rely on sessionId
    const target = sessionId.startsWith('coder-') ? sessionId : sessionId;
    
    // Check if we can identify the pane
    // If paneId starts with %, use it. Otherwise use session target
    const tmuxTarget = paneId.startsWith('%') ? paneId : target;

    const output = execSync(`tmux capture-pane -p -t "${tmuxTarget}" -S -100 2>/dev/null`, { 
      encoding: 'utf8',
      timeout: 1000 
    });

    if (!output) return null;

    // Parse for usage stats
    // Patterns to look for:
    // "Total cost: $0.05"
    // "Tokens: 1234"
    // "Context tokens: 1000"
    // "Generated tokens: 50"
    // Claude TUI: "Current session" -> "98% used"
    
    const stats = {};
    const lines = output.split('\n');
    
    // Check for Claude TUI visual usage patterns (multi-line)
    const fullText = lines.join('\n');
    const sessionPercentMatch = fullText.match(/Current session\s*\n[█\s]*(\d+)%\s*used/);
    if (sessionPercentMatch) {
      stats.sessionLimitPercent = parseInt(sessionPercentMatch[1], 10);
    }
    
    const weeklyPercentMatch = fullText.match(/Current week \(all models\)\s*\n[█\s]*(\d+)%\s*used/);
    if (weeklyPercentMatch) {
      stats.weeklyLimitPercent = parseInt(weeklyPercentMatch[1], 10);
    }
    
    // Reverse iterate to find the most recent stats (text based)
    for (let i = lines.length - 1; i >= 0; i--) {
      const line = lines[i].trim();
      
      // Cost
      if (!stats.cost) {
        const costMatch = line.match(/Total cost:\s*\$([0-9.]+)/i) || 
                          line.match(/Cost:\s*\$([0-9.]+)/i);
        if (costMatch) {
          stats.cost = costMatch[1];
        }
      }

      // Tokens
      if (!stats.tokens) {
        // Look for comprehensive token stats first
        const tokenMatch = line.match(/Tokens:\s*(\d+)/i) ||
                           line.match(/Total tokens:\s*(\d+)/i);
        if (tokenMatch) {
          stats.tokens = parseInt(tokenMatch[1], 10);
        }
      }
      
      if (!stats.apiCalls) {
        const callsMatch = line.match(/API calls:\s*(\d+)/i);
        if (callsMatch) {
          stats.apiCalls = parseInt(callsMatch[1], 10);
        }
      }

      // Stop if we found everything or went back too far (e.g. 50 lines)
      if ((stats.cost && stats.tokens) || lines.length - i > 50) {
        break;
      }
    }

    return Object.keys(stats).length > 0 ? stats : null;
  } catch (e) {
    // console.error('Failed to get usage stats:', e.message);
    return null;
  }
}

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

  const usage = getUsageStats();

  const data = {
    paneId,
    sessionId,
    timestamp: Date.now(),
    parentSessionId,
    usage
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
