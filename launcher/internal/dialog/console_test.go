package dialog

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	return <-done
}

// quietOutput silences the console prompts for the tests that only assert on
// the returned answer, not on the printed text.
func quietOutput(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetOutput(defaultOutput()) })
	SetOutput(Output{
		Println: func(...any) {},
		Printf:  func(string, ...any) {},
	})
}

// testServers builds candidates whose only field is the description, which is
// what the console prints.
func testServers(count int) []ServerCandidate {
	servers := make([]ServerCandidate, count)
	for i := range servers {
		servers[i] = ServerCandidate{Description: strings.Repeat("x", i+1)}
	}
	return servers
}

func TestConsoleSelectServerValidInput(t *testing.T) {
	quietOutput(t)
	for _, tc := range []struct {
		input string
		want  int
	}{
		{"1\n", 0},
		{"2\n", 1},
		{"3\n", 2},
	} {
		reader := strings.NewReader(tc.input)
		got, ok := consoleDialog{}.SelectServer(testServers(3), reader)
		if !ok || got != tc.want {
			t.Errorf("input %q: = (%d, %t), want (%d, true)", tc.input, got, ok, tc.want)
		}
	}
}

// Regression: out-of-range input retries, but a broken reader must give up
// immediately.
func TestConsoleSelectServerInvalidThenValidRetries(t *testing.T) {
	quietOutput(t)
	reader := strings.NewReader("99\n0\n2\n")
	got, ok := consoleDialog{}.SelectServer(testServers(3), reader)
	if !ok || got != 1 {
		t.Fatalf("= (%d, %t), want (1, true) (0-based after retry)", got, ok)
	}
}

// Regression: on stdin EOF the old loop printed the list forever because it
// `continue`d on every Scan error. It must give up so the caller can fall back
// to starting its own server.
func TestConsoleSelectServerEOFReturnsNotOK(t *testing.T) {
	quietOutput(t)
	got, ok := (consoleDialog{}).SelectServer(testServers(3), strings.NewReader(""))
	if ok {
		t.Fatalf("EOF = (%d, true), want ok=false", got)
	}
}

// Regression: the selection prompt lost the numbered list of discovered servers
// (name/IP/latency/...), leaving only the bare "Enter the number of..." prompt.
// Every row must be printed, numbered from 1, on each attempt.
func TestConsoleSelectServerPrintsServerList(t *testing.T) {
	out := captureStdout(t, func() {
		if got, ok := (consoleDialog{}).SelectServer(testServers(3), strings.NewReader("2\n")); !ok || got != 1 {
			t.Errorf("= (%d, %t), want (1, true)", got, ok)
		}
	})
	for _, want := range []string{
		"Found the following servers:",
		"1. x\n",
		"2. xx\n",
		"3. xxx\n",
		"Enter the number of the server (1-3): ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

// Regression: ConfirmStartServer returns true even on EOF. The prompt only ever
// offered "continue", so there was no way to say no and an EOF must not abort
// the launch.
func TestConsoleConfirmStartServerEOFReturnsTrue(t *testing.T) {
	out := captureStdout(t, func() {
		if !(consoleDialog{}).ConfirmStartServer("Starting.", strings.NewReader("")) {
			t.Error("EOF must be treated as 'continue', not as a cancellation")
		}
	})
	if !strings.Contains(out, "Starting. Press enter to continue...") {
		t.Errorf("output missing the prompt, got:\n%s", out)
	}
}

func TestConsoleConfirmStartServerPrintsPrompt(t *testing.T) {
	out := captureStdout(t, func() {
		if !(consoleDialog{}).ConfirmStartServer("Do it.", strings.NewReader("\n")) {
			t.Error("a newline must be treated as 'continue'")
		}
	})
	if !strings.Contains(out, "Do it. Press enter to continue...") {
		t.Errorf("output missing the prompt, got:\n%s", out)
	}
}

func TestConsoleName(t *testing.T) {
	if got := (consoleDialog{}).Name(); got != "console" {
		t.Fatalf("Name() = %q, want %q", got, "console")
	}
}

// ListCandidates is the listing SelectServer prints before its prompt, and
// nothing more: the caller adds whatever comes after it.
func TestConsoleListCandidatesPrintsTheSameList(t *testing.T) {
	out := captureStdout(t, func() {
		(consoleDialog{}).ListCandidates(testServers(3))
	})
	want := "Found the following servers:\n1. x\n2. xx\n3. xxx\n"
	if out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}
