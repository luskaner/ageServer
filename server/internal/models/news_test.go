package models

import (
	"os"
	"path/filepath"
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
date_visible = "1793294401"
date_expiry = "1798480801"

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
		if got := resources.news[0][index]; got != want {
			t.Fatalf("news 1 field %d = %v, want %v", index, got, want)
		}
	}
	if got := resources.news[1][0]; got != 2 {
		t.Fatalf("second news id = %v, want 2", got)
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
