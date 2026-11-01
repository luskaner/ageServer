package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/steam"
)

var (
	steamConfigPathFn     = steam.ConfigPath
	steamConfigPathAltFn  = steam.ConfigPathAlt
	gameUserProfilePathFn = game.UserProfilePath
	windowsToUnixPathFn   = WindowsToUnixPath
	fmtPrintFn            = fmt.Print
	fmtFprintlnFn         = func(a ...any) (n int, err error) { return fmt.Fprintln(os.Stderr, a...) }
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run executes the requested subcommand. Errors are returned instead of being
// silently swallowed: an empty stdout with a zero exit code would be
// indistinguishable from a successfully converted empty path.
func run(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: config-helper <windowsToUnixPath|configPath|userProfilePath> [args...]")
	}
	extra := args[2:]
	switch args[1] {
	case "windowsToUnixPath":
		if len(extra) < 1 {
			return errors.New("windowsToUnixPath requires a path argument")
		}
		return convertAndPrint(extra[0])
	case "configPath":
		if len(extra) < 1 {
			return errors.New("configPath requires a boolean argument")
		}
		var result string
		if extra[0] == "true" {
			result = steamConfigPathAltFn()
		} else {
			result = steamConfigPathFn()
		}
		return convertAndPrint(result)
	case "userProfilePath":
		if len(extra) < 1 {
			return errors.New("userProfilePath requires a profile argument")
		}
		return convertAndPrint(gameUserProfilePathFn(extra[0]))
	default:
		return fmt.Errorf("unknown command %q", args[1])
	}
}

func convertAndPrint(path string) error {
	convertedResult, err := windowsToUnixPathFn(path)
	if err != nil {
		return err
	}
	fmtPrintFn(convertedResult)
	return nil
}
