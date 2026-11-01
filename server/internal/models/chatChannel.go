package models

import (
	"iter"
	"strconv"

	"github.com/luskaner/ageServer/server/internal"
)

type ChatChannel interface {
	GetId() int32
	GetName() string
	GetUsers() iter.Seq[User]
	AddUser(user User, clientLibVersion uint16) (exists bool, encodedUsers internal.A)
	RemoveUser(user User) bool
	HasUser(user User) bool
	Encode() internal.A
}

type MainChatChannel struct {
	Id    int32
	Name  string
	users *internal.SafeOrderedMap[int32, User]
}

func NewChatChannel(id int32, name string) *MainChatChannel {
	return &MainChatChannel{
		Id:    id,
		Name:  name,
		users: internal.NewSafeOrderedMap[int32, User](),
	}
}

func (channel *MainChatChannel) GetId() int32 {
	return channel.Id
}

func (channel *MainChatChannel) GetName() string {
	return channel.Name
}

func (channel *MainChatChannel) Encode() internal.A {
	return internal.A{
		channel.Id,
		channel.Name,
		"",
		channel.users.Len(),
	}
}

func (channel *MainChatChannel) GetUsers() iter.Seq[User] {
	return func(yield func(user User) bool) {
		_, users := channel.users.Values()
		for v := range users {
			if !yield(v) {
				return
			}
		}
	}
}

func (channel *MainChatChannel) AddUser(user User, clientLibVersion uint16) (exists bool, encodedUsers internal.A) {
	exists, _ = channel.users.IterAndStore(user.GetId(), user, nil, func(length int, users iter.Seq2[int32, User]) {
		i := 0
		encodedUsers = make(internal.A, length)
		for _, el := range users {
			encodedUsers[i] = internal.A{0, el.EncodeProfileInfo(clientLibVersion)}
			i++
		}
	})
	return
}

func (channel *MainChatChannel) RemoveUser(user User) bool {
	return channel.users.Delete(user.GetId())
}

func (channel *MainChatChannel) HasUser(user User) bool {
	_, ok := channel.users.Load(user.GetId())
	return ok
}

type ChatChannels interface {
	Initialize(chatChannels map[string]*MainChatChannel)
	Encode() internal.A
	GetById(id int32) (*MainChatChannel, bool)
	Iter() iter.Seq2[int32, *MainChatChannel]
}

type MainChatChannels struct {
	index *internal.ReadOnlyOrderedMap[int32, *MainChatChannel]
}

func (channels *MainChatChannels) Initialize(chatChannels map[string]*MainChatChannel) {
	keys := make([]int32, len(chatChannels))
	values := make(map[int32]*MainChatChannel, len(chatChannels))
	j := 0
	for id, channel := range chatChannels {
		idInt, err := strconv.ParseInt(id, 10, 32)
		if err != nil {
			panic(err)
		}
		c := NewChatChannel(int32(idInt), channel.GetName())
		keys[j] = c.GetId()
		values[c.GetId()] = c
		j++
	}
	channels.index = internal.NewReadOnlyOrderedMap[int32, *MainChatChannel](keys, values)
}

func (channels *MainChatChannels) Encode() internal.A {
	c := make(internal.A, channels.index.Len())
	i := 0
	for el := range channels.index.Values() {
		c[i] = el.Encode()
		i++
	}
	return c
}

func (channels *MainChatChannels) GetById(id int32) (*MainChatChannel, bool) {
	return channels.index.Load(id)
}

func (channels *MainChatChannels) Iter() iter.Seq2[int32, *MainChatChannel] {
	return channels.index.Iter()
}
