package launcher

import "sync/atomic"

// Spinner is an in-place line that runs while something is taking a while.
//
// It is separate from Reporter because it is not a message: it is a position on
// screen that exists only between Start and its ending, and a frontend draws that
// as a spinner, as a progress bar, or as nothing at all.
type Spinner interface {
	// Done ends it with the line that says it worked.
	Done(format string, a ...any)
	// Fail ends it with the line that says it did not.
	Fail(format string, a ...any)
	// Info ends it with a line that is neither.
	Info(format string, a ...any)
	// Stop ends it without saying anything, for when something else already
	// said the outcome.
	Stop()
}

// Progress is the frontend's own progress indicator: a fraction of the way
// through, not a message.
type Progress interface {
	// Set reports how far along it is, as a percentage.
	Set(percent int)
	// Done ends it as finished.
	Done()
	// Fail ends it as failed.
	Fail()
}

// Presenter is how a frontend shows where a run is, as opposed to what it says.
//
// Reporter answers "what happened". Presenter answers "where is it now": the
// banner, the headings, the summary of what was configured, and the indicators
// that run while something is in progress. The two are separate because a window
// wants both and draws them differently, and because the parts of Presenter that
// write on their own, the spinner and the progress bar, are the parts that would
// otherwise scribble on a terminal a graphical frontend does not own.
//
// The default shows nothing. That is a deliberate default rather than a missing
// one: a frontend that installs nothing gets a run with no decoration instead of
// escape sequences on whatever stdout happens to be attached.
type Presenter interface {
	// Banner is the header naming the program and version.
	Banner(program, version string)
	// Section starts a phase.
	Section(title string)
	// KV adds one line to a section's body.
	KV(indent int, key, value string)
	// Hint says something about the frontend itself, such as that the console
	// cannot show what it could otherwise show. It returns nothing when there is
	// nothing to say, which is the usual case.
	Hint() string
	// ApplyOutput records an explicit choice about how much decoration to use. A
	// frontend whose drawing depends on what the output can do acts on it; one
	// that does not ignores it.
	ApplyOutput(choice string)
	// Start begins an in-place line.
	Start(label string) Spinner
	// BeginProgress starts the progress indicator.
	BeginProgress() Progress
	// ClearProgress removes any progress indicator left on screen.
	ClearProgress()
}

var activePresenter atomic.Pointer[Presenter]

// SetPresenter installs how the run shows where it is. It is a slot rather than a
// parameter because the operations that draw are several layers down and are
// shared by both frontends, exactly like the dialog.
func SetPresenter(p Presenter) { activePresenter.Store(&p) }

// ActivePresenter returns the installed Presenter, or one that shows nothing.
func ActivePresenter() Presenter {
	if p := activePresenter.Load(); p != nil && *p != nil {
		return *p
	}
	return SilentPresenter{}
}

// ResetPresenter removes the installed Presenter.
func ResetPresenter() { activePresenter.Store(nil) }

// SilentPresenter shows nothing at all.
type SilentPresenter struct{}

func (SilentPresenter) Banner(string, string)   {}
func (SilentPresenter) Section(string)          {}
func (SilentPresenter) KV(int, string, string)  {}
func (SilentPresenter) Hint() string            { return "" }
func (SilentPresenter) ApplyOutput(string)      {}
func (SilentPresenter) Start(string) Spinner    { return silentSpinner{} }
func (SilentPresenter) BeginProgress() Progress { return silentProgress{} }
func (SilentPresenter) ClearProgress()          {}

type silentSpinner struct{}

func (silentSpinner) Done(string, ...any) {}
func (silentSpinner) Fail(string, ...any) {}
func (silentSpinner) Info(string, ...any) {}
func (silentSpinner) Stop()               {}

type silentProgress struct{}

func (silentProgress) Set(int) {}
func (silentProgress) Done()   {}
func (silentProgress) Fail()   {}
