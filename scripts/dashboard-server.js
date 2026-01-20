#!/usr/bin/env node

/**
 * Coders Dashboard Server
 *
 * A web-based dashboard to monitor all spawned AI coder sessions.
 * Shows heartbeats, status, and allows interaction with sessions.
 */

import http from 'http';
import { createClient } from 'redis';
import { execSync } from 'child_process';
import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const PORT = process.env.DASHBOARD_PORT || 3030;
const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';
const HEARTBEAT_CHANNEL = 'coders:heartbeats';
const PANE_KEY_PREFIX = 'coders:pane:';

// Store active sessions
const sessions = new Map();

// Redis clients
let redisClient;
let redisSubscriber;

// Connect to Redis
async function connectRedis() {
  redisClient = createClient({ url: REDIS_URL });
  redisSubscriber = redisClient.duplicate();

  redisClient.on('error', (err) => console.error('[Redis] Error:', err));
  redisSubscriber.on('error', (err) => console.error('[Redis Sub] Error:', err));

  await redisClient.connect();
  await redisSubscriber.connect();

  console.log('[Redis] Connected to', REDIS_URL);
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
    } catch (e) {
      console.error('[Heartbeat] Parse error:', e);
    }
  });

  console.log('[Redis] Subscribed to heartbeats');
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
      `tmux capture-pane -t ${sessionId} -p -S -${lines} 2>/dev/null || echo "Session not found"`,
      { encoding: 'utf8' }
    );
    return output;
  } catch (e) {
    return 'Error capturing output';
  }
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
    const tmuxSessions = getTmuxSessions();
    const now = Date.now();

    const sessionList = tmuxSessions.map(tmux => {
      const heartbeat = Array.from(sessions.values())
        .find(s => s.sessionId === tmux.sessionId);

      const lastSeen = heartbeat?.lastSeen || 0;
      const isAlive = heartbeat && (now - lastSeen < 180000); // 3 min threshold

      return {
        sessionId: tmux.sessionId,
        windows: tmux.windows,
        status: isAlive ? 'alive' : 'unknown',
        lastHeartbeat: heartbeat?.timestamp || null,
        lastSeen: lastSeen || null,
        paneId: heartbeat?.paneId || null,
        lastActivity: heartbeat?.lastActivity || 'unknown'
      };
    });

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

  // API: Server-Sent Events for live updates
  if (url.pathname === '/api/events') {
    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive'
    });

    const interval = setInterval(() => {
      const tmuxSessions = getTmuxSessions();
      res.write(`data: ${JSON.stringify({ type: 'sessions', data: tmuxSessions })}\n\n`);
    }, 2000);

    req.on('close', () => {
      clearInterval(interval);
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

  server.listen(PORT, () => {
    console.log(`\n🎯 Coders Dashboard running at http://localhost:${PORT}`);
    console.log(`📊 Monitoring sessions with Redis heartbeats\n`);
  });
}

start().catch(console.error);
