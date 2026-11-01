## Starting the Battle Server

### Path

Depending on the game and version, the `BattleServer.exe` is included in the game installation:

There is an exception: AoM: RT includes a buggy implementation and requires a more modern version. The best is to use
the newer version found in AoE II: DE. The same applies for AoE IV: AE as it does not include the Battle Server
executable.

Note: AoE II: DE natively on macOS does not include it.

### Executable

#### Arguments

##### Basic

* `-region` (string): the region identifier. It must be an unique string for a given game, but it is recommended to use
  a meaningful name such as the PC name in lower case. E.g., `roger-pc`.
* `-name` (string): the label for the region. It should be an unique string for a given game, but it is recommended to
  use a meaningful name such as the PC name. Only used in AoE II: DE. E.g., `Roger-PC`.

##### Ports

* `-relaybroadcastPort` (1-65535, UDP): used for broadcasting the server presence in the LAN. Recommended to be set to
  `0`
  to not interfere with the LAN Battle Server and because the server must be manually configured too.
* `-bsPort` (1-65535, TCP): main port to connect to the Battle Server.
* `-webSocketPort` (1-65535, TCP): websocket port to connect to the Battle Server.
* `-outOfBandPort` (1-65535, TCP): out-of-band (observer) communication. Missing in AoE: DE.

Note: All ports must be free and are recommended to be above `1024` to avoid requiring admin rights. The
host firewall will need to
allow incoming connections to these ports.

##### SSL

* `-sslCert` (string): path to the SSL certificate in PEM format. It can be any certificate that all clients trust.
  Recommended to use the server certificate as residing in `<server>/resources/certificates/cert.pem` and using the
  launcher on the clients to ensure it is trusted.
* `-sslKey` (string): path to the SSL private key in PEM format. Recommended to use the server key as residing in
  `<server>/resources/certificates/key.pem` and using the launcher on the clients to ensure the certificate is trusted.

### Examples

The examples assume the server is installed in `C:\AgeServer`, the server certificate and key are already
generated and that you are located in the battle server directory using the `Powershell` interpreter.

#### AoE: DE

`& ".\BattleServer.exe" -simulationPeriod 25 -relayBroadcastPort 0 -region local -bsPort 30001 webSocketPort 30002 -sslKey C:\AgeServer\resources\certificates\key.pem -sslCert C:\AgeServer\resources\certificates\cert.pem`

#### AoE II: DE

`& ".\BattleServer.exe" -simulationPeriod 125 -relayBroadcastPort 0 -region local -name Local -bsPort 30003 webSocketPort 30004 -outOfBandPort 30005 -sslKey C:\AgeServer\resources\certificates\key.pem -sslCert C:\AgeServer\resources\certificates\cert.pem`

#### AoE III: DE

`& ".\BattleServer.exe" -simulationPeriod 50 -relayBroadcastPort 0 -region local -bsPort 30006 webSocketPort 30007 -outOfBandPort 30008 -sslKey C:\AgeServer\resources\certificates\key.pem -sslCert C:\AgeServer\resources\certificates\cert.pem`

#### AoE IV: AE

`& ".\BattleServer.exe" -simulationPeriod 50 -relayBroadcastPort 0 -region local -bsPort 30009 webSocketPort 30010 -outOfBandPort 30011 -sslKey C:\AgeServer\resources\certificates\key.pem -sslCert C:\AgeServer\resources\certificates\cert.pem`

#### AoM: RT

`& ".\BattleServer.exe" -simulationPeriod 50 -relayBroadcastPort 0 -region local -bsPort 30012 webSocketPort 30013 -outOfBandPort 30014 -sslKey C:\AgeServer\resources\certificates\key.pem -sslCert C:\AgeServer\resources\certificates\cert.pem`

## Configuring the Server

For each battle server instance you run you will need to edit `<server>/resources/config.toml` and inside `[Games]`
section, add `[[Games.<game>.BattleServers]]` where game is one of `age1`, `age2`, `age3`, `age4` or `athens`.
inside that subsection the following properties are required (with the arguments matching):

| Battle Server argument | Server configuration key |
|------------------------|--------------------------|
| `-region`              | `Region`                 |
| `-name`                | `Name`                   |
| `-bsPort`              | `BsPort`                 |
| `-outOfBandPort`       | `OutOfBandPort`          |
| `-webSocketPort`       | `WebSocketPort`          |

Additionally, the `IPv4` which should point to a user-accessible IP. E.g., `192.168.1.2`. When the Battle Server resides
in the same machine as the server, using `auto` is the best option as it will ensure the same IP the client uses to
connect to
the server is also used to connect to the Battle Server.

### Examples

The examples mirror the battle server examples. They assume the battle servers are reachable at `192.168.1.2`

#### AoE: DE

```toml
[[Games.age1.BattleServers]]
Region = 'local'
IPv4 = '192.168.1.2'
BsPort = 30001
WebSocketPort = 30002
```

#### AoE II: DE

```toml
[[Games.age2.BattleServers]]
Region = 'local'
Name = 'Local'
IPv4 = '192.168.1.2'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005
```

#### AoE III: DE

```toml
[[Games.age3.BattleServers]]
Region = 'local'
IPv4 = '192.168.1.2'
BsPort = 30006
WebSocketPort = 30007
OutOfBandPort = 30008
```

#### AoE IV: AE

```toml
[[Games.age4.BattleServers]]
Region = 'local'
IPv4 = '192.168.1.2'
BsPort = 30009
WebSocketPort = 30010
OutOfBandPort = 30011
```

#### AoM: RT

```toml
[[Games.athens.BattleServers]]
Region = 'local'
IPv4 = '192.168.1.2'
BsPort = 30012
WebSocketPort = 30013
OutOfBandPort = 30014
```

## Simplest way to use an online-like Battle Server

### Assumptions

* You will only run a single Battle Server instance per game at most.
* The server is installed in `<server>`. E.g., `C:\AgeServer`.
* The game is installed in `<game>`. E.g. `C:\Program Files (x86)\Steam\steamapps\common\AoE2DE`.

Replace the placeholders with the actual paths.

### Steps

1. Make sure the server has the certificate pair generated in `<server>/resources/certificates`, otherwise, generate it
   by running `<server>/bin/genCert`.
2. Execute the `PowerShell` interpreter (can be installed on non-Windows systems too).
3. Change directory to the game directory with `cd "<game>"`.
4. Run `ls` to check if `BattleServer.exe` exists, if it does not, run `cd BattleServer`.
5. Time to run the `BattleServer.exe`, run one or more of the following commands depending on the game you want to;
    * AoE: DE:
   ```
   & ".\BattleServer.exe" -simulationPeriod 25 -relayBroadcastPort 0 -region local -bsPort 30001 webSocketPort 30002 -sslKey "<server>/resources/certificates/key.pem" -sslCert "<server>/resources/certificates/cert.pem" 
   ```
    * AoE II: DE:
    ```
    & ".\BattleServer.exe" -simulationPeriod 125 -relayBroadcastPort 0 -region local -name Local -bsPort 30003 webSocketPort 30004 -outOfBandPort 30005 -sslKey "<server>/resources/certificates/key.pem" -sslCert "<server>/resources/certificates/cert.pem"
    ```
    * AoE III: DE:
    ```
    & ".\BattleServer.exe" -simulationPeriod 50 -relayBroadcastPort 0 -region local -bsPort 30006 webSocketPort 30007 -outOfBandPort 30008 -sslKey "<server>/resources/certificates/key.pem" -sslCert "<server>/resources/certificates/cert.pem"
    ```
    * AoE IV: AE:
    ```
    & ".\BattleServer.exe" -simulationPeriod 50 -relayBroadcastPort 0 -region local -bsPort 30009 webSocketPort 30010 -outOfBandPort 30011 -sslKey "<server>/resources/certificates/key.pem" -sslCert "<server>/resources/certificates/cert.pem"
    ```
    * AoM: RT:
    ```
    & ".\BattleServer.exe" -simulationPeriod 50 -relayBroadcastPort 0 -region local -bsPort 30012 webSocketPort 30013 -outOfBandPort 30014 -sslKey "<server>/resources/certificates/key.pem" -sslCert "<server>/resources/certificates/cert.pem"
    ```
6. Open `<server>/resources/config.toml` and add the corresponding configuration inside `[Games]` for the game you are
   running the
   Battle Server for. Replace `<server-ip>` with the actual server IP address (e.g., `192.168.1.2`):
    * AoE: DE:
   ```toml
    [[Games.age1.BattleServers]]
    Region = 'local'
    IPv4 = '<server-ip>'
    BsPort = 30001
    WebSocketPort = 30002   
   ```
    * AoE II: DE:
    ```toml
    [[Games.age2.BattleServers]]
    Region = 'local'
    Name = 'Local'
    IPv4 = '<server-ip>'
    BsPort = 30003
    WebSocketPort = 30004
    OutOfBandPort = 30005
    ```
    * AoE III: DE:
    ```toml
    [[Games.age3.BattleServers]]
    Region = 'local'
    IPv4 = '<server-ip>'
    BsPort = 30006
    WebSocketPort = 30007
    OutOfBandPort = 30008
    ```
    * AoE IV: AE:
    ```toml
    [[Games.age4.BattleServers]]
    Region = 'local'
    IPv4 = '<server-ip>'
    BsPort = 30009
    WebSocketPort = 30010
    OutOfBandPort = 30011
    ```
    * AoM: RT:
    ```toml
    [[Games.athens.BattleServers]]
    Region = 'local'
    IPv4 = '<server-ip>'
    BsPort = 30012
    WebSocketPort = 30013
    OutOfBandPort = 30014
    ```
7. Start the game as you normally would and then:
    * AoE DE: Click on "Multiplayer" then on "Create Game", you may select the "Region", "local", or leave as default.
      Players can
      join matches by clicking on "Multiplayer" and then on "Lobby Browser".
    * AoE II: DE: Click on "Multiplayer" then on "Host Game", you may select the "Server", "Local" or leave as default.
      Players can join matches by clicking on "Multiplayer" and then on "Find Custom Game" (Lobby Browser) or by being
      invited.
      Players can observe games by going to "Multiplayer" then "Find Custom Game" and finally "Spectate Games".
    * AoE III: DE: Click on "Multiplayer" (make sure the that "Online" is selected at top right) and then on "Host a
      Casual Game", you may select the "Region", "local", or leave as default. Players can join games by going to "
      Multiplayer" then "Browse Games" and, observe games by enabling here "Spectator Mode".
    * AoM: RT: Click on "Multiplayer" and then on "Host", you may select the "Region", "local", or leave as default.
      Players can join games by going to "Multiplayer" then "Browse Games" (and observe games too).

## Embedded Battle Server

Instead of running a separate `BattleServer.exe` process, you can embed the battle server directly inside the ageLAN server process. This is configured per game under `Games.<game>.EmbeddedBattleServer`.

### Configuration

The embedded battle server mirrors the external battle server command-line options:

| Battle Server argument | Server configuration key |
|------------------------|--------------------------|
| `-region`              | `Region`                 |
| `-name`                | `Name`                   |
| `-bsPort`              | `BsPort`                 |
| `-webSocketPort`       | `WebSocketPort`          |
| `-outOfBandPort`       | `OutOfBandPort`          |
| `-simulationPeriod`    | `SimulationPeriod`       |
| `-logLevel`            | `LogLevel`               |

Additionally, `IPv4` should point to a user-accessible IP (e.g., `192.168.1.2` or `auto`).

When `BsPort`, `WebSocketPort`, or `OutOfBandPort` is set to `0`, the server automatically assigns a free port.

`SimulationPeriod` defaults by game when set to `0`:
- `age1`: 25 ms
- `age3` / `athens`: 50 ms
- `age2` / `age4`: 125 ms

`LogLevel` accepts: `none`, `error`, `warning`, `notice`, `info`, `debug`.

**StartIfMissing** (default: `"true"`): Controls when the embedded battle server auto-starts if no external battle server is configured (`[[Games.<game>.BattleServers]]`) and none is detected by battle-server-manager. Accepted values:
- `"true"` (default): always start if missing
- `"false"`: never auto-start
- `"required"`: start if missing for age4/athens, otherwise false

### Example

```toml
[Games.age2.EmbeddedBattleServer]
StartIfMissing = "true"
Region = 'local-embedded'
Name = 'Embedded Local'
IPv4 = 'auto'
BsPort = 0
WebSocketPort = 0
OutOfBandPort = 0
SimulationPeriod = 0
LogLevel = 'info'
```

### Battle Server Replays

When enabled, the embedded battle server writes the recorded game data to self-describing `.replay` files. This is configured globally under `[BattleServerReplays]` and mirrors the `[CustomFiles]` settings:

| Key            | Description |
|----------------|-------------|
| `Enabled`      | Whether to enable replay recording. |
| `Folder`       | Folder where replay files are stored. Use `'auto'` for `<resources>/userData/<game>/replays`. |
| `MaxFileSize`  | Maximum size of a single replay file (e.g., `32MB`). |
| `MaxTotalSize` | Maximum accumulated size per game (e.g., `256MB`). |
| `Expiry`       | How long a replay file is kept since its last write (e.g., `12h`). Set to `0` or negative (e.g., `-1`) to keep replay files forever (no TTL-based expiration). |

```toml
[BattleServerReplays]
Enabled = true
Folder = 'auto'
MaxFileSize = '32MB'
MaxTotalSize = '256MB'
Expiry = '12h'
```

### Notes

- The embedded battle server uses the same certificate logic as the external battle server: `cert.pem`/`key.pem` for `age4`/`athens`, `selfsigned_cert.pem`/`selfsigned_key.pem` for other games.
- Relay broadcast is disabled. Lobby is disabled. Public port equals `BsPort`.
- The embedded server appears in the game's battle server list alongside any external battle servers.
- Replay files use a custom container format (`ALSBREPL` magic, version byte, JSON header, then timestamped records).
- One file is kept per session (session ID 0 is ignored). Per-file TTL and total quota with oldest-file eviction are enforced.

### StartIfMissing Behavior

By default (`StartIfMissing = "true"`), the embedded battle server is **automatically started** for a game when:
- The game is enabled in `[Games.Enabled]`
- No battle servers are configured in `[[Games.<game>.BattleServers]]`
- No battle servers are detected by the battle-server-manager (no `.toml` files in `<TEMP>/ageServer/battle-servers/<game>/`)

This means you don't need to explicitly configure `[Games.<game>.EmbeddedBattleServer]` to get a working battle server - it will start automatically with sensible defaults (region = `<game>-embedded`, name = "Embedded Battle Server", IPv4 = "auto", auto-assigned ports).

**Values**:
- `"true"` (default): always start if missing
- `"false"`: never auto-start
- `"required"`: start if missing for age4/athens, otherwise false

To disable this auto-start behavior, explicitly set `StartIfMissing = "false"` in `[Games.<game>.EmbeddedBattleServer]`, or configure an external battle server via `[[Games.<game>.BattleServers]]` or the battle-server-manager.
