package cloud

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type getFileURLRequest struct {
	Names i.Json[[]string] `json:"names"`
}

func GetFileURL(w http.ResponseWriter, r *http.Request) {
	var req getFileURLRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{nil}})
		return
	}
	g := models.G(r)
	cloudFiles := g.Resources().CloudFiles()
	if cloudFiles.Value == nil {
		i.JSON(&w, i.A{2, slices.Repeat(i.A{nil}, len(req.Names.Data))})
		return
	}
	descriptions := make(i.A, len(req.Names.Data))
	gameTitle := g.Title()
	var errorCode int
	for j, name := range req.Names.Data {
		fileData, ok := cloudFiles.Value[name]
		if !ok {
			descriptions[j] = i.A{nil}
			errorCode = 2
			continue
		}
		finalPart := fileData.Key
		description := i.A{
			name,
			fileData.Length,
			fileData.Id,
			fmt.Sprintf("https://%s/cloudfiles/%s", r.Host, finalPart),
		}
		if gameTitle == game.AoE2 {
			description = append(
				description,
				finalPart,
			)
		}
		descriptions[j] = description
	}
	i.JSON(&w, i.A{errorCode, descriptions})
}
