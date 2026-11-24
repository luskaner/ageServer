package cmd

import (
	"strconv"
	"strings"

	"github.com/knadh/koanf/v2"

	"github.com/luskaner/ageServer/server/internal"
)

// normalizeSafeEnabled converts a TOML boolean written without quotes into the
// string contract [Safe] uses. koanf/mapstructure would otherwise decode true
// as "1"; normalizing here keeps the decoded Configuration canonical.
func normalizeSafeEnabled(k *koanf.Koanf) {
	switch value := k.Get("Safe.Enabled").(type) {
	case bool:
		_ = k.Set("Safe.Enabled", strconv.FormatBool(value))
	case string:
		_ = k.Set("Safe.Enabled", strings.ToLower(strings.TrimSpace(value)))
	}
}

// applySafeConfig validates [Safe] and publishes the resolved runtime values.
func applySafeConfig(cfg *internal.Configuration) error {
	mode, err := internal.ParseSafeMode(cfg.Safe.Enabled)
	if err != nil {
		return err
	}
	internal.SafeConfigRuntime.Enabled = mode
	internal.SafeConfigRuntime.PlatformUserIdExceptions = cfg.Safe.PlatformUserIdExceptions
	return nil
}
