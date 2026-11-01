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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const configFileName = "config.toml"

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
		Config:           config,
		ValidationErrors: validateConfiguration(config),
	}, nil
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
	path, err := application.Get().Dialog.OpenFile().
		SetTitle("Select custom files folder").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

// CreateConfigFile validates a new document and creates it at the requested path.
func (s *ConfigService) CreateConfigFile(path string, configuration Configuration, overwrite bool) (*ConfigFile, error) {
	template, err := s.OpenTemplate()
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, errors.New("configuration destination cannot be empty")
	}
	if _, err := os.Stat(path); err == nil && !overwrite {
		return nil, fmt.Errorf("configuration file %q already exists; confirmation required", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("check configuration destination %q: %w", path, err)
	}
	updated, err := prepareConfigurationDocument([]byte(template.Content), configuration)
	if err != nil {
		return nil, fmt.Errorf("create configuration file %q: %w", path, err)
	}
	if overwrite {
		if err := writeConfigFile(path, updated); err != nil {
			return nil, err
		}
	} else if err := writeNewConfigFile(path, updated); err != nil {
		return nil, err
	}
	return s.readConfigFile(path)
}

// SaveConfiguration validates and saves all editable configuration fields.
func (s *ConfigService) SaveConfiguration(path string, configuration Configuration) error {
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

// SaveConfigurationAs validates and saves an edited configuration to a new path.
func (s *ConfigService) SaveConfigurationAs(sourcePath string, destination string, configuration Configuration, overwrite bool) (*ConfigFile, error) {
	if sourcePath == "" || destination == "" {
		return nil, errors.New("configuration source and destination are required")
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

	if _, err := os.Stat(destination); err == nil && !overwrite {
		return nil, fmt.Errorf("configuration file %q already exists; confirmation required", destination)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("check configuration destination %q: %w", destination, err)
	}
	if overwrite {
		if err := writeConfigFile(destination, updated); err != nil {
			return nil, err
		}
	} else if err := writeNewConfigFile(destination, updated); err != nil {
		return nil, err
	}
	return s.readConfigFile(destination)
}

// prepareConfigurationDocument validates and transforms a configuration document
// into its final form, centralising the logic shared by every save/preview path.
func prepareConfigurationDocument(content []byte, configuration Configuration) ([]byte, error) {
	if validationErrors := errorValidationIssues(validateConfiguration(&configuration)); len(validationErrors) > 0 {
		return nil, fmt.Errorf("configuration is invalid: %s", validationErrors[0].Message)
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
		{"Games.age1", "Hosts", tomlArray(configuration.Games.Age1.Hosts)},
		{"Games.age2", "Hosts", tomlArray(configuration.Games.Age2.Hosts)},
		{"Games.age3", "Hosts", tomlArray(configuration.Games.Age3.Hosts)},
		{"Games.age4", "Hosts", tomlArray(configuration.Games.Age4.Hosts)},
		{"Games.athens", "Hosts", tomlArray(configuration.Games.Athens.Hosts)},
		{"Announcement", "Enabled", strconv.FormatBool(configuration.Announcement.Enabled)},
		{"Announcement", "Multicast", strconv.FormatBool(configuration.Announcement.Multicast)},
		{"Announcement", "Port", strconv.Itoa(configuration.Announcement.Port)},
		{"Announcement", "MulticastGroup", tomlString(configuration.Announcement.MulticastGroup)},
		{"CustomFiles", "Enabled", strconv.FormatBool(configuration.CustomFiles.Enabled)},
		{"CustomFiles", "Folder", tomlString(configuration.CustomFiles.Folder)},
		{"CustomFiles", "MaxFileSize", tomlString(configuration.CustomFiles.MaxFileSize)},
		{"CustomFiles", "MaxTotalSize", tomlString(configuration.CustomFiles.MaxTotalSize)},
		{"CustomFiles", "Expiry", tomlString(configuration.CustomFiles.Expiry)},
		{"BattleServerReplays", "Enabled", strconv.FormatBool(configuration.BattleServerReplays.Enabled)},
		{"BattleServerReplays", "Folder", tomlString(configuration.BattleServerReplays.Folder)},
		{"BattleServerReplays", "MaxFileSize", tomlString(configuration.BattleServerReplays.MaxFileSize)},
		{"BattleServerReplays", "MaxTotalSize", tomlString(configuration.BattleServerReplays.MaxTotalSize)},
		{"BattleServerReplays", "Expiry", tomlString(configuration.BattleServerReplays.Expiry)},
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
	for _, gameID := range supportedGameIDs() {
		updated, err = rewriteBattleServers(updated, gameID, gameBattleServers(&configuration, gameID))
		if err != nil {
			return nil, fmt.Errorf("edit Games.%s.BattleServers: %w", gameID, err)
		}
	}
	for _, gameID := range supportedGameIDs() {
		updated, err = rewriteEmbeddedBattleServer(updated, gameID, gameEmbeddedBattleServer(&configuration, gameID))
		if err != nil {
			return nil, fmt.Errorf("edit Games.%s.EmbeddedBattleServer: %w", gameID, err)
		}
	}
	return updated, nil
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

// readConfigFile reads, parses and validates a configuration file from disk.
func (s *ConfigService) readConfigFile(path string) (*ConfigFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %q: %w", path, err)
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
	return &ConfigFile{
		Path:             path,
		Content:          string(content),
		Config:           config,
		ValidationErrors: validateConfiguration(config),
	}, nil
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
		Games: GamesConfiguration{
			Enabled: []string{},
			// The embedded battle server is optional: the server only defaults
			// StartIfMissing (see initConfig in server/internal/cmd/root.go) and
			// fills the region and name with "Server" at runtime. Inventing a
			// region here would make the editor show a value the server never
			// uses, and write it on the next save.
			Age1:   GameConfig{Hosts: []string{"0.0.0.0"}, EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Age2:   GameConfig{Hosts: []string{"0.0.0.0"}, EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Age3:   GameConfig{Hosts: []string{"0.0.0.0"}, EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Age4:   GameConfig{Hosts: []string{"0.0.0.0"}, EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
			Athens: GameConfig{Hosts: []string{"0.0.0.0"}, EmbeddedBattleServer: EmbeddedBattleServerConfig{StartIfMissing: "true"}},
		},
		Announcement: AnnouncementConfig{
			Enabled:        true,
			Multicast:      true,
			Port:           31978,
			MulticastGroup: "239.31.97.8",
		},
		CustomFiles: CustomFilesConfiguration{
			Enabled:      true,
			Folder:       "auto",
			MaxFileSize:  "32MB",
			MaxTotalSize: "256MB",
			Expiry:       "12h",
		},
		BattleServerReplays: BattleServerReplaysConfiguration{
			Enabled:      false,
			Folder:       "auto",
			MaxFileSize:  "32MB",
			MaxTotalSize: "256MB",
			Expiry:       "12h",
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
		temporary.Close()
		os.Remove(temporaryPath)
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
	validationErrors = append(validationErrors, validateGameHosts(config)...)
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
	validationErrors = append(validationErrors, validateBattleServerReplays(config)...)
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

	maxFileSize, fileSizeErr := parseByteSize(config.CustomFiles.MaxFileSize)
	maxTotalSize, totalSizeErr := parseByteSize(config.CustomFiles.MaxTotalSize)
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

// validateBattleServerReplays checks the storage limits only while the feature
// is enabled: when disabled the server never uses those values.
func validateBattleServerReplays(config *Configuration) []ValidationError {
	if !config.BattleServerReplays.Enabled {
		return nil
	}
	var validationErrors []ValidationError
	if strings.TrimSpace(config.BattleServerReplays.Folder) == "" {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.Folder",
			Message: "Must not be empty",
		})
	}

	maxFileSize, fileSizeErr := parseByteSize(config.BattleServerReplays.MaxFileSize)
	maxTotalSize, totalSizeErr := parseByteSize(config.BattleServerReplays.MaxTotalSize)
	fileSizeValid := true
	totalSizeValid := true
	if fileSizeErr != nil {
		fileSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.MaxFileSize",
			Message: "Must be a size such as 32MB",
		})
	} else if maxFileSize <= 0 {
		fileSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.MaxFileSize",
			Message: "Must be greater than zero",
		})
	}
	if totalSizeErr != nil {
		totalSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.MaxTotalSize",
			Message: "Must be a size such as 256MB",
		})
	} else if maxTotalSize <= 0 {
		totalSizeValid = false
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.MaxTotalSize",
			Message: "Must be greater than zero",
		})
	}
	if fileSizeValid && totalSizeValid && maxFileSize > maxTotalSize {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.MaxFileSize",
			Message: "Must not exceed the total size limit",
		})
	}

	if _, err := time.ParseDuration(strings.TrimSpace(config.BattleServerReplays.Expiry)); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "BattleServerReplays.Expiry",
			Message: "Must be a duration such as 12h or 30m, or 0/negative to keep forever",
		})
	}
	// Expiry <= 0 means keep forever (no TTL-based expiration)
	return validationErrors
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

// parseByteSize mirrors the server's size parser: a plain byte count or an
// integer followed by B, KB, MB or GB.
func parseByteSize(value string) (int64, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return 0, errors.New("empty size")
	}
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"GB", 1 << 30},
		{"G", 1 << 30},
		{"MB", 1 << 20},
		{"M", 1 << 20},
		{"KB", 1 << 10},
		{"K", 1 << 10},
		{"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			n, err := strconv.ParseInt(number, 10, 64)
			if err != nil {
				return 0, err
			}
			return n * unit.multiplier, nil
		}
	}
	return strconv.ParseInt(value, 10, 64)
}

// hostOwner is a host of a game, with the position it holds in its list.
type hostOwner struct {
	game  string
	index int
}

// validateGameHosts checks the hosts of every game. An enabled game must
// have hosts that resolve to IPv4 and cannot share an address with another
// enabled game, as the server would not be able to bind it twice. A disabled
// game is not bound at all, so a shared address only warns about it.
func validateGameHosts(config *Configuration) []ValidationError {
	var validationErrors []ValidationError
	enabled := make(map[string]bool, len(config.Games.Enabled))
	for _, gameID := range config.Games.Enabled {
		enabled[gameID] = true
	}

	// The games using each address, so a conflict can be told on both sides.
	owners := make(map[string][]hostOwner)

	for _, gameID := range supportedGameIDs() {
		hosts, supported := gameHosts(config, gameID)
		if !supported {
			continue
		}
		if enabled[gameID] && len(hosts) == 0 {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fmt.Sprintf("Games.%s.Hosts", gameID),
				Message: "Must contain at least one host",
			})
		}
		for index, host := range hosts {
			addresses := resolveIPv4(host)
			if len(addresses) == 0 {
				if enabled[gameID] {
					validationErrors = append(validationErrors, ValidationError{
						Field:   fmt.Sprintf("Games.%s.Hosts.%d", gameID, index),
						Message: "Must resolve to an IPv4 address",
					})
				}
				continue
			}
			for _, address := range addresses {
				owners[address] = append(owners[address], hostOwner{game: gameID, index: index})
			}
		}
	}

	addresses := make([]string, 0, len(owners))
	for address := range owners {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	for _, address := range addresses {
		addressOwners := owners[address]
		if len(addressOwners) < 2 {
			continue
		}
		// The enabled games are the ones the server binds: two of them cannot
		// share an address, as it could not be bound twice.
		var bound, unbound []hostOwner
		for _, owner := range addressOwners {
			if enabled[owner.game] {
				bound = append(bound, owner)
			} else {
				unbound = append(unbound, owner)
			}
		}
		if len(bound) > 1 {
			for _, owner := range bound {
				validationErrors = append(validationErrors, ValidationError{
					Field:   fmt.Sprintf("Games.%s.Hosts.%d", owner.game, owner.index),
					Message: fmt.Sprintf("Already used by %s", strings.Join(otherHostOwners(bound, owner), ", ")),
				})
			}
		}
		// A game that is not enabled is not bound, so it does not take the
		// address away: it is only told about the conflict.
		for _, owner := range unbound {
			validationErrors = append(validationErrors, ValidationError{
				Field:    fmt.Sprintf("Games.%s.Hosts.%d", owner.game, owner.index),
				Message:  fmt.Sprintf("Also used by %s", strings.Join(otherHostOwners(addressOwners, owner), ", ")),
				Severity: severityWarning,
			})
		}
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

// otherHostOwners returns the games of the other owners of an address. Two
// hosts of the same game resolving to the same address count as owners of each
// other, so a duplicated host names its own game instead of an empty message.
func otherHostOwners(owners []hostOwner, subject hostOwner) []string {
	var others []string
	for _, owner := range owners {
		if owner.game == subject.game && owner.index == subject.index {
			continue
		}
		others = append(others, owner.game)
	}
	return others
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
			BsPort:         0,
			WebSocketPort:  0,
			OutOfBandPort:  0,
			LogLevel:       embeddedLogLevelDefault,
		}
	}
	return defaults
}

// embeddedBattleServerDefaults is the embedded battle server a game gets when
// the document has no [Games.<game>.EmbeddedBattleServer] table: the editor
// reads it through serverDefaults, so it can tell "still the default" from
// "configured here".
func embeddedBattleServerDefaults(gameID string) EmbeddedBattleServerConfig {
	games := serverDefaults().Games
	switch gameID {
	case "age1":
		return games.Age1.EmbeddedBattleServer
	case "age2":
		return games.Age2.EmbeddedBattleServer
	case "age3":
		return games.Age3.EmbeddedBattleServer
	case "age4":
		return games.Age4.EmbeddedBattleServer
	case "athens":
		return games.Athens.EmbeddedBattleServer
	default:
		return EmbeddedBattleServerConfig{}
	}
}

// embeddedBattleServerIsDefault reports whether the table only repeats the
// values the server already uses without it. Writing those out would add a
// block to every game on the first save, and move the document away from the
// commented example the server ships.
//
// The empty spellings the server resolves on its own compare equal to what
// they resolve to: an empty IPv4 is "auto" and an empty StartIfMissing is
// "true" (see the startIfMissing evaluation in server/internal/cmd/root.go).
func embeddedBattleServerIsDefault(gameID string, ebs *EmbeddedBattleServerConfig) bool {
	got := *ebs
	want := embeddedBattleServerDefaults(gameID)
	if strings.TrimSpace(got.IPv4) == "" {
		got.IPv4 = "auto"
	}
	if strings.TrimSpace(want.IPv4) == "" {
		want.IPv4 = "auto"
	}
	if strings.TrimSpace(got.StartIfMissing) == "" {
		got.StartIfMissing = "true"
	}
	if strings.TrimSpace(want.StartIfMissing) == "" {
		want.StartIfMissing = "true"
	}
	return got == want
}

// rewriteEmbeddedBattleServer rewrites the embedded battle server of one game
// as a single inline table [Games.<game>.EmbeddedBattleServer]. When disabled or
// empty, the definition is removed.
//
// An existing table is edited key by key, so the comments it holds (and any key
// this editor does not know about) stay where the user wrote them.
func rewriteEmbeddedBattleServer(content []byte, gameID string, ebs *EmbeddedBattleServerConfig) ([]byte, error) {
	section := "Games." + gameID
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
	if ebs == nil || embeddedBattleServerIsDefault(gameID, ebs) {
		return removeTableBlocks(updated, blocks), nil
	}
	if len(blocks) == 0 {
		return insertInlineTable(updated, section, embeddedBattleServerDocumentValue(gameID, ebs))
	}
	rewritten, err := writeTableKeys(updated[blocks[0].start:blocks[0].end], path, embeddedBattleServerEntries(gameID, ebs))
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
	section := "Games." + gameID
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
	for index := len(blocks) - 1; index >= 0; index-- {
		block := updated[blocks[index].start:blocks[index].end]
		rewritten, err := writeTableKeys(block, path, battleServerEntries(gameID, servers[index]))
		if err != nil {
			return nil, err
		}
		updated = splice(updated, blocks[index].start, blocks[index].end, rewritten)
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
	block := battleServersDocumentValue(gameID, extra)
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
	for index := len(blocks) - 1; index >= 0; index-- {
		content = splice(content, blocks[index].start, blockCutEnd(content, blocks[index]), nil)
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

// battleServersDocumentValue formats the battle servers of a game as the TOML
// array of tables the server documents.
func battleServersDocumentValue(gameID string, servers []BattleServerConfig) string {
	var builder strings.Builder
	for index, server := range servers {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString("[[Games.")
		builder.WriteString(gameID)
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

// embeddedBattleServerEntries is the key set [Games.<game>.EmbeddedBattleServer]
// holds, in the documented order.
func embeddedBattleServerEntries(gameID string, ebs *EmbeddedBattleServerConfig) []tableEntry {
	entries := []tableEntry{
		{Key: "StartIfMissing"},
		{Key: "Region"},
		{Key: "Name"},
		{Key: "IPv4"},
		{Key: "BsPort", Value: strconv.Itoa(ebs.BsPort)},
		{Key: "WebSocketPort", Value: strconv.Itoa(ebs.WebSocketPort)},
		{Key: "OutOfBandPort"},
		{Key: "LogLevel"},
	}
	if ebs.StartIfMissing != "" {
		entries[0].Value = tomlString(ebs.StartIfMissing)
	}
	if ebs.Region != "" {
		entries[1].Value = tomlString(ebs.Region)
	}
	if ebs.Name != "" {
		entries[2].Value = tomlString(ebs.Name)
	}
	if ebs.IPv4 != "" {
		entries[3].Value = tomlString(ebs.IPv4)
	}
	if gameSupportsOutOfBandPort(gameID) {
		entries[6].Value = strconv.Itoa(ebs.OutOfBandPort)
	}
	if ebs.LogLevel != "" {
		entries[7].Value = tomlString(ebs.LogLevel)
	}
	return entries
}

// embeddedBattleServerDocumentValue formats the embedded battle server of a game
// as a single inline TOML table the server documents. Keys the server resolves
// on its own are only written when they carry a value, so an empty Region keeps
// meaning "Server" and an empty IPv4 keeps meaning "auto" instead of being
// pinned to a spelling the user never chose.
func embeddedBattleServerDocumentValue(gameID string, ebs *EmbeddedBattleServerConfig) string {
	var builder strings.Builder
	builder.WriteString("[Games.")
	builder.WriteString(gameID)
	builder.WriteString(".EmbeddedBattleServer]\n")
	if ebs.StartIfMissing != "" {
		builder.WriteString("StartIfMissing = ")
		builder.WriteString(tomlString(ebs.StartIfMissing))
		builder.WriteByte('\n')
	}
	if ebs.Region != "" {
		builder.WriteString("Region = ")
		builder.WriteString(tomlString(ebs.Region))
		builder.WriteByte('\n')
	}
	if ebs.Name != "" {
		builder.WriteString("Name = ")
		builder.WriteString(tomlString(ebs.Name))
		builder.WriteByte('\n')
	}
	if ebs.IPv4 != "" {
		builder.WriteString("IPv4 = ")
		builder.WriteString(tomlString(ebs.IPv4))
		builder.WriteByte('\n')
	}
	builder.WriteString("BsPort = ")
	builder.WriteString(strconv.Itoa(ebs.BsPort))
	builder.WriteByte('\n')
	builder.WriteString("WebSocketPort = ")
	builder.WriteString(strconv.Itoa(ebs.WebSocketPort))
	builder.WriteByte('\n')
	// AoE: DE has no out-of-band port; omit the key for age1
	if gameID != "age1" {
		builder.WriteString("OutOfBandPort = ")
		builder.WriteString(strconv.Itoa(ebs.OutOfBandPort))
		builder.WriteByte('\n')
	}
	if ebs.LogLevel != "" {
		builder.WriteString("LogLevel = ")
		builder.WriteString(tomlString(ebs.LogLevel))
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
