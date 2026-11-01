import React, { useEffect, useRef } from 'react';
import { Button, Group, TextInput } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconFolderOpen } from '@tabler/icons-react';
import { ConfigService } from '../../bindings/changeme';
import { ManualValueProps } from './AutoManualCard';

export interface FolderPickerInputProps extends ManualValueProps {
    /** Called once a folder was picked, so the new value can be validated. */
    onSelect?: () => void;
}

/**
 * Path shown read-only next to the button that opens the platform's folder
 * picker: the folder is chosen in the native dialog, never typed by hand.
 */
export const FolderPickerInput: React.FC<FolderPickerInputProps> = ({
    value,
    onChange,
    disabled = false,
    onSelect,
}) => {
    // The native picker is modal, so the button has no busy state to show: it
    // looks the same while the dialog is open. The flag only keeps a second
    // call from piling up when the dialog is closed before the binding
    // answers; it is not rendered, so a slow answer never leaves the button
    // looking like it is still loading.
    const picking = useRef(false);
    const pickGeneration = useRef(0);

    // The window gets the focus back as soon as the native picker closes, which
    // can be a little before the binding answers: releasing the flag here lets
    // the next pick start right away instead of waiting for that answer.
    useEffect(() => {
        const releasePicker = () => {
            picking.current = false;
        };
        window.addEventListener('focus', releasePicker);
        return () => window.removeEventListener('focus', releasePicker);
    }, []);

    const pickFolder = async () => {
        if (picking.current) return;
        picking.current = true;
        const generation = ++pickGeneration.current;
        try {
            // Closing the picker answers with an empty path, which keeps the
            // folder that is already set.
            const folder = await ConfigService.SelectCustomFilesFolder();
            if (!folder) return;
            onChange(folder);
            onSelect?.();
        } catch (error) {
            notifications.show({
                color: 'red',
                title: 'Folder not selected',
                message: error instanceof Error ? error.message : String(error),
            });
        } finally {
            // A pick started after this one owns the flag now.
            if (generation === pickGeneration.current) picking.current = false;
        }
    };

    return (
        <Group gap={0} wrap="nowrap" align="center" w="100%">
            <TextInput
                value={value}
                readOnly
                disabled={disabled}
                placeholder="No folder selected"
                aria-label="Custom files folder"
                title={value || undefined}
                style={{ flex: 1, minWidth: 0 }}
                styles={{
                    input: {
                        cursor: 'default',
                        borderTopRightRadius: 0,
                        borderBottomRightRadius: 0,
                    },
                }}
            />
            <Button
                variant="default"
                leftSection={<IconFolderOpen size={16} stroke={1.6} />}
                onClick={pickFolder}
                disabled={disabled}
                styles={{
                    root: {
                        flexShrink: 0,
                        borderTopLeftRadius: 0,
                        borderBottomLeftRadius: 0,
                        borderLeftWidth: 0,
                    },
                }}
            >
                Browse…
            </Button>
        </Group>
    );
};
