package cmd

import (
	"os"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/spf13/pflag"
)

func TestRootFlagSet_ExecuteWithArgs_Help(t *testing.T) {
	r := NewRootFlagSet()
	r.RegisterCommand("test", func(args []string) (error, int) {
		t.Fatal("command should not be called with --help")
		return nil, 0
	})
	err, code := r.ExecuteWithArgs("1.0", []string{"--help"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRootFlagSet_ExecuteWithArgs_Version(t *testing.T) {
	r := NewRootFlagSet()
	r.RegisterCommand("test", func(args []string) (error, int) {
		t.Fatal("command should not be called with --version")
		return nil, 0
	})
	err, code := r.ExecuteWithArgs("1.2.3", []string{"--version"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRootFlagSet_ExecuteWithArgs_UnknownCommand(t *testing.T) {
	r := NewRootFlagSet()
	err, code := r.ExecuteWithArgs("1.0", []string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if code != common.ErrSyntax {
		t.Errorf("exit code = %d, want %d", code, common.ErrSyntax)
	}
}

func TestRootFlagSet_ExecuteWithArgs_DispatchesCommand(t *testing.T) {
	r := NewRootFlagSet()
	var receivedArgs []string
	r.RegisterCommand("mycommand", func(args []string) (error, int) {
		receivedArgs = args
		return nil, 0
	})
	err, code := r.ExecuteWithArgs("1.0", []string{"mycommand", "arg1", "arg2"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if len(receivedArgs) != 2 || receivedArgs[0] != "arg1" || receivedArgs[1] != "arg2" {
		t.Errorf("command received args = %v, want [arg1 arg2]", receivedArgs)
	}
}

func TestRootFlagSet_ExecuteWithArgs_NoCommandsShowsHelp(t *testing.T) {
	r := NewRootFlagSet()
	err, code := r.ExecuteWithArgs("1.0", []string{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestSingleFlagSet_ExecuteWithArgs_Help(t *testing.T) {
	s := NewSingleFlagSet(func(fs *pflag.FlagSet) (error, int) {
		t.Fatal("command should not be called with --help")
		return nil, 0
	}, "1.0")
	err, code := s.ExecuteWithArgs([]string{"--help"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestSingleFlagSet_ExecuteWithArgs_Version(t *testing.T) {
	s := NewSingleFlagSet(func(fs *pflag.FlagSet) (error, int) {
		return nil, 0
	}, "2.0")
	err, code := s.ExecuteWithArgs([]string{"--version"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestSingleFlagSet_ExecuteWithArgs_DispatchesCommand(t *testing.T) {
	var called bool
	s := NewSingleFlagSet(func(fs *pflag.FlagSet) (error, int) {
		called = true
		return nil, 0
	}, "1.0")
	err, code := s.ExecuteWithArgs([]string{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !called {
		t.Error("command was not called")
	}
}

func TestSingleFlagSet_Fs(t *testing.T) {
	s := NewSingleFlagSet(func(fs *pflag.FlagSet) (error, int) {
		return nil, 0
	}, "1.0")
	fs := s.Fs()
	if fs == nil {
		t.Fatal("Fs() should not return nil")
	}
}

func TestRootFlagSet_ExecuteWithArgs_ParseError(t *testing.T) {
	r := NewRootFlagSet()
	r.RegisterCommand("test", func(args []string) (error, int) {
		t.Fatal("command should not be called on parse error")
		return nil, 0
	})
	err, code := r.ExecuteWithArgs("1.0", []string{"--bogus-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if code != common.ErrSyntax {
		t.Errorf("exit code = %d, want %d", code, common.ErrSyntax)
	}
}

func TestSingleFlagSet_ExecuteWithArgs_ParseError(t *testing.T) {
	s := NewSingleFlagSet(func(fs *pflag.FlagSet) (error, int) {
		t.Fatal("command should not be called on parse error")
		return nil, 0
	}, "1.0")
	err, code := s.ExecuteWithArgs([]string{"--bogus-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if code != common.ErrSyntax {
		t.Errorf("exit code = %d, want %d", code, common.ErrSyntax)
	}
}

func TestRootFlagSet_Execute(t *testing.T) {
	r := NewRootFlagSet()
	r.RegisterCommand("test", func(args []string) (error, int) {
		return nil, 0
	})
	origArgs := os.Args
	os.Args = []string{"test", "test"}
	defer func() { os.Args = origArgs }()
	err, code := r.Execute("1.0")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestSingleFlagSet_Execute(t *testing.T) {
	var called bool
	s := NewSingleFlagSet(func(fs *pflag.FlagSet) (error, int) {
		called = true
		return nil, 0
	}, "1.0")
	origArgs := os.Args
	os.Args = []string{"test"}
	defer func() { os.Args = origArgs }()
	err, code := s.Execute()
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !called {
		t.Error("command was not called")
	}
}
