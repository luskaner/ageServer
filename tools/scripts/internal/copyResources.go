package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	c "scripts/internal/constants"
	"slices"
	"strings"

	"github.com/luskaner/ageServer/common/game"
	p "github.com/luskaner/ageServer/common/paths"
)

func BuildResourcePath(module string) string {
	return filepath.Join(c.BuildDir, module, p.ResourcesDir)
}

func ResourcePath(module string) string {
	return filepath.Join(module, p.ResourcesDir)
}

func CopyMainConfig(module string) {
	if err := Cp(filepath.Join(ResourcePath(module), c.ConfigFileName), filepath.Join(BuildResourcePath(module), c.ConfigFileName)); err != nil {
		log.Fatal(err)
	}
}

func CopyGameConfigs(module string) {
	CopyGameConfigsTo(module, "")
}

// CopyGameConfigsTo installs one config.<game>.toml per supported game under
// subdir of the module's resources folder. The launcher and the battle server
// manager read them from resources itself; the server reads them from
// resources/config, next to config.toml, which is where the release puts them.
func CopyGameConfigsTo(module, subdir string) {
	CopyGameConfigsToWithTemplate(module, subdir, filepath.Join(ResourcePath(module), fmt.Sprintf(c.GameConfigFileName, "game")))
}

// CopyGameConfigsToWithTemplate is like CopyGameConfigsTo but allows specifying
// a custom template source path (e.g., for server where the template lives in
// resources/config/config.game.toml instead of resources/config.game.toml).
func CopyGameConfigsToWithTemplate(module, subdir, templateSrc string) {
	dstPath := filepath.Join(BuildResourcePath(module), subdir)
	MkdirP(dstPath)
	for g := range game.SupportedGames.Iter() {
		dst := filepath.Join(dstPath, fmt.Sprintf(c.GameConfigFileName, g))
		if err := Cp(templateSrc, dst); err != nil {
			log.Fatal(err)
		}
	}
	removeStaleGameConfigs(dstPath)
}

func removeStaleGameConfigs(dstPath string) {
	entries, err := os.ReadDir(dstPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == c.ConfigFileName || !strings.HasPrefix(name, c.ConfigPrefix) || !strings.HasSuffix(name, c.ConfigExtension) {
			continue
		}
		gameId := strings.TrimSuffix(strings.TrimPrefix(name, c.ConfigPrefix), c.ConfigExtension)
		if game.SupportedGames.Contains(gameId) {
			continue
		}
		if err := os.Remove(filepath.Join(dstPath, name)); err != nil {
			log.Fatal(err)
		}
	}
}

// SyncDir mirrors src into dst, copying files and directories and removing
// entries under dst that no longer exist under src. Top-level dst entries whose
// name is listed in preserve are left untouched (runtime generated data).
func SyncDir(src, dst string, preserve ...string) error {
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			MkdirP(target)
			return nil
		}
		return Cp(path, target)
	}); err != nil {
		return err
	}
	return removeStale(dst, src, preserve)
}

func removeStale(dst, src string, preserve []string) error {
	return filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dst {
			return nil
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		if slices.Contains(preserve, firstPathSegment(rel)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if _, err := os.Lstat(filepath.Join(src, rel)); os.IsNotExist(err) {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
}

func firstPathSegment(rel string) string {
	// Cut on os.PathSeparator rather than a literal: this tool also runs on
	// the Linux CI runners, where the separator is '/'.
	if before, _, ok := strings.Cut(rel, string(os.PathSeparator)); ok {
		return before
	}
	return rel
}
