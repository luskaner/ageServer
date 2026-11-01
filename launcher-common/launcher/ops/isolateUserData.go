package ops

import (
	"io"
	"strings"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

func ResolveIsolateValue(value string, officialLauncher bool) bool {
	switch value {
	case "true":
		return true
	case "false":
		return false
	case "required":
		return officialLauncher
	default:
		return false
	}
}

func (c *Config) IsolateUserData(metadata bool, profiles bool, path string) (exitCode int) {
	if metadata || profiles {
		var isolateItems []string
		if metadata {
			isolateItems = append(isolateItems, "metadata")
		}
		if profiles {
			isolateItems = append(isolateItems, "profiles")
		}
		c.report().Step("%s", "Backing up "+strings.Join(isolateItems, " and ")+".")
		var err error
		if err = commonLogger.FileLogger.Buffer("config_setup_isolate", func(writer io.Writer) {
			cfgSetupOpts := executor.NewConfigSetupOptions()
			cfgSetupOpts.Out = writer
			cfgSetupOpts.OptionsFn = func(options *exec.Options) {
				commonLogger.Println("run config setup for data isolation", options.String())
			}
			cfgSetupOpts.GameId = c.gameId
			cfgSetupOpts.Metadata = metadata
			cfgSetupOpts.Profiles = profiles
			cfgSetupOpts.DataPath = path
			cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
			if result := cfgSetupOpts.RunSetUp(); !result.Success() {
				isolateMsg := "Failed to backup "
				c.report().Fail("%s", isolateMsg+strings.Join(isolateItems, " or ")+".")
				exitCode = launcher.ErrMetadataProfilesSetup
				if result.Err != nil {
					c.report().Fault("Error message: %s", result.Err.Error())
				}
				if result.ExitCode != common.ErrSuccess {
					c.report().Fault("Exit code: %d.", result.ExitCode)
				}
			}
		}); err != nil {
			c.report().Fail("Failed to write isolate setup log: %s", err)
		}
	}
	return
}
