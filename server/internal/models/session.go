package models

import (
	"net/http"
	"time"

	"github.com/luskaner/ageServer/server/internal"
)

type Session interface {
	Id() SessionKey
	GetUserId() int32
	GetClientLibVersion() uint16
	AddMessage(message internal.A)
	WaitForMessages(ackNum uint) (uint, []internal.A)
}

const sessionDuration = 5 * time.Minute

var sessionLetters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func generateSessionId() string {
	sessionId := make([]rune, 30)
	internal.WithRng(func(rand *internal.RandReader) {
		for j := range sessionId {
			sessionId[j] = sessionLetters[rand.IntN(len(sessionLetters))]
		}
	})
	return string(sessionId)
}

type SessionData struct {
	id               SessionKey
	clientLibVersion uint16
	userId           int32
	messageChan      chan internal.A
}

func (s *SessionData) Id() SessionKey {
	return s.id
}

func (s *SessionData) GetUserId() int32 {
	return s.userId
}

func (s *SessionData) GetClientLibVersion() uint16 {
	return s.clientLibVersion
}

func (s *SessionData) AddMessage(message internal.A) {
	// Non-blocking send: if the channel is full (client stopped polling),
	// drop the message instead of blocking the caller's goroutine forever.
	select {
	case s.messageChan <- message:
	default:
	}
}

func (s *SessionData) WaitForMessages(ackNum uint) (uint, []internal.A) {
	var results []internal.A
	timer := time.NewTimer(19 * time.Second)
	defer timer.Stop()

	for {
		select {
		case msg := <-s.messageChan:
			results = append(results, msg)
			for len(results) < cap(s.messageChan) {
				select {
				case msg = <-s.messageChan:
					results = append(results, msg)
				default:
					if len(results) > 0 {
						ackNum++
					}
					return ackNum, results
				}
			}
		case <-timer.C:
			return ackNum, results
		}
	}
}

type SessionKey = string

type Sessions interface {
	Create(userId int32, clientLibVersion uint16) string
	GetById(id string) (Session, bool)
	GetByUserId(userId int32) (Session, bool)
	Delete(id string)
	ResetExpiry(id string)
	Initialize()
}

type MainSessions struct {
	baseSessions *BaseSessions[SessionKey, SessionData]
}

func (s *MainSessions) Initialize() {
	s.baseSessions = NewBaseSessions[SessionKey, SessionData](sessionDuration)
}

func (s *MainSessions) Create(userId int32, clientLibVersion uint16) string {
	newId := generateSessionId()
	sess := &SessionData{
		id:               newId,
		userId:           userId,
		clientLibVersion: clientLibVersion,
		messageChan:      make(chan internal.A, 100),
	}
	s.baseSessions.CreateSession(func() string { return newId }, sess)
	return newId
}

func (s *MainSessions) GetById(id string) (Session, bool) {
	baseSess, exists := s.baseSessions.Get(id)
	if !exists {
		return nil, false
	}
	return baseSess.Data(), true
}

func (s *MainSessions) getFirstByCondition(fn func(sess Session) bool) (Session, bool) {
	for sess := range s.baseSessions.Values() {
		if data := sess.Data(); fn(sess.Data()) {
			return data, true
		}
	}
	return nil, false
}

func (s *MainSessions) GetByUserId(userId int32) (Session, bool) {
	for sess := range s.baseSessions.Values() {
		if data := sess.Data(); data.GetUserId() == userId {
			return data, true
		}
	}
	return nil, false
}

func (s *MainSessions) Delete(id string) {
	s.baseSessions.Delete(id)
}

func (s *MainSessions) ResetExpiry(id string) {
	s.baseSessions.ResetExpiryTimer(id)
}

func SessionOrPanic(r *http.Request) Session {
	sessAny, ok := session(r)
	if !ok {
		panic("Session should have been set already")
	}
	return sessAny
}

func session(r *http.Request) (Session, bool) {
	sess, ok := r.Context().Value("session").(Session)
	return sess, ok
}
