package main

import (
	"strings"
	"testing"
)

// The server's default is an enabled tree, so a document that predates the
// section has to keep behaving as it did.
func TestApiDefaultsToEnabled(t *testing.T) {
	if !serverDefaults().Api.Enabled {
		t.Fatal("serverDefaults must keep the API tree enabled")
	}
	parsed, err := parseConfiguration([]byte("[Mods]\nEnabled = true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Api.Enabled {
		t.Fatal("a document without [Api] must read as enabled")
	}
}

func TestConfigurationDocumentWritesApiSettings(t *testing.T) {
	original := []byte("[Api]\nEnabled = true\n")
	configuration := serverDefaults()
	configuration.Mods.Enabled = false
	configuration.Api.Enabled = false

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Count(string(updated), "[Api]") != 1 {
		t.Fatalf("the API table was duplicated or lost: %v", updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v (%v)", err, updated)
	}
	if parsed.Api.Enabled {
		t.Fatal("the API switch did not round-trip")
	}
}

func TestValidateConfigurationApiConflictsWithMods(t *testing.T) {
	config := serverDefaults()
	config.Games.Enabled = []string{"age2"}
	config.Games.Age2.Hosts = []string{"127.0.0.1"}
	config.Api.Enabled = false

	issues := errorValidationIssues(validateConfiguration(&config))
	if !hasField(issues, "Api.Enabled") {
		t.Fatalf("the API/mods conflict was not reported: %#v", issues)
	}

	config.Mods.Enabled = false
	if issues := errorValidationIssues(validateConfiguration(&config)); hasField(issues, "Api.Enabled") {
		t.Fatalf("a tree that is off with no mods API must not be a conflict: %#v", issues)
	}
}

func hasField(issues []ValidationError, field string) bool {
	for _, issue := range issues {
		if issue.Field == field {
			return true
		}
	}
	return false
}
