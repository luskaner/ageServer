package executables

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWindowsFileName(t *testing.T) {
	if got := WindowsFileName("server"); got != "server.exe" {
		t.Errorf("WindowsFileName(%q) = %q, want %q", "server", got, "server.exe")
	}
}

func TestUnixFileName(t *testing.T) {
	if got := UnixFileName("server"); got != "server" {
		t.Errorf("UnixFileName(%q) = %q, want %q", "server", got, "server")
	}
}

func TestBaseNameNoExt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"server.exe", "server"},
		{"agent.exe", "agent"},
		{"noext", "noext"},
		{"path/to/file.bin", "path/to/file"},
		{".hidden", ""},
		{"file.tar.gz", "file.tar"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := BaseNameNoExt(tt.input); got != tt.want {
				t.Errorf("BaseNameNoExt(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNativeFileName(t *testing.T) {
	got := NativeFileName(true, "server")
	// On Windows it should be "server.exe", on Unix "server"
	if runtime.GOOS == "windows" {
		if got != "server.exe" {
			t.Errorf("NativeFileName(true, %q) = %q, want %q", "server", got, "server.exe")
		}
	} else {
		if got != "server" {
			t.Errorf("NativeFileName(true, %q) = %q, want %q", "server", got, "server")
		}
	}
}

func TestFileName_Bin(t *testing.T) {
	// With custom transfileName
	tr := func(name string) string { return name + ".custom" }
	got := FileName(true, "myapp", tr)
	if got != "myapp.custom" {
		t.Errorf("FileName(true, %q, tr) = %q, want %q", "myapp", got, "myapp.custom")
	}
}

func TestFileName_NotBin(t *testing.T) {
	got := FileName(false, "server", nil)
	want := filepath.Join("bin", fileName("server"))
	if got != want {
		t.Errorf("FileName(false, %q, nil) = %q, want %q", "server", got, want)
	}
}

func TestArchFileName(t *testing.T) {
	got := ArchFileName(true, "server", nil)
	// Should contain runtime.GOARCH
	expected := fileName("server_" + runtime.GOARCH)
	if got != expected {
		t.Errorf("ArchFileName(true, %q, nil) = %q, want %q", "server", got, expected)
	}
}

func TestFindPath_NotFound(t *testing.T) {
	result := FindPath("this-executable-definitely-does-not-exist-12345.exe")
	if result != "" {
		t.Errorf("FindPath for nonexistent exe = %q, want empty", result)
	}
}

func TestFindPath_SelfFound(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot resolve executable")
	}
	// The test binary is in a temp dir; the directories loop checks the
	// executable's own directory and its parents. With our test binary it
	// may not find itself, so just verify it doesn't panic and returns a string.
	_ = FindPath(filepath.Base(exe))
}

func TestFindPath_OsExecutableError(t *testing.T) {
	orig := osExecutableFn
	osExecutableFn = func() (string, error) { return "", os.ErrInvalid }
	defer func() { osExecutableFn = orig }()
	if got := FindPath("foo.exe"); got != "" {
		t.Errorf("FindPath with os.Executable error = %q, want empty", got)
	}
}

type mockFileInfo struct{ isDir bool }

func (m mockFileInfo) Name() string       { return "mock" }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestFindPath_Found(t *testing.T) {
	origExec := osExecutableFn
	origStat := osStatFn
	osExecutableFn = func() (string, error) { return `C:\fake\path\app.exe`, nil }
	osStatFn = func(name string) (os.FileInfo, error) {
		// First entry: C:\fake\path\foo\foo.exe → pretend found
		return mockFileInfo{isDir: false}, nil
	}
	defer func() { osExecutableFn = origExec; osStatFn = origStat }()
	got := FindPath("foo.exe")
	if got == "" {
		t.Error("FindPath should find mocked file")
	}
	if filepath.Base(got) != "foo.exe" {
		t.Errorf("FindPath = %q, want base foo.exe", got)
	}
}

func TestFindPath_IsDirSkipped(t *testing.T) {
	origExec := osExecutableFn
	origStat := osStatFn
	calls := 0
	osExecutableFn = func() (string, error) { return `C:\fake\path\app.exe`, nil }
	osStatFn = func(name string) (os.FileInfo, error) {
		calls++
		if calls == 1 {
			return mockFileInfo{isDir: true}, nil // dir → skip
		}
		return mockFileInfo{isDir: false}, nil // found on second try
	}
	defer func() { osExecutableFn = origExec; osStatFn = origStat }()
	got := FindPath("foo.exe")
	if got == "" {
		t.Error("FindPath should skip dir and find next")
	}
}
