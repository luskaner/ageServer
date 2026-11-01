export interface EmbeddedBattleServerConfig {
  StartIfMissing: string;
  Region: string;
  Name: string;
  IPv4: string;
  BsPort: number;
  WebSocketPort: number;
  OutOfBandPort: number;
  LogLevel: string;
}

/**
 * What the server uses for a game when the document leaves the embedded battle
 * server out. Comes from Go (`ConfigService.EmbeddedBattleServerDefaults`), so
 * the editor shows the real values instead of keeping a second copy of them.
 */
export type EmbeddedBattleServerDefaults = EmbeddedBattleServerConfig;

export interface GameHostsConfig {
  Hosts: string[];
  BattleServers: BattleServerConfig[];
  EmbeddedBattleServer: EmbeddedBattleServerConfig;
}

export interface BattleServerConfig {
  Region: string;
  Name: string;
  IPv4: string;
  BsPort: number;
  WebSocketPort: number;
  OutOfBandPort: number;
}

export interface GamesConfig {
  Enabled: string[];
  age1?: GameHostsConfig;
  age2?: GameHostsConfig;
  age3?: GameHostsConfig;
  age4?: GameHostsConfig;
  athens?: GameHostsConfig;
  [key: string]: any;
}

export interface AnnouncementConfig {
  Enabled: boolean;
  Multicast: boolean;
  Port: number;
  MulticastGroup: string;
}

export interface InternetConfig {
  Enabled: boolean;
  IP: string;
}

export interface SignConfig {
  Enabled: boolean;
  PlatformUserIdExceptions: string[];
}

export interface CustomFilesConfig {
  Enabled: boolean;
  Folder: string;
  MaxFileSize: string;
  MaxTotalSize: string;
  Expiry: string;
}

export interface BattleServerReplaysConfig {
  Enabled: boolean;
  Folder: string;
  MaxFileSize: string;
  MaxTotalSize: string;
  Expiry: string;
}

export interface AppConfig {
  Log: boolean;
  GeneratePlatformUserId: boolean;
  Sign: SignConfig;
  Authentication: 'required' | 'cached' | 'adaptive' | 'disabled';
  Internet: InternetConfig;
  Games: GamesConfig;
  Announcement: AnnouncementConfig;
  CustomFiles: CustomFilesConfig;
  BattleServerReplays: BattleServerReplaysConfig;
}

export interface ValidationErrors {
  [key: string]: string | undefined;
}

export const AVAILABLE_GAMES = [
  { id: 'age1', name: 'Age of Empires: DE', code: 'AoE: DE' },
  { id: 'age2', name: 'Age of Empires II: DE', code: 'AoE II: DE' },
  { id: 'age3', name: 'Age of Empires III: DE', code: 'AoE III: DE' },
  { id: 'age4', name: 'Age of Empires IV: AE', code: 'AoE IV: AE' },
  { id: 'athens', name: 'Age of Mythology: Retold', code: 'AoM: RT' },
];

const GAME_NAMES = new Map(AVAILABLE_GAMES.map((game) => [game.id, game.name]));
const GAME_ID_PATTERN = new RegExp(`\\b(?:${AVAILABLE_GAMES.map((game) => game.id).join('|')})\\b`, 'g');

/**
 * Label of a validation field as the form shows it: the card the user sees,
 * never the configuration key or the list position behind it
 * (`Announcement.Port`, `Games.age2.Hosts.0`). Go keeps those keys: they are
 * what the editor uses to map every answer back to its control.
 */
export function fieldLabel(field: string): string {
  if (field === 'Authentication') return 'Authentication method';
  if (field === 'GeneratePlatformUserId') return 'Generate platform user ID';
  if (field === 'Sign.Enabled') return 'Re-sign catalogues';
  if (field === 'Sign.PlatformUserIdExceptions') return 'Platform user ID exceptions';
  if (field === 'Internet.Enabled') return 'Internet access';
  if (field === 'Internet.IP') return 'Public IP address';
  if (field === 'Announcement.Port') return 'Announcement port';
  if (field === 'Announcement.MulticastGroup') return 'Multicast group';
  if (field === 'CustomFiles.Enabled') return 'Custom files';
  if (field === 'CustomFiles.Folder') return 'Custom files folder';
  if (field === 'CustomFiles.MaxFileSize') return 'Maximum file size';
  if (field === 'CustomFiles.MaxTotalSize') return 'Total size limit';
  if (field === 'CustomFiles.Expiry') return 'File expiry';
  if (field === 'BattleServerReplays.Enabled') return 'Replay recording';
  if (field === 'BattleServerReplays.Folder') return 'Replay folder';
  if (field === 'BattleServerReplays.MaxFileSize') return 'Maximum replay size';
  if (field === 'BattleServerReplays.MaxTotalSize') return 'Replay total size limit';
  if (field === 'BattleServerReplays.Expiry') return 'Replay expiry';
  if (field.startsWith('Games.Enabled')) return 'Enabled games';
  if (field.startsWith('Sign.PlatformUserIdExceptions')) return 'Platform user ID exceptions';

  const hosts = /^Games\.([^.]+)\.Hosts(?:\.\d+)?$/.exec(field);
  if (hosts) return `Hosts of ${GAME_NAMES.get(hosts[1]) ?? hosts[1]}`;

  const battleServers = /^Games\.([^.]+)\.BattleServers$/.exec(field);
  if (battleServers) return `Battle servers of ${GAME_NAMES.get(battleServers[1]) ?? battleServers[1]}`;

  const battleServerList = /^Games\.([^.]+)\.BattleServers\.(\d+)$/.exec(field);
  if (battleServerList) {
    const gameName = GAME_NAMES.get(battleServerList[1]) ?? battleServerList[1];
    return `Battle server ${parseInt(battleServerList[2], 10) + 1} of ${gameName}`;
  }

  const battleServer = /^Games\.([^.]+)\.BattleServers\.(\d+)\.(.+)$/.exec(field);
  if (battleServer) {
    const gameName = GAME_NAMES.get(battleServer[1]) ?? battleServer[1];
    const idx = parseInt(battleServer[2], 10) + 1;
    const fieldName = battleServer[3];
    const labelMap: Record<string, string> = {
      Region: 'Region',
      Name: 'Name',
      IPv4: 'IPv4 address',
      BsPort: 'Battle server port',
      WebSocketPort: 'WebSocket port',
      OutOfBandPort: 'Out-of-band port',
    };
    return `Battle server ${idx} ${labelMap[fieldName] ?? fieldName} of ${gameName}`;
  }

  // Embedded battle server fields
  if (isEmbeddedBattleServerField(field)) {
    return embeddedBattleServerFieldLabel(field);
  }

  // A rule pointing at something this map does not know yet: the name of its
  // last step, spread out into words, so nothing technical reaches the user.
  const words = field.split('.').filter((segment) => !/^\d+$/.test(segment));
  return (words[words.length - 1] ?? field).replace(/([a-z0-9])([A-Z])/g, '$1 $2');
}

const embeddedBattleServerFields = new Set([
  'StartIfMissing',
  'Region',
  'Name',
  'IPv4',
  'BsPort',
  'WebSocketPort',
  'OutOfBandPort',
  'LogLevel',
]);

export function isEmbeddedBattleServerField(field: string): boolean {
  const match = /^Games\.([^.]+)\.EmbeddedBattleServer\.(.+)$/.exec(field);
  if (!match) return false;
  return embeddedBattleServerFields.has(match[2]);
}

export function embeddedBattleServerFieldLabel(field: string): string {
  const match = /^Games\.([^.]+)\.EmbeddedBattleServer\.(.+)$/.exec(field);
  if (!match) return field;
  const gameName = GAME_NAMES.get(match[1]) ?? match[1];
  const fieldName = match[2];
  const labelMap: Record<string, string> = {
    StartIfMissing: 'Start if missing',
    Region: 'Region',
    Name: 'Name',
    IPv4: 'IPv4 address',
    BsPort: 'Battle server port',
    WebSocketPort: 'WebSocket port',
    OutOfBandPort: 'Out-of-band port',
    LogLevel: 'Log level',
  };
  return `${labelMap[fieldName] ?? fieldName} of embedded battle server for ${gameName}`;
}

/**
 * A Go service error as the user should read it. Wails prefixes a rejected
 * binding call with "Binding call failed: Bound method returned an error:",
 * which says nothing the user can act on.
 */
export function errorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error);
  return raw.replace(/^Binding call failed:\s*(?:Bound method returned an error:\s*)?/i, '');
}

/**
 * A Go validation message as it reads to the user: a game is named the way the
 * form names it, instead of by its identifier in the configuration file.
 */
export function humanMessage(message: string): string {
  return message.replace(GAME_ID_PATTERN, (id) => GAME_NAMES.get(id) ?? id);
}
