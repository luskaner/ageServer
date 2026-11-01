// Package cmd is the console launcher's front end.
//
// The session itself is not here. It is in launcher-common/launcher/session, so
// that anything that keeps running, such as a window, can perform one. What is
// left is the part only a console has: the flag set it parses, where the
// narration goes, and which backend answers the questions when the run asks.
package cmd

import (
	"context"

	"os"

	"github.com/luskaner/ageServer/common"
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/ops/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher/session"
	"github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher/internal/dialog"
	"github.com/spf13/pflag"
)

// Version is set from main and shown in the session summary.
var Version string

// Execute parses the command line and performs the session.
func Execute() (err error, exitCode int) {
	singleFs := commonCmd.NewSingleFlagSet(runRoot, Version)
	// Registered by the shared launcher, not here, so that a frontend which
	// registers the same options on a flag set of its own gets the same names,
	// the same defaults and the same help out of the same files.
	if err := session.BindFlags(singleFs.Fs()); err != nil {
		return err, common.ErrSyntax
	}
	return singleFs.Execute()
}

// runRoot exists to be the function a flag set can call, so it takes no context
// and gets one.
func runRoot(fs *pflag.FlagSet) (err error, exitCode int) {
	return runSession(context.Background(), fs)
}

// runSession is what a console run performs. A frontend that keeps running
// calls session.Run with a context it can cancel instead.
func runSession(ctx context.Context, fs *pflag.FlagSet) (err error, exitCode int) {
	// Installed per run rather than at init: the reporter writes to a log file
	// that a test replaces, and one captured at init would keep writing to the
	// log of whichever run happened to be first.
	configure()
	return session.Run(ctx, fs)
}

// configure hands the session everything that is specific to a console.
//
// It is a function rather than an argument because the answer to "which backend
// asks the questions" is not known until a run has read its configuration, and
// it is the same answer for every run of this program.
func configure() {
	// One reporter, shared. The window frontend wraps the same one to draw the
	// same lines a second time somewhere else, which is only possible because
	// this half of the dual sink is not the console's private business.
	report := logger.Reporter{}
	session.Configure(session.Setup{
		Version: Version,
		Report:  report,
		NewDialog: func(enable bool) launcher.Resolution {
			resolution := dialog.New(enable)
			// The console backend prints its questions through the same sinks as
			// everything else, so they reach the log file too.
			dialog.SetOutput(dialog.Output{Println: report.Println, Printf: report.Printf})
			return resolution
		},
		Stdin:     os.Stdin,
		Presenter: ui.Presenter{},
	})
}
