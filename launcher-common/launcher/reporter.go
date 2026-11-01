package launcher

// Reporter is how the shared launcher logic talks to whoever started it.
//
// The console launcher prints and exits. A graphical one has to show the same
// lines live and let the user watch a session that has not finished yet. Neither
// of those belongs in the logic itself, so every message goes through here
// instead of straight to a logger.
//
// The signatures are printf-style on purpose: the existing logger already has
// them, so the console side needs an adapter rather than a rewrite, and the level
// stays attached to the call rather than to a string a caller formats twice.
type Reporter interface {
	// Ok reports something that finished as intended.
	Ok(format string, a ...any)
	// Fail reports the step that failed. It is not necessarily fatal: the run
	// usually carries on to clean up before returning a failure.
	Fail(format string, a ...any)
	// Warn reports something the user should know about that did not stop the
	// run.
	Warn(format string, a ...any)
	// Info reports context the run decided on its own, such as a value that
	// came from the config instead of a flag.
	Info(format string, a ...any)
	// Step reports the beginning of a stage of the run.
	Step(format string, a ...any)
	// Detail reports the supporting detail under a Fail, typically an error
	// string.
	Detail(format string, a ...any)
	// Fault marks a line as the cause of a failure the user will see elsewhere.
	Fault(format string, a ...any)
	// Println and Printf write undecorated text. They exist for the prompts and
	// summaries that must not carry a level marker.
	Println(a ...any)
	Printf(format string, a ...any)
}

// Discard is a Reporter that throws everything away.
//
// It keeps a caller that has no interest in progress from having to implement
// seven methods, and it makes the absence of a reporter explicit at a call site
// instead of leaving a nil interface waiting to panic.
type Discard struct{}

func (Discard) Ok(string, ...any)     {}
func (Discard) Fail(string, ...any)   {}
func (Discard) Warn(string, ...any)   {}
func (Discard) Info(string, ...any)   {}
func (Discard) Step(string, ...any)   {}
func (Discard) Detail(string, ...any) {}
func (Discard) Fault(string, ...any)  {}
func (Discard) Println(...any)        {}
func (Discard) Printf(string, ...any) {}
