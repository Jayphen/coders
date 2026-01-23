import { Box, Text } from 'ink';

interface Props {
  sessionCount: number;
  completedCount?: number;
}

export function StatusBar({ sessionCount, completedCount = 0 }: Props) {
  const activeCount = sessionCount - completedCount;

  return (
    <Box
      marginTop={1}
      paddingTop={1}
      borderStyle="single"
      borderTop
      borderBottom={false}
      borderLeft={false}
      borderRight={false}
      borderColor="gray"
      flexDirection="column"
    >
      <Box>
        <Box flexGrow={1}>
          <Text dimColor>
            {activeCount} active
            {completedCount > 0 && (
              <Text color="gray">, {completedCount} completed</Text>
            )}
          </Text>
        </Box>

        <Box>
          <Text dimColor>
            <Text color="cyan">↑↓/jk</Text> nav
            <Text color="cyan"> a/↵</Text> attach
            <Text color="cyan"> s</Text> spawn
            <Text color="cyan"> K</Text> kill
            <Text color="cyan"> r</Text> refresh
            <Text color="cyan"> q</Text> quit
          </Text>
        </Box>
      </Box>

      <Box marginTop={0}>
        <Box flexGrow={1}>
          <Text dimColor>
            Return to TUI: <Text color="yellow">Ctrl-b L</Text> (last session)
          </Text>
        </Box>
        {completedCount > 0 && (
          <Box>
            <Text dimColor>
              <Text color="cyan">R</Text> resume
              <Text color="cyan"> C</Text> kill all completed
            </Text>
          </Box>
        )}
      </Box>
    </Box>
  );
}
