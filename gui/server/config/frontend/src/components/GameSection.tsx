import React from 'react';
import {
    ActionIcon,
    Box,
    Button,
    Group,
    Input,
    NumberInput,
    Select,
    SimpleGrid,
    Stack,
    Text,
    TextInput,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import {
    IconAlertTriangle,
    IconCpu,
    IconPlus,
    IconServerCog,
    IconTrash,
    IconWorld,
} from '@tabler/icons-react';
import { AppConfig, BattleServerConfig, EmbeddedBattleServerConfig, EmbeddedBattleServerDefaults, ValidationErrors } from '../types';
import { BattleServerDialog } from './BattleServerDialog';
import { InfoBar } from './InfoBar';
import { IPv4Input } from './IPv4Input';
import { SectionHeading } from './SectionHeading';
import { SettingCard } from './SettingCard';
import { AutoManualCard } from './AutoManualCard';

export interface GameInfo {
    id: string;
    name: string;
    code: string;
}

/**
 * Summary shown on the right of the battle servers card: the server regions in
 * order, separated by commas. The region is unique within the game, so it is
 * enough to identify each server.
 */
function battleServerSummary(servers: BattleServerConfig[]): string {
    if (servers.length === 0) return 'None';
    return servers.map((server) => server.Region.trim() || '—').join(', ');
}

const START_IF_MISSING_OPTIONS = [
    { value: 'true', label: 'Always', hint: 'Start it whenever no other battle server is available.' },
    { value: 'false', label: 'Never', hint: 'Only use battle servers configured or discovered by the manager.' },
    { value: 'required', label: 'Required games only', hint: 'Age of Empires IV and Age of Mythology: Retold only.' },
];

const LOG_LEVEL_OPTIONS = [
    { value: 'none', label: 'None' },
    { value: 'error', label: 'Error' },
    { value: 'warning', label: 'Warning' },
    { value: 'notice', label: 'Notice' },
    { value: 'info', label: 'Info' },
    { value: 'debug', label: 'Debug' },
];

/** Label of the auto-start mode, as the card summary shows it. */
function startIfMissingLabel(value: string): string {
    return START_IF_MISSING_OPTIONS.find((option) => option.value === value)?.label ?? 'Always';
}

/**
 * Embedded battle server of a game whose document leaves the table out. The
 * server only defaults StartIfMissing ("true") and reads an empty IPv4 as
 * "auto" (common/battleconfig); everything else stays at its zero value.
 */
const EMPTY_EMBEDDED_BATTLE_SERVER: EmbeddedBattleServerConfig = {
    StartIfMissing: 'true',
    Region: '',
    Name: '',
    IPv4: 'auto',
    BsPort: 0,
    WebSocketPort: 0,
    OutOfBandPort: 0,
    LogLevel: '',
};

/** Summary of the embedded battle server: which region it announces and when it starts. */
function embeddedBattleServerSummary(ebs: EmbeddedBattleServerConfig, regionFallback: string): string {
    const region = ebs.Region.trim() || regionFallback;
    return `${region} · ${startIfMissingLabel(ebs.StartIfMissing)}`;
}

/**
 * A labelled block inside the embedded battle server details, so the fields read
 * as groups instead of one long column.
 */
const FieldGroup: React.FC<{ label: string; children: React.ReactNode }> = ({ label, children }) => (
    <Stack gap="xs">
        <SectionHeading>{label}</SectionHeading>
        {children}
    </Stack>
);

export interface GameSectionProps {
    game: GameInfo;
    config: AppConfig;
    errors: ValidationErrors;
    warnings: ValidationErrors;
    /** Values the server uses when the document leaves the embedded table out. */
    embeddedDefaults?: EmbeddedBattleServerDefaults;
    /**
     * A game the server does not load is not editable: its cards stay closed and
     * cannot be opened until the game is enabled in the header.
     */
    disabled?: boolean;
    onHostChange: (gameId: string, index: number, value: string) => void;
    onAddHost: (gameId: string) => void;
    onRemoveHost: (gameId: string, index: number) => void;
    onChangeBattleServers: (gameId: string, servers: BattleServerConfig[]) => void;
    onChangeEmbeddedBattleServer: (gameId: string, config: EmbeddedBattleServerConfig) => void;
    onCommit: () => void;
}

/**
 * One game's page: three collapsible cards. Hosts shows the bound addresses as a
 * comma-separated summary and lets them be edited when expanded; Battle servers
 * lists its regions the same way (they are unique within the game) and
 * expanding lists each server's address/ports with a delete button. Adding a
 * server opens a single-entry dialog. Embedded Battle Server configures the
 * built-in battle server that runs inside the ageLAN server process.
 */
export const GameSection: React.FC<GameSectionProps> = ({
    game,
    config,
    errors,
    warnings,
    onHostChange,
    onAddHost,
    onRemoveHost,
    onChangeBattleServers,
    onChangeEmbeddedBattleServer,
    embeddedDefaults,
    disabled = false,
    onCommit,
}) => {
    const [hostsExpanded, hostsHandlers] = useDisclosure(false);
    const [serversExpanded, serversHandlers] = useDisclosure(false);
    const [ebsExpanded, ebsHandlers] = useDisclosure(false);
    const [addServerOpen, addServerHandlers] = useDisclosure(false);

    const gameConfig = config.Games[game.id];
    const hosts = Array.isArray(gameConfig?.Hosts) ? gameConfig.Hosts : [];
    const servers = Array.isArray(gameConfig?.BattleServers) ? gameConfig.BattleServers : [];
    const ebs = gameConfig?.EmbeddedBattleServer ?? EMPTY_EMBEDDED_BATTLE_SERVER;

    const hostsError = errors[`Games.${game.id}.Hosts`];
    const hostRowErrors = hosts.map((_, index) => errors[`Games.${game.id}.Hosts.${index}`]);
    const hostRowWarnings = hosts.map((_, index) => warnings[`Games.${game.id}.Hosts.${index}`]);
    const battleServersError = errors[`Games.${game.id}.BattleServers`];
    const battleServerEntryErrors = Object.keys(errors).filter((field) =>
        field.startsWith(`Games.${game.id}.BattleServers.`),
    );
    const ebsError = errors[`Games.${game.id}.EmbeddedBattleServer`];
    const ebsFieldErrors = (field: string) => errors[`Games.${game.id}.EmbeddedBattleServer.${field}`];

    const updateEbs = (patch: Partial<EmbeddedBattleServerConfig>) => {
        onChangeEmbeddedBattleServer(game.id, { ...ebs, ...patch });
        onCommit();
    };

    // The server fills these in when the document leaves them out; until Go has
    // answered, the labels stand on their own.
    const regionDefault = embeddedDefaults?.Region ?? 'Server';
    const nameDefault = embeddedDefaults?.Name ?? 'Server';
    const logLevelDefault = embeddedDefaults?.LogLevel ?? 'debug';
    // The embedded server only starts when the game has no other battle server:
    // saying so here answers the question the two cards raise together.
    const embeddedStarts = servers.length === 0;

    return (
        <Stack gap="sm">
            {warnings['Games.Enabled'] && (
                <InfoBar severity="warning">
                    <Text size="sm">{warnings['Games.Enabled']}</Text>
                </InfoBar>
            )}
            {disabled && (
                <InfoBar severity="info">
                    <Text size="sm">
                        This game is not enabled, so the server never loads its settings. Turn it on in the header to
                        edit them.
                    </Text>
                </InfoBar>
            )}

            <SettingCard
                icon={<IconWorld size={20} stroke={1.6} />}
                title="Hosts"
                description="IPv4 addresses the server binds to for this game."
                value={hosts.length > 0 ? hosts.join(', ') : 'None'}
                disabled={disabled}
                error={hostsError}
                invalid={hostRowErrors.some((message) => Boolean(message))}
                expanded={hostsExpanded}
                onToggle={hostsHandlers.toggle}
                details={
                    <Stack gap="xs" maw={520}>
                        {hosts.map((host, index) => (
                            <Stack key={index} gap={4}>
                                <Group gap="sm" wrap="nowrap" justify="flex-start">
                                    <IPv4Input
                                        value={host}
                                        onChange={(value) => onHostChange(game.id, index, value)}
                                    />
                                    <ActionIcon
                                        variant="subtle"
                                        color="gray"
                                        size="sm"
                                        aria-label={`Remove host ${index + 1} of ${game.name}`}
                                        // A game needs a host to bind to, so the last row stays.
                                        disabled={hosts.length <= 1}
                                        onClick={() => onRemoveHost(game.id, index)}
                                    >
                                        <IconTrash size={16} stroke={1.6} />
                                    </ActionIcon>
                                </Group>
                                {hostRowErrors[index] && <Input.Error>{hostRowErrors[index]}</Input.Error>}
                                {!hostRowErrors[index] && hostRowWarnings[index] && (
                                    <Group gap={6} wrap="nowrap" align="flex-start">
                                        <IconAlertTriangle
                                            size={13}
                                            stroke={1.8}
                                            style={{
                                                color: 'var(--mantine-color-warning)',
                                                flexShrink: 0,
                                                marginTop: 2,
                                            }}
                                        />
                                        <Text size="xs" style={{ color: 'var(--mantine-color-warning)' }}>
                                            {hostRowWarnings[index]}
                                        </Text>
                                    </Group>
                                )}
                            </Stack>
                        ))}
                        <Button
                            variant="subtle"
                            size="compact-sm"
                            w="fit-content"
                            leftSection={<IconPlus size={15} stroke={1.8} />}
                            onClick={() => onAddHost(game.id)}
                        >
                            Add host
                        </Button>
                    </Stack>
                }
            />

            <SectionHeading level="section" spaced>
                Battle servers
            </SectionHeading>
            <Text size="xs" c="dimmed" mb={2}>
                Whatever is configured here, the server also uses the battle servers the battle-server-manager finds
                on this PC.
            </Text>

            <SettingCard
                icon={<IconServerCog size={20} stroke={1.6} />}
                title="Standalone"
                description="Battle servers this document announces for the game."
                value={battleServerSummary(servers)}
                disabled={disabled}
                error={battleServersError}
                invalid={battleServerEntryErrors.length > 0}
                expanded={serversExpanded}
                onToggle={serversHandlers.toggle}
                details={
                    <Stack gap="sm" maw={720}>
                        {servers.length === 0 && (
                            <Text size="sm" c="dimmed">
                                No battle server is announced by this document. The embedded one below covers the
                                game when it is allowed to start, and the battle-server-manager may still find one on
                                this PC.
                            </Text>
                        )}
                        {servers.map((server, index) => {
                            const prefix = `Games.${game.id}.BattleServers.${index}`;
                            const entryErrors = Object.entries(errors)
                                .filter(([field]) => field.startsWith(`${prefix}.`))
                                .map(([, message]) => message);
                            return (
                                <Group key={index} gap="sm" wrap="nowrap" justify="space-between" align="flex-start">
                                    <Box style={{ minWidth: 0 }}>
                                        <Text size="sm" fw={500}>
                                            {server.Region || `Battle server ${index + 1}`}
                                            {server.Name ? ` — ${server.Name}` : ''}
                                        </Text>
                                        <Text size="xs" c="dimmed">
                                            IPv4: {server.IPv4} · Ports: {server.BsPort} / {server.WebSocketPort}
                                            {game.id !== 'age1' ? ` / ${server.OutOfBandPort}` : ''}
                                        </Text>
                                        {entryErrors.length > 0 && (
                                            <Input.Error>{entryErrors.join(' · ')}</Input.Error>
                                        )}
                                    </Box>
                                    <ActionIcon
                                        variant="subtle"
                                        color="red"
                                        size="sm"
                                        aria-label={`Remove battle server ${index + 1} of ${game.name}`}
                                        onClick={() => {
                                            const next = [...servers];
                                            next.splice(index, 1);
                                            onChangeBattleServers(game.id, next);
                                            onCommit();
                                        }}
                                    >
                                        <IconTrash size={16} stroke={1.6} />
                                    </ActionIcon>
                                </Group>
                            );
                        })}
                        <Button
                            variant="subtle"
                            size="compact-sm"
                            w="fit-content"
                            leftSection={<IconPlus size={15} stroke={1.8} />}
                            onClick={addServerHandlers.open}
                        >
                            Add battle server
                        </Button>
                    </Stack>
                }
            />
            <SettingCard
                icon={<IconCpu size={20} stroke={1.6} />}
                title="Embedded"
                description="Battle server that runs inside the ageLAN server process, so the game works without a separate battle server program."
                value={embeddedBattleServerSummary(ebs, regionDefault)}
                disabled={disabled}
                error={ebsError}
                expanded={ebsExpanded}
                onToggle={ebsHandlers.toggle}
                details={
                    <Stack gap="md">
                        <InfoBar severity={embeddedStarts ? 'info' : 'warning'}>
                            <Text size="sm">
                                {embeddedStarts
                                    ? 'This game has no battle server of its own, so the embedded one is the one the server announces, unless the battle-server-manager finds another on this PC.'
                                    : `This game already has ${servers.length === 1 ? 'a battle server' : `${servers.length} battle servers`}. The embedded one only starts when neither this document nor the battle-server-manager provides one.`}
                            </Text>
                        </InfoBar>

                        <FieldGroup label="Startup">
                            <Select
                                label="Start if missing"
                                description={
                                    START_IF_MISSING_OPTIONS.find((option) => option.value === ebs.StartIfMissing)
                                        ?.hint ?? 'Start the embedded battle server when no other one is available.'
                                }
                                value={ebs.StartIfMissing || null}
                                data={START_IF_MISSING_OPTIONS.map(({ value, label }) => ({ value, label }))}
                                onChange={(value) => value && updateEbs({ StartIfMissing: value })}
                                error={ebsFieldErrors('StartIfMissing')}
                                placeholder="Always"
                                allowDeselect={false}
                                w={{ base: '100%', sm: 360 }}
                            />
                            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
                                <TextInput
                                    label="Region"
                                    description={`Empty announces it as “${regionDefault}”`}
                                    value={ebs.Region}
                                    onChange={(event) => updateEbs({ Region: event.target.value })}
                                    placeholder={regionDefault}
                                    error={ebsFieldErrors('Region')}
                                />
                                <TextInput
                                    label="Name"
                                    description="Only used by Age of Empires II: DE."
                                    value={ebs.Name}
                                    onChange={(event) => updateEbs({ Name: event.target.value })}
                                    placeholder={nameDefault}
                                    error={ebsFieldErrors('Name')}
                                />
                            </SimpleGrid>
                        </FieldGroup>

                        <FieldGroup label="Address">
                            <AutoManualCard
                                variant="inline"
                                icon={<IconWorld size={20} stroke={1.6} />}
                                title="IPv4 address"
                                description="Address the clients use to reach the embedded battle server. Automatic uses the address the client uses to reach the server."
                                value={ebs.IPv4}
                                onChange={(value) => updateEbs({ IPv4: value })}
                                error={ebsFieldErrors('IPv4')}
                            >
                                {(manual) => <IPv4Input {...manual} />}
                            </AutoManualCard>
                        </FieldGroup>

                        <FieldGroup label="Ports">
                            <SimpleGrid cols={{ base: 1, xs: 3 }} spacing="sm">
                                <NumberInput
                                    label="Battle server"
                                    description="0 picks a free port"
                                    min={0}
                                    max={65535}
                                    clampBehavior="blur"
                                    value={ebs.BsPort}
                                    error={ebsFieldErrors('BsPort')}
                                    onChange={(value) =>
                                        updateEbs({ BsPort: typeof value === 'number' ? value : parseInt(value, 10) || 0 })
                                    }
                                />
                                <NumberInput
                                    label="WebSocket"
                                    description="0 picks a free port"
                                    min={0}
                                    max={65535}
                                    clampBehavior="blur"
                                    value={ebs.WebSocketPort}
                                    error={ebsFieldErrors('WebSocketPort')}
                                    onChange={(value) =>
                                        updateEbs({ WebSocketPort: typeof value === 'number' ? value : parseInt(value, 10) || 0 })
                                    }
                                />
                                {game.id !== 'age1' && (
                                    <NumberInput
                                        label="Out-of-band"
                                        description="0 picks a free port"
                                        min={0}
                                        max={65535}
                                        clampBehavior="blur"
                                        value={ebs.OutOfBandPort}
                                        error={ebsFieldErrors('OutOfBandPort')}
                                        onChange={(value) =>
                                            updateEbs({ OutOfBandPort: typeof value === 'number' ? value : parseInt(value, 10) || 0 })
                                        }
                                    />
                                )}
                            </SimpleGrid>
                        </FieldGroup>

                        <FieldGroup label="Behaviour">
                            <Select
                                label="Log level"
                                description="Detail the embedded battle server prints."
                                value={ebs.LogLevel || null}
                                data={LOG_LEVEL_OPTIONS}
                                onChange={(value) => updateEbs({ LogLevel: value ?? '' })}
                                error={ebsFieldErrors('LogLevel')}
                                placeholder={`Server default (${logLevelDefault})`}
                                allowDeselect={false}
                                w={{ base: '100%', sm: 360 }}
                            />
                        </FieldGroup>
                    </Stack>
                }
            />
            <BattleServerDialog
                opened={addServerOpen}
                gameId={game.id}
                gameName={game.name}
                servers={servers}
                onAdd={(server) => {
                    onChangeBattleServers(game.id, [...servers, server]);
                    onCommit();
                    addServerHandlers.close();
                }}
                onClose={addServerHandlers.close}
            />
        </Stack>
    );
};