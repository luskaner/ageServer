package Automatch2

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetAutomatchMap(w http.ResponseWriter, r *http.Request) {
	i.JSON(&w, models.G(r).Resources().ArrayFiles()["automatchMaps.json"])
}
