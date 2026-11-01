package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// message pairs the message as it was written before any of this with the
// renderer that replaced it. Every entry is the TierASCII guarantee stated as a
// test: the renderer must add decoration and nothing else.
type message struct {
	// today is the wording as it stood before the styling, quotes and all. It is
	// the historical baseline, so dropping the quotes around a program name does
	// not silently become the new baseline the next change is measured against.
	today string
	// tmpl is the message template the renderer is handed, together with args.
	tmpl string
	// render is the ui function under test.
	render func(string, ...any) string
	// args are the arguments the message is actually built with.
	args []any
}

// catalogue is every canonical user facing message of the four modules in scope.
// Adding a message to the code without adding it here is the way parity silently
// rots, so the grepless tests below are the point of the file.
func catalogue() []message {
	return []message{
		// launcher/internal/cmd/root.go
		{today: "Failed to lock pid file. Kill process 'launcher' if it is running in your task manager.", tmpl: Fail("Failed to lock pid file. Kill process launcher if it is running in your task manager."), render: Fail},
		{today: "Internet usage is disabled via config.", tmpl: Info("Internet usage is disabled via config."), render: Info},
		{today: "No internet connectivity, some features will fallback gracefully.", tmpl: Warn("No internet connectivity, some features will fallback gracefully."), render: Warn},
		{today: "Invalid game type", tmpl: Fail("Invalid game type"), render: Fail},
		{today: "Failed to parse 'server' executable arguments", tmpl: Fail("Failed to parse server executable arguments"), render: Fail},
		{today: "Invalid isolation path", tmpl: Fail("Invalid isolation path"), render: Fail},
		{today: "Invalid 'server' executable", tmpl: Fail("Invalid server executable"), render: Fail},
		{today: "Isolating profiles and metadata is a must when using an official 'launcher'.", tmpl: Fail("Isolating profiles and metadata is a must when using an official launcher."), render: Fail},
		{today: "Running as administrator, this is not recommended for security reasons. It will request isolated admin privileges if/when needed.", tmpl: Warn("Running as administrator, this is not recommended for security reasons. It will request isolated admin privileges if/when needed."), render: Warn},
		{today: "Looking for the game...", tmpl: Step("Looking for the game..."), render: Step},
		{today: "Game found on steam.exe.", tmpl: Ok("Game found on steam.exe."), render: Ok},
		{today: "Game not found.", tmpl: Fail("Game not found."), render: Fail},
		{today: "'agent' did not exit on its own.", tmpl: Warn("agent did not exit on its own."), render: Warn},
		{today: "'config-admin-agent' from a previous run is still active, stopping it...", tmpl: Step("config-admin-agent from a previous run is still active, stopping it..."), render: Step},
		{today: "'config-admin-agent' stopped.", tmpl: Ok("config-admin-agent stopped."), render: Ok},
		{today: "Failed to stop 'config-admin-agent'.", tmpl: Fail("Failed to stop config-admin-agent."), render: Fail},
		{today: "Cleaning up (if needed)...", tmpl: Step("Cleaning up (if needed)..."), render: Step},
		{today: "Setting up...", tmpl: Step("Setting up..."), render: Step},
		{today: "Canceled starting the 'server'.", tmpl: Fail("Canceled starting the server."), render: Fail},
		{today: "Cannot find 'server' executable path. Set it manually in Server.Executable.", tmpl: Fail("Cannot find server executable path. Set it manually in Server.Executable."), render: Fail},
		{today: "Failed to read certificate from 192.168.1.50. Try to access it with your browser and checking the certificate.", tmpl: Fail("Failed to read certificate from 192.168.1.50. Try to access it with your browser and checking the certificate."), render: Fail},
		// launcher-common/launcher/ops
		{today: "Failed to trust certificate", tmpl: Fail("Failed to trust certificate"), render: Fail},
		{today: "Saving 'server' certificate to 'C:\\Temp\\cert.pem' file...", tmpl: "Saving server certificate to 'C:\\Temp\\cert.pem' file...", render: Step},
		{today: "Failed to save certificate to file", tmpl: Fail("Failed to save certificate to file"), render: Fail},
		{today: "Error message: exit status 1", tmpl: "Error message: %s", render: Detail, args: []any{"exit status 1"}},
		{today: "Error message: none (check the 'config_setup_hosts' and 'config-admin_setup_hosts' log files for details).", tmpl: Detail("Error message: none (check the config_setup_hosts and config-admin_setup_hosts log files for details)."), render: Detail},
		{today: "Failed to kill it: access is denied, try using the task manager.", tmpl: "Failed to kill it: %s, try using the task manager.", render: Warn, args: []any{"access is denied"}},
		// launcher-config/internal/cmd/setUp.go and revert.go
		{today: "Successfully added user certificate", tmpl: Ok("Successfully added user certificate"), render: Ok},
		{today: "Failed to add user certificate", tmpl: Fail("Failed to add user certificate"), render: Fail},
		{today: "Successfully backed up metadata", tmpl: Ok("Successfully backed up metadata"), render: Ok},
		{today: "Failed to back up metadata", tmpl: Fail("Failed to back up metadata"), render: Fail},
		{today: "Successfully restored metadata", tmpl: Ok("Successfully restored metadata"), render: Ok},
		{today: "Failed to restore metadata", tmpl: Fail("Failed to restore metadata"), render: Fail},
		{today: "Successfully removed user certificate", tmpl: Ok("Successfully removed user certificate"), render: Ok},
		{today: "Failed to remove user certificate", tmpl: Fail("Failed to remove user certificate"), render: Fail},
		{today: "Successfully communicated with 'config-admin-agent'", tmpl: Ok("Successfully communicated with config-admin-agent"), render: Ok},
		{today: "Failed to communicate with 'config-admin-agent'", tmpl: Fail("Failed to communicate with config-admin-agent"), render: Fail},
		{today: "Received error:", tmpl: Detail("Received error:"), render: Detail},
		{today: "Received exit code:", tmpl: Detail("Received exit code:"), render: Detail},
		{today: "Failed to start 'config-admin-agent'", tmpl: Fail("Failed to start config-admin-agent"), render: Fail},
		// launcher-config/internal/cacert.go and userData
		{today: "Copying data from original to backup", tmpl: "Copying data from %s to %s", render: Detail, args: []any{"original", "backup"}},
		{today: "Switching C:\\a <-> C:\\b", tmpl: "Switching %s <-> %s", render: Detail, args: []any{"C:\\a", "C:\\b"}},
		{today: "Creating all path hierarchy: C:\\a", tmpl: "Creating all path hierarchy: %s", render: Detail, args: []any{"C:\\a"}},
		// launcher-config-admin
		{today: "Trying to stop 'config-admin-agent'.", tmpl: Step("Trying to stop config-admin-agent."), render: Step},
		{today: "Stopped 'config-admin-agent'", tmpl: Ok("Stopped config-admin-agent"), render: Ok},
		{today: "Failed to stop 'config-admin-agent'", tmpl: Fail("Failed to stop config-admin-agent"), render: Fail},
		{today: "Successfully killed 'config-admin-agent'.", tmpl: Ok("Successfully killed config-admin-agent."), render: Ok},
	}
}

// decoratorTokens is the closed set of tokens decoration is allowed to add. A
// token outside this list means somebody reworded a message and did not think
// about the Windows 7 output.
//
// The bracketed markers contribute their word and their two brackets, since the
// tokenizer drops punctuation that carries no letters of its own.
var decoratorTokens = map[string]bool{
	"[": true, "]": true,
	"OK": true, "FAIL": true, "WARN": true, "INFO": true, "WAIT": true, "?": true,
}

// tokenize splits a message into its information carrying tokens: runs of
// letters, digits and the punctuation a value may be made of.
//
// Punctuation at the edge of a token is not information, and where it lands
// depends on the quoting: 'server'. contributes "server" plus a full stop while
// server. contributes one token. Both carry the same words, so the edge
// punctuation is trimmed and a token left with no letter or digit is dropped.
func tokenize(s string) map[string]int {
	out := map[string]int{}
	var word strings.Builder
	flush := func() {
		token := strings.Trim(word.String(), ".,:")
		word.Reset()
		if token == "" || !hasAlnum(token) {
			return
		}
		out[token]++
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == ',', r == ':', r == '_', r == '/', r == '\\',
			r == '-', r == '=', r == '%':
			word.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

func hasAlnum(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// stripDecorators removes the marker and the indentation the renderer added, so
// the rendered output can be compared against the wording as it stands today.
func stripDecorators(rendered string) string {
	var out []string
	for _, line := range strings.Split(rendered, "\n") {
		for _, token := range []string{":-)", ":-(", ":-/", ":-|", ":-?"} {
			line = strings.ReplaceAll(line, token, "")
		}
		line = strings.TrimLeft(line, " ")
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// assertParity enforces P3 and P4: every information token of the original
// message survives, in the same order, and nothing new appears uninvited.
func assertParity(t *testing.T, original, rendered string) {
	t.Helper()
	plain := stripDecorators(rendered)
	origTokens := tokenize(original)
	newTokens := tokenize(plain)
	for tok, n := range origTokens {
		if newTokens[tok] < n {
			t.Fatalf("parity broken: %q x%d missing from\n  original: %q\n  tmpl: %q", tok, n, original, plain)
		}
	}
	for tok, n := range newTokens {
		if _, ok := origTokens[tok]; !ok && !decoratorTokens[tok] {
			t.Fatalf("parity broken: new token %q x%d in\n  original: %q\n  tmpl: %q", tok, n, original, plain)
		}
	}
	if !isSubsequence(tokenSequence(plain), tokenSequence(original)) {
		t.Fatalf("parity broken: token order changed\n  original: %q\n  tmpl: %q", original, plain)
	}
}

// tokenSequence is the token list of a message in reading order, with the same
// edge punctuation rules as tokenize.
func tokenSequence(s string) []string {
	var out []string
	var word strings.Builder
	flush := func() {
		token := strings.Trim(word.String(), ".,:")
		word.Reset()
		if token == "" || !hasAlnum(token) {
			return
		}
		out = append(out, token)
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == ',', r == ':', r == '_', r == '/', r == '\\',
			r == '-', r == '=', r == '%':
			word.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// isSubsequence reports whether want can be read out of got in order, which is
// what P4 means: no token moved and none disappeared.
func isSubsequence(got, want []string) bool {
	i := 0
	for _, tok := range got {
		if i < len(want) && tok == want[i] {
			i++
		}
	}
	return i == len(want)
}

// TestParityOfTheMessageCatalogue is the test that materialises the requirement
// for an old terminal: no message may lose a word on its way through the
// renderer.
func TestParityOfTheMessageCatalogue(t *testing.T) {
	with(t, at(colorprofile.NoTTY, TierASCII, 0))
	for _, m := range catalogue() {
		t.Run(m.today, func(t *testing.T) {
			assertParity(t, m.today, m.render(m.tmpl, m.args...))
		})
	}
}

// P1, P2 and P5 for the whole catalogue: in TierASCII the output is printable
// ASCII with no escape sequence, and wrapping a message never makes it shorter.
func TestTierASCIIOutputShape(t *testing.T) {
	for _, columns := range []int{0, 40, 80, 200} {
		with(t, at(colorprofile.NoTTY, TierASCII, columns))
		for _, m := range catalogue() {
			got := m.render(m.tmpl, m.args...)
			if !AllASCII(got) {
				t.Errorf("columns %d: %q is not printable ASCII", columns, got)
			}
			if ContainsANSI(got) {
				t.Errorf("columns %d: %q carries an escape sequence", columns, got)
			}
			if columns > 0 {
				want := strings.Count(m.today, "\n") + 1
				if got := strings.Count(got, "\n") + 1; got < want {
					t.Errorf("columns %d: %d lines, want at least %d", columns, got, want)
				}
				for _, line := range strings.Split(got, "\n") {
					if w := lineWidth(line); w > columns {
						t.Errorf("columns %d: a %d cell line does not fit: %q", columns, w, line)
					}
				}
			}
		}
	}
}

// The catalogue must actually be exercised: a renderer that dropped every entry
// would otherwise pass the two tests above.
func TestCatalogueIsNotEmpty(t *testing.T) {
	if len(catalogue()) < 40 {
		t.Fatalf("the catalogue has %d entries, it is meant to cover every canonical message", len(catalogue()))
	}
}
