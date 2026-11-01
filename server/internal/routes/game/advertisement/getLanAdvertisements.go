package advertisement

import (
	"net/http"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
)

type lanQuery struct {
	RelayRegions string `schema:"lanServerGuids"`
}

func GetLanAdvertisements(w http.ResponseWriter, r *http.Request) {
	var q lanQuery
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	if q.RelayRegions == "[]" {
		i.JSON(&w,
			i.A{0, i.A{}, i.A{}},
		)
		return
	}
	lanServerGuids := strings.Split(strings.ReplaceAll(strings.Trim(q.RelayRegions, `[]`), `"`, ``), ",")
	lanServerGuidsMap := make(map[string]struct{}, len(lanServerGuids))
	for _, guid := range lanServerGuids {
		lanServerGuidsMap[guid] = struct{}{}
	}
	findAdvertisements(w, r, 0, 0, false, lanServerGuidsMap, nil, false)
}
