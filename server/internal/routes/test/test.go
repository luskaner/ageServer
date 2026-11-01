package test

import (
	"net/http"
	"strconv"

	"github.com/luskaner/ageServer/common"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func Test(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(common.IdHeader, i.Id.String())
	w.Header().Set(common.VersionHeader, strconv.Itoa(common.AnnounceVersionLatest))
	i.JSON(&w, i.AnnounceMessageData[models.G(r).Title()])
}
