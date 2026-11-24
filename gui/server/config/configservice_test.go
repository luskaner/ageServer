package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseConfiguration(t *testing.T) {
	config, err := parseConfiguration([]byte(`
Log = true
GeneratePlatformUserId = false
Authentication = 'cached'

[Sign]
Enabled = true
PlatformUserIdExceptions = []

[Internet]
Enabled = false
IP = '203.0.113.10'

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['192.168.1.10']

[Announcement]
Enabled = true
Multicast = true
Port = 32000
MulticastGroup = '239.31.97.8'

[CustomFiles]
Enabled = false
Folder = 'D:\saves'
MaxFileSize = '64MB'
MaxTotalSize = '1GB'
Expiry = '30m'

[Mods]
Enabled = false
DBPath = 'D:\mods.db'
UploadDir = 'D:\mods'
MaxUploadSize = '512MB'
RequireAuthForRead = true
`))
	if err != nil {
		t.Fatalf("parseConfiguration returned an error: %v", err)
	}
	if !config.Log || config.Authentication != "cached" || config.Announcement.Port != 32000 {
		t.Fatalf("unexpected parsed configuration: %+v", config)
	}
	if !config.Sign.Enabled {
		t.Fatal("Sign.Enabled was not parsed")
	}
	if config.Internet.Enabled || config.Internet.IP != "203.0.113.10" {
		t.Fatalf("unexpected internet configuration: %#v", config.Internet)
	}
	if config.CustomFiles.Enabled || config.CustomFiles.Folder != `D:\saves` ||
		config.CustomFiles.MaxFileSize != "64MB" || config.CustomFiles.MaxTotalSize != "1GB" ||
		config.CustomFiles.Expiry != "30m" {
		t.Fatalf("unexpected custom files configuration: %#v", config.CustomFiles)
	}
	if config.Mods.Enabled || config.Mods.DBPath != `D:\mods.db` ||
		config.Mods.UploadDir != `D:\mods` || config.Mods.MaxUploadSize != "512MB" ||
		!config.Mods.RequireAuthForRead {
		t.Fatalf("unexpected mods configuration: %#v", config.Mods)
	}
	if len(config.Games.Enabled) != 1 || config.Games.Enabled[0] != "age2" {
		t.Fatalf("unexpected enabled games: %#v", config.Games.Enabled)
	}
	if len(config.Games.Age2.Hosts) != 1 || config.Games.Age2.Hosts[0] != "192.168.1.10" {
		t.Fatalf("unexpected hosts: %#v", config.Games.Age2.Hosts)
	}
}

func TestParseConfigurationAppliesServerDefaults(t *testing.T) {
	config, err := parseConfiguration([]byte("Log = true\n"))
	if err != nil {
		t.Fatalf("parseConfiguration returned an error: %v", err)
	}
	if config.Authentication != "disabled" {
		t.Fatalf("Authentication = %q, want the server default", config.Authentication)
	}
	if !config.Internet.Enabled || config.Internet.IP != "auto" {
		t.Fatalf("unexpected internet defaults: %#v", config.Internet)
	}
	if !config.Announcement.Enabled || !config.Announcement.Multicast ||
		config.Announcement.Port != 31978 || config.Announcement.MulticastGroup != "239.31.97.8" {
		t.Fatalf("unexpected announcement defaults: %#v", config.Announcement)
	}
	if !config.CustomFiles.Enabled || config.CustomFiles.Folder != "resources/custom-files" ||
		config.CustomFiles.MaxFileSize != "32MB" || config.CustomFiles.MaxTotalSize != "256MB" ||
		config.CustomFiles.Expiry != "12h" {
		t.Fatalf("unexpected custom files defaults: %#v", config.CustomFiles)
	}
	if !config.Mods.Enabled || config.Mods.DBPath != "resources/mods/mods.db" ||
		config.Mods.UploadDir != "resources/mods/uploads" ||
		config.Mods.MaxUploadSize != "256MB" || config.Mods.RequireAuthForRead {
		t.Fatalf("unexpected mods defaults: %#v", config.Mods)
	}
	if len(config.Hosts.Modern) != 1 || config.Hosts.Modern[0] != "0.0.0.0" || len(config.Hosts.Legacy) != 0 {
		t.Fatalf("unexpected listener defaults: %#v", config.Hosts)
	}
}

func TestParseConfigurationKeepsExplicitFalseValues(t *testing.T) {
	config, err := parseConfiguration([]byte("[Internet]\nEnabled = false\n\n[CustomFiles]\nEnabled = false\n"))
	if err != nil {
		t.Fatalf("parseConfiguration returned an error: %v", err)
	}
	if config.Internet.Enabled || config.CustomFiles.Enabled {
		t.Fatalf("explicit disabled switches were replaced by defaults: %#v", config)
	}
}

func TestParseConfigurationRejectsInvalidTOML(t *testing.T) {
	if _, err := parseConfiguration([]byte("[Announcement\nPort = 32000")); err == nil {
		t.Fatal("parseConfiguration accepted invalid TOML")
	}
}

func TestParseConfigurationRejectsInvalidType(t *testing.T) {
	if _, err := parseConfiguration([]byte("[Announcement]\nPort = 'not-a-port'")); err == nil {
		t.Fatal("parseConfiguration accepted an invalid port type")
	}
}

func TestContainsConfigurationField(t *testing.T) {
	if hasFields, err := containsConfigurationField([]byte("[other]\nValue = true")); err != nil || hasFields {
		t.Fatalf("unrelated TOML was identified as configuration: fields=%v, err=%v", hasFields, err)
	}
	if hasFields, err := containsConfigurationField([]byte("[Announcement]\nPort = 0")); err != nil || !hasFields {
		t.Fatalf("invalid known field was not identified: fields=%v, err=%v", hasFields, err)
	}
}

func TestReadConfigFileAcceptsInvalidKnownValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := []byte("Authentication = 'unsupported'\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("create test configuration: %v", err)
	}
	file, err := (&ConfigService{}).readConfigFile(path)
	if err != nil {
		t.Fatalf("readConfigFile rejected a known but invalid value: %v", err)
	}
	if len(file.ValidationErrors) == 0 || file.ValidationErrors[0].Field != "Authentication" {
		t.Fatalf("unexpected validation errors: %#v", file.ValidationErrors)
	}
}

func TestReadConfigFileRejectsUnrelatedTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[other]\nValue = true\n"), 0600); err != nil {
		t.Fatalf("create test configuration: %v", err)
	}
	if _, err := (&ConfigService{}).readConfigFile(path); err == nil {
		t.Fatal("readConfigFile accepted TOML without supported configuration fields")
	}
}

func TestOpenConfigFileWithoutDefaultKeepsTheLoadScreen(t *testing.T) {
	// No default configuration next to the test binary or in the working
	// directory: OpenConfigFile must not fall back to the native picker.
	t.Chdir(t.TempDir())
	file, err := (&ConfigService{}).OpenConfigFile()
	if err != nil {
		t.Fatalf("OpenConfigFile returned an error: %v", err)
	}
	if file != nil {
		t.Fatalf("OpenConfigFile returned a file without a default configuration")
	}
}

func TestValidateConfiguration(t *testing.T) {
	cases := []struct {
		name       string
		config     Configuration
		field      string
		hasErrors  bool
		hasWarning bool
	}{
		{
			name: "valid",
			config: Configuration{
				Authentication: "disabled",
				Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
				Games:          GamesConfiguration{Enabled: []string{"age2"}},
				Announcement:   AnnouncementConfig{Port: 31978},
			},
		},
		{
			name: "unknown authentication",
			config: Configuration{
				Authentication: "unknown",
				Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
				Games:          GamesConfiguration{Enabled: []string{"age2"}},
				Announcement:   AnnouncementConfig{Port: 31978},
			},
			field:     "Authentication",
			hasErrors: true,
		},
		{
			name: "empty games list only warns",
			config: Configuration{
				Authentication: "disabled",
				Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
				Announcement:   AnnouncementConfig{Port: 31978},
			},
			field:      "Games.Enabled",
			hasWarning: true,
		},
		{
			name: "invalid port",
			config: Configuration{
				Authentication: "disabled",
				Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
				Announcement:   AnnouncementConfig{Port: 65536},
			},
			field:     "Announcement.Port",
			hasErrors: true,
		},
		{
			name: "unknown game",
			config: Configuration{
				Authentication: "disabled",
				Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
				Games:          GamesConfiguration{Enabled: []string{"age5"}},
				Announcement:   AnnouncementConfig{Port: 31978},
			},
			field:     "Games.Enabled.0",
			hasErrors: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			issues := validateConfiguration(&testCase.config)
			errors := errorValidationIssues(issues)
			if !testCase.hasErrors && len(errors) != 0 {
				t.Fatalf("validateConfiguration returned unexpected errors: %#v", errors)
			}
			if testCase.hasErrors && len(errors) == 0 {
				t.Fatal("validateConfiguration accepted invalid configuration")
			}
			if testCase.field != "" && len(errors) > 0 && errors[0].Field != testCase.field {
				t.Fatalf("first error field = %q, want %q", errors[0].Field, testCase.field)
			}
			if testCase.hasErrors && len(errors) > 0 && errors[0].Severity != severityError {
				t.Fatalf("first error severity = %q, want %q", errors[0].Severity, severityError)
			}
			if testCase.hasWarning {
				if len(issues) == 0 {
					t.Fatal("validateConfiguration did not warn")
				}
				warning := issues[0]
				if warning.Field != testCase.field {
					t.Fatalf("warning field = %q, want %q", warning.Field, testCase.field)
				}
				if warning.Severity != severityWarning {
					t.Fatalf("warning severity = %q, want %q", warning.Severity, severityWarning)
				}
				if len(errors) != 0 {
					t.Fatalf("the warning blocked saving: %#v", errors)
				}
			}
		})
	}
}

func TestValidateConfigurationDuplicateHosts(t *testing.T) {
	// A modern and a legacy listener on the same address would bind it twice.
	config := Configuration{
		Authentication: "disabled",
		Hosts: HostsConfiguration{
			Modern: []string{"192.0.2.10"},
			Legacy: []string{"192.0.2.10"},
		},
		Announcement: AnnouncementConfig{Port: 31978},
	}
	issues := validateConfiguration(&config)
	errors := errorValidationIssues(issues)
	if len(errors) != 2 {
		t.Fatalf("the shared address was not reported on both lists: %#v", issues)
	}
	if errors[0].Field != "Hosts.Modern.0" || errors[1].Field != "Hosts.Legacy.0" {
		t.Fatalf("unexpected error fields: %#v", errors)
	}
	for _, issue := range issues {
		if issue.Severity == severityWarning && issue.Field != "Games.Enabled" {
			t.Fatalf("the conflict warned instead of blocking: %#v", issues)
		}
	}

	// No listener at all is an error: the server would have nothing to bind.
	config.Hosts = HostsConfiguration{}
	issues = validateConfiguration(&config)
	if errors := errorValidationIssues(issues); len(errors) != 1 || errors[0].Field != "Hosts" {
		t.Fatalf("both empty lists were not reported: %#v", issues)
	}
}

func TestValidateConfigurationHostsWildcard(t *testing.T) {
	config := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"0.0.0.0", "192.0.2.10"}},
		Announcement:   AnnouncementConfig{Port: 31978},
	}
	issues := validateConfiguration(&config)
	errors := errorValidationIssues(issues)
	if len(errors) != 1 || errors[0].Field != "Hosts.Modern.0" {
		t.Fatalf("the wildcard in one list was not reported: %#v", issues)
	}

	// 0.0.0.0 covers every interface, so another mode cannot bind an address.
	config.Hosts = HostsConfiguration{Modern: []string{"0.0.0.0"}, Legacy: []string{"192.0.2.11"}}
	issues = validateConfiguration(&config)
	errors = errorValidationIssues(issues)
	if len(errors) != 1 || errors[0].Field != "Hosts.Modern.0" {
		t.Fatalf("the wildcard next to another mode was not reported: %#v", issues)
	}

	// A class with no listener only warns: the other certificate may still work.
	config.Hosts = HostsConfiguration{Modern: []string{"127.0.0.1"}}
	config.Games.Enabled = []string{"age2", "age4"}
	found := false
	for _, issue := range validateConfiguration(&config) {
		if issue.Field == "Hosts.Legacy" && issue.Severity == severityWarning {
			found = true
		}
	}
	if !found {
		t.Fatal("a legacy game without a legacy listener did not warn")
	}
}

func TestValidateConfigurationPlaceholderListeners(t *testing.T) {
	// 0.0.0.0 in both lists is the server's placeholder: one listener follows
	// the certificate class of the enabled games, so it must not block saving.
	config := serverDefaults()
	config.Hosts = HostsConfiguration{Modern: []string{"0.0.0.0"}, Legacy: []string{"0.0.0.0"}}
	config.Games.Enabled = []string{"age2"}
	if errors := errorValidationIssues(validateConfiguration(&config)); len(errors) != 0 {
		t.Fatalf("the placeholder blocked saving: %#v", errors)
	}

	// A single class is what the placeholder resolves to, so there is nothing
	// to warn about either.
	for _, issue := range validateConfiguration(&config) {
		if issue.Field == "Hosts.Legacy" || issue.Field == "Hosts.Modern" {
			t.Fatalf("one class with the placeholder warned: %#v", issue)
		}
	}

	// A single 0.0.0.0 list is the same placeholder, so the class it does not
	// name is not reported as a missing listener.
	config.Hosts = HostsConfiguration{Modern: []string{"0.0.0.0"}}
	for _, issue := range validateConfiguration(&config) {
		if issue.Field == "Hosts.Legacy" || issue.Field == "Hosts.Modern" {
			t.Fatalf("the single-list placeholder warned: %#v", issue)
		}
	}

	// Both classes need two addresses, and the server refuses to start without
	// them, so the editor says so before the document is saved.
	config.Hosts = HostsConfiguration{Modern: []string{"0.0.0.0"}, Legacy: []string{"0.0.0.0"}}
	config.Games.Enabled = []string{"age2", "age4"}
	found := false
	for _, issue := range validateConfiguration(&config) {
		if issue.Field == "Hosts.Legacy" && issue.Severity == severityWarning {
			found = true
		}
	}
	if !found {
		t.Fatal("the placeholder with both classes did not warn")
	}
}

func TestValidateConfigurationPrivilegedPortWarnsWithoutBlocking(t *testing.T) {
	config := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}, Legacy: []string{"127.0.0.2"}},
		Games:          GamesConfiguration{Enabled: []string{"age2"}},
		Announcement:   AnnouncementConfig{Enabled: true, Port: 80},
	}
	issues := validateConfiguration(&config)
	if len(issues) != 1 || issues[0].Field != "Announcement.Port" || issues[0].Severity != severityWarning {
		t.Fatalf("unexpected privileged port issues: %#v", issues)
	}
	if blocking := errorValidationIssues(issues); len(blocking) != 0 {
		t.Fatalf("privileged port blocked saving: %#v", blocking)
	}

	// The port is out of play while the announcement is disabled.
	config.Announcement.Enabled = false
	if issues := validateConfiguration(&config); len(issues) != 0 {
		t.Fatalf("disabled announcement returned issues: %#v", issues)
	}

	// A port outside the valid range is an error, not a warning.
	config.Announcement.Enabled = true
	config.Announcement.Port = 0
	issues = validateConfiguration(&config)
	if len(issues) != 1 || issues[0].Severity != severityError {
		t.Fatalf("unexpected invalid port issues: %#v", issues)
	}
}

func TestValidateConfigurationMulticastIsConditional(t *testing.T) {
	config := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}, Legacy: []string{"127.0.0.2"}},
		Games:          GamesConfiguration{Enabled: []string{"age2"}},
		Announcement: AnnouncementConfig{
			Enabled:        true,
			Multicast:      true,
			Port:           31978,
			MulticastGroup: "192.168.1.1",
		},
	}
	errors := validateConfiguration(&config)
	if len(errors) != 1 || errors[0].Field != "Announcement.MulticastGroup" {
		t.Fatalf("unexpected multicast errors: %#v", errors)
	}

	config.Announcement.Multicast = false
	if errors := validateConfiguration(&config); len(errors) != 0 {
		t.Fatalf("disabled multicast returned errors: %#v", errors)
	}
}

func TestValidateConfigurationInternet(t *testing.T) {
	config := serverDefaults()
	config.Games.Enabled = []string{"age2"}
	config.Games.Age2.Hosts = []string{"127.0.0.1"}
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("defaults were not valid: %#v", issues)
	}

	config.Internet.IP = "not-an-address"
	issues := errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 1 || issues[0].Field != "Internet.IP" || issues[0].Severity != severityError {
		t.Fatalf("unexpected internet address issues: %#v", issues)
	}

	config.Internet.IP = "203.0.113.10"
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("a manual IPv4 address was rejected: %#v", issues)
	}

	// The address is out of play while the server does not use the Internet.
	config.Internet.Enabled = false
	config.Internet.IP = "not-an-address"
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("disabled internet still validated its address: %#v", issues)
	}

	// Authentication cannot be required without Internet access: the server
	// refuses to start in that combination.
	config.Authentication = "required"
	issues = errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 1 || issues[0].Field != "Authentication" {
		t.Fatalf("unexpected required authentication issues: %#v", issues)
	}
}

func TestValidateConfigurationCustomFiles(t *testing.T) {
	config := serverDefaults()
	config.Games.Enabled = []string{"age2"}
	config.Games.Age2.Hosts = []string{"127.0.0.1"}

	config.CustomFiles.MaxFileSize = "32"
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("a plain byte count was rejected: %#v", issues)
	}

	config.CustomFiles.MaxFileSize = "32 MB"
	config.CustomFiles.MaxTotalSize = "1GB"
	config.CustomFiles.Expiry = "1h30m"
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("valid sizes and durations were rejected: %#v", issues)
	}

	config.CustomFiles.MaxFileSize = "big"
	issues := errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 1 || issues[0].Field != "CustomFiles.MaxFileSize" {
		t.Fatalf("unexpected file size issues: %#v", issues)
	}

	config.CustomFiles.MaxFileSize = "2GB"
	config.CustomFiles.MaxTotalSize = "1GB"
	issues = errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 1 || issues[0].Field != "CustomFiles.MaxFileSize" {
		t.Fatalf("unexpected size relation issues: %#v", issues)
	}

	config.CustomFiles.MaxFileSize = "32MB"
	config.CustomFiles.MaxTotalSize = "256MB"
	config.CustomFiles.Folder = "  "
	config.CustomFiles.Expiry = "12"
	issues = errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 2 ||
		issues[0].Field != "CustomFiles.Folder" || issues[1].Field != "CustomFiles.Expiry" {
		t.Fatalf("unexpected folder or expiry issues: %#v", issues)
	}

	// Disabled endpoints never use these values, so they do not block saving.
	config.CustomFiles.Enabled = false
	config.CustomFiles.MaxFileSize = "big"
	config.CustomFiles.MaxTotalSize = "bigger"
	config.CustomFiles.Expiry = "12"
	config.CustomFiles.Folder = ""
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("disabled custom files returned issues: %#v", issues)
	}
}

func TestConfigurationDocumentWritesModsSettings(t *testing.T) {
	original := []byte(`[Mods]
Enabled = true
`)
	configuration := serverDefaults()
	configuration.Mods.Enabled = false
	configuration.Mods.DBPath = "custom/mods.db"
	configuration.Mods.UploadDir = "custom/uploads"
	configuration.Mods.MaxUploadSize = "512MB"
	configuration.Mods.RequireAuthForRead = true

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Count(string(updated), "[Mods]") != 1 {
		t.Fatalf("the mods table was duplicated or lost: %v", updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v (%v)", err, updated)
	}
	if parsed.Mods.Enabled || parsed.Mods.DBPath != "custom/mods.db" ||
		parsed.Mods.UploadDir != "custom/uploads" || parsed.Mods.MaxUploadSize != "512MB" ||
		!parsed.Mods.RequireAuthForRead {
		t.Fatalf("the mods settings did not round-trip: %#v", parsed.Mods)
	}
}

func TestValidateConfigurationMods(t *testing.T) {
	config := serverDefaults()
	config.Games.Enabled = []string{"age2"}
	config.Games.Age2.Hosts = []string{"127.0.0.1"}

	config.Mods.DBPath = ""
	config.Mods.UploadDir = ""
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("empty mods paths were rejected: %#v", issues)
	}

	config.Mods.MaxUploadSize = "256MB"
	config.Mods.RequireAuthForRead = true
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("valid mods settings were rejected: %#v", issues)
	}

	config.Mods.MaxUploadSize = "big"
	issues := errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 1 || issues[0].Field != "Mods.MaxUploadSize" {
		t.Fatalf("unexpected mods size issues: %#v", issues)
	}

	config.Mods.MaxUploadSize = "0MB"
	issues = errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 1 || issues[0].Field != "Mods.MaxUploadSize" {
		t.Fatalf("unexpected zero mods size issues: %#v", issues)
	}

	// Disabled, the server ignores the section and does not parse the size.
	config.Mods.Enabled = false
	config.Mods.MaxUploadSize = "big"
	if issues := errorValidationIssues(validateConfiguration(&config)); len(issues) != 0 {
		t.Fatalf("disabled mods returned issues: %#v", issues)
	}
}

func TestConfigurationDocumentAddsMissingSettings(t *testing.T) {
	original := []byte("Log = false\n\n[Announcement]\nEnabled = true\nMulticast = false\nPort = 31978\nMulticastGroup = '239.31.97.8'\n")
	configuration := serverDefaults()
	configuration.CustomFiles.MaxFileSize = "64MB"

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Count(string(updated), "[CustomFiles]") != 1 || strings.Count(string(updated), "[Internet]") != 1 {
		t.Fatalf("tables were duplicated or not added:\n%s", updated)
	}
	if strings.Index(string(updated), "Authentication = 'disabled'") > strings.Index(string(updated), "[Announcement]") {
		t.Fatalf("a root key was written inside a table:\n%s", updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if parsed.Sign.Enabled || !parsed.Internet.Enabled || parsed.Internet.IP != "auto" {
		t.Fatalf("unexpected internet configuration: %#v", parsed.Internet)
	}
	if parsed.CustomFiles.MaxFileSize != "64MB" || parsed.CustomFiles.Expiry != "12h" {
		t.Fatalf("unexpected custom files configuration: %#v", parsed.CustomFiles)
	}
	if len(parsed.Hosts.Modern) != 1 || parsed.Hosts.Modern[0] != "0.0.0.0" {
		t.Fatalf("unexpected default listeners: %#v", parsed.Hosts)
	}
}

func TestReplaceAnnouncementPortPreservesDocument(t *testing.T) {
	original := []byte("# Keep this comment.\nLog = false\n\n[Announcement] # Keep this section comment.\nPort = 31978 # Keep this inline comment.\nUnknown = 'keep me'\n")
	updated, err := replaceDocumentValue(original, "Announcement", "Port", "32000")
	if err != nil {
		t.Fatalf("replaceDocumentValue returned an error: %v", err)
	}
	if !strings.Contains(string(updated), "# Keep this comment.") ||
		!strings.Contains(string(updated), "# Keep this inline comment.") ||
		!strings.Contains(string(updated), "Unknown = 'keep me'") {
		t.Fatalf("replacement did not preserve the original document: %s", updated)
	}
	if !strings.Contains(string(updated), "Port = 32000 # Keep this inline comment.") {
		t.Fatalf("replacement did not update the port: %s", updated)
	}
	config, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v", err)
	}
	if config.Announcement.Port != 32000 {
		t.Fatalf("updated port = %d, want 32000", config.Announcement.Port)
	}
}

func TestConfigurationDocumentRoundTripsTheOfficialTemplate(t *testing.T) {
	templatePath, err := templateConfigPath()
	if err != nil {
		t.Fatalf("templateConfigPath returned an error: %v", err)
	}
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	original, err := parseConfiguration(templateContent)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	updated, err := configurationDocument(templateContent, *original)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	roundTripped, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("round-tripped template is invalid: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(*roundTripped, *original) {
		t.Fatalf("round trip changed the configuration:\noriginal: %#v\nupdated:  %#v", *original, *roundTripped)
	}
}

func TestGameDocumentWritesBattleServersAsTables(t *testing.T) {
	servers := []BattleServerConfig{
		{Region: "local", Name: "Local", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
		{Region: "other", IPv4: "192.168.1.2", BsPort: 30013, WebSocketPort: 30014, OutOfBandPort: 30015},
	}
	updated, err := gameDocument("age2", nil, GameConfig{BattleServers: servers})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "BattleServers =") {
		t.Fatalf("battle servers were written as an inline array:\n%s", updated)
	}
	if count := strings.Count(string(updated), "[[Game.BattleServers]]"); count != 2 {
		t.Fatalf("battle server tables = %d, want 2:\n%s", count, updated)
	}
	parsed, err := parseGameConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.BattleServers, servers) {
		t.Fatalf("battle servers did not round-trip: %#v", parsed.BattleServers)
	}

	// Saving again must not append a second copy of the same tables.
	twice, err := gameDocument("age2", updated, GameConfig{BattleServers: servers})
	if err != nil {
		t.Fatalf("second gameDocument returned an error: %v", err)
	}
	if string(twice) != string(updated) {
		t.Fatalf("a second save changed the document:\nfirst:\n%s\nsecond:\n%s", updated, twice)
	}
}

func TestGameDocumentReplacesInlineBattleServers(t *testing.T) {
	original := []byte(`[Game]
Hosts = ['0.0.0.0']
BattleServers = [{Region = 'old', IPv4 = 'auto', BsPort = 1, WebSocketPort = 2}]
`)
	servers := []BattleServerConfig{
		{Region: "local", Name: "Local", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
	}
	updated, err := gameDocument("age2", original, GameConfig{BattleServers: servers})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "BattleServers = [") {
		t.Fatalf("the inline BattleServers key was kept:\n%s", updated)
	}
	if strings.Contains(string(updated), "Hosts = ") {
		t.Fatalf("the deprecated Game.Hosts key was kept:\n%s", updated)
	}
	if !strings.Contains(string(updated), "[[Game.BattleServers]]") {
		t.Fatalf("battle servers were not written as tables:\n%s", updated)
	}
	parsed, err := parseGameConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.BattleServers, servers) {
		t.Fatalf("battle servers did not round-trip: %#v", parsed.BattleServers)
	}
}

func TestGameDocumentRemovesEmptyBattleServers(t *testing.T) {
	original := []byte(`[Game]
Hosts = ['0.0.0.0']

[[Game.BattleServers]]
Region = 'local'
IPv4 = 'auto'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005
`)
	updated, err := gameDocument("age2", original, GameConfig{})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "BattleServers") || strings.Contains(string(updated), "Hosts = ") {
		t.Fatalf("the removed keys were kept:\n%s", updated)
	}
	if _, err := parseGameConfiguration(updated); err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
}

func TestGameDocumentHandlesInterleavedBattleServerTables(t *testing.T) {
	original := []byte(`[Game]
Hosts = ['0.0.0.0']

[[Game.BattleServers]]
Region = 'old'
IPv4 = 'auto'
BsPort = 1
WebSocketPort = 2

[Game.EmbeddedBattleServer]
Region = 'embedded'

[[Game.BattleServers]]
Region = 'old-2'
IPv4 = 'auto'
BsPort = 5
WebSocketPort = 6
`)
	servers := []BattleServerConfig{
		{Region: "new", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
	}
	updated, err := gameDocument("age2", original, GameConfig{
		BattleServers:        servers,
		EmbeddedBattleServer: EmbeddedBattleServerConfig{Region: "embedded"},
	})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "old") {
		t.Fatalf("a stale battle server table was kept:\n%s", updated)
	}
	if count := strings.Count(string(updated), "[[Game.BattleServers]]"); count != 1 {
		t.Fatalf("battle server tables = %d, want 1:\n%s", count, updated)
	}
	parsed, err := parseGameConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.BattleServers, servers) {
		t.Fatalf("unexpected battle servers: %#v", parsed.BattleServers)
	}
	if parsed.EmbeddedBattleServer.Region != "embedded" {
		t.Fatalf("the embedded table was lost: %#v", parsed.EmbeddedBattleServer)
	}
}

func TestValidateConfigurationBattleServers(t *testing.T) {
	config := serverDefaults()
	config.Games.Enabled = []string{"age4"}
	config.Games.Age4.Hosts = []string{"127.0.0.1"}
	// The embedded battle server counts: with the default StartIfMissing the
	// server starts it and AoE IV is fine without a [[Games.age4.BattleServers]].
	issues := errorValidationIssues(validateConfiguration(&config))
	if len(issues) != 0 {
		t.Fatalf("a game covered by the embedded battle server was reported: %#v", issues)
	}

	config.Games.Age4.EmbeddedBattleServer.StartIfMissing = "false"
	// The battle-server-manager may still find one, so this only advises.
	advised := false
	for _, issue := range validateConfiguration(&config) {
		if issue.Field != "Games.age4.BattleServers" {
			continue
		}
		if issue.Severity != severityWarning {
			t.Fatalf("the advisory was not a warning: %#v", issue)
		}
		advised = true
	}
	if !advised {
		t.Fatal("an enabled game with neither battle servers nor a starting embedded one was not advised")
	}

	config.Games.Age4.EmbeddedBattleServer.StartIfMissing = "true"
	config.Games.Age4.BattleServers = []BattleServerConfig{
		{Region: "local", IPv4: "auto", BsPort: 30009, WebSocketPort: 30010, OutOfBandPort: 30011},
		{Region: "local", IPv4: "not-an-address", BsPort: 0, WebSocketPort: 70000, OutOfBandPort: -1},
	}
	issues = errorValidationIssues(validateConfiguration(&config))
	fields := make(map[string]bool, len(issues))
	for _, issue := range issues {
		fields[issue.Field] = true
	}
	for _, field := range []string{
		"Games.age4.BattleServers.1.Region",
		"Games.age4.BattleServers.1.IPv4",
		"Games.age4.BattleServers.1.BsPort",
		"Games.age4.BattleServers.1.WebSocketPort",
		"Games.age4.BattleServers.1.OutOfBandPort",
	} {
		if !fields[field] {
			t.Fatalf("%s was not reported: %#v", field, issues)
		}
	}
}

func TestWriteConfigFileReplacesExistingFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.toml")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatalf("create test configuration: %v", err)
	}
	if err := writeConfigFile(path, []byte("new")); err != nil {
		t.Fatalf("writeConfigFile returned an error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced configuration: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("replaced content = %q, want %q", content, "new")
	}
}

func TestSaveConfigurationDoesNotOverwriteInvalidConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.toml")
	original := []byte(`Log = false
GeneratePlatformUserId = false
Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['127.0.0.1']

[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatalf("create test configuration: %v", err)
	}

	configuration := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
		Games:          GamesConfiguration{Enabled: []string{"age2"}},
		Announcement:   AnnouncementConfig{Port: 0},
	}
	if err := (&ConfigService{}).SaveConfiguration(path, configuration); err == nil {
		t.Fatal("SaveConfiguration accepted an invalid configuration")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read unchanged configuration: %v", err)
	}
	if string(content) != string(original) {
		t.Fatalf("invalid save changed the configuration: %s", content)
	}
}

func TestSaveConfigurationReportsMissingPath(t *testing.T) {
	configuration := Configuration{Authentication: "disabled"}
	if err := (&ConfigService{}).SaveConfiguration(filepath.Join(t.TempDir(), "missing.toml"), configuration); err == nil {
		t.Fatal("SaveConfiguration accepted a missing path")
	}
}

func TestSaveConfigurationAsPreservesSourceAndRequiresOverwriteConfirmation(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.toml")
	destinationPath := filepath.Join(directory, "destination.toml")
	original := []byte(`# Preserve this comment.
	Log = false
	GeneratePlatformUserId = false
	Authentication = 'disabled'

	[Games]
	Enabled = ['age2']

	[Games.age2]
	Hosts = ['127.0.0.1']

	[Games.age1]
	Hosts = ['127.0.0.2']

	[Games.age3]
	Hosts = ['127.0.0.3']

	[Games.age4]
	Hosts = ['127.0.0.4']

	[Games.athens]
	Hosts = ['127.0.0.5']

	[Announcement]
	Enabled = true
	Multicast = false
	Port = 31978
	MulticastGroup = '239.31.97.8'
	Unknown = 'preserve me'
	`)
	if err := os.WriteFile(sourcePath, original, 0600); err != nil {
		t.Fatalf("create source configuration: %v", err)
	}
	if err := os.WriteFile(destinationPath, []byte("existing destination"), 0600); err != nil {
		t.Fatalf("create destination configuration: %v", err)
	}
	configuration := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
		Games:          GamesConfiguration{Enabled: []string{"age2"}},
		Announcement:   AnnouncementConfig{Port: 32000, MulticastGroup: "239.31.97.8"},
	}
	service := &ConfigService{}
	if _, err := service.SaveConfigurationAs(sourcePath, destinationPath, configuration, false); err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("SaveConfigurationAs error = %v, want overwrite confirmation", err)
	}
	file, err := service.SaveConfigurationAs(sourcePath, destinationPath, configuration, true)
	if err != nil {
		t.Fatalf("SaveConfigurationAs returned an error: %v", err)
	}
	if file.Path != destinationPath || file.Config.Announcement.Port != 32000 {
		t.Fatalf("unexpected saved configuration: %+v", file)
	}
	if !strings.Contains(file.Content, "# Preserve this comment.") || !strings.Contains(file.Content, "Unknown = 'preserve me'") {
		t.Fatalf("save-as did not preserve unedited content: %s", file.Content)
	}
	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source configuration: %v", err)
	}
	if string(sourceContent) != string(original) {
		t.Fatalf("save-as changed source configuration: %s", sourceContent)
	}
}

func TestWriteNewConfigFileDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	if err := writeNewConfigFile(path, []byte("new")); err != nil {
		t.Fatalf("writeNewConfigFile returned an error: %v", err)
	}
	if err := writeNewConfigFile(path, []byte("replacement")); err == nil {
		t.Fatal("writeNewConfigFile overwrote an existing file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created configuration: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("created content = %q, want %q", content, "new")
	}
}

func TestCreateConfigFileUsesOfficialTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	configuration := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
		Games: GamesConfiguration{
			Enabled: []string{"age2"},
		},
		Announcement: AnnouncementConfig{
			Enabled:        true,
			Multicast:      false,
			Port:           32000,
			MulticastGroup: "239.31.97.8",
		},
	}
	templatePath, err := templateConfigPath()
	if err != nil {
		t.Fatalf("templateConfigPath returned an error: %v", err)
	}
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	updated, err := configurationDocument(templateContent, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if _, err := parseConfiguration(updated); err != nil {
		t.Fatalf("configurationDocument produced invalid TOML:\n%s\n%v", updated, err)
	}
	file, err := (&ConfigService{}).CreateConfigFile(path, configuration, false)
	if err != nil {
		t.Fatalf("CreateConfigFile returned an error: %v", err)
	}
	if file.Path != path || file.Config.Announcement.Port != 32000 {
		t.Fatalf("unexpected created file: %+v", file)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created configuration: %v", err)
	}
	if !strings.Contains(string(content), "# Whether to log all the info") {
		t.Fatal("created configuration did not preserve the official template comments")
	}
}

func TestCreateConfigFileRequiresOverwriteConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatalf("create existing configuration: %v", err)
	}
	configuration := Configuration{
		Authentication: "disabled",
		Hosts:          HostsConfiguration{Modern: []string{"127.0.0.1"}},
		Games:          GamesConfiguration{Enabled: []string{"age2"}},
		Announcement:   AnnouncementConfig{Port: 31978},
	}
	if _, err := (&ConfigService{}).CreateConfigFile(path, configuration, false); err == nil {
		t.Fatal("CreateConfigFile did not require overwrite confirmation")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing configuration: %v", err)
	}
	if string(content) != "existing" {
		t.Fatalf("existing content changed after rejected creation: %q", content)
	}
}

// The embedded battle server table is optional: a document that does not have
// it must not gain one, and a table with a non-default value is written with
// only the keys that carry one.
func TestGameDocumentEmbeddedBattleServerTable(t *testing.T) {
	original := []byte("[Game]\n")
	updated, err := gameDocument("age2", original, GameConfig{})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "EmbeddedBattleServer") {
		t.Fatalf("a default embedded battle server was written out:\n%s", updated)
	}

	// A value the server would not resolve by itself has to reach the document.
	updated, err = gameDocument("age2", original, GameConfig{
		EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "false"},
	})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if !strings.Contains(string(updated), "[Game.EmbeddedBattleServer]\nStartIfMissing = 'false'\n") {
		t.Fatalf("the embedded battle server table was not written:\n%s", updated)
	}
	// Keys the server resolves on its own stay out while they are empty.
	for _, key := range []string{"Region", "Name", "IPv4", "LogLevel"} {
		if strings.Contains(string(updated), key+" = ") {
			t.Fatalf("%s was written although it is empty:\n%s", key, updated)
		}
	}

	parsed, err := parseGameConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if parsed.EmbeddedBattleServer.StartIfMissing != "false" {
		t.Fatalf("unexpected embedded StartIfMissing: %#v", parsed.EmbeddedBattleServer)
	}

	// And going back to the server's own answer removes it again.
	updated, err = gameDocument("age2", updated, GameConfig{})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "EmbeddedBattleServer") {
		t.Fatalf("the default table was kept:\n%s", updated)
	}
}

// The editor edits the tables it knows key by key, so the comments a user wrote
// inside them survive a save.
func TestGameDocumentKeepsCommentsInEditedTables(t *testing.T) {
	original := []byte(`[Game]
Hosts = ['0.0.0.0']

[[Game.BattleServers]]
# Description of this server.
Region = 'lan' # keep the region comment
IPv4 = 'auto'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005
Extra = 'keep me' # a key this editor does not know

# This comment closes the game and must stay.
`)
	game := GameConfig{
		BattleServers: []BattleServerConfig{
			{Region: "lan", IPv4: "auto", BsPort: 30013, WebSocketPort: 30014, OutOfBandPort: 30015},
		},
		EmbeddedBattleServer: EmbeddedBattleServerConfig{Region: "embedded"},
	}
	updated, err := gameDocument("age2", original, game)
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	for _, want := range []string{
		"# Description of this server.",
		"Region = 'lan' # keep the region comment",
		"Extra = 'keep me' # a key this editor does not know",
		"# This comment closes the game and must stay.",
	} {
		if !strings.Contains(string(updated), want) {
			t.Fatalf("%q was lost:\n%s", want, updated)
		}
	}
	if !strings.Contains(string(updated), "BsPort = 30013") {
		t.Fatalf("the port was not updated:\n%s", updated)
	}
	parsed, err := parseGameConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.BattleServers, game.BattleServers) {
		t.Fatalf("unexpected battle servers: %#v", parsed.BattleServers)
	}
	if parsed.EmbeddedBattleServer.Region != "embedded" {
		t.Fatalf("unexpected embedded battle server: %#v", parsed.EmbeddedBattleServer)
	}
}

// An existing embedded battle server table is edited in place too. Only the
// keys that differ from the inherited values are written, and a key that
// matches them is dropped so config.toml keeps applying.
func TestGameDocumentKeepsCommentsInEmbeddedTable(t *testing.T) {
	original := []byte(`[Game]

[Game.EmbeddedBattleServer]
# Pinned so clients always find it.
StartIfMissing = 'false' # start it when nothing else answers
Region = 'old'
IPv4 = 'auto'
BsPort = 0
WebSocketPort = 0
OutOfBandPort = 0
`)
	game := GameConfig{EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "false", Region: "new"}}
	updated, err := gameDocument("age2", original, game)
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	for _, want := range []string{
		"# Pinned so clients always find it.",
		"StartIfMissing = 'false' # start it when nothing else answers",
		"Region = 'new'",
	} {
		if !strings.Contains(string(updated), want) {
			t.Fatalf("%q was lost:\n%s", want, updated)
		}
	}
}

// A per-game file only writes what differs from config.toml: an inherited
// port is not pinned to a zero after an open and save.
func TestGameDocumentEmbeddedDeltaAgainstInheritedValues(t *testing.T) {
	base := EmbeddedBattleServerConfig{StartIfMissing: "true", BsPort: 30003}
	inherited := EmbeddedBattleServerConfig{StartIfMissing: "true", BsPort: 30003}
	updated, err := gameDocumentWithBase("age2", nil, GameConfig{EmbeddedBattleServer: inherited}, base)
	if err != nil {
		t.Fatalf("gameDocumentWithBase returned an error: %v", err)
	}
	if updated != nil {
		t.Fatalf("an all-inherited table was written:\n%s", updated)
	}

	changed := inherited
	changed.BsPort = 30005
	updated, err = gameDocumentWithBase("age2", nil, GameConfig{EmbeddedBattleServer: changed}, base)
	if err != nil {
		t.Fatalf("gameDocumentWithBase returned an error: %v", err)
	}
	if !strings.Contains(string(updated), "BsPort = 30005") {
		t.Fatalf("the changed port was not written:\n%s", updated)
	}
	if strings.Contains(string(updated), "StartIfMissing") || strings.Contains(string(updated), "WebSocketPort") {
		t.Fatalf("inherited keys were written:\n%s", updated)
	}
}

// Reading a per-game file overlays only the keys the file assigns, the same
// field-by-field merge the server does.
func TestReadGameFilesMergesEmbeddedBattleServerByLeaf(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.toml")
	main := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2.EmbeddedBattleServer]
Region = 'inherited'
BsPort = 30003
`)
	if err := os.WriteFile(path, main, 0600); err != nil {
		t.Fatalf("write main configuration: %v", err)
	}
	if err := os.WriteFile(gameFilePath(path, "age2"), []byte("[Game.EmbeddedBattleServer]\nStartIfMissing = 'false'\n"), 0600); err != nil {
		t.Fatalf("write game configuration: %v", err)
	}
	config, err := parseConfiguration(main)
	if err != nil {
		t.Fatalf("parse main configuration: %v", err)
	}
	if err := readGameFiles(path, config); err != nil {
		t.Fatalf("readGameFiles returned an error: %v", err)
	}
	embedded := config.Games.Age2.EmbeddedBattleServer
	if embedded.StartIfMissing != "false" {
		t.Fatalf("StartIfMissing = %q, want the file value", embedded.StartIfMissing)
	}
	if embedded.Region != "inherited" || embedded.BsPort != 30003 {
		t.Fatalf("the file dropped inherited values: %#v", embedded)
	}
}

// A v2.0 document without [Hosts] migrates its per-game hosts to the class of
// the enabled games instead of losing them on the next save.
func TestReadConfigFileMigratesGameHosts(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.toml")
	content := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['127.0.0.1']

[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	file, err := (&ConfigService{}).readConfigFile(path)
	if err != nil {
		t.Fatalf("readConfigFile returned an error: %v", err)
	}
	if len(file.Config.Hosts.Legacy) != 1 || file.Config.Hosts.Legacy[0] != "127.0.0.1" {
		t.Fatalf("Hosts.Legacy = %#v, want the migrated list", file.Config.Hosts.Legacy)
	}
	if len(file.Config.Hosts.Modern) != 0 {
		t.Fatalf("Hosts.Modern = %#v, want it emptied", file.Config.Hosts.Modern)
	}
	updated, err := prepareConfigurationDocument(content, *file.Config)
	if err != nil {
		t.Fatalf("prepareConfigurationDocument returned an error: %v", err)
	}
	saved, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("saved document is invalid: %v", err)
	}
	if len(saved.Hosts.Legacy) != 1 || saved.Hosts.Legacy[0] != "127.0.0.1" {
		t.Fatalf("saved Hosts.Legacy = %#v, want the migrated list", saved.Hosts.Legacy)
	}
	if len(saved.Games.Age2.Hosts) != 0 {
		t.Fatalf("saved Games.age2.Hosts = %#v, want the per-game key removed", saved.Games.Age2.Hosts)
	}
}

// Differing per-game host lists and no [Hosts] cannot be saved silently.
func TestPrepareConfigurationDocumentRefusesDifferingGameHosts(t *testing.T) {
	content := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age1', 'age2']

[Games.age1]
Hosts = ['127.0.0.1']

[Games.age2]
Hosts = ['192.168.1.10']
`)
	config, err := parseConfiguration(content)
	if err != nil {
		t.Fatalf("parse configuration: %v", err)
	}
	if _, err := prepareConfigurationDocument(content, *config); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatalf("prepareConfigurationDocument error = %v, want the differing-lists refusal", err)
	}
}

// Removing every battle server of a game drops the blocks, not the comment that
// closes the game after them.
func TestGameDocumentKeepsCommentsAfterRemovedBlocks(t *testing.T) {
	original := []byte(`[Game]
Hosts = ['0.0.0.0']

[[Game.BattleServers]]
Region = 'lan'
IPv4 = 'auto'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005

# This comment closes the game and must stay.
`)
	updated, err := gameDocument("age2", original, GameConfig{})
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "[[Game.BattleServers]]") {
		t.Fatalf("the block was not removed:\n%s", updated)
	}
	if !strings.Contains(string(updated), "# This comment closes the game and must stay.") {
		t.Fatalf("the trailing comment was removed with the block:\n%s", updated)
	}
}

// A document without listeners gains the global [Hosts] table, and the old
// per-game keys disappear instead of being written with a stale value.
func TestConfigurationDocumentAddsGlobalHostsAndDropsPerGameHosts(t *testing.T) {
	original := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['127.0.0.1']

[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if !strings.Contains(string(updated), "[Hosts]") || !strings.Contains(string(updated), "Modern = ['0.0.0.0']") {
		t.Fatalf("the global [Hosts] table was not written:\n%s", updated)
	}
	if strings.Contains(string(updated), "[Games.age2]\nHosts = ") {
		t.Fatalf("the deprecated per-game Hosts key was kept:\n%s", updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if len(parsed.Hosts.Modern) != 1 || parsed.Hosts.Modern[0] != "0.0.0.0" || len(parsed.Games.Age2.Hosts) != 0 {
		t.Fatalf("unexpected hosts after save: %#v / %#v", parsed.Hosts, parsed.Games.Age2.Hosts)
	}
}

// Every comment the official template carries must survive a save that changes
// values across the document.
func TestConfigurationDocumentKeepsEveryTemplateComment(t *testing.T) {
	templatePath, err := templateConfigPath()
	if err != nil {
		t.Fatalf("templateConfigPath returned an error: %v", err)
	}
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	configuration, err := parseConfiguration(content)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	configuration.Log = true
	configuration.Announcement.Port = 32100
	configuration.CustomFiles.MaxFileSize = "64MB"
	configuration.Games.Enabled = []string{"age2", "age4"}
	configuration.Games.Age2.BattleServers = []BattleServerConfig{
		{Region: "local", Name: "Local", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
	}
	configuration.Games.Age2.EmbeddedBattleServer.Region = "embedded"
	configuration.Games.Age2.EmbeddedBattleServer.LogLevel = "info"

	updated, err := configurationDocument(content, *configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		comment := strings.TrimSpace(line)
		if !strings.HasPrefix(comment, "#") {
			continue
		}
		if !strings.Contains(string(updated), comment) {
			t.Fatalf("the template comment %q was lost:\n%s", comment, updated)
		}
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !parsed.Log || parsed.Announcement.Port != 32100 || parsed.CustomFiles.MaxFileSize != "64MB" {
		t.Fatalf("the changed values did not round-trip: %#v", parsed)
	}
	if len(parsed.Games.Age2.BattleServers) != 0 {
		t.Fatalf("battle servers were written to config.toml: %#v", parsed.Games.Age2.BattleServers)
	}
	document, err := gameDocument("age2", nil, configuration.Games.Age2)
	if err != nil {
		t.Fatalf("gameDocument returned an error: %v", err)
	}
	game, err := parseGameConfiguration(document)
	if err != nil {
		t.Fatalf("per-game document could not be parsed: %v\n%s", err, document)
	}
	if len(game.BattleServers) != 1 || game.BattleServers[0].Region != "local" {
		t.Fatalf("unexpected battle servers: %#v", game.BattleServers)
	}
	if game.EmbeddedBattleServer.Region != "embedded" {
		t.Fatalf("unexpected embedded battle server: %#v", game.EmbeddedBattleServer)
	}
}

// Saving config.toml writes that document and nothing else. The per-game files
// belong to documents of their own, so their content is left untouched.
func TestSaveConfigurationWritesOnlyTheMainDocument(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.toml")
	gamePath := filepath.Join(directory, "config.age2.toml")
	gameContent := []byte("[Game]\n\n[[Game.BattleServers]]\n# Owned by config.age2.toml.\nRegion = 'lan'\nIPv4 = 'auto'\nBsPort = 30003\nWebSocketPort = 30004\nOutOfBandPort = 30005\n")
	if err := os.WriteFile(gamePath, gameContent, 0600); err != nil {
		t.Fatalf("create per-game configuration: %v", err)
	}
	original := []byte(`Authentication = 'disabled'

[Hosts]
Modern = ['127.0.0.1']
Legacy = []

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['192.0.2.10']

[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatalf("create test configuration: %v", err)
	}
	configuration, err := parseConfiguration(original)
	if err != nil {
		t.Fatalf("parse test configuration: %v", err)
	}
	configuration.Games.Age2.BattleServers = []BattleServerConfig{
		{Region: "other", IPv4: "auto", BsPort: 30009, WebSocketPort: 30004, OutOfBandPort: 30005},
	}
	configuration.Games.Age2.EmbeddedBattleServer = EmbeddedBattleServerConfig{StartIfMissing: "false"}
	service := &ConfigService{}
	if err := service.SaveConfiguration(path, *configuration); err != nil {
		t.Fatalf("SaveConfiguration returned an error: %v", err)
	}
	mainContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved configuration: %v", err)
	}
	for _, unwanted := range []string{"[[Games.age2.BattleServers]]", "[Games.age2.EmbeddedBattleServer]"} {
		if strings.Contains(string(mainContent), unwanted) {
			t.Fatalf("%q was written to config.toml:\n%s", unwanted, mainContent)
		}
	}
	if strings.Contains(string(mainContent), "Hosts = ['192.0.2.10']") {
		t.Fatalf("the deprecated per-game hosts were kept:\n%s", mainContent)
	}
	savedGame, err := os.ReadFile(gamePath)
	if err != nil {
		t.Fatalf("read per-game configuration: %v", err)
	}
	if string(savedGame) != string(gameContent) {
		t.Fatalf("saving config.toml changed the per-game document:\n%s", savedGame)
	}
}

// Saving a configuration somewhere else writes only that document: the
// per-game files are not copied along with it.
func TestSaveConfigurationAsWritesOnlyTheDestination(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.toml")
	destinationPath := filepath.Join(directory, "destination.toml")
	mainContent := []byte(`Authentication = 'disabled'

[Hosts]
Modern = ['127.0.0.1']
Legacy = []

[Games]
Enabled = ['age2']
`)
	gameContent := []byte("[Game]\n\n[[Game.BattleServers]]\n# Keep this comment.\nRegion = 'lan'\n")
	if err := os.WriteFile(sourcePath, mainContent, 0600); err != nil {
		t.Fatalf("create source configuration: %v", err)
	}
	gamePath := filepath.Join(directory, "config.age2.toml")
	if err := os.WriteFile(gamePath, gameContent, 0600); err != nil {
		t.Fatalf("create source game configuration: %v", err)
	}
	service := &ConfigService{}
	loaded, err := service.readConfigFile(sourcePath)
	if err != nil {
		t.Fatalf("read source configuration: %v", err)
	}
	configuration := *loaded.Config
	configuration.Hosts.Modern = []string{"192.0.2.20"}
	file, err := service.SaveConfigurationAs(sourcePath, destinationPath, configuration, true)
	if err != nil {
		t.Fatalf("SaveConfigurationAs returned an error: %v", err)
	}
	if file.Path != destinationPath {
		t.Fatalf("unexpected saved file: %+v", file)
	}
	destination, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatalf("read destination configuration: %v", err)
	}
	if !strings.Contains(string(destination), "['192.0.2.20']") {
		t.Fatalf("the destination was not written:\n%s", destination)
	}
	savedGame, err := os.ReadFile(gamePath)
	if err != nil {
		t.Fatalf("read per-game configuration: %v", err)
	}
	if string(savedGame) != string(gameContent) {
		t.Fatalf("save as changed the per-game document:\n%s", savedGame)
	}
}

// A per-game document is its own file: reading it exposes only its game, and a
// save rewrites that document without touching the config.toml next to it.
func TestSaveGameConfigurationWritesOnlyThatFile(t *testing.T) {
	directory := t.TempDir()
	mainPath := filepath.Join(directory, "config.toml")
	gamePath := filepath.Join(directory, "config.age2.toml")
	mainContent := []byte(`Authentication = 'disabled'

[Hosts]
Modern = ['127.0.0.1']
Legacy = []

[Games]
Enabled = ['age2']

[Games.age2.EmbeddedBattleServer]
Region = 'inherited'
BsPort = 30003
`)
	gameContent := []byte("[Game]\n\n[[Game.BattleServers]]\nRegion = 'lan'\nIPv4 = 'auto'\nBsPort = 30003\nWebSocketPort = 30004\nOutOfBandPort = 30005\n")
	if err := os.WriteFile(mainPath, mainContent, 0600); err != nil {
		t.Fatalf("create main configuration: %v", err)
	}
	if err := os.WriteFile(gamePath, gameContent, 0600); err != nil {
		t.Fatalf("create per-game configuration: %v", err)
	}
	service := &ConfigService{}
	document, err := service.readConfigFile(gamePath)
	if err != nil {
		t.Fatalf("readConfigFile returned an error: %v", err)
	}
	if document.Kind != documentGame || document.Game != "age2" {
		t.Fatalf("unexpected document kind: %+v", document)
	}
	if document.New || document.Path != gamePath || document.SuggestedName != "config.age2.toml" {
		t.Fatalf("unexpected document state: %+v", document)
	}
	if !slices.Equal(document.Config.Games.Enabled, []string{"age2"}) {
		t.Fatalf("a per-game document enabled other games: %v", document.Config.Games.Enabled)
	}

	configuration := *document.Config
	configuration.Games.Age2.BattleServers = append(configuration.Games.Age2.BattleServers, BattleServerConfig{
		Region: "extra", IPv4: "auto", BsPort: 30103, WebSocketPort: 30104, OutOfBandPort: 30105,
	})
	if err := service.SaveConfiguration(gamePath, configuration); err != nil {
		t.Fatalf("SaveConfiguration returned an error: %v", err)
	}
	savedMain, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main configuration: %v", err)
	}
	if string(savedMain) != string(mainContent) {
		t.Fatalf("saving a per-game document changed config.toml:\n%s", savedMain)
	}
	savedGame, err := os.ReadFile(gamePath)
	if err != nil {
		t.Fatalf("read per-game configuration: %v", err)
	}
	if !strings.Contains(string(savedGame), "Region = 'extra'") {
		t.Fatalf("the new battle server was not written:\n%s", savedGame)
	}
	if strings.Contains(string(savedGame), "Region = 'inherited'") || strings.Contains(string(savedGame), "[Game.EmbeddedBattleServer]") {
		t.Fatalf("the per-game document pinned inherited values:\n%s", savedGame)
	}
}

// A per-game document starts from the server defaults, inherits the embedded
// battle server of the sibling config.toml and keeps only its own game.
func TestReadGameDocumentInheritsFromMainConfiguration(t *testing.T) {
	directory := t.TempDir()
	mainPath := filepath.Join(directory, "config.toml")
	gamePath := filepath.Join(directory, "config.age2.toml")
	mainContent := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2.EmbeddedBattleServer]
Region = 'inherited'
BsPort = 30003
`)
	gameContent := []byte("[Game.EmbeddedBattleServer]\nStartIfMissing = 'false'\n")
	if err := os.WriteFile(mainPath, mainContent, 0600); err != nil {
		t.Fatalf("create main configuration: %v", err)
	}
	if err := os.WriteFile(gamePath, gameContent, 0600); err != nil {
		t.Fatalf("create per-game configuration: %v", err)
	}
	document, err := (&ConfigService{}).readConfigFile(gamePath)
	if err != nil {
		t.Fatalf("readConfigFile returned an error: %v", err)
	}
	embedded := document.Config.Games.Age2.EmbeddedBattleServer
	if embedded.StartIfMissing != "false" {
		t.Fatalf("StartIfMissing = %q, want the file value", embedded.StartIfMissing)
	}
	if embedded.Region != "inherited" || embedded.BsPort != 30003 {
		t.Fatalf("the per-game document dropped inherited values: %#v", embedded)
	}
	if document.Config.Authentication != "disabled" {
		t.Fatalf("the per-game document did not start from the server defaults: %+v", document.Config)
	}
}

func TestDocumentGameID(t *testing.T) {
	cases := map[string]string{
		"config.age2.toml":     "age2",
		"CONFIG.Athens.toml":   "athens",
		"config.toml":          "",
		"config.unknown.toml":  "",
		"other.age2.toml":      "",
		"config.age2.yaml":     "",
		"config.age2.toml.bak": "",
	}
	for name, want := range cases {
		gameID, ok := documentGameID(filepath.Join("dir", name))
		if want == "" {
			if ok {
				t.Fatalf("%q was taken as a per-game document for %q", name, gameID)
			}
			continue
		}
		if !ok || gameID != want {
			t.Fatalf("%q: got %q (ok=%v), want %q", name, gameID, ok, want)
		}
	}
}

// Creating a document whose name is config.<game>.toml writes that file only.
func TestCreateConfigFileCreatesGameDocument(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.age3.toml")
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age3"}
	configuration.Games.Age3.BattleServers = []BattleServerConfig{
		{Region: "lan", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
	}
	document, err := (&ConfigService{}).CreateConfigFile(path, configuration, false)
	if err != nil {
		t.Fatalf("CreateConfigFile returned an error: %v", err)
	}
	if document.Kind != documentGame || document.Game != "age3" || document.New {
		t.Fatalf("unexpected document: %+v", document)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created configuration: %v", err)
	}
	for _, want := range []string{"[[Game.BattleServers]]", "Region = 'lan'"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("%q was not written:\n%s", want, content)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, configFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("creating a per-game document created config.toml: %v", err)
	}
}

// Opening a game document from another one targets the sibling file, and a
// missing file becomes a new document aimed at that path instead of a failure.
func TestOpenGameDocumentTargetsSiblingFile(t *testing.T) {
	directory := t.TempDir()
	mainPath := filepath.Join(directory, "config.toml")
	if err := os.WriteFile(mainPath, []byte("Authentication = 'disabled'\n\n[Games]\nEnabled = ['age4']\n"), 0600); err != nil {
		t.Fatalf("create main configuration: %v", err)
	}
	service := &ConfigService{}
	document, err := service.OpenGameDocument(mainPath, "age4")
	if err != nil {
		t.Fatalf("OpenGameDocument returned an error: %v", err)
	}
	target := filepath.Join(directory, "config.age4.toml")
	if !document.New || document.Path != target || document.Game != "age4" {
		t.Fatalf("unexpected document: %+v", document)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenGameDocument created the file: %v", err)
	}

	if err := os.WriteFile(target, []byte("[Game]\n\n[[Game.BattleServers]]\nRegion = 'lan'\n"), 0600); err != nil {
		t.Fatalf("create per-game configuration: %v", err)
	}
	document, err = service.OpenGameDocument(mainPath, "age4")
	if err != nil {
		t.Fatalf("OpenGameDocument returned an error: %v", err)
	}
	if document.New || document.Path != target || len(document.Config.Games.Age4.BattleServers) != 1 {
		t.Fatalf("unexpected loaded document: %+v", document)
	}
}

func TestOpenGameTemplate(t *testing.T) {
	document, err := (&ConfigService{}).OpenGameTemplate("age4")
	if err != nil {
		t.Fatalf("OpenGameTemplate returned an error: %v", err)
	}
	if !document.New || document.Path != "" || document.Kind != documentGame || document.Game != "age4" {
		t.Fatalf("unexpected document: %+v", document)
	}
	if document.SuggestedName != "config.age4.toml" {
		t.Fatalf("unexpected suggested name: %q", document.SuggestedName)
	}
	if !slices.Equal(document.Config.Games.Enabled, []string{"age4"}) {
		t.Fatalf("unexpected enabled games: %v", document.Config.Games.Enabled)
	}
	if _, err := (&ConfigService{}).OpenGameTemplate("unknown"); err == nil {
		t.Fatal("an unknown game was accepted")
	}
}
