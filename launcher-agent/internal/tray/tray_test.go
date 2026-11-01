package tray

import (
	"bytes"
	"image"
	"image/png"
	"sync"
	"testing"
	"time"

	"github.com/luskaner/ageServer/launcher-agent/internal"
)

// panickingIcon is a trayIcon that panics on every operation. It simulates a
// platform with no notification area at all.
type panickingIcon struct{}

func (panickingIcon) SetIcon([]byte)        { panic("no shell") }
func (panickingIcon) SetTooltip(string)      { panic("no shell") }
func (panickingIcon) SetMenu(trayMenu)       { panic("no shell") }
func (panickingIcon) Show()                  { panic("no shell") }
func (panickingIcon) Run() error             { panic("no shell") }
func (panickingIcon) Remove()                { panic("no shell") }

func bytesReader(data []byte) *bytes.Reader { return bytes.NewReader(data) }

// distinctColours counts the different colours an image is made of, which is how
// a drawing is told from a fill without asserting on exact pixels.
func distinctColours(img image.Image) int {
	seen := make(map[[4]uint32]struct{})
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			seen[[4]uint32{r >> 8, g >> 8, b >> 8, a >> 8}] = struct{}{}
		}
	}
	return len(seen)
}

// fakes stand in for the notification area. They record what was asked of them so
// that a test can assert on the menu a user would see, without one ever being put
// on the desktop of whoever is running the tests.
type fakes struct {
	mu sync.Mutex

	icon    []byte
	shown   bool
	removed bool
	runErr  error

	tooltip string
	menu    *fakeMenu

	// loop is released when the icon's message loop is asked to return.
	loop    chan struct{}
	running bool
}

type fakeMenu struct {
	items []*fakeItem
}

type fakeItem struct {
	label    string
	disabled bool
	onClick  func()
}

func newFakes() *fakes {
	return &fakes{loop: make(chan struct{})}
}

func (f *fakes) SetIcon(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.icon = data
}

func (f *fakes) SetTooltip(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tooltip = text
}

func (f *fakes) SetMenu(menu trayMenu) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.menu, _ = menu.(*fakeMenu)
}

func (f *fakes) Show() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = true
}

// Run stands in for the platform's message loop: it does nothing until the icon
// is destroyed, which is exactly what the real one does.
func (f *fakes) Run() error {
	f.mu.Lock()
	f.running = true
	f.mu.Unlock()
	<-f.loop
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()
	return f.runErr
}

func (f *fakes) Remove() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = true
	select {
	case <-f.loop:
	default:
		close(f.loop)
	}
}

func (f *fakeItem) SetLabel(label string) { f.label = label }

func (f *fakeItem) SetDisabled(disabled bool) { f.disabled = disabled }

func (f *fakeMenu) Add(label string, onClick func()) trayItem {
	item := &fakeItem{label: label, onClick: onClick}
	f.items = append(f.items, item)
	return item
}

func (f *fakeMenu) AddSeparator() {}

// install points the package at the fakes and returns the function that puts the
// originals back.
func (f *fakes) install(t *testing.T) func() {
	t.Helper()
	origIcon, origMenu := newIconFn, newMenuFn
	newIconFn = func() trayIcon { return f }
	newMenuFn = func() trayMenu { return &fakeMenu{} }
	return func() { newIconFn, newMenuFn = origIcon, origMenu }
}

// labels are the menu a user sees, with the separators left out because they
// carry no text and their position is fixed.
func (f *fakes) labels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.menu.items))
	for _, item := range f.menu.items {
		out = append(out, item.label)
	}
	return out
}

func (f *fakes) item(index int) *fakeItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.menu.items[index]
}

func (f *fakes) tooltipText() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tooltip
}

// The tray is the only thing on screen while a run works, and it has to say what
// the run is doing both on hover and in the menu, because those are the two ways
// a user reads an icon without having committed to opening anything.
func TestTrayShowsWhatTheAgentIsDoing(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	systemTray := Start("age2", func() {})
	internal.SetActivity("Reverting the configuration", "age2 — aoe2.exe (pid 11)")
	systemTray.Stop()

	labels := f.labels()
	if len(labels) != 3 {
		t.Fatalf("menu has %d rows, want 3: %v", len(labels), labels)
	}
	if labels[0] != "Reverting the configuration" {
		t.Errorf("first row is %q, want the current step", labels[0])
	}
	if labels[1] != "age2 — aoe2.exe (pid 11)" {
		t.Errorf("second row is %q, want the session", labels[1])
	}
	if !f.item(0).disabled || !f.item(1).disabled {
		t.Error("the two rows that report state must not be clickable")
	}
	if got := f.tooltipText(); got != Title+" — Reverting the configuration" {
		t.Errorf("tooltip is %q", got)
	}
}

// The two rows that say what is happening are rows and not buttons. A user who
// clicks one is asking a question, and there is nothing behind them to answer it.
func TestTrayRowsReportingStateCannotBeClicked(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	systemTray := Start("age2", func() {})
	defer systemTray.Stop()
	if f.item(0).disabled != true || f.item(1).disabled != true {
		t.Fatal("the rows reporting state are enabled")
	}
	if f.item(2).disabled {
		t.Error("Exit must be clickable: it is the only action the tray offers")
	}
}

// Exit has to reach the run, and it has to say what it is doing while it does.
//
// The run it asks to stop is not the tray's to end: the teardown has an order and
// an exactly-once guarantee that belong to the run, so the row only asks. The
// label is what the user is left looking at between the click and the icon going
// away, and "Exit" still saying "Exit" there is a row offering something that has
// already happened.
func TestTrayExitAsksTheRunToStop(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	var stops int
	systemTray := Start("age2", func() { stops++ })

	exit := f.item(2)
	exit.onClick()

	if stops != 1 {
		t.Fatalf("the run was asked to stop %d times, want 1", stops)
	}
	if got := f.labels()[2]; got != ExitDisabledLabel {
		t.Errorf("Exit now reads %q, want %q", got, ExitDisabledLabel)
	}
	if !exit.disabled {
		t.Error("Exit must not stay clickable after it has been used")
	}
	systemTray.Stop()
}

// A second click must not become a second teardown.
//
// This is reachable: the row is drawn by the window manager and the click arrives
// on the platform's thread, so the second one can be dispatched before the run
// has noticed the first.
func TestTrayExitOnlyAsksOnce(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	var stops int
	systemTray := Start("age2", func() { stops++ })
	exit := f.item(2)
	exit.onClick()
	exit.onClick()
	exit.onClick()

	if stops != 1 {
		t.Errorf("the run was asked to stop %d times, want 1", stops)
	}
	systemTray.Stop()
}

// Nothing is decided by whether an icon came up.
//
// A tray is decoration over a run that has to work without one, so the case where
// the platform gives nothing back is not a degenerate one to shrug at: every
// method has to survive it, because the exit path calls Stop on the tray of every
// run whether the icon is on screen or not. That a user cannot turn this off is
// what makes it a real requirement rather than a feature.
func TestTrayThatNeverGotAnIconIsInertAndSafe(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	// Replace the icon with one that panics on creation, which is what a platform
	// without a notification area does. The tray must still come back to the run.
	newIconFn = func() trayIcon { return &panickingIcon{} }
	defer func() { newIconFn = func() trayIcon { return &systrayIcon{icon: systrayNew()} } }()

	systemTray := Start("age2", func() { t.Error("nothing can click an icon that was never shown") })
	internal.SetActivity("Waiting for the game to start", "age2")
	// Both of these have to be no-ops rather than nil dereferences: they run on
	// the exit path of every run, tray or no tray.
	systemTray.Stop()
	systemTray.Stop()

	if f.shown {
		t.Error("an icon was shown for a run that asked for none")
	}
	if f.removed {
		t.Error("an icon that was never shown was removed")
	}
}

// The icon has to be a PNG the platform can actually load, and it is the only
// asset this package has: there is no file to fall back on.
func TestIconIsALoadablePNG(t *testing.T) {
	data := iconPNG()
	if len(data) == 0 {
		t.Fatal("the icon encoded to nothing")
	}
	img, err := png.Decode(bytesReader(data))
	if err != nil {
		t.Fatalf("the icon is not a PNG the shell could load: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != iconSize || bounds.Dy() != iconSize {
		t.Errorf("icon is %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), iconSize, iconSize)
	}
	// A tray icon that is one flat colour is a square. The globe is drawn by
	// contrasting parts of it, so an image with a single distinct colour means
	// the drawing did not happen.
	if distinctColours(img) < 2 {
		t.Error("the icon is a single flat colour")
	}
}

// The icon comes up blank if it is drawn entirely transparent, which is a shape
// the notification area shows as nothing at all. It has to have something opaque
// in the middle.
func TestIconHasSomethingOpaqueToShow(t *testing.T) {
	img, err := png.Decode(bytesReader(iconPNG()))
	if err != nil {
		t.Fatal(err)
	}
	middle := img.Bounds().Dx() / 2
	opaque := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if _, _, _, a := img.At(middle-1+x, middle-1+y).RGBA(); a >= 0xff00 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Error("nothing in the middle of the icon is opaque, so the shell would show nothing")
	}
}

// Start waits for the icon before returning, because the run publishes what it is
// doing from its very first step. A tray that came up after that would show the
// first step late, or not at all.
func TestStartReturnsWithTheIconAlreadyUp(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	systemTray := Start("age2", func() {})
	f.mu.Lock()
	shown := f.shown
	f.mu.Unlock()
	if !shown {
		t.Error("Start returned before the icon was shown")
	}

	// Stop must also finish the loop before returning, or the process could exit
	// with the notification area still holding an icon for a program that is
	// gone.
	systemTray.Stop()
	f.mu.Lock()
	running := f.running
	f.mu.Unlock()
	if running {
		t.Error("Stop returned while the message loop was still running")
	}
}

// A platform that cannot give us an icon must not take the run with it. The icon
// is decoration, so a failure in creating it is reported and everything else goes
// on as if there had never been an icon.
func TestStartReturnsOnIconPanic(t *testing.T) {
	newIconFn = func() trayIcon { return &panickingIcon{} }
	defer func() { newIconFn = func() trayIcon { return &systrayIcon{icon: systrayNew()} } }()

	// Start returning at all is the assertion: it waits on a channel the panicking
	// icon never reaches, so a Start that blocked here would hang the test rather
	// than fail it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		systemTray := Start("age2", func() {})
		internal.SetActivity("Waiting for the game to start", "age2")
		systemTray.Stop()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Start did not return after the icon failed to start")
	}
}

// The tray methods that the exit path calls — Stop twice, Update with activity —
// must all be no-ops rather than nil dereferences when the icon never came up.
// This is the same panic-as-icon scenario, but exercised through the methods
// rather than through Start's return.
func TestTrayMethodsAreSafeWhenNoIcon(t *testing.T) {
	f := newFakes()
	defer f.install(t)()

	// Replace the icon with one that panics on creation.
	newIconFn = func() trayIcon { return &panickingIcon{} }
	defer func() { newIconFn = func() trayIcon { return &systrayIcon{icon: systrayNew()} } }()

	systemTray := Start("age2", func() { t.Error("nothing can click an icon that was never shown") })
	internal.SetActivity("Waiting for the game to start", "age2")
	// Both of these have to be no-ops rather than nil dereferences: they run on
	// the exit path of every run, tray or no tray.
	systemTray.Stop()
	systemTray.Stop()

	if f.shown {
		t.Error("an icon was shown for a run that asked for none")
	}
	if f.removed {
		t.Error("an icon that was never shown was removed")
	}
}


