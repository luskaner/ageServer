import React, { useEffect, useRef } from 'react';
import { Box, Switch } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { SettingCard, SettingCardProps } from './SettingCard';

/** What the control for the manual value receives from the card. */
export interface ManualValueProps {
    value: string;
    onChange: (value: string) => void;
    disabled?: boolean;
}

export interface AutoManualCardProps extends Omit<
    SettingCardProps,
    'control' | 'children' | 'expanded' | 'onToggle' | 'details'
> {
    /** Current setting value: the automatic one, or the manual one. */
    value: string;
    onChange: (value: string) => void;
    /** Value that stands for automatic detection (default: 'auto'). */
    automaticValue?: string;
    /** Called after the switch changes the mode, so the new value is validated. */
    onCommit?: () => void;
    disabled?: boolean;
    /** Renders the control for the manual value, revealed while automatic is off. */
    children: (manual: ManualValueProps) => React.ReactNode;
}

/**
 * Setting row for a value that can stay on `auto`: a switch turns automatic
 * detection on and off, and the manual value sits in the collapsible area the
 * card opens when it is the one in use.
 */
export const AutoManualCard: React.FC<AutoManualCardProps> = ({
    value,
    onChange,
    automaticValue = 'auto',
    onCommit,
    disabled = false,
    children,
    ...cardProps
}) => {
    const automatic = value === automaticValue;
    // What was entered by hand is kept aside while automatic is on, so turning
    // it off again restores it.
    const lastManualValue = useRef(automatic ? '' : value);
    // With automatic detection there is nothing to reveal, so the card only
    // opens while a manual value is in use (and starts open then).
    const [expanded, handlers] = useDisclosure(!automatic && !disabled);
    const closeDetails = handlers.close;

    // Turning the setting off (Internet access, custom files) takes the whole
    // card away: its details close instead of staying open behind a row that
    // can no longer be clicked.
    useEffect(() => {
        if (disabled) closeDetails();
    }, [disabled, closeDetails]);

    const handleModeChange = (nextAutomatic: boolean) => {
        if (nextAutomatic) {
            lastManualValue.current = value;
            onChange(automaticValue);
            handlers.close();
        } else {
            onChange(lastManualValue.current);
            handlers.open();
        }
        onCommit?.();
    };

    return (
        <SettingCard
            {...cardProps}
            disabled={disabled}
            control={
                <Box onClick={(event) => event.stopPropagation()}>
                    <Switch
                        checked={automatic}
                        onChange={(event) => handleModeChange(event.currentTarget.checked)}
                        label={automatic ? 'Automatic' : 'Manual'}
                        labelPosition="left"
                        withThumbIndicator={false}
                        disabled={disabled}
                    />
                </Box>
            }
            expanded={expanded && !automatic}
            onToggle={automatic ? undefined : handlers.toggle}
            details={children({ value: automatic ? '' : value, onChange, disabled })}
        />
    );
};
