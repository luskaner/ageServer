# Configurator — agent notes

This Go + React (Wails v3) application edits the server's `config.toml` in place.
The server is the source of truth: the editor must read and write the same keys,
with the same defaults, that `server/internal/config.go` and
`server/internal/cmd/root.go` use.

## Configuration contract

The model in `models.go` mirrors `server/internal/config.go`. Do not invent keys.

| TOML key | Notes |
| --- | --- |
| `Log` | Root boolean. |
| `GeneratePlatformUserId` | Root boolean. Only valid with `Authentication = 'disabled'`. |
| `Authentication` | **Root key**, not under `[Internet]`. Values: `required`, `cached`, `adaptive`, `disabled`. Older documents that still hold `[Internet] Authentication` are kept in sync on save, but the root key is the one the server reads. |
| `[Sign] Enabled` | Re-sign catalogues with this server's key. |
| `[Sign] PlatformUserIdExceptions` | List of platform user ids excluded from signing. |
| `[Internet] Enabled` | Allow DNS check, authentication and public IP lookup. |
| `[Internet] IP` | `'auto'` or an IPv4 address. |
| `[Games] Enabled` | `age1`, `age2`, `age3`, `age4`, `athens`. |
| `[Games.<game>] Hosts` | IPv4 addresses the server binds for that game; must be unique among enabled games. |
| `[[Games.<game>.BattleServers]]` | One table per battle server. See below. |
| `[Announcement]` | `Enabled`, `Multicast`, `Port`, `MulticastGroup`. |
| `[CustomFiles]` | `Enabled`, `Folder`, `MaxFileSize`, `MaxTotalSize`, `Expiry`. |
| `[BattleServerReplays]` | Recording of the data the embedded battle server receives: `Enabled`, `Folder`, `MaxFileSize`, `MaxTotalSize`, `Expiry`. `Expiry` `<= 0` keeps replays forever. |

### Battle servers

The documented shape is an array of tables, not an inline array:

```toml
[[Games.age2.BattleServers]]
Region = 'local'
Name = 'Local'          # only AoE II: DE uses it; written only when non-empty
IPv4 = 'auto'           # or a user-accessible IPv4 address
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005   # not written for age1
```

`configurationDocument` rewrites every game through `rewriteBattleServers`:

- removes an old inline `BattleServers = [...]` key and every
  `[[Games.<game>.BattleServers]]` block;
- writes the canonical tables back only when the list has entries, so a save
  never appends empty `BattleServers` and a second save is a no-op;
- keeps the rest of the document (comments, unknown keys) untouched.

Both `rewriteBattleServers` and `rewriteEmbeddedBattleServer` edit the tables
they find **key by key** (`writeTableKeys`) instead of deleting and recreating
them: a key that is already there keeps its line, its inline comment and its
position, a missing key is appended after the last one, and a key the editor
does not know is left alone. Only a block the game no longer has is removed, and
even then a comment run separated from the block by a blank line stays in the
document (`blockCutEnd`), because it closes the section that follows. A whole
table the document is missing is added next to its siblings (`[Games.age3]` with
the other games), not at the end of the file.

`TestConfigurationDocumentKeepsEveryTemplateComment` asserts that every comment
of the shipped template survives a save that changes values across the document;
the other `…KeepsComments…` tests cover an edited battle server table, an edited
embedded table, a removed block and a missing game table.

The add dialog (`frontend/src/components/BattleServerDialog.tsx`) handles a
single entry: `GameSection` appends it to the form list. A new entry starts with
the ports used in `server/BattleServers.md` for that game and is validated
locally (region required and unique, IPv4 `auto` or valid, ports in range)
before it reaches the form. The IPv4 field reuses the app's `AutoManualCard`:
automatic (`auto`) by default, manual with the four-octet `IPv4Input`.

### Game pages

Each game is its own sub-item under **Games** in the sidebar. The page title is
the game name and the header carries its enabled switch. The page holds the
**Hosts** card and then a **Battle servers** section with two cards:

- **Hosts**: the right side lists the hosts separated by commas; expanding shows
  one IPv4 input per host with add and delete buttons.
- **Standalone**: the battle servers this document announces. The right side
  lists their regions separated by commas (the region is unique within the
  game); expanding lists each server's region/name, address and ports with a
  delete button. "Add battle server" opens the single-entry dialog. Its panel
  uses the opaque Win11-style surface from `style.css` (`.app-modal-*`), never
  the translucent card fill.
- **Embedded**: the built-in battle server that runs inside the ageLAN server
  process (`[Games.<game>.EmbeddedBattleServer]`). The summary reads
  `region · auto-start mode`; expanding groups the fields under Startup,
  Address, Ports and Behaviour. It states up front whether the game already has
  a battle server of its own, because the embedded one only starts when it does
  not. `0` means "the server decides" for every port.

The section says, once, that the server also uses the battle servers the
battle-server-manager finds on the PC: neither card above describes the whole
picture, and a user must not read "None" as "no battle server at all".

A game that is not in `Games.Enabled` is read-only: its three cards are passed
`disabled`, so they keep their summary but cannot be opened, and the page says
why. Only the header switch (and the sidebar) lead to them.

In the sidebar only the game is the selected item, never the **Games** group it
lives in: the group is a container, and two highlighted rows would say the same
thing twice.

### Embedded battle server defaults

The server only defaults `StartIfMissing` (`initConfig` in
`server/internal/cmd/root.go` sets `"true"` for every game) and resolves the
rest while it starts, so `serverDefaults()` keeps everything else at its zero
value:

| Key | Document | Server |
| --- | --- | --- |
| `StartIfMissing` | empty | `"true"` |
| `Region` | empty | `"Server"` |
| `Name` | empty | `"Server"` |
| `IPv4` | empty | `"auto"` |
| `BsPort`, `WebSocketPort`, `OutOfBandPort` | `0` | a free port |
| `LogLevel` | empty | `"debug"` (the battle server only overrides its logger when the value is set) |

The simulation period is **not** part of the configuration: the embedded server
always runs with `common/battleconfig.SimulationPeriodDefaults` for the game, so
neither the document nor the editor has a key for it.

`ConfigService.EmbeddedBattleServerDefaults()` returns that right-hand column to
the frontend, which shows it as placeholders and hints: the editor must not
carry its own copy of those values. An empty `Region` is valid, and
`rewriteEmbeddedBattleServer` only writes the keys that carry a value, so an
empty region keeps meaning "Server".

`rewriteEmbeddedBattleServer` writes the table only when it *differs* from the
config defaults (an empty `IPv4` compares equal to `"auto"` and an empty
`StartIfMissing` to `"true"`): a document without an embedded section saves back
unchanged instead of gaining one block per game. A save that only touches other
settings must stay byte-identical.

### Defaults

`serverDefaults()` replicates the defaults in `server/internal/cmd/root.go`:
`Authentication = 'disabled'`, `Sign.Enabled = false`, `Internet.Enabled = true`,
`Internet.IP = 'auto'`, `Announcement` enabled with port `31978` and group
`239.31.97.8`, `CustomFiles` enabled with `auto`/`32MB`/`256MB`/`12h`, and
`Hosts = ['0.0.0.0']` for every game.

## Validation

Validation lives in Go (`validateConfiguration`) and the frontend only renders
it through `ConfigService.ValidateConfiguration`. Errors block saving; warnings
do not. It mirrors what the server would do at startup:

- authentication values and the `GeneratePlatformUserId`/Internet combinations;
- supported games, and at least one host for an enabled game;
- hosts must resolve to IPv4 and not be shared by two enabled games (disabled
  games only warn);
- enabled `age4` and `athens` need a battle server, and an embedded one that is
  allowed to start counts (`validateGameBattleServers`): the server starts the
  embedded instance before initializing the game and registers it with the rest
  (`InitializeBattleServers`). With neither, it is only a **warning**: the
  battle-server-manager may still find one on the PC at startup, which a
  configuration editor cannot see, so it must not block saving;
- battle server `Region` is required and unique within the game, `IPv4` is
  `auto` or an IPv4 address, and the ports are in range;
- the embedded battle server accepts every key empty: `StartIfMissing` must be
  `true`, `false` or `required` when set, `IPv4` `auto` or an IPv4 address, the
  ports `0..65535` and `LogLevel` one of the six levels;
- announcement port/group, and custom file sizes/duration (sizes accept
  `B/KB/MB/GB`, durations are Go durations).

## Look and feel

The interface follows Fluent 2 as Windows 11 draws it (WinUI 3), and the same
element always looks the same way:

- `SettingCard` is the only row component: layer fill, hairline stroke, a 20 px
  monochrome icon (never a tinted tile), a semibold 14 px title, a 12 px dimmed
  description, the current value in the normal text colour and the control at
  the end of the row. `SwitchSettingCard`, `AutoManualCard` and the game cards
  all build on it. `variant="inline"` draws the same row without the surface,
  for a setting that belongs to another card: a card is never nested in a card.
- A page whose feature has a single master switch keeps it in the header, next
  to the title, as a game page does with the game's switch; the page body only
  lists what that switch turns on (Announcement, Custom files, Replays). Sign
  and Internet are the exceptions: their switches stay on the page as cards of
  their own.
- A control that a master switch turns off is disabled **through its card**, not
  only through the input: `SettingCard` gets `disabled`, so the whole row dims
  the same way everywhere (the card cannot be opened either).
- The save button reads "Save" when the document has a path and "Save as…" when
  it does not, because a new document always asks for the destination.
- `InfoBar` is the only inline message: severity tint, hairline stroke and the
  severity icon. Page warnings, validation errors and the notes inside a card
  all go through it, never a bare icon-and-text group or a Mantine `Alert`.
- `SectionHeading` has two levels only: `section` (14 px, normal colour) names a
  block of cards such as "Battle servers", `group` (12 px, dimmed) names a part
  of one card such as "Ports" or the "Multicast" group on Announcement.
- Sizes come from the theme: `defaultRadius` is `sm` (4 px, WinUI
  `ControlCornerRadius`) for controls, cards use `md` (8 px) and dialogs `md`
  (`OverlayCornerRadius`); the colours live in `style.css` as `--app-card-*`,
  `--app-infobar-*` and `--app-modal-*` tokens with a light and a dark value.

`public/style.css` is a static asset: Vite does not hot-reload it, so a change
there needs a window reload (`window_control` `force_reload` does not bypass the
webview cache; load the page again with a different query string).

## Development

Run from `gui/server/config`:

```text
wails3 dev -config ./build/config.yml -port 9245
```

**Set `NODE_ENV=development` first.** With `NODE_ENV=production`, the
`npm install` run by the Wails build task omits devDependencies, `tsc`
disappears from `node_modules/.bin`, and the dev build fails.

The Vite config uses `server.watch.usePolling` because the project may live on a
network drive where native watchers do not deliver events. Without polling,
edits can leave the dev server serving a stale module.

Build/typecheck/tests:

```text
npx tsc --noEmit          # typecheck without rewriting dist
go test ./...
```

Avoid `npm run build` while a `wails3 dev` session is being exercised: writing
`frontend/dist` can trigger a full reload and discard the form state.

## Live testing with MCP

Start with the MCP build tag and a fixed port:

```powershell
$env:NODE_ENV='development'
$env:WAILS_MCP='1'
$env:WAILS_MCP_PORT='9099'
wails3 dev -config ./build/config.yml -port 9245
```

The app serves a stateless JSON-RPC MCP endpoint at `http://127.0.0.1:9099/mcp`
(tools: `app_info`, `windows_list`, `window_control`, `js_eval`, `dom_html`,
`dom_query`, `screenshot_dom`, `mouse_*`, `keyboard_*`, `call_bound_method`,
`emit_event`, `wait_for_event`).

On mount the app tries the default configuration paths (`openConfigFile`). When
none exists it stays on the load screen and does not open the native picker;
the picker only opens when the user selects a file explicitly. In an automated
session, load the file with `files-dropped` (or use the Select button).

Useful workflow for an end-to-end check:

1. `emit_event` `files-dropped` with `{ "files": ["C:\\path\\config.toml"] }`
   to load a copy through the real drag-and-drop path.
2. `js_eval` / `dom_query` to inspect the rendered state; use `mouse_click` with
   a CSS selector for elements below the fold (the tool scrolls them into view).
3. `keyboard_type` / `keyboard_press` for real text input.
4. Click Save, then read the file back and confirm the exact TOML shape.

`ConfigService` is bound as `changeme.ConfigService`; the generated `Call.ByID`
ids (e.g. `2068630133` for `OpenConfigFileAtPath`) can be used from `js_eval`
when a direct service call is needed.

## Things that are easy to get wrong

- `Authentication` is a **root** key. The server's koanf defaults use the root
  key and there is no migration for `[Internet] Authentication`.
- `BattleServers` is an **array of tables**. Adding an inline `BattleServers =`
  key to a file that already has `[[...BattleServers]]` is a duplicate-key error.
- An empty battle server list must remove the existing definitions, not write
  `BattleServers = []`.
- The editor must remount when another document is loaded (see `loadKey` in
  `App.tsx`): Mantine's `useForm` keeps its initial values for the life of the
  component.
- The unstable go-toml parser does not set `Raw` for table headers;
  `documentTables` finds header offsets by scanning whole header lines.
