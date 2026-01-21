import { Box, Text } from 'ink';

interface Props {
  sessionCount: number;
}

export function StatusBar({ sessionCount }: Props) {
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
            {sessionCount} session{sessionCount !== 1 ? 's' : ''}
          </Text>
        </Box>

        <Box>
          <Text dimColor>
            <Text color="cyan">↑↓/jk</Text> nav
            <Text color="cyan"> a/↵</Text> attach
            <Text color="cyan"> K</Text> kill
            <Text color="cyan"> r</Text> refresh
            <Text color="cyan"> q</Text> quit
          </Text>
        </Box>
      </Box>

      <Box marginTop={0}>
        <Text dimColor>
          Return to TUI: <Text color="yellow">Ctrl-b L</Text> (last session)
        </Text>
      </Box>
    </Box>
  );
}
