// Package notify sends a desktop notification when the game session is over.
//
// It exists because of when the agent runs: it was started by the launcher, it
// outlives it, and by the time it has something to say the terminal it printed
// to is a window nobody is looking at any more. The notification is the only
// signal that reaches the user at the moment they care about, which is minutes
// after they stopped paying attention.
//
// The notification never decides anything. A failure to show one is logged and
// otherwise ignored, because a user who cannot see a toast must still get the
// same exit code, and a user with no notification service at all must not get a
// different one.
package notify

import (
	"errors"
	"fmt"

	"github.com/ncruces/zenity"
)

// Title is the notification's title on every platform.
const Title = "AgeLAN Server"

// SessionEnded says the game closed and the configuration went back to how it was
// before the launcher touched it.
//
// It is worth saying out loud, because it is the answer to the question the user
// has while the game is loading and cannot ask yet: whether closing the game will
// leave the machine modified.
func SessionEnded(game string) error { return show(endedText(game), zenity.InfoIcon) }

// SessionStopped says the agent was stopped on purpose and the configuration went
// back anyway.
//
// It is a separate message from SessionEnded rather than the same one with
// different wording, because the game did not close: somebody is still playing.
// Telling them "Game closed" about a game they are looking at is worse than
// saying nothing, and reusing the failure message would have them go and read
// logs for a teardown that completed.
func SessionStopped(game string) error { return show(stoppedText(game), zenity.InfoIcon) }

// SessionFailed says the session ended with something wrong, and says what to do
// about it: the revert is best effort, so the honest message names the logs rather
// than claiming the machine is clean.
func SessionFailed(game string, exitCode int) error {
	return show(failedText(game, exitCode), zenity.ErrorIcon)
}

// endedText is the body of the good news.
func endedText(game string) string {
	return withGame(game, "Game closed. Everything has been reverted.")
}

// stoppedText is the body of the news for a run the user ended. It says the
// teardown happened rather than that the game closed, because the game did not.
func stoppedText(game string) string {
	return withGame(game, "Stopped. Everything has been reverted.")
}

// failedText is the body of the bad news. It carries the exit code because that is
// the one thing the user cannot get from anywhere else, and it points at the logs
// because the revert is best effort: without them there is no way to tell which
// change survived.
func failedText(game string, exitCode int) string {
	return withGame(game, fmt.Sprintf(
		"Session ended with error %d. Some changes may not have been reverted, check the logs.", exitCode))
}

// withGame names the game only when it is known: "Game closed" reads and
// ": Game closed" does not.
func withGame(game string, message string) string {
	if game == "" {
		return message
	}
	return game + ": " + message
}

// notifyFn is indirected so a test can prove a notification was attempted without
// putting one on the desktop of whoever is running the tests.
var notifyFn = zenity.Notify

// show is the one place that talks to zenity, so the title and the icon cannot
// drift apart between the two outcomes.
func show(body string, icon zenity.DialogIcon) error {
	return notifyFn(body, zenity.Title(Title), icon)
}

// Unsupported reports whether the platform cannot show a notification at all, as
// opposed to a service that is merely not running.
//
// It is here so a caller can log something more useful than "notification failed":
// there is a difference between "this system has no way to show one" and "the
// notification daemon is not running", and only one of them is worth telling
// anybody about.
func Unsupported(err error) bool {
	return errors.Is(err, zenity.ErrUnsupported)
}
