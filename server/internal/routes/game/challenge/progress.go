package challenge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// decodeProgressPayload reads the progress document of a client. AoE IV sends
// JSON, while other games post the same endpoint as form data, so both shapes
// are accepted instead of failing the request on a content type mismatch.
func decodeProgressPayload(r *http.Request) (any, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var payload any
	if len(bytes.TrimSpace(body)) > 0 && json.Unmarshal(body, &payload) == nil {
		return payload, nil
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err = r.ParseForm(); err != nil {
		return nil, err
	}
	form := make(map[string]any, len(r.PostForm))
	for key, values := range r.PostForm {
		if len(values) == 0 {
			continue
		}
		var decoded any
		if json.Unmarshal([]byte(values[0]), &decoded) == nil {
			form[key] = decoded
		} else {
			form[key] = values[0]
		}
	}
	return form, nil
}

// collectProgress extracts progress records from single records, arrays, and
// nested envelopes used by different AoE IV client builds.
func collectProgress(value any, out *[]models.ChallengeProgress) {
	switch v := value.(type) {
	case map[string]any:
		if _, ok := v["progressID"]; ok {
			*out = append(*out, models.ChallengeProgress(v))
			return
		}
		if _, ok := v["definition.definitionID"]; ok {
			*out = append(*out, models.ChallengeProgress(v))
			return
		}
		for _, child := range v {
			collectProgress(child, out)
		}
	case []any:
		for _, child := range v {
			collectProgress(child, out)
		}
	}
}

func updateProgress(w http.ResponseWriter, r *http.Request) {
	payload, err := decodeProgressPayload(r)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	var incoming []models.ChallengeProgress
	collectProgress(payload, &incoming)
	if len(incoming) == 0 {
		// Some builds send progress events rather than full records. Do not
		// fabricate progress from an unknown event; acknowledge safely.
		i.JSON(&w, i.A{0, i.A{}, i.A{}})
		return
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	user, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok || user.GetChallengeProgress() == nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	err = user.GetChallengeProgress().WithReadWrite(func(data *[]models.ChallengeProgress) error {
		if data == nil {
			return nil
		}
		// Index both identifiers so partial records that omit progressID still
		// update the matching challenge instead of creating duplicates.
		byID := make(map[string]int, len(*data)*2)
		indexRecord := func(idx int, record models.ChallengeProgress) {
			for _, field := range []string{"progressID", "definition.definitionID"} {
				if id, exists := record[field]; exists {
					byID[field+":"+jsonKey(id)] = idx
				}
			}
		}
		for idx, current := range *data {
			indexRecord(idx, current)
		}
		for _, next := range incoming {
			idx, found := -1, false
			// The definition is stable across resource updates while progressID
			// is reintroduced, so it takes precedence when both are present.
			for _, field := range []string{"definition.definitionID", "progressID"} {
				if id, exists := next[field]; exists {
					if existing, ok := byID[field+":"+jsonKey(id)]; ok {
						idx, found = existing, true
						break
					}
				}
			}
			if found {
				// Merge fields: batched/event updates often contain only status or
				// progress and must not erase the other persisted fields.
				for key, value := range next {
					(*data)[idx][key] = value
				}
				indexRecord(idx, (*data)[idx])
				continue
			}
			if _, hasProgressID := next["progressID"]; !hasProgressID {
				if _, hasDefinitionID := next["definition.definitionID"]; !hasDefinitionID {
					continue
				}
			}
			idx = len(*data)
			copyRecord := make(models.ChallengeProgress, len(next))
			for key, value := range next {
				copyRecord[key] = value
			}
			*data = append(*data, copyRecord)
			indexRecord(idx, copyRecord)
		}
		return nil
	})
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	i.JSON(&w, i.A{0, i.A{}, i.A{}})
}

func jsonKey(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
