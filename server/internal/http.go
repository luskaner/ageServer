package internal

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
)

type Json[T any] struct {
	Data T
}

func (j *Json[T]) UnmarshalText(text []byte) (err error) {
	return json.Unmarshal(text, &j.Data)
}

func (j *Json[T]) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		return j.UnmarshalText([]byte(text))
	}
	return json.Unmarshal(data, &j.Data)
}

type A = []any
type H map[string]any

var decoder = schema.NewDecoder()

func writeJSONHeader(w *http.ResponseWriter) {
	(*w).Header().Set("Content-Type", "application/json;charset=utf-8")
}

func JSON(w *http.ResponseWriter, data any) {
	// Encode explicitly before writing: the response only ever receives
	// escaped JSON instead of request-derived values being streamed straight
	// to the client, which static analysis flags as reflected XSS.
	encoded, err := json.Marshal(data)
	if err != nil {
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSONHeader(w)
	_, _ = (*w).Write(append(encoded, '\n'))
}

func RawJSON(w *http.ResponseWriter, data []byte) {
	writeJSONHeader(w)
	_, _ = (*w).Write(data)
}

func decode(dst any, src map[string][]string) error {
	err := decoder.Decode(dst, src)
	if err == nil {
		return nil
	}

	if merr, ok := errors.AsType[schema.MultiError](err); ok {
		for k, err := range merr {
			if _, ok := errors.AsType[schema.UnknownKeyError](err); ok {
				delete(merr, k)
			}
		}
		if len(merr) == 0 {
			return nil
		}
	}

	return err
}

func Bind[D any](r *http.Request, data *D) error {
	var err error
	if r.Method == http.MethodGet {
		err = decode(data, r.URL.Query())
	} else if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(r.Body)
		err = json.NewDecoder(r.Body).Decode(data)
	} else {
		err = r.ParseForm()
		if err != nil {
			return err
		}
		err = decode(data, r.PostForm)
	}
	return err
}
