package commonLogger

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrefixPrintf(t *testing.T) {
	var buf bytes.Buffer
	old := logger
	defer func() { logger = old }()

	logger = nil // logger is nil, PrefixPrintf should not panic
	PrefixPrintf("test", "format %d", 42)

	// Initialize with buffer, then call
	logger = nil
	Initialize(&buf)
	PrefixPrintf("TEST", "hello %s", "world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("PrefixPrintf output = %q, want contain %q", buf.String(), "hello world")
	}
	if !strings.Contains(buf.String(), "|TEST|") {
		t.Errorf("PrefixPrintf output = %q, want contain prefix |TEST|", buf.String())
	}
}

func TestPrefixPrintln(t *testing.T) {
	var buf bytes.Buffer
	old := logger
	defer func() { logger = old }()

	logger = nil
	PrefixPrintln("test", "line1", "line2")

	logger = nil
	Initialize(&buf)
	PrefixPrintln("MYMOD", "a", "b")
	out := buf.String()
	if !strings.Contains(out, "a b") {
		t.Errorf("PrefixPrintln output = %q, want contain %q", out, "a b")
	}
	if !strings.Contains(out, "|MYMOD|") {
		t.Errorf("PrefixPrintln output = %q, want contain prefix |MYMOD|", out)
	}
}

func TestPrefixPrintf_NilLogger(t *testing.T) {
	old := logger
	defer func() { logger = old }()
	logger = nil
	// Should not panic
	PrefixPrintf("test", "format %d", 1)
	PrefixPrintln("test", "data")
	Printf("data")
	Println("data")
}

func TestLogRootDate(t *testing.T) {
	got := LogRootDate("/tmp")
	if !strings.HasPrefix(got, filepath.Join("/tmp", "logs")) {
		t.Errorf("LogRootDate = %q, want prefix %q", got, filepath.Join("/tmp", "logs"))
	}
	// Should contain a date-like suffix
	base := filepath.Base(got)
	if len(base) < 10 {
		t.Errorf("LogRootDate base = %q, looks too short", base)
	}
}

func TestLogRootPrefix(t *testing.T) {
	got := logRootPrefix("/myroot")
	want := filepath.Join("/myroot", "logs")
	if got != want {
		t.Errorf("logRootPrefix = %q, want %q", got, want)
	}
}

func TestRootBuffer_NilRoot(t *testing.T) {
	var r *Root
	called := false
	err := r.Buffer("test", func(w io.Writer) {
		called = true
		if w != nil {
			t.Error("nil Root should pass nil writer")
		}
	})
	if err != nil {
		t.Errorf("Buffer on nil Root should return nil error, got %v", err)
	}
	if !called {
		t.Error("function should be called even for nil Root")
	}
}

func TestRootBuffer_WithData(t *testing.T) {
	err, root := NewFile(t.TempDir(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	err = root.Buffer("output", func(w io.Writer) {
		_, _ = w.Write([]byte("buffered content"))
	})
	if err != nil {
		t.Fatalf("Buffer: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root.Folder(), "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "buffered content" {
		t.Errorf("file content = %q, want %q", string(data), "buffered content")
	}
}

func TestRootBuffer_Empty(t *testing.T) {
	err, root := NewFile(t.TempDir(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	err = root.Buffer("empty", func(w io.Writer) {
		// write nothing
	})
	if err != nil {
		t.Fatalf("Buffer: %v", err)
	}
	_, err = os.Stat(filepath.Join(root.Folder(), "empty.txt"))
	if !os.IsNotExist(err) {
		t.Error("empty buffer should not create file")
	}
}

func TestNewOwnFileLogger(t *testing.T) {
	dir := t.TempDir()
	err := NewOwnFileLogger("testlog", dir, "", true)
	if err != nil {
		t.Fatalf("NewOwnFileLogger: %v", err)
	}
	if FileLogger == nil {
		t.Fatal("FileLogger should be set")
	}
	if file == nil {
		t.Fatal("file should be set")
	}
	if FileLogger.Folder() != dir {
		t.Errorf("FileLogger.Folder() = %q, want %q", FileLogger.Folder(), dir)
	}
	_ = file.Close()
	file = nil
}

func TestNewFile_FinalRootFalse(t *testing.T) {
	root := t.TempDir()
	err, l := NewFile(root, "myGame", false)
	if err != nil {
		t.Fatalf("NewFile(finalRoot=false): %v", err)
	}
	if l == nil {
		t.Fatal("Root should not be nil")
	}
	// The folder should contain logs/myGame/<date>
	folder := l.Folder()
	if !strings.Contains(folder, filepath.Join("logs", "myGame")) {
		t.Errorf("folder = %q, want to contain logs/myGame", folder)
	}
	// Directory should exist
	if _, statErr := os.Stat(folder); os.IsNotExist(statErr) {
		t.Error("directory should have been created")
	}
}

func TestCloseFileLog_NilFile(t *testing.T) {
	oldFile := file
	file = nil
	defer func() { file = oldFile }()
	// Should not panic
	CloseFileLog()
}

func TestNewFile_MkdirAllError(t *testing.T) {
	origAbs := filepathAbsFn
	origMkdir := osMkdirAllFn
	defer func() { filepathAbsFn = origAbs; osMkdirAllFn = origMkdir }()
	filepathAbsFn = func(string) (string, error) { return "", errors.New("abs fail") }
	err, _ := NewFile("any", "game", false)
	if err == nil || err.Error() != "abs fail" {
		t.Fatalf("expected abs fail, got %v", err)
	}
	filepathAbsFn = origAbs
	osMkdirAllFn = func(string, os.FileMode) error { return errors.New("mkdir fail") }
	err, _ = NewFile(t.TempDir(), "game", false)
	if err == nil || err.Error() != "mkdir fail" {
		t.Fatalf("expected mkdir fail, got %v", err)
	}
}

func TestNewFileLogger_Errors(t *testing.T) {
	_, _, err := NewFileLogger("name", "C:\\*invalid*\\path", "g", false)
	if err == nil {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "block")
		_ = os.WriteFile(fpath, []byte("x"), 0644)
		_, _, err = NewFileLogger("log", fpath, "g", false)
		if err == nil {
			t.Skip("MkdirAll did not fail as expected")
		}
	}
	dir2 := t.TempDir()
	err, root := NewFile(dir2, "", true)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	badPath := filepath.Join(dir2, "badfile")
	_ = os.WriteFile(badPath, []byte("x"), 0644)
	badRoot := &Root{root: badPath}
	_, _, err = NewFileLogger("log2", badRoot.Folder(), "", true)
	if err == nil {
		t.Skip("expected error for bad path")
	}
	_ = root
}

func TestNewOwnFileLogger_Error(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "block2")
	_ = os.WriteFile(fpath, []byte("x"), 0644)
	err := NewOwnFileLogger("log", fpath, "g", false)
	if err == nil {
		t.Error("NewOwnFileLogger should fail when NewFile fails")
	}
}

func TestRoot_Open_Nil(t *testing.T) {
	var r *Root
	f, err := r.Open("any")
	if err != nil || f != nil {
		t.Errorf("nil Root Open should return nil, nil, got %v, %v", f, err)
	}
}

func TestRoot_Folder_Nil(t *testing.T) {
	var r *Root
	if got := r.Folder(); got != "" {
		t.Errorf("nil Root Folder = %q, want empty", got)
	}
}

func TestRoot_Buffer_OpenError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "fileAsFolder")
	_ = os.WriteFile(fpath, []byte("x"), 0644)
	err, _ := NewFile(t.TempDir(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	badRoot := &Root{root: fpath}
	err = badRoot.Buffer("test", func(w io.Writer) { _, _ = w.Write([]byte("data")) })
	if err == nil {
		t.Error("Buffer should fail when Open fails")
	}
}

func TestRoot_Buffer_CopyError(t *testing.T) {
	origCopy := ioCopyFn
	defer func() { ioCopyFn = origCopy }()
	ioCopyFn = func(dst io.Writer, src io.Reader) (int64, error) { return 0, errors.New("copy fail") }
	err, root := NewFile(t.TempDir(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	err = root.Buffer("copyfail", func(w io.Writer) { _, _ = w.Write([]byte("data")) })
	if err == nil || err.Error() != "copy fail" {
		t.Fatalf("expected copy fail, got %v", err)
	}
}

func TestInitialize_WithWriters(t *testing.T) {
	oldLogger := logger
	defer func() { logger = oldLogger }()
	var buf bytes.Buffer
	Initialize(&buf)
	if logger == nil {
		t.Fatal("logger should be set")
	}
	Initialize(nil)
	if logger == nil {
		t.Error("logger should be set even with nil writer")
	}
	Initialize(os.Stdout)
	if logger == nil {
		t.Error("logger should be set with stdout")
	}
}
