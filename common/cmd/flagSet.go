package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/luskaner/ageServer/common"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/spf13/pflag"
)

type commandHandler func(args []string) (err error, exitCode int)
type RootFlagSet struct {
	fs       *pflag.FlagSet
	commands map[string]commandHandler
}

func NewRootFlagSet() *RootFlagSet {
	return &RootFlagSet{
		fs:       pflag.NewFlagSet("root", pflag.ContinueOnError),
		commands: make(map[string]commandHandler),
	}
}

func (r *RootFlagSet) RegisterCommand(name string, h commandHandler) {
	r.commands[name] = h
}

func (r *RootFlagSet) Execute(version string) (err error, exitCode int) {
	return r.ExecuteWithArgs(version, os.Args[1:])
}

func (r *RootFlagSet) ExecuteWithArgs(version string, args []string) (err error, exitCode int) {
	var showHelp, showVersion bool
	addDefaultFlags(r.fs, &showHelp, &showVersion)
	if err = r.fs.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}

	if !checkVersion(showVersion, version) {
		return
	}

	remaining := r.fs.Args()

	if showHelp || len(remaining) < 1 {
		commonLogger.Println("Available commands:")
		// deterministic order
		keys := make([]string, 0, len(r.commands))
		for k := range r.commands {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			commonLogger.Println("  ", k)
		}
		return
	}

	cmdName := remaining[0]
	if h, ok := r.commands[cmdName]; ok {
		return h(remaining[1:])
	}
	return errors.New("unknown command: " + cmdName + ", see --help"), common.ErrSyntax
}

func checkVersion(showVersion bool, version string) bool {
	if showVersion {
		if version == "" {
			commonLogger.Println("version: unknown")
		} else {
			commonLogger.Println("version:", version)
		}
		return false
	}
	return true
}

func addDefaultFlags(fs *pflag.FlagSet, showHelp *bool, showVersion *bool) {
	fs.SetInterspersed(false)
	fs.BoolVarP(showHelp, "help", "h", false, "Show help")
	fs.BoolVarP(showVersion, "version", "v", false, "Show version")
}

type singleCommandHandler func(fs *pflag.FlagSet) (err error, exitCode int)

type SingleFlagSet struct {
	command     singleCommandHandler
	fs          *pflag.FlagSet
	version     string
	showHelp    bool
	showVersion bool
}

func NewSingleFlagSet(command singleCommandHandler, version string) *SingleFlagSet {
	s := &SingleFlagSet{
		command: command,
		fs:      pflag.NewFlagSet(filepath.Base(os.Args[0]), pflag.ContinueOnError),
		version: version,
	}
	addDefaultFlags(s.fs, &s.showHelp, &s.showVersion)
	return s
}

func (s *SingleFlagSet) Fs() *pflag.FlagSet {
	return s.fs
}

func (s *SingleFlagSet) Execute() (err error, exitCode int) {
	return s.ExecuteWithArgs(os.Args[1:])
}

func (s *SingleFlagSet) ExecuteWithArgs(args []string) (err error, exitCode int) {
	if err = s.fs.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	if !checkVersion(s.showVersion, s.version) {
		return
	}
	if s.showHelp {
		s.fs.PrintDefaults()
		return
	}
	return s.command(s.fs)
}
