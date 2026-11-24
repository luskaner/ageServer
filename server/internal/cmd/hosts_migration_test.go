package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/server/internal"
)

func migrationConfig(enabled []string, hosts map[string][]string) internal.Configuration {
	config := internal.Configuration{
		Hosts: internal.Hosts{Modern: []string{"0.0.0.0"}},
		Games: internal.Games{Enabled: enabled},
	}
	for gameID, list := range hosts {
		switch gameID {
		case "age1":
			config.Games.Age1.Hosts = list
		case "age2":
			config.Games.Age2.Hosts = list
		case "age3":
			config.Games.Age3.Hosts = list
		case "age4":
			config.Games.Age4.Hosts = list
		case "athens":
			config.Games.Athens.Hosts = list
		}
	}
	return config
}

func writeMigrationConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	return path
}

func migrationConfigFor(t *testing.T, enabled []string) (string, internal.Configuration) {
	t.Helper()
	quoted := make([]string, len(enabled))
	hosts := make(map[string][]string, len(enabled))
	for index, gameID := range enabled {
		quoted[index] = "'" + gameID + "'"
		hosts[gameID] = []string{"127.0.0.1"}
	}
	path := writeMigrationConfig(t, "[Games]\nEnabled = ["+strings.Join(quoted, ", ")+"]\n")
	return path, migrationConfig(enabled, hosts)
}

func TestMigrateHostsDestinationByClass(t *testing.T) {
	cases := []struct {
		name       string
		enabled    []string
		wantLegacy []string
		wantModern []string
	}{
		{"all legacy", []string{"age1", "age2"}, []string{"127.0.0.1"}, nil},
		{"all modern", []string{"age4", "athens"}, nil, []string{"127.0.0.1"}},
		{"mixed", []string{"age2", "athens"}, nil, []string{"127.0.0.1"}},
		{"single legacy", []string{"age1"}, []string{"127.0.0.1"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, config := migrationConfigFor(t, tc.enabled)
			if err := migrateHosts(path, &config); err != nil {
				t.Fatalf("migrateHosts returned an error: %v", err)
			}
			if !slices.Equal(config.Hosts.Legacy, tc.wantLegacy) {
				t.Fatalf("Hosts.Legacy = %#v, want %#v", config.Hosts.Legacy, tc.wantLegacy)
			}
			if !slices.Equal(config.Hosts.Modern, tc.wantModern) {
				t.Fatalf("Hosts.Modern = %#v, want %#v", config.Hosts.Modern, tc.wantModern)
			}
		})
	}
}

func TestMigrateHostsDifferingListsFail(t *testing.T) {
	path := writeMigrationConfig(t, "[Games]\nEnabled = ['age1', 'age2']\n")
	config := migrationConfig([]string{"age1", "age2"}, map[string][]string{
		"age1": {"127.0.0.1"},
		"age2": {"192.168.1.10"},
	})
	err := migrateHosts(path, &config)
	if err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatalf("migrateHosts error = %v, want the differing-lists error", err)
	}
	if !strings.Contains(err.Error(), "age1") || !strings.Contains(err.Error(), "age2") ||
		!strings.Contains(err.Error(), "192.168.1.10") {
		t.Fatalf("migrateHosts error = %v, want both games and lists named", err)
	}
}

func TestMigrateHostsKeepsGlobalSection(t *testing.T) {
	path := writeMigrationConfig(t, "[Hosts]\nModern = ['192.0.2.1']\nLegacy = []\n")
	config := migrationConfig([]string{"age1", "age2"}, map[string][]string{
		"age1": {"127.0.0.1"},
		"age2": {"127.0.0.1"},
	})
	if err := migrateHosts(path, &config); err != nil {
		t.Fatalf("migrateHosts returned an error: %v", err)
	}
	if len(config.Hosts.Modern) != 1 || config.Hosts.Modern[0] != "0.0.0.0" {
		t.Fatalf("Hosts.Modern = %#v, want the configured default untouched", config.Hosts.Modern)
	}
	if len(config.Hosts.Legacy) != 0 {
		t.Fatalf("Hosts.Legacy = %#v, want it untouched", config.Hosts.Legacy)
	}
}
