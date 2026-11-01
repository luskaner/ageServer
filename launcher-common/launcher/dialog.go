package launcher

import (
	"io"
	"sync/atomic"
)

// Mode values accepted by Config.CanUseDialogs and --dialog.
const (
	ModeAuto  = "auto"
	ModeTrue  = "true"
	ModeFalse = "false"
)

// ServerCandidate is one discovered server offered to the user.
type ServerCandidate struct {
	// Description is the full line, with every discovered IP and hostname. The
	// console shows it as is.
	Description string
	// Label is a compact summary for backends with little horizontal room.
	// Graphical backends use this so the decisive part (address, latency,
	// version) survives instead of being clipped off the end.
	Label string
}

// Dialog is the set of interactive prompts the launcher needs.
//
// It lives here, and not with the backends, because the questions are part of
// deciding what to do and every frontend asks the same ones. Only the way they
// are asked differs: the console reads from stdin, a graphical window draws a
// list. A frontend that cannot ask anything implements it by taking the default,
// which is what "auto" already does when no dialog is available.
type Dialog interface {
	// Name returns the backend identifier recorded in the log file.
	Name() string

	// SelectServer asks which of the discovered servers to use. servers
	// holds the candidates already sorted by latency. It returns the 0-based
	// index into servers.
	// ok is false when the user declined to pick one, in which case the
	// caller must fall back to its own default (start its own server).
	// stdin is only read by the console implementation.
	SelectServer(servers []ServerCandidate, stdin io.Reader) (index int, ok bool)

	// ListCandidates shows the candidate list without asking anything. The
	// console backend prints it, exactly as SelectServer does before its
	// prompt, so paths that answer on their own still leave the console user
	// with a trace of what was considered. Backends that render the list in
	// their own window leave it as a no-op.
	ListCandidates(servers []ServerCandidate)

	// ConfirmStartServer asks whether to go ahead and start the server.
	// It returns false only when the user actively declined.
	ConfirmStartServer(text string, stdin io.Reader) bool
}

// Resolution is the outcome of resolving the configured mode to a backend.
type Resolution struct {
	Dialog Dialog
	// Name identifies the backend, for the log file and for a frontend that
	// wants to say which one answered.
	Name string
	// Reason is non-empty when the configured mode could not be honoured and
	// the fallback was used instead. It is meant to be logged: the user asked
	// for one thing and got another, which they can see happening but not why.
	Reason string
}

var (
	activeDialog atomic.Pointer[Dialog]

	// DefaultDialog answers when nothing has been installed.
	//
	// A frontend sets it once at init, because what "no dialog" means depends on
	// the frontend: for the console launcher it is the console, which is also
	// what the run falls back to when graphical dialogs turn out to be
	// unavailable. Until then DefaultsDialog takes every answer on its own.
	DefaultDialog Dialog = DefaultsDialog{}
)

// SetDialog installs the dialog that answers for the rest of the session. A
// frontend calls it once it has decided how it wants to ask.
func SetDialog(d Dialog) { activeDialog.Store(&d) }

// ActiveDialog returns the installed dialog, or DefaultDialog. It never returns
// nil, so the prompts keep working on early call paths.
func ActiveDialog() Dialog {
	if d := activeDialog.Load(); d != nil && *d != nil {
		return *d
	}
	if DefaultDialog != nil {
		return DefaultDialog
	}
	return DefaultsDialog{}
}

// ResetDialog removes the installed dialog, so the run goes back to
// DefaultDialog.
func ResetDialog() { activeDialog.Store(nil) }

// DefaultsDialog is a dialog that asks nothing and takes every default.
//
// It is what a graphical frontend falls back to when it has nothing to show, and
// what the shared logic sees before any frontend has installed a backend. It
// declines both questions on purpose: refusing to pick a server is the signal
// for "start my own", which is the answer that works with no window open.
type DefaultsDialog struct{}

func (DefaultsDialog) Name() string { return "defaults" }

func (DefaultsDialog) SelectServer([]ServerCandidate, io.Reader) (int, bool) {
	return 0, false
}

func (DefaultsDialog) ListCandidates([]ServerCandidate) {}

func (DefaultsDialog) ConfirmStartServer(string, io.Reader) bool { return false }
