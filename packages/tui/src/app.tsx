import { useState, useEffect, useRef, useCallback } from 'react';
import { Box, Text, useApp, useInput } from 'ink';
import { SessionList } from './components/SessionList.js';
import { SessionDetail } from './components/SessionDetail.js';
import { StatusBar } from './components/StatusBar.js';
import { Header } from './components/Header.js';
import type { Session } from './types.js';
import { getTmuxSessions, attachSession, killSession } from './tmux.js';

export function App() {
  const { exit } = useApp();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [initialLoading, setInitialLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const sessionsRef = useRef<Session[]>([]);

  const refreshSessions = useCallback(async (showLoading = false) => {
    try {
      if (showLoading) setInitialLoading(true);
      const tmuxSessions = await getTmuxSessions();
      sessionsRef.current = tmuxSessions;
      setSessions(tmuxSessions);
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

  useInput(useCallback((input: string, key: { upArrow?: boolean; downArrow?: boolean; return?: boolean }) => {
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
  }, [exit, refreshSessions, selectedIndex]));

  const selectedSession = sessions[selectedIndex] || null;

  return (
    <Box flexDirection="column" padding={1}>
      <Header />

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

      <StatusBar sessionCount={sessions.length} />
    </Box>
  );
}
