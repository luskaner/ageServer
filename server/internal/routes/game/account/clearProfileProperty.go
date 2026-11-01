package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func ClearProfileProperty(w http.ResponseWriter, r *http.Request) {
	response := i.A{0}
	var req profilePropertiesRequest
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
			delete(*data, req.PropertyId)
			return nil
		})
	}
	i.JSON(&w, response)
}
