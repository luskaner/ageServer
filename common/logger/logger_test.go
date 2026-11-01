package commonLogger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Regression: Root.Open had no O_TRUNC, so rewriting a file with shorter
// content left the tail of the previous content behind (corrupted logs).
func TestRootOpenTruncatesPreviousContent(t *testing.T) {
	err, root := NewFile(t.TempDir(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	const short = "short payload"

	for _, content := range []string{strings.Repeat("X", 300), strings.Repeat("Y", 120), short} {
		f, openErr := root.Open("log")
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, err = f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()

		data, readErr := os.ReadFile(filepath.Join(root.Folder(), "log.txt"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(data) != content {
			t.Fatalf("file = %q (%d bytes), want exactly %q — stale tail survived", data, len(data), content)
		}
	}
}

// Regression: CloseFileLog read Buf.buffer without its mutex and skipped
// Close on write errors. It must snapshot under lock and stay consistent
// under concurrent logging (verified by -race).
func TestCloseFileLogDumpsBufferSafely(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "filelog.txt")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}

	oldLogger, oldFile := logger, file
	file = f
	Initialize(nil)
	Buf.buffer = bytes.Buffer{}
	defer func() {
		logger, file = oldLogger, oldFile
		Buf.buffer = bytes.Buffer{}
	}()

	const marker = "payload-marker"
	Printf(marker)

	var wg sync.WaitGroup
	// Several writers, so the writes really are concurrent with each other and
	// the race detector has something to look at.
	//
	// They are waited for rather than raced against the close. This test is
	// about what CloseFileLog promises about the buffer it took: that the
	// snapshot reached the file and that the buffer is empty afterwards. A
	// writer still running after the close refills the buffer, so the emptiness
	// assertion below is only meaningful once they have stopped, and a sleep used
	// to stand in for that, which made the test a report about how busy the
	// machine was.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				Printf("concurrent line %d", i)
			}
		}()
	}
	wg.Wait()

	CloseFileLog()
	_ = f.Sync()

	content, readErr := os.ReadFile(tmp)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(content), marker) || !strings.Contains(string(content), "concurrent line") {
		t.Fatalf("dumped log missing buffered content: %q", string(content))
	}
	sizeAfterFirst := len(content)

	// Idempotence: buffer was emptied under lock; a second call must not duplicate output.
	Buf.mu.Lock()
	empty := Buf.buffer.Len() == 0
	Buf.mu.Unlock()
	if !empty {
		t.Fatal("buffer must be emptied after CloseFileLog")
	}
	CloseFileLog()
	data2, _ := os.ReadFile(tmp)
	if len(data2) < sizeAfterFirst {
		t.Fatal("second CloseFileLog truncated the log")
	}
}

// Sanity for the buffer wrapper contract used by CloseFileLog.
func TestBufferWrapperWriteAndReset(t *testing.T) {
	var b bufferWrapper
	if n, err := b.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("Write = %d, %v", n, err)
	}
	b.mu.Lock()
	got := b.buffer.String()
	b.mu.Unlock()
	if got != "abc" {
		t.Fatalf("buffer = %q", got)
	}
	var _ io.Writer = &b // must keep satisfying io.Writer
}
