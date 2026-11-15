// Package news implements the public news endpoint.
package news

import (
	"net/http"
	"sort"

	commonGame "github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// Item is one news entry of the public API. The official service names the same
// values the game route sends positionally, plus a hidden flag and the three
// localisation ids (§1.a).
type Item struct {
	// ID is any because news.toml keys are forwarded as received: a numeric key
	// becomes an int and a non-numeric one stays a string, exactly like
	// initializeNews does for the game client.
	ID                      any    `json:"id"`
	Hidden                  int32  `json:"hidden"`
	NewsCategory            string `json:"newscategory"`
	NewsTitleLocStringID    int32  `json:"newstitle_locstringid"`
	NewsSubtitleLocStringID int32  `json:"newssubtitle_locstringid"`
	NewsTextLocStringID     int32  `json:"newstext_locstringid"`
	NewsImage               string `json:"newsimage"`
	NewsBody                string `json:"newsbody"`
	Metadata                string `json:"metadata"`
	DateVisible             int64  `json:"datevisible"`
	ExpiryTime              int64  `json:"expirytime"`
}

// Response is the endpoint body: the shared result envelope plus the entries.
type Response struct {
	Result shared.Result `json:"result"`
	News   []Item        `json:"news"`
}

// GetNews returns the news items of the title as objects.
//
// The resource is the same resources/config/<game>/news.toml the game client
// reads through /game/news/getNews, but with two differences the official
// public API has and the game route does not:
//
//   - the field layout is named and has 11 fields instead of 10 positions,
//   - the order is datevisible descending, while initializeNews sorts by id
//     ascending (C9).
//
// locstringid fields are -1: the official service delegates the text to its
// localisation table, which this server does not ship. The raw text travels in
// newsimage/newsbody instead, which is the documented deviation of §1.a.
func GetNews(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	items := game.Resources().News()
	response := Response{Result: shared.OK(), News: make([]Item, 0, len(items))}
	for _, entry := range items {
		response.News = append(response.News, encodeItem(game.Title(), entry))
	}
	// datevisible descending, id descending as the tie-break. A stable sort
	// keeps the resource order for equal dates.
	sort.SliceStable(response.News, func(left, right int) bool {
		if response.News[left].DateVisible != response.News[right].DateVisible {
			return response.News[left].DateVisible > response.News[right].DateVisible
		}
		leftID, _ := i.NumberToInt64(response.News[left].ID)
		rightID, _ := i.NumberToInt64(response.News[right].ID)
		return leftID > rightID
	})
	i.JSON(&w, response)
}

// encodeItem maps one positional news entry to its object form.
//
// newsimage is title dependent, which the live service confirms: age1-age4 send
// the headline text in newsimage, while AoM sends a real image URL from position
// 5 (image_url) and leaves newsbody empty because its article link travels in
// metadata.
func encodeItem(title string, entry i.A) Item {
	image := shared.StringAt(entry, 2)
	if title == commonGame.AoM {
		image = shared.StringAt(entry, 5)
	}
	dateVisible, _ := shared.Int64At(entry, 8)
	expiryTime, _ := shared.Int64At(entry, 9)
	return Item{
		ID:                      shared.At(entry, 0),
		Hidden:                  0,
		NewsCategory:            shared.StringAt(entry, 1),
		NewsTitleLocStringID:    -1,
		NewsSubtitleLocStringID: -1,
		NewsTextLocStringID:     -1,
		NewsImage:               image,
		NewsBody:                shared.StringAt(entry, 6),
		Metadata:                shared.StringAt(entry, 7),
		DateVisible:             dateVisible,
		ExpiryTime:              expiryTime,
	}
}
