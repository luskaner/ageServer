# ageServer Configurator

The GUI loads and edits the official `config.toml` contract. It can also create a new configuration from the server template without maintaining a second template copy.

The load screen accepts files dragged from the operating system. Wails supplies the native path to Go, where the file is parsed and validated before it reaches the editor. On startup the app tries the default configuration paths; when none exists it stays on the load screen and does not open the picker automatically.

Each game has its own page in the sidebar: **Hosts**, then a **Battle servers** section with a **Standalone** card (the servers this document announces) and an **Embedded** card (the battle server that runs inside the ageLAN server process). The page notes that the server also uses the battle servers the battle-server-manager finds on the PC. **Replays** configures the recording of the match data the embedded battle server receives.

Pages whose feature has one master switch (Internet, Announcement, Custom files, Replays) carry it next to the page title, as a game page does with the game's switch; the page body only lists what that switch turns on.

The interface follows Fluent 2 as Windows 11 draws it: settings cards with a hairline stroke over the window's Mica backdrop, InfoBars for messages, a 4 px corner radius and monochrome icons. `AGENTS.md` lists the components that own each of those.

The server is the source of truth for every key and default. See [AGENTS.md](./AGENTS.md) for the full contract, the battle server table shape (`[[Games.<game>.BattleServers]]`), the validation rules and the live-testing workflow.

## Development

Set `NODE_ENV=development` before starting: with `NODE_ENV=production`, the `npm install` run by Wails omits devDependencies and the dev build cannot find `tsc`.

```powershell
$env:NODE_ENV='development'
wails3 dev -config ./build/config.yml -port 9245
```

To test the running application through the built-in MCP server:

```powershell
$env:NODE_ENV='development'
$env:WAILS_MCP='1'
$env:WAILS_MCP_PORT='9099'
wails3 dev -config ./build/config.yml -port 9245
```

The endpoint is `http://127.0.0.1:9099/mcp`. The app exposes DOM inspection, mouse/keyboard input and JavaScript evaluation tools; AGENTS.md describes an end-to-end check.

Typecheck and run the Go tests with:

```text
npx tsc --noEmit
go test ./...
```

`npm run build` remains the production frontend build. Avoid it while a `wails3 dev` session is open: writing `frontend/dist` can trigger a full reload.

## Release

The configurator is released as the portable `config` executable that ships with the Windows server archives.
It is built by the goreleaser configuration generated in `tools/scripts`, which builds the frontend and generates the
Windows resources (icon, manifest and version information) before compiling the application. The same happens in the
release workflow, so the archives carry the same binary as a local `task build`.

It is compiled against [luskaner/wails](https://github.com/luskaner/wails), a fork of Wails v3 that carries the
fixes for the two panics that made Wails unusable on 32-bit Windows. The fork is wired in through the `replace`
in the workspace (`go.work`) and it is republished, with the patch applied, on every new upstream v3 release.

## Creating a configuration

Use **Create New Configuration** to open `server/resources/config/config.toml` as an unsaved draft. Select the destination when saving. The configuration is validated in Go before it is created; an existing destination requires an explicit overwrite confirmation.

The template is searched in the installed `resources/config` directory and in the repository layout used during development. The official server template remains the source of truth.
