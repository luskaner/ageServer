package router

import (
	"net/http"
	"strings"

	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/community"
)

type ApiCommunity struct{}

func (a *ApiCommunity) Name() string {
	return "apiCommunity"
}

func (a *ApiCommunity) Initialize(_ string) bool {
	return true
}

func (a *ApiCommunity) Check(r *http.Request) bool {
	return r.URL.Path == "/community" || strings.HasPrefix(r.URL.Path, "/community/")
}

func (a *ApiCommunity) InitializeRoutes(gameId string, _ http.Handler) http.Handler {
	if !internal.CommunityAPIEnabled {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add CORS headers even for 404
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
			http.NotFound(w, r)
		})
	}

	mux := http.NewServeMux()
	communityGroup := &Group{mux: mux}
	community.Register(communityGroup)

	// Wrap with CORS handler for ALL /community/* responses
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers to ALL responses
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")

		// Handle CORS preflight
		if r.Method == http.MethodOptions && (r.URL.Path == "/community" || strings.HasPrefix(r.URL.Path, "/community/")) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Handle actual requests
		mux.ServeHTTP(w, r)
	})
}

func NewApiCommunity() *ApiCommunity {
	return &ApiCommunity{}
}