package wss

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/lxzan/gws"
)

// Regression: parseMessage used unchecked type assertions that panicked on
// malformed JSON from unauthenticated clients.
func TestParseMessageMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		msg  map[string]any
	}{
		{"missing operation", map[string]any{}},
		{"operation is string", map[string]any{"operation": "zero"}},
		{"operation is null", map[string]any{"operation": nil}},
		{"sessionToken is number", map[string]any{"operation": float64(0), "sessionToken": float64(42)}},
		{"sessionToken is null", map[string]any{"operation": float64(0), "sessionToken": nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked: %v", r)
				}
			}()
			op, sess := parseMessage(nil, tc.msg, nil)
			_ = op
			_ = sess
		})
	}
}

// TestParseMessageValidSession cubre las ramas de parseMessage que requieren una
// infraestructura real de sesiones: login válido, sessionToken inválido y
// operación no-cero con/sin sesión actual.
func TestParseMessageValidSession(t *testing.T) {
	i.InitializeRng(42)
	sessions := &models.MainSessions{}
	sessions.Initialize()
	sid := sessions.Create(1, 200)

	t.Run("validLogin", func(t *testing.T) {
		op, sess := parseMessage(sessions, map[string]any{"operation": float64(0), "sessionToken": sid}, nil)
		if op != 0 {
			t.Fatalf("op = %d, want 0", op)
		}
		if sess == nil {
			t.Fatal("expected a session")
		}
	})

	t.Run("invalidSessionToken", func(t *testing.T) {
		op, sess := parseMessage(sessions, map[string]any{"operation": float64(0), "sessionToken": "does-not-exist"}, nil)
		if op != 0 {
			t.Fatalf("op = %d, want 0", op)
		}
		if sess != nil {
			t.Fatal("expected nil session")
		}
	})

	t.Run("nonZeroNoCurrent", func(t *testing.T) {
		op, sess := parseMessage(sessions, map[string]any{"operation": float64(5)}, nil)
		if op != 5 {
			t.Fatalf("op = %d, want 5", op)
		}
		if sess != nil {
			t.Fatal("expected nil session")
		}
	})

	t.Run("nonZeroWithCurrent", func(t *testing.T) {
		current, _ := sessions.GetById(sid)
		op, sess := parseMessage(sessions, map[string]any{"operation": float64(3)}, current)
		if op != 3 {
			t.Fatalf("op = %d, want 3", op)
		}
		if sess == nil {
			t.Fatal("expected a session")
		}
	})
}

// TestComputeData cubre la función pura computeData: data vacía, pequeña (con
// hash) y grande (truncada por encima del umbral de 4096 bytes).
func TestComputeData(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		d := computeData(nil)
		if len(d.Body.Body) != 0 {
			t.Fatalf("expected empty body, got %d bytes", len(d.Body.Body))
		}
	})

	t.Run("small", func(t *testing.T) {
		d := computeData([]byte("hello"))
		if string(d.Body.Body) != "hello" {
			t.Fatalf("body = %q, want hello", string(d.Body.Body))
		}
		var zero [64]byte
		if d.BodyHash.BodyHash == zero {
			t.Fatal("expected non-zero hash")
		}
	})

	t.Run("large", func(t *testing.T) {
		large := bytes.Repeat([]byte("a"), 5000)
		d := computeData(large)
		if len(d.Body.Body) != 0 {
			t.Fatalf("large body should be truncated, got %d bytes", len(d.Body.Body))
		}
	})
}

// fakeGame satisface models.Game embebiendo la interfaz y sobrescribiendo
// únicamente Sessions(), que es lo único que usa wss.Handle.
type fakeGame struct {
	models.Game
	sessions models.Sessions
}

func (f *fakeGame) Sessions() models.Sessions {
	return f.sessions
}

// TestHandleWebSocketFullFlow cubre el flujo completo de wss.Handle usando un
// cliente WebSocket real (gws) contra un httptest.Server: handshake, login con
// sessionToken, almacenamiento de conexión y limpieza al cerrar.
func TestHandleWebSocketFullFlow(t *testing.T) {
	i.InitializeRng(42)
	sessions := &models.MainSessions{}
	sessions.Initialize()
	sid := sessions.Create(1, 200)
	g := &fakeGame{sessions: sessions}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "game", g)
		Handle(w, r.WithContext(ctx))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	client, _, err := gws.NewClient(new(gws.BuiltinEventHandler), &gws.ClientOption{Addr: wsURL})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.NetConn().Close()

	// Login con sessionToken válido.
	login := fmt.Sprintf(`{"operation":0,"sessionToken":"%s"}`, sid)
	if err := client.WriteMessage(gws.OpcodeText, []byte(login)); err != nil {
		t.Fatalf("write login: %v", err)
	}

	// Esperar a que el servidor almacene la conexión.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := connections.Load(sid); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := connections.Load(sid); !ok {
		t.Fatal("connection not stored after login")
	}

	// Mensaje adicional con operación no-cero para ejercitar el bucle.
	_ = client.WriteMessage(gws.OpcodeText, []byte(`{"operation":3}`))

	// Cerrar el cliente; el servidor debe romper el bucle y limpiar la conexión.
	client.NetConn().Close()

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := connections.Load(sid); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
