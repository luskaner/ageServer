package cmd

import (
	"crypto/x509"
	"flag"
	"fmt"
	"game/internal/battleServer"
	"game/internal/cmdUtils"
	"game/internal/gameLogs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
	processGame "github.com/luskaner/ageServer/common/process/game"
	"github.com/luskaner/ageServer/common/server"
	"github.com/luskaner/ageServer/common/uuid"
)

var dataPath string
var hostFilePath string
var certFilePath string
var waitBeforeExit time.Duration

func rootCmd() error {
	var gameId string
	if executablePath, err := os.Executable(); err != nil {
		return fmt.Errorf("failed to get executable path (for gameId): %w", err)
	} else {
		executable := filepath.Base(executablePath)
		if gameId = processGame.Game(executable, false); gameId == "" {
			return fmt.Errorf("no game id could be derived from %s", executable)
		}
	}
	if hostFilePath != "" {
		if err := cmdUtils.HandleHostFile(hostFilePath); err != nil {
			return err
		}
	}
	var rootCAs *x509.CertPool
	if certFilePath != "" {
		if pool, err := common.ReadCertsPool(certFilePath); err != nil {
			return fmt.Errorf("failed to read cert file (%s): %w", certFilePath, err)
		} else {
			rootCAs = pool
		}
	}
	var someSucceeded bool
	for _, domain := range common.AllHosts(gameId, false) {
		var connectionInsecureErr error
		connectionSecureErr := server.CheckConnectionFromServer(domain, false, rootCAs)
		if connectionSecureErr != nil {
			connectionInsecureErr = server.CheckConnectionFromServer(domain, true, rootCAs)
		} else {
			connectionInsecureErr = nil
		}
		var lanInsecure bool
		lanSecure := server.LanServerHost(uuid.Nil(), gameId, domain, false, rootCAs)
		if !lanSecure {
			lanInsecure = server.LanServerHost(uuid.Nil(), gameId, domain, true, rootCAs)
		} else {
			lanInsecure = true
		}
		if connectionSecureErr == nil && lanSecure {
			someSucceeded = true
		}
		log.Printf("Domain %s: connection secure: %s, connection insecure: %s, lan secure: %t, lan insecure: %t\n", domain, connectionSecureErr, connectionInsecureErr, lanSecure, lanInsecure)
	}
	if !someSucceeded {
		return fmt.Errorf("no host could be connected to successfully")
	}
	if gameId == game.AoE1 || gameId == game.AoE2 || gameId == game.AoE3 {
		if err := battleServer.StartAndCheck(gameId); err != nil {
			return fmt.Errorf("failed to start and check battle server: %w", err)
		}
	}
	if dataPath != "" {
		log.Println("Data path provided, simulating logs...")
		if err := gameLogs.CreateLogs(gameId, dataPath); err != nil {
			return fmt.Errorf("failed to create logs: %w", err)
		}
	} else {
		log.Printf("Data path not provided, skipping log simulation. Provide --dataPath to enable it.")
	}
	log.Println("Waiting before exiting...")
	time.Sleep(waitBeforeExit)
	return nil
}

func Execute() error {
	log.Printf("Arguments: %s", os.Args)
	flag.StringVar(&dataPath, "dataPath", "", "Path to game's data")
	flag.StringVar(&hostFilePath, "overrideHosts", "", "Override hosts file")
	flag.StringVar(&certFilePath, "overrideCerts", "", "Override cert store")
	flag.DurationVar(&waitBeforeExit, "waitBeforeExit", time.Minute, "Wait before exit")
	flag.Parse()
	return rootCmd()
}
