package shared

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

type response struct {
	Code   int32  `json:"code"`
	Status string `json:"status"`
}

type okResponse struct {
	response
	Data any `json:"data"`
}

type errResponse struct {
	response
	ErrorCode    int32  `json:"errorCode"`
	Error        string `json:"error"`
	ErrorMessage string `json:"errorMessage"`
}

func Respond(w *http.ResponseWriter, code int32, status string, data any) {
	r := okResponse{
		Code:   code,
		Status: status,
		Data:   data,
	}
	i.JSON(w, r)
}

func RespondOK(w *http.ResponseWriter, data any) {
	Respond(w, 200, "OK", data)
}

func RespondError(w *http.ResponseWriter, code int32, status string, errorCode int32, error string, errorMessage string) {
	r := errResponse{
		Code:         code,
		Status:       status,
		ErrorCode:    errorCode,
		Error:        error,
		ErrorMessage: errorMessage,
	}
	i.JSON(w, r)
}

func RespondBadRequest(w *http.ResponseWriter) {
	RespondError(
		w,
		400,
		"BadRequest",
		-1,
		"Could not parse request body.",
		"",
	)
}

func RespondNotAvailable(w *http.ResponseWriter) {
	RespondError(
		w,
		503,
		"ServiceUnavailable",
		-2,
		"Service is currently not available.",
		"",
	)
}
