package chat

import (
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

// notifyMembers delivers a channel membership delta to the given members.
//
// A client builds the roster of a channel from the roster joinChannel returns
// plus the ChannelJoinMessage/ChannelLeaveMessage deltas it receives afterwards,
// so a delta must only reach the clients that are already inside the channel.
// Telling a player about a channel it has not joined makes it count that user
// twice: once from the roster it gets when it finally joins and once from the
// stale delta that was queued in the meantime (issue #58, "Duplicate players in
// channels").
//
// The members are the ones observed in the same critical section as the
// membership change: for a join, the members that were there before the
// newcomer (they cannot know about it yet); for a leave, the members that are
// left (they still have the leaving player in their roster).
func notifyMembers(sessions models.Sessions, members []models.User, action string, encode func(clientLibVersion uint16) i.A) {
	for _, member := range members {
		memberSession, ok := sessions.GetByUserId(member.GetId())
		if !ok {
			continue
		}
		// Encode per receiver: the profile info layout depends on the client
		// library version of the client that is going to parse it.
		wss.SendOrStoreMessage(memberSession, action, encode(memberSession.GetClientLibVersion()))
	}
}

// NotifyJoinChannel tells the members that were already inside the channel that
// user has joined it.
func NotifyJoinChannel(sessions models.Sessions, members []models.User, user models.User, chatChannelIdStr string) {
	notifyMembers(sessions, members, "ChannelJoinMessage", func(clientLibVersion uint16) i.A {
		return i.A{chatChannelIdStr, i.A{0, user.EncodeProfileInfo(clientLibVersion)}}
	})
}

// NotifyLeaveChannel tells the members that are still inside the channel that
// user has left it.
func NotifyLeaveChannel(sessions models.Sessions, members []models.User, user models.User, chatChannelId int32) {
	chatChannelIdStr := strconv.Itoa(int(chatChannelId))
	notifyMembers(sessions, members, "ChannelLeaveMessage", func(clientLibVersion uint16) i.A {
		return i.A{chatChannelIdStr, user.EncodeProfileInfo(clientLibVersion)}
	})
}
