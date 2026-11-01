package news

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetNews(w http.ResponseWriter, r *http.Request) {
	news := models.G(r).Resources().News()
	if news == nil {
		news = []i.A{}
	}
	i.JSON(&w, i.A{0, news, i.A{}})
}
