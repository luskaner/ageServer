## Docker

All commands must be run from the **repository root** (`ageServer/`).

### Dockerfile

| Image                                                                | Description                                                                                                          |
|----------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------|
| [Server](Dockerfile/server/Dockerfile)                               | Main game server. Multi-stage: compiles with `golang:1.27-alpine3.24`, compresses with `upx`, runs on `alpine:3.24`. |
| [Gen-Cert](Dockerfile/genCert/Dockerfile)                            | One-shot certificate generator. Multi-stage: compiles, compresses with `upx`, runs on `scratch`.                     |
| [Battle-Server-Manager](Dockerfile/battle-server-manager/Dockerfile) | Battle server manager. Multi-stage: compiles, compresses with `upx`, runs on `alpine:3.24` with `wine` (win64).      |

---

### Compose

All compose files are located under `tools/server-docker/Compose/`. Run them with:

```sh
docker compose -f tools/server-docker/Compose/<file>.yml up
```

#### Choosing the right compose file

Two dimensions determine the file to use:

|                                                            | Bridge network                     | Host network                     |
|------------------------------------------------------------|------------------------------------|----------------------------------|
| **Server only — with announcement**                        | `server.bridge.yml`                | `server.host.yml`                |
| **Server only — no announcement**                          | `server.no.announce.bridge.yml`    | `server.no.announce.host.yml`    |
| **Full (server + BSM) — age1 fixed**                       | `full.age1.bridge.yml`             | `full.age1.host.yml`             |
| **Full (server + BSM) — age1 fixed, no announcement**      | `full.no.announce.age1.bridge.yml` | `full.no.announce.age1.host.yml` |
| **Full (server + BSM) — game selectable**                  | `full.all.bridge.yml`              | `full.all.host.yml`              |
| **Full (server + BSM) — game selectable, no announcement** | `full.no.announce.all.bridge.yml`  | `full.no.announce.all.host.yml`  |

**Bridge vs Host:**
- **Bridge**: ports are explicitly published (`443/tcp`, `31978/udp`, `27012/tcp`, `27112/tcp`, `27212/tcp`). Works on Linux, macOS, and Windows.
- **Host**: the container shares the host's network stack directly. Linux only. Required for AoM: RT and AoE IV: AE.

**Announcement:**
- Files **without** `no.announce` publish port `31978/udp` and enable the LAN announcement broadcast.
- Files **with** `no.announce` set `AGESERVER_SERVER_Announcement_Enabled=false` and do not publish that port. Use this when running behind a router that already handles discovery, or when announcement causes conflicts.

**BSM (`full.*`):**
- Adds the `battle-server-manager` service alongside the server.
- Required for AoE II: DE on macOS (native), AoM: RT, and AoE IV: AE.
- `age1` variants hardcode `GAME_ID=age1`. `all` variants require you to set `GAME_ID` yourself.

---

### Environment variables

#### Server (`server.*` and `full.*` compose files)

| Variable       | Required                         | Default   | Description                                                                                           |
|----------------|----------------------------------|-----------|-------------------------------------------------------------------------------------------------------|
| `GAME_ID`      | **Yes** (except `age1` variants) | —         | Game to run. One of: `age1`, `age2`, `age3`, `age4`, `athens`.                                        |
| `GENCERT_ARGS` | No                               | _(empty)_ | Extra args for the `genCert` binary. `--ignoreIfExisting` is always passed.                           |
| `SERVER_ARGS`  | No                               | _(empty)_ | Extra args for the `server` binary. `-e <GAME>`, `--log`, `--flatLog`, `--logRoot` are always passed. |

#### Battle-Server-Manager (`full.*` compose files only)

| Variable            | Required | Default   | Description                                                                                                  |
|---------------------|----------|-----------|--------------------------------------------------------------------------------------------------------------|
| `BATTLE_SERVER_EXE` | **Yes**  | —         | Absolute path on the host to `BattleServer.exe`. Bind-mounted into the container at `/app/BattleServer.exe`. |
| `BS_MANAGER_ARGS`   | No       | _(empty)_ | Extra args for `battle-server-manager start`. `--hideWindow`, `-e <GAME>`, `--logRoot` are always passed.    |

---

### Ports (bridge mode)

| Port    | Protocol | Service               | Description                                           |
|---------|----------|-----------------------|-------------------------------------------------------|
| `443`   | TCP      | server                | HTTPS / main game traffic                             |
| `31978` | UDP      | server                | LAN announcement (only in non `no.announce` variants) |
| `27012` | TCP      | battle-server-manager | Battle server port                                    |
| `27112` | TCP      | battle-server-manager | WebSocket port                                        |
| `27212` | TCP      | battle-server-manager | Out-of-band port (`full.all.*` only)                  |

---

### Volumes

| Volume name         | Mount point                                        | Description                                                                                                 |
|---------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------------------|
| `certs`             | `/app/server/resources/certificates`               | TLS certificates generated by `gen-cert`. Shared between `gen-cert`, `server`, and `battle-server-manager`. |
| `config_server`     | `/app/server/resources/config` (read-only)         | Server configuration.                                                                                       |
| `logs`              | `/app/logs`                                        | Log output for all services.                                                                                |
| `config_bs`         | `/tmp/ageServer/battle-servers`                 | Battle server runtime data. Shared between `server` (read-only) and `battle-server-manager`.                |
| `config_bs_manager` | `/app/battle-server-manager/resources` (read-only) | Battle-server-manager configuration.                                                                        |

---

### Examples

**Server only, bridge, age2:**
```sh
GAME_ID=age2 docker compose -f tools/server-docker/Compose/server.bridge.yml up
```

**Server only, host, age3, no announcement:**
```sh
GAME_ID=age3 docker compose -f tools/server-docker/Compose/server.no.announce.host.yml up
```

**Full stack, bridge, age1 (fixed):**
```sh
BATTLE_SERVER_EXE=/absolute/path/to/BattleServer.exe \
  docker compose -f tools/server-docker/Compose/full.age1.bridge.yml up
```

**Full stack, host, selectable game:**
```sh
GAME_ID=athens \
BATTLE_SERVER_EXE=/absolute/path/to/BattleServer.exe \
  docker compose -f tools/server-docker/Compose/full.all.host.yml up
```

**Rebuild images before starting:**
```sh
GAME_ID=age1 docker compose -f tools/server-docker/Compose/server.bridge.yml up --build
```
