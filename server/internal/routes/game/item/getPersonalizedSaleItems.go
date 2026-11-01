package item

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func GetPersonalizedSaleItems(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{0, i.A{}, i.A{}})
}
