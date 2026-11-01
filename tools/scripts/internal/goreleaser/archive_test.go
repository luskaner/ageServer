package goreleaser

import (
	"testing"
)

// Regression: NewMergedArchive indexed archives[0] unconditionally and
// panicked when invoked without archives.
func TestNewMergedArchiveWithoutArchivesDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	a := NewMergedArchive("full", nil)
	if a == nil {
		t.Fatal("expected a usable empty archive")
	}
	if builds := a.Builds(); len(builds) != 0 {
		t.Fatalf("builds = %d, want 0", len(builds))
	}
	if archives := a.Archives(nil); len(archives) != 0 {
		t.Fatalf("archives = %d, want 0", len(archives))
	}
}

func TestNewFileDataPerGoos(t *testing.T) {
	cases := []struct {
		goos   string
		base   string
		srcExt string
		dstExt string
		dstDoc string
	}{
		{"windows", "windows", "bat", "bat", "txt"},
		{"linux", "unix", "sh", "sh", ""},
		{"darwin", "unix", "sh", "command", ""},
		{"solaris", "", "", "", ""},
	}
	for _, tc := range cases {
		f := NewFileData(tc.goos)
		if f.BaseOS != tc.base || f.SrcScriptExt != tc.srcExt || f.DstScriptExt != tc.dstExt || f.DstDocExt != tc.dstDoc {
			t.Errorf("%s: %+v", tc.goos, f)
		}
	}
}

func TestDefaultDestStripsFirstSegment(t *testing.T) {
	if got := defaultDest("server/resources/config"); got != "resources/config" {
		t.Errorf("got %q", got)
	}
	if got := defaultDest("LICENSE"); got != "LICENSE" {
		t.Errorf("no-slash passthrough failed: %q", got)
	}
}

func TestAddSrcOsDstFileSkipsIgnoredSources(t *testing.T) {
	targets := NewBinaryTargets()
	targets.AddTarget(OSLinux, ArchAmd64)
	a := NewArchive("srv", targets, nil)

	ignored := SourceIgnoreFn{
		"linux": func(path string) bool { return path == "skip-me" },
	}
	a.AddSrcOsDstFile(
		LiteralString[FileData]("skip-me"),
		ignored,
		nil,
		nil,
		0744,
		false,
	)
	a.AddSrcOsDstFile(
		LiteralString[FileData]("keep-me"),
		ignored,
		nil,
		nil,
		0744,
		false,
	)

	for file := range a.files.Iter() {
		if file.source == "skip-me" {
			t.Fatal("ignored source was added")
		}
	}
	found := false
	for file := range a.files.Iter() {
		if file.source == "keep-me" && file.mode == 0744 {
			found = true
		}
	}
	if !found {
		t.Fatal("non-ignored source missing or mode lost (mode applies to unix targets)")
	}
}

func TestRemoveFiles(t *testing.T) {
	targets := NewBinaryTargets()
	targets.AddTarget(OSLinux, ArchAmd64)
	a := NewArchive("srv", targets, nil)
	a.AddSrcDstFile("a.txt", "a.txt")
	a.AddSrcDstFile("b.txt", "b.txt")

	a.RemoveFiles("a.txt")

	for file := range a.files.Iter() {
		if file.source == "a.txt" {
			t.Fatal("file not removed")
		}
	}
}
