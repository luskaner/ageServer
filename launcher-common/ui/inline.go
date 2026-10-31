package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// spanKind is what a run of a message line refers to.
type spanKind uint8

const (
	spanPlain spanKind = iota
	spanComponent
	spanKey
	spanPath
	spanFlag
)

// style returns the style for a run. A component gets no colour of its own: it
// inherits the message's and only adds the chip behind it, so highlighting a name
// never changes what the line reads as.
func (k spanKind) style(base lipgloss.Style) lipgloss.Style {
	switch k {
	case spanComponent:
		return base.Background(stChip.GetBackground()).Bold(true)
	case spanKey:
		return stKey
	case spanPath:
		return stPath
	case spanFlag:
		return stFlag
	default:
		return base
	}
}

// styleLine renders one already laid out line, giving the spans it mentions their
// own styles.
//
// Runs are styled one at a time instead of nesting a style around an already
// styled string: a nested reset would end the outer colour half way through the
// line, which is the classic way to make a message look like a bug report.
func styleLine(base lipgloss.Style, line string) string {
	// Below ANSI every style is the zero style, so there is nothing to split and
	// the line goes out exactly as it came in.
	if !Current().ColorOk() {
		return base.Render(line)
	}
	var out strings.Builder
	out.Grow(len(line) + 16)
	plainFrom := 0
	flush := func(to int) {
		if to <= plainFrom {
			return
		}
		run := line[plainFrom:to]
		// A gap between two styled runs is whitespace the terminal already draws
		// in the default colour; spending an escape pair on it helps nobody.
		if strings.TrimLeft(run, " \t") == "" && run != "" {
			out.WriteString(run)
			return
		}
		out.WriteString(base.Render(run))
	}
	for i := 0; i < len(line); {
		kind, n := spanAt(line, i)
		if n == 0 {
			i++
			continue
		}
		flush(i)
		out.WriteString(kind.style(base).Render(line[i : i+n]))
		i += n
		plainFrom = i
	}
	flush(len(line))
	return out.String()
}

// componentNames are the programs and processes this workspace ships, longest
// first so that config-admin-agent wins over config-admin.
//
// "battle-server" is deliberately absent. "Battle Server" is a feature, not a
// program: chipping the words turns the name of a thing the user reads about into
// a file name, and the reader has to work out which of the two it is looking at.
// The executable that does the work is still a component, so it is named where it
// is used and marked there.
var componentNames = []string{
	"launcher-config-admin-agent",
	"launcher-config-admin",
	"launcher-config",
	"battle-server-manager",
	"battle-server-broadcast",
	"config-admin-agent",
	"config-admin",
	"launcher-agent",
	"AgeServer",
	"launcher",
	"agent",
	"server",
}

// identifierNames are the field and flag names that appear inside messages.
// They are code, not prose, so they read as identifiers rather than as words.
// Keep the list short and specific: a name that also happens to be an ordinary
// English word lights up half the output.
var identifierNames = []string{
	"clientExeArgs",
	"Config.Certificate.CanTrustInPc",
	"config-admin_setup_hosts",
	"config_setup_hosts",
	"Server.Executable",
	"Server.Start",
	"Server.Stop",
	"Client.Executable",
	"Client.Isolation.Path",
	"Config.CanUseDialogs",
	"Config.Log",
	"localCert",
	"caStoreCert",
	"HostFilePath",
	"CertFilePath",
	"SigKeyFilePath",
	"SetUp",
	"Revert",
	"setupCommand",
	"revertCommand",
	"serverPath",
	"serverStop",
	"serverStart",
	"LogRoot",
	"GameId",
	"MapIp",
}

// pathExtensions are the file names a message is likely to mention. Anything with
// one of these is a file, not two words that happen to be glued with a dot.
var pathExtensions = []string{
	".toml", ".txt", ".pem", ".exe", ".bat", ".cmd", ".log", ".crt", ".cer",
	".key", ".p12", ".pfx", ".cfg", ".json", ".yaml", ".yml", ".ini", ".dll",
	".so", ".dylib", ".zip",
}

// spanAt classifies whatever starts at line[i]. A zero length means "nothing
// here worth colouring", and the caller moves on one byte.
func spanAt(line string, i int) (spanKind, int) {
	if beforeToken(line, i) {
		if n := matchFlag(line, i); n > 0 {
			return spanFlag, n
		}
		if n := matchURL(line, i); n > 0 {
			return spanPath, n
		}
		if n := matchIPv4(line, i); n > 0 {
			return spanPath, n
		}
		if n := matchName(line, i, identifierNames); n > 0 {
			return spanKey, n
		}
		if n := matchName(line, i, componentNames); n > 0 {
			return spanComponent, n
		}
		if n := matchPath(line, i); n > 0 {
			return spanPath, n
		}
	}
	return spanPlain, 0
}

// beforeToken reports whether i starts a token, so a name never matches in the
// middle of one.
func beforeToken(line string, i int) bool {
	return i == 0 || isSeparator(line[i-1])
}

// afterToken reports whether i is past the end of a token that started at
// start, which is what stops server from matching the head of serverStart.
func afterToken(line string, i int) bool {
	return i >= len(line) || isSeparator(line[i])
}

// isSeparator reports whether b ends one token and starts the next.
func isSeparator(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\'', '"', '(', ')', '[', ']', '{', '}', ',', '.', ':',
		';', '=', '>', '<', '|', '&', '/', '\\', '!', '?', '*', '#', '@', '$',
		'%', '^', '~', '`', '+':
		return true
	}
	return false
}

func isLetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isFlagByte(b byte) bool {
	return isLetter(b) || isDigit(b) || b == '-' || b == '_'
}

func isPathByte(b byte) bool {
	return isLetter(b) || isDigit(b) || b == '-' || b == '_' || b == '.' || b == '\\' || b == '/' || b == ':' || b == '$' || b == '@' || b == '+'
}

// matchName matches one of names case insensitively, preserving the case the
// message used. Longest first, so config-admin-agent beats config-admin.
func matchName(line string, i int, names []string) int {
	for _, name := range names {
		n := len(name)
		if i+n > len(line) || !hasPrefixFold(line[i:i+n], name) {
			continue
		}
		// The name has to end here too, otherwise server would match the start
		// of serverStart.
		if !afterToken(line, i+n) {
			continue
		}
		return n
	}
	return 0
}

// hasPrefixFold reports whether the ASCII prefix s equals name, ignoring case.
func hasPrefixFold(s, name string) bool {
	for i := 0; i < len(name); i++ {
		if lower(s[i]) != lower(name[i]) {
			return false
		}
	}
	return true
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// matchFlag matches --some-flag.
func matchFlag(line string, i int) int {
	if i+2 >= len(line) || line[i] != '-' || line[i+1] != '-' || !isLetter(line[i+2]) {
		return 0
	}
	n := 2
	for i+n < len(line) && isFlagByte(line[i+n]) {
		n++
	}
	return n
}

// matchURL matches an http or https URL.
func matchURL(line string, i int) int {
	for _, scheme := range []string{"https://", "http://"} {
		if !hasPrefixFold(line[i:], scheme) {
			continue
		}
		n := len(scheme)
		for i+n < len(line) && line[i+n] != ' ' && line[i+n] != '\t' && line[i+n] != '\'' && line[i+n] != '"' {
			n++
		}
		return trimPunctuation(line, i, n)
	}
	return 0
}

// matchIPv4 matches a dotted quad, which is how every address in this output
// appears.
func matchIPv4(line string, i int) int {
	n, groups := 0, 0
	for groups < 4 {
		start := n
		for i+n < len(line) && isDigit(line[i+n]) && n-start < 3 {
			n++
		}
		if n == start {
			return 0
		}
		if v, err := strconv.Atoi(line[i+start : i+n]); err != nil || v > 255 {
			return 0
		}
		groups++
		if groups == 4 {
			break
		}
		if i+n >= len(line) || line[i+n] != '.' {
			return 0
		}
		n++
	}
	if i+n < len(line) && isDigit(line[i+n]) {
		return 0
	}
	return n
}

// matchPath matches a filesystem path or a file name. A backslash is treated as
// conclusive on its own, because every path in this output is a Windows one and
// "and/or" in prose must not turn into a path.
func matchPath(line string, i int) int {
	// A continuation of a path the previous line started, such as the
	// "Files\AgeServer\config.toml" half of a wrapped "C:\Program Files\...".
	if line[i] == '\\' || line[i] == '/' {
		n := 1
		for i+n < len(line) && isPathByte(line[i+n]) {
			n++
		}
		if n > 1 {
			return trimPunctuation(line, i, n)
		}
		return 0
	}
	n := 0
	for i+n < len(line) && isPathByte(line[i+n]) {
		n++
	}
	if n == 0 {
		return 0
	}
	token := line[i : i+n]
	if strings.Contains(token, "\\") {
		return trimPunctuation(line, i, n)
	}
	// An absolute POSIX path needs a second separator to be one.
	if token[0] == '/' && strings.Count(token, "/") > 1 {
		return trimPunctuation(line, i, n)
	}
	lower := strings.ToLower(token)
	for _, ext := range pathExtensions {
		if strings.HasSuffix(lower, ext) {
			return trimPunctuation(line, i, n)
		}
	}
	return 0
}

// trimPunctuation drops the separators a run greedily swallowed at its end, so a
// path never eats the comma or the closing bracket that follows it.
func trimPunctuation(line string, i, n int) int {
	for n > 0 {
		switch line[i+n-1] {
		case '.', ',', ';', ':', '\\', '/', '-', '_':
			n--
			continue
		}
		break
	}
	return n
}
