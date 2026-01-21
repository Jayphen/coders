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
  // Promise (completion) data
  promise?: CoderPromise;
  hasPromise?: boolean;
}

export interface HeartbeatData {
  paneId: string;
  sessionId: string;
  timestamp: number;
  status: string;
  lastActivity?: string;
  parentSessionId?: string;
}

export interface CoderPromise {
  sessionId: string;
  timestamp: number;
  summary: string;
  status: 'completed' | 'blocked' | 'needs-review';
  filesChanged?: string[];
  blockers?: string[];
}
