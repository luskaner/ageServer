package advertisement

import (
	"net/http"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type searchQuery struct {
	AppBinaryChecksum int32 `schema:"appBinaryChecksum"`
	DataChecksum      int32 `schema:"dataChecksum"`
	// AoE3 does not send match type when searching for observable games
	MatchType      *uint8 `schema:"matchType_id"`
	ModDllFile     string `schema:"modDLLFile"`
	ModDllChecksum int32  `schema:"modDLLChecksum"`
	ModName        string `schema:"modName"`
	ModVersion     string `schema:"modVersion"`
	VersionFlags   uint32 `schema:"versionFlags"`
}

type wanQuery struct {
	Length int `schema:"count"`
	Offset int `schema:"start"`
}

func findAdvResp(errorCode int, advs i.A, usersProfileInfo i.A) i.A {
	resp := getAdvResp(errorCode, advs)
	resp = append(resp, usersProfileInfo) // user profiles
	return resp
}

func findAdvertisements(w http.ResponseWriter, r *http.Request, length int, offset int, ongoing bool, lanRegions map[string]struct{}, extraCheck func(models.Advertisement) bool, includeUserProfiles bool) {
	var q searchQuery
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, findAdvResp(2, i.A{}, i.A{})) // resultCode=2 (error), empty advertisements, empty user profiles
		return
	}
	g := models.G(r)
	title := g.Title()
	sess := models.SessionOrPanic(r)
	currentUserId := sess.GetUserId()
	var battleServers models.BattleServers
	if len(lanRegions) == 0 {
		battleServers = g.BattleServers()
	}
	var tagsCheck func(models.Advertisement) bool
	if battleServers != nil && (title == game.AoE2 || title == game.AoM || title == game.AoE4) {
		ok, numericTags, stringTags := parseTags(r)
		if ok {
			tagsCheck = func(adv models.Advertisement) bool {
				return adv.UnsafeMatchesTags(numericTags, stringTags)
			}
		}
	}
	var userIds mapset.Set[int32]
	if includeUserProfiles {
		userIds = mapset.NewThreadUnsafeSet[int32]()
	}
	advs := g.Advertisements().LockedFindAdvertisementsEncoded(title, sess.GetClientLibVersion(), length, offset, true, func(adv models.Advertisement) bool {
		peers := adv.GetPeers()
		_, isPeer := peers.Load(currentUserId)
		var matchesBattleServer bool
		if battleServers == nil {
			_, matchesBattleServer = lanRegions[adv.GetRelayRegion()]
		} else {
			_, matchesBattleServer = battleServers.Get(adv.GetRelayRegion())
		}
		if !isPeer &&
			(tagsCheck == nil || tagsCheck(adv)) &&
			(adv.UnsafeGetJoinable() != ongoing || adv.UnsafeGetVisible() != ongoing) &&
			adv.UnsafeGetAppBinaryChecksum() == q.AppBinaryChecksum &&
			adv.UnsafeGetDataChecksum() == q.DataChecksum &&
			(q.MatchType == nil || adv.UnsafeGetMatchType() == *q.MatchType) &&
			adv.UnsafeGetModDllFile() == q.ModDllFile &&
			adv.UnsafeGetModDllChecksum() == q.ModDllChecksum &&
			adv.UnsafeGetModName() == q.ModName &&
			adv.UnsafeGetModVersion() == q.ModVersion &&
			adv.UnsafeGetVersionFlags() == q.VersionFlags &&
			matchesBattleServer &&
			(extraCheck == nil || extraCheck(adv)) {
			if includeUserProfiles {
				_, peersIter := peers.Iter()
				for userId := range peersIter {
					userIds.Add(userId)
				}
			}
			return true
		}
		return false
	})
	var usersProfileInfoEncoded i.A
	if includeUserProfiles {
		users := g.Users()
		usersProfileInfoEncoded = make(i.A, userIds.Cardinality())
		j := 0
		for userId := range userIds.Iter() {
			user, _ := users.GetUserById(userId)
			usersProfileInfoEncoded[j] = user.EncodeProfileInfo(sess.GetClientLibVersion())
			j++
		}
	}
	i.JSON(&w, findAdvResp(0, advs, usersProfileInfoEncoded))
}

func FindAdvertisements(w http.ResponseWriter, r *http.Request) {
	var q wanQuery
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, findAdvResp(2, i.A{}, i.A{})) // resultCode=2 (error), empty advertisements, empty user profiles
		return
	}
	findAdvertisements(w, r, q.Length, q.Offset, false, nil, nil, false)
}
