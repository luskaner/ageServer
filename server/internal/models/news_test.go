package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/paths"
	i "github.com/luskaner/ageServer/server/internal"
)

// configForGame points the config loader at a temporary resources/config tree
// holding the given files for one game.
func configForGame(t *testing.T, gameId string, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, gameId)
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	old := paths.ConfigsPath
	paths.ConfigsPath = dir
	t.Cleanup(func() { paths.ConfigsPath = old })
}

const newsTOML = `
[2]
category = "second"
title = "Second title"
subtitle = "Second subtitle"
text = "Second text"
image_url = "https://example.com/second.jpg"
body = "Second body"
metadata = "{ \"version\": 2 }"
date_visible = 1793294401
date_expiry = 1798480801

[1]
category = "first"
title = "First title"
subtitle = "First subtitle"
text = "First text"
image_url = "https://example.com/first.jpg"
body = "First body"
metadata = "{ \"version\": 1 }"
date_visible = "1793294400"
date_expiry = "1798480800"
`

// The table key is the news id and the entries go out in id order, with every
// field in the position the client expects.
func TestInitializeNews_LoadsEntriesInOrder(t *testing.T) {
	configForGame(t, "age4", map[string]string{"news.toml": newsTOML})
	resources := &MainResources{}

	resources.initializeNews("age4")

	if len(resources.news) != 2 {
		t.Fatalf("loaded %d news, want 2", len(resources.news))
	}
	wantFirst := i.A{
		1, "first", "First title", "First subtitle", "First text",
		"https://example.com/first.jpg", "First body", `{ "version": 1 }`,
		"1793294400", "1798480800",
	}
	for index, want := range wantFirst {
		got := resources.news[0][index]
		// subtitle is stored as *string (a nil pointer means "no subtitle").
		if subtitle, ok := got.(*string); ok {
			got = *subtitle
		}
		if got != want {
			t.Fatalf("news 1 field %d = %v, want %v", index, got, want)
		}
	}
	if got := resources.news[1][0]; got != 2 {
		t.Fatalf("second news id = %v, want 2", got)
	}
	if got := resources.news[1][8]; got != "1793294401" {
		t.Errorf("second news visible date = %v, want string timestamp %q", got, "1793294401")
	}
	if got := resources.news[1][9]; got != "1798480801" {
		t.Errorf("second news expiry date = %v, want string timestamp %q", got, "1798480801")
	}
}

// A news.toml without dates is valid: the client receives an empty date, not
// a server panic.
func TestInitializeNews_AllowsMissingDates(t *testing.T) {
	configForGame(t, "age4", map[string]string{
		"news.toml": `
[1]
title = "Only a title"
`,
	})
	resources := &MainResources{}

	resources.initializeNews("age4")

	if len(resources.news) != 1 {
		t.Fatalf("loaded %d news, want 1", len(resources.news))
	}
	if got := resources.news[0][8]; got != "" {
		t.Errorf("date_visible = %v, want empty", got)
	}
	if got := resources.news[0][9]; got != "" {
		t.Errorf("date_expiry = %v, want empty", got)
	}
}

// TOML does not reliably distinguish 1 from 1.0, so an integral float is
// accepted.
func TestInitializeNews_AllowsIntegralFloatDates(t *testing.T) {
	configForGame(t, "age4", map[string]string{
		"news.toml": `
[1]
title = "Float date"
date_visible = 1793294400.0
`,
	})
	resources := &MainResources{}

	resources.initializeNews("age4")

	if got := resources.news[0][8]; got != "1793294400" {
		t.Fatalf("date_visible = %v, want %q", got, "1793294400")
	}
}

func TestInitializeNews_RejectsFloatTimestamp(t *testing.T) {
	configForGame(t, "age4", map[string]string{
		"news.toml": `
[1]
title = "Bad date"
date_visible = 1.5
`,
	})
	resources := &MainResources{}
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected a panic for a fractional timestamp")
		}
		if !strings.Contains(fmt.Sprint(recovered), "date_visible") {
			t.Fatalf("panic does not name the field: %v", recovered)
		}
	}()

	resources.initializeNews("age4")
}

func TestInitializeNews_RejectsInvalidType(t *testing.T) {
	for name, value := range map[string]string{
		"boolean": "true",
		"array":   "[1, 2]",
	} {
		t.Run(name, func(t *testing.T) {
			configForGame(t, "age4", map[string]string{
				"news.toml": "[1]\ntitle = \"Bad type\"\ndate_expiry = " + value + "\n",
			})
			resources := &MainResources{}
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("expected a panic for an invalid timestamp type")
				}
				if !strings.Contains(fmt.Sprint(recovered), "date_expiry") {
					t.Fatalf("panic does not name the field: %v", recovered)
				}
			}()

			resources.initializeNews("age4")
		})
	}
}

// A game without news.toml simply has no news: the route still returns the
// expected three-element envelope.
func TestInitializeNews_MissingFileLeavesNoNews(t *testing.T) {
	configForGame(t, "age4", nil)
	resources := &MainResources{}

	resources.initializeNews("age4")

	if resources.news != nil {
		t.Fatalf("news = %v, want none", resources.news)
	}
}

// A malformed news.toml is a configuration error and must fail at startup
// instead of silently serving an empty list.
func TestInitializeNews_MalformedFilePanics(t *testing.T) {
	configForGame(t, "age4", map[string]string{"news.toml": "%%% not toml %%%"})
	resources := &MainResources{}
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for malformed news.toml")
		}
	}()

	resources.initializeNews("age4")
}
