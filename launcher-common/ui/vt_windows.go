package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

// minUTF8OutputBuild is the first Windows build whose console is reliable with the
// UTF-8 code page.
//
// Build 10586 was the first with VT, and it shipped with the well known 65001
// problems: cmd.exe drew its own output wrong and the cursor drifted. Those were
// fixed over the following releases, and 19042 (Windows 11) is the first build
// where setting the code page is a plain improvement with nothing to weigh against
// it. Below that, and outside Windows Terminal, the ASCII fallback is a better
// outcome than a console that misrenders every program on it.
const minUTF8OutputBuild = 19042

// ensureUTF8Output puts the console on the UTF-8 code page when it safely can.
//
// It is not cosmetic. With code page 437 or 850, which is what a Spanish or a US
// Windows console still boots into, a UTF-8 byte sequence is decoded as two or
// three unrelated characters: every symbol in the catalogue arrives as garbage,
// and the glyph tier has no choice but to fall back to ASCII. Telling the user to
// run chcp, or to start the launcher through start.bat, means the program looks
// broken until they go and read something: the program already knows how to set
// the code page, so it sets it.
//
// Deliberately narrow, because this changes global state shared with every other
// process on the console:
//
//   - output only: the input code page is left alone, since changing it would
//     change how the keyboard is decoded for everyone else too;
//   - only when stdout really is a console, so a piped run leaves the terminal
//     that launched it untouched;
//   - only where the console is UTF-8 capable, and never on Windows 7, which is
//     the one case where the ASCII fallback is the contract rather than a
//     limitation;
//   - never when the code page is already UTF-8, so the common case does not
//     touch anything.
//
// It is best effort: a failure here is not an error, it only means the tier stays
// where it was.
func ensureUTF8Output() {
	// A handle that is not a console fails this query, which is how a pipe is
	// told apart from a console without guessing from the file type.
	if !stdoutIsConsole() {
		return
	}
	// Windows Terminal is UTF-8 by construction and keeps its own code page, so
	// there is nothing to set there and nothing to gain by trying.
	if os.Getenv("WT_SESSION") != "" {
		return
	}
	major, build := osVersion()
	if major < 10 || build < minUTF8OutputBuild {
		return
	}
	if outputCode() == utf8CodePage {
		return
	}
	_ = windows.SetConsoleOutputCP(utf8CodePage)
}

// stdoutIsConsole reports whether the standard output handle really is a console,
// as opposed to a pipe, a file or nothing at all.
func stdoutIsConsole() bool {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || h == 0 || h == windows.InvalidHandle {
		return false
	}
	var mode uint32
	return windows.GetConsoleMode(h, &mode) == nil
}

// vtUsable reports whether ANSI escape sequences will be interpreted rather than
// printed verbatim.
//
// The bit has to be verified, not assumed: Windows 10 with a plain conhost is
// detected as TrueColor by colorprofile even though nothing enabled VT, and the
// result is a screen full of "<ESC>[32m" literals.
func vtUsable() bool {
	// Emulators that ship their own VT interpreter are trustworthy as they are.
	if os.Getenv("WT_SESSION") != "" || os.Getenv("ConEmuANSI") == "ON" {
		return true
	}
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING was introduced in Windows 10 1511
	// (build 10586). Below that the bit does not exist, so there is no point in
	// trying, and Windows 7 keeps its console mode untouched.
	major, build := osVersion()
	if major < 10 || build < 10586 {
		return false
	}
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || h == 0 || h == windows.InvalidHandle {
		return false
	}
	var mode uint32
	if err = windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	const enableVT = windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if mode&enableVT != 0 {
		return true
	}
	// Enable it and **verify with a second read**: on a console that does not
	// support the bit, SetConsoleMode can succeed and simply do nothing.
	//
	// Only the VT bit is OR-ed onto the output console mode, no other bit is
	// cleared, and only if it was not already set. AGE_LANSERVER_OUTPUT=ascii
	// avoids even this.
	if err = windows.SetConsoleMode(h, mode|enableVT); err != nil {
		return false
	}
	if err = windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	return mode&enableVT != 0
}

// outputCode returns the console output code page, or 0 when there is no console.
//
// It reads the value out of the return register, which is what
// windows.GetConsoleOutputCP does: the code page comes back in the register and
// not through the out parameter the documented signature shows. That looks wrong
// enough to be "corrected" one day, so it is asserted against a real console in
// TestOutputCodeTracksTheConsoleCodePage.
func outputCode() uint32 {
	cp, err := windows.GetConsoleOutputCP()
	if err != nil {
		return 0
	}
	return cp
}

// consoleColumns returns the console width, or 0 when it cannot be determined.
func consoleColumns() int {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || h == 0 || h == windows.InvalidHandle {
		return 0
	}
	var info windows.ConsoleScreenBufferInfo
	if err = windows.GetConsoleScreenBufferInfo(h, &info); err != nil {
		return 0
	}
	return int(info.Window.Right - info.Window.Left + 1)
}

// localeUTF8 is meaningless on Windows: the code page and the console font decide
// what renders, not the locale.
func localeUTF8() bool { return false }

// osVersion returns the Windows NT major version and build number.
func osVersion() (major, build uint32) {
	major, _, build = windows.RtlGetNtVersionNumbers()
	return major, build
}
