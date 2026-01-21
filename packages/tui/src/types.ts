export interface Session {
  name: string;
  tool: 'claude' | 'gemini' | 'codex' | 'opencode' | 'unknown';
  task?: string;
  cwd: string;
  createdAt?: Date;
  parentSessionId?: string;
  isOrchestrator: boolean;
  heartbeatStatus?: 'healthy' | 'stale' | 'dead';
  lastActivity?: Date;
}

export interface HeartbeatData {
  paneId: string;
  sessionId: string;
  timestamp: number;
  status: string;
  lastActivity?: string;
  parentSessionId?: string;
}
