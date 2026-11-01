package ops

import (
	"fmt"
	"io"
	"net"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/hosts"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/common/server"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

func (c *Config) MapHosts(gameId string, ip string, macOsExclusiveMappings bool, canMap bool, customHostFile bool) (exitCode int) {
	var mapIP bool
	if !customHostFile {
		for _, domain := range common.AllHosts(gameId, macOsExclusiveMappings) {
			if !common.Matches(ip, domain) {
				if !canMap {
					c.report().Fail("%s", "serverStart is false and canAddHost is false but server does not match "+domain+". You should have added the host ip mapping to it in the hosts file (or just set canAddHost to true).")
					exitCode = launcher.ErrConfigIpMap
					return
				}
				mapIP = true
			} else if err := server.CheckConnectionFromServer(domain, true, nil); err != nil {
				c.report().Fail("%s", "serverStart is false and host matches. "+domain+" must be reachable. Review the host is reachable via this domain to TCP port 443 (HTTPS).")
				c.report().Fault("Error: %s", err.Error())
				exitCode = launcher.ErrServerUnreachable
				return
			}
		}
	} else {
		mapIP = true
	}
	if mapIP {
		var str string
		if customHostFile {
			hostFileLock, err := hosts.CreateTemp()
			if err != nil {
				c.report().Fail("Failed to create temp hosts file with IP %s: %s", ip, err.Error())
				return launcher.ErrConfigIpMapAdd
			}
			tmpName := hostFileLock.File.Name()
			c.hostFilePath, _ = filepath.Abs(tmpName)
			str += fmt.Sprintf("Saving hosts to '%s' file", tmpName)
			if err = hostFileLock.Unlock(); err != nil {
				c.report().Fail("Failed to unlock temp hosts file %s: %s", tmpName, err.Error())
				return launcher.ErrConfigIpMapAdd
			}
		} else {
			// No quotes around the address: the inline styler already marks it as a
			// value, and quotes on top of that only add noise.
			str += fmt.Sprintf("Adding hosts to hosts file with IP %s", ip)
		}
		c.report().Step("%s", str+"...")
		var err error
		if err = commonLogger.FileLogger.Buffer("config_setup_hosts", func(writer io.Writer) {
			cfgSetupOpts := executor.NewConfigSetupOptions()
			cfgSetupOpts.Out = writer
			cfgSetupOpts.OptionsFn = func(options *exec.Options) {
				commonLogger.Println("run config setup for hosts", options.String())
			}
			cfgSetupOpts.GameId = gameId
			cfgSetupOpts.MapIp = net.ParseIP(ip)
			cfgSetupOpts.MacOsExclusiveMappings = macOsExclusiveMappings
			cfgSetupOpts.HostFilePath = c.hostFilePath
			cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
			if result := cfgSetupOpts.RunSetUp(); !result.Success() {
				// The address is not repeated: the line above already named it, and a
				// failure is about the step, not about the value that step was given.
				c.report().Fail("Failed to add hosts to hosts file.")
				if result.Err != nil {
					c.report().Fault("Error message: %s", result.Err.Error())
				} else {
					c.report().Fault("Error message: none (check the config_setup_hosts and config-admin_setup_hosts log files for details).")
				}
				if result.ExitCode != common.ErrSuccess {
					c.report().Fault("Exit code: %d.", result.ExitCode)
				} else {
					c.report().Fault("Exit code: none (process reported failure without exit code).")
				}
				if logFolder := commonLogger.FileLogger.Folder(); logFolder != "" {
					c.report().Fault("Check log folder %s for config_setup_hosts* and config-admin_setup_hosts* files.", logFolder)
				}
				exitCode = launcher.ErrConfigIpMapAdd
			} else if customHostFile {
				if parsedIP := net.ParseIP(ip); parsedIP != nil {
					mappings := hosts.Mappings(gameId, parsedIP, macOsExclusiveMappings)
					for hostToCache, ipToCache := range mappings {
						common.CacheMapping(string(hostToCache), ipToCache.String())
					}
				} else {
					exitCode = launcher.ErrConfigIpMapAdd
				}
			}
		}); err != nil {
			c.report().Fail("Failed to write hosts setup log: %s", err)
		}
	}
	return
}
