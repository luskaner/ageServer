# Age Server
<img width="1024" height="572" alt="a10329cc-cfe5-4e59-bd93-efef22df81e0" src="https://github.com/user-attachments/assets/faec2789-dbd0-4905-800c-24ec0615c92d" />

Age Server (formerly *Age LAN Server*, originally *AoE 2 DE LAN Server*) is a web server (with its launcher) that allows you to play multiplayer
lobbies without having an
internet
connection **to the game server**  ensuring the game multiplayer functionality is still available even if the official
server
is in maintenance or is eventually shutdown.

> [!IMPORTANT]
> You will still need a custom launcher to bypass the online-only restriction that is imposed by the game to being
> connected to the internet and Xbox Live or Steam, depending on the platform and version, to fully play offline. My
> other [project](https://github.com/luskaner/ageServerLauncherCompanion) provides the files and information to
> download a Steam Emulator and play 100% offline.

**🎮 Supported games:**

* **Age of Empires: Definitive Edition**.
* **Age of Empires II: Definitive Edition**.
* **Age of Empires III: Definitive Edition**.
* **Age of Empires IV: Anniversary Edition**.
* **Age of Mythology: Retold** ([currently](https://github.com/luskaner/ageServer/issues/245) only the *Steam*
  version).

## ⚙️Features

- 🌐 Scenarios.
- 🗺️ Map transferring in-lobby.
- ↕️ Restore game.
- 📦 Data mods.
- 🗣️ Lobby chatting.
- 🎮 Crossplay Xbox & Steam.

> [!TIP]
> See more details
> in [Questions and Answers (QA)](https://github.com/luskaner/ageServer/wiki/Questions-and-Answers-(QA)).

### AoE II: DE, AoE III: DE, AoE IV: AE and AoM: RT

<details>
<summary>List of features</summary>

- 👁️ Observing (including *CapturAge* for *AoE II: DE*).
- 📩 Lobby invite.
- 🔗 Share lobby link.
- 🔍 Player Search.

</details>

### AoE II: DE and AoE III: DE

<details>
<summary>List of features</summary>

- 🧑‍🤝‍🧑 Co-Op Campaigns.
- 🔄 Rematch.

</details>

### AoE II: DE

<details>
<summary>List of features</summary>

- 💬 Channels.
- 🗣️ Whispering.

</details>

### AoE III: DE, AoE IV: AE

<details>
<summary>List of features</summary>

- 👥 Groups

</details>

### AoE IV: AE

<details>
<summary>List of features</summary>
  
- 🏛️Crucible Co-Op

</details>

### AoM: RT

<details>
<summary>List of features</summary>

- 🗣️ Whispering.
- 🙏 Arena of the Gods.

</details>

### General limitations

<details>
<summary>List of limitations</summary>

- ⚠️ Joining a game lobby from a link only works if the game is already running.
- ⚠️ Subscribing to online mods only works if using the official launcher.
- ❌ No Xbox and Steam **friend integration**.

</details>

#### AoE III: DE, AoE IV: AE

<details>
<summary>List of limitations</summary>

- ⚠️ **Friend list** will instead show all online users as if they were friends.

</details>

#### AoM: RT

<details>
<summary>List of limitations</summary>

- ⚠️ **Friend list** will instead show all online users as if they were friends.
- ℹ️ **Arena of Gods**:
    * No rewards are gained, **all blessings and rarities are unlocked by default**, including those not available in
      the official server.
    * Favor stash is infinite.
    * Story mode has all missions unlocked by default.
    * Challenge mode lifes are infinite, you have access to all legends, and you are max level (99) by default.

</details>

## Minimum system requirements

### Server

#### Stable

- **Windows**: 7 or equivalent (10 or higher recommended).
- **Linux**: kernel 3.2 (5.10 or higher recommended, see [here](https://go.dev/wiki/Linux) for more details).
- **macOS**: Ventura v13 (Sequoia v15 or higher recommended).

Admin rights or firewall permission to listen on port 443 (https) will likely be required depending on the operating
system and configuration.

<details>
<summary>Experimental</summary>

- BSD-based (OpenBSD, DragonFly BSD, FreeBSD and NetBSD).
- Solaris-based (Solaris and Illumos).
- AIX.

Note: For the full list see [minimum requirements for Go](https://go.dev/wiki/MinimumRequirements) 1.27.

</details>

### Launcher and Battle Server Manager

- Windows without S edition/mode (recommended):
    - 7 on x86-64 (10 or higher recommended).
    - 11 on ARM.
- Linux with kernel 3.2 (5.10 or higher recommended):
    - x86-64 (recommended).
    - ARM64.
- macOS Ventura v13 (Sequoia v15 or higher recommended):
    - Intel (recommended).
    - Apple Silicon. Required for AoE II: DE native Steam version in Launcher.

**Note for launcher: If you allow (and is needed) to handle the hosts file, local certificate, or an elevated custom
game launcher,
it will require admin rights elevation.**

### Client

- Age of Empires: Definitive Edition
  on [Xbox](https://www.xbox.com/games/store/age-of-empires-definitive-edition/9njwtjsvgvlj) or
  [Steam](https://store.steampowered.com/app/1017900/Age_of_Empires_Definitive_Edition).
  Recommended version *100.2.31845.0* or later.
- Age of Empires II: Definitive Edition
  on [Xbox](https://www.xbox.com/games/store/age-of-empires-ii-definitive-edition/9N42SSSX2MTG/0010) or
  [Steam](https://store.steampowered.com/app/813780/Age_of_Empires_II_Definitive_Edition).
  Minimum a mid 2024 version or later.
- Age of Empires III: Definitive Edition
  on [Xbox](https://www.xbox.com/games/store/age-of-empires-iii-definitive-edition/9n1hf804qxn4) or
  [Steam](https://store.steampowered.com/app/933110/Age_of_Empires_III_Definitive_Edition).
  Recommended a late 2023 version or later.
- Age of Empires IV: Anniversary Edition
  on [Xbox](https://www.xbox.com/games/store/age-of-empires-iv-anniversary-edition/9n94ncgm1q2n) or
  [Steam](https://store.steampowered.com/app/1466860/Age_of_Empires_IV_Anniversary_Edition).
  Recommended a 2025 version or later.
- Age of Mythology: Retold
  on [Steam](https://store.steampowered.com/app/1934680/Age_of_Mythology_Retold). Recommended a 2025 version or later.

*Note: An up-to-date (or slightly older) version is highly recommended as there are known issues with older versions.*

## Binaries

See the [releases page](https://github.com/luskaner/ageServer/releases) for server and launcher binaries for a
subset of
supported operating systems.
<details>
    <summary>Provided archives</summary>

* Full:
    * Windows:
        * **7 on x86-64**: ...\_full\_*A.B.C*_win7_x86-64.zip
        * **10 on x86-64**: ...\_full\_*A.B.C*_win10_x86-64.zip
        * **11 on ARM**: ...\_full\_*A.B.C*_win11_arm64.zip
    * Linux:
        * **x86-64**: ...\_full\_*A.B.C*_linux_x86-64.tar.gz
        * **ARM64**: ...\_full\_*A.B.C*_linux_arm64.tar.gz
    * macOS: ...\_full\_*A.B.C*_mac.tar.gz
* Launcher:
    * Windows:
        * **7 on x86-64**: ...\_launcher\_*A.B.C*_win7_x86-64.zip
        * **10 on x86-64**: ...\_launcher\_*A.B.C*_win10_x86-64.zip
        * **11 on ARM**: ...\_launcher\_*A.B.C*_win11_arm64.zip
    * Linux:
        * **x86-64**: ...\_launcher\_*A.B.C*_linux_x86-64.tar.gz
        * **ARM64**: ...\_launcher\_*A.B.C*_linux_arm64.tar.gz
    * macOS: ...\_launcher\_*A.B.C*_mac.tar.gz
* Battle Server Manager:
    * Windows:
        * **7 on x86-64**: ...\_battle-server-manager\_*A.B.C*_win7_x86-64.zip
        * **10 on x86-64**: ...\_battle-server-manager\_*A.B.C*_win10_x86-64.zip
        * **11 on ARM**: ...\_battle-server-manager\_*A.B.C*_win11_arm64.zip
    * Linux:
        * **x86-64**: ...\_battle-server-manager\_*A.B.C*_linux_x86-64.tar.gz
        * **ARM64**: ...\_battle-server-manager\_*A.B.C*_linux_arm64.tar.gz
    * macOS: ...\_battle-server-manager\_*A.B.C*_mac.tar.gz
* Server:
    * Windows:
        * **7, Server 2008 R2, Home Server 2011, Embedded 7 on x86-64**:
          ...\_server\_
          *A.B.C*_
          win7_x86-64.zip
        * **7, Embedded 7, Thin PC on x86-32**:
          ...\_server\_
          *A.B.C*_
          win7_x86-32.zip
        * **10 (IoT), Server (IoT) 2025 on ARM64**: ...\_server\_*A.B.C*_win10_arm64.zip
        * **10 (IoT), (Storage) Server 2016, Server IoT 2019 on x86-64**: ...\_server\_*A.B.C*_win10_x86-64.zip
        * **10 (IoT) on x86-32**: ...\_server\_*A.B.C*_win10_x86-32.zip
    * Linux:
        * **ARM64**: ...\_server\_*A.B.C*_linux_arm64.tar.gz
        * **ARM32**:
            * ARMv5 (armel): ...\_server\_*A.B.C*_linux_arm-5.tar.gz
            * ARMv6 (sometimes called armhf): ...\_server\_*A.B.C*_linux_arm-6.tar.gz
        * **x86-64**: ...\_server\_*A.B.C*_linux_x86-64.tar.gz
        * **x86-32**: ...\_server\_*A.B.C*_linux_x86-32.tar.gz
    * macOS - Ventura (v13): ...\_server\_*A.B.C*_mac.tar.gz

</details>

*Note: If you are using Antivirus it may flag one or more executables as virus, this is a **false positive***.

*Note: On macOS, downloaded binaries are quarantined by Gatekeeper and may be refused as unverified as releases
are not notarized. From the extracted download directory, run the following in the terminal to clear it:*
`xattr -dr com.apple.quarantine .` *(alternatively, right-click each binary and choose Open). Self-built binaries
are unaffected.*

### Verification

The verification process ensures that the files you download are the same as the ones that were uploaded by the
maintainer.

<details>
    <summary>Verification steps</summary>

1. Check the release tag is verified with the committer's signature key (*as all commits must be*).
2. Download the ```..._checksums.txt``` and ```..._checksums.txt.sig``` files.
3. Import the [release public key](release_public.key) and import it to your keyring if you haven't already.
4. Verify the ```..._checksums.txt``` file with the ```..._checksums.txt.sig``` file.
5. Verify the SHA-256 checksum list inside ```..._checksums.txt``` with the downloaded archives.

Exceptions on tag/commit signature:

* Tags:
    * *v1.2.0-rc.5*: mantainer error.
* Commits:
    * *631cfa1* through *9eb66cf* (*both included*): rebase and merge PR issue.
    * *55697d4*: rebase of dependabot.
    * *feb28de*: partially verified due to dependabot.
    * *d2b1749*, *82ca9f1* and *baa75ce*: merge mistake.

</details>

## Installation

Both the launcher and server work out of the box without any installation. Just download the archives,
decompress and run them.

## How it works

### Server

The server is simple web server that listens to the game's API requests. The server reimplements
the minimum required API surface to allow the game to work with lobbies. Some requests are partially proxied to cdn.ageofempires.com and api.ageofempires.com and a request is used to get the public IP

*Note: See the [server README](server/README.md) for more details.*

### Launcher

The launcher allows to easily play the game in lobbies while still allowing the official launcher to be used for online
play.

<details>
    <summary>Features</summary>

- Automatically start/stop the server or connect to an existing one.
- (Optional) Use an isolated metadata (except AoE I) directory to avoid potential issues with the official
  game.
- (Optional) Modify the hosts file to:
    - Redirect the game's API requests to the LAN server.
    - Proxy certain requests:
        - CDN so it does not detect the official game status.
        - Chat and usernames show properly.
- (Optional) Install a self-signed certificate to allow the game to connect to the LAN server.
- (except AoE I and AoE IV) Add a certificate to the game's store to allow the game to connect to the LAN server.
- (Optional) Run custom configuration commands to set up/revert the configuration.
- (Windows Optional) Re-broadcast the battle server through other network interfaces apart from the most priority one.
- Automatically find and start the game.

Afterward, it reverses any changes to allow the official launcher to connect to the official servers.
</details>

*Note: See the [launcher README](launcher/README.md) for more details.*

## Simplest way to use it

1. **Download** the proper *full* asset from the latest
   stable release from https://github.com/luskaner/ageServer/releases.
2. **Uncompress** it somewhere.
3. *Windows Optional*: *You may need to add the launcher/server binaries to the exception list of your Antivirus*.
4. *Windows Optional*: Unblock the `.exe` files as
   explained [here](https://www.tenforums.com/tutorials/5357-unblock-file-windows-10-a.html)
5. If not using the Xbox or Steam launcher, **edit the `launcher/resources/config.<game_title>.toml` file** with a text
   editor (like Notepad)
   and modify the `Client.Executable` section to point to the game launcher path. In this case, you also need to modify
   `Client.Path` to point to the game's directory.
   **You will need to use a custom launcher (plus what my
   other [repo](https://github.com/luskaner/ageServerLauncherCompanion) provides) for 100% offline play**.
6. **Execute `launcher/start_<game_title>` script**: you will be asked for admin elevation and
   confirmation of other dialogs as needed, you will also need to allow the connections via the Microsoft Defender
   Firewall or any other.
7. **Repeat the above steps for every PC** (except the point 6) you want to play in LAN with by running the *launcher*,
   the first PC to
   launch it will host the "server" and the rest will auto-discover and be prompted to connect to it.
8. In the game, just host a new lobby via the Multiplayer section. Setting it to public visibility is recommended.
9. If the lobby is Public, they can join directly in the browser or you can **Invite friends** by searching them by
    name
    and sending an invitation as needed. If the game allows, you can share the link to join the lobby automatically (
    only
    works if already
    in-game).

## Standalone execution

<details>
    <summary>Server instructions</summary>

1. **Download** the proper *server* asset from latest
   stable [release](https://github.com/luskaner/ageServer/releases).
2. **Generate the certificate** by simply executing `bin/genCert`.
3. If needed **edit the [config](server/resources/config/config.toml) file**.
4. **Run** the `server` binary for all games or the `start_` game title specific script.

</details>

<details>
    <summary>Launcher instructions</summary>

1. **Download** the proper *launcher* asset from latest
   stable [release](https://github.com/luskaner/ageServer/releases).
2. If needed **edit the `launcher/resources/config.<game_title>.toml` and/or `launcher/resources/config.toml` files**.
   You will need to edit the `Client.Executable` section to point to the game launcher path if using a custom launcher
   which you will need to use
   a custom launcher for 100% offline play.
3. **Run** the `start_...` script.

*Note: If you have any issues run the `bin/config revert -a`.*
</details>

<details>
    <summary>Battle Server Manager instructions</summary>

1. **Download** the proper *battle-server-manager* asset from latest
   stable [release](https://github.com/luskaner/ageServer/releases).
2. If needed **edit the `battle-server-manager/resources/config.<game_title>.toml`**.
3. **Run** the `start_...` script.

</details>

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) to see how to develop and release builds.

## Additional Terms of Use for the Downloadable Package

**Important Notice:**  
This software is distributed under the [AGPL](https://www.gnu.org/licenses/agpl-3.0.html) (Affero General Public
License), which guarantees every user the right to use, study, modify, and redistribute the source code. The following
additional terms govern only the contractual relationship between the provider of the downloadable package and the user
who obtains it through this channel. These terms **do not affect or restrict** the rights granted under the AGPL, which
shall prevail over any additional restrictions when it comes to redistribution, modification, or access to the code.

**By downloading and using this package, you agree to the following:**

1. **Game License:**  
   You are only authorized to use this downloadable package if you possess a valid and legal license for the
   corresponding game, including any downloadable content (DLC) required for the software to operate properly.
2. **Compliance with Game Terms:**  
   The use of the software is contingent upon your full compliance with the terms of service and any applicable
   conditions established for the game.
3. **Personal Use Only:**  
   This downloadable package is intended for **strictly personal use**. Commercial use or any use beyond personal
   purposes is prohibited unless express written consent is obtained from the provider.
4. **Limitation of Provider's Liability:**  
   These additional terms apply solely to the original downloadable package provided by the provider. The provider
   assumes no responsibility for any misuse of the software or for intellectual property infringements resulting from
   its use contrary to these terms. Any liability arising from the improper use of the software lies exclusively with
   you.

**Disclaimer: This software is not affiliated or endorsed by any publisher or developer of the games.**
