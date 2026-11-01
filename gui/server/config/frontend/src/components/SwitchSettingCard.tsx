import React from 'react';
import { Box, Switch } from '@mantine/core';
import { SettingCard, SettingCardProps } from './SettingCard';

export interface SwitchSettingCardProps extends Omit<SettingCardProps, 'control'> {
    /** Whether the setting is on. */
    checked: boolean;
    /** Receives the new state whenever the switch changes. */
    onChange: (checked: boolean) => void;
    /** Called once the change is applied, so the new value can be validated. */
    onCommit?: () => void;
    disabled?: boolean;
    /** Labels for the on and off states (default: Enabled / Disabled). */
    labels?: [string, string];
    /** Accessible name for the switch, when the state labels are not enough. */
    ariaLabel?: string;
}

/**
 * Setting row whose control is a switch labelled with its state. The switch is
 * kept from reaching the row, which toggles the card itself when the card is
 * expandable.
 */
export const SwitchSettingCard: React.FC<SwitchSettingCardProps> = ({
    checked,
    onChange,
    onCommit,
    disabled = false,
    labels = ['Enabled', 'Disabled'],
    ariaLabel,
    ...cardProps
}) => (
    <SettingCard
        {...cardProps}
        disabled={disabled}
        control={
            <Box onClick={(event) => event.stopPropagation()}>
                <Switch
                    checked={checked}
                    onChange={(event) => {
                        onChange(event.currentTarget.checked);
                        onCommit?.();
                    }}
                    label={checked ? labels[0] : labels[1]}
                    labelPosition="left"
                    aria-label={ariaLabel}
                    withThumbIndicator={false}
                    disabled={disabled}
                />
            </Box>
        }
    />
);
