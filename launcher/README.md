# Battle-Server-Manager

The launcher is a tool that allows you to launch the game to connect to the LAN server. It also handles configuring the
system and reverting that configuration upon exit.

## Minimum system Requirements

- Windows without S edition/mode (recommended):
    - 7 on x86-64 (10 or higher recommended).
    - 11 on ARM.
- Linux with kernel 3.2 (5.10 or higher recommended):
    - x86-64 (recommended).
    - ARM64.
- macOS Ventura v13 (Sequoia v15 or higher recommended):
    - Intel (recommended).
    - Apple Silicon. Required for AoE II: DE native Steam version in Launcher.

**Note: If you allow it to handle the hosts file, local certificate, or an elevated custom game launcher, it will
require admin rights elevation.**

## Features

## Server

- Generate a self-signed certificate.
- Start the server.
- Discover the server.
- Stop the server.

## Battle-Server-Manager

- Start the Online-like Battle Server.
- Stop the Online-like Battle Server.

## Client (via [`bin\config`](../launcher-config/README.md))

- Isolated metadata directory (except AoE I).
- Isolated profiles directory.
- Smart modify the hosts file.
- Smart install of a self-signed certificate.
- Add certificate to the game's trusted store (except AoE I and AoE IV).

All possible client modifications are reverted upon the launcher's exit.

## Command Line

CLI is available with similar options as the configuration. You can see the available options with
`launcher -h`. Some configuration options are only exclusive to the CLI and some to the configuration files.

## Configuration

The configuration options are available in the [`config.toml`](resources/config.toml) and [
`config.game.toml`](resources/config.game.toml) files. The files contain comments
that
should help you understand the options.

## Exit Codes

* [Base codes](../common/errors.go).
* [Launcher shared codes](../launcher-common/errors.go).
* [Own codes](internal/errors.go).
