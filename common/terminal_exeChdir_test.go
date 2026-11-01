package common

import (
	"os"
	"testing"
)

type mockTerminal struct {
	isTerminal bool
}

func (m *mockTerminal) IsTerminal(fd int) bool { return m.isTerminal }
func (m *mockTerminal) StdinFd() uintptr       { return 1 }
func (m *mockTerminal) StdoutFd() uintptr      { return 2 }

func TestInteractiveBothTerminals(t *testing.T) {
	defer SetTerminal(&mockTerminal{isTerminal: true})()
	if !Interactive() {
		t.Fatal("expected true when both stdin and stdout are terminals")
	}
}

func TestInteractiveNeitherTerminal(t *testing.T) {
	defer SetTerminal(&mockTerminal{isTerminal: false})()
	if Interactive() {
		t.Fatal("expected false when neither is a terminal")
	}
}

func TestChdirToExeMock(t *testing.T) {
	var chdirTo string
	defer SetProcessInfo(&mockProcess{
		executable: func() (string, error) {
			return "C:\\fake\\app.exe", nil
		},
		chdir: func(dir string) error {
			chdirTo = dir
			return nil
		},
	})()
	ChdirToExe()
	if chdirTo != "C:\\fake" {
		t.Fatalf("expected C:\\fake, got %q", chdirTo)
	}
}

func TestChdirToExeError(t *testing.T) {
	defer SetProcessInfo(&mockProcess{
		executable: func() (string, error) {
			return "", os.ErrNotExist
		},
	})()
	ChdirToExe() // should not panic
}

func TestDefaultTerminalInfo(t *testing.T) {
	d := &defaultTerminalInfo{}
	_ = d.IsTerminal(0)
	_ = d.StdinFd()
	_ = d.StdoutFd()
}

func TestDefaultProcessInfo(t *testing.T) {
	d := &defaultProcessInfo{}
	_, _ = d.Executable()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = d.Chdir(dir)
	_ = d.Chdir(orig)
}

type mockProcess struct {
	executable func() (string, error)
	chdir      func(dir string) error
}

func (m *mockProcess) Executable() (string, error) {
	if m.executable != nil {
		return m.executable()
	}
	return "", nil
}

func (m *mockProcess) Chdir(dir string) error {
	if m.chdir != nil {
		return m.chdir(dir)
	}
	return nil
}
