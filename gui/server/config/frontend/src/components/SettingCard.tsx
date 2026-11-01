import React from 'react';
import { Box, Card, Collapse, Group, Input, Text, UnstyledButton } from '@mantine/core';
import { IconAlertTriangle, IconChevronDown } from '@tabler/icons-react';

export interface SettingCardProps {
    /** Icon shown at the start of the row, in the same weight as any other. */
    icon?: React.ReactNode;
    title: string;
    description?: string;
    /** Control rendered at the end of the row (a Switch, an input, ...). */
    control?: React.ReactNode;
    /** Current value shown just before the control (used by expandable rows). */
    value?: string;
    /** Extra content always shown under the row. */
    children?: React.ReactNode;
    /** Makes the whole row clickable; clicking toggles `details`. */
    onToggle?: () => void;
    /** Whether `details` is currently revealed. */
    expanded?: boolean;
    /** Content revealed under the row while `expanded`. */
    details?: React.ReactNode;
    /** Validation message: paints the card border red and renders the message below the row. */
    error?: string;
    /** Marks the card invalid when the message sits inside it, next to the row it belongs to. */
    invalid?: boolean;
    /** Advisory message: renders below the row without marking the card as invalid. */
    warning?: string;
    /** Disabled cards keep their details closed and cannot be expanded. */
    disabled?: boolean;
    /**
     * `card` (default) is the settings surface. `inline` draws the same row
     * without the surface, for a setting that lives inside another card: a card
     * is never nested in another card.
     */
    variant?: 'card' | 'inline';
}

/**
 * Width of the leading icon column. The messages and the details below the row
 * line up with the title through it, so every row indents its content the same
 * way whether it is a card or not.
 */
const ICON_COLUMN = 20;
const INDENT = `calc(${ICON_COLUMN}px + 2 * var(--mantine-spacing-md))`;

/**
 * One setting, in the shape Windows 11 gives its settings: a layer-filled card
 * with a hairline stroke, the icon, the title and its description, and the
 * control at the end of the row. Rows can be expandable (`onToggle` +
 * `details`), hold extra content (`children`) and show a card-level validation
 * `error`. With `variant="inline"` it loses the surface and becomes a row of
 * the card that contains it.
 */
export const SettingCard: React.FC<SettingCardProps> = ({
    icon,
    title,
    description,
    control,
    value,
    children,
    onToggle,
    expanded = false,
    details,
    error,
    invalid,
    warning,
    disabled = false,
    variant = 'card',
}) => {
    // A disabled setting has nothing to reveal: its row no longer toggles the
    // details and the chevron is gone, so the collapsible cannot be opened.
    const expandable = Boolean(onToggle) && !disabled;
    const open = expandable && expanded;
    const inline = variant === 'inline';
    const row = (
        <Group wrap="nowrap" justify="space-between" gap="md" px={inline ? 0 : 'md'} py="sm">
            <Group wrap="nowrap" gap="md" style={{ flex: 1, minWidth: 0 }}>
                {icon && (
                    <Box
                        style={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            flexShrink: 0,
                            width: ICON_COLUMN,
                            height: ICON_COLUMN,
                        }}
                    >
                        {icon}
                    </Box>
                )}
                <Box style={{ minWidth: 0 }}>
                    <Text size="sm" fw={600}>
                        {title}
                    </Text>
                    {description && (
                        <Text size="xs" c="dimmed" mt={2}>
                            {description}
                        </Text>
                    )}
                </Box>
            </Group>
            <Group gap="sm" wrap="nowrap">
                {value && (
                    <Text size="sm" style={{ textAlign: 'right' }}>
                        {value}
                    </Text>
                )}
                {control}
                {expandable && (
                    <IconChevronDown
                        size={16}
                        stroke={1.6}
                        style={{
                            flexShrink: 0,
                            color: 'var(--mantine-color-dimmed)',
                            transition: 'transform 150ms ease',
                            transform: open ? 'rotate(180deg)' : undefined,
                        }}
                    />
                )}
            </Group>
        </Group>
    );
    const body = (
        <>
            {expandable ? (
                <UnstyledButton
                    className={inline ? 'app-row' : 'app-card-row'}
                    w="100%"
                    onClick={onToggle}
                    aria-expanded={open}
                >
                    {row}
                </UnstyledButton>
            ) : (
                row
            )}
            {error && (
                <Input.Error pb="sm" my={0} pr={inline ? 0 : 'md'} pl={icon ? INDENT : 0}>
                    {error}
                </Input.Error>
            )}
            {warning && (
                <Group
                    gap={6}
                    wrap="nowrap"
                    align="flex-start"
                    pb="sm"
                    pr={inline ? 0 : 'md'}
                    pl={icon ? INDENT : 0}
                >
                    <IconAlertTriangle
                        size={14}
                        stroke={1.8}
                        style={{ color: 'var(--mantine-color-warning)', flexShrink: 0, marginTop: 2 }}
                    />
                    <Text size="xs" style={{ color: 'var(--mantine-color-warning)' }}>
                        {warning}
                    </Text>
                </Group>
            )}
            {children && (
                <Box px={inline ? 0 : 'md'} pb="sm">
                    {children}
                </Box>
            )}
            {details && (
                <Collapse expanded={open}>
                    <Box pt="sm" pb="md" pr={inline ? 0 : 'md'} pl={icon ? INDENT : 0}>
                        {details}
                    </Box>
                </Collapse>
            )}
        </>
    );

    if (inline) return body;

    return (
        <Card
            withBorder
            radius="md"
            p={0}
            className={disabled ? 'app-card app-card--disabled' : 'app-card'}
            style={{
                borderColor: error || invalid ? 'var(--mantine-color-error)' : 'var(--app-card-border)',
            }}
        >
            {body}
        </Card>
    );
};
