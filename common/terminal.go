package common

import (
	"os"

	"golang.org/x/term"
)

// TerminalInfo abstracts terminal detection for testability.
type TerminalInfo interface {
	IsTerminal(fd int) bool
	StdinFd() uintptr
	StdoutFd() uintptr
}

type defaultTerminalInfo struct{}

func (d *defaultTerminalInfo) IsTerminal(fd int) bool { return term.IsTerminal(fd) }
func (d *defaultTerminalInfo) StdinFd() uintptr       { return os.Stdin.Fd() }
func (d *defaultTerminalInfo) StdoutFd() uintptr      { return os.Stdout.Fd() }

var terminal TerminalInfo = &defaultTerminalInfo{}

// SetTerminal replaces the terminal info provider for tests.
func SetTerminal(t TerminalInfo) (restore func()) {
	orig := terminal
	terminal = t
	return func() { terminal = orig }
}

func Interactive() bool {
	return terminal.IsTerminal(int(terminal.StdinFd())) &&
		terminal.IsTerminal(int(terminal.StdoutFd()))
}
