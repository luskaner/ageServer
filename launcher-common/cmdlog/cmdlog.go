// Package cmdlog is the console renderer every program in this workspace prints
// through.
//
// It exists so that the rule is the same in all of them: a message goes to the
// file log as plain text and to the console as a styled line, and the two are the
// same words. The file log is read with grep and with editors that do not
// understand a terminal, so a marker, a colour or an emoji in there is corruption;
// the console is the only place where any of it means something.
//
// The levels are the ones in launcher-common/ui. Which one a message gets is the
// only judgement call: a failure stops the run, a warning does not, a step is
// happening, info is a note, and Detail and Fault are the lines under the message
// above them, the latter in red because it is the part of a failure the reader acts
// on.
package cmdlog

import (
	"fmt"
	"os"

	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/ui"
)

// prefix is the section name the file log files these messages under. It is set
// once at start up by the program, because it is the program's own name and every
// program has a different one.
var prefix = "main"

// SetPrefix names the file log section these messages go under.
func SetPrefix(name string) {
	if name != "" {
		prefix = name
	}
}

// Initialize resolves the terminal capability and points the file log at a buffer.
//
// The nil writer is the whole trick. commonLogger writes to whatever it is given,
// and the console has to hear from the renderer and nowhere else: given os.Stdout
// it would print the plain text a second time, with a timestamp and a |MAIN|
// prefix, right above the styled line. Given nil it fills the buffer that becomes
// the file log, and the file log then gets exactly what the renderer took out.
//
// It also has to run before anything prints, because the profile decides whether
// the log lines carry a timestamp and a prefix, and that decision is made here.
func Initialize() {
	ui.Initialize(os.Stdout, os.Environ())
	commonLogger.Initialize(nil)
}

// dual is the shape of every renderer below.
func dual(plain string, rendered string) {
	commonLogger.PrefixPrintln(prefix, plain)
	ui.Println(rendered)
}

// text applies fmt only when arguments were supplied, so a message with a stray
// percent sign and no arguments survives untouched.
func text(format string, a ...any) string {
	if len(a) == 0 {
		return format
	}
	return fmt.Sprintf(format, a...)
}

// Ok reports a completed operation.
func Ok(format string, a ...any) { dual(text(format, a...), ui.Ok(format, a...)) }

// Fail reports a failed operation.
func Fail(format string, a ...any) { dual(text(format, a...), ui.Fail(format, a...)) }

// Warn reports a non blocking problem.
func Warn(format string, a ...any) { dual(text(format, a...), ui.Warn(format, a...)) }

// Info reports a note.
func Info(format string, a ...any) { dual(text(format, a...), ui.Info(format, a...)) }

// Step reports an action that is being carried out.
func Step(format string, a ...any) { dual(text(format, a...), ui.Step(format, a...)) }

// Detail reports a line subordinate to the message above it.
func Detail(format string, a ...any) { dual(text(format, a...), ui.Detail(format, a...)) }

// Fault reports a subordinate line that belongs to a failure, in red.
func Fault(format string, a ...any) { dual(text(format, a...), ui.Fault(format, a...)) }

// Println writes a message to both sinks untouched, for the text that is not one
// of the levels above: data, a stack trace, a file's contents.
func Println(a ...any) {
	commonLogger.PrefixPrintln(prefix, a...)
	ui.Println(fmt.Sprint(a...))
}

// Printf is Println for a format string.
func Printf(format string, a ...any) {
	commonLogger.PrefixPrintf(prefix, format, a...)
	ui.Printf(format, a...)
}

// Banner prints the program header. It prints nothing when the console width is
// unknown, so a redirected run never gets one.
func Banner(program string, version string) { ui.Banner(program, version) }

// Section starts a phase.
func Section(title string) { ui.Section(title) }

// KV prints a key and its value as one sentence.
func KV(indent int, key string, value string) { ui.KV(indent, key, value) }

// Spinner animates a line in place where the console can do it, and prints one
// plain line where it cannot.
func Spinner(label string) *ui.Spinner { return ui.Start(label) }

// BeginProgress takes the terminal's own progress indicator for a bounded task.
func BeginProgress() *ui.Progress { return ui.BeginProgress() }

// ClearProgress removes that indicator, which every exit path needs.
func ClearProgress() { ui.ClearProgress() }
