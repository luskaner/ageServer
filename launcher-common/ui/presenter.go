package ui

import "github.com/luskaner/ageServer/launcher-common/launcher"

// Presenter draws a run on a terminal.
//
// It is the console's implementation of launcher.Presenter, and it is all of
// what a terminal frontend needs: the ui package already knew how to draw every
// one of these, and this is the adapter that lets the shared logic ask for them
// without knowing that a terminal is what is listening.
//
// The types are named for the role rather than for the package, so that a
// frontend installing one does not sound like it is installing a renderer.
type Presenter struct{}

// The interface is checked here rather than where it is declared: an adapter that
// quietly stopped covering a method would compile until a run asked for it.
var _ launcher.Presenter = Presenter{}

func (Presenter) Banner(program, version string) { Banner(program, version) }

func (Presenter) Section(title string) { Section(title) }

func (Presenter) KV(indent int, key, value string) { KV(indent, key, value) }

// Hint is the terminal's own complaint about itself: that it is showing less than
// it could. The session prints it as an info line, so it reaches the log file too.
func (Presenter) Hint() string { return UpgradeHint() }

func (Presenter) ApplyOutput(choice string) { ApplyOverride(choice) }

// Start begins the in-place line.
func (Presenter) Start(label string) launcher.Spinner { return &SpinnerLine{s: Start(label)} }

// BeginProgress starts the terminal's own progress indicator.
func (Presenter) BeginProgress() launcher.Progress { return &ProgressBar{p: BeginProgress()} }

func (Presenter) ClearProgress() { ClearProgress() }

// SpinnerLine is a launcher.Spinner drawn in place on a terminal. It is named for
// what it is rather than Spinner, because this package already has the type that
// does the drawing.
type SpinnerLine struct{ s *Spinner }

var _ launcher.Spinner = (*SpinnerLine)(nil)

func (b *SpinnerLine) Done(format string, a ...any) { b.s.Done(format, a...) }
func (b *SpinnerLine) Fail(format string, a ...any) { b.s.Fail(format, a...) }
func (b *SpinnerLine) Info(format string, a ...any) { b.s.Info(format, a...) }
func (b *SpinnerLine) Stop()                        { b.s.Stop() }

// ProgressBar is a launcher.Progress drawn in place on a terminal.
type ProgressBar struct{ p *Progress }

var _ launcher.Progress = (*ProgressBar)(nil)

func (b *ProgressBar) Set(percent int) { b.p.Set(percent) }
func (b *ProgressBar) Done()           { b.p.Done() }
func (b *ProgressBar) Fail()           { b.p.Fail() }
