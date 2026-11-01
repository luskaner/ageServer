package models

import (
	"sync/atomic"

	i "github.com/luskaner/ageServer/server/internal"
)

type MainPeerMutable struct {
	RaceID int32
	TeamID int32
}

type Peer interface {
	GetUserId() int32
	GetParty() int32
	Encode() i.A
	Invite(user User) bool
	Uninvite(user User) bool
	GetMutable() *MainPeerMutable
	UpdateMutable(raceID int32, teamID int32)
}

type MainPeer struct {
	advertisementId int32
	party           int32
	advertisementIp string
	userId          int32
	statGroupID     int32
	mutable         atomic.Pointer[MainPeerMutable]
	invites         *i.SafeSet[User]
}

func NewPeer(advertisementId int32, advertisementIp string, userId int32, statGroupID int32, party int32, raceID int32, teamID int32) Peer {
	peer := &MainPeer{
		advertisementId: advertisementId,
		party:           party,
		advertisementIp: advertisementIp,
		userId:          userId,
		statGroupID:     statGroupID,
		invites:         i.NewSafeSet[User](),
	}
	peer.UpdateMutable(raceID, teamID)
	return peer
}

func (peer *MainPeer) GetUserId() int32 {
	return peer.userId
}

func (peer *MainPeer) GetParty() int32 {
	return peer.party
}

func (peer *MainPeer) GetMutable() *MainPeerMutable {
	return peer.mutable.Load()
}

func (peer *MainPeer) Encode() i.A {
	mutable := peer.GetMutable()
	return i.A{
		peer.advertisementId, // sessionID
		peer.userId,          // profileID
		-1,                   // ranking
		peer.statGroupID,
		mutable.RaceID,
		mutable.TeamID,
		peer.advertisementIp, // advertisementIP
	}
}

func (peer *MainPeer) Invite(user User) bool {
	return peer.invites.Store(user)
}

func (peer *MainPeer) Uninvite(user User) bool {
	return peer.invites.Delete(user)
}

func (peer *MainPeer) UpdateMutable(raceID int32, teamID int32) {
	peer.mutable.Store(&MainPeerMutable{raceID, teamID})
}
