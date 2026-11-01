package certStore

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
)

var cachedCertPool *x509.CertPool

func ReloadSystemCertificates() {
	cachedCertPool = nil
}

func CertPool() *x509.CertPool {
	if cachedCertPool == nil {
		cachedCertPool = x509.NewCertPool()
		for _, userStore := range []bool{false, true} {
			keychain, _, err := KeychainPath(userStore)
			if err != nil {
				return nil
			}
			if output, err := RunCommandWithOutput(
				false,
				`find-certificate`,
				`-a`,
				`-p`,
				keychain,
			); err != nil {
				return nil
			} else {
				cachedCertPool.AppendCertsFromPEM([]byte(output))
			}
		}
	}
	return cachedCertPool
}

func RunCommandWithOutput(asAdmin bool, args ...string) (output string, err error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := runCommand(asAdmin, func(options *exec.Options) {
		options.Stdout = &stdout
		options.Stderr = &stderr
	}, args...)
	if result.Err != nil {
		return "", result.Err
	}
	if result.ExitCode != common.ErrSuccess {
		errText := stderr.String()
		if errText == "" {
			errText = stdout.String()
		}
		errText = strings.TrimSpace(errText)
		if errText == "" {
			errText = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return "", fmt.Errorf("command failed: %s", errText)
	}
	return stdout.String(), nil
}

func runCommand(asAdmin bool, optionsFn func(options *exec.Options), args ...string) *exec.Result {
	options := exec.Options{
		File:        "security",
		SpecialFile: true,
		Args:        args,
		AsAdmin:     asAdmin,
		Wait:        true,
		ExitCode:    true,
	}
	if optionsFn != nil {
		optionsFn(&options)
	}
	return options.Exec()
}

func RunCommandWithoutOutput(asAdmin bool, optionsFn func(*exec.Options), args ...string) error {
	if optionsFn == nil {
		optionsFn = func(c *exec.Options) {}
	}
	result := runCommand(asAdmin, func(options *exec.Options) {
		optionsFn(options)
	}, args...)
	if result.Err != nil {
		return result.Err
	} else if result.ExitCode != common.ErrSuccess {
		return fmt.Errorf("command failed with exit code %d", result.ExitCode)
	}
	return nil
}

func defaultUserKeychain() (value string, err error) {
	var output string
	output, err = RunCommandWithOutput(false, "default-keychain")
	if err != nil {
		return
	}
	path := strings.TrimSpace(output)
	value = strings.Trim(path, `"`)
	return
}

func KeychainPath(userStore bool) (path string, updateAsAdmin bool, err error) {
	if userStore {
		path, err = defaultUserKeychain()
		return path, false, err
	}
	return "/Library/Keychains/System.keychain", true, nil
}
