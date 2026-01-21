import { Box, Text } from 'ink';
import type { Session } from '../types.js';

interface Props {
  session: Session | null;
}

function formatAge(date: Date | undefined): string {
  if (!date) return 'unknown';
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function SessionDetail({ session }: Props) {
  if (!session) {
    return null;
  }

  const taskDisplay = session.task
    ? session.task.replace(/-/g, ' ')
    : 'No task specified';

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="gray"
      paddingX={2}
      paddingY={1}
      marginTop={1}
    >
      <Box marginBottom={1}>
        <Text bold color="cyan">
          {session.isOrchestrator ? '🎯 ' : ''}{session.name}
        </Text>
      </Box>

      <Box flexDirection="column" gap={0}>
        <Box>
          <Box width={12}>
            <Text dimColor>Task:</Text>
          </Box>
          <Text wrap="wrap">{taskDisplay}</Text>
        </Box>

        <Box>
          <Box width={12}>
            <Text dimColor>Tool:</Text>
          </Box>
          <Text>{session.tool}</Text>
        </Box>

        <Box>
          <Box width={12}>
            <Text dimColor>Directory:</Text>
          </Box>
          <Text wrap="truncate-end">{session.cwd}</Text>
        </Box>

        <Box>
          <Box width={12}>
            <Text dimColor>Created:</Text>
          </Box>
          <Text>{formatAge(session.createdAt)}</Text>
        </Box>

        {session.parentSessionId && (
          <Box>
            <Box width={12}>
              <Text dimColor>Parent:</Text>
            </Box>
            <Text>{session.parentSessionId}</Text>
          </Box>
        )}
      </Box>
    </Box>
  );
}
