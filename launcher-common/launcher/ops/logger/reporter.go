package logger

import "github.com/luskaner/ageServer/launcher-common/launcher"

// Reporter is this log seen as the Reporter a session narrates itself through.
//
// It is the part of being a frontend that every frontend shares: the lines a run
// produces belong in launcher.txt, whichever one is listening, because that file
// is what a user attaches to a bug report and what the next run reads when it
// works out what a previous one left behind. A frontend that wrote its own
// nine-method adapter would either skip the file log, and lose both, or
// reimplement the dual sink and get one of the two halves subtly wrong.
//
// The console draws the other half, through ui, and so does this: a frontend with
// a window of its own has nothing listening on the terminal, so that half goes
// nowhere and costs a string per line. What a frontend adds is a destination of
// its own, by wrapping this rather than replacing it.
type Reporter struct{}

// Checked here rather than where the interface is declared, for the same reason
// ui checks its own: an adapter that quietly stopped covering a method would
// compile until a run asked for it.
var _ launcher.Reporter = Reporter{}

func (Reporter) Ok(format string, a ...any) { Ok(format, a...) }

func (Reporter) Fail(format string, a ...any) { Fail(format, a...) }

func (Reporter) Warn(format string, a ...any) { Warn(format, a...) }

func (Reporter) Info(format string, a ...any) { Info(format, a...) }

func (Reporter) Step(format string, a ...any) { Step(format, a...) }

func (Reporter) Detail(format string, a ...any) { Detail(format, a...) }

func (Reporter) Fault(format string, a ...any) { Fault(format, a...) }

func (Reporter) Println(a ...any) { Println(a...) }

func (Reporter) Printf(format string, a ...any) { Printf(format, a...) }
