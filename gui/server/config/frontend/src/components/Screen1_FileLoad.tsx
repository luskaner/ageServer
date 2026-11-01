import React from 'react';
import { Alert, Button, Center, Code, List, Paper, Stack, Text, Title } from '@mantine/core';
import {
    IconAlertCircle,
    IconFilePlus,
    IconFileText,
    IconFileUpload,
    IconFolderOpen,
} from '@tabler/icons-react';
import { ValidationError } from '../../bindings/changeme';
import { fieldLabel, humanMessage } from '../types';

interface Screen1Props {
    onSelectConfig?: () => void;
    onCreateConfig?: () => void;
    errorMsg?: string;
    validationErrors?: ValidationError[];
}

export const Screen1_FileLoad: React.FC<Screen1Props> = ({ onSelectConfig, onCreateConfig, errorMsg, validationErrors = [] }) => {
    // Warnings do not block anything, so they are not listed as errors here.
    const blockingErrors = validationErrors.filter((issue) => issue.severity !== 'warning');
    return (
        <Center mih="100vh" p="xl">
            <Stack w="100%" maw={520} gap="lg">
                <Stack gap={6} align="center">
                    <IconFileText size={48} stroke={1.4} className="hero-icon" />
                    <Title order={2} ta="center">
                        Load configuration file
                    </Title>
                    <Text size="sm" c="dimmed" ta="center">
                        Select the server's <Code>config.toml</Code> file to see and edit
                    </Text>
                </Stack>

                <Paper
                    id="config-dropzone"
                    data-file-drop-target
                    withBorder
                    radius="md"
                    p="xl"
                    className="dropzone"
                    onClick={() => onSelectConfig?.()}
                >
                    <Stack align="center" gap="sm">
                        <IconFileUpload size={34} stroke={1.4} className="dropzone-icon" />
                        <Text fw={600}>Drag your config.toml file here</Text>
                        <Text size="sm" c="dimmed">
                            or browse your files to select it
                        </Text>
                        <Button
                            variant="default"
                            leftSection={<IconFolderOpen size={16} />}
                            mt="xs"
                            onClick={(event) => {
                                event.stopPropagation();
                                onSelectConfig?.();
                            }}
                        >
                            Browse file
                        </Button>
                    </Stack>
                </Paper>

                <Button size="md" leftSection={<IconFilePlus size={16} />} onClick={() => onCreateConfig?.()}>
                    Create new configuration
                </Button>

                {errorMsg && (
                    <Alert color="red" variant="light" icon={<IconAlertCircle size={20} />} title="Error loading TOML">
                        {errorMsg}
                    </Alert>
                )}

                {blockingErrors.length > 0 && (
                    <Alert
                        color="red"
                        variant="light"
                        icon={<IconAlertCircle size={20} />}
                        title="Configuration validation errors"
                    >
                        <List size="sm" spacing={2}>
                            {blockingErrors.map((error) => (
                                <List.Item key={`${error.field}-${error.message}`}>
                                    <Text span size="sm" fw={600}>
                                        {fieldLabel(error.field)}
                                    </Text>
                                    : {humanMessage(error.message)}
                                </List.Item>
                            ))}
                        </List>
                    </Alert>
                )}
            </Stack>
        </Center>
    );
};
