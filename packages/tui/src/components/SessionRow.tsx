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

const PROMISE_STATUS_INDICATORS: Record<string, { symbol: string; color: string }> = {
  completed: { symbol: '✓', color: 'green' },
  blocked: { symbol: '!', color: 'red' },
  'needs-review': { symbol: '?', color: 'yellow' },
};

export function SessionRow({ session, isSelected }: Props) {
  const toolColor = TOOL_COLORS[session.tool];
  const heartbeatStatus = STATUS_INDICATORS[session.heartbeatStatus || 'healthy'];

  // Use promise status if available, otherwise use heartbeat status
  const promiseStatus = session.promise?.status
    ? PROMISE_STATUS_INDICATORS[session.promise.status]
    : null;

  const displayName = session.isOrchestrator
    ? 'orchestrator'
    : session.name.replace('coder-', '');

  // For completed sessions, show promise summary; otherwise show task
  const displayText = session.promise
    ? session.promise.summary
    : session.task;

  const truncatedText = displayText
    ? (displayText.length > 18 ? displayText.slice(0, 15) + '...' : displayText)
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
          color={session.isOrchestrator ? 'cyan' : session.hasPromise ? 'gray' : undefined}
          bold={session.isOrchestrator || isSelected}
          dimColor={session.hasPromise && !isSelected}
        >
          {session.isOrchestrator ? '🎯 ' : session.parentSessionId ? '├─ ' : ''}
          {displayName.slice(0, session.isOrchestrator ? 24 : 22)}
        </Text>
      </Box>

      <Box width={10}>
        <Text color={toolColor} dimColor={session.hasPromise}>
          {session.tool}
        </Text>
      </Box>

      <Box width={20}>
        <Text dimColor={!displayText || session.hasPromise}>
          {truncatedText}
        </Text>
      </Box>

      <Box width={8}>
        {promiseStatus ? (
          <Text color={promiseStatus.color}>
            {promiseStatus.symbol}
          </Text>
        ) : (
          <Text color={heartbeatStatus.color}>
            {heartbeatStatus.symbol}
          </Text>
        )}
      </Box>
    </Box>
  );
}
