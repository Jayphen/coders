import { execSync, spawn, spawnSync } from 'child_process';
import type { Session } from './types.js';

export function getCurrentSession(): string | null {
  try {
    return execSync('tmux display-message -p "#{session_name}"', { encoding: 'utf-8' }).trim();
  } catch {
    return null;
  }
}

export async function getTmuxSessions(): Promise<Session[]> {
  try {
    // Get session info including pane title (which may contain task description)
    const output = execSync(
      'tmux list-sessions -F "#{session_name}|#{session_created}|#{pane_current_path}|#{pane_title}" 2>/dev/null',
      { encoding: 'utf-8' }
    );

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

        return {
          name,
          tool: ['claude', 'gemini', 'codex', 'opencode'].includes(toolName)
            ? toolName
            : 'unknown',
          task,
          cwd: cwd || process.cwd(),
          createdAt: created ? new Date(parseInt(created) * 1000) : undefined,
          isOrchestrator,
          heartbeatStatus: 'healthy' as const, // TODO: integrate with Redis
        };
      })
      // Sort: orchestrator first, then by creation time (newest first)
      .sort((a, b) => {
        if (a.isOrchestrator) return -1;
        if (b.isOrchestrator) return 1;
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
  } catch {
    // Session may already be dead
  }
}
