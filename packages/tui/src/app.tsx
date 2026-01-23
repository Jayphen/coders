import { useState, useEffect, useRef, useCallback } from 'react';
import { Box, Text, useApp, useInput } from 'ink';
import TextInput from 'ink-text-input';
import { SessionList } from './components/SessionList.js';
import { SessionDetail } from './components/SessionDetail.js';
import { StatusBar } from './components/StatusBar.js';
import { Header } from './components/Header.js';
import type { Session } from './types.js';
import { getTmuxSessions, attachSession, killSession, killCompletedSessions, resumeSession, spawnSession } from './tmux.js';

interface Props {
  version?: string;
}

function areSessionsEqual(prev: Session[], next: Session[]): boolean {
  if (prev === next) return true;
  if (prev.length !== next.length) return false;
  for (let i = 0; i < prev.length; i += 1) {
    const a = prev[i];
    const b = next[i];
    if (
      a.name !== b.name ||
      a.tool !== b.tool ||
      a.task !== b.task ||
      a.cwd !== b.cwd ||
      a.isOrchestrator !== b.isOrchestrator ||
      a.heartbeatStatus !== b.heartbeatStatus ||
      a.hasPromise !== b.hasPromise ||
      a.parentSessionId !== b.parentSessionId
    ) {
      return false;
    }

    if ((a.createdAt?.getTime() ?? 0) !== (b.createdAt?.getTime() ?? 0)) {
      return false;
    }

    const aPromise = a.promise;
    const bPromise = b.promise;
    if (
      aPromise?.status !== bPromise?.status ||
      aPromise?.summary !== bPromise?.summary ||
      aPromise?.timestamp !== bPromise?.timestamp ||
      (aPromise?.blockers?.join('|') ?? '') !== (bPromise?.blockers?.join('|') ?? '') ||
      (aPromise?.filesChanged?.join('|') ?? '') !== (bPromise?.filesChanged?.join('|') ?? '')
    ) {
      return false;
    }

    const aUsage = a.usage;
    const bUsage = b.usage;
    if (
      aUsage?.cost !== bUsage?.cost ||
      aUsage?.tokens !== bUsage?.tokens ||
      aUsage?.apiCalls !== bUsage?.apiCalls ||
      aUsage?.sessionLimitPercent !== bUsage?.sessionLimitPercent ||
      aUsage?.weeklyLimitPercent !== bUsage?.weeklyLimitPercent
    ) {
      return false;
    }
  }
  return true;
}

export function App({ version }: Props) {
  const { exit } = useApp();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [initialLoading, setInitialLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [confirmKillCompleted, setConfirmKillCompleted] = useState(false);
  const [spawnMode, setSpawnMode] = useState(false);
  const [spawnArgs, setSpawnArgs] = useState('');
  const [spawning, setSpawning] = useState(false);
  const sessionsRef = useRef<Session[]>([]);

  const refreshSessions = useCallback(async (showLoading = false) => {
    try {
      if (showLoading) setInitialLoading(true);
      const tmuxSessions = await getTmuxSessions();
      const hasChanges = !areSessionsEqual(sessionsRef.current, tmuxSessions);
      sessionsRef.current = tmuxSessions;
      if (hasChanges) {
        setSessions(tmuxSessions);
        setSelectedIndex(current =>
          tmuxSessions.length > 0 && current >= tmuxSessions.length
            ? tmuxSessions.length - 1
            : current
        );
      }
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get sessions');
    } finally {
      if (showLoading) setInitialLoading(false);
    }
  }, []);

  useEffect(() => {
    refreshSessions(true);
    const interval = setInterval(() => refreshSessions(false), 5000);
    return () => clearInterval(interval);
  }, [refreshSessions]);

  // Clear status message after 3 seconds
  useEffect(() => {
    if (statusMessage) {
      const timeout = setTimeout(() => setStatusMessage(null), 3000);
      return () => clearTimeout(timeout);
    }
  }, [statusMessage]);

  const handleSpawnSubmit = useCallback((value: string) => {
    const trimmed = value.trim();
    if (!trimmed) {
      setSpawnMode(false);
      setSpawnArgs('');
      setStatusMessage('Spawn cancelled');
      return;
    }

    setSpawnMode(false);
    setSpawnArgs('');
    setSpawning(true);
    setStatusMessage('Spawning session...');

    spawnSession(trimmed)
      .then((result) => {
        if (result.ok) {
          setStatusMessage('Spawn command sent');
        } else {
          setStatusMessage(`Spawn failed: ${result.message ?? 'unknown error'}`);
        }
        refreshSessions(false);
      })
      .finally(() => {
        setSpawning(false);
      });
  }, [refreshSessions]);

  useInput(useCallback((input: string, key: { upArrow?: boolean; downArrow?: boolean; return?: boolean; escape?: boolean }) => {
    if (spawnMode) {
      if (key.escape) {
        setSpawnMode(false);
        setSpawnArgs('');
        setStatusMessage('Spawn cancelled');
      }
      return;
    }

    // Handle confirmation dialog
    if (confirmKillCompleted) {
      if (input === 'y' || input === 'Y') {
        setConfirmKillCompleted(false);
        killCompletedSessions().then(({ killed, failed }) => {
          if (killed.length > 0) {
            setStatusMessage(`Killed ${killed.length} completed session(s)`);
          } else {
            setStatusMessage('No completed sessions to kill');
          }
          refreshSessions(false);
        });
      } else if (input === 'n' || input === 'N' || key.return) {
        setConfirmKillCompleted(false);
        setStatusMessage('Cancelled');
      }
      return;
    }

    if (input === 's') {
      if (spawning) {
        setStatusMessage('Spawn already in progress');
      } else {
        setSpawnMode(true);
      }
      return;
    }

    if (input === 'q') {
      exit();
      return;
    }

    if (input === 'r') {
      refreshSessions(false);
      return;
    }

    if (key.upArrow || input === 'k') {
      setSelectedIndex(i => Math.max(0, i - 1));
      return;
    }

    if (key.downArrow || input === 'j') {
      setSelectedIndex(i => Math.min(sessionsRef.current.length - 1, i + 1));
      return;
    }

    if (key.return || input === 'a') {
      const session = sessionsRef.current[selectedIndex];
      if (session) {
        // Switch to the session - TUI stays alive in its session
        // User can return with Ctrl-b L (last session)
        attachSession(session.name);
      }
      return;
    }

    if (input === 'K') {
      const session = sessionsRef.current[selectedIndex];
      if (session) {
        killSession(session.name);
        refreshSessions(false);
      }
      return;
    }

    // Bulk kill completed sessions (Shift+C)
    if (input === 'C') {
      const completedCount = sessionsRef.current.filter(s => s.hasPromise && !s.isOrchestrator).length;
      if (completedCount > 0) {
        setConfirmKillCompleted(true);
      } else {
        setStatusMessage('No completed sessions to kill');
      }
      return;
    }

    // Resume selected session (clear its promise)
    if (input === 'R') {
      const session = sessionsRef.current[selectedIndex];
      if (session?.hasPromise) {
        const success = resumeSession(session.name);
        if (success) {
          setStatusMessage(`Resumed: ${session.name.replace('coder-', '')}`);
        } else {
          setStatusMessage('Failed to resume session');
        }
        refreshSessions(false);
      } else {
        setStatusMessage('Selected session is not completed');
      }
      return;
    }
  }, [exit, refreshSessions, selectedIndex, confirmKillCompleted, spawnMode, spawning]));

  const selectedSession = sessions[selectedIndex] || null;
  const completedCount = sessions.filter(s => s.hasPromise && !s.isOrchestrator).length;

  return (
    <Box flexDirection="column" padding={1}>
      <Header version={version} />

      {/* Confirmation dialog */}
      {confirmKillCompleted && (
        <Box marginY={1} paddingX={2} paddingY={1} borderStyle="round" borderColor="yellow">
          <Text color="yellow">
            Kill all {completedCount} completed session(s)? (y/n)
          </Text>
        </Box>
      )}

      {/* Spawn prompt */}
      {spawnMode && (
        <Box marginY={1} paddingX={2} paddingY={1} borderStyle="round" borderColor="cyan" flexDirection="column">
          <Text color="cyan">Spawn a new session</Text>
          <Box marginTop={1}>
            <Text dimColor>Args: </Text>
            <TextInput
              value={spawnArgs}
              onChange={setSpawnArgs}
              onSubmit={handleSpawnSubmit}
              placeholder='claude --task "Fix the bug"'
            />
          </Box>
          <Text dimColor>Enter to spawn, Esc to cancel</Text>
        </Box>
      )}

      {/* Status message */}
      {statusMessage && !confirmKillCompleted && (
        <Box marginY={1} paddingX={2}>
          <Text color="cyan">{statusMessage}</Text>
        </Box>
      )}

      {error ? (
        <Box marginY={1}>
          <Text color="red">Error: {error}</Text>
        </Box>
      ) : (
        <>
          <SessionList
            sessions={sessions}
            selectedIndex={selectedIndex}
            loading={initialLoading}
          />
          <SessionDetail session={selectedSession} />
        </>
      )}

      <StatusBar sessionCount={sessions.length} completedCount={completedCount} />
    </Box>
  );
}
