package models

import (
	"iter"
	"maps"

	mapset "github.com/deckarep/golang-set/v3"
	i "github.com/luskaner/ageServer/server/internal"
)

type ItemLoadout interface {
	Encode(profileID int32) i.A
	Update(loadoutName string, loadoutType int32, itemOrLocIds mapset.Set[int32])
}

type ItemLoadouts interface {
	Get(id int32) ItemLoadout
	NewItemLoadout(loadoutName string, loadoutType int32, itemOrLocIds mapset.Set[int32], profileID int32) i.A
	Iter() iter.Seq[ItemLoadout]
}

type MainItemLoadout struct {
	LoadoutID    int32
	LoadoutName  string
	ItemOrLocIDs mapset.Set[int32]
	LoadoutType  int32
	// TODO: Implement fields
	// AtributeKeys  []any
	// RecurseLevels []int32
}

func (l *MainItemLoadout) Encode(profileID int32) i.A {
	return i.A{
		l.LoadoutID,
		profileID,
		l.LoadoutName,
		l.LoadoutType,
		// FIXME: Change to appropriate data
		"[]", // attributes (empty JSON array)
	}
}

func (l *MainItemLoadout) Update(loadoutName string, loadoutType int32, itemOrLocIds mapset.Set[int32]) {
	l.LoadoutName = loadoutName
	l.LoadoutType = loadoutType
	l.ItemOrLocIDs = itemOrLocIds
}

type MainItemLoadouts struct {
	ItemLoadouts map[int32]ItemLoadout `json:"itemLoadouts"`
}

func (l *MainItemLoadouts) Get(id int32) ItemLoadout {
	itemLoadout, ok := l.ItemLoadouts[id]
	if !ok {
		return nil
	}
	return itemLoadout
}

func (l *MainItemLoadouts) NewItemLoadout(loadoutName string, loadoutType int32, itemOrLocIds mapset.Set[int32], profileID int32) i.A {
	var loadoutID int32
	i.WithRng(func(rand *i.RandReader) {
		for loadoutID = rand.Int32(); ; {
			if _, exists := l.ItemLoadouts[loadoutID]; !exists {
				break
			}
		}
	})
	itemLoadout := &MainItemLoadout{
		LoadoutID:    loadoutID,
		LoadoutName:  loadoutName,
		ItemOrLocIDs: itemOrLocIds,
		LoadoutType:  loadoutType,
	}
	l.ItemLoadouts[loadoutID] = itemLoadout
	return itemLoadout.Encode(profileID)
}

func (l *MainItemLoadouts) Iter() iter.Seq[ItemLoadout] {
	return maps.Values(l.ItemLoadouts)
}

type ItemLoadoutsUpgradableDefaultData struct {
	InitialUpgradableDefaultData[*MainItemLoadouts]
}

func NewItemLoadoutsUpgradableDefaultData() *ItemLoadoutsUpgradableDefaultData {
	return &ItemLoadoutsUpgradableDefaultData{
		InitialUpgradableDefaultData: InitialUpgradableDefaultData[*MainItemLoadouts]{},
	}
}

func (i *ItemLoadoutsUpgradableDefaultData) Default() *MainItemLoadouts {
	return &MainItemLoadouts{
		ItemLoadouts: make(map[int32]ItemLoadout),
	}
}
