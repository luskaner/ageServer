import React, { useEffect, useRef, useState } from 'react';
import {
    ActionIcon,
    AppShell,
    Box,
    Burger,
    Button,
    Group,
    Input,
    List,
    Menu,
    Modal,
    NavLink,
    NumberInput,
    Radio,
    ScrollArea,
    Stack,
    Switch,
    Text,
    Title,
    Tooltip,
    UnstyledButton,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { useDebouncedValue, useDisclosure, useMediaQuery } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import {
    IconAlertTriangle,
    IconBroadcast,
    IconChevronDown,
    IconCircleCheck,
    IconClock,
    IconDatabase,
    IconDeviceFloppy,
    IconDeviceGamepad2,
    IconFile,
    IconFiles,
    IconFolder,
    IconFolderOpen,
    IconId,
    IconLogs,
    IconMovie,
    IconNetwork,
    IconPlus,
    IconSettings,
    IconShield,
    IconSignature,
    IconSpeakerphone,
    IconTrash,
    IconWorld,
} from '@tabler/icons-react';
import { AppConfig, AVAILABLE_GAMES, EmbeddedBattleServerDefaults, ValidationErrors, errorMessage, fieldLabel, humanMessage } from '../types';
import { AutoManualCard } from './AutoManualCard';
import { FolderPickerInput } from './FolderPickerInput';
import { GameSection } from './GameSection';
import { IPv4Input } from './IPv4Input';
import { InfoBar } from './InfoBar';
import { SectionHeading } from './SectionHeading';
import { SettingCard } from './SettingCard';
import { SwitchSettingCard } from './SwitchSettingCard';
import { UnitInput, UnitInputUnit } from './UnitInput';
import { ConfigFile, ConfigService, Configuration, ValidationError } from '../../bindings/changeme';
function toServiceConfiguration(config: AppConfig): Configuration {
    const game = (gameId: string) => ({
        Hosts: config.Games[gameId]?.Hosts ?? [],
        BattleServers: config.Games[gameId]?.BattleServers ?? [],
        EmbeddedBattleServer: config.Games[gameId]?.EmbeddedBattleServer ?? {
            StartIfMissing: 'true',
            Region: '',
            Name: '',
            IPv4: 'auto',
            BsPort: 0,
            WebSocketPort: 0,
            OutOfBandPort: 0,
            SimulationPeriod: 0,
            LogLevel: '',
        },
    });
    return {
        Log: config.Log,
        GeneratePlatformUserId: config.GeneratePlatformUserId,
        Sign: { Enabled: config.Sign.Enabled, PlatformUserIdExceptions: config.Sign.PlatformUserIdExceptions },
        Authentication: config.Authentication,
        Internet: { Enabled: config.Internet.Enabled, IP: config.Internet.IP },
        Games: {
            Enabled: config.Games.Enabled,
            age1: game('age1'),
            age2: game('age2'),
            age3: game('age3'),
            age4: game('age4'),
            athens: game('athens'),
        },
        Announcement: config.Announcement,
        CustomFiles: config.CustomFiles,
        BattleServerReplays: config.BattleServerReplays ?? {
            Enabled: false,
            Folder: 'auto',
            MaxFileSize: '32MB',
            MaxTotalSize: '256MB',
            Expiry: '12h',
        },
    };
}
type GameSectionKey = `game:${string}`;
type StaticSectionKey = 'general' | 'internet' | 'announcement' | 'customFiles' | 'replays' | 'sign';
type SectionKey = StaticSectionKey | GameSectionKey;
const SECTION_LABELS: Record<StaticSectionKey, { title: string; subtitle: string }> = {
    general: { title: 'General', subtitle: 'Basic server configuration options.' },
    internet: { title: 'Internet', subtitle: 'Internet access and the address the server announces.' },
    announcement: { title: 'Announcement', subtitle: 'Configure LAN discovery and announcement settings.' },
    customFiles: { title: 'Custom files', subtitle: 'Storage of the savegames and replays uploaded by the game.' },
    replays: { title: 'Replays', subtitle: 'Recording of the match data the embedded battle server receives.' },
    sign: { title: 'Sign', subtitle: 'Configure catalogue signing and platform user exceptions.' },
};
function isGameSection(section: SectionKey): section is GameSectionKey {
    return section.startsWith('game:');
}
function sectionInfo(section: SectionKey): { title: string; subtitle: string } {
    if (isGameSection(section)) {
        const game = AVAILABLE_GAMES.find((candidate) => `game:${candidate.id}` === section);
        return {
            title: game?.name ?? 'Game',
            subtitle: 'Hosts, battle servers and the embedded battle server for this game.',
        };
    }
    return SECTION_LABELS[section];
}
const AUTH_OPTIONS = [
    { value: 'disabled', label: 'Disabled', description: 'No authentication as it worked before.' },
    { value: 'required', label: 'Required', description: 'Authenticate the user with the official servers.' },
    {
        value: 'cached',
        label: 'Cached',
        description: 'Authenticate with the official server at least once every 30 days.',
    },
    {
        value: 'adaptive',
        label: 'Adaptive',
        description: "Resolve to 'cached' if internet is detected, or 'disabled' otherwise.",
    },
];
/** Sizes are stored with their suffix, and the server reads MB and GB. */ const SIZE_UNITS: UnitInputUnit[] = [
    { label: 'MB', storedUnit: 'MB' },
    { label: 'GB', storedUnit: 'GB' },
];
/** * The server parses Go durations, which have hours but no days: a day picked * in the dropdown is written as its 24 hours. */ const EXPIRY_UNITS: UnitInputUnit[] =
    [
        { label: 'hours', storedUnit: 'h' },
        { label: 'days', storedUnit: 'h', factor: 24 },
    ];
/** Section that owns a validation error, so the summary can jump to it. */ function sectionForField(
    field: string,
): SectionKey {
    if (field.startsWith('Games.')) {
        const gameId = field.split('.')[1];
        if (AVAILABLE_GAMES.some((game) => game.id === gameId)) return `game:${gameId}`;
        return 'general';
    }
    if (field.startsWith('Internet.')) return 'internet';
    if (field.startsWith('Announcement.')) return 'announcement';
    if (field.startsWith('CustomFiles.')) return 'customFiles';
    if (field.startsWith('BattleServerReplays.')) return 'replays';
    if (field.startsWith('Sign.')) return 'sign';
    return 'general';
}
/** Splits the result of the Go validation into blocking errors and warnings. */ function splitIssues(
    issues: ValidationError[] | null | undefined,
): { errors: ValidationErrors; warnings: ValidationErrors } {
    const errors: ValidationErrors = {};
    const warnings: ValidationErrors = {};
    (issues ?? []).forEach((issue) => {
        if (!issue?.field) return;
        if (issue.severity === 'warning') warnings[issue.field] = humanMessage(issue.message);
        else errors[issue.field] = humanMessage(issue.message);
    });
    return { errors, warnings };
}
/** * Keeps the messages that the live pass still reports while it reflects the * current values: one goes away as soon as its field is fine again, without * waiting for another blur. The live pass never brings a message up. */ function currentMessages(
    shown: ValidationErrors,
    live: ValidationErrors,
    liveIsCurrent: boolean,
): ValidationErrors {
    if (!liveIsCurrent) return shown;
    return Object.fromEntries(
        Object.entries(shown)
            .filter(([field]) => live[field])
            .map(([field]) => [field, live[field]]),
    );
}
interface Screen2Props {
    initialConfig: AppConfig;
    backendValidationErrors: ValidationError[];
    embeddedDefaults?: Record<string, EmbeddedBattleServerDefaults | undefined> | undefined;
    fileName: string;
    isNew: boolean;
    onSaved: (file: ConfigFile) => void;
    onBack: () => void;
}
export const Screen2_ConfigEditor: React.FC<Screen2Props> = ({
    initialConfig,
    backendValidationErrors,
    embeddedDefaults,
    fileName,
    isNew,
    onSaved,
    onBack,
}) => {
    const form = useForm<AppConfig>({ mode: 'controlled', initialValues: initialConfig });
    const config = form.values;
    const [authExpanded, authExpandedHandlers] = useDisclosure(false);
    const [multicastExpanded, multicastExpandedHandlers] = useDisclosure(false);
    const [exceptionsExpanded, exceptionsExpandedHandlers] = useDisclosure(false); // Multicast sits under the LAN announcement: switching the announcement off
    // takes the whole card away, so its details close instead of staying open
    // behind a row that can no longer be clicked.
    const closeMulticastDetails = multicastExpandedHandlers.close;
    useEffect(() => {
        if (!config.Announcement.Enabled) closeMulticastDetails();
    }, [config.Announcement.Enabled, closeMulticastDetails]);
    const [navbarOpened, navbarHandlers] = useDisclosure(true); // Below the App Shell breakpoint the sidebar becomes an overlay on top of the
    // content instead of a column beside it, so it keeps its own state (closed to
    // begin with) and a surface of its own: transparent, it would let the content
    // show through as it slid over it.
    const [overlayNavbarOpened, overlayNavbarHandlers] = useDisclosure(false);
    const isNarrowWindow = useMediaQuery('(max-width: 47.99375em)'); // Matches the App Shell media query that switches the navbar into an
    // overlay (breakpoint minus a tenth of a pixel).
    const sidebarOpened = isNarrowWindow ? overlayNavbarOpened : navbarOpened;
    const toggleSidebar = isNarrowWindow ? overlayNavbarHandlers.toggle : navbarHandlers.toggle;
    const sidebarWidth = 240;
    const [activeSection, setActiveSection] = useState<SectionKey>('general');
    const [gamesOpened, gamesHandlers] = useDisclosure(true);
    const activeGame = isGameSection(activeSection)
        ? AVAILABLE_GAMES.find((game) => `game:${game.id}` === activeSection)
        : undefined;
    const selectSection = (section: SectionKey) => {
        if (isGameSection(section)) gamesHandlers.open();
        setActiveSection(section);
    }; // The file's own validation result seeds the messages shown when it opens;
    // every blur (or committed control) replaces them with the fresh result.
    const [backendIssues] = useState(() => splitIssues(backendValidationErrors));
    const [errors, setErrors] = useState<ValidationErrors>(backendIssues.errors);
    const [warnings, setWarnings] = useState<ValidationErrors>(backendIssues.warnings); // Values that were last validated. They only change when a control loses
    // focus, so neither the field errors nor the general summary react to
    // every keystroke.
    const [validationValues, setValidationValues] = useState<AppConfig>(initialConfig); // Validation rules live in Go (ConfigService.ValidateConfiguration). The
    // frontend only asks Go to evaluate them and renders the resulting errors.
    useEffect(() => {
        let cancelled = false;
        ConfigService.ValidateConfiguration(toServiceConfiguration(validationValues))
            .then((result) => {
                if (cancelled) return;
                const issues = splitIssues(result);
                setErrors(issues.errors);
                setWarnings(issues.warnings);
            })
            .catch(() => {
                // Keep the last known errors if validation cannot be reached.
            });
        return () => {
            cancelled = true;
        };
    }, [validationValues]); // Blur is the moment a value is considered finished, and the same request
    // is made by the actions that edit a list as a whole. The timeout lets the
    // control's own blur handling settle first (NumberInput clamps on blur),
    // so the values that reach Go are the ones the control shows.
    const latestValues = useRef(form.values);
    latestValues.current = form.values;
    const requestValidation = () => {
        window.setTimeout(() => setValidationValues(latestValues.current), 0);
    }; // Save, though, has to know whether the form is valid right now: an invalid
    // value keeps it disabled while it is still being typed, before the blur
    // that brings up the message. This pass runs on a short debounce, and its
    // result is never rendered as an error by itself.
    const [liveIssues, setLiveIssues] = useState(backendIssues); // Values the live result was computed for. The debounce catching up is not
    // an answer yet: until the pass for the values on screen arrives, the last
    // result stays out of the way instead of briefly filtering the messages
    // that are already shown (which reads as the summary flickering).
    const [liveValues, setLiveValues] = useState<AppConfig>(initialConfig);
    const [debouncedValues] = useDebouncedValue(form.values, 200);
    useEffect(() => {
        let cancelled = false;
        ConfigService.ValidateConfiguration(toServiceConfiguration(debouncedValues))
            .then((result) => {
                if (cancelled) return;
                setLiveIssues(splitIssues(result));
                setLiveValues(debouncedValues);
            })
            .catch(() => {
                // Keep the last known state if validation cannot be reached.
            });
        return () => {
            cancelled = true;
        };
    }, [debouncedValues]); // Controls that commit a value by themselves (a switch, a radio) have no blur
    // of their own, so they ask for validation as soon as they change; only text
    // typed inside a field waits until it is left.
    const commitProps = <Props extends { onChange?: (value: any) => void }>(props: Props): Props => ({
        ...props,
        onChange: (value: any) => {
            props.onChange?.(value);
            requestValidation();
        },
    }); // Switch state as the form holds it, with the change marked as finished:
    // SwitchSettingCard renders the control and validates through `onCommit`.
    const switchProps = (path: string) => {
        const props = form.getInputProps(path, { type: 'checkbox' });
        return {
            checked: Boolean(props.checked),
            onChange: (checked: boolean) => props.onChange(checked),
            onCommit: requestValidation,
        };
    }; // While the live pass reflects the values as they are right now, it also
    // decides which messages stay: one goes away as soon as its field is fine
    // again, without waiting for another blur. A message never *appears* this
    // way, only disappears, so nothing shows up mid-typing.
    const liveIsCurrent = liveValues === form.values;
    const allErrors = currentMessages(errors, liveIssues.errors, liveIsCurrent);
    const allWarnings = currentMessages(warnings, liveIssues.warnings, liveIsCurrent);
    const errorEntries = Object.entries(allErrors); // Save follows the live pass: the messages above are a snapshot taken when a
    // control was left, so a value typed right now is not in it yet. Warnings are
    // advisories and never block saving.
    const hasErrors = Object.keys(liveIssues.errors).length > 0;
    const handleGameToggle = (gameId: string) => {
        const enabled = [...form.values.Games.Enabled];
        const index = enabled.indexOf(gameId);
        if (index === -1) {
            enabled.push(gameId);
        } else {
            enabled.splice(index, 1);
        }
        form.setFieldValue('Games.Enabled', enabled);
        requestValidation();
    };
    const handleGameHostChange = (gameId: string, index: number, value: string) => {
        form.setFieldValue(`Games.${gameId}.Hosts.${index}`, value);
    }; // Adding and removing a host change the shape of the list, which is not
    // something that produces a blur: they count as finished edits themselves.
    const handleAddGameHost = (gameId: string) => {
        const hosts = form.values.Games[gameId]?.Hosts ?? [];
        form.setFieldValue(`Games.${gameId}.Hosts`, [...hosts, '0.0.0.0']);
        requestValidation();
    };
    const handleRemoveGameHost = (gameId: string, index: number) => {
        const hosts = [...(form.values.Games[gameId]?.Hosts ?? [])];
        hosts.splice(index, 1);
        form.setFieldValue(`Games.${gameId}.Hosts`, hosts);
        requestValidation();
    };
    const handlePortChange = (value: string | number) => {
        const port = typeof value === 'string' ? Number.parseInt(value, 10) : value;
        if (Number.isNaN(port)) return;
        form.setFieldValue('Announcement.Port', port);
    };
    const showSuccessToast = (message: string) => {
        notifications.show({ message, color: 'accent', icon: <IconCircleCheck size={20} />, autoClose: 4000 });
    };
    const showErrorToast = (message: string) => {
        notifications.show({ message, color: 'red', icon: <IconAlertTriangle size={20} />, autoClose: 6000 });
    };
    // The webview has no window.confirm (the native prompt is never shown, so
    // it always answers "no"): overwriting a file is confirmed with the app's
    // own dialog. The promise keeps the save code reading top to bottom.
    const [confirmRequest, setConfirmRequest] = useState<{ message: string; resolve: (ok: boolean) => void } | null>(
        null,
    );
    const askConfirmation = (message: string) =>
        new Promise<boolean>((resolve) => setConfirmRequest({ message, resolve }));
    const answerConfirmation = (confirmed: boolean) => {
        confirmRequest?.resolve(confirmed);
        setConfirmRequest(null);
    };
    const handleSave = async () => {
        if (hasErrors) return;
        try {
            if (!isNew) {
                await ConfigService.SaveConfiguration(fileName, toServiceConfiguration(form.values));
                showSuccessToast('Configuration saved successfully.');
                return;
            }
            const destination = await ConfigService.SelectConfigDestination();
            if (!destination) return;
            try {
                const created = await ConfigService.CreateConfigFile(
                    destination,
                    toServiceConfiguration(form.values),
                    false,
                );
                if (!created) throw new Error('The created configuration was not returned.');
                onSaved(created);
                showSuccessToast('Configuration created successfully.');
            } catch (error) {
                if (!errorMessage(error).includes('confirmation required')) throw error;
                // Declining the overwrite is a decision, not a failure.
                if (!(await askConfirmation('The selected file already exists. Overwrite it?'))) return;
                const created = await ConfigService.CreateConfigFile(
                    destination,
                    toServiceConfiguration(form.values),
                    true,
                );
                if (!created) throw new Error('The overwritten configuration was not returned.');
                onSaved(created);
                showSuccessToast('Configuration overwritten successfully.');
            }
        } catch (error) {
            showErrorToast(`Error saving configuration: ${errorMessage(error)}`);
        }
    };
    const handleSaveAs = async () => {
        if (hasErrors) return;
        try {
            const destination = await ConfigService.SelectConfigSaveAsDestination(fileName);
            if (!destination) return;
            try {
                const saved = await ConfigService.SaveConfigurationAs(
                    fileName,
                    destination,
                    toServiceConfiguration(form.values),
                    false,
                );
                if (!saved) throw new Error('The saved configuration was not returned.');
                onSaved(saved);
                showSuccessToast('Configuration saved successfully to the new location.');
            } catch (error) {
                if (!errorMessage(error).includes('confirmation required')) throw error;
                // Declining the overwrite is a decision, not a failure.
                if (!(await askConfirmation('The selected file already exists. Overwrite it?'))) return;
                const saved = await ConfigService.SaveConfigurationAs(
                    fileName,
                    destination,
                    toServiceConfiguration(form.values),
                    true,
                );
                if (!saved) throw new Error('The overwritten configuration was not returned.');
                onSaved(saved);
                showSuccessToast('Configuration overwritten successfully.');
            }
        } catch (error) {
            showErrorToast(`Error saving configuration: ${errorMessage(error)}`);
        }
    };
    const authLabel = AUTH_OPTIONS.find((option) => option.value === config.Authentication)?.label ?? 'Disabled';
    // The exceptions are a list, so the card summarises how many there are
    // instead of printing every id on the row.
    const exceptionCount = (config.Sign.PlatformUserIdExceptions ?? []).length;
    const exceptionsValue =
        exceptionCount === 0 ? 'None' : `${exceptionCount} ${exceptionCount === 1 ? 'exception' : 'exceptions'}`;
    const section = sectionInfo(activeSection);
    const navItems: Array<{ key: StaticSectionKey; label: string; icon: React.ReactNode }> = [
        { key: 'general', label: 'General', icon: <IconSettings size={18} stroke={1.6} /> },
        { key: 'internet', label: 'Internet', icon: <IconWorld size={18} stroke={1.6} /> },
        { key: 'announcement', label: 'Announcement', icon: <IconSpeakerphone size={18} stroke={1.6} /> },
        { key: 'customFiles', label: 'Custom files', icon: <IconFiles size={18} stroke={1.6} /> },
        { key: 'replays', label: 'Replays', icon: <IconMovie size={18} stroke={1.6} /> },
        { key: 'sign', label: 'Sign', icon: <IconSignature size={18} stroke={1.6} /> },
    ];
    // A page whose whole feature hangs on one switch carries it next to the
    // title, the way a game page carries the game's, so the page itself only
    // lists the settings that switch turns on.
    const headerToggle = (() => {
        switch (activeSection) {
            case 'announcement':
                return { ...switchProps('Announcement.Enabled'), ariaLabel: 'LAN announcement' };
            case 'customFiles':
                return { ...switchProps('CustomFiles.Enabled'), ariaLabel: 'Custom files' };
            case 'replays':
                return { ...switchProps('BattleServerReplays.Enabled'), ariaLabel: 'Replay recording' };
            default:
                return null;
        }
    })(); // WinUI 3 navigation items: rounded rows inset from the pane edges, with
    // a subtle highlight on hover and when selected instead of an accent
    // fill. The states cannot go through the styles prop (it only supports
    // flat properties), so they live in this small stylesheet.
    const winuiNavItemClass = 'winui-nav-item';
    const winuiNavItemCss = `        .${winuiNavItemClass} [data-position='left'] {            color: var(--mantine-color-dimmed);        }        @media (hover: hover) {            .${winuiNavItemClass}:hover {                background-color: color-mix(in srgb, var(--mantine-color-text) 5%, transparent);            }        }        .${winuiNavItemClass}[data-active] {            background-color: color-mix(in srgb, var(--mantine-color-text) 7%, transparent);            color: var(--mantine-color-text);        }        .${winuiNavItemClass}[data-active] [data-position='left'] {            color: var(--mantine-color-text);        }    `;
    const renderStaticNavItem = (item: { key: StaticSectionKey; label: string; icon: React.ReactNode }) => (
        <NavLink
            key={item.key}
            label={item.label}
            leftSection={item.icon}
            active={activeSection === item.key}
            onClick={() => selectSection(item.key)}
            className={winuiNavItemClass}
            styles={{
                root: {
                    // WinUI 3 navigation item: rounded. The inset comes from the
                    // section padding (the items are width: 100%, so a margin
                    // would overflow the pane and bring up a horizontal
                    // scrollbar).
                    borderRadius: 4,
                    paddingBlock: 8,
                    paddingInline: 8,
                },
            }}
        />
    );
    return (
        <AppShell
            header={{ height: 64 }}
            navbar={{
                width: sidebarWidth,
                breakpoint: 'sm',
                collapsed: { mobile: !overlayNavbarOpened, desktop: !navbarOpened },
            }}
            padding="lg" // The window must not scroll as a whole (see AppShell.Main below), so
            // the shell keeps clear of the fixed header with padding: a margin
            // here would collapse out of the shell and leave the window
            // scrollable by exactly the header's height.
            style={{ paddingTop: 'var(--app-shell-header-offset, 0rem)' }}
        >
            <AppShell.Header>
                <Group h="100%" px="md" justify="space-between" wrap="nowrap" gap="md">
                    <Group gap="md" wrap="nowrap" style={{ flex: 1, minWidth: 0 }}>
                        <Burger
                            opened={sidebarOpened}
                            onClick={toggleSidebar}
                            size="sm"
                            aria-label={sidebarOpened ? 'Hide sidebar' : 'Show sidebar'}
                        />
                        <Box style={{ minWidth: 0 }}>
                            <Title order={3}>{section.title}</Title>
                            <Text size="xs" c="dimmed" truncate>
                                {section.subtitle}
                            </Text>
                        </Box>
                    </Group>
                    <Group gap="md" wrap="nowrap">
                        {activeGame && (
                            <Switch
                                checked={config.Games.Enabled.includes(activeGame.id)}
                                onChange={() => handleGameToggle(activeGame.id)}
                                label={config.Games.Enabled.includes(activeGame.id) ? 'Enabled' : 'Disabled'}
                                labelPosition="left"
                                aria-label={`Enable ${activeGame.name}`}
                                withThumbIndicator={false}
                            />
                        )}
                        {headerToggle && (
                            <Switch
                                checked={headerToggle.checked}
                                onChange={(event) => {
                                    headerToggle.onChange(event.currentTarget.checked);
                                    requestValidation();
                                }}
                                label={headerToggle.checked ? 'Enabled' : 'Disabled'}
                                labelPosition="left"
                                aria-label={headerToggle.ariaLabel}
                                withThumbIndicator={false}
                            />
                        )}
                        <Button.Group>
                            <Button
                                leftSection={<IconDeviceFloppy size={16} />}
                                onClick={handleSave}
                                disabled={hasErrors}
                            >
                                {/* A new document has no path yet: saving always
                                    asks for the destination, so the button says
                                    what it will do. */}
                                {isNew ? 'Save as…' : 'Save'}
                            </Button>
                            {!isNew && (
                                <Menu position="bottom-end" width={190}>
                                    <Menu.Target>
                                        <Button px={8} aria-label="More save options" disabled={hasErrors}>
                                            <IconChevronDown size={16} />
                                        </Button>
                                    </Menu.Target>
                                    <Menu.Dropdown>
                                        <Menu.Item leftSection={<IconDeviceFloppy size={16} />} onClick={handleSaveAs}>
                                            Save as…
                                        </Menu.Item>
                                    </Menu.Dropdown>
                                </Menu>
                            )}
                        </Button.Group>
                    </Group>
                </Group>
            </AppShell.Header>
            <AppShell.Navbar
                style={
                    isNarrowWindow
                        ? {
                              // As an overlay the sidebar lies on top of the
                              // content, so it cannot be transparent like it is
                              // beside it, and Mantine gives an overlay navbar
                              // the whole window width: a 240 px acrylic pane,
                              // like the one a Windows 11 app slides in, with a
                              // frosted fill that blurs what is behind.
                              width: sidebarWidth,
                              background: 'color-mix(in srgb, var(--mantine-color-default) 90%, transparent)',
                              backdropFilter: 'blur(28px) saturate(150%)',
                              boxShadow: 'var(--mantine-shadow-md)',
                          }
                        : undefined
                }
            >
                <style>{winuiNavItemCss}</style>
                <AppShell.Section grow component={ScrollArea} px={12} py="sm">
                    {navItems.slice(0, 2).map(renderStaticNavItem)}
                    <NavLink
                        label="Games"
                        leftSection={<IconDeviceGamepad2 size={18} stroke={1.6} />}
                        // Only the game itself is the selected item: the group
                        // stays a plain, expanded container so the pane never
                        // shows two selections at once.
                        opened={gamesOpened}
                        onClick={gamesHandlers.toggle}
                        className={winuiNavItemClass}
                        styles={{
                            root: {
                                borderRadius: 4,
                                paddingBlock: 8,
                                paddingInline: 8,
                            },
                        }}
                    >
                        {AVAILABLE_GAMES.map((game) => (
                            <NavLink
                                key={game.id}
                                label={game.name}
                                active={activeSection === `game:${game.id}`}
                                onClick={() => selectSection(`game:${game.id}`)}
                                className={winuiNavItemClass}
                                styles={{
                                    root: {
                                        borderRadius: 4,
                                        paddingBlock: 6,
                                        paddingInline: 8,
                                    },
                                }}
                            />
                        ))}
                    </NavLink>
                    {navItems.slice(2).map(renderStaticNavItem)}
                </AppShell.Section>
                <AppShell.Section px={12} py="xs">
                    <Group gap={8} wrap="nowrap">
                        <Box w={8} h={8} bg="teal" style={{ borderRadius: 999, flexShrink: 0 }} />
                        <Text size="xs" c="dimmed" truncate style={{ flex: 1 }}>
                            {fileName || 'New configuration'}
                        </Text>
                        <Tooltip label="Open another configuration file" withArrow openDelay={400}>
                            <ActionIcon
                                variant="subtle"
                                size="sm"
                                onClick={onBack}
                                aria-label="Open another configuration file"
                            >
                                <IconFolderOpen size={15} stroke={1.6} />
                            </ActionIcon>
                        </Tooltip>
                    </Group>
                </AppShell.Section>
            </AppShell.Navbar>
            <AppShell.Main // Validation is requested when a control in the content gives
                // up focus (see requestValidation).
                onBlur={requestValidation} // The window itself does not scroll: the content is the scroll
                // area, so it stays clipped below the header instead of sliding
                // behind it (the header is transparent and lets the window
                // backdrop through). The header offset is a padding of the shell
                // (see .app-shell in style.css) and is dropped from the padding
                // here, so the content is not offset twice.
                style={
                    {
                        paddingTop: 'var(--app-shell-padding, 1.5rem)',
                        height: 'calc(100dvh - var(--app-shell-header-offset, 4rem))',
                        minHeight: 0,
                        overflowX: 'hidden',
                        overflowY: 'auto',
                    } as React.CSSProperties
                }
            >
                {/* The window can be far wider than a setting row reads well:
                    the content keeps a desktop-app column, so a card never
                    grows into a full-width strip with its control at the far
                    end. */}
                <Stack gap="md" maw={1080}>
                    {errorEntries.length > 0 && (
                        <InfoBar severity="error">
                            <Text size="sm" fw={600} mb={6}>
                                {errorEntries.length} validation {errorEntries.length === 1 ? 'error' : 'errors'}
                            </Text>
                            <List size="sm" spacing={4}>
                                {errorEntries.map(([field, message]) => (
                                    <List.Item key={`${field}-${message}`}>
                                        <UnstyledButton
                                            onClick={() => selectSection(sectionForField(field))}
                                            style={{ textAlign: 'left' }}
                                        >
                                            <Text size="sm">
                                                <Text span fw={600}>
                                                    {fieldLabel(field)}
                                                </Text>
                                                : {message}
                                            </Text>
                                        </UnstyledButton>
                                    </List.Item>
                                ))}
                            </List>
                        </InfoBar>
                    )}
                    {/* SECTION 1: GENERAL */}
                    {activeSection === 'general' && (
                        <Stack gap="sm">
                            <SwitchSettingCard
                                icon={<IconLogs size={20} stroke={1.6} />}
                                title="Log information"
                                description="Enables logging to terminal and saves data to a file for debugging."
                                ariaLabel="Log information"
                                {...switchProps('Log')}
                            />
                            <SwitchSettingCard
                                icon={<IconId size={20} stroke={1.6} />}
                                title="Generate platform user ID"
                                description="Generate a unique ID. Only if multiple users share a launcher. Incompatible with active authentication."
                                error={allErrors.GeneratePlatformUserId}
                                ariaLabel="Generate platform user ID"
                                {...switchProps('GeneratePlatformUserId')}
                            />
                            <SettingCard
                                icon={<IconShield size={20} stroke={1.6} />}
                                title="Authentication method"
                                description="Define how users are authenticated: required, cached, adaptive, or disabled."
                                value={authLabel}
                                error={allErrors.Authentication}
                                expanded={authExpanded}
                                onToggle={authExpandedHandlers.toggle}
                                details={
                                    <Radio.Group {...commitProps(form.getInputProps('Authentication'))}>
                                        <Stack gap="xs">
                                            {AUTH_OPTIONS.map((option) => (
                                                <Radio
                                                    key={option.value}
                                                    value={option.value}
                                                    label={option.label}
                                                    description={option.description}
                                                />
                                            ))}
                                        </Stack>
                                    </Radio.Group>
                                }
                            />
                        </Stack>
                    )}
                    {/* SECTION 2: INTERNET */}
                    {activeSection === 'internet' && (
                        <Stack gap="sm">
                            <SwitchSettingCard
                                icon={<IconWorld size={20} stroke={1.6} />}
                                title="Internet access"
                                description="Allow the server to use the Internet: connectivity check, authentication and public IP detection."
                                error={allErrors['Internet.Enabled']}
                                ariaLabel="Internet access"
                                {...switchProps('Internet.Enabled')}
                            />
                            <AutoManualCard
                                icon={<IconNetwork size={20} stroke={1.6} />}
                                title="Public IP address"
                                description="IPv4 address other networks use to reach this server. Automatic asks api.ipify.org for it."
                                error={allErrors['Internet.IP']}
                                value={config.Internet.IP}
                                onChange={(value) => form.setFieldValue('Internet.IP', value)}
                                onCommit={requestValidation}
                                disabled={!config.Internet.Enabled}
                            >
                                {(manual) => <IPv4Input {...manual} />}
                            </AutoManualCard>
                        </Stack>
                    )}
                    {/* SECTION 3: ONE GAME AT A TIME */}
                    {activeGame && (
                        <GameSection
                            key={activeGame.id}
                            game={activeGame}
                            config={config}
                            errors={allErrors}
                            warnings={allWarnings}
                            embeddedDefaults={embeddedDefaults?.[activeGame.id]}
                            disabled={!config.Games.Enabled.includes(activeGame.id)}
                            onHostChange={handleGameHostChange}
                            onAddHost={handleAddGameHost}
                            onRemoveHost={handleRemoveGameHost}
                            onChangeBattleServers={(gameId, servers) =>
                                form.setFieldValue(`Games.${gameId}.BattleServers`, servers)
                            }
                            onChangeEmbeddedBattleServer={(gameId, embedded) =>
                                form.setFieldValue(`Games.${gameId}.EmbeddedBattleServer`, embedded)
                            }
                            onCommit={requestValidation}
                        />
                    )}
                    {/* SECTION 4: ANNOUNCEMENT */}
                    {activeSection === 'announcement' && (
                        <Stack gap="sm">
                            <SettingCard
                                icon={<IconNetwork size={20} stroke={1.6} />}
                                title="Announcement port"
                                description="UDP port for announcing the server to launchers (default: 31978)."
                                error={allErrors['Announcement.Port']}
                                warning={allWarnings['Announcement.Port']}
                                disabled={!config.Announcement.Enabled}
                                control={
                                    <NumberInput
                                        min={1}
                                        max={65535}
                                        clampBehavior="blur"
                                        value={config.Announcement.Port}
                                        onChange={handlePortChange} // The port only means something while the server
                                        // announces itself on the LAN.
                                        disabled={!config.Announcement.Enabled}
                                        aria-label="Announcement port"
                                        w={140}
                                    />
                                }
                            />
                            <SectionHeading spaced>Multicast</SectionHeading>
                            <SwitchSettingCard
                                icon={<IconBroadcast size={20} stroke={1.6} />}
                                title="Multicast announcements"
                                description="Respond to discovery queries at the multicast address."
                                error={allErrors['Announcement.MulticastGroup']}
                                ariaLabel="Multicast"
                                {...switchProps('Announcement.Multicast')} // Multicast announcements are one way of announcing:
                                // without the plain announcement there is nothing to
                                // send over multicast.
                                disabled={!config.Announcement.Enabled}
                                expanded={multicastExpanded}
                                onToggle={multicastExpandedHandlers.toggle}
                                details={
                                    <SettingCard
                                        variant="inline"
                                        title="Multicast group"
                                        description="IPv4 multicast group for responding to announcements (default: 239.31.97.8; class D range 224.0.0.0 – 239.255.255.255)."
                                        control={
                                            <IPv4Input
                                                onChange={(value) =>
                                                    form.setFieldValue('Announcement.MulticastGroup', value)
                                                }
                                                value={config.Announcement.MulticastGroup} // The group is only used while multicast is on
                                                // (and multicast needs the announcement).
                                                disabled={!config.Announcement.Enabled || !config.Announcement.Multicast}
                                            />
                                        }
                                    />
                                }
                            />
                        </Stack>
                    )}
                    {/* SECTION 5: CUSTOM FILES */}
                    {activeSection === 'customFiles' && (
                        <Stack gap="sm">
                            <AutoManualCard
                                icon={<IconFolder size={20} stroke={1.6} />}
                                title="Storage folder"
                                description="Folder where the files are stored. Automatic uses the system temporary folder."
                                error={allErrors['CustomFiles.Folder']}
                                value={config.CustomFiles.Folder}
                                onChange={(value) => form.setFieldValue('CustomFiles.Folder', value)}
                                onCommit={requestValidation}
                                disabled={!config.CustomFiles.Enabled}
                            >
                                {(manual) => <FolderPickerInput {...manual} onSelect={requestValidation} />}
                            </AutoManualCard>
                            <SettingCard
                                icon={<IconFile size={20} stroke={1.6} />}
                                title="Maximum file size"
                                description="Largest single file accepted, in megabytes or gigabytes (default: 32MB)."
                                error={allErrors['CustomFiles.MaxFileSize']}
                                disabled={!config.CustomFiles.Enabled}
                                control={
                                    <UnitInput
                                        value={config.CustomFiles.MaxFileSize}
                                        onChange={(value) => form.setFieldValue('CustomFiles.MaxFileSize', value)}
                                        units={SIZE_UNITS}
                                        ariaLabel="Maximum file size"
                                        onCommit={requestValidation}
                                        disabled={!config.CustomFiles.Enabled}
                                    />
                                }
                            />
                            <SettingCard
                                icon={<IconDatabase size={20} stroke={1.6} />}
                                title="Total size limit"
                                description="Maximum accumulated size per game. When exceeded, the oldest files are evicted first (default: 256MB)."
                                error={allErrors['CustomFiles.MaxTotalSize']}
                                disabled={!config.CustomFiles.Enabled}
                                control={
                                    <UnitInput
                                        value={config.CustomFiles.MaxTotalSize}
                                        onChange={(value) => form.setFieldValue('CustomFiles.MaxTotalSize', value)}
                                        units={SIZE_UNITS}
                                        ariaLabel="Total size limit"
                                        onCommit={requestValidation}
                                        disabled={!config.CustomFiles.Enabled}
                                    />
                                }
                            />
                            <SettingCard
                                icon={<IconClock size={20} stroke={1.6} />}
                                title="File expiry"
                                description="How long a file is kept since its upload, in hours or days (default: 12h)."
                                error={allErrors['CustomFiles.Expiry']}
                                disabled={!config.CustomFiles.Enabled}
                                control={
                                    <UnitInput
                                        value={config.CustomFiles.Expiry}
                                        onChange={(value) => form.setFieldValue('CustomFiles.Expiry', value)}
                                        units={EXPIRY_UNITS}
                                        ariaLabel="File expiry"
                                        onCommit={requestValidation}
                                        disabled={!config.CustomFiles.Enabled}
                                    />
                                }
                            />
                        </Stack>
                    )}
                    {/* SECTION 6: REPLAYS */}
                    {activeSection === 'replays' && (
                        <Stack gap="sm">
                            <AutoManualCard
                                icon={<IconFolder size={20} stroke={1.6} />}
                                title="Storage folder"
                                description="Folder where the replays are stored. Automatic uses the user data folder of the server (<resources>/userData/<game>/replays)."
                                error={allErrors['BattleServerReplays.Folder']}
                                value={config.BattleServerReplays.Folder}
                                onChange={(value) => form.setFieldValue('BattleServerReplays.Folder', value)}
                                onCommit={requestValidation}
                                disabled={!config.BattleServerReplays.Enabled}
                            >
                                {(manual) => <FolderPickerInput {...manual} onSelect={requestValidation} />}
                            </AutoManualCard>
                            <SettingCard
                                icon={<IconFile size={20} stroke={1.6} />}
                                title="Maximum replay size"
                                description="Largest single .replay file kept, in megabytes or gigabytes (default: 32MB)."
                                error={allErrors['BattleServerReplays.MaxFileSize']}
                                disabled={!config.BattleServerReplays.Enabled}
                                control={
                                    <UnitInput
                                        value={config.BattleServerReplays.MaxFileSize}
                                        onChange={(value) => form.setFieldValue('BattleServerReplays.MaxFileSize', value)}
                                        units={SIZE_UNITS}
                                        ariaLabel="Maximum replay size"
                                        onCommit={requestValidation}
                                        disabled={!config.BattleServerReplays.Enabled}
                                    />
                                }
                            />
                            <SettingCard
                                icon={<IconDatabase size={20} stroke={1.6} />}
                                title="Total size limit"
                                description="Maximum accumulated size per game. When exceeded, the oldest replays are evicted first (default: 256MB)."
                                error={allErrors['BattleServerReplays.MaxTotalSize']}
                                disabled={!config.BattleServerReplays.Enabled}
                                control={
                                    <UnitInput
                                        value={config.BattleServerReplays.MaxTotalSize}
                                        onChange={(value) => form.setFieldValue('BattleServerReplays.MaxTotalSize', value)}
                                        units={SIZE_UNITS}
                                        ariaLabel="Replay total size limit"
                                        onCommit={requestValidation}
                                        disabled={!config.BattleServerReplays.Enabled}
                                    />
                                }
                            />
                            <SettingCard
                                icon={<IconClock size={20} stroke={1.6} />}
                                title="Replay expiry"
                                description="How long a replay is kept since its last write, in hours or days (default: 12h). 0 keeps them forever."
                                error={allErrors['BattleServerReplays.Expiry']}
                                disabled={!config.BattleServerReplays.Enabled}
                                control={
                                    <UnitInput
                                        value={config.BattleServerReplays.Expiry}
                                        onChange={(value) => form.setFieldValue('BattleServerReplays.Expiry', value)}
                                        units={EXPIRY_UNITS}
                                        ariaLabel="Replay expiry"
                                        onCommit={requestValidation}
                                        disabled={!config.BattleServerReplays.Enabled}
                                    />
                                }
                            />
                        </Stack>
                    )}
                    {/* SECTION 7: SIGN */}
                    {activeSection === 'sign' && (
                        <Stack gap="sm">
                            <SwitchSettingCard
                                icon={<IconSignature size={20} stroke={1.6} />}
                                title="Sign catalogues"
                                description="Sign the catalogues with this server's key instead of the captured signatures, for every user. Only for clients that trust this server's signing key."
                                error={allErrors['Sign.Enabled']}
                                ariaLabel="Sign catalogues"
                                {...switchProps('Sign.Enabled')}
                            />
                            <SettingCard
                                icon={<IconId size={20} stroke={1.6} />}
                                title="Platform user ID exceptions"
                                // The list is the opposite of the switch, exactly
                                // as the template documents it.
                                description={
                                    config.Sign.Enabled
                                        ? 'Users listed here keep the captured signatures. One ID per line.'
                                        : 'Users listed here get this server’s signatures while everyone else keeps the captured ones. One ID per line.'
                                }
                                value={exceptionsValue}
                                error={allErrors['Sign.PlatformUserIdExceptions']}
                                expanded={exceptionsExpanded}
                                onToggle={exceptionsExpandedHandlers.toggle}
                                details={
                                    <div style={{ width: '100%', maxWidth: 560 }}>
                                        <Stack gap="xs" style={{ width: '100%' }}>
                                            {(config.Sign.PlatformUserIdExceptions ?? []).map((id, index) => (
                                                <Stack key={index} gap={4}>
                                                    <Group gap="xs" wrap="nowrap" justify="space-between">
                                                        <Input
                                                            value={id}
                                                            onChange={(e) => {
                                                                const arr = [...(config.Sign.PlatformUserIdExceptions ?? [])];
                                                                arr[index] = e.target.value;
                                                                form.setFieldValue('Sign.PlatformUserIdExceptions', arr);
                                                            }}
                                                            onBlur={requestValidation}
                                                            placeholder="Platform user ID"
                                                            style={{ flex: 1 }}
                                                        />
                                                        <ActionIcon
                                                            variant="subtle"
                                                            color="gray"
                                                            size="sm"
                                                            aria-label={`Remove exception ${index + 1}`}
                                                            disabled={(config.Sign.PlatformUserIdExceptions ?? []).length <= 1}
                                                            onClick={() => {
                                                                const arr = [...(config.Sign.PlatformUserIdExceptions ?? [])];
                                                                arr.splice(index, 1);
                                                                form.setFieldValue('Sign.PlatformUserIdExceptions', arr);
                                                                requestValidation();
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
                                                disabled={(config.Sign.PlatformUserIdExceptions ?? []).some((id) => id === '')}
                                                onClick={() => {
                                                    const arr = [...(config.Sign.PlatformUserIdExceptions ?? [])];
                                                    arr.push('');
                                                    form.setFieldValue('Sign.PlatformUserIdExceptions', arr);
                                                }}
                                            >
                                                Add exception
                                            </Button>
                                        </Stack>
                                    </div>
                                }
                            />
                        </Stack>
                    )}
                </Stack>
            </AppShell.Main>
            <Modal
                opened={confirmRequest !== null}
                onClose={() => answerConfirmation(false)}
                title="Overwrite file"
                centered
                radius="md"
                overlayProps={{ backgroundOpacity: 0.45, blur: 3 }}
                classNames={{
                    content: 'app-modal-content',
                    header: 'app-modal-header',
                    title: 'app-modal-title',
                }}
            >
                <Stack gap="md">
                    <Text size="sm">{confirmRequest?.message}</Text>
                    <Group justify="flex-end" gap="sm">
                        <Button variant="default" onClick={() => answerConfirmation(false)}>
                            Cancel
                        </Button>
                        <Button onClick={() => answerConfirmation(true)}>Overwrite</Button>
                    </Group>
                </Stack>
            </Modal>
        </AppShell>
    );
};
