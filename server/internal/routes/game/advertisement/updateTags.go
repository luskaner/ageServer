package advertisement

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
)

type tagRequest struct {
	NumericTagNames  i.Json[[]string] `schema:"numericTagNames"`
	NumericTagValues i.Json[[]int32]  `schema:"numericTagValues"`
	StringTagNames   i.Json[[]string] `schema:"stringTagNames"`
	StringTagValues  i.Json[[]string] `schema:"stringTagValues"`
}

func parseTags(r *http.Request) (ok bool, numericTags map[string]int32, stringTags map[string]string) {
	var t tagRequest
	if err := i.Bind(r, &t); err != nil {
		return
	}
	if (len(t.StringTagNames.Data) != len(t.StringTagValues.Data)) || (len(t.NumericTagNames.Data) != len(t.NumericTagValues.Data)) {
		return
	}
	numericTags = make(map[string]int32, len(t.NumericTagNames.Data))
	stringTags = make(map[string]string, len(t.StringTagNames.Data))
	for j := 0; j < len(t.NumericTagNames.Data); j++ {
		numericTags[t.NumericTagNames.Data[j]] = t.NumericTagValues.Data[j]
	}
	for j := 0; j < len(t.StringTagNames.Data); j++ {
		stringTags[t.StringTagNames.Data[j]] = t.StringTagValues.Data[j]
	}
	ok = true
	return
}

func updateTagsReturnError(w *http.ResponseWriter) {
	i.JSON(w, i.A{2})
}

func UpdateTags(w http.ResponseWriter, r *http.Request) {
	ok, numericTags, stringTags := parseTags(r)
	if !ok {
		updateTagsReturnError(&w)
		return
	}
	var q shared.AdvertisementId
	if err := i.Bind(r, &q); err != nil {
		updateTagsReturnError(&w)
		return
	}
	game := models.G(r)
	advertisements := game.Advertisements()
	matchingAdv, foundAdv := advertisements.GetAdvertisement(q.AdvertisementId)
	if !foundAdv {
		updateTagsReturnError(&w)
		return
	}
	sess := models.SessionOrPanic(r)
	advertisements.WithWriteLock(matchingAdv.GetId(), func() {
		if matchingAdv.UnsafeGetHostId() != sess.GetUserId() {
			updateTagsReturnError(&w)
			ok = false
			return
		}
		matchingAdv.UnsafeUpdateTags(numericTags, stringTags)
	})
	if !ok {
		return
	}
	i.JSON(&w, i.A{0})
}
