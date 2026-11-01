package playfab

import (
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/uuid"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

const sessionDuration = 24 * time.Hour

type SessionKey = string

type SessionData struct {
	playfabId string
	token     string
	user      models.User
}

func (s *SessionData) PlayfabId() string {
	return s.playfabId
}

func (s *SessionData) Token() string {
	return s.token
}

func (s *SessionData) User() models.User {
	return s.user
}

type MainSessions struct {
	baseSessions *models.BaseSessions[SessionKey, SessionData]
}

func (s *MainSessions) Initialize() {
	s.baseSessions = models.NewBaseSessions[SessionKey, SessionData](sessionDuration)
}

func (s *MainSessions) create(user models.User) SessionKey {
	sess := &SessionData{
		token: uuid.New().String(),
		user:  user,
	}
	stored := s.baseSessions.CreateSession(generateId, sess)
	sess.playfabId = stored.Id()
	return sess.playfabId
}

func (s *MainSessions) CreateWithSteamUserId(users models.Users, steamUserId uint64) SessionKey {
	if user, found := users.GetUserByPlatformUserId(false, steamUserId); !found {
		return ""
	} else {
		return s.create(user)
	}
}

func (s *MainSessions) CreateWithUserId(users models.Users, userId int32) SessionKey {
	if user, found := users.GetUserById(userId); !found {
		return ""
	} else {
		return s.create(user)
	}
}

func (s *MainSessions) GetById(entityToken string) (*SessionData, bool) {
	baseSess, exists := s.baseSessions.Get(entityToken)
	if !exists {
		return nil, false
	}
	return baseSess.Data(), true
}

func (s *MainSessions) ResetExpiry(entityToken string) {
	s.baseSessions.ResetExpiryTimer(entityToken)
}

func generateId() string {
	bytes := make([]byte, 8)
	internal.WithRng(func(rand *internal.RandReader) {
		binary.BigEndian.PutUint64(bytes, rand.Uint64())
	})
	return hex.EncodeToString(bytes)
}

func SessionOrPanic(r *http.Request) *SessionData {
	sessAny, ok := session(r)
	if !ok {
		panic("Session should have been set already")
	}
	return sessAny
}

func session(r *http.Request) (*SessionData, bool) {
	sess, ok := r.Context().Value("session").(*SessionData)
	return sess, ok
}
