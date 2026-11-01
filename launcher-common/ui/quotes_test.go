package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A value the renderer already marks does not need quotes around it.
//
// The renderer styles component names, paths, addresses and flags on its own, so
// quotes are a second, uglier way of saying the same thing, and they are the way
// that survives into the file log where there is no styling at all. They were
// removed from every message in the four modules; this test is what keeps them out,
// in this module and in every module that comes later.
//
// It is deliberately narrow: it only flags a quoted token that the styler would
// mark. Quoting something the styler ignores, like a whole command line, is the
// writer's choice and is left alone.
func TestNoQuotesAroundValuesTheRendererAlreadyMarks(t *testing.T) {
	root := repoRoot(t)
	for _, module := range styledModules {
		module := module
		t.Run(module, func(t *testing.T) {

			entries, err := os.ReadDir(filepath.Join(root, module))
			if err != nil {
				t.Skipf("%s is not in the workspace: %v", module, err)
			}
			for _, entry := range entries {
				if entry.IsDir() {
					walkGoFiles(t, filepath.Join(root, module, entry.Name()))
				}
			}
		})
	}
}

// quotedValues returns the quoted tokens of a string literal, which is what the
// guard looks at.
func quotedValues(literal string) []string {
	var out []string
	for i := 0; i < len(literal); i++ {
		switch literal[i] {
		case '\'', '"':
			quote := literal[i]
			end := strings.IndexByte(literal[i+1:], quote)
			if end < 0 {
				return out
			}
			value := literal[i+1 : i+1+end]
			if value != "" {
				out = append(out, value)
			}
			i += end + 1
		}
	}
	return out
}

// markedByTheStyler reports whether the inline styler would already mark a token,
// which is what makes the quotes around it redundant.
//
// It asks the styler itself rather than keeping a second list of names: a guard
// that duplicates the rule it is guarding stops guarding anything the day the rule
// changes without the guard.
func markedByTheStyler(value string) bool {
	if matchName(value, 0, componentNames) == len(value) {
		return true
	}
	if matchURL(value, 0) == len(value) || matchIPv4(value, 0) == len(value) {
		return true
	}
	if matchFlag(value, 0) == len(value) {
		return true
	}
	return len(value) > 2 && strings.ContainsAny(value, `\/`) && matchPath(value, 0) == len(value)
}

// checkNoRedundantQuotes parses a file and fails on every string literal holding a
// quoted token the styler would already mark.
func checkNoRedundantQuotes(t *testing.T, path string) {
	t.Helper()
	// The file walker is shared with the glyph guard, which does cover the two
	// agent modules; this rule does not, because they print unstyled text.
	if !underStyledModule(path) {
		return
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Skipf("cannot parse %s: %v", path, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		position := set.Position(literal.Pos())
		for _, quoted := range quotedValues(value) {
			if _, allowed := allowedQuotedValues[quoted]; allowed {
				continue
			}
			if markedByTheStyler(quoted) {
				t.Errorf("%s:%d: %q is marked by the renderer already, the quotes are redundant: %s",
					filepath.Base(path), position.Line, quoted, value)
			}
		}
		return true
	})
}

// styledModules are the modules whose console output goes through this package.
//
// The two helpers that are not in it are there for a reason rather than by
// omission: config-helper writes a converted path to stdout for its parent to read,
// so anything decorative on that line is corruption of the contract, and
// battle-server-broadcast is a library that prints nothing.
var styledModules = []string{
	"launcher",
	"launcher-config",
	"launcher-config-admin",
	"launcher-agent",
	"launcher-config-admin-agent",
	"battle-server-manager",
	"server",
	"server-genCert",
}

// allowedQuotedValues are the quotes that stay, each with the reason it is not
// decoration. An entry here is a decision someone had to think about, so the reason
// is part of it.
var allowedQuotedValues = map[string]string{
	"server": "the zenity dialog is a graphical window with no styling, so the quotes are " +
		"the only thing telling the reader it is a program name",
}

// underStyledModule reports whether a path belongs to a module whose output goes
// through this package.
func underStyledModule(path string) bool {
	normalised := filepath.ToSlash(path)
	for _, module := range styledModules {
		if strings.Contains(normalised, "/"+module+"/") {
			return true
		}
	}
	return false
}
