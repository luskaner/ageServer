package internal

import (
	"context"
	"net/http"
)

// ModsServer is the embedded ageAPIServer library mounted on the Age of
// Empires hosts. It serves the mods API and the match savegame endpoints. It
// is an interface so the router does not have to depend on the library
// directly.
type ModsServer interface {
	// Handler serves every mods endpoint, with its own routing and auth.
	Handler() http.Handler
	// Shutdown releases the database.
	Shutdown(context.Context) error
}

// ModsAPI is the constructed library. Nil means neither feature is on, so the
// hosts keep forwarding those paths where they used to: upstream.
var ModsAPI ModsServer

// ModsConfiguration is the resolved [Mods] section plus the [CustomFiles]
// settings the same library instance needs.
type ModsConfiguration struct {
	// DBPath is the SQLite file of the mods API.
	DBPath string
	// UploadDir holds the archives and screenshots Publish stores.
	UploadDir string
	// MaxUploadSize caps a request body of the mods API.
	MaxUploadSize int64
	// RequireAuthForRead makes the endpoints the spec declares anonymous
	// reject requests without credentials.
	RequireAuthForRead bool
	// MatchContent is the resolved [CustomFiles] section, served by the same
	// library instance.
	MatchContent CustomFilesConfiguration
}

// ModsEnabled reports whether the resolved configuration asked for the API.
var ModsEnabled bool

// ApiEnabled reports whether the API tree is served at all. With it false the
// hosts answer those paths with 404 instead of the embedded library or the
// official service, even when [Mods] is on: the tree is off as a whole.
//
// It starts on because that is the documented default of [Api] Enabled; startup
// replaces it with the resolved value before anything is served.
var ApiEnabled = true

// ModsConfig is the resolved configuration published at startup.
var ModsConfig ModsConfiguration
