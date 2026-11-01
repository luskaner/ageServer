package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

// inScopeModules are the modules whose Go code may reference a glyph, and only
// through its name. Every other module is outside this package's scope and must
// not grow a dependency on launcher-common/ui.
var inScopeModules = []string{
	"launcher", "launcher-config", "launcher-config-admin", "launcher-agent",
	"launcher-config-admin-agent",
}

// repoRoot walks up from this file until it finds the go.work file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test source")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.work above %s", filepath.Dir(file))
		}
		dir = parent
	}
}

// TestNoLiteralGlyphsOutsideTheCatalogue enforces rule 4 of the catalogue: the
// name is the only thing allowed to appear in the calling code. A literal ✔, ✅
// or "[ ok ]" in a message would bypass the tier detection entirely and end up
// as a replacement box on a Windows 7 console.
func TestNoLiteralGlyphsOutsideTheCatalogue(t *testing.T) {
	root := repoRoot(t)
	for _, module := range inScopeModules {
		module := module
		t.Run(module, func(t *testing.T) {
			if module == "launcher-common" {
				t.Skip("the catalogue itself lives here")
			}
			entries, err := os.ReadDir(filepath.Join(root, module))
			if err != nil {
				t.Skipf("%s is not in the workspace: %v", module, err)
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				walkGoFiles(t, filepath.Join(root, module, entry.Name()))
			}
		})
	}
}

func walkGoFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if entry.Name() == "ui" || entry.Name() == "testdata" {
				continue
			}
			walkGoFiles(t, path)
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		checkNoLiteralGlyphs(t, path)
		checkNoRedundantQuotes(t, path)
	}
}

// checkNoLiteralGlyphs fails on any character outside ASCII that could be a
// rendered marker: box drawing, arrows, geometric shapes, dingbats and emoji.
//
// Only non-test files are scanned: what matters is what reaches the terminal,
// and a test is free to name the characters it asserts on.
func checkNoLiteralGlyphs(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	for lineNo, line := range strings.Split(string(content), "\n") {
		for _, r := range line {
			if r < 0x80 || unicode.IsLetter(r) && r < 0x2000 {
				continue
			}
			switch {
			case r == '\t':
			case isMarkerGlyph(r):
				t.Errorf("%s:%d: the literal %q must go through the glyph catalogue", path, lineNo+1, r)
			}
		}
	}
}

// isMarkerGlyph reports whether r is one of the characters the catalogue exists
// to keep out of the calling code.
func isMarkerGlyph(r rune) bool {
	return r == 0xFE0F ||
		(r >= 0x2190 && r <= 0x21FF) || // arrows
		(r >= 0x2500 && r <= 0x257F) || // box drawing
		(r >= 0x25A0 && r <= 0x25FF) || // geometric shapes
		(r >= 0x2600 && r <= 0x27BF) || // misc symbols and dingbats
		(r >= 0x2B00 && r <= 0x2BFF) || // misc symbols and arrows
		r >= 0x1F300 // emoji
}
