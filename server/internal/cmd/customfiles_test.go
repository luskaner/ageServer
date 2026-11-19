package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/paths"
	"github.com/luskaner/ageServer/server/internal"
)

// TestCustomFilesConfigReachesTheLibrary pins the handoff: the [CustomFiles]
// section no longer configures a per-game store of the host, it is passed to
// the embedded library, and it survives [Mods] being disabled.
func TestCustomFilesConfigReachesTheLibrary(t *testing.T) {
	oldFiles := internal.CustomFilesConfig
	oldModsConfig := internal.ModsConfig
	oldModsEnabled := internal.ModsEnabled
	t.Cleanup(func() {
		internal.CustomFilesConfig = oldFiles
		internal.ModsConfig = oldModsConfig
		internal.ModsEnabled = oldModsEnabled
	})

	cfg := &internal.Configuration{
		CustomFiles: internal.CustomFiles{
			Enabled:      true,
			Folder:       "match-store",
			MaxFileSize:  "32MB",
			MaxTotalSize: "256MB",
			Expiry:       "12h",
		},
	}
	if err := applyCustomFilesConfig(cfg); err != nil {
		t.Fatalf("applyCustomFilesConfig: %v", err)
	}
	if err := applyModsConfig(cfg); err != nil {
		t.Fatalf("applyModsConfig: %v", err)
	}

	got := internal.ModsConfig.MatchContent
	if !got.Enabled || got.Folder != "match-store" || got.Expiry != 12*time.Hour {
		t.Fatalf("match content config = %+v", got)
	}
	if got.MaxFileSize != 32<<20 || got.MaxTotalSize != 256<<20 {
		t.Fatalf("match content sizes = %+v", got)
	}
	// With the mods tree off the library gets no on-disk database, so a
	// MatchContent-only server does not create mods.db.
	if internal.ModsConfig.DBPath != "" || internal.ModsConfig.UploadDir != "" {
		t.Fatalf("mods paths resolved while disabled: %+v", internal.ModsConfig)
	}
}

// TestCustomFilesAutoFolderUsesResources pins the default folder of the match
// content store: it lives under resources, like the mods paths, instead of the
// system temporary directory the library uses on its own.
func TestCustomFilesAutoFolderUsesResources(t *testing.T) {
	oldFiles := internal.CustomFilesConfig
	t.Cleanup(func() { internal.CustomFilesConfig = oldFiles })

	want := filepath.Join(paths.ResourcesDir, "custom-files")
	for _, folder := range []string{"", "auto"} {
		cfg := &internal.Configuration{CustomFiles: internal.CustomFiles{
			Enabled:      true,
			Folder:       folder,
			MaxFileSize:  "32MB",
			MaxTotalSize: "256MB",
			Expiry:       "12h",
		}}
		if err := applyCustomFilesConfig(cfg); err != nil {
			t.Fatalf("applyCustomFilesConfig(%q): %v", folder, err)
		}
		if got := internal.CustomFilesConfig.Folder; got != want {
			t.Fatalf("folder for %q = %q, want %q", folder, got, want)
		}
	}
}
