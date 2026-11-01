//go:build darwin && !cgo

package tray

// This build intentionally avoids the darwin systray implementation, because the
// current dependency chain pulls in github.com/go-webgpu/goffi, whose fakecgo
// symbols clash with github.com/ebitengine/purego on macOS when CGO is disabled.
// We still keep the tray abstraction alive so the agent can start without a tray
// on systems where a native notification area is unavailable or unsupported.

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

type systrayIcon struct {
	icon *systrayIcon
}

func (s *systrayIcon) SetIcon([]byte)    {}
func (s *systrayIcon) SetTooltip(string) {}
func (s *systrayIcon) SetMenu(trayMenu)  {}
func (s *systrayIcon) Show()             {}
func (s *systrayIcon) Run() error        { return nil }
func (s *systrayIcon) Remove()           {}

type systrayMenu struct {
	menu *systrayMenu
}

func (s *systrayMenu) Add(string, func()) trayItem { return &systrayItem{} }
func (s *systrayMenu) AddSeparator()               {}

type systrayItem struct{}

func (s *systrayItem) SetLabel(string) {}
func (s *systrayItem) SetDisabled(bool) {}

var (
	systrayNew     = func() *systrayIcon { return &systrayIcon{} }
	systrayNewMenu = func() *systrayMenu { return &systrayMenu{} }
)

var (
	_ trayIcon = (*systrayIcon)(nil)
	_ trayMenu = (*systrayMenu)(nil)
	_ trayItem = (*systrayItem)(nil)
)
