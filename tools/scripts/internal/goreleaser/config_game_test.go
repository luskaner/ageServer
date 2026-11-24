package goreleaser

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type generatedArchive struct {
	ID    string `yaml:"id"`
	Files []struct {
		Source      string `yaml:"src"`
		Destination string `yaml:"dst"`
	} `yaml:"files"`
}

// hasGameConfig reports whether an archive ships the per-game template at the
// expected destination for every supported game.
func hasGameConfig(archive generatedArchive, destination string) (bool, string) {
	destinations := make(map[string]bool, len(archive.Files))
	for _, file := range archive.Files {
		if file.Source == "server/resources/config.game.toml" {
			destinations[file.Destination] = true
		}
	}
	for _, gameID := range []string{"age1", "age2", "age3", "age4", "athens"} {
		want := strings.Replace(destination, "{{game}}", gameID, 1)
		if !destinations[want] {
			return false, want
		}
	}
	return true, ""
}

// The generated release configuration has to ship a config.<game>.toml next to
// config.toml. The server reads the per-game settings from those files, so a
// template that only exists in the repository would leave every packaged
// server without them, and nothing else would notice.
func TestGeneratedServerArchivesShipGameConfigs(t *testing.T) {
	dir := t.TempDir()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := Generate(); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Archives []generatedArchive `yaml:"archives"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatalf("parse generated configuration: %v", err)
	}

	serverArchives := 0
	fullChecked := false
	for _, archive := range project.Archives {
		var destination string
		switch {
		case strings.HasPrefix(archive.ID, "server_"):
			serverArchives++
			destination = "resources/config/config.{{game}}.toml"
		case strings.HasPrefix(archive.ID, "full_"):
			fullChecked = true
			destination = "server/resources/config/config.{{game}}.toml"
		default:
			continue
		}
		if ok, missing := hasGameConfig(archive, destination); !ok {
			t.Errorf("archive %q does not ship %q", archive.ID, missing)
		}
	}
	if serverArchives == 0 {
		t.Fatal("the generated configuration has no server archive")
	}
	if !fullChecked {
		t.Fatal("the generated configuration has no full archive")
	}
}
