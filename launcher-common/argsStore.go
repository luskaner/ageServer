package launcher_common

import (
	"io"
	"os"
	"strings"
	"sync"

	mapset "github.com/deckarep/golang-set/v3"
)

const argsStoreSep = "|"

// argsStoreByteToStringSlice splits stored content, dropping empty entries so
// empty files or leading/trailing separators never produce phantom flags.
func argsStoreByteToStringSlice(s []byte) []string {
	parts := strings.Split(string(s), argsStoreSep)
	flags := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			flags = append(flags, part)
		}
	}
	return flags
}

type ArgsStore struct {
	filePath string
	mutex    sync.RWMutex
}

func NewArgsStore(filePath string) *ArgsStore {
	return &ArgsStore{
		filePath: filePath,
	}
}

func (s *ArgsStore) Load() (err error, flags []string) {
	var content []byte
	s.mutex.RLock()
	func() {
		defer s.mutex.RUnlock()
		content, err = os.ReadFile(s.filePath)
	}()
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	if len(content) == 0 {
		// An empty file (e.g. created but never written) must not yield a
		// phantom empty-string flag.
		return
	}
	flags = argsStoreByteToStringSlice(content)
	return
}

func (s *ArgsStore) Store(flags []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	f, err := os.OpenFile(s.filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	var content []byte
	content, err = io.ReadAll(f)
	if err != nil {
		return err
	}
	flagsToSave := argsStoreByteToStringSlice(content)
	existingFlags := mapset.NewSet[string](flagsToSave...)
	for _, flag := range flags {
		if flag == "" {
			continue
		}
		if !existingFlags.ContainsOne(flag) {
			flagsToSave = append(flagsToSave, flag)
			existingFlags.Add(flag)
		}
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	joined := strings.Join(flagsToSave, argsStoreSep)
	_, err = f.WriteString(joined)
	if err != nil {
		return err
	}
	// Truncate in case new content is shorter than previous file (e.g. leading
	// separators or filtered empty flags produced a shorter join).
	if err = f.Truncate(int64(len(joined))); err != nil {
		return err
	}
	return err
}

func (s *ArgsStore) Delete() error {
	s.mutex.Lock()
	var err error
	func() {
		defer s.mutex.Unlock()
		err = os.Remove(s.filePath)
	}()
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
