package goreleaser

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goreleaser/goreleaser/v2/pkg/config"
	"gopkg.in/yaml.v3"
)

func universalBinaries(binaries []config.Build) []config.UniversalBinary {
	var result []config.UniversalBinary
	for _, binary := range binaries {
		if slices.Contains(binary.Goos, OSMacOS.Goos()) {
			result = append(result, config.UniversalBinary{
				ID:           binary.ID,
				NameTemplate: binary.Binary,
				Replace:      true,
			})
		}
	}
	return result
}

func GenerateConfig(archives ...*Archive) error {
	return generateConfig(nil, archives...)
}

func GenerateCurrentConfig(currentWindowsOS OperatingSystem, archives ...*Archive) error {
	return generateConfig(currentWindowsOS, archives...)
}

func generateConfig(currentWindowsOS OperatingSystem, archives ...*Archive) error {
	// Only single-target configs (GenerateCurrentConfig) are restricted to the
	// host OS/arch. The full config (GenerateConfig) must contain every
	// supported target regardless of the machine that generates it.
	var operatingSystem OperatingSystem
	var architecture Architecture
	if currentWindowsOS != nil {
		operatingSystem = currentWindowsOS
		architecture = currentWindowsArch()
	}

	project := config.Project{
		Version: 2,
	}
	for _, a := range archives {
		// A regular release ships a single universal macOS archive per
		// package, so the per-architecture macOS builds are merged into one
		// multi-architecture build (and universalized below). Single-target
		// builds (build-current) keep one build per host architecture.
		var mergedOSes []OperatingSystem
		if operatingSystem == nil {
			mergedOSes = []OperatingSystem{OSMacOS}
		}
		archiveBuilds := a.Builds(mergedOSes...)
		if operatingSystem != nil {
			archiveBuilds = slices.DeleteFunc(archiveBuilds, func(build config.Build) bool {
				// Build IDs use OS.Name() (e.g., "linux", "mac", "win7", "win10")
				osName := operatingSystem.Name()
				return !strings.Contains(build.ID, "_"+osName+"_") ||
					!slices.Contains(build.Goarch, architecture.Goarch())
			})
		}
		project.Builds = append(project.Builds, archiveBuilds...)
		project.Archives = append(project.Archives, a.Archives(archiveBuilds)...)
	}
	main := "./config-helper"
	windowsTargets := Targets64Windows
	if currentWindowsOS == OSWindowsLegacy {
		windowsTargets = NewBinaryTargets()
		windowsTargets.AddTarget(OSWindowsLegacy, ArchAmd64)
	}
	// Filter config-helper builds to only current target
	if operatingSystem != nil {
		filteredWindowsTargets := NewBinaryTargets()
		for target, archs := range *windowsTargets {
			if target.Goos() == operatingSystem.Goos() {
				for arch := range archs {
					if arch.Goarch() == architecture.Goarch() {
						filteredWindowsTargets.AddTarget(target, arch)
					}
				}
			}
		}
		windowsTargets = filteredWindowsTargets
	}
	for target, archs := range *windowsTargets {
		for arch := range archs {
			binary := filepath.Join("bin", fmt.Sprintf("%s_%s", main, arch.Goarch()))
			id := fmt.Sprintf("%s_%s_%s", binary, target.Goos(), arch.Goarch())
			build := config.Build{
				ID:     id,
				Goos:   []string{target.Goos()},
				Goarch: []string{arch.Goarch()},
				Main:   main,
				Binary: binary,
			}
			if tool := target.Tool(); tool != "" {
				build.Tool = tool
			}
			project.Builds = append(project.Builds, build)
		}
	}
	for i, arch := range project.Archives {
		if strings.Contains(arch.ID, "win") {
			continue
		}
		var targets []string
		if strings.Contains(arch.ID, "launcher") || strings.Contains(arch.ID, "battle-server-manager") {
			targets = []string{"."}
		} else if strings.Contains(arch.ID, "full") {
			targets = []string{"./launcher", "./battle-server-manager"}
		} else {
			continue
		}
		amd64 := strings.Contains(arch.ID, "amd64")
		arm64 := strings.Contains(arch.ID, "arm64")
		if !amd64 && !arm64 {
			amd64 = true
			arm64 = true
		}
		var names []string
		if amd64 {
			names = append(names, "dist/bin/config-helper_amd64_windows_amd64_windows_amd64_v1")
		}
		if arm64 {
			names = append(names, "dist/bin/config-helper_arm64_windows_arm64_windows_arm64_v8.0")
		}
		for _, name := range names {
			for _, target := range targets {
				project.Archives[i].Files = append(project.Archives[i].Files, config.File{
					Source:      name,
					Destination: target,
				})
			}
		}
	}
	project.UniversalBinaries = universalBinaries(project.Builds)
	if operatingSystem != nil {
		project.UniversalBinaries = nil
	}

	project.Checksum = config.Checksum{
		NameTemplate: `{{ .ProjectName }}_{{ .RawVersion }}_checksums.txt`,
	}
	project.Signs = []config.Sign{
		{
			Artifacts: "checksum",
			Cmd:       "gpg2",
			Args:      []string{"--batch", "-u", "{{ .Env.GPG_FINGERPRINT }}", "--output", "${signature}", "--detach-sign", "${artifact}"},
		},
	}
	project.Release = config.Release{
		Draft: true,
		// Re-running the workflow for the same tag must not reuse a draft that
		// already holds assets from a previous run: GitHub rejects those
		// uploads with 422 already_exists and GoReleaser has to delete and
		// re-upload each one. Replacing the draft keeps every run idempotent.
		ReplaceExistingDraft:     true,
		UseExistingDraft:         false,
		ReplaceExistingArtifacts: true,
		Prerelease:               `auto`,
	}
	marshal, err := yaml.Marshal(&project)
	if err != nil {
		return err
	}
	return os.WriteFile(".goreleaser.yaml", marshal, 0o644)
}
