package wss

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	internalClient "github.com/luskaner/ageServer/server-replay/internal/client"
)

var dialer *websocket.Dialer
var pool map[string]*websocket.Conn
var mu sync.RWMutex

func init() {
	dialer = &websocket.Dialer{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: 5 * time.Second,
		TLSClientConfig:  internalClient.TlsClientConfig,
	}
	pool = make(map[string]*websocket.Conn)
}

func New(key string, url string, header http.Header) error {
	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	pool[key] = conn
	return nil
}

func Get(id string) *websocket.Conn {
	mu.RLock()
	client, ok := pool[id]
	mu.RUnlock()
	if ok {
		return client
	}
	return nil
}
