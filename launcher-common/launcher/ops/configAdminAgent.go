package ops

import (
	"io"

	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/spf13/pflag"
)

func (c *Config) RunStopAgent() (result *exec.Result) {
	options := exec.Options{
		File:     executables.NativeFileName(false, executables.LauncherConfig),
		Wait:     true,
		Args:     cmd.FlagSetToArgs(pflag.NewFlagSet("stopAgent", pflag.ContinueOnError), true),
		ExitCode: true,
	}
	if buffErr := commonLogger.FileLogger.Buffer("stop_agent", func(writer io.Writer) {
		commonLogger.Println("run stop agent", options.String())
		if writer != nil {
			options.Stderr = writer
			options.Stdout = writer
		}
		result = options.Exec()
	}); buffErr != nil {
		commonLogger.Println("Failed to write stop_agent log:", buffErr)
	}
	return
}
