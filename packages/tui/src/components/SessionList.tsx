import { Box, Text } from 'ink';
import Spinner from 'ink-spinner';
import type { Session } from '../types.js';
import { SessionRow } from './SessionRow.js';

interface Props {
  sessions: Session[];
  selectedIndex: number;
  loading: boolean;
}

export function SessionList({ sessions, selectedIndex, loading }: Props) {
  if (loading && sessions.length === 0) {
    return (
      <Box marginY={1}>
        <Text>
          <Spinner type="dots" /> Loading sessions...
        </Text>
      </Box>
    );
  }

  if (sessions.length === 0) {
    return (
      <Box
        marginY={1}
        paddingX={2}
        paddingY={1}
        borderStyle="round"
        borderColor="gray"
      >
        <Text dimColor>No active coder sessions</Text>
      </Box>
    );
  }

  // Split sessions into active and completed
  const activeSessions = sessions.filter(s => !s.hasPromise);
  const completedSessions = sessions.filter(s => s.hasPromise);

  // Track cumulative index for selection highlighting
  let cumulativeIndex = 0;

  return (
    <Box flexDirection="column" marginY={1}>
      <Box marginBottom={1} paddingX={1}>
        <Box width={3}><Text dimColor> </Text></Box>
        <Box width={28}><Text dimColor bold>SESSION</Text></Box>
        <Box width={10}><Text dimColor bold>TOOL</Text></Box>
        <Box width={20}><Text dimColor bold>TASK/SUMMARY</Text></Box>
        <Box width={8}><Text dimColor bold>STATUS</Text></Box>
      </Box>

      {/* Active Sessions */}
      {activeSessions.length > 0 && (
        <>
          <Box paddingX={1} marginBottom={0}>
            <Text color="green" bold>Active ({activeSessions.length})</Text>
          </Box>
          {activeSessions.map((session) => {
            const index = cumulativeIndex++;
            return (
              <SessionRow
                key={session.name}
                session={session}
                isSelected={index === selectedIndex}
              />
            );
          })}
        </>
      )}

      {/* Completed Sessions */}
      {completedSessions.length > 0 && (
        <>
          <Box paddingX={1} marginTop={activeSessions.length > 0 ? 1 : 0} marginBottom={0}>
            <Text color="gray" bold>Completed ({completedSessions.length})</Text>
          </Box>
          {completedSessions.map((session) => {
            const index = cumulativeIndex++;
            return (
              <SessionRow
                key={session.name}
                session={session}
                isSelected={index === selectedIndex}
              />
            );
          })}
        </>
      )}
    </Box>
  );
}
