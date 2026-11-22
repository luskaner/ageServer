module changeme

go 1.27.4

require (
	github.com/luskaner/ageServer/common v0.0.0-20261115212246-f9bde3f2fcd3
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/wailsapp/wails/v3 v3.1.0
)

replace github.com/wailsapp/wails/v3 => github.com/luskaner/wails/v3 v3.1.0

require (
	github.com/coder/websocket v1.8.15 // indirect
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.4 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
)
