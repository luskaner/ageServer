package gameLogs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGameAoE1PathsNoLogsDir(t *testing.T) {
	g := GameAoE1{}
	paths := g.Paths(t.TempDir())
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoE1PathsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Logs"), 0755); err != nil {
		t.Fatal(err)
	}
	g := GameAoE1{}
	paths := g.Paths(dir)
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoE1PathsWithStartupLog(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "Logs")
	if err := os.MkdirAll(logs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "StartupLog.txt"), []byte(" startup"), 0644); err != nil {
		t.Fatal(err)
	}
	g := GameAoE1{}
	paths := g.Paths(dir)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %v", paths)
	}
}

func TestGameAoE3PathsNoLogsDir(t *testing.T) {
	g := GameAoE3{}
	paths := g.Paths(t.TempDir())
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoE3PathsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Logs"), 0755); err != nil {
		t.Fatal(err)
	}
	g := GameAoE3{}
	paths := g.Paths(dir)
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoE3PathsWithFiles(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "Logs")
	if err := os.MkdirAll(logs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "Age3SessionData.txt"), []byte("sd"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "Age3Log.txt"), []byte("log"), 0644); err != nil {
		t.Fatal(err)
	}
	g := GameAoE3{}
	paths := g.Paths(dir)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
}

func TestGameAoMPathsNoLogsDir(t *testing.T) {
	g := GameAoM{}
	paths := g.Paths(t.TempDir())
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoMPathsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "temp", "Logs"), 0755); err != nil {
		t.Fatal(err)
	}
	g := GameAoM{}
	paths := g.Paths(dir)
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoMPathsWithFiles(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "temp", "Logs")
	if err := os.MkdirAll(logs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "mythsessiondata.txt"), []byte("sd"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "mythlog.txt"), []byte("log"), 0644); err != nil {
		t.Fatal(err)
	}
	g := GameAoM{}
	paths := g.Paths(dir)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
}

func TestGameAoE4PathsWithSessionDataAndWarnings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "session_data.txt"), []byte("sd"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "warnings.log"), []byte("warn"), 0644); err != nil {
		t.Fatal(err)
	}
	g := GameAoE4{}
	paths := g.Paths(dir)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
}

func TestGameAoE4PathsNoLogFilesDir(t *testing.T) {
	dir := t.TempDir()
	g := GameAoE4{}
	paths := g.Paths(dir)
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoE2PathsNoLogsDir(t *testing.T) {
	g := GameAoE2{}
	paths := g.Paths(t.TempDir())
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %v", paths)
	}
}

func TestGameAoE2PathsWithSessionData(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "Age2SessionData.txt"), []byte("sd"), 0644); err != nil {
		t.Fatal(err)
	}
	g := GameAoE2{}
	paths := g.Paths(dir)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %v", paths)
	}
}
