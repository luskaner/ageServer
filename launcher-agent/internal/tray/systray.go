//go:build !darwin || cgo

package tray

import "github.com/gogpu/systray"

// systrayNew and systrayNewMenu are the two constructors, behind vars so that a
// test can replace them together with the interfaces rather than reaching for
// the library.
var (
	systrayNew     = systray.New
	systrayNewMenu = systray.NewMenu
)

// The three interfaces below are the whole of what the agent asks of a system
// tray, and nothing more.
//
// They exist so that the part of this package worth being sure about — what the
// menu says, when it greys itself out, what Exit does and what happens when the
// icon refuses to come up — can be exercised without a desktop. Only the calls
// are the library's; deciding them is not, and a test on a machine with no
// notification area would otherwise be a test of whether CI has one.

// trayIcon is one icon in the notification area.
type trayIcon interface {
	SetIcon(png []byte)
	SetTooltip(text string)
	SetMenu(menu trayMenu)
	Show()
	Run() error
	Remove()
}

// trayMenu is the menu that opens when the icon is right clicked.
type trayMenu interface {
	Add(label string, onClick func()) trayItem
	AddSeparator()
}

// trayItem is one row of it.
type trayItem interface {
	SetLabel(label string)
	SetDisabled(disabled bool)
}

// systrayIcon is trayIcon over *systray.SystemTray.
//
// The library's setters return the icon so that they can be chained. An
// interface cannot: a method that returns its own interface type is a different
// method from one that returns the concrete type. The chaining is therefore
// dropped here, once, instead of at every call site in this package.
type systrayIcon struct {
	icon *systray.SystemTray
}

func (s *systrayIcon) SetIcon(png []byte)     { s.icon.SetIcon(png) }
func (s *systrayIcon) SetTooltip(text string) { s.icon.SetTooltip(text) }
func (s *systrayIcon) SetMenu(menu trayMenu) {
	s.icon.SetMenu(toSystrayMenu(menu))
}
func (s *systrayIcon) Show()      { s.icon.Show() }
func (s *systrayIcon) Run() error { return s.icon.Run() }
func (s *systrayIcon) Remove()    { s.icon.Remove() }

// systrayMenu is trayMenu over *systray.Menu.
type systrayMenu struct {
	menu *systray.Menu
}

func toSystrayMenu(menu trayMenu) *systray.Menu {
	if concrete, ok := menu.(*systrayMenu); ok {
		return concrete.menu
	}
	// A test's menu reaches here. It is handed to a tray icon that is itself a
	// test's, so returning nil is the honest answer: there is no native menu to
	// build from it.
	return nil
}

func (s *systrayMenu) Add(label string, onClick func()) trayItem {
	return &systrayItem{item: s.menu.Add(label, onClick)}
}
func (s *systrayMenu) AddSeparator() { s.menu.AddSeparator() }

// systrayItem is trayItem over *systray.MenuItem.
type systrayItem struct {
	item *systray.MenuItem
}

func (s *systrayItem) SetLabel(label string)     { s.item.SetLabel(label) }
func (s *systrayItem) SetDisabled(disabled bool) { s.item.SetDisabled(disabled) }

var (
	_ trayIcon = (*systrayIcon)(nil)
	_ trayMenu = (*systrayMenu)(nil)
	_ trayItem = (*systrayItem)(nil)
)
