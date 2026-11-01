package party

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func CreateOrReportSinglePlayer(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{2, 0, "", i.A{}, i.A{}, i.A{}, i.A{}, nil, 0, 0, i.A{}, i.A{}, nil, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}})
}
