package shutdown

import (
	"net"
	"net/http"
	"os"
	"slices"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server/internal"
)

func Shutdown(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ips := common.ResolveUnspecifiedIps()
	if len(ips) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !slices.Contains(ips, ip) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	internal.StopSignal <- os.Interrupt
}
