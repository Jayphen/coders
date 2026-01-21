import { Box, Text } from 'ink';

export function Header() {
  return (
    <Box flexDirection="column" marginBottom={1}>
      <Text bold color="cyan">
        Coders Session Manager
      </Text>
      <Text dimColor>
        Manage your AI coding sessions
      </Text>
    </Box>
  );
}
