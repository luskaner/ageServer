package Client

import (
	"net/http"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models/athens/user"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/models/playfab/data"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type getUserReadOnlyDataResponse struct {
	DataVersion uint32
	Data        map[string]any
}

type getUserReadonlyDataRequest struct {
	IfChangedFromDataVersion *uint32
	Keys                     []string
	PlayFabId                *string
}

func nullableData[T any](val *data.Value[T]) any {
	if val == nil {
		return nil
	}
	return val
}

func getValue(key string, userData *user.Data) any {
	switch {
	case key == "PunchCardProgress":
		return userData.PunchCardProgress.ToValue()
	case key == "CurrentGauntletProgress":
		return nullableData(userData.Challenge.Progress.ToValue())
	case key == "CurrentGauntletLabyrinth":
		return nullableData(userData.Challenge.Labyrinth.ToValue())
	case strings.HasPrefix(key, "Mission_Season0_"):
		storyMission := userData.StoryMissions[key]
		return storyMission.ToValue()
	default:
		return nil
	}
}

func GetUserReadOnlyData(w http.ResponseWriter, r *http.Request) {
	var req getUserReadonlyDataRequest
	err := i.Bind(r, &req)
	if err != nil || len(req.Keys) == 0 {
		shared.RespondBadRequest(&w)
		return
	}
	sess := playfab.SessionOrPanic(r)
	u := sess.User()
	d := u.(*user.User).PlayfabData
	var response getUserReadOnlyDataResponse
	_ = d.WithReadOnly(func(d *user.Data) error {
		response = getUserReadOnlyDataResponse{
			DataVersion: d.DataVersion,
			Data:        make(map[string]any),
		}
		if req.IfChangedFromDataVersion == nil || *req.IfChangedFromDataVersion < d.DataVersion {
			for _, key := range req.Keys {
				if val := getValue(key, d); val != nil {
					response.Data[key] = val
				}
			}
		}
		return nil
	})
	shared.RespondOK(&w, response)
}
