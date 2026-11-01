package common

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// KoanfFileLoadError wraps parser/load errors for a specific config file path.
type KoanfFileLoadError struct {
	Path string
	Err  error
}

func (e *KoanfFileLoadError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func (e *KoanfFileLoadError) Unwrap() error {
	return e.Err
}

// LoadKoanfLayers applies config layers in this order:
// defaults < env < file < flags.
func LoadKoanfLayers(
	k *koanf.Koanf,
	defaults map[string]any,
	fileCandidates []string,
	parser koanf.Parser,
	fs *pflag.FlagSet,
	fsBindings map[string]string,
	envPrefix string,
) (string, error) {
	if k == nil {
		return "", fmt.Errorf("koanf instance is nil")
	}

	_ = k.Load(confmap.Provider(defaults, "."), nil)

	_ = k.Load(koanfEnvProvider(Name+"_"+envPrefix), nil)

	usedFile := ""
	for _, candidate := range fileCandidates {
		if candidate == "" {
			continue
		}
		if err := k.Load(file.Provider(candidate), parser); err == nil {
			usedFile = candidate
			break
		} else if !os.IsNotExist(err) {
			return "", &KoanfFileLoadError{Path: candidate, Err: err}
		}
	}

	var posFlag *posflag.Posflag
	if fsBindings == nil {
		posFlag = posflag.Provider(fs, ".", k)
	} else {
		posFlag = posflag.ProviderWithFlag(
			fs,
			".",
			k,
			func(f *pflag.Flag) (string, any) {
				key := f.Name
				if binding, ok := fsBindings[key]; ok {
					key = binding
				}
				return key, posflag.FlagVal(fs, f)
			})
	}
	_ = k.Load(posFlag, nil)
	return usedFile, nil
}

// osExitFn is injectable for tests.
var osExitFn = os.Exit

func LoadKoanfLayersOrExit(
	k *koanf.Koanf,
	defaults map[string]any,
	fileCandidates []string,
	parser koanf.Parser,
	fs *pflag.FlagSet,
	fsBindings map[string]string,
	envPrefix string,
	printlnFn func(...any),
) string {
	return LoadKoanfLayersOrExitWith(k, defaults, fileCandidates, parser, fs, fsBindings, envPrefix, printlnFn, osExitFn)
}

func LoadKoanfLayersOrExitWith(
	k *koanf.Koanf,
	defaults map[string]any,
	fileCandidates []string,
	parser koanf.Parser,
	fs *pflag.FlagSet,
	fsBindings map[string]string,
	envPrefix string,
	printlnFn func(...any),
	exitFn func(int),
) string {
	usedFile, err := LoadKoanfLayers(k, defaults, fileCandidates, parser, fs, fsBindings, envPrefix)
	if err != nil {
		if fileErr, ok := errors.AsType[*KoanfFileLoadError](err); ok {
			printlnFn("Error parsing config file:", fileErr.Path+":", fileErr.Err.Error())
		} else {
			printlnFn("Error loading config:", err.Error())
		}
		exitFn(ErrConfigParse)
	}
	return usedFile
}

// koanfEnvProvider returns a provider that maps ENV keys to koanf keys using '.' delimiters.
// It strips the prefix, replaces '_' with '.' and preserves the rest of the key case, so
// 'PREFIX_Ports_Bs' matches the 'Ports.Bs' configuration key. Values with spaces become string slices.
func koanfEnvProvider(prefix string) *env.Env {
	finalPrefix := strings.ReplaceAll(strings.ToUpper(prefix), "-", "_") + "_"
	return env.Provider(".", env.Opt{
		Prefix: finalPrefix,
		TransformFunc: func(key, value string) (string, any) {
			k := strings.TrimPrefix(key, finalPrefix)
			k = strings.ReplaceAll(k, "_", ".")
			if strings.Contains(value, " ") {
				return k, strings.Split(value, " ")
			}
			return k, value
		},
	})
}
