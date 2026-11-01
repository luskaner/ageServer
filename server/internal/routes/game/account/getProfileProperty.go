package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type profilePropertiesRequest struct {
	PropertyId string `schema:"property_id"`
}

type getProfilePropertyRequest struct {
	profilePropertiesRequest
	ProfileID int32 `schema:"profile_id"`
}

func GetProfileProperty(w http.ResponseWriter, r *http.Request) {
	response := i.A{0, i.A{}}
	var req getProfilePropertyRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, response)
		return
	}
	game := models.G(r)
	if u, found := game.Users().GetUserById(req.ProfileID); !found {
		i.JSON(&w, response)
		return
	} else {
		profileProperties := u.GetProfileProperties()
		if profileProperties != nil {
			_ = profileProperties.WithReadOnly(func(data *map[string]string) error {
				if propValue, foundProp := (*data)[req.PropertyId]; foundProp {
					response[1] = append(response[1].(i.A), propValue)
				}
				return nil
			})
		}
	}
	i.JSON(&w, response)
}
