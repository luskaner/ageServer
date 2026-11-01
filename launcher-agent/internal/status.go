package internal

import "sync"

// Activity is the one thing the agent is doing at this instant.
//
// It exists because of where the agent spends its life. It is started by the
// launcher, it outlives it, it is started detached with no console, and almost
// all of its run is spent inside a single wait: up to a minute looking for a
// game that may never start, then however long the game lasts. None of that has
// anywhere to say what it is up to, so the process that is currently rewriting
// the machine's hosts file and certificate stores is invisible by construction.
//
// One value, replaced rather than appended to, is what makes this usable: a
// history of steps is what the file log already is, and reading it is a job
// nobody can do while waiting for a game to load. What is missing is the present
// tense.
type Activity struct {
	// Doing is the step happening right now, in the present tense and without a
	// trailing period, because it is rendered as a menu row and a hover tooltip
	// rather than as a line of a log.
	Doing string

	// Session says what it is happening for: the game, and once they are known,
	// the processes being watched. It is empty before the game is found, which is
	// the one moment where the tray can only offer the step itself.
	Session string
}

// activity holds the current value and whoever is rendering it.
//
// A mutex rather than an atomic because publishing a value and handing it to the
// listeners is one act: an atomic swap of the value alone would let a listener
// be registered between the store and the read and silently miss everything
// published in between, which is how the tray would come up already blank.
var activity = struct {
	sync.Mutex
	current   Activity
	listeners map[uint64]func(Activity)
	nextID    uint64
}{}

// SetActivity publishes what the agent is doing now.
//
// It is called from the goroutine doing the work, and it must never be the
// reason that work stops: the listeners are handed a copy outside the lock and
// whatever they do with it belongs to them.
func SetActivity(doing string, session string) {
	next := Activity{Doing: doing, Session: session}
	activity.Lock()
	activity.current = next
	listeners := make([]func(Activity), 0, len(activity.listeners))
	for _, listener := range activity.listeners {
		listeners = append(listeners, listener)
	}
	activity.Unlock()
	for _, listener := range listeners {
		listener(next)
	}
}

// CurrentActivity returns what is being done as of right now. It is the value
// before anything has been published, which is a blank one rather than an error:
// nothing has gone wrong, the agent simply has not said yet.
func CurrentActivity() Activity {
	activity.Lock()
	defer activity.Unlock()
	return activity.current
}

// OnActivity installs a renderer for the activity and hands it the current value
// right away, so a subscriber that arrives late shows the step in progress
// instead of waiting for the next one.
//
// It returns a function that removes the listener. The tray uses it to stop
// writing into a menu the platform has already torn down. Removing by
// subscription rather than by closing the tray's icon is what keeps a stopped
// tray from being registered again the next time one starts.
func OnActivity(listener func(Activity)) (unsubscribe func()) {
	activity.Lock()
	activity.nextID++
	id := activity.nextID
	if activity.listeners == nil {
		activity.listeners = make(map[uint64]func(Activity))
	}
	activity.listeners[id] = listener
	current := activity.current
	activity.Unlock()
	listener(current)
	var once sync.Once
	return func() {
		once.Do(func() {
			activity.Lock()
			defer activity.Unlock()
			delete(activity.listeners, id)
		})
	}
}
