package router

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/initializer"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

// TestMaxBodySizeMiddleware_httptest valida que el middleware envuelve el body con
// MaxBytesReader y que un body que excede el límite produce un error de lectura en
// el siguiente handler.
func TestMaxBodySizeMiddleware_httptest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h := MaxBodySizeMiddleware(10, next)

	t.Run("WithinLimit", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/x", strings.NewReader("hello"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200 got %d", rr.Code)
		}
	})

	t.Run("ExceedsLimit", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/x", strings.NewReader("this body is longer than the ten byte limit"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413 got %d", rr.Code)
		}
	})

	// A path that only reaches a raised prefix before normalisation must keep
	// the global cap: the handler it lands on is not the one that would apply
	// its own, larger limit.
	t.Run("TraversalKeepsGlobalLimit", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/MatchContent/../x", strings.NewReader("this body is longer than the ten byte limit"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413 got %d", rr.Code)
		}
	})

	t.Run("RaisedPrefixSkipsLimit", func(t *testing.T) {
		for _, p := range []string{"/api/MatchContent/12", "/api/MatchContent/", "/api/MatchContent"} {
			req := httptest.NewRequest("POST", p, strings.NewReader("this body is longer than the ten byte limit"))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: want 200 got %d", p, rr.Code)
			}
		}
	})

	// A prefix that only shares its first characters is a different route.
	t.Run("SiblingPrefixKeepsLimit", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/MatchContentElsewhere/", strings.NewReader("this body is longer than the ten byte limit"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413 got %d", rr.Code)
		}
	})
}

// TestResponseWriterWrapper_httptest valida que el wrapper registra correctamente
// el código de estado y el body mientras reenvía al ResponseWriter subyacente.
func TestResponseWriterWrapper_httptest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello world"))
	})
	h := NewLoggingMiddleware(next)

	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201 got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "hello world" {
		t.Fatalf("body %q", body)
	}
}

// TestResponseWriterWrapperStatusDefault_httptest: sin WriteHeader explícito, el
// código por defecto debe ser 200 al final.
func TestResponseWriterWrapperStatusDefault_httptest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	h := NewLoggingMiddleware(next)

	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rr.Code)
	}
}

// TestPlayfabMiddleware_Auth_httptest cubre las ramas de autenticación del
// middleware Playfab: rutas anónimas pasan directo, rutas protegidas sin game
// Playfab (o sin token válido) devuelven 401 JSON, y las rutas estáticas pasan.
func TestPlayfabMiddleware_Auth_httptest(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	g := newTestGame(t, game.AoE4)

	t.Run("AnonymousPassthrough", func(t *testing.T) {
		nextCalled = false
		h := PlayfabMiddleware(next)
		req := httptest.NewRequest("POST", "/Client/LoginWithCustomID", strings.NewReader(`{"CustomId":"1"}`))
		req = req.WithContext(context.WithValue(req.Context(), "game", g))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if !nextCalled {
			t.Fatal("anonymous path should reach next")
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200 got %d", rr.Code)
		}
	})

	t.Run("ProtectedNoPlayfabGame401", func(t *testing.T) {
		nextCalled = false
		h := PlayfabMiddleware(next)
		req := httptest.NewRequest("POST", "/Client/GetPlayerCombinedInfo", strings.NewReader(`{}`))
		req = req.WithContext(context.WithValue(req.Context(), "game", g))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if nextCalled {
			t.Fatal("protected path should not reach next without valid playfab game/token")
		}
		// RespondError devuelve HTTP 200 con el código de PlayFab en el body JSON,
		// no un estado HTTP 401.
		body := rr.Body.String()
		if !strings.Contains(body, `"code":401`) {
			t.Fatalf("want playfab code 401 in body, got %q", body)
		}
		if !strings.Contains(body, "Invalid") {
			t.Fatalf("want 'Invalid' in body, got %q", body)
		}
	})

	t.Run("StaticPassthrough", func(t *testing.T) {
		nextCalled = false
		h := PlayfabMiddleware(next)
		req := httptest.NewRequest("GET", playfab.StaticSuffix+"/somefile.json", nil)
		req = req.WithContext(context.WithValue(req.Context(), "game", g))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if !nextCalled {
			t.Fatal("static path should reach next")
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200 got %d", rr.Code)
		}
	})
}

// fakePlayfabGame envuelve un models.Game añadiéndole playfabSessions para
// satisfacer la interfaz playfab.Game sin depender del modelo real de cada juego.
type fakePlayfabGame struct {
	models.Game
	sessions *playfab.MainSessions
}

func (f *fakePlayfabGame) PlayfabSessions() *playfab.MainSessions {
	return f.sessions
}

// newTestPlayfabGame crea un models.Game, un usuario y una sesión Playfab y
// devuelve el game fake junto con la clave de sesión (equivalente al SessionTicket).
func newTestPlayfabGame(t *testing.T, gameId string) (*fakePlayfabGame, models.User, string) {
	t.Helper()
	g := newTestGame(t, gameId)
	ps := &playfab.MainSessions{}
	ps.Initialize()

	var avatarDefs models.AvatarStatDefinitions
	if gameId != game.AoE1 {
		avatarDefs = g.LeaderboardDefinitions().AvatarStatDefinitions()
	}
	u := g.Users().GetOrCreateUser(gameId, g.Items(), avatarDefs, "127.0.0.3:1234", "AA:BB:CC:DD:EE:FF", false, 76561198000000123, "pfUser")
	if u == nil {
		t.Fatalf("failed to create playfab user")
	}
	key := ps.CreateWithUserId(g.Users(), u.GetId())
	return &fakePlayfabGame{Game: g, sessions: ps}, u, key
}

// TestPlayfabMiddleware_AuthToken_httptest verifica la rama positiva del
// middleware Playfab: con un token válido (X-Sessionticket en AoE4, X-Entitytoken
// en AoM) la ruta protegida llega al handler.
func TestPlayfabMiddleware_AuthToken_httptest(t *testing.T) {
	for _, tc := range []struct {
		gameId string
		header string
	}{
		{game.AoE4, "X-Sessionticket"},
		{game.AoM, "X-Entitytoken"},
	} {
		t.Run(tc.gameId, func(t *testing.T) {
			fg, _, key := newTestPlayfabGame(t, tc.gameId)
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})
			h := PlayfabMiddleware(next)
			req := httptest.NewRequest("POST", "/Client/GetPlayerCombinedInfo", strings.NewReader(`{}`))
			req.Header.Set(tc.header, key)
			req = req.WithContext(context.WithValue(req.Context(), "game", fg))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if !nextCalled {
				t.Fatalf("valid %s token should reach next", tc.header)
			}
			if rr.Code != http.StatusOK {
				t.Fatalf("want 200 got %d", rr.Code)
			}
		})
	}

	// Token inválido: no debe llegar al handler y debe devolver code 401 PlayFab.
	t.Run("InvalidToken", func(t *testing.T) {
		fg, _, _ := newTestPlayfabGame(t, game.AoE4)
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})
		h := PlayfabMiddleware(next)
		req := httptest.NewRequest("POST", "/Client/GetPlayerCombinedInfo", strings.NewReader(`{}`))
		req.Header.Set("X-Sessionticket", "does-not-exist")
		req = req.WithContext(context.WithValue(req.Context(), "game", fg))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if nextCalled {
			t.Fatal("invalid token should not reach next")
		}
		if !strings.Contains(rr.Body.String(), `"code":401`) {
			t.Fatalf("want playfab code 401, got %q", rr.Body.String())
		}
	})
}

// TestPlayfabLoginThenProtected_httptest cubre el flujo completo de autenticación
// PlayFab: LoginWithCustomID devuelve un SessionTicket que luego se reutiliza como
// header para acceder a un endpoint protegido.
func TestPlayfabLoginThenProtected_httptest(t *testing.T) {
	fg, u, _ := newTestPlayfabGame(t, game.AoE4)
	p := &PlayfabApi{}
	h := p.InitializeRoutes(game.AoE4, nil)

	// 1) Login: obtiene SessionTicket (clave de sesión).
	customID := strconv.Itoa(int(u.GetId()))
	loginReq := httptest.NewRequest("POST", "/Client/LoginWithCustomID", strings.NewReader(`{"CustomId":"`+customID+`","CreateAccount":true}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq = loginReq.WithContext(context.WithValue(loginReq.Context(), "game", fg))
	loginRr := httptest.NewRecorder()
	h.ServeHTTP(loginRr, loginReq)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			SessionTicket string `json:"SessionTicket"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login response not json: %v body=%q", err, loginRr.Body.String())
	}
	if resp.Code != 200 || resp.Data.SessionTicket == "" {
		t.Fatalf("login failed: code=%d body=%s", resp.Code, loginRr.Body.String())
	}

	// 2) Endpoint protegido con el SessionTicket obtenido.
	protReq := httptest.NewRequest("POST", "/Client/GetPlayerCombinedInfo", strings.NewReader(`{}`))
	protReq.Header.Set("Content-Type", "application/json")
	protReq.Header.Set("X-Sessionticket", resp.Data.SessionTicket)
	protReq = protReq.WithContext(context.WithValue(protReq.Context(), "game", fg))
	protRr := httptest.NewRecorder()
	h.ServeHTTP(protRr, protReq)

	var protResp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(protRr.Body.Bytes(), &protResp); err != nil {
		t.Fatalf("protected response not json: %v body=%q", err, protRr.Body.String())
	}
	if protResp.Code != 200 {
		t.Fatalf("protected endpoint want code 200 got %d body=%s", protResp.Code, protRr.Body.String())
	}
}

// TestAuthMiddlewareOffline_httptest cubre las dos ramas del modo "offline" de
// autenticación: credencial con deadline futuro válido deja pasar al siguiente
// handler; deadline vencido/cero devuelve el error de login PlayFab de plataforma.
func TestAuthMiddlewareOffline_httptest(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	u := g.Users().GetOrCreateUser(game.AoE2, g.Items(), g.LeaderboardDefinitions().AvatarStatDefinitions(), "127.0.0.1:1234", "00:11:22:33:44:55", false, 76561198000000099, "offlineUser")
	if u == nil {
		t.Fatal("failed to create user")
	}
	now := time.Now().UTC()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	makeRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/x", nil)
		ctx := context.WithValue(req.Context(), "user", u)
		ctx = context.WithValue(ctx, "time", now)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		AuthMiddlewareOffline(next).ServeHTTP(rr, req)
		return rr
	}

	t.Run("ValidDeadline", func(t *testing.T) {
		_ = u.GetAuth().WithReadWrite(func(d *time.Time) error {
			*d = now.Add(time.Hour)
			return nil
		})
		rr := makeRequest()
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200 got %d", rr.Code)
		}
		if rr.Body.String() != "ok" {
			t.Fatalf("want next called, body=%q", rr.Body.String())
		}
	})

	t.Run("ExpiredDeadline", func(t *testing.T) {
		_ = u.GetAuth().WithReadWrite(func(d *time.Time) error {
			*d = time.Time{}
			return nil
		})
		rr := makeRequest()
		// PlatformLoginError devuelve JSON [2, ...]
		var arr i.A
		if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
			t.Fatalf("expected json error, got %v body=%q", err, rr.Body.String())
		}
		if len(arr) == 0 || int(arr[0].(float64)) != 2 {
			t.Fatalf("want first=2 got body=%q", rr.Body.String())
		}
	})
}

// TestTitleMiddleware_httptest verifica que el middleware inyecta el juego del
// registro global initializer.Games en el contexto de la request.
func TestTitleMiddleware_httptest(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	initializer.Games[game.AoE2] = g
	defer delete(initializer.Games, game.AoE2)

	var captured models.Game
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = models.G(r)
		w.WriteHeader(http.StatusOK)
	})
	h := TitleMiddleware(game.AoE2, next)

	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rr.Code)
	}
	if captured == nil {
		t.Fatal("game not set in context")
	}
}

// TestSessionMiddleware_InvalidSession_httptest cubre la rama de SessionMiddleware
// donde el sessionID existe pero no corresponde a ninguna sesión válida.
func TestSessionMiddleware_InvalidSession_httptest(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := SessionMiddleware(next)

	req := httptest.NewRequest("GET", "/game/item/getItemLoadouts?sessionID=does-not-exist", nil)
	req = req.WithContext(context.WithValue(req.Context(), "game", g))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rr.Code)
	}
}

// TestLoginUserMiddleware_BindError_httptest cubre la rama de error de binding
// (dato con tipo inválido) que devuelve PlatformLoginError sin llegar al handler.
func TestLoginUserMiddleware_BindError_httptest(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	h := LoginUserMiddleware(next.ServeHTTP)

	// clientLibVersion es uint16; un valor no numérico produce error de schema.
	body := "accountType=STEAM&platformUserID=123&alias=x&title=age2&macAddress=00:11:22:33:44:55&clientLibVersion=notanumber"
	req := httptest.NewRequest("POST", "/game/login/platformlogin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), "game", g))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatal("bind error should not reach next")
	}
	var arr i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
		t.Fatalf("expected json error, got %v body=%q", err, rr.Body.String())
	}
	if len(arr) == 0 || int(arr[0].(float64)) != 2 {
		t.Fatalf("want first=2 got body=%q", rr.Body.String())
	}
}

// TestFormatDurationWithDays_httptest cubre la rama con días de formatDurationWithDays.
func TestFormatDurationWithDays_httptest(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"subday", 90 * time.Minute, "01:30:00"},
		{"exactDay", 25 * time.Hour, "1d 01:00:00"},
		{"seconds", 30 * time.Second, "00:00:30"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDurationWithDays(tc.d); got != tc.want {
				t.Fatalf("formatDurationWithDays(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
