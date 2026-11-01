package leaderboard

import (
	"net/http"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

// TODO: Any fixed stats for aoe4?

var fixedAvatarNames = map[string]mapset.Set[string]{
	game.AoM: mapset.NewSet[string]("STAT_GAUNTLET_REWARD_FAVOUR", "STAT_GAUNTLET_REWARD_XP"),
}

type setAvatarStatValuesRequest struct {
	AvatarStatIds i.Json[[]int32] `schema:"avatarStat_ids"`
	Values        i.Json[[]int64] `schema:"values"`
	// TODO: Implement when we know what it means
	UpdateTypes i.Json[[]int32] `schema:"updateTypes"`
}

func SetAvatarStatValues(w http.ResponseWriter, r *http.Request) {
	var req setAvatarStatValuesRequest
	if err := i.Bind(r, &req); err != nil || len(req.Values.Data) < 1 || len(req.AvatarStatIds.Data) != len(req.Values.Data) || len(req.UpdateTypes.Data) != len(req.Values.Data) {
		i.JSON(&w, i.A{2})
	}
	g := models.G(r)
	users := g.Users()
	sess := models.SessionOrPanic(r)
	u, ok := users.GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	var encodedAvatarStats i.A
	var currentGameFixedAvatarNames mapset.Set[string]
	avatarStatDefinitions := g.LeaderboardDefinitions().AvatarStatDefinitions()
	data := u.GetAvatarStats()
	fixedAvatarIds := mapset.NewThreadUnsafeSet[int32]()
	if currentGameFixedAvatarNames, ok = fixedAvatarNames[g.Title()]; ok {
		for name := range currentGameFixedAvatarNames.Iter() {
			if id, ok := avatarStatDefinitions.GetIdByName(name); ok {
				fixedAvatarIds.Add(id)
			}
		}
	}
	values := make(map[int32]int64)
	for j := 0; j < len(req.AvatarStatIds.Data); j++ {
		avatarStatId := req.AvatarStatIds.Data[j]
		if fixedAvatarIds.ContainsOne(avatarStatId) {
			continue
		}
		values[avatarStatId] = req.Values.Data[j]
	}
	if len(values) > 0 {
		_ = data.WithReadWrite(func(avatarStats *models.AvatarStats) error {
			for avatarStatId, value := range values {
				var avatarStat models.AvatarStat
				if avatarStat, ok = avatarStats.GetStat(avatarStatId); ok {
					avatarStat.SetValue(value)
				} else {
					avatarStat = models.NewAvatarStat(avatarStatId, value)
					avatarStats.AddStat(avatarStat)
				}
				encodedAvatarStats = append(encodedAvatarStats, avatarStat.Encode(u.GetProfileId()))
			}
			return nil
		})
	}
	logger.Printf("leaderboard.setAvatarStatValues: user=%d ids=%v values=%v updateTypes=%v\n", u.GetId(), req.AvatarStatIds.Data, req.Values.Data, req.UpdateTypes.Data)
	if len(encodedAvatarStats) > 0 {
		// TODO: Do others need to be notified?
		// TODO: Does client support multiple AvatarStats in one message?
		wss.SendOrStoreMessage(
			sess,
			"AvatarStatsUpdatedMessage",
			i.A{
				encodedAvatarStats,
			},
		)
	}
	i.JSON(&w, i.A{0})
}
