package appx

import (
	"os"
	"path/filepath"
	"unsafe"

	"github.com/luskaner/ageServer/common/game"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	appNamePrefix  = "Microsoft."
	appPublisherId = "8wekyb3d8bbwe"
)

var (
	modkernelbase                   = windows.NewLazyDLL("kernelbase.dll")
	procFindPackagesByPackageFamily = modkernelbase.NewProc("FindPackagesByPackageFamily")
)

type Game struct {
	familyName string
	fullName   string
}

func NewGame(gameId string) (game *Game, ok bool) {
	if !appxAPIAvailable() {
		return
	}
	familyName := appNamePrefix + appNameSuffix(gameId) + "_" + appPublisherId
	var fullName string
	ok, fullName = packageFamilyNameToFullName(familyName)
	if !ok {
		return
	}
	game = &Game{
		familyName,
		fullName,
	}
	ok = true
	return
}

func appxAPIAvailable() bool {
	return procFindPackagesByPackageFamily.Find() == nil
}

const (
	PackageFilterHead       uint32 = 0x10
	ErrorSuccess            uint32 = 0
	ErrorInsufficientBuffer uint32 = 122
)

func appNameSuffix(gameTitle string) string {
	switch gameTitle {
	case game.AoE1:
		return "Darwin"
	case game.AoE2:
		return "MSPhoenix"
	case game.AoE3:
		return "MSGPBoston"
	case game.AoE4:
		return "Cardinal"
	// FIXME: Add common.GameAoM
	default:
		return ""
	}
}

func packageFamilyNameToFullName(packageFamilyName string) (ok bool, fullName string) {
	pfnUTF16, err := windows.UTF16PtrFromString(packageFamilyName)
	if err != nil {
		return
	}

	var count uint32
	var bufferLength uint32

	ret, _, _ := procFindPackagesByPackageFamily.Call(
		uintptr(unsafe.Pointer(pfnUTF16)),
		uintptr(PackageFilterHead),
		uintptr(unsafe.Pointer(&count)),
		0,
		uintptr(unsafe.Pointer(&bufferLength)),
		0,
		0,
		0,
	)

	result := uint32(ret)

	if result == ErrorInsufficientBuffer {
		fullNames := make([]*uint16, count)
		buffer := make([]uint16, bufferLength)

		ret, _, _ = procFindPackagesByPackageFamily.Call(
			uintptr(unsafe.Pointer(pfnUTF16)),
			uintptr(PackageFilterHead),
			uintptr(unsafe.Pointer(&count)),
			uintptr(unsafe.Pointer(&fullNames[0])),
			uintptr(unsafe.Pointer(&bufferLength)),
			uintptr(unsafe.Pointer(&buffer[0])),
			0,
			0,
		)

		if uint32(ret) == ErrorSuccess && count > 0 {
			fullName = windows.UTF16PtrToString(fullNames[0])
			ok = true
		}
	}
	return
}

func (g Game) FamilyName() string {
	return g.familyName
}

func (g Game) FullName() string {
	return g.fullName
}

func (g Game) basePath() (ok bool, installLocation string) {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`SOFTWARE\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\Repository\Packages\`+g.fullName,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return
	}
	defer func(key registry.Key) {
		_ = key.Close()
	}(key)
	installLocation, _, err = key.GetStringValue("PackageRootFolder")
	if err != nil {
		return
	}
	ok = true
	return
}

func (g Game) Path() (folder string) {
	if ok, path := g.basePath(); ok {
		folder = filepath.Join(path, "Game")
		if f, err := os.Stat(folder); err != nil || !f.IsDir() {
			folder = ""
		}
	}
	return
}
