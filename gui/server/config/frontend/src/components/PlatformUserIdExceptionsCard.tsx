import React from 'react';
import { ActionIcon, Button, Group, Input, Stack } from '@mantine/core';
import { IconId, IconPlus, IconTrash } from '@tabler/icons-react';
import { SettingCard } from './SettingCard';

interface PlatformUserIdExceptionsCardProps {
    ids: string[];
    description: string;
    error?: string;
    expanded: boolean;
    onToggle: () => void;
    onChange: (ids: string[]) => void;
    onCommit: () => void;
}

/**
 * The list of platform user IDs that invert a boolean setting, shared by Sign
 * and Safe. Both sections document the same XOR rule, so the control and its
 * count live here instead of being copied into each page.
 */
export const PlatformUserIdExceptionsCard: React.FC<PlatformUserIdExceptionsCardProps> = ({
    ids,
    description,
    error,
    expanded,
    onToggle,
    onChange,
    onCommit,
}) => {
    const count = ids.length;
    const value = count === 0 ? 'None' : `${count} ${count === 1 ? 'exception' : 'exceptions'}`;
    return (
        <SettingCard
            icon={<IconId size={20} stroke={1.6} />}
            title="Platform user ID exceptions"
            description={description}
            value={value}
            error={error}
            expanded={expanded}
            onToggle={onToggle}
            details={
                <div style={{ width: '100%', maxWidth: 560 }}>
                    <Stack gap="xs" style={{ width: '100%' }}>
                        {ids.map((id, index) => (
                            <Stack key={index} gap={4}>
                                <Group gap="xs" wrap="nowrap" justify="space-between">
                                    <Input
                                        value={id}
                                        onChange={(e) => {
                                            const next = [...ids];
                                            next[index] = e.target.value;
                                            onChange(next);
                                        }}
                                        onBlur={onCommit}
                                        placeholder="Platform user ID"
                                        style={{ flex: 1 }}
                                    />
                                    <ActionIcon
                                        variant="subtle"
                                        color="gray"
                                        size="sm"
                                        aria-label={`Remove exception ${index + 1}`}
                                        disabled={ids.length <= 1}
                                        onClick={() => {
                                            const next = [...ids];
                                            next.splice(index, 1);
                                            onChange(next);
                                            onCommit();
                                        }}
                                    >
                                        <IconTrash size={16} stroke={1.6} />
                                    </ActionIcon>
                                </Group>
                            </Stack>
                        ))}
                        <Button
                            variant="subtle"
                            size="compact-sm"
                            w="fit-content"
                            leftSection={<IconPlus size={15} stroke={1.8} />}
                            disabled={ids.some((id) => id === '')}
                            onClick={() => onChange([...ids, ''])}
                        >
                            Add exception
                        </Button>
                    </Stack>
                </div>
            }
        />
    );
};
