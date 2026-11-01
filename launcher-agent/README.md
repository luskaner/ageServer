# Launcher Agent

The agent is run after the launcher starts the game and is responsible for reverting the configuration applied by it
(via `config`) and stopping the server if necessary after the game exits. It may also apply a `BattleServer` fix if
configured to do so. It resides in `bin` subdirectory.

## System tray

The agent outlives the launcher and is started detached, with no console of its own, so it leaves
an icon in the system tray for as long as it is working. The icon says what the agent is doing, both on hover and on a
greyed out row of its menu, and its only action is an `Exit` which stops the run and reverts the configuration through
the same teardown a closed game goes through.

This is not a feature that can be turned off: the agent is the process with no launcher left to close it, and the
icon is the only thing on screen that says it exists and the only thing on screen that can stop it.

## Exit Codes

* [Base codes](../common/errors.go).
* [Launcher shared codes](../launcher-common/errors.go).
* [Own codes](internal/errors.go).
