package process

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

const (
	procAllPids = 1
)

const (
	KernProcargs2 = 49
)

type ProcPidBsdInfo struct {
	PbiFlags     uint32
	PbiStatus    uint32
	PbiXstatus   uint32
	PbiPid       uint32
	PbiPpid      uint32
	PbiUid       uint32
	PbiGid       uint32
	PbiRuid      uint32
	PbiRgid      uint32
	PbiSvuid     uint32
	PbiSvgid     uint32
	Rfu1         uint32
	PbiComm      [16]byte
	PbiName      [1024]byte
	PbiNfiles    uint32
	PbiPgid      uint32
	PbiPjobc     uint32
	PbiTdev      uint32
	PbiTpgid     uint32
	PbiNice      uint32
	PbiStartSec  uint64
	PbiStartUsec uint64
}

var (
	libHandle       uintptr
	loadOnce        sync.Once
	loadErr         error
	procPidinfoPtr  func(int32, int32, uint64, uintptr, int32) int32
	procListpidsPtr func(uint32, uint32, uintptr, int32) int32
	procPidpathPtr  func(int32, uintptr, uint32) int32
)

var wineBinaries = []string{"wine", "wine64", "wine-preloader", "wine64-preloader", "wineloader"}

func loadLib() {
	loadOnce.Do(func() {
		h, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW)
		if err != nil {
			loadErr = fmt.Errorf("dlopen libSystem failed: %w", err)
			return
		}
		libHandle = h
		purego.RegisterLibFunc(&procPidinfoPtr, libHandle, "proc_pidinfo")
		purego.RegisterLibFunc(&procListpidsPtr, libHandle, "proc_listpids")
		purego.RegisterLibFunc(&procPidpathPtr, libHandle, "proc_pidpath")
	})
}

func GetProcessStartTime(pid int) (int64, error) {
	loadLib()
	if loadErr != nil {
		return 0, loadErr
	}
	if pid != os.Getpid() {
		return 0, unix.EPERM
	}
	return time.Now().UnixMicro(), nil
}

func getProcessArgs(pid int) ([]string, error) {
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, fmt.Errorf("sysctl kern.procargs2 failed: %w", err)
	}
	if len(buf) < 4 {
		return nil, fmt.Errorf("kern.procargs2 buffer too small")
	}
	argc := int(binary.LittleEndian.Uint32(buf[0:4]))
	if argc <= 0 {
		return nil, nil
	}
	i := 4
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	for i < len(buf) && buf[i] == 0 {
		i++
	}
	args := make([]string, 0, argc)
	for len(args) < argc && i < len(buf) {
		start := i
		for i < len(buf) && buf[i] != 0 {
			i++
		}
		args = append(args, strings.ReplaceAll(string(buf[start:i]), `\`, `/`))
		i++
	}
	if args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}
	return args, nil
}

func ProcessesByNames(names []string) map[string]*os.Process {
	processesPid := make(map[string]*os.Process)
	if len(names) == 0 {
		return processesPid
	}
	loadLib()
	if loadErr != nil || procListpidsPtr == nil || procPidpathPtr == nil {
		return processesPid
	}
	const maxPids = 16384
	pidBuf := make([]int32, maxPids)
	bufBytes := int32(len(pidBuf) * int(unsafe.Sizeof(pidBuf[0])))
	ret := procListpidsPtr(procAllPids, 0, uintptr(unsafe.Pointer(&pidBuf[0])), bufBytes)
	if ret <= 0 {
		return processesPid
	}
	numPids := int(ret) / int(unsafe.Sizeof(pidBuf[0]))
	if numPids > len(pidBuf) {
		numPids = len(pidBuf)
	}
	namesLeft := mapset.NewThreadUnsafeSet[string](names...)
	var err error
	for i := 0; !namesLeft.IsEmpty() && i < numPids; i++ {
		pid := pidBuf[i]
		if pid <= 0 {
			continue
		}
		var args []string
		if args, err = getProcessArgs(int(pid)); err != nil {
			continue
		}
		if len(args) == 0 {
			continue
		}
		args2OrMore := len(args) > 1
		baseName := filepath.Base(args[0])
		var matchingName string
		for name := range namesLeft.Iter() {
			if strings.HasSuffix(name, `.exe`) {
				if args2OrMore && filepath.Base(args[1]) == name && slices.Contains(wineBinaries, baseName) {
					matchingName = name
					break
				}
			} else if baseName == name {
				matchingName = name
				break
			}
		}
		if matchingName != "" {
			if localProc, err := FindProcess(int(pid)); err == nil {
				processesPid[matchingName] = localProc
				namesLeft.Remove(matchingName)
			}
		}
	}
	return processesPid
}
