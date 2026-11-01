package main

import (
	"os"
	"path/filepath"
	"reflect"
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
	if !config.CustomFiles.Enabled || config.CustomFiles.Folder != "auto" ||
		config.CustomFiles.MaxFileSize != "32MB" || config.CustomFiles.MaxTotalSize != "256MB" ||
		config.CustomFiles.Expiry != "12h" {
		t.Fatalf("unexpected custom files defaults: %#v", config.CustomFiles)
	}
	if len(config.Games.Age2.Hosts) != 1 || config.Games.Age2.Hosts[0] != "0.0.0.0" {
		t.Fatalf("unexpected host defaults: %#v", config.Games.Age2.Hosts)
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
				Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
				Announcement:   AnnouncementConfig{Port: 31978},
			},
		},
		{
			name: "unknown authentication",
			config: Configuration{
				Authentication: "unknown",
				Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
				Announcement:   AnnouncementConfig{Port: 31978},
			},
			field:     "Authentication",
			hasErrors: true,
		},
		{
			name: "empty games list only warns",
			config: Configuration{
				Authentication: "disabled",
				Announcement:   AnnouncementConfig{Port: 31978},
			},
			field:      "Games.Enabled",
			hasWarning: true,
		},
		{
			name: "invalid port",
			config: Configuration{
				Authentication: "disabled",
				Announcement:   AnnouncementConfig{Port: 65536},
			},
			field:     "Announcement.Port",
			hasErrors: true,
		},
		{
			name: "unknown game",
			config: Configuration{
				Authentication: "disabled",
				Games:          GamesConfiguration{Enabled: []string{"age5"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
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
	// Two enabled games cannot bind the same address: it is an error on both
	// sides, as neither of them could be served.
	config := Configuration{
		Authentication: "disabled",
		Games: GamesConfiguration{
			Enabled: []string{"age1", "age2"},
			Age1:    GameConfig{Hosts: []string{"192.0.2.10"}},
			Age2:    GameConfig{Hosts: []string{"192.0.2.10"}},
		},
		Announcement: AnnouncementConfig{Port: 31978},
	}
	issues := validateConfiguration(&config)
	errors := errorValidationIssues(issues)
	if len(errors) != 2 {
		t.Fatalf("the shared address was not reported on both games: %#v", issues)
	}
	if errors[0].Field != "Games.age1.Hosts.0" || errors[1].Field != "Games.age2.Hosts.0" {
		t.Fatalf("unexpected error fields: %#v", errors)
	}
	for _, issue := range issues {
		if issue.Severity == severityWarning {
			t.Fatalf("the conflict between enabled games warned: %#v", issues)
		}
	}

	// A game that is not enabled is not bound, so it does not take the address
	// away from the enabled one: only the disabled one is told about it.
	config = Configuration{
		Authentication: "disabled",
		Games: GamesConfiguration{
			Enabled: []string{"age1"},
			Age1:    GameConfig{Hosts: []string{"192.0.2.10"}},
			Age2:    GameConfig{Hosts: []string{"192.0.2.10"}},
		},
		Announcement: AnnouncementConfig{Port: 31978},
	}
	issues = validateConfiguration(&config)
	if errors := errorValidationIssues(issues); len(errors) != 0 {
		t.Fatalf("a disabled game blocked the enabled one: %#v", errors)
	}
	if len(issues) != 1 || issues[0].Field != "Games.age2.Hosts.0" || issues[0].Severity != severityWarning {
		t.Fatalf("the disabled game was not warned about: %#v", issues)
	}

	// Two disabled games do not conflict at all, they only warn about it.
	config = Configuration{
		Authentication: "disabled",
		Games: GamesConfiguration{
			Enabled: []string{"age1"},
			Age1:    GameConfig{Hosts: []string{"192.0.2.10"}},
			Age2:    GameConfig{Hosts: []string{"192.0.2.10"}},
			Age3:    GameConfig{Hosts: []string{"192.0.2.10"}},
		},
		Announcement: AnnouncementConfig{Port: 31978},
	}
	issues = validateConfiguration(&config)
	if errors := errorValidationIssues(issues); len(errors) != 0 {
		t.Fatalf("the disabled games blocked saving: %#v", errors)
	}
	if len(issues) != 2 {
		t.Fatalf("the disabled games were not warned about: %#v", issues)
	}
	for _, issue := range issues {
		if issue.Severity != severityWarning {
			t.Fatalf("a disabled game reported an error: %#v", issues)
		}
	}
}
func TestValidateConfigurationDuplicateHostsWithinGame(t *testing.T) {
	config := Configuration{
		Authentication: "disabled",
		Games: GamesConfiguration{
			Enabled: []string{"age2"},
			Age2:    GameConfig{Hosts: []string{"192.0.2.10", "192.0.2.10"}},
		},
		Announcement: AnnouncementConfig{Port: 31978},
	}
	issues := validateConfiguration(&config)
	errors := errorValidationIssues(issues)
	if len(errors) != 2 {
		t.Fatalf("the duplicated host was not reported on both rows: %#v", issues)
	}
	for _, issue := range errors {
		if !strings.Contains(issue.Message, "age2") {
			t.Fatalf("a duplicated host did not name its game: %#v", issue)
		}
		if strings.HasSuffix(issue.Message, "by ") {
			t.Fatalf("a duplicated host produced an empty owner list: %#v", issue)
		}
	}
}

func TestValidateConfigurationPrivilegedPortWarnsWithoutBlocking(t *testing.T) {
	config := Configuration{
		Authentication: "disabled",
		Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
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
		Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
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
	if len(parsed.Games.Age2.Hosts) != 1 || parsed.Games.Age2.Hosts[0] != "0.0.0.0" {
		t.Fatalf("unexpected default hosts: %#v", parsed.Games.Age2.Hosts)
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

func TestConfigurationDocumentWritesBattleServersAsTables(t *testing.T) {
	original := []byte("Authentication = 'disabled'\n\n[Games]\nEnabled = []\n")
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}
	configuration.Games.Age2.BattleServers = []BattleServerConfig{
		{Region: "local", Name: "Local", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
		{Region: "other", IPv4: "192.168.1.2", BsPort: 30013, WebSocketPort: 30014, OutOfBandPort: 30015},
	}

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "BattleServers =") {
		t.Fatalf("battle servers were written as an inline array:\n%s", updated)
	}
	if count := strings.Count(string(updated), "[[Games.age2.BattleServers]]"); count != 2 {
		t.Fatalf("battle server tables = %d, want 2:\n%s", count, updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.Games.Age2.BattleServers, configuration.Games.Age2.BattleServers) {
		t.Fatalf("battle servers did not round-trip: %#v", parsed.Games.Age2.BattleServers)
	}

	// Saving again must not append a second copy of the same tables.
	twice, err := configurationDocument(updated, configuration)
	if err != nil {
		t.Fatalf("second configurationDocument returned an error: %v", err)
	}
	if string(twice) != string(updated) {
		t.Fatalf("a second save changed the document:\nfirst:\n%s\nsecond:\n%s", updated, twice)
	}
}

func TestConfigurationDocumentReplacesInlineBattleServers(t *testing.T) {
	original := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']
BattleServers = [{Region = 'old', IPv4 = 'auto', BsPort = 1, WebSocketPort = 2}]
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}
	configuration.Games.Age2.BattleServers = []BattleServerConfig{
		{Region: "local", Name: "Local", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
	}

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "BattleServers = [") {
		t.Fatalf("the inline BattleServers key was kept:\n%s", updated)
	}
	if !strings.Contains(string(updated), "[[Games.age2.BattleServers]]") {
		t.Fatalf("battle servers were not written as tables:\n%s", updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.Games.Age2.BattleServers, configuration.Games.Age2.BattleServers) {
		t.Fatalf("battle servers did not round-trip: %#v", parsed.Games.Age2.BattleServers)
	}
}

func TestConfigurationDocumentRemovesEmptyBattleServers(t *testing.T) {
	original := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']

[[Games.age2.BattleServers]]
Region = 'local'
IPv4 = 'auto'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "BattleServers") {
		t.Fatalf("an empty battle server list was kept:\n%s", updated)
	}
	if _, err := parseConfiguration(updated); err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
}

func TestConfigurationDocumentHandlesInterleavedBattleServerTables(t *testing.T) {
	original := []byte(`Authentication = 'disabled'

[Games]
Enabled = ['age2', 'age3']

[Games.age2]
Hosts = ['0.0.0.0']

[[Games.age2.BattleServers]]
Region = 'old-age2'
IPv4 = 'auto'
BsPort = 1
WebSocketPort = 2

[Games.age3]
Hosts = ['0.0.0.0']

[[Games.age3.BattleServers]]
Region = 'age3'
IPv4 = 'auto'
BsPort = 3
WebSocketPort = 4

[[Games.age2.BattleServers]]
Region = 'old-age2-2'
IPv4 = 'auto'
BsPort = 5
WebSocketPort = 6

[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2", "age3"}
	configuration.Games.Age2.BattleServers = []BattleServerConfig{
		{Region: "new-age2", IPv4: "auto", BsPort: 30003, WebSocketPort: 30004, OutOfBandPort: 30005},
	}
	configuration.Games.Age3.BattleServers = []BattleServerConfig{
		{Region: "age3", IPv4: "auto", BsPort: 30006, WebSocketPort: 30007, OutOfBandPort: 30008},
	}

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "old-age2") {
		t.Fatalf("a stale battle server table was kept:\n%s", updated)
	}
	if count := strings.Count(string(updated), "[[Games.age2.BattleServers]]"); count != 1 {
		t.Fatalf("age2 battle server tables = %d, want 1:\n%s", count, updated)
	}
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.Games.Age2.BattleServers, configuration.Games.Age2.BattleServers) {
		t.Fatalf("unexpected age2 battle servers: %#v", parsed.Games.Age2.BattleServers)
	}
	if !reflect.DeepEqual(parsed.Games.Age3.BattleServers, configuration.Games.Age3.BattleServers) {
		t.Fatalf("unexpected age3 battle servers: %#v", parsed.Games.Age3.BattleServers)
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
		Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
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
		Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
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
		Games: GamesConfiguration{
			Enabled: []string{"age2"},
			Age2:    GameConfig{Hosts: []string{"127.0.0.1"}},
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
		Games:          GamesConfiguration{Enabled: []string{"age2"}, Age2: GameConfig{Hosts: []string{"127.0.0.1"}}},
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
func TestEmbeddedBattleServerTable(t *testing.T) {
	original := []byte(`[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}
	configuration.Games.Age2.Hosts = []string{"0.0.0.0"}

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "EmbeddedBattleServer") {
		t.Fatalf("a default embedded battle server was written out:\n%s", updated)
	}

	// A value the server would not resolve by itself has to reach the document.
	configuration.Games.Age2.EmbeddedBattleServer.StartIfMissing = "false"
	updated, err = configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if !strings.Contains(string(updated), "[Games.age2.EmbeddedBattleServer]\nStartIfMissing = 'false'\n") {
		t.Fatalf("the embedded battle server table was not written:\n%s", updated)
	}
	// Keys the server resolves on its own stay out while they are empty.
	for _, key := range []string{"Region", "Name", "IPv4", "LogLevel"} {
		if strings.Contains(string(updated), key+" = ") {
			t.Fatalf("%s was written although it is empty:\n%s", key, updated)
		}
	}

	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if parsed.Games.Age2.EmbeddedBattleServer.StartIfMissing != "false" {
		t.Fatalf("unexpected embedded StartIfMissing: %#v", parsed.Games.Age2.EmbeddedBattleServer)
	}

	// And going back to the server's own answer removes it again.
	configuration.Games.Age2.EmbeddedBattleServer.StartIfMissing = "true"
	updated, err = configurationDocument(updated, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	if strings.Contains(string(updated), "EmbeddedBattleServer") {
		t.Fatalf("the default table was kept:\n%s", updated)
	}
}

// The editor edits the tables it knows key by key, so the comments a user wrote
// inside them survive a save.
func TestConfigurationDocumentKeepsCommentsInEditedTables(t *testing.T) {
	original := []byte(`[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']

[[Games.age2.BattleServers]]
# Description of this server.
Region = 'lan' # keep the region comment
IPv4 = 'auto'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005
Extra = 'keep me' # a key this editor does not know

# This comment closes the game and must stay.
[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}
	configuration.Games.Age2.BattleServers = []BattleServerConfig{
		{Region: "lan", IPv4: "auto", BsPort: 30013, WebSocketPort: 30014, OutOfBandPort: 30015},
	}
	configuration.Games.Age2.EmbeddedBattleServer.Region = "embedded"

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
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
	parsed, err := parseConfiguration(updated)
	if err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
	}
	if !reflect.DeepEqual(parsed.Games.Age2.BattleServers, configuration.Games.Age2.BattleServers) {
		t.Fatalf("unexpected battle servers: %#v", parsed.Games.Age2.BattleServers)
	}
	if parsed.Games.Age2.EmbeddedBattleServer.Region != "embedded" {
		t.Fatalf("unexpected embedded battle server: %#v", parsed.Games.Age2.EmbeddedBattleServer)
	}
}

// An existing embedded battle server table is edited in place too.
func TestConfigurationDocumentKeepsCommentsInEmbeddedTable(t *testing.T) {
	original := []byte(`[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']

[Games.age2.EmbeddedBattleServer]
# Pinned so clients always find it.
StartIfMissing = 'true' # start it when nothing else answers
Region = 'old'
IPv4 = 'auto'
BsPort = 0
WebSocketPort = 0
OutOfBandPort = 0

[Announcement]
Enabled = true
Multicast = false
Port = 31978
MulticastGroup = '239.31.97.8'
`)
	configuration := serverDefaults()
	configuration.Games.Enabled = []string{"age2"}
	configuration.Games.Age2.EmbeddedBattleServer.Region = "new"

	updated, err := configurationDocument(original, configuration)
	if err != nil {
		t.Fatalf("configurationDocument returned an error: %v", err)
	}
	for _, want := range []string{
		"# Pinned so clients always find it.",
		"StartIfMissing = 'true' # start it when nothing else answers",
		"Region = 'new'",
	} {
		if !strings.Contains(string(updated), want) {
			t.Fatalf("%q was lost:\n%s", want, updated)
		}
	}
}

// Removing every battle server of a game drops the blocks, not the comment that
// closes the game after them.
func TestConfigurationDocumentKeepsCommentsAfterRemovedBlocks(t *testing.T) {
	original := []byte(`[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']

[[Games.age2.BattleServers]]
Region = 'lan'
IPv4 = 'auto'
BsPort = 30003
WebSocketPort = 30004
OutOfBandPort = 30005

# This comment closes the game and must stay.
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
	if strings.Contains(string(updated), "[[Games.age2.BattleServers]]") {
		t.Fatalf("the block was not removed:\n%s", updated)
	}
	if !strings.Contains(string(updated), "# This comment closes the game and must stay.") {
		t.Fatalf("the trailing comment was removed with the block:\n%s", updated)
	}
}

// A table the document does not have is added next to its siblings, so the
// document keeps reading in order after a save.
func TestConfigurationDocumentAddsMissingGameNextToTheOthers(t *testing.T) {
	original := []byte(`[Games]
Enabled = ['age2']

[Games.age2]
Hosts = ['0.0.0.0']

# Closing comment.
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
	text := string(updated)
	announcement := strings.Index(text, "[Announcement]")
	if announcement < 0 {
		t.Fatalf("the announcement table was lost:\n%s", updated)
	}
	for _, game := range []string{"[Games.age1]", "[Games.age3]", "[Games.age4]", "[Games.athens]"} {
		at := strings.Index(text, game)
		if at < 0 {
			t.Fatalf("%s was not added:\n%s", game, updated)
		}
		if at > announcement {
			t.Fatalf("%s was appended after the other tables:\n%s", game, updated)
		}
	}
	if _, err := parseConfiguration(updated); err != nil {
		t.Fatalf("updated document could not be parsed: %v\n%s", err, updated)
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
	for _, line := range strings.Split(string(content), "\n") {
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
	if len(parsed.Games.Age2.BattleServers) != 1 || parsed.Games.Age2.BattleServers[0].Region != "local" {
		t.Fatalf("unexpected battle servers: %#v", parsed.Games.Age2.BattleServers)
	}
	if parsed.Games.Age2.EmbeddedBattleServer.Region != "embedded" {
		t.Fatalf("unexpected embedded battle server: %#v", parsed.Games.Age2.EmbeddedBattleServer)
	}
}
