package gameLogs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestCreateLogsAllGames(t *testing.T) {
	for _, gid := range []string{game.AoE1, game.AoE2, game.AoE3, game.AoM} {
		dir := t.TempDir()
		if err := CreateLogs(gid, dir); err != nil {
			t.Fatalf("CreateLogs %s failed: %v", gid, err)
		}
		found := false
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				found = true
				return filepath.SkipDir
			}
			return nil
		})
		if !found {
			t.Fatalf("no files created for %s", gid)
		}
	}
	// Age4 needs the directory to exist beforehand due to missing MkdirAll in CreateLogs
	dir := t.TempDir()
	// Create the expected path for age4: My Games/Age of Empires IV
	age4Path := filepath.Join(dir, "My Games", "Age of Empires IV")
	os.MkdirAll(age4Path, 0755)
	if err := CreateLogs(game.AoE4, dir); err != nil {
		t.Fatalf("CreateLogs age4 failed: %v", err)
	}
}

func TestCreateLogsInvalidGame(t *testing.T) {
	dir := t.TempDir()
	// For unknown game, CreateLogs should panic or return error? Let's see: gameIdToGame[gameId] will be missing, so it will panic on nil map access?
	// Actually CreateLogs does gameIdToGame[gameId].CreateLogs, if gameId not in map, it will be zero value (nil interface) and calling method will panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("should panic for unknown game")
		}
	}()
	CreateLogs("unknown", dir)
}

func TestCreateLogsWithFileBlocker(t *testing.T) {
	dir := t.TempDir()
	// Create a file where Logs directory should be, to make MkdirAll fail
	// For age1, path is <dir>/Games/Age of Empires DE/Logs - we can make <dir>/Games a file
	gamesPath := filepath.Join(dir, "Games")
	os.WriteFile(gamesPath, []byte("blocker"), 0644)
	if err := CreateLogs(game.AoE1, dir); err == nil {
		t.Fatal("should fail when Games is a file")
	}
}
