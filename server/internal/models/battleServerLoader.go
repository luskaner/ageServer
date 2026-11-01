package models

import (
	"fmt"
	"sync"

	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

var BattleServersStore = make(map[string][]BattleServer)

// EmbeddedBattleServersStore holds embedded battle servers per game, keyed by region.
var EmbeddedBattleServersStore = make(map[string]map[string]BattleServer)
var embeddedStoreMu sync.RWMutex

// RegisterEmbeddedBattleServer adds an embedded battle server to the store.
// It returns an error if a server with the same region already exists (from config or manager).
func RegisterEmbeddedBattleServer(gameId, region string, bs BattleServer) error {
	embeddedStoreMu.Lock()
	defer embeddedStoreMu.Unlock()

	if EmbeddedBattleServersStore[gameId] == nil {
		EmbeddedBattleServersStore[gameId] = make(map[string]BattleServer)
	}
	if _, exists := EmbeddedBattleServersStore[gameId][region]; exists {
		return fmt.Errorf("embedded battle server for region %q already exists", region)
	}
	if _, exists := BattleServersStore[gameId]; exists {
		for _, existing := range BattleServersStore[gameId] {
			if existing.Region() == region {
				return fmt.Errorf("battle server for region %q already exists (external or manager-discovered)", region)
			}
		}
	}
	EmbeddedBattleServersStore[gameId][region] = bs
	return nil
}

func InitializeBattleServers(gameId string, configBattleServers []i.BattleServer) error {
	var battleServers []BattleServer
	for _, bs := range configBattleServers {
		battleServers = append(battleServers, &MainBattleServer{
			Base: bs.Base,
		})
	}
	tmpBattleServer, err := battleServer.Configs(gameId, true, true)
	if err != nil {
		return err
	}
	for _, bs := range tmpBattleServer {
		battleServers = append(battleServers, &MainBattleServer{
			Base: bs.Base,
		})
	}

	// Append embedded battle servers for this game.
	embeddedStoreMu.RLock()
	for _, bs := range EmbeddedBattleServersStore[gameId] {
		battleServers = append(battleServers, bs)
	}
	embeddedStoreMu.RUnlock()

	if len(battleServers) == 0 {
		switch gameId {
		case game.AoE2:
			logger.Warn("No Battle Server for AoE 2: DE. macOS native clients will not be able to play/observe games.")
		case game.AoE4, game.AoM:
			return fmt.Errorf("no battle server")
		}
	}
	BattleServersStore[gameId] = battleServers
	return nil
}
