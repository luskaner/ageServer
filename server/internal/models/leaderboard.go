package models

import i "github.com/luskaner/ageServer/server/internal"

type LeaderboardDefinitions interface {
	Initialize(leaderboards i.A)
	AvatarStatDefinitions() AvatarStatDefinitions
}

type MainLeaderboardDefinitions struct {
	avatarStatDefinitions *MainAvatarStatDefinitions
}

func (l *MainLeaderboardDefinitions) Initialize(leaderboards i.A) {
	l.avatarStatDefinitions = newAvatarStatDefinitions(leaderboards)
}

func (l *MainLeaderboardDefinitions) AvatarStatDefinitions() AvatarStatDefinitions {
	return l.avatarStatDefinitions
}

func newAvatarStatDefinitions(leaderboards i.A) *MainAvatarStatDefinitions {
	avatarStats := leaderboards[8].(i.A) // avatar stats
	l := &MainAvatarStatDefinitions{
		nameToId: make(map[string]int32, len(avatarStats)),
		idToName: make(map[int32]string, len(avatarStats)),
	}
	for _, avatarStat := range avatarStats {
		id := int32(avatarStat.(i.A)[0].(float64)) // id
		name := avatarStat.(i.A)[1].(string)       // name
		l.nameToId[name] = id
		l.idToName[id] = name
	}
	return l
}

type AvatarStatDefinitions interface {
	GetIdByName(name string) (id int32, ok bool)
	GetNameById(id int32) (name string, ok bool)
}

type MainAvatarStatDefinitions struct {
	nameToId map[string]int32
	idToName map[int32]string
}

func (as *MainAvatarStatDefinitions) GetIdByName(name string) (id int32, ok bool) {
	id, ok = as.nameToId[name]
	return
}

func (as *MainAvatarStatDefinitions) GetNameById(id int32) (name string, ok bool) {
	name, ok = as.idToName[id]
	return
}
