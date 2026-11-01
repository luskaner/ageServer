package battleServerBroadcast

import "github.com/luskaner/ageServer/battle-server-broadcast"

func Required() bool {
	mostPriority, restInterfaces, err := battle_server_broadcast.RetrieveBsInterfaceAddresses()
	if err == nil && mostPriority != nil && len(restInterfaces) > 0 {
		return true
	}
	return false
}
