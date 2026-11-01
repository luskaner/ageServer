package notify

import (
	"errors"
	"strings"
	"testing"

	"github.com/ncruces/zenity"
)

// The notification is the only signal that reaches the user at the moment they
// care about, so what it says is the thing under test.
func TestSessionEnded(t *testing.T) {
	got := endedText("age2")
	if !strings.Contains(got, "reverted") {
		t.Errorf("the good news is that everything is back to normal, got %q", got)
	}
	if !strings.Contains(got, "age2") {
		t.Errorf("the game is not named, got %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("the body must be one line: %q", got)
	}
}

// The bad news has to say what to do, not just that something happened: the revert
// is best effort, so "some changes may not have been reverted" is the truth and
// anything stronger would be a lie the user acts on.
func TestSessionFailed(t *testing.T) {
	got := failedText("age4", 42)
	if !strings.Contains(got, "42") {
		t.Errorf("the exit code is missing, got %q", got)
	}
	if !strings.Contains(got, "logs") {
		t.Errorf("the message must say where to look, got %q", got)
	}
	if !strings.Contains(got, "may not") {
		t.Errorf("a failed session must not claim everything was reverted, got %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("the body must be one line: %q", got)
	}
}

// A game id the agent was not told is left out rather than printed empty.
func TestGameNameIsOmittedWhenUnknown(t *testing.T) {
	if got := withGame("", "Game closed."); got != "Game closed." {
		t.Errorf("got %q", got)
	}
	if got := withGame("age2", "Game closed."); got != "age2: Game closed." {
		t.Errorf("got %q", got)
	}
}

// Both outcomes must actually attempt a notification, and neither may swallow the
// error: the caller logs it, and a user who saw nothing needs to know why.
func TestBothOutcomesNotifyAndReportFailure(t *testing.T) {
	orig := notifyFn
	t.Cleanup(func() { notifyFn = orig })

	var shown []string
	notifyFn = func(body string, _ ...zenity.Option) error {
		shown = append(shown, body)
		return nil
	}
	if err := SessionEnded("age2"); err != nil {
		t.Errorf("SessionEnded: %v", err)
	}
	if err := SessionFailed("age2", 7); err != nil {
		t.Errorf("SessionFailed: %v", err)
	}
	if len(shown) != 2 {
		t.Fatalf("got %d notifications, want 2: %q", len(shown), shown)
	}

	// A failure to notify is passed on, not turned into a success.
	notifyFn = func(string, ...zenity.Option) error { return errors.New("no notification service") }
	if err := SessionEnded("age2"); err == nil {
		t.Error("a failed notification must not be reported as delivered")
	}
}

// A platform that cannot show one at all is worth telling apart from a service
// that is merely not running.
func TestUnsupportedIsDistinguishable(t *testing.T) {
	if !Unsupported(zenity.ErrUnsupported) {
		t.Error("ErrUnsupported must be recognised")
	}
	if Unsupported(errors.New("connection refused")) {
		t.Error("a service that is not running is not the same as not being supported")
	}
	if Unsupported(nil) {
		t.Error("no error is not an unsupported platform")
	}
}
