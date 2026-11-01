// Package cmdlog is the console renderer every program in this workspace prints
// through.
//
// It exists so that the rule is the same in all of them: a message goes to the
// file log as plain text and to the console as a styled line, and the two are the
// same words. The file log is read with grep and with editors that do not
// understand a terminal, so a marker, a colour or an emoji in there is corruption;
// the console is the only place where any of it means something.
//
// This is a simplified version that doesn't depend on launcher-common/ui,
// using only standard library and common/logger.
package cmdlog

import (
	"fmt"
	"os"

	commonLogger "github.com/luskaner/ageServer/common/logger"
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
	commonLogger.Initialize(nil)
}

// dual is the shape of every renderer below.
func dual(plain string, rendered string) {
	commonLogger.PrefixPrintln("main", plain)
	_, _ = fmt.Fprintln(os.Stdout, rendered)
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
func Ok(format string, a ...any) { dual(text(format, a...), okStyle(format, a...)) }

// Fail reports a failed operation.
func Fail(format string, a ...any) { dual(text(format, a...), failStyle(format, a...)) }

// Warn reports a non blocking problem.
func Warn(format string, a ...any) { dual(text(format, a...), warnStyle(format, a...)) }

// Info reports a note.
func Info(format string, a ...any) { dual(text(format, a...), infoStyle(format, a...)) }

// Step reports an action that is being carried out.
func Step(format string, a ...any) { dual(text(format, a...), stepStyle(format, a...)) }

// Detail reports a line subordinate to the message above it.
func Detail(format string, a ...any) { dual(text(format, a...), detailStyle(format, a...)) }

// Fault reports a subordinate line that belongs to a failure, in red.
func Fault(format string, a ...any) { dual(text(format, a...), faultStyle(format, a...)) }

// Println writes a message to both sinks untouched, for the text that is not one
// of the levels above: data, a stack trace, a file's contents.
func Println(a ...any) {
	commonLogger.PrefixPrintln("main", a...)
	_, _ = fmt.Fprintln(os.Stdout, fmt.Sprint(a...))
}

// Printf is Println for a format string.
func Printf(format string, a ...any) {
	commonLogger.PrefixPrintf("main", format, a...)
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", a...)
}

// Banner prints the program header.
func Banner(program string, version string) {
	line := fmt.Sprintf("=== %s %s ===", program, version)
	_, _ = fmt.Fprintln(os.Stdout, line)
	commonLogger.PrefixPrintln("main", line)
}

// Section starts a phase.
func Section(title string) {
	line := fmt.Sprintf("--- %s ---", title)
	_, _ = fmt.Fprintln(os.Stdout, line)
	commonLogger.PrefixPrintln("main", line)
}

// KV prints a key and its value as one sentence.
func KV(indent int, key string, value string) {
	line := fmt.Sprintf("%s%s: %s", indentString(indent), key, value)
	_, _ = fmt.Fprintln(os.Stdout, line)
	commonLogger.PrefixPrintln("main", line)
}

// Spinner animates a line in place where the console can do it, and prints one
// plain line where it cannot.
func Spinner(label string) *SpinnerHandle {
	h := &SpinnerHandle{label: label, active: true}
	_, _ = fmt.Fprintf(os.Stdout, "  %s... ", label)
	return h
}

type SpinnerHandle struct {
	label   string
	active  bool
	doneMsg string
}

func (h *SpinnerHandle) Done(format string, a ...any) {
	if !h.active {
		return
	}
	h.active = false
	h.doneMsg = fmt.Sprintf(format, a...)
	_, _ = fmt.Fprintf(os.Stdout, "\r  %s... %s\n", h.label, okStyle(h.doneMsg))
}

func (h *SpinnerHandle) Fail(format string, a ...any) {
	if !h.active {
		return
	}
	h.active = false
	h.doneMsg = fmt.Sprintf(format, a...)
	_, _ = fmt.Fprintf(os.Stdout, "\r  %s... %s\n", h.label, failStyle(h.doneMsg))
}

func (h *SpinnerHandle) Info(format string, a ...any) {
	if !h.active {
		return
	}
	h.active = false
	h.doneMsg = fmt.Sprintf(format, a...)
	_, _ = fmt.Fprintf(os.Stdout, "\r  %s... %s\n", h.label, infoStyle(h.doneMsg))
}

func (h *SpinnerHandle) Stop() {
	if !h.active {
		return
	}
	h.active = false
	_, _ = fmt.Fprintf(os.Stdout, "\r  %s... %s\n", h.label, warnStyle("stopped"))
}

// ProgressHandle is a simple progress indicator.
type ProgressHandle struct {
	total int
}

func BeginProgress() *ProgressHandle {
	return &ProgressHandle{total: 100}
}

func (p *ProgressHandle) Set(percent int) {
	// Simple progress - just print percentage
	_, _ = fmt.Fprintf(os.Stdout, "\rProgress: %d%%", percent)
}

func (p *ProgressHandle) Done() {
	_, _ = fmt.Fprintf(os.Stdout, "\rProgress: 100%% - Done\n")
}

func (p *ProgressHandle) Fail() {
	_, _ = fmt.Fprintf(os.Stdout, "\rProgress: Failed\n")
}

func ClearProgress() {
	_, _ = fmt.Fprint(os.Stdout, "\r                                    \r")
}

// --- Style helpers ---

func okStyle(format string, a ...any) string {
	return "[OK] " + fmt.Sprintf(format, a...)
}

func failStyle(format string, a ...any) string {
	return "[FAIL] " + fmt.Sprintf(format, a...)
}

func warnStyle(format string, a ...any) string {
	return "[WARN] " + fmt.Sprintf(format, a...)
}

func infoStyle(format string, a ...any) string {
	return "[INFO] " + fmt.Sprintf(format, a...)
}

func stepStyle(format string, a ...any) string {
	return "[STEP] " + fmt.Sprintf(format, a...)
}

func detailStyle(format string, a ...any) string {
	return "    " + fmt.Sprintf(format, a...)
}

func faultStyle(format string, a ...any) string {
	return "[FAULT] " + fmt.Sprintf(format, a...)
}

func indentString(indent int) string {
	if indent <= 0 {
		return ""
	}
	return fmt.Sprintf("%*s", indent*2, "")
}