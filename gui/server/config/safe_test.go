package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseConfigurationSafeMode(t *testing.T) {
	cases := map[string]string{
		"":                          "auto",
		"[Safe]\nEnabled = 'auto'":  "auto",
		"[Safe]\nEnabled = 'TRUE'":  "true",
		"[Safe]\nEnabled = true":    "true",
		"[Safe]\nEnabled = false":   "false",
		"[Safe]\nEnabled = 'false'": "false",
	}
	for document, want := range cases {
		config, err := parseConfiguration([]byte(document))
		if err != nil {
			t.Fatalf("parseConfiguration(%q): %v", document, err)
		}
		if config.Safe.Enabled != want {
			t.Errorf("parseConfiguration(%q).Safe.Enabled = %v, want %q", document, config.Safe.Enabled, want)
		}
	}
}

func TestValidateConfigurationSafeMode(t *testing.T) {
	config, err := parseConfiguration([]byte("[Safe]\nEnabled = 'sometimes'\n"))
	if err != nil {
		t.Fatalf("parseConfiguration: %v", err)
	}
	found := false
	for _, issue := range validateConfiguration(config) {
		if issue.Field == "Safe.Enabled" && issue.Severity == severityError {
			found = true
		}
	}
	if !found {
		t.Fatal("an invalid Safe mode did not block saving")
	}

	config.Safe.Enabled = "auto"
	for _, issue := range validateConfiguration(config) {
		if issue.Field == "Safe.Enabled" {
			t.Fatalf("a valid Safe mode was rejected: %#v", issue)
		}
	}
}

func TestConfigurationDocumentWritesSafe(t *testing.T) {
	document := []byte("# head\n[Safe]\n# keep this comment\nEnabled = 'auto'\nPlatformUserIdExceptions = []\n")
	config, err := parseConfiguration(document)
	if err != nil {
		t.Fatalf("parseConfiguration: %v", err)
	}
	config.Safe.Enabled = "true"
	config.Safe.PlatformUserIdExceptions = []string{"42"}

	updated, err := configurationDocument(document, *config)
	if err != nil {
		t.Fatalf("configurationDocument: %v", err)
	}
	if !strings.Contains(string(updated), "# keep this comment") {
		t.Fatalf("the Safe comment was lost:\n%s", updated)
	}
	reparsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if reparsed.Safe.Enabled != "true" {
		t.Fatalf("Safe.Enabled = %v, want true", reparsed.Safe.Enabled)
	}
	if !slices.Equal(reparsed.Safe.PlatformUserIdExceptions, []string{"42"}) {
		t.Fatalf("Safe exceptions = %v, want [42]", reparsed.Safe.PlatformUserIdExceptions)
	}
}
