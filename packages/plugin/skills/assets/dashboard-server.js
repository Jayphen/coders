#!/usr/bin/env node

/**
 * Coders Dashboard Server
 *
 * A web-based dashboard to monitor all spawned AI coder sessions.
 * Shows heartbeats, status, and allows interaction with sessions.
 */

import http from 'http';
import { execSync } from 'child_process';
import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const ORCHESTRATOR_SESSION_ID = 'coder-orchestrator';

const PORT = process.env.DASHBOARD_PORT || 3030;
const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const HEARTBEAT_CHANNEL = 'coders:heartbeats';
const PROMISES_CHANNEL = 'coders:promises';
const PANE_KEY_PREFIX = 'coders:pane:';
const SESSION_META_KEY_PREFIX = 'coders:session-meta:';
const PROMISE_KEY_PREFIX = 'coders:promise:';
const RESPONSE_SAMPLE_LINES = 20;
const RESPONSE_SAMPLE_INTERVAL_MS = 5000;
const RECONNECT_BASE_MS = 500;
const RECONNECT_MAX_MS = 30000;

// Store active sessions and SSE clients
const sessions = new Map();
const sseClients = new Set();
const responseStates = new Map();
const sessionMetadataCache = new Map(); // Cache for displayName and other metadata
const promisesCache = new Map(); // Cache for session promises (completion summaries)
let lastResponseSampleAt = 0;

// Clean stale sessions and orphaned heartbeat processes every 30 seconds
setInterval(() => {
  const now = Date.now();
  const STALE_THRESHOLD = 5 * 60 * 1000; // 5 minutes

  // Get current tmux sessions
  const tmuxSessions = getTmuxSessions();
  const activeTmuxSessionIds = new Set(tmuxSessions.map(s => s.sessionId));

  for (const [paneId, session] of sessions.entries()) {
    // Remove if session no longer exists in tmux
    if (!activeTmuxSessionIds.has(session.sessionId)) {
      sessions.delete(paneId);
      responseStates.delete(session.sessionId);
      console.log(`[Cleanup] Removed dead session: ${session.sessionId}`);

      // Kill orphaned heartbeat process for this session
      try {
        execSync(`pkill -f "heartbeat.js.*${session.sessionId}" 2>/dev/null || true`, { encoding: 'utf8' });
        console.log(`[Cleanup] Killed orphaned heartbeat for: ${session.sessionId}`);
      } catch {}
    }
    // Also remove if stale (no heartbeat in 5 minutes)
    else if (now - session.lastSeen > STALE_THRESHOLD) {
      sessions.delete(paneId);
      console.log(`[Cleanup] Removed stale session: ${paneId}`);
    }
  }

  // Broadcast updated session list after cleanup
  broadcast('sessions', getFormattedSessionList());
}, 30000);

// Redis clients
let redisClient;
let redisSubscriber;
let reconnectTimer = null;
let reconnectAttempts = 0;
let reconnectInFlight = false;
let isShuttingDown = false;

function getReconnectDelay(attempt) {
  const jitter = Math.floor(Math.random() * 250);
  return Math.min(RECONNECT_BASE_MS * Math.pow(2, attempt), RECONNECT_MAX_MS) + jitter;
}

function scheduleReconnect(reason) {
  if (isShuttingDown || reconnectTimer) return;
  const delay = getReconnectDelay(reconnectAttempts++);
  console.warn(`[Redis] Disconnected (${reason}). Reconnecting in ${delay}ms`);
  reconnectTimer = setTimeout(async () => {
    reconnectTimer = null;
    if (isShuttingDown) return;
    await reconnectRedis();
  }, delay);
}

async function cleanupRedisClients() {
  try {
    if (redisSubscriber?.isOpen) {
      await redisSubscriber.unsubscribe();
      await redisSubscriber.quit();
    }
  } catch {}
  try {
    if (redisClient?.isOpen) {
      await redisClient.quit();
    }
  } catch {}
  redisSubscriber = null;
  redisClient = null;
}

// Broadcast to all connected SSE clients
function broadcast(type, data) {
  const message = `data: ${JSON.stringify({ type, data })}\n\n`;
  for (const client of sseClients) {
    client.write(message);
  }
}

// Generate the full session list with status
function getFormattedSessionList() {
  const tmuxSessions = getTmuxSessions();
  const now = Date.now();

  refreshResponseTimes(tmuxSessions);

  // Clean up sessions Map: remove heartbeat data for sessions no longer in tmux
  const activeTmuxSessionIds = new Set(tmuxSessions.map(s => s.sessionId));
  for (const [paneId, session] of sessions.entries()) {
    if (!activeTmuxSessionIds.has(session.sessionId)) {
      sessions.delete(paneId);
      console.log(`[Cleanup] Removed heartbeat data for dead session: ${session.sessionId}`);
    }
  }

  const sessionList = tmuxSessions.map(tmux => {
    const heartbeat = Array.from(sessions.values())
      .find(s => s.sessionId === tmux.sessionId);
    const responseState = responseStates.get(tmux.sessionId);

    const lastSeen = heartbeat?.lastSeen || 0;
    const hasRecentHeartbeat = heartbeat && (now - lastSeen < 180000); // 3 min threshold
    const isOrchestrator = tmux.sessionId === ORCHESTRATOR_SESSION_ID;

    // Compute meaningful activity based on output changes
    const lastResponseAt = responseState?.lastResponseAt || heartbeat?.lastResponseAt || null;
    let computedActivity = 'unknown';
    if (lastResponseAt) {
      const responseAge = now - lastResponseAt;
      if (responseAge <= 60000) {
        computedActivity = 'active';  // Output changed in last 60 seconds (AI is producing output)
      } else if (responseAge <= 5 * 60 * 1000) {
        computedActivity = 'idle';    // No output in 1-5 minutes (AI might be thinking or waiting)
      } else {
        computedActivity = 'stale';   // No output in 5+ minutes (session might be stuck or done)
      }
    } else if (hasRecentHeartbeat) {
      computedActivity = 'waiting';   // Heartbeat alive but no output tracked yet
    }

    // Use activity as the primary status since it's more meaningful
    // "alive" is redundant - if a session is in the list, tmux says it exists
    const status = computedActivity;

    // Get displayName from metadata cache
    const metadata = sessionMetadataCache.get(tmux.sessionId);
    const displayName = metadata?.displayName || null;
    const task = metadata?.task || null;

    // Get promise (completion) data
    const promise = promisesCache.get(tmux.sessionId);
    const hasPromise = !!promise;

    return {
      sessionId: tmux.sessionId,
      displayName: displayName,
      task: task,
      windows: tmux.windows,
      status: status,
      hasHeartbeat: hasRecentHeartbeat,
      lastHeartbeat: heartbeat?.timestamp || null,
      lastSeen: lastSeen || null,
      lastResponseAt: lastResponseAt,
      paneId: heartbeat?.paneId || null,
      lastActivity: computedActivity,
      isOrchestrator: isOrchestrator,
      cwd: getSessionCwd(tmux.sessionId),
      parentSessionId: heartbeat?.parentSessionId || null,
      // Promise data
      promise: promise || null,
      hasPromise: hasPromise
    };
  });

  // Build child session lookup map
  const childrenMap = new Map();
  sessionList.forEach(session => {
    if (session.parentSessionId) {
      if (!childrenMap.has(session.parentSessionId)) {
        childrenMap.set(session.parentSessionId, []);
      }
      childrenMap.get(session.parentSessionId).push(session.sessionId);
    }
  });

  // Add children info to each session
  sessionList.forEach(session => {
    session.children = childrenMap.get(session.sessionId) || [];
  });

  // Sort sessions: orchestrator first, then root sessions (no parent), then by sessionId
  return sessionList.sort((a, b) => {
    if (a.isOrchestrator) return -1;
    if (b.isOrchestrator) return 1;
    // Root sessions (no parent) come before child sessions
    const aIsRoot = !a.parentSessionId;
    const bIsRoot = !b.parentSessionId;
    if (aIsRoot && !bIsRoot) return -1;
    if (!aIsRoot && bIsRoot) return 1;
    return a.sessionId.localeCompare(b.sessionId);
  });
}

// Connect to Redis (with dynamic import)
async function connectRedis() {
  try {
    await cleanupRedisClients();
    // Use dynamic import to avoid top-level import errors
    const { createClient } = await import('redis');

    redisClient = createClient({ url: REDIS_URL });
    redisSubscriber = redisClient.duplicate();

    redisClient.on('error', (err) => console.error('[Redis] Error:', err));
    redisSubscriber.on('error', (err) => console.error('[Redis Sub] Error:', err));
    redisClient.on('end', () => {
      if (isShuttingDown) return;
      scheduleReconnect('client-end');
    });
    redisSubscriber.on('end', () => {
      if (isShuttingDown) return;
      scheduleReconnect('subscriber-end');
    });
    redisClient.on('ready', () => {
      reconnectAttempts = 0;
    });

    await redisClient.connect();
    await redisSubscriber.connect();

    console.log('[Redis] Connected to', REDIS_URL);
  } catch (err) {
    console.error('[Redis] Failed to load redis module:', err.message);
    throw err;
  }
}

async function reconnectRedis() {
  if (reconnectInFlight || isShuttingDown) return;
  reconnectInFlight = true;
  try {
    await connectRedis();
    await subscribeToHeartbeats();
    await subscribeToPromises();
    await loadInitialState();
    await loadSessionMetadata();
    await loadPromises();
    reconnectAttempts = 0;
  } catch (err) {
    console.error('[Redis] Reconnect failed:', err.message);
    scheduleReconnect('retry');
  } finally {
    reconnectInFlight = false;
  }
}

// Subscribe to heartbeats
async function subscribeToHeartbeats() {
  await redisSubscriber.subscribe(HEARTBEAT_CHANNEL, (message) => {
    try {
      const data = JSON.parse(message);
      sessions.set(data.paneId, {
        ...data,
        lastSeen: Date.now()
      });
      // Broadcast update immediately
      broadcast('sessions', getFormattedSessionList());
    } catch (e) {
      console.error('[Heartbeat] Parse error:', e);
    }
  });

  console.log('[Redis] Subscribed to heartbeats');
}

// Subscribe to promises channel
async function subscribeToPromises() {
  await redisSubscriber.subscribe(PROMISES_CHANNEL, (message) => {
    try {
      const data = JSON.parse(message);
      promisesCache.set(data.sessionId, data);
      console.log(`[Promise] Received promise for ${data.sessionId}: ${data.summary}`);
      // Broadcast update immediately
      broadcast('sessions', getFormattedSessionList());
    } catch (e) {
      console.error('[Promise] Parse error:', e);
    }
  });

  console.log('[Redis] Subscribed to promises');
}

// Load session metadata (displayName, etc.) from Redis
async function loadSessionMetadata() {
  try {
    const keys = await redisClient.keys(`${SESSION_META_KEY_PREFIX}*`);
    for (const key of keys) {
      try {
        const value = await redisClient.get(key);
        if (value) {
          const sessionId = key.replace(SESSION_META_KEY_PREFIX, '');
          const data = JSON.parse(value);
          sessionMetadataCache.set(sessionId, data);
        }
      } catch (e) {
        console.error('[Metadata Load] Error reading key', key, e);
      }
    }
    console.log(`[Redis] Loaded metadata for ${sessionMetadataCache.size} sessions`);
  } catch (e) {
    console.error('[Metadata Load] Failed to list keys:', e);
  }
}

// Load promises (completion summaries) from Redis
async function loadPromises() {
  try {
    const keys = await redisClient.keys(`${PROMISE_KEY_PREFIX}*`);
    for (const key of keys) {
      try {
        const value = await redisClient.get(key);
        if (value) {
          const sessionId = key.replace(PROMISE_KEY_PREFIX, '');
          const data = JSON.parse(value);
          promisesCache.set(sessionId, data);
        }
      } catch (e) {
        console.error('[Promise Load] Error reading key', key, e);
      }
    }
    console.log(`[Redis] Loaded ${promisesCache.size} promises`);
  } catch (e) {
    console.error('[Promise Load] Failed to list keys:', e);
  }
}

// Load initial state from Redis keys
async function loadInitialState() {
  try {
    const keys = await redisClient.keys(`${PANE_KEY_PREFIX}*`);
    for (const key of keys) {
      try {
        const value = await redisClient.get(key);
        if (value) {
          const data = JSON.parse(value);
          // usage of data.timestamp for lastSeen is an approximation
          sessions.set(data.paneId, {
            ...data,
            lastSeen: Date.now()
          });
        }
      } catch (e) {
        console.error('[Initial Load] Error reading key', key, e);
      }
    }
    console.log(`[Redis] Loaded ${keys.length} sessions from cache`);
  } catch (e) {
    console.error('[Initial Load] Failed to list keys:', e);
  }
}

// Get session current working directory
function getSessionCwd(sessionId) {
  try {
    const output = execSync(
      `tmux display-message -t ${sessionId} -p "#{pane_current_path}" 2>/dev/null || echo ""`,
      { encoding: 'utf8' }
    );
    return output.trim() || null;
  } catch (e) {
    return null;
  }
}

// Get tmux session info
function getTmuxSessions() {
  try {
    const output = execSync('tmux list-sessions 2>/dev/null || echo ""', { encoding: 'utf8' });
    return output
      .split('\n')
      .filter(line => line.includes('coder-'))
      .map(line => {
        const match = line.match(/^(coder-[^:]+):\s*(\d+)\s*windows/);
        if (match) {
          return {
            sessionId: match[1],
            windows: parseInt(match[2]),
            raw: line
          };
        }
        return null;
      })
      .filter(Boolean);
  } catch (e) {
    return [];
  }
}

// Get session output
function getSessionOutput(sessionId, lines = 30) {
  try {
    const output = execSync(
      `tmux capture-pane -t ${sessionId} -p -e -S -${lines} 2>/dev/null || echo "Session not found"`,
      { encoding: 'utf8' }
    );
    return output;
  } catch (e) {
    return 'Error capturing output';
  }
}

function getSessionOutputSnapshot(sessionId, lines = RESPONSE_SAMPLE_LINES) {
  try {
    const output = execSync(
      `tmux capture-pane -t ${sessionId} -p -S -${lines} 2>/dev/null || echo ""`,
      { encoding: 'utf8' }
    );
    return output.trimEnd();
  } catch (e) {
    return '';
  }
}

function refreshResponseTimes(tmuxSessions) {
  const now = Date.now();
  if (now - lastResponseSampleAt < RESPONSE_SAMPLE_INTERVAL_MS) {
    return;
  }
  lastResponseSampleAt = now;

  const activeSessionIds = new Set(tmuxSessions.map(session => session.sessionId));
  for (const sessionId of responseStates.keys()) {
    if (!activeSessionIds.has(sessionId)) {
      responseStates.delete(sessionId);
    }
  }

  for (const session of tmuxSessions) {
    const snapshot = getSessionOutputSnapshot(session.sessionId);
    if (!snapshot) continue;

    const state = responseStates.get(session.sessionId) || {
      lastSignature: null,
      lastResponseAt: null
    };

    if (state.lastSignature === null) {
      state.lastSignature = snapshot;
      if (state.lastResponseAt === null) {
        state.lastResponseAt = now;
      }
    } else if (state.lastSignature !== snapshot) {
      state.lastSignature = snapshot;
      state.lastResponseAt = now;
    }

    responseStates.set(session.sessionId, state);
  }
}

function isValidSessionId(sessionId) {
  return typeof sessionId === 'string' && /^coder-[A-Za-z0-9._-]+$/.test(sessionId);
}

// Send message to session
function sendMessageToSession(sessionId, message) {
  try {
    // Two-step send for TUI compatibility
    execSync(`tmux send-keys -t ${sessionId} "${message.replace(/"/g, '\\"')}"`);
    execSync(`sleep 0.5`);
    execSync(`tmux send-keys -t ${sessionId} C-m`);
    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

// Allowed special keys (tmux send-keys format)
const ALLOWED_KEYS = new Set([
  // Control keys
  'C-c', 'C-d', 'C-o', 'C-b', 'C-l', 'C-g', 'C-r', 'C-y', 'C-t', 'C-s',
  'C-x', 'C-j', 'C-k', 'C-p', 'C-z', 'C-a', 'C-e', 'C-u', 'C-w', 'C-n',
  // Meta/Alt keys
  'M-m', 'M-p', 'M-t', 'M-b', 'M-f',
  // Special keys
  'Escape', 'Enter', 'Tab', 'BTab', 'Up', 'Down', 'Left', 'Right',
  'Home', 'End', 'PageUp', 'PageDown', 'Space',
  // Function keys
  'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12',
  // Shift+Function keys
  'S-F1', 'S-F2', 'S-F3', 'S-F4', 'S-F5', 'S-F6', 'S-F7', 'S-F8', 'S-F9', 'S-F10', 'S-F11', 'S-F12',
]);

// Send special key to session
function sendKeyToSession(sessionId, key, delay = 0) {
  if (!isValidSessionId(sessionId)) {
    return { success: false, error: 'Invalid session id' };
  }

  // Parse keys (may be space-separated for multi-key sequences like "Escape Escape")
  const keys = key.split(' ').filter(k => k.length > 0);

  // Validate all keys
  for (const k of keys) {
    if (!ALLOWED_KEYS.has(k)) {
      return { success: false, error: `Invalid key: ${k}` };
    }
  }

  try {
    for (let i = 0; i < keys.length; i++) {
      execSync(`tmux send-keys -t ${sessionId} ${keys[i]}`);
      // Add delay between keys if specified (for multi-key sequences)
      if (delay > 0 && i < keys.length - 1) {
        execSync(`sleep ${delay / 1000}`);
      }
    }
    return { success: true, keysSent: keys };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

function killSession(sessionId) {
  if (!isValidSessionId(sessionId)) {
    return { success: false, error: 'Invalid session id' };
  }

  if (sessionId === ORCHESTRATOR_SESSION_ID) {
    return { success: false, error: 'Orchestrator session cannot be killed' };
  }

  // Kill associated heartbeat process first
  try {
    execSync(`pkill -f "heartbeat.js.*${sessionId}"`);
  } catch {} // May not exist

  try {
    execSync(`tmux kill-session -t "${sessionId}"`);

    // Clean up sessions Map entries for this sessionId
    for (const [paneId, session] of sessions.entries()) {
      if (session.sessionId === sessionId) {
        sessions.delete(paneId);
      }
    }

    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

// HTTP Server
const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);

  // CORS headers
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }

  // Serve dashboard HTML
  if (url.pathname === '/' || url.pathname === '/index.html') {
    const html = readFileSync(join(__dirname, 'dashboard.html'), 'utf8');
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(html);
    return;
  }

  // API: Get all sessions
  if (url.pathname === '/api/sessions') {
    const sessionList = getFormattedSessionList();
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(sessionList));
    return;
  }

  // API: Get session output
  if (url.pathname === '/api/output') {
    const sessionId = url.searchParams.get('session');
    const lines = parseInt(url.searchParams.get('lines') || '30');

    if (!sessionId) {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Missing session parameter' }));
      return;
    }

    const output = getSessionOutput(sessionId, lines);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ output }));
    return;
  }

  // API: Send message to session
  if (url.pathname === '/api/send' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', () => {
      try {
        const { session, message } = JSON.parse(body);
        const result = sendMessageToSession(session, message);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(result));
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: false, error: e.message }));
      }
    });
    return;
  }

  // API: Send special key to session
  if (url.pathname === '/api/send-key' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', () => {
      try {
        const { session, key, delay } = JSON.parse(body);
        if (!session || !key) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ success: false, error: 'Missing session or key parameter' }));
          return;
        }
        const result = sendKeyToSession(session, key, delay || 0);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(result));
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: false, error: e.message }));
      }
    });
    return;
  }

  // API: Kill session
  if (url.pathname === '/api/kill' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', () => {
      try {
        const { session } = JSON.parse(body);
        const result = killSession(session);
        if (result.success) {
          broadcast('sessions', getFormattedSessionList());
        }
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(result));
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: false, error: e.message }));
      }
    });
    return;
  }

  // API: Bulk kill completed sessions (sessions with promises)
  if (url.pathname === '/api/kill-completed' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
      try {
        // Find all sessions with promises (completed sessions)
        const sessionList = getFormattedSessionList();
        const completedSessions = sessionList.filter(s => s.hasPromise && !s.isOrchestrator);

        const killed = [];
        const failed = [];

        for (const session of completedSessions) {
          const result = killSession(session.sessionId);
          if (result.success) {
            // Also clean up the promise from cache and Redis
            promisesCache.delete(session.sessionId);
            try {
              await redisClient.del(`${PROMISE_KEY_PREFIX}${session.sessionId}`);
            } catch (e) {
              console.error(`[Bulk Kill] Failed to delete promise for ${session.sessionId}:`, e);
            }
            killed.push(session.sessionId);
          } else {
            failed.push({ sessionId: session.sessionId, error: result.error });
          }
        }

        if (killed.length > 0) {
          broadcast('sessions', getFormattedSessionList());
        }

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          success: true,
          killed,
          failed,
          totalKilled: killed.length,
          totalFailed: failed.length
        }));
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: false, error: e.message }));
      }
    });
    return;
  }

  // API: Resume a session (clear its promise)
  if (url.pathname === '/api/resume' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
      try {
        const { session } = JSON.parse(body);
        if (!isValidSessionId(session)) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ success: false, error: 'Invalid session id' }));
          return;
        }

        // Remove promise from cache and Redis
        promisesCache.delete(session);
        try {
          await redisClient.del(`${PROMISE_KEY_PREFIX}${session}`);
        } catch (e) {
          console.error(`[Resume] Failed to delete promise for ${session}:`, e);
        }

        broadcast('sessions', getFormattedSessionList());

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: true, sessionId: session }));
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: false, error: e.message }));
      }
    });
    return;
  }

  // API: Server-Sent Events for live updates
  if (url.pathname === '/api/events') {
    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive'
    });

    // Add to active clients
    sseClients.add(res);

    // Send initial data immediately
    res.write(`data: ${JSON.stringify({ type: 'sessions', data: getFormattedSessionList() })}\n\n`);

    // Keep alive / reconcile interval (every 5s)
    // This detects new sessions that haven't sent heartbeats yet, or dead ones
    const interval = setInterval(() => {
      res.write(`data: ${JSON.stringify({ type: 'sessions', data: getFormattedSessionList() })}\n\n`);
    }, 5000);

    req.on('close', () => {
      clearInterval(interval);
      sseClients.delete(res);
    });
    return;
  }

  // 404
  res.writeHead(404, { 'Content-Type': 'text/plain' });
  res.end('Not Found');
});

// Start server
async function start() {
  await connectRedis();
  await subscribeToHeartbeats();
  await subscribeToPromises();
  await loadInitialState();
  await loadSessionMetadata();
  await loadPromises();

  server.listen(PORT, () => {
    console.log(`\n🎯 Coders Dashboard running at http://localhost:${PORT}`);
    console.log(`📊 Monitoring sessions with Redis heartbeats\n`);
  });
}

// Handle shutdown gracefully
async function shutdown() {
  console.log('\n[Dashboard] Shutting down...');
  isShuttingDown = true;
  if (reconnectTimer) clearTimeout(reconnectTimer);

  // Close all SSE connections
  for (const client of sseClients) {
    client.end();
  }
  sseClients.clear();

  // Disconnect Redis clients
  try {
    await cleanupRedisClients();
    console.log('[Dashboard] Redis connections closed');
  } catch (e) {
    console.error('[Dashboard] Error closing Redis:', e.message);
  }

  server.close(() => {
    console.log('[Dashboard] Server closed');
    process.exit(0);
  });
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

process.on('uncaughtException', (err) => {
  console.error('[Dashboard] Uncaught Exception:', err);
});

process.on('unhandledRejection', (reason, promise) => {
  console.error('[Dashboard] Unhandled Rejection at:', promise, 'reason:', reason);
});

start().catch(err => {
  console.error('[Dashboard] Start failed:', err);
  process.exit(1);
});
