package userData

import (
	"path/filepath"
	"strings"

	"github.com/luskaner/ageServer/common/game"
)

const backupSuffix = ".bak"
const serverSuffix = ".lan"

const prefix = "Games"
const aoe4Prefix = "My " + prefix

const (
	TypeActive = iota
	TypeServer
	TypeBackup
)

var suffixToType = map[string]int{
	backupSuffix: TypeBackup,
	serverSuffix: TypeServer,
}

type Data struct {
	typ  int
	path string
}

func (d *Data) Type() int {
	return d.typ
}

func (d *Data) Path() string {
	return d.path
}

type Path struct {
	path   string
	gameId string
}

func (u *Path) String() string {
	return u.path
}

func (u *Path) GameId() string {
	return u.gameId
}

func NewPath(path string, gameId string) *Path {
	var s string
	prefixToUse := prefix
	switch gameId {
	case game.AoE1:
		s = `Age of Empires DE`
	case game.AoE2:
		s = `Age of Empires 2 DE`
	case game.AoE3:
		s = `Age of Empires 3 DE`
	case game.AoE4:
		s = `Age of Empires IV`
		prefixToUse = aoe4Prefix
	case game.AoM:
		s = `Age of Mythology Retold`
	}
	return &Path{filepath.Join(path, prefixToUse, s), gameId}
}

func typ(path string) (typ int, ext string) {
	ext = filepath.Ext(path)
	var ok bool
	if typ, ok = suffixToType[ext]; !ok {
		typ = TypeActive
	}
	return
}

func suffix(typ int) string {
	switch typ {
	case TypeServer:
		return serverSuffix
	case TypeBackup:
		return backupSuffix
	default:
		return ""
	}
}

func TransformPath(path string, srcType int, dstType int) (ok bool, transformedPath string) {
	t, ext := typ(path)
	if t != srcType {
		ok = false
		return
	}
	if t == dstType {
		ok = true
		transformedPath = path
		return
	}
	ok = true
	transformedPath = strings.TrimSuffix(path, ext) + suffix(dstType)
	return
}
