import React, { useEffect, useState } from 'react';
import {
    Button,
    Group,
    Input,
    Modal,
    NumberInput,
    SimpleGrid,
    Stack,
    Text,
} from '@mantine/core';
import { IconNetwork } from '@tabler/icons-react';
import { BattleServerConfig } from '../types';
import { validateIPv4 } from '../validation';
import { AutoManualCard } from './AutoManualCard';
import { IPv4Input } from './IPv4Input';

/**
 * Ports the battle server documentation uses for each game. A new entry starts
 * with those values, so it is a working configuration as soon as it is added.
 */
const BATTLE_SERVER_DEFAULTS: Record<
    string,
    Pick<BattleServerConfig, 'BsPort' | 'WebSocketPort' | 'OutOfBandPort'>
> = {
    age1: { BsPort: 30001, WebSocketPort: 30002, OutOfBandPort: 0 },
    age2: { BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005 },
    age3: { BsPort: 30006, WebSocketPort: 30007, OutOfBandPort: 30008 },
    age4: { BsPort: 30009, WebSocketPort: 30010, OutOfBandPort: 30011 },
    athens: { BsPort: 30012, WebSocketPort: 30013, OutOfBandPort: 30014 },
};

function defaultBattleServer(gameId: string): BattleServerConfig {
    return {
        Region: 'local',
        Name: gameId === 'age2' ? 'Local' : '',
        IPv4: 'auto',
        ...BATTLE_SERVER_DEFAULTS[gameId],
    };
}

interface FieldErrors {
    Region?: string;
    IPv4?: string;
    BsPort?: string;
    WebSocketPort?: string;
    OutOfBandPort?: string;
}

export interface BattleServerDialogProps {
    opened: boolean;
    gameId: string;
    gameName: string;
    /** Existing servers, used to keep the region unique within the game. */
    servers: BattleServerConfig[];
    onAdd: (server: BattleServerConfig) => void;
    onClose: () => void;
}

/**
 * Dialog for adding one battle server. It validates the entry locally before
 * handing it to the form, so the server's Go validation only has to look at
 * complete entries.
 */
export const BattleServerDialog: React.FC<BattleServerDialogProps> = ({
    opened,
    gameId,
    gameName,
    servers,
    onAdd,
    onClose,
}) => {
    const [draft, setDraft] = useState<BattleServerConfig>(() => defaultBattleServer(gameId));
    const [errors, setErrors] = useState<FieldErrors>({});

    useEffect(() => {
        if (!opened) return;
        setDraft(defaultBattleServer(gameId));
        setErrors({});
    }, [opened, gameId]);

    const update = (patch: Partial<BattleServerConfig>) => {
        setDraft((current) => ({ ...current, ...patch }));
        // Editing a field clears its message; the next Add validates again.
        setErrors((current) => {
            const next = { ...current };
            for (const key of Object.keys(patch)) {
                delete next[key as keyof FieldErrors];
            }
            return next;
        });
    };

    const add = () => {
        const nextErrors: FieldErrors = {};
        const region = draft.Region.trim();
        if (!region) {
            nextErrors.Region = 'Must not be empty';
        } else if (servers.some((server) => server.Region === region)) {
            nextErrors.Region = 'Already used by another battle server';
        }
        const address = draft.IPv4.trim();
        if (!address) {
            nextErrors.IPv4 = 'Must not be empty';
        } else if (address !== 'auto' && !validateIPv4(address)) {
            nextErrors.IPv4 = "Must be 'auto' or an IPv4 address";
        }
        if (draft.BsPort < 1 || draft.BsPort > 65535) {
            nextErrors.BsPort = 'Must be between 1 and 65535';
        }
        if (draft.WebSocketPort < 1 || draft.WebSocketPort > 65535) {
            nextErrors.WebSocketPort = 'Must be between 1 and 65535';
        }
        if (gameId !== 'age1' && (draft.OutOfBandPort < 0 || draft.OutOfBandPort > 65535)) {
            nextErrors.OutOfBandPort = 'Must be between 0 and 65535';
        }
        setErrors(nextErrors);
        if (Object.keys(nextErrors).length > 0) return;
        onAdd({ ...draft, Region: region, IPv4: address });
    };

    return (
        <Modal
            opened={opened}
            onClose={onClose}
            title={`Add battle server — ${gameName}`}
            size="lg"
            centered
            radius="lg"
            overlayProps={{ backgroundOpacity: 0.45, blur: 3 }}
            classNames={{
                content: 'app-modal-content',
                header: 'app-modal-header',
                title: 'app-modal-title',
            }}
        >
            <Stack gap="sm">
                <Text size="xs" c="dimmed">
                    The region identifies this battle server in the game and must be unique. The ports must be free
                    and reachable by the clients. Automatic IPv4 uses the address the client uses to reach the server.
                </Text>
                <Input.Wrapper label="Region" required error={errors.Region}>
                    <Input
                        value={draft.Region}
                        onChange={(event) => update({ Region: event.target.value })}
                        placeholder="local"
                    />
                </Input.Wrapper>
                {gameId === 'age2' && (
                    <Input.Wrapper label="Name">
                        <Input
                            value={draft.Name}
                            onChange={(event) => update({ Name: event.target.value })}
                            placeholder="Local"
                        />
                    </Input.Wrapper>
                )}
                <AutoManualCard
                    variant="inline"
                    icon={<IconNetwork size={20} stroke={1.6} />}
                    title="IPv4 address"
                    description="Address the clients use to reach the battle server. Automatic uses the address the client uses to reach the server."
                    value={draft.IPv4}
                    onChange={(value) => update({ IPv4: value })}
                    error={errors.IPv4}
                >
                    {(manual) => <IPv4Input {...manual} />}
                </AutoManualCard>
                <SimpleGrid cols={{ base: 1, xs: gameId === 'age1' ? 2 : 3 }} spacing="sm">
                    <NumberInput
                        label="Battle server port"
                        min={1}
                        max={65535}
                        clampBehavior="blur"
                        value={draft.BsPort}
                        error={errors.BsPort}
                        onChange={(value) =>
                            update({ BsPort: typeof value === 'number' ? value : parseInt(value, 10) || 0 })
                        }
                    />
                    <NumberInput
                        label="WebSocket port"
                        min={1}
                        max={65535}
                        clampBehavior="blur"
                        value={draft.WebSocketPort}
                        error={errors.WebSocketPort}
                        onChange={(value) =>
                            update({ WebSocketPort: typeof value === 'number' ? value : parseInt(value, 10) || 0 })
                        }
                    />
                    {gameId !== 'age1' && (
                        <NumberInput
                            label="Out-of-band port"
                            min={0}
                            max={65535}
                            clampBehavior="blur"
                            value={draft.OutOfBandPort}
                            error={errors.OutOfBandPort}
                            onChange={(value) =>
                                update({
                                    OutOfBandPort: typeof value === 'number' ? value : parseInt(value, 10) || 0,
                                })
                            }
                        />
                    )}
                </SimpleGrid>
                <Group justify="flex-end" mt="sm">
                    <Button variant="default" onClick={onClose}>
                        Cancel
                    </Button>
                    <Button onClick={add}>Add</Button>
                </Group>
            </Stack>
        </Modal>
    );
};
