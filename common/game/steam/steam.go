package steam

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andygrunwald/vdf"
	"github.com/luskaner/ageServer/common/game"
)

type ConfigPathFn func() string

type PathTranslateFn func(s string) string

type Game struct {
	appId       string
	libraryPath string
}

func NewGame(gameId string) (game *Game, ok bool) {
	return NewCustomGame(gameId, ConfigPath, ConfigPathAlt, func(s string) string {
		return s
	})
}

func NewCustomGame(gameId string, configPathFn ConfigPathFn, configPathAltFn ConfigPathFn, pathTransFn PathTranslateFn) (game *Game, ok bool) {
	id := appId(gameId)
	if libraryPath := libraryFolder(id, configPathFn, configPathAltFn); libraryPath != "" {
		if transLibraryPath := pathTransFn(libraryPath); transLibraryPath != "" {
			game = &Game{id, transLibraryPath}
			ok = true
		}
	}
	return
}

func appId(id string) string {
	switch id {
	case game.AoE1:
		return "1017900"
	case game.AoE2:
		return "813780"
	case game.AoE3:
		return "933110"
	case game.AoE4:
		return "1466860"
	case game.AoM:
		return "1934680"
	default:
		return ""
	}
}

func (g Game) OpenUri() string {
	return fmt.Sprintf("steam://rungameid/%s", g.appId)
}

func openLibraryFolder(path string) (f *os.File, err error) {
	return os.Open(filepath.Join(path, "config", "libraryfolders.vdf"))
}

func libraryFolder(appId string, configPathFn ConfigPathFn, configPathAltFn ConfigPathFn) (folder string) {
	p := configPathFn()
	if p == "" {
		return
	}
	f, err := openLibraryFolder(p)
	if err != nil {
		// Likely a Steam Emulator messed up the config, try the alternative way
		p = configPathAltFn()
		if f, err = openLibraryFolder(p); err != nil {
			return
		}
	}
	defer func() {
		_ = f.Close()
	}()
	parser := vdf.NewParser(f)
	var data map[string]any
	data, err = parser.Parse()
	if err != nil {
		return
	}
	libraryFolders, ok := data["libraryfolders"].(map[string]any)
	if !ok {
		return
	}
	var folderMap map[string]any
	for _, dir := range libraryFolders {
		folderMap, ok = dir.(map[string]any)
		if !ok {
			continue
		}
		var apps map[string]any
		apps, ok = folderMap["apps"].(map[string]any)
		if !ok {
			continue
		}
		if _, exists := apps[appId]; exists {
			if path, ok := folderMap["path"].(string); ok {
				return path
			}
		}
	}
	return
}

func (g Game) Path() (folder string) {
	basePath := filepath.Join(g.libraryPath, "steamapps")
	f, err := os.Open(filepath.Join(basePath, fmt.Sprintf("appmanifest_%s.acf", g.appId)))
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()
	parser := vdf.NewParser(f)
	var data map[string]any
	data, err = parser.Parse()
	if err != nil {
		return
	}
	appState, ok := data["AppState"].(map[string]any)
	if !ok {
		return
	}
	installDir, ok := appState["installdir"].(string)
	if !ok {
		return
	}
	folder = filepath.Join(basePath, "common", installDir)
	if f, err := os.Stat(folder); err != nil || !f.IsDir() {
		return ""
	}
	return
}
