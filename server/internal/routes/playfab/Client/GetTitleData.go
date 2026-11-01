package Client

import (
	"fmt"
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type getTitleDataResponseData struct {
	CdnUrl        string
	CdnPathConfig string
}

type getTitleDataResponse struct {
	getTitleDataResponseData `json:"Data"`
}

func GetTitleData(w http.ResponseWriter, r *http.Request) {
	shared.RespondOK(
		&w,
		getTitleDataResponse{
			getTitleDataResponseData{
				CdnUrl:        fmt.Sprintf("https://%s%s", r.Host, playfab.StaticSuffix),
				CdnPathConfig: playfab.StaticConfig,
			},
		},
	)
}
