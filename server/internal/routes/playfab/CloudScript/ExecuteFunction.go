package CloudScript

import (
	"encoding/json"
	"net/http"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens/routes/playfab/cloudScriptFunction"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type executeFunctionRequest struct {
	CustomTags              struct{}
	Entity                  *struct{}
	FunctionName            string
	GeneratePlayStreamEvent *struct{}
	FunctionParameter       json.RawMessage
}

type executeFunctionResponse struct {
	ExecutionTimeMilliseconds int64
	FunctionName              string
	FunctionResult            json.RawMessage `json:",omitempty"`
	FunctionResultSize        uint32
}

func ExecuteFunction(w http.ResponseWriter, r *http.Request) {
	var req executeFunctionRequest
	err := i.Bind(r, &req)
	if err != nil || req.FunctionName == "" {
		shared.RespondBadRequest(&w)
		return
	}
	if fn, ok := cloudScriptFunction.Store[req.FunctionName]; ok {
		reqPar := fn.NewParameters()
		err = json.Unmarshal(req.FunctionParameter, &reqPar)
		if err != nil {
			shared.RespondBadRequest(&w)
			return
		}
		u := playfab.SessionOrPanic(r).User()
		game := models.G(r)
		t := time.Now()
		result := fn.Run(game, u, reqPar)
		duration := time.Since(t).Milliseconds()
		var tmpResultBytes []byte
		tmpResultBytes, err = json.Marshal(result)
		if err != nil {
			shared.RespondBadRequest(&w)
			return
		}
		var resultSize uint32
		var resultBytes []byte
		if string(tmpResultBytes) != "null" {
			resultBytes = tmpResultBytes
			resultSize = uint32(len(resultBytes))
		}
		shared.RespondOK(&w, &executeFunctionResponse{
			ExecutionTimeMilliseconds: duration,
			FunctionName:              req.FunctionName,
			FunctionResultSize:        resultSize,
			FunctionResult:            resultBytes,
		})
	} else {
		shared.RespondBadRequest(&w)
		return
	}
}
