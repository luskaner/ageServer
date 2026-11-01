import { useEffect, useRef, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { ConfigFile, ConfigService, Configuration } from '../bindings/changeme';
import {
    AppConfig,
    BattleServerConfig,
    EmbeddedBattleServerConfig,
    EmbeddedBattleServerDefaults,
    errorMessage,
} from './types';
import { Screen1_FileLoad } from './components/Screen1_FileLoad';
import { Screen2_ConfigEditor } from './components/Screen2_ConfigEditor';

/**
 * The embedded battle server as the engine should hold it. The document may
 * leave the table out entirely, and may leave IPv4 out: the server reads that
 * as "auto" (common/battleconfig BuildConfig), so the editor does the same
 * instead of showing the address as unset.
 */
function toEditorEmbeddedBattleServer(embedded: EmbeddedBattleServerConfig | null | undefined) {
    const value: EmbeddedBattleServerConfig = {
        StartIfMissing: 'true',
        Region: '',
        Name: '',
        IPv4: 'auto',
        BsPort: 0,
        WebSocketPort: 0,
        OutOfBandPort: 0,
        LogLevel: '',
        ...(embedded ?? {}),
    };
    if (!value.IPv4) value.IPv4 = 'auto';
    return value;
}

function toEditorConfig(config: Configuration): AppConfig {
    const game = (hosts: string[] | null, battleServers: BattleServerConfig[] | null, embeddedBattleServer: any) => ({
        Hosts: hosts ?? [],
        BattleServers: battleServers ?? [],
        EmbeddedBattleServer: toEditorEmbeddedBattleServer(embeddedBattleServer),
    });
    return {
        Log: config.Log,
        GeneratePlatformUserId: config.GeneratePlatformUserId,
        Sign: { Enabled: config.Sign.Enabled, PlatformUserIdExceptions: config.Sign.PlatformUserIdExceptions ?? [] },
        Authentication: config.Authentication as AppConfig['Authentication'],
        Internet: config.Internet,
        Games: {
            Enabled: config.Games.Enabled ?? [],
            age1: game(config.Games.age1.Hosts, config.Games.age1.BattleServers, config.Games.age1.EmbeddedBattleServer),
            age2: game(config.Games.age2.Hosts, config.Games.age2.BattleServers, config.Games.age2.EmbeddedBattleServer),
            age3: game(config.Games.age3.Hosts, config.Games.age3.BattleServers, config.Games.age3.EmbeddedBattleServer),
            age4: game(config.Games.age4.Hosts, config.Games.age4.BattleServers, config.Games.age4.EmbeddedBattleServer),
            athens: game(config.Games.athens.Hosts, config.Games.athens.BattleServers, config.Games.athens.EmbeddedBattleServer),
        },
        Announcement: config.Announcement,
        CustomFiles: {
            ...config.CustomFiles,
            MaxFileSize: toEditorSize(config.CustomFiles.MaxFileSize),
            MaxTotalSize: toEditorSize(config.CustomFiles.MaxTotalSize),
        },
        BattleServerReplays: config.BattleServerReplays ?? {
            Enabled: false,
            Folder: 'auto',
            MaxFileSize: '32MB',
            MaxTotalSize: '256MB',
            Expiry: '12h',
        },
    };
}

/**
 * The server also reads single-letter sizes (32M, 256G): the editor offers MB
 * and GB only, so those spellings are taken as their full unit.
 */
function toEditorSize(size: string): string {
    return size.replace(/^(\d+)\s*M$/i, '$1MB').replace(/^(\d+)\s*G$/i, '$1GB');
}

export function App() {
    const [loadedFile, setLoadedFile] = useState<ConfigFile | null>(null);
    const [errorMsg, setErrorMsg] = useState<string | null>(null);
    // The editor keeps its form state from the moment it mounts, so loading a
    // different document while it is open has to remount it: the key changes on
    // every load, including dropping the same file again after editing it.
    const [loadKey, setLoadKey] = useState(0);
    const hasStartedLoading = useRef(false);
    // Values the server resolves for an embedded battle server the document
    // does not describe. Go owns them (`configservice.go` mirrors
    // server/internal/cmd/root.go and common/battleconfig), so the editor never
    // carries a second copy that can drift.
    const [embeddedDefaults, setEmbeddedDefaults] = useState<
        Record<string, EmbeddedBattleServerDefaults | undefined> | undefined
    >();

    useEffect(() => {
        let cancelled = false;
        ConfigService.EmbeddedBattleServerDefaults()
            .then((defaults) => {
                if (!cancelled) setEmbeddedDefaults(defaults ?? undefined);
            })
            .catch(() => {
                // Without them the editor falls back to plain placeholders.
            });
        return () => {
            cancelled = true;
        };
    }, []);

    const showLoadedFile = (file: ConfigFile | null) => {
        if (!file) return;
        setLoadedFile(file);
        setLoadKey((key) => key + 1);
    };

    const openConfigFile = () => {
        setErrorMsg(null);
        ConfigService.OpenConfigFile()
            .then(showLoadedFile)
            .catch((error: unknown) => setErrorMsg(errorMessage(error)));
    };

    const selectConfigFile = () => {
        setErrorMsg(null);
        ConfigService.SelectConfigFile()
            .then(showLoadedFile)
            .catch((error: unknown) => setErrorMsg(errorMessage(error)));
    };

    const createConfigFile = () => {
        setErrorMsg(null);
        ConfigService.OpenTemplate()
            .then(showLoadedFile)
            .catch((error: unknown) => setErrorMsg(errorMessage(error)));
    };

    // Try the default configuration on startup. When there is none the load
    // screen stays in place: the picker only opens when the user asks for it.
    useEffect(() => {
        if (hasStartedLoading.current) return;
        hasStartedLoading.current = true;
        openConfigFile();
    }, []);

    useEffect(() => {
        return Events.On('files-dropped', (event) => {
            const data = event.data as { files?: string[] } | undefined;
            const droppedPath = data?.files?.[0];
            if (!droppedPath) {
                setErrorMsg('No file path was received from the dropped item.');
                return;
            }
            setErrorMsg(null);
            ConfigService.OpenConfigFileAtPath(droppedPath)
                .then(showLoadedFile)
                .catch((error: unknown) => setErrorMsg(errorMessage(error)));
        });
    }, []);

    return loadedFile ? (
        <Screen2_ConfigEditor
            key={loadKey}
            initialConfig={toEditorConfig(loadedFile.config!)}
            backendValidationErrors={loadedFile.validationErrors ?? []}
            embeddedDefaults={embeddedDefaults}
            fileName={loadedFile.path}
            isNew={!loadedFile.path}
            onSaved={showLoadedFile}
            onBack={() => setLoadedFile(null)}
        />
    ) : (
        <Screen1_FileLoad
            onSelectConfig={selectConfigFile}
            onCreateConfig={createConfigFile}
            errorMsg={errorMsg || undefined}
            validationErrors={[]}
        />
    );
}

export default App;
