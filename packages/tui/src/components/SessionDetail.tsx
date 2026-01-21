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

function formatTimestamp(timestamp: number | undefined): string {
  if (!timestamp) return 'unknown';
  return formatAge(new Date(timestamp));
}

const PROMISE_STATUS_LABELS: Record<string, { label: string; color: string }> = {
  completed: { label: 'Completed', color: 'green' },
  blocked: { label: 'Blocked', color: 'red' },
  'needs-review': { label: 'Needs Review', color: 'yellow' },
};

function renderProgressBar(percent: number, width: number = 20): string {
  const filled = Math.round((percent / 100) * width);
  const empty = width - filled;
  const bar = '█'.repeat(Math.max(0, filled)) + '░'.repeat(Math.max(0, empty));
  return bar;
}

export function SessionDetail({ session }: Props) {
  if (!session) {
    return null;
  }

  const taskDisplay = session.task
    ? session.task.replace(/-/g, ' ')
    : 'No task specified';

  const promiseStatusInfo = session.promise?.status
    ? PROMISE_STATUS_LABELS[session.promise.status]
    : null;

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor={session.hasPromise ? 'gray' : 'gray'}
      paddingX={2}
      paddingY={1}
      marginTop={1}
    >
      <Box marginBottom={1}>
        <Text bold color={session.hasPromise ? 'gray' : 'cyan'}>
          {session.isOrchestrator ? '🎯 ' : ''}{session.name}
        </Text>
        {session.hasPromise && promiseStatusInfo && (
          <Text color={promiseStatusInfo.color}> [{promiseStatusInfo.label}]</Text>
        )}
      </Box>

      <Box flexDirection="column" gap={0}>
        {/* Show promise summary for completed sessions */}
        {session.promise && (
          <>
            <Box>
              <Box width={12}>
                <Text dimColor>Summary:</Text>
              </Box>
              <Text wrap="wrap" color="green">{session.promise.summary}</Text>
            </Box>

            <Box>
              <Box width={12}>
                <Text dimColor>Finished:</Text>
              </Box>
              <Text>{formatTimestamp(session.promise.timestamp)}</Text>
            </Box>

            {session.promise.blockers && session.promise.blockers.length > 0 && (
              <Box>
                <Box width={12}>
                  <Text dimColor>Blockers:</Text>
                </Box>
                <Text color="red">{session.promise.blockers.join(', ')}</Text>
              </Box>
            )}
          </>
        )}

        <Box>
          <Box width={12}>
            <Text dimColor>Task:</Text>
          </Box>
          <Text wrap="wrap" dimColor={session.hasPromise}>{taskDisplay}</Text>
        </Box>

        <Box>
          <Box width={12}>
            <Text dimColor>Tool:</Text>
          </Box>
          <Text dimColor={session.hasPromise}>{session.tool}</Text>
        </Box>

        <Box>
          <Box width={12}>
            <Text dimColor>Directory:</Text>
          </Box>
          <Text wrap="truncate-end" dimColor={session.hasPromise}>{session.cwd}</Text>
        </Box>

        <Box>
          <Box width={12}>
            <Text dimColor>Created:</Text>
          </Box>
          <Text dimColor={session.hasPromise}>{formatAge(session.createdAt)}</Text>
        </Box>

        {session.usage && (
          <Box flexDirection="column" marginTop={0}>
            <Box>
              <Box width={12}>
                <Text dimColor>Usage:</Text>
              </Box>
            </Box>
            {session.usage.cost && (
              <Box marginLeft={2}>
                <Text color="yellow">Cost: {session.usage.cost}</Text>
              </Box>
            )}
            {session.usage.tokens && (
              <Box marginLeft={2}>
                <Text>Tokens: {session.usage.tokens.toLocaleString()}</Text>
              </Box>
            )}
            {session.usage.apiCalls && (
              <Box marginLeft={2}>
                <Text>API Calls: {session.usage.apiCalls}</Text>
              </Box>
            )}
            {session.usage.sessionLimitPercent !== undefined && (
              <Box marginLeft={2} flexDirection="column">
                <Box>
                  <Text dimColor>Session Limit: </Text>
                  <Text color={session.usage.sessionLimitPercent > 90 ? 'red' : 'green'}>
                    {session.usage.sessionLimitPercent}%
                  </Text>
                </Box>
                <Text color={session.usage.sessionLimitPercent > 90 ? 'red' : 'green'}>
                  {renderProgressBar(session.usage.sessionLimitPercent)}
                </Text>
              </Box>
            )}
            {session.usage.weeklyLimitPercent !== undefined && (
              <Box marginLeft={2} flexDirection="column" marginTop={1}>
                <Box>
                  <Text dimColor>Weekly Limit: </Text>
                  <Text color={session.usage.weeklyLimitPercent > 90 ? 'red' : 'green'}>
                    {session.usage.weeklyLimitPercent}%
                  </Text>
                </Box>
                <Text color={session.usage.weeklyLimitPercent > 90 ? 'red' : 'green'}>
                  {renderProgressBar(session.usage.weeklyLimitPercent)}
                </Text>
              </Box>
            )}
          </Box>
        )}

        {session.parentSessionId && (
          <Box>
            <Box width={12}>
              <Text dimColor>Parent:</Text>
            </Box>
            <Text dimColor={session.hasPromise}>{session.parentSessionId}</Text>
          </Box>
        )}
      </Box>
    </Box>
  );
}
