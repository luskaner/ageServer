package gameLogs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/luskaner/ageServer/launcher-common/userData"
)

const elementSepDotGlob = "."
const minElementGlob = "??"
const minElementSepGlob = minElementGlob + elementSepDotGlob
const dateDotGlob = minElementGlob + minElementSepGlob + minElementSepGlob + minElementGlob
const dateTimePrefixGlob = dateDotGlob + "-" + minElementGlob
const dateTimeGlob = dateTimePrefixGlob + elementSepDotGlob + minElementSepGlob + minElementGlob
const dateTimeNoDotGlob = dateTimePrefixGlob + minElementSepGlob + minElementGlob

type Game interface {
	Paths(path string) []string
}

var gameIdToGame = map[string]Game{
	game.AoE1: GameAoE1{},
	game.AoE2: GameAoE2{},
	game.AoE3: GameAoE3{},
	game.AoE4: GameAoE4{},
	game.AoM:  GameAoM{},
}

func addNewestPath(basePath string, tmpPaths []string, checkFn func(info os.FileInfo) bool, paths *[]string) {
	var filesInfo []os.FileInfo
	for _, path := range tmpPaths {
		if f, err := os.Stat(path); err == nil && checkFn(f) {
			filesInfo = append(filesInfo, f)
		}
	}
	if len(filesInfo) > 0 {
		sortByModTime(&filesInfo)
		*paths = append(*paths, filepath.Join(basePath, filesInfo[0].Name()))
	}
}

func copyFileContent(src, dst string) (ok bool) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer func(sourceFile *os.File) {
		_ = sourceFile.Close()
	}(sourceFile)

	destinationFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func(destinationFile *os.File) {
		_ = destinationFile.Close()
	}(destinationFile)

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return
	}
	if info, err := sourceFile.Stat(); err == nil {
		_ = os.Chmod(dst, info.Mode())
	}
	return true
}

func copyPathToDir(srcPath, dstDir string) (ok bool) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return
	}
	baseName := filepath.Base(srcPath)
	finalDstPath := filepath.Join(dstDir, baseName)
	if !info.IsDir() {
		if err = os.MkdirAll(dstDir, 0755); err != nil {
			return
		}
		return copyFileContent(srcPath, finalDstPath)
	}
	if err = os.MkdirAll(finalDstPath, info.Mode()); err != nil {
		return
	}
	if err = filepath.WalkDir(srcPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		newDstPath := filepath.Join(finalDstPath, relPath)
		if d.IsDir() {
			return os.MkdirAll(newDstPath, d.Type().Perm())
		} else if !copyFileContent(path, newDstPath) {
			return fmt.Errorf("copyPathToDir: file %s already exists and is not a directory", path)
		}
		return nil
	}); err == nil {
		ok = true
	}
	return
}

func sortByModTime(filesInfo *[]os.FileInfo) {
	slices.SortFunc(*filesInfo, func(first, second os.FileInfo) int {
		firstTime := first.ModTime()
		secondTime := second.ModTime()
		if firstTime.After(secondTime) {
			return -1
		}
		if secondTime.After(firstTime) {
			return 1
		}
		return 0
	})
}

func CopyGameLogs(gameId string, basePath string, logRoot string) {
	cmdlog.Step("Copying game logs...")
	if g, ok := gameIdToGame[gameId]; ok {
		finalPath := userData.NewPath(basePath, gameId).String()
		paths := g.Paths(finalPath)
		for _, path := range paths {
			str := fmt.Sprintf("\tCopying %s... ", path)
			if copyPathToDir(path, logRoot) {
				str += "OK"
			} else {
				str += "KO"
			}
			cmdlog.Println(str)
		}
	}
}
