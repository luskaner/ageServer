package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// Issue #58: "Duplicate players in channels".
//
// The client builds the roster of a channel from two sources: the roster that
// joinChannel returns and the ChannelJoinMessage/ChannelLeaveMessage deltas it
// receives afterwards. The server must therefore announce every member to each
// client exactly once: either in the roster (members already there when the
// client joined) or in a delta (members that join/leave afterwards). It must
// also never send a delta for a channel the client is not in, otherwise the
// client counts that player in the roster and again when the delta arrives and
// the channel shows one player too many. The reported symptom (2 players, one
// client seeing 3 while the other sees 2) is exactly that asymmetry.

const testChannelID = 1

// chatChannelTestGame builds an AoE3 game with a single chat channel (id 1).
func chatChannelTestGame(t *testing.T) models.Game {
	t.Helper()
	return newTestGameWithResourceSetup(t, game.AoE3, func(f *fakeResources) {
		f.chatChannels = map[string]*models.MainChatChannel{
			fmt.Sprint(testChannelID): models.NewChatChannel(testChannelID, "General"),
		}
	})
}

func profileID(t *testing.T, raw any) int32 {
	t.Helper()
	info, ok := raw.(i.A)
	if !ok || len(info) < 2 {
		t.Fatalf("bad profileInfo %#v", raw)
	}
	num, ok := info[1].(float64)
	if !ok {
		t.Fatalf("bad profile id %#v", info[1])
	}
	return int32(num)
}

// joinChannelRoster returns the profile ids of the roster the client gets when
// it joins a channel ([0, "<channel>", 0, [[0, profileInfo], ...]]).
func joinChannelRoster(t *testing.T, body []byte) []int32 {
	t.Helper()
	var resp i.A
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("joinChannel response %q: %v", body, err)
	}
	if len(resp) < 4 {
		t.Fatalf("joinChannel response too short: %q", body)
	}
	if code, ok := resp[0].(float64); !ok || int(code) != 0 {
		t.Fatalf("joinChannel did not succeed: %q", body)
	}
	entries, ok := resp[3].(i.A)
	if !ok {
		t.Fatalf("joinChannel roster is not an array: %q", body)
	}
	ids := make([]int32, 0, len(entries))
	for _, entry := range entries {
		wrapped, ok := entry.(i.A)
		if !ok || len(wrapped) < 2 {
			t.Fatalf("bad roster entry %#v", entry)
		}
		ids = append(ids, profileID(t, wrapped[1]))
	}
	return ids
}

func joinChannel(t *testing.T, h http.Handler, g models.Game, sid string, channel int32) []int32 {
	t.Helper()
	rr := doRequest(h, "POST", "/game/chat/joinChannel", g, sid, fmt.Sprintf("chatroomID=%d", channel), nil)
	return joinChannelRoster(t, rr.Body.Bytes())
}

// readSessionMessages returns the messages that readSession hands to the
// client. A sentinel is queued first so the call never waits the 19s poll.
func readSessionMessages(t *testing.T, h http.Handler, g models.Game, sid string) i.A {
	t.Helper()
	sess, ok := g.Sessions().GetById(sid)
	if !ok {
		t.Fatalf("session %s not found", sid)
	}
	sess.AddMessage(i.A{0, "TestSentinel", sess.GetUserId(), i.A{}})
	rr := doRequest(h, "POST", "/game/login/readSession", g, sid, "ack=0", nil)
	body := strings.TrimSpace(rr.Body.String())
	idx := strings.Index(body, ",")
	if idx < 0 {
		t.Fatalf("unexpected readSession body %q", body)
	}
	// The wire format is "<ack>, [[message, ...]]": one extra array wraps the
	// list of pending messages.
	var envelope i.A
	if err := json.Unmarshal([]byte(body[idx+1:]), &envelope); err != nil {
		t.Fatalf("readSession body %q: %v", body, err)
	}
	if len(envelope) == 0 {
		return nil
	}
	messages, ok := envelope[0].(i.A)
	if !ok {
		t.Fatalf("readSession messages are not an array: %q", body)
	}
	return messages
}

// channelAnnouncements returns, for channelId, the profile ids announced by
// each channel delta action.
func channelAnnouncements(t *testing.T, messages i.A, channelId string) map[string][]int32 {
	t.Helper()
	announced := map[string][]int32{}
	for _, raw := range messages {
		msg, ok := raw.(i.A)
		if !ok || len(msg) < 4 {
			continue
		}
		action, _ := msg[1].(string)
		payload, ok := msg[3].(i.A)
		if !ok || len(payload) < 2 {
			continue
		}
		if id, _ := payload[0].(string); id != channelId {
			continue
		}
		switch action {
		case "ChannelJoinMessage":
			// ["<channel>", [0, profileInfo]]
			wrapped, ok := payload[1].(i.A)
			if !ok || len(wrapped) < 2 {
				continue
			}
			announced[action] = append(announced[action], profileID(t, wrapped[1]))
		case "ChannelLeaveMessage":
			// ["<channel>", profileInfo]
			announced[action] = append(announced[action], profileID(t, payload[1]))
		}
	}
	return announced
}

// TestChatChannelDeltasOnlyForMembers_Httptest covers the reproducible part of
// issue #58: a client that is logged in but has not joined the channel yet was
// told about every join of that channel. By the time it joined, the roster from
// joinChannel already contained those players, so the stale delta showed them a
// second time.
func TestChatChannelDeltasOnlyForMembers_Httptest(t *testing.T) {
	g := chatChannelTestGame(t)
	h := gameHandler(game.AoE3)
	uA, sidA := createUserAndSessionForGame(t, g, 76561198000001001, "playerA")
	uB, sidB := createUserAndSessionForGame(t, g, 76561198000001002, "playerB")

	// A enters multiplayer first and joins the general channel.
	if roster := joinChannel(t, h, g, sidA, testChannelID); len(roster) != 0 {
		t.Fatalf("first member should get an empty roster, got %v", roster)
	}

	// B is only logged in: it never joined the channel, so it must not be told
	// about anything happening inside it.
	if announced := channelAnnouncements(t, readSessionMessages(t, h, g, sidB), fmt.Sprint(testChannelID)); len(announced) != 0 {
		t.Fatalf("non-member B received channel deltas: %v", announced)
	}

	// B joins: A is already there, so A comes in the roster and nowhere else.
	roster := joinChannel(t, h, g, sidB, testChannelID)
	if len(roster) != 1 || roster[0] != uA.GetId() {
		t.Fatalf("B roster = %v, want exactly [%d]", roster, uA.GetId())
	}
	if announced := channelAnnouncements(t, readSessionMessages(t, h, g, sidB), fmt.Sprint(testChannelID)); len(announced) != 0 {
		t.Fatalf("B got deltas for players already in its roster: %v", announced)
	}

	// A is a member, so it must be told exactly once that B arrived.
	joins := channelAnnouncements(t, readSessionMessages(t, h, g, sidA), fmt.Sprint(testChannelID))["ChannelJoinMessage"]
	if len(joins) != 1 || joins[0] != uB.GetId() {
		t.Fatalf("A join deltas = %v, want exactly [%d]", joins, uB.GetId())
	}
}

// TestChatChannelConcurrentJoinsAnnounceMembersOnce_Httptest checks the
// invariant the client relies on while several players join at the same time:
// every other member is announced exactly once, either in the roster or in a
// delta, so the channel can never show duplicates.
func TestChatChannelConcurrentJoinsAnnounceMembersOnce_Httptest(t *testing.T) {
	const players = 4

	g := chatChannelTestGame(t)
	h := gameHandler(game.AoE3)
	userIDs := make([]int32, players)
	sids := make([]string, players)
	for n := range sids {
		user, sid := createUserAndSessionForGame(t, g, uint64(76561198000002000+n), fmt.Sprintf("player%d", n))
		userIDs[n] = user.GetId()
		sids[n] = sid
	}

	// A barrier makes the joins as simultaneous as the scheduler allows.
	start := make(chan struct{})
	var wg sync.WaitGroup
	bodies := make([][]byte, players)
	for n, sid := range sids {
		wg.Add(1)
		go func(n int, sid string) {
			defer wg.Done()
			<-start
			rr := doRequest(h, "POST", "/game/chat/joinChannel", g, sid, fmt.Sprintf("chatroomID=%d", testChannelID), nil)
			bodies[n] = append([]byte(nil), rr.Body.Bytes()...)
		}(n, sid)
	}
	close(start)
	wg.Wait()

	for n := range sids {
		roster := joinChannelRoster(t, bodies[n])
		announced := map[int32]int{}
		for _, id := range roster {
			announced[id]++
		}
		for _, ids := range channelAnnouncements(t, readSessionMessages(t, h, g, sids[n]), fmt.Sprint(testChannelID)) {
			for _, id := range ids {
				announced[id]++
			}
		}
		for other, id := range userIDs {
			if id == userIDs[n] {
				if announced[id] != 0 {
					t.Fatalf("player %d was announced to itself %d time(s)", n, announced[id])
				}
				continue
			}
			if got := announced[id]; got != 1 {
				t.Fatalf("player %d got player %d announced %d time(s), want exactly 1 (roster %v, announcements %v)",
					n, other, got, roster, announced)
			}
		}
	}
}

// TestChatChannelLeaveNotifiesMembersOnly_Httptest checks that leaving a
// channel is only announced to the players that are still inside it.
func TestChatChannelLeaveNotifiesMembersOnly_Httptest(t *testing.T) {
	g := chatChannelTestGame(t)
	h := gameHandler(game.AoE3)
	uA, sidA := createUserAndSessionForGame(t, g, 76561198000003001, "leaveA")
	uB, sidB := createUserAndSessionForGame(t, g, 76561198000003002, "leaveB")
	uC, sidC := createUserAndSessionForGame(t, g, 76561198000003003, "leaveC")
	_, sidD := createUserAndSessionForGame(t, g, 76561198000003004, "leaveD")

	for _, sid := range []string{sidA, sidB, sidC} {
		joinChannel(t, h, g, sid, testChannelID)
	}

	if rr := doRequest(h, "POST", "/game/chat/leaveChannel", g, sidC, fmt.Sprintf("chatroomID=%d", testChannelID), nil); !strings.HasPrefix(rr.Body.String(), "[0") {
		t.Fatalf("leaveChannel failed: %s", rr.Body.String())
	}

	for name, sid := range map[string]string{"A": sidA, "B": sidB} {
		leaves := channelAnnouncements(t, readSessionMessages(t, h, g, sid), fmt.Sprint(testChannelID))["ChannelLeaveMessage"]
		if len(leaves) != 1 || leaves[0] != uC.GetId() {
			t.Fatalf("member %s leave deltas = %v, want exactly [%d]", name, leaves, uC.GetId())
		}
	}

	// The leaving player must not be told about its own departure.
	if announced := channelAnnouncements(t, readSessionMessages(t, h, g, sidC), fmt.Sprint(testChannelID)); len(announced) != 0 {
		t.Fatalf("leaving player C got channel deltas: %v", announced)
	}

	// D never joined the channel, so it must not know about the departure.
	if announced := channelAnnouncements(t, readSessionMessages(t, h, g, sidD), fmt.Sprint(testChannelID)); len(announced) != 0 {
		t.Fatalf("non-member D got channel deltas: %v", announced)
	}

	_ = uA
	_ = uB
}

// TestChatChannelLoginDropsStaleMemberships_Httptest checks the other half of
// "Channels might show incorrect number of people inside": when the game is
// closed without logging out (crash, force close, expired session) the player
// stays counted in every channel. Logging in again has to drop those stale
// memberships, otherwise they keep the count too high and the new login cannot
// join the channel ("already a member") so the client never receives a roster.
func TestChatChannelLoginDropsStaleMemberships_Httptest(t *testing.T) {
	g := chatChannelTestGame(t)
	h := gameHandler(game.AoE3)
	const reloginPlatformId = 76561198000004001
	uA, sidA := createUserAndSessionForGame(t, g, reloginPlatformId, "reloginA")
	_, sidB := createUserAndSessionForGame(t, g, 76561198000004002, "reloginB")

	joinChannel(t, h, g, sidA, testChannelID)
	joinChannel(t, h, g, sidB, testChannelID)
	// Drain B so only the login notification can be left in its queue.
	readSessionMessages(t, h, g, sidB)

	channel, ok := g.ChatChannels().GetById(testChannelID)
	if !ok {
		t.Fatalf("channel %d not found", testChannelID)
	}
	if !channel.HasUser(uA) {
		t.Fatal("A should be a member after joining")
	}

	// A restarts the game: platformlogin must drop its stale membership.
	body := fmt.Sprintf("accountType=STEAM&platformUserID=%d&alias=reloginA&title=%s&macAddress=AA:BB:CC:DD:EE:FF&clientLibVersion=200",
		reloginPlatformId, game.AoE3)
	if rr := doRequest(h, "POST", "/game/login/platformlogin", g, "", body, nil); !strings.HasPrefix(rr.Body.String(), "[0") {
		t.Fatalf("platformlogin failed: %s", rr.Body.String())
	}
	if channel.HasUser(uA) {
		t.Fatal("stale channel membership survived the login")
	}

	leaves := channelAnnouncements(t, readSessionMessages(t, h, g, sidB), fmt.Sprint(testChannelID))["ChannelLeaveMessage"]
	if len(leaves) != 1 || leaves[0] != uA.GetId() {
		t.Fatalf("remaining member leave deltas = %v, want exactly [%d]", leaves, uA.GetId())
	}
}
