package dialog

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ncruces/zenity"
)

// fakeZenity replaces the zenity entry points so no test can open a real
// dialog window.
type fakeZenity struct {
	text  string
	items []string
}

func (f *fakeZenity) install(t *testing.T, list func(string, []string, ...zenity.Option) (string, error), question func(string, ...zenity.Option) error) {
	t.Helper()
	origList, origQuestion := zenityListFn, zenityQuestionFn
	zenityListFn = func(text string, items []string, _ ...zenity.Option) (string, error) {
		f.text = text
		f.items = items
		return list(text, items)
	}
	zenityQuestionFn = func(text string, _ ...zenity.Option) error {
		f.text = text
		return question(text)
	}
	t.Cleanup(func() { zenityListFn, zenityQuestionFn = origList, origQuestion })
}

func okList(selected string) func(string, []string, ...zenity.Option) (string, error) {
	return func(string, []string, ...zenity.Option) (string, error) { return selected, nil }
}

func failList(err error) func(string, []string, ...zenity.Option) (string, error) {
	return func(string, []string, ...zenity.Option) (string, error) { return "", err }
}

func okQuestion() func(string, ...zenity.Option) error {
	return func(string, ...zenity.Option) error { return nil }
}

func failQuestion(err error) func(string, ...zenity.Option) error {
	return func(string, ...zenity.Option) error { return err }
}

func TestZenityName(t *testing.T) {
	if got := (zenityDialog{}).Name(); got != "zenity" {
		t.Fatalf("Name() = %q, want %q", got, "zenity")
	}
}

// The graphical window renders the candidate list itself, so the no-prompt
// paths must not also dump it to the console.
func TestZenityListCandidatesIsSilent(t *testing.T) {
	out := captureStdout(t, func() {
		(zenityDialog{}).ListCandidates(testServers(2))
	})
	if out != "" {
		t.Fatalf("ListCandidates printed %q, want nothing", out)
	}
}

func TestZenitySelectServerReturnsIndexedItem(t *testing.T) {
	fake := &fakeZenity{}
	fake.install(t, okList("2. b"), okQuestion())

	got, ok := (zenityDialog{}).SelectServer(
		[]ServerCandidate{{Description: "a"}, {Description: "b"}},
		strings.NewReader(""),
	)
	if !ok || got != 1 {
		t.Fatalf("= (%d, %t), want (1, true)", got, ok)
	}
	if len(fake.items) != 2 || fake.items[0] != "1. a" || fake.items[1] != "2. b" {
		t.Errorf("items = %v, want [1. a 2. b]", fake.items)
	}
	if fake.text != "Choose a server to join:" {
		t.Errorf("text = %q", fake.text)
	}
}

// Regression: the Windows list is a fixed 241 px wide with no horizontal
// scrollbar, so the items must be the compact Label. Using the full Description
// clipped its tail off, hiding the latency and the version.
func TestZenitySelectServerUsesCompactLabelNotFullDescription(t *testing.T) {
	fake := &fakeZenity{}
	fake.install(t, okList("1. 192.168.1.10 - 5 ms (v1.11.0)"), okQuestion())

	servers := []ServerCandidate{
		{
			Description: "192.168.1.10, 192.168.1.11 (a.local, b.local) - 5 ms (v1.11.0)",
			Label:       "192.168.1.10 - 5 ms (v1.11.0)",
		},
	}
	if _, ok := (zenityDialog{}).SelectServer(servers, strings.NewReader("")); !ok {
		t.Fatal("SelectServer did not succeed")
	}
	if len(fake.items) != 1 {
		t.Fatalf("items = %v, want 1", fake.items)
	}
	if fake.items[0] != "1. 192.168.1.10 - 5 ms (v1.11.0)" {
		t.Errorf("item = %q, want the compact label", fake.items[0])
	}
	if strings.Contains(fake.items[0], "a.local") {
		t.Errorf("item %q still carries the hostnames, it will clip", fake.items[0])
	}
}

// A candidate without a compact label must still be usable: the full
// description is shown rather than an empty row.
func TestZenitySelectServerFallsBackToDescriptionWithoutLabel(t *testing.T) {
	fake := &fakeZenity{}
	fake.install(t, okList("1. only the description"), okQuestion())

	if _, ok := (zenityDialog{}).SelectServer(
		[]ServerCandidate{{Description: "only the description"}},
		strings.NewReader(""),
	); !ok {
		t.Fatal("SelectServer did not succeed")
	}
	if len(fake.items) != 1 || fake.items[0] != "1. only the description" {
		t.Errorf("items = %v, want [1. only the description]", fake.items)
	}
}

// Cancel means "none of these", which is what the caller already does by
// starting its own server. It must never read stdin: delegating there would
// return ok=true and silently pick a server the user rejected.
func TestZenitySelectServerCancelReturnsNotOK(t *testing.T) {
	fake := &fakeZenity{}
	fake.install(t, failList(zenity.ErrCanceled), okQuestion())

	reader := strings.NewReader("1\n")
	got, ok := (zenityDialog{}).SelectServer(testServers(2), reader)
	if ok {
		t.Fatalf("cancel = (%d, true), want ok=false", got)
	}
	if n := reader.Len(); n != 2 {
		t.Errorf("stdin still has %d bytes left, the canceled dialog must not read it", n)
	}
}

func TestZenitySelectServerUnsupportedFallsBackToConsole(t *testing.T) {
	quietOutput(t)
	fake := &fakeZenity{}
	fake.install(t, failList(zenity.ErrUnsupported), okQuestion())

	got, ok := (zenityDialog{}).SelectServer(testServers(2), strings.NewReader("1\n"))
	if !ok || got != 0 {
		t.Fatalf("= (%d, %t), want (0, true) from the console fallback", got, ok)
	}
}

func TestZenitySelectServerOtherErrorFallsBackToConsole(t *testing.T) {
	quietOutput(t)
	fake := &fakeZenity{}
	fake.install(t, failList(errors.New("boom")), okQuestion())

	got, ok := (zenityDialog{}).SelectServer(testServers(3), strings.NewReader("3\n"))
	if !ok || got != 2 {
		t.Fatalf("= (%d, %t), want (2, true) from the console fallback", got, ok)
	}
}

func TestZenitySelectServerUnparsableItemFallsBackToConsole(t *testing.T) {
	quietOutput(t)
	fake := &fakeZenity{}
	fake.install(t, okList("not an item"), okQuestion())

	got, ok := (zenityDialog{}).SelectServer(testServers(2), strings.NewReader("2\n"))
	if !ok || got != 1 {
		t.Fatalf("= (%d, %t), want (1, true) from the console fallback", got, ok)
	}
}

func TestZenityConfirmOK(t *testing.T) {
	fake := &fakeZenity{}
	fake.install(t, okList(""), okQuestion())

	if !(zenityDialog{}).ConfirmStartServer("Do it.", strings.NewReader("")) {
		t.Error("a confirmed dialog must return true")
	}
	if fake.text != "Do it." {
		t.Errorf("text = %q, want %q", fake.text, "Do it.")
	}
}

func TestZenityConfirmCancelReturnsFalse(t *testing.T) {
	fake := &fakeZenity{}
	fake.install(t, okList(""), failQuestion(zenity.ErrCanceled))

	reader := strings.NewReader("\n")
	if (zenityDialog{}).ConfirmStartServer("Do it.", reader) {
		t.Fatal("a canceled dialog must return false so the launcher aborts")
	}
	if n := reader.Len(); n != 1 {
		t.Errorf("stdin still has %d bytes left, the canceled dialog must not read it", n)
	}
}

func TestZenityConfirmUnsupportedFallsBackToConsole(t *testing.T) {
	quietOutput(t)
	fake := &fakeZenity{}
	fake.install(t, okList(""), failQuestion(zenity.ErrUnsupported))

	if !(zenityDialog{}).ConfirmStartServer("Do it.", strings.NewReader("")) {
		t.Error("the console fallback must return true so the launch continues")
	}
}

func TestZenityConfirmOtherErrorFallsBackToConsole(t *testing.T) {
	quietOutput(t)
	fake := &fakeZenity{}
	fake.install(t, okList(""), failQuestion(errors.New("boom")))

	if !(zenityDialog{}).ConfirmStartServer("Do it.", strings.NewReader("")) {
		t.Error("the console fallback must return true so the launch continues")
	}
}

// Regression: zenity.IsAvailable does PATH lookups, so "false" must never
// consult it and the other modes must consult it at most once per process.
func TestAvailableOnceEvaluatedAtMostOnce(t *testing.T) {
	orig := availableOnce
	t.Cleanup(func() { availableOnce = orig })

	calls := 0
	availableOnce = sync.OnceValue(func() bool { calls++; return true })
	if New(true).Name != "zenity" || New(true).Name != "zenity" {
		t.Fatal("auto with available dialogs must resolve to zenity")
	}
	if calls != 1 {
		t.Errorf("IsAvailable consulted %d times, want 1", calls)
	}

	calls = 0
	availableOnce = func() bool { calls++; return true }
	if New(false).Name != "console" {
		t.Error("false must always resolve to the console")
	}
	if calls != 0 {
		t.Errorf("IsAvailable consulted %d times for mode=false, want 0", calls)
	}
}
