package common

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseCommandArgsFromSliceJoined(t *testing.T) {
	args, err := ParseCommandArgsFromSlice(
		[]string{"--exe", "{GAME_PATH}", "--flag"},
		map[string]string{"GAME_PATH": "some/path"},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 {
		t.Fatalf("separateFields=false must yield exactly one arg, got %v", args)
	}
	if !strings.Contains(args[0], "some/path") || !strings.Contains(args[0], "--flag") {
		t.Fatalf("arg = %q", args[0])
	}
}

func TestEnhancedViperStringToStringSlice(t *testing.T) {
	got := EnhancedViperStringToStringSlice("value")
	if len(got) != 1 || got[0] != "value" {
		t.Fatalf("got %v", got)
	}
}

func TestParseCommandArgsFromSliceSeparateFields(t *testing.T) {
	args, err := ParseCommandArgsFromSlice(
		[]string{"--exe", "path with spaces"},
		nil,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) == 0 {
		t.Fatal("expected args")
	}
}

func TestParsePath(t *testing.T) {
	f, err := os.CreateTemp("", "parse_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	_, path, err := ParsePath([]string{f.Name()}, nil)
	if err != nil {
		t.Fatalf("ParsePath: %v", err)
	}
	if path == "" {
		t.Fatal("expected path")
	}
}

func TestParsePathInvalid(t *testing.T) {
	_, _, err := ParsePath([]string{"a", "b"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid path (len !=1)")
	}
}

func TestParsePathNotFound(t *testing.T) {
	_, _, err := ParsePath([]string{"C:\\nonexistent\\path\\12345\\file.txt"}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParsePathStatErrorInjected(t *testing.T) {
	orig := parseStatFn
	defer func() { parseStatFn = orig }()
	parseStatFn = func(string) (os.FileInfo, error) { return nil, errors.New("stat fail") }
	_, _, err := ParsePath([]string{"anything"}, nil)
	if err == nil || err.Error() != "stat fail" {
		t.Fatalf("expected stat fail, got %v", err)
	}
}

func TestUserAgentMentionsProject(t *testing.T) {
	ua := UserAgent()
	if !strings.Contains(ua, Name) {
		t.Fatalf("UserAgent %q missing project name %q", ua, Name)
	}
}

func TestAnnounceHeaderMatchesName(t *testing.T) {
	if AnnounceHeader == "" {
		t.Fatal("AnnounceHeader is empty")
	}
	if AnnounceVersionLatest < AnnounceVersion2 {
		t.Fatalf("AnnounceVersionLatest = %d, must be >= %d", AnnounceVersionLatest, AnnounceVersion2)
	}
}
