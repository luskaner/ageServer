package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type addProfilePropertyRequest struct {
	profilePropertiesRequest
	PropertyValue string `schema:"property_value"`
}

func AddProfileProperty(w http.ResponseWriter, r *http.Request) {
	response := i.A{0}
	var req addProfilePropertyRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, response)
		return
	}
	sess := models.SessionOrPanic(r)
	u, _ := models.G(r).Users().GetUserById(sess.GetUserId())
	profileProperties := u.GetProfileProperties()
	if profileProperties != nil {
		_ = profileProperties.WithReadWrite(func(data *map[string]string) error {
			(*data)[req.PropertyId] = req.PropertyValue
			return nil
		})
	}
	i.JSON(&w, response)
}
