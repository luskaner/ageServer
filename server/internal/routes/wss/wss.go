package wss

import (
	"crypto/sha512"
	"encoding/json"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/lxzan/gws"
)

var connections = i.NewSafeMap[string, *gws.Conn]()

func computeData(data []byte) *wss.Data {
	var dataHash [64]byte
	if len(data) > 0 {
		dataHash = sha512.Sum512(data)
	}
	if len(data) > 4_096 {
		data = []byte{}
	}
	return &wss.Data{
		Body:     serverCommunication.Body{Body: data},
		BodyHash: serverCommunication.BodyHash{BodyHash: dataHash},
	}
}

func logJSON(sender string, receiver string, data any) {
	if logger.CommBuffer == nil {
		return
	}
	dataMarshalled, _ := json.Marshal(data)
	d := computeData(dataMarshalled)
	logger.CommBuffer.Log(new(wss.NewWrite(
		*d,
		serverCommunication.Uptime{Uptime: logger.Uptime(nil)},
		serverCommunication.Sender{Sender: sender},
		receiver,
	)))
}

func logClose(sender string, receiver string) {
	if logger.CommBuffer == nil {
		return
	}
	logger.CommBuffer.Log(new(wss.NewWrite(
		wss.Disconnection{},
		serverCommunication.Uptime{Uptime: logger.Uptime(nil)},
		serverCommunication.Sender{Sender: sender},
		receiver,
	)))
}

func logConnection(sender string, receiver string) {
	if logger.CommBuffer == nil {
		return
	}
	logger.CommBuffer.Log(new(wss.NewWrite(
		wss.Connection{},
		serverCommunication.Uptime{Uptime: logger.Uptime(nil)},
		serverCommunication.Sender{Sender: sender},
		receiver,
	)))
}

func parseMessage(sessions models.Sessions, message i.H, currentSession models.Session) (uint32, models.Session) {
	sess := models.Session(nil)
	op := uint32(0)
	rawOp, opOk := message["operation"]
	if !opOk {
		return 0, nil
	}
	opFloat, opOk := rawOp.(float64)
	if !opOk {
		return 0, nil
	}
	op = uint32(opFloat)
	if op == 0 {
		sessionToken, ok := message["sessionToken"]
		if ok {
			tokenStr, tokOk := sessionToken.(string)
			if !tokOk {
				return 0, nil
			}
			sess, ok = sessions.GetById(tokenStr)
			if ok {
				return 0, sess
			}
			return 0, nil
		}
	}
	if currentSession != nil {
		var ok bool
		sess, ok = sessions.GetById(currentSession.Id())
		if !ok {
			return 0, nil
		}
	}
	return op, sess
}

// writeMessage marshals and sends a JSON message over the WebSocket.
func writeMessage(socket *gws.Conn, v any) bool {
	data, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return socket.WriteMessage(gws.OpcodeText, data) == nil
}

func Handle(w http.ResponseWriter, r *http.Request) {
	sessions := models.G(r).Sessions()

	option := &gws.ServerOption{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		CheckUtf8Enabled: true,
	}
	upgrader := gws.NewUpgrader(&gws.BuiltinEventHandler{}, option)

	socket, err := upgrader.Upgrade(w, r)
	if err != nil {
		return
	}

	localAddr := socket.LocalAddr().String()
	remoteAddr := socket.RemoteAddr().String()
	logConnection(remoteAddr, localAddr)

	socket.SetReadDeadline(time.Now().Add(time.Minute))

	msg, err := socket.ReadMessage()
	if err != nil {
		writeClose(socket, "Missing initial message")
		_ = socket.NetConn().Close()
		return
	}

	var loginMsg i.H
	if err = json.Unmarshal(msg.Bytes(), &loginMsg); err != nil {
		writeClose(socket, "Invalid login message")
		_ = socket.NetConn().Close()
		return
	}

	_, sess := parseMessage(sessions, loginMsg, nil)
	if sess == nil {
		writeClose(socket, "Invalid login message data")
		_ = socket.NetConn().Close()
		return
	}

	sessionToken := sess.Id()
	connections.Store(sessionToken, socket, nil)
	sessions.ResetExpiry(sess.Id())

	// Keep session alive while the connection is open.
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sessions.ResetExpiry(sess.Id())
			case <-done:
				return
			}
		}
	}()

	defer func() {
		connections.Delete(sessionToken)
		logClose(localAddr, remoteAddr)
		_ = socket.WriteClose(1000, nil)
		_ = socket.NetConn().Close()
	}()

	var op uint32
	for {
		socket.SetReadDeadline(time.Now().Add(time.Minute))
		msg, err := socket.ReadMessage()
		if err != nil {
			break
		}

		var msgData i.H
		if err = json.Unmarshal(msg.Bytes(), &msgData); err != nil {
			break
		}
		logJSON(remoteAddr, localAddr, msgData)

		op, sess = parseMessage(sessions, msgData, sess)
		if op == 0 {
			if sess == nil {
				break
			}
			if sessId := sess.Id(); sessId != sessionToken {
				connections.StoreAndDelete(sessId, socket, sessionToken)
				sessionToken = sessId
			}
		} else if _, ok := sessions.GetById(sessionToken); !ok {
			break
		}
		sessions.ResetExpiry(sess.Id())
	}
}

func sendMessage(sessionId string, message i.A) bool {
	conn, ok := connections.Load(sessionId)
	if !ok {
		return false
	}
	data, err := json.Marshal(message)
	if err != nil {
		return false
	}
	return conn.WriteMessage(gws.OpcodeText, data) == nil
}

func SendOrStoreMessage(session models.Session, action string, message any) {
	finalMessage := i.A{0, action, session.GetUserId(), message}
	go func(session models.Session, finalMessage i.A) {
		if ok := sendMessage(session.Id(), finalMessage); !ok {
			session.AddMessage(finalMessage)
		}
	}(session, finalMessage)
}

func writeClose(socket *gws.Conn, reason string) {
	_ = socket.WriteClose(1000, []byte(reason))
}

func SendMatchNotifications(notifications []models.MatchNotification) {
	for _, notification := range notifications {
		if notification.Session == nil {
			continue
		}
		SendOrStoreMessage(notification.Session, notification.Action, notification.Message)
	}
}
