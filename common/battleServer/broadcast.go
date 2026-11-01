package battleServer

import (
	"encoding/binary"
	"fmt"

	battleServerBroadcast "github.com/luskaner/ageLANServer/battle-server-broadcast"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/uuid"
)

const DefaultName = "My Server"

type BroadcastMessage struct {
	Id            uuid.UUID
	Name          string
	PublicPort    uint16
	WebsocketPort uint16
	OutOfBandPort uint16
}

func ParseBroadcastMessage(data []byte, length int) (message *BroadcastMessage, err error) {
	if !battleServerBroadcast.ValidData(data, length) {
		err = fmt.Errorf("invalid data")
		return
	}
	filledData := data[:length]
	restMsg := filledData[3:]
	var id uuid.UUID
	id, err = uuid.Parse(string(restMsg[:battleServerBroadcast.GuidLength]))
	if err != nil {
		return
	}
	restMsg = restMsg[battleServerBroadcast.GuidLength:]
	nameLength := binary.LittleEndian.Uint16(restMsg[:2])
	restMsg = restMsg[2:]
	if len(restMsg) != int(nameLength)+3*battleServerBroadcast.PortSize {
		err = fmt.Errorf("invalid data size, expected %d, actual %d", int(nameLength)+3*battleServerBroadcast.PortSize, len(restMsg))
		return
	}
	name := string(restMsg[:nameLength])
	restMsg = restMsg[nameLength:]
	message = &BroadcastMessage{
		Id:            id,
		Name:          name,
		PublicPort:    binary.LittleEndian.Uint16(restMsg[:battleServerBroadcast.PortSize]),
		WebsocketPort: binary.LittleEndian.Uint16(restMsg[battleServerBroadcast.PortSize : 2*battleServerBroadcast.PortSize]),
		OutOfBandPort: binary.LittleEndian.Uint16(restMsg[2*battleServerBroadcast.PortSize : 3*battleServerBroadcast.PortSize]),
	}
	return
}

func BroadcastPort(gameId string) uint16 {
	if gameId == game.AoE1 {
		return 8888
	}
	return 9999
}
