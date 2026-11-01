// Package dialog holds the launcher console's answers to the interactive
// prompts. The prompts themselves, and the Dialog interface, live in
// launcher-common/launcher: every frontend asks the same questions, and only the
// way they are asked differs.
//
// It must not import the shared operations or their logger: those need dialog,
// and their logger needs dialog too, so console output goes through the
// injectable Output sinks instead.
package dialog

import (
	"fmt"
	"sync"

	"github.com/luskaner/ageServer/launcher-common/launcher"
)

// The vocabulary belongs to the shared package, so a graphical frontend answers
// the same questions with the same types. These aliases keep this package's
// callers saying dialog.ServerCandidate instead of repeating where it came from.
type (
	Dialog          = launcher.Dialog
	ServerCandidate = launcher.ServerCandidate
	Resolution      = launcher.Resolution
)

// label is the compact summary of a candidate, falling back to the full
// description when no compact one was provided.
//
// It is a backend detail rather than a method on the candidate: which of the two
// forms fits depends on how much room the backend has.
func label(c ServerCandidate) string {
	if c.Label == "" {
		return c.Description
	}
	return c.Label
}

// Output are the sinks the console prompts write to. They mirror
// the shared logger so prompt output keeps reaching both the log file and stdout.
type Output struct {
	Println func(...any)
	Printf  func(string, ...any)
}

var (
	outputMu sync.RWMutex
	output   = defaultOutput()
)

func defaultOutput() Output {
	return Output{
		Println: func(a ...any) { fmt.Println(a...) },
		Printf:  func(f string, a ...any) { fmt.Printf(f, a...) },
	}
}

// sinks returns the current sinks, tolerating a partially installed Output.
func sinks() Output {
	outputMu.RLock()
	o := output
	outputMu.RUnlock()
	if o.Println == nil || o.Printf == nil {
		return defaultOutput()
	}
	return o
}

// New resolves mode ("auto", "true" or "false") to a dialog backend. It never
// returns a nil Dialog.
func New(enable bool) Resolution {
	if enable {
		if availableOnce() {
			return Resolution{Dialog: zenityDialog{}, Name: zenityDialog{}.Name()}
		}
		// mode=true never aborts the launcher: the library only reports
		// unavailable on Linux/BSD when qarma/zenity/matedialog are missing
		// from PATH, and someone who explicitly asked for graphical dialogs
		// must not be locked out by that.
		return consoleResolution("Graphical dialogs are not available in this system, using the console instead.")
	}
	return consoleResolution("")
}

// consoleResolution builds the console fallback, optionally explaining why the
// configured backend could not be honoured.
func consoleResolution(reason string) Resolution {
	d := consoleDialog{}
	return Resolution{Dialog: d, Name: d.Name(), Reason: reason}
}

// Set installs the dialog for the rest of the session. Set is safe to call
// from tests; RunRoot resets it after teardown.
//
// The slot itself belongs to the shared package, because the logic that asks
// the questions lives there and must not depend on a console backend to reach
// it. What is installed is still decided here, where the backends are.
func Set(d Dialog) { launcher.SetDialog(d) }

// Active returns the installed dialog, falling back to the console when nothing
// is installed.
func Active() Dialog { return launcher.ActiveDialog() }

// Reset removes the installed dialog.
func Reset() { launcher.ResetDialog() }

// SetOutput installs the sinks used by the console dialog.
func SetOutput(o Output) {
	outputMu.Lock()
	output = o
	outputMu.Unlock()
}
