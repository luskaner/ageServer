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

// MapHosts maps the domains of the game to ip. When the API is a separate
// server it leaves the API domains alone: the pass that maps them is the one
// that points them at that server.
func (c *Config) MapHosts(gameId string, ip string, macOsExclusiveMappings bool, canMap bool, customHostFile bool, apiSeparate bool) (exitCode int) {
	return c.mapHosts(gameId, ip, macOsExclusiveMappings, canMap, customHostFile, apiSeparate, false)
}

// MapApiHosts maps the API domains to the address of a standalone API server.
func (c *Config) MapApiHosts(gameId string, ip string, macOsExclusiveMappings bool, canMap bool, customHostFile bool) (exitCode int) {
	return c.mapHosts(gameId, ip, macOsExclusiveMappings, canMap, customHostFile, false, true)
}

func (c *Config) mapHosts(gameId string, ip string, macOsExclusiveMappings bool, canMap bool, customHostFile bool, apiSeparate bool, apiOnly bool) (exitCode int) {
	domains := func() []string {
		switch {
		case apiOnly:
			return common.ApiHosts(gameId, macOsExclusiveMappings)
		case apiSeparate:
			return common.RootHosts(gameId, macOsExclusiveMappings)
		default:
			return common.AllHosts(gameId, macOsExclusiveMappings)
		}
	}
	parsedIP := net.ParseIP(ip)
	mappings := func() hosts.HostMappings {
		switch {
		case apiOnly:
			return hosts.ApiMappings(gameId, parsedIP, macOsExclusiveMappings)
		case apiSeparate:
			return hosts.RootMappings(gameId, parsedIP, macOsExclusiveMappings)
		default:
			return hosts.Mappings(gameId, parsedIP, macOsExclusiveMappings)
		}
	}
	var mapIP bool
	if !customHostFile {
		for _, domain := range domains() {
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
			// The two passes share one file: the API pass creates it and the
			// server pass adds to it, so the client is handed every domain.
			if c.hostFilePath == "" {
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
				str += fmt.Sprintf("Saving hosts to '%s' file", c.hostFilePath)
			}
		} else if apiOnly {
			str += fmt.Sprintf("Adding API hosts to hosts file with IP %s", ip)
		} else {
			// No quotes around the address: the inline styler already marks it as a
			// value, and quotes on top of that only add noise.
			str += fmt.Sprintf("Adding hosts to hosts file with IP %s", ip)
		}
		c.report().Step("%s", str+"...")
		var err error
		logName := "config_setup_hosts"
		if apiOnly {
			logName = "config_setup_api_hosts"
		}
		if err = commonLogger.FileLogger.Buffer(logName, func(writer io.Writer) {
			cfgSetupOpts := executor.NewConfigSetupOptions()
			cfgSetupOpts.Out = writer
			cfgSetupOpts.OptionsFn = func(options *exec.Options) {
				commonLogger.Println("run config setup for hosts", options.String())
			}
			cfgSetupOpts.GameId = gameId
			cfgSetupOpts.MapIp = parsedIP
			cfgSetupOpts.MacOsExclusiveMappings = macOsExclusiveMappings
			cfgSetupOpts.HostFilePath = c.hostFilePath
			cfgSetupOpts.Api = apiOnly
			// With the API server apart, the root pass maps only its own
			// half: without the flag the elevated side would use the whole
			// set and write the API domains back at the root's address,
			// overwriting the API pass.
			cfgSetupOpts.RootOnly = apiSeparate
			cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
			if result := cfgSetupOpts.RunSetUp(); !result.Success() {
				// The address is not repeated: the line above already named it, and a
				// failure is about the step, not about the value that step was given.
				if apiOnly {
					c.report().Fail("Failed to add API hosts to hosts file.")
				} else {
					c.report().Fail("Failed to add hosts to hosts file.")
				}
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
				if parsedIP == nil {
					exitCode = launcher.ErrConfigIpMapAdd
				} else {
					for hostToCache, ipToCache := range mappings() {
						common.CacheMapping(string(hostToCache), ipToCache.String())
					}
				}
			}
		}); err != nil {
			c.report().Fail("Failed to write hosts setup log: %s", err)
		}
	}
	return
}
