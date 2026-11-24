package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	ageAPIServer "github.com/luskaner/ageAPIServer"
	"github.com/luskaner/ageServer/common/bytesize"
	"github.com/luskaner/ageServer/common/paths"
	"github.com/luskaner/ageServer/server/internal"
)

// modsAutoFolder is the config value that resolves to the default folder.
const modsAutoFolder = "auto"

// applyModsConfig validates the Mods section and publishes the resolved values
// in the internal runtime globals, like applyCustomFilesConfig does.
func applyModsConfig(cfg *internal.Configuration) error {
	internal.ModsEnabled = cfg.Mods.Enabled
	internal.ModsConfig = internal.ModsConfiguration{
		DBPath:             cfg.Mods.DBPath,
		UploadDir:          cfg.Mods.UploadDir,
		RequireAuthForRead: cfg.Mods.RequireAuthForRead,
		MatchContent:       internal.CustomFilesConfig,
	}
	if !cfg.Mods.Enabled {
		// The library can still be built to serve [CustomFiles] alone. With the
		// mods tree unmounted its database is unused, so an empty path keeps it
		// in memory instead of resolving 'auto' into a file, and a typo in the
		// mods settings must not stop that server either.
		internal.ModsConfig.DBPath = ""
		internal.ModsConfig.UploadDir = ""
		return nil
	}
	size, err := bytesize.Parse(cfg.Mods.MaxUploadSize)
	if err != nil {
		return fmt.Errorf("MaxUploadSize: %w", err)
	}
	if size <= 0 {
		return fmt.Errorf("MaxUploadSize must be greater than zero")
	}
	internal.ModsConfig.MaxUploadSize = size

	folder := filepath.Join(paths.ResourcesDir, "mods")
	if internal.ModsConfig.DBPath == "" || internal.ModsConfig.DBPath == modsAutoFolder {
		internal.ModsConfig.DBPath = filepath.Join(folder, "mods.db")
	}
	if internal.ModsConfig.UploadDir == "" || internal.ModsConfig.UploadDir == modsAutoFolder {
		internal.ModsConfig.UploadDir = filepath.Join(folder, "uploads")
	}
	for _, dir := range []string{filepath.Dir(internal.ModsConfig.DBPath), internal.ModsConfig.UploadDir} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// newModsServerFn builds the mods API. It is a seam so the startup tests can
// fail it without opening a database.
var newModsServerFn = newModsServer

// newModsServer builds the embedded library with the resolved [Mods] and
// [CustomFiles] settings.
//
// The library's console sink is discarded: the host's own access log already
// records one line per request under the prefixed writer of the host, and a
// second line per request would duplicate it.
func newModsServer(cfg internal.ModsConfiguration) (internal.ModsServer, error) {
	server, err := ageAPIServer.New(ageAPIServer.Config{
		DBPath:      cfg.DBPath,
		UploadDir:   cfg.UploadDir,
		MaxBodySize: cfg.MaxUploadSize,
		MatchContent: ageAPIServer.MatchContentConfig{
			Enabled:      cfg.MatchContent.Enabled,
			Folder:       cfg.MatchContent.Folder,
			MaxFileSize:  cfg.MatchContent.MaxFileSize,
			MaxTotalSize: cfg.MatchContent.MaxTotalSize,
			Expiry:       cfg.MatchContent.Expiry,
		},
		Console: io.Discard,
	})
	if err != nil {
		return nil, err
	}
	return server, nil
}
