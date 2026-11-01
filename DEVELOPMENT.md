### System requirements

- OS requirements correspond to the server/launcher ones. Cross-compilation works on all systems out-the-box.
- Go 1.27:
    * Officially from https://go.dev/dl/ if not running Windows or is 10 and higher (or equivalent).
    * Unofficially from either [thongtech/go-legacy-win7](https://github.com/thongtech/go-legacy-win7) or [XTLS/go-win7](https://github.com/XTLS/go-win7) if running Windows
      7-8.X (or equivalent). Regardless if you install the official version, you need to install this one too for
      release.
- [Git](https://git-scm.com/downloads), with the latest supported for Windows 7-8 being v2.46.2.
- [Task](https://taskfile.dev/installation/).
- [GoReleaser](https://goreleaser.com/).
- [Node.js](https://nodejs.org/) 22+ (for Wails GUI frontend build).

### Setup

Copy `.env.example` to `.env` and set:

* ```GPG_FINGERPRINT``` to the fingerprint of the key. Required only for `task release`.
* ```GOROOT_LEGACY``` to the legacy go installation path. Required for `task build` and
  `task release`.

### Tools

Helper tools live in `tools/scripts/cmd` (one directory per tool) and are registered as `tool`
directives in `tools/scripts/go.mod`. There is no manual build step: `go` compiles and caches them
automatically.

Run any of them from the repository root:

```
go tool <name> [args]
```

Available tools: `copyBattleServerManagerResources`, `copyLauncherResources`, `copyServerResources`,
`createGameMockCA`, `createGameStructure`, `createServerResourcesFolder`, `generateGoreleaserConfig`.

List them with `go list tool`. They are normally invoked through `task`
(e.g. `task debug:prepare-server` or `task tools:run PROGRAM=createGameMockCA ARGS=age2`).

### GUI Configuration Application (Wails v3)

The `gui/server/config` directory contains a Wails v3 (Go + React) application for editing `config.toml`.

**Development:**
```bash
cd gui/server/config
# Set NODE_ENV=development first (required for dev dependencies)
export NODE_ENV=development
wails3 dev -config ./build/config.yml -port 9245
```

**Build (production):**
```bash
cd gui/server/config
task build
# Or manually:
# 1. npm run build in frontend/
# 2. go build with -tags production
```

**Requirements:**
- Node.js 22+
- Go 1.27+ with CGO_ENABLED=0
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

The frontend is in `gui/server/config/frontend/` (React + TypeScript + Vite).
The Go entry point is `gui/server/config/main.go` which embeds `frontend/dist` via `//go:embed all:frontend/dist`.

### Debug

It is recommended to use an IDE such as [GoLand](https://www.jetbrains.com/go/) (free for academia)
or [Visual Studio Code](https://code.visualstudio.com/) (free) with
the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go).

Depending on the module you want to debug, you will need to run the corresponding task **before**:

- server: ```task debug:prepare-server```
    - genCert: ```task debug:prepare-server-genCert```
- launcher: ```task debug:prepare-launcher```
    - config: ```task debug:build-config```
    - config-admin-agent: ```task debug:build-config-admin-agent```
    - agent: ```task debug:build-launcher-agent```
- battle-server-manager: ```task debug:prepare-battle-server-manager```

### Build

1. Make sure you have CGO disabled with ```go env -w CGO_ENABLED=0```
2. Run ```task build``` to build for the current platform only.

To build binaries for all supported platforms, run
```task build-all```. On Windows, this includes Windows 7-compatible binaries (requires GOROOT_LEGACY)
and Windows 10+ binaries (amd64, arm64).

**What `task build` does:**
1. Generates GoReleaser config for current platform (`build-goreleaser-cfg`)
2. Builds the GUI config app (`build-gui-config`)
3. Runs GoReleaser for single target (`goreleaser build --clean --snapshot --single-target`)

### Release

1. Install [gpg2](https://docs.releng.linuxfoundation.org/en/latest/gpg.html) if needed.
2. Create a new sign-only GPG key pair (*RSA 4096-bit*) with a passphrase.
3. Make sure you have CGO disabled with ```go env -w CGO_ENABLED=0```
4. Create a signed tag: `git tag -s vX.Y.Z -m "vX.Y.Z"`
5. Push the tag: `git push origin vX.Y.Z`
6. Finally run ```task release``` (or let GitHub Actions handle it on tag push)

*Note: You will also need a local tag in semantic form like vX.Y.Z*

**What `task release` does:**
1. Generates full GoReleaser config (`build-goreleaser-cfg-all`)
2. Runs GoReleaser release (`goreleaser release --clean --snapshot --fail-fast`)

### GitHub Actions CI/CD

The `.github/workflows/releaser.yml` workflow triggers on tag push (`v*.*.*`):
1. Sets up Go 1.27.3
2. Sets up Node.js 22 and builds the Wails frontend (`npm ci && npm run build` in `gui/server/config/frontend`)
3. Downloads the legacy Go toolchain for Windows 7 compatibility
4. Generates GoReleaser config with `generateGoreleaserConfig`
5. Runs GoReleaser v2.19.0 with `release --fail-fast`

The GoReleaser config is generated by `tools/scripts/cmd/generateGoreleaserConfig` which creates `.goreleaser.yaml` with:
- Multi-platform builds (Linux x86_64/ARM64/ARM/386, Windows x86_64/ARM64/386, macOS x86_64/ARM64)
- Windows 7 (win7) and Windows 10+ (win10/win11) variants
- Archives: server, battle-server-manager, launcher, full (merged)
- GUI config binary placed at root of server archive and at `server/` in full archive
- config-helper binaries included in Windows archives

### Project Structure (key modules)

```
server/                    # Main server binary
server-genCert/            # Certificate generation tool
launcher/                  # Game launcher
launcher-agent/            # Launcher background agent
launcher-config/           # Launcher configuration CLI
launcher-config-admin/     # Admin config tool
launcher-config-admin-agent/ # Admin agent
battle-server-manager/     # Battle server manager
battle-server-broadcast/   # Broadcast service
config-helper/             # Cross-platform config helper
gui/server/config/         # Wails GUI config editor (Go + React)
tools/scripts/             # Build/release automation tools
common/                    # Shared Go packages
```