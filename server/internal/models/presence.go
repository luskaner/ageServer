package models

import (
	i "github.com/luskaner/ageServer/server/internal"
)

type PresenceDefinitions interface {
	Initialize(presence i.A)
	Get(id int32) *PresenceDefinition
}

type PresenceDefinition interface {
	GetId() int32
	GetLabel() *string
}

type MainPresenceDefinitions struct {
	data map[int32]PresenceDefinition
}

func (pd *MainPresenceDefinitions) Initialize(presence i.A) {
	if len(presence) > 0 {
		rawData := presence[1].(i.A) // presence definitions
		pd.data = make(map[int32]PresenceDefinition, len(rawData))
		for _, rawPresence := range rawData {
			rawPresenceArr := rawPresence.(i.A)
			id := int32(rawPresenceArr[0].(float64)) // id
			var label *string
			if rawPresenceArr[2] != nil { // label
				label = new(rawPresenceArr[2].(string))
			}
			pd.data[id] = &MainPresenceDefinition{
				id:    id,
				label: label,
			}
		}
	} else {
		pd.data = map[int32]PresenceDefinition{
			// Offline
			0: &MainPresenceDefinition{
				id:    0,
				label: nil,
			},
			// Online
			1: &MainPresenceDefinition{
				id:    1,
				label: nil,
			},
		}
	}
}

func (pd *MainPresenceDefinitions) Get(id int32) *PresenceDefinition {
	presenceDefinition, ok := pd.data[id]
	if !ok {
		return nil
	}
	return &presenceDefinition
}

type MainPresenceDefinition struct {
	id    int32
	label *string
}

func (pd *MainPresenceDefinition) GetId() int32 {
	return pd.id
}

func (pd *MainPresenceDefinition) GetLabel() *string {
	return pd.label
}
