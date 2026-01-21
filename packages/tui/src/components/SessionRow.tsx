import { Box, Text } from 'ink';
import type { Session } from '../types.js';

interface Props {
  session: Session;
  isSelected: boolean;
}

const TOOL_COLORS: Record<Session['tool'], string> = {
  claude: 'magenta',
  gemini: 'blue',
  codex: 'green',
  opencode: 'yellow',
  unknown: 'gray',
};

const STATUS_INDICATORS: Record<NonNullable<Session['heartbeatStatus']>, { symbol: string; color: string }> = {
  healthy: { symbol: '●', color: 'green' },
  stale: { symbol: '◐', color: 'yellow' },
  dead: { symbol: '○', color: 'red' },
};

export function SessionRow({ session, isSelected }: Props) {
  const toolColor = TOOL_COLORS[session.tool];
  const status = STATUS_INDICATORS[session.heartbeatStatus || 'healthy'];

  const displayName = session.isOrchestrator
    ? 'orchestrator'
    : session.name.replace('coder-', '');

  const truncatedTask = session.task
    ? (session.task.length > 18 ? session.task.slice(0, 15) + '...' : session.task)
    : '-';

  return (
    <Box paddingX={1}>
      <Box width={3}>
        <Text color={isSelected ? 'cyan' : undefined}>
          {isSelected ? '❯' : ' '}
        </Text>
      </Box>

      <Box width={28}>
        <Text
          color={session.isOrchestrator ? 'cyan' : undefined}
          bold={session.isOrchestrator || isSelected}
        >
          {session.isOrchestrator ? '🎯 ' : session.parentSessionId ? '├─ ' : ''}
          {displayName.slice(0, session.isOrchestrator ? 24 : 22)}
        </Text>
      </Box>

      <Box width={10}>
        <Text color={toolColor}>
          {session.tool}
        </Text>
      </Box>

      <Box width={20}>
        <Text dimColor={!session.task}>
          {truncatedTask}
        </Text>
      </Box>

      <Box width={8}>
        <Text color={status.color}>
          {status.symbol}
        </Text>
      </Box>
    </Box>
  );
}
