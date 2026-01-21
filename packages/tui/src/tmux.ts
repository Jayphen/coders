import { execSync, spawn, spawnSync } from 'child_process';
import type { Session, CoderPromise, HeartbeatData } from './types.js';

const PROMISE_KEY_PREFIX = 'coders:promise:';
const PANE_KEY_PREFIX = 'coders:pane:';
const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';

export function getCurrentSession(): string | null {
  try {
    return execSync('tmux display-message -p "#{session_name}"', { encoding: 'utf-8' }).trim();
  } catch {
    return null;
  }
}

/**
 * Query Redis for all session promises using redis-cli
 */
function getPromises(): Map<string, CoderPromise> {
  const promises = new Map<string, CoderPromise>();

  try {
    // Get all promise keys
    const keysOutput = execSync(
      `redis-cli --no-auth-warning KEYS "${PROMISE_KEY_PREFIX}*" 2>/dev/null`,
      { encoding: 'utf-8', timeout: 2000 }
    ).trim();

    if (!keysOutput) return promises;

    const keys = keysOutput.split('\n').filter(k => k.length > 0);

    for (const key of keys) {
      try {
        const value = execSync(
          `redis-cli --no-auth-warning GET "${key}" 2>/dev/null`,
          { encoding: 'utf-8', timeout: 1000 }
        ).trim();

        if (value) {
          const promise = JSON.parse(value) as CoderPromise;
          promises.set(promise.sessionId, promise);
        }
      } catch {
        // Ignore individual key errors
      }
    }
  } catch {
    // Redis not available or error - just return empty map
  }

  return promises;
}

/**
 * Query Redis for all session heartbeats using redis-cli
 */
function getHeartbeats(): Map<string, HeartbeatData> {
  const heartbeats = new Map<string, HeartbeatData>();

  try {
    // Get all pane keys
    const keysOutput = execSync(
      `redis-cli --no-auth-warning KEYS "${PANE_KEY_PREFIX}*" 2>/dev/null`,
      { encoding: 'utf-8', timeout: 2000 }
    ).trim();

    if (!keysOutput) return heartbeats;

    const keys = keysOutput.split('\n').filter(k => k.length > 0);

    for (const key of keys) {
      try {
        const value = execSync(
          `redis-cli --no-auth-warning GET "${key}" 2>/dev/null`,
          { encoding: 'utf-8', timeout: 1000 }
        ).trim();

        if (value) {
          const data = JSON.parse(value) as HeartbeatData;
          // Store by sessionId for easy lookup
          if (data.sessionId) {
            heartbeats.set(data.sessionId, data);
          }
        }
      } catch {
        // Ignore individual key errors
      }
    }
  } catch {
    // Redis not available or error
  }

  return heartbeats;
}

export async function getTmuxSessions(): Promise<Session[]> {
  try {
    // Get session info including pane title (which may contain task description)
    const output = execSync(
      'tmux list-sessions -F "#{session_name}|#{session_created}|#{pane_current_path}|#{pane_title}" 2>/dev/null',
      { encoding: 'utf-8' }
    );

    // Get promises from Redis
    const promises = getPromises();
    // Get heartbeats from Redis
    const heartbeats = getHeartbeats();

    const sessions: Session[] = output
      .trim()
      .split('\n')
      .filter(line => line.startsWith('coder-'))
      .map(line => {
        const [name, created, cwd, paneTitle] = line.split('|');

        // Parse tool from session name (coder-{tool}-{task})
        const parts = name.replace('coder-', '').split('-');
        const toolName = parts[0] as Session['tool'];
        const taskFromName = parts.slice(1).join('-');

        const isOrchestrator = name === 'coder-orchestrator';

        // Try to get a better task description:
        // 1. From pane title if it's not a default shell prompt
        // 2. Fall back to the slugified name
        let task = taskFromName || undefined;
        if (paneTitle && !paneTitle.includes('bash') && !paneTitle.includes('zsh') && paneTitle !== name) {
          // Pane title might have the full task
          task = paneTitle;
        }

        // Get promise data if available
        const promise = promises.get(name);
        
        // Get heartbeat data if available
        const heartbeat = heartbeats.get(name);
        
        // Determine heartbeat status
        let heartbeatStatus: Session['heartbeatStatus'] = 'dead';
        if (heartbeat) {
          const age = Date.now() - heartbeat.timestamp;
          if (age < 60000) { // < 1 min
            heartbeatStatus = 'healthy';
          } else if (age < 300000) { // < 5 min
            heartbeatStatus = 'stale';
          } else {
            heartbeatStatus = 'dead';
          }
        } else if (isOrchestrator) {
           // Orchestrator is assumed healthy if we are running the TUI (which usually means orchestrator is up)
           // But better to check if it has a heartbeat
           heartbeatStatus = 'healthy'; 
        }

        return {
          name,
          tool: ['claude', 'gemini', 'codex', 'opencode'].includes(toolName)
            ? toolName
            : 'unknown',
          task,
          cwd: cwd || process.cwd(),
          createdAt: created ? new Date(parseInt(created) * 1000) : undefined,
          isOrchestrator,
          heartbeatStatus,
          promise,
          hasPromise: !!promise,
          usage: heartbeat?.usage,
        };
      })
      // Sort: orchestrator first, then active sessions, then completed sessions
      .sort((a, b) => {
        if (a.isOrchestrator) return -1;
        if (b.isOrchestrator) return 1;
        // Active (no promise) before completed (has promise)
        if (a.hasPromise && !b.hasPromise) return 1;
        if (!a.hasPromise && b.hasPromise) return -1;
        // Within same category, sort by creation time (newest first)
        if (a.createdAt && b.createdAt) {
          return b.createdAt.getTime() - a.createdAt.getTime();
        }
        return 0;
      });

    return sessions;
  } catch {
    return [];
  }
}

export function attachSession(sessionName: string): void {
  const tmuxEnv = process.env.TMUX;

  if (!tmuxEnv) {
    // Outside tmux - attach directly
    spawn('tmux', ['attach', '-t', sessionName], {
      stdio: 'inherit',
      detached: true,
    });
    return;
  }

  // Switch to the session - user can switch back with Ctrl-b L (last session)
  execSync(`tmux switch-client -t "${sessionName}"`, { stdio: 'inherit' });
}

export function killSession(sessionName: string): void {
  try {
    execSync(`tmux kill-session -t "${sessionName}" 2>/dev/null`);
    // Also try to clean up the promise from Redis
    try {
      execSync(`redis-cli --no-auth-warning DEL "${PROMISE_KEY_PREFIX}${sessionName}" 2>/dev/null`);
    } catch {
      // Redis cleanup is best-effort
    }
  } catch {
    // Session may already be dead
  }
}

/**
 * Kill all sessions that have promises (completed sessions)
 * Returns the number of sessions killed
 */
export async function killCompletedSessions(): Promise<{ killed: string[]; failed: string[] }> {
  const sessions = await getTmuxSessions();
  const completedSessions = sessions.filter(s => s.hasPromise && !s.isOrchestrator);

  const killed: string[] = [];
  const failed: string[] = [];

  for (const session of completedSessions) {
    try {
      execSync(`tmux kill-session -t "${session.name}" 2>/dev/null`);
      // Clean up promise from Redis
      try {
        execSync(`redis-cli --no-auth-warning DEL "${PROMISE_KEY_PREFIX}${session.name}" 2>/dev/null`);
      } catch {
        // Redis cleanup is best-effort
      }
      killed.push(session.name);
    } catch {
      failed.push(session.name);
    }
  }

  return { killed, failed };
}

/**
 * Resume a session by clearing its promise
 */
export function resumeSession(sessionName: string): boolean {
  try {
    const result = execSync(
      `redis-cli --no-auth-warning DEL "${PROMISE_KEY_PREFIX}${sessionName}" 2>/dev/null`,
      { encoding: 'utf-8' }
    ).trim();
    return result === '1';
  } catch {
    return false;
  }
}
