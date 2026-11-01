package http

import (
	"net"
	"net/http"
	"sync"
	"time"

	internalClient "github.com/luskaner/ageServer/server-replay/internal/client"
)

var pool map[string]*http.Client
var mu sync.RWMutex

func init() {
	pool = make(map[string]*http.Client)
}

func GetOrNew(id string) *http.Client {
	mu.RLock()
	client, ok := pool[id]
	mu.RUnlock()
	if ok {
		return client
	}
	mu.Lock()
	client, ok = pool[id]
	if ok {
		mu.Unlock()
		return client
	}
	newClient := &http.Client{
		Timeout: time.Second * 45,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Second * 4,
				KeepAlive: time.Second * 30,
			}).DialContext,
			IdleConnTimeout: time.Second * 35,
			MaxConnsPerHost: 1,
			TLSClientConfig: internalClient.TlsClientConfig,
		},
	}
	pool[id] = newClient
	mu.Unlock()
	return newClient
}
