// Package tray puts the agent in the system tray for as long as it is working,
// and offers the one thing a user can do about a process they cannot see: stop
// it.
//
// The agent is the only part of a session that outlives the launcher, and it is
// started detached with no console of its own. By the time it has something to
// say, the window that launched it is closed and the pipes it printed to go
// nowhere. It spends almost its whole run inside a single wait — up to a minute
// for a game that may never start, then however long the game lasts — so that
// gap is not an edge case, it is the shape of the run. A tray icon is the one
// piece of interface that is on screen for the whole of it.
//
// It carries two things, and the second is why the first is not decoration:
//
//   - what the agent is doing, both on hover and on a row of the menu that
//     cannot be clicked, so that the state of a run that is silently rewriting
//     the hosts file and the certificate stores is visible while it is happening
//     and not only in the log afterwards.
//
//   - an Exit, which is the same request a terminate signal makes: stop waiting,
//     put the machine back the way it was, and leave. Without it the only way to
//     stop the agent is to find it in the task manager, which is a thing a user
//     does when they have already concluded that something is wrong.
//
// Neither decides anything. A tray that cannot be created, a menu that cannot be
// updated and an icon that will not go away are all reported to the log and then
// ignored: the run's exit code and its teardown are the same with an icon on
// screen and with none. That is not an escape hatch for turning it off — there is
// none, and there is no flag — it is what happens on a desktop that has no
// notification area to put an icon in.
package tray

import (
	"runtime"
	"sync"
	"time"

	"github.com/luskaner/ageServer/launcher-agent/internal"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
)

// Title is what the tooltip and the notification area attribute the icon to. It
// is the product rather than the binary: an icon called "agent" tells a user
// nothing about which of their programs is asking them to stop something.
const Title = "AgeLAN Server"

// ExitLabel is the row that asks for the teardown.
//
// It says what it does rather than what it is. "Exit" on its own, in a menu
// whose every other row is a greyed out report of something happening, reads as
// leaving a screen. This is the opposite: it puts the machine back the way it was
// before the agent touched it, which is worth spelling out at the point of the
// click rather than only in the log afterwards.
const ExitLabel = "Exit (revert configuration and stop)"

// ExitDisabledLabel is what the row says once it has been used.
//
// The click is not repeated because the callback cannot be, and a menu that
// still offers an action which has already happened is a menu that lies. It also
// says what to wait for, which is the only thing the user is left wanting to
// know after they have clicked it.
const ExitDisabledLabel = "Stopping and reverting..."

// Unknown is shown before the agent has said anything, which is only the handful
// of steps between starting and the first one being reported.
const Unknown = "Starting..."

// stopTimeout bounds how long Stop waits for the platform's message loop to end
// after the icon has been asked to go away.
//
// The icon is removed before the process exits so that the notification area is
// empty by the time the user looks at it, rather than emptied a moment later by
// the shell noticing the process is gone. Waiting for that removal is worth
// doing, but not at the price of an agent that refuses to exit over it, so the
// wait is bounded and gives up.
var stopTimeout = 5 * time.Second

// Tray is the agent's icon in the notification area.
//
// A Tray is always safe to use, including the one Start hands back when it was
// asked not to create anything: every method checks for the icon it never had.
type Tray struct {
	icon    trayIcon
	doing   trayItem
	session trayItem
	exit    trayItem

	// onExit is what Exit means. It is a request, not an action: the run's
	// goroutine is the one that stops waiting, tears down and ends.
	onExit func()

	// mu guards the four rows and the icon against each other and against a Stop
	// that arrives while the platform thread is still building them. exitAsked is
	// the guard on Exit itself, which is the one row a user can aim at twice.
	mu        sync.Mutex
	exitAsked bool
	stopOnce  sync.Once

	// ready is closed once the icon exists and has been shown, so that Start can
	// return a tray whose rows can be written to.
	ready     chan struct{}
	readyOnce sync.Once

	// done is closed when the platform's message loop has returned.
	done     chan struct{}
	doneOnce sync.Once

	// unsubscriber stops the activity from reaching a menu that is gone.
	unsubscriber func()
}

// Start shows the tray icon.
//
// It is not an option. The agent is started detached and outlives the launcher
// that started it, so on the platforms where it runs there is nothing else on
// screen saying that a process is currently rewriting the hosts file and the
// certificate stores, and nothing on screen that can stop it. That is not a
// feature some runs can be run without: it is the only interface the run has for
// as long as it lasts.
//
// gameID names the session in the menu, which is what tells a user with a second
// window open which of their games the icon belongs to.
//
// onExit is called when the user picks Exit. It runs on the platform's own
// thread, inside the library's handling of the click, so it has to return
// promptly and must not wait for anything.
func Start(gameID string, onExit func()) *Tray {
	t := &Tray{
		onExit: onExit,
		ready:  make(chan struct{}),
		done:   make(chan struct{}),
	}
	go t.serve(gameID)
	<-t.ready
	return t
}

// serve builds the icon and pumps its message loop.
//
// Everything in here is on one goroutine locked to one operating system thread,
// including the icon's creation. That is not a precaution: on Windows the
// library backs the tray with a message-only window and pumps it with
// GetMessage, which only ever sees the windows belonging to the thread it is
// called on. Go is free to move a goroutine to another thread between the two
// calls, and if it does the icon appears and then never answers a click.
//
// It closes ready even when it fails, because Start is waiting on it and the run
// does not stop for an icon.
func (t *Tray) serve(gameID string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Closed last, and on the way out of every branch, because Stop waits on it
	// to know that touching the platform any more is unsafe.
	defer t.doneOnce.Do(func() { close(t.done) })
	defer func() {
		if recovered := recover(); recovered != nil {
			// A panic in here would unwind into a goroutine nobody watches, take
			// the run down with it, and leave Start blocked forever on ready. An
			// icon is not worth ending a session over.
			cmdlog.Detail("The system tray icon could not be started: %v", recovered)
			// There is no icon to take away and nothing left registered against
			// it, so the exit path must not try.
			t.mu.Lock()
			t.icon = nil
			t.mu.Unlock()
			t.signalReady()
		}
	}()

	icon := newIconFn()
	menu := newMenuFn()
	doing := menu.Add(Unknown, func() {})
	doing.SetDisabled(true)
	// The game is known from the first moment, so the row has something to say
	// before the first step is reported.
	session := menu.Add(gameID, func() {})
	session.SetDisabled(true)
	menu.AddSeparator()
	exit := menu.Add(ExitLabel, t.requestExit)

	// Published before the icon is asked for, so that nothing can stop the tray
	// in the middle of building it and leave rows pointing at nothing.
	t.mu.Lock()
	t.icon, t.doing, t.session, t.exit = icon, doing, session, exit
	t.mu.Unlock()

	// Every call into the platform happens outside the lock. They are the calls
	// that can panic, and a panic while holding this package's own lock would
	// leave every later caller of it waiting for a lock nobody will release.
	icon.SetMenu(menu)
	icon.SetIcon(iconPNG())
	// Subscribed after the rows exist and before the icon is shown. OnActivity
	// calls back with the current value straight away, which is why this cannot
	// be done while holding the lock that update takes.
	unsubscribe := internal.OnActivity(t.update)
	t.mu.Lock()
	t.unsubscriber = unsubscribe
	t.mu.Unlock()

	icon.Show()
	t.signalReady()
	if err := icon.Run(); err != nil {
		// The loop ended on its own, which on every platform means the icon was
		// destroyed. There is nothing to recover and nothing to undo: Stop is the
		// only thing that destroys one, and it is idempotent.
		cmdlog.Detail("The system tray icon stopped: %s", err)
	}
}

// requestExit is what the Exit row does.
//
// It runs on the platform's thread and gives the run the reason to stop; it does
// not stop it. The teardown happens where every other teardown happens, on the
// run's own goroutine, and through the same once, so a click here and a
// terminate signal arriving a moment later produce one teardown and not two.
func (t *Tray) requestExit() {
	t.mu.Lock()
	// One request, however many clicks arrive. The row is greyed out below, which
	// on every platform stops the next click from being dispatched at all — but a
	// second one can already be on its way from the window manager by the time the
	// first is handled, and a run asked to stop twice is a run with two teardowns
	// contending for the same hosts file.
	if t.exit == nil || t.exitAsked {
		t.mu.Unlock()
		return
	}
	t.exitAsked = true
	t.exit.SetLabel(ExitDisabledLabel)
	t.exit.SetDisabled(true)
	t.mu.Unlock()
	if t.onExit != nil {
		t.onExit()
	}
}

// update redraws the icon from what the agent is currently doing.
//
// It is called from the goroutine doing the work, which is not the one holding
// the icon, because the platform is what allows it: updating a menu row is a
// call to the window manager and is safe from anywhere.
func (t *Tray) update(activity internal.Activity) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.icon == nil {
		return
	}
	t.icon.SetTooltip(tooltip(activity))
	if activity.Doing != "" {
		t.doing.SetLabel(activity.Doing)
	}
	if activity.Session != "" {
		t.session.SetLabel(activity.Session)
	}
}

// tooltip is what the icon says on hover, which is the one piece of this the
// user gets without having decided to look.
func tooltip(activity internal.Activity) string {
	doing := activity.Doing
	if doing == "" {
		doing = Unknown
	}
	return Title + " — " + doing
}

// Stop takes the icon away and waits for the message loop to end.
//
// It is safe to call more than once and safe to call on a tray that never got
// an icon, because the exit path runs it on a defer and the exit path also runs
// when there was nothing to stop.
func (t *Tray) Stop() {
	t.stopOnce.Do(func() {
		t.mu.Lock()
		icon, unsubscribe := t.icon, t.unsubscriber
		t.icon, t.unsubscriber = nil, nil
		t.mu.Unlock()
		// Unsubscribed before the icon is destroyed, so that a step reported while
		// it is being taken away cannot write into a menu the platform is
		// already freeing.
		if unsubscribe != nil {
			unsubscribe()
		}
		if icon == nil {
			return
		}
		icon.Remove()
		select {
		case <-t.done:
		case <-time.After(stopTimeout):
			cmdlog.Detail("The system tray icon did not go away in %s, leaving it to the system.", stopTimeout)
		}
	})
}

// signalReady releases Start, whether or not an icon was made.
func (t *Tray) signalReady() { t.readyOnce.Do(func() { close(t.ready) }) }

// The three seams below are where the library is reached. Everything above them
// is the part of this package that has a decision in it.
var (
	newIconFn = func() trayIcon { return &systrayIcon{icon: systrayNew()} }
	newMenuFn = func() trayMenu { return &systrayMenu{menu: systrayNewMenu()} }
)
