import { Box, Text } from 'ink';

interface Props {
  version?: string;
}

export function Header({ version }: Props) {
  return (
    <Box flexDirection="column" marginBottom={1}>
      <Box gap={1}>
        <Text bold color="cyan">
          Coders Session Manager
        </Text>
        {version && (
          <Text dimColor>
            v{version}
          </Text>
        )}
      </Box>
      <Text dimColor>
        Manage your AI coding sessions
      </Text>
    </Box>
  );
}
