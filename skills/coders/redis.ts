/**
 * Redis Manager for Coders Skill
 * 
 * Provides heartbeat publishing, dead-letter handling, and pane ID injection
 * for tmux-based AI agent sessions.
 */

import { createClient, RedisClientType } from 'redis';
import { execSync, spawn, ChildProcess } from 'child_process';
import * as os from 'os';

const SNAPSHOT_DIR = os.homedir() + '/.coders/snapshots';
export const HEARTBEAT_CHANNEL = 'coders:heartbeats';
export const DEAD_LETTER_KEY = 'coders:dead-letter';
const PANE_ID_KEY_PREFIX = 'coders:pane:';

// Types
export interface RedisConfig {
  url?: string;
  host?: string;
  port?: number;
  password?: string;
}

export interface PaneInfo {
  paneId: string;
  sessionId: string;
  windowId: string;
  tool: string;
  task: string;
  pid: number;
  createdAt: number;
}

export interface HeartbeatData {
  paneId: string;
  sessionId: string;
  timestamp: number;
  status: 'alive' | 'processing' | 'idle';
  lastActivity: string;
}

export interface SnapshotData {
  version: number;
  timestamp: number;
  sessionId: string;
  panes: PaneInfo[];
  layout: any;
}

/**
 * Get pane ID from environment or generate one
 */
export function getPaneId(): string {
  return process.env.CODERS_PANE_ID || `pane-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * Inject pane ID into system context for the agent
 */
export function injectPaneIdContext(systemPrompt: string, paneId: string): string {
  const contextBlock = `
<!-- SYSTEM CONTEXT -->
<!-- PANE_ID: ${paneId} -->
<!-- HEARTBEAT_CHANNEL: ${HEARTBEAT_CHANNEL} -->

Your session has a unique pane ID for heartbeat monitoring.
Publish heartbeats to Redis channel "${HEARTBEAT_CHANNEL}" with your pane ID.
If you become unresponsive for >2 minutes, you will be auto-respawned.

To publish a heartbeat:
\`\`\`javascript
await redis.publish('${HEARTBEAT_CHANNEL}', JSON.stringify({
  paneId: '${paneId}',
  status: 'alive',
  timestamp: Date.now()
}));
\`\`\`
`;
  
  // Insert after any existing system context markers
  if (systemPrompt.includes('<!-- SYSTEM CONTEXT -->')) {
    return systemPrompt.replace(
      '<!-- SYSTEM CONTEXT -->',
      contextBlock
    );
  }
  
  return contextBlock + '\n\n' + systemPrompt;
}

export class RedisManager {
  private client!: RedisClientType;
  private config: RedisConfig;
  private paneId: string;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private deadLetterListener: ChildProcess | null = null;
  private subscribers: Map<string, RedisClientType> = new Map();

  constructor(config: RedisConfig = {}) {
    this.config = {
      url: config.url || process.env.REDIS_URL || 'redis://localhost:6379',
      host: config.host || 'localhost',
      port: config.port || 6379,
      password: config.password || process.env.REDIS_PASSWORD || undefined
    };
    this.paneId = getPaneId();
  }

  /**
   * Connect to Redis
   */
  async connect(): Promise<void> {
    if (this.client?.isOpen) return;

    this.client = createClient({
      url: this.config.url
    });

    this.client.on('error', (err) => {
      console.error('[Redis] Error:', err.message);
    });

    await this.client.connect();
    console.log('[Redis] Connected to', this.config.url);
  }

  /**
   * Disconnect from Redis
   */
  async disconnect(): Promise<void> {
    this.stopHeartbeat();
    // Clean up all subscribers
    for (const [channel, sub] of this.subscribers.entries()) {
      try {
        if (sub.isOpen) await sub.quit();
      } catch {}
    }
    this.subscribers.clear();
    if (this.client?.isOpen) {
      await this.client.quit();
    }
  }

  /**
   * Get the Redis client (public accessor)
   */
  getClient(): RedisClientType | undefined {
    return this.client;
  }

  /**
   * Get the pane ID for this session
   */
  getPaneId(): string {
    return this.paneId;
  }

  /**
   * Set a custom pane ID
   */
  setPaneId(id: string): void {
    this.paneId = id;
    process.env.CODERS_PANE_ID = id;
  }

  /**
   * Publish heartbeat for this pane
   */
  async publishHeartbeat(status: HeartbeatData['status'] = 'alive', lastActivity: string = 'working'): Promise<void> {
    if (!this.client?.isOpen) return;

    const data: HeartbeatData = {
      paneId: this.paneId,
      sessionId: this.getSessionId(),
      timestamp: Date.now(),
      status,
      lastActivity
    };

    await this.client.publish(HEARTBEAT_CHANNEL, JSON.stringify(data));
    
    // Also set expiration-based key for dead-letter queue
    await this.client.set(`${PANE_ID_KEY_PREFIX}${this.paneId}`, JSON.stringify(data), {
      EX: 150 // 2.5 min TTL (longer than dead-letter timeout)
    });
  }

  /**
   * Start periodic heartbeat publishing
   */
  startHeartbeat(intervalMs: number = 30000): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
    }

    this.heartbeatInterval = setInterval(() => {
      this.publishHeartbeat('alive').catch(console.error);
    }, intervalMs);
    
    console.log(`[Redis] Heartbeat started (interval: ${intervalMs}ms, pane: ${this.paneId})`);
  }

  /**
   * Stop heartbeat publishing
   */
  stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  /**
   * Get the tmux session ID
   */
  private getSessionId(): string {
    return process.env.CODERS_SESSION_ID || 'coder-unknown';
  }

  /**
   * Publish to inter-agent communication channel
   */
  async publishMessage(channel: string, message: any): Promise<void> {
    if (!this.client?.isOpen) await this.connect();
    
    const topic = `coders:msg:${channel}`;
    await this.client.publish(topic, JSON.stringify({
      fromPane: this.paneId,
      timestamp: Date.now(),
      message
    }));
  }

  /**
   * Subscribe to inter-agent communication channel
   */
  async subscribeToChannel(channel: string, callback: (message: any) => void): Promise<void> {
    if (!this.client?.isOpen) await this.connect();

    const topic = `coders:msg:${channel}`;
    const subscriber = this.client.duplicate();
    await subscriber.connect();

    await subscriber.subscribe(topic, (message) => {
      try {
        const data = JSON.parse(message);
        callback(data);
      } catch (e) {
        console.error('[Redis] Failed to parse message:', e);
      }
    });

    this.subscribers.set(channel, subscriber);
  }

  /**
   * Unsubscribe from inter-agent communication channel
   */
  async unsubscribe(channel: string): Promise<void> {
    const topic = `coders:msg:${channel}`;
    const subscriber = this.subscribers.get(channel);
    if (subscriber) {
      await subscriber.unsubscribe(topic);
      if (subscriber.isOpen) await subscriber.quit();
      this.subscribers.delete(channel);
    }
  }

  /**
   * Get all active panes from Redis
   */
  async getActivePanes(): Promise<PaneInfo[]> {
    if (!this.client?.isOpen) return [];
    
    const keys = await this.client.keys(`${PANE_ID_KEY_PREFIX}*`);
    const panes: PaneInfo[] = [];
    
    for (const key of keys) {
      const data = await this.client.get(key);
      if (data) {
        try {
          panes.push(JSON.parse(data));
        } catch (e) {}
      }
    }
    
    return panes;
  }

  /**
   * Mark pane as dead (called by dead-letter listener)
   */
  async markPaneDead(paneId: string): Promise<void> {
    if (!this.client?.isOpen) return;
    
    await this.client.rPush(DEAD_LETTER_KEY, paneId);
    console.log(`[Redis] Marked pane ${paneId} as dead`);
  }
}

/**
 * Dead-letter listener - watches for dead panes and respawns them
 */
export class DeadLetterListener {
  private redisManager: RedisManager;
  private listenerProcess: ChildProcess | null = null;
  private isRunning: boolean = false;

  constructor(redisConfig?: RedisConfig) {
    this.redisManager = new RedisManager(redisConfig);
  }

  /**
   * Start listening for dead letters
   */
  async start(timeoutMs: number = 120000): Promise<void> {
    if (this.isRunning) return;
    
    await this.redisManager.connect();
    this.isRunning = true;

    const checkInterval = setInterval(async () => {
      try {
        const client = this.redisManager.getClient();
        const deadPane = await client?.blPop(DEAD_LETTER_KEY, 0);
        
        if (deadPane?.element) {
          await this.handleDeadPane(deadPane.element);
        }
      } catch (e) {
        console.error('[DeadLetter] Error:', e);
      }
    }, 1000);

    // Store interval handle for cleanup
    (this as any)._interval = checkInterval;
    
    console.log('[DeadLetter] Listener started (timeout:', timeoutMs / 1000, 's)');
  }

  /**
   * Handle a dead pane - respawn it
   */
  private async handleDeadPane(paneId: string): Promise<void> {
    console.log(`[DeadLetter] Respawning dead pane: ${paneId}`);
    
    try {
      // Extract session info from pane ID
      const sessionId = this.extractSessionId(paneId);
      
      if (sessionId) {
        // Kill and respawn the tmux pane
        this.respawnPane(paneId, sessionId);
      }
    } catch (e) {
      console.error('[DeadLetter] Failed to respawn pane:', e);
    }
  }

  /**
   * Extract session ID from pane ID
   */
  private extractSessionId(paneId: string): string | null {
    // pane IDs are typically in format: pane-{timestamp}-{random}
    // Session info is stored in Redis
    return null; // Override to extract from your specific naming convention
  }

  /**
   * Respawn a tmux pane
   */
  private respawnPane(paneId: string, sessionId: string): void {
    try {
      // Kill the dead pane
      execSync(`respawn-pane -k -t ${paneId} 2>/dev/null || tmux kill-pane -t ${paneId} 2>/dev/null`, {
        encoding: 'utf8'
      });
      
      console.log(`[DeadLetter] Respawned pane: ${paneId}`);
    } catch (e) {
      console.error('[DeadLetter] Respawn failed:', e);
    }
  }

  /**
   * Stop the listener
   */
  stop(): void {
    this.isRunning = false;
    if ((this as any)._interval) {
      clearInterval((this as any)._interval);
    }
    this.redisManager.disconnect();
  }
}

/**
 * Factory function for creating a Redis manager with heartbeat
 */
export async function createHeartbeatSession(
  config?: RedisConfig
): Promise<RedisManager> {
  const manager = new RedisManager(config);
  await manager.connect();
  manager.startHeartbeat();
  return manager;
}

export default {
  RedisManager,
  DeadLetterListener,
  createHeartbeatSession,
  getPaneId,
  injectPaneIdContext,
  SNAPSHOT_DIR,
  HEARTBEAT_CHANNEL,
  DEAD_LETTER_KEY
};
