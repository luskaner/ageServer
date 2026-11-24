package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/bytesize"
	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const configFileName = "config.toml"

// Document kinds. One load is one document, and a save writes only that one:
// either the global config.toml or a single config.<game>.toml.
const (
	documentMain = "main"
	documentGame = "game"
)

const gameConfigPrefix = "config."

// documentGameID returns the game a path names as a per-game document.
func documentGameID(path string) (string, bool) {
	base := strings.ToLower(filepath.Base(path))
	if !strings.HasPrefix(base, gameConfigPrefix) || !strings.HasSuffix(base, ".toml") {
		return "", false
	}
	gameID := strings.TrimSuffix(strings.TrimPrefix(base, gameConfigPrefix), ".toml")
	if !slices.Contains(supportedGameIDs(), gameID) {
		return "", false
	}
	return gameID, true
}

func gameConfigFileName(gameID string) string {
	return gameConfigPrefix + gameID + ".toml"
}

type ConfigService struct{}

// OpenConfigFile tries the default configuration paths and returns the file's
// path and raw contents. When there is no default file it returns no file and
// the load screen stays in place: the native picker only opens when the user
// asks for it. Parsing and editing are deliberately not part of this first
// loading step.
func (s *ConfigService) OpenConfigFile() (*ConfigFile, error) {
	path, err := defaultConfigPath()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}
	return s.readConfigFile(path)
}

// SelectConfigFile always opens the native picker. It is used when the user
// explicitly wants a configuration. Closing the picker is a normal answer: the
// load screen stays as it is.
func (s *ConfigService) SelectConfigFile() (*ConfigFile, error) {
	path, err := application.Get().Dialog.OpenFile().
		SetTitle("Select configuration file").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AddFilter("TOML files", "*.toml").
		PromptForSingleSelection()
	if err != nil {
		return nil, fmt.Errorf("open configuration picker: %w", err)
	}
	if path == "" {
		return nil, nil
	}
	return s.readConfigFile(path)
}

// OpenConfigFileAtPath loads a configuration dropped onto the application window.
func (s *ConfigService) OpenConfigFileAtPath(path string) (*ConfigFile, error) {
	if path == "" {
		return nil, errors.New("dropped configuration path is empty")
	}
	return s.readConfigFile(path)
}

// OpenTemplate loads the official server template as a new, unsaved document.
func (s *ConfigService) OpenTemplate() (*ConfigFile, error) {
	path, err := templateConfigPath()
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration template %q: %w", path, err)
	}
	config, err := parseConfiguration(content)
	if err != nil {
		return nil, fmt.Errorf("parse configuration template %q: %w", path, err)
	}
	return &ConfigFile{
		Content:          string(content),
		Kind:             documentMain,
		SuggestedName:    configFileName,
		New:              true,
		Config:           config,
		ValidationErrors: validateMainDocument(config),
	}, nil
}

// OpenGameTemplate loads a new per-game document for the given game. It is
// unsaved, so its save asks for the destination, with config.<game>.toml as
// the proposed name.
func (s *ConfigService) OpenGameTemplate(gameID string) (*ConfigFile, error) {
	config, err := newGameDocumentConfiguration(gameID)
	if err != nil {
		return nil, err
	}
	return &ConfigFile{
		Kind:             documentGame,
		Game:             gameID,
		SuggestedName:    gameConfigFileName(gameID),
		New:              true,
		Config:           config,
		ValidationErrors: validateGameDocument(config, gameID),
	}, nil
}

// OpenGameDocument opens the config.<game>.toml next to path, or a new document
// already aimed at that path when the file does not exist yet. It is the only
// way one document leads to another: the editor never holds two at once.
func (s *ConfigService) OpenGameDocument(path string, gameID string) (*ConfigFile, error) {
	if path == "" {
		return nil, errors.New("configuration path is empty")
	}
	config, err := newGameDocumentConfiguration(gameID)
	if err != nil {
		return nil, err
	}
	target := gameFilePath(path, gameID)
	if _, statErr := os.Stat(target); statErr == nil {
		return s.readConfigFile(target)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("check game configuration %q: %w", target, statErr)
	}
	return &ConfigFile{
		Path:             target,
		Kind:             documentGame,
		Game:             gameID,
		SuggestedName:    gameConfigFileName(gameID),
		New:              true,
		Config:           config,
		ValidationErrors: validateGameDocument(config, gameID),
	}, nil
}

// newGameDocumentConfiguration is the model a new per-game document starts
// from: the server defaults with only its game enabled and empty tables.
func newGameDocumentConfiguration(gameID string) (*Configuration, error) {
	if !slices.Contains(supportedGameIDs(), gameID) {
		return nil, fmt.Errorf("unknown game %q", gameID)
	}
	config := serverDefaults()
	config.Games.Enabled = []string{gameID}
	setGameConfig(&config, gameID, GameConfig{})
	return &config, nil
}

// SelectConfigDestination opens the native dialog used for a new configuration.
func (s *ConfigService) SelectConfigDestination() (string, error) {
	path, err := application.Get().Dialog.SaveFile().
		SetMessage("Choose where to create the configuration").
		SetFilename(configFileName).
		AddFilter("TOML files", "*.toml").
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("open configuration destination picker: %w", err)
	}
	if path == "" {
		return "", errors.New("no configuration destination selected")
	}
	return path, nil
}

// SelectConfigSaveAsDestination opens a native dialog for an existing configuration's new path.
func (s *ConfigService) SelectConfigSaveAsDestination(fileName string) (string, error) {
	path, err := application.Get().Dialog.SaveFile().
		SetMessage("Choose where to save the configuration copy").
		SetFilename(filepath.Base(fileName)).
		AddFilter("TOML files", "*.toml").
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("open configuration save-as picker: %w", err)
	}
	if path == "" {
		return "", errors.New("no configuration destination selected")
	}
	return path, nil
}

// SelectCustomFilesFolder opens the native directory picker used for the
// custom files storage folder. Closing the picker is a normal answer: the empty
// path it returns leaves the setting as it is.
func (s *ConfigService) SelectCustomFilesFolder() (string, error) {
	return selectFolder("Select custom files folder")
}

// SelectModsUploadFolder opens the native directory picker used for the mods
// upload folder. Closing the picker is a normal answer: the empty path it
// returns leaves the setting as it is.
func (s *ConfigService) SelectModsUploadFolder() (string, error) {
	return selectFolder("Select mods upload folder")
}

// SelectModsDatabasePath opens the native save dialog used for the mods SQLite
// file. Closing the picker is a normal answer: the empty path it returns keeps
// the setting as it is.
func (s *ConfigService) SelectModsDatabasePath() (string, error) {
	path, err := application.Get().Dialog.SaveFile().
		SetMessage("Choose the mods database file").
		SetFilename("mods.db").
		AddFilter("SQLite database", "*.db").
		PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

// selectFolder opens a native directory picker. Closing it is a normal answer.
func selectFolder(title string) (string, error) {
	path, err := application.Get().Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(false).
		CanChooseDirectories(true).
		PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

// CreateConfigFile validates a new document and creates it at the requested
// path. The file name decides the kind: config.<game>.toml creates a per-game
// document, anything else the global configuration.
func (s *ConfigService) CreateConfigFile(path string, configuration Configuration, overwrite bool) (*ConfigFile, error) {
	if path == "" {
		return nil, errors.New("configuration destination cannot be empty")
	}
	if _, err := os.Stat(path); err == nil && !overwrite {
		return nil, fmt.Errorf("configuration file %q already exists; confirmation required", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("check configuration destination %q: %w", path, err)
	}
	if gameID, isGame := documentGameID(path); isGame {
		document, err := gameDocumentContent(path, gameID, nil, &configuration)
		if err != nil {
			return nil, fmt.Errorf("create configuration file %q: %w", path, err)
		}
		if err := writeDocument(path, document, overwrite); err != nil {
			return nil, err
		}
		return s.readConfigFile(path)
	}
	template, err := s.OpenTemplate()
	if err != nil {
		return nil, err
	}
	updated, err := prepareConfigurationDocument([]byte(template.Content), configuration)
	if err != nil {
		return nil, fmt.Errorf("create configuration file %q: %w", path, err)
	}
	if err := writeDocument(path, updated, overwrite); err != nil {
		return nil, err
	}
	return s.readConfigFile(path)
}

// SaveConfiguration validates and saves the loaded document. It writes exactly
// one file: the per-game document the path names, or the global configuration.
func (s *ConfigService) SaveConfiguration(path string, configuration Configuration) error {
	if gameID, isGame := documentGameID(path); isGame {
		existing, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read configuration file %q: %w", path, err)
		}
		document, err := gameDocumentContent(path, gameID, existing, &configuration)
		if err != nil {
			return fmt.Errorf("edit configuration file %q: %w", path, err)
		}
		return writeConfigFile(path, document)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read configuration file %q: %w", path, err)
	}
	if _, err := parseConfiguration(content); err != nil {
		return fmt.Errorf("parse configuration file %q: %w", path, err)
	}

	updated, err := prepareConfigurationDocument(content, configuration)
	if err != nil {
		return fmt.Errorf("edit configuration file %q: %w", path, err)
	}
	return writeConfigFile(path, updated)
}

// SaveConfigurationAs validates and saves an edited document to a new path.
// The destination's file name decides what is written, so a game document can
// be copied next to another configuration without touching its config.toml.
func (s *ConfigService) SaveConfigurationAs(sourcePath string, destination string, configuration Configuration, overwrite bool) (*ConfigFile, error) {
	if sourcePath == "" || destination == "" {
		return nil, errors.New("configuration source and destination are required")
	}
	if gameID, isGame := documentGameID(destination); isGame {
		// Only a per-game document is a base for another one: a config.toml is
		// not edited here.
		var base []byte
		if _, sourceIsGame := documentGameID(sourcePath); sourceIsGame {
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return nil, fmt.Errorf("read configuration file %q: %w", sourcePath, err)
			}
			base = content
		}
		document, err := gameDocumentContent(destination, gameID, base, &configuration)
		if err != nil {
			return nil, fmt.Errorf("edit configuration file %q: %w", sourcePath, err)
		}
		if err := writeDocument(destination, document, overwrite); err != nil {
			return nil, err
		}
		return s.readConfigFile(destination)
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %q: %w", sourcePath, err)
	}
	if _, err := parseConfiguration(content); err != nil {
		return nil, fmt.Errorf("parse configuration file %q: %w", sourcePath, err)
	}
	updated, err := prepareConfigurationDocument(content, configuration)
	if err != nil {
		return nil, fmt.Errorf("edit configuration file %q: %w", sourcePath, err)
	}

	if err := writeDocument(destination, updated, overwrite); err != nil {
		return nil, err
	}
	return s.readConfigFile(destination)
}

// gameDocumentContent validates the per-game part of the model and builds the
// document for one game. The embedded battle server is written as a delta
// against the sibling config.toml, which is only read.
func gameDocumentContent(path string, gameID string, existing []byte, configuration *Configuration) ([]byte, error) {
	if issues := errorValidationIssues(validateGameDocument(configuration, gameID)); len(issues) > 0 {
		return nil, fmt.Errorf("configuration is invalid: %s", issues[0].Message)
	}
	game := GameConfig{BattleServers: gameBattleServers(configuration, gameID)}
	if embedded := gameEmbeddedBattleServer(configuration, gameID); embedded != nil {
		game.EmbeddedBattleServer = *embedded
	}
	document, err := gameDocumentWithBase(gameID, existing, game, inheritedEmbedded(path, gameID))
	if err != nil {
		return nil, err
	}
	if document == nil {
		// A brand-new document with nothing but inherited values still needs a
		// [Game] table to be a document at all.
		document = []byte(gameDocumentHeader(gameID))
	}
	return document, nil
}

// writeDocument writes a whole document, replacing it only when the caller has
// already accepted the overwrite.
func writeDocument(path string, content []byte, overwrite bool) error {
	if _, err := os.Stat(path); err == nil {
		if !overwrite {
			return fmt.Errorf("configuration file %q already exists; confirmation required", path)
		}
		return writeConfigFile(path, content)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check configuration destination %q: %w", path, err)
	}
	return writeNewConfigFile(path, content)
}

// prepareConfigurationDocument validates and transforms a configuration document
// into its final form, centralising the logic shared by every save/preview path.
func prepareConfigurationDocument(content []byte, configuration Configuration) ([]byte, error) {
	if validationErrors := errorValidationIssues(validateMainDocument(&configuration)); len(validationErrors) > 0 {
		return nil, fmt.Errorf("configuration is invalid: %s", validationErrors[0].Message)
	}
	// A document with differing per-game host lists and no [Hosts] cannot be
	// saved: dropping the lists would lose the addresses, and merging them
	// would silently change the binding model.
	if err := validateHostMigration(content); err != nil {
		return nil, err
	}
	updated, err := configurationDocument(content, configuration)
	if err != nil {
		return nil, err
	}
	if _, err := parseConfiguration(updated); err != nil {
		return nil, fmt.Errorf("validate configuration document: %w", err)
	}
	return updated, nil
}

// inheritedEmbedded is the [Games.<game>.EmbeddedBattleServer] of the config.toml
// next to a per-game document, which that document inherits field by field.
// The main file is never written from a per-game document.
func inheritedEmbedded(configPath string, gameID string) EmbeddedBattleServerConfig {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(configPath), configFileName))
	if err != nil {
		return EmbeddedBattleServerConfig{}
	}
	config, err := parseConfiguration(content)
	if err != nil {
		return EmbeddedBattleServerConfig{}
	}
	if embedded := gameEmbeddedBattleServer(config, gameID); embedded != nil {
		return *embedded
	}
	return EmbeddedBattleServerConfig{}
}

// legacyBattleServers are the [Games.<game>.BattleServers] of the sibling
// config.toml, the fallback the server uses when the per-game file lists none.
func legacyBattleServers(configPath string, gameID string) []BattleServerConfig {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(configPath), configFileName))
	if err != nil {
		return nil
	}
	config, err := parseConfiguration(content)
	if err != nil {
		return nil
	}
	return gameBattleServers(config, gameID)
}

// documentHostsTable reports whether a document declares the global [Hosts]
// table. Its absence is what tells an old per-game document from a migrated one.
func documentHostsTable(content []byte) (bool, error) {
	tables, err := documentTables(content)
	if err != nil {
		return false, err
	}
	for _, table := range tables {
		if table.path == "Hosts" && !table.arrayTable {
			return true, nil
		}
	}
	return false, nil
}

// migrateGameHosts moves the deprecated Games.<game>.Hosts lists into the
// global [Hosts] model when the document has none yet, using the same
// certificate-class destination as the server. Differing lists are left alone:
// validateHostMigration refuses that save with an instruction instead.
func migrateGameHosts(content []byte, config *Configuration) error {
	hasHosts, err := documentHostsTable(content)
	if err != nil || hasHosts {
		return err
	}
	var migrated []string
	firstGame := ""
	for _, gameID := range supportedGameIDs() {
		if !slices.Contains(config.Games.Enabled, gameID) {
			continue
		}
		hosts, _ := gameHosts(config, gameID)
		if len(hosts) == 0 {
			continue
		}
		if firstGame == "" {
			firstGame = gameID
			migrated = slices.Clone(hosts)
			continue
		}
		if !slices.Equal(migrated, hosts) {
			return nil
		}
	}
	if firstGame == "" {
		return nil
	}
	allLegacy := true
	for _, gameID := range config.Games.Enabled {
		if common.GameCertMode(gameID) != common.CertModeLegacy {
			allLegacy = false
		}
	}
	// Moving to one class empties the other: the default Modern list would
	// otherwise keep listening on both.
	config.Hosts.Modern = []string{}
	config.Hosts.Legacy = []string{}
	if allLegacy {
		config.Hosts.Legacy = migrated
	} else {
		config.Hosts.Modern = migrated
	}
	return nil
}

// validateHostMigration refuses a document that still holds differing per-game
// host lists and no [Hosts] table.
func validateHostMigration(content []byte) error {
	hasHosts, err := documentHostsTable(content)
	if err != nil || hasHosts {
		return err
	}
	config, err := parseConfiguration(content)
	if err != nil {
		return err
	}
	firstGame := ""
	var firstList []string
	for _, gameID := range supportedGameIDs() {
		if !slices.Contains(config.Games.Enabled, gameID) {
			continue
		}
		hosts, _ := gameHosts(config, gameID)
		if len(hosts) == 0 {
			continue
		}
		if firstGame == "" {
			firstGame, firstList = gameID, hosts
			continue
		}
		if !slices.Equal(firstList, hosts) {
			return fmt.Errorf("Games.%s.Hosts %v and Games.%s.Hosts %v differ: hosts are global in [Hosts].Modern/Legacy now, so set one list there and remove the per-game ones before saving", firstGame, firstList, gameID, hosts)
		}
	}
	return nil
}

// gameDocument edits one per-game file without an inherited base: it is the
// form the tests and a new file use, where the server defaults are the base.
func gameDocument(gameID string, existing []byte, game GameConfig) ([]byte, error) {
	return gameDocumentWithBase(gameID, existing, game, EmbeddedBattleServerConfig{})
}

// gameDocumentWithBase edits one per-game file. It creates a small header when
// the file does not exist yet, and reports nil when there is nothing worth
// creating: an empty file would make the server load a document that says
// nothing. base is the [Games.<game>.EmbeddedBattleServer] of config.toml: only
// the keys that differ from it are written, so an inherited port is not pinned
// to a zero the document never chose.
func gameDocumentWithBase(gameID string, existing []byte, game GameConfig, base EmbeddedBattleServerConfig) ([]byte, error) {
	content := existing
	created := len(content) == 0
	if created {
		content = []byte(gameDocumentHeader(gameID))
	}
	var err error
	if content, err = ensureGameSection(content); err != nil {
		return nil, err
	}
	// [Game.Hosts] is the per-game key the server ignores: the GUI migrates it
	// to [Hosts] and does not keep it in the per-game document.
	updated, _, err := removeDocumentValue(content, "Game", "Hosts")
	if err != nil {
		return nil, fmt.Errorf("remove Game.Hosts: %w", err)
	}
	updated, err = rewriteBattleServersSection(updated, "Game", gameID, game.BattleServers)
	if err != nil {
		return nil, fmt.Errorf("edit Game.BattleServers: %w", err)
	}
	updated, err = rewriteEmbeddedBattleServerSection(updated, "Game", gameID, &game.EmbeddedBattleServer, base)
	if err != nil {
		return nil, fmt.Errorf("edit Game.EmbeddedBattleServer: %w", err)
	}
	if created {
		tables, err := documentTables(updated)
		if err != nil {
			return nil, err
		}
		hasGameTable := false
		for _, table := range tables {
			if strings.HasPrefix(table.path, "Game.") {
				hasGameTable = true
				break
			}
		}
		if !hasGameTable {
			return nil, nil
		}
	}
	return updated, nil
}

// configurationDocument applies editable values without serializing the whole model.
func configurationDocument(content []byte, configuration Configuration) ([]byte, error) {
	updated := content
	replacements := []struct {
		section string
		key     string
		value   string
	}{
		{"", "Log", strconv.FormatBool(configuration.Log)},
		{"", "GeneratePlatformUserId", strconv.FormatBool(configuration.GeneratePlatformUserID)},
		{"Sign", "Enabled", strconv.FormatBool(configuration.Sign.Enabled)},
		{"Sign", "PlatformUserIdExceptions", tomlArray(configuration.Sign.PlatformUserIdExceptions)},
		{"Internet", "Enabled", strconv.FormatBool(configuration.Internet.Enabled)},
		{"Internet", "IP", tomlString(configuration.Internet.IP)},
		{"Games", "Enabled", tomlArray(configuration.Games.Enabled)},
		{"Hosts", "Modern", tomlArray(configuration.Hosts.Modern)},
		{"Hosts", "Legacy", tomlArray(configuration.Hosts.Legacy)},
		{"Announcement", "Enabled", strconv.FormatBool(configuration.Announcement.Enabled)},
		{"Announcement", "Multicast", strconv.FormatBool(configuration.Announcement.Multicast)},
		{"Announcement", "Port", strconv.Itoa(configuration.Announcement.Port)},
		{"Announcement", "MulticastGroup", tomlString(configuration.Announcement.MulticastGroup)},
		{"CustomFiles", "Enabled", strconv.FormatBool(configuration.CustomFiles.Enabled)},
		{"CustomFiles", "Folder", tomlString(configuration.CustomFiles.Folder)},
		{"CustomFiles", "MaxFileSize", tomlString(configuration.CustomFiles.MaxFileSize)},
		{"CustomFiles", "MaxTotalSize", tomlString(configuration.CustomFiles.MaxTotalSize)},
		{"CustomFiles", "Expiry", tomlString(configuration.CustomFiles.Expiry)},
		{"Replays", "Enabled", strconv.FormatBool(configuration.Replays.Enabled)},
		{"Replays", "MaxFileSize", tomlString(configuration.Replays.MaxFileSize)},
		{"Replays", "MaxTotalSize", tomlString(configuration.Replays.MaxTotalSize)},
		{"Replays", "Expiry", tomlString(configuration.Replays.Expiry)},
		{"Mods", "Enabled", strconv.FormatBool(configuration.Mods.Enabled)},
		{"Mods", "DBPath", tomlString(configuration.Mods.DBPath)},
		{"Mods", "UploadDir", tomlString(configuration.Mods.UploadDir)},
		{"Mods", "MaxUploadSize", tomlString(configuration.Mods.MaxUploadSize)},
		{"Mods", "RequireAuthForRead", strconv.FormatBool(configuration.Mods.RequireAuthForRead)},
		{"Api", "Enabled", strconv.FormatBool(configuration.Api.Enabled)},
	}
	// The server reads Authentication at the root level. The shipped template
	// used to keep it under [Internet], so an existing copy there is kept in
	// sync for documents written before the contract was clarified, while the
	// root key is always written: that is the one the server actually uses.
	authValue := tomlString(configuration.Authentication)
	if updated2, err := replaceDocumentValue(updated, "Internet", "Authentication", authValue); err == nil {
		updated = updated2
	}
	var err error
	updated, err = ensureDocumentValue(updated, "", "Authentication", authValue)
	if err != nil {
		return nil, fmt.Errorf("edit Authentication: %w", err)
	}
	for _, replacement := range replacements {
		updated, err = ensureDocumentValue(updated, replacement.section, replacement.key, replacement.value)
		if err != nil {
			return nil, fmt.Errorf("edit %s.%s: %w", replacement.section, replacement.key, err)
		}
	}
	// Hosts moved to the global [Hosts] table. The old per-game keys are removed
	// on save instead of being written with a stale value the server ignores.
	for _, gameID := range supportedGameIDs() {
		updated, _, err = removeDocumentValue(updated, "Games."+gameID, "Hosts")
		if err != nil {
			return nil, fmt.Errorf("remove Games.%s.Hosts: %w", gameID, err)
		}
	}
	return updated, nil
}

// gameDocumentHeader introduces a per-game file the editor creates.
func gameDocumentHeader(gameID string) string {
	return "# Configuration for " + gameID + ".\n" +
		"# Only [Game.BattleServers] and [Game.EmbeddedBattleServer] belong here.\n" +
		"# Hosts are global in [Hosts] of config.toml.\n\n" +
		"[Game]\n"
}

// ensureGameSection makes sure a per-game document has the [Game] table its
// subtables are inserted after. A document with only comments has none.
func ensureGameSection(content []byte) ([]byte, error) {
	tables, err := documentTables(content)
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		if table.path == "Game" && !table.arrayTable {
			return content, nil
		}
	}
	return append([]byte("[Game]\n\n"), content...), nil
}

// gameFilePath is the per-game document next to the main configuration file.
func gameFilePath(configPath string, gameID string) string {
	return filepath.Join(filepath.Dir(configPath), "config."+gameID+".toml")
}

// tomlString formats a value as a TOML string, preferring the literal form the
// official template uses for paths and addresses.
func tomlString(value string) string {
	if !strings.Contains(value, "'") {
		return "'" + value + "'"
	}
	return strconv.Quote(value)
}

// tomlArray formats string values as a TOML array.
func tomlArray(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = tomlString(value)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// readConfigFile reads, parses and validates one document from disk. The file
// name decides what it is: config.<game>.toml is a per-game document, anything
// else is the global configuration.
func (s *ConfigService) readConfigFile(path string) (*ConfigFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %q: %w", path, err)
	}
	if gameID, isGame := documentGameID(path); isGame {
		return readGameDocument(path, content, gameID)
	}
	config, err := parseConfiguration(content)
	if err != nil {
		return nil, fmt.Errorf("parse configuration file %q: %w", path, err)
	}
	hasFields, err := containsConfigurationField(content)
	if err != nil {
		return nil, fmt.Errorf("inspect configuration file %q: %w", path, err)
	}
	if !hasFields {
		return nil, fmt.Errorf("configuration file %q does not contain supported configuration fields", path)
	}
	if err := readGameFiles(path, config); err != nil {
		return nil, err
	}
	// An old document without [Hosts] keeps its bindings in the per-game keys:
	// migrate them into the model before the editor shows (and saves) defaults
	// that would drop them.
	if err := migrateGameHosts(content, config); err != nil {
		return nil, fmt.Errorf("inspect configuration file %q: %w", path, err)
	}
	return &ConfigFile{
		Path:             path,
		Content:          string(content),
		Kind:             documentMain,
		SuggestedName:    configFileName,
		Config:           config,
		ValidationErrors: validateMainDocument(config),
	}, nil
}

// readGameDocument reads one config.<game>.toml. Its battle servers and embedded
// battle server start from the server defaults, then inherit field by field from
// the config.toml next to it and finally take the keys the file assigns.
func readGameDocument(path string, content []byte, gameID string) (*ConfigFile, error) {
	game, err := parseGameConfiguration(content)
	if err != nil {
		return nil, fmt.Errorf("parse configuration file %q: %w", path, err)
	}
	keys, err := gameEmbeddedKeys(content)
	if err != nil {
		return nil, fmt.Errorf("parse configuration file %q: %w", path, err)
	}
	embedded := mergeEmbeddedBattleServer(inheritedEmbedded(path, gameID), game.EmbeddedBattleServer, keys)
	servers := game.BattleServers
	if !hasGameBattleServers(content) {
		servers = legacyBattleServers(path, gameID)
	}
	config := serverDefaults()
	config.Games.Enabled = []string{gameID}
	setGameConfig(&config, gameID, GameConfig{BattleServers: servers, EmbeddedBattleServer: embedded})
	return &ConfigFile{
		Path:             path,
		Content:          string(content),
		Kind:             documentGame,
		Game:             gameID,
		SuggestedName:    gameConfigFileName(gameID),
		Config:           &config,
		ValidationErrors: validateGameDocument(&config, gameID),
	}, nil
}

// readGameFiles loads the per-game documents that exist next to the main one
// and merges their battle servers into the model, so the global document holds
// what the server would actually use. They are read-only here: each one is
// edited by opening it, which is a document of its own.
func readGameFiles(configPath string, config *Configuration) error {
	enabled := make(map[string]bool, len(config.Games.Enabled))
	for _, gameID := range config.Games.Enabled {
		enabled[gameID] = true
	}
	for _, gameID := range supportedGameIDs() {
		if !enabled[gameID] {
			continue
		}
		path := gameFilePath(configPath, gameID)
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read game configuration file %q: %w", path, err)
		}
		game, err := parseGameConfiguration(content)
		if err != nil {
			return fmt.Errorf("parse game configuration file %q: %w", path, err)
		}
		// The file overrides field by field, as the server does: a key it does
		// not carry keeps the value inherited from config.toml.
		keys, err := gameEmbeddedKeys(content)
		if err != nil {
			return fmt.Errorf("parse game configuration file %q: %w", path, err)
		}
		merged := EmbeddedBattleServerConfig{}
		if base := gameEmbeddedBattleServer(config, gameID); base != nil {
			merged = *base
		}
		merged = mergeEmbeddedBattleServer(merged, game.EmbeddedBattleServer, keys)
		servers := game.BattleServers
		if !hasGameBattleServers(content) {
			servers = gameBattleServers(config, gameID)
		}
		setGameConfig(config, gameID, GameConfig{BattleServers: servers, EmbeddedBattleServer: merged})
	}
	return nil
}

// gameEmbeddedKeys reports which keys of [Game.EmbeddedBattleServer] the
// per-game document actually assigns, so only those override config.toml.
func gameEmbeddedKeys(content []byte) (map[string]bool, error) {
	keys := make(map[string]bool, len(gameEmbeddedKeyNames))
	for _, key := range gameEmbeddedKeyNames {
		found, err := hasDocumentValue(content, "Game.EmbeddedBattleServer", key)
		if err != nil {
			return nil, err
		}
		keys[key] = found
	}
	return keys, nil
}

var gameEmbeddedKeyNames = []string{
	"StartIfMissing", "Region", "Name", "IPv4", "BsPort", "WebSocketPort", "OutOfBandPort", "LogLevel",
}

// mergeEmbeddedBattleServer overlays the keys the file assigns onto the values
// inherited from config.toml.
func mergeEmbeddedBattleServer(base, file EmbeddedBattleServerConfig, keys map[string]bool) EmbeddedBattleServerConfig {
	if keys["StartIfMissing"] {
		base.StartIfMissing = file.StartIfMissing
	}
	if keys["Region"] {
		base.Region = file.Region
	}
	if keys["Name"] {
		base.Name = file.Name
	}
	if keys["IPv4"] {
		base.IPv4 = file.IPv4
	}
	if keys["BsPort"] {
		base.BsPort = file.BsPort
	}
	if keys["WebSocketPort"] {
		base.WebSocketPort = file.WebSocketPort
	}
	if keys["OutOfBandPort"] {
		base.OutOfBandPort = file.OutOfBandPort
	}
	if keys["LogLevel"] {
		base.LogLevel = file.LogLevel
	}
	return base
}

// hasGameBattleServers reports whether a per-game document defines the battle
// server list, inline or as [[Game.BattleServers]] tables.
func hasGameBattleServers(content []byte) bool {
	if found, err := hasDocumentValue(content, "Game", "BattleServers"); err == nil && found {
		return true
	}
	blocks, err := tableBlocks(content, "Game.BattleServers", true)
	return err == nil && len(blocks) > 0
}

// parseGameConfiguration decodes the [Game] table of a per-game document.
func parseGameConfiguration(content []byte) (GameConfig, error) {
	var document struct {
		Game GameConfig `toml:"Game"`
	}
	if err := toml.Unmarshal(content, &document); err != nil {
		return GameConfig{}, fmt.Errorf("invalid TOML or configuration type: %w", err)
	}
	return document.Game, nil
}

// containsConfigurationField reports whether a TOML document contains a field
// understood by the configuration editor, regardless of that field's value.
func containsConfigurationField(content []byte) (bool, error) {
	parser := &unstable.Parser{KeepComments: true}
	parser.Reset(content)
	section := ""
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table:
			section = nodePath(expression)
		case unstable.KeyValue:
			if isConfigurationField(section, nodeKey(expression)) {
				return true, nil
			}
		}
	}
	if err := parser.Error(); err != nil {
		return false, fmt.Errorf("invalid TOML: %w", err)
	}
	return false, nil
}

// isConfigurationField identifies keys belonging to the public GUI contract.
func isConfigurationField(section string, key string) bool {
	if section == "" {
		switch key {
		case "Log", "GeneratePlatformUserId", "Authentication":
			return true
		}
	}
	if section == "Sign" {
		switch key {
		case "Enabled", "PlatformUserIdExceptions":
			return true
		}
	}
	if section == "Internet" {
		switch key {
		case "Enabled", "IP", "Authentication":
			return true
		}
	}
	if section == "Games" && key == "Enabled" {
		return true
	}
	if section == "Hosts" {
		switch key {
		case "Modern", "Legacy":
			return true
		}
	}
	if section == "Announcement" {
		switch key {
		case "Enabled", "Multicast", "Port", "MulticastGroup":
			return true
		}
	}
	if section == "CustomFiles" {
		switch key {
		case "Enabled", "Folder", "MaxFileSize", "MaxTotalSize", "Expiry":
			return true
		}
	}
	if section == "Replays" {
		switch key {
		case "Enabled", "MaxFileSize", "MaxTotalSize", "Expiry":
			return true
		}
	}
	if section == "Mods" {
		switch key {
		case "Enabled", "DBPath", "UploadDir", "MaxUploadSize", "RequireAuthForRead":
			return true
		}
	}
	if section == "Api" {
		return key == "Enabled"
	}
	if strings.HasPrefix(section, "Games.") {
		if key == "Hosts" {
			return true
		}
		if strings.HasSuffix(section, ".BattleServers") {
			switch key {
			case "Region", "Name", "IPv4", "BsPort", "WebSocketPort", "OutOfBandPort":
				return true
			}
		}
	}
	return false
}

// serverDefaults mirrors the defaults the server applies before reading a
// configuration file, so the editor shows the values the server would use for
// settings an older document does not have yet.
func serverDefaults() Configuration {
	return Configuration{
		Authentication: "disabled",
		Sign: SignConfig{
			Enabled:                  false,
			PlatformUserIdExceptions: []string{},
		},
		Internet: InternetConfiguration{
			Enabled: true,
			IP:      "auto",
		},
		Hosts: HostsConfiguration{
			Modern: []string{"0.0.0.0"},
			Legacy: []string{},
		},
		Games: GamesConfiguration{
			Enabled: []string{},
			// The embedded battle server is optional: the server only defaults
			// StartIfMissing (see initConfig in server/internal/cmd/root.go) and
			// fills the region and name with "Server" at runtime. Inventing a
			// region here would make the editor show a value the server never
			// uses, and write it on the next save.
			Age1:   GameConfig{EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Age2:   GameConfig{EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Age3:   GameConfig{EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Age4:   GameConfig{EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Athens: GameConfig{EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
		},
		Announcement: AnnouncementConfig{
			Enabled:        true,
			Multicast:      true,
			Port:           31978,
			MulticastGroup: "239.31.97.8",
		},
		CustomFiles: CustomFilesConfiguration{
			Enabled:      true,
			Folder:       "resources/custom-files",
			MaxFileSize:  "32MB",
			MaxTotalSize: "256MB",
			Expiry:       "12h",
		},
		Replays: ReplaysConfiguration{
			Enabled:      true,
			MaxFileSize:  "32MB",
			MaxTotalSize: "1GB",
			Expiry:       "-1s",
		},
		Mods: ModsConfiguration{
			Enabled:       true,
			DBPath:        "resources/mods/mods.db",
			UploadDir:     "resources/mods/uploads",
			MaxUploadSize: "256MB",
		},
		Api: ApiConfiguration{
			Enabled: true,
		},
	}
}

// parseConfiguration decodes TOML into the public configuration model.
func parseConfiguration(content []byte) (*Configuration, error) {
	config := serverDefaults()
	if err := toml.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("invalid TOML or configuration type: %w", err)
	}
	if config.Games.Enabled == nil {
		config.Games.Enabled = []string{}
	}
	if config.Hosts.Modern == nil {
		config.Hosts.Modern = []string{}
	}
	if config.Hosts.Legacy == nil {
		config.Hosts.Legacy = []string{}
	}
	return &config, nil
}

// replaceDocumentValue replaces one value in a named TOML table.
func replaceDocumentValue(content []byte, targetSection string, key string, replacement string) ([]byte, error) {
	parser := &unstable.Parser{KeepComments: true}
	parser.Reset(content)
	section := ""
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table, unstable.ArrayTable:
			section = nodePath(expression)
		case unstable.KeyValue:
			if section == targetSection && nodeKey(expression) == key {
				raw := expression.Raw
				rawContent := content[raw.Offset : raw.Offset+raw.Length]
				equals := bytes.IndexByte(rawContent, '=')
				if equals < 0 {
					return nil, fmt.Errorf("key %s.%s has no assignment", targetSection, key)
				}
				valueOffset := int(raw.Offset) + equals + 1
				for valueOffset < len(content) && (content[valueOffset] == ' ' || content[valueOffset] == '\t') {
					valueOffset++
				}
				valueEnd := int(raw.Offset + raw.Length)
				for valueEnd > valueOffset && (content[valueEnd-1] == ' ' || content[valueEnd-1] == '\t' || content[valueEnd-1] == '\r' || content[valueEnd-1] == '\n') {
					valueEnd--
				}
				replacementBytes := []byte(replacement)
				updated := make([]byte, 0, len(content)-(valueEnd-valueOffset)+len(replacementBytes))
				updated = append(updated, content[:valueOffset]...)
				updated = append(updated, replacementBytes...)
				updated = append(updated, content[valueEnd:]...)
				return updated, nil
			}
		}
	}
	if err := parser.Error(); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	notFoundSection := targetSection
	if notFoundSection == "" {
		notFoundSection = "root"
	}
	return nil, fmt.Errorf("%s.%s was not found", notFoundSection, key)
}

// documentTable is a table header of a TOML document and the byte offset where
// it starts.
type documentTable struct {
	path       string
	offset     int
	arrayTable bool
}

// documentTables lists the table headers of a TOML document in order.
func documentTables(content []byte) ([]documentTable, error) {
	parser := &unstable.Parser{KeepComments: true}
	parser.Reset(content)
	var tables []documentTable
	// A path can start several tables (`[[Games.age2.BattleServers]]` repeats
	// once per server), so each header is matched to its own occurrence.
	seen := make(map[string]int)
	for parser.NextExpression() {
		expression := parser.Expression()
		if expression.Kind != unstable.Table && expression.Kind != unstable.ArrayTable {
			continue
		}
		path := nodePath(expression)
		arrayTable := expression.Kind == unstable.ArrayTable
		key := fmt.Sprintf("%t:%s", arrayTable, path)
		occurrence := seen[key]
		seen[key]++
		// The unstable parser does not fill Raw for table headers, so the
		// offset is taken from the header line itself.
		offset, err := tableHeaderOffset(content, path, arrayTable, occurrence)
		if err != nil {
			return nil, err
		}
		tables = append(tables, documentTable{path: path, offset: offset, arrayTable: arrayTable})
	}
	if err := parser.Error(); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	return tables, nil
}

// tableHeaderOffset finds the byte offset of the `[path]` (or `[[path]]`) line
// that starts a table. Matching whole lines, and not the first occurrence of
// the header text, keeps headers quoted inside comments from being taken for
// the table itself.
func tableHeaderOffset(content []byte, path string, arrayTable bool, occurrence int) (int, error) {
	header := "[" + path + "]"
	if arrayTable {
		header = "[[" + path + "]]"
	}
	found := 0
	offset := 0
	for offset < len(content) {
		next := bytes.IndexByte(content[offset:], '\n')
		var line []byte
		if next < 0 {
			line = content[offset:]
			next = len(content)
		} else {
			line = content[offset : offset+next]
			next = offset + next + 1
		}
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, header) {
			// A table header may be followed by a comment on the same line.
			rest := strings.TrimSpace(trimmed[len(header):])
			if rest == "" || strings.HasPrefix(rest, "#") {
				if found == occurrence {
					return offset, nil
				}
				found++
			}
		}
		offset = next
	}
	return 0, fmt.Errorf("table %s occurrence %d was not found", header, occurrence)
}

// ensureDocumentValue replaces an editable value, adding the key (and the
// table that holds it) when an older document does not have it yet: documents
// written before a setting existed would otherwise fail to save.
func ensureDocumentValue(content []byte, targetSection string, key string, replacement string) ([]byte, error) {
	if updated, err := replaceDocumentValue(content, targetSection, key, replacement); err == nil {
		return updated, nil
	}
	tables, err := documentTables(content)
	if err != nil {
		return nil, err
	}
	line := key + " = " + replacement + "\n"
	if targetSection == "" {
		// Root keys have to stay above the first table.
		insertAt := len(content)
		if len(tables) > 0 {
			insertAt = tables[0].offset
		}
		return insertDocumentLine(content, insertAt, line), nil
	}
	sectionIndex := -1
	for index, table := range tables {
		if table.path == targetSection {
			sectionIndex = index
			break
		}
	}
	if sectionIndex == -1 {
		// The table is missing too. It goes next to its siblings ([Games.age3]
		// with the other games), before whatever closes that group, so the
		// document keeps reading in order; when the group is missing as well it
		// is appended at the end.
		block := "[" + targetSection + "]\n" + line
		if dot := strings.LastIndex(targetSection, "."); dot >= 0 {
			parent := targetSection[:dot+1]
			last := -1
			for index, table := range tables {
				if strings.HasPrefix(table.path, parent) {
					last = index
				}
			}
			if last >= 0 {
				end := len(content)
				if last+1 < len(tables) {
					end = tables[last+1].offset
				}
				insertAt := blockCutEnd(content, tableBlock{start: tables[last].offset, end: end})
				return insertDocumentLine(content, insertAt, block), nil
			}
		}
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content = append(content, '\n')
		}
		return append(content, []byte("\n"+block)...), nil
	}
	// Insert the key at the end of its table, before the next header.
	insertAt := len(content)
	if sectionIndex+1 < len(tables) {
		insertAt = tables[sectionIndex+1].offset
	}
	return insertDocumentLine(content, insertAt, line), nil
}

// insertDocumentLine inserts a complete TOML line at the given offset, keeping
// it on a line of its own and away from the line that follows it.
func insertDocumentLine(content []byte, offset int, line string) []byte {
	prefix := ""
	if offset > 0 && content[offset-1] != '\n' {
		prefix = "\n"
	}
	suffix := ""
	if offset < len(content) {
		// A blank line separates the new key from the table header that follows.
		suffix = "\n"
	}
	updated := make([]byte, 0, len(content)+len(prefix)+len(line)+len(suffix))
	updated = append(updated, content[:offset]...)
	updated = append(updated, prefix...)
	updated = append(updated, line...)
	updated = append(updated, suffix...)
	updated = append(updated, content[offset:]...)
	return updated
}

// nodeKey returns the first key component from an AST node.
func nodeKey(node *unstable.Node) string {
	keys := node.Key()
	if !keys.Next() {
		return ""
	}
	return string(keys.Node().Data)
}

// nodePath returns all key components from an AST table node.
func nodePath(node *unstable.Node) string {
	keys := node.Key()
	var parts []string
	for keys.Next() {
		parts = append(parts, string(keys.Node().Data))
	}
	return strings.Join(parts, ".")
}

// writeConfigFile writes a new document through a temporary file replacement.
func writeConfigFile(path string, content []byte) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat configuration file: %w", err)
	}
	temporaryPath, err := createTemporaryConfig(filepath.Dir(path), content, fileInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporaryPath) }()

	return replaceConfigFile(path, temporaryPath)
}

// writeNewConfigFile creates a file without replacing an existing destination.
func writeNewConfigFile(path string, content []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("create configuration file: destination %q already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check configuration destination: %w", err)
	}
	temporaryPath, err := createTemporaryConfig(directory, content, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("create configuration file: %w", err)
	}
	return nil
}

// createTemporaryConfig writes content to a temporary file with the given permissions.
func createTemporaryConfig(directory string, content []byte, permissions os.FileMode) (string, error) {
	temporary, err := os.CreateTemp(directory, ".config.toml-*")
	if err != nil {
		return "", fmt.Errorf("create temporary configuration file: %w", err)
	}
	temporaryPath := temporary.Name()

	if err := temporary.Chmod(permissions); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("set temporary configuration permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("write temporary configuration file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("close temporary configuration file: %w", err)
	}
	return temporaryPath, nil
}

// replaceConfigFile swaps the original with the temporary file and rolls back on failure.
func replaceConfigFile(path string, temporaryPath string) error {
	backup, err := os.CreateTemp(filepath.Dir(path), ".config.toml-backup-*")
	if err != nil {
		return fmt.Errorf("create configuration backup: %w", err)
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("close configuration backup: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("prepare configuration backup: %w", err)
	}
	defer func() { _ = os.Remove(backupPath) }()

	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("create configuration backup: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if restoreErr := os.Rename(backupPath, path); restoreErr != nil {
			return fmt.Errorf("replace configuration file: %w; restore failed: %v", err, restoreErr)
		}
		return fmt.Errorf("replace configuration file: %w", err)
	}
	return nil
}

// validateMainDocument validates config.toml on its own. The battle servers and
// the embedded battle server belong to the per-game documents, which this file
// does not edit, so their rules must not block saving it: those tables are only
// a read-only view of files that another document owns.
func validateMainDocument(config *Configuration) []ValidationError {
	trimmed := *config
	for _, gameID := range supportedGameIDs() {
		setGameConfig(&trimmed, gameID, GameConfig{})
	}
	return validateConfiguration(&trimmed)
}

// validateGameDocument validates one config.<game>.toml: only the rules about
// the game it holds, because the global settings live in another document.
func validateGameDocument(config *Configuration, gameID string) []ValidationError {
	trimmed := *config
	for _, otherID := range supportedGameIDs() {
		if otherID != gameID {
			setGameConfig(&trimmed, otherID, GameConfig{})
		}
	}
	trimmed.Games.Enabled = []string{gameID}
	var gameIssues []ValidationError
	for _, issue := range validateConfiguration(&trimmed) {
		if strings.HasPrefix(issue.Field, "Games.") {
			gameIssues = append(gameIssues, issue)
		}
	}
	return gameIssues
}

// validateConfiguration checks the semantic rules currently supported by the GUI.
func validateConfiguration(config *Configuration) []ValidationError {
	var validationErrors []ValidationError

	supportedAuthentication := map[string]bool{
		"required": true,
		"cached":   true,
		"adaptive": true,
		"disabled": true,
	}
	if !supportedAuthentication[config.Authentication] {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Authentication",
			Message: "Must be one of required, cached, adaptive or disabled",
		})
	}
	if config.GeneratePlatformUserID && config.Authentication != "disabled" {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "GeneratePlatformUserId",
			Message: "Requires Authentication to be disabled",
		})
	}
	// The server refuses to start when authentication is required without
	// Internet access, because it cannot reach the official servers.
	if config.Authentication == "required" && !config.Internet.Enabled {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Authentication",
			Message: "Requires Internet access",
		})
	}

	supportedGames := map[string]bool{
		"age1":   true,
		"age2":   true,
		"age3":   true,
		"age4":   true,
		"athens": true,
	}
	for index, game := range config.Games.Enabled {
		if !supportedGames[game] {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.Enabled.%d", index),
				Message: fmt.Sprintf("Unsupported game %q", game),
			})
		}
	}
	// Without a game the server has nothing to serve: it has to be told which
	// one to run, so the value is accepted but it warns about it.
	if len(config.Games.Enabled) == 0 {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "Games.Enabled",
			Message:  "No game enabled: the game will have to be passed as a parameter",
			Severity: severityWarning,
		})
	}
	validationErrors = append(validationErrors, validateHosts(config)...)
	validationErrors = append(validationErrors, validateGameBattleServers(config)...)

	if config.Announcement.Port < 1 || config.Announcement.Port > 65535 {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Announcement.Port",
			Message: "Must be an integer between 1 and 65535",
		})
	}
	// Ports under 1024 are only privileged on Unix-like systems: the value is
	// accepted everywhere, saving included, but it may need elevated rights.
	if config.Announcement.Enabled && config.Announcement.Port >= 1 && config.Announcement.Port < 1024 {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "Announcement.Port",
			Message:  "Privileged port: may require elevated rights on Linux and macOS",
			Severity: severityWarning,
		})
	}
	if config.Announcement.Enabled && config.Announcement.Multicast {
		address, err := netip.ParseAddr(config.Announcement.MulticastGroup)
		if err != nil || !address.Is4() || !address.IsMulticast() {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "Announcement.MulticastGroup",
				Message: "Must be an IPv4 multicast address",
			})
		}
	}

	// The address is only used to reach the Internet, so it is out of play
	// while Internet access is disabled.
	if config.Internet.Enabled {
		if address := strings.TrimSpace(config.Internet.IP); address != "auto" {
			if parsed, err := netip.ParseAddr(address); err != nil || !parsed.Is4() {
				validationErrors = append(validationErrors, ValidationError{
					Field:   "Internet.IP",
					Message: "Must be 'auto' or an IPv4 address",
				})
			}
		}
	}
	validationErrors = append(validationErrors, validateCustomFiles(config)...)
	validationErrors = append(validationErrors, validateReplays(config)...)
	validationErrors = append(validationErrors, validateMods(config)...)
	if !config.Api.Enabled && config.Mods.Enabled {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Api.Enabled",
			Message: "Mods cannot run while the Age of Empires API is disabled",
		})
	}
	validationErrors = append(validationErrors, validateEmbeddedBattleServers(config)...)

	for index := range validationErrors {
		if validationErrors[index].Severity == "" {
			validationErrors[index].Severity = severityError
		}
	}

	return validationErrors
}

// errorValidationIssues drops the warnings: only errors block saving.
func errorValidationIssues(issues []ValidationError) []ValidationError {
	var blocking []ValidationError
	for _, issue := range issues {
		if issue.Severity != severityWarning {
			blocking = append(blocking, issue)
		}
	}
	return blocking
}

// ValidateConfiguration exposes the authoritative configuration validation to the
// frontend so that all validation rules live in Go rather than TypeScript.
func (s *ConfigService) ValidateConfiguration(configuration Configuration) []ValidationError {
	return validateConfiguration(&configuration)
}

// validateCustomFiles checks the storage limits only while the endpoints are
// enabled: disabled, the server never uses those values.
func validateCustomFiles(config *Configuration) []ValidationError {
	if !config.CustomFiles.Enabled {
		return nil
	}
	var validationErrors []ValidationError
	if strings.TrimSpace(config.CustomFiles.Folder) == "" {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.Folder",
			Message: "Must not be empty",
		})
	}

	maxFileSize, fileSizeErr := bytesize.Parse(config.CustomFiles.MaxFileSize)
	maxTotalSize, totalSizeErr := bytesize.Parse(config.CustomFiles.MaxTotalSize)
	fileSizeValid := true
	totalSizeValid := true
	if fileSizeErr != nil {
		fileSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.MaxFileSize",
			Message: "Must be a size such as 32MB",
		})
	} else if maxFileSize <= 0 {
		fileSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.MaxFileSize",
			Message: "Must be greater than zero",
		})
	}
	if totalSizeErr != nil {
		totalSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.MaxTotalSize",
			Message: "Must be a size such as 256MB",
		})
	} else if maxTotalSize <= 0 {
		totalSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.MaxTotalSize",
			Message: "Must be greater than zero",
		})
	}
	if fileSizeValid && totalSizeValid && maxFileSize > maxTotalSize {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.MaxFileSize",
			Message: "Must not exceed the total size limit",
		})
	}

	if expiry, err := time.ParseDuration(strings.TrimSpace(config.CustomFiles.Expiry)); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.Expiry",
			Message: "Must be a duration such as 12h or 30m",
		})
	} else if expiry <= 0 {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "CustomFiles.Expiry",
			Message: "Must be greater than zero",
		})
	}
	return validationErrors
}

// validateReplays checks the storage limits only while the feature is enabled:
// when disabled the server never uses those values.
func validateReplays(config *Configuration) []ValidationError {
	if !config.Replays.Enabled {
		return nil
	}
	var validationErrors []ValidationError
	maxFileSize, fileSizeErr := bytesize.Parse(config.Replays.MaxFileSize)
	maxTotalSize, totalSizeErr := bytesize.Parse(config.Replays.MaxTotalSize)
	fileSizeValid := true
	totalSizeValid := true
	if fileSizeErr != nil {
		fileSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Replays.MaxFileSize",
			Message: "Must be a size such as 32MB",
		})
	} else if maxFileSize <= 0 {
		fileSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Replays.MaxFileSize",
			Message: "Must be greater than zero",
		})
	}
	if totalSizeErr != nil {
		totalSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Replays.MaxTotalSize",
			Message: "Must be a size such as 256MB",
		})
	} else if maxTotalSize <= 0 {
		totalSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Replays.MaxTotalSize",
			Message: "Must be greater than zero",
		})
	}
	if fileSizeValid && totalSizeValid && maxFileSize > maxTotalSize {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Replays.MaxFileSize",
			Message: "Must not exceed the total size limit",
		})
	}

	if _, err := time.ParseDuration(strings.TrimSpace(config.Replays.Expiry)); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Replays.Expiry",
			Message: "Must be a duration such as 12h or 30m, or 0/negative to keep forever",
		})
	}
	// Expiry <= 0 means keep forever (no TTL-based expiration)
	return validationErrors
}

// validateMods checks the mods API settings only while it is enabled: when
// disabled the server ignores these values and does not even parse the size.
func validateMods(config *Configuration) []ValidationError {
	if !config.Mods.Enabled {
		return nil
	}
	size, err := bytesize.Parse(config.Mods.MaxUploadSize)
	if err != nil {
		return []ValidationError{{
			Field:   "Mods.MaxUploadSize",
			Message: "Must be a size such as 256MB",
		}}
	}
	if size <= 0 {
		return []ValidationError{{
			Field:   "Mods.MaxUploadSize",
			Message: "Must be greater than zero",
		}}
	}
	return nil
}

// validateEmbeddedBattleServers checks the embedded battle server configuration
// for each game. Every key is optional: the server fills Region and Name with
// "Server", reads an empty IPv4 as "auto" and an empty StartIfMissing as "true"
// (server/internal/cmd/root.go), so only values it would reject are reported.
func validateEmbeddedBattleServers(config *Configuration) []ValidationError {
	var validationErrors []ValidationError
	validStartIfMissing := map[string]bool{"": true, "true": true, "false": true, "required": true}
	for _, gameID := range supportedGameIDs() {
		ebs := gameEmbeddedBattleServer(config, gameID)
		if ebs == nil {
			continue
		}
		if !validStartIfMissing[strings.ToLower(strings.TrimSpace(ebs.StartIfMissing))] {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.%s.EmbeddedBattleServer.StartIfMissing", gameID),
				Message: "Must be one of true, false or required",
			})
		}
		// An empty region is not a mistake: the server announces the embedded
		// battle server as "Server" when the document leaves it out.
		if address := strings.TrimSpace(ebs.IPv4); address != "" && address != "auto" {
			if parsed, err := netip.ParseAddr(address); err != nil || !parsed.Is4() {
				validationErrors = append(validationErrors, ValidationError{
					Field:   fmt.Sprintf("Games.%s.EmbeddedBattleServer.IPv4", gameID),
					Message: "Must be 'auto' or an IPv4 address",
				})
			}
		}
		if ebs.BsPort < 0 || ebs.BsPort > 65535 {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.%s.EmbeddedBattleServer.BsPort", gameID),
				Message: "Must be between 0 and 65535",
			})
		}
		if ebs.WebSocketPort < 0 || ebs.WebSocketPort > 65535 {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.%s.EmbeddedBattleServer.WebSocketPort", gameID),
				Message: "Must be between 0 and 65535",
			})
		}
		if ebs.OutOfBandPort < 0 || ebs.OutOfBandPort > 65535 {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.%s.EmbeddedBattleServer.OutOfBandPort", gameID),
				Message: "Must be between 0 and 65535",
			})
		}
		validLevels := map[string]bool{
			"none": true, "error": true, "warning": true, "notice": true, "info": true, "debug": true,
		}
		if ebs.LogLevel != "" && !validLevels[strings.ToLower(ebs.LogLevel)] {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.%s.EmbeddedBattleServer.LogLevel", gameID),
				Message: "Must be one of none, error, warning, notice, info, debug",
			})
		}
	}
	return validationErrors
}

// hostOwner is one configured host and the list it belongs to.
type hostOwner struct {
	list  string
	index int
}

// placeholderListeners mirrors the server's listener placeholder: 0.0.0.0 in
// either list means "follow the certificate class of the enabled games", so the
// two lists sharing it is not a bind conflict.
func placeholderListeners(hosts HostsConfiguration) bool {
	lists := 0
	for _, list := range [][]string{hosts.Modern, hosts.Legacy} {
		if len(list) == 0 {
			continue
		}
		lists++
		if len(list) != 1 || strings.TrimSpace(list[0]) != "0.0.0.0" {
			return false
		}
	}
	return lists > 0
}

// validateHosts checks the global listener lists. A host must resolve to IPv4,
// Modern and Legacy cannot share an address (the process would bind it twice),
// and 0.0.0.0 cannot be combined with another address in the same list or with
// a non-empty list of the other mode. Games are not compared with each other:
// one listener serves all of them.
func validateHosts(config *Configuration) []ValidationError {
	var validationErrors []ValidationError
	placeholder := placeholderListeners(config.Hosts)
	lists := []struct {
		name  string
		hosts []string
	}{
		{"Modern", config.Hosts.Modern},
		{"Legacy", config.Hosts.Legacy},
	}
	if len(config.Hosts.Modern) == 0 && len(config.Hosts.Legacy) == 0 {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "Hosts",
			Message: "At least one listener is required: set Hosts.Modern or Hosts.Legacy",
		})
	}

	owners := make(map[string][]hostOwner)
	wildcard := make(map[string]int)
	for _, list := range lists {
		for index, host := range list.hosts {
			field := fmt.Sprintf("Hosts.%s.%d", list.name, index)
			addresses := resolveIPv4(host)
			if len(addresses) == 0 {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field,
					Message: "Must resolve to an IPv4 address",
				})
				continue
			}
			for _, address := range addresses {
				owners[address] = append(owners[address], hostOwner{list: list.name, index: index})
				if address == "0.0.0.0" {
					wildcard[list.name] = index
				}
			}
		}
		if index, isWildcard := wildcard[list.name]; isWildcard && len(list.hosts) > 1 {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Hosts.%s.%d", list.name, index),
				Message: "0.0.0.0 cannot be combined with another address in the same list",
			})
		}
	}

	addresses := make([]string, 0, len(owners))
	for address := range owners {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	for _, address := range addresses {
		if placeholder {
			break
		}
		var modern, legacy []hostOwner
		for _, owner := range owners[address] {
			if owner.list == "Modern" {
				modern = append(modern, owner)
			} else {
				legacy = append(legacy, owner)
			}
		}
		if len(modern) == 0 || len(legacy) == 0 {
			continue
		}
		for _, owner := range append(modern, legacy...) {
			other := "Hosts.Legacy"
			if owner.list == "Legacy" {
				other = "Hosts.Modern"
			}
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Hosts.%s.%d", owner.list, owner.index),
				Message: fmt.Sprintf("Already used by %s", other),
			})
		}
	}

	if index, ok := wildcard["Modern"]; ok && len(config.Hosts.Legacy) > 0 && !placeholder {
		validationErrors = append(validationErrors, ValidationError{
			Field:   fmt.Sprintf("Hosts.Modern.%d", index),
			Message: "0.0.0.0 leaves no address for Hosts.Legacy to bind",
		})
	}
	if index, ok := wildcard["Legacy"]; ok && len(config.Hosts.Modern) > 0 && !placeholder {
		validationErrors = append(validationErrors, ValidationError{
			Field:   fmt.Sprintf("Hosts.Legacy.%d", index),
			Message: "0.0.0.0 leaves no address for Hosts.Modern to bind",
		})
	}

	// A game whose certificate class has no listener may still connect with the
	// other class (that is what the compatibility spike decides), so this only
	// warns. Listing the games makes it actionable.
	byClass := map[common.CertMode][]string{}
	for _, gameID := range config.Games.Enabled {
		if !slices.Contains(supportedGameIDs(), gameID) {
			continue
		}
		class := common.GameCertMode(gameID)
		byClass[class] = append(byClass[class], gameID)
	}
	// The placeholder resolves to the class the games need, so a list it does not
	// name is not a missing listener.
	if games := byClass[common.CertModeLegacy]; len(games) > 0 && len(config.Hosts.Legacy) == 0 && !placeholder {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "Hosts.Legacy",
			Message:  fmt.Sprintf("No self-signed listener: %s may reject the regular certificate", strings.Join(games, ", ")),
			Severity: severityWarning,
		})
	}
	if games := byClass[common.CertModeModern]; len(games) > 0 && len(config.Hosts.Modern) == 0 && !placeholder {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "Hosts.Modern",
			Message:  fmt.Sprintf("No regular listener: %s may reject the self-signed certificate", strings.Join(games, ", ")),
			Severity: severityWarning,
		})
	}
	// 0.0.0.0 is a placeholder that follows the games, but it can only follow
	// one class per address: the server refuses to start when the enabled games
	// need both, so say it here instead of at boot.
	if placeholder && len(byClass[common.CertModeLegacy]) > 0 && len(byClass[common.CertModeModern]) > 0 {
		validationErrors = append(validationErrors, ValidationError{
			Field: "Hosts.Legacy",
			Message: fmt.Sprintf("0.0.0.0 follows the certificate class of the games, but %s need the self-signed one and %s the regular one: set one list to a concrete address or enable games of a single class",
				strings.Join(byClass[common.CertModeLegacy], ", "), strings.Join(byClass[common.CertModeModern], ", ")),
			Severity: severityWarning,
		})
	}

	// The old per-game keys are read only to migrate a document. Saving drops
	// them, so a warning here explains why they disappear.
	for _, gameID := range supportedGameIDs() {
		hosts, _ := gameHosts(config, gameID)
		if len(hosts) == 0 {
			continue
		}
		validationErrors = append(validationErrors, ValidationError{
			Field:    fmt.Sprintf("Games.%s.Hosts", gameID),
			Message:  "Deprecated: hosts are global in [Hosts].Modern/Legacy and this key is removed on save",
			Severity: severityWarning,
		})
	}
	return validationErrors
}

// embeddedBattleServerStarts reports whether the server would start the
// embedded battle server for a game that has no battle server of its own. It is
// the evaluation server/internal/cmd/root.go does, minus the servers the
// battle-server-manager may discover at runtime, which a configuration editor
// cannot know: an unknown value is read as "true" there, after a warning.
func embeddedBattleServerStarts(gameID string, ebs *EmbeddedBattleServerConfig) bool {
	switch strings.ToLower(strings.TrimSpace(ebs.StartIfMissing)) {
	case "false":
		return false
	case "required":
		return gameID == "age4" || gameID == "athens"
	default:
		return true
	}
}

// validateGameBattleServers checks the battle servers of every enabled game.
// AoE IV and AoM: RT cannot run without a battle server, but the embedded one
// counts: it is started before the game is initialized and registered with the
// others (models.InitializeBattleServers), so a game without [[...BattleServers]]
// is only an error when the embedded one will not start either.
func validateGameBattleServers(config *Configuration) []ValidationError {
	var validationErrors []ValidationError
	enabled := make(map[string]bool, len(config.Games.Enabled))
	for _, gameID := range config.Games.Enabled {
		enabled[gameID] = true
	}
	for _, gameID := range supportedGameIDs() {
		if !enabled[gameID] {
			continue
		}
		servers := gameBattleServers(config, gameID)
		if len(servers) == 0 {
			if gameID == "age4" || gameID == "athens" {
				ebs := gameEmbeddedBattleServer(config, gameID)
				if ebs == nil || !embeddedBattleServerStarts(gameID, ebs) {
					// The battle-server-manager may still find one on this PC
					// at startup, which a configuration editor cannot see, so
					// this is an advisory and not a blocking error.
					validationErrors = append(validationErrors, ValidationError{
						Field:    fmt.Sprintf("Games.%s.BattleServers", gameID),
						Message:  "Needs a battle server: neither this document nor the embedded one provides one, so the battle-server-manager has to find it",
						Severity: severityWarning,
					})
				}
			}
			continue
		}
		regions := make(map[string]int, len(servers))
		for index, server := range servers {
			field := fmt.Sprintf("Games.%s.BattleServers.%d", gameID, index)
			region := strings.TrimSpace(server.Region)
			if region == "" {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field + ".Region",
					Message: "Must not be empty",
				})
			} else if previous, found := regions[region]; found {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field + ".Region",
					Message: fmt.Sprintf("Already used by battle server %d", previous+1),
				})
			} else {
				regions[region] = index
			}
			address := strings.TrimSpace(server.IPv4)
			if address == "" {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field + ".IPv4",
					Message: "Must not be empty",
				})
			} else if address != "auto" {
				if parsed, err := netip.ParseAddr(address); err != nil || !parsed.Is4() {
					validationErrors = append(validationErrors, ValidationError{
						Field:   field + ".IPv4",
						Message: "Must be 'auto' or an IPv4 address",
					})
				}
			}
			if server.BsPort < 1 || server.BsPort > 65535 {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field + ".BsPort",
					Message: "Must be an integer between 1 and 65535",
				})
			}
			if server.WebSocketPort < 1 || server.WebSocketPort > 65535 {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field + ".WebSocketPort",
					Message: "Must be an integer between 1 and 65535",
				})
			}
			if server.OutOfBandPort < 0 || server.OutOfBandPort > 65535 {
				validationErrors = append(validationErrors, ValidationError{
					Field:   field + ".OutOfBandPort",
					Message: "Must be an integer between 0 and 65535",
				})
			}
		}
	}
	return validationErrors
}

// supportedGameIDs returns the game identifiers the configuration knows about,
// in the order they are shown.
func supportedGameIDs() []string {
	return []string{"age1", "age2", "age3", "age4", "athens"}
}

// gameHosts returns the host list belonging to a supported game identifier.
func gameHosts(config *Configuration, gameID string) ([]string, bool) {
	switch gameID {
	case "age1":
		return config.Games.Age1.Hosts, true
	case "age2":
		return config.Games.Age2.Hosts, true
	case "age3":
		return config.Games.Age3.Hosts, true
	case "age4":
		return config.Games.Age4.Hosts, true
	case "athens":
		return config.Games.Athens.Hosts, true
	default:
		return nil, false
	}
}

// setGameConfig replaces the battle-server half of one game. Hosts are global
// now, so the old per-game list is deliberately left untouched: it is only read
// to migrate documents and never written back.
func setGameConfig(config *Configuration, gameID string, game GameConfig) {
	switch gameID {
	case "age1":
		config.Games.Age1.BattleServers = game.BattleServers
		config.Games.Age1.EmbeddedBattleServer = game.EmbeddedBattleServer
	case "age2":
		config.Games.Age2.BattleServers = game.BattleServers
		config.Games.Age2.EmbeddedBattleServer = game.EmbeddedBattleServer
	case "age3":
		config.Games.Age3.BattleServers = game.BattleServers
		config.Games.Age3.EmbeddedBattleServer = game.EmbeddedBattleServer
	case "age4":
		config.Games.Age4.BattleServers = game.BattleServers
		config.Games.Age4.EmbeddedBattleServer = game.EmbeddedBattleServer
	case "athens":
		config.Games.Athens.BattleServers = game.BattleServers
		config.Games.Athens.EmbeddedBattleServer = game.EmbeddedBattleServer
	}
}

// gameBattleServers returns the battle server list belonging to a supported game identifier.
func gameBattleServers(config *Configuration, gameID string) []BattleServerConfig {
	switch gameID {
	case "age1":
		return config.Games.Age1.BattleServers
	case "age2":
		return config.Games.Age2.BattleServers
	case "age3":
		return config.Games.Age3.BattleServers
	case "age4":
		return config.Games.Age4.BattleServers
	case "athens":
		return config.Games.Athens.BattleServers
	default:
		return nil
	}
}

// gameEmbeddedBattleServer returns the embedded battle server configuration for a supported game.
func gameEmbeddedBattleServer(config *Configuration, gameID string) *EmbeddedBattleServerConfig {
	switch gameID {
	case "age1":
		return &config.Games.Age1.EmbeddedBattleServer
	case "age2":
		return &config.Games.Age2.EmbeddedBattleServer
	case "age3":
		return &config.Games.Age3.EmbeddedBattleServer
	case "age4":
		return &config.Games.Age4.EmbeddedBattleServer
	case "athens":
		return &config.Games.Athens.EmbeddedBattleServer
	default:
		return nil
	}
}

// embeddedLogLevelDefault is the level the battle server logs at when the
// configuration leaves LogLevel out: it only overrides its own logger when the
// value is not empty, and that logger starts at debug.
const embeddedLogLevelDefault = "debug"

// EmbeddedBattleServerDefaults is what the server uses for a game when the
// document leaves the embedded battle server out. It is not what
// serverDefaults decodes: the server fills Region and Name with "Server" at
// startup (server/internal/cmd/root.go) and resolves the rest at runtime, so
// these are the values the editor shows as placeholders and hints.
type EmbeddedBattleServerDefaults struct {
	StartIfMissing string `json:"StartIfMissing"`
	Region         string `json:"Region"`
	Name           string `json:"Name"`
	IPv4           string `json:"IPv4"`
	BsPort         int    `json:"BsPort"`
	WebSocketPort  int    `json:"WebSocketPort"`
	OutOfBandPort  int    `json:"OutOfBandPort"`
	LogLevel       string `json:"LogLevel"`
}

// EmbeddedBattleServerDefaults exposes those values for every supported game,
// so the editor never has to carry a second copy of them.
func (s *ConfigService) EmbeddedBattleServerDefaults() map[string]EmbeddedBattleServerDefaults {
	defaults := make(map[string]EmbeddedBattleServerDefaults, len(supportedGameIDs()))
	for _, gameID := range supportedGameIDs() {
		defaults[gameID] = EmbeddedBattleServerDefaults{
			StartIfMissing: "true",
			Region:         "Server",
			Name:           "Server",
			IPv4:           "auto",
			// 0 in the document: the battle server picks a free port.
			BsPort:        0,
			WebSocketPort: 0,
			OutOfBandPort: 0,
			LogLevel:      embeddedLogLevelDefault,
		}
	}
	return defaults
}

// normalizeEmbedded resolves the empty spellings the server fills on its own:
// an empty StartIfMissing is "true" and an empty IPv4 is "auto" (see the
// evaluation in server/internal/cmd/root.go).
func normalizeEmbedded(ebs EmbeddedBattleServerConfig) EmbeddedBattleServerConfig {
	if strings.TrimSpace(ebs.StartIfMissing) == "" {
		ebs.StartIfMissing = "true"
	}
	if strings.TrimSpace(ebs.IPv4) == "" {
		ebs.IPv4 = "auto"
	}
	return ebs
}

// embeddedEntriesWrite reports whether the delta carries a value: an empty one
// means the file inherits everything from config.toml and has nothing to say.
func embeddedEntriesWrite(entries []tableEntry) bool {
	for _, entry := range entries {
		if entry.Value != "" {
			return true
		}
	}
	return false
}

// rewriteEmbeddedBattleServerSection rewrites the embedded battle server table
// of a per-game file. Only the keys that differ from base (the values inherited
// from config.toml) are written; a key that matches the inherited value is
// removed, so a port inherited as 30003 is not pinned to a zero the document
// never chose. With nothing left to say the definition is removed.
//
// An existing table is edited key by key, so the comments it holds (and any key
// this editor does not know about) stay where the user wrote them.
func rewriteEmbeddedBattleServerSection(content []byte, section, gameID string, ebs *EmbeddedBattleServerConfig, base EmbeddedBattleServerConfig) ([]byte, error) {
	path := section + ".EmbeddedBattleServer"

	updated, _, err := removeDocumentValue(content, section, "EmbeddedBattleServer")
	if err != nil {
		return nil, err
	}
	// An older document may hold the setting as an array of tables; it is not
	// the documented shape, so every one of them goes.
	arrayBlocks, err := tableBlocks(updated, path, true)
	if err != nil {
		return nil, err
	}
	updated = removeTableBlocks(updated, arrayBlocks)

	blocks, err := tableBlocks(updated, path, false)
	if err != nil {
		return nil, err
	}
	entries := embeddedBattleServerEntries(gameID, ebs, base)
	if ebs == nil || !embeddedEntriesWrite(entries) {
		return removeTableBlocks(updated, blocks), nil
	}
	if len(blocks) == 0 {
		return insertInlineTable(updated, section, embeddedBattleServerDocumentValueSection(section, ebs, base, gameID))
	}
	rewritten, err := writeTableKeys(updated[blocks[0].start:blocks[0].end], path, entries)
	if err != nil {
		return nil, err
	}
	updated = splice(updated, blocks[0].start, blocks[0].end, rewritten)
	if len(blocks) > 1 {
		// Only one table describes the setting: the duplicates go, with the
		// offsets they have after the first one was rewritten.
		remaining, err := tableBlocks(updated, path, false)
		if err != nil {
			return nil, err
		}
		if len(remaining) > 1 {
			updated = removeTableBlocks(updated, remaining[1:])
		}
	}
	return updated, nil
}

// rewriteBattleServers rewrites the battle servers of one game in the
// documented shape, an array of tables ([[Games.<game>.BattleServers]]). Older
// documents holding the inline `BattleServers = [...]` key are migrated.
// With no servers, every definition of that game is removed.
//
// The blocks that already describe a server are edited key by key instead of
// being recreated, so their comments and any extra key stay in place; only the
// blocks the document has too many of are dropped.
func rewriteBattleServers(content []byte, gameID string, servers []BattleServerConfig) ([]byte, error) {
	return rewriteBattleServersSection(content, "Games."+gameID, gameID, servers)
}

// rewriteBattleServersSection is rewriteBattleServers for any section: the main
// document uses Games.<id>, a per-game file uses Game.
func rewriteBattleServersSection(content []byte, section, gameID string, servers []BattleServerConfig) ([]byte, error) {
	path := section + ".BattleServers"

	updated, _, err := removeDocumentValue(content, section, "BattleServers")
	if err != nil {
		return nil, err
	}
	blocks, err := tableBlocks(updated, path, true)
	if err != nil {
		return nil, err
	}
	// Servers the document has no block for are gone: drop the extra blocks
	// from the bottom, so the offsets above them stay valid.
	if len(blocks) > len(servers) {
		updated = removeTableBlocks(updated, blocks[len(servers):])
		blocks = blocks[:len(servers)]
	}
	// Rewrite the surviving blocks from the bottom up, for the same reason.
	for index, table := range slices.Backward(blocks) {
		block := updated[table.start:table.end]
		rewritten, err := writeTableKeys(block, path, battleServerEntries(gameID, servers[index]))
		if err != nil {
			return nil, err
		}
		updated = splice(updated, table.start, table.end, rewritten)
	}
	if len(servers) == 0 {
		return updated, nil
	}
	extra := servers[len(blocks):]
	if len(extra) == 0 {
		return updated, nil
	}
	// The servers the document did not describe yet go after the last block of
	// the game, before the comments that close it.
	block := battleServersDocumentValueSection(section, gameID, extra)
	if len(blocks) == 0 {
		return insertArrayTables(updated, section, block)
	}
	last := blocks[len(blocks)-1]
	at := last.start + lastKeyValueEnd(updated[last.start:last.end], path)
	return splice(updated, at, at, []byte("\n"+block)), nil
}

// removeDocumentValue removes a key assigned inside a named table, together
// with the rest of its line. It reports whether the key was there.
func removeDocumentValue(content []byte, targetSection string, key string) ([]byte, bool, error) {
	parser := &unstable.Parser{KeepComments: true}
	parser.Reset(content)
	section := ""
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table, unstable.ArrayTable:
			section = nodePath(expression)
		case unstable.KeyValue:
			if section != targetSection || nodeKey(expression) != key {
				continue
			}
			start := int(expression.Raw.Offset)
			end := start + int(expression.Raw.Length)
			for end < len(content) && content[end] != '\n' {
				end++
			}
			if end < len(content) {
				end++
			}
			updated := make([]byte, 0, len(content)-(end-start))
			updated = append(updated, content[:start]...)
			updated = append(updated, content[end:]...)
			return updated, true, nil
		}
	}
	if err := parser.Error(); err != nil {
		return nil, false, fmt.Errorf("invalid TOML: %w", err)
	}
	return content, false, nil
}

// tableBlock is a table header and the byte range it owns: from the header to
// where the next table starts (or the end of the document).
type tableBlock struct {
	start int
	end   int
}

// tableBlocks lists the blocks of one path in document order.
func tableBlocks(content []byte, path string, arrayTable bool) ([]tableBlock, error) {
	tables, err := documentTables(content)
	if err != nil {
		return nil, err
	}
	var blocks []tableBlock
	for index, table := range tables {
		if table.path != path || table.arrayTable != arrayTable {
			continue
		}
		end := len(content)
		if index+1 < len(tables) {
			end = tables[index+1].offset
		}
		blocks = append(blocks, tableBlock{start: table.offset, end: end})
	}
	return blocks, nil
}

// documentLine is one line of a range of a document, with where it starts.
type documentLine struct {
	start int
	text  string
}

// linesOf splits a range of a document into trimmed lines.
func linesOf(content []byte, start, end int) []documentLine {
	var lines []documentLine
	for index := start; index < end; {
		lineEnd := end
		if next := bytes.IndexByte(content[index:end], '\n'); next >= 0 {
			lineEnd = index + next + 1
		}
		lines = append(lines, documentLine{
			start: index,
			text:  strings.TrimSpace(string(content[index:lineEnd])),
		})
		index = lineEnd
	}
	return lines
}

// blockCutEnd narrows where a block ends so the comments that follow it stay: a
// run of comment lines separated from the block by a blank line describes what
// comes next, not the block being removed.
func blockCutEnd(content []byte, block tableBlock) int {
	lines := linesOf(content, block.start, block.end)
	last := len(lines) - 1
	for last >= 0 && lines[last].text == "" {
		last--
	}
	if last < 0 || !strings.HasPrefix(lines[last].text, "#") {
		return block.end
	}
	first := last
	for first > 0 && strings.HasPrefix(lines[first-1].text, "#") {
		first--
	}
	if first == 0 || lines[first-1].text != "" {
		return block.end
	}
	return lines[first].start
}

// removeTableBlocks removes blocks from a document, bottom first, keeping the
// comments that close them when those belong to what follows.
func removeTableBlocks(content []byte, blocks []tableBlock) []byte {
	for _, block := range slices.Backward(blocks) {
		content = splice(content, block.start, blockCutEnd(content, block), nil)
	}
	return content
}

// splice replaces the bytes in [start, end) of a document.
func splice(content []byte, start, end int, replacement []byte) []byte {
	updated := make([]byte, 0, len(content)-(end-start)+len(replacement))
	updated = append(updated, content[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, content[end:]...)
	return updated
}

// tableEntry is one key of a table block the editor writes. An empty Value
// means the key must not be in the document.
type tableEntry struct {
	Key   string
	Value string
}

// hasDocumentValue reports whether a named table assigns a key.
func hasDocumentValue(content []byte, targetSection string, key string) (bool, error) {
	parser := &unstable.Parser{KeepComments: true}
	parser.Reset(content)
	section := ""
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table, unstable.ArrayTable:
			section = nodePath(expression)
		case unstable.KeyValue:
			if section == targetSection && nodeKey(expression) == key {
				return true, nil
			}
		}
	}
	if err := parser.Error(); err != nil {
		return false, fmt.Errorf("invalid TOML: %w", err)
	}
	return false, nil
}

// lastKeyValueEnd is where the last key of a table block ends, the newline
// included: a key added to the block goes there, before the comments that close
// it.
func lastKeyValueEnd(block []byte, path string) int {
	parser := &unstable.Parser{KeepComments: true}
	parser.Reset(block)
	section := ""
	end := 0
	lineEnd := func(offset, length uint32) int {
		position := int(offset) + int(length)
		for position < len(block) && block[position] != '\n' {
			position++
		}
		if position < len(block) {
			position++
		}
		return position
	}
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table, unstable.ArrayTable:
			section = nodePath(expression)
			if end == 0 {
				end = lineEnd(expression.Raw.Offset, expression.Raw.Length)
			}
		case unstable.KeyValue:
			if section == path {
				end = lineEnd(expression.Raw.Offset, expression.Raw.Length)
			}
		}
	}
	return end
}

// writeTableKeys rewrites the keys of one table block in place: a key that is
// there keeps its line and the comment at its end, a missing one is appended
// after the last key, and a key with an empty value is removed. The header, the
// comments and the keys this editor does not know are left untouched.
func writeTableKeys(block []byte, path string, entries []tableEntry) ([]byte, error) {
	updated := block
	var missing []tableEntry
	for _, entry := range entries {
		found, err := hasDocumentValue(updated, path, entry.Key)
		if err != nil {
			return nil, err
		}
		if !found {
			if entry.Value != "" {
				missing = append(missing, entry)
			}
			continue
		}
		if entry.Value == "" {
			updated, _, err = removeDocumentValue(updated, path, entry.Key)
		} else {
			updated, err = replaceDocumentValue(updated, path, entry.Key, entry.Value)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(missing) == 0 {
		return updated, nil
	}
	var builder strings.Builder
	for _, entry := range missing {
		builder.WriteString(entry.Key)
		builder.WriteString(" = ")
		builder.WriteString(entry.Value)
		builder.WriteByte('\n')
	}
	at := lastKeyValueEnd(updated, path)
	return splice(updated, at, at, []byte(builder.String())), nil
}

// insertArrayTables adds array-of-tables blocks at the end of a named table,
// before the table header that follows it.
func insertArrayTables(content []byte, section string, block string) ([]byte, error) {
	tables, err := documentTables(content)
	if err != nil {
		return nil, err
	}
	sectionIndex := -1
	for index, table := range tables {
		if table.path == section && !table.arrayTable {
			sectionIndex = index
			break
		}
	}
	if sectionIndex == -1 {
		return nil, fmt.Errorf("table [%s] was not found", section)
	}
	insertAt := len(content)
	if sectionIndex+1 < len(tables) {
		insertAt = tables[sectionIndex+1].offset
	}
	prefix := ""
	if insertAt > 0 && content[insertAt-1] != '\n' {
		prefix = "\n"
	}
	updated := make([]byte, 0, len(content)+len(prefix)+len(block)+1)
	updated = append(updated, content[:insertAt]...)
	updated = append(updated, prefix...)
	updated = append(updated, block...)
	updated = append(updated, '\n')
	updated = append(updated, content[insertAt:]...)
	return updated, nil
}

// battleServersDocumentValueSection formats the tables under any section: the
// main document uses Games.<id>, a per-game file uses Game.
func battleServersDocumentValueSection(section string, gameID string, servers []BattleServerConfig) string {
	var builder strings.Builder
	for index, server := range servers {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString("[[")
		builder.WriteString(section)
		builder.WriteString(".BattleServers]]\n")
		builder.WriteString("Region = ")
		builder.WriteString(tomlString(server.Region))
		builder.WriteByte('\n')
		if server.Name != "" {
			builder.WriteString("Name = ")
			builder.WriteString(tomlString(server.Name))
			builder.WriteByte('\n')
		}
		builder.WriteString("IPv4 = ")
		builder.WriteString(tomlString(server.IPv4))
		builder.WriteByte('\n')
		builder.WriteString("BsPort = ")
		builder.WriteString(strconv.Itoa(server.BsPort))
		builder.WriteByte('\n')
		builder.WriteString("WebSocketPort = ")
		builder.WriteString(strconv.Itoa(server.WebSocketPort))
		builder.WriteByte('\n')
		// AoE: DE has no out-of-band port, so the key is not written for it.
		if gameSupportsOutOfBandPort(gameID) {
			builder.WriteString("OutOfBandPort = ")
			builder.WriteString(strconv.Itoa(server.OutOfBandPort))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

// gameSupportsOutOfBandPort reports whether a game's battle server uses an
// out-of-band (observer) port.
func gameSupportsOutOfBandPort(gameID string) bool {
	return gameID != "age1"
}

// battleServerEntries is the key set one [[Games.<game>.BattleServers]] block
// holds, in the documented order. A key the document keeps but the editor does
// not need is removed.
func battleServerEntries(gameID string, server BattleServerConfig) []tableEntry {
	entries := []tableEntry{
		{Key: "Region", Value: tomlString(server.Region)},
		{Key: "Name"},
		{Key: "IPv4", Value: tomlString(server.IPv4)},
		{Key: "BsPort", Value: strconv.Itoa(server.BsPort)},
		{Key: "WebSocketPort", Value: strconv.Itoa(server.WebSocketPort)},
		{Key: "OutOfBandPort"},
	}
	if server.Name != "" {
		entries[1].Value = tomlString(server.Name)
	}
	if gameSupportsOutOfBandPort(gameID) {
		entries[5].Value = strconv.Itoa(server.OutOfBandPort)
	}
	return entries
}

// embeddedBattleServerEntries is the delta of one per-game table against the
// values inherited from config.toml, in the documented order. A key equal to
// the inherited value is left empty: it is removed from the document, so the
// inherited value keeps applying instead of being pinned to a copy.
func embeddedBattleServerEntries(gameID string, ebs *EmbeddedBattleServerConfig, base EmbeddedBattleServerConfig) []tableEntry {
	got := normalizeEmbedded(*ebs)
	inherited := normalizeEmbedded(base)
	entries := []tableEntry{
		{Key: "StartIfMissing"},
		{Key: "Region"},
		{Key: "Name"},
		{Key: "IPv4"},
		{Key: "BsPort"},
		{Key: "WebSocketPort"},
		{Key: "OutOfBandPort"},
		{Key: "LogLevel"},
	}
	if got.StartIfMissing != inherited.StartIfMissing {
		entries[0].Value = tomlString(got.StartIfMissing)
	}
	if got.Region != inherited.Region {
		entries[1].Value = tomlString(got.Region)
	}
	if got.Name != inherited.Name {
		entries[2].Value = tomlString(got.Name)
	}
	if got.IPv4 != inherited.IPv4 {
		entries[3].Value = tomlString(got.IPv4)
	}
	if got.BsPort != inherited.BsPort {
		entries[4].Value = strconv.Itoa(got.BsPort)
	}
	if got.WebSocketPort != inherited.WebSocketPort {
		entries[5].Value = strconv.Itoa(got.WebSocketPort)
	}
	if gameSupportsOutOfBandPort(gameID) && got.OutOfBandPort != inherited.OutOfBandPort {
		entries[6].Value = strconv.Itoa(got.OutOfBandPort)
	}
	if got.LogLevel != inherited.LogLevel {
		entries[7].Value = tomlString(got.LogLevel)
	}
	return entries
}

// embeddedBattleServerDocumentValueSection formats a new per-game table with
// only the delta entries that carry a value: the rest stays inherited from
// config.toml.
func embeddedBattleServerDocumentValueSection(section string, ebs *EmbeddedBattleServerConfig, base EmbeddedBattleServerConfig, gameID string) string {
	var builder strings.Builder
	builder.WriteString("[")
	builder.WriteString(section)
	builder.WriteString(".EmbeddedBattleServer]\n")
	for _, entry := range embeddedBattleServerEntries(gameID, ebs, base) {
		if entry.Value == "" {
			continue
		}
		builder.WriteString(entry.Key)
		builder.WriteString(" = ")
		builder.WriteString(entry.Value)
		builder.WriteByte('\n')
	}
	return builder.String()
}

// insertInlineTable adds a single inline table at the end of a named table,
// before the table header that follows it.
func insertInlineTable(content []byte, section string, block string) ([]byte, error) {
	tables, err := documentTables(content)
	if err != nil {
		return nil, err
	}
	sectionIndex := -1
	for index, table := range tables {
		if table.path == section && !table.arrayTable {
			sectionIndex = index
			break
		}
	}
	if sectionIndex == -1 {
		return nil, fmt.Errorf("section %s not found", section)
	}
	insertAt := len(content)
	if sectionIndex+1 < len(tables) {
		insertAt = tables[sectionIndex+1].offset
	}
	updated := make([]byte, 0, len(content)+len(block)+4)
	prefix := content[:insertAt]
	updated = append(updated, prefix...)
	// The document keeps one blank line between tables, and the block is one:
	// match that on both sides instead of gluing it to its neighbours.
	if len(prefix) > 0 {
		if prefix[len(prefix)-1] != '\n' {
			updated = append(updated, '\n')
		}
		if len(prefix) < 2 || prefix[len(prefix)-2] != '\n' {
			updated = append(updated, '\n')
		}
	}
	updated = append(updated, block...)
	if len(block) > 0 && block[len(block)-1] != '\n' {
		updated = append(updated, '\n')
	}
	if insertAt < len(content) && content[insertAt] != '\n' {
		updated = append(updated, '\n')
	}
	updated = append(updated, content[insertAt:]...)
	return updated, nil
}

// resolveIPv4 returns the IPv4 addresses resolved from a host value.
func resolveIPv4(host string) []string {
	if address, err := netip.ParseAddr(host); err == nil && address.Is4() {
		return []string{address.String()}
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return nil
	}
	var ipv4 []string
	for _, address := range addresses {
		if parsed, err := netip.ParseAddr(address.IP.String()); err == nil && parsed.Is4() {
			ipv4 = append(ipv4, parsed.String())
		}
	}
	return ipv4
}

// firstExistingPath returns the first candidate that exists on disk.
func firstExistingPath(candidates []string, description string) (string, error) {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("check %s %q: %w", description, candidate, err)
		}
	}
	return "", os.ErrNotExist
}

// defaultConfigPath finds config.toml in the server's standard candidate paths.
func defaultConfigPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find executable: %w", err)
	}
	executableDirectory := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(executableDirectory, "resources", "config", configFileName),
		filepath.Join(executableDirectory, configFileName),
		filepath.Join("resources", "config", configFileName),
		configFileName,
	}
	path, err := firstExistingPath(candidates, "configuration file")
	if err != nil {
		return "", err
	}
	return path, nil
}

// templateConfigPath finds the official template in installed and development layouts.
func templateConfigPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find executable: %w", err)
	}
	executableDirectory := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(executableDirectory, "resources", "config", configFileName),
		filepath.Join(executableDirectory, "..", "..", "..", "..", "server", "resources", "config", configFileName),
		filepath.Join("..", "..", "..", "server", "resources", "config", configFileName),
		filepath.Join("server", "resources", "config", configFileName),
	}
	path, err := firstExistingPath(candidates, "configuration template")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("configuration template %q was not found", configFileName)
		}
		return "", err
	}
	return path, nil
}
