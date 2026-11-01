package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type setAvatarMetadataRequest struct {
	Metadata string `json:"metaData"`
}

func SetAvatarMetadata(w http.ResponseWriter, r *http.Request) {
	var req setAvatarMetadataRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	sess := models.SessionOrPanic(r)
	u, _ := models.G(r).Users().GetUserById(sess.GetUserId())
	_ = u.GetAvatarMetadata().WithReadWrite(func(data *string) error {
		*data = req.Metadata
		return nil
	})
	i.JSON(&w, i.A{0, u.EncodeProfileInfo(sess.GetClientLibVersion())})
}
